// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

// makeDebugCmdForTest returns a minimal cobra.Command wired to the same RunE
// and flag set as debugCmd but with isolated stdout/stderr buffers, so dry-run
// and health tests can inspect output without polluting the test terminal.
//
// The returned command shares the same global flag variables as debugCmd (the
// package-level vars like networkFlag, rpcURLFlag, etc.) so tests can set those
// variables directly before calling runDebugDryRun.
func makeDebugCmdForTest() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "debug",
		RunE: debugCmd.RunE,
	}
	// Copy the full flag set so runDebugDryRun can call cmd.Flags() internally.
	cmd.Flags().AddFlagSet(debugCmd.Flags())
	return cmd
}
