// Package version exposes the launcher build version. The default "dev"
// is overridden at release-build time via:
//
//	go build -ldflags "-X 'launcher/pkg/version.Version=v0.2.1'"
//
// or with wails build's -ldflags equivalent. The release workflow passes
// the git tag name so users see the same version they downloaded.
package version

import (
	"runtime/debug"
	"strings"
)

// Version is the human-readable launcher version. Set via ldflags at release
// build time; falls back to a vcs.revision short SHA on `go run`/`wails dev`,
// or the literal "dev" if no VCS info is embedded.
var Version = "dev"

// String returns the launcher version, augmenting tagged releases with a
// short SHA when available so bug reports can pinpoint the exact build.
// Example outputs:
//
//	"v0.2.1"          — release build, ldflags-injected
//	"v0.2.1+a1b2c3d"  — release build with VCS info (if both are present)
//	"dev+a1b2c3d"     — local build from a git checkout
//	"dev"             — local build with no VCS info (e.g. tarball)
func String() string {
	v := Version
	sha := shortSHA()
	if sha != "" && !strings.Contains(v, sha) {
		return v + "+" + sha
	}
	return v
}

// shortSHA returns the first 7 chars of the build's vcs.revision, or "".
// Wails builds embed VCS info by default when the working tree is clean.
func shortSHA() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			return s.Value[:7]
		}
	}
	return ""
}
