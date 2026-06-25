// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dotandev/glassbox/internal/errors"
)

var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6d} // \0asm

// ValidateWasmMagic performs a fast pre-flight check that data starts with the
// WASM magic bytes and has a plausible minimum length. It does not parse the
// full binary structure — use AnalyzeWasmSize for that.
//
// Returns nil when the data looks like a valid WASM binary, or an error with an
// actionable message suitable for display to the user.
func ValidateWasmMagic(data []byte, path string) error {
	if len(data) < 8 {
		return fmt.Errorf("%s: file is too small to be a valid WASM binary (%d bytes; minimum 8)", path, len(data))
	}
	if data[0] != wasmMagic[0] || data[1] != wasmMagic[1] ||
		data[2] != wasmMagic[2] || data[3] != wasmMagic[3] {
		return fmt.Errorf("%s: not a valid WASM binary (bad magic bytes — expected \\0asm; "+
			"make sure you are passing a compiled .wasm file, not a source file or other binary)", path)
	}
	return nil
}

// ExtractCustomSection parses a WASM binary and returns the payload of the
// custom section with the given name. Returns (nil, nil) if the section is
// not present.
func ExtractCustomSection(wasm []byte, name string) ([]byte, error) {
	if len(wasm) < 8 {
		return nil, errors.WrapWasmInvalid("file too short")
	}
	if wasm[0] != wasmMagic[0] || wasm[1] != wasmMagic[1] ||
		wasm[2] != wasmMagic[2] || wasm[3] != wasmMagic[3] {
		return nil, errors.WrapWasmInvalid("bad magic bytes")
	}
	// bytes 4-7: version (we accept any version)

	offset := 8
	for offset < len(wasm) {
		sectionID := wasm[offset]
		offset++

		sectionLen, n, err := decodeLEB128(wasm, offset)
		if err != nil {
			return nil, errors.WrapWasmInvalid(fmt.Sprintf("bad section length at offset %d: %v", offset, err))
		}
		offset += n

		if offset+int(sectionLen) > len(wasm) {
			return nil, errors.WrapWasmInvalid("section extends past end of file")
		}

		sectionEnd := offset + int(sectionLen)

		if sectionID == 0 { // custom section
			nameLen, nn, err := decodeLEB128(wasm, offset)
			if err != nil {
				return nil, errors.WrapWasmInvalid(fmt.Sprintf("bad custom section name length: %v", err))
			}
			offset += nn

			if offset+int(nameLen) > sectionEnd {
				return nil, errors.WrapWasmInvalid("custom section name extends past section")
			}

			sectionName := string(wasm[offset : offset+int(nameLen)])
			offset += int(nameLen)

			if sectionName == name {
				payload := make([]byte, sectionEnd-offset)
				copy(payload, wasm[offset:sectionEnd])
				return payload, nil
			}
		}

		offset = sectionEnd
	}

	return nil, nil
}

// decodeLEB128 decodes an unsigned LEB128 integer from wasm at the given
// offset. Returns the value, the number of bytes consumed, and any error.
func decodeLEB128(data []byte, offset int) (uint32, int, error) {
	var result uint32
	var shift uint
	for i := 0; i < 5; i++ { // u32 needs at most 5 bytes
		if offset+i >= len(data) {
			return 0, 0, io.ErrUnexpectedEOF
		}
		b := data[offset+i]
		result |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, i + 1, nil
		}
		shift += 7
	}
	return 0, 0, fmt.Errorf("LEB128 integer too large")
}
// ExtractContractMetadata extracts metadata from WASM custom sections.
// It looks for "contractmeta" and "soroban" sections that may contain
// metadata extracted from Soroban contract manifests or annotations.
func ExtractContractMetadata(wasm []byte) (*ContractMetadata, error) {
	metadata := &ContractMetadata{}

	// Try to extract from contractmeta section (custom section with metadata)
	contractMetaData, err := ExtractCustomSection(wasm, "contractmeta")
	if err != nil {
		return nil, fmt.Errorf("extracting contractmeta section: %w", err)
	}
	if contractMetaData != nil {
		parseContractMetaData(contractMetaData, metadata)
	}

	// Try to extract from soroban section (contains Soroban-specific metadata)
	sorobanData, err := ExtractCustomSection(wasm, "soroban")
	if err != nil {
		return nil, fmt.Errorf("extracting soroban section: %w", err)
	}
	if sorobanData != nil {
		parseSorobanMetadata(sorobanData, metadata)
	}

	// Try to extract from name section (WASM name section with function names)
	nameData, err := ExtractCustomSection(wasm, "name")
	if err != nil {
		return nil, fmt.Errorf("extracting name section: %w", err)
	}
	if nameData != nil {
		parseNameSection(nameData, metadata)
	}

	return metadata, nil
}

// parseContractMetaData parses the contractmeta custom section.
func parseContractMetaData(data []byte, metadata *ContractMetadata) {
	// The contractmeta section typically contains JSON or key-value pairs
	// Try parsing as JSON first
	var metaMap map[string]interface{}
	if err := json.Unmarshal(data, &metaMap); err == nil {
		if v, ok := metaMap["name"].(string); ok {
			metadata.Name = v
		}
		if v, ok := metaMap["version"].(string); ok {
			metadata.Version = v
		}
		if v, ok := metaMap["description"].(string); ok {
			metadata.Description = v
		}
		if v, ok := metaMap["author"].(string); ok {
			metadata.Author = v
		}
		if v, ok := metaMap["license"].(string); ok {
			metadata.License = v
		}
		if v, ok := metaMap["source"].(string); ok {
			metadata.SourceFile = v
		}
		if bi, ok := metaMap["build_info"].(map[string]interface{}); ok {
			if v, ok := bi["rust_version"].(string); ok {
				metadata.BuildInfo.RustVersion = v
			}
			if v, ok := bi["cargo_version"].(string); ok {
				metadata.BuildInfo.CargoVersion = v
			}
			if v, ok := bi["timestamp"].(string); ok {
				metadata.BuildInfo.BuildTimestamp = v
			}
			if v, ok := bi["profile"].(string); ok {
				metadata.BuildInfo.Profile = v
			}
			if v, ok := bi["target"].(string); ok {
				metadata.BuildInfo.Target = v
			}
		}
		return
	}

	// Fallback: try parsing as simple key=value format
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "name":
				metadata.Name = value
			case "version":
				metadata.Version = value
			case "description":
				metadata.Description = value
			case "author":
				metadata.Author = value
			case "license":
				metadata.License = value
			case "source":
				metadata.SourceFile = value
			case "rust_version":
				metadata.BuildInfo.RustVersion = value
			case "cargo_version":
				metadata.BuildInfo.CargoVersion = value
			case "build_timestamp":
				metadata.BuildInfo.BuildTimestamp = value
			case "profile":
				metadata.BuildInfo.Profile = value
			case "target":
				metadata.BuildInfo.Target = value
			}
		}
	}
}

// parseSorobanMetadata parses the soroban custom section.
func parseSorobanMetadata(data []byte, metadata *ContractMetadata) {
	// Soroban sections may contain contract name and other info
	var sorobanMeta struct {
		Contract struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"desc"`
		} `json:"contract"`
	}

	if err := json.Unmarshal(data, &sorobanMeta); err == nil {
		if metadata.Name == "" && sorobanMeta.Contract.Name != "" {
			metadata.Name = sorobanMeta.Contract.Name
		}
		if metadata.Version == "" && sorobanMeta.Contract.Version != "" {
			metadata.Version = sorobanMeta.Contract.Version
		}
		if metadata.Description == "" && sorobanMeta.Contract.Description != "" {
			metadata.Description = sorobanMeta.Contract.Description
		}
	}
}

// parseNameSection parses the WASM name section to extract the module name.
func parseNameSection(data []byte, metadata *ContractMetadata) {
	// The name section is a custom section with subsection IDs
	// Subsection 0 = module name
	offset := 0

	// Skip section size
	_, n, err := decodeLEB128(data, offset)
	if err != nil {
		return
	}
	offset += n

	// Subsection 0 = module name
	if offset >= len(data) || data[offset] != 0 {
		return
	}
	offset++

	// Name string size
	nameLen, n, err := decodeLEB128(data, offset)
	if err != nil {
		return
	}
	offset += n

	if offset+int(nameLen) > len(data) {
		return
	}

	// If no contract name set, use module name
	if metadata.Name == "" {
		metadata.Name = string(data[offset : offset+int(nameLen)])
	}
}
