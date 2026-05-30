// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// sandboxTimeout is the maximum time a sandboxed plugin call may take.
const sandboxTimeout = 10 * time.Second

// SandboxRequest is the JSON envelope sent to a sandboxed plugin process over stdin.
type SandboxRequest struct {
	Method    string          `json:"method"`
	EventType string          `json:"event_type,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// SandboxResponse is the JSON envelope read from a sandboxed plugin process over stdout.
type SandboxResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// SandboxedPlugin wraps a plugin manifest and executes the plugin binary in a
// child process, communicating over stdin/stdout JSON. This provides runtime
// isolation: a crashing or misbehaving plugin cannot corrupt the host process.
type SandboxedPlugin struct {
	mu       sync.Mutex
	manifest *Manifest
	// binaryPath is the resolved absolute path to the plugin binary.
	binaryPath string
}

// NewSandboxedPlugin creates a SandboxedPlugin from a manifest.
// The manifest's Entrypoint is resolved relative to manifestDir.
func NewSandboxedPlugin(manifest *Manifest, manifestDir string) (*SandboxedPlugin, error) {
	binaryPath := manifest.Entrypoint
	if !isAbsPath(binaryPath) {
		binaryPath = joinPaths(manifestDir, binaryPath)
	}

	if _, err := os.Stat(binaryPath); err != nil {
		return nil, fmt.Errorf("plugin binary not found at %s: %w", binaryPath, err)
	}

	if manifest.Checksum != "" {
		if err := verifyChecksum(binaryPath, manifest.Checksum); err != nil {
			return nil, fmt.Errorf("plugin binary checksum mismatch: %w", err)
		}
	}

	return &SandboxedPlugin{
		manifest:   manifest,
		binaryPath: binaryPath,
	}, nil
}

// Name returns the plugin name from its manifest.
func (s *SandboxedPlugin) Name() string { return s.manifest.Name }

// Version returns the plugin version from its manifest.
func (s *SandboxedPlugin) Version() string { return s.manifest.Version }

// CanDecode reports whether this plugin can handle the given event type.
func (s *SandboxedPlugin) CanDecode(eventType string) bool {
	for _, et := range s.manifest.EventTypes {
		if et == eventType {
			return true
		}
	}
	return false
}

// Decode invokes the plugin binary in a sandboxed child process and returns
// the decoded result. If the child process crashes or times out the error is
// returned without affecting the host process.
func (s *SandboxedPlugin) Decode(data []byte) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req := SandboxRequest{
		Method: "decode",
		Data:   json.RawMessage(data),
	}
	return s.call(req)
}

// Metadata returns plugin capabilities derived from the manifest.
func (s *SandboxedPlugin) Metadata() Metadata {
	caps := make([]string, len(s.manifest.Capabilities))
	for i, c := range s.manifest.Capabilities {
		caps[i] = string(c)
	}
	return Metadata{
		Name:        s.manifest.Name,
		Version:     s.manifest.Version,
		APIVersion:  s.manifest.APIVersion,
		EventTypes:  s.manifest.EventTypes,
		Description: s.manifest.Description,
	}
}

// Init sends an initialization request to the plugin process.
// Errors are non-fatal: a plugin that fails to initialise is logged but
// does not prevent other plugins from loading.
func (s *SandboxedPlugin) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.call(SandboxRequest{Method: "init"})
	return err
}

// Cleanup sends a cleanup request to the plugin process.
func (s *SandboxedPlugin) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.call(SandboxRequest{Method: "cleanup"})
	return err
}

// call spawns the plugin binary, writes req as JSON to its stdin, reads the
// JSON response from stdout, and returns the result. The child process is
// killed if it does not respond within sandboxTimeout.
func (s *SandboxedPlugin) call(req SandboxRequest) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sandboxTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.binaryPath) //nolint:gosec
	cmd.Env = buildSandboxEnv(s.manifest)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	// Discard stderr to prevent plugin output from polluting the host terminal.
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start plugin process: %w", err)
	}

	// Write request.
	enc := json.NewEncoder(stdin)
	if err := enc.Encode(req); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("failed to write request to plugin: %w", err)
	}
	_ = stdin.Close()

	// Read response.
	var resp SandboxResponse
	dec := json.NewDecoder(stdout)
	if err := dec.Decode(&resp); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("failed to read response from plugin: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		// Non-zero exit is a plugin error, not a host error.
		if !resp.OK {
			return nil, fmt.Errorf("plugin %s returned error: %s", s.manifest.Name, resp.Error)
		}
	}

	if !resp.OK {
		return nil, fmt.Errorf("plugin %s: %s", s.manifest.Name, resp.Error)
	}

	return resp.Result, nil
}

// buildSandboxEnv constructs a minimal environment for the plugin process.
// Only variables explicitly required by the plugin's declared permissions are
// forwarded; all others are stripped to limit information leakage.
func buildSandboxEnv(m *Manifest) []string {
	env := []string{
		"GLASSBOX_PLUGIN_NAME=" + m.Name,
		"GLASSBOX_PLUGIN_VERSION=" + m.Version,
		"GLASSBOX_API_VERSION=" + m.APIVersion,
	}
	// Forward PATH so the binary can locate shared libraries.
	if p := os.Getenv("PATH"); p != "" {
		env = append(env, "PATH="+p)
	}
	return env
}

// verifyChecksum computes the SHA-256 digest of the file at path and compares
// it against the expected hex string.
func verifyChecksum(path, expected string) error {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return fmt.Errorf("cannot open binary for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to hash binary: %w", err)
	}

	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return fmt.Errorf("expected %s, got %s", expected, got)
	}
	return nil
}

// isAbsPath reports whether p is an absolute filesystem path.
func isAbsPath(p string) bool {
	if len(p) == 0 {
		return false
	}
	// Unix absolute
	if p[0] == '/' {
		return true
	}
	// Windows absolute: C:\ or \\
	if len(p) >= 3 && p[1] == ':' {
		return true
	}
	if len(p) >= 2 && p[0] == '\\' && p[1] == '\\' {
		return true
	}
	return false
}

// joinPaths joins two path segments using the OS separator.
func joinPaths(base, rel string) string {
	if base == "" {
		return rel
	}
	sep := string(os.PathSeparator)
	if base[len(base)-1] == os.PathSeparator {
		return base + rel
	}
	return base + sep + rel
}
