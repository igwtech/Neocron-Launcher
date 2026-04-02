package addon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// AddonManifest is the addon.json parsed from a GitHub repo.
type AddonManifest struct {
	Name        string      `json:"name"`
	ID          string      `json:"id"`
	Version     string      `json:"version"`
	Author      string      `json:"author"`
	Description string      `json:"description"`
	Category    string      `json:"category"` // graphics, audio, ui, scripts, translation, effects, other
	Tags        []string    `json:"tags"`
	Files       []FileEntry `json:"files"`
	Conflicts   []string    `json:"conflicts"`
	Requires    []string    `json:"requires"`
}

// FileEntry maps source paths in the addon repo to destination paths in the game dir.
type FileEntry struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

// InstalledAddon tracks an addon that has been installed.
type InstalledAddon struct {
	ID          string        `json:"id"`
	RepoURL     string        `json:"repoUrl"`
	Version     string        `json:"version"`
	Enabled     bool          `json:"enabled"`
	InstalledAt time.Time     `json:"installedAt"`
	Manifest    AddonManifest `json:"manifest"`
}

// AddonUpdate describes an available update for an installed addon.
type AddonUpdate struct {
	AddonID        string `json:"addonId"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	RepoURL        string `json:"repoUrl"`
}

// DownloadProgress reports addon download/install progress.
type DownloadProgress struct {
	Status  string  `json:"status"` // "downloading", "extracting", "installing", "done", "error"
	Percent float64 `json:"percent"`
	Message string  `json:"message"`
}

// state is the persisted addon state.
type state struct {
	Addons []InstalledAddon `json:"addons"`
}

// DefaultDataDir returns the base directory for addon data.
func DefaultDataDir() string {
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "neocron-launcher", "addons")
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "neocron-launcher", "addons")
}

func statePath(dataDir string) string {
	return filepath.Join(dataDir, "state.json")
}

func loadState(dataDir string) (*state, error) {
	data, err := os.ReadFile(statePath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return &state{}, nil
		}
		return nil, err
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func saveState(dataDir string, s *state) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(dataDir), data, 0644)
}

// addonDir returns the directory for a specific addon's cached files and backups.
func addonDir(dataDir, addonID string) string {
	return filepath.Join(dataDir, addonID)
}

func addonFilesDir(dataDir, addonID string) string {
	return filepath.Join(addonDir(dataDir, addonID), "files")
}

func addonBackupDir(dataDir, addonID string) string {
	return filepath.Join(addonDir(dataDir, addonID), "backup")
}
