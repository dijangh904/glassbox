// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildSignedLog creates and writes a valid SignedAuditLog to a temp file,
// returning its path and the key pair used.
func buildSignedLog(t *testing.T, payload interface{}) (string, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	canonical, err := marshalCanonical(payload)
	require.NoError(t, err)
	hash := sha256.Sum256(canonical)
	sig := ed25519.Sign(priv, hash[:])

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	log := SignedAuditLog{
		Version:   "1.0.0",
		Timestamp: time.Now().UTC(),
		TraceHash: hex.EncodeToString(hash[:]),
		Signature: hex.EncodeToString(sig),
		PublicKey: hex.EncodeToString(pub),
		Provider:  "software",
		Payload:   json.RawMessage(raw),
	}

	data, err := json.MarshalIndent(log, "", "  ")
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "audit.json")
	require.NoError(t, os.WriteFile(path, data, 0600))
	return path, pub, priv
}

func TestAuditVerify_ValidLog(t *testing.T) {
	payload := map[string]interface{}{"input": "data", "events": []interface{}{"e1"}}
	path, _, _ := buildSignedLog(t, payload)

	auditVerifyFile = path
	auditVerifyPublicKey = ""
	auditVerifySchema = ""
	auditVerifyJSON = false

	cmd := auditVerifyCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cmd.RunE(cmd, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "VALID")
	assert.Contains(t, buf.String(), "[PASS] Payload hash")
	assert.Contains(t, buf.String(), "[PASS] Signature")
}

func TestAuditVerify_TamperedPayload(t *testing.T) {
	payload := map[string]interface{}{"input": "original"}
	path, _, _ := buildSignedLog(t, payload)

	// Read and tamper with the file.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var log SignedAuditLog
	require.NoError(t, json.Unmarshal(data, &log))
	log.Payload = json.RawMessage(`{"input":"tampered"}`)
	tampered, err := json.Marshal(log)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, tampered, 0600))

	auditVerifyFile = path
	auditVerifyPublicKey = ""
	auditVerifySchema = ""
	auditVerifyJSON = false

	cmd := auditVerifyCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err = cmd.RunE(cmd, nil)
	assert.Error(t, err, "tampered payload should fail verification")
	assert.Contains(t, buf.String(), "[FAIL]")
	assert.Contains(t, buf.String(), "INVALID")
}

func TestAuditVerify_InvalidSignature(t *testing.T) {
	payload := map[string]interface{}{"input": "data"}
	path, _, _ := buildSignedLog(t, payload)

	// Replace signature with garbage.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var log SignedAuditLog
	require.NoError(t, json.Unmarshal(data, &log))
	log.Signature = strings.Repeat("aa", ed25519.SignatureSize)
	badData, err := json.Marshal(log)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, badData, 0600))

	auditVerifyFile = path
	auditVerifyPublicKey = ""
	auditVerifySchema = ""
	auditVerifyJSON = false

	cmd := auditVerifyCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err = cmd.RunE(cmd, nil)
	assert.Error(t, err)
	assert.Contains(t, buf.String(), "[FAIL] Signature")
}

func TestAuditVerify_PublicKeyOverride(t *testing.T) {
	payload := map[string]interface{}{"x": 1}
	path, pub, _ := buildSignedLog(t, payload)

	auditVerifyFile = path
	auditVerifyPublicKey = hex.EncodeToString(pub)
	auditVerifySchema = ""
	auditVerifyJSON = false

	cmd := auditVerifyCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cmd.RunE(cmd, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "VALID")
}

func TestAuditVerify_WrongPublicKey(t *testing.T) {
	payload := map[string]interface{}{"x": 1}
	path, _, _ := buildSignedLog(t, payload)

	wrongPub, _, _ := ed25519.GenerateKey(nil)

	auditVerifyFile = path
	auditVerifyPublicKey = hex.EncodeToString(wrongPub)
	auditVerifySchema = ""
	auditVerifyJSON = false

	cmd := auditVerifyCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cmd.RunE(cmd, nil)
	assert.Error(t, err)
	assert.Contains(t, buf.String(), "[FAIL] Signature")
}

func TestAuditVerify_JSONOutput(t *testing.T) {
	payload := map[string]interface{}{"key": "val"}
	path, _, _ := buildSignedLog(t, payload)

	auditVerifyFile = path
	auditVerifyPublicKey = ""
	auditVerifySchema = ""
	auditVerifyJSON = true

	cmd := auditVerifyCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cmd.RunE(cmd, nil)
	require.NoError(t, err)

	var result auditVerifyResult
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.True(t, result.Valid)
	assert.True(t, result.HashValid)
	assert.True(t, result.SignatureValid)
}

func TestAuditVerify_SchemaValid(t *testing.T) {
	payload := map[string]interface{}{"input": "data", "events": []interface{}{}}
	path, _, _ := buildSignedLog(t, payload)

	schema := map[string]interface{}{
		"required": []interface{}{"input", "events"},
		"properties": map[string]interface{}{
			"input": map[string]interface{}{"type": "string"},
		},
	}
	schemaData, err := json.Marshal(schema)
	require.NoError(t, err)
	schemaPath := filepath.Join(t.TempDir(), "schema.json")
	require.NoError(t, os.WriteFile(schemaPath, schemaData, 0600))

	auditVerifyFile = path
	auditVerifyPublicKey = ""
	auditVerifySchema = schemaPath
	auditVerifyJSON = false

	cmd := auditVerifyCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err = cmd.RunE(cmd, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "[PASS] Schema")
}

func TestAuditVerify_SchemaInvalid(t *testing.T) {
	payload := map[string]interface{}{"events": []interface{}{}}
	path, _, _ := buildSignedLog(t, payload)

	schema := map[string]interface{}{
		"required": []interface{}{"input", "events"},
	}
	schemaData, err := json.Marshal(schema)
	require.NoError(t, err)
	schemaPath := filepath.Join(t.TempDir(), "schema.json")
	require.NoError(t, os.WriteFile(schemaPath, schemaData, 0600))

	auditVerifyFile = path
	auditVerifyPublicKey = ""
	auditVerifySchema = schemaPath
	auditVerifyJSON = false

	cmd := auditVerifyCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err = cmd.RunE(cmd, nil)
	assert.Error(t, err)
	assert.Contains(t, buf.String(), "[FAIL] Schema")
}

func TestAuditVerify_MissingFile(t *testing.T) {
	auditVerifyFile = ""
	auditVerifyPublicKey = ""
	auditVerifySchema = ""
	auditVerifyJSON = false

	cmd := auditVerifyCmd
	err := cmd.PreRunE(cmd, nil)
	assert.Error(t, err)
}
