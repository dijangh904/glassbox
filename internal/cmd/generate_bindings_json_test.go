// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/dotandev/glassbox/internal/clioutput"
)

func TestGenerateBindingsOutput_JSONEnvelope(t *testing.T) {
	var buf bytes.Buffer
	out := generateBindingsOutput{
		Package: "my-contract",
		Output:  "./bindings",
		Runtime: "node",
		Files:   []string{"./bindings/client.ts"},
	}
	if err := clioutput.Write(&buf, "generate-bindings", out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var env clioutput.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Command != "generate-bindings" {
		t.Fatalf("command = %q", env.Command)
	}
}
