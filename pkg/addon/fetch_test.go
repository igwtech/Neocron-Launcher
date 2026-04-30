package addon

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeZip builds an in-memory zip archive containing the given path -> content
// entries.
func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func newFetchManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{
		DataDir:    t.TempDir(),
		InstallDir: t.TempDir(),
		Logger:     log.New(os.Stderr, "[fetch-test] ", 0),
	}
}

func TestStageFetched_zipWithNestedFile(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{
		"MS/x86/D3D8.dll":         "DGVOODOO-DLL-CONTENT",
		"MS/x64/D3D8.dll":         "WRONG-ARCH",
		"3Dfx/Glide.dll":          "UNRELATED",
		"dgVoodooCpl.exe":         "CPL-EXE",
		"License/dgVoodoo.license": "LICENSE-TEXT",
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipBytes)
	}))
	defer srv.Close()

	m := newFetchManager(t)
	cacheDir := filepath.Join(m.DataDir, "cache")
	os.MkdirAll(cacheDir, 0755)

	entry := FetchEntry{
		From:    srv.URL + "/dgVoodoo2.zip",
		Extract: "zip",
		Files: []FileEntry{
			{Src: "MS/x86/D3D8.dll", Dst: "D3D8.dll"},
		},
	}

	if err := m.stageFetched(entry, cacheDir, nil); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(cacheDir, "D3D8.dll"))
	if err != nil {
		t.Fatalf("expected D3D8.dll in cache: %v", err)
	}
	if string(got) != "DGVOODOO-DLL-CONTENT" {
		t.Errorf("wrong x86 D3D8.dll selected — got %q", got)
	}

	// Other files MUST NOT have leaked into cache.
	for _, leaked := range []string{"MS/x64/D3D8.dll", "3Dfx/Glide.dll", "dgVoodooCpl.exe"} {
		if _, err := os.Stat(filepath.Join(cacheDir, leaked)); err == nil {
			t.Errorf("file %q should not have been staged", leaked)
		}
	}
}

func TestStageFetched_tarGz(t *testing.T) {
	tarBytes := makeTarGz(t, map[string]string{
		"shaders/Shaders/SMAA.fx":  "SMAA-FX",
		"shaders/Textures/lut.png": "LUT-PNG",
		"shaders/README.md":        "README",
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(tarBytes)
	}))
	defer srv.Close()

	m := newFetchManager(t)
	cacheDir := filepath.Join(m.DataDir, "cache")
	os.MkdirAll(cacheDir, 0755)

	entry := FetchEntry{
		From:    srv.URL + "/shaders.tar.gz",
		Extract: "tar.gz",
		Files: []FileEntry{
			{Src: "shaders/Shaders", Dst: "reshade-shaders/Shaders"},
			{Src: "shaders/Textures", Dst: "reshade-shaders/Textures"},
		},
	}

	if err := m.stageFetched(entry, cacheDir, nil); err != nil {
		t.Fatal(err)
	}

	if got, _ := os.ReadFile(filepath.Join(cacheDir, "reshade-shaders/Shaders/SMAA.fx")); string(got) != "SMAA-FX" {
		t.Errorf("Shaders/SMAA.fx not staged correctly: got %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(cacheDir, "reshade-shaders/Textures/lut.png")); string(got) != "LUT-PNG" {
		t.Errorf("Textures/lut.png not staged correctly: got %q", got)
	}
	// README should NOT have been staged — not in entry.Files.
	if _, err := os.Stat(filepath.Join(cacheDir, "reshade-shaders/README.md")); err == nil {
		t.Errorf("README should not have been staged")
	}
}

func TestStageFetched_rawSingleFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("RAW-DLL-BYTES"))
	}))
	defer srv.Close()

	m := newFetchManager(t)
	cacheDir := filepath.Join(m.DataDir, "cache")
	os.MkdirAll(cacheDir, 0755)

	entry := FetchEntry{
		From: srv.URL + "/d3d8.dll",
		// Extract: "" — raw mode
		Files: []FileEntry{
			{Src: "d3d8.dll", Dst: "D3D8.dll"},
		},
	}

	if err := m.stageFetched(entry, cacheDir, nil); err != nil {
		t.Fatal(err)
	}

	if got, _ := os.ReadFile(filepath.Join(cacheDir, "D3D8.dll")); string(got) != "RAW-DLL-BYTES" {
		t.Errorf("raw download not staged: got %q", got)
	}
}

func TestStageFetched_httpErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := newFetchManager(t)
	cacheDir := filepath.Join(m.DataDir, "cache")
	os.MkdirAll(cacheDir, 0755)

	err := m.stageFetched(FetchEntry{
		From:    srv.URL + "/missing.zip",
		Extract: "zip",
		Files:   []FileEntry{{Src: "x", Dst: "x"}},
	}, cacheDir, nil)

	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 error, got %v", err)
	}
}

func TestStageFetched_rejectsNonHTTPURL(t *testing.T) {
	m := newFetchManager(t)
	err := m.stageFetched(FetchEntry{
		From:    "file:///etc/passwd",
		Extract: "",
		Files:   []FileEntry{{Src: "passwd", Dst: "x"}},
	}, t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "http") {
		t.Errorf("file:// URL should be rejected, got %v", err)
	}
}

func TestUnzip_pathTraversalIgnored(t *testing.T) {
	zipBytes := makeZip(t, map[string]string{
		"../escape.txt":      "ESCAPED",
		"normal/inside.txt":  "OK",
	})
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "a.zip")
	os.WriteFile(archive, zipBytes, 0644)
	dest := filepath.Join(tmp, "extracted")

	if err := unzipTo(archive, dest); err != nil {
		t.Fatal(err)
	}
	// The traversal entry must not have written outside dest.
	if _, err := os.Stat(filepath.Join(tmp, "escape.txt")); err == nil {
		t.Errorf("path traversal entry escaped destDir")
	}
	if _, err := os.Stat(filepath.Join(dest, "normal/inside.txt")); err != nil {
		t.Errorf("legitimate entry should have been extracted: %v", err)
	}
}

func TestValidateManifest_fetch(t *testing.T) {
	cases := []struct {
		name      string
		m         AddonManifest
		wantError string
	}{
		{
			name: "valid fetch entry",
			m: AddonManifest{
				ID: "x", Name: "X", Version: "1",
				Fetch: []FetchEntry{
					{From: "https://example.com/a.zip", Extract: "zip", Files: []FileEntry{{Src: "a", Dst: "a"}}},
				},
			},
		},
		{
			name: "fetch missing from",
			m: AddonManifest{
				ID: "x", Name: "X", Version: "1",
				Fetch: []FetchEntry{{Extract: "zip", Files: []FileEntry{{Src: "a", Dst: "a"}}}},
			},
			wantError: "'fetch' entries need 'from'",
		},
		{
			name: "fetch non-http url",
			m: AddonManifest{
				ID: "x", Name: "X", Version: "1",
				Fetch: []FetchEntry{{From: "file:///etc/passwd", Extract: "", Files: []FileEntry{{Src: "x", Dst: "x"}}}},
			},
			wantError: "must be http(s) URL",
		},
		{
			name: "fetch unsupported extract",
			m: AddonManifest{
				ID: "x", Name: "X", Version: "1",
				Fetch: []FetchEntry{{From: "https://x/y.7z", Extract: "7z", Files: []FileEntry{{Src: "x", Dst: "x"}}}},
			},
			wantError: "fetch.extract",
		},
		{
			name: "fetch missing files",
			m: AddonManifest{
				ID: "x", Name: "X", Version: "1",
				Fetch: []FetchEntry{{From: "https://x/y.zip", Extract: "zip"}},
			},
			wantError: "need at least one 'files' mapping",
		},
		{
			name: "fetch traversal in dst",
			m: AddonManifest{
				ID: "x", Name: "X", Version: "1",
				Fetch: []FetchEntry{{From: "https://x/y.zip", Extract: "zip", Files: []FileEntry{{Src: "x", Dst: "../../etc/passwd"}}}},
			},
			wantError: "must not contain '..'",
		},
		{
			name: "fetch-only manifest (no files)",
			m: AddonManifest{
				ID: "x", Name: "X", Version: "1",
				Fetch: []FetchEntry{{From: "https://x/y.zip", Extract: "zip", Files: []FileEntry{{Src: "x", Dst: "x"}}}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateManifest(tc.m)
			if tc.wantError == "" {
				if err != nil {
					t.Errorf("expected success, got %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tc.wantError)
				return
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("expected error containing %q, got %q", tc.wantError, err.Error())
			}
		})
	}
}
