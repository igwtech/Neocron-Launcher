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
// onOutput receives stdout/stderr lines. onExit is called when the process ends.
func (l *Launcher) Launch(cfg *config.Config, onOutput func(string), onExit func(GameStatus)) error {
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

	exePath := filepath.Join(cfg.InstallDir, cfg.GameExe)

	var cmd *exec.Cmd
	var env []string

	switch {
	case runtime.GOOS == "windows" || cfg.RuntimeMode == "native":
		cmd = exec.Command(exePath)
		env = os.Environ()

	case cfg.RuntimeMode == "proton":
		protonPath := cfg.ProtonPath
		if protonPath == "" {
			return fmt.Errorf("no Proton build selected — configure one in Settings")
		}

		prefixMgr := proton.NewPrefixManager(cfg.PrefixPath)
		envOpts := proton.LaunchEnvOpts{
			EnableDXVK:     cfg.EnableDXVK,
			EnableMangoHud: cfg.EnableMangoHud,
			ServerAddress:  server.Address,
			ServerPort:     server.Port,
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
			fmt.Sprintf("NC_SERVER=%s", server.Address),
			fmt.Sprintf("NC_PORT=%d", server.Port),
			"WINEDEBUG=-all,err+module",
			"WINEDLLOVERRIDES=msvcrt=n;quartz=n",
		)
		if cfg.PrefixPath != "" {
			env = append(env, fmt.Sprintf("WINEPREFIX=%s", cfg.PrefixPath))
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

// RunSysConfig launches the game's graphics configuration dialog via Wine.
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
			"WINEDLLOVERRIDES=msvcrt=n;quartz=n",
		)
		if cfg.PrefixPath != "" {
			env = append(env, fmt.Sprintf("WINEPREFIX=%s", cfg.PrefixPath))
		}
		cmd.Env = env
	}

	return cmd.Run()
}
