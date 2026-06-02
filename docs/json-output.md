# JSON CLI Output

Several Glassbox commands support machine-readable JSON for automation.

## Envelope format

When `--json` is set (or `--format json`), output is wrapped as:

```json
{
  "schema_version": "1.0",
  "glassbox_version": "0.0.0-dev",
  "generated_at": "2026-06-01T12:00:00Z",
  "command": "debug",
  "data": { }
}
```

## Commands

| Command | Flags |
|---------|--------|
| `debug` | `--json`, `--format json` |
| `audit:sign` | `--json` (schema envelope; legacy output without the flag) |
| `protocol:diagnose` | `--json` |
| `generate-bindings` | `--json`, `--format json` |
| `check-bindings` | `--json` |
| `config show` | `--json` |
| `bench` | `--json` |

## Example

```bash
glassbox debug --wasm ./contract.wasm --json --format json
```
