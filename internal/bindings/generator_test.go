// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package bindings

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestMapTypeDefToTS(t *testing.T) {
	g := &Generator{}

	tests := []struct {
		name     string
		typeDef  xdr.ScSpecTypeDef
		expected string
	}{
		{"Bool", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeBool}, "boolean"},
		{"U32", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU32}, "number"},
		{"I32", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeI32}, "number"},
		{"U64", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU64}, "bigint"},
		{"I64", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeI64}, "bigint"},
		{"U128", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU128}, "bigint"},
		{"I128", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeI128}, "bigint"},
		{"String", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeString}, "string"},
		{"Symbol", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeSymbol}, "SorobanSymbol"},
		{"Address", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}, "Address"},
		{"MuxedAddress", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeMuxedAddress}, "Address"},
		{"Bytes", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeBytes}, "Bytes"},
		{"Void", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeVoid}, "void"},
		{"Val", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeVal}, "unknown"},
		{
			"Option<string>",
			xdr.ScSpecTypeDef{
				Type:   xdr.ScSpecTypeScSpecTypeOption,
				Option: &xdr.ScSpecTypeOption{ValueType: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeString}},
			},
			"string | null",
		},
		{
			"Vec<Address>",
			xdr.ScSpecTypeDef{
				Type: xdr.ScSpecTypeScSpecTypeVec,
				Vec:  &xdr.ScSpecTypeVec{ElementType: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
			},
			"Array<Address>",
		},
		{
			"Map<string,bigint>",
			xdr.ScSpecTypeDef{
				Type: xdr.ScSpecTypeScSpecTypeMap,
				Map: &xdr.ScSpecTypeMap{
					KeyType:   xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeString},
					ValueType: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU64},
				},
			},
			"Map<string, bigint>",
		},
		{
			"BytesN(32)",
			xdr.ScSpecTypeDef{
				Type:   xdr.ScSpecTypeScSpecTypeBytesN,
				BytesN: &xdr.ScSpecTypeBytesN{N: 32},
			},
			"Uint8Array /* length: 32 */",
		},
		{
			"UDT",
			xdr.ScSpecTypeDef{
				Type: xdr.ScSpecTypeScSpecTypeUdt,
				Udt:  &xdr.ScSpecTypeUdt{Name: "MyStruct"},
			},
			"MyStruct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.mapTypeDefToTS(tt.typeDef)
			if result != tt.expected {
				t.Errorf("mapTypeDefToTS() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello-world", "HelloWorld"},
		{"my_contract", "MyContract"},
		{"simple", "Simple"},
		{"multi-word-test", "MultiWordTest"},
		{"already", "Already"},
		{"UPPER", "Upper"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
