package updater

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"launcher/pkg/pak"
)

// ErrNotFound is returned when a file does not exist on the CDN (HTTP 404).
var ErrNotFound = errors.New("file not found on server")

// Progress tracks the state of an update/install operation.
type Progress struct {
	TotalFiles   int     `json:"totalFiles"`
	CurrentFile  int     `json:"currentFile"`
	CurrentName  string  `json:"currentName"`
	Percent      float64 `json:"percent"`
	BytesTotal   int64   `json:"bytesTotal"`
	BytesDone    int64   `json:"bytesDone"`
	Speed        float64 `json:"speed"`        // bytes per second
	SkippedFiles int     `json:"skippedFiles"` // files that 404'd on the CDN
	Status       string  `json:"status"`       // "checking", "downloading", "installing", "done", "error", "paused"
	ErrorMessage string  `json:"errorMessage,omitempty"`
}

// UpdateCheckResult is returned by CheckForUpdate.
type UpdateCheckResult struct {
	NeedsUpdate   bool   `json:"needsUpdate"`
	ServerVersion string `json:"serverVersion"`
	LocalVersion  string `json:"localVersion"`
	IsInstalled   bool   `json:"isInstalled"`
}

// downloadState persists across interrupted downloads for resume.
type downloadState struct {
	Files     []pendingFile `json:"files"`
	Completed []string      `json:"completed"`
}

type pendingFile struct {
	RemotePath string `json:"remotePath"`
	Hash       string `json:"hash"`
}

const (
	maxRetries     = 3
	maxConcurrent  = 3
	retryBaseDelay = time.Second
)

// Updater manages downloading and patching game files from a CDN.
type Updater struct {
	CDNBaseURL string
	InstallDir string

	mu       sync.Mutex
	progress Progress
	cancel   chan struct{}
}

// NewUpdater creates a new Updater for the given CDN and install directory.
func NewUpdater(cdnBaseURL, installDir string) *Updater {
	return &Updater{
		CDNBaseURL: strings.TrimRight(cdnBaseURL, "/"),
		InstallDir: installDir,
		cancel:     make(chan struct{}),
	}
}

// GetProgress returns the current update progress.
func (u *Updater) GetProgress() Progress {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.progress
}

func (u *Updater) setProgress(p Progress) {
	u.mu.Lock()
	u.progress = p
	u.mu.Unlock()
}

// Cancel signals the updater to stop the current operation.
func (u *Updater) Cancel() {
	select {
	case <-u.cancel:
	default:
		close(u.cancel)
	}
}

func (u *Updater) isCancelled() bool {
	select {
	case <-u.cancel:
		return true
	default:
		return false
	}
}

// --- XML types ---

type hashDataXML struct {
	XMLName xml.Name       `xml:"ArrayOfHashData"`
	Entries []hashEntryXML `xml:"HashData"`
}

type hashEntryXML struct {
	File string `xml:"File"`
	Hash string `xml:"Hash"`
}

// --- Public API ---

// IsInstalled checks if the game appears to be installed.
func (u *Updater) IsInstalled() bool {
	// Check for key game files
	markers := []string{"nc2.exe", "pak__version._", "neocron.ini"}
	for _, m := range markers {
		path := filepath.Join(u.InstallDir, m)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// CheckForUpdate compares local and server versions.
func (u *Updater) CheckForUpdate() UpdateCheckResult {
	result := UpdateCheckResult{
		IsInstalled: u.IsInstalled(),
	}

	localVer, err := u.GetLocalVersion()
	if err != nil {
		localVer = "0.0"
	}
	result.LocalVersion = localVer

	serverVer, err := u.GetServerVersion()
	if err != nil {
		result.ServerVersion = "unknown"
		return result
	}
	result.ServerVersion = serverVer

	result.NeedsUpdate = versionToNum(serverVer) > versionToNum(localVer)
	return result
}

// GetServerVersion fetches the version string from the CDN.
func (u *Updater) GetServerVersion() (string, error) {
	resp, err := httpGetWithRetry(u.CDNBaseURL+"/_version._", maxRetries)
	if err != nil {
		return "", fmt.Errorf("fetch version: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

// GetLocalVersion reads the local version from the pak__version._ file.
func (u *Updater) GetLocalVersion() (string, error) {
	versionFile := filepath.Join(u.InstallDir, "pak__version._")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "0.0", nil
		}
		return "", err
	}
	content, err := pak.DecompressSingleFromMemory(data)
	if err != nil {
		return "", fmt.Errorf("decompress version: %w", err)
	}
	return strings.TrimSpace(string(content)), nil
}

// Install performs a full game installation (download all files).
func (u *Updater) Install(onProgress func(Progress)) error {
	return u.doUpdate("installing", true, onProgress)
}

// Update performs a delta update (only changed files).
func (u *Updater) Update(onProgress func(Progress)) error {
	return u.doUpdate("downloading", false, onProgress)
}

func (u *Updater) doUpdate(statusLabel string, forceAll bool, onProgress func(Progress)) error {
	u.cancel = make(chan struct{})
	startTime := time.Now()

	report := func(p Progress) {
		u.setProgress(p)
		if onProgress != nil {
			onProgress(p)
		}
	}

	report(Progress{Status: "checking", CurrentName: "Fetching file list..."})

	// Create install dir
	if err := os.MkdirAll(u.InstallDir, 0755); err != nil {
		p := Progress{Status: "error", ErrorMessage: err.Error()}
		report(p)
		return err
	}

	// Check for resume state
	var toDownload []pendingFile
	state := u.loadState()
	if state != nil && len(state.Files) > 0 {
		// Resume from saved state
		completed := make(map[string]bool)
		for _, c := range state.Completed {
			completed[c] = true
		}
		for _, f := range state.Files {
			if !completed[f.RemotePath] {
				toDownload = append(toDownload, f)
			}
		}
		report(Progress{
			Status:      statusLabel,
			CurrentName: fmt.Sprintf("Resuming... %d files remaining", len(toDownload)),
		})
	} else {
		// Fresh: fetch hashdata and compare
		entries, err := u.fetchHashData()
		if err != nil {
			p := Progress{Status: "error", ErrorMessage: err.Error()}
			report(p)
			return err
		}

		totalFiles := len(entries)

		if forceAll {
			for _, e := range entries {
				toDownload = append(toDownload, pendingFile{RemotePath: e.File, Hash: e.Hash})
			}
		} else {
			for i, entry := range entries {
				if u.isCancelled() {
					report(Progress{Status: "error", ErrorMessage: "cancelled"})
					return fmt.Errorf("cancelled")
				}

				localRelPath := strings.ReplaceAll(entry.File, "\\", string(os.PathSeparator))
				localPath := filepath.Join(u.InstallDir, localRelPath)

				localHash, err := calcFileHash(localPath)
				if err != nil {
					p := Progress{Status: "error", ErrorMessage: err.Error()}
					report(p)
					return err
				}

				if localHash != entry.Hash {
					toDownload = append(toDownload, pendingFile{RemotePath: entry.File, Hash: entry.Hash})
				}

				report(Progress{
					Status:      "checking",
					TotalFiles:  totalFiles,
					CurrentFile: i + 1,
					CurrentName: entry.File,
					Percent:     float64(i+1) / float64(totalFiles) * 100,
				})
			}
		}

		// Save state for resume
		if len(toDownload) > 0 {
			u.saveState(&downloadState{Files: toDownload})
		}
	}

	if len(toDownload) == 0 {
		u.clearState()
		report(Progress{Status: "done", TotalFiles: 0, Percent: 100})
		return nil
	}

	// Concurrent downloads
	totalFiles := len(toDownload)
	var completedCount int64
	var skippedCount int64
	var totalBytesDownloaded int64

	sem := make(chan struct{}, maxConcurrent)
	errChan := make(chan error, totalFiles)
	var wg sync.WaitGroup

	for _, file := range toDownload {
		if u.isCancelled() {
			break
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(f pendingFile) {
			defer wg.Done()
			defer func() { <-sem }()

			if u.isCancelled() {
				return
			}

			localRelPath := strings.ReplaceAll(f.RemotePath, "\\", string(os.PathSeparator))
			localPath := filepath.Join(u.InstallDir, localRelPath)

			bytesWritten, err := u.downloadFileTracked(f.RemotePath, localPath)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					// File doesn't exist on CDN — skip it (stale hashdata entry)
					atomic.AddInt64(&skippedCount, 1)
					done := atomic.AddInt64(&completedCount, 1)
					report(Progress{
						Status:       statusLabel,
						TotalFiles:   totalFiles,
						CurrentFile:  int(done),
						CurrentName:  f.RemotePath + " (skipped — not on server)",
						Percent:      float64(done) / float64(totalFiles) * 100,
						SkippedFiles: int(atomic.LoadInt64(&skippedCount)),
					})
					u.markCompleted(f.RemotePath)
					return
				}
				errChan <- fmt.Errorf("%s: %w", f.RemotePath, err)
				return
			}

			atomic.AddInt64(&totalBytesDownloaded, bytesWritten)
			done := atomic.AddInt64(&completedCount, 1)

			elapsed := time.Since(startTime).Seconds()
			totalBytes := atomic.LoadInt64(&totalBytesDownloaded)
			speed := float64(0)
			if elapsed > 0 {
				speed = float64(totalBytes) / elapsed
			}

			report(Progress{
				Status:       statusLabel,
				TotalFiles:   totalFiles,
				CurrentFile:  int(done),
				CurrentName:  f.RemotePath,
				Percent:      float64(done) / float64(totalFiles) * 100,
				BytesDone:    totalBytes,
				Speed:        speed,
				SkippedFiles: int(atomic.LoadInt64(&skippedCount)),
			})

			// Update resume state
			u.markCompleted(f.RemotePath)
		}(file)
	}

	wg.Wait()
	close(errChan)

	if u.isCancelled() {
		report(Progress{Status: "paused", ErrorMessage: "Download paused — will resume next time"})
		return fmt.Errorf("cancelled")
	}

	// Check for errors
	var dlErrors []string
	for err := range errChan {
		dlErrors = append(dlErrors, err.Error())
	}
	if len(dlErrors) > 0 {
		errMsg := fmt.Sprintf("%d download errors: %s", len(dlErrors), dlErrors[0])
		report(Progress{Status: "error", ErrorMessage: errMsg})
		return fmt.Errorf(errMsg)
	}

	u.clearState()
	report(Progress{Status: "done", TotalFiles: totalFiles, Percent: 100, SkippedFiles: int(skippedCount)})
	return nil
}

// --- File operations ---

func (u *Updater) fetchHashData() ([]hashEntryXML, error) {
	// Use a transport with DisableCompression to prevent Go from
	// auto-decompressing, since hashdata.dat is a gzip file served
	// as application/octet-stream (not Content-Encoding: gzip).
	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}

	var resp *http.Response
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(retryBaseDelay * time.Duration(math.Pow(2, float64(i-1))))
		}
		resp, lastErr = client.Get(u.CDNBaseURL + "/hashdata.dat")
		if lastErr == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("fetch hashdata: %w", lastErr)
	}
	defer resp.Body.Close()

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	body, err := io.ReadAll(gr)
	if err != nil {
		// The Neocron CDN serves a gzip file with a truncated footer/checksum.
		// The XML payload itself is complete, so tolerate unexpected EOF
		// as long as we got data that looks like valid XML.
		if err.Error() == "unexpected EOF" && len(body) > 0 && bytes.Contains(body, []byte("</ArrayOfHashData>")) {
			// Data is complete, ignore the gzip trailer error
		} else {
			return nil, fmt.Errorf("read hashdata: %w", err)
		}
	}

	var hashList hashDataXML
	if err := xml.Unmarshal(body, &hashList); err != nil {
		return nil, fmt.Errorf("parse hashdata xml: %w", err)
	}
	return hashList.Entries, nil
}

func calcFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// downloadFileTracked downloads a file with retry and returns bytes written.
func (u *Updater) downloadFileTracked(remotePath, localPath string) (int64, error) {
	urlPath := strings.ReplaceAll(remotePath, "\\", "/")
	parts := strings.Split(urlPath, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	fullURL := u.CDNBaseURL + strings.Join(parts, "/")

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return 0, err
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if u.isCancelled() {
			return 0, fmt.Errorf("cancelled")
		}

		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
			time.Sleep(delay)
		}

		resp, err := http.Get(fullURL)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				return 0, ErrNotFound // File doesn't exist on CDN
			}
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return 0, lastErr // Don't retry client errors
			}
			continue
		}

		out, err := os.Create(localPath)
		if err != nil {
			resp.Body.Close()
			return 0, err
		}

		written, err := io.Copy(out, resp.Body)
		out.Close()
		resp.Body.Close()

		if err != nil {
			lastErr = err
			os.Remove(localPath) // Partial file
			continue
		}

		return written, nil
	}

	return 0, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

// --- Resume state management ---

func (u *Updater) statePath() string {
	return filepath.Join(u.InstallDir, ".update-state.json")
}

func (u *Updater) loadState() *downloadState {
	data, err := os.ReadFile(u.statePath())
	if err != nil {
		return nil
	}
	var state downloadState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	return &state
}

func (u *Updater) saveState(state *downloadState) {
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	os.WriteFile(u.statePath(), data, 0644)
}

func (u *Updater) markCompleted(remotePath string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	state := u.loadState()
	if state == nil {
		return
	}
	state.Completed = append(state.Completed, remotePath)
	data, _ := json.Marshal(state)
	os.WriteFile(u.statePath(), data, 0644)
}

func (u *Updater) clearState() {
	os.Remove(u.statePath())
}

// --- Utility ---

func httpGetWithRetry(url string, retries int) (*http.Response, error) {
	var lastErr error
	for i := 0; i < retries; i++ {
		if i > 0 {
			time.Sleep(retryBaseDelay * time.Duration(math.Pow(2, float64(i-1))))
		}
		resp, err := http.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		resp.Body.Close()
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil, lastErr
}

func versionToNum(version string) int {
	parts := strings.Split(version, ".")
	acc := 0
	for i, p := range parts {
		val := 0
		fmt.Sscanf(p, "%d", &val)
		exp := len(parts) - i
		mul := 1
		for j := 0; j < exp; j++ {
			mul *= 100
		}
		acc += mul * val
	}
	return acc
}
