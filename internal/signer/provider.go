// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

// Package signer provides a generic signing abstraction with a pluggable
// provider registry. New backends (AWS KMS, SSH agent, etc.) can be added
// by implementing SignerProvider and calling Registry.Register.
package signer

// ProviderConfig carries the resolved configuration for a signing provider.
// Flags and environment variables are both funnelled through this struct so
// that provider implementations remain decoupled from the CLI layer.
type ProviderConfig struct {
	// SoftwareKeyPEM is a PKCS#8 PEM-encoded Ed25519 private key used by the
	// "software" provider. May be the literal PEM text or a file path.
	SoftwareKeyPEM string

	// SoftwareKeyHex is a hex-encoded Ed25519 private key (seed or full key)
	// used by the "software" provider as a fallback to SoftwareKeyPEM.
	SoftwareKeyHex string

	// PKCS11ModulePath is the filesystem path to the PKCS#11 shared library.
	// Required for the "pkcs11" provider.
	PKCS11ModulePath string

	// PKCS11PIN is the user PIN for the PKCS#11 token.
	PKCS11PIN string

	// PKCS11TokenLabel selects a token by label.
	PKCS11TokenLabel string

	// PKCS11KeyLabel selects a private key by its CKA_LABEL attribute.
	PKCS11KeyLabel string

	// PKCS11KeyIDHex selects a private key by its CKA_ID attribute (hex).
	PKCS11KeyIDHex string

	// PKCS11SlotIndex selects a slot by numeric index when TokenLabel is empty.
	PKCS11SlotIndex int

	// Extra holds provider-specific key/value pairs for future backends
	// (e.g. AWS KMS key ARN, SSH agent socket path).
	Extra map[string]string
}

// SignerProvider is the extension point for signing backends. Implement this
// interface and register it with DefaultRegistry to add a new provider.
//
// Lifecycle:
//
//	Validate(cfg) → Create(cfg) → use Signer → Close (if Closer)
type SignerProvider interface {
	// Name returns the canonical provider identifier used in --signing-provider
	// and GLASSBOX_SIGNING_PROVIDER. Examples: "software", "pkcs11".
	Name() string

	// Description returns a short human-readable description shown in help text.
	Description() string

	// Validate checks that cfg contains all required fields for this provider
	// and returns a descriptive error if anything is missing or invalid.
	Validate(cfg ProviderConfig) error

	// Create instantiates a Signer from the validated configuration.
	// The caller is responsible for calling Close() on the returned Signer
	// if it implements io.Closer.
	Create(cfg ProviderConfig) (Signer, error)

	// EnvVars returns the environment variable names that configure this
	// provider, used to generate documentation and validation hints.
	EnvVars() []EnvVarDoc
}

// EnvVarDoc documents a single environment variable for a provider.
type EnvVarDoc struct {
	// Name is the environment variable name (e.g. "GLASSBOX_PKCS11_MODULE").
	Name string
	// Required indicates whether the variable must be set.
	Required bool
	// Description is a short human-readable description.
	Description string
}
