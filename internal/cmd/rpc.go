// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dotandev/glassbox/internal/config"
	"github.com/spf13/cobra"
)

var (
	rpcHealthURLFlag string
)

var rpcCmd = &cobra.Command{
	Use:     "rpc",
	GroupID: "utility",
	Short:   "Manage and monitor RPC endpoints",
	Long: `Manage and monitor Soroban RPC endpoints.

Glassbox supports multiple Soroban RPC endpoints with adaptive failover.
When more than one endpoint is configured (via rpc_urls or soroban_rpc_urls
in the config file, or GLASSBOX_SOROBAN_RPC_URLS in the environment), the
client automatically routes requests to the healthiest endpoint and fails
over transparently when one becomes unavailable.

Failover strategies (set via failover_strategy in config or
GLASSBOX_FAILOVER_STRATEGY in the environment):

  weighted    Probabilistic selection proportional to health score.
              Healthy endpoints receive more traffic; degraded ones are
              avoided but still occasionally probed for recovery.
              This is the default.

  sticky      Always use the single healthiest endpoint. Only switches
              when the current endpoint is marked degraded.

  round_robin Cycle through all healthy endpoints in order.

Health state is tracked per-endpoint and includes success rate, average
latency, and consecutive failure count. An endpoint is marked degraded
after failure_threshold consecutive failures and re-probed after
retry_timeout seconds.`,
}

var rpcHealthCmd = &cobra.Command{
	Use:     "health",
	Aliases: []string{"rpc:health"},
	Short:   "Check the health of configured RPC endpoints",
	Long: `Check the health of one or more Soroban RPC endpoints.

When multiple endpoints are configured, this command probes each one and
reports its HTTP status, latency, and (if available) the health score
tracked by the adaptive failover layer.

Examples:
  glassbox rpc health
  glassbox rpc health --rpc https://soroban-testnet.stellar.org,https://rpc2.example.com`,
	RunE: func(cmd *cobra.Command, args []string) error {
		urls := []string{}
		cfg, cfgErr := config.Load()
		timeout := 15 * time.Second
		if rpcHealthURLFlag != "" {
			urls = strings.Split(rpcHealthURLFlag, ",")
		} else {
			if cfgErr == nil {
				// Prefer soroban_rpc_urls, then rpc_urls, then rpc_url.
				if len(cfg.SorobanRpcUrls) > 0 {
					urls = cfg.SorobanRpcUrls
				} else if len(cfg.RpcUrls) > 0 {
					urls = cfg.RpcUrls
				} else if cfg.RpcUrl != "" {
					urls = []string{cfg.RpcUrl}
				}
				if cfg.RequestTimeout > 0 {
					timeout = time.Duration(cfg.RequestTimeout) * time.Second
				}
			}
		}

		if len(urls) == 0 {
			return fmt.Errorf("no RPC URLs configured and none provided via --rpc")
		}

		fmt.Println("[STATS] RPC Endpoint Status:")
		fmt.Println()

		client := &http.Client{Timeout: timeout}

		for i, url := range urls {
			url = strings.TrimSpace(url)
			if url == "" {
				continue
			}

			start := time.Now()
			status := "[OK]"
			success := true
			errStr := ""

			resp, err := client.Get(url)
			if err != nil {
				status = "[FAIL]"
				success = false
				errStr = err.Error()
			} else {
				defer resp.Body.Close()
				if resp.StatusCode >= 400 {
					status = "[FAIL]"
					success = false
					errStr = fmt.Sprintf("HTTP %d", resp.StatusCode)
				}
			}

			duration := time.Since(start)

			if success {
				fmt.Printf("  [%d]  %s\n", i+1, url)
				fmt.Printf("      Status:  %s\n", status)
				fmt.Printf("      Latency: %v\n", duration.Round(time.Millisecond))
			} else {
				fmt.Printf("  [%d] %s %s\n", i+1, status, url)
				fmt.Printf("      Error: %s\n", errStr)
			}
			fmt.Println()
		}

		if len(urls) > 1 {
			fmt.Println("  Tip: configure failover_strategy in your config file to control")
			fmt.Println("  how Glassbox selects among these endpoints (weighted/sticky/round_robin).")
			fmt.Println()
		}

		return nil
	},
}

func init() {
	rpcHealthCmd.Flags().StringVar(&rpcHealthURLFlag, "rpc", "", "RPC URLs to check (comma-separated)")
	rpcCmd.AddCommand(rpcHealthCmd)

	// Add the rpc:health as a top-level command for compatibility
	rpcHealthAliasCmd := *rpcHealthCmd
	rpcHealthAliasCmd.Use = "rpc:health"
	rpcHealthAliasCmd.Hidden = true
	rootCmd.AddCommand(&rpcHealthAliasCmd)

	rootCmd.AddCommand(rpcCmd)
}
