package main

import (
	"fmt"
	"strings"
)

type mapSpec struct {
	TypeName     string
	Constructor  string
	KeyBoxName   string
	ValueBoxName string
	KeyBits      int
	KeyGoType    string
	KeyZero      string
	KeyTLBTag    string
	ValueGoType  string
	ValueZero    string
	ValueTLBTag  string
}

type dictTypeInfo struct {
	GoType    string
	TLBTag    string
	Bits      int
	Supported bool
	Reason    string
	Zero      string
}

func (g *generator) mapTypeForTLB(typ abiType, suggestedName string) typeInfo {
	if typ.Key == nil || typ.Value == nil {
		return unsupported("mapKV without key or value type")
	}

	key := g.dictKeyType(*typ.Key, map[string]bool{})
	if !key.Supported {
		return unsupported("map key " + key.Reason)
	}

	value := g.dictValueType(*typ.Value)
	if !value.Supported {
		return unsupported("map value " + value.Reason)
	}

	typeName := g.mapTypeName(typ, suggestedName)
	if !g.mapTypeSet[typeName] {
		spec := mapSpec{
			TypeName:     typeName,
			Constructor:  g.mapConstructorName(typeName),
			KeyBoxName:   unexportedName(typeName) + "KeyBox",
			ValueBoxName: unexportedName(typeName) + "ValueBox",
			KeyBits:      key.Bits,
			KeyGoType:    key.GoType,
			KeyZero:      key.Zero,
			KeyTLBTag:    key.TLBTag,
			ValueGoType:  value.GoType,
			ValueZero:    value.Zero,
			ValueTLBTag:  value.TLBTag,
		}
		g.mapTypeSet[typeName] = true
		g.mapTypes = append(g.mapTypes, spec)
	}

	return typeInfo{
		GoType:    typeName,
		TLBTag:    ".",
		Supported: true,
		Kind:      "map",
		Zero:      typeName + "{}",
	}
}

func (g *generator) mapTypeName(typ abiType, suggestedName string) string {
	key := suggestedName
	if key == "" {
		key = "Dict" + mapTypeSignature(typ)
	}
	if typeName, ok := g.mapTypeNames[key]; ok {
		return typeName
	}

	base := exportedName(suggestedName)
	if base == "" {
		base = "Dict"
	}
	if g.mapAliasCanUseName(base, typ) {
		g.mapTypeNames[key] = base
		return base
	}
	if g.mapTypeNameTaken(base, typ) {
		base += "Map"
	}
	if g.names != nil {
		typeName := g.names.uniquePackage(base, "Dict")
		g.mapTypeNames[key] = typeName
		return typeName
	}

	typeName := base
	for i := 2; g.mapTypeNameTaken(typeName, typ) || g.mapTypeSet[typeName]; i++ {
		typeName = fmt.Sprintf("%s%d", base, i)
	}
	g.mapTypeNames[key] = typeName
	return typeName
}

func (g *generator) mapConstructorName(typeName string) string {
	if g.names == nil {
		return "New" + typeName
	}
	return g.names.uniquePackage("New"+typeName, "NewDict")
}

func (g *generator) mapTypeNameTaken(name string, typ abiType) bool {
	return g.typeNameTaken(name) && !g.mapAliasCanUseName(name, typ)
}

func (g *generator) mapAliasCanUseName(name string, typ abiType) bool {
	for _, decl := range g.aliases {
		if exportedName(decl.Name) == name && len(decl.TypeParams) == 0 && mapTypeSignature(decl.Target) == mapTypeSignature(typ) {
			return true
		}
	}
	return false
}

func mapTypeSignature(typ abiType) string {
	var b strings.Builder
	writeABITypeSignature(&b, typ)
	return b.String()
}

func writeABITypeSignature(b *strings.Builder, typ abiType) {
	b.WriteString(typ.Kind)
	if typ.N != 0 {
		fmt.Fprintf(b, "#%d", typ.N)
	}
	if typ.AliasName != "" {
		b.WriteString(":alias=")
		b.WriteString(typ.AliasName)
	}
	if typ.StructName != "" {
		b.WriteString(":struct=")
		b.WriteString(typ.StructName)
	}
	if typ.EnumName != "" {
		b.WriteString(":enum=")
		b.WriteString(typ.EnumName)
	}
	if typ.NameT != "" {
		b.WriteString(":T=")
		b.WriteString(typ.NameT)
	}
	if typ.StackTypeID != nil {
		fmt.Fprintf(b, ":sid=%d", *typ.StackTypeID)
	}
	if typ.StackWidth != nil {
		fmt.Fprintf(b, ":sw=%d", *typ.StackWidth)
	}
	writeABITypeListSignature(b, "args", typ.TypeArgs)
	if typ.Inner != nil {
		b.WriteString(":inner(")
		writeABITypeSignature(b, *typ.Inner)
		b.WriteString(")")
	}
	writeABITypeListSignature(b, "items", typ.Items)
	if len(typ.Variants) > 0 {
		b.WriteString(":variants[")
		for i, variant := range typ.Variants {
			if i > 0 {
				b.WriteString(",")
			}
			if variant.PrefixNum != nil {
				fmt.Fprintf(b, "prefix=%d/%d:", *variant.PrefixNum, variant.PrefixLen)
			}
			if variant.StackTypeID != nil {
				fmt.Fprintf(b, "sid=%d:", *variant.StackTypeID)
			}
			if variant.StackWidth != nil {
				fmt.Fprintf(b, "sw=%d:", *variant.StackWidth)
			}
			writeABITypeSignature(b, variant.Type)
		}
		b.WriteString("]")
	}
	if typ.Key != nil {
		b.WriteString(":key(")
		writeABITypeSignature(b, *typ.Key)
		b.WriteString(")")
	}
	if typ.Value != nil {
		b.WriteString(":value(")
		writeABITypeSignature(b, *typ.Value)
		b.WriteString(")")
	}
}

func writeABITypeListSignature(b *strings.Builder, label string, items []abiType) {
	if len(items) == 0 {
		return
	}
	b.WriteString(":")
	b.WriteString(label)
	b.WriteString("[")
	for i, item := range items {
		if i > 0 {
			b.WriteString(",")
		}
		writeABITypeSignature(b, item)
	}
	b.WriteString("]")
}

func (g *generator) typeNameTaken(name string) bool {
	if name == "" {
		return true
	}
	if g.names != nil && g.names.packageNameUsed(name) {
		return true
	}
	if _, ok := g.aliases[name]; ok {
		return true
	}
	for _, decl := range g.aliases {
		if exportedName(decl.Name) == name {
			return true
		}
	}
	if _, ok := g.enums[name]; ok {
		return true
	}
	for _, decl := range g.enums {
		if exportedName(decl.Name) == name {
			return true
		}
	}
	if _, ok := g.structs[name]; ok {
		return true
	}
	for _, decl := range g.structs {
		if exportedName(decl.Name) == name {
			return true
		}
	}
	return goKeywords[name]
}

func (g *generator) dictValueType(typ abiType) typeInfo {
	if typ.Kind == "slice" {
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return typeInfo{
			GoType:    "*cell.Cell",
			TLBTag:    ".",
			Supported: true,
			Kind:      "slice",
			Zero:      "nil",
		}
	}

	info := g.typeForTLBNamed(typ, true, "")
	if !info.Supported {
		return info
	}
	if info.TLBTag == "" {
		return unsupported("type has no TLB tag")
	}
	return info
}

func (g *generator) dictKeyType(typ abiType, seen map[string]bool) dictTypeInfo {
	switch typ.Kind {
	case "uintN":
		goType := intGoType(typ.N, false)
		if goType == "*big.Int" {
			g.useImport("math/big")
		}
		return dictTypeInfo{GoType: goType, TLBTag: fmt.Sprintf("## %d", typ.N), Bits: typ.N, Supported: true, Zero: zeroValue(goType)}
	case "intN":
		goType := intGoType(typ.N, true)
		if goType == "*big.Int" {
			g.useImport("math/big")
		}
		return dictTypeInfo{GoType: goType, TLBTag: fmt.Sprintf("## %d", typ.N), Bits: typ.N, Supported: true, Zero: zeroValue(goType)}
	case "bool":
		return dictTypeInfo{GoType: "bool", TLBTag: "bool", Bits: 1, Supported: true, Zero: "false"}
	case "bitsN":
		return dictTypeInfo{GoType: "[]byte", TLBTag: fmt.Sprintf("bits %d", typ.N), Bits: typ.N, Supported: true, Zero: "nil"}
	case "bytesN":
		return dictTypeInfo{GoType: "[]byte", TLBTag: fmt.Sprintf("bits %d", typ.N*8), Bits: typ.N * 8, Supported: true, Zero: "nil"}
	case "address":
		g.useImport("github.com/xssnick/tonutils-go/address")
		return dictTypeInfo{GoType: "*address.Address", TLBTag: "addr std required", Bits: 267, Supported: true, Zero: "nil"}
	case "EnumRef":
		decl, ok := g.enums[typ.EnumName]
		if !ok || decl.EncodedAs == nil {
			return unsupportedDictKey("unknown enum " + typ.EnumName)
		}
		base := g.dictKeyType(*decl.EncodedAs, seen)
		if !base.Supported {
			return unsupportedDictKey("enum " + typ.EnumName + " encoding: " + base.Reason)
		}
		base.GoType = exportedName(typ.EnumName)
		return base
	case "AliasRef":
		decl, ok := g.aliases[typ.AliasName]
		if !ok {
			return unsupportedDictKey("unknown alias " + typ.AliasName)
		}
		if len(decl.TypeParams) > 0 || len(typ.TypeArgs) > 0 {
			return unsupportedDictKey("generic alias " + typ.AliasName)
		}
		key := "alias:" + typ.AliasName
		if seen[key] {
			return unsupportedDictKey("recursive alias " + typ.AliasName)
		}
		seen[key] = true
		base := g.dictKeyType(decl.Target, seen)
		delete(seen, key)
		if !base.Supported {
			return unsupportedDictKey("alias " + typ.AliasName + " target: " + base.Reason)
		}
		base.GoType = exportedName(typ.AliasName)
		return base
	case "StructRef":
		if len(typ.TypeArgs) > 0 {
			return unsupportedDictKey("generic struct " + typ.StructName)
		}
		decl, ok := g.structs[typ.StructName]
		if !ok {
			return unsupportedDictKey("unknown struct " + typ.StructName)
		}
		if len(decl.TypeParams) > 0 {
			return unsupportedDictKey("generic struct " + typ.StructName)
		}
		bits, reason := g.flatStructKeyBits(decl, seen)
		if reason != "" {
			return unsupportedDictKey("struct " + typ.StructName + ": " + reason)
		}
		return dictTypeInfo{GoType: exportedName(typ.StructName), TLBTag: ".", Bits: bits, Supported: true}
	default:
		return unsupportedDictKey("type " + typ.Kind + " is not fixed-width flat bits")
	}
}

func (g *generator) flatStructKeyBits(decl declaration, seen map[string]bool) (int, string) {
	key := "struct:" + decl.Name
	if seen[key] {
		return 0, "recursive struct"
	}
	seen[key] = true
	defer delete(seen, key)

	bits := 0
	if decl.Prefix != nil {
		bits += decl.Prefix.PrefixLen
	}
	for _, fld := range decl.Fields {
		switch fld.Type.Kind {
		case "StructRef":
			return 0, "nested struct field " + fld.Name
		case "cell", "cellOf", "slice", "builder", "string", "arrayOf", "nullable", "mapKV", "varuintN", "varintN", "coins", "addressExt", "addressOpt", "addressAny", "remaining":
			return 0, "field " + fld.Name + " is not fixed-width flat bits"
		}
		info := g.dictKeyType(fld.Type, seen)
		if !info.Supported {
			return 0, "field " + fld.Name + ": " + info.Reason
		}
		bits += info.Bits
	}
	return bits, ""
}

func unsupportedDictKey(reason string) dictTypeInfo {
	return dictTypeInfo{
		Supported: false,
		Reason:    reason,
	}
}
