// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/dotandev/glassbox/internal/clioutput"
)

func TestProtocolDiagnose_JSONEnvelope(t *testing.T) {
	var buf bytes.Buffer
	report := map[string]string{"status": "ok"}
	if err := clioutput.Write(&buf, "protocol:diagnose", report); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var env clioutput.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.SchemaVersion != clioutput.SchemaVersion {
		t.Fatalf("schema_version = %q", env.SchemaVersion)
	}
}
