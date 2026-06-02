// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/dotandev/glassbox/internal/clioutput"
	"github.com/dotandev/glassbox/internal/protocolreg"
	"github.com/spf13/cobra"
)

// protocolDiagnoseJSON controls whether protocol:diagnose emits JSON output.
var protocolDiagnoseJSON bool

// protocolVerifyProbe runs an optional glassbox://doctor-probe handler check.
var protocolVerifyProbe bool

var protocolRegisterCmd = &cobra.Command{
	Use:     "protocol:register",
	Aliases: []string{"pb:register"},
	Short:   "Register the glassbox:// protocol handler in the operating system",
	GroupID: "utility",
	RunE: func(cmd *cobra.Command, args []string) error {
		registrar, err := protocolreg.NewRegistrar()
		if err != nil {
			return err
		}
		if err := registrar.Register(); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Registered GLASSBOX Protocol handler for %s://\n", protocolreg.Scheme)
		return nil
	},
}

var protocolUnregisterCmd = &cobra.Command{
	Use:     "protocol:unregister",
	Aliases: []string{"pb:unregister"},
	Short:   "Unregister the glassbox:// protocol handler from the operating system",
	GroupID: "utility",
	RunE: func(cmd *cobra.Command, args []string) error {
		registrar, err := protocolreg.NewRegistrar()
		if err != nil {
			return err
		}
		if err := registrar.Unregister(); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Unregistered GLASSBOX Protocol handler")
		return nil
	},
}

var protocolStatusCmd = &cobra.Command{
	Use:     "protocol:status",
	Aliases: []string{"pb:status"},
	Short:   "Check current registration status of the glassbox:// protocol handler",
	GroupID: "utility",
	RunE: func(cmd *cobra.Command, args []string) error {
		registrar, err := protocolreg.NewRegistrar()
		if err != nil {
			return err
		}

		if registrar.IsRegistered() {
			fmt.Fprintln(cmd.OutOrStdout(), "GLASSBOX Protocol handler is currently REGISTERED")
			return nil
		}

		return fmt.Errorf("GLASSBOX Protocol handler is NOT REGISTERED")
	},
}

var protocolVerifyCmd = &cobra.Command{
	Use:     "protocol:verify",
	Aliases: []string{"pb:verify", "verify-protocol-registration"},
	Short:   "Verify the native OS registration for the glassbox:// protocol handler",
	Long: `Verify that the glassbox:// protocol handler is registered correctly on this system.

The command inspects OS registration artefacts and reports pass/fail for each check.
Use --probe to simulate a glassbox://doctor-probe deep link and confirm the handler
responds without side effects.

On failure, remediation steps are printed to help repair the registration.`,
	GroupID: "utility",
	RunE: func(cmd *cobra.Command, args []string) error {
		registrar, err := protocolreg.NewRegistrar()
		if err != nil {
			return err
		}

		report, err := registrar.Verify()
		for _, check := range report.Checks {
			fmt.Fprintf(cmd.OutOrStdout(), "[OK] %s\n", check)
		}
		for _, issue := range report.Issues {
			fmt.Fprintf(cmd.ErrOrStderr(), "[FAIL] %s\n", issue)
		}

		probePassed := true
		if protocolVerifyProbe {
			exePath, exeErr := os.Executable()
			if exeErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "[FAIL] Handler probe: cannot resolve executable: %v\n", exeErr)
				probePassed = false
			} else if deeplink.ProbeHandler(exePath) {
				fmt.Fprintf(cmd.OutOrStdout(), "[OK] Handler probe (%s) exited cleanly\n", deeplink.MockURL)
			} else {
				probePassed = false
				fmt.Fprintf(cmd.ErrOrStderr(), "[FAIL] Handler probe (%s) did not complete successfully\n", deeplink.MockURL)
				fmt.Fprintf(cmd.ErrOrStderr(), "       Ensure the binary handles --deep-link %s\n", deeplink.MockURL)
			}
		}

		if err != nil || !probePassed {
			diag := registrar.Diagnose()
			if len(diag.RemediationSteps) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "\nRemediation steps:\n")
				for i, step := range diag.RemediationSteps {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %d. %s\n", i+1, step)
				}
			}
		}

		if err != nil {
			return err
		}
		if !probePassed {
			return fmt.Errorf("protocol handler probe failed")
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Verified GLASSBOX Protocol registration on %s\n", report.Platform)
		return nil
	},
}

var protocolHandlerCmd = &cobra.Command{
	Use:     "protocol:handle <uri>",
	Aliases: []string{"protocol-handler", "pb:handle"},
	Short:   "Handle an glassbox:// protocol URI and dispatch it to the debugger",
	GroupID: "utility",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parsed, err := protocolreg.ParseDebugURI(args[0])
		if err != nil {
			return err
		}

		executablePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable path: %w", err)
		}

		debugArgs := []string{"debug", parsed.TransactionHash, "--network", parsed.Network}

		// Forward the operation index when present (prefer Op, fall back to Operation).
		opIdx := parsed.Op
		if opIdx == nil {
			opIdx = parsed.Operation
		}
		if opIdx != nil {
			debugArgs = append(debugArgs, "--op", fmt.Sprintf("%d", *opIdx))
		}

		// Forward the view mode when present.
		if parsed.View != "" {
			debugArgs = append(debugArgs, "--view", parsed.View)
		}

		child := exec.CommandContext(cmd.Context(), executablePath, debugArgs...)
		child.Stdout = cmd.OutOrStdout()
		child.Stderr = cmd.ErrOrStderr()
		return child.Run()
	},
}

var protocolDiagnoseCmd = &cobra.Command{
	Use:     "protocol:diagnose",
	Aliases: []string{"pb:diagnose"},
	Short:   "Inspect the glassbox:// protocol registration and report root causes",
	GroupID: "utility",
	Long: `Inspect the glassbox:// protocol registration on the current OS and produce
a structured diagnostic report.

The command checks:
  • Whether the protocol handler is registered with the OS
  • Whether the registered handler points to the current executable
  • Platform-specific registration artefacts (.desktop file, registry key, app bundle)
  • xdg-mime / LaunchServices / registry consistency

Exit codes:
  0  — registration is healthy
  1  — registration is missing or broken (issues are printed to stderr)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		registrar, err := protocolreg.NewRegistrar()
		if err != nil {
			return err
		}

		report := registrar.Diagnose()

		if protocolDiagnoseJSON {
			return clioutput.Write(cmd.OutOrStdout(), "protocol:diagnose", report)
		}

		for _, check := range report.Checks {
			fmt.Fprintf(cmd.OutOrStdout(), "[OK]   %s\n", check)
		}
		for _, issue := range report.Issues {
			fmt.Fprintf(cmd.ErrOrStderr(), "[FAIL] %s\n", issue)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\nStatus: %s  (platform: %s, scheme: %s://)\n",
			report.Status, report.Platform, report.Scheme)

		if report.RegisteredHandler != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Registered handler: %s\n", report.RegisteredHandler)
			if report.HandlerMatchesSelf {
				fmt.Fprintf(cmd.OutOrStdout(), "Handler matches current executable: yes\n")
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Handler matches current executable: NO (stale path)\n")
			}
		}

		if len(report.RemediationSteps) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "\nRemediation steps:\n")
			for i, step := range report.RemediationSteps {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %d. %s\n", i+1, step)
			}
		}

		if report.Status != protocolreg.StatusOK {
			return fmt.Errorf("protocol registration is %s", report.Status)
		}
		return nil
	},
}

var protocolRepairCmd = &cobra.Command{
	Use:     "protocol:repair",
	Aliases: []string{"pb:repair"},
	Short:   "Repair a broken or missing glassbox:// protocol registration",
	GroupID: "utility",
	Long: `Attempt to repair the glassbox:// protocol handler registration.

The command first runs a diagnostic check to understand what is broken, then
re-registers the protocol handler using the best available platform mechanism:

  • Linux  — recreates the .desktop file and updates xdg-mime
  • macOS  — rebuilds the app bundle and re-registers with LaunchServices
  • Windows — updates the HKEY_CURRENT_USER registry keys

After repair, a verification pass confirms the fix was successful.

PERMISSION NOTES
  On Windows, registry writes to HKEY_CURRENT_USER do not require elevation.
  On Linux and macOS, the handler is installed per-user (~/.local/share or
  ~/Applications) and does not require root.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		registrar, err := protocolreg.NewRegistrar()
		if err != nil {
			return err
		}

		result := registrar.Repair()

		for _, action := range result.Actions {
			fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", action)
		}

		if result.PermissionHint != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "\nPermission hint: %s\n", result.PermissionHint)
		}

		if result.Err != nil {
			return result.Err
		}

		if result.Repaired {
			fmt.Fprintf(cmd.OutOrStdout(), "\nRepair successful. The %s:// protocol handler is now registered.\n",
				protocolreg.Scheme)
		}
		return nil
	},
}

func init() {
	protocolDiagnoseCmd.Flags().BoolVar(&protocolDiagnoseJSON, "json", false,
		"Emit diagnostic report as JSON (for machine consumption)")
	protocolVerifyCmd.Flags().BoolVar(&protocolVerifyProbe, "probe", false,
		"Run a dry-run glassbox://doctor-probe handler check after registration verification")

	rootCmd.AddCommand(protocolRegisterCmd)
	rootCmd.AddCommand(protocolUnregisterCmd)
	rootCmd.AddCommand(protocolStatusCmd)
	rootCmd.AddCommand(protocolVerifyCmd)
	rootCmd.AddCommand(protocolHandlerCmd)
	rootCmd.AddCommand(protocolDiagnoseCmd)
	rootCmd.AddCommand(protocolRepairCmd)
}
