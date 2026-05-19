package main

import (
	"fmt"
	"strings"
)

func (g *generator) typeForResult(typ abiType) typeInfo {
	switch typ.Kind {
	case "void":
		return typeInfo{
			GoType:    "struct{}",
			Supported: true,
			Kind:      "void",
			Zero:      "struct{}{}",
		}
	case "null", "nullLiteral":
		return typeInfo{
			GoType:    "any",
			Supported: true,
			Kind:      "null",
			ResultDecode: func(target string, index uint, errReturn string) []string {
				return []string{fmt.Sprintf("%s %s nil", target, assignOp(target))}
			},
			Zero: "nil",
		}
	case "tensor", "shapedTuple":
		items := typ.Items
		for i, item := range items {
			info := g.typeForResult(item)
			if !info.Supported {
				return unsupported(fmt.Sprintf("tuple item %s: %s", tupleFieldName(i), info.Reason))
			}
		}
		tupleInfo := g.tupleTypeForStack(typ, "")
		if !tupleInfo.Supported {
			return tupleInfo
		}
		return typeInfo{
			GoType:       tupleInfo.GoType,
			Supported:    true,
			Kind:         "tuple",
			ResultDecode: tupleInfo.ResultDecode,
			Zero:         "nil",
		}
	case "int":
		g.useImport("math/big")
		return typeInfo{
			GoType:    "*big.Int",
			Supported: true,
			Kind:      "int",
			ResultDecode: func(target string, index uint, errReturn string) []string {
				return []string{
					fmt.Sprintf("%s, err %s result.Int(%d)", target, assignOp(target), index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
				}
			},
			Zero: "nil",
		}
	case "uintN":
		return g.resultIntType(typ.N, false)
	case "intN":
		return g.resultIntType(typ.N, true)
	case "bool":
		return typeInfo{
			GoType:    "bool",
			Supported: true,
			Kind:      "bool",
			ResultDecode: func(target string, index uint, errReturn string) []string {
				return []string{
					fmt.Sprintf("raw%d, err := result.Int(%d)", index, index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
					fmt.Sprintf("%s %s raw%d.Sign() != 0", target, assignOp(target), index),
				}
			},
			Zero: "false",
		}
	case "string":
		return typeInfo{
			GoType:    "string",
			Supported: true,
			Kind:      "string",
			ResultDecode: func(target string, index uint, errReturn string) []string {
				g.useHelper(helperLoadString)
				return []string{
					fmt.Sprintf("%s, err %s loadStringResult(result, %d)", target, assignOp(target), index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
				}
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
			ResultDecode: func(target string, index uint, errReturn string) []string {
				return []string{
					fmt.Sprintf("%s, err %s result.Int(%d)", target, assignOp(target), index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
				}
			},
			Zero: "nil",
		}
	case "coins":
		g.useImport("github.com/xssnick/tonutils-go/tlb")
		return typeInfo{
			GoType:    "tlb.Coins",
			Supported: true,
			Kind:      "coins",
			ResultDecode: func(target string, index uint, errReturn string) []string {
				return []string{
					fmt.Sprintf("raw%d, err := result.Int(%d)", index, index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
					fmt.Sprintf("%s %s tlb.FromNanoTON(raw%d)", target, assignOp(target), index),
				}
			},
			Zero: "tlb.Coins{}",
		}
	case "address", "addressExt", "addressOpt", "addressAny":
		g.useImport("github.com/xssnick/tonutils-go/address")
		return typeInfo{
			GoType:    "*address.Address",
			Supported: true,
			Kind:      "addr",
			ResultDecode: func(target string, index uint, errReturn string) []string {
				g.useHelper(helperLoadAddr)
				return []string{
					fmt.Sprintf("%s, err %s loadAddressResult(result, %d)", target, assignOp(target), index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
				}
			},
			Zero: "nil",
		}
	case "bitsN":
		return typeInfo{
			GoType:    "[]byte",
			Supported: true,
			Kind:      "bits",
			Bits:      typ.N,
			ResultDecode: func(target string, index uint, errReturn string) []string {
				g.useHelper(helperLoadBits)
				return []string{
					fmt.Sprintf("%s, err %s loadBitsResult(result, %d, %d)", target, assignOp(target), index, typ.N),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
				}
			},
			Zero: "nil",
		}
	case "bytesN":
		return typeInfo{
			GoType:    "[]byte",
			Supported: true,
			Kind:      "bits",
			Bits:      typ.N * 8,
			ResultDecode: func(target string, index uint, errReturn string) []string {
				g.useHelper(helperLoadBits)
				return []string{
					fmt.Sprintf("%s, err %s loadBitsResult(result, %d, %d)", target, assignOp(target), index, typ.N*8),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
				}
			},
			Zero: "nil",
		}
	case "arrayOf":
		return g.resultArrayType(typ)
	case "lispListOf":
		return g.lispListTypeForResult(typ)
	case "nullable":
		return g.nullableTypeForResult(typ)
	case "cellOf":
		return g.resultCellOfType(typ)
	case "mapKV":
		return g.resultMapType(typ)
	case "cell":
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return typeInfo{
			GoType:    "*cell.Cell",
			Supported: true,
			Kind:      "cell",
			ResultDecode: func(target string, index uint, errReturn string) []string {
				return []string{
					fmt.Sprintf("%s, err %s result.Cell(%d)", target, assignOp(target), index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
				}
			},
			Zero: "nil",
		}
	case "slice", "remaining":
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return typeInfo{
			GoType:    "*cell.Slice",
			Supported: true,
			Kind:      "slice",
			ResultDecode: func(target string, index uint, errReturn string) []string {
				return []string{
					fmt.Sprintf("%s, err %s result.Slice(%d)", target, assignOp(target), index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
				}
			},
			Zero: "nil",
		}
	case "builder":
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return typeInfo{
			GoType:    "*cell.Builder",
			Supported: true,
			Kind:      "builder",
			ResultDecode: func(target string, index uint, errReturn string) []string {
				return []string{
					fmt.Sprintf("%s, err %s result.Builder(%d)", target, assignOp(target), index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
				}
			},
			Zero: "nil",
		}
	case "AliasRef":
		return g.aliasTypeForResult(typ)
	case "EnumRef":
		return g.enumTypeForResult(typ)
	case "StructRef":
		return g.structTypeForResult(typ)
	case "union":
		return g.unionTypeForResult(typ, "")
	case "unknown":
		return typeInfo{
			GoType:    "any",
			Supported: true,
			Kind:      "unknown",
			ResultDecode: func(target string, index uint, errReturn string) []string {
				return []string{
					fmt.Sprintf("if uint(len(result.AsTuple())) <= %d {", index),
					fmt.Sprintf("\treturn %s, ton.ErrResultIndexOutOfRange", errReturn),
					"}",
					fmt.Sprintf("%s %s result.AsTuple()[%d]", target, assignOp(target), index),
				}
			},
			Zero: "nil",
		}
	default:
		return unsupported("unsupported ABI type kind " + typ.Kind)
	}
}

func (g *generator) typeForResultNamed(typ abiType, suggestedName string) typeInfo {
	switch typ.Kind {
	case "tensor", "shapedTuple":
		items := typ.Items
		for i, item := range items {
			info := g.typeForResult(item)
			if !info.Supported {
				return unsupported(fmt.Sprintf("tuple item %s: %s", tupleFieldName(i), info.Reason))
			}
		}
		tupleInfo := g.tupleTypeForStack(typ, suggestedName)
		if !tupleInfo.Supported {
			return tupleInfo
		}
		return typeInfo{
			GoType:       tupleInfo.GoType,
			Supported:    true,
			Kind:         "tuple",
			ResultDecode: tupleInfo.ResultDecode,
			Zero:         "nil",
		}
	case "union":
		return g.unionTypeForResult(typ, suggestedName)
	case "nullable":
		return g.nullableTypeForResultNamed(typ, suggestedName)
	default:
		return g.typeForResult(typ)
	}
}

func (g *generator) nullableTypeForResult(typ abiType) typeInfo {
	return g.nullableTypeForResultNamed(typ, "")
}

func (g *generator) nullableTypeForResultNamed(typ abiType, suggestedName string) typeInfo {
	if typ.Inner == nil {
		return unsupported("nullable without inner type")
	}

	inner := g.typeForResultNamed(*typ.Inner, suggestedName)
	if !inner.Supported {
		return inner
	}

	goType := inner.GoType
	if !inner.Interface {
		goType = nullableGoType(inner.GoType)
	}
	return typeInfo{
		GoType:    goType,
		Supported: true,
		Kind:      "nullable",
		Interface: inner.Interface,
		ResultDecode: func(target string, index uint, errReturn string) []string {
			nilVar := fmt.Sprintf("isNil%d", index)
			assign := assignOp(target)
			lines := []string{}
			if assign == ":=" {
				lines = append(lines, fmt.Sprintf("var %s %s", target, goType))
				assign = "="
			}
			lines = append(lines,
				fmt.Sprintf("%s, err := result.IsNil(%d)", nilVar, index),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
				fmt.Sprintf("if %s {", nilVar),
				fmt.Sprintf("\t%s %s nil", target, assign),
				"} else {",
			)
			tmp := fmt.Sprintf("decoded%d", index)
			for _, line := range inner.ResultDecode(tmp, index, errReturn) {
				lines = append(lines, "\t"+line)
			}
			if inner.Interface || isDirectlyNullableResultType(inner.GoType) {
				lines = append(lines, fmt.Sprintf("\t%s %s %s", target, assign, tmp))
			} else {
				lines = append(lines, fmt.Sprintf("\t%s %s &%s", target, assign, tmp))
			}
			lines = append(lines, "}")
			return lines
		},
		Zero: "nil",
	}
}

func (g *generator) lispListTypeForResult(typ abiType) typeInfo {
	if typ.Inner == nil {
		return unsupported("lispListOf without inner type")
	}

	inner := g.typeForResult(*typ.Inner)
	if !inner.Supported {
		return unsupported("lispListOf inner type: " + inner.Reason)
	}

	goType := "[]" + inner.GoType
	return typeInfo{
		GoType:    goType,
		Supported: true,
		Kind:      "lispList",
		ResultDecode: func(target string, index uint, errReturn string) []string {
			raw := fmt.Sprintf("raw%d", index)
			lines := []string{
				fmt.Sprintf("if uint(len(result.AsTuple())) <= %d {", index),
				fmt.Sprintf("\treturn %s, ton.ErrResultIndexOutOfRange", errReturn),
				"}",
				fmt.Sprintf("%s := result.AsTuple()[%d]", raw, index),
			}
			lines = append(lines, g.lispListDecodeLines(*typ.Inner, target, raw, errReturn, fmt.Sprintf("decoded%d", index))...)
			return lines
		},
		Zero: "nil",
	}
}

func (g *generator) structTypeForResult(typ abiType) typeInfo {
	if len(typ.TypeArgs) > 0 {
		return unsupported("generic struct " + typ.StructName)
	}
	decl, ok := g.structs[typ.StructName]
	if !ok {
		return unsupported("unknown struct " + typ.StructName)
	}
	for _, fld := range decl.Fields {
		if !g.typeForResult(fld.Type).Supported {
			return unsupported("struct " + typ.StructName + " field " + fld.Name + ": " + g.typeForResult(fld.Type).Reason)
		}
	}

	name := exportedName(typ.StructName)
	return typeInfo{
		GoType:    "*" + name,
		Supported: true,
		Kind:      "struct",
		ResultDecode: func(target string, index uint, errReturn string) []string {
			g.useStackResultDecoder(typ.StructName)
			tupleVar := fmt.Sprintf("tuple%d", index)
			return []string{
				fmt.Sprintf("%s, err := result.Tuple(%d)", tupleVar, index),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
				fmt.Sprintf("%s, err %s decode%sResult(%s)", target, assignOp(target), name, tupleVar),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
			}
		},
		Zero: "nil",
	}
}

func (g *generator) resultIntType(bits int, signed bool) typeInfo {
	info := g.intType(bits, signed, false)
	info.ResultDecode = func(target string, index uint, errReturn string) []string {
		if info.GoType == "*big.Int" {
			return []string{
				fmt.Sprintf("%s, err %s result.Int(%d)", target, assignOp(target), index),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
			}
		}

		read := "Int64"
		if !signed {
			read = "Uint64"
		}
		return []string{
			fmt.Sprintf("raw%d, err := result.Int(%d)", index, index),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
			fmt.Sprintf("%s %s %s(raw%d.%s())", target, assignOp(target), info.GoType, index, read),
		}
	}
	return info
}

func isDirectlyNullableResultType(goType string) bool {
	return strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") || goType == "any"
}
