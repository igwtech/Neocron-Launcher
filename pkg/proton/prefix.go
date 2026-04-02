package proton

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// PrefixStatus describes the state of a Wine/Proton prefix.
type PrefixStatus struct {
	Initialized bool   `json:"initialized"`
	Path        string `json:"path"`
	Message     string `json:"message"`
}

// PrefixManager handles Wine/Proton prefix creation and configuration.
type PrefixManager struct {
	PrefixPath string
}

// NewPrefixManager creates a prefix manager for the given path.
func NewPrefixManager(prefixPath string) *PrefixManager {
	if prefixPath == "" {
		prefixPath = defaultPrefixPath()
	}
	return &PrefixManager{PrefixPath: prefixPath}
}

func defaultPrefixPath() string {
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "neocron-launcher", "prefix")
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "neocron-launcher", "prefix")
}

// GetStatus checks whether the prefix is initialized and usable.
func (pm *PrefixManager) GetStatus() PrefixStatus {
	systemReg := filepath.Join(pm.PrefixPath, "pfx", "system.reg")
	if _, err := os.Stat(systemReg); err == nil {
		return PrefixStatus{
			Initialized: true,
			Path:        pm.PrefixPath,
			Message:     "Prefix ready",
		}
	}

	// Also check the non-Proton Wine prefix layout
	systemReg2 := filepath.Join(pm.PrefixPath, "system.reg")
	if _, err := os.Stat(systemReg2); err == nil {
		return PrefixStatus{
			Initialized: true,
			Path:        pm.PrefixPath,
			Message:     "Prefix ready",
		}
	}

	return PrefixStatus{
		Initialized: false,
		Path:        pm.PrefixPath,
		Message:     "Prefix needs setup",
	}
}

// Setup initializes the prefix using the specified Proton build.
// For Proton: runs "proton run" with a dummy command to bootstrap the prefix.
// Falls back to wineboot if no Proton script is found.
func (pm *PrefixManager) Setup(protonBuildPath string, onOutput func(string)) error {
	if err := os.MkdirAll(pm.PrefixPath, 0755); err != nil {
		return fmt.Errorf("create prefix dir: %w", err)
	}

	protonScript := GetProtonScript(protonBuildPath)
	if protonScript != "" {
		return pm.setupViaProton(protonScript, protonBuildPath, onOutput)
	}

	wineBin := GetBuildWineBinary(protonBuildPath)
	if wineBin != "" {
		return pm.setupViaWine(wineBin, onOutput)
	}

	// Last resort: try system wine
	if _, err := exec.LookPath("wine"); err == nil {
		return pm.setupViaWine("wine", onOutput)
	}

	return fmt.Errorf("no Proton or Wine binary found in %s", protonBuildPath)
}

func (pm *PrefixManager) setupViaProton(protonScript, buildPath string, onOutput func(string)) error {
	emit := func(msg string) {
		if onOutput != nil {
			onOutput(msg)
		}
	}

	emit("Initializing prefix via Proton...")

	env := pm.buildProtonEnv(buildPath)

	// Run proton run with a harmless Windows command to init the prefix
	cmd := exec.Command("python3", protonScript, "run", "cmd", "/c", "echo", "prefix initialized")
	cmd.Env = env
	cmd.Dir = buildPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Proton may return non-zero but still set up the prefix fine
		emit(fmt.Sprintf("Proton output: %s", string(output)))
		// Check if prefix was actually created despite the error
		if pm.GetStatus().Initialized {
			emit("Prefix initialized successfully (despite non-zero exit)")
			return nil
		}
		return fmt.Errorf("proton setup failed: %w\n%s", err, string(output))
	}

	emit("Prefix initialized successfully")

	// Set Windows version to Win10
	if err := pm.setWindowsVersion(buildPath); err != nil {
		emit(fmt.Sprintf("Warning: could not set Windows version: %v", err))
	}

	return nil
}

func (pm *PrefixManager) setupViaWine(wineBinary string, onOutput func(string)) error {
	emit := func(msg string) {
		if onOutput != nil {
			onOutput(msg)
		}
	}

	emit("Initializing prefix via Wine...")

	winebootPath := filepath.Join(filepath.Dir(wineBinary), "wineboot")
	if _, err := os.Stat(winebootPath); err != nil {
		winebootPath = "wineboot"
	}

	cmd := exec.Command(winebootPath, "--init")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("WINEPREFIX=%s", pm.PrefixPath),
		"WINEDEBUG=-all",
	)

	output, err := cmd.CombinedOutput()
	emit(string(output))
	if err != nil {
		if pm.GetStatus().Initialized {
			emit("Prefix initialized successfully (despite non-zero exit)")
			return nil
		}
		return fmt.Errorf("wineboot failed: %w", err)
	}

	emit("Prefix initialized successfully")
	return nil
}

// setWindowsVersion sets the prefix to report as Windows 10.
func (pm *PrefixManager) setWindowsVersion(buildPath string) error {
	// Find the user.reg location
	regPaths := []string{
		filepath.Join(pm.PrefixPath, "pfx", "user.reg"),
		filepath.Join(pm.PrefixPath, "user.reg"),
	}

	for _, regPath := range regPaths {
		if _, err := os.Stat(regPath); err == nil {
			// Use wine regedit via a .reg file
			return pm.applyWin10Registry(buildPath, filepath.Dir(regPath))
		}
	}
	return nil
}

func (pm *PrefixManager) applyWin10Registry(buildPath, prefixDir string) error {
	regContent := `REGEDIT4

[HKEY_CURRENT_USER\Software\Wine\AppDefaults\nc2.exe]
"Version"="win10"
`
	tmpFile, err := os.CreateTemp("", "neocron-win10-*.reg")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(regContent)
	tmpFile.Close()

	wineBin := GetBuildWineBinary(buildPath)
	if wineBin == "" {
		wineBin = "wine"
	}
	regeditPath := filepath.Join(filepath.Dir(wineBin), "regedit")
	if _, err := os.Stat(regeditPath); err != nil {
		regeditPath = "regedit"
	}

	cmd := exec.Command(regeditPath, tmpFile.Name())
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("WINEPREFIX=%s", prefixDir),
		"WINEDEBUG=-all",
	)
	return cmd.Run()
}

// buildProtonEnv constructs the environment variables for running a game via Proton.
func (pm *PrefixManager) buildProtonEnv(protonBuildPath string) []string {
	env := os.Environ()

	// Filter out any existing conflicting vars
	var filtered []string
	for _, e := range env {
		key := strings.SplitN(e, "=", 2)[0]
		switch key {
		case "STEAM_COMPAT_DATA_PATH", "STEAM_COMPAT_CLIENT_INSTALL_PATH",
			"WINEPREFIX", "WINEDEBUG":
			continue
		default:
			filtered = append(filtered, e)
		}
	}

	filtered = append(filtered,
		fmt.Sprintf("STEAM_COMPAT_DATA_PATH=%s", pm.PrefixPath),
		fmt.Sprintf("STEAM_COMPAT_CLIENT_INSTALL_PATH=%s", protonBuildPath),
		"WINEDEBUG=-all",
	)

	return filtered
}

// BuildGameEnv returns the environment variables needed to launch a game through Proton.
func (pm *PrefixManager) BuildGameEnv(protonBuildPath string, opts LaunchEnvOpts) []string {
	env := pm.buildProtonEnv(protonBuildPath)

	if opts.EnableDXVK {
		env = append(env, "PROTON_USE_WINED3D=0")
	} else {
		env = append(env, "PROTON_USE_WINED3D=1")
	}

	if opts.EnableMangoHud {
		env = append(env, "MANGOHUD=1")
	}

	if opts.ServerAddress != "" {
		env = append(env, fmt.Sprintf("NC_SERVER=%s", opts.ServerAddress))
	}
	if opts.ServerPort > 0 {
		env = append(env, fmt.Sprintf("NC_PORT=%d", opts.ServerPort))
	}

	return env
}

// LaunchEnvOpts configures the game launch environment.
type LaunchEnvOpts struct {
	EnableDXVK    bool
	EnableMangoHud bool
	ServerAddress string
	ServerPort    int
}
