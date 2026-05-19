package main

import "fmt"

func (g *generator) stackWidth(typ abiType) (int, bool, string) {
	switch typ.Kind {
	case "void":
		return 0, true, ""
	case "tensor":
		total := 0
		for i, item := range typ.Items {
			width, ok, reason := g.stackWidth(item)
			if !ok {
				return 0, false, fmt.Sprintf("tensor item %d: %s", i+1, reason)
			}
			total += width
		}
		return total, true, ""
	case "StructRef":
		if len(typ.TypeArgs) > 0 {
			return 0, false, "generic struct " + typ.StructName
		}
		decl, ok := g.structs[typ.StructName]
		if !ok {
			return 0, false, "unknown struct " + typ.StructName
		}
		total := 0
		for _, fld := range decl.Fields {
			width, ok, reason := g.stackWidth(fld.Type)
			if !ok {
				return 0, false, "struct " + typ.StructName + " field " + fld.Name + ": " + reason
			}
			total += width
		}
		return total, true, ""
	case "AliasRef":
		decl, ok := g.aliases[typ.AliasName]
		if !ok {
			return 0, false, "unknown alias " + typ.AliasName
		}
		if len(decl.TypeParams) > 0 || len(typ.TypeArgs) > 0 {
			return 0, false, "generic alias " + typ.AliasName
		}
		return g.stackWidth(decl.Target)
	case "nullable":
		if typ.StackWidth != nil {
			return *typ.StackWidth, true, ""
		}
		return 1, true, ""
	case "union":
		if typ.StackWidth == nil {
			return 0, false, "union without stack_width"
		}
		return *typ.StackWidth, true, ""
	case "genericT":
		return 0, false, "generic type " + typ.NameT
	default:
		return 1, true, ""
	}
}

func (g *generator) stackValueFlattens(typ abiType) bool {
	switch typ.Kind {
	case "tensor", "StructRef", "union":
		return true
	case "nullable":
		return typ.StackWidth != nil
	case "AliasRef":
		decl, ok := g.aliases[typ.AliasName]
		if !ok || len(decl.TypeParams) > 0 || len(typ.TypeArgs) > 0 {
			return false
		}
		return g.stackValueFlattens(decl.Target)
	default:
		return false
	}
}

func (g *generator) stackValueSliceExpr(typ abiType, value string) string {
	info := g.typeForStack(typ)
	if !info.Supported {
		return "nil"
	}
	return g.stackValueSliceExprWithInfo(typ, info, value)
}

func (g *generator) stackValueSliceExprWithInfo(typ abiType, info typeInfo, value string) string {
	if g.stackValueFlattens(typ) {
		return info.StackExpr(value)
	}
	return "[]any{" + info.StackExpr(value) + "}"
}

func (g *generator) stackValueItemExpr(typ abiType, value string) string {
	info := g.typeForStack(typ)
	if !info.Supported {
		return value
	}
	if g.stackValueFlattens(typ) {
		return g.stackValueSliceExpr(typ, value)
	}
	return info.StackExpr(value)
}

func (g *generator) appendStackValueLines(typ abiType, out, value string) []string {
	info := g.typeForStack(typ)
	if !info.Supported {
		return []string{fmt.Sprintf("%s = append(%s, %s)", out, out, value)}
	}
	if g.stackValueFlattens(typ) {
		return []string{fmt.Sprintf("%s = append(%s, %s...)", out, out, info.StackExpr(value))}
	}
	return []string{fmt.Sprintf("%s = append(%s, %s)", out, out, info.StackExpr(value))}
}

func isNullABIType(typ abiType) bool {
	return typ.Kind == "null" || typ.Kind == "nullLiteral"
}
