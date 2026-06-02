// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPassphrase = "test-passphrase-123"
const testPlaintext = `rpc_url = "https://soroban-testnet.stellar.org"
network = "testnet"
log_level = "debug"
`

func TestEncryptDecryptRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	encrypted, err := EncryptConfig([]byte(testPlaintext), testPassphrase, path)
	if err != nil {
		t.Fatalf("EncryptConfig failed: %v", err)
	}

	if !strings.HasPrefix(string(encrypted), encryptedConfigMagic) {
		t.Errorf("encrypted output missing magic prefix")
	}

	// Write to file so LoadEncryptedConfig can read it.
	if err := os.WriteFile(path, encrypted, 0600); err != nil {
		t.Fatal(err)
	}

	plaintext, err := LoadEncryptedConfig(path, testPassphrase, nil)
	if err != nil {
		t.Fatalf("LoadEncryptedConfig failed: %v", err)
	}

	if string(plaintext) != testPlaintext {
		t.Errorf("round-trip mismatch:\ngot:  %q\nwant: %q", plaintext, testPlaintext)
	}
}

func TestLoadEncryptedConfig_PlainFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(testPlaintext), 0600); err != nil {
		t.Fatal(err)
	}

	data, err := LoadEncryptedConfig(path, "", nil)
	if err != nil {
		t.Fatalf("unexpected error for plain file: %v", err)
	}
	if string(data) != testPlaintext {
		t.Errorf("plain file content mismatch")
	}
}

func TestLoadEncryptedConfig_WrongPassphrase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	encrypted, err := EncryptConfig([]byte(testPlaintext), testPassphrase, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encrypted, 0600); err != nil {
		t.Fatal(err)
	}

	_, err = LoadEncryptedConfig(path, "wrong-passphrase", nil)
	if err == nil {
		t.Error("expected error for wrong passphrase")
	}
}

func TestLoadEncryptedConfig_MissingPassphrase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	encrypted, err := EncryptConfig([]byte(testPlaintext), testPassphrase, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encrypted, 0600); err != nil {
		t.Fatal(err)
	}

	_, err = LoadEncryptedConfig(path, "", nil)
	if err == nil {
		t.Error("expected error when passphrase is empty for encrypted file")
	}
}

func TestLoadEncryptedConfig_CustomHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	// Write a file with the magic prefix so the hook is invoked.
	content := encryptedConfigMagic + "aGVsbG8=" // base64("hello")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	hookCalled := false
	hook := func(ciphertext []byte) ([]byte, error) {
		hookCalled = true
		return []byte("decrypted-by-hook"), nil
	}

	result, err := LoadEncryptedConfig(path, "", hook)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hookCalled {
		t.Error("custom hook was not called")
	}
	if string(result) != "decrypted-by-hook" {
		t.Errorf("hook result mismatch: %q", result)
	}
}

func TestLoadEncryptedConfig_MissingFile(t *testing.T) {
	_, err := LoadEncryptedConfig("/nonexistent/config.toml", testPassphrase, nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestIsEncryptedConfig_Encrypted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	encrypted, err := EncryptConfig([]byte(testPlaintext), testPassphrase, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encrypted, 0600); err != nil {
		t.Fatal(err)
	}

	ok, err := IsEncryptedConfig(path)
	if err != nil {
		t.Fatalf("IsEncryptedConfig error: %v", err)
	}
	if !ok {
		t.Error("expected IsEncryptedConfig to return true for encrypted file")
	}
}

func TestIsEncryptedConfig_Plain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(testPlaintext), 0600); err != nil {
		t.Fatal(err)
	}

	ok, err := IsEncryptedConfig(path)
	if err != nil {
		t.Fatalf("IsEncryptedConfig error: %v", err)
	}
	if ok {
		t.Error("expected IsEncryptedConfig to return false for plain file")
	}
}

func TestEncryptConfig_Deterministic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	enc1, err := EncryptConfig([]byte(testPlaintext), testPassphrase, path)
	if err != nil {
		t.Fatal(err)
	}
	enc2, err := EncryptConfig([]byte(testPlaintext), testPassphrase, path)
	if err != nil {
		t.Fatal(err)
	}
	if string(enc1) != string(enc2) {
		t.Error("EncryptConfig should be deterministic for same inputs")
	}
}
