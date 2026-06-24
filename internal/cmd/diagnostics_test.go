// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestPrintDiagnosticsDashboard_HealthyOutput verifies that the dashboard
// renders all expected section headings and key fields without crashing.
func TestPrintDiagnosticsDashboard_HealthyOutput(t *testing.T) {
	out := DiagnosticsOutput{
		Version:       "1.2.3",
		Timestamp:     time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		OverallHealth: "Healthy",
		System: SystemInfo{
			OS:      "linux",
			Arch:    "amd64",
			HomeDir: "/home/user",
		},
		RPC: []RPCStatus{
			{URL: "https://rpc.example.com", Status: "[OK]", Healthy: true, Latency: 42 * time.Millisecond},
		},
		Cache: CacheStatus{
			Directory: "/home/user/.glassbox/cache",
			Size:      "1.2 MB",
			MaxSize:   "500.0 MB",
			FileCount: 10,
			Healthy:   true,
		},
		Config: ConfigSummary{
			Source:  "/home/user/.glassbox.toml",
			Network: "mainnet",
			RPCURL:  "https://rpc.example.com",
			Healthy: true,
		},
		Plugins: []PluginSummary{},
	}

	var buf bytes.Buffer
	printDiagnosticsDashboard(&buf, out)
	rendered := buf.String()

	sections := []string{
		"GLASSBOX DIAGNOSTICS",
		"SYSTEM INFO",
		"RPC ENDPOINTS",
		"CACHE STATUS",
		"CONFIGURATION",
		"PLUGINS",
		"Version:     1.2.3",
		"OS:          linux",
		"Arch:        amd64",
		"Home:        /home/user",
		"https://rpc.example.com",
		"/home/user/.glassbox/cache",
		"mainnet",
		"Healthy",
	}
	for _, want := range sections {
		if !strings.Contains(rendered, want) {
			t.Errorf("printDiagnosticsDashboard output missing %q\n\nGot:\n%s", want, rendered)
		}
	}
}

// TestPrintDiagnosticsDashboard_DegradedRPC verifies that a failing RPC
// endpoint is rendered with [FAIL] and includes a fix hint.
func TestPrintDiagnosticsDashboard_DegradedRPC(t *testing.T) {
	out := DiagnosticsOutput{
		Version:       "1.0.0",
		Timestamp:     time.Now(),
		OverallHealth: "Degraded",
		System:        SystemInfo{OS: "darwin", Arch: "arm64"},
		RPC: []RPCStatus{
			{
				URL:     "https://bad-endpoint.example.com",
				Status:  "[FAIL]",
				Healthy: false,
				Error:   "connection refused",
				Latency: 0,
			},
		},
		Cache: CacheStatus{Healthy: true, Size: "0 B", MaxSize: "500.0 MB"},
	}

	var buf bytes.Buffer
	printDiagnosticsDashboard(&buf, out)
	rendered := buf.String()

	if !strings.Contains(rendered, "[FAIL]") {
		t.Error("expected [FAIL] tag for unreachable RPC endpoint")
	}
	if !strings.Contains(rendered, "connection refused") {
		t.Error("expected error message in RPC section")
	}
	// Fix hint must be present for actionable diagnostics.
	if !strings.Contains(rendered, "Fix:") {
		t.Error("expected actionable fix hint when RPC endpoint is unhealthy")
	}
}

// TestPrintDiagnosticsDashboard_NoRPC verifies that the no-endpoint message
// and configuration tip are printed when no RPC URLs are configured.
func TestPrintDiagnosticsDashboard_NoRPC(t *testing.T) {
	out := DiagnosticsOutput{
		Version:       "1.0.0",
		Timestamp:     time.Now(),
		OverallHealth: "Healthy",
		System:        SystemInfo{OS: "windows", Arch: "amd64"},
		RPC:           []RPCStatus{},
		Cache:         CacheStatus{Healthy: true, Size: "0 B", MaxSize: "500.0 MB"},
	}

	var buf bytes.Buffer
	printDiagnosticsDashboard(&buf, out)
	rendered := buf.String()

	if !strings.Contains(rendered, "No RPC endpoints configured") {
		t.Error("expected no-endpoints message")
	}
	// Must suggest a fix.
	if !strings.Contains(rendered, "GLASSBOX_RPC_URL") {
		t.Error("expected GLASSBOX_RPC_URL environment variable hint")
	}
}

// TestPrintDiagnosticsDashboard_CacheOverLimit verifies the over-limit
// message and remediation hint are rendered when the cache exceeds its cap.
func TestPrintDiagnosticsDashboard_CacheOverLimit(t *testing.T) {
	out := DiagnosticsOutput{
		Version:       "1.0.0",
		Timestamp:     time.Now(),
		OverallHealth: "Degraded",
		System:        SystemInfo{OS: "linux", Arch: "amd64"},
		RPC:           []RPCStatus{},
		Cache: CacheStatus{
			Directory: "/tmp/cache",
			Size:      "600.0 MB",
			MaxSize:   "500.0 MB",
			FileCount: 1234,
			Healthy:   false,
		},
	}

	var buf bytes.Buffer
	printDiagnosticsDashboard(&buf, out)
	rendered := buf.String()

	if !strings.Contains(rendered, "Over limit") {
		t.Error("expected over-limit cache status")
	}
	if !strings.Contains(rendered, "cache clear") {
		t.Error("expected cache clear remediation hint")
	}
}

// TestPrintDiagnosticsDashboard_ConfigUnhealthy verifies that a degraded
// config state is surfaced with a descriptive message.
func TestPrintDiagnosticsDashboard_ConfigUnhealthy(t *testing.T) {
	out := DiagnosticsOutput{
		Version:       "1.0.0",
		Timestamp:     time.Now(),
		OverallHealth: "Degraded",
		System:        SystemInfo{OS: "linux", Arch: "amd64"},
		RPC:           []RPCStatus{},
		Cache:         CacheStatus{Healthy: true, Size: "0 B", MaxSize: "500.0 MB"},
		Config:        ConfigSummary{Healthy: false},
	}

	var buf bytes.Buffer
	printDiagnosticsDashboard(&buf, out)
	rendered := buf.String()

	if !strings.Contains(rendered, "No config file found") {
		t.Error("expected no-config-file message when Config.Healthy is false")
	}
	if !strings.Contains(rendered, "defaults will be used") {
		t.Error("expected defaults-in-use hint when Config.Healthy is false")
	}
}

// TestPrintDiagnosticsDashboard_Plugins verifies that plugin details are
// rendered when plugins are present.
func TestPrintDiagnosticsDashboard_Plugins(t *testing.T) {
	out := DiagnosticsOutput{
		Version:       "1.0.0",
		Timestamp:     time.Now(),
		OverallHealth: "Healthy",
		System:        SystemInfo{OS: "linux", Arch: "amd64"},
		RPC:           []RPCStatus{},
		Cache:         CacheStatus{Healthy: true, Size: "0 B", MaxSize: "500.0 MB"},
		Plugins: []PluginSummary{
			{Name: "my-plugin", Version: "2.0.0", Healthy: true},
		},
	}

	var buf bytes.Buffer
	printDiagnosticsDashboard(&buf, out)
	rendered := buf.String()

	if !strings.Contains(rendered, "my-plugin") {
		t.Error("expected plugin name in output")
	}
	if !strings.Contains(rendered, "v2.0.0") {
		t.Error("expected plugin version in output")
	}
}

// TestPrintDiagnosticsDashboard_NoPlugins verifies the no-plugins message.
func TestPrintDiagnosticsDashboard_NoPlugins(t *testing.T) {
	out := DiagnosticsOutput{
		Version:       "1.0.0",
		Timestamp:     time.Now(),
		OverallHealth: "Healthy",
		System:        SystemInfo{OS: "linux", Arch: "amd64"},
		RPC:           []RPCStatus{},
		Cache:         CacheStatus{Healthy: true, Size: "0 B", MaxSize: "500.0 MB"},
		Plugins:       []PluginSummary{},
	}

	var buf bytes.Buffer
	printDiagnosticsDashboard(&buf, out)
	rendered := buf.String()

	if !strings.Contains(rendered, "No plugins discovered") {
		t.Error("expected no-plugins message")
	}
}

// TestDiagnosticsOutput_JSONRoundtrip verifies that DiagnosticsOutput
// marshals and unmarshals cleanly, covering the --json output path.
func TestDiagnosticsOutput_JSONRoundtrip(t *testing.T) {
	original := DiagnosticsOutput{
		Version:       "1.0.0",
		Timestamp:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		OverallHealth: "Healthy",
		System:        SystemInfo{OS: "linux", Arch: "amd64", HomeDir: "/home/user"},
		RPC: []RPCStatus{
			{URL: "https://rpc.example.com", Status: "[OK]", Healthy: true, Latency: 10 * time.Millisecond},
		},
		Cache:   CacheStatus{Directory: "/tmp", Size: "1 MB", MaxSize: "500 MB", FileCount: 5, Healthy: true},
		Config:  ConfigSummary{Source: "/etc/glassbox.toml", Network: "testnet", RPCURL: "https://rpc.testnet.example.com", Healthy: true},
		Plugins: []PluginSummary{{Name: "plugin-a", Version: "1.0.0", Healthy: true}},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded DiagnosticsOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.Version != original.Version {
		t.Errorf("Version: got %q, want %q", decoded.Version, original.Version)
	}
	if decoded.OverallHealth != original.OverallHealth {
		t.Errorf("OverallHealth: got %q, want %q", decoded.OverallHealth, original.OverallHealth)
	}
	if len(decoded.RPC) != len(original.RPC) {
		t.Errorf("RPC length: got %d, want %d", len(decoded.RPC), len(original.RPC))
	}
	if decoded.System.OS != original.System.OS {
		t.Errorf("System.OS: got %q, want %q", decoded.System.OS, original.System.OS)
	}
}

// TestDiagnosticsCmd_FlagRegistered verifies that the --json flag is
// properly registered on the diagnostics command.
func TestDiagnosticsCmd_FlagRegistered(t *testing.T) {
	flag := diagnosticsCmd.Flags().Lookup("json")
	if flag == nil {
		t.Fatal("--json flag must be registered on the diagnostics command")
	}
	if flag.DefValue != "false" {
		t.Errorf("--json default should be false, got %q", flag.DefValue)
	}
}
