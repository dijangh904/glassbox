# Sandboxed WASM Replay

Simulator requests can opt into sandbox mode with explicit resource and host
function exposure controls.

Sandbox mode requires:

- A non-zero `memory_limit`
- An explicit `allowed_host_functions` allowlist
- The existing simulator subprocess isolation and bounded stdout/stderr buffers

Example request fields:

```json
{
  "sandbox_mode": true,
  "memory_limit": 67108864,
  "allowed_host_functions": ["storage_get", "storage_put", "storage_del"]
}
```

Glassbox rejects sandbox requests that omit the memory limit or allowlist before
starting the simulator process. The allowlist and memory limit are also passed to
the simulator custom configuration for runtime enforcement by integrations that
support restricted host function exposure.
