// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package clioutput

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWantsJSON(t *testing.T) {
	if !WantsJSON(true, "") {
		t.Fatal("expected --json to enable JSON")
	}
	if !WantsJSON(false, "json") {
		t.Fatal("expected --format json to enable JSON")
	}
	if WantsJSON(false, "text") {
		t.Fatal("did not expect text format to enable JSON")
	}
}

func TestWriteEnvelope(t *testing.T) {
	var buf bytes.Buffer
	payload := map[string]string{"status": "ok"}
	if err := Write(&buf, "test-cmd", payload); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q, want %q", env.SchemaVersion, SchemaVersion)
	}
	if env.GlassboxVersion == "" {
		t.Fatal("expected glassbox_version to be set")
	}
	var inner map[string]string
	if err := json.Unmarshal(env.Data, &inner); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if inner["status"] != "ok" {
		t.Fatalf("data.status = %q", inner["status"])
	}
}
