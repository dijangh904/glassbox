# Report Command

`glassbox report` summarizes diagnostics from a completed trace.

Severity-based diagnostic reports are available in text and JSON:

```sh
glassbox report --file trace.json --format text --output reports
glassbox report --file trace.json --format json --output reports
```

The JSON report contains:

- `severity`: `critical`, `high`, `medium`, `low`, or `info`
- `category`: execution, WASM, authorization, or another diagnostic group
- `summary`: short issue title
- `details`: supporting context
- `action`: recommended next step
- `step` and `contract_id` when available

HTML and PDF formats remain available for the existing full execution report.
