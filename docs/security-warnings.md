# Contract Security Warnings

Glassbox emits proactive warnings for insecure Soroban patterns detected in
contract source or metadata.

Current contract-level checks include:

- Open authorization: privileged functions such as owner/admin/mint/update paths
  without an obvious `require_auth`, `require_auth_for_args`, or `check_auth`.
- Unchecked minting: token mint paths without both an authorization guard and a
  visible supply cap or policy check.
- Unsafe signature types: raw signature verification without obvious domain
  separation, nonce, or network binding.

Warnings are heuristic. Treat them as prompts for review, not proof of a bug.
Each warning includes severity, evidence, and a safer alternative.

The Go API is:

```go
detector := security.NewDetector()
findings := detector.AnalyzeContractSource(security.SourceContext{
    Path: "contract/src/lib.rs",
    Source: sourceCode,
    Metadata: metadata,
})
```
