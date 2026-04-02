# Troubleshooting

## Common Issues

### "File not found on server" during install

Some files in the CDN's hash list no longer exist (e.g., `pak_ip_beam_hit`, `pak_ip_beam_shot`). The launcher skips these with a message like:

> Complete! (2 files skipped — not on server)

This is expected and safe — the skipped files are stale duplicates.

### Game doesn't start on Linux

1. **Check runtime status** — The footer shows a colored dot:
   - Green = ready
   - Yellow = needs setup (run Setup Prefix in Settings > Runtime)
   - Red = no Proton/Wine found

2. **Run Setup Prefix** — Settings > Runtime > Setup Prefix. This installs required dependencies.

3. **Run Graphics Config** — Settings > Runtime > Graphics Config. Set appropriate resolution and color depth.

4. **Check the game log** — The log panel shows Wine/Proton output. Look for DLL loading errors.

### "wine not found in PATH"

Install Wine:
```bash
sudo pacman -S wine  # Arch/Manjaro
sudo apt install wine  # Debian/Ubuntu
```

### "winetricks not available"

The launcher tries to download winetricks automatically. If that fails:
```bash
sudo pacman -S winetricks  # Arch/Manjaro
sudo apt install winetricks  # Debian/Ubuntu
```

### Choppy audio / PulseAudio issues

If audio is choppy under PulseAudio:
```bash
# Kill PulseAudio before launching
pulseaudio -k
# Launch the game
# Restart PulseAudio after
pulseaudio --start
```

Or set in `/etc/pulse/default.pa`:
```
load-module module-alsa-sink device=default
```

### Connection failures

1. Verify the server address and port in the server list
2. Check your firewall allows outbound connections on the game port
3. If using a local server, ensure the Neocron server emulator is running

### Console spam / "fixme" messages

The launcher sets `WINEDEBUG=-all,err+module` to suppress most Wine debug output. If you need full debug output for troubleshooting:

1. Settings > General > Extra Launch Arguments: leave empty
2. The game log panel will show any remaining errors

### Update interrupted / stuck

If an update was interrupted, the launcher saves progress to `.update-state.json` in the install directory. On next update:
- It automatically resumes where it left off
- To force a fresh update, delete `~/Neocron2/.update-state.json`

### macOS security warning

macOS may block the launcher. To fix:
1. Right-click the binary and select "Open"
2. Or: `xattr -d com.apple.quarantine launcher-darwin-arm64`

## Getting Help

- [GitHub Issues](https://github.com/igwtech/Neocron-Launcher/issues) — Report bugs
- [TechHaven Wiki](https://wiki.techhaven.org/Running_Neocron_in_Linux) — Community Linux guide
