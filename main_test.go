package main

import (
  "bytes"
  "encoding/json"
  "os"
  "os/exec"
  "path/filepath"
  "regexp"
  "sort"
  "strings"
  "testing"
)

func TestExportedNameSplitsReadableIdentifiers(t *testing.T) {
  tests := map[string]string{
    "Storage@me":          "StorageMe",
    "skipBitsNValidation": "SkipBitsNValidation",
    "dnsresolve":          "DNSResolve",
    "metadata_uri":        "MetadataURI",
    "EStoredAsInt8":       "EStoredAsInt8",
  }

  for input, want := range tests {
    if got := exportedName(input); got != want {
      t.Fatalf("exportedName(%q) = %q, want %q", input, got, want)
    }
  }
}

func TestGenerateContractAPINameAndAddressCollision(t *testing.T) {
  abi := abiFile{
    ContractName: "Wallet",
    GetMethods: []getMethod{
      {
        Name:        "address",
        TVMMethodID: 1,
        ReturnType:  abiType{Kind: "uintN", N: 32},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  text := string(src)
  for _, want := range []string{
    `type ContractAPI interface`,
    `func NewWallet\(api ContractAPI, addr \*address\.Address\) \*Wallet`,
    `func \(c \*Wallet\) Address\(\) \*address\.Address`,
    `func \(c \*Wallet\) RunMethodAddress\(ctx context\.Context, block \*ton\.BlockIDExt\) \(uint32, error\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
  for _, notWant := range []string{
    "APIProvider",
    "Address2",
    "func (c *Wallet) Address(ctx context.Context",
  } {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source contains %q:\n%s", notWant, src)
    }
  }
}

func TestGetMethodReceiverNamesUseRunMethodPrefix(t *testing.T) {
  tests := map[string]string{
    "lala":            "RunMethodLala",
    "get_date":        "RunMethodGetDate",
    "run_method_lala": "RunMethodLala",
  }
  for input, want := range tests {
    if got := getMethodReceiverName(input); got != want {
      t.Fatalf("getMethodReceiverName(%q) = %q, want %q", input, got, want)
    }
  }
}

func TestGenerateABIConstants(t *testing.T) {
  const data = `{
		"abi_schema_version": "1.0",
		"contract_name": "Sample",
		"constants": [
			{"name": "CONFIG_PARAM_MANDATORY_PARAMS", "value": {"kind": "int", "v": "9"}},
			{"name": "MAX_ORDER_SEQNO", "value": {"kind": "int", "v": "115792089237316195423570985008687907853269984665640564039457584007913129639935"}},
			{"name": "ADMIN_ADDRESS_FOR_CUSTOM_PARAMS", "value": {"kind": "address", "addr": "EQA_o6NFLu73wozeYNERTsW8lkU5OarbRbIkoNuWdy5SPDA_"}},
			{"name": "COMMENT_APPROVE", "value": {"kind": "slice", "hex": "617070726f7665"}},
			{"name": "ONE_TON", "value": {"kind": "castTo", "inner": {"kind": "int", "v": "1000000000"}, "cast_to": {"kind": "coins"}}}
		]
	}`

  result, err := GenerateJSON([]byte(data), Options{PackageName: "sample"})
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `func ConstConfigParamMandatoryParams\(\) \*big\.Int \{\s+return big\.NewInt\(9\)\s+\}`,
    `func ConstMaxOrderSeqno\(\) \*big\.Int \{\s+return tugenMustBigInt\("115792089237316195423570985008687907853269984665640564039457584007913129639935"\)\s+\}`,
    `func ConstAdminAddressForCustomParams\(\) \*address\.Address \{\s+return address\.MustParseAddr\("EQA_o6NFLu73wozeYNERTsW8lkU5OarbRbIkoNuWdy5SPDA_"\)\s+\}`,
    `func ConstCommentApprove\(\) \*cell\.Slice \{\s+return cell\.BeginCell\(\)\.MustStoreSlice\(\[\]byte\{0x61, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x65\}, 56\)\.ToSlice\(\)\s+\}`,
    `func ConstOneTON\(\) tlb\.Coins \{\s+return tlb\.FromNanoTON\(big\.NewInt\(1000000000\)\)\s+\}`,
    `func tugenMustBigInt\(value string\) \*big\.Int`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTLBStringArrayNullable(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Payload",
        Fields: []field{
          {Name: "id", Type: abiType{Kind: "uintN", N: 32}},
        },
      },
      {
        Kind: "struct",
        Name: "Model",
        Fields: []field{
          {Name: "name", Type: abiType{Kind: "string"}},
          {Name: "delta", Type: abiType{Kind: "varintN", N: 16}},
          {Name: "tags", Type: abiType{Kind: "arrayOf", Inner: &abiType{Kind: "string"}}},
          {Name: "maybeName", Type: abiType{Kind: "nullable", Inner: &abiType{Kind: "string"}}},
          {Name: "maybeItems", Type: abiType{Kind: "nullable", Inner: &abiType{Kind: "arrayOf", Inner: &abiType{Kind: "uintN", N: 8}}}},
          {Name: "maybePayload", Type: abiType{Kind: "nullable", Inner: &abiType{Kind: "StructRef", StructName: "Payload"}}},
          {Name: "external", Type: abiType{Kind: "addressExt"}},
          {Name: "tail", Type: abiType{Kind: "remaining"}},
        },
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `Name\s+string\s+` + "`" + `tlb:"string"` + "`",
    `Delta\s+\*big\.Int\s+` + "`" + `tlb:"var int 16"` + "`",
    `Tags\s+\[\]string\s+` + "`" + `tlb:"array string"` + "`",
    `MaybeName\s+\*string\s+` + "`" + `tlb:"maybe string"` + "`",
    `MaybeItems\s+\[\]uint8\s+` + "`" + `tlb:"maybe array ## 8"` + "`",
    `MaybePayload\s+\*Payload\s+` + "`" + `tlb:"maybe \."` + "`",
    `External\s+\*address\.Address\s+` + "`" + `tlb:"addr ext required"` + "`",
    `Tail\s+\*cell\.Cell\s+` + "`" + `tlb:"\."` + "`",
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTLBCellOfOptionalPayloadWrappers(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Model",
        Fields: []field{
          {
            Name: "plainAddressCell",
            Type: abiType{
              Kind:  "cellOf",
              Inner: &abiType{Kind: "address"},
            },
          },
          {
            Name: "addressCell",
            Type: abiType{
              Kind:  "cellOf",
              Inner: &abiType{Kind: "addressOpt"},
            },
          },
          {
            Name: "maybeAddressCell",
            Type: abiType{
              Kind: "nullable",
              Inner: &abiType{
                Kind:  "cellOf",
                Inner: &abiType{Kind: "addressOpt"},
              },
            },
          },
          {
            Name: "maybeAnyAddressCell",
            Type: abiType{
              Kind: "nullable",
              Inner: &abiType{
                Kind: "cellOf",
                Inner: &abiType{
                  Kind:  "nullable",
                  Inner: &abiType{Kind: "addressAny"},
                },
              },
            },
          },
          {
            Name: "maybeStringCell",
            Type: abiType{
              Kind: "nullable",
              Inner: &abiType{
                Kind: "cellOf",
                Inner: &abiType{
                  Kind:  "nullable",
                  Inner: &abiType{Kind: "string"},
                },
              },
            },
          },
        },
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  text := string(src)
  for _, notWant := range []string{"cellOf optional inner type", "unsupported TLB field"} {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source unexpectedly contains %q:\n%s", notWant, src)
    }
  }
  for _, want := range []string{
    `PlainAddressCell\s+AddressCell\s+` + "`" + `tlb:"\^"` + "`",
    `AddressCell\s+MaybeAddressCell\s+` + "`" + `tlb:"\^"` + "`",
    `MaybeAddressCell\s+\*MaybeAddressCell\s+` + "`" + `tlb:"maybe \^"` + "`",
    `MaybeAnyAddressCell\s+\*MaybeAnyAddressCell\s+` + "`" + `tlb:"maybe \^"` + "`",
    `MaybeStringCell\s+\*MaybeStringCell\s+` + "`" + `tlb:"maybe \^"` + "`",
    `type AddressCell struct \{\s+Value \*address\.Address\s+` + "`" + `tlb:"addr std required"` + "`" + `\s+\}`,
    `type MaybeAddressCell struct \{\s+Value \*address\.Address\s+` + "`" + `tlb:"addr std"` + "`" + `\s+\}`,
    `type MaybeAnyAddressCell struct \{\s+Value \*address\.Address\s+` + "`" + `tlb:"maybe addr"` + "`" + `\s+\}`,
    `type MaybeStringCell struct \{\s+Value \*string\s+` + "`" + `tlb:"maybe string"` + "`" + `\s+\}`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTLBVoidAndLispList(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Point",
        Fields: []field{
          {Name: "x", Type: abiType{Kind: "intN", N: 32}},
          {Name: "y", Type: abiType{Kind: "intN", N: 32}},
        },
      },
      {
        Kind: "struct",
        Name: "Model",
        Fields: []field{
          {Name: "id", Type: abiType{Kind: "uintN", N: 32}},
          {Name: "skip", Type: abiType{Kind: "void"}},
          {Name: "items", Type: abiType{Kind: "lispListOf", Inner: &abiType{Kind: "uintN", N: 16}}},
          {Name: "points", Type: abiType{Kind: "lispListOf", Inner: &abiType{Kind: "StructRef", StructName: "Point"}}},
        },
      },
    },
    Storage: &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "Model"}},
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  if output, err := compileGeneratedWrapper(t, src); err != nil {
    t.Fatalf("generated wrapper does not compile: %v\n%s", err, output)
  }
  roundTripTest := []byte(`package sample

import (
	"reflect"
	"testing"

	"github.com/xssnick/tonutils-go/tlb"
)

func TestGeneratedTLBLispListRoundTrip(t *testing.T) {
	in := Model{
		ID:     7,
		Skip:   struct{}{},
		Items:  ModelItems{1, 2, 3},
		Points: ModelPoints{{X: 4, Y: 5}, {X: -6, Y: 7}},
	}
	c, err := tlb.ToCell(in)
	if err != nil {
		t.Fatalf("to cell: %v", err)
	}

	var out Model
	if err := tlb.LoadFromCell(&out, c.MustBeginParse()); err != nil {
		t.Fatalf("load from cell: %v", err)
	}
	if !reflect.DeepEqual(out.Items, in.Items) {
		t.Fatalf("items = %#v, want %#v", out.Items, in.Items)
	}
	if !reflect.DeepEqual(out.Points, in.Points) {
		t.Fatalf("points = %#v, want %#v", out.Points, in.Points)
	}
}
`)
  if output, err := runGeneratedWrapperTest(t, src, roundTripTest); err != nil {
    t.Fatalf("generated wrapper roundtrip test failed: %v\n%s", err, output)
  }

  text := string(src)
  for _, notWant := range []string{"unsupported TLB field Skip", "unsupported TLB field Items", "unsupported TLB field Points"} {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source unexpectedly contains %q:\n%s", notWant, src)
    }
  }
  for _, want := range []string{
    `Skip\s+struct\{\}\s+` + "`" + `tlb:"-"` + "`",
    `Items\s+ModelItems\s+` + "`" + `tlb:"\."` + "`",
    `Points\s+ModelPoints\s+` + "`" + `tlb:"\."` + "`",
    `type ModelItems \[\]uint16`,
    `type modelItemsItemBox struct \{\s+Value uint16\s+` + "`" + `tlb:"## 16"` + "`" + `\s+\}`,
    `func \(l \*ModelItems\) LoadFromCell\(loader \*cell\.Slice\) error`,
    `func \(l ModelItems\) ToCell\(\) \(\*cell\.Cell, error\)`,
    `type ModelPoints \[\]Point`,
    `type modelPointsItemBox struct \{\s+Value Point\s+` + "`" + `tlb:"\."` + "`" + `\s+\}`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTLBCustomPackUnpackStruct(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Payload",
        Fields: []field{
          {Name: "id", Type: abiType{Kind: "uintN", N: 32}},
        },
        CustomPackUnpack: &customPackUnpack{
          PackToBuilder:   true,
          UnpackFromSlice: true,
        },
      },
    },
    Storage: &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "Payload"}},
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  if len(result.CustomSerializers) != 1 {
    t.Fatalf("custom serializers = %v, want one entry", result.CustomSerializers)
  }
  if got := result.CustomSerializers[0]; got.TypeName != "Payload" || got.LoadFromCellSetterName != "SetPayloadLoadFromCell" || got.ToCellSetterName != "SetPayloadToCell" {
    t.Fatalf("custom serializer = %+v", got)
  }

  for _, want := range []string{
    `type Payload struct \{\s+Value\s+any\s+` + "`" + `tlb:"-"` + "`" + `\s+\}`,
    `var payloadLoadFromCell = func\(value \*Payload, loader \*cell\.Slice\) error \{\s+panic\("SetPayloadLoadFromCell must be called before decoding Payload"\)\s+\}`,
    `func SetPayloadLoadFromCell\(fn func\(value \*Payload, loader \*cell\.Slice\) error\)`,
    `var payloadToCell = func\(value \*Payload\) \(\*cell\.Cell, error\) \{\s+panic\("SetPayloadToCell must be called before encoding Payload"\)\s+\}`,
    `func SetPayloadToCell\(fn func\(value \*Payload\) \(\*cell\.Cell, error\)\)`,
    `func \(v \*Payload\) LoadFromCell\(loader \*cell\.Slice\) error \{\s+return payloadLoadFromCell\(v, loader\)\s+\}`,
    `func \(v Payload\) ToCell\(\) \(\*cell\.Cell, error\) \{\s+return payloadToCell\(&v\)\s+\}`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
  if regexp.MustCompile(`ID\s+uint32\s+` + "`" + `tlb:"## 32"` + "`").Match(src) {
    t.Fatalf("custom struct should be represented as Value any, got fields:\n%s", src)
  }

  roundTripTest := []byte(`package sample

import (
	"testing"

	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/tvm/cell"
)

func TestGeneratedCustomPackUnpackRoundTrip(t *testing.T) {
	SetPayloadToCell(func(value *Payload) (*cell.Cell, error) {
		b := cell.BeginCell()
		if err := b.StoreUInt(value.Value.(uint64), 16); err != nil {
			return nil, err
		}
		return b.EndCell(), nil
	})
	SetPayloadLoadFromCell(func(value *Payload, loader *cell.Slice) error {
		raw, err := loader.LoadUInt(16)
		if err != nil {
			return err
		}
		value.Value = raw
		return nil
	})

	c, err := tlb.ToCell(Payload{Value: uint64(42)})
	if err != nil {
		t.Fatalf("to cell: %v", err)
	}
	var out Payload
	if err := tlb.LoadFromCell(&out, c.MustBeginParse()); err != nil {
		t.Fatalf("load from cell: %v", err)
	}
	if out.Value != uint64(42) {
		t.Fatalf("value = %#v, want 42", out.Value)
	}
}
`)
  if output, err := runGeneratedWrapperTest(t, src, roundTripTest); err != nil {
    t.Fatalf("generated custom roundtrip test failed: %v\n%s", err, output)
  }
}

func TestGenerateTLBCustomPackUnpackAliasAndEnum(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind:   "alias",
        Name:   "TelegramString",
        Target: abiType{Kind: "slice"},
        CustomPackUnpack: &customPackUnpack{
          PackToBuilder:   true,
          UnpackFromSlice: true,
        },
      },
      {
        Kind:      "enum",
        Name:      "Color",
        EncodedAs: &abiType{Kind: "uintN", N: 8},
        Members: []enumMember{
          {Name: "Red", Value: "0"},
          {Name: "Green", Value: "1"},
        },
        CustomPackUnpack: &customPackUnpack{
          PackToBuilder:   true,
          UnpackFromSlice: true,
        },
      },
      {
        Kind: "struct",
        Name: "Payload",
        Fields: []field{
          {Name: "text", Type: abiType{Kind: "AliasRef", AliasName: "TelegramString"}},
          {Name: "color", Type: abiType{Kind: "EnumRef", EnumName: "Color"}},
        },
      },
    },
    Storage: &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "Payload"}},
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `type TelegramString struct \{\s+Value \*cell\.Slice\s+` + "`" + `tlb:"-"` + "`" + `\s+\}`,
    `func SetTelegramStringLoadFromCell\(fn func\(value \*TelegramString, loader \*cell\.Slice\) error\)`,
    `func SetTelegramStringToCell\(fn func\(value \*TelegramString\) \(\*cell\.Cell, error\)\)`,
    `type Color struct \{\s+Value uint8\s+` + "`" + `tlb:"-"` + "`" + `\s+\}`,
    `ColorRed\s+= Color\{Value: uint8\(0\)\}`,
    `Text\s+TelegramString\s+` + "`" + `tlb:"\."` + "`",
    `Color\s+Color\s+` + "`" + `tlb:"\."` + "`",
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTLBCustomPackUnpackAliasSupportsOneSidedDirections(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind:   "alias",
        Name:   "OnlyWithPack",
        Target: abiType{Kind: "intN", N: 8},
        CustomPackUnpack: &customPackUnpack{
          PackToBuilder: true,
        },
      },
      {
        Kind:   "alias",
        Name:   "OnlyWithUnpack",
        Target: abiType{Kind: "uintN", N: 8},
        CustomPackUnpack: &customPackUnpack{
          UnpackFromSlice: true,
        },
      },
      {
        Kind: "struct",
        Name: "Payload",
        Fields: []field{
          {Name: "pack", Type: abiType{Kind: "AliasRef", AliasName: "OnlyWithPack"}},
          {Name: "unpack", Type: abiType{Kind: "AliasRef", AliasName: "OnlyWithUnpack"}},
        },
      },
    },
    Storage: &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "Payload"}},
  }

  result, err := newGenerator(abi, "sample").withStrict(true).Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  if len(result.CustomSerializers) != 2 {
    t.Fatalf("custom serializers = %v, want two entries", result.CustomSerializers)
  }
  if got := result.CustomSerializers[0]; got.TypeName != "OnlyWithPack" || got.LoadFromCellSetterName != "" || got.ToCellSetterName != "SetOnlyWithPackToCell" {
    t.Fatalf("first custom serializer = %+v", got)
  }
  if got := result.CustomSerializers[1]; got.TypeName != "OnlyWithUnpack" || got.LoadFromCellSetterName != "SetOnlyWithUnpackLoadFromCell" || got.ToCellSetterName != "" {
    t.Fatalf("second custom serializer = %+v", got)
  }
  text := string(src)
  for _, want := range []string{
    `type OnlyWithPack struct`,
    `Value int8 ` + "`" + `tlb:"-"` + "`",
    `func (v *OnlyWithPack) LoadFromCell(loader *cell.Slice) error`,
    `panic("OnlyWithPack has no custom unpack_from_slice")`,
    `func SetOnlyWithPackToCell(fn func(value *OnlyWithPack) (*cell.Cell, error))`,
    `type OnlyWithUnpack struct`,
    `Value uint8 ` + "`" + `tlb:"-"` + "`",
    `func SetOnlyWithUnpackLoadFromCell(fn func(value *OnlyWithUnpack, loader *cell.Slice) error)`,
    `func (v OnlyWithUnpack) ToCell() (*cell.Cell, error)`,
    `panic("OnlyWithUnpack has no custom pack_to_builder")`,
  } {
    if !strings.Contains(text, want) {
      t.Fatalf("generated source does not contain %q:\n%s", want, src)
    }
  }
  for _, want := range []string{
    `Pack\s+OnlyWithPack\s+` + "`" + `tlb:"\."` + "`",
    `Unpack\s+OnlyWithUnpack\s+` + "`" + `tlb:"\."` + "`",
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
  for _, notWant := range []string{
    `func SetOnlyWithPackLoadFromCell`,
    `func SetOnlyWithUnpackToCell`,
    `var CustomOnlyWithPack`,
    `var CustomOnlyWithUnpack`,
    `pack_to_builder without unpack_from_slice`,
    `unpack_from_slice without pack_to_builder`,
  } {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source contains %q:\n%s", notWant, src)
    }
  }
}

func TestGenerateOnlyUsedStackHelpers(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    GetMethods: []getMethod{
      {
        Name: "sum_values",
        Parameters: []parameter{
          {
            Name: "values",
            Type: abiType{Kind: "arrayOf", Inner: &abiType{Kind: "uintN", N: 32}},
          },
        },
        ReturnType: abiType{Kind: "uintN", N: 32},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  text := string(src)
  if !strings.Contains(text, "func stackArray[T any]") {
    t.Fatalf("stackArray helper was not generated:\n%s", src)
  }
  for _, notWant := range []string{
    "func stackLispList[T any]",
    "func stackNullablePtr[T any]",
    "func stackNullableValue[T any]",
    "func mustStackCellOf",
    "func decodeStackLispList",
    `"reflect"`,
  } {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source unexpectedly contains %q:\n%s", notWant, src)
    }
  }
}

func TestGenerateOnlyReferencedStackStructInternals(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Reply",
        Fields: []field{
          {Name: "owner", Type: abiType{Kind: "address"}},
          {Name: "balance", Type: abiType{Kind: "uintN", N: 64}},
        },
      },
    },
    GetMethods: []getMethod{
      {
        Name:       "get_reply",
        ReturnType: abiType{Kind: "StructRef", StructName: "Reply"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  text := string(src)
  for _, want := range []string{
    "func decodeReplyResult(values []any) (*Reply, error)",
    "func decodeStackAddress(value any) (*address.Address, error)",
  } {
    if !strings.Contains(text, want) {
      t.Fatalf("generated source does not contain %q:\n%s", want, src)
    }
  }
  for _, notWant := range []string{
    "func stackReply(value *Reply) []any",
    "func loadAddressResult(result *ton.ExecutionResult, index uint) (*address.Address, error)",
    "\tif value == nil {\n\t\treturn nil, nil\n\t}",
  } {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source unexpectedly contains %q:\n%s", notWant, src)
    }
  }
}

func TestGenerateAddressExtStackMethod(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    GetMethods: []getMethod{
      {
        Name: "echo_ext",
        Parameters: []parameter{
          {Name: "addr", Type: abiType{Kind: "addressExt"}},
        },
        ReturnType: abiType{Kind: "addressExt"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  text := string(src)
  for _, want := range []string{
    "func (c *Sample) RunMethodEchoExt(ctx context.Context, block *ton.BlockIDExt, addr *address.Address) (*address.Address, error)",
    "RunGetMethodByID(ctx, block, c.addr, uint64(0), addr)",
    "loadAddressResult(result, 0)",
  } {
    if !strings.Contains(text, want) {
      t.Fatalf("generated source does not contain %q:\n%s", want, src)
    }
  }
}

func TestGenerateGetMethodNegativeIDLiteral(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    GetMethods: []getMethod{
      {
        Name:        "custom",
        TVMMethodID: -1,
        ReturnType:  abiType{Kind: "void"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  if !strings.Contains(string(src), "RunGetMethodByID(ctx, block, c.addr, uint64(18446744073709551615))") {
    t.Fatalf("generated source does not contain uint64 two's-complement method id:\n%s", src)
  }
}

func TestGenerateMapKVDictWrapper(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "alias",
        Name: "U32CellMap",
        Target: abiType{
          Kind:  "mapKV",
          Key:   &abiType{Kind: "uintN", N: 32},
          Value: &abiType{Kind: "cell"},
        },
      },
      {
        Kind: "alias",
        Name: "AddressMap",
        Target: abiType{
          Kind:  "mapKV",
          Key:   &abiType{Kind: "address"},
          Value: &abiType{Kind: "uintN", N: 8},
        },
      },
      {
        Kind: "struct",
        Name: "FlatKey",
        Fields: []field{
          {Name: "shard", Type: abiType{Kind: "uintN", N: 8}},
          {Name: "flag", Type: abiType{Kind: "bool"}},
        },
      },
      {
        Kind: "struct",
        Name: "FlatAddressKey",
        Fields: []field{
          {Name: "owner", Type: abiType{Kind: "address"}},
          {Name: "shard", Type: abiType{Kind: "uintN", N: 8}},
        },
      },
      {
        Kind: "struct",
        Name: "Storage",
        Fields: []field{
          {
            Name: "direct",
            Type: abiType{
              Kind:  "mapKV",
              Key:   &abiType{Kind: "uintN", N: 8},
              Value: &abiType{Kind: "uintN", N: 16},
            },
          },
          {Name: "aliased", Type: abiType{Kind: "AliasRef", AliasName: "U32CellMap"}},
          {Name: "byOwner", Type: abiType{Kind: "AliasRef", AliasName: "AddressMap"}},
          {
            Name: "structured",
            Type: abiType{
              Kind:  "mapKV",
              Key:   &abiType{Kind: "StructRef", StructName: "FlatKey"},
              Value: &abiType{Kind: "bool"},
            },
          },
          {
            Name: "structuredAddress",
            Type: abiType{
              Kind:  "mapKV",
              Key:   &abiType{Kind: "StructRef", StructName: "FlatAddressKey"},
              Value: &abiType{Kind: "uintN", N: 16},
            },
          },
        },
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  if strings.Contains(string(src), "dict key should serialize") {
    t.Fatalf("generated map key encoder contains redundant self-check:\n%s", src)
  }
  if strings.Contains(string(src), "err == cell.ErrNoSuchKeyInDict") {
    t.Fatalf("generated map getter compares errors directly:\n%s", src)
  }

  for _, want := range []string{
    `errors\.Is\(err, cell\.ErrNoSuchKeyInDict\)`,
    `type U32CellMap struct \{\s+Dict \*cell\.Dictionary\s+` + "`" + `tlb:"dict 32"` + "`" + `\s+\}`,
    `func NewU32CellMap\(\) \*U32CellMap`,
    `func \(m \*U32CellMap\) Get\(key uint32\) \(\*cell\.Cell, bool, error\)`,
    `func \(m \*U32CellMap\) Set\(key uint32, value \*cell\.Cell\) error`,
    `func \(m \*U32CellMap\) Delete\(key uint32\) error`,
    `func \(m \*U32CellMap\) LoadAll\(\) \(\[\]uint32, \[\]\*cell\.Cell, error\)`,
    `type AddressMap struct \{\s+Dict \*cell\.Dictionary\s+` + "`" + `tlb:"dict 267"` + "`" + `\s+\}`,
    `type addressMapKeyBox struct \{\s+Key \*address\.Address\s+` + "`" + `tlb:"addr std required"` + "`" + `\s+\}`,
    `func \(m \*AddressMap\) Get\(key \*address\.Address\) \(uint8, bool, error\)`,
    `Direct\s+StorageDirect\s+` + "`" + `tlb:"\."` + "`",
    `type StorageDirect struct \{\s+Dict \*cell\.Dictionary\s+` + "`" + `tlb:"dict 8"` + "`" + `\s+\}`,
    `Aliased\s+U32CellMap\s+` + "`" + `tlb:"\."` + "`",
    `ByOwner\s+AddressMap\s+` + "`" + `tlb:"\."` + "`",
    `Structured\s+StorageStructured\s+` + "`" + `tlb:"\."` + "`",
    `type StorageStructured struct \{\s+Dict \*cell\.Dictionary\s+` + "`" + `tlb:"dict 9"` + "`" + `\s+\}`,
    `func \(m \*StorageStructured\) Get\(key FlatKey\) \(bool, bool, error\)`,
    `StructuredAddress\s+StorageStructuredAddress\s+` + "`" + `tlb:"\."` + "`",
    `type StorageStructuredAddress struct \{\s+Dict \*cell\.Dictionary\s+` + "`" + `tlb:"dict 275"` + "`" + `\s+\}`,
    `type storageStructuredAddressKeyBox struct \{\s+Key FlatAddressKey\s+` + "`" + `tlb:"\."` + "`" + `\s+\}`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTLBEmptyTupleAndShape(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind:   "alias",
        Name:   "EmptyTensor",
        Target: abiType{Kind: "tensor"},
      },
      {
        Kind:   "alias",
        Name:   "EmptyShape",
        Target: abiType{Kind: "shapedTuple"},
      },
      {
        Kind: "struct",
        Name: "Point",
        Fields: []field{
          {Name: "x", Type: abiType{Kind: "intN", N: 32}},
        },
      },
      {
        Kind: "struct",
        Name: "Storage",
        Fields: []field{
          {Name: "emptyTensor", Type: abiType{Kind: "AliasRef", AliasName: "EmptyTensor"}},
          {Name: "emptyShape", Type: abiType{Kind: "AliasRef", AliasName: "EmptyShape"}},
          {
            Name: "nested",
            Type: abiType{Kind: "shapedTuple", Items: []abiType{
              {Kind: "uintN", N: 8},
              {Kind: "AliasRef", AliasName: "EmptyShape"},
              {Kind: "StructRef", StructName: "Point"},
            }},
          },
        },
      },
    },
    Storage: &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "Storage"}},
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  text := string(src)
  if strings.Contains(text, "tensor without items") {
    t.Fatalf("generated source still rejects empty tuple/shape:\n%s", src)
  }
  for _, want := range []string{
    `type EmptyTensor struct \{\s+\}`,
    `type EmptyShape struct \{\s+\}`,
    `EmptyTensor\s+EmptyTensor\s+` + "`" + `tlb:"\."` + "`",
    `EmptyShape\s+EmptyShape\s+` + "`" + `tlb:"\."` + "`",
    `Nested\s+StorageNested\s+` + "`" + `tlb:"\."` + "`",
    `type StorageNested struct \{\s+Uint8\s+uint8\s+` + "`" + `tlb:"## 8"` + "`" + `\s+EmptyShape\s+EmptyShape\s+` + "`" + `tlb:"\."` + "`" + `\s+Point\s+Point\s+` + "`" + `tlb:"\."` + "`" + `\s+\}`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateMapKVRejectsNonFlatStructKey(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "BadKey",
        Fields: []field{
          {Name: "ref", Type: abiType{Kind: "cell"}},
        },
      },
      {
        Kind: "struct",
        Name: "Storage",
        Fields: []field{
          {
            Name: "bad",
            Type: abiType{
              Kind:  "mapKV",
              Key:   &abiType{Kind: "StructRef", StructName: "BadKey"},
              Value: &abiType{Kind: "uintN", N: 8},
            },
          },
        },
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  want := `TODO: unsupported TLB field Bad: map key struct BadKey: field ref is not fixed-width flat bits\.`
  if !regexp.MustCompile(want).Match(src) {
    t.Fatalf("generated source does not reject non-flat struct key:\n%s", src)
  }
}

func TestGenerateSeparatesTLBAndStackDeclarationRoles(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Storage",
        Fields: []field{
          {Name: "id", Type: abiType{Kind: "uintN", N: 32}},
        },
      },
      {
        Kind: "struct",
        Name: "Reply",
        Fields: []field{
          {Name: "answer", Type: abiType{Kind: "int"}},
          {Name: "maybeFlag", Type: abiType{Kind: "nullable", Inner: &abiType{Kind: "bool"}}},
        },
      },
    },
    Storage: &abiStorage{
      StorageType: abiType{Kind: "StructRef", StructName: "Storage"},
    },
    GetMethods: []getMethod{
      {
        Name:        "get_data",
        TVMMethodID: 1,
        ReturnType:  abiType{Kind: "StructRef", StructName: "Reply"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `ID\s+uint32\s+` + "`" + `tlb:"## 32"` + "`",
    `type Reply struct \{\s+Answer\s+\*big\.Int\s+MaybeFlag\s+\*bool\s+\}`,
    `func \(c \*Sample\) RunMethodGetData\(ctx context\.Context, block \*ton\.BlockIDExt\) \(\*Reply, error\)`,
    `decodeReplyResult\(result\.AsTuple\(\)\[0:2\]\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
  if strings.Contains(string(src), "unsupported TLB field Answer") {
    t.Fatalf("stack-only reply was emitted as a TLB struct:\n%s", src)
  }
}

func TestGenerateStackUnionUsesABIWideLayout(t *testing.T) {
  stackWidth := 2
  leftTypeID := 10
  rightTypeID := 11
  variantWidth := 1
  unionType := abiType{
    Kind:       "union",
    StackWidth: &stackWidth,
    Items: []abiType{
      {Kind: "uintN", N: 32},
      {Kind: "bool"},
    },
    Variants: []abiTypeVariant{
      {StackTypeID: &leftTypeID, StackWidth: &variantWidth},
      {StackTypeID: &rightTypeID, StackWidth: &variantWidth},
    },
  }
  abi := abiFile{
    ContractName: "Sample",
    GetMethods: []getMethod{
      {
        Name:        "echo_choice",
        TVMMethodID: 777,
        Parameters:  []parameter{{Name: "choice", Type: unionType}},
        ReturnType:  unionType,
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `params = append\(params, stackUnion\(choice\)\.\.\.\)`,
    `RunGetMethodByID\(ctx, block, c\.addr, uint64\(777\), params\.\.\.\)`,
    `rawVariant, err := decodeStackInt\(values\[1\]\)`,
    `case 10:\s+out\.Variant = UnionUint32`,
    `case 11:\s+out\.Variant = UnionBool`,
    `out = append\(out, int64\(10\)\)`,
    `decodeUnionStackUnion\(result\.AsTuple\(\)\[0:2\]\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateInterfaceStackUnionUsesABIStackTypeIDs(t *testing.T) {
  prefix0 := uint64(0)
  prefix1 := uint64(1)
  stackWidth := 2
  leftTypeID := 130
  rightTypeID := 140
  variantWidth := 1
  unionType := abiType{
    Kind:       "union",
    StackWidth: &stackWidth,
    Items: []abiType{
      {Kind: "StructRef", StructName: "Left"},
      {Kind: "StructRef", StructName: "Right"},
    },
    Variants: []abiTypeVariant{
      {StackTypeID: &leftTypeID, StackWidth: &variantWidth},
      {StackTypeID: &rightTypeID, StackWidth: &variantWidth},
    },
  }
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind:   "struct",
        Name:   "Left",
        Prefix: &prefix{PrefixNum: &prefix0, PrefixLen: 1},
        Fields: []field{
          {Name: "value", Type: abiType{Kind: "uintN", N: 32}},
        },
      },
      {
        Kind:   "struct",
        Name:   "Right",
        Prefix: &prefix{PrefixNum: &prefix1, PrefixLen: 1},
        Fields: []field{
          {Name: "value", Type: abiType{Kind: "bool"}},
        },
      },
    },
    GetMethods: []getMethod{
      {
        Name:        "echo_choice",
        TVMMethodID: 778,
        Parameters:  []parameter{{Name: "choice", Type: unionType}},
        ReturnType:  unionType,
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `type Union interface \{\s+isUnion\(\)\s+\}`,
    `out = append\(out, int64\(130\)\)`,
    `out = append\(out, int64\(140\)\)`,
    `rawVariant, err := decodeStackInt\(values\[1\]\)`,
    `case 130:\s+var value \*Left`,
    `case 140:\s+var value \*Right`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
  if regexp.MustCompile(`case 0:\s+var value`).Match(src) {
    t.Fatalf("interface stack union used zero stack type ids:\n%s", src)
  }
}

func TestGenerateStackRemainingResultFields(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Reply",
        Fields: []field{
          {Name: "head", Type: abiType{Kind: "uintN", N: 32}},
          {Name: "tail", Type: abiType{Kind: "remaining"}},
        },
      },
    },
    GetMethods: []getMethod{
      {
        Name:        "get_reply",
        TVMMethodID: 779,
        ReturnType:  abiType{Kind: "StructRef", StructName: "Reply"},
      },
      {
        Name:        "get_tail",
        TVMMethodID: 780,
        ReturnType:  abiType{Kind: "remaining"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  text := string(src)
  for _, notWant := range []string{"unsupported stack field Tail", "typed result for RunMethodGetReply is not generated", "typed result for RunMethodGetTail is not generated"} {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source contains %q:\n%s", notWant, src)
    }
  }
  for _, want := range []string{
    `type Reply struct \{\s+Head uint32\s+Tail \*cell\.Slice\s+\}`,
    `RunMethodGetReply\(ctx context\.Context, block \*ton\.BlockIDExt\) \(\*Reply, error\)`,
    `out\.Tail, err = decodeStackSlice\(values\[1\]\)`,
    `RunMethodGetTail\(ctx context\.Context, block \*ton\.BlockIDExt\) \(\*cell\.Slice, error\)`,
    `out, err := result\.Slice\(0\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateGetMethodTupleResultWithUnionItems(t *testing.T) {
  unionWidth2 := 2
  unionWidth3 := 3
  leftID := 1
  rightID := 2
  tupleID := 3
  width1 := 1
  width2 := 2
  firstUnion := abiType{
    Kind:       "union",
    StackWidth: &unionWidth2,
    Items: []abiType{
      {Kind: "intN", N: 32},
      {Kind: "bool"},
    },
    Variants: []abiTypeVariant{
      {StackTypeID: &leftID, StackWidth: &width1},
      {StackTypeID: &rightID, StackWidth: &width1},
    },
  }
  secondUnion := abiType{
    Kind:       "union",
    StackWidth: &unionWidth3,
    Items: []abiType{
      {Kind: "intN", N: 32},
      {Kind: "tensor", Items: []abiType{{Kind: "intN", N: 32}, {Kind: "intN", N: 32}}},
    },
    Variants: []abiTypeVariant{
      {StackTypeID: &leftID, StackWidth: &width1},
      {StackTypeID: &tupleID, StackWidth: &width2},
    },
  }
  abi := abiFile{
    ContractName: "Sample",
    GetMethods: []getMethod{
      {
        Name:        "union_tuple",
        TVMMethodID: 781,
        ReturnType:  abiType{Kind: "tensor", Items: []abiType{firstUnion, secondUnion}},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  text := string(src)
  for _, notWant := range []string{"typed result for RunMethodUnionTuple is not generated", "union contains unsupported stack variant", "TODO"} {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source contains %q:\n%s", notWant, src)
    }
  }
  for _, want := range []string{
    `RunMethodUnionTuple\(ctx context\.Context, block \*ton\.BlockIDExt\) \(\*SampleRunMethodUnionTupleResult, error\)`,
    `decodeUnionStackUnion\(result\.AsTuple\(\)\[0:2\]\)`,
    `decodeUnion2StackUnion\(result\.AsTuple\(\)\[2:5\]\)`,
    `case 3:\s+out\.Variant = Union2Tuple`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTupleResultAliasArrayDoesNotRedeclare(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind:   "alias",
        Name:   "IntList",
        Target: abiType{Kind: "arrayOf", Inner: &abiType{Kind: "intN", N: 32}},
      },
    },
    GetMethods: []getMethod{
      {
        Name:        "list_tuple",
        TVMMethodID: 782,
        ReturnType: abiType{Kind: "tensor", Items: []abiType{
          {Kind: "AliasRef", AliasName: "IntList"},
          {
            Kind: "arrayOf",
            Inner: &abiType{
              Kind:  "arrayOf",
              Inner: &abiType{Kind: "intN", N: 32},
            },
          },
        }},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  text := string(src)
  if strings.Contains(text, "var decoded0Alias []int32") {
    t.Fatalf("alias array decoder predeclared a variable that decodeStackArrayLines defines:\n%s", src)
  }
  if strings.Contains(text, "var decoded1Item []int32") {
    t.Fatalf("nested array decoder predeclared a variable that decodeStackArrayLines defines:\n%s", src)
  }
  for _, want := range []string{
    `decoded0Alias := make\(\[\]int32, 0, len\(decoded0BaseTuple\)\)`,
    `out\.IntList = IntList\(decoded0Alias\)`,
    `decoded1Item := make\(\[\]int32, 0, len\(decoded1ItemValueTuple\)\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTopLevelTupleAliasResultIsNotEmptyResultStruct(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "alias",
        Name: "Pair",
        Target: abiType{Kind: "tensor", Items: []abiType{
          {Kind: "intN", N: 32},
          {Kind: "bool"},
        }},
      },
    },
    GetMethods: []getMethod{
      {
        Name:        "get_pair",
        TVMMethodID: 783,
        ReturnType:  abiType{Kind: "AliasRef", AliasName: "Pair"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  text := string(src)
  if strings.Contains(text, "SampleRunMethodGetPairResult") {
    t.Fatalf("top-level tuple alias should not be emitted as an empty result struct:\n%s", src)
  }
  for _, want := range []string{
    `type Pair struct \{\s+Int32\s+int32\s+Bool\s+bool\s+\}`,
    `RunMethodGetPair\(ctx context\.Context, block \*ton\.BlockIDExt\) \(\*Pair, error\)`,
    `out, err := decodePairStackTuple\(result\.AsTuple\(\)\[0:2\]\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateWideNullableStackLayout(t *testing.T) {
  stackWidth := 3
  stackTypeID := 135
  maybePoint := abiType{
    Kind:        "nullable",
    Inner:       &abiType{Kind: "StructRef", StructName: "Point"},
    StackTypeID: &stackTypeID,
    StackWidth:  &stackWidth,
  }
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Point",
        Fields: []field{
          {Name: "x", Type: abiType{Kind: "uintN", N: 32}},
          {Name: "y", Type: abiType{Kind: "uintN", N: 32}},
        },
      },
    },
    GetMethods: []getMethod{
      {
        Name:        "id_maybe_point",
        TVMMethodID: 778,
        Parameters:  []parameter{{Name: "point", Type: maybePoint}},
        ReturnType:  maybePoint,
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `params = append\(params, stackWideNullableValue\(point, 3, int64\(135\), func\(v \*Point\) \[\]any \{ return stackPoint\(v\) \}\)\.\.\.\)`,
    `func stackWideNullableValue\[T any\]\(value T, stackWidth int, stackTypeID int64, encode func\(T\) \[\]any\) \[\]any`,
    `decoded0TypeID, err := decodeStackInt\(result\.AsTuple\(\)\[2\]\)`,
    `decoded0TypeID\.Uint64\(\) != 135`,
    `decodePointResult\(result\.AsTuple\(\)\[0:2\]\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateCompilerGenericAliasInstantiationStackLayout(t *testing.T) {
  data := []byte(`{
		"contract_name": "Sample",
		"unique_types": [
			{"kind": "uintN", "n": 32},
			{"kind": "bool"},
			{"kind": "genericT", "name_t": "X"},
			{"kind": "genericT", "name_t": "Y"},
			{"kind": "union", "variants": [
				{"variant_ty_idx": 2},
				{"variant_ty_idx": 3}
			]},
			{"kind": "AliasRef", "alias_name": "RawEither", "type_args_ty_idx": [0, 1]},
			{"kind": "union", "stack_width": 2, "variants": [
				{"variant_ty_idx": 0, "stack_type_id": 20, "stack_width": 1},
				{"variant_ty_idx": 1, "stack_type_id": 21, "stack_width": 1}
			]}
		],
		"alias_instantiations": [
			{"ty_idx": 5, "alias_name": "RawEither", "monomorphic_target_ty_idx": 6}
		],
		"declarations": [
			{"kind": "alias", "name": "RawEither", "ty_idx": 4, "type_params": ["X", "Y"], "target_ty_idx": 4}
		],
		"get_methods": [
			{"name": "echo", "tvm_method_id": 779, "parameters": [{"name": "value", "ty_idx": 5}], "return_ty_idx": 5}
		]
	}`)

  result, err := GenerateJSON(data, Options{PackageName: "sample"})
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  text := string(src)
  if strings.Contains(text, "genericT") || strings.Contains(text, "TODO") {
    t.Fatalf("generated source contains unresolved generic/TODO:\n%s", src)
  }
  for _, want := range []string{
    `type RawEitherUint32Bool struct \{\s+Variant RawEitherUint32BoolVariant\s+Value\s+any\s+\}`,
    `RunMethodEcho\(ctx context\.Context, block \*ton\.BlockIDExt, value \*RawEitherUint32Bool\) \(\*RawEitherUint32Bool, error\)`,
    `params = append\(params, stackRawEitherUint32Bool\(\(\*RawEitherUint32Bool\)\(value\)\)\.\.\.\)`,
    `case 20:\s+out\.Variant = RawEitherUint32BoolUint32`,
    `case 21:\s+out\.Variant = RawEitherUint32BoolBool`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateStackCellOfTypedPayload(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Reply",
        Fields: []field{
          {
            Name: "contentDict",
            Type: abiType{
              Kind: "cellOf",
              Inner: &abiType{
                Kind:  "mapKV",
                Key:   &abiType{Kind: "uintN", N: 256},
                Value: &abiType{Kind: "cell"},
              },
            },
          },
        },
      },
    },
    GetMethods: []getMethod{
      {
        Name:        "get_data",
        TVMMethodID: 1,
        ReturnType:  abiType{Kind: "StructRef", StructName: "Reply"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `type Reply struct \{\s+ContentDict\s+Dict\s+\}`,
    `type Dict struct \{\s+Dict \*cell\.Dictionary\s+` + "`" + `tlb:"dict 256"` + "`" + `\s+\}`,
    `func \(c \*Sample\) RunMethodGetData\(ctx context\.Context, block \*ton\.BlockIDExt\) \(\*Reply, error\)`,
    `tlb\.Parse\(&out\.ContentDict, replyContentDictCell\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
  if strings.Contains(string(src), "unsupported stack field ContentDict") {
    t.Fatalf("stack cellOf was not generated as typed payload:\n%s", src)
  }
}

func TestGenerateStackTupleAlias(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "alias",
        Name: "tuple",
        Target: abiType{
          Kind:  "arrayOf",
          Inner: &abiType{Kind: "unknown"},
        },
      },
    },
    GetMethods: []getMethod{
      {
        Name:        "get_proposal",
        TVMMethodID: 1,
        ReturnType: abiType{
          Kind:  "nullable",
          Inner: &abiType{Kind: "AliasRef", AliasName: "tuple"},
        },
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `type Tuple \[\]any`,
    `func \(c \*Sample\) RunMethodGetProposal\(ctx context\.Context, block \*ton\.BlockIDExt\) \(\*Tuple, error\)`,
    `out = &decoded0`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
  if strings.Contains(string(src), "alias Tuple is not generated") {
    t.Fatalf("stack-only tuple alias was not emitted:\n%s", src)
  }
}

func TestGenerateStackLispListResult(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "alias",
        Name: "tuple",
        Target: abiType{
          Kind:  "arrayOf",
          Inner: &abiType{Kind: "unknown"},
        },
      },
    },
    GetMethods: []getMethod{
      {
        Name:        "list_proposals",
        TVMMethodID: 1,
        ReturnType: abiType{
          Kind:  "lispListOf",
          Inner: &abiType{Kind: "AliasRef", AliasName: "tuple"},
        },
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `type Tuple \[\]any`,
    `func \(c \*Sample\) RunMethodListProposals\(ctx context\.Context, block \*ton\.BlockIDExt\) \(\[\]Tuple, error\)`,
    `decodeStackLispList\(raw0\)`,
    `out = append\(out, decoded0Item\)`,
    `func decodeStackLispList\(value any\) \(\[\]any, error\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
  if strings.Contains(string(src), "typed result for RunMethodListProposals is not generated") {
    t.Fatalf("lispListOf result was not generated:\n%s", src)
  }
}

func TestGenerateStackLispListParam(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "alias",
        Name: "id_list",
        Target: abiType{
          Kind:  "lispListOf",
          Inner: &abiType{Kind: "uintN", N: 32},
        },
      },
    },
    GetMethods: []getMethod{
      {
        Name:        "sum_ids",
        TVMMethodID: 1,
        Parameters: []parameter{
          {
            Name: "ids",
            Type: abiType{
              Kind:  "lispListOf",
              Inner: &abiType{Kind: "uintN", N: 32},
            },
          },
          {
            Name: "alias_ids",
            Type: abiType{Kind: "AliasRef", AliasName: "id_list"},
          },
        },
        ReturnType: abiType{Kind: "int"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `type IDList \[\]uint32`,
    `func \(c \*Sample\) RunMethodSumIds\(ctx context\.Context, block \*ton\.BlockIDExt, ids \[\]uint32, aliasIds IDList\) \(\*big\.Int, error\)`,
    `stackLispList\(ids, func\(v uint32\) any \{ return uint32\(v\) \}\)`,
    `stackLispList\(\[\]uint32\(aliasIds\), func\(v uint32\) any \{ return uint32\(v\) \}\)`,
    `func stackLispList\[T any\]\(values \[\]T, encode func\(T\) any\) any`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateRemainingOnlyAtTail(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Model",
        Fields: []field{
          {Name: "tail", Type: abiType{Kind: "remaining"}},
          {Name: "id", Type: abiType{Kind: "uintN", N: 32}},
        },
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  if !regexp.MustCompile(`TODO: unsupported TLB field Tail: remaining must be the last field\.`).Match(src) {
    t.Fatalf("generated source does not reject non-tail remaining:\n%s", src)
  }
}

func TestStrictRejectsTODOGeneration(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Model",
        Fields: []field{
          {
            Name: "bad",
            Type: abiType{
              Kind:  "mapKV",
              Key:   &abiType{Kind: "cell"},
              Value: &abiType{Kind: "uintN", N: 8},
            },
          },
        },
      },
    },
  }

  result, err := newGenerator(abi, "sample").withStrict(true).Generate()
  src := result.Source
  if err == nil {
    t.Fatalf("expected strict generation error")
  }
  if src != nil {
    t.Fatalf("strict generation returned source: %s", src)
  }
  if !strings.Contains(err.Error(), "unsupported TLB field Bad: map key type cell is not fixed-width flat bits.") {
    t.Fatalf("strict error does not include TODO diagnostic: %v", err)
  }
}

func TestStrictRejectsFallbackMethodResult(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    GetMethods: []getMethod{
      {
        Name:       "unsupportedResult",
        ReturnType: abiType{Kind: "unknownResult"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").withStrict(true).Generate()
  src := result.Source
  if err == nil {
    t.Fatalf("expected strict generation error")
  }
  if src != nil {
    t.Fatalf("strict generation returned source: %s", src)
  }
  if !strings.Contains(err.Error(), "typed result for RunMethodUnsupportedResult is not generated yet: unsupported ABI type kind unknownResult.") {
    t.Fatalf("strict error does not include fallback method diagnostic: %v", err)
  }
}

func TestStrictAllowsUnknownABIFields(t *testing.T) {
  abi := []byte(`{
		"abi_schema_version": "1.0",
		"contract_name": "Sample",
		"declarations": [
			{
				"kind": "struct",
				"name": "Model",
				"fields": [
					{
						"name": "id",
						"ty": {"kind": "uintN", "n": 32},
						"default_value": {"kind": "int", "v": "1"}
					}
				]
			}
		],
		"get_methods": []
	}`)

  result, err := GenerateJSON(abi, Options{PackageName: "sample", Strict: true})
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  if !regexp.MustCompile(`ID\s+uint32\s+` + "`" + `tlb:"## 32"` + "`").Match(src) {
    t.Fatalf("generated source does not include model id:\n%s", src)
  }
}

func TestStrictRejectsMissingRequiredABIFields(t *testing.T) {
  abi := []byte(`{
		"abi_schema_version": "1.0",
		"contract_name": "Sample",
		"declarations": [
			{
				"kind": "struct",
				"name": "Model",
				"fields": [
					{"name": "id"}
				]
			}
		],
		"get_methods": []
	}`)

  result, err := GenerateJSON(abi, Options{PackageName: "sample", Strict: true})
  src := result.Source
  if err == nil {
    t.Fatalf("expected strict schema error")
  }
  if src != nil {
    t.Fatalf("strict generation returned source: %s", src)
  }
  if !strings.Contains(err.Error(), "abi.declarations[0].fields[0].ty: required field is missing or empty") {
    t.Fatalf("strict error does not include missing field diagnostic: %v", err)
  }
}

func TestNonStrictAllowsUnknownABIFields(t *testing.T) {
  abi := []byte(`{
		"abi_schema_version": "1.0",
		"contract_name": "Sample",
		"declarations": [
			{
				"kind": "struct",
				"name": "Model",
				"fields": [
					{
						"name": "id",
						"ty": {"kind": "uintN", "n": 32},
						"default_value": {"kind": "int", "v": "1"}
					}
				]
			}
		],
		"get_methods": []
	}`)

  result, err := GenerateJSON(abi, Options{PackageName: "sample"})
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  if !regexp.MustCompile(`ID\s+uint32\s+` + "`" + `tlb:"## 32"` + "`").Match(src) {
    t.Fatalf("generated source does not include model id:\n%s", src)
  }
}

func TestGenerateFromReader(t *testing.T) {
  abi := `{
		"abi_schema_version": "1.0",
		"contract_name": "Sample",
		"declarations": [],
		"get_methods": []
	}`

  result, err := Generate(strings.NewReader(abi), Options{PackageName: "sample"})
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  if !regexp.MustCompile(`type Sample struct`).Match(src) {
    t.Fatalf("generated source does not include contract wrapper:\n%s", src)
  }
}

func TestGenerateStackVarintMethod(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    GetMethods: []getMethod{
      {
        Name:        "echoDelta",
        TVMMethodID: 12345,
        Parameters:  []parameter{{Name: "delta", Type: abiType{Kind: "varintN", N: 16}}},
        ReturnType:  abiType{Kind: "varintN", N: 16},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `RunMethodEchoDelta\(ctx context\.Context, block \*ton\.BlockIDExt, delta \*big\.Int\) \(\*big\.Int, error\)`,
    `RunGetMethodByID\(ctx, block, c\.addr, uint64\(12345\), delta\)`,
    `out, err := result\.Int\(0\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateStackStringMethod(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    GetMethods: []getMethod{
      {
        Name:        "echoLabel",
        TVMMethodID: 23456,
        Parameters:  []parameter{{Name: "label", Type: abiType{Kind: "string"}}},
        ReturnType:  abiType{Kind: "string"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `RunMethodEchoLabel\(ctx context\.Context, block \*ton\.BlockIDExt, label string\) \(string, error\)`,
    `RunGetMethodByID\(ctx, block, c\.addr, uint64\(23456\), stackString\(label\)\)`,
    `func stackString\(value string\) \*cell\.Slice`,
    `func loadStringResult\(result \*ton\.ExecutionResult, index uint\) \(string, error\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTLBBytesTensorUnion(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Small",
        Fields: []field{
          {Name: "id", Type: abiType{Kind: "uintN", N: 16}},
        },
      },
      {
        Kind: "struct",
        Name: "Payload",
        Fields: []field{
          {Name: "hash", Type: abiType{Kind: "bytesN", N: 32}},
          {
            Name: "pair",
            Type: abiType{Kind: "tensor", Items: []abiType{
              {Kind: "uintN", N: 8},
              {Kind: "bool"},
            }},
          },
          {
            Name: "shaped",
            Type: abiType{Kind: "shapedTuple", Items: []abiType{
              {Kind: "uintN", N: 16},
              {Kind: "string"},
            }},
          },
          {
            Name: "choice",
            Type: abiType{Kind: "union", Items: []abiType{
              {Kind: "uintN", N: 8},
              {Kind: "StructRef", StructName: "Small"},
              {Kind: "null"},
            }},
          },
        },
      },
    },
    Storage: &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "Payload"}},
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `Hash\s+\[\]byte\s+` + "`" + `tlb:"bits 256"` + "`",
    `Pair\s+PayloadPair\s+` + "`" + `tlb:"\."` + "`",
    `type PayloadPair struct \{\s+Uint8\s+uint8\s+` + "`" + `tlb:"## 8"` + "`" + `\s+Bool\s+bool\s+` + "`" + `tlb:"bool"` + "`" + `\s+\}`,
    `Shaped\s+PayloadShaped\s+` + "`" + `tlb:"\."` + "`",
    `type PayloadShaped struct \{\s+Uint16\s+uint16\s+` + "`" + `tlb:"## 16"` + "`" + `\s+String\s+string\s+` + "`" + `tlb:"string"` + "`" + `\s+\}`,
    `Choice\s+PayloadChoice\s+` + "`" + `tlb:"\."` + "`",
    `type PayloadChoice struct \{\s+Variant\s+PayloadChoiceVariant\s+Value\s+any\s+\}`,
    `func \(u \*PayloadChoice\) LoadFromCell\(loader \*cell\.Slice\) error`,
    `func \(u PayloadChoice\) ToCell\(\) \(\*cell\.Cell, error\)`,
    `case 2:\s+u\.Variant = PayloadChoiceNull\s+u\.Value = nil`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateStackUnknownParam(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind:   "alias",
        Name:   "RawValue",
        Target: abiType{Kind: "unknown"},
      },
    },
    GetMethods: []getMethod{
      {
        Name:        "pass_raw",
        TVMMethodID: 34567,
        Parameters: []parameter{
          {Name: "direct", Type: abiType{Kind: "unknown"}},
          {Name: "named", Type: abiType{Kind: "AliasRef", AliasName: "RawValue"}},
        },
        ReturnType: abiType{Kind: "void"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `type RawValue any`,
    `PassRaw\(ctx context\.Context, block \*ton\.BlockIDExt, direct any, named RawValue\) error`,
    `RunGetMethodByID\(ctx, block, c\.addr, uint64\(34567\), direct, any\(named\)\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTLBGenericMonomorphs(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "RichPayload",
        Fields: []field{
          {Name: "id", Type: abiType{Kind: "uintN", N: 16}},
        },
      },
      {
        Kind:       "struct",
        Name:       "Box",
        TypeParams: []string{"T"},
        Fields: []field{
          {Name: "value", Type: abiType{Kind: "genericT", NameT: "T"}},
        },
      },
      {
        Kind:       "alias",
        Name:       "MaybeBox",
        TypeParams: []string{"T"},
        Target: abiType{
          Kind: "nullable",
          Inner: &abiType{
            Kind:       "StructRef",
            StructName: "Box",
            TypeArgs:   []abiType{{Kind: "genericT", NameT: "T"}},
          },
        },
      },
      {
        Kind:       "struct",
        Name:       "PairBox",
        TypeParams: []string{"K", "V"},
        Fields: []field{
          {Name: "key", Type: abiType{Kind: "genericT", NameT: "K"}},
          {Name: "value", Type: abiType{Kind: "genericT", NameT: "V"}},
        },
      },
      {
        Kind: "struct",
        Name: "Payload",
        Fields: []field{
          {
            Name: "genericBox",
            Type: abiType{
              Kind:       "StructRef",
              StructName: "Box",
              TypeArgs: []abiType{{
                Kind:  "cellOf",
                Inner: &abiType{Kind: "StructRef", StructName: "RichPayload"},
              }},
            },
          },
          {
            Name: "route",
            Type: abiType{
              Kind:      "AliasRef",
              AliasName: "MaybeBox",
              TypeArgs:  []abiType{{Kind: "uintN", N: 32}},
            },
          },
          {
            Name: "nestedRoute",
            Type: abiType{
              Kind:      "AliasRef",
              AliasName: "MaybeBox",
              TypeArgs: []abiType{{
                Kind:  "cellOf",
                Inner: &abiType{Kind: "StructRef", StructName: "RichPayload"},
              }},
            },
          },
          {
            Name: "keyed",
            Type: abiType{
              Kind:       "StructRef",
              StructName: "PairBox",
              TypeArgs: []abiType{
                {Kind: "bitsN", N: 256},
                {Kind: "uintN", N: 64},
              },
            },
          },
        },
      },
    },
    Storage: &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "Payload"}},
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  text := string(src)
  for _, notWant := range []string{"generic parameters", "genericT", "TODO"} {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source unexpectedly contains %q:\n%s", notWant, src)
    }
  }
  for _, want := range []string{
    `type BoxRichPayloadCell struct \{\s+Value\s+RichPayload\s+` + "`" + `tlb:"\^"` + "`" + `\s+\}`,
    `GenericBox\s+BoxRichPayloadCell\s+` + "`" + `tlb:"\."` + "`",
    `type BoxUint32 struct \{\s+Value\s+uint32\s+` + "`" + `tlb:"## 32"` + "`" + `\s+\}`,
    `type MaybeBoxUint32 \*BoxUint32`,
    `Route\s+MaybeBoxUint32\s+` + "`" + `tlb:"maybe \."` + "`",
    `type MaybeBoxRichPayloadCell \*BoxRichPayloadCell`,
    `NestedRoute\s+MaybeBoxRichPayloadCell\s+` + "`" + `tlb:"maybe \."` + "`",
    `type PairBoxBits256Uint64 struct \{\s+Key\s+\[\]byte\s+` + "`" + `tlb:"bits 256"` + "`" + `\s+Value\s+uint64\s+` + "`" + `tlb:"## 64"` + "`" + `\s+\}`,
    `Keyed\s+PairBoxBits256Uint64\s+` + "`" + `tlb:"\."` + "`",
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateCompilerGenericInstantiationDisplayNamesTLB(t *testing.T) {
  data := []byte(`{
		"contract_name": "Sample",
		"unique_types": [
			{"kind": "intN", "n": 8},
			{"kind": "genericT", "name_t": "T"},
			{"kind": "StructRef", "struct_name": "Box", "type_args_ty_idx": [0]},
			{"kind": "StructRef", "struct_name": "Box", "type_args_ty_idx": [1]},
			{"kind": "nullable", "inner_ty_idx": 3},
			{"kind": "AliasRef", "alias_name": "MaybeAlias", "type_args_ty_idx": [0]},
			{"kind": "nullable", "inner_ty_idx": 2},
			{"kind": "StructRef", "struct_name": "Holder"}
		],
		"struct_instantiations": [
			{"ty_idx": 2, "struct_name": "Box<int8>", "monomorphic_fields_ty_idx": [0]}
		],
		"alias_instantiations": [
			{"ty_idx": 5, "alias_name": "MaybeAlias<int8>", "monomorphic_target_ty_idx": 6}
		],
		"declarations": [
			{"kind": "struct", "name": "Box", "type_params": ["T"], "fields": [{"name": "value", "ty_idx": 1}]},
			{"kind": "alias", "name": "MaybeAlias", "type_params": ["T"], "target_ty_idx": 4},
			{"kind": "struct", "name": "Holder", "fields": [
				{"name": "direct", "ty_idx": 2},
				{"name": "viaAlias", "ty_idx": 5}
			]}
		],
		"storage": {"storage_ty_idx": 7}
	}`)

  result, err := GenerateJSON(data, Options{PackageName: "sample"})
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  if output, err := compileGeneratedWrapper(t, src); err != nil {
    t.Fatalf("generated wrapper does not compile: %v\n%s", err, output)
  }

  text := string(src)
  for _, notWant := range []string{"generic struct Box", "generic alias MaybeAlias", "undefined: Box", "TODO"} {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source unexpectedly contains %q:\n%s", notWant, src)
    }
  }
  for _, want := range []string{
    `type BoxInt8 struct \{\s+Value\s+int8\s+` + "`" + `tlb:"## 8"` + "`" + `\s+\}`,
    `type MaybeAliasInt8 \*BoxInt8`,
    `Direct\s+BoxInt8\s+` + "`" + `tlb:"\."` + "`",
    `ViaAlias\s+MaybeAliasInt8\s+` + "`" + `tlb:"maybe \."` + "`",
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateLargeEnumValuesAndMemberCollisions(t *testing.T) {
  encodedBig := abiType{Kind: "intN", N: 257}
  encodedSmall := abiType{Kind: "uintN", N: 8}
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind:      "enum",
        Name:      "EMinMax",
        EncodedAs: &encodedBig,
        Members: []enumMember{
          {Name: "min_int", Value: "-115792089237316195423570985008687907853269984665640564039457584007913129639936"},
          {Name: "max_int", Value: "115792089237316195423570985008687907853269984665640564039457584007913129639935"},
        },
      },
      {
        Kind:      "enum",
        Name:      "ECollisionNames",
        EncodedAs: &encodedSmall,
        Members: []enumMember{
          {Name: "toCell", Value: "2"},
          {Name: "ToCell", Value: "3"},
        },
      },
      {
        Kind: "struct",
        Name: "Holder",
        Fields: []field{
          {Name: "big", Type: abiType{Kind: "EnumRef", EnumName: "EMinMax"}},
          {Name: "collision", Type: abiType{Kind: "EnumRef", EnumName: "ECollisionNames"}},
        },
      },
    },
    Storage: &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "Holder"}},
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  if output, err := compileGeneratedWrapper(t, src); err != nil {
    t.Fatalf("generated wrapper does not compile: %v\n%s", err, output)
  }

  for _, want := range []string{
    `type EMinMax = \*big\.Int`,
    `var \(\s+EMinMaxMinInt EMinMax = tugenMustBigInt\("-115792089237316195423570985008687907853269984665640564039457584007913129639936"\)`,
    `EMinMaxMaxInt EMinMax = tugenMustBigInt\("115792089237316195423570985008687907853269984665640564039457584007913129639935"\)`,
    `func tugenMustBigInt\(value string\) \*big\.Int`,
    `ECollisionNamesToCell\s+ECollisionNames = 2`,
    `ECollisionNamesToCell2\s+ECollisionNames = 3`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateGenericUnionAliasAsInterface(t *testing.T) {
  prefix0 := uint64(0)
  prefix1 := uint64(1)
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Small",
        Fields: []field{
          {Name: "id", Type: abiType{Kind: "uintN", N: 8}},
        },
      },
      {
        Kind:   "struct",
        Name:   "AbiNone",
        Prefix: &prefix{PrefixNum: &prefix0, PrefixLen: 1},
      },
      {
        Kind:       "struct",
        Name:       "AbiJust",
        TypeParams: []string{"X"},
        Prefix:     &prefix{PrefixNum: &prefix1, PrefixLen: 1},
        Fields: []field{
          {Name: "value", Type: abiType{Kind: "genericT", NameT: "X"}},
        },
      },
      {
        Kind:       "alias",
        Name:       "AbiMaybe",
        TypeParams: []string{"X"},
        Target: abiType{Kind: "union", Items: []abiType{
          {Kind: "StructRef", StructName: "AbiNone"},
          {
            Kind:       "StructRef",
            StructName: "AbiJust",
            TypeArgs:   []abiType{{Kind: "genericT", NameT: "X"}},
          },
        }},
      },
      {
        Kind: "alias",
        Name: "MaybeSmall",
        Target: abiType{
          Kind:      "AliasRef",
          AliasName: "AbiMaybe",
          TypeArgs:  []abiType{{Kind: "StructRef", StructName: "Small"}},
        },
      },
      {
        Kind: "struct",
        Name: "Payload",
        Fields: []field{
          {Name: "choice", Type: abiType{Kind: "AliasRef", AliasName: "MaybeSmall"}},
        },
      },
    },
    Storage: &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "Payload"}},
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `type ABIJustSmall struct \{\s+_\s+tlb\.Magic ` + "`" + `tlb:"\$1"` + "`" + `\s+Value\s+Small\s+` + "`" + `tlb:"\."` + "`" + `\s+\}`,
    `type ABIMaybeSmall interface \{\s+isABIMaybeSmall\(\)\s+\}`,
    `type MaybeSmall = ABIMaybeSmall`,
    `func \(ABINone\) isABIMaybeSmall\(\) \{\}`,
    `func \(ABIJustSmall\) isABIMaybeSmall\(\) \{\}`,
    `Choice\s+MaybeSmall\s+` + "`" + `tlb:"\[ABINone,ABIJustSmall\]"` + "`",
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTLBGenericEitherMaybeAliases(t *testing.T) {
  prefix0 := uint64(0)
  prefix1 := uint64(1)
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind:       "struct",
        Name:       "Left",
        TypeParams: []string{"T"},
        Prefix:     &prefix{PrefixNum: &prefix0, PrefixLen: 1},
        Fields: []field{
          {Name: "value", Type: abiType{Kind: "genericT", NameT: "T"}},
        },
      },
      {
        Kind:       "struct",
        Name:       "Right",
        TypeParams: []string{"T"},
        Prefix:     &prefix{PrefixNum: &prefix1, PrefixLen: 1},
        Fields: []field{
          {Name: "value", Type: abiType{Kind: "genericT", NameT: "T"}},
        },
      },
      {
        Kind:       "alias",
        Name:       "Either",
        TypeParams: []string{"L", "R"},
        Target: abiType{Kind: "union", Items: []abiType{
          {
            Kind:       "StructRef",
            StructName: "Left",
            TypeArgs:   []abiType{{Kind: "genericT", NameT: "L"}},
          },
          {
            Kind:       "StructRef",
            StructName: "Right",
            TypeArgs:   []abiType{{Kind: "genericT", NameT: "R"}},
          },
        }},
      },
      {
        Kind:   "struct",
        Name:   "None",
        Prefix: &prefix{PrefixNum: &prefix0, PrefixLen: 1},
      },
      {
        Kind:       "struct",
        Name:       "Just",
        TypeParams: []string{"T"},
        Prefix:     &prefix{PrefixNum: &prefix1, PrefixLen: 1},
        Fields: []field{
          {Name: "value", Type: abiType{Kind: "genericT", NameT: "T"}},
        },
      },
      {
        Kind:       "alias",
        Name:       "Maybe",
        TypeParams: []string{"T"},
        Target: abiType{Kind: "union", Items: []abiType{
          {Kind: "StructRef", StructName: "None"},
          {
            Kind:       "StructRef",
            StructName: "Just",
            TypeArgs:   []abiType{{Kind: "genericT", NameT: "T"}},
          },
        }},
      },
      {
        Kind: "alias",
        Name: "MaybeChoice",
        Target: abiType{
          Kind:      "AliasRef",
          AliasName: "Maybe",
          TypeArgs: []abiType{{
            Kind:      "AliasRef",
            AliasName: "Either",
            TypeArgs: []abiType{
              {Kind: "uintN", N: 16},
              {Kind: "bool"},
            },
          }},
        },
      },
      {
        Kind: "struct",
        Name: "Model",
        Fields: []field{
          {
            Name: "choice",
            Type: abiType{
              Kind:      "AliasRef",
              AliasName: "Either",
              TypeArgs: []abiType{
                {Kind: "uintN", N: 8},
                {Kind: "bool"},
              },
            },
          },
          {Name: "maybe", Type: abiType{Kind: "AliasRef", AliasName: "MaybeChoice"}},
        },
      },
    },
    Storage: &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "Model"}},
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }
  if output, err := compileGeneratedWrapper(t, src); err != nil {
    t.Fatalf("generated wrapper does not compile: %v\n%s", err, output)
  }

  text := string(src)
  for _, notWant := range []string{"generic alias Either", "generic alias Maybe", "genericT", "TODO"} {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source unexpectedly contains %q:\n%s", notWant, src)
    }
  }
  for _, want := range []string{
    `Choice\s+EitherUint8Bool\s+` + "`" + `tlb:"\[LeftUint8,RightBool\]"` + "`",
    `Maybe\s+MaybeChoice\s+` + "`" + `tlb:"\[None,JustEitherUint16Bool\]"` + "`",
    `type EitherUint8Bool interface`,
    `type MaybeEitherUint16Bool interface`,
    `type MaybeChoice = MaybeEitherUint16Bool`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateFixtureGenericTLBAliasesKeepCodecs(t *testing.T) {
  abi, diagnostics, err := loadABIFile(filepath.Join("testdata", "wrapper-fixtures", "lots-of-wrappers.abi.json"), false)
  if err != nil {
    t.Fatalf("load fixture: %v", err)
  }
  if len(diagnostics) > 0 {
    t.Fatalf("fixture has diagnostics: %v", diagnostics)
  }
  abi.GetMethods = nil
  abi.Storage = &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "MsgTransfer"}}

  result, err := newGenerator(abi, "fixtures").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate fixture: %v", err)
  }

  text := string(src)
  for _, notWant := range []string{
    "unsupported TLB field Params",
    "generic alias Either",
    "generic alias Maybe",
  } {
    if strings.Contains(text, notWant) {
      t.Fatalf("generated source unexpectedly contains %q:\n%s", notWant, src)
    }
  }
  for _, want := range []string{
    `type MsgTransfer struct \{\s+_\s+tlb\.Magic\s+` + "`" + `tlb:"#fb3701ff"` + "`" + `\s+Params\s+EitherTransferParamsTransferParamsCell\s+` + "`" + `tlb:"\."` + "`" + `\s+\}`,
    `Value\s+TransferParams\s+` + "`" + `tlb:"\[TransferParams1,TransferParams2\]"` + "`",
    `Value\s+TransferParams\s+` + "`" + `tlb:"\^ \[TransferParams1,TransferParams2\]"` + "`",
    `type EitherTransferParamsTransferParamsCell struct`,
    `type TransferParams interface`,
    `tlb\.Register\(TransferParams1\{\}\)`,
    `tlb\.Register\(TransferParams2\{\}\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateStackGenericStruct(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind:       "struct",
        Name:       "Box",
        TypeParams: []string{"T"},
        Fields: []field{
          {Name: "value", Type: abiType{Kind: "genericT", NameT: "T"}},
        },
      },
    },
    GetMethods: []getMethod{
      {
        Name:        "echo_box",
        TVMMethodID: 45678,
        Parameters: []parameter{{
          Name: "box",
          Type: abiType{
            Kind:       "StructRef",
            StructName: "Box",
            TypeArgs:   []abiType{{Kind: "uintN", N: 32}},
          },
        }},
        ReturnType: abiType{
          Kind:       "StructRef",
          StructName: "Box",
          TypeArgs:   []abiType{{Kind: "uintN", N: 32}},
        },
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `type BoxUint32 struct \{\s+Value uint32\s+\}`,
    `RunMethodEchoBox\(ctx context\.Context, block \*ton\.BlockIDExt, box \*BoxUint32\) \(\*BoxUint32, error\)`,
    `params = append\(params, stackBoxUint32\(box\)\.\.\.\)`,
    `RunGetMethodByID\(ctx, block, c\.addr, uint64\(45678\), params\.\.\.\)`,
    `func stackBoxUint32\(value \*BoxUint32\) \[\]any`,
    `func decodeBoxUint32Result\(values \[\]any\) \(\*BoxUint32, error\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
}

func TestGenerateTLBPrefixedStructUnionAsInterface(t *testing.T) {
  prefix0 := uint64(0)
  prefix1 := uint64(1)
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind:   "struct",
        Name:   "PayloadInline",
        Prefix: &prefix{PrefixNum: &prefix0, PrefixLen: 1},
        Fields: []field{
          {Name: "value", Type: abiType{Kind: "uintN", N: 8}},
        },
      },
      {
        Kind:   "struct",
        Name:   "PayloadInRef",
        Prefix: &prefix{PrefixNum: &prefix1, PrefixLen: 1},
        Fields: []field{
          {Name: "value", Type: abiType{Kind: "uintN", N: 16}},
        },
      },
      {
        Kind: "alias",
        Name: "Payload",
        Target: abiType{Kind: "union", Items: []abiType{
          {Kind: "StructRef", StructName: "PayloadInline"},
          {Kind: "StructRef", StructName: "PayloadInRef"},
        }},
      },
      {
        Kind: "struct",
        Name: "Message",
        Fields: []field{
          {Name: "payload", Type: abiType{Kind: "AliasRef", AliasName: "Payload"}},
        },
      },
    },
    Storage: &abiStorage{StorageType: abiType{Kind: "StructRef", StructName: "Message"}},
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `Payload\s+Payload\s+` + "`" + `tlb:"\[PayloadInline,PayloadInRef\]"` + "`",
    `type Payload interface \{\s+isPayload\(\)\s+\}`,
    `func \(PayloadInline\) isPayload\(\) \{\}`,
    `func \(PayloadInRef\) isPayload\(\) \{\}`,
    `func NewPayloadInline\(value PayloadInline\) Payload`,
    `func NewPayloadInRef\(value PayloadInRef\) Payload`,
    `tlb\.Register\(PayloadInline\{\}\)`,
    `tlb\.Register\(PayloadInRef\{\}\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
  for _, notWant := range []string{
    `type Payload struct \{\s+Variant\s+PayloadVariant\s+Value\s+any\s+\}`,
    `func \(u \*Payload\) LoadFromCell`,
    `func \(u Payload\) ToCell`,
  } {
    if regexp.MustCompile(notWant).Match(src) {
      t.Fatalf("generated source unexpectedly matches %q:\n%s", notWant, src)
    }
  }
}

func TestGenerateStackCompoundParamsAndResults(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    Declarations: []declaration{
      {
        Kind: "struct",
        Name: "Point",
        Fields: []field{
          {Name: "x", Type: abiType{Kind: "uintN", N: 32}},
          {Name: "ok", Type: abiType{Kind: "bool"}},
        },
      },
      {
        Kind: "alias",
        Name: "Items",
        Target: abiType{
          Kind:  "mapKV",
          Key:   &abiType{Kind: "uintN", N: 16},
          Value: &abiType{Kind: "uintN", N: 32},
        },
      },
      {
        Kind: "struct",
        Name: "Reply",
        Fields: []field{
          {Name: "values", Type: abiType{Kind: "arrayOf", Inner: &abiType{Kind: "uintN", N: 8}}},
          {Name: "maybeFlag", Type: abiType{Kind: "nullable", Inner: &abiType{Kind: "bool"}}},
          {Name: "payload", Type: abiType{Kind: "cellOf", Inner: &abiType{Kind: "StructRef", StructName: "Point"}}},
          {Name: "items", Type: abiType{Kind: "AliasRef", AliasName: "Items"}},
        },
      },
    },
    GetMethods: []getMethod{
      {
        Name: "query",
        Parameters: []parameter{
          {Name: "point", Type: abiType{Kind: "StructRef", StructName: "Point"}},
          {Name: "values", Type: abiType{Kind: "arrayOf", Inner: &abiType{Kind: "uintN", N: 32}}},
          {Name: "maybeFlag", Type: abiType{Kind: "nullable", Inner: &abiType{Kind: "bool"}}},
          {Name: "payload", Type: abiType{Kind: "cellOf", Inner: &abiType{Kind: "StructRef", StructName: "Point"}}},
          {Name: "items", Type: abiType{Kind: "AliasRef", AliasName: "Items"}},
        },
        ReturnType: abiType{Kind: "StructRef", StructName: "Reply"},
      },
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `func \(c \*Sample\) RunMethodQuery\(ctx context\.Context, block \*ton\.BlockIDExt, point \*Point, values \[\]uint32, maybeFlag \*bool, payload Point, items Items\) \(\*Reply, error\)`,
    `params = append\(params, stackPoint\(point\)\.\.\.\)`,
    `params = append\(params, stackArray\(values, func\(v uint32\) any \{ return uint32\(v\) \}\)\)`,
    `params = append\(params, stackNullablePtr\(maybeFlag, func\(v bool\) any \{ return boolToStack\(v\) \}\)\)`,
    `payloadStack, err := stackCellOf\(payload\)`,
    `if err != nil \{\s+return nil, fmt\.Errorf\("encode stack parameter payload: %w", err\)\s+\}`,
    `params = append\(params, payloadStack\)`,
    `itemsStack, err := stackCellOf\(Dict\(items\)\)`,
    `params = append\(params, itemsStack\)`,
    `RunGetMethodByID\(ctx, block, c\.addr, uint64\(0\), params\.\.\.\)`,
    `type Reply struct \{\s+Values\s+\[\]uint8\s+MaybeFlag\s+\*bool\s+Payload\s+Point\s+Items\s+Items\s+\}`,
    `out\.Values = make\(\[\]uint8, 0, len\(replyValuesTuple\)\)`,
    `tlb\.Parse\(&out\.Payload, replyPayloadCell\)`,
    `out\.Items = Items\(replyItemsAlias\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
  if strings.Contains(string(src), "case bool:") {
    t.Fatalf("generated stack bool decoder accepts raw Go bool:\n%s", src)
  }
}

func TestGenerateWrapperFixtureABIs(t *testing.T) {
  fixtures, err := filepath.Glob("testdata/wrapper-fixtures/*.abi.json")
  if err != nil {
    t.Fatalf("glob fixtures: %v", err)
  }
  if len(fixtures) == 0 {
    t.Fatalf("no wrapper ABI fixtures found")
  }
  sort.Strings(fixtures)

  knownCompileGaps := map[string][]string{}

  for _, fixture := range fixtures {
    fixture := fixture
    name := strings.TrimSuffix(filepath.Base(fixture), ".abi.json")
    t.Run(name, func(t *testing.T) {
      result, err := GenerateFile(fixture, Options{PackageName: "fixtures"})
      src := result.Source
      if err != nil {
        t.Fatalf("generate %s: %v", fixture, err)
      }

      output, err := compileGeneratedWrapper(t, src)
      if expected, ok := knownCompileGaps[name]; ok {
        if err == nil {
          t.Fatalf("known compile gap for %s is fixed; remove it from knownCompileGaps", name)
        }
        if !containsAny(string(output), expected) {
          t.Fatalf("known compile gap for %s failed with unexpected output:\n%s", name, output)
        }
        t.Logf("known compile gap for %s is still present", name)
        return
      }
      if err != nil {
        t.Fatalf("generated wrapper does not compile: %v\n%s", err, output)
      }
    })
  }
}

func TestFixtureABIAnnotations(t *testing.T) {
  abi := readRawFixtureABI(t, "lots-of-annotations")
  if got := rawString(t, abi, "contract_name"); got != "LotsOfAnnotations" {
    t.Fatalf("contract_name = %q", got)
  }
  if got := rawString(t, abi, "author"); got != "A K" {
    t.Fatalf("author = %q", got)
  }
  if got := rawString(t, abi, "version"); got != "1.0" {
    t.Fatalf("version = %q", got)
  }
  if got := rawString(t, abi, "description"); got != "some d" {
    t.Fatalf("description = %q", got)
  }
  if got := rawString(t, abi, "compiler_name"); got != "tolk" {
    t.Fatalf("compiler_name = %q", got)
  }
  if got := rawString(t, abi, "compiler_version"); !strings.HasPrefix(got, "1.") {
    t.Fatalf("compiler_version = %q", got)
  }

  msg1 := findRootByBodyStruct(t, rawSlice(t, abi, "incoming_messages"), abi, "Msg1")
  if got := rawString(t, msg1, "description"); got != "mmm1\nmmm2" {
    t.Fatalf("Msg1 description = %q", got)
  }
  reset := findRootByBodyTypeArgs(t, rawSlice(t, abi, "incoming_messages"), abi)
  if got := rawString(t, reset, "description"); got != "mmmReset" {
    t.Fatalf("generic reset description = %q", got)
  }
  ext := rawMapAt(t, rawSlice(t, abi, "incoming_external"), 0)
  extTy := rawTypeAt(t, abi, ext["body_ty_idx"])
  if rawString(t, extTy, "kind") != "StructRef" || rawString(t, extTy, "struct_name") != "ActualExternalShape" {
    t.Fatalf("incoming external type = %#v", extTy)
  }
  if got := rawString(t, ext, "description"); got != "mmmShape" {
    t.Fatalf("incoming external description = %q", got)
  }

  getFirst := findGetMethodRaw(t, abi, "getFirst")
  if got := rawString(t, getFirst, "description"); got != "get1" {
    t.Fatalf("getFirst description = %q", got)
  }
  if got := rawInt(t, getFirst, "tvm_method_id"); got != 90137 {
    t.Fatalf("getFirst method id = %d", got)
  }
  param := rawMapAt(t, rawSlice(t, getFirst, "parameters"), 0)
  if rawString(t, param, "name") != "spec" || rawString(t, param, "description") != "some number" {
    t.Fatalf("getFirst param = %#v", param)
  }
  if def := rawObj(t, param, "default_value"); rawString(t, def, "kind") != "int" || rawString(t, def, "v") != "50" {
    t.Fatalf("getFirst default = %#v", def)
  }

  outgoing := rawSlice(t, abi, "outgoing_messages")
  assertRawType(t, rawTypeAt(t, abi, rawMapAt(t, outgoing, 0)["body_ty_idx"]), "intN", "", 8)
  assertRawType(t, rawTypeAt(t, abi, rawMapAt(t, outgoing, 1)["body_ty_idx"]), "StructRef", "Transfer", 0)
  assertRawType(t, rawTypeAt(t, abi, rawMapAt(t, outgoing, 2)["body_ty_idx"]), "StructRef", "Out2", 0)
  out3 := rawTypeAt(t, abi, rawMapAt(t, outgoing, 3)["body_ty_idx"])
  if rawString(t, out3, "kind") != "StructRef" || rawString(t, out3, "struct_name") != "Out3" || len(rawSlice(t, out3, "type_args_ty_idx")) != 1 {
    t.Fatalf("Out3 type = %#v", out3)
  }

  events := rawSlice(t, abi, "emitted_events")
  if len(events) != 1 {
    t.Fatalf("emitted_events len = %d", len(events))
  }
  event := rawMapAt(t, events, 0)
  assertRawType(t, rawTypeAt(t, abi, event["body_ty_idx"]), "StructRef", "OutExt4", 0)
  if got := rawString(t, event, "description"); got != "mmmOut4" {
    t.Fatalf("event description = %q", got)
  }

  transfer := findDeclRaw(t, abi, "struct", "Transfer")
  forwardPayload := findFieldRaw(t, transfer, "forwardPayload")
  if got := rawString(t, forwardPayload, "description"); got != "actually it's not a slice" {
    t.Fatalf("forwardPayload description = %q", got)
  }
}

func TestFixtureABIStorageAtDeployment(t *testing.T) {
  abi := readRawFixtureABI(t, "has-not-init-storage")
  storage := rawObj(t, abi, "storage")
  ty := rawTypeAt(t, abi, storage["storage_at_deployment_ty_idx"])
  assertRawType(t, ty, "StructRef", "NftItemStorageNotInitialized", 0)
}

func TestFixtureABIDefaultValues(t *testing.T) {
  abi := readRawFixtureABI(t, "lots-of-storage")
  st := findDeclRaw(t, abi, "struct", "StWithAllDefaults")
  for _, rawField := range rawSlice(t, st, "fields") {
    field := asRawMap(t, rawField)
    if _, ok := field["default_value"]; !ok {
      t.Fatalf("field %s has no default_value", rawString(t, field, "name"))
    }
  }

  assertDefault := func(name string) map[string]any {
    return rawObj(t, findFieldRaw(t, st, name), "default_value")
  }
  i2 := assertDefault("i2")
  if rawString(t, i2, "kind") != "castTo" || rawString(t, rawObj(t, i2, "inner"), "kind") != "int" || rawString(t, rawObj(t, i2, "inner"), "v") != "50000000" {
    t.Fatalf("i2 default = %#v", i2)
  }
  if i5 := assertDefault("i5"); rawString(t, i5, "kind") != "int" || rawString(t, i5, "v") != "1267650600228229401496703205376" {
    t.Fatalf("i5 default = %#v", i5)
  }
  if rawString(t, assertDefault("i7"), "kind") != "null" {
    t.Fatalf("i7 default is not null")
  }
  if b3 := assertDefault("b3"); rawString(t, b3, "kind") != "bool" || b3["v"] != false {
    t.Fatalf("b3 default = %#v", b3)
  }
  if s1 := assertDefault("s1"); rawString(t, s1, "kind") != "slice" || rawString(t, s1, "hex") != "0102" {
    t.Fatalf("s1 default = %#v", s1)
  }
  if s4 := assertDefault("s4"); rawString(t, s4, "kind") != "slice" || rawString(t, s4, "hex") != "68656c6c6f312340" {
    t.Fatalf("s4 default = %#v", s4)
  }
  if a2 := assertDefault("a2"); rawString(t, a2, "kind") != "address" || rawString(t, a2, "addr") != "EQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAM9c" {
    t.Fatalf("a2 default = %#v", a2)
  }
  if rawString(t, assertDefault("a3"), "kind") != "null" {
    t.Fatalf("a3 default is not null")
  }
  if a4 := assertDefault("a4"); rawString(t, a4, "kind") != "castTo" || rawString(t, rawObj(t, a4, "inner"), "kind") != "address" || len(rawString(t, rawObj(t, a4, "inner"), "addr")) != 48 {
    t.Fatalf("a4 default = %#v", a4)
  }
  t3 := assertDefault("t3")
  if items := rawSlice(t, t3, "items"); rawString(t, asRawMap(t, items[0]), "kind") != "null" || rawString(t, asRawMap(t, items[1]), "kind") != "tensor" {
    t.Fatalf("t3 default = %#v", t3)
  }
  t4Items := rawSlice(t, assertDefault("t4"), "items")
  var t4Values []string
  for _, item := range t4Items {
    item := asRawMap(t, item)
    if rawString(t, item, "kind") != "int" {
      t.Fatalf("t4 item = %#v", item)
    }
    t4Values = append(t4Values, rawString(t, item, "v"))
  }
  if strings.Join(t4Values, ",") != "907060870,50018,20329878786436204988385760252021328656300425018755239228739303522659023427620,754077114,448378203247" {
    t.Fatalf("t4 values = %#v", t4Values)
  }
  sh1 := assertDefault("sh1")
  if sh1Items := rawSlice(t, sh1, "items"); rawString(t, asRawMap(t, sh1Items[0]), "v") != "1" || rawString(t, asRawMap(t, sh1Items[1]), "kind") != "null" {
    t.Fatalf("sh1 default = %#v", sh1)
  }
  sh2 := assertDefault("sh2")
  sh2Items := rawSlice(t, sh2, "items")
  if len(rawSlice(t, asRawMap(t, sh2Items[0]), "items")) != 2 {
    t.Fatalf("sh2 default = %#v", sh2)
  }
  if nested := rawSlice(t, asRawMap(t, sh2Items[1]), "items"); rawString(t, asRawMap(t, nested[0]), "kind") != "string" || rawString(t, asRawMap(t, nested[0]), "str") != "10" {
    t.Fatalf("sh2 nested default = %#v", nested)
  }
  if o2 := assertDefault("o2"); rawString(t, o2, "kind") != "object" || rawString(t, o2, "struct_name") != "Inner" || len(rawSlice(t, o2, "fields")) != 2 {
    t.Fatalf("o2 default = %#v", o2)
  }
  if rawString(t, assertDefault("o3"), "kind") != "null" {
    t.Fatalf("o3 default is not null")
  }
}

func TestFixtureABIThrows(t *testing.T) {
  abi := readRawFixtureABI(t, "lots-of-throws")
  errors := rawSlice(t, abi, "thrown_errors")
  var unnamed int
  var has201 bool
  for _, rawErr := range errors {
    errObj := asRawMap(t, rawErr)
    if _, ok := errObj["name"]; !ok {
      unnamed++
    }
    if rawInt(t, errObj, "err_code") == 201 {
      has201 = true
    }
  }
  if unnamed != 3 {
    t.Fatalf("unnamed thrown_errors = %d", unnamed)
  }
  if has201 {
    t.Fatalf("unexpected thrown error 201")
  }
  if err200 := findThrownErrorRaw(t, abi, 200); rawString(t, err200, "kind") != "plain_int" {
    t.Fatalf("err 200 = %#v", err200)
  }
  if got := rawString(t, findThrownErrorRaw(t, abi, 105), "description"); got != "desc for 105" {
    t.Fatalf("ERR_105 description = %q", got)
  }
  if got := rawString(t, findThrownErrorByNameRaw(t, abi, "Err.EInEnum2"), "description"); got != "desc for EInEnum2" {
    t.Fatalf("Err.EInEnum2 description = %q", got)
  }

  external := rawSlice(t, abi, "incoming_external")
  if len(external) != 1 {
    t.Fatalf("incoming_external len = %d", len(external))
  }
  extTy := rawTypeAt(t, abi, rawMapAt(t, external, 0)["body_ty_idx"])
  if rawString(t, extTy, "kind") != "slice" {
    t.Fatalf("incoming external type = %#v", extTy)
  }

  withUnsupported := findDeclRaw(t, abi, "struct", "WithUnsupportedDefaults")
  for _, rawField := range rawSlice(t, withUnsupported, "fields") {
    field := asRawMap(t, rawField)
    if _, ok := field["default_value"]; !ok {
      t.Fatalf("field %s has no default_value", rawString(t, field, "name"))
    }
  }
}

func TestGenerateThrownErrorSentinels(t *testing.T) {
  abi := abiFile{
    ContractName: "Sample",
    GetMethods: []getMethod{
      {
        Name:        "main",
        TVMMethodID: 42,
        ReturnType:  abiType{Kind: "void"},
      },
    },
    ThrownErrors: []thrownError{
      {Kind: "constant", Name: "ERR_FOO", Description: "foo happened", Code: 42},
      {Kind: "constant", Name: "AGAIN_FOO", Code: 42},
      {Kind: "plain_int", Code: 7},
    },
  }

  result, err := newGenerator(abi, "sample").Generate()
  src := result.Source
  if err != nil {
    t.Fatalf("generate: %v", err)
  }

  for _, want := range []string{
    `ErrFoo\s+= errors\.New\("ERR_FOO: foo happened \(exit code 42\)"\)`,
    `ErrAgainFoo\s+= errors\.New\("AGAIN_FOO \(exit code 42\)"\)`,
    `ErrCode7\s+= errors\.New\("contract exit code 7"\)`,
    `case 42:\s+return errors\.Join\(ErrFoo, ErrAgainFoo\)`,
    `return mapContractError\(err\)`,
  } {
    if !regexp.MustCompile(want).Match(src) {
      t.Fatalf("generated source does not match %q:\n%s", want, src)
    }
  }
  for _, notWant := range []string{
    "*ton.ContractExecError",
    "execErrPtr",
  } {
    if strings.Contains(string(src), notWant) {
      t.Fatalf("generated source contains %q:\n%s", notWant, src)
    }
  }

  runtimeTest := []byte(`package sample

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/ton"
)

type fakeAPI struct {
	err error
}

func (f fakeAPI) RunGetMethodByID(ctx context.Context, block *ton.BlockIDExt, addr *address.Address, methodID uint64, params ...any) (*ton.ExecutionResult, error) {
	return nil, f.err
}

func TestGeneratedThrownErrorMapping(t *testing.T) {
	addr := address.NewAddress(0, 0, make([]byte, 32))
	contract := NewSample(fakeAPI{err: fmt.Errorf("wrapped: %w", ton.ContractExecError{Code: 42})}, addr)
	err := contract.RunMethodMain(context.Background(), nil)
	if !errors.Is(err, ErrFoo) {
		t.Fatalf("err does not match ErrFoo: %v", err)
	}
	if !errors.Is(err, ErrAgainFoo) {
		t.Fatalf("err does not match ErrAgainFoo: %v", err)
	}
	if !errors.Is(err, ton.ContractExecError{Code: 42}) {
		t.Fatalf("err does not preserve ContractExecError: %v", err)
	}

	contract = NewSample(fakeAPI{err: ton.ContractExecError{Code: 99}}, addr)
	err = contract.RunMethodMain(context.Background(), nil)
	if errors.Is(err, ErrFoo) {
		t.Fatalf("unknown exit code should not match ErrFoo: %v", err)
	}
	if !errors.Is(err, ton.ContractExecError{Code: 99}) {
		t.Fatalf("unknown exit code should preserve original error: %v", err)
	}
}
`)
  if output, err := runGeneratedWrapperTest(t, src, runtimeTest); err != nil {
    t.Fatalf("generated thrown error mapping test failed: %v\n%s", err, output)
  }
}

func compileGeneratedWrapper(t *testing.T, src []byte) ([]byte, error) {
  t.Helper()

  dir := t.TempDir()
  writeGeneratedWrapperModule(t, dir)
  out := filepath.Join(dir, "wrapper.go")
  if err := os.WriteFile(out, src, 0o600); err != nil {
    t.Fatalf("write generated wrapper: %v", err)
  }

  cmd := exec.Command("go", "test", "-mod=mod", ".")
  cmd.Dir = dir
  return cmd.CombinedOutput()
}

func runGeneratedWrapperTest(t *testing.T, src []byte, testSrc []byte) ([]byte, error) {
  t.Helper()

  dir := t.TempDir()
  writeGeneratedWrapperModule(t, dir)
  wrapper := filepath.Join(dir, "wrapper.go")
  if err := os.WriteFile(wrapper, src, 0o600); err != nil {
    t.Fatalf("write generated wrapper: %v", err)
  }
  testFile := filepath.Join(dir, "wrapper_test.go")
  if err := os.WriteFile(testFile, testSrc, 0o600); err != nil {
    t.Fatalf("write generated wrapper test: %v", err)
  }

  cmd := exec.Command("go", "test", "-mod=mod", ".")
  cmd.Dir = dir
  return cmd.CombinedOutput()
}

func writeGeneratedWrapperModule(t *testing.T, dir string) {
  t.Helper()

  const mod = `module generatedwrapper

go 1.25.6

require github.com/xssnick/tonutils-go v1.17.0
`
  if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o600); err != nil {
    t.Fatalf("write generated wrapper module: %v", err)
  }
  sum, err := os.ReadFile("go.sum")
  if err != nil {
    t.Fatalf("read generator module sums: %v", err)
  }
  if err := os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0o600); err != nil {
    t.Fatalf("write generated wrapper sums: %v", err)
  }
}

func readRawFixtureABI(t *testing.T, name string) map[string]any {
  t.Helper()
  path := filepath.Join("testdata", "wrapper-fixtures", name+".abi.json")
  data, err := os.ReadFile(path)
  if err != nil {
    t.Fatalf("read fixture ABI %s: %v", name, err)
  }
  dec := json.NewDecoder(bytes.NewReader(data))
  dec.UseNumber()
  var out map[string]any
  if err := dec.Decode(&out); err != nil {
    t.Fatalf("decode fixture ABI %s: %v", name, err)
  }
  return out
}

func asRawMap(t *testing.T, value any) map[string]any {
  t.Helper()
  out, ok := value.(map[string]any)
  if !ok {
    t.Fatalf("expected object, got %T", value)
  }
  return out
}

func rawObj(t *testing.T, obj map[string]any, key string) map[string]any {
  t.Helper()
  return asRawMap(t, obj[key])
}

func rawSlice(t *testing.T, obj map[string]any, key string) []any {
  t.Helper()
  out, ok := obj[key].([]any)
  if !ok {
    t.Fatalf("expected %s array, got %T", key, obj[key])
  }
  return out
}

func rawMapAt(t *testing.T, items []any, index int) map[string]any {
  t.Helper()
  if index < 0 || index >= len(items) {
    t.Fatalf("index %d out of %d", index, len(items))
  }
  return asRawMap(t, items[index])
}

func rawString(t *testing.T, obj map[string]any, key string) string {
  t.Helper()
  out, ok := obj[key].(string)
  if !ok {
    t.Fatalf("expected %s string, got %T", key, obj[key])
  }
  return out
}

func rawInt(t *testing.T, obj map[string]any, key string) int64 {
  t.Helper()
  num, ok := obj[key].(json.Number)
  if !ok {
    t.Fatalf("expected %s number, got %T", key, obj[key])
  }
  out, err := num.Int64()
  if err != nil {
    t.Fatalf("parse %s number: %v", key, err)
  }
  return out
}

func rawTypeAt(t *testing.T, abi map[string]any, rawIndex any) map[string]any {
  t.Helper()
  num, ok := rawIndex.(json.Number)
  if !ok {
    t.Fatalf("expected type index number, got %T", rawIndex)
  }
  index, err := num.Int64()
  if err != nil {
    t.Fatalf("parse type index: %v", err)
  }
  return rawMapAt(t, rawSlice(t, abi, "unique_types"), int(index))
}

func findDeclRaw(t *testing.T, abi map[string]any, kind, name string) map[string]any {
  t.Helper()
  for _, rawDecl := range rawSlice(t, abi, "declarations") {
    decl := asRawMap(t, rawDecl)
    if rawString(t, decl, "kind") == kind && rawString(t, decl, "name") == name {
      return decl
    }
  }
  t.Fatalf("declaration %s %s not found", kind, name)
  return nil
}

func findFieldRaw(t *testing.T, decl map[string]any, name string) map[string]any {
  t.Helper()
  for _, rawField := range rawSlice(t, decl, "fields") {
    field := asRawMap(t, rawField)
    if rawString(t, field, "name") == name {
      return field
    }
  }
  t.Fatalf("field %s not found in %s", name, rawString(t, decl, "name"))
  return nil
}

func findGetMethodRaw(t *testing.T, abi map[string]any, name string) map[string]any {
  t.Helper()
  for _, rawMethod := range rawSlice(t, abi, "get_methods") {
    method := asRawMap(t, rawMethod)
    if rawString(t, method, "name") == name {
      return method
    }
  }
  t.Fatalf("get method %s not found", name)
  return nil
}

func findThrownErrorRaw(t *testing.T, abi map[string]any, code int64) map[string]any {
  t.Helper()
  for _, rawErr := range rawSlice(t, abi, "thrown_errors") {
    errObj := asRawMap(t, rawErr)
    if rawInt(t, errObj, "err_code") == code {
      return errObj
    }
  }
  t.Fatalf("thrown error %d not found", code)
  return nil
}

func findThrownErrorByNameRaw(t *testing.T, abi map[string]any, name string) map[string]any {
  t.Helper()
  for _, rawErr := range rawSlice(t, abi, "thrown_errors") {
    errObj := asRawMap(t, rawErr)
    if got, ok := errObj["name"].(string); ok && got == name {
      return errObj
    }
  }
  t.Fatalf("thrown error %s not found", name)
  return nil
}

func findRootByBodyStruct(t *testing.T, roots []any, abi map[string]any, structName string) map[string]any {
  t.Helper()
  for _, rawRoot := range roots {
    root := asRawMap(t, rawRoot)
    ty := rawTypeAt(t, abi, root["body_ty_idx"])
    if rawString(t, ty, "kind") == "StructRef" && rawString(t, ty, "struct_name") == structName {
      return root
    }
  }
  t.Fatalf("root with body struct %s not found", structName)
  return nil
}

func findRootByBodyTypeArgs(t *testing.T, roots []any, abi map[string]any) map[string]any {
  t.Helper()
  for _, rawRoot := range roots {
    root := asRawMap(t, rawRoot)
    ty := rawTypeAt(t, abi, root["body_ty_idx"])
    if rawString(t, ty, "kind") == "StructRef" {
      if args, ok := ty["type_args_ty_idx"].([]any); ok && len(args) > 0 {
        return root
      }
    }
  }
  t.Fatalf("root with generic body type not found")
  return nil
}

func assertRawType(t *testing.T, ty map[string]any, kind, name string, n int64) {
  t.Helper()
  if got := rawString(t, ty, "kind"); got != kind {
    t.Fatalf("type kind = %q, want %q: %#v", got, kind, ty)
  }
  if name != "" {
    if got := rawString(t, ty, "struct_name"); got != name {
      t.Fatalf("struct_name = %q, want %q: %#v", got, name, ty)
    }
  }
  if n != 0 {
    if got := rawInt(t, ty, "n"); got != n {
      t.Fatalf("n = %d, want %d: %#v", got, n, ty)
    }
  }
}

func containsAny(text string, options []string) bool {
  for _, option := range options {
    if strings.Contains(text, option) {
      return true
    }
  }
  return false
}
