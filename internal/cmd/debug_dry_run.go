// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dotandev/glassbox/internal/rpc"
	"github.com/spf13/cobra"
)

// runDebugDryRun validates debug command inputs and environment without running
// the simulator or fetching full transaction data for replay.
func runDebugDryRun(cmd *cobra.Command, txHash string) error {
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	var failures []string

	fmt.Fprintf(out, "Dry-run: validating debug parameters (no replay will be executed)\n\n")

	// Transaction hash format
	if err := rpc.ValidateTransactionHash(txHash); err != nil {
		failures = append(failures, fmt.Sprintf("transaction hash: %v", err))
		fmt.Fprintf(errOut, "[FAIL] Invalid transaction hash format: %v\n", err)
	} else {
		fmt.Fprintf(out, "[OK]   Transaction hash format is valid (%d hex chars)\n", len(txHash))
	}

	// Network selection — accept both built-in and custom-configured networks.
	if err := validateNetworkName(networkFlag); err != nil {
		failures = append(failures, fmt.Sprintf("network: %v", err))
		fmt.Fprintf(errOut, "[FAIL] Invalid network %q — must be testnet, mainnet, futurenet, or a custom network defined in config\n", networkFlag)
	} else {
		fmt.Fprintf(out, "[OK]   Network selection: %s\n", networkFlag)
	}

	if compareNetworkFlag != "" {
		if err := validateNetworkName(compareNetworkFlag); err != nil {
			failures = append(failures, fmt.Sprintf("compare-network: %v", err))
			fmt.Fprintf(errOut, "[FAIL] Invalid compare network %q — must be testnet, mainnet, futurenet, or a custom network defined in config\n", compareNetworkFlag)
		} else {
			fmt.Fprintf(out, "[OK]   Compare network: %s\n", compareNetworkFlag)
		}
	}

	// RPC endpoint reachability
	token := rpcTokenFlag
	if token == "" {
		token = os.Getenv("GLASSBOX_RPC_TOKEN")
	}

	opts, err := networkClientOptions(networkFlag)
	if err != nil {
		failures = append(failures, fmt.Sprintf("rpc client: %v", err))
		fmt.Fprintf(errOut, "[FAIL] Failed to build RPC client options for network %q: %v\n", networkFlag, err)
	} else {
		if rpcTokenFlag != "" {
			opts = append(opts, rpc.WithToken(rpcTokenFlag))
		}
		if token != "" && rpcTokenFlag == "" {
			opts = append(opts, rpc.WithToken(token))
		}
		if rpcURLFlag != "" {
			opts = append(opts, rpc.WithHorizonURL(rpcURLFlag))
			fmt.Fprintf(out, "[OK]   Custom RPC URL configured: %s\n", rpcURLFlag)
		}

		client, clientErr := rpc.NewClient(opts...)
		if clientErr != nil {
			failures = append(failures, fmt.Sprintf("rpc client: %v", clientErr))
			fmt.Fprintf(errOut, "[FAIL] Failed to create RPC client: %v\n", clientErr)
		} else {
			probeCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			health, healthErr := client.GetHealth(probeCtx)
			if healthErr != nil {
				failures = append(failures, fmt.Sprintf("endpoint reachability: %v", healthErr))
				fmt.Fprintf(errOut, "[FAIL] RPC endpoint unreachable: %v\n"+
					"       Fix: check your network connection or run 'glassbox doctor' for a full diagnosis\n",
					healthErr)
			} else {
				status := "unknown"
				if health != nil && health.Result.Status != "" {
					status = health.Result.Status
				}
				if status == "unknown" || status == "" {
					// RPC responded but returned no status — treat as a warning, not a pass.
					failures = append(failures, "endpoint reachability: RPC responded but returned no health status")
					fmt.Fprintf(errOut, "[FAIL] RPC endpoint returned an empty health status — the node may be starting up\n"+
						"       Fix: wait for the node to become ready or check its logs\n")
				} else {
					fmt.Fprintf(out, "[OK]   RPC endpoint reachable (status: %s)\n", status)
				}
			}
		}
	}

	// Simulator binary compatibility
	simDep := checkSimulator(verbose)
	if !simDep.Installed {
		failures = append(failures, "simulator: "+simDep.FixHint)
		fmt.Fprintf(errOut, "[FAIL] Simulator binary not found or incompatible\n")
		if simDep.FixHint != "" {
			fmt.Fprintf(errOut, "       Fix: %s\n", simDep.FixHint)
		}
	} else {
		fmt.Fprintf(out, "[OK]   Simulator binary found: %s\n", simDep.Path)
		if simDep.Version != "" {
			fmt.Fprintf(out, "       Version: %s\n", simDep.Version)
		}
	}

	fmt.Fprintln(out)
	if len(failures) > 0 {
		fmt.Fprintf(errOut, "Dry-run FAILED: %d validation error(s)\n", len(failures))
		for i, f := range failures {
			fmt.Fprintf(errOut, "  %d. %s\n", i+1, f)
		}
		return fmt.Errorf("dry-run validation failed with %d error(s)", len(failures))
	}

	fmt.Fprintf(out, "Dry-run PASSED: all checks succeeded for transaction %s on %s\n", txHash, networkFlag)
	return nil
}
