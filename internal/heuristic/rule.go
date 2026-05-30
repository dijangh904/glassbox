// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

// Package heuristic provides a rule-driven engine for producing plain-English
// explanations of Soroban transaction failures.
//
// Rules can be defined in JSON or YAML files and loaded at runtime, so new
// patterns can be added without recompiling the binary.  The built-in rules
// are embedded as a fallback when no external rule files are present.
package heuristic

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// MatchKind describes how a rule's patterns are evaluated against the combined
// signal string.
type MatchKind string

const (
	// MatchAny fires when at least one pattern matches (logical OR).
	MatchAny MatchKind = "any"
	// MatchAll fires only when every pattern matches (logical AND).
	MatchAll MatchKind = "all"
)

// Rule is a single heuristic definition.  Rules are evaluated in ascending
// Priority order (lower number = higher priority).
type Rule struct {
	// ID is a unique, stable identifier for the rule (e.g. "auth-cross-contract").
	ID string `json:"id" yaml:"id"`

	// Description is a short human-readable summary of what the rule detects.
	Description string `json:"description" yaml:"description"`

	// Priority controls evaluation order.  Lower values are checked first.
	// Rules with the same priority are evaluated in load order.
	Priority int `json:"priority" yaml:"priority"`

	// Severity is informational metadata: "error", "warning", or "info".
	Severity string `json:"severity" yaml:"severity"`

	// MatchKind is "any" (default) or "all".
	MatchKind MatchKind `json:"match_kind" yaml:"match_kind"`

	// Patterns is a list of case-insensitive substrings or regular expressions
	// that are tested against the combined signal string (error + events + logs).
	// Prefix a pattern with "re:" to treat it as a regular expression.
	Patterns []string `json:"patterns" yaml:"patterns"`

	// RequireEvents, when true, means the rule only fires when at least one
	// DiagnosticEvent is present in the input.
	RequireEvents bool `json:"require_events" yaml:"require_events"`

	// RequireBudgetOver100, when true, means the rule only fires when
	// BudgetUsage reports >= 100 % for the relevant resource.
	RequireBudgetOver100 bool `json:"require_budget_over_100" yaml:"require_budget_over_100"`

	// Template is the Go text/template used to render the suggestion.
	// Available template variables:
	//   .TxHash   – short transaction hash
	//   .Network  – network name
	//   .Caller   – first contract ID extracted from DiagnosticEvents (may be "")
	//   .Callee   – last  contract ID extracted from DiagnosticEvents (may be "")
	//   .Error    – raw error string (sanitized, max 200 chars)
	Template string `json:"template" yaml:"template"`

	// Source is set by the loader to the file path the rule was loaded from.
	// Empty for built-in rules.
	Source string `json:"-" yaml:"-"`

	// compiled holds the pre-compiled regexp patterns (populated by Compile).
	compiled []*regexp.Regexp
}

// Compile pre-compiles all regexp patterns in the rule.  It must be called
// before the rule is used for matching.
func (r *Rule) Compile() error {
	r.compiled = make([]*regexp.Regexp, 0, len(r.Patterns))

	for _, p := range r.Patterns {
		if strings.HasPrefix(p, "re:") {
			expr := strings.TrimPrefix(p, "re:")
			re, err := regexp.Compile("(?i)" + expr)
			if err != nil {
				return fmt.Errorf("rule %q: invalid regexp %q: %w", r.ID, expr, err)
			}
			r.compiled = append(r.compiled, re)
		} else {
			// Plain substring – wrap in a case-insensitive regexp for uniform handling.
			re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(p))
			if err != nil {
				return fmt.Errorf("rule %q: failed to compile pattern %q: %w", r.ID, p, err)
			}
			r.compiled = append(r.compiled, re)
		}
	}
	return nil
}

// Matches reports whether the rule fires for the given combined signal string.
// It does NOT evaluate RequireEvents or RequireBudgetOver100 – those are
// checked by the engine before calling Matches.
func (r *Rule) Matches(combined string) bool {
	if len(r.compiled) == 0 {
		return false
	}

	kind := r.MatchKind
	if kind == "" {
		kind = MatchAny
	}

	switch kind {
	case MatchAll:
		for _, re := range r.compiled {
			if !re.MatchString(combined) {
				return false
			}
		}
		return true
	default: // MatchAny
		for _, re := range r.compiled {
			if re.MatchString(combined) {
				return true
			}
		}
		return false
	}
}

// RuleSet is an ordered collection of compiled rules.
type RuleSet struct {
	rules []*Rule
}

// Rules returns the rules in evaluation order (ascending priority).
func (rs *RuleSet) Rules() []*Rule {
	return rs.rules
}

// RuleFile is the top-level structure of a JSON/YAML rule file.
type RuleFile struct {
	// Version is the schema version of the rule file (currently "1").
	Version string `json:"version" yaml:"version"`
	// Rules is the list of rule definitions.
	Rules []*Rule `json:"rules" yaml:"rules"`
}

// UnmarshalJSON implements json.Unmarshaler so that MatchKind defaults to
// "any" when the field is absent.
func (r *Rule) UnmarshalJSON(data []byte) error {
	// ruleJSON is a plain struct (no methods) that mirrors Rule's JSON fields.
	// Using a separate type avoids the infinite recursion that would occur if
	// we used a type alias of Rule (which inherits the UnmarshalJSON method).
	type ruleJSON struct {
		ID                   string    `json:"id"`
		Description          string    `json:"description"`
		Priority             int       `json:"priority"`
		Severity             string    `json:"severity"`
		MatchKind            string    `json:"match_kind"`
		Patterns             []string  `json:"patterns"`
		RequireEvents        bool      `json:"require_events"`
		RequireBudgetOver100 bool      `json:"require_budget_over_100"`
		Template             string    `json:"template"`
	}
	var raw ruleJSON
	raw.MatchKind = string(MatchAny) // default
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.ID = raw.ID
	r.Description = raw.Description
	r.Priority = raw.Priority
	r.Severity = raw.Severity
	r.MatchKind = MatchKind(raw.MatchKind)
	r.Patterns = raw.Patterns
	r.RequireEvents = raw.RequireEvents
	r.RequireBudgetOver100 = raw.RequireBudgetOver100
	r.Template = raw.Template
	return nil
}
