package addon

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager handles addon discovery, installation, and management.
type Manager struct {
	DataDir    string // ~/.local/share/neocron-launcher/addons/
	InstallDir string // game install directory
	Logger     *log.Logger

	mu sync.Mutex
}

// NewManager creates a new addon manager.
func NewManager(installDir string) *Manager {
	dataDir := DefaultDataDir()
	os.MkdirAll(dataDir, 0755)

	// Open log file for addon operations
	logPath := filepath.Join(dataDir, "addon.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	var logger *log.Logger
	if err == nil {
		logger = log.New(logFile, "", log.LstdFlags)
	} else {
		logger = log.New(os.Stderr, "[addon] ", log.LstdFlags)
	}

	return &Manager{
		DataDir:    dataDir,
		InstallDir: installDir,
		Logger:     logger,
	}
}

func (m *Manager) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if m.Logger != nil {
		m.Logger.Println(msg)
	}
}

// ListInstalled returns all installed addons.
func (m *Manager) ListInstalled() ([]InstalledAddon, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := loadState(m.DataDir)
	if err != nil {
		return nil, err
	}
	return s.Addons, nil
}

// InstallFromRepo downloads and installs an addon from a GitHub repo URL.
func (m *Manager) InstallFromRepo(repoURL string, onProgress func(DownloadProgress)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	report := func(p DownloadProgress) {
		m.log("[%s] %.0f%% %s", p.Status, p.Percent, p.Message)
		if onProgress != nil {
			onProgress(p)
		}
	}

	// Normalize repo URL
	repoURL = strings.TrimRight(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Extract owner/repo from URL
	ownerRepo := extractOwnerRepo(repoURL)
	if ownerRepo == "" {
		return fmt.Errorf("invalid GitHub URL: %s", repoURL)
	}

	m.log("InstallFromRepo: %s (owner/repo: %s, installDir: %s)", repoURL, ownerRepo, m.InstallDir)
	report(DownloadProgress{Status: "downloading", Percent: 0, Message: "Downloading addon..."})

	// Download repo as tarball via GitHub API
	tarURL := fmt.Sprintf("https://api.github.com/repos/%s/tarball", ownerRepo)
	m.log("Downloading tarball: %s", tarURL)
	tmpDir, err := os.MkdirTemp(m.DataDir, "addon-download-*")
	if err != nil {
		os.MkdirAll(m.DataDir, 0755)
		tmpDir, err = os.MkdirTemp(m.DataDir, "addon-download-*")
		if err != nil {
			m.log("ERROR: create temp dir: %v", err)
			return fmt.Errorf("create temp dir: %w", err)
		}
	}
	defer os.RemoveAll(tmpDir)

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := downloadAndExtractTarball(tarURL, extractDir, func(pct float64, msg string) {
		report(DownloadProgress{Status: "downloading", Percent: pct, Message: msg})
	}); err != nil {
		m.log("ERROR: download: %v", err)
		return fmt.Errorf("download repo: %w", err)
	}
	m.log("Download and extraction complete")

	// Find the extracted root (GitHub tarballs have a root dir like owner-repo-sha/)
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return fmt.Errorf("read extract dir: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("empty archive — no files extracted")
	}
	// Find the first directory entry (the repo root)
	repoRoot := ""
	for _, e := range entries {
		if e.IsDir() {
			repoRoot = filepath.Join(extractDir, e.Name())
			break
		}
	}
	if repoRoot == "" {
		repoRoot = filepath.Join(extractDir, entries[0].Name())
	}
	m.log("Repo root: %s", repoRoot)

	// Parse addon.json
	report(DownloadProgress{Status: "extracting", Percent: 60, Message: "Reading addon manifest..."})

	manifestPath := filepath.Join(repoRoot, "addon.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("addon.json not found in repo: %w", err)
	}

	var manifest AddonManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("parse addon.json: %w", err)
	}

	if manifest.ID == "" {
		return fmt.Errorf("addon.json missing 'id' field")
	}
	m.log("Manifest: id=%s name=%s version=%s files=%d", manifest.ID, manifest.Name, manifest.Version, len(manifest.Files))

	// Check if already installed
	s, err := loadState(m.DataDir)
	if err != nil {
		return err
	}
	for _, a := range s.Addons {
		if a.ID == manifest.ID {
			return fmt.Errorf("addon '%s' is already installed (version %s)", manifest.Name, a.Version)
		}
	}

	report(DownloadProgress{Status: "installing", Percent: 70, Message: "Caching addon files..."})

	// Cache addon files
	cacheDir := addonFilesDir(m.DataDir, manifest.ID)
	os.RemoveAll(cacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	// Copy addon files to cache
	totalEntries := len(manifest.Files)
	for idx, fe := range manifest.Files {
		pct := 70.0 + (float64(idx)/float64(totalEntries))*10.0
		srcPath := filepath.Join(repoRoot, filepath.FromSlash(fe.Src))
		cacheDst := filepath.Join(cacheDir, filepath.FromSlash(fe.Dst))
		m.log("Caching [%d/%d]: %s -> %s", idx+1, totalEntries, srcPath, cacheDst)

		// Check if source exists
		if _, serr := os.Stat(srcPath); serr != nil {
			m.log("SKIP: source not found: %s (%v)", srcPath, serr)
			report(DownloadProgress{Status: "installing", Percent: pct, Message: fmt.Sprintf("Skipping %s (not found)...", fe.Src)})
			continue
		}

		report(DownloadProgress{Status: "installing", Percent: pct, Message: fmt.Sprintf("Caching %s...", fe.Dst)})
		if err := copyTree(srcPath, cacheDst); err != nil {
			return fmt.Errorf("cache addon files: %w", err)
		}
	}

	// Backup originals and apply addon files
	report(DownloadProgress{Status: "installing", Percent: 80, Message: "Applying to game directory..."})
	backupDir := addonBackupDir(m.DataDir, manifest.ID)
	os.MkdirAll(backupDir, 0755)

	if err := m.applyAddon(manifest.ID, cacheDir, backupDir); err != nil {
		return fmt.Errorf("apply addon: %w", err)
	}
	report(DownloadProgress{Status: "installing", Percent: 95, Message: "Saving state..."})

	// Save state
	s.Addons = append(s.Addons, InstalledAddon{
		ID:          manifest.ID,
		RepoURL:     repoURL,
		Version:     manifest.Version,
		Enabled:     true,
		InstalledAt: time.Now(),
		Manifest:    manifest,
	})

	if err := saveState(m.DataDir, s); err != nil {
		return err
	}

	report(DownloadProgress{Status: "done", Percent: 100, Message: fmt.Sprintf("'%s' v%s installed", manifest.Name, manifest.Version)})
	return nil
}

// Uninstall removes an addon and restores original files.
func (m *Manager) Uninstall(addonID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.uninstallLocked(addonID)
}

func (m *Manager) uninstallLocked(addonID string) error {
	m.log("Uninstall: %s", addonID)

	s, err := loadState(m.DataDir)
	if err != nil {
		return err
	}

	idx := -1
	for i, a := range s.Addons {
		if a.ID == addonID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("addon '%s' not found", addonID)
	}

	addon := s.Addons[idx]

	// Restore backups if enabled
	if addon.Enabled {
		backupDir := addonBackupDir(m.DataDir, addonID)
		m.log("Restoring backups from %s", backupDir)
		m.restoreBackup(backupDir)
	}

	// Remove addon data
	addonPath := addonDir(m.DataDir, addonID)
	m.log("Removing addon data: %s", addonPath)
	os.RemoveAll(addonPath)

	// Remove from state
	s.Addons = append(s.Addons[:idx], s.Addons[idx+1:]...)
	return saveState(m.DataDir, s)
}

// Enable re-applies addon files from cache.
func (m *Manager) Enable(addonID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.log("Enable: %s", addonID)

	s, err := loadState(m.DataDir)
	if err != nil {
		return err
	}

	for i, a := range s.Addons {
		if a.ID == addonID {
			if a.Enabled {
				return nil
			}

			cacheDir := addonFilesDir(m.DataDir, addonID)
			backupDir := addonBackupDir(m.DataDir, addonID)

			// Check cache dir exists and has files
			if _, serr := os.Stat(cacheDir); os.IsNotExist(serr) {
				return fmt.Errorf("addon cache missing — reinstall '%s'", addonID)
			}

			os.MkdirAll(backupDir, 0755)

			m.log("Applying addon from cache: %s", cacheDir)
			if err := m.applyAddon(addonID, cacheDir, backupDir); err != nil {
				return fmt.Errorf("apply: %w", err)
			}

			s.Addons[i].Enabled = true
			return saveState(m.DataDir, s)
		}
	}
	return fmt.Errorf("addon '%s' not found", addonID)
}

// Disable restores original files from backup.
func (m *Manager) Disable(addonID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.log("Disable: %s", addonID)

	s, err := loadState(m.DataDir)
	if err != nil {
		return err
	}

	for i, a := range s.Addons {
		if a.ID == addonID {
			if !a.Enabled {
				return nil
			}

			backupDir := addonBackupDir(m.DataDir, addonID)
			m.log("Restoring backups from %s", backupDir)
			m.restoreBackup(backupDir)

			s.Addons[i].Enabled = false
			return saveState(m.DataDir, s)
		}
	}
	return fmt.Errorf("addon '%s' not found", addonID)
}

// Update re-downloads and re-applies an addon.
func (m *Manager) Update(addonID string, onProgress func(DownloadProgress)) error {
	m.mu.Lock()

	m.log("Update: %s", addonID)

	s, err := loadState(m.DataDir)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	var repoURL string
	for _, a := range s.Addons {
		if a.ID == addonID {
			repoURL = a.RepoURL
			break
		}
	}
	if repoURL == "" {
		m.mu.Unlock()
		return fmt.Errorf("addon '%s' not found", addonID)
	}

	// Uninstall old version (already holding lock)
	if err := m.uninstallLocked(addonID); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("remove old version: %w", err)
	}

	// Release lock before InstallFromRepo (which takes its own lock)
	m.mu.Unlock()

	// Install new version
	return m.InstallFromRepo(repoURL, onProgress)
}

// CheckUpdates queries GitHub for newer versions of installed addons.
func (m *Manager) CheckUpdates() ([]AddonUpdate, error) {
	s, err := loadState(m.DataDir)
	if err != nil {
		return nil, err
	}

	var updates []AddonUpdate
	for _, a := range s.Addons {
		ownerRepo := extractOwnerRepo(a.RepoURL)
		if ownerRepo == "" {
			continue
		}

		latest, err := fetchLatestTag(ownerRepo)
		if err != nil || latest == "" {
			continue
		}

		if latest != a.Version {
			updates = append(updates, AddonUpdate{
				AddonID:        a.ID,
				CurrentVersion: a.Version,
				LatestVersion:  latest,
				RepoURL:        a.RepoURL,
			})
		}
	}
	return updates, nil
}

// --- Internal helpers ---

// applyAddon copies files from cache to the game directory, backing up originals.
func (m *Manager) applyAddon(addonID, cacheDir, backupDir string) error {
	count := 0
	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		// Skip .gitkeep placeholder files
		if info.Name() == ".gitkeep" {
			return nil
		}

		rel, err := filepath.Rel(cacheDir, path)
		if err != nil {
			return err
		}

		gamePath := filepath.Join(m.InstallDir, rel)
		backupPath := filepath.Join(backupDir, rel)

		// Backup original if it exists and we haven't already
		if _, err := os.Stat(gamePath); err == nil {
			if _, err := os.Stat(backupPath); os.IsNotExist(err) {
				os.MkdirAll(filepath.Dir(backupPath), 0755)
				if cerr := copyFile(gamePath, backupPath); cerr != nil {
					m.log("  backup failed: %s -> %s: %v", gamePath, backupPath, cerr)
				}
			}
		}

		// Copy addon file to game dir
		os.MkdirAll(filepath.Dir(gamePath), 0755)
		if cerr := copyFile(path, gamePath); cerr != nil {
			m.log("  apply failed: %s -> %s: %v", path, gamePath, cerr)
			return cerr
		}
		count++
		return nil
	})
	m.log("applyAddon: %s — %d files applied", addonID, count)
	return err
}

// restoreBackup copies backed-up originals back to the game directory.
func (m *Manager) restoreBackup(backupDir string) {
	count := 0
	filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(backupDir, path)
		gamePath := filepath.Join(m.InstallDir, rel)
		if cerr := copyFile(path, gamePath); cerr != nil {
			m.log("  restore failed: %s -> %s: %v", path, gamePath, cerr)
		} else {
			count++
		}
		return nil
	})
	m.log("restoreBackup: %d files restored", count)
}

func extractOwnerRepo(repoURL string) string {
	repoURL = strings.TrimRight(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Handle https://github.com/owner/repo or git@github.com:owner/repo
	if strings.Contains(repoURL, "github.com/") {
		parts := strings.SplitN(repoURL, "github.com/", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	if strings.Contains(repoURL, "github.com:") {
		parts := strings.SplitN(repoURL, "github.com:", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

func fetchLatestTag(ownerRepo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", ownerRepo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	json.NewDecoder(resp.Body).Decode(&release)
	return release.TagName, nil
}

func downloadAndExtractTarball(url, destDir string, onProgress func(float64, string)) error {
	if onProgress != nil {
		onProgress(5, "Downloading archive...")
	}

	// Use plain http.Get which follows redirects automatically.
	// The GitHub API tarball endpoint (302) redirects to a CDN URL.
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, resp.Request.URL.Host, string(body))
	}

	if onProgress != nil {
		onProgress(10, fmt.Sprintf("Downloading (%s)...", resp.Request.URL.Host))
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("not a gzip archive (Content-Type: %s): %w", resp.Header.Get("Content-Type"), err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	if onProgress != nil {
		onProgress(30, "Extracting files...")
	}

	fileCount := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		target := filepath.Join(destDir, header.Name)

		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			io.Copy(f, tr)
			f.Close()
			fileCount++
			if onProgress != nil && fileCount%100 == 0 {
				pct := 30.0 + float64(fileCount)/100.0 // rough progress
				if pct > 54 {
					pct = 54
				}
				onProgress(pct, fmt.Sprintf("Extracting... %d files", fileCount))
			}
		}
	}

	if onProgress != nil {
		onProgress(55, fmt.Sprintf("Extracted %d files", fileCount))
	}
	return nil
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(src, path)
			return copyFile(path, filepath.Join(dst, rel))
		})
	}

	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	os.MkdirAll(filepath.Dir(dst), 0755)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
