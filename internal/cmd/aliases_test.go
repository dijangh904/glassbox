// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommandAliasesRegistered verifies that ergonomic short aliases are
// registered on the primary commands and that Cobra can dispatch via them.
func TestCommandAliasesRegistered(t *testing.T) {
	tests := []struct {
		primaryName string
		alias       string
	}{
		{"debug", "db"},
		{"profile", "ps"},
		{"protocol:register", "pb:register"},
		{"protocol:unregister", "pb:unregister"},
		{"protocol:status", "pb:status"},
		{"protocol:verify", "pb:verify"},
		{"protocol:handle", "pb:handle"},
		{"protocol:diagnose", "pb:diagnose"},
		{"protocol:repair", "pb:repair"},
	}

	for _, tc := range tests {
		t.Run(tc.primaryName, func(t *testing.T) {
			cmd, _, err := rootCmd.Find([]string{tc.alias})
			require.NoError(t, err, "rootCmd.Find(%q) returned error", tc.alias)
			assert.Equal(t, tc.primaryName, cmd.Name(),
				"alias %q should dispatch to %q, got %q", tc.alias, tc.primaryName, cmd.Name())
		})
	}
}

// TestDebugAliasHasAlias confirms the db alias is set on the debug command.
func TestDebugAliasHasAlias(t *testing.T) {
	assert.True(t, debugCmd.HasAlias("db"), "debugCmd should have alias 'db'")
}

// TestProfileAliasHasAlias confirms the ps alias is set on the profile command.
func TestProfileAliasHasAlias(t *testing.T) {
	assert.True(t, profileCmd.HasAlias("ps"), "profileCmd should have alias 'ps'")
}

// TestProtocolCommandsHavePbAliases verifies every protocol command carries
// the corresponding pb: short alias.
func TestProtocolCommandsHavePbAliases(t *testing.T) {
	tests := []struct {
		cmd   interface{ HasAlias(string) bool }
		alias string
	}{
		{protocolRegisterCmd, "pb:register"},
		{protocolUnregisterCmd, "pb:unregister"},
		{protocolStatusCmd, "pb:status"},
		{protocolVerifyCmd, "pb:verify"},
		{protocolHandlerCmd, "pb:handle"},
		{protocolDiagnoseCmd, "pb:diagnose"},
		{protocolRepairCmd, "pb:repair"},
	}
	for _, tc := range tests {
		assert.True(t, tc.cmd.HasAlias(tc.alias), "expected alias %q to be present", tc.alias)
	}
}

// TestAliasesResolveToSameFunctionality verifies that the debug alias does not
// have its own Run function – it shares the primary command's handler.
func TestAliasesResolveToSameFunctionality(t *testing.T) {
	aliasCmd, _, err := rootCmd.Find([]string{"db"})
	require.NoError(t, err)
	primaryCmd, _, err := rootCmd.Find([]string{"debug"})
	require.NoError(t, err)
	assert.Equal(t, primaryCmd, aliasCmd, "alias 'db' should resolve to the same *cobra.Command as 'debug'")
}
