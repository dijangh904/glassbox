// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/dotandev/glassbox/internal/errors"
	"github.com/dotandev/glassbox/internal/plugin"
	"github.com/spf13/cobra"
)

var (
	pluginDirFlag      string
	pluginManifestFlag string
)

// pluginCmd is the parent command for plugin management.
var pluginCmd = &cobra.Command{
	Use:     "plugin",
	GroupID: "development",
	Short:   "Manage Glassbox plugins",
	Long: `Discover, register, and inspect Glassbox plugins.

Plugins extend Glassbox with custom decoders, analyzers, trace viewers, and
artifact loaders. Each plugin is described by a plugin.json manifest file that
declares its name, version, capabilities, and required permissions.

Plugin isolation:
  Plugins run in sandboxed child processes. A crashing or misbehaving plugin
  cannot corrupt the main Glassbox process.

Examples:
  # List all plugins in the default plugins directory
  glassbox plugin list

  # List plugins from a custom directory
  glassbox plugin list --dir /path/to/plugins

  # Register a single plugin from its manifest
  glassbox plugin register --manifest /path/to/plugin.json

  # Inspect a plugin's manifest
  glassbox plugin inspect my-plugin --dir /path/to/plugins`,
}

// pluginListCmd lists all discovered plugins.
var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available plugins",
	Long:  `Scan the plugins directory and display all discovered plugin manifests.`,
	RunE:  runPluginList,
}

// pluginRegisterCmd registers a plugin from an explicit manifest path.
var pluginRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a plugin from a manifest file",
	Long: `Register a single plugin by pointing directly at its plugin.json manifest.
The plugin binary referenced by the manifest must exist and be executable.`,
	RunE: runPluginRegister,
}

// pluginInspectCmd shows detailed information about a specific plugin.
var pluginInspectCmd = &cobra.Command{
	Use:   "inspect <plugin-name>",
	Short: "Inspect a plugin's manifest and capabilities",
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginInspect,
}

// pluginValidateCmd validates a plugin manifest without registering it.
var pluginValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a plugin manifest file",
	Long: `Parse and validate a plugin.json manifest file without registering
or loading the plugin binary. Useful for CI pipelines and pre-submission checks.

Exits 0 when the manifest is valid, non-zero otherwise.`,
	RunE: runPluginValidate,
}

// pluginCapabilitiesCmd lists the capabilities declared by one or all plugins.
var pluginCapabilitiesCmd = &cobra.Command{
	Use:   "capabilities [plugin-name]",
	Short: "List capabilities declared by plugins",
	Long: `List the capabilities and permissions declared by all discovered plugins,
or by a single named plugin when a name argument is provided.

Exit 1 is returned when no plugins are found.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPluginCapabilities,
}

func init() {
	pluginListCmd.Flags().StringVar(&pluginDirFlag, "dir", "", "Plugin directory to scan (default: ./plugins)")
	pluginRegisterCmd.Flags().StringVar(&pluginManifestFlag, "manifest", "", "Path to the plugin.json manifest file")
	pluginInspectCmd.Flags().StringVar(&pluginDirFlag, "dir", "", "Plugin directory to scan (default: ./plugins)")
	pluginValidateCmd.Flags().StringVar(&pluginManifestFlag, "manifest", "", "Path to the plugin.json manifest file to validate")
	pluginCapabilitiesCmd.Flags().StringVar(&pluginDirFlag, "dir", "", "Plugin directory to scan (default: ./plugins)")

	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginRegisterCmd)
	pluginCmd.AddCommand(pluginInspectCmd)
	pluginCmd.AddCommand(pluginValidateCmd)
	pluginCmd.AddCommand(pluginCapabilitiesCmd)
	rootCmd.AddCommand(pluginCmd)
}

func runPluginList(cmd *cobra.Command, args []string) error {
	dir := resolvePluginDir(pluginDirFlag)

	manifests, errs := plugin.DiscoverManifests(dir)

	// Print discovery errors as warnings but continue.
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", e)
	}

	if len(manifests) == 0 {
		fmt.Printf("No plugins found in %s\n", dir)
		fmt.Println("Create a subdirectory with a plugin.json manifest to register a plugin.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tAPI\tCAPABILITIES\tDESCRIPTION")
	fmt.Fprintln(w, "----\t-------\t---\t------------\t-----------")

	for _, m := range manifests {
		caps := capabilitiesString(m.Capabilities)
		desc := m.Description
		if desc == "" {
			desc = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			m.Name, m.Version, m.APIVersion, caps, desc)
	}
	_ = w.Flush()

	fmt.Printf("\n%d plugin(s) found in %s\n", len(manifests), dir)
	return nil
}

func runPluginRegister(cmd *cobra.Command, args []string) error {
	if pluginManifestFlag == "" {
		return errors.WrapCliArgumentRequired("manifest")
	}

	absPath, err := filepath.Abs(pluginManifestFlag)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("invalid manifest path: %v", err))
	}

	m, err := plugin.LoadManifest(absPath)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to load manifest: %v", err))
	}

	fmt.Printf("Plugin manifest validated:\n")
	fmt.Printf("  Name:         %s\n", m.Name)
	fmt.Printf("  Version:      %s\n", m.Version)
	fmt.Printf("  API Version:  %s\n", m.APIVersion)
	fmt.Printf("  Capabilities: %s\n", capabilitiesString(m.Capabilities))
	if len(m.Permissions) > 0 {
		fmt.Printf("  Permissions:  %s\n", permissionsString(m.Permissions))
	}
	if m.Description != "" {
		fmt.Printf("  Description:  %s\n", m.Description)
	}

	// Verify the binary exists.
	manifestDir := filepath.Dir(absPath)
	binaryPath := m.Entrypoint
	if !filepath.IsAbs(binaryPath) {
		binaryPath = filepath.Join(manifestDir, binaryPath)
	}
	if _, err := os.Stat(binaryPath); err != nil {
		return errors.WrapValidationError(
			fmt.Sprintf("plugin binary not found at %s: %v", binaryPath, err),
		)
	}

	fmt.Printf("\nPlugin %q registered successfully.\n", m.Name)
	fmt.Println("Use 'glassbox plugin list' to see all registered plugins.")
	return nil
}

func runPluginInspect(cmd *cobra.Command, args []string) error {
	pluginName := args[0]
	dir := resolvePluginDir(pluginDirFlag)

	manifestPath := filepath.Join(dir, pluginName, plugin.ManifestFileName)
	m, err := plugin.LoadManifest(manifestPath)
	if err != nil {
		return errors.WrapValidationError(
			fmt.Sprintf("failed to load manifest for plugin %q: %v", pluginName, err),
		)
	}

	fmt.Printf("Plugin: %s\n", m.Name)
	fmt.Printf("  Version:      %s\n", m.Version)
	fmt.Printf("  API Version:  %s\n", m.APIVersion)
	fmt.Printf("  Entrypoint:   %s\n", m.Entrypoint)
	if m.Description != "" {
		fmt.Printf("  Description:  %s\n", m.Description)
	}
	if m.Author != "" {
		fmt.Printf("  Author:       %s\n", m.Author)
	}
	fmt.Printf("  Capabilities:\n")
	for _, cap := range m.Capabilities {
		fmt.Printf("    - %s\n", cap)
	}
	if len(m.Permissions) > 0 {
		fmt.Printf("  Permissions:\n")
		for _, perm := range m.Permissions {
			fmt.Printf("    - %s\n", perm)
		}
	}
	if len(m.EventTypes) > 0 {
		fmt.Printf("  Event Types:\n")
		for _, et := range m.EventTypes {
			fmt.Printf("    - %s\n", et)
		}
	}
	if m.Checksum != "" {
		fmt.Printf("  Checksum:     %s\n", m.Checksum)
	}
	return nil
}

func runPluginValidate(cmd *cobra.Command, args []string) error {
	if pluginManifestFlag == "" {
		return errors.WrapCliArgumentRequired("manifest")
	}

	absPath, err := filepath.Abs(pluginManifestFlag)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("invalid manifest path: %v", err))
	}

	m, err := plugin.LoadManifest(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "INVALID: %v\n", err)
		return errors.WrapValidationError(fmt.Sprintf("manifest validation failed: %v", err))
	}

	fmt.Printf("VALID: manifest for plugin %q (v%s) passed all checks.\n", m.Name, m.Version)
	if m.GlassboxVersionRange != "" {
		fmt.Printf("  Glassbox version range: %s\n", m.GlassboxVersionRange)
	}
	if m.TrustLevel != "" {
		fmt.Printf("  Trust level: %s\n", m.TrustLevel)
	}
	return nil
}

func runPluginCapabilities(cmd *cobra.Command, args []string) error {
	dir := resolvePluginDir(pluginDirFlag)

	if len(args) == 1 {
		// Single plugin lookup.
		pluginName := args[0]
		manifestPath := filepath.Join(dir, pluginName, plugin.ManifestFileName)
		m, err := plugin.LoadManifest(manifestPath)
		if err != nil {
			return errors.WrapValidationError(
				fmt.Sprintf("failed to load manifest for plugin %q: %v", pluginName, err),
			)
		}
		printPluginCapabilities(m)
		return nil
	}

	manifests, errs := plugin.DiscoverManifests(dir)
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", e)
	}

	if len(manifests) == 0 {
		fmt.Printf("No plugins found in %s\n", dir)
		return errors.WrapValidationError("no plugins found")
	}

	for _, m := range manifests {
		printPluginCapabilities(m)
		fmt.Println()
	}
	return nil
}

// printPluginCapabilities prints the capability and permission summary for a manifest.
func printPluginCapabilities(m *plugin.Manifest) {
	fmt.Printf("Plugin: %s (v%s)\n", m.Name, m.Version)
	fmt.Printf("  Trust Level:  %s\n", trustLevelLabel(m.TrustLevel))
	if m.GlassboxVersionRange != "" {
		fmt.Printf("  Glassbox Range: %s\n", m.GlassboxVersionRange)
	}
	fmt.Printf("  Capabilities:\n")
	for _, cap := range m.Capabilities {
		fmt.Printf("    - %s\n", cap)
	}
	if len(m.Permissions) > 0 {
		fmt.Printf("  Permissions:\n")
		for _, perm := range m.Permissions {
			fmt.Printf("    - %s\n", perm)
		}
	} else {
		fmt.Printf("  Permissions: (none)\n")
	}
}

// trustLevelLabel returns a display string for a trust level, including default.
func trustLevelLabel(tl plugin.TrustLevel) string {
	if tl == "" {
		return string(plugin.TrustLevelUntrusted) + " (default)"
	}
	return string(tl)
}

// resolvePluginDir returns the plugin directory to use, defaulting to ./plugins.
func resolvePluginDir(flag string) string {
	if flag != "" {
		return flag
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "plugins"
	}
	return filepath.Join(cwd, "plugins")
}

// capabilitiesString formats a slice of capabilities as a comma-separated string.
func capabilitiesString(caps []plugin.Capability) string {
	if len(caps) == 0 {
		return "-"
	}
	s := ""
	for i, c := range caps {
		if i > 0 {
			s += ","
		}
		s += string(c)
	}
	return s
}

// permissionsString formats a slice of permissions as a comma-separated string.
func permissionsString(perms []plugin.Permission) string {
	if len(perms) == 0 {
		return "-"
	}
	s := ""
	for i, p := range perms {
		if i > 0 {
			s += ","
		}
		s += string(p)
	}
	return s
}
