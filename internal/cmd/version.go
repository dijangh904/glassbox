// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/dotandev/glassbox/internal/errors"
	"github.com/dotandev/glassbox/internal/version"
	"github.com/spf13/cobra"
)

// VersionInfo holds all metadata surfaced by the `version` command.
// Every field that is "unknown" at build time is labeled clearly so users
// know whether the binary was properly stamped.
type VersionInfo struct {
	Version   string `json:"version"`
	CommitSHA string `json:"commit_sha"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	// IsDev is true when the binary was built without ldflags version injection.
	IsDev bool `json:"is_dev,omitempty"`
	// UserAgent is the formatted string used in RPC / diagnostic headers.
	UserAgent string `json:"user_agent"`
}

// versionCmd represents the version command.
var versionCmd = &cobra.Command{
	Use:     "version",
	GroupID: "utility",
	Short:   "Show version information",
	Long: `Display detailed build information for this Glassbox binary.

Includes the release version, git commit SHA, build date, and Go toolchain
version. Use --json to obtain machine-readable output.

When the binary was not built with proper ldflags (e.g. 'go run ./...'),
fields will show "0.0.0-dev" or "unknown" and is_dev will be true.`,
	Example: `  # Human-readable output
  glassbox version

  # Machine-readable JSON (useful in CI / scripts)
  glassbox version --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")

		info := getVersionInfo()

		if jsonOutput {
			output, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				return errors.WrapMarshalFailed(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(output))
			return nil
		}

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Glassbox Version: %s", info.Version)
		if info.IsDev {
			fmt.Fprint(out, "  (dev build — version not stamped via ldflags)")
		}
		fmt.Fprintln(out)
		fmt.Fprintf(out, "Commit SHA:       %s\n", info.CommitSHA)
		fmt.Fprintf(out, "Build Date:       %s\n", info.BuildDate)
		fmt.Fprintf(out, "Go Version:       %s\n", info.GoVersion)
		fmt.Fprintf(out, "User-Agent:       %s\n", info.UserAgent)
		return nil
	},
}

// getVersionInfo assembles VersionInfo, falling back to runtime/debug build
// info when ldflags were not supplied (e.g. during local development).
func getVersionInfo() VersionInfo {
	info := VersionInfo{
		Version:   version.Version,
		CommitSHA: version.CommitSHA,
		BuildDate: version.BuildDate,
		GoVersion: "unknown",
		IsDev:     version.IsDev(),
		UserAgent: version.UserAgent(),
	}

	// Use runtime/debug as fallback for fields not set by ldflags.
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		info.GoVersion = buildInfo.GoVersion

		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.CommitSHA == "unknown" && setting.Value != "" {
					info.CommitSHA = setting.Value
					// Refresh UserAgent now that CommitSHA is resolved.
					info.UserAgent = fmt.Sprintf("glassbox/%s (%s)", info.Version, version.ShortSHA())
				}
			case "vcs.time":
				if info.BuildDate == "unknown" && setting.Value != "" {
					if t, err := time.Parse(time.RFC3339, setting.Value); err == nil {
						info.BuildDate = t.Format("2006-01-02 15:04:05 UTC")
					}
				}
			}
		}
	}

	return info
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().Bool("json", false, "Output version information as machine-readable JSON")
}
