package proton

import "testing"

func TestComposeDLLOverrides(t *testing.T) {
	tests := []struct {
		name  string
		extra []string
		want  string
	}{
		{
			name:  "nil keeps baseline only",
			extra: nil,
			want:  "WINEDLLOVERRIDES=quartz=n,b",
		},
		{
			name:  "empty slice keeps baseline only",
			extra: []string{},
			want:  "WINEDLLOVERRIDES=quartz=n,b",
		},
		{
			name:  "dgVoodoo + ReShade chain",
			extra: []string{"d3d8", "dxgi"},
			want:  "WINEDLLOVERRIDES=quartz=n,b;d3d8=n,b;dxgi=n,b",
		},
		{
			name:  "dedup against baseline (quartz)",
			extra: []string{"quartz", "d3d8"},
			want:  "WINEDLLOVERRIDES=quartz=n,b;d3d8=n,b",
		},
		{
			name:  "case insensitive dedup",
			extra: []string{"D3D8", "d3d8", "DxGi"},
			want:  "WINEDLLOVERRIDES=quartz=n,b;d3d8=n,b;dxgi=n,b",
		},
		{
			name:  "whitespace and empties dropped",
			extra: []string{"  d3d8  ", "", "   ", "dxgi"},
			want:  "WINEDLLOVERRIDES=quartz=n,b;d3d8=n,b;dxgi=n,b",
		},
		{
			name:  "preserves order of first occurrence",
			extra: []string{"dxgi", "d3d8", "ddraw"},
			want:  "WINEDLLOVERRIDES=quartz=n,b;dxgi=n,b;d3d8=n,b;ddraw=n,b",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ComposeDLLOverrides(tc.extra)
			if got != tc.want {
				t.Errorf("ComposeDLLOverrides(%v)\n  got:  %q\n  want: %q", tc.extra, got, tc.want)
			}
		})
	}
}
