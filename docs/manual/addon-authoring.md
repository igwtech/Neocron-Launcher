# Authoring an Addon

The Neocron Launcher installs addons from any public GitHub repository
containing an `addon.json` manifest. This is the authoring reference.

## Quickstart

```
your-addon-repo/
├── addon.json          ← required
├── README.md           ← user docs
├── ... files referenced by the manifest ...
```

Push to GitHub, then in the launcher's **Addons** tab, paste the repo URL
and click **Install**.

## Manifest reference (`addon.json`)

```json
{
  "id":          "neocron-graphics-pack",
  "name":        "Enhanced Graphics (dgVoodoo2 + ReShade)",
  "version":     "0.1.0",
  "author":      "Neocron Community",
  "description": "DX8 → D3D11 wrapper + post-processing shaders.",
  "category":    "graphics",
  "tags":        ["dgvoodoo", "reshade", "directx"],

  "files": [
    { "src": "dgVoodoo.conf", "dst": "dgVoodoo.conf" }
  ],

  "fetch": [
    {
      "from":    "https://github.com/dege-diosg/dgVoodoo2/releases/download/v2.87.1/dgVoodoo2_87_1.zip",
      "extract": "zip",
      "files":   [{ "src": "MS/x86/D3D8.dll", "dst": "D3D8.dll" }]
    }
  ],

  "wineDllOverrides": ["d3d8", "dxgi"],

  "expects": [
    "dxgi.dll",
    "reshade-shaders/Shaders"
  ],

  "requires":  [],
  "conflicts": []
}
```

### Required fields

| Field | Type | Notes |
|---|---|---|
| `id` | string | Unique identifier. Used for state tracking; never change after publishing. |
| `name` | string | Display name in the UI. |
| `version` | string | Semantic-ish; compared exactly with the latest GitHub release tag for update checks. |

A manifest must declare at least one of `files`, `fetch`, or
`wineDllOverrides` — otherwise it has no effect.

### `files` — repo-bundled drop-ins

Each entry copies one path from the repo into the addon's cache, which is
then stamped onto the game install dir. `dst` is install-dir-relative.

```json
"files": [
  { "src": "configs/dgVoodoo.conf",     "dst": "dgVoodoo.conf" },
  { "src": "shaders",                   "dst": "reshade-shaders/Shaders" }
]
```

`src` may be a directory — the whole tree is copied. Empty dirs are skipped
(`.gitkeep` placeholders are ignored). Both `src` and `dst` must be
non-empty; `dst` may not contain `..`.

### `fetch` — auto-downloaded archives

Each entry downloads an archive from a URL, extracts it, and copies declared
files into the cache. Use this to reference upstream binaries without
redistributing them in your repo.

```json
"fetch": [
  {
    "from":    "https://github.com/owner/repo/releases/download/v1.2/x.zip",
    "extract": "zip",
    "files":   [{ "src": "subdir/binary.dll", "dst": "binary.dll" }]
  }
]
```

| Field | Required | Notes |
|---|---|---|
| `from` | yes | `http://` or `https://` URL. Redirects followed. |
| `extract` | no | `"zip"`, `"tar.gz"` (alias `"tgz"`), or `""` for raw single-file. Default `""`. |
| `files` | yes | Same shape as the top-level `files`. `src` is relative to the extracted archive root. |

For raw single-file downloads, the file is placed at `extracted/<basename>`
inside the temp dir, and `src` should match that basename.

Path-traversal entries inside archives are silently skipped.

### `wineDllOverrides` — DLL hooks

List of DLL basenames (no extension, lowercased) that should be set to
native-then-builtin in `WINEDLLOVERRIDES` when this addon is enabled.

```json
"wineDllOverrides": ["d3d8", "dxgi"]
```

The launcher composes `WINEDLLOVERRIDES=quartz=n,b;d3d8=n,b;dxgi=n,b` from
the union of every enabled addon's overrides. Baseline `quartz=n,b` is
always present.

Use this when your addon drops in a wrapper DLL (dgVoodoo2's `D3D8.dll`,
ReShade's `dxgi.dll`, etc.) — without the override, Wine prefers the
built-in stub and the wrapper is ignored.

### `expects` — user-supplied files

Install-dir-relative paths the addon needs the user to provide manually,
that the launcher cannot auto-fetch (e.g. ReShade's `dxgi.dll`, which
upstream asks redistributors not to bundle).

```json
"expects": [
  "dxgi.dll",
  "reshade-shaders/Shaders"
]
```

Until each path exists in the install dir, the addon card shows a pulsing
**"N file(s) missing"** badge. Live-checked on every refresh — drops in
clear it without re-installing.

### `requires` and `conflicts` — dependency graph

Both are arrays of other addon IDs.

```json
"requires":  ["base-pack", "compatibility-shim"],
"conflicts": ["alternative-graphics-pack"]
```

**Requires** semantics:
- At install time: every required addon must already be installed.
- At enable time: the launcher auto-enables transitive deps in topological
  order. Cycles are detected and rejected.
- At disable/uninstall: refused if any enabled addon requires this one.

**Conflicts** semantics:
- At install time: refused if a conflicting addon is currently *enabled*
  (disabled conflicts can coexist on disk).
- At enable time: refused in both directions — you can't enable an addon
  that conflicts with an enabled one, and you can't enable one if an
  already-enabled addon lists it as a conflict.

Self-deps (an addon listing its own ID) and duplicate entries are rejected
at install.

## Lifecycle and ordering

1. **Install** — manifest validated, `fetch` URLs downloaded, repo `files`
   copied to the addon cache, pristine snapshots captured for any path the
   addon will touch (first capture wins — shared across all addons).
2. **Enable / disable** — recomputes the install stack: every snapshotted
   path is restored from pristine, then enabled addons are stamped in
   priority order (lower priority first; higher wins file conflicts). New
   installs go to the top.
3. **Reorder** — same recompute. Up/down arrows in the UI flip priority.
4. **CDN game update** — two-phase: addons un-stamp to pristine, updater
   runs, pristine pool refreshes from the now-updated install dir, addons
   re-stamp. Wrapper DLLs survive game patches.
5. **Update (this addon)** — full reinstall from the repo, but priority
   and enabled state are preserved across the cycle.
6. **Uninstall** — addon removed from state; recompute restores pristine
   for any path it touched (including paths only this addon touched).

## Categories

The `category` field is metadata only (controls a colored badge):
`graphics`, `audio`, `ui`, `scripts`, `translation`, `effects`, `other`.

## Versioning + updates

The launcher's update checker queries `releases/latest` on the repo.
`tag_name` is compared exactly (string equality) against the installed
`version`. Tag releases as you go — `git tag v0.2.0 && git push --tags` is
enough. `Update` does an in-place reinstall preserving the user's priority
and enabled state.

## Testing your addon before publishing

Currently the launcher only installs from public GitHub repos. To test
locally:

1. Push your addon to a private or throwaway GitHub repo.
2. Install via the Addons tab → paste the repo URL.
3. Inspect `~/.local/share/neocron-launcher/addons/addon.log` (Linux) or
   `~/Library/Application Support/neocron-launcher/addons/addon.log`
   (macOS) for the install trace.

The cache lives under
`~/.local/share/neocron-launcher/addons/<addon-id>/files/`; the pristine
pool is under `_pristine/`. Both are safe to inspect.

## Security and constraints

- HTTP(S) URLs only in `fetch.from` — `file://`, `git://`, etc. rejected.
- `dst` paths may not contain `..` — protects the install dir from
  traversal.
- Path-traversal entries in zip/tar archives are silently dropped during
  extraction.
- Installed addon files are copied (not symlinked) so a CDN update
  modifying a file the launcher doesn't track can't propagate weirdly.

## Examples

- **Enhanced Graphics pack** — `wineDllOverrides` + `fetch` + `expects`:
  https://github.com/igwtech/neocron-graphics-pack
- A minimal "drop-in audio replacement" addon needs only `id`, `name`,
  `version`, and `files` — no fetch, no overrides.
