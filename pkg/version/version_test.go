package version

import (
	"strings"
	"testing"
)

func TestStringRespectsLdflagsOverride(t *testing.T) {
	saved := Version
	defer func() { Version = saved }()

	Version = "v9.9.9"
	got := String()
	if !strings.HasPrefix(got, "v9.9.9") {
		t.Errorf("String() should start with the injected version; got %q", got)
	}
}

func TestStringFallsBackToDevWhenNotInjected(t *testing.T) {
	saved := Version
	defer func() { Version = saved }()

	Version = "dev"
	got := String()
	if !strings.HasPrefix(got, "dev") {
		t.Errorf("String() should start with %q when Version=dev; got %q", "dev", got)
	}
}

func TestShortSHAFromBuildInfo(t *testing.T) {
	// shortSHA reads the binary's embedded VCS info. In `go test` we can't
	// reliably predict the SHA, but it should either be empty (no VCS) or
	// 7 chars of hex.
	sha := shortSHA()
	if sha == "" {
		t.Skip("no VCS info embedded — environment-dependent, not a failure")
	}
	if len(sha) != 7 {
		t.Errorf("shortSHA() should be 7 chars when present; got %d (%q)", len(sha), sha)
	}
	for _, r := range sha {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !isHex {
			t.Errorf("shortSHA() should be hex; got %q", sha)
			break
		}
	}
}
