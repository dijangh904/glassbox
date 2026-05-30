// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package signer

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

// ---- helpers ----------------------------------------------------------------

func newTestPEM(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("key generation failed: %v", err)
	}
	raw, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal PKCS#8 failed: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}))
}

// ---- Registry ---------------------------------------------------------------

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&SoftwareProvider{})

	p, err := r.Get("software")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if p.Name() != "software" {
		t.Fatalf("unexpected name: %s", p.Name())
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestRegistry_RegisterDuplicate_Panics(t *testing.T) {
	r := NewRegistry()
	r.Register(&SoftwareProvider{})

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r.Register(&SoftwareProvider{})
}

func TestRegistry_RegisterOrReplace(t *testing.T) {
	r := NewRegistry()
	r.Register(&SoftwareProvider{})
	// Should not panic
	r.RegisterOrReplace(&SoftwareProvider{})

	p, err := r.Get("software")
	if err != nil {
		t.Fatalf("Get after replace failed: %v", err)
	}
	if p.Name() != "software" {
		t.Fatalf("unexpected name: %s", p.Name())
	}
}

func TestRegistry_Names(t *testing.T) {
	r := NewRegistry()
	r.Register(&SoftwareProvider{})
	r.Register(&PKCS11Provider{})

	names := r.Names()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d: %v", len(names), names)
	}
	// Names must be sorted
	if names[0] != "pkcs11" || names[1] != "software" {
		t.Fatalf("unexpected names order: %v", names)
	}
}

func TestRegistry_CreateSigner_Software(t *testing.T) {
	r := NewRegistry()
	r.Register(&SoftwareProvider{})

	pemKey := newTestPEM(t)
	s, err := r.CreateSigner("software", ProviderConfig{SoftwareKeyPEM: pemKey})
	if err != nil {
		t.Fatalf("CreateSigner failed: %v", err)
	}

	data := []byte("test payload")
	sig, err := s.Sign(data)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	pub, err := s.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey failed: %v", err)
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), data, sig) {
		t.Fatal("signature verification failed")
	}
}

func TestRegistry_CreateSigner_InvalidProvider(t *testing.T) {
	r := NewRegistry()
	_, err := r.CreateSigner("aws-kms", ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for unregistered provider")
	}
}

func TestRegistry_CreateSigner_InvalidConfig(t *testing.T) {
	r := NewRegistry()
	r.Register(&SoftwareProvider{})

	// No key provided — should fail validation
	_, err := r.CreateSigner("software", ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

// ---- DefaultRegistry --------------------------------------------------------

func TestDefaultRegistry_HasBuiltins(t *testing.T) {
	names := DefaultRegistry.Names()
	has := func(name string) bool {
		for _, n := range names {
			if n == name {
				return true
			}
		}
		return false
	}
	if !has("software") {
		t.Error("DefaultRegistry missing 'software' provider")
	}
	if !has("pkcs11") {
		t.Error("DefaultRegistry missing 'pkcs11' provider")
	}
}

// ---- SoftwareProvider -------------------------------------------------------

func TestSoftwareProvider_Name(t *testing.T) {
	p := &SoftwareProvider{}
	if p.Name() != "software" {
		t.Fatalf("unexpected name: %s", p.Name())
	}
}

func TestSoftwareProvider_Validate_WithPEM(t *testing.T) {
	p := &SoftwareProvider{}
	pemKey := newTestPEM(t)
	if err := p.Validate(ProviderConfig{SoftwareKeyPEM: pemKey}); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
}

func TestSoftwareProvider_Validate_WithEnvPEM(t *testing.T) {
	p := &SoftwareProvider{}
	pemKey := newTestPEM(t)
	t.Setenv("GLASSBOX_AUDIT_PRIVATE_KEY_PEM", pemKey)
	if err := p.Validate(ProviderConfig{}); err != nil {
		t.Fatalf("Validate with env PEM failed: %v", err)
	}
}

func TestSoftwareProvider_Validate_NoKey(t *testing.T) {
	p := &SoftwareProvider{}
	t.Setenv("GLASSBOX_AUDIT_PRIVATE_KEY_PEM", "")
	t.Setenv("GLASSBOX_SOFTWARE_PRIVATE_KEY_HEX", "")
	if err := p.Validate(ProviderConfig{}); err == nil {
		t.Fatal("expected error when no key is provided")
	}
}

func TestSoftwareProvider_Create_PEM(t *testing.T) {
	p := &SoftwareProvider{}
	pemKey := newTestPEM(t)
	s, err := p.Create(ProviderConfig{SoftwareKeyPEM: pemKey})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if s.Algorithm() != "ed25519" {
		t.Fatalf("unexpected algorithm: %s", s.Algorithm())
	}
}

func TestSoftwareProvider_EnvVars(t *testing.T) {
	p := &SoftwareProvider{}
	docs := p.EnvVars()
	if len(docs) == 0 {
		t.Fatal("EnvVars must not be empty")
	}
	for _, d := range docs {
		if d.Name == "" || d.Description == "" {
			t.Fatalf("incomplete EnvVarDoc: %+v", d)
		}
	}
}

// ---- PKCS11Provider ---------------------------------------------------------

func TestPKCS11Provider_Name(t *testing.T) {
	p := &PKCS11Provider{}
	if p.Name() != "pkcs11" {
		t.Fatalf("unexpected name: %s", p.Name())
	}
}

func TestPKCS11Provider_Validate_MissingModule(t *testing.T) {
	p := &PKCS11Provider{}
	t.Setenv("GLASSBOX_PKCS11_MODULE", "")
	t.Setenv("GLASSBOX_PKCS11_PIN", "1234")
	if err := p.Validate(ProviderConfig{}); err == nil {
		t.Fatal("expected error when module is missing")
	}
}

func TestPKCS11Provider_Validate_MissingPIN(t *testing.T) {
	p := &PKCS11Provider{}
	// Use a path that exists on all platforms (the test binary itself)
	t.Setenv("GLASSBOX_PKCS11_MODULE", "")
	t.Setenv("GLASSBOX_PKCS11_PIN", "")
	if err := p.Validate(ProviderConfig{PKCS11ModulePath: "/nonexistent/module.so"}); err == nil {
		t.Fatal("expected error for non-existent module path")
	}
}

func TestPKCS11Provider_Validate_ModuleNotFound(t *testing.T) {
	p := &PKCS11Provider{}
	err := p.Validate(ProviderConfig{
		PKCS11ModulePath: "/nonexistent/path/libpkcs11.so",
		PKCS11PIN:        "1234",
	})
	if err == nil {
		t.Fatal("expected error when module file does not exist")
	}
}

func TestPKCS11Provider_EnvVars(t *testing.T) {
	p := &PKCS11Provider{}
	docs := p.EnvVars()
	if len(docs) == 0 {
		t.Fatal("EnvVars must not be empty")
	}
	// Verify required fields are documented
	requiredCount := 0
	for _, d := range docs {
		if d.Required {
			requiredCount++
		}
		if d.Name == "" || d.Description == "" {
			t.Fatalf("incomplete EnvVarDoc: %+v", d)
		}
	}
	if requiredCount == 0 {
		t.Fatal("PKCS11Provider must document at least one required env var")
	}
}

// ---- DiscoverPKCS11Modules --------------------------------------------------

func TestDiscoverPKCS11Modules_ReturnsSlice(t *testing.T) {
	// Just verify it doesn't panic and returns a slice (may be empty on CI).
	modules := DiscoverPKCS11Modules()
	_ = modules // may be nil on systems without PKCS#11 libraries
}

// ---- Interface compliance ---------------------------------------------------

func TestSignerProviderInterface_Software(t *testing.T) {
	var _ SignerProvider = &SoftwareProvider{}
}

func TestSignerProviderInterface_PKCS11(t *testing.T) {
	var _ SignerProvider = &PKCS11Provider{}
}
