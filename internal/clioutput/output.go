// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

// Package clioutput provides a shared machine-readable JSON envelope for CLI commands.
package clioutput

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dotandev/glassbox/internal/version"
)

// SchemaVersion is the stable schema version for structured CLI output.
const SchemaVersion = "1.0"

// Envelope wraps structured command output with schema metadata.
type Envelope struct {
	SchemaVersion   string          `json:"schema_version"`
	GlassboxVersion string          `json:"glassbox_version"`
	GeneratedAt     time.Time       `json:"generated_at"`
	Command         string          `json:"command,omitempty"`
	Data            json.RawMessage `json:"data"`
}

// WantsJSON reports whether JSON output was requested via --json or --format json.
func WantsJSON(jsonFlag bool, format string) bool {
	if jsonFlag {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(format), "json")
}

// Write writes a JSON envelope to w.
func Write(w io.Writer, command string, data interface{}) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal output data: %w", err)
	}
	env := Envelope{
		SchemaVersion:   SchemaVersion,
		GlassboxVersion: version.Version,
		GeneratedAt:     time.Now().UTC(),
		Command:         command,
		Data:            raw,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

// WriteStdout is a convenience wrapper around Write(os.Stdout, ...).
func WriteStdout(command string, data interface{}) error {
	return Write(os.Stdout, command, data)
}
