package updater

import "testing"

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
