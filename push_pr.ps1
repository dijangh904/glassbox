Set-Location "c:\Users\LEO PC\OneDrive\Desktop\yosemite\glassbox"

git config user.email "dev@glassbox.local"
git config user.name "Glassbox Dev"

git add .

git commit -F commit_msg.txt

git push -u origin improve/debug-source-discovery-fallback

$prBody = @"
## Summary
Resolves silent failure modes and ambiguous error handling in the debug command source discovery and fallback workflow.

## Changes
- DiscoverLocalSymbols returns DiscoveryResult with Warnings; validates empty projectRoot and path-is-file
- Resolver.Resolve validates --contract-source override path; WithNonInteractive() returns ErrSourceNotFound instead of blocking stdin in CI
- AutoDiscoverLocalSymbols validates both inputs upfront
- validateSourceDiscoveryFlags() covers --wasm/--demo early-return paths
- dry-run source discovery pre-flight with [OK]/[FAIL] and Fix: hints
- ErrSourceDiscoveryFailed sentinel and WrapSourceDiscoveryFailed with Hint field
- Fixed pre-existing workspace.go compile errors (SourceMapData -> SourceMaps)
- Fixed pre-existing duplicate contains() in external_repos_test.go
- 47 new tests across 4 new test files
- Updated docs/source-mapping.md and docs/debug-command.md

## Testing
47 new unit tests covering all new validation paths and failure conditions.

## Breaking changes
None. DiscoverLocalSymbolsLegacy shim preserves all existing callers.
"@

$prBody | Out-File -FilePath "pr_body.txt" -Encoding utf8

gh pr create --title "fix(debug): improve source discovery validation and fallback diagnostics" --body-file pr_body.txt --base main
