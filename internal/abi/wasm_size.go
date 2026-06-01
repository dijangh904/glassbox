// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"fmt"

	"github.com/dotandev/glassbox/internal/errors"
)

// Size thresholds for WASM binary analysis.
const (
	// WasmSizeWarnThreshold is the total binary size (bytes) above which a
	// warning is emitted. 256 KiB is a practical upper bound for well-optimised
	// Soroban contracts.
	WasmSizeWarnThreshold = 256 * 1024

	// WasmSizeCriticalThreshold is the total binary size (bytes) above which a
	// critical warning is emitted. Binaries this large are likely to be rejected
	// or incur high transaction fees on-chain.
	WasmSizeCriticalThreshold = 512 * 1024

	// WasmSectionWarnThreshold is the maximum size (bytes) of any single WASM
	// section before a per-section warning is added.
	WasmSectionWarnThreshold = 128 * 1024
)

// wasmSectionNames maps standard WASM section IDs to human-readable names.
var wasmSectionNames = map[uint8]string{
	0:  "custom",
	1:  "type",
	2:  "import",
	3:  "function",
	4:  "table",
	5:  "memory",
	6:  "global",
	7:  "export",
	8:  "start",
	9:  "element",
	10: "code",
	11: "data",
	12: "data_count",
}

// WasmSectionInfo records the ID, name, and byte size of one WASM section.
type WasmSectionInfo struct {
	ID   uint8
	Name string
	Size uint32
}

// WasmSizeAnalysis is the result of AnalyzeWasmSize.
type WasmSizeAnalysis struct {
	// TotalSize is the full length of the WASM binary in bytes.
	TotalSize int
	// Sections lists every section found in the binary.
	Sections []WasmSectionInfo
	// Warnings contains human-readable messages for oversized sections or totals.
	Warnings []string
}

// HasWarnings reports whether the analysis produced any size warnings.
func (a *WasmSizeAnalysis) HasWarnings() bool {
	return len(a.Warnings) > 0
}

// AnalyzeWasmSize inspects the section layout of a WASM binary and returns
// size metrics together with any threshold warnings.
//
// The function validates the WASM magic header; non-WASM input returns an
// error. It does not modify the input slice.
func AnalyzeWasmSize(wasm []byte) (*WasmSizeAnalysis, error) {
	if len(wasm) < 8 {
		return nil, errors.WrapWasmInvalid("binary too short to be valid WASM")
	}
	if wasm[0] != wasmMagic[0] || wasm[1] != wasmMagic[1] ||
		wasm[2] != wasmMagic[2] || wasm[3] != wasmMagic[3] {
		return nil, errors.WrapWasmInvalid("missing WASM magic bytes")
	}

	analysis := &WasmSizeAnalysis{
		TotalSize: len(wasm),
	}

	offset := 8 // skip magic + version
	for offset < len(wasm) {
		sectionID := wasm[offset]
		offset++

		sectionLen, n, err := decodeLEB128(wasm, offset)
		if err != nil {
			return nil, errors.WrapWasmInvalid(
				fmt.Sprintf("invalid section length at offset %d: %v", offset, err))
		}
		offset += n

		if offset+int(sectionLen) > len(wasm) {
			return nil, errors.WrapWasmInvalid("section extends beyond end of binary")
		}

		name, ok := wasmSectionNames[sectionID]
		if !ok {
			name = fmt.Sprintf("unknown_%d", sectionID)
		}

		analysis.Sections = append(analysis.Sections, WasmSectionInfo{
			ID:   sectionID,
			Name: name,
			Size: sectionLen,
		})

		if sectionLen > WasmSectionWarnThreshold {
			analysis.Warnings = append(analysis.Warnings,
				fmt.Sprintf("section %q is large (%d bytes, threshold %d bytes)",
					name, sectionLen, WasmSectionWarnThreshold))
		}

		offset += int(sectionLen)
	}

	// Total-size warnings
	switch {
	case analysis.TotalSize >= WasmSizeCriticalThreshold:
		analysis.Warnings = append(analysis.Warnings,
			fmt.Sprintf("WASM binary is critically large (%d bytes, critical threshold %d bytes); "+
				"on-chain upload may be rejected",
				analysis.TotalSize, WasmSizeCriticalThreshold))
	case analysis.TotalSize >= WasmSizeWarnThreshold:
		analysis.Warnings = append(analysis.Warnings,
			fmt.Sprintf("WASM binary is large (%d bytes, warning threshold %d bytes); "+
				"consider running 'glassbox wasm-optimize' to reduce size",
				analysis.TotalSize, WasmSizeWarnThreshold))
	}

	return analysis, nil
}

// FormatWasmSizeWarnings returns a multi-line string with all warnings from an
// analysis, or an empty string when there are none.
func FormatWasmSizeWarnings(a *WasmSizeAnalysis) string {
	if a == nil || len(a.Warnings) == 0 {
		return ""
	}
	out := fmt.Sprintf("WASM size analysis (%d bytes, %d section(s)):\n",
		a.TotalSize, len(a.Sections))
	for _, w := range a.Warnings {
		out += "  [!] " + w + "\n"
	}
	return out
}
