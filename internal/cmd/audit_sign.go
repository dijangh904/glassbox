// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dotandev/glassbox/internal/errors"
	"github.com/dotandev/glassbox/internal/signer"
	"github.com/spf13/cobra"
)

// auditSign flag variables — package-level so tests can set them directly.
var (
	auditSignPayload     string
	auditSignPayloadFile string
	auditSignValidateOnly bool

	// auditSignSoftwareKey accepts a PKCS#8 PEM Ed25519 private key (literal
	// PEM text or a file path). Equivalent to GLASSBOX_AUDIT_PRIVATE_KEY_PEM.
	auditSignSoftwareKey string

	// auditSignHSMProvider is kept for backward compatibility. When set to
	// "pkcs11" it is equivalent to --signing-provider pkcs11.
	// Deprecated: prefer --signing-provider.
	auditSignHSMProvider string

	// auditSignProvider selects the signing backend by name.
	// Supported values: "software" (default), "pkcs11".
	// Additional providers can be registered via signer.DefaultRegistry.
	auditSignProvider string

	// PKCS#11 flag overrides — these take precedence over the corresponding
	// GLASSBOX_PKCS11_* environment variables.
	auditSignPKCS11Module     string
	auditSignPKCS11PIN        string
	auditSignPKCS11TokenLabel string
	auditSignPKCS11KeyLabel   string
	auditSignPKCS11KeyIDHex   string

	// Provenance flags — optional metadata attached to the signed audit log.
	auditSignSignerIdentity       string
	auditSignKeyID                string
	auditSignCertChainFile        string
	auditSignPreviousSignatureHash string
)

// SignatureProvenance carries metadata about the signing key and identity,
// enabling independent verification of who signed an audit log and with what key.
type SignatureProvenance struct {
	// SignerIdentity is a human-readable identifier for the signing entity
	// (e.g. "ops-team@example.com", "ci-pipeline", "hsm-slot-0").
	SignerIdentity string `json:"signer_identity,omitempty"`

	// KeyID is an opaque identifier for the specific key used to sign
	// (e.g. PKCS#11 CKA_LABEL, KMS key ARN, or a fingerprint).
	KeyID string `json:"key_id,omitempty"`

	// Algorithm describes the signing algorithm (e.g. "Ed25519", "ECDSA-P256").
	Algorithm string `json:"algorithm,omitempty"`

	// CertificateChain holds PEM-encoded certificates in order from leaf to
	// root, enabling chain-of-trust verification when an HSM or PKI is used.
	// May be empty for software/bare-key signing.
	CertificateChain []string `json:"certificate_chain,omitempty"`

	// PreviousSignatureHash is the hex-encoded SHA-256 of the immediately
	// preceding signed audit log, forming a tamper-evident verification chain.
	// Empty for the first entry in a chain.
	PreviousSignatureHash string `json:"previous_signature_hash,omitempty"`
}

// SignedAuditLog is the JSON output produced by audit:sign.
type SignedAuditLog struct {
	Version    string               `json:"version"`
	Timestamp  time.Time            `json:"timestamp"`
	TraceHash  string               `json:"trace_hash"`
	Signature  string               `json:"signature"`
	PublicKey  string               `json:"public_key"`
	Provider   string               `json:"provider"`
	Provenance *SignatureProvenance  `json:"provenance,omitempty"`
	Payload    json.RawMessage      `json:"payload"`
}

var auditSignCmd = &cobra.Command{
	Use:     "audit:sign",
	GroupID: "utility",
	Short:   "Generate a deterministic signed audit log from a JSON payload",
	Long: `Generate a deterministic signed audit log from a JSON payload.

The payload can be supplied as a string via --payload, as a file via
--payload-file, or piped on stdin.

SIGNING PROVIDERS
  software  (default) — Ed25519 in-process signing using a PEM or hex key.
            Required: --software-private-key or GLASSBOX_AUDIT_PRIVATE_KEY_PEM
                      or GLASSBOX_SOFTWARE_PRIVATE_KEY_HEX

  pkcs11    — Hardware security module signing via PKCS#11 Cryptoki.
            Required: GLASSBOX_PKCS11_MODULE, GLASSBOX_PKCS11_PIN
            Optional: GLASSBOX_PKCS11_TOKEN_LABEL, GLASSBOX_PKCS11_KEY_LABEL,
                      GLASSBOX_PKCS11_KEY_ID

PROVIDER SELECTION ORDER
  1. --signing-provider flag
  2. --hsm-provider flag (deprecated alias; "pkcs11" maps to the pkcs11 provider)
  3. GLASSBOX_SIGNING_PROVIDER environment variable
  4. GLASSBOX_SIGNER_TYPE environment variable (legacy)
  5. "software" (default)

EXAMPLES
  # Software signing with an inline PEM key
  glassbox audit:sign \
    --payload '{"input":{},"state":{},"events":[],"timestamp":"2026-01-01T00:00:00.000Z"}' \
    --software-private-key "$(cat ./ed25519-private-key.pem)"

  # Software signing with a key file
  glassbox audit:sign --payload-file payload.json \
    --software-private-key ./ed25519-private-key.pem

  # PKCS#11 HSM signing
  glassbox audit:sign --payload-file payload.json \
    --signing-provider pkcs11 \
    --pkcs11-module /usr/lib/softhsm/libsofthsm2.so \
    --pkcs11-pin 1234`,
	Args: cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return validateAuditSignArgs(auditSignPayload, auditSignPayloadFile, auditSignProvider)
	},
	RunE: runAuditSign,
}

func init() {
	// Payload source flags
	auditSignCmd.Flags().StringVar(&auditSignPayload, "payload", "", "JSON payload to sign")
	auditSignCmd.Flags().StringVar(&auditSignPayloadFile, "payload-file", "", "Path to JSON payload file")

	// Provider selection
	auditSignCmd.Flags().StringVar(&auditSignProvider, "signing-provider", "",
		fmt.Sprintf("Signing provider to use (%s)", strings.Join(signer.DefaultRegistry.Names(), ", ")))
	auditSignCmd.Flags().StringVar(&auditSignHSMProvider, "hsm-provider", "",
		"Deprecated: use --signing-provider instead. HSM provider (pkcs11)")
	_ = auditSignCmd.Flags().MarkDeprecated("hsm-provider", "use --signing-provider instead")

	// Software provider flags
	auditSignCmd.Flags().StringVar(&auditSignSoftwareKey, "software-private-key", "",
		"PKCS#8 PEM Ed25519 private key for software signing (literal PEM or file path)")

	// PKCS#11 provider flags
	auditSignCmd.Flags().StringVar(&auditSignPKCS11Module, "pkcs11-module", "",
		"Path to PKCS#11 shared library (overrides GLASSBOX_PKCS11_MODULE)")
	auditSignCmd.Flags().StringVar(&auditSignPKCS11PIN, "pkcs11-pin", "",
		"PKCS#11 user PIN (overrides GLASSBOX_PKCS11_PIN)")
	auditSignCmd.Flags().StringVar(&auditSignPKCS11TokenLabel, "pkcs11-token-label", "",
		"PKCS#11 token label (overrides GLASSBOX_PKCS11_TOKEN_LABEL)")
	auditSignCmd.Flags().StringVar(&auditSignPKCS11KeyLabel, "pkcs11-key-label", "",
		"PKCS#11 key CKA_LABEL (overrides GLASSBOX_PKCS11_KEY_LABEL)")
	auditSignCmd.Flags().StringVar(&auditSignPKCS11KeyIDHex, "pkcs11-key-id", "",
		"PKCS#11 key CKA_ID in hex (overrides GLASSBOX_PKCS11_KEY_ID)")
	auditSignCmd.Flags().BoolVar(&auditSignValidateOnly, "validate-only", false,
		"Run PKCS#11 preflight checks and exit without signing")

	// Provenance flags
	auditSignCmd.Flags().StringVar(&auditSignSignerIdentity, "signer-identity", "",
		"Human-readable signer identity (e.g. email, team name) stored in provenance metadata")
	auditSignCmd.Flags().StringVar(&auditSignKeyID, "key-id", "",
		"Opaque key identifier stored in provenance metadata (e.g. PKCS#11 label, KMS ARN)")
	auditSignCmd.Flags().StringVar(&auditSignCertChainFile, "cert-chain", "",
		"Path to a PEM file containing the certificate chain (leaf first) for provenance")
	auditSignCmd.Flags().StringVar(&auditSignPreviousSignatureHash, "previous-signature-hash", "",
		"Hex-encoded SHA-256 of the previous signed audit log to form a verification chain")

	rootCmd.AddCommand(auditSignCmd)
}

func runAuditSign(cmd *cobra.Command, args []string) error {
	// --validate-only: run PKCS#11 preflight checks and exit without signing.
	if auditSignValidateOnly {
		return runPkcs11Preflight(cmd)
	}

	if auditSignPayload != "" && auditSignPayloadFile != "" {
		return errors.WrapValidationError("only one of --payload or --payload-file may be provided")
	}

	payloadBytes, err := readAuditPayload(auditSignPayload, auditSignPayloadFile)
	if err != nil {
		return err
	}

	if len(strings.TrimSpace(string(payloadBytes))) == 0 {
		return errors.WrapValidationError("payload is required")
	}

	var payload interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("invalid JSON payload: %v", err))
	}

	canonicalPayload, err := marshalCanonical(payload)
	if err != nil {
		return errors.WrapMarshalFailed(err)
	}

	providerName, cfg := resolveProviderAndConfig()

	signerImpl, err := signer.DefaultRegistry.CreateSigner(providerName, cfg)
	if err != nil {
		return err
	}
	defer func() {
		if closer, ok := signerImpl.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()

	hash := sha256.Sum256(canonicalPayload)
	signature, err := signerImpl.Sign(hash[:])
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("signing failed: %v", err))
	}

	publicKey, err := signerImpl.PublicKey()
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to retrieve public key: %v", err))
	}

	auditLog := SignedAuditLog{
		Version:   "1.0.0",
		Timestamp: time.Now().UTC(),
		TraceHash: hex.EncodeToString(hash[:]),
		Signature: hex.EncodeToString(signature),
		PublicKey: hex.EncodeToString(publicKey),
		Provider:  providerName,
		Payload:   json.RawMessage(payloadBytes),
	}

	// Attach provenance metadata when any provenance flag is set.
	prov, err := buildProvenance(providerName)
	if err != nil {
		return err
	}
	if prov != nil {
		auditLog.Provenance = prov
	}

	output, err := json.MarshalIndent(auditLog, "", "  ")
	if err != nil {
		return errors.WrapMarshalFailed(err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}

// resolveProviderAndConfig determines the active provider name and builds a
// ProviderConfig from CLI flags and environment variables.
//
// Provider selection order:
//  1. --signing-provider flag
//  2. --hsm-provider flag (deprecated; "pkcs11" maps to "pkcs11")
//  3. GLASSBOX_SIGNING_PROVIDER environment variable
//  4. GLASSBOX_SIGNER_TYPE environment variable (legacy)
//  5. "software" (default)
func resolveProviderAndConfig() (string, signer.ProviderConfig) {
	cfg := signer.ProviderConfig{
		SoftwareKeyPEM:   auditSignSoftwareKey,
		PKCS11ModulePath: auditSignPKCS11Module,
		PKCS11PIN:        auditSignPKCS11PIN,
		PKCS11TokenLabel: auditSignPKCS11TokenLabel,
		PKCS11KeyLabel:   auditSignPKCS11KeyLabel,
		PKCS11KeyIDHex:   auditSignPKCS11KeyIDHex,
	}

	// Determine provider name
	name := auditSignProvider
	if name == "" && auditSignHSMProvider != "" {
		// Legacy --hsm-provider flag: map "pkcs11" → "pkcs11"
		name = strings.ToLower(auditSignHSMProvider)
	}
	if name == "" {
		name = os.Getenv("GLASSBOX_SIGNING_PROVIDER")
	}
	if name == "" {
		// Legacy GLASSBOX_SIGNER_TYPE support
		if legacy := os.Getenv("GLASSBOX_SIGNER_TYPE"); legacy != "" {
			name = strings.ToLower(legacy)
		}
	}
	if name == "" {
		name = "software"
	}

	return name, cfg
}

// readAuditPayload reads the payload from the provided string, file, or stdin.
func readAuditPayload(payload, payloadFile string) ([]byte, error) {
	if payloadFile != "" {
		b, err := os.ReadFile(payloadFile)
		if err != nil {
			return nil, errors.WrapValidationError(fmt.Sprintf("failed to read payload file: %v", err))
		}
		return b, nil
	}

	if payload != "" {
		return []byte(payload), nil
	}

	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, errors.WrapValidationError(fmt.Sprintf("failed to inspect stdin: %v", err))
	}

	if stat.Mode()&os.ModeCharDevice == 0 {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, errors.WrapValidationError(fmt.Sprintf("failed to read payload from stdin: %v", err))
		}
		return b, nil
	}

	return nil, nil
}

// resolveAuditSigner is kept for backward compatibility with existing tests
// and callers that have not yet migrated to the registry-based API.
//
// Deprecated: use signer.DefaultRegistry.CreateSigner instead.
func resolveAuditSigner() (signer.Signer, error) {
	name, cfg := resolveProviderAndConfig()
	return signer.DefaultRegistry.CreateSigner(name, cfg)
}

// runPkcs11Preflight executes the PKCS#11 preflight validator and prints a
// human-readable report. It exits with a non-zero status if any check fails.
func runPkcs11Preflight(cmd *cobra.Command) error {
	if !strings.EqualFold(auditSignHSMProvider, "pkcs11") {
		return errors.WrapValidationError("--validate-only requires --hsm-provider pkcs11")
	}

	cfg, err := signer.Pkcs11ConfigFromEnv()
	if err != nil {
		return err
	}

	vcfg := signer.DefaultValidatorConfig()
	validator := signer.NewPkcs11Validator(*cfg, vcfg, &signer.OsPkcs11Provider{})

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Running PKCS#11 preflight checks...")
	fmt.Fprintln(out)

	report := validator.Validate(context.Background())

	for _, r := range report.Results {
		if r.OK {
			fmt.Fprintf(out, "  [PASS] %-14s %s\n", r.Step, r.Message)
		} else {
			fmt.Fprintf(out, "  [FAIL] %-14s %s\n", r.Step, r.Message)
			fmt.Fprintf(out, "         %-14s Remediation: %s\n", "", r.Remediation)
		}
	}

	fmt.Fprintln(out)
	if report.Ready {
		fmt.Fprintln(out, "Result: PKCS#11 configuration is valid and ready for signing.")
		return nil
	}
	return errors.WrapValidationError("PKCS#11 preflight checks failed; review the output above for remediation steps")
}


// buildProvenance constructs a SignatureProvenance from CLI flags.
// Returns nil when no provenance flags are set (backward-compatible behaviour).
func buildProvenance(providerName string) (*SignatureProvenance, error) {
	hasAny := auditSignSignerIdentity != "" ||
		auditSignKeyID != "" ||
		auditSignCertChainFile != "" ||
		auditSignPreviousSignatureHash != ""

	if !hasAny {
		return nil, nil
	}

	prov := &SignatureProvenance{
		SignerIdentity:        auditSignSignerIdentity,
		KeyID:                 auditSignKeyID,
		PreviousSignatureHash: auditSignPreviousSignatureHash,
	}

	// Derive algorithm from provider name.
	switch providerName {
	case "pkcs11":
		prov.Algorithm = "ECDSA-P256"
	default:
		prov.Algorithm = "Ed25519"
	}

	// Load certificate chain from file when provided.
	if auditSignCertChainFile != "" {
		chainPEM, err := os.ReadFile(auditSignCertChainFile)
		if err != nil {
			return nil, errors.WrapValidationError(fmt.Sprintf("failed to read cert chain file: %v", err))
		}
		prov.CertificateChain = parsePEMChain(chainPEM)
	}

	return prov, nil
}

// parsePEMChain splits a PEM file into individual certificate strings.
func parsePEMChain(data []byte) []string {
	var certs []string
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			certs = append(certs, string(pem.EncodeToMemory(block)))
		}
	}
	return certs
}
