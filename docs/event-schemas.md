# Contract Event Schemas

Glassbox can decode contract event payloads in audit logs and trace output when
you provide event schemas.

Use JSON schema files:

```json
{
  "events": [
    {
      "name": "transfer",
      "fields": [
        {"name": "from", "type": "address"},
        {"name": "to", "type": "address"},
        {"name": "amount", "type": "i128"}
      ]
    }
  ]
}
```

ABI-style event entries are also accepted:

```json
[
  {
    "type": "event",
    "name": "mint",
    "inputs": [
      {"name": "admin", "type": "address"},
      {"name": "amount", "type": "i128"}
    ]
  }
]
```

Trace output can load schemas with:

```sh
glassbox trace --print --event-schema events.json trace.json
```

When schemas are loaded, trace JSON written by Glassbox includes decoded events
under `decoded_events`.

Audit generation can attach decoded events by setting
`GenerateOptions.EventSchemas`. Decoded events are written to
`payload.decoded_events` and are covered by the audit hash and signature.
