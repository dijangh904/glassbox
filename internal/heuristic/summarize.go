// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package heuristic

import (
	"fmt"
	"strings"

	"github.com/dotandev/glassbox/internal/simulator"
)

// Input collects all available signals about a transaction execution.
type Input struct {
	TxHash           string
	Network          string
	Status           string
	Error            string
	Events           []string
	Logs             []string
	DiagnosticEvents []simulator.DiagnosticEvent
	BudgetUsage      *simulator.BudgetUsage
}

// Summarize returns a single-paragraph plain-English explanation of why the
// transaction executed as it did.  For failed transactions the default engine
// (built-in rules) is used.  Call SetDefaultEngine to override with custom
// rules loaded from external files.
func Summarize(in Input) string {
	if in.Status == "success" {
		return fmt.Sprintf(
			"Transaction %s executed successfully on %s with no detected errors.",
			shortHash(in.TxHash), in.Network,
		)
	}
	return defaultEngine.Evaluate(in)
}

// extractCallerCallee returns the last two distinct contract IDs encountered in
// diagnostic events, treating the earlier one as the caller and the later one as
// the callee that triggered the failure.
func extractCallerCallee(events []simulator.DiagnosticEvent) (caller, callee string) {
	seen := make([]string, 0, 4)
	dedup := make(map[string]struct{})
	for _, e := range events {
		if e.ContractID == nil {
			continue
		}
		id := *e.ContractID
		if _, ok := dedup[id]; !ok {
			dedup[id] = struct{}{}
			seen = append(seen, id)
		}
	}
	switch len(seen) {
	case 0:
		return "", ""
	case 1:
		return "", seen[0]
	default:
		return seen[len(seen)-2], seen[len(seen)-1]
	}
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:6] + "..." + hash[len(hash)-6:]
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
