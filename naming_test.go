package main

import (
	"regexp"
	"testing"
)

func TestGeneratedMapConstructorUsesAllocator(t *testing.T) {
	abi := abiFile{
		ContractName: "Sample",
		Declarations: []declaration{
			{
				Kind:   "alias",
				Name:   "NewU32CellMap",
				Target: abiType{Kind: "uintN", N: 8},
			},
			{
				Kind: "alias",
				Name: "U32CellMap",
				Target: abiType{
					Kind:  "mapKV",
					Key:   &abiType{Kind: "uintN", N: 32},
					Value: &abiType{Kind: "cell"},
				},
			},
		},
		Storage: &abiStorage{StorageType: abiType{Kind: "AliasRef", AliasName: "U32CellMap"}},
	}

	result, err := newGenerator(abi, "sample").Generate()
	src := result.Source
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	want := `func NewU32CellMap2\(\) \*U32CellMap`
	if !regexp.MustCompile(want).Match(src) {
		t.Fatalf("generated source does not match %q:\n%s", want, src)
	}
	if regexp.MustCompile(`func NewU32CellMap\(\) \*U32CellMap`).Match(src) {
		t.Fatalf("map constructor collided with package declaration:\n%s", src)
	}
}

func TestGeneratedUnionConstructorUsesAllocator(t *testing.T) {
	prefix0 := uint64(0)
	prefix1 := uint64(1)
	abi := abiFile{
		ContractName: "Sample",
		Declarations: []declaration{
			{
				Kind:   "alias",
				Name:   "NewPayloadInline",
				Target: abiType{Kind: "uintN", N: 8},
			},
			{
				Kind:   "struct",
				Name:   "PayloadInline",
				Prefix: &prefix{PrefixNum: &prefix0, PrefixLen: 1},
				Fields: []field{{Name: "value", Type: abiType{Kind: "uintN", N: 8}}},
			},
			{
				Kind:   "struct",
				Name:   "PayloadInRef",
				Prefix: &prefix{PrefixNum: &prefix1, PrefixLen: 1},
				Fields: []field{{Name: "value", Type: abiType{Kind: "uintN", N: 16}}},
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
		`func NewPayloadInline2\(value PayloadInline\) Payload`,
		`func NewPayloadInRef\(value PayloadInRef\) Payload`,
	} {
		if !regexp.MustCompile(want).Match(src) {
			t.Fatalf("generated source does not match %q:\n%s", want, src)
		}
	}
	if regexp.MustCompile(`func NewPayloadInline\(value PayloadInline\) Payload`).Match(src) {
		t.Fatalf("union constructor collided with package declaration:\n%s", src)
	}
}
