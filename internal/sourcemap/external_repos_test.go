// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExternalRepoRegistry_GitHubURL(t *testing.T) {
	root := t.TempDir()
	vendor := filepath.Join(root, "vendor", "lib")
	if err := os.MkdirAll(filepath.Join(vendor, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(vendor, "src", "token.rs")
	if err := os.WriteFile(srcFile, []byte("fn main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	reg := NewExternalRepoRegistry([]ExternalRepoMapping{{
		Prefix:    vendor,
		RemoteURL: "https://github.com/example/external-lib.git",
		Branch:    "develop",
	}})

	url, err := reg.GitHubURL(srcFile)
	if err != nil {
		t.Fatalf("GitHubURL: %v", err)
	}
	want := "https://github.com/example/external-lib/blob/develop/src/token.rs"
	if url != want {
		t.Fatalf("url = %q, want %q", url, want)
	}
}

func TestResolveGitHubURL_ExternalFallback(t *testing.T) {
	root := t.TempDir()
	vendor := filepath.Join(root, "vendor", "lib")
	if err := os.MkdirAll(filepath.Join(vendor, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(vendor, "src", "lib.rs")
	if err := os.WriteFile(srcFile, []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	reg := NewExternalRepoRegistry([]ExternalRepoMapping{{
		Prefix:    vendor,
		RemoteURL: "https://github.com/example/external-lib",
		Branch:    "main",
	}})

	url, err := reg.GitHubURL(srcFile)
	if err != nil {
		t.Fatalf("GitHubURL: %v", err)
	}
	if !contains(url, "github.com/example/external-lib/blob/main/src/lib.rs") {
		t.Fatalf("unexpected url: %s", url)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
