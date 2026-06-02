// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"
	"time"
)

func TestCompareTraces_Identical(t *testing.T) {
	baseline := &ExecutionTrace{
		TransactionHash: "test123",
		StartTime:       time.Now(),
		EndTime:         time.Now().Add(time.Second),
		States: []ExecutionState{
			{
				Step:      0,
				Operation: "contract_call",
				ContractID: "CABC123",
				Function:  "init",
			},
		},
	}
	
	current := &ExecutionTrace{
		TransactionHash: "test123",
		StartTime:       baseline.StartTime,
		EndTime:         baseline.EndTime,
		States: []ExecutionState{
			{
				Step:      0,
				Operation: "contract_call",
				ContractID: "CABC123",
				Function:  "init",
			},
		},
	}
	
	result := CompareTraces(baseline, current, "", "")
	if result.HasDivergence {
		t.Fatalf("Expected identical traces, got divergence")
	}
}

func TestCompareTraces_DifferentStepCount(t *testing.T) {
	baseline := &ExecutionTrace{
		TransactionHash: "test123",
		States: []ExecutionState{
			{Step: 0, Operation: "contract_call"},
		},
	}
	
	current := &ExecutionTrace{
		TransactionHash: "test123",
		States: []ExecutionState{
			{Step: 0, Operation: "contract_call"},
			{Step: 1, Operation: "host_function"},
		},
	}
	
	result := CompareTraces(baseline, current, "", "")
	if !result.HasDivergence {
		t.Fatalf("Expected divergence due to different step counts, got none")
	}
	if len(result.StateDiffs) == 0 {
		t.Fatalf("Expected state diffs, got none")
	}
}

func TestCompareTraces_DifferentOperation(t *testing.T) {
	baseline := &ExecutionTrace{
		TransactionHash: "test123",
		States: []ExecutionState{
			{Step: 0, Operation: "contract_call", ContractID: "CABC123"},
		},
	}
	
	current := &ExecutionTrace{
		TransactionHash: "test123",
		States: []ExecutionState{
			{Step: 0, Operation: "host_function", ContractID: "CABC123"},
		},
	}
	
	result := CompareTraces(baseline, current, "", "")
	if !result.HasDivergence {
		t.Fatalf("Expected divergence due to different operations, got none")
	}
	foundOpDiff := false
	for _, diff := range result.StateDiffs {
		if diff.Field == "Operation" {
			foundOpDiff = true
			break
		}
	}
	if !foundOpDiff {
		t.Fatalf("Expected operation diff, but none found")
	}
}
