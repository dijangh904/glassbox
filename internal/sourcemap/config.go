// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import "github.com/dotandev/glassbox/internal/config"

// RegistryFromConfig builds an external repo registry from application config.
func RegistryFromConfig(cfg *config.Config) *ExternalRepoRegistry {
	if cfg == nil || len(cfg.ExternalSourceRepos) == 0 {
		return nil
	}
	mappings := make([]ExternalRepoMapping, 0, len(cfg.ExternalSourceRepos))
	for _, r := range cfg.ExternalSourceRepos {
		mappings = append(mappings, ExternalRepoMapping{
			Prefix:    r.Prefix,
			RemoteURL: r.RemoteURL,
			Branch:    r.Branch,
		})
	}
	return NewExternalRepoRegistry(mappings)
}
