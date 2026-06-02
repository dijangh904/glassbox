// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dotandev/glassbox/internal/errors"
	"github.com/spf13/cobra"
)

var (
	auditVerifyFile      string
	auditVerifyPublicKey string
	auditVerifySchema    string
	auditVerifyJSON      bool
)

// auditVerifyResult is the structured output produced when --json is requested.
type auditVerifyResult struct {
	Valid           bool   `json:"valid"`
	Version         string `json:"version,omitempty"`
	Timestamp       string `json:"timestamp,omitempty"`
	TraceHash       string `json:"trace_hash,omitempty"`
	PublicKey       string `json:"public_key,omitempty"`
	Provider        string `json:"provider,omitempty"`
	SignatureValid  bool   `json:"signature_valid"`
	HashValid       bool   `json:"hash_valid"`
	SchemaValid     *bool  `json:"schema_valid,omitempty"`
	ProvenanceValid *bool  `json:"provenance_valid,omitempty"`
	Error           string `json:"error,omitempty"`
}

var auditVerifyCmd = &cobra.Command{
	Use:     "audit:verify",
	GroupID: "utility",
	Short:   "Verify the signature and payload of a signed audit log",
	Long: `Verify the integrity and authenticity of a signed audit log produced by
audit:sign. The command re-derives the payload hash, checks it against the
embedded trace_hash field, and verifies the Ed25519 signature.

EXAMPLES
  # Verify with the public key embedded in the file
  glassbox audit:verify --audit-log signed-audit.json

  # Override the public key for independent verification
  glassbox audit:verify --audit-log signed-audit.json --public-key <hex>

  # Validate payload structure against a JSON schema
  glassbox audit:verify --audit-log signed-audit.json --schema payload-schema.json

  # Machine-readable JSON output
  glassbox audit:verify --audit-log signed-audit.json --json`,
	Args: cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if auditVerifyFile == "" {
			return errors.WrapCliArgumentRequired("audit-log")
		}
		return validateFilePath("audit-log", auditVerifyFile)
	},
	RunE: runAuditVerify,
}

func init() {
	auditVerifyCmd.Flags().StringVar(&auditVerifyFile, "audit-log", "", "Path to signed audit log JSON file (required)")
	auditVerifyCmd.Flags().StringVar(&auditVerifyPublicKey, "public-key", "", "Hex-encoded Ed25519 public key (overrides key embedded in the log)")
	auditVerifyCmd.Flags().StringVar(&auditVerifySchema, "schema", "", "Path to a JSON schema file to validate the payload against")
	auditVerifyCmd.Flags().BoolVar(&auditVerifyJSON, "json", false, "Output verification result as JSON")

	rootCmd.AddCommand(auditVerifyCmd)
}

func runAuditVerify(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(auditVerifyFile)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to read audit log: %v", err))
	}

	var log SignedAuditLog
	if err := json.Unmarshal(data, &log); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to parse audit log JSON: %v", err))
	}

	result := auditVerifyResult{
		Version:   log.Version,
		Timestamp: log.Timestamp.Format(time.RFC3339),
		TraceHash: log.TraceHash,
		PublicKey: log.PublicKey,
		Provider:  log.Provider,
	}

	// Resolve public key: flag takes precedence over embedded key.
	pubKeyHex := log.PublicKey
	if auditVerifyPublicKey != "" {
		pubKeyHex = auditVerifyPublicKey
	}

	// Step 1: Re-derive payload hash.
	var payload interface{}
	if err := json.Unmarshal(log.Payload, &payload); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to parse payload JSON: %v", err))
	}
	canonicalBytes, err := marshalCanonical(payload)
	if err != nil {
		return errors.WrapMarshalFailed(err)
	}
	derivedHash := sha256.Sum256(canonicalBytes)
	derivedHashHex := hex.EncodeToString(derivedHash[:])

	result.HashValid = derivedHashHex == log.TraceHash

	// Step 2: Verify Ed25519 signature.
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		msg := "invalid public key"
		if err != nil {
			msg = fmt.Sprintf("invalid public key hex: %v", err)
		}
		result.Error = msg
		return outputVerifyResult(cmd, result, false)
	}

	sigBytes, err := hex.DecodeString(log.Signature)
	if err != nil {
		result.Error = fmt.Sprintf("invalid signature hex: %v", err)
		return outputVerifyResult(cmd, result, false)
	}

	result.SignatureValid = ed25519.Verify(ed25519.PublicKey(pubKeyBytes), derivedHash[:], sigBytes)

	// Step 3: Optional schema validation.
	if auditVerifySchema != "" {
		schemaValid, schemaErr := validatePayloadAgainstSchema(log.Payload, auditVerifySchema)
		result.SchemaValid = &schemaValid
		if schemaErr != nil && result.Error == "" {
			result.Error = fmt.Sprintf("schema validation: %v", schemaErr)
		}
	}

	// Step 4: Validate provenance fields when present.
	if log.Provenance != nil {
		provValid, provErr := validateProvenance(log.Provenance)
		result.ProvenanceValid = &provValid
		if provErr != nil && result.Error == "" {
			result.Error = fmt.Sprintf("provenance validation: %v", provErr)
		}
	}

	result.Valid = result.HashValid && result.SignatureValid &&
		(result.SchemaValid == nil || *result.SchemaValid) &&
		(result.ProvenanceValid == nil || *result.ProvenanceValid)

	return outputVerifyResult(cmd, result, result.Valid)
}

// outputVerifyResult prints the verification result in human-readable or JSON format.
func outputVerifyResult(cmd *cobra.Command, r auditVerifyResult, valid bool) error {
	out := cmd.OutOrStdout()

	if auditVerifyJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}

	fmt.Fprintln(out, "Audit Log Verification")
	fmt.Fprintln(out, strings.Repeat("─", 50))

	if r.Version != "" {
		fmt.Fprintf(out, "  Version:    %s\n", r.Version)
	}
	if r.Timestamp != "" {
		fmt.Fprintf(out, "  Timestamp:  %s\n", r.Timestamp)
	}
	if r.Provider != "" {
		fmt.Fprintf(out, "  Provider:   %s\n", r.Provider)
	}
	if r.PublicKey != "" {
		// Show a shortened key identifier: first 8 + last 8 hex chars.
		keyID := r.PublicKey
		if len(keyID) > 16 {
			keyID = keyID[:8] + "…" + keyID[len(keyID)-8:]
		}
		fmt.Fprintf(out, "  Key ID:     %s\n", keyID)
	}
	if r.TraceHash != "" {
		fmt.Fprintf(out, "  Trace Hash: %s\n", r.TraceHash)
	}

	fmt.Fprintln(out)
	printCheck(out, "Payload hash", r.HashValid)
	printCheck(out, "Signature   ", r.SignatureValid)
	if r.SchemaValid != nil {
		printCheck(out, "Schema      ", *r.SchemaValid)
	}
	if r.ProvenanceValid != nil {
		printCheck(out, "Provenance  ", *r.ProvenanceValid)
	}

	fmt.Fprintln(out)
	if valid {
		fmt.Fprintln(out, "Result: VALID — audit log integrity confirmed.")
		return nil
	}

	if r.Error != "" {
		fmt.Fprintf(out, "Error: %s\n", r.Error)
	}
	fmt.Fprintln(out, "Result: INVALID — audit log failed verification.")
	return errors.WrapAuditLogInvalid("audit log verification failed")
}

func printCheck(out interface{ Write([]byte) (int, error) }, label string, ok bool) {
	status := "PASS"
	if !ok {
		status = "FAIL"
	}
	fmt.Fprintf(out, "  [%s] %s\n", status, label)
}

// validatePayloadAgainstSchema validates payload bytes against a JSON schema file.
// It performs structural validation: checks that the schema's required keys exist
// in the payload and that the payload is a valid JSON object.
func validatePayloadAgainstSchema(payload json.RawMessage, schemaPath string) (bool, error) {
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return false, fmt.Errorf("failed to read schema file %q: %w", schemaPath, err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return false, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	var payloadMap map[string]interface{}
	if err := json.Unmarshal(payload, &payloadMap); err != nil {
		return false, fmt.Errorf("payload is not a JSON object: %w", err)
	}

	// Validate required fields declared in the schema.
	if required, ok := schema["required"]; ok {
		requiredList, ok := required.([]interface{})
		if !ok {
			return false, fmt.Errorf("schema 'required' field must be an array")
		}
		for _, field := range requiredList {
			fieldStr, ok := field.(string)
			if !ok {
				continue
			}
			if _, exists := payloadMap[fieldStr]; !exists {
				return false, fmt.Errorf("payload missing required field %q", fieldStr)
			}
		}
	}

	// Validate property types when a 'properties' section is present.
	if props, ok := schema["properties"]; ok {
		propsMap, ok := props.(map[string]interface{})
		if ok {
			for key, propDef := range propsMap {
				val, exists := payloadMap[key]
				if !exists {
					continue
				}
				propDefMap, ok := propDef.(map[string]interface{})
				if !ok {
					continue
				}
				expectedType, ok := propDefMap["type"].(string)
				if !ok {
					continue
				}
				if err := checkJSONType(key, val, expectedType); err != nil {
					return false, err
				}
			}
		}
	}

	return true, nil
}

// checkJSONType validates that val matches the expected JSON Schema type string.
func checkJSONType(field string, val interface{}, expected string) error {
	var actual string
	switch val.(type) {
	case string:
		actual = "string"
	case float64:
		actual = "number"
	case bool:
		actual = "boolean"
	case map[string]interface{}:
		actual = "object"
	case []interface{}:
		actual = "array"
	case nil:
		actual = "null"
	default:
		actual = "unknown"
	}

	if actual != expected {
		return fmt.Errorf("field %q has type %q, expected %q", field, actual, expected)
	}
	return nil
}

// validateProvenance checks that provenance metadata fields are well-formed.
// It validates the certificate chain PEM blocks and the previous-signature hash
// format when they are present.
func validateProvenance(p *SignatureProvenance) (bool, error) {
	if p == nil {
		return true, nil
	}

	// Validate certificate chain: each entry must be a parseable PEM block.
	for i, certPEM := range p.CertificateChain {
		block, _ := pem.Decode([]byte(certPEM))
		if block == nil {
			return false, fmt.Errorf("certificate_chain[%d] is not valid PEM", i)
		}
		if block.Type != "CERTIFICATE" {
			return false, fmt.Errorf("certificate_chain[%d] has unexpected PEM type %q", i, block.Type)
		}
	}

	// Validate previous_signature_hash: must be a 64-char hex string when set.
	if p.PreviousSignatureHash != "" {
		if len(p.PreviousSignatureHash) != 64 {
			return false, fmt.Errorf("previous_signature_hash must be a 64-character hex string (SHA-256)")
		}
		for _, c := range p.PreviousSignatureHash {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false, fmt.Errorf("previous_signature_hash contains non-hex character %q", c)
			}
		}
	}

	return true, nil
}
