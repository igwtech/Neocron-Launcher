package addon

// Loads the frida-instrument addon manifest from the in-repo source of
// truth (ceres-j/tools/frida_re/launcher-addon/addon.json) and runs it
// through the same validateManifest path the installer uses. Keeps the
// addon honest as the schema evolves — if a future addon.json change
// here breaks validation, this test catches it before publishing.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// repoRelativeAddonJSON resolves the in-repo path of the addon manifest.
// Computed from the test's working directory (launcher/pkg/addon/) so
// it survives `go test ./...` from anywhere in the launcher module.
func repoRelativeAddonJSON(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	// cwd == .../Neocron/launcher/pkg/addon
	// target == .../Neocron/ceres-j/tools/frida_re/launcher-addon/addon.json
	repoRoot := filepath.Clean(filepath.Join(cwd, "..", "..", ".."))
	return filepath.Join(repoRoot,
		"ceres-j", "tools", "frida_re", "launcher-addon", "addon.json")
}

func TestFridaInstrumentManifest_LoadsAndValidates(t *testing.T) {
	path := repoRelativeAddonJSON(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read addon.json: %v (path: %s)", err, path)
	}

	var manifest AddonManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse addon.json: %v", err)
	}

	if err := validateManifest(manifest); err != nil {
		t.Fatalf("validateManifest: %v", err)
	}

	// Lock-in the addon contract — failing here means the addon
	// changed its shape and consumers (launcher UI, this project's
	// docs, IT tooling) may need an update.
	if manifest.ID != "frida-instrument" {
		t.Errorf("manifest.ID = %q, want frida-instrument", manifest.ID)
	}
	if manifest.Name == "" {
		t.Error("manifest.Name empty")
	}
	if manifest.Version == "" {
		t.Error("manifest.Version empty")
	}

	// Must drop the ASI loader, the renamed gadget, and the gadget
	// config into the install dir. The two-stage proxy means we need
	// all three or the gadget never loads.
	wantDst := map[string]bool{
		"dinput8.dll":              false,
		"frida-gadget.asi":         false,
		"frida-gadget.config.json": false,
	}
	for _, fe := range manifest.Files {
		if _, ok := wantDst[fe.Dst]; ok {
			wantDst[fe.Dst] = true
		}
	}
	for dst, found := range wantDst {
		if !found {
			t.Errorf("manifest.Files missing required dst %q", dst)
		}
	}

	// Must declare dinput8 in WineDLLOverrides so the launcher composes
	// it into WINEDLLOVERRIDES at launch time. Without this Wine may
	// resolve the local file via its KnownDLLs path and load the
	// builtin instead — the ASI loader (and the gadget it chain-loads)
	// would never run.
	hasOverride := false
	for _, dll := range manifest.WineDLLOverrides {
		if dll == "dinput8" {
			hasOverride = true
		}
	}
	if !hasOverride {
		t.Errorf("manifest.WineDLLOverrides missing 'dinput8' (got %v)",
			manifest.WineDLLOverrides)
	}
}

func TestFridaInstrumentManifest_FilesExist(t *testing.T) {
	// Every src declared in the manifest must exist in the addon repo
	// — otherwise InstallFromRepo would silently skip them.
	path := repoRelativeAddonJSON(t)
	addonDir := filepath.Dir(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read addon.json: %v", err)
	}
	var manifest AddonManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse addon.json: %v", err)
	}

	for _, fe := range manifest.Files {
		src := filepath.Join(addonDir, filepath.FromSlash(fe.Src))
		info, err := os.Stat(src)
		if err != nil {
			t.Errorf("manifest.Files src %q not found at %s: %v",
				fe.Src, src, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("manifest.Files src %q is empty (size 0)", fe.Src)
		}
	}
}

func TestFridaInstrumentManifest_GadgetIsPE32(t *testing.T) {
	// Both bundled PE binaries (ASI loader + Frida gadget) must be
	// 32-bit Windows DLLs — anything else (text, .xz still compressed,
	// wrong arch) means the addon will silently fail to load in
	// Wine's 32-bit loader.
	path := repoRelativeAddonJSON(t)
	addonDir := filepath.Dir(path)

	for _, name := range []string{"dinput8.dll", "frida-gadget.asi"} {
		t.Run(name, func(t *testing.T) {
			if err := assertPE32i386(filepath.Join(addonDir, name)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func assertPE32i386(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Read first 256 bytes — covers MZ + e_lfanew lookup for most PEs.
	hdr := make([]byte, 256)
	n, err := f.Read(hdr)
	if err != nil || n < 64 {
		return err
	}

	if hdr[0] != 'M' || hdr[1] != 'Z' {
		return errPE(filename, "missing 'MZ' DOS signature")
	}

	// PE header offset at 0x3c, LE32.
	eLfanew := uint32(hdr[0x3c]) | uint32(hdr[0x3d])<<8 |
		uint32(hdr[0x3e])<<16 | uint32(hdr[0x3f])<<24
	if eLfanew < 64 || int(eLfanew)+24 > len(hdr) {
		hdr = make([]byte, eLfanew+24)
		if _, err := f.ReadAt(hdr, 0); err != nil {
			return err
		}
	}
	if string(hdr[eLfanew:eLfanew+4]) != "PE\x00\x00" {
		return errPE(filename, "missing PE signature")
	}
	machine := uint16(hdr[eLfanew+4]) | uint16(hdr[eLfanew+5])<<8
	// 0x014c = IMAGE_FILE_MACHINE_I386 (x86 32-bit)
	if machine != 0x014c {
		return errPE(filename, fmt.Sprintf(
			"Machine = 0x%04x, want 0x014c (i386). "+
				"Wine's 32-bit loader will reject other architectures.",
			machine))
	}
	return nil
}

func errPE(filename, msg string) error {
	return fmt.Errorf("%s: %s", filepath.Base(filename), msg)
}
