// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGitHub builds a minimal test server that simulates the GitHub API and
// raw content delivery needed by GitHubRetriever.
type fakeGitHub struct {
	defaultBranch string
	files         map[string]string // path -> content
}

func (f *fakeGitHub) handler() http.Handler {
	mux := http.NewServeMux()

	// Repo metadata endpoint.
	mux.HandleFunc("/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		branch := f.defaultBranch
		if branch == "" {
			branch = "main"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"default_branch": branch,
		})
	})

	// Tree endpoint.
	mux.HandleFunc("/repos/owner/repo/git/trees/", func(w http.ResponseWriter, r *http.Request) {
		type treeEntry struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
			Size int    `json:"size"`
		}
		var entries []treeEntry
		for path, content := range f.files {
			entries = append(entries, treeEntry{
				Path: path,
				Type: "blob",
				Size: len(content),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"tree":      entries,
			"truncated": false,
		})
	})

	// Raw file content endpoint.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Strip leading "/owner/repo/<branch>/" prefix.
		path := r.URL.Path
		for _, prefix := range []string{"/owner/repo/main/", "/owner/repo/feature/"} {
			if len(path) > len(prefix) && path[:len(prefix)] == prefix {
				filePath := path[len(prefix):]
				if content, ok := f.files[filePath]; ok {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(content))
					return
				}
			}
		}
		http.NotFound(w, r)
	})

	return mux
}

func newTestRetriever(t *testing.T, fg *fakeGitHub, cache *SourceCache) (*GitHubRetriever, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(fg.handler())
	t.Cleanup(srv.Close)

	cfg := GitHubConfig{
		Repository:     "owner/repo",
		RequestTimeout: 5 * time.Second,
	}
	ret := NewGitHubRetriever(cfg, cache)
	// Point the HTTP client at our test server.
	ret.client = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &rewriteTransport{
			base:   http.DefaultTransport,
			apiURL: srv.URL,
			rawURL: srv.URL,
		},
	}
	return ret, srv
}

// rewriteTransport rewrites github.com and raw.githubusercontent.com URLs to
// point at a local test server.
type rewriteTransport struct {
	base   http.RoundTripper
	apiURL string
	rawURL string
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	switch req.URL.Host {
	case "api.github.com":
		clone.URL.Scheme = "http"
		clone.URL.Host = rt.apiURL[7:] // strip "http://"
	case "raw.githubusercontent.com":
		clone.URL.Scheme = "http"
		clone.URL.Host = rt.rawURL[7:]
	}
	return rt.base.RoundTrip(clone)
}

func TestGitHubRetriever_Disabled(t *testing.T) {
	ret := NewGitHubRetriever(GitHubConfig{}, nil)
	src, err := ret.Retrieve(context.Background(), "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC")
	require.NoError(t, err)
	assert.Nil(t, src, "expected nil when no repository is configured")
}

func TestGitHubRetriever_FetchesSourceFiles(t *testing.T) {
	fg := &fakeGitHub{
		defaultBranch: "main",
		files: map[string]string{
			"src/lib.rs":   "fn hello() {}",
			"Cargo.toml":   `[package]\nname = "contract"`,
			"README.md":    "not a source file",
		},
	}
	ret, _ := newTestRetriever(t, fg, nil)

	src, err := ret.Retrieve(context.Background(), "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC")
	require.NoError(t, err)
	require.NotNil(t, src)
	assert.Equal(t, "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC", src.ContractID)
	assert.Contains(t, src.Files, "src/lib.rs")
	assert.Contains(t, src.Files, "Cargo.toml")
	assert.NotContains(t, src.Files, "README.md", "non-source files should be excluded")
}

func TestGitHubRetriever_CachesResult(t *testing.T) {
	fg := &fakeGitHub{
		defaultBranch: "main",
		files:         map[string]string{"src/lib.rs": "fn main() {}"},
	}
	cache, err := NewSourceCache(t.TempDir())
	require.NoError(t, err)

	ret, srv := newTestRetriever(t, fg, cache)

	contractID := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	src1, err := ret.Retrieve(context.Background(), contractID)
	require.NoError(t, err)
	require.NotNil(t, src1)

	// Close the server so the second call must use cache.
	srv.Close()

	src2, err := ret.Retrieve(context.Background(), contractID)
	require.NoError(t, err)
	require.NotNil(t, src2, "expected cache hit after server close")
	assert.Equal(t, src1.Files, src2.Files)
}

func TestGitHubRetriever_UsesConfiguredRevision(t *testing.T) {
	fg := &fakeGitHub{
		files: map[string]string{"src/lib.rs": "// pinned revision"},
	}
	srv := httptest.NewServer(fg.handler())
	defer srv.Close()

	cfg := GitHubConfig{
		Repository:     "owner/repo",
		Revision:       "feature",
		RequestTimeout: 5 * time.Second,
	}
	ret := NewGitHubRetriever(cfg, nil)
	ret.client = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &rewriteTransport{
			base:   http.DefaultTransport,
			apiURL: srv.URL,
			rawURL: srv.URL,
		},
	}

	src, err := ret.Retrieve(context.Background(), "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC")
	require.NoError(t, err)
	require.NotNil(t, src)
}

func TestGitHubRetriever_RetrieveFile_SHAValidation(t *testing.T) {
	content := "fn transfer() {}"
	hash := sha256.Sum256([]byte(content))
	correctSHA := hex.EncodeToString(hash[:])

	fg := &fakeGitHub{
		defaultBranch: "main",
		files:         map[string]string{"src/lib.rs": content},
	}
	ret, _ := newTestRetriever(t, fg, nil)

	// Correct SHA must pass.
	got, err := ret.RetrieveFile(context.Background(), "src/lib.rs", correctSHA)
	require.NoError(t, err)
	assert.Equal(t, content, got)

	// Wrong SHA must fail.
	_, err = ret.RetrieveFile(context.Background(), "src/lib.rs", "deadbeef")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SHA validation failed")
}

func TestGitHubRetriever_RetrieveFile_NoSHAValidation(t *testing.T) {
	fg := &fakeGitHub{
		defaultBranch: "main",
		files:         map[string]string{"Cargo.toml": "[package]"},
	}
	ret, _ := newTestRetriever(t, fg, nil)

	got, err := ret.RetrieveFile(context.Background(), "Cargo.toml", "")
	require.NoError(t, err)
	assert.Equal(t, "[package]", got)
}

func TestGitHubRetriever_RetrieveFile_NotFound(t *testing.T) {
	fg := &fakeGitHub{
		defaultBranch: "main",
		files:         map[string]string{},
	}
	ret, _ := newTestRetriever(t, fg, nil)

	_, err := ret.RetrieveFile(context.Background(), "missing.rs", "")
	assert.Error(t, err)
}

func TestGitHubRetriever_InvalidRepository(t *testing.T) {
	cfg := GitHubConfig{Repository: "not-a-valid-url!!!???"}
	ret := NewGitHubRetriever(cfg, nil)
	_, err := ret.Retrieve(context.Background(), "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC")
	assert.Error(t, err)
}

func TestValidateContentSHA_Match(t *testing.T) {
	data := []byte("hello")
	hash := sha256.Sum256(data)
	err := validateContentSHA(data, hex.EncodeToString(hash[:]))
	assert.NoError(t, err)
}

func TestValidateContentSHA_PrefixMatch(t *testing.T) {
	data := []byte("hello")
	hash := sha256.Sum256(data)
	prefix := hex.EncodeToString(hash[:])[:12]
	err := validateContentSHA(data, prefix)
	assert.NoError(t, err)
}

func TestValidateContentSHA_Mismatch(t *testing.T) {
	err := validateContentSHA([]byte("hello"), "deadbeef")
	assert.Error(t, err)
}
