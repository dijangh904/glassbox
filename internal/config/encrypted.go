// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// encryptedConfigMagic is the prefix that identifies an encrypted config file.
const encryptedConfigMagic = "GLASSBOX-ENC-V1:"

// DecryptHook is a function that receives ciphertext bytes and returns
// plaintext bytes. Callers can supply a custom hook to integrate with
// external key management systems.
type DecryptHook func(ciphertext []byte) ([]byte, error)

// LoadEncryptedConfig loads a config file, decrypting it if it is encrypted.
//
// Detection: if the file content starts with the magic prefix
// "GLASSBOX-ENC-V1:", the remainder is treated as base64-encoded ciphertext
// and decrypted using AES-256-GCM.
//
// Key derivation: HKDF-SHA256(passphrase, salt=sha256(path), info="glassbox-config").
//
// If hook is non-nil it is called instead of the built-in AES-GCM decryptor,
// allowing callers to integrate with HSMs or KMS services.
//
// If the file is not encrypted (no magic prefix) it is returned as-is.
func LoadEncryptedConfig(path, passphrase string, hook DecryptHook) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, encryptedConfigMagic) {
		return data, nil
	}

	encoded := strings.TrimPrefix(content, encryptedConfigMagic)
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("encrypted config %q: invalid base64 encoding: %w", path, err)
	}

	if hook != nil {
		return hook(ciphertext)
	}

	if passphrase == "" {
		return nil, fmt.Errorf("encrypted config %q requires a passphrase (--config-passphrase or GLASSBOX_CONFIG_PASSPHRASE)", path)
	}

	return decryptAESGCM(ciphertext, passphrase, path)
}

// deriveKey derives a 32-byte AES key from passphrase using HKDF-SHA256.
// salt = SHA256(path), info = "glassbox-config".
func deriveKey(passphrase, path string) []byte {
	salt := sha256.Sum256([]byte(path))
	info := []byte("glassbox-config")

	// HKDF extract: prk = HMAC-SHA256(salt, passphrase)
	mac := hmac.New(sha256.New, salt[:])
	mac.Write([]byte(passphrase))
	prk := mac.Sum(nil)

	// HKDF expand: okm = HMAC-SHA256(prk, info || 0x01)
	mac2 := hmac.New(sha256.New, prk)
	mac2.Write(info)
	mac2.Write([]byte{0x01})
	return mac2.Sum(nil) // 32 bytes
}

// decryptAESGCM decrypts ciphertext using AES-256-GCM.
// Ciphertext layout: [12-byte nonce][GCM ciphertext+tag].
func decryptAESGCM(ciphertext []byte, passphrase, path string) ([]byte, error) {
	const nonceSize = 12
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("encrypted config: ciphertext too short")
	}

	key := deriveKey(passphrase, path)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("encrypted config: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encrypted config: failed to create GCM: %w", err)
	}

	nonce := ciphertext[:nonceSize]
	ct := ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypted config %q: decryption failed (wrong passphrase?): %w", path, err)
	}

	return plaintext, nil
}

// EncryptConfig encrypts plaintext config bytes using AES-256-GCM and returns
// the encoded file content (magic prefix + base64 ciphertext).
// This is the inverse of LoadEncryptedConfig.
func EncryptConfig(plaintext []byte, passphrase, path string) ([]byte, error) {
	const nonceSize = 12

	key := deriveKey(passphrase, path)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("encrypt config: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encrypt config: failed to create GCM: %w", err)
	}

	// Deterministic nonce: SHA256(key || plaintext)[:12] — safe for single-use per key+plaintext pair.
	nonceHash := sha256.Sum256(append(key, plaintext...))
	nonce := nonceHash[:nonceSize]

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return []byte(encryptedConfigMagic + encoded), nil
}

// IsEncryptedConfig reports whether the file at path appears to be an
// encrypted Glassbox config file.
func IsEncryptedConfig(path string) (bool, error) {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, len(encryptedConfigMagic))
	n, _ := f.Read(buf)
	return string(buf[:n]) == encryptedConfigMagic, nil
}
