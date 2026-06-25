// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strings"
	"testing"
)

// TestCommandExamples verifies that key subcommands expose non-empty Example
// fields so that `--help` output includes representative usage guidance.
func TestCommandExamples(t *testing.T) {
	cases := []struct {
		name    string
		example string
	}{
		{"trace", traceCmd.Example},
		{"compare", compareCmd.Example},
		{"heuristic", heuristicCmd.Example},
		{"debug", debugCmd.Example},
		{"session", sessionCmd.Example},
		{"cache", cacheCmd.Example},
		{"report", reportCmd.Example},
		{"regression-test", regressionTestCmd.Example},
		{"version", versionCmd.Example},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.TrimSpace(tc.example) == "" {
				t.Errorf("command %q has no Example field; add usage examples to help reduce onboarding friction", tc.name)
			}
		})
	}
}

// TestDryRunCommandExample verifies the dry-run subcommand has examples.
func TestDryRunCommandExample(t *testing.T) {
	if strings.TrimSpace(dryRunCmd.Example) == "" {
		t.Error("dry-run command has no Example field")
	}
}

// TestCommandExampleContent verifies that examples reference real flag names.
func TestCommandExampleContent(t *testing.T) {
	cases := []struct {
		name        string
		example     string
		mustContain []string
	}{
		{
			name:        "trace",
			example:     traceCmd.Example,
			mustContain: []string{"glassbox trace", "--print", "--theme"},
		},
		{
			name:        "compare",
			example:     compareCmd.Example,
			mustContain: []string{"glassbox compare", "--wasm", "--network"},
		},
		{
			name:        "dry-run",
			example:     dryRunCmd.Example,
			mustContain: []string{"glassbox dry-run", "--network"},
		},
		{
			name:        "heuristic",
			example:     heuristicCmd.Example,
			mustContain: []string{"glassbox heuristic list", "glassbox heuristic validate"},
		},
		{
			name:        "report",
			example:     reportCmd.Example,
			mustContain: []string{"glassbox report", "--file", "--format"},
		},
		{
			name:        "regression-test",
			example:     regressionTestCmd.Example,
			mustContain: []string{"glassbox regression-test", "--count", "--workers"},
		},
		{
			name:        "version",
			example:     versionCmd.Example,
			mustContain: []string{"glassbox version", "--json"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, substr := range tc.mustContain {
				if !strings.Contains(tc.example, substr) {
					t.Errorf("command %q Example missing expected content %q", tc.name, substr)
				}
			}
		})
	}
}

// TestCommandLongDescriptions verifies that the key improved commands have
// non-empty Long descriptions that mention validation behavior.
func TestCommandLongDescriptions(t *testing.T) {
	cases := []struct {
		name        string
		long        string
		mustContain []string
	}{
		{
			name:        "regression-test",
			long:        regressionTestCmd.Long,
			mustContain: []string{"--count", "--network"},
		},
		{
			name:        "report",
			long:        reportCmd.Long,
			mustContain: []string{"--file", "--format"},
		},
		{
			name:        "version",
			long:        versionCmd.Long,
			mustContain: []string{"--json"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.TrimSpace(tc.long) == "" {
				t.Errorf("command %q has no Long description", tc.name)
			}
			for _, substr := range tc.mustContain {
				if !strings.Contains(tc.long, substr) {
					t.Errorf("command %q Long description missing %q", tc.name, substr)
				}
			}
		})
	}
}
