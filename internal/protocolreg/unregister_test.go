// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package protocolreg

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// newUnregisterTestRegistrar creates a Registrar that writes into a temp
// directory so tests do not touch the real user home directory.
func newUnregisterTestRegistrar(t *testing.T) *Registrar {
	t.Helper()
	dir := t.TempDir()
	return &Registrar{
		executablePath: "/usr/local/bin/glassbox",
		homeDir:        dir,
	}
}

// TestUnregister_NotRegistered verifies that Unregister succeeds gracefully
// when no handler files are present (partial or missing registration state).
func TestUnregister_NotRegistered(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux/macOS file-based unregister test; Windows uses registry")
	}

	r := newUnregisterTestRegistrar(t)
	// Nothing has been registered — Unregister must not return an error.
	if err := r.Unregister(); err != nil {
		t.Errorf("Unregister on empty state should succeed, got: %v", err)
	}
}

// TestUnregister_Linux_RemovesFiles verifies that the Linux unregister path
// removes the desktop file and wrapper script when they exist.
func TestUnregister_Linux_RemovesFiles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific unregister test")
	}

	r := newUnregisterTestRegistrar(t)

	// Create the files that registerLinux would have written.
	desktopDir := filepath.Dir(r.linuxDesktopPath())
	wrapperDir := filepath.Dir(r.linuxWrapperPath())
	if err := os.MkdirAll(desktopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.linuxDesktopPath(), []byte("[Desktop Entry]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.linuxWrapperPath(), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := r.Unregister(); err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if _, err := os.Stat(r.linuxDesktopPath()); !os.IsNotExist(err) {
		t.Error("desktop file should have been removed")
	}
	if _, err := os.Stat(r.linuxWrapperPath()); !os.IsNotExist(err) {
		t.Error("wrapper script should have been removed")
	}
}

// TestUnregister_Linux_PartialState verifies that Unregister succeeds when
// only one of the two files exists (partial registration).
func TestUnregister_Linux_PartialState(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific unregister test")
	}

	r := newUnregisterTestRegistrar(t)

	// Only create the desktop file; leave the wrapper absent.
	desktopDir := filepath.Dir(r.linuxDesktopPath())
	if err := os.MkdirAll(desktopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.linuxDesktopPath(), []byte("[Desktop Entry]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := r.Unregister(); err != nil {
		t.Fatalf("Unregister with partial state should succeed, got: %v", err)
	}

	if _, err := os.Stat(r.linuxDesktopPath()); !os.IsNotExist(err) {
		t.Error("desktop file should have been removed")
	}
}

// TestUnregister_Darwin_NotRegistered verifies that macOS unregister is a
// no-op when the app bundle does not exist.
func TestUnregister_Darwin_NotRegistered(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific unregister test")
	}

	r := newUnregisterTestRegistrar(t)
	if err := r.Unregister(); err != nil {
		t.Errorf("Unregister on empty state should succeed on macOS, got: %v", err)
	}
}

// TestUnregister_IdempotentOnLinux verifies that calling Unregister twice in a
// row does not return an error (idempotent removal).
func TestUnregister_IdempotentOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific idempotency test")
	}

	r := newUnregisterTestRegistrar(t)

	// First call: nothing registered — must succeed.
	if err := r.Unregister(); err != nil {
		t.Fatalf("first Unregister failed: %v", err)
	}
	// Second call: still nothing — must also succeed.
	if err := r.Unregister(); err != nil {
		t.Fatalf("second Unregister failed: %v", err)
	}
}
