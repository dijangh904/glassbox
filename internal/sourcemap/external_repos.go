// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dotandev/glassbox/internal/pathutil"
)

// ExternalRepoMapping maps a local source path prefix to a remote Git repository.
type ExternalRepoMapping struct {
	// Prefix is the absolute or workspace-relative path prefix for sources in the external repo.
	Prefix string `json:"prefix"`
	// RemoteURL is the GitHub repository URL (HTTPS or git@github.com: form).
	RemoteURL string `json:"remote_url"`
	// Branch is used in blob URLs when set; otherwise "main" is used.
	Branch string `json:"branch,omitempty"`
}

// ExternalRepoRegistry resolves GitHub links for files that live outside the workspace repo.
type ExternalRepoRegistry struct {
	mappings []ExternalRepoMapping
}

// NewExternalRepoRegistry creates a registry from zero or more mappings.
func NewExternalRepoRegistry(mappings []ExternalRepoMapping) *ExternalRepoRegistry {
	normalized := make([]ExternalRepoMapping, 0, len(mappings))
	for _, m := range mappings {
		if m.Prefix == "" || m.RemoteURL == "" {
			continue
		}
		prefix, err := filepath.Abs(m.Prefix)
		if err != nil {
			prefix = filepath.Clean(m.Prefix)
		}
		branch := m.Branch
		if branch == "" {
			branch = "main"
		}
		normalized = append(normalized, ExternalRepoMapping{
			Prefix:    pathutil.Normalize(prefix),
			RemoteURL: m.RemoteURL,
			Branch:    branch,
		})
	}
	return &ExternalRepoRegistry{mappings: normalized}
}

// GitHubURL returns a GitHub blob URL when absFilePath falls under a configured external prefix.
func (r *ExternalRepoRegistry) GitHubURL(absFilePath string) (string, error) {
	if r == nil || len(r.mappings) == 0 {
		return "", fmt.Errorf("no external source mappings configured")
	}
	abs, err := filepath.Abs(absFilePath)
	if err != nil {
		return "", err
	}
	abs = pathutil.Normalize(abs)

	for _, m := range r.mappings {
		prefix := m.Prefix
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		if abs != strings.TrimSuffix(m.Prefix, "/") && !strings.HasPrefix(abs, prefix) {
			continue
		}
		owner, repo, err := parseGitHubURL(m.RemoteURL)
		if err != nil {
			return "", err
		}
		rel, err := pathutil.RelToSlash(m.Prefix, abs)
		if err != nil {
			rel = filepath.Base(abs)
		}
		if strings.HasPrefix(rel, "../") {
			return "", fmt.Errorf("file %q is outside external prefix %q", absFilePath, m.Prefix)
		}
		return fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s",
			owner, repo, m.Branch, rel), nil
	}
	return "", fmt.Errorf("no external mapping for %q", absFilePath)
}

// ResolveGitHubURL tries the workspace GitLinker first, then external mappings.
func ResolveGitHubURL(startPath, filePath string, external *ExternalRepoRegistry) (string, error) {
	abs := filePath
	if !filepath.IsAbs(abs) {
		if linker, err := NewGitLinker(startPath); err == nil {
			if url, err := linker.NormalizeSourcePath(filePath); err == nil {
				return url, nil
			}
		}
		if external != nil {
			joined := pathutil.Join(startPath, filePath)
			if url, err := external.GitHubURL(joined); err == nil {
				return url, nil
			}
		}
		return "", fmt.Errorf("could not resolve GitHub URL for %q", filePath)
	}

	if linker, err := NewGitLinker(abs); err == nil {
		if url, err := linker.GitHubURL(abs); err == nil {
			return url, nil
		}
	}
	if external != nil {
		return external.GitHubURL(abs)
	}
	return "", fmt.Errorf("could not resolve GitHub URL for %q", filePath)
}
