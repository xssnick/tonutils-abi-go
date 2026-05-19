package main

import (
	"bytes"
	"fmt"
)

type helperID string

const (
	helperStackBits         helperID = "stackBits"
	helperLoadBits          helperID = "loadBits"
	helperBool              helperID = "bool"
	helperLoadAddr          helperID = "loadAddr"
	helperStackString       helperID = "stackString"
	helperLoadString        helperID = "loadString"
	helperDecodeInt         helperID = "decodeInt"
	helperDecodeBool        helperID = "decodeBool"
	helperDecodeAddr        helperID = "decodeAddr"
	helperDecodeCell        helperID = "decodeCell"
	helperDecodeSlice       helperID = "decodeSlice"
	helperDecodeBuilder     helperID = "decodeBuilder"
	helperDecodeBits        helperID = "decodeBits"
	helperDecodeString      helperID = "decodeString"
	helperDecodeTuple       helperID = "decodeTuple"
	helperDecodeList        helperID = "decodeList"
	helperStackList         helperID = "stackList"
	helperStackArray        helperID = "stackArray"
	helperNullablePtr       helperID = "nullablePtr"
	helperNullableValue     helperID = "nullableValue"
	helperWideNullablePtr   helperID = "wideNullablePtr"
	helperWideNullableValue helperID = "wideNullableValue"
	helperStackCell         helperID = "stackCell"
	helperBigIntLiteral     helperID = "bigIntLiteral"
)

type helperRegistry map[helperID]bool

func newHelperRegistry() helperRegistry {
	return helperRegistry{}
}

func (h helperRegistry) use(id helperID) {
	h[id] = true
}

func (h helperRegistry) has(id helperID) bool {
	return h != nil && h[id]
}

func (g *generator) useHelper(id helperID) {
	if g.helpers == nil {
		g.helpers = newHelperRegistry()
	}
	g.helpers.use(id)
}

func (g *generator) hasHelper(id helperID) bool {
	return g.helpers.has(id)
}

func (g *generator) useDecodeStackInt() {
	g.useHelper(helperDecodeInt)
}

func (g *generator) useDecodeStackBool() {
	g.useHelper(helperDecodeBool)
}

func (g *generator) useDecodeStackAddress() {
	g.useHelper(helperDecodeAddr)
	g.useDecodeStackSlice()
}

func (g *generator) useDecodeStackCell() {
	g.useHelper(helperDecodeCell)
}

func (g *generator) useDecodeStackSlice() {
	g.useHelper(helperDecodeSlice)
}

func (g *generator) useDecodeStackBuilder() {
	g.useHelper(helperDecodeBuilder)
}

func (g *generator) useDecodeStackBits() {
	g.useHelper(helperDecodeBits)
	g.useDecodeStackSlice()
}

func (g *generator) useDecodeStackString() {
	g.useHelper(helperDecodeString)
	g.useDecodeStackSlice()
}

func (g *generator) useDecodeStackTuple() {
	g.useHelper(helperDecodeTuple)
}

func (g *generator) useDecodeStackLispList() {
	g.useHelper(helperDecodeList)
}

func (g *generator) useStackStructEncoder(name string) {
	if name != "" {
		g.stackStructEncoders[name] = true
	}
}

func (g *generator) useStackResultDecoder(name string) {
	if name != "" {
		g.stackResultDecoders[name] = true
	}
}

func (g *generator) writeHelpers(dst *bytes.Buffer) {
	g.writeMapTypes(dst)
	g.writeStackResultDecoders(dst)
	g.writeStackStructEncoders(dst)
	g.writeGeneratedTypes(dst)

	for _, typ := range g.resultTypes {
		dst.WriteString(typ)
	}

	if g.hasHelper(helperStackBits) {
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		dst.WriteString("func stackBits(value []byte, bitLen uint) *cell.Slice {\n")
		dst.WriteString("\treturn cell.BeginCell().MustStoreSlice(value, bitLen).ToSlice()\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperLoadBits) {
		dst.WriteString("func loadBitsResult(result *ton.ExecutionResult, index uint, bitLen uint) ([]byte, error) {\n")
		dst.WriteString("\tslice, err := result.Slice(index)\n")
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn nil, err\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn slice.LoadSlice(bitLen)\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperBool) {
		dst.WriteString("func boolToStack(value bool) int64 {\n")
		dst.WriteString("\tif value {\n")
		dst.WriteString("\t\treturn -1\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn 0\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperLoadAddr) {
		g.useImport("github.com/xssnick/tonutils-go/address")
		dst.WriteString("func loadAddressResult(result *ton.ExecutionResult, index uint) (*address.Address, error) {\n")
		dst.WriteString("\tslice, err := result.Slice(index)\n")
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn nil, err\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn slice.LoadAddr()\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperStackString) {
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		dst.WriteString("func stackString(value string) *cell.Slice {\n")
		dst.WriteString("\treturn cell.BeginCell().MustStoreStringSnake(value).EndCell().MustBeginParse()\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperLoadString) {
		dst.WriteString("func loadStringResult(result *ton.ExecutionResult, index uint) (string, error) {\n")
		dst.WriteString("\tslice, err := result.Slice(index)\n")
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn \"\", err\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn slice.LoadStringSnake()\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperDecodeInt) {
		g.useImport("fmt")
		g.useImport("math/big")
		dst.WriteString("func decodeStackInt(value any) (*big.Int, error) {\n")
		dst.WriteString("\tout, ok := value.(*big.Int)\n")
		dst.WriteString("\tif !ok {\n")
		dst.WriteString("\t\treturn nil, fmt.Errorf(\"expected stack int, got %T\", value)\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn out, nil\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperDecodeBool) {
		g.useImport("fmt")
		g.useImport("math/big")
		dst.WriteString("func decodeStackBool(value any) (bool, error) {\n")
		dst.WriteString("\tout, ok := value.(*big.Int)\n")
		dst.WriteString("\tif !ok {\n")
		dst.WriteString("\t\treturn false, fmt.Errorf(\"expected stack bool, got %T\", value)\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn out.Sign() != 0, nil\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperDecodeAddr) {
		g.useImport("github.com/xssnick/tonutils-go/address")
		dst.WriteString("func decodeStackAddress(value any) (*address.Address, error) {\n")
		dst.WriteString("\tslice, err := decodeStackSlice(value)\n")
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn nil, err\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn slice.LoadAddr()\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperDecodeCell) {
		g.useImport("fmt")
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		dst.WriteString("func decodeStackCell(value any) (*cell.Cell, error) {\n")
		dst.WriteString("\tout, ok := value.(*cell.Cell)\n")
		dst.WriteString("\tif !ok {\n")
		dst.WriteString("\t\treturn nil, fmt.Errorf(\"expected stack cell, got %T\", value)\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn out, nil\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperDecodeSlice) {
		g.useImport("fmt")
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		dst.WriteString("func decodeStackSlice(value any) (*cell.Slice, error) {\n")
		dst.WriteString("\tswitch v := value.(type) {\n")
		dst.WriteString("\tcase *cell.Slice:\n")
		dst.WriteString("\t\treturn v, nil\n")
		dst.WriteString("\tcase *cell.Cell:\n")
		dst.WriteString("\t\treturn v.BeginParse()\n")
		dst.WriteString("\tdefault:\n")
		dst.WriteString("\t\treturn nil, fmt.Errorf(\"expected stack slice, got %T\", value)\n")
		dst.WriteString("\t}\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperDecodeBuilder) {
		g.useImport("fmt")
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		dst.WriteString("func decodeStackBuilder(value any) (*cell.Builder, error) {\n")
		dst.WriteString("\tout, ok := value.(*cell.Builder)\n")
		dst.WriteString("\tif !ok {\n")
		dst.WriteString("\t\treturn nil, fmt.Errorf(\"expected stack builder, got %T\", value)\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn out, nil\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperDecodeBits) {
		dst.WriteString("func decodeStackBits(value any, bitLen uint) ([]byte, error) {\n")
		dst.WriteString("\tslice, err := decodeStackSlice(value)\n")
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn nil, err\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn slice.LoadSlice(bitLen)\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperDecodeString) {
		dst.WriteString("func decodeStackString(value any) (string, error) {\n")
		dst.WriteString("\tslice, err := decodeStackSlice(value)\n")
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn \"\", err\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn slice.LoadStringSnake()\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperDecodeTuple) {
		g.useImport("fmt")
		dst.WriteString("func decodeStackTuple(value any) ([]any, error) {\n")
		dst.WriteString("\tout, ok := value.([]any)\n")
		dst.WriteString("\tif !ok {\n")
		dst.WriteString("\t\treturn nil, fmt.Errorf(\"expected stack tuple, got %T\", value)\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn out, nil\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperDecodeList) {
		g.useImport("fmt")
		dst.WriteString("func decodeStackLispList(value any) ([]any, error) {\n")
		dst.WriteString("\tvar out []any\n")
		dst.WriteString("\tfor value != nil {\n")
		dst.WriteString("\t\tpair, ok := value.([]any)\n")
		dst.WriteString("\t\tif !ok {\n")
		dst.WriteString("\t\t\treturn nil, fmt.Errorf(\"expected stack lisp list pair, got %T\", value)\n")
		dst.WriteString("\t\t}\n")
		dst.WriteString("\t\tif len(pair) != 2 {\n")
		dst.WriteString("\t\t\treturn nil, fmt.Errorf(\"expected stack lisp list pair length 2, got %d\", len(pair))\n")
		dst.WriteString("\t\t}\n")
		dst.WriteString("\t\tout = append(out, pair[0])\n")
		dst.WriteString("\t\tvalue = pair[1]\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn out, nil\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperStackList) {
		dst.WriteString("func stackLispList[T any](values []T, encode func(T) any) any {\n")
		dst.WriteString("\tvar out any\n")
		dst.WriteString("\tfor i := len(values) - 1; i >= 0; i-- {\n")
		dst.WriteString("\t\tout = []any{encode(values[i]), out}\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn out\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperStackArray) {
		dst.WriteString("func stackArray[T any](values []T, encode func(T) any) []any {\n")
		dst.WriteString("\tout := make([]any, 0, len(values))\n")
		dst.WriteString("\tfor _, value := range values {\n")
		dst.WriteString("\t\tout = append(out, encode(value))\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn out\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperNullablePtr) {
		dst.WriteString("func stackNullablePtr[T any](value *T, encode func(T) any) any {\n")
		dst.WriteString("\tif value == nil {\n")
		dst.WriteString("\t\treturn nil\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn encode(*value)\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperNullableValue) {
		g.useImport("reflect")
		dst.WriteString("func stackNullableValue[T any](value T, encode func(T) any) any {\n")
		dst.WriteString("\trv := reflect.ValueOf(value)\n")
		dst.WriteString("\tif !rv.IsValid() {\n")
		dst.WriteString("\t\treturn nil\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\tswitch rv.Kind() {\n")
		dst.WriteString("\tcase reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:\n")
		dst.WriteString("\t\tif rv.IsNil() {\n")
		dst.WriteString("\t\t\treturn nil\n")
		dst.WriteString("\t\t}\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn encode(value)\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperWideNullablePtr) {
		dst.WriteString("func stackWideNullablePtr[T any](value *T, stackWidth int, stackTypeID int64, encode func(T) []any) []any {\n")
		dst.WriteString("\tif value == nil {\n")
		dst.WriteString("\t\tout := make([]any, 0, stackWidth)\n")
		dst.WriteString("\t\tfor i := 0; i < stackWidth-1; i++ {\n")
		dst.WriteString("\t\t\tout = append(out, nil)\n")
		dst.WriteString("\t\t}\n")
		dst.WriteString("\t\treturn append(out, int64(0))\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\tout := encode(*value)\n")
		dst.WriteString("\treturn append(out, stackTypeID)\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperWideNullableValue) {
		g.useImport("reflect")
		dst.WriteString("func stackWideNullableValue[T any](value T, stackWidth int, stackTypeID int64, encode func(T) []any) []any {\n")
		dst.WriteString("\trv := reflect.ValueOf(value)\n")
		dst.WriteString("\tif !rv.IsValid() {\n")
		dst.WriteString("\t\tout := make([]any, 0, stackWidth)\n")
		dst.WriteString("\t\tfor i := 0; i < stackWidth-1; i++ {\n")
		dst.WriteString("\t\t\tout = append(out, nil)\n")
		dst.WriteString("\t\t}\n")
		dst.WriteString("\t\treturn append(out, int64(0))\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\tswitch rv.Kind() {\n")
		dst.WriteString("\tcase reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:\n")
		dst.WriteString("\t\tif rv.IsNil() {\n")
		dst.WriteString("\t\t\tout := make([]any, 0, stackWidth)\n")
		dst.WriteString("\t\t\tfor i := 0; i < stackWidth-1; i++ {\n")
		dst.WriteString("\t\t\t\tout = append(out, nil)\n")
		dst.WriteString("\t\t\t}\n")
		dst.WriteString("\t\t\treturn append(out, int64(0))\n")
		dst.WriteString("\t\t}\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\tout := encode(value)\n")
		dst.WriteString("\treturn append(out, stackTypeID)\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperStackCell) {
		g.useImport("github.com/xssnick/tonutils-go/tlb")
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		dst.WriteString("func stackCellOf(value any) (*cell.Cell, error) {\n")
		dst.WriteString("\tif c, ok := value.(*cell.Cell); ok {\n")
		dst.WriteString("\t\treturn c, nil\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\tc, err := tlb.ToCell(value)\n")
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn nil, err\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn c, nil\n")
		dst.WriteString("}\n\n")
		dst.WriteString("func mustStackCellOf(value any) *cell.Cell {\n")
		dst.WriteString("\tc, err := stackCellOf(value)\n")
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\tpanic(err)\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn c\n")
		dst.WriteString("}\n\n")
	}

	if g.hasHelper(helperBigIntLiteral) {
		g.useImport("math/big")
		dst.WriteString("func tugenMustBigInt(value string) *big.Int {\n")
		dst.WriteString("\tout, ok := new(big.Int).SetString(value, 10)\n")
		dst.WriteString("\tif !ok {\n")
		dst.WriteString("\t\tpanic(\"invalid generated big.Int literal: \" + value)\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn out\n")
		dst.WriteString("}\n\n")
	}
}

func (g *generator) writeGeneratedTypes(dst *bytes.Buffer) {
	for _, typ := range g.generatedTypes {
		dst.WriteString(typ)
	}
}

func (g *generator) writeMapTypes(dst *bytes.Buffer) {
	for _, spec := range g.mapTypes {
		g.useImport("errors")
		g.useImport("fmt")
		g.useImport("github.com/xssnick/tonutils-go/tlb")
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")

		fmt.Fprintf(dst, "type %s struct {\n", spec.TypeName)
		fmt.Fprintf(dst, "\tDict *cell.Dictionary `tlb:%q`\n", fmt.Sprintf("dict %d", spec.KeyBits))
		dst.WriteString("}\n\n")

		fmt.Fprintf(dst, "type %s struct {\n", spec.KeyBoxName)
		fmt.Fprintf(dst, "\tKey %s `tlb:%q`\n", spec.KeyGoType, spec.KeyTLBTag)
		dst.WriteString("}\n\n")

		fmt.Fprintf(dst, "type %s struct {\n", spec.ValueBoxName)
		fmt.Fprintf(dst, "\tValue %s `tlb:%q`\n", spec.ValueGoType, spec.ValueTLBTag)
		dst.WriteString("}\n\n")

		fmt.Fprintf(dst, "func %s() *%s {\n", spec.Constructor, spec.TypeName)
		fmt.Fprintf(dst, "\treturn &%s{Dict: cell.NewDict(%d)}\n", spec.TypeName, spec.KeyBits)
		dst.WriteString("}\n\n")

		fmt.Fprintf(dst, "func (m *%s) dict() *cell.Dictionary {\n", spec.TypeName)
		dst.WriteString("\tif m.Dict == nil {\n")
		fmt.Fprintf(dst, "\t\tm.Dict = cell.NewDict(%d)\n", spec.KeyBits)
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn m.Dict\n")
		dst.WriteString("}\n\n")

		keyToCell := unexportedName(spec.TypeName) + "KeyToCell"
		keyFromSlice := unexportedName(spec.TypeName) + "KeyFromSlice"

		fmt.Fprintf(dst, "func %s(key %s) (*cell.Cell, error) {\n", keyToCell, spec.KeyGoType)
		fmt.Fprintf(dst, "\tc, err := tlb.ToCell(%s{Key: key})\n", spec.KeyBoxName)
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn nil, err\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn c, nil\n")
		dst.WriteString("}\n\n")

		fmt.Fprintf(dst, "func %s(loader *cell.Slice) (%s, error) {\n", keyFromSlice, spec.KeyGoType)
		keyZero := "zero"
		if spec.KeyZero == "nil" {
			keyZero = "nil"
		} else {
			fmt.Fprintf(dst, "\tvar zero %s\n", spec.KeyGoType)
		}
		fmt.Fprintf(dst, "\tvar box %s\n", spec.KeyBoxName)
		dst.WriteString("\tif err := tlb.LoadFromCell(&box, loader); err != nil {\n")
		fmt.Fprintf(dst, "\t\treturn %s, err\n", keyZero)
		dst.WriteString("\t}\n")
		dst.WriteString("\tif loader.BitsLeft() != 0 || loader.RefsNum() != 0 {\n")
		fmt.Fprintf(dst, "\t\treturn %s, fmt.Errorf(\"dict key has trailing data: %%d bits and %%d refs\", loader.BitsLeft(), loader.RefsNum())\n", keyZero)
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn box.Key, nil\n")
		dst.WriteString("}\n\n")

		fmt.Fprintf(dst, "func (m *%s) Get(key %s) (%s, bool, error) {\n", spec.TypeName, spec.KeyGoType, spec.ValueGoType)
		valueZero := "zero"
		if spec.ValueZero == "nil" {
			valueZero = "nil"
		} else {
			fmt.Fprintf(dst, "\tvar zero %s\n", spec.ValueGoType)
		}
		fmt.Fprintf(dst, "\tkeyCell, err := %s(key)\n", keyToCell)
		dst.WriteString("\tif err != nil {\n")
		fmt.Fprintf(dst, "\t\treturn %s, false, err\n", valueZero)
		dst.WriteString("\t}\n")
		dst.WriteString("\tvalue, err := m.dict().LoadValue(keyCell)\n")
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\tif errors.Is(err, cell.ErrNoSuchKeyInDict) {\n")
		fmt.Fprintf(dst, "\t\t\treturn %s, false, nil\n", valueZero)
		dst.WriteString("\t\t}\n")
		fmt.Fprintf(dst, "\t\treturn %s, false, err\n", valueZero)
		dst.WriteString("\t}\n")
		fmt.Fprintf(dst, "\tvar box %s\n", spec.ValueBoxName)
		dst.WriteString("\tif err := tlb.LoadFromCell(&box, value); err != nil {\n")
		fmt.Fprintf(dst, "\t\treturn %s, true, err\n", valueZero)
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn box.Value, true, nil\n")
		dst.WriteString("}\n\n")

		fmt.Fprintf(dst, "func (m *%s) Set(key %s, value %s) error {\n", spec.TypeName, spec.KeyGoType, spec.ValueGoType)
		fmt.Fprintf(dst, "\tkeyCell, err := %s(key)\n", keyToCell)
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn err\n")
		dst.WriteString("\t}\n")
		fmt.Fprintf(dst, "\tvalueCell, err := tlb.ToCell(%s{Value: value})\n", spec.ValueBoxName)
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn err\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn m.dict().Set(keyCell, valueCell)\n")
		dst.WriteString("}\n\n")

		fmt.Fprintf(dst, "func (m *%s) Delete(key %s) error {\n", spec.TypeName, spec.KeyGoType)
		dst.WriteString("\tif m.Dict == nil {\n")
		dst.WriteString("\t\treturn nil\n")
		dst.WriteString("\t}\n")
		fmt.Fprintf(dst, "\tkeyCell, err := %s(key)\n", keyToCell)
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn err\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn m.Dict.Delete(keyCell)\n")
		dst.WriteString("}\n\n")

		fmt.Fprintf(dst, "func (m *%s) LoadAll() ([]%s, []%s, error) {\n", spec.TypeName, spec.KeyGoType, spec.ValueGoType)
		dst.WriteString("\titems, err := m.dict().LoadAll()\n")
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn nil, nil, err\n")
		dst.WriteString("\t}\n")
		fmt.Fprintf(dst, "\tkeys := make([]%s, 0, len(items))\n", spec.KeyGoType)
		fmt.Fprintf(dst, "\tvalues := make([]%s, 0, len(items))\n", spec.ValueGoType)
		dst.WriteString("\tfor _, item := range items {\n")
		fmt.Fprintf(dst, "\t\tkey, err := %s(item.Key)\n", keyFromSlice)
		dst.WriteString("\t\tif err != nil {\n")
		dst.WriteString("\t\t\treturn nil, nil, err\n")
		dst.WriteString("\t\t}\n")
		fmt.Fprintf(dst, "\t\tvar box %s\n", spec.ValueBoxName)
		dst.WriteString("\t\tif err := tlb.LoadFromCell(&box, item.Value); err != nil {\n")
		dst.WriteString("\t\t\treturn nil, nil, err\n")
		dst.WriteString("\t\t}\n")
		dst.WriteString("\t\tkeys = append(keys, key)\n")
		dst.WriteString("\t\tvalues = append(values, box.Value)\n")
		dst.WriteString("\t}\n")
		dst.WriteString("\treturn keys, values, nil\n")
		dst.WriteString("}\n\n")
	}
}
