// Package version provides build and version information.
package version

import (
	"fmt"
	"runtime"
)

// Build information set via ldflags.
var (
	// Version is the semantic version.
	Version = "1.0.0"
	// Commit is the git commit hash.
	Commit = "unknown"
	// BuildDate is the build timestamp.
	BuildDate = "unknown"
)

// Info returns formatted version information.
func Info() string {
	return fmt.Sprintf("audiobookshelf-sonos-bridge %s (commit: %s, built: %s, go: %s)",
		Version, Commit, BuildDate, runtime.Version())
}

// Short returns just the version number.
func Short() string {
	return Version
}

// Full returns detailed version information as a map.
func Full() map[string]string {
	return map[string]string{
		"version":    Version,
		"commit":     Commit,
		"build_date": BuildDate,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}
}
