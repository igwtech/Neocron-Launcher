# Wine / Proton Setup

Running Neocron 2 on Linux and macOS via Wine or Proton.

## Quick Start (Proton)

1. Open Settings > Runtime tab
2. Click **Download GE-Proton** and install the latest version
3. Select the build from the dropdown
4. Click **Setup Prefix** — initializes prefix, sets Win98 mode, installs dependencies
5. Click **Graphics Config** to set display resolution
6. Close settings and click **LAUNCH**

## Required Dependencies

Installed automatically during prefix setup:

| Package | Description |
|---------|------------|
| `corefonts` | Microsoft core fonts (UI rendering) |
| `vcrun6` | Visual C++ Runtime 6 (game engine) |
| `mfc42` | MFC 4.2 libraries (game UI) |

## DLL Overrides

Set automatically on launch:

```
WINEDLLOVERRIDES="msvcrt=n;quartz=n"
```

## Windows Version

Neocron 2 requires **Windows 98** compatibility mode, configured automatically in the prefix registry.

## Graphics Configuration

Click **Graphics Config** in Runtime settings to launch `neocronclient.exe -sysconfig` for display configuration.

## Manual Wine Setup

```bash
# Install
sudo pacman -S wine winetricks  # Arch
sudo apt install wine winetricks  # Ubuntu

# Setup
WINEPREFIX=~/.wine winecfg
WINEPREFIX=~/.wine winetricks -q corefonts vcrun6 mfc42

# Launch
WINEDEBUG=-all,err+module \
WINEDLLOVERRIDES="msvcrt=n;quartz=n" \
WINEPREFIX=~/.wine \
wine ~/Neocron2/neocronclient.exe
```

## Troubleshooting

See [Troubleshooting](Troubleshooting) for common Wine/Proton issues.
