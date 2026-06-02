// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"testing"

	"github.com/dotandev/glassbox/internal/config"
)

func TestRegistryFromConfig(t *testing.T) {
	cfg := &config.Config{
		ExternalSourceRepos: []config.ExternalSourceRepo{{
			Prefix:    "/vendor/lib",
			RemoteURL: "https://github.com/example/lib",
			Branch:    "main",
		}},
	}
	reg := RegistryFromConfig(cfg)
	if reg == nil {
		t.Fatal("expected registry")
	}
	url, err := reg.GitHubURL("/vendor/lib/src/lib.rs")
	if err != nil {
		t.Fatalf("GitHubURL: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty url")
	}
}
