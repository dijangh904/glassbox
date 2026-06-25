// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package version

import "fmt"

var (
	// Version is the SDK version, populated by ldflags during build.
	// Defaults to "0.0.0-dev" when running without a proper build.
	Version = "0.0.0-dev"
	// CommitSHA is the git commit SHA, populated by ldflags during build.
	CommitSHA = "unknown"
	// BuildDate is the build date, populated by ldflags during build.
	BuildDate = "unknown"
)

// IsDev reports whether the binary was built without ldflags version injection.
// This is true when Version still carries the default "0.0.0-dev" placeholder.
func IsDev() bool {
	return Version == "0.0.0-dev"
}

// ShortSHA returns the first 8 characters of CommitSHA, or "unknown" if unset.
func ShortSHA() string {
	if len(CommitSHA) >= 8 {
		return CommitSHA[:8]
	}
	return CommitSHA
}

// UserAgent returns a User-Agent / metadata string suitable for RPC headers
// and diagnostic output: "glassbox/<version> (<commit>)".
func UserAgent() string {
	return fmt.Sprintf("glassbox/%s (%s)", Version, ShortSHA())
}
