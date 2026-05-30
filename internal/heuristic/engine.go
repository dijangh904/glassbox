// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package heuristic

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed builtin_rules.json
var builtinRulesJSON []byte

// defaultEngine is the package-level engine initialised with the built-in
// rules.  It is replaced by SetDefaultEngine when the caller supplies external
// rule files.
var defaultEngine *Engine

func init() {
	var err error
	defaultEngine, err = newEngineFromBytes(builtinRulesJSON, ".json", "<builtin>")
	if err != nil {
		// Built-in rules are compiled into the binary; a parse error is a
		// programming mistake, not a runtime condition.
		panic(fmt.Sprintf("heuristic: failed to load built-in rules: %v", err))
	}
}

// Engine evaluates a prioritised RuleSet against an Input and renders the
// matching rule's template.
type Engine struct {
	rs *RuleSet
}

// newEngineFromBytes creates an Engine from raw rule data.
func newEngineFromBytes(data []byte, ext, source string) (*Engine, error) {
	rules, err := rulesFromBytes(data, ext, source)
	if err != nil {
		return nil, err
	}
	rs, err := buildRuleSet(rules)
	if err != nil {
		return nil, err
	}
	return &Engine{rs: rs}, nil
}

// rulesFromBytes parses rule data and returns compiled rules.
func rulesFromBytes(data []byte, ext, source string) ([]*Rule, error) {
	var rf RuleFile
	switch ext {
	case ".json":
		if err := jsonUnmarshalRuleFile(data, &rf); err != nil {
			return nil, fmt.Errorf("parsing JSON rules from %q: %w", source, err)
		}
	case ".yaml", ".yml":
		if err := yamlUnmarshal(data, &rf); err != nil {
			return nil, fmt.Errorf("parsing YAML rules from %q: %w", source, err)
		}
	default:
		return nil, fmt.Errorf("unsupported format %q", ext)
	}

	rules := make([]*Rule, 0, len(rf.Rules))
	for _, r := range rf.Rules {
		if r.ID == "" {
			return nil, fmt.Errorf("rule in %q is missing required field 'id'", source)
		}
		if r.Template == "" {
			return nil, fmt.Errorf("rule %q in %q: missing required field 'template'", r.ID, source)
		}
		if len(r.Patterns) == 0 {
			return nil, fmt.Errorf("rule %q in %q: 'patterns' must not be empty", r.ID, source)
		}
		r.Source = source
		if err := r.Compile(); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// LoadEngine creates an Engine from one or more explicit rule files.  The
// built-in rules are NOT included; the caller is responsible for providing a
// complete rule set.
func LoadEngine(paths []string) (*Engine, error) {
	rs, err := LoadRulesFromFiles(paths)
	if err != nil {
		return nil, err
	}
	return &Engine{rs: rs}, nil
}

// LoadEngineFromDir creates an Engine from all rule files found in dir.
func LoadEngineFromDir(dir string) (*Engine, error) {
	rs, err := LoadRulesFromDir(dir)
	if err != nil {
		return nil, err
	}
	return &Engine{rs: rs}, nil
}

// LoadEngineWithBuiltins creates an Engine that merges the built-in rules with
// additional rules loaded from the supplied paths.  External rules with lower
// priority numbers take precedence over built-in rules.
func LoadEngineWithBuiltins(paths []string) (*Engine, error) {
	builtinRules, err := rulesFromBytes(builtinRulesJSON, ".json", "<builtin>")
	if err != nil {
		return nil, err
	}

	var extraRules []*Rule
	for _, p := range paths {
		rules, err := LoadRulesFromFile(p)
		if err != nil {
			return nil, err
		}
		extraRules = append(extraRules, rules...)
	}

	all := append(builtinRules, extraRules...) //nolint:gocritic
	rs, err := buildRuleSet(all)
	if err != nil {
		return nil, err
	}
	return &Engine{rs: rs}, nil
}

// RuleSet returns the engine's underlying rule set (useful for listing / conflict detection).
func (e *Engine) RuleSet() *RuleSet {
	return e.rs
}

// templateData is the data passed to each rule's Go template.
type templateData struct {
	TxHash  string
	Network string
	Caller  string
	Callee  string
	Error   string
}

// Evaluate applies the engine's rules to in and returns the rendered
// suggestion string.  If no rule matches, a generic fallback is returned.
func (e *Engine) Evaluate(in Input) string {
	combined := strings.Join(append(in.Events, in.Logs...), " ") + " " + in.Error

	caller, callee := extractCallerCallee(in.DiagnosticEvents)

	data := templateData{
		TxHash:  shortHash(in.TxHash),
		Network: in.Network,
		Caller:  caller,
		Callee:  callee,
		Error:   sanitize(in.Error),
	}

	for _, rule := range e.rs.Rules() {
		if !ruleApplies(rule, in, combined) {
			continue
		}
		rendered, err := renderTemplate(rule.Template, data)
		if err != nil {
			// Template error is a rule authoring mistake; fall through to next rule.
			continue
		}
		return rendered
	}

	// Generic fallback.
	if in.Error != "" {
		return fmt.Sprintf(
			"Transaction %s failed on %s. The simulator reported: %s.",
			shortHash(in.TxHash), in.Network, sanitize(in.Error),
		)
	}
	return fmt.Sprintf(
		"Transaction %s failed on %s. No diagnostic information was produced; inspect the raw XDR for details.",
		shortHash(in.TxHash), in.Network,
	)
}

// ruleApplies checks all preconditions for a rule before pattern matching.
func ruleApplies(rule *Rule, in Input, combined string) bool {
	// Auth rules are split into three variants based on available context.
	switch rule.ID {
	case "auth-cross-contract":
		caller, callee := extractCallerCallee(in.DiagnosticEvents)
		if caller == "" || callee == "" {
			return false
		}
	case "auth-single-contract":
		caller, callee := extractCallerCallee(in.DiagnosticEvents)
		if callee == "" || caller != "" {
			return false
		}
	case "budget-cpu-and-memory":
		cpuOver, memOver := budgetFlags(in, combined)
		if !(cpuOver && memOver) {
			return false
		}
		// Budget flags already incorporate BudgetUsage; skip pattern matching
		// when the signal comes purely from BudgetUsage (no error string).
		return true
	case "budget-cpu":
		cpuOver, memOver := budgetFlags(in, combined)
		if !cpuOver || memOver {
			return false
		}
		return true
	case "budget-memory":
		cpuOver, memOver := budgetFlags(in, combined)
		if !memOver || cpuOver {
			return false
		}
		return true
	}

	if rule.RequireEvents && len(in.DiagnosticEvents) == 0 {
		return false
	}

	return rule.Matches(combined)
}

// budgetFlags returns whether CPU and/or memory budgets are exceeded.
func budgetFlags(in Input, combined string) (cpuOver, memOver bool) {
	lc := strings.ToLower(combined)
	cpuOver = strings.Contains(lc, "cpulimitexceeded") ||
		strings.Contains(lc, "cpu limit exceeded") ||
		strings.Contains(lc, "error(budget, cpu")
	memOver = strings.Contains(lc, "memlimitexceeded") ||
		strings.Contains(lc, "memory limit exceeded") ||
		strings.Contains(lc, "error(budget, mem")

	if in.BudgetUsage != nil {
		if in.BudgetUsage.CPUUsagePercent >= 100 {
			cpuOver = true
		}
		if in.BudgetUsage.MemoryUsagePercent >= 100 {
			memOver = true
		}
	}
	return
}

// renderTemplate executes a Go text/template with the given data.
func renderTemplate(tmpl string, data templateData) (string, error) {
	t, err := template.New("rule").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buf.String(), nil
}

// DefaultEngine returns the package-level engine initialised with the
// built-in rules.
func DefaultEngine() *Engine {
	return defaultEngine
}

// SetDefaultEngine replaces the package-level engine.  This is intended for
// use in main() after loading external rule files, and in tests.
func SetDefaultEngine(e *Engine) {
	defaultEngine = e
}
