package launcher

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"launcher/pkg/config"
	"launcher/pkg/proton"
)

// GameStatus describes the running state of the game process.
type GameStatus struct {
	Running  bool   `json:"running"`
	PID      int    `json:"pid"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}

// Launcher manages starting and monitoring the game process.
type Launcher struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	status GameStatus
}

// NewLauncher creates a new Launcher instance.
func NewLauncher() *Launcher {
	return &Launcher{}
}

// GetStatus returns the current game process status.
func (l *Launcher) GetStatus() GameStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.status
}

// Launch starts the game with the given configuration.
// extraDLLOverrides is a list of DLL basenames (no extension) that should be
// set to native,builtin via WINEDLLOVERRIDES — typically supplied by the addon
// manager from enabled wrapper addons (dgVoodoo2, ReShade, etc.).
// extraEnv are additional KEY=VALUE entries — typically Vulkan-layer toggles
// or per-platform engine flags from addons that don't ship DLL overrides
// (vkBasalt etc).
// onOutput receives stdout/stderr lines. onExit is called when the process ends.
func (l *Launcher) Launch(cfg *config.Config, extraDLLOverrides []string, extraEnv map[string]string, onOutput func(string), onExit func(GameStatus)) error {
	l.mu.Lock()
	if l.status.Running {
		l.mu.Unlock()
		return fmt.Errorf("game is already running")
	}
	l.mu.Unlock()

	server := cfg.GetActiveServer()
	if server == nil {
		return fmt.Errorf("no server selected")
	}

	// Write server address to neocron.ini before launching
	if err := writeServerConfig(cfg.InstallDir, server.Address, server.Port); err != nil {
		if onOutput != nil {
			onOutput(fmt.Sprintf("[launcher] Warning: could not update neocron.ini: %v", err))
		}
	}

	exePath := filepath.Join(cfg.InstallDir, cfg.GameExe)

	var cmd *exec.Cmd
	var env []string

	switch {
	case runtime.GOOS == "windows" || cfg.RuntimeMode == "native":
		cmd = exec.Command(exePath)
		env = os.Environ()
		for k, v := range extraEnv {
			env = append(env, k+"="+v)
		}

	case cfg.RuntimeMode == "proton":
		protonPath := cfg.ProtonPath
		if protonPath == "" {
			return fmt.Errorf("no Proton build selected — configure one in Settings")
		}

		prefixMgr := proton.NewPrefixManager(cfg.PrefixPath)
		envOpts := proton.LaunchEnvOpts{
			EnableDXVK:        cfg.EnableDXVK,
			EnableMangoHud:    cfg.EnableMangoHud,
			ExtraDLLOverrides: extraDLLOverrides,
			ExtraEnv:          extraEnv,
		}
		env = prefixMgr.BuildGameEnv(protonPath, envOpts)

		protonScript := proton.GetProtonScript(protonPath)
		if protonScript != "" {
			cmd = exec.Command("python3", protonScript, "run", exePath)
		} else {
			wineBin := proton.GetBuildWineBinary(protonPath)
			if wineBin == "" {
				return fmt.Errorf("no proton script or wine binary found in %s", protonPath)
			}
			cmd = exec.Command(wineBin, exePath)
		}

	case cfg.RuntimeMode == "wine":
		winePath, err := exec.LookPath("wine")
		if err != nil {
			return fmt.Errorf("wine not found in PATH")
		}
		cmd = exec.Command(winePath, exePath)
		env = os.Environ()
		env = append(env,
			"WINEDEBUG=-all,err+module",
			proton.ComposeDLLOverrides(extraDLLOverrides),
		)
		if cfg.PrefixPath != "" {
			env = append(env, fmt.Sprintf("WINEPREFIX=%s", cfg.PrefixPath))
		}
		for k, v := range extraEnv {
			env = append(env, k+"="+v)
		}

	default:
		return fmt.Errorf("unsupported runtime mode: %s", cfg.RuntimeMode)
	}

	// Add extra launch args
	if cfg.LaunchArgs != "" {
		args := strings.Fields(cfg.LaunchArgs)
		cmd.Args = append(cmd.Args, args...)
	}

	// Wrap with gamemoderun if enabled and available
	if cfg.EnableGameMode && runtime.GOOS == "linux" {
		if gamemodePath, err := exec.LookPath("gamemoderun"); err == nil {
			cmd.Args = append([]string{gamemodePath}, cmd.Args...)
			cmd.Path = gamemodePath
		}
	}

	cmd.Dir = cfg.InstallDir
	cmd.Env = env

	// Set up stdout/stderr piping
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start game: %w", err)
	}

	l.mu.Lock()
	l.cmd = cmd
	l.status = GameStatus{Running: true, PID: cmd.Process.Pid}
	l.mu.Unlock()

	emit := func(line string) {
		if onOutput != nil {
			onOutput(line)
		}
	}

	// Stream output in background
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			emit("[stdout] " + scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			emit("[stderr] " + scanner.Text())
		}
	}()

	// Wait for exit in background
	go func() {
		err := cmd.Wait()

		l.mu.Lock()
		exitCode := 0
		errMsg := ""
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				errMsg = err.Error()
			}
		}
		l.status = GameStatus{
			Running:  false,
			PID:      l.status.PID,
			ExitCode: exitCode,
			Error:    errMsg,
		}
		l.cmd = nil
		l.mu.Unlock()

		if onExit != nil {
			onExit(l.status)
		}
	}()

	return nil
}

// Kill terminates the running game process.
func (l *Launcher) Kill() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cmd == nil || l.cmd.Process == nil {
		return fmt.Errorf("no game running")
	}
	return l.cmd.Process.Kill()
}

// writeServerConfig updates server address in both neocron.ini and ini/updater.ini.
// The Neocron 2 client reads GAMESERVERIP from ini/updater.ini and NETBASEIP from neocron.ini.
func writeServerConfig(installDir, address string, port int) error {
	// 1. Update neocron.ini — NETBASEIP
	if err := updateIniKey(
		filepath.Join(installDir, "neocron.ini"),
		"NETBASEIP",
		fmt.Sprintf("\"%s:%d\"", address, port),
	); err != nil {
		return fmt.Errorf("neocron.ini: %w", err)
	}

	// 2. Update ini/updater.ini — GAMESERVERIP and SERVERIP
	updaterPath := filepath.Join(installDir, "ini", "updater.ini")
	if err := updateIniKey(updaterPath, "GAMESERVERIP", address); err != nil {
		return fmt.Errorf("updater.ini GAMESERVERIP: %w", err)
	}
	if err := updateIniKey(updaterPath, "SERVERIP", address); err != nil {
		return fmt.Errorf("updater.ini SERVERIP: %w", err)
	}

	return nil
}

// updateIniKey reads an ini-style file and replaces or appends a key=value pair.
// Supports both `KEY = "value"` (neocron.ini) and `KEY=value` (updater.ini) formats.
// Preserves original line endings (\r\n or \n) and all existing content.
func updateIniKey(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			dir := filepath.Dir(path)
			os.MkdirAll(dir, 0755)
			return os.WriteFile(path, []byte(fmt.Sprintf("%s=%s\n", key, value)), 0644)
		}
		return err
	}

	content := string(data)

	// Detect line ending style
	lineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}

	lines := strings.Split(content, lineEnding)
	found := false

	for i, line := range lines {
		// Strip any remaining \r for comparison
		trimmed := strings.TrimSpace(line)

		// Match KEY= or KEY = (with optional spaces around =)
		if strings.HasPrefix(trimmed, key+"=") || strings.HasPrefix(trimmed, key+" =") || strings.HasPrefix(trimmed, key+" ") {
			// Preserve the original format style
			if strings.Contains(line, " = ") {
				lines[i] = fmt.Sprintf("%s = %s", key, value)
			} else {
				lines[i] = fmt.Sprintf("%s=%s", key, value)
			}
			found = true
			break
		}
	}

	if !found {
		// Append before the last line if it's empty (preserve trailing newline)
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = append(lines[:len(lines)-1], fmt.Sprintf("%s=%s", key, value), "")
		} else {
			lines = append(lines, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, lineEnding)), 0644)
}

// RunSysConfig launches the game's graphics configuration dialog via Wine.
// The sysconfig dialog must run against the unwrapped DirectX, so wrapper DLL
// overrides are intentionally not applied here.
func RunSysConfig(cfg *config.Config) error {
	exePath := filepath.Join(cfg.InstallDir, cfg.GameExe)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command(exePath, "-sysconfig")
	} else if cfg.RuntimeMode == "proton" && cfg.ProtonPath != "" {
		protonScript := proton.GetProtonScript(cfg.ProtonPath)
		if protonScript != "" {
			prefixMgr := proton.NewPrefixManager(cfg.PrefixPath)
			env := prefixMgr.BuildGameEnv(cfg.ProtonPath, proton.LaunchEnvOpts{})
			cmd = exec.Command("python3", protonScript, "run", exePath, "-sysconfig")
			cmd.Env = env
		} else {
			wineBin := proton.GetBuildWineBinary(cfg.ProtonPath)
			if wineBin == "" {
				return fmt.Errorf("no wine binary in Proton build")
			}
			cmd = exec.Command(wineBin, exePath, "-sysconfig")
		}
	} else {
		winePath, err := exec.LookPath("wine")
		if err != nil {
			return fmt.Errorf("wine not found in PATH")
		}
		cmd = exec.Command(winePath, exePath, "-sysconfig")
	}

	cmd.Dir = cfg.InstallDir
	if cmd.Env == nil {
		env := os.Environ()
		env = append(env,
			"WINEDEBUG=-all,err+module",
			proton.ComposeDLLOverrides(nil),
		)
		if cfg.PrefixPath != "" {
			env = append(env, fmt.Sprintf("WINEPREFIX=%s", cfg.PrefixPath))
		}
		cmd.Env = env
	}

	return cmd.Run()
}
