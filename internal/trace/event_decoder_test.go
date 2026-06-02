// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"
	"time"
)

func TestDecodeContractEvent_JSON(t *testing.T) {
	raw := `{"contract_id":"CABC","topics":["transfer","from"],"data":"100","type":"contract"}`
	ev := DecodeContractEvent(raw)
	if ev.ContractID != "CABC" {
		t.Errorf("ContractID = %q, want %q", ev.ContractID, "CABC")
	}
	if len(ev.Topics) != 2 || ev.Topics[0] != "transfer" {
		t.Errorf("Topics = %v, want [transfer from]", ev.Topics)
	}
	if ev.Data != "100" {
		t.Errorf("Data = %q, want %q", ev.Data, "100")
	}
	if ev.Type != "contract" {
		t.Errorf("Type = %q, want %q", ev.Type, "contract")
	}
}

func TestDecodeContractEvent_TextFormat(t *testing.T) {
	raw := `contract:CDEF type:system data:ledger_bump`
	ev := DecodeContractEvent(raw)
	if ev.ContractID != "CDEF" {
		t.Errorf("ContractID = %q, want %q", ev.ContractID, "CDEF")
	}
	if ev.Type != "system" {
		t.Errorf("Type = %q, want %q", ev.Type, "system")
	}
}

func TestDecodeContractEvent_Fallback(t *testing.T) {
	raw := `plain text event`
	ev := DecodeContractEvent(raw)
	if ev.Data != raw {
		t.Errorf("Data = %q, want %q", ev.Data, raw)
	}
	if ev.TraceStep != -1 {
		t.Errorf("TraceStep = %d, want -1", ev.TraceStep)
	}
}

func TestDecodeContractEvents(t *testing.T) {
	raws := []string{
		`{"contract_id":"CA","topics":["mint"],"data":"50"}`,
		`{"contract_id":"CB","topics":["burn"],"data":"25"}`,
	}
	evs := DecodeContractEvents(raws)
	if len(evs) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(evs))
	}
	if evs[0].ContractID != "CA" || evs[1].ContractID != "CB" {
		t.Errorf("unexpected contract IDs: %s, %s", evs[0].ContractID, evs[1].ContractID)
	}
}

func TestDecodeContractEventWithSchemas_JSONSchema(t *testing.T) {
	schemas, err := ParseEventSchemas([]byte(`{
		"events": [{
			"name": "transfer",
			"fields": [
				{"name": "from", "type": "address"},
				{"name": "to", "type": "address"},
				{"name": "amount", "type": "i128"}
			]
		}]
	}`))
	if err != nil {
		t.Fatalf("ParseEventSchemas returned error: %v", err)
	}

	ev := DecodeContractEventWithSchemas(
		`{"contract_id":"CA","topics":["transfer"],"data":["alice","bob","100"]}`,
		schemas,
	)

	if ev.Name != "transfer" {
		t.Fatalf("Name = %q, want transfer", ev.Name)
	}
	if ev.Decoded["from"] != "alice" || ev.Decoded["to"] != "bob" || ev.Decoded["amount"] != "100" {
		t.Fatalf("Decoded = %#v, want named transfer fields", ev.Decoded)
	}
}

func TestParseEventSchemas_ABIStyle(t *testing.T) {
	schemas, err := ParseEventSchemas([]byte(`[
		{"type":"function","name":"transfer"},
		{"type":"event","name":"mint","inputs":[{"name":"admin"},{"name":"amount"}]}
	]`))
	if err != nil {
		t.Fatalf("ParseEventSchemas returned error: %v", err)
	}

	ev := DecodeContractEventWithSchemas(
		`{"contract_id":"CA","topics":["mint"],"data":["root","50"]}`,
		schemas,
	)

	if ev.Name != "mint" {
		t.Fatalf("Name = %q, want mint", ev.Name)
	}
	if ev.Decoded["admin"] != "root" || ev.Decoded["amount"] != "50" {
		t.Fatalf("Decoded = %#v, want ABI input names", ev.Decoded)
	}
}

func TestCorrelateEvents(t *testing.T) {
	tr := &ExecutionTrace{
		TransactionHash: "abc",
		StartTime:       time.Now(),
		EndTime:         time.Now(),
		States: []ExecutionState{
			{Step: 0, ContractID: "CA", Function: "transfer"},
			{Step: 1, ContractID: "CB", Function: "mint"},
			{Step: 2, ContractID: "CA", Function: "approve"},
		},
	}

	evs := []*ContractEvent{
		{ContractID: "CA", Data: "100", Type: "contract", TraceStep: -1},
		{ContractID: "CB", Data: "50", Type: "contract", TraceStep: -1},
		{ContractID: "CA", Data: "200", Type: "contract", TraceStep: -1},
	}

	CorrelateEvents(evs, tr)

	if evs[0].TraceStep != 0 {
		t.Errorf("event[0].TraceStep = %d, want 0", evs[0].TraceStep)
	}
	if evs[1].TraceStep != 1 {
		t.Errorf("event[1].TraceStep = %d, want 1", evs[1].TraceStep)
	}
	if evs[2].TraceStep != 2 {
		t.Errorf("event[2].TraceStep = %d, want 2", evs[2].TraceStep)
	}
}

func TestAttachEventsToNode(t *testing.T) {
	root := NewTraceNode("root", "simulation")
	child := NewTraceNode("c1", "contract_call")
	child.ContractID = "CA"
	root.AddChild(child)

	evs := []*ContractEvent{
		{ContractID: "CA", Topics: []string{"transfer"}, Data: "100", Type: "contract", TraceStep: 0},
	}

	AttachEventsToNode(root, evs)

	if len(child.Children) != 1 {
		t.Fatalf("child should have 1 event node, got %d", len(child.Children))
	}
	if child.Children[0].Type != "contract_event" {
		t.Errorf("expected contract_event type, got %q", child.Children[0].Type)
	}
}
