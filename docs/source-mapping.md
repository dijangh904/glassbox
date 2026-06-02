# Source Mapping

Glassbox maps WASM instruction failures back to Rust source code lines using
DWARF debug symbols embedded in the compiled WASM binary.

## Automatic Discovery

When a contract fails, Glassbox attempts to resolve the source location through
the following pipeline:

1. **Local cache** — previously resolved source is returned immediately.
2. **Registry** — queries [stellar.expert](https://stellar.expert) for a
   verified source link.
3. **GitHub fallback** — downloads source from the linked repository when a
   `GitHubRetriever` is configured.
4. **`--contract-source` override** — uses the explicitly provided local path
   (see below).
5. **Interactive prompt** — asks the user for a WASM path when all automatic
   methods fail.

## `--contract-source` Override

When automatic discovery fails (e.g. the contract is not yet verified on
stellar.expert, or you are working with a private repository), you can provide
the path to the contract source directory explicitly:

```bash
glassbox debug --wasm ./target/wasm32-unknown-unknown/release/my_contract.wasm \
               --contract-source ./src \
               <transaction-hash>
```

Or for local WASM replay:

```bash
glassbox debug --wasm ./contract.wasm \
               --contract-source /path/to/contract/src
```

### How it works

- When `--contract-source <path>` is set and automatic source resolution fails,
  Glassbox uses `<path>` as the root directory for resolving source file
  references from DWARF debug info.
- The path is tried directly, then as a prefix for the relative file path
  reported by the DWARF info, and finally as a prefix for just the filename.
- The path is also forwarded to the simulator via `ContractSourcePath` in the
  `SimulationRequest`, allowing the Rust simulator to resolve source lines
  during execution.

### When to use it

| Situation | Recommendation |
|-----------|---------------|
| Contract not verified on stellar.expert | `--contract-source ./src` |
| Private repository | `--contract-source /path/to/repo/src` |
| Monorepo with multiple contracts | `--contract-source ./contracts/my_contract/src` |
| CI/CD pipeline (non-interactive) | `--contract-source $CONTRACT_SRC_DIR` |

### Compiling with debug symbols

For best results, compile your contract with debug symbols:

```toml
# Cargo.toml
[profile.release]
debug = true
```

Then build:

```bash
cargo build --target wasm32-unknown-unknown --release
```

See [docs/debug-symbols-guide.md](debug-symbols-guide.md) for more details.

## Cross-repository source links

When contract sources live in another Git repository, map local path prefixes to
remote GitHub URLs in `.glassbox.toml`:

```toml
external_source_map = '[{"prefix":"/path/to/vendor/lib","remote_url":"https://github.com/org/lib","branch":"main"}]'
```

Glassbox uses these mappings when a source file path falls outside the workspace
repository but under the configured prefix.

## Skip source mapping

For faster raw replay when you only need WASM offsets and traces:

```bash
glassbox debug --wasm ./contract.wasm --skip-source-mapping
```

This bypasses DWARF parsing and Git link generation in the simulator.

## Trace verbosity

Control trace detail with `--trace-verbosity`:

| Level | Output |
|-------|--------|
| `summary` | Step names and status only |
| `normal` | Source locations and links (default) |
| `verbose` | Arguments, WASM instructions, and full event payloads |

```bash
glassbox debug --wasm ./contract.wasm --trace-verbosity summary
glassbox trace --print --trace-verbosity verbose execution.json
```
