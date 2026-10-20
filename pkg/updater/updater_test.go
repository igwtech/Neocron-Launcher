package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"NeocronEvolution-2.5.897", "2.5.110", 1},
		{"NeocronEvolution-2.5.897", "NeocronEvolution-2.5.897", 0},
		{"NeocronEvolution-2.5.862", "NeocronEvolution-2.5.897", -1},
		{"2.5.110", "2.5.99", 1},
		{"2.6", "2.5.999", 1},
		{"2.5", "2.5.0", 0},
		{"0.0", "NeocronEvolution-2.5.897", -1},
		{"unknown", "0.0", 0},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestLocalVersionPrefersRecordedCDNVersion(t *testing.T) {
	dir := t.TempDir()
	u := NewUpdater("http://example.invalid", dir)
	if v, _ := u.GetLocalVersion(); v != "0.0" {
		t.Fatalf("empty install: got %q, want 0.0", v)
	}
	if err := os.WriteFile(filepath.Join(dir, installedVersionFile), []byte("NeocronEvolution-2.5.897\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if v, _ := u.GetLocalVersion(); v != "NeocronEvolution-2.5.897" {
		t.Fatalf("got %q, want the recorded version", v)
	}
}
