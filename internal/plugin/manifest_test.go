// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func validManifest() *Manifest {
	return &Manifest{
		SchemaVersion: ManifestVersion,
		Name:          "test-plugin",
		Version:       "1.0.0",
		APIVersion:    Version,
		Entrypoint:    "plugin-binary",
		Capabilities:  []Capability{CapabilityDecoder},
	}
}

func writeManifest(t *testing.T, dir string, m *Manifest) string {
	t.Helper()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	path := filepath.Join(dir, ManifestFileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	return path
}

// --- Manifest.Validate ---

func TestManifestValidate_Valid(t *testing.T) {
	m := validManifest()
	if err := m.Validate(); err != nil {
		t.Errorf("expected valid manifest to pass, got: %v", err)
	}
}

func TestManifestValidate_WrongSchemaVersion(t *testing.T) {
	m := validManifest()
	m.SchemaVersion = "99"
	if err := m.Validate(); err == nil {
		t.Error("expected error for wrong schema version")
	}
}

func TestManifestValidate_InvalidName(t *testing.T) {
	cases := []string{"", "1starts-with-digit", "has space", "toolooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooong"}
	for _, name := range cases {
		m := validManifest()
		m.Name = name
		if err := m.Validate(); err == nil {
			t.Errorf("expected error for name %q", name)
		}
	}
}

func TestManifestValidate_ValidNames(t *testing.T) {
	cases := []string{"a", "my-plugin", "MyPlugin123", "plugin_v2"}
	for _, name := range cases {
		m := validManifest()
		m.Name = name
		if err := m.Validate(); err != nil {
			t.Errorf("expected valid name %q to pass, got: %v", name, err)
		}
	}
}

func TestManifestValidate_InvalidVersion(t *testing.T) {
	cases := []string{"", "1.0", "v1.0.0", "1.0.0.0"}
	for _, v := range cases {
		m := validManifest()
		m.Version = v
		if err := m.Validate(); err == nil {
			t.Errorf("expected error for version %q", v)
		}
	}
}

func TestManifestValidate_ValidVersionWithPreRelease(t *testing.T) {
	m := validManifest()
	m.Version = "1.2.3-beta.1"
	if err := m.Validate(); err != nil {
		t.Errorf("expected pre-release version to pass, got: %v", err)
	}
}

func TestManifestValidate_WrongAPIVersion(t *testing.T) {
	m := validManifest()
	m.APIVersion = "99.0.0"
	if err := m.Validate(); err == nil {
		t.Error("expected error for wrong API version")
	}
}

func TestManifestValidate_EmptyEntrypoint(t *testing.T) {
	m := validManifest()
	m.Entrypoint = ""
	if err := m.Validate(); err == nil {
		t.Error("expected error for empty entrypoint")
	}
}

func TestManifestValidate_NoCapabilities(t *testing.T) {
	m := validManifest()
	m.Capabilities = nil
	if err := m.Validate(); err == nil {
		t.Error("expected error for empty capabilities")
	}
}

func TestManifestValidate_UnknownCapability(t *testing.T) {
	m := validManifest()
	m.Capabilities = []Capability{"unknown_cap"}
	if err := m.Validate(); err == nil {
		t.Error("expected error for unknown capability")
	}
}

func TestManifestValidate_AllCapabilities(t *testing.T) {
	caps := []Capability{
		CapabilityDecoder,
		CapabilityAnalyzer,
		CapabilityTraceViewer,
		CapabilityArtifactLoader,
	}
	for _, cap := range caps {
		m := validManifest()
		m.Capabilities = []Capability{cap}
		if err := m.Validate(); err != nil {
			t.Errorf("expected capability %q to be valid, got: %v", cap, err)
		}
	}
}

func TestManifestValidate_UnknownPermission(t *testing.T) {
	m := validManifest()
	m.Permissions = []Permission{"sudo"}
	if err := m.Validate(); err == nil {
		t.Error("expected error for unknown permission")
	}
}

func TestManifestValidate_ValidPermissions(t *testing.T) {
	m := validManifest()
	m.Permissions = []Permission{PermissionReadFS, PermissionNetwork, PermissionWriteFS}
	if err := m.Validate(); err != nil {
		t.Errorf("expected valid permissions to pass, got: %v", err)
	}
}

// --- HasCapability / HasPermission ---

func TestManifestHasCapability(t *testing.T) {
	m := validManifest()
	m.Capabilities = []Capability{CapabilityDecoder, CapabilityAnalyzer}

	if !m.HasCapability(CapabilityDecoder) {
		t.Error("expected HasCapability(decoder) = true")
	}
	if !m.HasCapability(CapabilityAnalyzer) {
		t.Error("expected HasCapability(analyzer) = true")
	}
	if m.HasCapability(CapabilityTraceViewer) {
		t.Error("expected HasCapability(trace_viewer) = false")
	}
}

func TestManifestHasPermission(t *testing.T) {
	m := validManifest()
	m.Permissions = []Permission{PermissionReadFS}

	if !m.HasPermission(PermissionReadFS) {
		t.Error("expected HasPermission(read_fs) = true")
	}
	if m.HasPermission(PermissionNetwork) {
		t.Error("expected HasPermission(network) = false")
	}
}

// --- LoadManifest ---

func TestLoadManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	m := validManifest()
	path := writeManifest(t, dir, m)

	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	if loaded.Name != m.Name {
		t.Errorf("expected name %q, got %q", m.Name, loaded.Name)
	}
}

func TestLoadManifest_NotFound(t *testing.T) {
	_, err := LoadManifest("/nonexistent/path/plugin.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestFileName)
	if err := os.WriteFile(path, []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadManifest_FailsValidation(t *testing.T) {
	dir := t.TempDir()
	m := validManifest()
	m.Name = "" // invalid
	path := writeManifest(t, dir, m)

	_, err := LoadManifest(path)
	if err == nil {
		t.Error("expected validation error")
	}
}

// --- DiscoverManifests ---

func TestDiscoverManifests_Empty(t *testing.T) {
	dir := t.TempDir()
	manifests, errs := DiscoverManifests(dir)
	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests, got %d", len(manifests))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}

func TestDiscoverManifests_SinglePlugin(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "my-plugin")
	if err := os.Mkdir(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	m := validManifest()
	m.Name = "my-plugin"
	writeManifest(t, pluginDir, m)

	manifests, errs := DiscoverManifests(dir)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}
	if manifests[0].Name != "my-plugin" {
		t.Errorf("expected name my-plugin, got %s", manifests[0].Name)
	}
}

func TestDiscoverManifests_MultiplePlugins(t *testing.T) {
	dir := t.TempDir()
	names := []string{"plugin-a", "plugin-b", "plugin-c"}
	for _, name := range names {
		pluginDir := filepath.Join(dir, name)
		if err := os.Mkdir(pluginDir, 0755); err != nil {
			t.Fatal(err)
		}
		m := validManifest()
		m.Name = name
		writeManifest(t, pluginDir, m)
	}

	manifests, errs := DiscoverManifests(dir)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(manifests) != 3 {
		t.Errorf("expected 3 manifests, got %d", len(manifests))
	}
}

func TestDiscoverManifests_InvalidManifestCollected(t *testing.T) {
	dir := t.TempDir()

	// Valid plugin
	validDir := filepath.Join(dir, "valid-plugin")
	if err := os.Mkdir(validDir, 0755); err != nil {
		t.Fatal(err)
	}
	m := validManifest()
	m.Name = "valid-plugin"
	writeManifest(t, validDir, m)

	// Invalid plugin (bad JSON)
	badDir := filepath.Join(dir, "bad-plugin")
	if err := os.Mkdir(badDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, ManifestFileName), []byte("{bad"), 0644); err != nil {
		t.Fatal(err)
	}

	manifests, errs := DiscoverManifests(dir)
	if len(manifests) != 1 {
		t.Errorf("expected 1 valid manifest, got %d", len(manifests))
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error for bad manifest, got %d", len(errs))
	}
}

func TestDiscoverManifests_NonexistentDir(t *testing.T) {
	_, errs := DiscoverManifests("/nonexistent/path")
	if len(errs) == 0 {
		t.Error("expected error for nonexistent directory")
	}
}

func TestDiscoverManifests_SkipsFiles(t *testing.T) {
	dir := t.TempDir()
	// A plain file at the top level should be skipped (not a directory).
	if err := os.WriteFile(filepath.Join(dir, "not-a-dir.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	manifests, errs := DiscoverManifests(dir)
	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests, got %d", len(manifests))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}
