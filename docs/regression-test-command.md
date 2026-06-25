# `glassbox regression-test` — Regression Test Command Reference

The `regression-test` command fetches historic failed transactions from the Stellar network
and verifies that the local simulator produces identical results, ensuring protocol changes
do not introduce regressions.

---

## Synopsis

```sh
glassbox regression-test [flags]
```

---

## Flags

| Flag | Default | Description |
|---|---|---|
| `--count` | `100` | Number of historic failed transactions to test. Must be between 1 and 1000 (inclusive). |
| `--workers` | `4` | Number of parallel test workers. Must be a positive integer. |
| `--network`, `-n` | `mainnet` | Stellar network: `testnet`, `mainnet`, or `futurenet`. |
| `--rpc-url` | _(config)_ | Custom RPC URL. Overrides the default for the selected network. |
| `--rpc-token` | _(env: `GLASSBOX_RPC_TOKEN`)_ | RPC authentication token. |
| `--start-seq` | `0` | Starting ledger sequence number (`0` = most recent). |
| `--protocol-version` | `0` | Override protocol version for all tests (`0` = use default). |
| `--verbose`, `-v` | `false` | Show per-transaction progress output. |

---

## Validation

All flags are validated in `PreRunE` before any network or simulator calls are made:

| Condition | Error message |
|---|---|
| `--count` ≤ 0 | `--count must be greater than 0 (got N)` |
| `--count` > 1000 | `--count N exceeds the maximum of 1000` |
| `--workers` < 0 | `--workers must be a positive integer (got N)` |
| `--network` unknown | `invalid --network "…"; must be one of: testnet, mainnet, futurenet` |
| `--protocol-version` unsupported | `invalid --protocol-version N: …` with a hint to run `glassbox version` |

Failures are surfaced immediately, before consuming any network quota.

---

## Output

On completion, a summary is printed:

```
Regression Test Summary:
  Total Tests: 100
  Passed:      97
  Failed:      2
  Errors:      1
  Success Rate: 97.0%
```

When failures occur, the first 10 are listed with their transaction hash and error message.
A hint is included to run `glassbox debug <tx-hash>` for a detailed trace of any failing
transaction.

---

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | All tests passed |
| `1` | One or more tests failed, or a validation error occurred |
| `2` | Simulator binary not found — run `glassbox doctor --fix` |

---

## Examples

```sh
# Run 100 regression tests on mainnet (default)
glassbox regression-test --count 100

# Use more parallel workers for faster runs
glassbox regression-test --count 1000 --workers 8

# Test against a specific protocol version
glassbox regression-test --count 500 --network mainnet --protocol-version 22

# Verbose output shows per-transaction progress
glassbox regression-test --count 50 --verbose

# Use a custom RPC endpoint
glassbox regression-test --count 200 --rpc-url https://my-rpc.example.com
```

---

## See Also

- [`glassbox debug`](./debug-command.md) — debug a single transaction in detail
- [`glassbox doctor`](./sandboxed-replay.md) — environment setup checker
- [`glassbox version`](./debug-command.md) — check supported protocol versions
