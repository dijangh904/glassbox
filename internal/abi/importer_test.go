// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"encoding/json"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── parseTypeDef ─────────────────────────────────────────────────────────────

func TestParseTypeDef_Primitives(t *testing.T) {
	cases := []struct {
		in  string
		typ xdr.ScSpecType
	}{
		{"Bool", xdr.ScSpecTypeScSpecTypeBool},
		{"Void", xdr.ScSpecTypeScSpecTypeVoid},
		{"U32", xdr.ScSpecTypeScSpecTypeU32},
		{"I32", xdr.ScSpecTypeScSpecTypeI32},
		{"U64", xdr.ScSpecTypeScSpecTypeU64},
		{"I64", xdr.ScSpecTypeScSpecTypeI64},
		{"U128", xdr.ScSpecTypeScSpecTypeU128},
		{"I128", xdr.ScSpecTypeScSpecTypeI128},
		{"U256", xdr.ScSpecTypeScSpecTypeU256},
		{"I256", xdr.ScSpecTypeScSpecTypeI256},
		{"String", xdr.ScSpecTypeScSpecTypeString},
		{"Symbol", xdr.ScSpecTypeScSpecTypeSymbol},
		{"Address", xdr.ScSpecTypeScSpecTypeAddress},
		{"MuxedAddress", xdr.ScSpecTypeScSpecTypeMuxedAddress},
		{"Bytes", xdr.ScSpecTypeScSpecTypeBytes},
		{"Timepoint", xdr.ScSpecTypeScSpecTypeTimepoint},
		{"Duration", xdr.ScSpecTypeScSpecTypeDuration},
		{"Val", xdr.ScSpecTypeScSpecTypeVal},
		{"Error", xdr.ScSpecTypeScSpecTypeError},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			td, err := parseTypeDef(c.in)
			require.NoError(t, err)
			assert.Equal(t, c.typ, td.Type)
		})
	}
}

func TestParseTypeDef_Option(t *testing.T) {
	td, err := parseTypeDef("Option<U64>")
	require.NoError(t, err)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeOption, td.Type)
	require.NotNil(t, td.Option)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeU64, td.Option.ValueType.Type)
}

func TestParseTypeDef_Vec(t *testing.T) {
	td, err := parseTypeDef("Vec<Address>")
	require.NoError(t, err)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeVec, td.Type)
	require.NotNil(t, td.Vec)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeAddress, td.Vec.ElementType.Type)
}

func TestParseTypeDef_Map(t *testing.T) {
	td, err := parseTypeDef("Map<String, U128>")
	require.NoError(t, err)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeMap, td.Type)
	require.NotNil(t, td.Map)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeString, td.Map.KeyType.Type)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeU128, td.Map.ValueType.Type)
}

func TestParseTypeDef_Result(t *testing.T) {
	td, err := parseTypeDef("Result<U32, Error>")
	require.NoError(t, err)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeResult, td.Type)
	require.NotNil(t, td.Result)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeU32, td.Result.OkType.Type)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeError, td.Result.ErrorType.Type)
}

func TestParseTypeDef_BytesN(t *testing.T) {
	td, err := parseTypeDef("BytesN(32)")
	require.NoError(t, err)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeBytesN, td.Type)
	require.NotNil(t, td.BytesN)
	assert.Equal(t, xdr.Uint32(32), td.BytesN.N)
}

func TestParseTypeDef_Tuple(t *testing.T) {
	td, err := parseTypeDef("(U32, String)")
	require.NoError(t, err)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeTuple, td.Type)
	require.NotNil(t, td.Tuple)
	require.Len(t, td.Tuple.ValueTypes, 2)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeU32, td.Tuple.ValueTypes[0].Type)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeString, td.Tuple.ValueTypes[1].Type)
}

func TestParseTypeDef_UDT(t *testing.T) {
	td, err := parseTypeDef("MyStruct")
	require.NoError(t, err)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeUdt, td.Type)
	require.NotNil(t, td.Udt)
	assert.Equal(t, "MyStruct", td.Udt.Name)
}

func TestParseTypeDef_NestedMap(t *testing.T) {
	// Map<String, Vec<U64>>
	td, err := parseTypeDef("Map<String, Vec<U64>>")
	require.NoError(t, err)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeMap, td.Type)
	require.NotNil(t, td.Map)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeVec, td.Map.ValueType.Type)
}

// ─── ImportFromJSON ───────────────────────────────────────────────────────────

// buildSampleJSONSpec returns a JSON ABI document for a simple token contract.
func buildSampleJSONSpec(t *testing.T) []byte {
	t.Helper()
	js := jsonSpec{
		Functions: []jsonFunction{
			{
				Name: "transfer",
				Doc:  "Transfer tokens.",
				Inputs: []jsonField{
					{Name: "from", Type: "Address"},
					{Name: "to", Type: "Address"},
					{Name: "amount", Type: "U128"},
				},
				Outputs: []string{"Void"},
			},
			{
				Name:    "balance",
				Inputs:  []jsonField{{Name: "account", Type: "Address"}},
				Outputs: []string{"U128"},
			},
		},
		Structs: []jsonStruct{
			{
				Name: "TokenInfo",
				Fields: []jsonField{
					{Name: "name", Type: "String"},
					{Name: "decimals", Type: "U32"},
				},
			},
		},
		Enums: []jsonEnum{
			{
				Name:  "Status",
				Cases: []jsonEnumCase{{Name: "Active", Value: 0}, {Name: "Paused", Value: 1}},
			},
		},
		ErrorEnums: []jsonEnum{
			{
				Name:  "TokenError",
				Cases: []jsonEnumCase{{Name: "InsufficientBalance", Value: 1}},
			},
		},
		Unions: []jsonUnion{
			{
				Name: "Outcome",
				Cases: []jsonUnionCase{
					{Name: "None"},
					{Name: "Value", Types: []string{"U64"}},
				},
			},
		},
		Events: []jsonEvent{
			{
				Name: "Transfer",
				Params: []jsonEventParam{
					{Name: "from", Type: "Address", Location: "topic"},
					{Name: "amount", Type: "U128", Location: "data"},
				},
			},
		},
	}
	data, err := json.Marshal(js)
	require.NoError(t, err)
	return data
}

func TestImportFromJSON_Functions(t *testing.T) {
	spec, err := ImportFromJSON(buildSampleJSONSpec(t))
	require.NoError(t, err)
	require.Len(t, spec.Functions, 2)

	transfer := spec.Functions[0]
	assert.Equal(t, "transfer", string(transfer.Name))
	assert.Equal(t, "Transfer tokens.", transfer.Doc)
	require.Len(t, transfer.Inputs, 3)
	assert.Equal(t, "from", transfer.Inputs[0].Name)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeAddress, transfer.Inputs[0].Type.Type)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeU128, transfer.Inputs[2].Type.Type)
	require.Len(t, transfer.Outputs, 1)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeVoid, transfer.Outputs[0].Type)
}

func TestImportFromJSON_Structs(t *testing.T) {
	spec, err := ImportFromJSON(buildSampleJSONSpec(t))
	require.NoError(t, err)
	require.Len(t, spec.Structs, 1)
	assert.Equal(t, "TokenInfo", string(spec.Structs[0].Name))
	require.Len(t, spec.Structs[0].Fields, 2)
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeU32, spec.Structs[0].Fields[1].Type.Type)
}

func TestImportFromJSON_Enums(t *testing.T) {
	spec, err := ImportFromJSON(buildSampleJSONSpec(t))
	require.NoError(t, err)
	require.Len(t, spec.Enums, 1)
	assert.Equal(t, "Status", string(spec.Enums[0].Name))
	assert.Len(t, spec.Enums[0].Cases, 2)
}

func TestImportFromJSON_ErrorEnums(t *testing.T) {
	spec, err := ImportFromJSON(buildSampleJSONSpec(t))
	require.NoError(t, err)
	require.Len(t, spec.ErrorEnums, 1)
	assert.Equal(t, "TokenError", string(spec.ErrorEnums[0].Name))
}

func TestImportFromJSON_Unions(t *testing.T) {
	spec, err := ImportFromJSON(buildSampleJSONSpec(t))
	require.NoError(t, err)
	require.Len(t, spec.Unions, 1)
	u := spec.Unions[0]
	assert.Equal(t, "Outcome", string(u.Name))
	require.Len(t, u.Cases, 2)
	assert.Equal(t, xdr.ScSpecUdtUnionCaseV0KindScSpecUdtUnionCaseVoidV0, u.Cases[0].Kind)
	assert.Equal(t, xdr.ScSpecUdtUnionCaseV0KindScSpecUdtUnionCaseTupleV0, u.Cases[1].Kind)
}

func TestImportFromJSON_Events(t *testing.T) {
	spec, err := ImportFromJSON(buildSampleJSONSpec(t))
	require.NoError(t, err)
	require.Len(t, spec.Events, 1)
	ev := spec.Events[0]
	assert.Equal(t, "Transfer", string(ev.Name))
	require.Len(t, ev.Params, 2)
	assert.Equal(t, xdr.ScSpecEventParamLocationV0ScSpecEventParamLocationTopicList, ev.Params[0].Location)
	assert.Equal(t, xdr.ScSpecEventParamLocationV0ScSpecEventParamLocationData, ev.Params[1].Location)
}

func TestImportFromJSON_InvalidJSON(t *testing.T) {
	_, err := ImportFromJSON([]byte(`{not valid json`))
	require.Error(t, err)
}

// ─── ImportFromXDR ────────────────────────────────────────────────────────────

func TestImportFromXDR_RoundTrip(t *testing.T) {
	// Build a spec, marshal it to XDR, then re-import it.
	fn := xdr.ScSpecFunctionV0{
		Name: "hello",
		Inputs: []xdr.ScSpecFunctionInputV0{
			{Name: "to", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
		},
		Outputs: []xdr.ScSpecTypeDef{{Type: xdr.ScSpecTypeScSpecTypeVoid}},
	}
	entry := xdr.ScSpecEntry{Kind: xdr.ScSpecEntryKindScSpecEntryFunctionV0, FunctionV0: &fn}
	data := marshalEntries(t, entry)

	spec, err := ImportFromXDR(data)
	require.NoError(t, err)
	require.Len(t, spec.Functions, 1)
	assert.Equal(t, "hello", string(spec.Functions[0].Name))
}

func TestImportFromXDR_InvalidData(t *testing.T) {
	_, err := ImportFromXDR([]byte{0xFF, 0xFE, 0xFD})
	require.Error(t, err)
}

// ─── DetectFormat ─────────────────────────────────────────────────────────────

func TestDetectFormat(t *testing.T) {
	assert.Equal(t, ImportFormatJSON, DetectFormat([]byte(`{"functions":[]}`)))
	assert.Equal(t, ImportFormatJSON, DetectFormat([]byte("  { }")))
	assert.Equal(t, ImportFormatXDR, DetectFormat([]byte{0x00, 0x01, 0x02}))
	assert.Equal(t, ImportFormatXDR, DetectFormat([]byte{}))
}

// ─── JSON round-trip via FormatJSON → ImportFromJSON ──────────────────────────

func TestJSONRoundTrip(t *testing.T) {
	// Build a ContractSpec, serialise to JSON via FormatJSON, then re-import.
	original := &ContractSpec{
		Functions: []xdr.ScSpecFunctionV0{
			{
				Name: "mint",
				Inputs: []xdr.ScSpecFunctionInputV0{
					{Name: "to", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
					{Name: "amount", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU128}},
				},
				Outputs: []xdr.ScSpecTypeDef{{Type: xdr.ScSpecTypeScSpecTypeVoid}},
			},
		},
		Enums: []xdr.ScSpecUdtEnumV0{
			{
				Name:  "Role",
				Cases: []xdr.ScSpecUdtEnumCaseV0{{Name: "Admin", Value: 0}},
			},
		},
	}

	jsonStr, err := FormatJSON(original)
	require.NoError(t, err)

	reimported, err := ImportFromJSON([]byte(jsonStr))
	require.NoError(t, err)

	require.Len(t, reimported.Functions, 1)
	assert.Equal(t, "mint", string(reimported.Functions[0].Name))
	assert.Equal(t, xdr.ScSpecTypeScSpecTypeU128, reimported.Functions[0].Inputs[1].Type.Type)

	require.Len(t, reimported.Enums, 1)
	assert.Equal(t, "Role", string(reimported.Enums[0].Name))
}
