// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

// Tests for Part A: debug command versioning and metadata improvements.
// Covers getVersionInfo(), IsDev detection, UserAgent formatting, and the
// version command's JSON / text output paths.

package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dotandev/glassbox/internal/version"
)

// ── getVersionInfo ────────────────────────────────────────────────────────────

// TestGetVersionInfo_VersionFieldPopulated verifies that getVersionInfo always
// returns a non-empty Version string.
func TestGetVersionInfo_VersionFieldPopulated(t *testing.T) {
	info := getVersionInfo()
	if info.Version == "" {
		t.Error("getVersionInfo() returned empty Version")
	}
}

// TestGetVersionInfo_UserAgentContainsVersion verifies that the UserAgent field
// carries the version string.
func TestGetVersionInfo_UserAgentContainsVersion(t *testing.T) {
	info := getVersionInfo()
	if !strings.Contains(info.UserAgent, info.Version) {
		t.Errorf("UserAgent %q does not contain version %q", info.UserAgent, info.Version)
	}
}

// TestGetVersionInfo_IsDev_DevBuild verifies IsDev is true for the default dev stamp.
func TestGetVersionInfo_IsDev_DevBuild(t *testing.T) {
	origV := version.Version
	t.Cleanup(func() { version.Version = origV })

	version.Version = "0.0.0-dev"
	info := getVersionInfo()
	if !info.IsDev {
		t.Error("IsDev should be true when Version is 0.0.0-dev")
	}
}

// TestGetVersionInfo_IsDev_ReleaseBuild verifies IsDev is false for stamped builds.
func TestGetVersionInfo_IsDev_ReleaseBuild(t *testing.T) {
	origV := version.Version
	t.Cleanup(func() { version.Version = origV })

	version.Version = "2.0.0"
	info := getVersionInfo()
	if info.IsDev {
		t.Error("IsDev should be false for a release version")
	}
}

// TestGetVersionInfo_GoVersionSet verifies that GoVersion is populated (not empty).
func TestGetVersionInfo_GoVersionSet(t *testing.T) {
	info := getVersionInfo()
	if info.GoVersion == "" {
		t.Error("getVersionInfo() returned empty GoVersion")
	}
}

// ── version command — text output ────────────────────────────────────────────

// TestVersionCmd_TextOutput_ContainsVersion verifies the text output includes the
// version string.
func TestVersionCmd_TextOutput_ContainsVersion(t *testing.T) {
	origV := version.Version
	t.Cleanup(func() { version.Version = origV })
	version.Version = "1.2.3"

	var out bytes.Buffer
	versionCmd.SetOut(&out)
	t.Cleanup(func() { versionCmd.SetOut(nil) })

	if err := versionCmd.RunE(versionCmd, []string{}); err != nil {
		t.Fatalf("version command returned error: %v", err)
	}
	if !strings.Contains(out.String(), "1.2.3") {
		t.Errorf("text output does not contain version; got: %q", out.String())
	}
}

// TestVersionCmd_TextOutput_DevWarning verifies that dev builds include a clear
// "(dev build)" warning in the text output so users know the binary is unstamped.
func TestVersionCmd_TextOutput_DevWarning(t *testing.T) {
	origV := version.Version
	t.Cleanup(func() { version.Version = origV })
	version.Version = "0.0.0-dev"

	var out bytes.Buffer
	versionCmd.SetOut(&out)
	t.Cleanup(func() { versionCmd.SetOut(nil) })

	if err := versionCmd.RunE(versionCmd, []string{}); err != nil {
		t.Fatalf("version command returned error: %v", err)
	}
	if !strings.Contains(out.String(), "dev build") {
		t.Errorf("text output should warn about dev build; got: %q", out.String())
	}
}

// TestVersionCmd_TextOutput_UserAgentLine verifies the text output includes the
// User-Agent line so operators can confirm what header will be sent to RPC.
func TestVersionCmd_TextOutput_UserAgentLine(t *testing.T) {
	var out bytes.Buffer
	versionCmd.SetOut(&out)
	t.Cleanup(func() { versionCmd.SetOut(nil) })

	if err := versionCmd.RunE(versionCmd, []string{}); err != nil {
		t.Fatalf("version command returned error: %v", err)
	}
	if !strings.Contains(out.String(), "User-Agent") {
		t.Errorf("text output should include User-Agent line; got: %q", out.String())
	}
}

// ── version command — JSON output ────────────────────────────────────────────

// TestVersionCmd_JSONOutput_Valid verifies the JSON output is valid JSON and
// contains all required fields.
func TestVersionCmd_JSONOutput_Valid(t *testing.T) {
	// Set the json flag on versionCmd then restore.
	if err := versionCmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("failed to set json flag: %v", err)
	}
	t.Cleanup(func() { _ = versionCmd.Flags().Set("json", "false") })

	var out bytes.Buffer
	versionCmd.SetOut(&out)
	t.Cleanup(func() { versionCmd.SetOut(nil) })

	if err := versionCmd.RunE(versionCmd, []string{}); err != nil {
		t.Fatalf("version command returned error: %v", err)
	}

	var parsed VersionInfo
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v\nOutput: %s", err, out.String())
	}

	if parsed.Version == "" {
		t.Error("JSON output missing version field")
	}
	if parsed.UserAgent == "" {
		t.Error("JSON output missing user_agent field")
	}
	if parsed.GoVersion == "" {
		t.Error("JSON output missing go_version field")
	}
}

// TestVersionCmd_JSONOutput_UserAgentContainsVersion verifies that the
// user_agent JSON field embeds the version string.
func TestVersionCmd_JSONOutput_UserAgentContainsVersion(t *testing.T) {
	origV := version.Version
	t.Cleanup(func() { version.Version = origV })
	version.Version = "3.1.4"

	if err := versionCmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("failed to set json flag: %v", err)
	}
	t.Cleanup(func() { _ = versionCmd.Flags().Set("json", "false") })

	var out bytes.Buffer
	versionCmd.SetOut(&out)
	t.Cleanup(func() { versionCmd.SetOut(nil) })

	if err := versionCmd.RunE(versionCmd, []string{}); err != nil {
		t.Fatalf("version command returned error: %v", err)
	}

	var parsed VersionInfo
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if !strings.Contains(parsed.UserAgent, "3.1.4") {
		t.Errorf("user_agent %q should contain version 3.1.4", parsed.UserAgent)
	}
}
