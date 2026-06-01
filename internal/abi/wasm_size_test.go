// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalWasm returns a minimal valid WASM binary containing a single custom
// section of the given size so we can test threshold logic without touching the
// file system.
func minimalWasm(sectionID uint8, payload []byte) []byte {
	// Build section body
	var body []byte
	if sectionID == 0 {
		// Custom section: prefix with an empty name (LEB128 length = 0)
		body = append(body, 0x00)
	}
	body = append(body, payload...)

	buf := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	buf = append(buf, sectionID)
	buf = appendLEB128(buf, uint32(len(body)))
	buf = append(buf, body...)
	return buf
}

func TestAnalyzeWasmSize_TooShort(t *testing.T) {
	_, err := AnalyzeWasmSize([]byte{0x00, 0x61})
	assert.Error(t, err)
}

func TestAnalyzeWasmSize_BadMagic(t *testing.T) {
	bad := []byte{0x01, 0x02, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}
	_, err := AnalyzeWasmSize(bad)
	assert.Error(t, err)
}

func TestAnalyzeWasmSize_EmptyPayload(t *testing.T) {
	wasm := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	a, err := AnalyzeWasmSize(wasm)
	require.NoError(t, err)
	assert.Equal(t, 8, a.TotalSize)
	assert.Empty(t, a.Sections)
	assert.Empty(t, a.Warnings)
}

func TestAnalyzeWasmSize_SectionNames(t *testing.T) {
	// code section (id=10) with minimal payload
	wasm := minimalWasm(10, []byte{0x00})
	a, err := AnalyzeWasmSize(wasm)
	require.NoError(t, err)
	require.Len(t, a.Sections, 1)
	assert.Equal(t, uint8(10), a.Sections[0].ID)
	assert.Equal(t, "code", a.Sections[0].Name)
}

func TestAnalyzeWasmSize_UnknownSectionID(t *testing.T) {
	wasm := minimalWasm(200, []byte{0xAA, 0xBB})
	a, err := AnalyzeWasmSize(wasm)
	require.NoError(t, err)
	require.Len(t, a.Sections, 1)
	assert.Equal(t, uint8(200), a.Sections[0].ID)
	assert.Contains(t, a.Sections[0].Name, "unknown")
}

func TestAnalyzeWasmSize_NoWarningsUnderThreshold(t *testing.T) {
	wasm := minimalWasm(1, make([]byte, 1024)) // 1 KB, well under warning threshold
	a, err := AnalyzeWasmSize(wasm)
	require.NoError(t, err)
	assert.False(t, a.HasWarnings())
}

func TestAnalyzeWasmSize_SectionWarning(t *testing.T) {
	// Create a section larger than WasmSectionWarnThreshold
	payload := make([]byte, WasmSectionWarnThreshold+1)
	wasm := minimalWasm(10, payload)
	a, err := AnalyzeWasmSize(wasm)
	require.NoError(t, err)
	assert.True(t, a.HasWarnings())
	found := false
	for _, w := range a.Warnings {
		if strings.Contains(w, "large") {
			found = true
		}
	}
	assert.True(t, found, "expected a section-large warning")
}

func TestAnalyzeWasmSize_TotalWarnThreshold(t *testing.T) {
	// Build a WASM just over the warn threshold but under critical
	payload := make([]byte, WasmSizeWarnThreshold)
	wasm := minimalWasm(11, payload) // data section
	a, err := AnalyzeWasmSize(wasm)
	require.NoError(t, err)
	assert.True(t, a.HasWarnings())
	found := false
	for _, w := range a.Warnings {
		if strings.Contains(w, "large") && strings.Contains(w, "optimize") {
			found = true
		}
	}
	assert.True(t, found, "expected total-size warning with optimize hint")
}

func TestAnalyzeWasmSize_TotalCriticalThreshold(t *testing.T) {
	payload := make([]byte, WasmSizeCriticalThreshold)
	wasm := minimalWasm(11, payload)
	a, err := AnalyzeWasmSize(wasm)
	require.NoError(t, err)
	found := false
	for _, w := range a.Warnings {
		if strings.Contains(w, "critically large") {
			found = true
		}
	}
	assert.True(t, found, "expected critical size warning")
}

func TestFormatWasmSizeWarnings_NoWarnings(t *testing.T) {
	a := &WasmSizeAnalysis{TotalSize: 100}
	assert.Equal(t, "", FormatWasmSizeWarnings(a))
}

func TestFormatWasmSizeWarnings_Nil(t *testing.T) {
	assert.Equal(t, "", FormatWasmSizeWarnings(nil))
}

func TestFormatWasmSizeWarnings_WithWarnings(t *testing.T) {
	a := &WasmSizeAnalysis{
		TotalSize: 300 * 1024,
		Sections:  []WasmSectionInfo{{ID: 10, Name: "code", Size: 300 * 1024}},
		Warnings:  []string{"section \"code\" is large (307200 bytes, threshold 131072 bytes)"},
	}
	out := FormatWasmSizeWarnings(a)
	assert.Contains(t, out, "[!]")
	assert.Contains(t, out, "code")
}
