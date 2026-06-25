# Trace Export Validation and Diagnostics

The `glassbox debug` command includes comprehensive validation and diagnostic capabilities for trace export operations. This document describes the validation checks, error handling, and troubleshooting guidance.

---

## Overview

Trace export validation occurs at multiple stages:

1. **Pre-flight validation** — CLI flag validation before any simulation
2. **Pre-export validation** — Trace data and configuration validation before export
3. **Format compatibility** — Format-specific checks for data compatibility
4. **Export execution** — File system and I/O validation during write

---

## Pre-flight Validation (CLI Flags)

The debug command validates trace-related flags in `PreRunE` before any network or simulator operations:

### `--trace-verbosity`

**Valid values:** `summary`, `normal`, `verbose`

**Validation:**
- Must be one of the three supported values (case-insensitive)
- Checked at parse time before any execution

**Error example:**
```
invalid --trace-verbosity "ultra" — must be one of: summary, normal, verbose
  Fix: use --trace-verbosity normal (default), summary (minimal), or verbose (detailed)
```

### `--format`

**Valid values:** `text`, `json`, `html`, `markdown` (or `md`)

**Validation:**
- Must be one of the supported export formats
- Checked before simulation begins

**Error example:**
```
invalid trace export format "yaml" — must be one of: text, json, html, markdown
  Fix: use --format html (interactive), json (machine-readable), markdown (shareable), or text (CLI output)
```

### `--trace-output`

**Validation:**
- Must be a file path, not a directory path (no trailing `/` or `\`)
- Cannot contain null bytes
- Path traversal sequences (`..`) trigger a security warning
- Parent directory must exist or be creatable

**Error examples:**
```
--trace-output "./traces/" looks like a directory path; provide a full file path
  Fix: specify a complete file path (e.g. ./traces/trace.html or ./output/trace.json)
  Example: glassbox debug --trace-output ./traces/debug-$(date +%Y%m%d).html <tx-hash>
```

```
--trace-output "../../../etc/passwd" contains directory traversal sequences (..)
  Fix: use absolute paths or relative paths without '..' for security
  Example: use './output/trace.html' instead of '../output/trace.html'
```

---

## Pre-export Validation

Before attempting to write a trace export, the system validates all export parameters:

### Trace Data Validation

**Checks:**
- Trace object is not nil
- Trace contains at least one execution state
- All state step indices match their position in the array
- Event types are recognized (unrecognized types trigger warnings)

**Error example:**
```
execution trace for transaction "5c0a1234..." contains no steps — trace export would be empty
  Possible causes:
    - Simulation did not produce any diagnostic events
    - Transaction envelope is invalid
    - Simulator version is incompatible
  Fix: verify the transaction executed successfully
  Recommended: run 'glassbox doctor' to check simulator compatibility
```

### Format and Path Validation

**Checks:**
- Export format is not empty and is supported
- Output path is not empty
- Output path is not a directory
- Output path does not contain invalid characters

**Error example:**
```
export format is empty — must specify one of: html, markdown, json, text
  Fix: provide --format html (default), markdown, json, or text
```

### Export Options Validation

**Checks:**
- Comment count does not exceed 100
- Individual comment length does not exceed 10,000 characters
- Session metadata keys and values are valid strings

**Error example:**
```
too many comments (150) — maximum is 100 comments per trace export
  Fix: reduce the number of comments or split into multiple exports
```

---

## Format Compatibility Checks

Each export format has specific compatibility requirements:

### JSON Format

**Requirements:**
- All trace data must be JSON-serializable
- No circular references
- Step indices must be sequential and match array position

**Error example:**
```
trace step mismatch at position 5: expected step 5 but got 10 — trace may be corrupted
```

### HTML Format

**Requirements:**
- Argument strings should not exceed 50,000 characters (browser rendering limit)
- Special characters are automatically HTML-escaped

**Error example:**
```
step 3 has very large arguments (75000 chars) that may cause browser rendering issues in HTML format — consider using JSON format instead
```

### Markdown Format

**Requirements:**
- Works with most data
- Very long lines may require viewer adjustments

### Text Format

**Requirements:**
- Most permissive format
- No special constraints

---

## Export Execution Errors

Errors that occur during file write operations include detailed remediation:

### Directory Creation Failure

```
failed to create trace export directory: permission denied
  Directory: /restricted/traces
  Fix: ensure you have write permissions to the parent directory
  Or choose a different output path with --trace-output
```

### File Write Failure

```
failed to write trace export file: no space left on device
  Path: /tmp/trace.html
  Fix: ensure you have write permissions and sufficient disk space
  Check: ls -la /tmp
```

### Template Rendering Failure

```
failed to generate HTML trace: template execution error: ...
  This may indicate invalid trace data or a template rendering error
  Check that all trace fields are properly populated
```

---

## Validation in Dry-Run Mode

When using `--dry-run`, trace output configuration is validated without executing the simulation:

```sh
glassbox debug --dry-run --trace-output ./invalid/ --network testnet <tx-hash>
```

**Output:**
```
Additional environment checks:
[FAIL] Trace output validation failed: --trace-output "./invalid/" looks like a directory path
       Fix: ensure trace output path is valid and format is correct
```

---

## Multiple Validation Errors

When multiple validation errors are detected, all failures are reported together so they can be fixed in a single pass:

```
3 trace input validation error(s):
  1. invalid --trace-verbosity "ultra" — must be one of: summary, normal, verbose
     Fix: use --trace-verbosity normal (default), summary (minimal), or verbose (detailed)
  2. invalid trace export format "yaml" — must be one of: text, json, html, markdown
     Fix: use --format html (interactive), json (machine-readable), markdown (shareable), or text (CLI output)
  3. --trace-output "./traces/" looks like a directory path; provide a full file path
     Fix: specify a complete file path (e.g. ./traces/trace.html or ./output/trace.json)
```

---

## Best Practices

### 1. Use Dry-Run for Validation

Always run with `--dry-run` first when setting up trace export in CI/CD:

```sh
glassbox debug --dry-run \
  --network testnet \
  --trace-output ./artifacts/trace.html \
  --format html \
  <tx-hash>
```

### 2. Choose the Right Format

- **HTML**: Interactive viewing in browsers, best for manual analysis
- **JSON**: Machine-readable, best for CI/CD and automated processing
- **Markdown**: Shareable in chat/issues, best for collaboration
- **Text**: Plain CLI output, best for simple logging

### 3. Organize Output Paths

Use dated directories for trace exports:

```sh
glassbox debug \
  --trace-output "./traces/$(date +%Y-%m-%d)/${TX_HASH}.html" \
  --format html \
  $TX_HASH
```

### 4. Validate Before Large Exports

For traces with many steps, validate format compatibility first:

```sh
# Check trace size first
glassbox debug --format json $TX_HASH | jq '.States | length'

# Use JSON for very large traces (>1000 steps)
if [ $STEPS -gt 1000 ]; then
  glassbox debug --format json --trace-output ./trace.json $TX_HASH
else
  glassbox debug --format html --trace-output ./trace.html $TX_HASH
fi
```

---

## Troubleshooting

### "Trace contains no steps"

**Cause:** The simulator did not produce any diagnostic events.

**Solutions:**
1. Verify the transaction hash is correct
2. Run `glassbox doctor` to check simulator compatibility
3. Check that the transaction actually executed on the network
4. Ensure the simulator binary is up-to-date

### "Trace step mismatch"

**Cause:** The trace data structure is corrupted.

**Solutions:**
1. Re-run the debug command to regenerate the trace
2. Check for filesystem corruption if using `--save-snapshots`
3. Verify the simulator version matches the CLI version

### "Very large arguments"

**Cause:** Contract arguments exceed browser rendering limits for HTML export.

**Solutions:**
1. Use JSON format instead: `--format json`
2. Filter the trace to specific event types
3. Use `--trace-verbosity summary` for less detail

### "Permission denied"

**Cause:** Insufficient write permissions to the output directory.

**Solutions:**
1. Choose a different output path with write permissions
2. Create the output directory manually with correct permissions
3. Check filesystem mount options (read-only mounts)

---

## See Also

- [Debug Command Reference](./debug-command.md)
- [Trace Export Annotations](./trace-export-annotations.md)
- [Event Schemas](./event-schemas.md)
- [JSON Output Format](./json-output.md)
