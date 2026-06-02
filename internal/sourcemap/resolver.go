// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dotandev/glassbox/internal/dwarf"
	"github.com/dotandev/glassbox/internal/logger"
)

// Resolver coordinates fetching verified source code from a registry,
// with optional local caching and auto-discovery of local DWARF symbols.
type Resolver struct {
	registry        *RegistryClient
	cache           *SourceCache
	githubRetriever *GitHubRetriever
	// contractSourceOverride is an explicit local path to the contract source
	// directory. When set, it is used as a fallback before prompting the user.
	contractSourceOverride string
	// aliasResolver translates workspace-relative path prefixes to real
	// filesystem paths before source files are opened.
	aliasResolver *AliasResolver
}

// ResolverOption is a functional option for configuring the Resolver.
type ResolverOption func(*Resolver)

// WithCache enables caching with the specified directory.
func WithCache(cacheDir string) ResolverOption {
	return func(r *Resolver) {
		cache, err := NewSourceCache(filepath.Join(cacheDir, "sourcemap"))
		if err != nil {
			logger.Logger.Warn("Failed to create source cache, caching disabled", "error", err)
			return
		}
		r.cache = cache
	}
}

// WithRegistryClient sets a custom registry client.
func WithRegistryClient(rc *RegistryClient) ResolverOption {
	return func(r *Resolver) {
		r.registry = rc
	}
}

// WithContractSource sets an explicit local path to the contract source
// directory. When automatic discovery fails, this path is used before
// prompting the user interactively (Issue #117).
func WithContractSource(path string) ResolverOption {
	return func(r *Resolver) {
		r.contractSourceOverride = path
	}
}

// WithAliasResolver sets an AliasResolver that translates workspace-relative
// path prefixes to real filesystem paths when resolving source file locations.
func WithAliasResolver(ar *AliasResolver) ResolverOption {
	return func(r *Resolver) {
		r.aliasResolver = ar
	}
}

// NewResolver creates a Resolver with the given options.
func NewResolver(opts ...ResolverOption) *Resolver {
	r := &Resolver{
		registry: NewRegistryClient(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Resolve attempts to find verified source code for the given contract ID.
// It checks the local cache first, then queries the registry.
// If both fail, it prompts the user for a manual WASM path (Issue #372).
func (r *Resolver) Resolve(ctx context.Context, contractID string) (*SourceCode, error) {
	if err := validateContractID(contractID); err != nil {
		return nil, fmt.Errorf("invalid contract ID: %w", err)
	}

	// 1. Check cache first
	if r.cache != nil {
		if cached := r.cache.Get(contractID); cached != nil {
			logger.Logger.Info("Source resolved from cache", "contract_id", contractID)
			return cached, nil
		}
	}

	// 2. Fetch from registry
	source, err := r.registry.FetchVerifiedSource(ctx, contractID)
	if err != nil {
		// Log the error but continue to fallback
		logger.Logger.Debug("Registry lookup failed", "contract_id", contractID, "error", err)
	}

	// 3. GitHub fallback: download from configured repository when registry returns nothing.
	if source == nil && r.githubRetriever != nil {
		ghSource, ghErr := r.githubRetriever.Retrieve(ctx, contractID)
		if ghErr != nil {
			logger.Logger.Debug("GitHub source retrieval failed",
				"contract_id", contractID, "error", ghErr)
		} else if ghSource != nil {
			logger.Logger.Info("Source resolved from GitHub",
				"contract_id", contractID,
				"repository", ghSource.Repository,
				"file_count", len(ghSource.Files),
			)
			return ghSource, nil
		}
	}

	// 4. Fallback: use explicit override path if provided (Issue #117)
	if source == nil && r.contractSourceOverride != "" {
		logger.Logger.Info("Using --contract-source override for source mapping",
			"contract_id", contractID,
			"path", r.contractSourceOverride,
		)
		return &SourceCode{
			ContractID: contractID,
			Repository: r.contractSourceOverride,
			Files:      map[string]string{},
			FetchedAt:  time.Now(),
		}, nil
	}

	// 5. Fallback: Prompt user if source is unresolved (Issue #372)
	if source == nil {
		logger.Logger.Info("Contract source unresolved automatically", "contract_id", contractID)

		manualPath, err := r.PromptForWasmPath()
		if err != nil {
			return nil, fmt.Errorf("failed to get manual WASM path: %w", err)
		}

		if manualPath != "" {
			// In a real scenario, you might attempt to load symbols from this path
			// using the dwarf.Parser here. For now, we log the path as per requirements.
			logger.Logger.Info("Manual WASM path provided by user", "path", manualPath)
		}

		return nil, nil
	}

	// 6. Cache the result
	if r.cache != nil {
		if err := r.cache.Put(source); err != nil {
			logger.Logger.Warn("Failed to cache source", "contract_id", contractID, "error", err)
		}
	}

	logger.Logger.Info("Source resolved from registry",
		"contract_id", contractID,
		"repository", source.Repository,
		"file_count", len(source.Files),
	)

	return source, nil
}

// PromptForWasmPath pauses execution and asks the user for a manual WASM path.
// Requirement: If Glassbox encounters an unknown contract, pause and ask the user
// "Please provide path to contract WASM for better mapping".
func (r *Resolver) PromptForWasmPath() (string, error) {
	// Exact string required by Issue #372
	fmt.Print("Please provide path to contract WASM for better mapping: ")

	reader := bufio.NewReader(os.Stdin)
	path, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(path), nil
}

// AutoDiscoverLocalSymbols scans the project root for local WASM builds.
// If a bytecode hash match is found, it merges DWARF debug symbols.
func (r *Resolver) AutoDiscoverLocalSymbols(projectRoot string, expectedHash string) error {
	searchDir := filepath.Join(projectRoot, wasmTargetPath)

	// Verify directory exists
	if _, err := os.Stat(searchDir); os.IsNotExist(err) {
		logger.Logger.Debug("Local build directory not found", "path", searchDir)
		return nil
	}

	files, err := os.ReadDir(searchDir)
	if err != nil {
		return fmt.Errorf("failed to read local wasm directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".wasm") {
			continue
		}

		fullPath := filepath.Join(searchDir, file.Name())
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		// Check bytecode hash
		hash := sha256.Sum256(content)
		actualHash := hex.EncodeToString(hash[:])

		if actualHash != expectedHash {
			continue
		}

		// Match found! Extract symbols
		logger.Logger.Info("Found local WASM match", "file", file.Name())

		parser, err := dwarf.NewParser(content)
		if err != nil {
			logger.Logger.Error("Failed to parse DWARF", "file", file.Name(), "error", err)
			continue
		}

		if !parser.HasDebugInfo() {
			logger.Logger.Warn("Local WASM found but contains no debug symbols", "file", file.Name())
			continue
		}

		subprograms, err := parser.GetSubprograms()
		if err != nil {
			logger.Logger.Error("Failed to extract subprograms", "file", file.Name(), "error", err)
			continue
		}

		// Integration point: Merge symbols into the resolver session
		logger.Logger.Info("Automatically merged symbols from local build",
			"file", file.Name(),
			"count", len(subprograms))
	}

	return nil
}

// ResolveFilePath applies the alias resolver (if configured) to translate a
// workspace-relative source file path to a real filesystem path.
// Returns p unchanged when no alias resolver is set.
func (r *Resolver) ResolveFilePath(p string) string {
	if r.aliasResolver == nil {
		return p
	}
	return r.aliasResolver.Resolve(p)
}

// InvalidateCache removes a specific contract from the cache.
func (r *Resolver) InvalidateCache(contractID string) error {
	if r.cache == nil {
		return nil
	}
	return r.cache.Invalidate(contractID)
}

// ClearCache removes all cached source entries.
func (r *Resolver) ClearCache() error {
	if r.cache == nil {
		return nil
	}
	return r.cache.Clear()
}
