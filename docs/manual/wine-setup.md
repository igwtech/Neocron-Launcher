# Wine / Proton Setup

This guide covers running Neocron 2 on Linux and macOS via Wine or Proton.

## Quick Start (Proton — Recommended)

1. Open Settings > Runtime tab
2. Click **Download GE-Proton** and install the latest version
3. Select the downloaded build from the "Proton Build" dropdown
4. Click **Setup Prefix** — this will:
   - Initialize a Wine prefix
   - Set Windows 98 compatibility mode
   - Install required dependencies (corefonts, vcrun6, mfc42)
5. Click **Graphics Config** to configure display settings
6. Close settings and click **LAUNCH**

## Proton vs Wine

| Feature | Proton | System Wine |
|---------|--------|-------------|
| Setup | Automated via launcher | Manual winetricks |
| DXVK | Built-in | Must install separately |
| Compatibility | Tuned for gaming | General-purpose |
| Prefix | Managed by launcher | System-wide or manual |

## Required Dependencies

Neocron 2 needs these Windows libraries installed in the Wine prefix:

| Package | Description | Why |
|---------|------------|-----|
| `corefonts` | Microsoft core fonts | UI text rendering |
| `vcrun6` | Visual C++ Runtime 6 | Game engine dependency |
| `mfc42` | MFC 4.2 libraries | Game UI framework |

The launcher installs these automatically via `winetricks` during prefix setup.

> If winetricks is not installed, the launcher downloads it automatically.

## DLL Overrides

The launcher automatically sets these DLL overrides when launching the game:

```
WINEDLLOVERRIDES="quartz=n,b"
```

- `quartz=n,b` — Prefer native DirectShow for video playback, fall back to builtin

> **Note:** The TechHaven wiki suggests overriding `msvcrt` to native, but this is incompatible with Proton and modern Wine — it causes cascade DLL loading failures (`advapi32.dll`, `user32.dll`, etc. all fail). The launcher does **not** override msvcrt. Wine's built-in msvcrt works correctly with Neocron 2.

## Windows Version

Neocron 2 requires **Windows 98** compatibility mode. The launcher configures this in the Wine registry during prefix setup:

```reg
[HKEY_CURRENT_USER\Software\Wine]
"Version"="win98"

[HKEY_CURRENT_USER\Software\Wine\AppDefaults\neocronclient.exe]
"Version"="win98"
```

## Graphics Configuration

Click **Graphics Config** in the Runtime settings tab to launch the game's built-in display configuration dialog (`neocronclient.exe -sysconfig`). This lets you set:

- Screen resolution
- Color depth
- Graphics quality
- Fullscreen/windowed mode

## Manual Wine Setup (Advanced)

If you prefer to use system Wine without the launcher's prefix management:

```bash
# Install Wine and winetricks
sudo pacman -S wine winetricks  # Arch/Manjaro
sudo apt install wine winetricks  # Debian/Ubuntu

# Initialize prefix
WINEPREFIX=~/.wine winecfg

# Install dependencies
WINEPREFIX=~/.wine winetricks -q corefonts vcrun6 mfc42

# Run the game
WINEDEBUG=-all,err+module \
WINEDLLOVERRIDES="quartz=n,b" \
WINEPREFIX=~/.wine \
wine ~/Neocron2/neocronclient.exe
```

## Prefix Locations

| Mode | Default Path |
|------|-------------|
| Proton | `~/.local/share/neocron-launcher/prefix/` |
| Wine | `~/.local/share/neocron-launcher/prefix/` |
| Custom | Set in Settings > Runtime > Prefix Path |

## Troubleshooting

See [Troubleshooting](troubleshooting.md) for common Wine/Proton issues.
