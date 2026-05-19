package main

import "fmt"

func (g *generator) typeForStack(typ abiType) typeInfo {
	switch typ.Kind {
	case "null", "nullLiteral":
		return typeInfo{
			GoType:    "any",
			Supported: true,
			Kind:      "null",
			StackExpr: func(name string) string { return "nil" },
			Zero:      "nil",
		}
	case "int":
		g.useImport("math/big")
		return typeInfo{
			GoType:    "*big.Int",
			Supported: true,
			Kind:      "int",
			StackExpr: func(name string) string { return name },
			Zero:      "nil",
		}
	case "uintN":
		return g.intType(typ.N, false, false)
	case "intN":
		return g.intType(typ.N, true, false)
	case "bool":
		g.useHelper(helperBool)
		return typeInfo{
			GoType:    "bool",
			Supported: true,
			Kind:      "bool",
			StackExpr: func(name string) string {
				return fmt.Sprintf("boolToStack(%s)", name)
			},
			Zero: "false",
		}
	case "string":
		g.useHelper(helperStackString)
		return typeInfo{
			GoType:    "string",
			Supported: true,
			Kind:      "string",
			StackExpr: func(name string) string {
				return fmt.Sprintf("stackString(%s)", name)
			},
			Zero: `""`,
		}
	case "varuintN", "varintN":
		g.useImport("math/big")
		return typeInfo{
			GoType:    "*big.Int",
			Supported: true,
			Kind:      varIntKind(typ.Kind),
			Bits:      typ.N,
			StackExpr: func(name string) string { return name },
			Zero:      "nil",
		}
	case "coins":
		g.useImport("github.com/xssnick/tonutils-go/tlb")
		return typeInfo{
			GoType:    "tlb.Coins",
			Supported: true,
			Kind:      "coins",
			StackExpr: func(name string) string { return name + ".Nano()" },
			Zero:      "tlb.Coins{}",
		}
	case "address", "addressExt", "addressOpt", "addressAny":
		g.useImport("github.com/xssnick/tonutils-go/address")
		return typeInfo{
			GoType:    "*address.Address",
			Supported: true,
			Kind:      "addr",
			StackExpr: func(name string) string { return name },
			Zero:      "nil",
		}
	case "bitsN":
		g.useHelper(helperStackBits)
		return typeInfo{
			GoType:    "[]byte",
			Supported: true,
			Kind:      "bits",
			Bits:      typ.N,
			StackExpr: func(name string) string {
				return fmt.Sprintf("stackBits(%s, %d)", name, typ.N)
			},
			Zero: "nil",
		}
	case "bytesN":
		g.useHelper(helperStackBits)
		return typeInfo{
			GoType:    "[]byte",
			Supported: true,
			Kind:      "bits",
			Bits:      typ.N * 8,
			StackExpr: func(name string) string {
				return fmt.Sprintf("stackBits(%s, %d)", name, typ.N*8)
			},
			Zero: "nil",
		}
	case "arrayOf":
		return g.stackArrayType(typ)
	case "nullable":
		return g.nullableTypeForStack(typ)
	case "cellOf":
		return g.stackCellOfType(typ)
	case "mapKV":
		return g.stackMapType(typ)
	case "StructRef":
		if len(typ.TypeArgs) > 0 {
			return unsupportedStack("generic struct " + typ.StructName)
		}
		name := exportedName(typ.StructName)
		if name == "" {
			return unsupportedStack("unnamed struct")
		}
		decl, ok := g.structs[typ.StructName]
		if !ok {
			return unsupportedStack("unknown struct " + typ.StructName)
		}
		for _, fld := range decl.Fields {
			if !g.typeForStack(fld.Type).Supported {
				return unsupportedStack("struct " + typ.StructName + " field " + fld.Name + ": " + g.typeForStack(fld.Type).Reason)
			}
		}
		g.useStackStructEncoder(typ.StructName)
		return typeInfo{
			GoType:    "*" + name,
			Supported: true,
			Kind:      "struct",
			StackExpr: func(nameArg string) string {
				return fmt.Sprintf("stack%s(%s)", name, nameArg)
			},
			Zero: "nil",
		}
	case "tensor", "shapedTuple":
		return g.tupleTypeForStack(typ, "")
	case "union":
		return g.unionTypeForStack(typ, "")
	case "cell":
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return typeInfo{
			GoType:    "*cell.Cell",
			Supported: true,
			Kind:      "cell",
			StackExpr: func(name string) string { return name },
			Zero:      "nil",
		}
	case "slice", "remaining":
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return typeInfo{
			GoType:    "*cell.Slice",
			Supported: true,
			Kind:      "slice",
			StackExpr: func(name string) string { return name },
			Zero:      "nil",
		}
	case "builder":
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return typeInfo{
			GoType:    "*cell.Builder",
			Supported: true,
			Kind:      "builder",
			StackExpr: func(name string) string { return name },
			Zero:      "nil",
		}
	case "unknown":
		return typeInfo{
			GoType:    "any",
			Supported: true,
			Kind:      "unknown",
			StackExpr: func(name string) string { return name },
			Zero:      "nil",
		}
	case "lispListOf":
		if typ.Inner == nil {
			return unsupportedStack("lispListOf without inner type")
		}
		inner := g.typeForStack(*typ.Inner)
		if !inner.Supported {
			return unsupportedStack("lispListOf inner type: " + inner.Reason)
		}
		g.useHelper(helperStackList)
		goType := "[]" + inner.GoType
		return typeInfo{
			GoType:    goType,
			Supported: true,
			Kind:      "lispList",
			StackExpr: func(name string) string {
				return fmt.Sprintf("stackLispList(%s, func(v %s) any { return %s })", name, inner.GoType, g.stackValueItemExpr(*typ.Inner, "v"))
			},
			Zero: "nil",
		}
	case "AliasRef":
		return g.aliasTypeForStack(typ)
	case "EnumRef":
		return g.enumTypeForStack(typ)
	default:
		return typeInfo{
			GoType:    "any",
			Supported: false,
			Reason:    "unsupported ABI type kind " + typ.Kind,
			StackExpr: func(name string) string { return name },
			Zero:      "nil",
		}
	}
}

func (g *generator) typeForStackNamed(typ abiType, suggestedName string) typeInfo {
	switch typ.Kind {
	case "tensor", "shapedTuple":
		return g.tupleTypeForStack(typ, suggestedName)
	case "union":
		return g.unionTypeForStack(typ, suggestedName)
	case "nullable":
		return g.nullableTypeForStackNamed(typ, suggestedName)
	default:
		return g.typeForStack(typ)
	}
}
