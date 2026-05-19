package main

import (
	"fmt"
	"strconv"
)

func (g *generator) enumTypeForTLB(typ abiType) typeInfo {
	decl, ok := g.enums[typ.EnumName]
	if !ok || decl.EncodedAs == nil {
		return unsupported("unknown enum " + typ.EnumName)
	}
	if decl.CustomPackUnpack != nil {
		if !customPackUnpackEnabled(decl.CustomPackUnpack) {
			return customPackUnpackUnsupported("enum "+typ.EnumName, decl.CustomPackUnpack)
		}
		name := exportedName(typ.EnumName)
		return typeInfo{
			GoType:    name,
			TLBTag:    ".",
			Supported: true,
			Kind:      "custom",
			Zero:      name + "{}",
		}
	}

	base := g.typeForTLB(*decl.EncodedAs, false)
	if !base.Supported || base.Kind == "bits" {
		return unsupported("unsupported enum encoding " + decl.EncodedAs.Kind)
	}
	base.GoType = exportedName(typ.EnumName)
	return base
}

func (g *generator) enumTypeForStack(typ abiType) typeInfo {
	decl, ok := g.enums[typ.EnumName]
	if !ok || decl.EncodedAs == nil {
		return unsupportedStack("unknown enum " + typ.EnumName)
	}

	base := g.typeForStack(*decl.EncodedAs)
	if !base.Supported || base.Kind == "bits" {
		return unsupportedStack("unsupported enum encoding " + decl.EncodedAs.Kind)
	}

	enumName := exportedName(typ.EnumName)
	base.GoType = enumName
	castType := intGoType(base.Bits, decl.EncodedAs.Kind == "intN")
	base.StackExpr = func(name string) string {
		if castType == "*big.Int" {
			return name
		}
		return fmt.Sprintf("%s(%s)", castType, name)
	}
	return base
}

func (g *generator) enumTypeForResult(typ abiType) typeInfo {
	decl, ok := g.enums[typ.EnumName]
	if !ok || decl.EncodedAs == nil {
		return unsupported("unknown enum " + typ.EnumName)
	}

	base := g.resultIntType(decl.EncodedAs.N, decl.EncodedAs.Kind == "intN")
	if !base.Supported || base.Kind == "bits" {
		return unsupported("unsupported enum encoding " + decl.EncodedAs.Kind)
	}

	enumName := exportedName(typ.EnumName)
	base.GoType = enumName
	base.ResultDecode = func(target string, index uint, errReturn string) []string {
		lines := g.resultIntType(decl.EncodedAs.N, decl.EncodedAs.Kind == "intN").ResultDecode("decoded"+strconv.Itoa(int(index)), index, errReturn)
		lines = append(lines, fmt.Sprintf("%s %s %s(decoded%d)", target, assignOp(target), enumName, index))
		return lines
	}
	return base
}
