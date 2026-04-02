# Troubleshooting

## "Files skipped — not on server" during install

Expected behavior. The CDN's hash list contains stale entries that no longer exist. The launcher skips them safely.

## Game doesn't start on Linux

1. Check the runtime status dot in the footer (green = ready, yellow = needs setup, red = missing)
2. Run **Setup Prefix** in Settings > Runtime
3. Run **Graphics Config** to set display resolution
4. Check the game log panel for Wine errors

## "wine not found in PATH"

```bash
sudo pacman -S wine  # Arch/Manjaro
sudo apt install wine  # Debian/Ubuntu
```

## "winetricks not available"

```bash
sudo pacman -S winetricks  # Arch/Manjaro
sudo apt install winetricks  # Debian/Ubuntu
```

## Choppy audio

Kill PulseAudio before launching, restart after:
```bash
pulseaudio -k
# launch game
pulseaudio --start
```

## Connection failures

- Verify server address and port
- Check firewall rules
- Ensure the server emulator is running

## Update stuck / interrupted

Delete `~/Neocron2/.update-state.json` to force a fresh update.

## macOS security warning

```bash
xattr -d com.apple.quarantine launcher-darwin-arm64
```

## Getting Help

- [GitHub Issues](https://github.com/igwtech/Neocron-Launcher/issues)
- [TechHaven Wiki](https://wiki.techhaven.org/Running_Neocron_in_Linux)
