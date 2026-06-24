# `glassbox debug` — Debug Command Reference

The `debug` command fetches a Soroban transaction from the Stellar network, runs it through the local simulator, and displays a detailed execution trace including contract events, token flows, budget usage, and security findings.

---

## Synopsis

```
glassbox debug [flags] <transaction-hash>
glassbox debug --wasm <path> [--args ...]
glassbox debug --demo
glassbox debug --dry-run --network testnet <transaction-hash>
glassbox debug --xdr-file <path>
glassbox debug --json-file <path>
glassbox debug --load-snapshots <registry-file>
```

---

## Arguments

| Argument | Description |
|---|---|
| `<transaction-hash>` | 64-character lowercase hex transaction hash. Required unless `--wasm`, `--demo`, `--xdr-file`, `--json-file`, or `--load-snapshots` is provided. |

**Validation:** The command validates the transaction hash format before making any network calls. An invalid hash (wrong length, non-hex characters) produces an explicit error with the expected format and the actual input.

---

## Core Flags

| Flag | Default | Description |
|---|---|---|
| `--network`, `-n` | `mainnet` | Stellar network: `testnet`, `mainnet`, or `futurenet`. Auto-detected from the transaction when omitted. |
| `--rpc-url` | _(config)_ | Custom RPC URL. Overrides config and environment. Accepts comma-separated URLs for fallback. |
| `--rpc-token` | _(env: `GLASSBOX_RPC_TOKEN`)_ | RPC authentication token. |
| `--compare-network` | _(none)_ | Run the same transaction on a second network and diff the results. |
| `--network` and `--compare-network` must be different networks.

---

## Validation & Dry-Run

`--dry-run` validates inputs and checks the environment **without executing a simulation**. Use it in CI or before a long replay to catch configuration errors early.

**Checks performed by `--dry-run`:**

1. Transaction hash format (64 hex chars)
2. Network name validity (`testnet`, `mainnet`, `futurenet`)
3. Compare-network name validity (when `--compare-network` is set)
4. RPC endpoint reachability (health check with a 10-second timeout)
5. Simulator binary presence and version compatibility

**Example:**

```sh
# Validate everything before committing to a full replay
glassbox debug --dry-run --network testnet 5c0a1234...ef7890ab

# Output on success:
# [OK]   Transaction hash format is valid (64 hex chars)
# [OK]   Network selection: testnet
# [OK]   RPC endpoint reachable (status: healthy)
# [OK]   Simulator binary found: /usr/local/bin/glassbox-sim
# Dry-run PASSED: all checks succeeded for transaction 5c0a1234... on testnet
```

**Exit code:** `0` on pass, `1` on any validation failure with a numbered list of all failures.

---

## Local Replay Modes

### WASM replay (no network required)

```sh
glassbox debug --wasm ./contract.wasm --args "arg1" "arg2"
```

Runs the contract locally with mock ledger state. Useful for rapid iteration during development.

### Hot reload

```sh
glassbox debug --wasm ./contract.wasm --hot-reload
```

Watches the WASM file for changes and prompts to re-run after each rebuild. Requires `--wasm`.

### Local envelope file

```sh
# From a raw base64 XDR file:
glassbox debug --xdr-file ./tx-envelope.xdr

# From a structured JSON export:
glassbox debug --json-file ./tx.json
```

The JSON format must contain an `envelope_xdr` field. Optionally include `result_meta_xdr` and `network`.

### Offline snapshot replay

```sh
glassbox debug --load-snapshots ./tx-registry.json
```

Replays a previously saved snapshot registry without any network connectivity. See [snapshot-deduplication.md](./snapshot-deduplication.md).

---

## Output Flags

| Flag | Default | Description |
|---|---|---|
| `--json` | `false` | Emit simulation results as machine-readable JSON. |
| `--format` | `text` | Output format: `text` or `json`. |
| `--trace-verbosity` | `normal` | Trace detail level: `summary`, `normal`, or `verbose`. |
| `--export-svg` | _(none)_ | Export the call graph as an SVG file. |
| `--show-metrics` | `false` | Print RPC and simulation performance metrics after the run. |
| `--verbose`, `-v` | `false` | Enable verbose logging (equivalent to `--log-level=debug`). |

---

## Simulation Control Flags

| Flag | Default | Description |
|---|---|---|
| `--snapshot` | _(none)_ | Load pre-captured ledger state from a JSON snapshot instead of fetching from the network. |
| `--live` / `--latest-ledger` | `false` | Replay against the current validated ledger state (live data). |
| `--protocol-version` | _(auto)_ | Override the Soroban protocol version for simulation. |
| `--mock-time` | `0` | Override the ledger timestamp (Unix seconds). |
| `--mock-base-fee` | `0` | Override the base fee (stroops) for fee sufficiency checks. |
| `--mock-gas-price` | `0` | Override the gas price multiplier. |
| `--mock-ledger-entry` | _(none)_ | Override individual ledger entries before simulation (`key:value`; repeatable). |
| `--mock-ledger-manifest` | _(none)_ | Path to a JSON manifest containing `ledger_entries` for bulk override. |
| `--op` / `--operation` | `-1` (all) | Select a specific zero-based operation index in multi-operation transactions. |

---

## Source Mapping Flags

| Flag | Default | Description |
|---|---|---|
| `--contract-source` | _(auto-discovery)_ | Explicit path to the contract source directory when auto-discovery fails. |
| `--skip-source-mapping` | `false` | Skip DWARF source mapping for faster raw trace replay. |
| `--source-alias` | _(none)_ | Path to a JSON file mapping embedded source paths to local filesystem paths. |

---

## Session & Watch Flags

| Flag | Default | Description |
|---|---|---|
| `--watch` | `false` | Poll for a pending transaction to appear on-chain before debugging. |
| `--watch-timeout` | `30` | Timeout in seconds for `--watch` mode. |
| `--save-snapshots` | _(none)_ | Save simulation results to a snapshot registry file. |
| `--pin-endpoint` | _(none)_ | Pin a specific RPC endpoint and store it with the session. Must match `--rpc-url` when both are provided. |
| `--no-cache` | `false` | Disable local ledger state caching for this run. |
| `--snapshots` | `false` | Enable snapshot capture inside the simulator. |

---

## Audit & Decentralised Storage

| Flag | Default | Description |
|---|---|---|
| `--audit-key` | _(none)_ | Ed25519 private key (PEM) used to sign the audit trail before publishing. |
| `--publish-ipfs` | `false` | Publish a signed audit trail to IPFS after simulation. Requires `--audit-key`. |
| `--publish-arweave` | `false` | Publish a signed audit trail to Arweave after simulation. Requires `--audit-key`. |
| `--ipfs-node` | _(public gateway)_ | IPFS node API URL. |
| `--arweave-gateway` | _(none)_ | Arweave gateway URL. |
| `--arweave-wallet` | _(none)_ | Path to an Arweave wallet JSON file. |

See [audit-signing.md](./audit-signing.md) for the full audit workflow.

---

## Error Handling & Diagnostics

The debug command returns explicit, actionable errors for all common failure modes:

| Failure | Error message |
|---|---|
| Invalid transaction hash | `invalid transaction hash "…" — expected 64 hexadecimal characters (got N)` |
| Invalid `--network` | `invalid network "…" — must be one of: testnet, mainnet, futurenet` |
| Invalid `--compare-network` | Same as above |
| Missing `--wasm` with `--hot-reload` | `--hot-reload requires --wasm` |
| Both `--xdr-file` and `--json-file` | `only one of --xdr-file or --json-file may be specified` |
| Hash + local file conflict | `cannot specify both a transaction hash and a local envelope file` |
| `--dry-run` with local modes | `--dry-run cannot be combined with --demo, --wasm, --load-snapshots, or local envelope input` |
| `--pin-endpoint` mismatch | `--pin-endpoint must match --rpc-url when both are provided` |
| RPC connection failure | `RPC connection failed: <underlying error>` |
| Transaction not found | `transaction not found` — check the hash and the selected network |
| Simulator not found | `simulator binary not found` — run `glassbox doctor --fix` |
| Simulation failure | `simulation execution failed: <detail>` — check the diagnostic section of the output |
| No simulation results | `no simulation results generated` — indicates an internal logic error |

For environment setup problems, run `glassbox doctor` for a comprehensive health check.

---

## Demo Mode

```sh
glassbox debug --demo
```

Prints sample output without making any network calls. Useful for testing terminal color detection.

---

## Examples

```sh
# Debug a transaction on mainnet (default)
glassbox debug 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab

# Debug on testnet
glassbox debug --network testnet abc123...def789

# Validate parameters without running a simulation
glassbox debug --dry-run --network testnet abc123...def789

# Compare execution between testnet and mainnet
glassbox debug --network testnet --compare-network mainnet abc123...def789

# Debug locally without a network connection
glassbox debug --wasm ./build/contract.wasm --args "my-arg"

# Debug from a saved XDR file
glassbox debug --xdr-file ./envelope.xdr

# Output machine-readable JSON
glassbox debug --json 5c0a1234...ef7890ab

# Save ledger snapshots for offline replay
glassbox debug --save-snapshots ./registry.json 5c0a1234...ef7890ab

# Replay from snapshots (no network)
glassbox debug --load-snapshots ./registry.json
```

---

## See Also

- [`glassbox diagnostics`](./debug-command.md#diagnostics) — unified health dashboard
- [`glassbox doctor`](./sandboxed-replay.md) — environment setup checker
- [`glassbox session`](./session-bookmarking.md) — save and restore debug sessions
- [Snapshot deduplication](./snapshot-deduplication.md)
- [Source mapping](./source-mapping.md)
- [Audit signing](./audit-signing.md)
