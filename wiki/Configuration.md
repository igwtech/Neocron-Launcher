# Configuration

## Config File Location

| OS | Path |
|----|------|
| Linux | `~/.config/neocron-launcher/config.json` |
| macOS | `~/Library/Application Support/neocron-launcher/config.json` |
| Windows | `%APPDATA%\neocron-launcher\config.json` |

## Settings

Access settings via the gear icon in the bottom-left corner.

![Settings - General](https://raw.githubusercontent.com/igwtech/Neocron-Launcher/main/docs/screenshots/settings-general.png)

### General Tab

| Setting | Default | Description |
|---------|---------|-------------|
| Install Directory | `~/Neocron2` | Where game files are downloaded |
| CDN Base URL | `http://cdn.neocron-game.com/apps/nc2retail/files` | CDN for game file downloads |
| Game Executable | `neocronclient.exe` | Main game binary to launch |
| API Base URL | `http://api.neocron-game.com:8100` | Neocron management API |
| Extra Launch Arguments | *(empty)* | Additional CLI args for the game |

### Runtime Tab

![Settings - Runtime](https://raw.githubusercontent.com/igwtech/Neocron-Launcher/main/docs/screenshots/settings-runtime.png)

| Setting | Default | Description |
|---------|---------|-------------|
| Runtime Mode | `proton` (Linux/macOS), `native` (Windows) | How to run the game |
| Proton Build | Auto-detect | Path to Proton installation |
| Enable DXVK | `true` | DirectX translation (recommended) |
| Enable GameMode | `true` (Linux) | Feral GameMode for performance |
| Enable MangoHud | `false` | Performance overlay |

### Runtime Modes

- **Proton** — Valve's Proton or GE-Proton. Recommended for Linux.
- **Wine** — System Wine. Simpler but may need manual configuration.
- **Native** — Direct execution. Windows only.

## JSON Config Reference

```json
{
  "installDir": "/home/user/Neocron2",
  "cdnBaseUrl": "http://cdn.neocron-game.com/apps/nc2retail/files",
  "gameExe": "neocronclient.exe",
  "apiBaseUrl": "http://api.neocron-game.com:8100",
  "servers": [
    {
      "name": "Local Server",
      "description": "Local development server",
      "address": "127.0.0.1",
      "port": 7000
    }
  ],
  "activeServer": 0,
  "runtimeMode": "proton",
  "protonPath": "",
  "enableDxvk": true,
  "enableGameMode": true,
  "enableMangoHud": false,
  "launchArgs": ""
}
```
