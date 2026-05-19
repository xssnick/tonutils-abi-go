package main

import (
	"bytes"
	"fmt"
	"strings"
)

type methodParam struct {
	Name string
	Type abiType
	Info typeInfo
}

func (g *generator) writeMethods(dst *bytes.Buffer) {
	for _, method := range g.abi.GetMethods {
		g.writeMethod(dst, g.contractName, method)
	}
}

func (g *generator) writeMethod(dst *bytes.Buffer, contractName string, method getMethod) {
	methodName := g.uniqueGetMethodName(method.Name)
	if methodName == "" {
		return
	}

	paramDecls := []string{
		"ctx context.Context",
		"block *ton.BlockIDExt",
	}
	params := make([]methodParam, 0, len(method.Parameters))
	usedParamNames := map[string]bool{
		"api":    true,
		"block":  true,
		"c":      true,
		"ctx":    true,
		"err":    true,
		"out":    true,
		"params": true,
		"result": true,
	}
	for i, param := range method.Parameters {
		paramName := uniqueParamName(unexportedName(param.Name), fmt.Sprintf("arg%d", i), usedParamNames)

		info := g.typeForStack(param.Type)
		paramDecls = append(paramDecls, paramName+" "+info.GoType)
		params = append(params, methodParam{
			Name: paramName,
			Type: param.Type,
			Info: info,
		})
	}

	returnInfo := g.typeForResult(method.ReturnType)
	resultTypeName := ""
	if returnInfo.Kind == "tuple" && returnInfo.Supported {
		resultTypeName = g.writeResultStruct(contractName, methodName, method.ReturnType.Items)
	}

	fmt.Fprintf(dst, "// %s runs get method %s (id: %d).\n", methodName, method.Name, method.TVMMethodID)
	g.writeUnsupportedParamDiagnostics(dst, methodName, params)
	switch {
	case returnInfo.Kind == "void":
		fmt.Fprintf(dst, "func (c *%s) %s(%s) error {\n", contractName, methodName, strings.Join(paramDecls, ", "))
		callArgs := g.writeParamStack(dst, params, "err")
		errAssign := ":="
		if g.paramsDeclareErr(params) {
			errAssign = "="
		}
		fmt.Fprintf(dst, "\t_, err %s c.api.RunGetMethodByID(ctx, block, c.addr, %s%s)\n", errAssign, methodIDExpr(method.TVMMethodID), callArgSuffix(callArgs))
		fmt.Fprintf(dst, "\treturn %s\n", g.contractErrorExpr("err"))
		dst.WriteString("}\n\n")
	case returnInfo.Kind == "tuple" && returnInfo.Supported:
		fmt.Fprintf(dst, "func (c *%s) %s(%s) (*%s, error) {\n", contractName, methodName, strings.Join(paramDecls, ", "), resultTypeName)
		callArgs := g.writeParamStack(dst, params, "nil, err")
		g.writeRunCall(dst, method.TVMMethodID, callArgs, "nil")
		totalWidth := 0
		for _, item := range method.ReturnType.Items {
			width, ok, _ := g.stackWidth(item)
			if ok {
				totalWidth += width
			}
		}
		fmt.Fprintf(dst, "\tif len(result.AsTuple()) < %d {\n", totalWidth)
		dst.WriteString("\t\treturn nil, ton.ErrResultIndexOutOfRange\n")
		dst.WriteString("\t}\n")
		fmt.Fprintf(dst, "\tout := &%s{}\n", resultTypeName)
		offset := 0
		fieldNames := resultFieldNames(method.ReturnType.Items)
		for i, item := range method.ReturnType.Items {
			itemInfo := g.typeForResult(item)
			fieldName := fieldNames[i]
			if itemInfo.Supported {
				for _, line := range g.stackDecodeLines(item, "out."+fieldName, "result.AsTuple()", offset, "nil", "decoded"+fmt.Sprint(i)) {
					fmt.Fprintf(dst, "\t%s\n", line)
				}
			}
			width, ok, _ := g.stackWidth(item)
			if ok {
				offset += width
			}
		}
		dst.WriteString("\treturn out, nil\n")
		dst.WriteString("}\n\n")
	case returnInfo.Supported:
		fmt.Fprintf(dst, "func (c *%s) %s(%s) (%s, error) {\n", contractName, methodName, strings.Join(paramDecls, ", "), returnInfo.GoType)
		callArgs := g.writeParamStack(dst, params, returnInfo.Zero+", err")
		g.writeRunCall(dst, method.TVMMethodID, callArgs, returnInfo.Zero)
		if g.stackValueFlattens(method.ReturnType) {
			width, ok, _ := g.stackWidth(method.ReturnType)
			if ok {
				fmt.Fprintf(dst, "\tif len(result.AsTuple()) < %d {\n", width)
				fmt.Fprintf(dst, "\t\treturn %s, ton.ErrResultIndexOutOfRange\n", returnInfo.Zero)
				dst.WriteString("\t}\n")
			}
			lines := g.stackDecodeLines(method.ReturnType, "out", "result.AsTuple()", 0, returnInfo.Zero, "decoded0")
			if len(lines) > 0 && strings.HasPrefix(lines[0], "out, err = ") {
				lines[0] = strings.Replace(lines[0], "out, err = ", "out, err := ", 1)
			} else {
				fmt.Fprintf(dst, "\tvar out %s\n", returnInfo.GoType)
			}
			for _, line := range lines {
				fmt.Fprintf(dst, "\t%s\n", line)
			}
		} else {
			for _, line := range returnInfo.ResultDecode("out", 0, returnInfo.Zero) {
				fmt.Fprintf(dst, "\t%s\n", line)
			}
		}
		dst.WriteString("\treturn out, nil\n")
		dst.WriteString("}\n\n")
	default:
		g.writeTODO(dst, "", "typed result for %s is not generated yet: %s.", methodName, returnInfo.Reason)
		fmt.Fprintf(dst, "func (c *%s) %s(%s) (*ton.ExecutionResult, error) {\n", contractName, methodName, strings.Join(paramDecls, ", "))
		callArgs := g.writeParamStack(dst, params, "nil, err")
		if g.hasThrownErrors() {
			fmt.Fprintf(dst, "\tresult, err := c.api.RunGetMethodByID(ctx, block, c.addr, %s%s)\n", methodIDExpr(method.TVMMethodID), callArgSuffix(callArgs))
			dst.WriteString("\tif err != nil {\n")
			dst.WriteString("\t\treturn nil, mapContractError(err)\n")
			dst.WriteString("\t}\n")
			dst.WriteString("\treturn result, nil\n")
		} else {
			fmt.Fprintf(dst, "\treturn c.api.RunGetMethodByID(ctx, block, c.addr, %s%s)\n", methodIDExpr(method.TVMMethodID), callArgSuffix(callArgs))
		}
		dst.WriteString("}\n\n")
	}
}

func (g *generator) writeRunCall(dst *bytes.Buffer, methodID int64, callArgs []string, errReturn string) {
	fmt.Fprintf(dst, "\tresult, err := c.api.RunGetMethodByID(ctx, block, c.addr, %s%s)\n", methodIDExpr(methodID), callArgSuffix(callArgs))
	dst.WriteString("\tif err != nil {\n")
	fmt.Fprintf(dst, "\t\treturn %s, %s\n", errReturn, g.contractErrorExpr("err"))
	dst.WriteString("\t}\n")
}

func (g *generator) contractErrorExpr(errExpr string) string {
	if !g.hasThrownErrors() {
		return errExpr
	}
	return "mapContractError(" + errExpr + ")"
}

func methodIDExpr(id int64) string {
	return fmt.Sprintf("uint64(%d)", uint64(id))
}

func (g *generator) writeResultStruct(contractName, methodName string, items []abiType) string {
	typeName := g.uniqueTypeName(contractName + methodName + "Result")
	var b bytes.Buffer
	fmt.Fprintf(&b, "type %s struct {\n", typeName)
	fieldNames := resultFieldNames(items)
	for i, item := range items {
		info := g.typeForResult(item)
		fieldName := fieldNames[i]
		fmt.Fprintf(&b, "\t%s %s\n", fieldName, info.GoType)
	}
	b.WriteString("}\n\n")
	g.resultTypes = append(g.resultTypes, b.String())
	return typeName
}
