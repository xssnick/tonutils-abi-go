package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

type constantValueInfo struct {
	GoType    string
	Expr      string
	Supported bool
	Reason    string
}

func (g *generator) writeConstants(dst *bytes.Buffer) {
	for _, constant := range g.abi.Constants {
		name := constantFunctionName(constant.Name)
		if g.names != nil {
			name = g.names.uniquePackage(name, "Const")
		}
		info := g.constantValueInfo(constant.Value)
		if !info.Supported {
			g.writeTODO(dst, "", "constant %s is not generated yet: %s.", exportedName(constant.Name), info.Reason)
			dst.WriteString("\n")
			continue
		}
		fmt.Fprintf(dst, "func %s() %s {\n", name, info.GoType)
		fmt.Fprintf(dst, "\treturn %s\n", info.Expr)
		dst.WriteString("}\n\n")
	}
}

func constantFunctionName(name string) string {
	name = exportedName(name)
	if name == "" {
		return "Const"
	}
	return "Const" + name
}

func (g *generator) constantValueInfo(expr abiConstExpression) constantValueInfo {
	switch expr.Kind {
	case "int":
		value, ok := constIntString(expr)
		if !ok {
			return unsupportedConstant("int value is missing")
		}
		return constantValueInfo{
			GoType:    "*big.Int",
			Expr:      g.constantBigIntExpr(value),
			Supported: true,
		}
	case "bool":
		value, ok := expr.Value.(bool)
		if !ok {
			return unsupportedConstant("bool value is missing")
		}
		if value {
			return constantValueInfo{GoType: "bool", Expr: "true", Supported: true}
		}
		return constantValueInfo{GoType: "bool", Expr: "false", Supported: true}
	case "slice":
		value, ok := constantHexBytesLiteral(expr.Hex)
		if !ok {
			return unsupportedConstant("slice hex literal is invalid")
		}
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return constantValueInfo{
			GoType:    "*cell.Slice",
			Expr:      fmt.Sprintf("cell.BeginCell().MustStoreSlice(%s, %d).ToSlice()", value, len(expr.Hex)*4),
			Supported: true,
		}
	case "string":
		return constantValueInfo{
			GoType:    "string",
			Expr:      strconv.Quote(expr.String),
			Supported: true,
		}
	case "address":
		if expr.Address == "" {
			return unsupportedConstant("address value is missing")
		}
		g.useImport("github.com/xssnick/tonutils-go/address")
		return constantValueInfo{
			GoType:    "*address.Address",
			Expr:      fmt.Sprintf("address.MustParseAddr(%q)", expr.Address),
			Supported: true,
		}
	case "castTo":
		return g.constantCastToValueInfo(expr)
	case "null":
		return constantValueInfo{GoType: "any", Expr: "nil", Supported: true}
	default:
		return unsupportedConstant("unsupported constant expression kind " + expr.Kind)
	}
}

func (g *generator) constantCastToValueInfo(expr abiConstExpression) constantValueInfo {
	if expr.Inner == nil {
		return unsupportedConstant("castTo without inner expression")
	}
	switch expr.CastTo.Kind {
	case "coins":
		if expr.Inner.Kind != "int" {
			return unsupportedConstant("coins cast from " + expr.Inner.Kind)
		}
		value, ok := constIntString(*expr.Inner)
		if !ok {
			return unsupportedConstant("coins value is missing")
		}
		g.useImport("github.com/xssnick/tonutils-go/tlb")
		return constantValueInfo{
			GoType:    "tlb.Coins",
			Expr:      fmt.Sprintf("tlb.FromNanoTON(%s)", g.constantBigIntExpr(value)),
			Supported: true,
		}
	case "int", "varuintN", "varintN":
		if expr.Inner.Kind != "int" {
			return unsupportedConstant(expr.CastTo.Kind + " cast from " + expr.Inner.Kind)
		}
		value, ok := constIntString(*expr.Inner)
		if !ok {
			return unsupportedConstant("int value is missing")
		}
		return constantValueInfo{
			GoType:    "*big.Int",
			Expr:      g.constantBigIntExpr(value),
			Supported: true,
		}
	case "uintN", "intN":
		if expr.Inner.Kind != "int" {
			return unsupportedConstant(expr.CastTo.Kind + " cast from " + expr.Inner.Kind)
		}
		value, ok := constIntString(*expr.Inner)
		if !ok {
			return unsupportedConstant("int value is missing")
		}
		goType := intGoType(expr.CastTo.N, expr.CastTo.Kind == "intN")
		if goType == "*big.Int" {
			return constantValueInfo{
				GoType:    "*big.Int",
				Expr:      g.constantBigIntExpr(value),
				Supported: true,
			}
		}
		return constantValueInfo{
			GoType:    goType,
			Expr:      fmt.Sprintf("%s(%s)", goType, value),
			Supported: true,
		}
	default:
		return unsupportedConstant("unsupported cast target " + expr.CastTo.Kind)
	}
}

func (g *generator) constantBigIntExpr(value string) string {
	g.useImport("math/big")
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return fmt.Sprintf("big.NewInt(%s)", value)
	}
	g.useHelper(helperBigIntLiteral)
	return fmt.Sprintf("tugenMustBigInt(%q)", value)
}

func constIntString(expr abiConstExpression) (string, bool) {
	switch value := expr.Value.(type) {
	case string:
		if value == "" {
			return "", false
		}
		return value, true
	case float64:
		return strconv.FormatInt(int64(value), 10), true
	case int:
		return strconv.Itoa(value), true
	case int64:
		return strconv.FormatInt(value, 10), true
	default:
		return "", false
	}
}

func constantHexBytesLiteral(value string) (string, bool) {
	if len(value)%2 != 0 {
		return "", false
	}
	raw, err := hex.DecodeString(value)
	if err != nil {
		return "", false
	}
	if len(raw) == 0 {
		return "[]byte{}", true
	}
	var b strings.Builder
	b.WriteString("[]byte{")
	for i, item := range raw {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "0x%02x", item)
	}
	b.WriteString("}")
	return b.String(), true
}

func unsupportedConstant(reason string) constantValueInfo {
	return constantValueInfo{
		Supported: false,
		Reason:    reason,
	}
}
