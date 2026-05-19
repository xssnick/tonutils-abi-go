package main

import (
	"bytes"
	"fmt"
	"strings"
)

func (g *generator) writeUnsupportedParamDiagnostics(dst *bytes.Buffer, methodName string, params []methodParam) {
	for _, param := range params {
		switch {
		case !param.Info.Supported:
			g.writeTODO(dst, "", "stack parameter %s for %s returns an encode error because its type is not generated yet: %s.", param.Name, methodName, param.Info.Reason)
		case g.paramNeedsNestedEncodingError(param):
			g.writeTODO(dst, "", "stack parameter %s for %s returns an encode error because nested fallible stack encoding is not generated yet.", param.Name, methodName)
		}
	}
}

func (g *generator) writeParamStack(dst *bytes.Buffer, params []methodParam, errReturn string) []string {
	if len(params) == 0 {
		return nil
	}
	if g.paramsCanUseDirectCall(params) {
		callArgs := make([]string, 0, len(params))
		for _, param := range params {
			callArgs = append(callArgs, param.Info.StackExpr(param.Name))
		}
		return callArgs
	}
	fmt.Fprintf(dst, "\tparams := make([]any, 0, %d)\n", len(params))
	for _, param := range params {
		for _, line := range g.appendStackParamValueLines(param, "params", errReturn) {
			dst.WriteString("\t")
			dst.WriteString(line)
			dst.WriteString("\n")
		}
	}
	return []string{"params..."}
}

func (g *generator) paramsCanUseDirectCall(params []methodParam) bool {
	for _, param := range params {
		if !param.Info.Supported {
			return false
		}
		if g.stackParamEncodingMayFail(param.Type) {
			return false
		}
		if g.stackValueFlattens(param.Type) {
			return false
		}
	}
	return true
}

func (g *generator) paramsDeclareErr(params []methodParam) bool {
	if g.paramsCanUseDirectCall(params) {
		return false
	}
	for _, param := range params {
		if param.Info.Supported && param.Info.StackErr && !g.stackValueFlattens(param.Type) {
			return true
		}
	}
	return false
}

func (g *generator) appendStackParamValueLines(param methodParam, out, errReturn string) []string {
	info := param.Info
	if !info.Supported {
		g.useImport("fmt")
		errExpr := fmt.Sprintf("fmt.Errorf(%q)", fmt.Sprintf("encode stack parameter %s: %s", param.Name, info.Reason))
		return []string{fmt.Sprintf("return %s", returnWithError(errReturn, errExpr))}
	}
	if g.paramNeedsNestedEncodingError(param) {
		g.useImport("fmt")
		errExpr := fmt.Sprintf("fmt.Errorf(%q)", fmt.Sprintf("encode stack parameter %s: nested fallible stack encoding is not generated yet", param.Name))
		return []string{fmt.Sprintf("return %s", returnWithError(errReturn, errExpr))}
	}
	if info.StackErr && info.StackErrExpr != nil && !g.stackValueFlattens(param.Type) {
		g.useImport("fmt")
		tmp := unexportedName(param.Name) + "Stack"
		return []string{
			fmt.Sprintf("%s, err := %s", tmp, info.StackErrExpr(param.Name)),
			"if err != nil {",
			fmt.Sprintf("\treturn %s", returnWithError(errReturn, fmt.Sprintf("fmt.Errorf(\"encode stack parameter %s: %%w\", err)", param.Name))),
			"}",
			fmt.Sprintf("%s = append(%s, %s)", out, out, tmp),
		}
	}
	return g.appendStackValueLines(param.Type, out, param.Name)
}

func (g *generator) paramNeedsNestedEncodingError(param methodParam) bool {
	return param.Info.Supported &&
		g.stackParamEncodingMayFail(param.Type) &&
		!(param.Info.StackErr && !g.stackValueFlattens(param.Type))
}

func (g *generator) stackParamEncodingMayFail(typ abiType) bool {
	info := g.typeForStack(typ)
	if !info.Supported {
		return true
	}
	if info.StackErr {
		return true
	}

	switch typ.Kind {
	case "AliasRef":
		decl, ok := g.aliases[typ.AliasName]
		if !ok || len(decl.TypeParams) > 0 || len(typ.TypeArgs) > 0 {
			return !ok
		}
		return g.stackParamEncodingMayFail(decl.Target)
	case "StructRef":
		decl, ok := g.structs[typ.StructName]
		if !ok || len(typ.TypeArgs) > 0 {
			return !ok
		}
		for _, fld := range decl.Fields {
			if g.stackParamEncodingMayFail(fld.Type) {
				return true
			}
		}
	case "arrayOf", "lispListOf", "nullable", "cellOf":
		if typ.Inner == nil {
			return true
		}
		return typ.Kind == "cellOf" && typ.Inner.Kind != "slice" || g.stackParamEncodingMayFail(*typ.Inner)
	case "tensor", "shapedTuple", "union":
		for _, item := range typ.Items {
			if g.stackParamEncodingMayFail(item) {
				return true
			}
		}
	}
	return false
}

func returnWithError(errReturn, errExpr string) string {
	if errReturn == "err" {
		return errExpr
	}
	if strings.HasSuffix(errReturn, ", err") {
		return strings.TrimSuffix(errReturn, "err") + errExpr
	}
	return errReturn
}
