// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package heuristic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotandev/glassbox/internal/simulator"
)

// ─── Rule compilation ─────────────────────────────────────────────────────────

func TestRuleCompile_PlainSubstring(t *testing.T) {
	r := &Rule{
		ID:       "test",
		Patterns: []string{"error(auth,"},
		Template: "auth failure",
	}
	if err := r.Compile(); err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}
	if !r.Matches("Error(Auth, NotAuthorized)") {
		t.Error("expected match for case-insensitive substring")
	}
}

func TestRuleCompile_Regexp(t *testing.T) {
	r := &Rule{
		ID:       "test-re",
		Patterns: []string{"re:error\\(budget,\\s*cpu"},
		Template: "cpu budget",
	}
	if err := r.Compile(); err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}
	if !r.Matches("Error(Budget, CpuLimitExceeded)") {
		t.Error("expected regexp match")
	}
}

func TestRuleCompile_InvalidRegexp(t *testing.T) {
	r := &Rule{
		ID:       "bad-re",
		Patterns: []string{"re:[invalid"},
		Template: "x",
	}
	if err := r.Compile(); err == nil {
		t.Error("expected compile error for invalid regexp")
	}
}

func TestRuleMatchKind_All(t *testing.T) {
	r := &Rule{
		ID:        "all-test",
		MatchKind: MatchAll,
		Patterns:  []string{"foo", "bar"},
		Template:  "both",
	}
	if err := r.Compile(); err != nil {
		t.Fatal(err)
	}
	if r.Matches("foo only") {
		t.Error("MatchAll should not fire when only one pattern matches")
	}
	if !r.Matches("foo and bar together") {
		t.Error("MatchAll should fire when all patterns match")
	}
}

// ─── Rule loading ─────────────────────────────────────────────────────────────

func writeRuleFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writing rule file: %v", err)
	}
	return path
}

const validJSONRules = `{
  "version": "1",
  "rules": [
    {
      "id": "custom-auth",
      "description": "Custom auth rule",
      "priority": 5,
      "severity": "error",
      "patterns": ["error(auth,"],
      "template": "Custom auth failure on {{.Network}}"
    }
  ]
}`

const validYAMLRules = `version: "1"
rules:
  - id: yaml-wasm
    description: YAML wasm trap rule
    priority: 55
    severity: error
    patterns:
      - "wasm trap"
    template: "YAML wasm trap on {{.Network}}"
`

func TestLoadRulesFromFile_JSON(t *testing.T) {
	dir := t.TempDir()
	path := writeRuleFile(t, dir, "rules.json", validJSONRules)

	rules, err := LoadRulesFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ID != "custom-auth" {
		t.Errorf("unexpected rule ID: %s", rules[0].ID)
	}
}

func TestLoadRulesFromFile_YAML(t *testing.T) {
	dir := t.TempDir()
	path := writeRuleFile(t, dir, "rules.yaml", validYAMLRules)

	rules, err := LoadRulesFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ID != "yaml-wasm" {
		t.Errorf("unexpected rule ID: %s", rules[0].ID)
	}
}

func TestLoadRulesFromFile_MissingID(t *testing.T) {
	dir := t.TempDir()
	path := writeRuleFile(t, dir, "bad.json", `{
  "version": "1",
  "rules": [{"patterns": ["foo"], "template": "bar"}]
}`)
	if _, err := LoadRulesFromFile(path); err == nil {
		t.Error("expected error for missing rule ID")
	}
}

func TestLoadRulesFromFile_MissingTemplate(t *testing.T) {
	dir := t.TempDir()
	path := writeRuleFile(t, dir, "bad.json", `{
  "version": "1",
  "rules": [{"id": "x", "patterns": ["foo"]}]
}`)
	if _, err := LoadRulesFromFile(path); err == nil {
		t.Error("expected error for missing template")
	}
}

func TestLoadRulesFromFile_EmptyPatterns(t *testing.T) {
	dir := t.TempDir()
	path := writeRuleFile(t, dir, "bad.json", `{
  "version": "1",
  "rules": [{"id": "x", "template": "y", "patterns": []}]
}`)
	if _, err := LoadRulesFromFile(path); err == nil {
		t.Error("expected error for empty patterns")
	}
}

func TestLoadRulesFromFile_UnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := writeRuleFile(t, dir, "rules.toml", `[rules]`)
	if _, err := LoadRulesFromFile(path); err == nil {
		t.Error("expected error for unsupported extension")
	}
}

func TestLoadRulesFromDir(t *testing.T) {
	dir := t.TempDir()
	writeRuleFile(t, dir, "a.json", validJSONRules)
	writeRuleFile(t, dir, "b.yaml", validYAMLRules)
	writeRuleFile(t, dir, "ignore.txt", "not a rule file")

	rs, err := LoadRulesFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rs.Rules()) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rs.Rules()))
	}
}

func TestBuildRuleSet_DuplicateID(t *testing.T) {
	dir := t.TempDir()
	dup := `{
  "version": "1",
  "rules": [
    {"id": "dup", "priority": 1, "patterns": ["foo"], "template": "a"},
    {"id": "dup", "priority": 2, "patterns": ["bar"], "template": "b"}
  ]
}`
	path := writeRuleFile(t, dir, "dup.json", dup)
	if _, err := LoadRulesFromFile(path); err == nil {
		t.Error("expected error for duplicate rule ID")
	}
}

func TestBuildRuleSet_PriorityOrder(t *testing.T) {
	dir := t.TempDir()
	content := `{
  "version": "1",
  "rules": [
    {"id": "low",  "priority": 100, "patterns": ["foo"], "template": "low"},
    {"id": "high", "priority": 1,   "patterns": ["bar"], "template": "high"}
  ]
}`
	path := writeRuleFile(t, dir, "order.json", content)
	rules, err := LoadRulesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rs, err := buildRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	if rs.Rules()[0].ID != "high" {
		t.Errorf("expected 'high' first, got %s", rs.Rules()[0].ID)
	}
}

// ─── Engine evaluation ────────────────────────────────────────────────────────

func TestEngine_AuthCrossContract(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "testnet",
		Status:  "error",
		Error:   "Error(Auth, InvalidAction)",
		DiagnosticEvents: []simulator.DiagnosticEvent{
			{ContractID: strPtr("CABC"), Topics: []string{"call"}},
			{ContractID: strPtr("CDEF"), Topics: []string{"require_auth"}},
		},
	}
	got := defaultEngine.Evaluate(in)
	if !strings.Contains(got, "CABC") || !strings.Contains(got, "CDEF") {
		t.Errorf("expected both contract IDs, got: %s", got)
	}
	if !strings.Contains(got, "authorization") {
		t.Errorf("expected 'authorization', got: %s", got)
	}
}

func TestEngine_BudgetCPU(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "Error(Budget, CpuLimitExceeded)",
		BudgetUsage: &simulator.BudgetUsage{
			CPUInstructions:    100_000_000,
			CPULimit:           100_000_000,
			CPUUsagePercent:    100.0,
			MemoryBytes:        10_000_000,
			MemoryLimit:        50_000_000,
			MemoryUsagePercent: 20.0,
		},
	}
	got := defaultEngine.Evaluate(in)
	if !strings.Contains(got, "CPU") {
		t.Errorf("expected CPU mention, got: %s", got)
	}
	if !strings.Contains(got, "100000000") {
		t.Errorf("expected CPU instructions value in output, got: %s", got)
	}
	if !strings.Contains(got, "Optimize contract logic") {
		t.Errorf("expected remediation hint in output, got: %s", got)
	}
}

func TestEngine_BudgetMemory(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "Error(Budget, MemLimitExceeded)",
		BudgetUsage: &simulator.BudgetUsage{
			CPUInstructions:    50_000_000,
			CPULimit:           100_000_000,
			CPUUsagePercent:    50.0,
			MemoryBytes:        50_000_000,
			MemoryLimit:        50_000_000,
			MemoryUsagePercent: 100.0,
		},
	}
	got := defaultEngine.Evaluate(in)
	if !strings.Contains(got, "memory") {
		t.Errorf("expected memory mention, got: %s", got)
	}
	if !strings.Contains(got, "50000000") {
		t.Errorf("expected memory bytes value in output, got: %s", got)
	}
	if !strings.Contains(got, "Reduce temporary memory allocation") {
		t.Errorf("expected remediation hint in output, got: %s", got)
	}
}

func TestEngine_BudgetBoth_ViaUsage(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		BudgetUsage: &simulator.BudgetUsage{
			CPUUsagePercent:    100.0,
			MemoryUsagePercent: 100.0,
			CPUInstructions:    100_000_000,
			CPULimit:           100_000_000,
			MemoryBytes:        50_000_000,
			MemoryLimit:        50_000_000,
		},
	}
	got := defaultEngine.Evaluate(in)
	if !strings.Contains(got, "CPU") || !strings.Contains(got, "memory") {
		t.Errorf("expected both CPU and memory, got: %s", got)
	}
}

func TestEngine_FallbackWithError(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "some unknown simulator error",
	}
	got := defaultEngine.Evaluate(in)
	if !strings.Contains(got, "some unknown simulator error") {
		t.Errorf("expected raw error in fallback, got: %s", got)
	}
}

func TestEngine_FallbackNoError(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
	}
	got := defaultEngine.Evaluate(in)
	if !strings.Contains(got, "failed") {
		t.Errorf("expected failure mention in fallback, got: %s", got)
	}
}

// ─── Custom rule overrides built-in ──────────────────────────────────────────

func TestEngine_CustomRuleOverridesBuiltin(t *testing.T) {
	dir := t.TempDir()
	// A custom rule with priority 1 (higher than built-in auth at 10).
	custom := `{
  "version": "1",
  "rules": [
    {
      "id": "custom-auth-override",
      "description": "Custom auth override",
      "priority": 1,
      "severity": "error",
      "patterns": ["error(auth,"],
      "template": "CUSTOM: auth failure on {{.Network}}"
    }
  ]
}`
	path := writeRuleFile(t, dir, "custom.json", custom)

	engine, err := LoadEngineWithBuiltins([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "testnet",
		Status:  "error",
		Error:   "Error(Auth, NotAuthorized)",
	}
	got := engine.Evaluate(in)
	if !strings.HasPrefix(got, "CUSTOM:") {
		t.Errorf("expected custom rule to fire first, got: %s", got)
	}
}

// ─── Conflict detection ───────────────────────────────────────────────────────

func TestDetectConflicts(t *testing.T) {
	dir := t.TempDir()
	content := `{
  "version": "1",
  "rules": [
    {"id": "a", "priority": 10, "patterns": ["error(auth,"], "template": "a"},
    {"id": "b", "priority": 20, "patterns": ["error(auth, notauthorized)"], "template": "b"}
  ]
}`
	path := writeRuleFile(t, dir, "conflict.json", content)
	rules, err := LoadRulesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rs, err := buildRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	conflicts := DetectConflicts(rs)
	if len(conflicts) == 0 {
		t.Error("expected at least one conflict")
	}
	if conflicts[0].WinnerID != "a" {
		t.Errorf("expected 'a' to win, got %s", conflicts[0].WinnerID)
	}
}

func TestDetectConflicts_NoConflict(t *testing.T) {
	dir := t.TempDir()
	content := `{
  "version": "1",
  "rules": [
    {"id": "a", "priority": 10, "patterns": ["cpulimitexceeded"], "template": "a"},
    {"id": "b", "priority": 20, "patterns": ["insufficient_balance"], "template": "b"}
  ]
}`
	path := writeRuleFile(t, dir, "noconflict.json", content)
	rules, err := LoadRulesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rs, err := buildRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	if conflicts := DetectConflicts(rs); len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %d", len(conflicts))
	}
}

// ─── Multiple rule files ──────────────────────────────────────────────────────

func TestLoadEngineFromDir_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeRuleFile(t, dir, "auth.json", `{
  "version": "1",
  "rules": [
    {"id": "ext-auth", "priority": 5, "patterns": ["error(auth,"], "template": "ext auth on {{.Network}}"}
  ]
}`)
	writeRuleFile(t, dir, "budget.yaml", `version: "1"
rules:
  - id: ext-budget
    priority: 15
    patterns:
      - "cpulimitexceeded"
    template: "ext cpu on {{.Network}}"
`)

	engine, err := LoadEngineFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rules := engine.RuleSet().Rules()
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	// Priority order: ext-auth (5) before ext-budget (15)
	if rules[0].ID != "ext-auth" {
		t.Errorf("expected ext-auth first, got %s", rules[0].ID)
	}
}

// ─── Validate ─────────────────────────────────────────────────────────────────

func TestValidateRuleFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := writeRuleFile(t, dir, "valid.json", validJSONRules)
	if err := ValidateRuleFile(path); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestValidateRuleFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := writeRuleFile(t, dir, "empty.json", `{"version":"1","rules":[]}`)
	if err := ValidateRuleFile(path); err == nil {
		t.Error("expected error for empty rule file")
	}
}

// ─── Template rendering ───────────────────────────────────────────────────────

func TestRenderTemplate(t *testing.T) {
	data := templateData{
		TxHash:  "abc...def",
		Network: "testnet",
		Caller:  "CABC",
		Callee:  "CDEF",
		Error:   "some error",
	}
	out, err := renderTemplate("tx {{.TxHash}} on {{.Network}}: caller={{.Caller}} callee={{.Callee}}", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "abc...def") || !strings.Contains(out, "testnet") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestRenderTemplate_InvalidTemplate(t *testing.T) {
	data := templateData{}
	if _, err := renderTemplate("{{.Missing", data); err == nil {
		t.Error("expected error for invalid template")
	}
}

// ─── Storage overflow heuristic rules ────────────────────────────────────────

func TestEngine_StorageOverflow_EntryCount(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "testnet",
		Status:  "error",
		Error:   "error(storage, full): too many ledger entries",
	}
	got := defaultEngine.Evaluate(in)
	if !strings.Contains(strings.ToLower(got), "storage") {
		t.Errorf("expected storage mention in output, got: %s", got)
	}
	if !strings.Contains(strings.ToLower(got), "archive") || !strings.Contains(strings.ToLower(got), "transaction") {
		t.Errorf("expected remediation guidance in output, got: %s", got)
	}
}

func TestEngine_StorageOverflow_EntrySize(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "ledger entry too large: value exceeds size limit",
	}
	got := defaultEngine.Evaluate(in)
	if !strings.Contains(strings.ToLower(got), "storage") || !strings.Contains(strings.ToLower(got), "size") {
		t.Errorf("expected storage/size mention in output, got: %s", got)
	}
}

func TestEngine_StorageOverflow_FootprintTooLarge(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "futurenet",
		Status:  "error",
		Error:   "footprint exceed: too many keys in read set",
	}
	got := defaultEngine.Evaluate(in)
	if !strings.Contains(strings.ToLower(got), "footprint") {
		t.Errorf("expected footprint mention in output, got: %s", got)
	}
}

func TestEngine_StorageOverflow_PrioritisedBeforeMissingEntry(t *testing.T) {
	// "storagefull" must fire storage-overflow (priority 38) before missing-ledger-entry
	// (priority 40) even though missing-ledger-entry has pattern "error(storage,".
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "StorageFull: ledger entry count limit exceeded",
	}
	got := defaultEngine.Evaluate(in)
	if !strings.Contains(strings.ToLower(got), "storage") {
		t.Errorf("expected storage-overflow rule to fire, got: %s", got)
	}
	if strings.Contains(strings.ToLower(got), "not present") {
		t.Errorf("missing-ledger-entry rule should NOT have fired, got: %s", got)
	}
}
