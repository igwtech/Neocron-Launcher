package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// ServerEndpoint represents a game server the launcher can connect to.
type ServerEndpoint struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
}

// Config holds all launcher configuration.
type Config struct {
	InstallDir   string           `json:"installDir"`
	CDNBaseURL   string           `json:"cdnBaseUrl"`
	GameExe      string           `json:"gameExe"`
	Servers      []ServerEndpoint `json:"servers"`
	ActiveServer int              `json:"activeServer"`

	// Runtime settings
	RuntimeMode   string `json:"runtimeMode"`   // "proton", "wine", "native"
	ProtonPath    string `json:"protonPath"`     // path to selected Proton build (empty = auto-detect)
	ProtonVersion string `json:"protonVersion"`  // version identifier
	PrefixPath    string `json:"prefixPath"`     // WINEPREFIX location (empty = default)

	// Feature toggles
	EnableDXVK     bool `json:"enableDxvk"`
	EnableGameMode bool `json:"enableGameMode"`
	EnableMangoHud bool `json:"enableMangoHud"`

	// Extra launch arguments
	LaunchArgs string `json:"launchArgs"`

	// API settings
	APIBaseURL string `json:"apiBaseUrl"` // Neocron management API (SOAP)

	configPath string
}

// DefaultConfig returns a config with sensible defaults for the Neocron emulator.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()

	runtimeMode := "proton"
	if runtime.GOOS == "windows" {
		runtimeMode = "native"
	}

	return &Config{
		InstallDir: filepath.Join(homeDir, "Neocron2"),
		CDNBaseURL: "http://cdn.neocron-game.com/apps/nc2retail/files",
		GameExe:    "nc2.exe",
		Servers: []ServerEndpoint{
			{
				Name:        "Local Server",
				Description: "Local development server",
				Address:     "127.0.0.1",
				Port:        7000,
			},
		},
		ActiveServer:   0,
		RuntimeMode:    runtimeMode,
		EnableDXVK:     true,
		EnableGameMode: runtime.GOOS == "linux",
		EnableMangoHud: false,
		APIBaseURL:     "http://api.neocron-game.com:8100",
	}
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir, _ = os.UserHomeDir()
	}
	return filepath.Join(configDir, "neocron-launcher", "config.json")
}

// Load reads config from disk. Returns default config if file doesn't exist.
func Load() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			cfg.configPath = path
			return cfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.configPath = path
	return &cfg, nil
}

// Save writes the config to disk.
func (c *Config) Save() error {
	path := c.configPath
	if path == "" {
		path = ConfigPath()
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetActiveServer returns the currently selected server endpoint.
func (c *Config) GetActiveServer() *ServerEndpoint {
	if c.ActiveServer < 0 || c.ActiveServer >= len(c.Servers) {
		return nil
	}
	return &c.Servers[c.ActiveServer]
}

// AddServer adds a new server endpoint to the config.
func (c *Config) AddServer(s ServerEndpoint) {
	c.Servers = append(c.Servers, s)
}

// RemoveServer removes a server endpoint by index.
func (c *Config) RemoveServer(index int) {
	if index < 0 || index >= len(c.Servers) {
		return
	}
	c.Servers = append(c.Servers[:index], c.Servers[index+1:]...)
	if c.ActiveServer >= len(c.Servers) {
		c.ActiveServer = len(c.Servers) - 1
	}
}
