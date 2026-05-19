package main

import (
	"fmt"
	"strconv"
	"strings"
)

func (g *generator) aliasTypeForTLB(typ abiType, field bool) typeInfo {
	decl, ok := g.aliases[typ.AliasName]
	if !ok {
		return unsupported("unknown alias " + typ.AliasName)
	}
	if len(decl.TypeParams) > 0 || len(typ.TypeArgs) > 0 {
		return unsupported("generic alias " + typ.AliasName)
	}
	if decl.CustomPackUnpack != nil {
		if !customPackUnpackEnabled(decl.CustomPackUnpack) {
			return unsupported(customPackUnpackReason(decl.CustomPackUnpack))
		}
		name := exportedName(typ.AliasName)
		return typeInfo{
			GoType:    name,
			TLBTag:    ".",
			Supported: true,
			Kind:      "custom",
			Zero:      name + "{}",
		}
	}

	base := g.typeForTLBNamed(decl.Target, field, typ.AliasName)
	if !base.Supported {
		return base
	}
	if base.Kind == "map" {
		return base
	}
	base.GoType = exportedName(typ.AliasName)
	return base
}

func (g *generator) aliasTypeForStack(typ abiType) typeInfo {
	decl, ok := g.aliases[typ.AliasName]
	if !ok {
		return unsupportedStack("unknown alias " + typ.AliasName)
	}
	if len(decl.TypeParams) > 0 || len(typ.TypeArgs) > 0 {
		return unsupportedStack("generic alias " + typ.AliasName)
	}

	base := g.typeForStackNamed(decl.Target, typ.AliasName)
	if !base.Supported {
		return base
	}

	aliasName := exportedName(typ.AliasName)
	baseGoType := base.GoType
	baseStackExpr := base.StackExpr
	baseStackErrExpr := base.StackErrExpr
	base.GoType = aliasGoTypeForStackOrResult(aliasName, base)
	if baseStackErrExpr != nil {
		base.StackErrExpr = func(name string) string {
			return baseStackErrExpr(stackAliasBaseCast(baseGoType, name))
		}
	}
	switch base.Kind {
	case "bits":
		base.StackExpr = func(name string) string {
			return fmt.Sprintf("stackBits([]byte(%s), %d)", name, base.Bits)
		}
	case "bool":
		base.StackExpr = func(name string) string {
			return fmt.Sprintf("boolToStack(bool(%s))", name)
		}
	case "string":
		base.StackExpr = func(name string) string {
			return fmt.Sprintf("stackString(string(%s))", name)
		}
	case "int":
		castType := intGoType(base.Bits, strings.HasPrefix(decl.Target.Kind, "int"))
		base.StackExpr = func(name string) string {
			if castType == "*big.Int" {
				return fmt.Sprintf("(*big.Int)(%s)", name)
			}
			return fmt.Sprintf("%s(%s)", castType, name)
		}
	case "varuint", "varint":
		base.StackExpr = func(name string) string {
			return fmt.Sprintf("(*big.Int)(%s)", name)
		}
	case "coins":
		base.StackExpr = func(name string) string {
			return fmt.Sprintf("tlb.Coins(%s).Nano()", name)
		}
	case "addr":
		base.StackExpr = func(name string) string {
			return fmt.Sprintf("(*address.Address)(%s)", name)
		}
	case "cell":
		base.StackExpr = func(name string) string {
			return fmt.Sprintf("(*cell.Cell)(%s)", name)
		}
	case "slice":
		base.StackExpr = func(name string) string {
			return fmt.Sprintf("(*cell.Slice)(%s)", name)
		}
	case "builder":
		base.StackExpr = func(name string) string {
			return fmt.Sprintf("(*cell.Builder)(%s)", name)
		}
	case "lispList", "array":
		base.StackExpr = func(name string) string {
			return baseStackExpr(stackAliasBaseCast(baseGoType, name))
		}
	case "nullable":
		base.StackExpr = func(name string) string {
			return baseStackExpr(stackAliasBaseCast(baseGoType, name))
		}
	case "cellOf", "map", "struct", "tupleStruct", "union", "unknown":
		base.StackExpr = func(name string) string {
			return baseStackExpr(stackAliasBaseCast(baseGoType, name))
		}
	default:
		return unsupportedStack("alias " + typ.AliasName + " stack type " + base.Kind)
	}
	return base
}

func stackAliasBaseCast(goType, name string) string {
	if strings.HasPrefix(goType, "*") {
		return fmt.Sprintf("(%s)(%s)", goType, name)
	}
	return fmt.Sprintf("%s(%s)", goType, name)
}

func (g *generator) aliasTypeForResult(typ abiType) typeInfo {
	decl, ok := g.aliases[typ.AliasName]
	if !ok {
		return unsupported("unknown alias " + typ.AliasName)
	}
	if len(decl.TypeParams) > 0 || len(typ.TypeArgs) > 0 {
		return unsupported("generic alias " + typ.AliasName)
	}

	base := g.typeForResultNamed(decl.Target, typ.AliasName)
	if !base.Supported {
		return base
	}

	aliasName := exportedName(typ.AliasName)
	targetInfo := base
	base.GoType = aliasGoTypeForStackOrResult(aliasName, targetInfo)
	if aliasDecodesDirectlyThroughTarget(decl.Target) {
		if base.Kind == "tuple" {
			base.Kind = "tupleStruct"
		}
		return base
	}
	baseKind := base.Kind
	switch base.Kind {
	case "bits":
		base.ResultDecode = func(target string, index uint, errReturn string) []string {
			return []string{
				fmt.Sprintf("bits%d, err := loadBitsResult(result, %d, %d)", index, index, base.Bits),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
				fmt.Sprintf("%s %s %s", target, assignOp(target), aliasConversionExpr(aliasName, targetInfo, fmt.Sprintf("bits%d", index))),
			}
		}
	case "int":
		baseType := g.resultIntType(base.Bits, strings.HasPrefix(decl.Target.Kind, "int"))
		base.ResultDecode = func(target string, index uint, errReturn string) []string {
			lines := baseType.ResultDecode("decoded"+strconv.Itoa(int(index)), index, errReturn)
			lines = append(lines, fmt.Sprintf("%s %s %s", target, assignOp(target), aliasConversionExpr(aliasName, targetInfo, fmt.Sprintf("decoded%d", index))))
			return lines
		}
	case "varuint", "varint", "coins", "bool", "addr", "cell", "slice", "builder", "nullable", "tupleAny", "lispList", "unknown", "struct", "array", "cellOf", "map", "tuple", "tupleStruct", "union":
		baseType := g.typeForResult(decl.Target)
		base.ResultDecode = func(target string, index uint, errReturn string) []string {
			tmp := "decoded" + strconv.Itoa(int(index)) + "Base"
			lines := baseType.ResultDecode(tmp, index, errReturn)
			lines = append(lines, fmt.Sprintf("%s %s %s", target, assignOp(target), aliasConversionExpr(aliasName, targetInfo, tmp)))
			return lines
		}
	case "string":
		baseType := g.typeForResult(decl.Target)
		base.ResultDecode = func(target string, index uint, errReturn string) []string {
			tmp := "decoded" + strconv.Itoa(int(index))
			lines := baseType.ResultDecode(tmp, index, errReturn)
			lines = append(lines, fmt.Sprintf("%s %s %s", target, assignOp(target), aliasConversionExpr(aliasName, targetInfo, tmp)))
			return lines
		}
	default:
		return unsupported("alias " + typ.AliasName + " result type " + base.Kind)
	}
	if baseKind == "tuple" {
		base.Kind = "tupleStruct"
	}
	return base
}

func aliasGoTypeForStackOrResult(aliasName string, base typeInfo) string {
	switch base.Kind {
	case "struct", "tupleStruct", "union", "tuple":
		if strings.HasPrefix(base.GoType, "*") {
			return "*" + aliasName
		}
		return aliasName
	default:
		return aliasName
	}
}

func aliasConversionExpr(aliasName string, base typeInfo, value string) string {
	goType := aliasGoTypeForStackOrResult(aliasName, base)
	if strings.HasPrefix(goType, "*") {
		return fmt.Sprintf("(%s)(%s)", goType, value)
	}
	return fmt.Sprintf("%s(%s)", goType, value)
}

func aliasDecodesDirectlyThroughTarget(typ abiType) bool {
	switch typ.Kind {
	case "StructRef", "tensor", "shapedTuple", "union":
		return true
	default:
		return false
	}
}
