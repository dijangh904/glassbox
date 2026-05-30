// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dotandev/glassbox/internal/plugin"
)

// writePluginManifest writes a plugin.json into dir/pluginName/plugin.json.
func writePluginManifest(t *testing.T, baseDir, pluginName string, m *plugin.Manifest) string {
	t.Helper()
	pluginDir := filepath.Join(baseDir, pluginName)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	path := filepath.Join(pluginDir, plugin.ManifestFileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	return path
}

func validTestManifest(name string) *plugin.Manifest {
	return &plugin.Manifest{
		SchemaVersion: plugin.ManifestVersion,
		Name:          name,
		Version:       "1.0.0",
		APIVersion:    plugin.Version,
		Entrypoint:    "plugin-binary",
		Capabilities:  []plugin.Capability{plugin.CapabilityDecoder},
		Description:   "A test plugin",
	}
}

// --- resolvePluginDir ---

func TestResolvePluginDir_WithFlag(t *testing.T) {
	dir := resolvePluginDir("/custom/plugins")
	if dir != "/custom/plugins" {
		t.Errorf("expected /custom/plugins, got %s", dir)
	}
}

func TestResolvePluginDir_Empty(t *testing.T) {
	dir := resolvePluginDir("")
	if dir == "" {
		t.Error("expected non-empty default plugin dir")
	}
}

// --- capabilitiesString ---

func TestCapabilitiesString_Empty(t *testing.T) {
	if got := capabilitiesString(nil); got != "-" {
		t.Errorf("expected -, got %s", got)
	}
}

func TestCapabilitiesString_Single(t *testing.T) {
	got := capabilitiesString([]plugin.Capability{plugin.CapabilityDecoder})
	if got != "decoder" {
		t.Errorf("expected decoder, got %s", got)
	}
}

func TestCapabilitiesString_Multiple(t *testing.T) {
	got := capabilitiesString([]plugin.Capability{plugin.CapabilityDecoder, plugin.CapabilityAnalyzer})
	if got != "decoder,analyzer" {
		t.Errorf("expected decoder,analyzer, got %s", got)
	}
}

// --- permissionsString ---

func TestPermissionsString_Empty(t *testing.T) {
	if got := permissionsString(nil); got != "-" {
		t.Errorf("expected -, got %s", got)
	}
}

func TestPermissionsString_Multiple(t *testing.T) {
	got := permissionsString([]plugin.Permission{plugin.PermissionReadFS, plugin.PermissionNetwork})
	if got != "read_fs,network" {
		t.Errorf("expected read_fs,network, got %s", got)
	}
}

// --- runPluginList ---

func TestRunPluginList_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	pluginDirFlag = dir
	defer func() { pluginDirFlag = "" }()

	cmd := pluginListCmd
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Errorf("expected no error for empty dir, got: %v", err)
	}
}

func TestRunPluginList_WithPlugins(t *testing.T) {
	dir := t.TempDir()
	writePluginManifest(t, dir, "my-decoder", validTestManifest("my-decoder"))
	writePluginManifest(t, dir, "my-analyzer", func() *plugin.Manifest {
		m := validTestManifest("my-analyzer")
		m.Capabilities = []plugin.Capability{plugin.CapabilityAnalyzer}
		return m
	}())

	pluginDirFlag = dir
	defer func() { pluginDirFlag = "" }()

	cmd := pluginListCmd
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestRunPluginList_NonexistentDir(t *testing.T) {
	pluginDirFlag = "/nonexistent/plugins"
	defer func() { pluginDirFlag = "" }()

	cmd := pluginListCmd
	// Should not return an error — it prints a warning and continues.
	_ = cmd.RunE(cmd, nil)
}

// --- runPluginRegister ---

func TestRunPluginRegister_MissingManifestFlag(t *testing.T) {
	pluginManifestFlag = ""
	cmd := pluginRegisterCmd
	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Error("expected error for missing --manifest flag")
	}
}

func TestRunPluginRegister_ValidManifestNoBinary(t *testing.T) {
	dir := t.TempDir()
	m := validTestManifest("test-plugin")
	path := filepath.Join(dir, plugin.ManifestFileName)
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	pluginManifestFlag = path
	defer func() { pluginManifestFlag = "" }()

	cmd := pluginRegisterCmd
	err := cmd.RunE(cmd, nil)
	// Binary doesn't exist, so we expect an error about the binary.
	if err == nil {
		t.Error("expected error for missing binary")
	}
}

func TestRunPluginRegister_ValidManifestWithBinary(t *testing.T) {
	dir := t.TempDir()
	// Create a dummy binary.
	binaryPath := filepath.Join(dir, "plugin-binary")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0755); err != nil {
		t.Fatal(err)
	}

	m := validTestManifest("test-plugin")
	path := filepath.Join(dir, plugin.ManifestFileName)
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	pluginManifestFlag = path
	defer func() { pluginManifestFlag = "" }()

	cmd := pluginRegisterCmd
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Errorf("expected success, got: %v", err)
	}
}

func TestRunPluginRegister_InvalidManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, plugin.ManifestFileName)
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}

	pluginManifestFlag = path
	defer func() { pluginManifestFlag = "" }()

	cmd := pluginRegisterCmd
	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Error("expected error for invalid manifest")
	}
}

// --- runPluginInspect ---

func TestRunPluginInspect_ValidPlugin(t *testing.T) {
	dir := t.TempDir()
	m := validTestManifest("inspect-plugin")
	m.Author = "Test Author"
	m.EventTypes = []string{"test.event"}
	m.Permissions = []plugin.Permission{plugin.PermissionReadFS}
	writePluginManifest(t, dir, "inspect-plugin", m)

	pluginDirFlag = dir
	defer func() { pluginDirFlag = "" }()

	cmd := pluginInspectCmd
	err := cmd.RunE(cmd, []string{"inspect-plugin"})
	if err != nil {
		t.Errorf("expected success, got: %v", err)
	}
}

func TestRunPluginInspect_NotFound(t *testing.T) {
	dir := t.TempDir()
	pluginDirFlag = dir
	defer func() { pluginDirFlag = "" }()

	cmd := pluginInspectCmd
	err := cmd.RunE(cmd, []string{"nonexistent-plugin"})
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}
