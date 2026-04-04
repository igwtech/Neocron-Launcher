package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"

	"launcher/pkg/addon"
	"launcher/pkg/config"
	"launcher/pkg/launcher"
	"launcher/pkg/neocronapi"
	"launcher/pkg/proton"
	"launcher/pkg/updater"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// PlatformInfo describes the running platform for the frontend.
type PlatformInfo struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// App is the main application struct bound to the Wails frontend.
type App struct {
	ctx          context.Context
	cfg          *config.Config
	updater      *updater.Updater
	protonMgr    *proton.Manager
	prefixMgr    *proton.PrefixManager
	gameLauncher *launcher.Launcher
	apiClient    *neocronapi.Client
	addonMgr     *addon.Manager
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Warning: could not load config, using defaults:", err)
		cfg = config.DefaultConfig()
	}
	a.cfg = cfg
	a.updater = updater.NewUpdater(cfg.CDNBaseURL, cfg.InstallDir)
	a.protonMgr = proton.NewManager()
	a.prefixMgr = proton.NewPrefixManager(cfg.PrefixPath)
	a.gameLauncher = launcher.NewLauncher()
	a.apiClient = neocronapi.NewClient(cfg.APIBaseURL)
	a.addonMgr = addon.NewManager(cfg.InstallDir)

	// Auto-check for updates on startup
	go func() {
		result := a.updater.CheckForUpdate()
		wailsRuntime.EventsEmit(ctx, "update:check-result", result)
	}()
}

// --- Platform ---

func (a *App) GetPlatformInfo() PlatformInfo {
	return PlatformInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
}

// --- Config bindings ---

func (a *App) GetConfig() *config.Config {
	return a.cfg
}

func (a *App) SaveConfig(cfg config.Config) error {
	a.cfg.InstallDir = cfg.InstallDir
	a.cfg.CDNBaseURL = cfg.CDNBaseURL
	a.cfg.GameExe = cfg.GameExe
	a.cfg.Servers = cfg.Servers
	a.cfg.ActiveServer = cfg.ActiveServer
	a.cfg.RuntimeMode = cfg.RuntimeMode
	a.cfg.ProtonPath = cfg.ProtonPath
	a.cfg.ProtonVersion = cfg.ProtonVersion
	a.cfg.PrefixPath = cfg.PrefixPath
	a.cfg.EnableDXVK = cfg.EnableDXVK
	a.cfg.EnableGameMode = cfg.EnableGameMode
	a.cfg.EnableMangoHud = cfg.EnableMangoHud
	a.cfg.LaunchArgs = cfg.LaunchArgs
	a.cfg.APIBaseURL = cfg.APIBaseURL

	a.updater = updater.NewUpdater(a.cfg.CDNBaseURL, a.cfg.InstallDir)
	a.prefixMgr = proton.NewPrefixManager(a.cfg.PrefixPath)
	a.apiClient = neocronapi.NewClient(a.cfg.APIBaseURL)
	return a.cfg.Save()
}

func (a *App) AddServer(s config.ServerEndpoint) error {
	a.cfg.AddServer(s)
	return a.cfg.Save()
}

func (a *App) RemoveServer(index int) error {
	a.cfg.RemoveServer(index)
	return a.cfg.Save()
}

func (a *App) SetActiveServer(index int) error {
	if index < 0 || index >= len(a.cfg.Servers) {
		return fmt.Errorf("invalid server index: %d", index)
	}
	a.cfg.ActiveServer = index
	return a.cfg.Save()
}

// --- Version and install status ---

func (a *App) GetLocalVersion() string {
	v, err := a.updater.GetLocalVersion()
	if err != nil {
		return "0.0"
	}
	return v
}

func (a *App) GetServerVersion() string {
	v, err := a.updater.GetServerVersion()
	if err != nil {
		return "unknown"
	}
	return v
}

func (a *App) IsGameInstalled() bool {
	return a.updater.IsInstalled()
}

func (a *App) CheckForUpdate() updater.UpdateCheckResult {
	return a.updater.CheckForUpdate()
}

// --- Install / Update ---

func (a *App) StartInstall() error {
	go func() {
		var lastProgress updater.Progress
		err := a.updater.Install(func(p updater.Progress) {
			lastProgress = p
			wailsRuntime.EventsEmit(a.ctx, "update:progress", p)
		})
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "update:error", err.Error())
		} else {
			wailsRuntime.EventsEmit(a.ctx, "update:complete", lastProgress)
		}
	}()
	return nil
}

func (a *App) StartUpdate() error {
	go func() {
		var lastProgress updater.Progress
		err := a.updater.Update(func(p updater.Progress) {
			lastProgress = p
			wailsRuntime.EventsEmit(a.ctx, "update:progress", p)
		})
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "update:error", err.Error())
		} else {
			wailsRuntime.EventsEmit(a.ctx, "update:complete", lastProgress)
		}
	}()
	return nil
}

func (a *App) CancelUpdate() {
	a.updater.Cancel()
}

func (a *App) GetUpdateProgress() updater.Progress {
	return a.updater.GetProgress()
}

// --- Neocron API bindings ---

func (a *App) APILogin(user, password string) (*neocronapi.SessionDetails, error) {
	return a.apiClient.Login(user, password)
}

func (a *App) APILogout() error {
	return a.apiClient.Logout()
}

func (a *App) APIIsSessionValid() bool {
	valid, _ := a.apiClient.IsSessionValid()
	return valid
}

func (a *App) APIGetToken() string {
	return a.apiClient.Token()
}

func (a *App) GetApplications() ([]neocronapi.Application, error) {
	return a.apiClient.GetAvailableApplications()
}

func (a *App) GetGameEndpoints(endpointName string) ([]neocronapi.Endpoint, error) {
	return a.apiClient.GetEndpoints(endpointName)
}

func (a *App) ImportEndpointsAsServers(endpointName string) error {
	endpoints, err := a.apiClient.GetEndpoints(endpointName)
	if err != nil {
		return err
	}
	for _, ep := range endpoints {
		a.cfg.AddServer(config.ServerEndpoint{
			Name:        ep.Name,
			Description: ep.Description,
			Address:     ep.Address,
			Port:        7000,
		})
	}
	return a.cfg.Save()
}

// --- Proton bindings ---

func (a *App) GetProtonBuilds() []proton.Build {
	return a.protonMgr.DetectBuilds()
}

func (a *App) GetAvailableProtonVersions() ([]proton.GHRelease, error) {
	return a.protonMgr.FetchAvailableVersions()
}

func (a *App) DownloadProton(releaseJSON string) error {
	var release proton.GHRelease
	if err := parseJSON(releaseJSON, &release); err != nil {
		return err
	}

	go func() {
		err := a.protonMgr.DownloadBuild(release, func(p proton.DownloadProgress) {
			wailsRuntime.EventsEmit(a.ctx, "proton:progress", p)
		})
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "proton:error", err.Error())
		} else {
			wailsRuntime.EventsEmit(a.ctx, "proton:complete", nil)
		}
	}()
	return nil
}

func (a *App) CancelProtonDownload() {
	a.protonMgr.Cancel()
}

func (a *App) RemoveProtonBuild(buildPath string) error {
	return a.protonMgr.RemoveBuild(buildPath)
}

func (a *App) SetProtonBuild(path, version string) error {
	a.cfg.ProtonPath = path
	a.cfg.ProtonVersion = version
	return a.cfg.Save()
}

// --- Prefix bindings ---

func (a *App) GetPrefixStatus() proton.PrefixStatus {
	return a.prefixMgr.GetStatus()
}

func (a *App) SetupPrefix() error {
	if a.cfg.ProtonPath == "" {
		return fmt.Errorf("no Proton build selected")
	}

	go func() {
		err := a.prefixMgr.Setup(a.cfg.ProtonPath, func(msg string) {
			wailsRuntime.EventsEmit(a.ctx, "prefix:output", msg)
		})
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "prefix:error", err.Error())
		} else {
			wailsRuntime.EventsEmit(a.ctx, "prefix:complete", nil)
		}
	}()
	return nil
}

// --- Game launch ---

func (a *App) LaunchGame() error {
	return a.gameLauncher.Launch(a.cfg,
		func(line string) {
			wailsRuntime.EventsEmit(a.ctx, "game:output", line)
		},
		func(status launcher.GameStatus) {
			wailsRuntime.EventsEmit(a.ctx, "game:exited", status)
		},
	)
}

func (a *App) KillGame() error {
	return a.gameLauncher.Kill()
}

func (a *App) GetGameStatus() launcher.GameStatus {
	return a.gameLauncher.GetStatus()
}

// --- Utility ---

// --- Addon bindings ---

func (a *App) GetInstalledAddons() ([]addon.InstalledAddon, error) {
	return a.addonMgr.ListInstalled()
}

func (a *App) InstallAddon(repoURL string) error {
	go func() {
		err := a.addonMgr.InstallFromRepo(repoURL, func(p addon.DownloadProgress) {
			wailsRuntime.EventsEmit(a.ctx, "addon:progress", p)
		})
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "addon:error", err.Error())
		} else {
			wailsRuntime.EventsEmit(a.ctx, "addon:complete", nil)
		}
	}()
	return nil
}

func (a *App) UninstallAddon(addonID string) error {
	return a.addonMgr.Uninstall(addonID)
}

func (a *App) EnableAddon(addonID string) error {
	return a.addonMgr.Enable(addonID)
}

func (a *App) DisableAddon(addonID string) error {
	return a.addonMgr.Disable(addonID)
}

func (a *App) UpdateAddon(addonID string) error {
	go func() {
		err := a.addonMgr.Update(addonID, func(p addon.DownloadProgress) {
			wailsRuntime.EventsEmit(a.ctx, "addon:progress", p)
		})
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "addon:error", err.Error())
		} else {
			wailsRuntime.EventsEmit(a.ctx, "addon:complete", nil)
		}
	}()
	return nil
}

func (a *App) CheckAddonUpdates() ([]addon.AddonUpdate, error) {
	return a.addonMgr.CheckUpdates()
}

func (a *App) RunSysConfig() error {
	go func() {
		err := launcher.RunSysConfig(a.cfg)
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "sysconfig:error", err.Error())
		} else {
			wailsRuntime.EventsEmit(a.ctx, "sysconfig:complete", nil)
		}
	}()
	return nil
}

func (a *App) SelectDirectory() (string, error) {
	return wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Neocron 2 Install Directory",
	})
}

func parseJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}
