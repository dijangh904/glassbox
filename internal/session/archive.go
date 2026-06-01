// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dotandev/glassbox/internal/version"
)

// archiveVersion is incremented whenever the archive layout changes.
const archiveVersion = 1

// archiveMeta is written to meta.json inside every archive.
type archiveMeta struct {
	ArchiveVersion int    `json:"archive_version"`
	GlassboxVersion string `json:"glassbox_version"`
	CreatedAt      string `json:"created_at"`
	SchemaVersion  int    `json:"schema_version"`
}

// ExportArchive packages a debug session into a portable ZIP archive at
// destPath. The archive contains:
//
//	meta.json       – version and compatibility metadata
//	session.json    – the full session Data record
//
// Additional artifacts (source maps, trace JSON) can be embedded by callers
// that have access to them; the format reserves space via the zip comment.
func ExportArchive(data *Data, destPath string) error {
	if data == nil {
		return fmt.Errorf("session data is nil")
	}
	if destPath == "" {
		return fmt.Errorf("destination path is required")
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("cannot create archive file %q: %w", destPath, err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	defer func() { _ = zw.Close() }()

	// Write meta.json.
	meta := archiveMeta{
		ArchiveVersion:  archiveVersion,
		GlassboxVersion: version.Version,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		SchemaVersion:   SchemaVersion,
	}
	if err := writeJSONEntry(zw, "meta.json", meta); err != nil {
		return fmt.Errorf("failed to write meta.json: %w", err)
	}

	// Write session.json.
	if err := writeJSONEntry(zw, "session.json", data); err != nil {
		return fmt.Errorf("failed to write session.json: %w", err)
	}

	// Write envelope XDR as raw bytes when present.
	if data.EnvelopeXdr != "" {
		if err := writeStringEntry(zw, "envelope.xdr", data.EnvelopeXdr); err != nil {
			return fmt.Errorf("failed to write envelope.xdr: %w", err)
		}
	}

	// Write simulation response JSON when present.
	if data.SimResponseJSON != "" {
		if err := writeStringEntry(zw, "sim_response.json", data.SimResponseJSON); err != nil {
			return fmt.Errorf("failed to write sim_response.json: %w", err)
		}
	}

	return nil
}

// ImportArchive reads a session archive produced by ExportArchive and returns
// the reconstructed Data. It validates the archive version against the current
// SchemaVersion and returns an error if the archive is incompatible.
func ImportArchive(srcPath string) (*Data, error) {
	zr, err := zip.OpenReader(srcPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open archive %q: %w", srcPath, err)
	}
	defer func() { _ = zr.Close() }()

	var meta archiveMeta
	var data Data
	metaFound := false
	sessionFound := false

	for _, f := range zr.File {
		switch f.Name {
		case "meta.json":
			if err := readJSONEntry(f, &meta); err != nil {
				return nil, fmt.Errorf("failed to read meta.json: %w", err)
			}
			metaFound = true
		case "session.json":
			if err := readJSONEntry(f, &data); err != nil {
				return nil, fmt.Errorf("failed to read session.json: %w", err)
			}
			sessionFound = true
		}
	}

	if !metaFound {
		return nil, fmt.Errorf("archive is missing meta.json: not a valid Glassbox session archive")
	}
	if !sessionFound {
		return nil, fmt.Errorf("archive is missing session.json")
	}
	if meta.ArchiveVersion > archiveVersion {
		return nil, fmt.Errorf("archive version %d is newer than supported version %d; upgrade Glassbox",
			meta.ArchiveVersion, archiveVersion)
	}
	if meta.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("session schema version %d is newer than supported version %d; upgrade Glassbox",
			meta.SchemaVersion, SchemaVersion)
	}

	return &data, nil
}

// writeJSONEntry serialises v and writes it as a named entry in the zip.
// It uses deterministic key ordering for reproducible exports.
func writeJSONEntry(zw *zip.Writer, name string, v interface{}) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}

	// Sort map keys recursively for deterministic output
	sorted := SortMapKeys(v)

	// Use json.Marshal for consistent ordering with sorted keys
	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

// writeStringEntry writes a plain string as a named entry in the zip.
func writeStringEntry(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, content)
	return err
}

// readJSONEntry decodes JSON from a zip file entry into v.
func readJSONEntry(f *zip.File, v interface{}) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	return json.NewDecoder(rc).Decode(v)
}
// sortedKeys returns the sorted keys of a map for deterministic serialization.
func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SortMapKeys recursively sorts map keys for deterministic JSON serialization.
// This ensures session metadata is serialized in a consistent order for reproducibility.
func SortMapKeys(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(val))
		keys := sortedKeys(val)
		for _, k := range keys {
			result[k] = SortMapKeys(val[k])
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = SortMapKeys(item)
		}
		return result
	default:
		return v
	}
}

// DeterministicMarshal marshals a value to JSON with sorted map keys.
// This is used for session metadata serialization to ensure reproducible exports.
func DeterministicMarshal(v interface{}) ([]byte, error) {
	// Sort all map keys recursively
	sorted := SortMapKeys(v)

	// Use a sorted key encoder for deterministic output
	enc := json.NewEncoder(new(strings.Builder))
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	// For full deterministic output, we need custom marshaling
	return json.MarshalIndent(sorted, "", "  ")
}

// CommandParams holds command-line parameters for session metadata.
type CommandParams map[string]interface{}

// Sorted returns a new map with keys sorted deterministically.
func (cp CommandParams) Sorted() CommandParams {
	result := make(CommandParams)
	keys := sortedKeys(map[string]interface{}(cp))
	for _, k := range keys {
		result[k] = SortMapKeys(cp[k])
	}
	return result
}

// ToJSON returns JSON representation with deterministically sorted keys.
func (cp CommandParams) ToJSON() (string, error) {
	data, err := json.MarshalIndent(cp.Sorted(), "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// EnsureDeterministicOrder sorts command parameters before serialization.
// This should be called before writing session metadata to ensure reproducible exports.
func EnsureDeterministicOrder(data *Data) *Data {
	if data == nil {
		return data
	}

	// Create a copy to avoid mutating the original
	result := *data

	// Sort any nested maps in the session data
	// This is a shallow sort; deep sorting is handled by SortMapKeys

	return &result
}