package main

import (
  "bytes"
  "fmt"
  "strings"
)

func (g *generator) typeForTLB(typ abiType, field bool) typeInfo {
  return g.typeForTLBNamed(typ, field, "")
}

func (g *generator) typeForTLBNamed(typ abiType, field bool, suggestedName string) typeInfo {
  switch typ.Kind {
  case "uintN":
    return g.intType(typ.N, false, true)
  case "intN":
    return g.intType(typ.N, true, true)
  case "bool":
    return typeInfo{
      GoType:    "bool",
      TLBTag:    "bool",
      Supported: true,
      Kind:      "bool",
      Zero:      "false",
    }
  case "void":
    return typeInfo{
      GoType:    "struct{}",
      TLBTag:    "-",
      Supported: true,
      Kind:      "void",
      Zero:      "struct{}{}",
    }
  case "string":
    return typeInfo{
      GoType:    "string",
      TLBTag:    "string",
      Supported: true,
      Kind:      "string",
      Zero:      `""`,
    }
  case "varuintN":
    g.useImport("math/big")
    return typeInfo{
      GoType:    "*big.Int",
      TLBTag:    fmt.Sprintf("var uint %d", typ.N),
      Supported: true,
      Kind:      "varuint",
      Bits:      typ.N,
      Zero:      "nil",
    }
  case "varintN":
    g.useImport("math/big")
    return typeInfo{
      GoType:    "*big.Int",
      TLBTag:    fmt.Sprintf("var int %d", typ.N),
      Supported: true,
      Kind:      "varint",
      Bits:      typ.N,
      Zero:      "nil",
    }
  case "coins":
    g.useImport("github.com/xssnick/tonutils-go/tlb")
    return typeInfo{
      GoType:    "tlb.Coins",
      TLBTag:    ".",
      Supported: true,
      Kind:      "coins",
      Zero:      "tlb.Coins{}",
    }
  case "address":
    g.useImport("github.com/xssnick/tonutils-go/address")
    return typeInfo{
      GoType:    "*address.Address",
      TLBTag:    "addr std required",
      Supported: true,
      Kind:      "addr",
      Zero:      "nil",
    }
  case "addressExt":
    g.useImport("github.com/xssnick/tonutils-go/address")
    return typeInfo{
      GoType:    "*address.Address",
      TLBTag:    "addr ext required",
      Supported: true,
      Kind:      "addr",
      Zero:      "nil",
    }
  case "addressOpt":
    g.useImport("github.com/xssnick/tonutils-go/address")
    return typeInfo{
      GoType:    "*address.Address",
      TLBTag:    "addr std",
      Supported: true,
      Kind:      "addr",
      Zero:      "nil",
    }
  case "addressAny":
    g.useImport("github.com/xssnick/tonutils-go/address")
    return typeInfo{
      GoType:    "*address.Address",
      TLBTag:    "addr",
      Supported: true,
      Kind:      "addr",
      Zero:      "nil",
    }
  case "bitsN":
    return typeInfo{
      GoType:    "[]byte",
      TLBTag:    fmt.Sprintf("bits %d", typ.N),
      Supported: true,
      Kind:      "bits",
      Bits:      typ.N,
      Zero:      "nil",
    }
  case "bytesN":
    return typeInfo{
      GoType:    "[]byte",
      TLBTag:    fmt.Sprintf("bits %d", typ.N*8),
      Supported: true,
      Kind:      "bits",
      Bits:      typ.N * 8,
      Zero:      "nil",
    }
  case "arrayOf":
    return g.arrayTypeForTLB(typ)
  case "lispListOf":
    return g.lispListTypeForTLB(typ, suggestedName)
  case "nullable":
    return g.nullableTypeForTLB(typ)
  case "mapKV":
    return g.mapTypeForTLB(typ, suggestedName)
  case "tensor", "shapedTuple":
    return g.tensorTypeForTLB(typ, suggestedName)
  case "union":
    return g.unionTypeForTLB(typ, suggestedName)
  case "cellOf":
    return g.cellOfTypeForTLB(typ)
  case "cell":
    g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
    return typeInfo{
      GoType:    "*cell.Cell",
      TLBTag:    "^",
      Supported: true,
      Kind:      "cell",
      Zero:      "nil",
    }
  case "remaining":
    g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
    return typeInfo{
      GoType:    "*cell.Cell",
      TLBTag:    ".",
      Supported: true,
      Kind:      "remaining",
      Zero:      "nil",
    }
  case "StructRef":
    if len(typ.TypeArgs) > 0 {
      return unsupported("generic struct " + typ.StructName)
    }
    return typeInfo{
      GoType:    exportedName(typ.StructName),
      TLBTag:    ".",
      Supported: true,
      Kind:      "struct",
      Zero:      exportedName(typ.StructName) + "{}",
    }
  case "AliasRef":
    return g.aliasTypeForTLB(typ, field)
  case "EnumRef":
    return g.enumTypeForTLB(typ)
  default:
    return unsupported("unsupported ABI type kind " + typ.Kind)
  }
}

func (g *generator) cellOfTypeForTLB(typ abiType) typeInfo {
  if typ.Inner == nil {
    return unsupported("cellOf without inner type")
  }

  inner := g.typeForTLB(*typ.Inner, true)
  if !inner.Supported {
    if typ.Inner.Kind == "slice" {
      g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
      return typeInfo{
        GoType:    "*cell.Cell",
        TLBTag:    "^",
        Supported: true,
        Kind:      "cellOf",
        Zero:      "nil",
      }
    }

    return inner
  }
  if inner.TLBTag == "" {
    return unsupported("cellOf inner type without TLB tag")
  }
  if cellOfNeedsPayloadWrapper(inner) {
    name := g.cellOfPayloadWrapperType(*typ.Inner, inner)
    return typeInfo{
      GoType:    name,
      TLBTag:    "^",
      Supported: true,
      Kind:      "cellOf",
      Zero:      name + "{}",
    }
  }

  if inner.Interface {
    inner.TLBTag = "^ " + inner.TLBTag
  } else {
    inner.TLBTag = "^"
  }
  inner.Kind = "cellOf"
  return inner
}

func cellOfNeedsPayloadWrapper(inner typeInfo) bool {
  if inner.Interface {
    return false
  }
  return inner.TLBTag != "."
}

func (g *generator) cellOfPayloadWrapperType(typ abiType, inner typeInfo) string {
  base := cellOfPayloadWrapperBase(typ)
  name := g.generatedName(base, "", typ)
  if g.generatedTypeSet[name] {
    return name
  }

  var b bytes.Buffer
  fmt.Fprintf(&b, "type %s struct {\n", name)
  fmt.Fprintf(&b, "\tValue %s `tlb:%q`\n", inner.GoType, inner.TLBTag)
  b.WriteString("}\n\n")
  g.addGeneratedType(name, b.String())
  return name
}

func cellOfPayloadWrapperBase(typ abiType) string {
  base := cellOfPayloadTypeNamePart(typ) + "Cell"
  if len(exportedName(base)) > 96 {
    base = "Cell" + shortTypeHash(mapTypeSignature(typ))
  }
  return base
}

func cellOfPayloadTypeNamePart(typ abiType) string {
  switch typ.Kind {
  case "address":
    return "Address"
  case "addressExt":
    return "ExternalAddress"
  case "addressOpt":
    return "MaybeAddress"
  case "addressAny":
    return "AnyAddress"
  case "nullable":
    if typ.Inner == nil {
      return "MaybeAny"
    }
    return "Maybe" + cellOfPayloadTypeNamePart(*typ.Inner)
  case "cellOf":
    if typ.Inner == nil {
      return "Cell"
    }
    return cellOfPayloadTypeNamePart(*typ.Inner) + "Cell"
  default:
    part := exportedName(genericTypeNamePart(typ))
    if part == "" {
      return "Value"
    }
    return part
  }
}

func (g *generator) arrayTypeForTLB(typ abiType) typeInfo {
  if typ.Inner == nil {
    return unsupported("arrayOf without inner type")
  }

  inner := g.typeForTLB(*typ.Inner, true)
  if !inner.Supported {
    return inner
  }
  if inner.TLBTag == "" {
    return unsupported("arrayOf inner type without TLB tag")
  }

  return typeInfo{
    GoType:    "[]" + inner.GoType,
    TLBTag:    "array " + inner.TLBTag,
    Supported: true,
    Kind:      "array",
    Zero:      "nil",
  }
}

func (g *generator) nullableTypeForTLB(typ abiType) typeInfo {
  if typ.Inner == nil {
    return unsupported("nullable without inner type")
  }

  inner := g.typeForTLB(*typ.Inner, true)
  if !inner.Supported {
    return inner
  }
  if inner.TLBTag == "" {
    return unsupported("nullable inner type without TLB tag")
  }
  if strings.HasPrefix(inner.TLBTag, "maybe ") {
    return unsupported("nested nullable")
  }

  if !inner.Interface {
    inner.GoType = nullableGoType(inner.GoType)
  }
  inner.TLBTag = "maybe " + inner.TLBTag
  inner.Kind = "nullable"
  inner.Zero = "nil"
  return inner
}
