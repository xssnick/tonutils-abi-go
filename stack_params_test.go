package main

import (
	"regexp"
	"testing"
)

func TestGenerateUnsupportedStackParamReturnsError(t *testing.T) {
	abi := abiFile{
		ContractName: "Sample",
		GetMethods: []getMethod{
			{
				Name:        "pass_bad",
				TVMMethodID: 77,
				Parameters: []parameter{
					{Name: "bad", Type: abiType{Kind: "unsupportedStackParam"}},
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
		`TODO: stack parameter bad for RunMethodPassBad returns an encode error because its type is not generated yet: unsupported ABI type kind unsupportedStackParam\.`,
		`return fmt\.Errorf\("encode stack parameter bad: unsupported ABI type kind unsupportedStackParam"\)`,
		`RunGetMethodByID\(ctx, block, c\.addr, uint64\(77\), params\.\.\.\)`,
	} {
		if !regexp.MustCompile(want).Match(src) {
			t.Fatalf("generated source does not match %q:\n%s", want, src)
		}
	}
	if regexp.MustCompile(`RunGetMethodByID\(ctx, block, c\.addr, uint64\(77\), bad\)`).Match(src) {
		t.Fatalf("unsupported param was passed raw:\n%s", src)
	}
}

func TestGeneratedStackCellParamReturnsEncodeError(t *testing.T) {
	abi := abiFile{
		ContractName: "Sample",
		Declarations: []declaration{
			{
				Kind: "struct",
				Name: "Payload",
				CustomPackUnpack: &customPackUnpack{
					PackToBuilder:   true,
					UnpackFromSlice: true,
				},
			},
		},
		GetMethods: []getMethod{
			{
				Name:        "send",
				TVMMethodID: 1,
				Parameters: []parameter{
					{
						Name: "payload",
						Type: abiType{
							Kind:  "cellOf",
							Inner: &abiType{Kind: "StructRef", StructName: "Payload"},
						},
					},
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

	runtimeTest := []byte(`package sample

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/tvm/cell"
)

type fakeAPI struct {
	calls int
}

func (f *fakeAPI) RunGetMethodByID(ctx context.Context, block *ton.BlockIDExt, addr *address.Address, methodID uint64, params ...any) (*ton.ExecutionResult, error) {
	f.calls++
	return nil, nil
}

func TestGeneratedStackCellParamReturnsEncodeError(t *testing.T) {
	api := &fakeAPI{}
	addr := address.NewAddress(0, 0, make([]byte, 32))
	contract := NewSample(api, addr)
	want := errors.New("boom")
	SetPayloadToCell(func(value *Payload) (*cell.Cell, error) {
		return nil, want
	})
	err := contract.RunMethodSend(context.Background(), nil, Payload{})
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("err = %v, want wrapped boom", err)
	}
	if !strings.Contains(err.Error(), "encode stack parameter payload") {
		t.Fatalf("err does not include parameter context: %v", err)
	}
	if api.calls != 0 {
		t.Fatalf("api was called before parameter encoding succeeded")
	}
}
`)
	if output, err := runGeneratedWrapperTest(t, src, runtimeTest); err != nil {
		t.Fatalf("generated stack cell param runtime test failed: %v\n%s", err, output)
	}
}
