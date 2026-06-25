// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"strings"
	"testing"
)

func TestIsDev_DefaultVersion(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "0.0.0-dev"
	if !IsDev() {
		t.Error("IsDev() should return true for default dev version")
	}
}

func TestIsDev_RealVersion(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "1.2.3"
	if IsDev() {
		t.Error("IsDev() should return false for a real version")
	}
}

func TestShortSHA_Full(t *testing.T) {
	orig := CommitSHA
	t.Cleanup(func() { CommitSHA = orig })

	CommitSHA = "abcdef1234567890"
	got := ShortSHA()
	if got != "abcdef12" {
		t.Errorf("ShortSHA() = %q; want %q", got, "abcdef12")
	}
}

func TestShortSHA_Short(t *testing.T) {
	orig := CommitSHA
	t.Cleanup(func() { CommitSHA = orig })

	CommitSHA = "abc"
	got := ShortSHA()
	if got != "abc" {
		t.Errorf("ShortSHA() = %q; want %q", got, "abc")
	}
}

func TestShortSHA_Unknown(t *testing.T) {
	orig := CommitSHA
	t.Cleanup(func() { CommitSHA = orig })

	CommitSHA = "unknown"
	got := ShortSHA()
	if got != "unknown" {
		t.Errorf("ShortSHA() = %q; want %q", got, "unknown")
	}
}

func TestUserAgent_Format(t *testing.T) {
	origV := Version
	origC := CommitSHA
	t.Cleanup(func() {
		Version = origV
		CommitSHA = origC
	})

	Version = "1.2.3"
	CommitSHA = "deadbeef1234"

	ua := UserAgent()
	if !strings.HasPrefix(ua, "glassbox/1.2.3") {
		t.Errorf("UserAgent() = %q; expected prefix glassbox/1.2.3", ua)
	}
	if !strings.Contains(ua, "deadbeef") {
		t.Errorf("UserAgent() = %q; expected short SHA deadbeef", ua)
	}
}

func TestUserAgent_DevBuild(t *testing.T) {
	origV := Version
	origC := CommitSHA
	t.Cleanup(func() {
		Version = origV
		CommitSHA = origC
	})

	Version = "0.0.0-dev"
	CommitSHA = "unknown"

	ua := UserAgent()
	if !strings.Contains(ua, "0.0.0-dev") {
		t.Errorf("UserAgent() = %q; expected dev version", ua)
	}
}
