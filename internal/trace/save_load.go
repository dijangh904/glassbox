// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dotandev/glassbox/internal/errors"
)

// SaveToFile saves an ExecutionTrace to a JSON file
func (t *ExecutionTrace) SaveToFile(path string) error {
	if t == nil {
		return errors.WrapValidationError("trace is nil")
	}

	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return errors.WrapMarshalFailed(err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to write trace file: %v", err))
	}

	return nil
}

// LoadExecutionTrace loads an ExecutionTrace from a JSON file
func LoadExecutionTrace(path string) (*ExecutionTrace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.WrapValidationError(fmt.Sprintf("failed to read trace file: %v", err))
	}

	var trace ExecutionTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "execution trace file")
	}

	return &trace, nil
}
