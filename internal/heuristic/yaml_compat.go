// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package heuristic

import "go.yaml.in/yaml/v2"

// yamlUnmarshal is a thin wrapper around the YAML library so that engine.go
// and loader.go can call it without importing the library directly.
func yamlUnmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}
