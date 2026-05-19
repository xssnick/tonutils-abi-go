package main

import (
	"fmt"
	"strings"
)

func customPackUnpackEnabled(custom *customPackUnpack) bool {
	return custom != nil && (custom.PackToBuilder || custom.UnpackFromSlice)
}

func customPackUnpackReason(custom *customPackUnpack) string {
	if custom == nil {
		return ""
	}
	if custom.PackToBuilder && !custom.UnpackFromSlice {
		return "custom pack/unpack has pack_to_builder without unpack_from_slice"
	}
	if !custom.PackToBuilder && custom.UnpackFromSlice {
		return "custom pack/unpack has unpack_from_slice without pack_to_builder"
	}
	return "custom pack/unpack is incomplete"
}

func customPackUnpackUnsupported(prefix string, custom *customPackUnpack) typeInfo {
	reason := customPackUnpackReason(custom)
	if prefix != "" {
		reason = prefix + ": " + reason
	}
	return unsupported(reason)
}

func (g *generator) customValueTypeForTLB(typ abiType, suggestedName string) typeInfo {
	switch typ.Kind {
	case "int":
		g.useImport("math/big")
		return typeInfo{GoType: "*big.Int", Supported: true, Kind: "int", Zero: "nil"}
	case "uintN":
		info := g.intType(typ.N, false, true)
		info.TLBTag = ""
		return info
	case "intN":
		info := g.intType(typ.N, true, true)
		info.TLBTag = ""
		return info
	case "bool":
		return typeInfo{GoType: "bool", Supported: true, Kind: "bool", Zero: "false"}
	case "string":
		return typeInfo{GoType: "string", Supported: true, Kind: "string", Zero: `""`}
	case "varuintN", "varintN":
		g.useImport("math/big")
		return typeInfo{GoType: "*big.Int", Supported: true, Kind: varIntKind(typ.Kind), Bits: typ.N, Zero: "nil"}
	case "coins":
		g.useImport("github.com/xssnick/tonutils-go/tlb")
		return typeInfo{GoType: "tlb.Coins", Supported: true, Kind: "coins", Zero: "tlb.Coins{}"}
	case "address", "addressExt", "addressOpt", "addressAny":
		g.useImport("github.com/xssnick/tonutils-go/address")
		return typeInfo{GoType: "*address.Address", Supported: true, Kind: "addr", Zero: "nil"}
	case "bitsN", "bytesN":
		return typeInfo{GoType: "[]byte", Supported: true, Kind: "bits", Bits: typ.N, Zero: "nil"}
	case "cell":
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return typeInfo{GoType: "*cell.Cell", Supported: true, Kind: "cell", Zero: "nil"}
	case "slice", "remaining":
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return typeInfo{GoType: "*cell.Slice", Supported: true, Kind: "slice", Zero: "nil"}
	case "builder":
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return typeInfo{GoType: "*cell.Builder", Supported: true, Kind: "builder", Zero: "nil"}
	case "arrayOf":
		if typ.Inner == nil {
			return unsupported("arrayOf without inner type")
		}
		inner := g.customValueTypeForTLB(*typ.Inner, exportedName(suggestedName)+"Item")
		if !inner.Supported {
			return unsupported("arrayOf inner type: " + inner.Reason)
		}
		return typeInfo{GoType: "[]" + inner.GoType, Supported: true, Kind: "array", Zero: "nil"}
	case "nullable":
		if typ.Inner == nil {
			return unsupported("nullable without inner type")
		}
		inner := g.customValueTypeForTLB(*typ.Inner, suggestedName)
		if !inner.Supported {
			return inner
		}
		if !inner.Interface {
			inner.GoType = nullableGoType(inner.GoType)
		}
		inner.Kind = "nullable"
		inner.Zero = "nil"
		return inner
	case "cellOf":
		info := g.cellOfTypeForTLB(typ)
		info.TLBTag = ""
		return info
	case "mapKV":
		info := g.mapTypeForTLB(typ, suggestedName)
		info.TLBTag = ""
		return info
	case "tensor", "shapedTuple":
		info := g.tensorTypeForTLB(typ, suggestedName)
		info.TLBTag = ""
		return info
	case "union":
		info := g.unionTypeForTLB(typ, suggestedName)
		info.TLBTag = ""
		return info
	case "StructRef":
		if len(typ.TypeArgs) > 0 {
			return unsupported("generic struct " + typ.StructName)
		}
		name := exportedName(typ.StructName)
		if name == "" {
			return unsupported("unnamed struct")
		}
		return typeInfo{GoType: name, Supported: true, Kind: "struct", Zero: name + "{}"}
	case "AliasRef":
		name := exportedName(typ.AliasName)
		if name == "" {
			return unsupported("unnamed alias")
		}
		return typeInfo{GoType: name, Supported: true, Kind: "alias", Zero: name + "{}"}
	case "EnumRef":
		name := exportedName(typ.EnumName)
		if name == "" {
			return unsupported("unnamed enum")
		}
		return typeInfo{GoType: name, Supported: true, Kind: "enum", Zero: name + "{}"}
	case "void":
		return typeInfo{GoType: "struct{}", Supported: true, Kind: "void", Zero: "struct{}{}"}
	default:
		return unsupported("unsupported ABI type kind " + typ.Kind)
	}
}

func customZeroValue(info typeInfo) string {
	if info.Zero != "" {
		return info.Zero
	}
	return zeroValue(info.GoType)
}

func customEnumValueExpr(goType, value string) string {
	if goType == "*big.Int" {
		return fmt.Sprintf("tugenMustBigInt(%q)", value)
	}
	if goType == "struct{}" {
		return "struct{}{}"
	}
	if strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") || goType == "any" {
		return customZeroValue(typeInfo{GoType: goType})
	}
	return fmt.Sprintf("%s(%s)", goType, value)
}
