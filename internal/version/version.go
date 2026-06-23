// Package version exposes build-time provenance for the KubePilot binary.
//
// The fields below are overridden at link time via -ldflags "-X ...". When the
// binary is built without those flags (e.g. `go run`/`go test`), the defaults
// describe a local, unstamped build.
package version

import (
	"fmt"
	"runtime"
)

// These values are injected at build time by scripts/build-kubepilot.sh and the
// Makefile. Keep the variable names and this import path in sync with the
// -X flags there.
var (
	// Version is the human-readable release/version string (git describe).
	Version = "dev"
	// BuildTime is the UTC build timestamp, formatted as YYYY-MM-DD.HHMMSSZ,
	// which doubles as the build number.
	BuildTime = "unknown"
	// Commit is the short git commit the binary was built from.
	Commit = "unknown"
)

// Info is a structured snapshot of the build provenance, suitable for JSON.
type Info struct {
	Version   string `json:"version"`
	Build     string `json:"build"`
	Commit    string `json:"commit"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// Get returns the current build provenance.
func Get() Info {
	return Info{
		Version:   Version,
		Build:     BuildTime,
		Commit:    Commit,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// String renders a single-line summary, e.g.
// "kubepilot v1.2.3 (build 2026-06-23.121617Z, commit baf2cca)".
func String() string {
	return fmt.Sprintf("kubepilot %s (build %s, commit %s)", Version, BuildTime, Commit)
}
