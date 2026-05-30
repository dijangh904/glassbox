// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package signer

import (
	"os"
	"strings"
)

// SoftwareProvider implements SignerProvider for in-process Ed25519 signing
// using a PEM-encoded PKCS#8 private key or a hex-encoded seed/full key.
//
// Configuration priority (highest to lowest):
//  1. ProviderConfig.SoftwareKeyPEM (literal PEM or file path)
//  2. ProviderConfig.SoftwareKeyHex
//  3. GLASSBOX_AUDIT_PRIVATE_KEY_PEM environment variable
//  4. GLASSBOX_SOFTWARE_PRIVATE_KEY_HEX environment variable
type SoftwareProvider struct{}

// Name returns "software".
func (p *SoftwareProvider) Name() string { return "software" }

// Description returns a short human-readable description.
func (p *SoftwareProvider) Description() string {
	return "Ed25519 software signing using a PEM or hex-encoded private key"
}

// EnvVars documents the environment variables recognised by this provider.
func (p *SoftwareProvider) EnvVars() []EnvVarDoc {
	return []EnvVarDoc{
		{
			Name:        "GLASSBOX_AUDIT_PRIVATE_KEY_PEM",
			Required:    false,
			Description: "PKCS#8 PEM-encoded Ed25519 private key (literal PEM text or file path)",
		},
		{
			Name:        "GLASSBOX_SOFTWARE_PRIVATE_KEY_HEX",
			Required:    false,
			Description: "Hex-encoded Ed25519 private key seed (32 bytes) or full key (64 bytes)",
		},
	}
}

// Validate checks that at least one key source is available.
func (p *SoftwareProvider) Validate(cfg ProviderConfig) error {
	if p.resolveKeyPEM(cfg) != "" || p.resolveKeyHex(cfg) != "" {
		return nil
	}
	return &Error{
		Op:  "software",
		Msg: "no private key provided; set --software-private-key, GLASSBOX_AUDIT_PRIVATE_KEY_PEM, or GLASSBOX_SOFTWARE_PRIVATE_KEY_HEX",
	}
}

// Create returns an InMemorySigner loaded from the first available key source.
func (p *SoftwareProvider) Create(cfg ProviderConfig) (Signer, error) {
	if pem := p.resolveKeyPEM(cfg); pem != "" {
		return NewInMemorySignerFromPEM(pem)
	}
	if hex := p.resolveKeyHex(cfg); hex != "" {
		return NewInMemorySigner(hex)
	}
	return nil, &Error{Op: "software", Msg: "no private key available after validation"}
}

// resolveKeyPEM returns the PEM key text from cfg or environment, reading
// from disk if the value looks like a file path rather than PEM text.
func (p *SoftwareProvider) resolveKeyPEM(cfg ProviderConfig) string {
	raw := cfg.SoftwareKeyPEM
	if raw == "" {
		raw = os.Getenv("GLASSBOX_AUDIT_PRIVATE_KEY_PEM")
	}
	if raw == "" {
		return ""
	}
	// If the value doesn't look like PEM, treat it as a file path.
	if !strings.Contains(raw, "-----BEGIN") {
		if data, err := os.ReadFile(raw); err == nil {
			return string(data)
		}
	}
	return raw
}

// resolveKeyHex returns the hex key from cfg or environment.
func (p *SoftwareProvider) resolveKeyHex(cfg ProviderConfig) string {
	if cfg.SoftwareKeyHex != "" {
		return cfg.SoftwareKeyHex
	}
	return os.Getenv("GLASSBOX_SOFTWARE_PRIVATE_KEY_HEX")
}
