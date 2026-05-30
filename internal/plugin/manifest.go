// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ManifestVersion is the current schema version for plugin manifests.
const ManifestVersion = "1"

// ManifestFileName is the conventional name for a plugin manifest file.
const ManifestFileName = "plugin.json"

// Capability represents a named capability a plugin declares it provides.
type Capability string

const (
	// CapabilityDecoder allows the plugin to decode custom event types.
	CapabilityDecoder Capability = "decoder"
	// CapabilityAnalyzer allows the plugin to contribute analysis hooks.
	CapabilityAnalyzer Capability = "analyzer"
	// CapabilityTraceViewer allows the plugin to register a custom trace viewer.
	CapabilityTraceViewer Capability = "trace_viewer"
	// CapabilityArtifactLoader allows the plugin to load custom artifact formats.
	CapabilityArtifactLoader Capability = "artifact_loader"
)

// Permission represents a runtime permission a plugin requires.
type Permission string

const (
	// PermissionReadFS allows the plugin to read from the filesystem.
	PermissionReadFS Permission = "read_fs"
	// PermissionNetwork allows the plugin to make outbound network calls.
	PermissionNetwork Permission = "network"
	// PermissionWriteFS allows the plugin to write to the filesystem.
	PermissionWriteFS Permission = "write_fs"
)

// validCapabilities is the set of recognised capability strings.
var validCapabilities = map[Capability]bool{
	CapabilityDecoder:        true,
	CapabilityAnalyzer:       true,
	CapabilityTraceViewer:    true,
	CapabilityArtifactLoader: true,
}

// validPermissions is the set of recognised permission strings.
var validPermissions = map[Permission]bool{
	PermissionReadFS:  true,
	PermissionNetwork: true,
	PermissionWriteFS: true,
}

// semverRE is a loose semver pattern: MAJOR.MINOR.PATCH with optional pre-release.
var semverRE = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?$`)

// pluginNameRE restricts plugin names to safe identifier characters.
var pluginNameRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

// Manifest describes a plugin's identity, capabilities, and requirements.
// It is loaded from a plugin.json file alongside the plugin binary.
type Manifest struct {
	// SchemaVersion is the manifest format version (currently "1").
	SchemaVersion string `json:"schema_version"`

	// Name is the unique plugin identifier. Must match [a-zA-Z][a-zA-Z0-9_-]{0,63}.
	Name string `json:"name"`

	// Version is the plugin's own semver string (e.g. "1.2.3").
	Version string `json:"version"`

	// APIVersion is the Glassbox plugin API version this plugin targets.
	// Must match the current plugin.Version constant.
	APIVersion string `json:"api_version"`

	// Entrypoint is the path to the plugin binary or shared library,
	// relative to the manifest file's directory.
	Entrypoint string `json:"entrypoint"`

	// Description is a human-readable summary of what the plugin does.
	Description string `json:"description,omitempty"`

	// Author is the plugin author or organisation name.
	Author string `json:"author,omitempty"`

	// Capabilities lists the extension points this plugin provides.
	Capabilities []Capability `json:"capabilities"`

	// Permissions lists the runtime permissions the plugin requires.
	// The loader will refuse to start a plugin that requests unknown permissions.
	Permissions []Permission `json:"permissions,omitempty"`

	// EventTypes lists the event type strings this plugin can decode.
	// Only meaningful when CapabilityDecoder is declared.
	EventTypes []string `json:"event_types,omitempty"`

	// Checksum is the optional SHA-256 hex digest of the entrypoint binary.
	// When present the loader verifies the binary before execution.
	Checksum string `json:"checksum,omitempty"`
}

// Validate checks that all required fields are present and well-formed.
func (m *Manifest) Validate() error {
	if m.SchemaVersion != ManifestVersion {
		return fmt.Errorf("unsupported manifest schema_version %q (expected %q)", m.SchemaVersion, ManifestVersion)
	}
	if !pluginNameRE.MatchString(m.Name) {
		return fmt.Errorf("invalid plugin name %q: must match [a-zA-Z][a-zA-Z0-9_-]{0,63}", m.Name)
	}
	if !semverRE.MatchString(m.Version) {
		return fmt.Errorf("invalid plugin version %q: must be semver (MAJOR.MINOR.PATCH)", m.Version)
	}
	if m.APIVersion != Version {
		return fmt.Errorf("plugin API version %q does not match current %q", m.APIVersion, Version)
	}
	if strings.TrimSpace(m.Entrypoint) == "" {
		return fmt.Errorf("entrypoint must not be empty")
	}
	if len(m.Capabilities) == 0 {
		return fmt.Errorf("plugin must declare at least one capability")
	}
	for _, cap := range m.Capabilities {
		if !validCapabilities[cap] {
			return fmt.Errorf("unknown capability %q", cap)
		}
	}
	for _, perm := range m.Permissions {
		if !validPermissions[perm] {
			return fmt.Errorf("unknown permission %q", perm)
		}
	}
	return nil
}

// HasCapability reports whether the manifest declares the given capability.
func (m *Manifest) HasCapability(cap Capability) bool {
	for _, c := range m.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// HasPermission reports whether the manifest requests the given permission.
func (m *Manifest) HasPermission(perm Permission) bool {
	for _, p := range m.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// LoadManifest reads and validates a manifest from the given file path.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest %s: %w", path, err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest %s: %w", path, err)
	}

	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("manifest %s is invalid: %w", path, err)
	}

	return &m, nil
}

// DiscoverManifests scans dir for subdirectories each containing a plugin.json
// and returns all valid manifests found. Errors for individual manifests are
// collected and returned alongside the valid set so callers can log them
// without aborting the entire discovery pass.
func DiscoverManifests(dir string) ([]*Manifest, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to read plugin directory %s: %w", dir, err)}
	}

	var manifests []*Manifest
	var errs []error

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), ManifestFileName)
		m, err := LoadManifest(manifestPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		manifests = append(manifests, m)
	}

	return manifests, errs
}
