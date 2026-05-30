// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dotandev/glassbox/internal/logger"
)

// GitHubConfig holds the configuration for GitHub-based source retrieval.
// All fields are optional; when Repository is empty the retriever is a no-op.
type GitHubConfig struct {
	// Repository is the GitHub repository in "owner/repo" or full URL form.
	// Supported formats: "owner/repo", "https://github.com/owner/repo",
	// "github.com/owner/repo", "git@github.com:owner/repo.git".
	Repository string

	// Revision is the Git commit SHA, tag, or branch name to fetch from.
	// Defaults to the repository's default branch when empty.
	Revision string

	// Token is an optional GitHub personal access token for private repos
	// or to avoid rate limiting. Falls back to unauthenticated requests when empty.
	Token string

	// RequestTimeout overrides the default HTTP request timeout.
	// Zero means DefaultRequestTimeout.
	RequestTimeout time.Duration
}

// GitHubRetriever downloads Soroban contract source files from a GitHub
// repository when local sources are unavailable.
//
// It integrates into the sourcemap pipeline and stores retrieved files in a
// SourceCache so that subsequent lookups avoid redundant network traffic.
type GitHubRetriever struct {
	cfg    GitHubConfig
	cache  *SourceCache
	client *http.Client
}

// NewGitHubRetriever constructs a retriever from the provided config.
// Pass a non-nil cache to enable local caching of downloaded source files.
func NewGitHubRetriever(cfg GitHubConfig, cache *SourceCache) *GitHubRetriever {
	timeout := cfg.RequestTimeout
	if timeout == 0 {
		timeout = DefaultRequestTimeout
	}
	return &GitHubRetriever{
		cfg:   cfg,
		cache: cache,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Retrieve attempts to download source files for contractID from the
// configured GitHub repository.
//
// Lookup order:
//  1. Local cache (when a cache is attached and the entry is still fresh).
//  2. GitHub raw file API using the configured revision (or default branch).
//
// Returns nil with no error when the retriever is disabled (no repository
// configured), so callers can safely treat a nil result as "not found".
func (r *GitHubRetriever) Retrieve(ctx context.Context, contractID string) (*SourceCode, error) {
	if r.cfg.Repository == "" {
		logger.Logger.Debug("GitHub retriever disabled (no repository configured)")
		return nil, nil
	}

	// 1. Try cache first.
	if r.cache != nil {
		if cached := r.cache.Get(contractID); cached != nil {
			logger.Logger.Info("GitHub source resolved from cache",
				"contract_id", contractID,
				"repository", cached.Repository,
			)
			return cached, nil
		}
	}

	// 2. Download from GitHub.
	owner, repo, err := parseGitHubURL(r.cfg.Repository)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL %q: %w", r.cfg.Repository, err)
	}

	revision, err := r.resolveRevision(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve revision: %w", err)
	}

	logger.Logger.Info("Fetching contract source from GitHub",
		"contract_id", contractID,
		"owner", owner,
		"repo", repo,
		"revision", revision,
	)

	files, err := r.fetchSourceTree(ctx, owner, repo, revision)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch source from GitHub (%s/%s@%s): %w", owner, repo, revision, err)
	}

	if len(files) == 0 {
		logger.Logger.Warn("No source files found in repository",
			"owner", owner, "repo", repo, "revision", revision)
		return nil, nil
	}

	source := &SourceCode{
		ContractID: contractID,
		Repository: fmt.Sprintf("https://github.com/%s/%s", owner, repo),
		Files:      files,
		FetchedAt:  time.Now(),
	}

	// 3. Store in cache.
	if r.cache != nil {
		if err := r.cache.Put(source); err != nil {
			logger.Logger.Warn("Failed to cache GitHub source",
				"contract_id", contractID, "error", err)
		}
	}

	return source, nil
}

// RetrieveFile downloads a single source file from GitHub and validates its
// SHA-256 digest when expectedSHA is non-empty.
//
// This is useful for targeted retrieval of a specific source file referenced
// in a debug session without downloading the full repository tree.
func (r *GitHubRetriever) RetrieveFile(ctx context.Context, filePath, expectedSHA string) (string, error) {
	if r.cfg.Repository == "" {
		return "", fmt.Errorf("GitHub retriever is not configured")
	}

	owner, repo, err := parseGitHubURL(r.cfg.Repository)
	if err != nil {
		return "", fmt.Errorf("invalid repository URL: %w", err)
	}

	revision, err := r.resolveRevision(ctx, owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to resolve revision: %w", err)
	}

	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		owner, repo, revision, filePath)

	content, statusCode, err := r.doGet(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %q: %w", filePath, err)
	}
	if statusCode == http.StatusNotFound {
		return "", fmt.Errorf("file %q not found in %s/%s@%s", filePath, owner, repo, revision)
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d fetching %q", statusCode, filePath)
	}

	if expectedSHA != "" {
		if err := validateContentSHA(content, expectedSHA); err != nil {
			return "", fmt.Errorf("SHA validation failed for %q: %w", filePath, err)
		}
	}

	return string(content), nil
}

// resolveRevision returns the configured revision or falls back to the
// repository's default branch by querying the GitHub API.
func (r *GitHubRetriever) resolveRevision(ctx context.Context, owner, repo string) (string, error) {
	if r.cfg.Revision != "" {
		return r.cfg.Revision, nil
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	body, statusCode, err := r.doGet(ctx, apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch repo metadata: %w", err)
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d for %s/%s", statusCode, owner, repo)
	}

	var meta struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return "", fmt.Errorf("failed to parse repo metadata: %w", err)
	}

	if meta.DefaultBranch == "" {
		return "main", nil
	}
	return meta.DefaultBranch, nil
}

// fetchSourceTree downloads the file tree for the given revision and retrieves
// all Soroban-relevant source files.
func (r *GitHubRetriever) fetchSourceTree(ctx context.Context, owner, repo, revision string) (map[string]string, error) {
	treeURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
		owner, repo, revision,
	)

	body, statusCode, err := r.doGet(ctx, treeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository tree: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d for tree of %s/%s@%s", statusCode, owner, repo, revision)
	}

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
			Size int    `json:"size"`
		} `json:"tree"`
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, fmt.Errorf("failed to parse repository tree: %w", err)
	}

	if tree.Truncated {
		logger.Logger.Warn("Repository tree was truncated by GitHub API",
			"owner", owner, "repo", repo, "revision", revision)
	}

	files := make(map[string]string)
	for _, entry := range tree.Tree {
		if entry.Type != "blob" || !isSourceFile(entry.Path) {
			continue
		}
		if entry.Size > MaxResponseSize {
			logger.Logger.Warn("Skipping oversized source file",
				"path", entry.Path, "size", entry.Size)
			continue
		}

		rawURL := fmt.Sprintf(
			"https://raw.githubusercontent.com/%s/%s/%s/%s",
			owner, repo, revision, entry.Path,
		)
		content, sc, err := r.doGet(ctx, rawURL)
		if err != nil {
			logger.Logger.Warn("Failed to fetch source file", "path", entry.Path, "error", err)
			continue
		}
		if sc != http.StatusOK {
			logger.Logger.Warn("Non-200 fetching source file", "path", entry.Path, "status", sc)
			continue
		}
		files[entry.Path] = string(content)
	}

	return files, nil
}

// doGet performs an authenticated GET request if a token is configured.
func (r *GitHubRetriever) doGet(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Glassbox/sourcemap-github")
	if r.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseSize))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, resp.StatusCode, nil
}

// validateContentSHA checks that the SHA-256 digest of content matches expected.
// expectedSHA may be a full 64-char hex string or a shorter prefix for partial matching.
func validateContentSHA(content []byte, expectedSHA string) error {
	hash := sha256.Sum256(content)
	actual := hex.EncodeToString(hash[:])

	expectedSHA = strings.ToLower(strings.TrimSpace(expectedSHA))
	if !strings.HasPrefix(actual, expectedSHA) {
		return fmt.Errorf("content SHA %q does not match expected %q", actual, expectedSHA)
	}
	return nil
}

// WithGitHubRetriever returns a ResolverOption that attaches a GitHubRetriever
// to the Resolver as a fallback source when local and registry lookups fail.
func WithGitHubRetriever(cfg GitHubConfig) ResolverOption {
	return func(r *Resolver) {
		r.githubRetriever = NewGitHubRetriever(cfg, r.cache)
	}
}
