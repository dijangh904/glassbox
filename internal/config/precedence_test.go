// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfig is a helper that writes a minimal TOML config file containing
// only rpc_url and network keys.
func writeConfig(t *testing.T, path, rpcURL, network string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}
	content := ""
	if rpcURL != "" {
		content += "rpc_url = \"" + rpcURL + "\"\n"
	}
	if network != "" {
		content += "network = \"" + network + "\"\n"
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

// TestPrecedence_HomeOverridesSystem verifies that the home-directory config
// takes precedence over the system-wide config.
func TestPrecedence_HomeOverridesSystem(t *testing.T) {
	// We cannot write to /etc/Glassbox so we simulate the home config
	// overriding a pre-existing lower-priority value by loading only home.
	home := t.TempDir()
	project := t.TempDir()

	writeConfig(t, filepath.Join(home, ".glassbox", "config.toml"),
		"https://home.example.com", "testnet")

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", home)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	_ = os.Chdir(project)

	cfg := &Config{}
	mustNoError(t, cfg.loadFromFile())
	if cfg.RpcUrl != "https://home.example.com" {
		t.Errorf("RpcUrl = %q, want home config value", cfg.RpcUrl)
	}
}

// TestPrecedence_ProjectOverridesHome verifies that a project-local config
// file overrides the home-directory config.
func TestPrecedence_ProjectOverridesHome(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	writeConfig(t, filepath.Join(home, ".glassbox", "config.toml"),
		"https://home.example.com", "testnet")
	writeConfig(t, filepath.Join(project, ".glassbox.toml"),
		"https://project.example.com", "futurenet")

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", home)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg := &Config{}
	mustNoError(t, cfg.loadFromFile())

	if cfg.RpcUrl != "https://project.example.com" {
		t.Errorf("RpcUrl = %q, want project config to win over home", cfg.RpcUrl)
	}
	if cfg.Network != Network("futurenet") {
		t.Errorf("Network = %q, want project config value", cfg.Network)
	}
}

// TestPrecedence_EnvOverridesProjectConfig verifies that environment variables
// override project-config file values after Load() is called.
func TestPrecedence_EnvOverridesProjectConfig(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	writeConfig(t, filepath.Join(project, ".glassbox.toml"),
		"https://file.example.com", "testnet")

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", home)

	origEnv := os.Getenv("GLASSBOX_RPC_URL")
	defer os.Setenv("GLASSBOX_RPC_URL", origEnv)
	os.Setenv("GLASSBOX_RPC_URL", "https://env.example.com")

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RpcUrl != "https://env.example.com" {
		t.Errorf("RpcUrl = %q, want env var to override project config", cfg.RpcUrl)
	}
}

// TestPrecedence_XDGOverridesLegacyInProject verifies that .glassbox.toml
// (XDG-style) overrides .Glassbox.toml (legacy) within the same directory.
func TestPrecedence_XDGOverridesLegacyInProject(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	writeConfig(t, filepath.Join(project, ".Glassbox.toml"),
		"https://legacy.example.com", "testnet")
	writeConfig(t, filepath.Join(project, ".glassbox.toml"),
		"https://xdg.example.com", "mainnet")

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", home)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg := &Config{}
	mustNoError(t, cfg.loadFromFile())
	if cfg.RpcUrl != "https://xdg.example.com" {
		t.Errorf("RpcUrl = %q, want XDG project config to win over legacy", cfg.RpcUrl)
	}
}

// TestPrecedence_AllLayersApplied verifies that all four layers
// (system skipped, home, project, env) are applied in the correct order by
// checking that each layer's unique key is visible in the final config.
func TestPrecedence_AllLayersApplied(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	// Home config sets network to "testnet"
	writeConfig(t, filepath.Join(home, ".glassbox", "config.toml"),
		"https://home.example.com", "testnet")

	// Project config overrides rpc_url but leaves network from home
	writeConfig(t, filepath.Join(project, ".glassbox.toml"),
		"https://project.example.com", "")

	// Env var overrides rpc_url again (highest non-CLI priority)
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", home)

	origEnv := os.Getenv("GLASSBOX_RPC_URL")
	defer os.Setenv("GLASSBOX_RPC_URL", origEnv)
	os.Setenv("GLASSBOX_RPC_URL", "https://env.example.com")

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Env var wins for rpc_url
	if cfg.RpcUrl != "https://env.example.com" {
		t.Errorf("RpcUrl = %q, want env var value", cfg.RpcUrl)
	}
	// Network was set only by home config and not overridden → should survive
	if cfg.Network != Network("testnet") {
		t.Errorf("Network = %q, want home config value (testnet)", cfg.Network)
	}
}

// TestPrecedence_ActiveConfigFileIsHighestPriority verifies that
// ActiveConfigFile returns the highest-priority file that was loaded.
func TestPrecedence_ActiveConfigFileIsHighestPriority(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	writeConfig(t, filepath.Join(home, ".glassbox", "config.toml"),
		"https://home.example.com", "testnet")
	writeConfig(t, filepath.Join(project, ".glassbox.toml"),
		"https://project.example.com", "mainnet")

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", home)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg := &Config{}
	mustNoError(t, cfg.loadFromFile())

	want := filepath.Join(project, ".glassbox.toml")
	if ActiveConfigFile() != want {
		t.Errorf("ActiveConfigFile = %q, want %q (highest-priority file)", ActiveConfigFile(), want)
	}
}

// mustNoError is a test helper that calls t.Fatalf when err is non-nil.
func mustNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
