package main

import (
	"bytes"
	"fmt"
	"strings"
)

func (g *generator) addGeneratedType(name string, body string) {
	if name == "" || g.generatedTypeSet[name] {
		return
	}
	g.generatedTypeSet[name] = true
	g.generatedTypes = append(g.generatedTypes, body)
}

func (g *generator) addGeneratedHelper(key string, body string) {
	if key == "" || g.generatedHelperSet[key] {
		return
	}
	g.generatedHelperSet[key] = true
	g.generatedTypes = append(g.generatedTypes, body)
}

func (g *generator) generatedName(prefix, suggestedName string, typ abiType) string {
	key := prefix + ":" + suggestedName + ":" + mapTypeSignature(typ)
	if name, ok := g.generatedTypeNames[key]; ok {
		return name
	}

	base := exportedName(suggestedName)
	if base == "" {
		base = exportedName(prefix)
	}
	if base == "" {
		base = "Generated"
	}

	name := base
	if suggestedName != "" {
		for i := 2; g.generatedTypeSet[name]; i++ {
			name = fmt.Sprintf("%s%d", base, i)
		}
		if g.names != nil {
			g.names.reservePackage(name)
		}
	} else if g.names != nil {
		name = g.names.uniquePackage(base, "Generated")
	} else {
		for i := 2; g.generatedTypeSet[name] || (suggestedName == "" && g.typeNameTaken(name)); i++ {
			name = fmt.Sprintf("%s%d", base, i)
		}
	}
	g.generatedTypeNames[key] = name
	return name
}

func (g *generator) tensorTypeForTLB(typ abiType, suggestedName string) typeInfo {
	name := g.generatedName("Tuple", suggestedName, typ)
	fieldNames := tupleFieldNames(typ.Items)
	var b bytes.Buffer
	fmt.Fprintf(&b, "type %s struct {\n", name)
	for i, item := range typ.Items {
		fieldName := fieldNames[i]
		info := g.typeForTLBNamed(item, true, name+fieldName)
		if !info.Supported {
			return unsupported("tensor item " + fieldName + ": " + info.Reason)
		}
		if info.TLBTag == "" {
			return unsupported("tensor item " + fieldName + " has no TLB tag")
		}
		fmt.Fprintf(&b, "\t%s %s `tlb:%q`\n", fieldName, info.GoType, info.TLBTag)
	}
	b.WriteString("}\n\n")
	g.addGeneratedType(name, b.String())

	return typeInfo{
		GoType:    name,
		TLBTag:    ".",
		Supported: true,
		Kind:      "tensor",
		Zero:      name + "{}",
	}
}

func (g *generator) lispListTypeForTLB(typ abiType, suggestedName string) typeInfo {
	if typ.Inner == nil {
		return unsupported("lispListOf without inner type")
	}

	inner := g.typeForTLBNamed(*typ.Inner, true, exportedName(suggestedName)+"Item")
	if !inner.Supported {
		return inner
	}
	if inner.TLBTag == "" {
		return unsupported("lispListOf inner type without TLB tag")
	}

	g.useImport("github.com/xssnick/tonutils-go/tlb")
	g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
	name := g.generatedName("LispList", suggestedName, typ)
	boxName := unexportedName(name) + "ItemBox"

	var b bytes.Buffer
	fmt.Fprintf(&b, "type %s []%s\n\n", name, inner.GoType)
	fmt.Fprintf(&b, "type %s struct {\n", boxName)
	fmt.Fprintf(&b, "\tValue %s `tlb:%q`\n", inner.GoType, inner.TLBTag)
	b.WriteString("}\n\n")

	fmt.Fprintf(&b, "func (l *%s) LoadFromCell(loader *cell.Slice) error {\n", name)
	b.WriteString("\thead, err := loader.LoadRef()\n")
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\treturn err\n")
	b.WriteString("\t}\n")
	fmt.Fprintf(&b, "\tout := make(%s, 0)\n", name)
	b.WriteString("\tfor head.RefsNum() > 0 {\n")
	b.WriteString("\t\ttail, err := head.LoadRef()\n")
	b.WriteString("\t\tif err != nil {\n")
	b.WriteString("\t\t\treturn err\n")
	b.WriteString("\t\t}\n")
	fmt.Fprintf(&b, "\t\tvar box %s\n", boxName)
	b.WriteString("\t\tif err := tlb.LoadFromCell(&box, head); err != nil {\n")
	b.WriteString("\t\t\treturn err\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t\tout = append(out, box.Value)\n")
	b.WriteString("\t\thead = tail\n")
	b.WriteString("\t}\n")
	b.WriteString("\tfor i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {\n")
	b.WriteString("\t\tout[i], out[j] = out[j], out[i]\n")
	b.WriteString("\t}\n")
	b.WriteString("\t*l = out\n")
	b.WriteString("\treturn nil\n")
	b.WriteString("}\n\n")

	fmt.Fprintf(&b, "func (l %s) ToCell() (*cell.Cell, error) {\n", name)
	b.WriteString("\ttail := cell.BeginCell().EndCell()\n")
	b.WriteString("\tfor i := 0; i < len(l); i++ {\n")
	fmt.Fprintf(&b, "\t\titem, err := tlb.ToCell(%s{Value: l[i]})\n", boxName)
	b.WriteString("\t\tif err != nil {\n")
	b.WriteString("\t\t\treturn nil, err\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t\titemBuilder := item.ToBuilder()\n")
	b.WriteString("\t\tif err := itemBuilder.StoreRef(tail); err != nil {\n")
	b.WriteString("\t\t\treturn nil, err\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t\ttail = itemBuilder.EndCell()\n")
	b.WriteString("\t}\n")
	b.WriteString("\tb := cell.BeginCell()\n")
	b.WriteString("\tif err := b.StoreRef(tail); err != nil {\n")
	b.WriteString("\t\treturn nil, err\n")
	b.WriteString("\t}\n")
	b.WriteString("\treturn b.EndCell(), nil\n")
	b.WriteString("}\n\n")
	g.addGeneratedType(name, b.String())

	return typeInfo{
		GoType:    name,
		TLBTag:    ".",
		Supported: true,
		Kind:      "lispList",
		Zero:      "nil",
	}
}

func (g *generator) tupleTypeForStack(typ abiType, suggestedName string) typeInfo {
	g.useImport("fmt")
	name := ""
	includeType := true
	if suggested := exportedName(suggestedName); suggested != "" && g.generatedTypeSet[suggested] {
		name = suggested
		includeType = false
	}
	if name == "" {
		name = g.generatedName("Tuple", suggestedName, typ)
	}
	includeType = includeType && !g.generatedTypeSet[name]
	expectedItems := len(typ.Items)
	if typ.Kind == "tensor" {
		width, ok, reason := g.stackWidth(typ)
		if !ok {
			return unsupported(reason)
		}
		expectedItems = width
	}
	var b bytes.Buffer
	fieldNames := tupleFieldNames(typ.Items)
	if includeType {
		fmt.Fprintf(&b, "type %s struct {\n", name)
		for i, item := range typ.Items {
			fieldName := fieldNames[i]
			info := g.typeForResult(item)
			if !info.Supported {
				return unsupported("tuple item " + fieldName + ": " + info.Reason)
			}
			fmt.Fprintf(&b, "\t%s %s\n", fieldName, info.GoType)
		}
		b.WriteString("}\n\n")
	}
	fmt.Fprintf(&b, "func stack%s(value *%s) []any {\n", name, name)
	b.WriteString("\tif value == nil {\n")
	b.WriteString("\t\treturn nil\n")
	b.WriteString("\t}\n")
	fmt.Fprintf(&b, "\tout := make([]any, 0, %d)\n", expectedItems)
	for i, item := range typ.Items {
		fieldName := fieldNames[i]
		if typ.Kind == "shapedTuple" {
			fmt.Fprintf(&b, "\tout = append(out, %s)\n", g.stackValueItemExpr(item, "value."+fieldName))
		} else {
			for _, line := range g.appendStackValueLines(item, "out", "value."+fieldName) {
				fmt.Fprintf(&b, "\t%s\n", line)
			}
		}
	}
	b.WriteString("\treturn out\n")
	b.WriteString("}\n\n")
	fmt.Fprintf(&b, "func decode%sStackTuple(values []any) (*%s, error) {\n", name, name)
	fmt.Fprintf(&b, "\tif len(values) < %d {\n", expectedItems)
	fmt.Fprintf(&b, "\t\treturn nil, fmt.Errorf(\"%s stack tuple expects %d items, got %%d\", len(values))\n", name, expectedItems)
	b.WriteString("\t}\n")
	fmt.Fprintf(&b, "\tout := &%s{}\n", name)
	offset := 0
	var body []string
	for i, item := range typ.Items {
		fieldName := fieldNames[i]
		temp := unexportedName(name + fieldName)
		var lines []string
		if typ.Kind == "shapedTuple" {
			lines = g.rawResultDecodeLines(item, "out."+fieldName, fmt.Sprintf("values[%d]", i), "nil", temp)
		} else {
			lines = g.stackDecodeLines(item, "out."+fieldName, "values", offset, "nil", temp)
			width, ok, _ := g.stackWidth(item)
			if ok {
				offset += width
			}
		}
		body = append(body, lines...)
	}
	if decodeLinesNeedErrVar(body) {
		b.WriteString("\tvar err error\n")
	}
	for _, line := range body {
		fmt.Fprintf(&b, "\t%s\n", line)
	}
	b.WriteString("\treturn out, nil\n")
	b.WriteString("}\n\n")
	if includeType {
		g.addGeneratedType(name, b.String())
		g.generatedHelperSet["stackTuple:"+name] = true
	} else {
		g.addGeneratedHelper("stackTuple:"+name, b.String())
	}

	return typeInfo{
		GoType:    "*" + name,
		Supported: true,
		Kind:      "tupleStruct",
		StackExpr: func(value string) string {
			return fmt.Sprintf("stack%s(%s)", name, value)
		},
		ResultDecode: func(target string, index uint, errReturn string) []string {
			return []string{
				fmt.Sprintf("tuple%d, err := result.Tuple(%d)", index, index),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
				fmt.Sprintf("%s, err %s decode%sStackTuple(tuple%d)", target, assignOp(target), name, index),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
			}
		},
		Zero: "nil",
	}
}

func (g *generator) writeStackTupleHelpers(dst *bytes.Buffer, declName string, fields []field) {
	name := exportedName(declName)
	if name == "" {
		return
	}
	width := 0
	for _, fld := range fields {
		fieldWidth, ok, _ := g.stackWidth(fld.Type)
		if ok {
			width += fieldWidth
		}
	}
	fmt.Fprintf(dst, "func stack%s(value *%s) []any {\n", name, name)
	dst.WriteString("\tif value == nil {\n")
	dst.WriteString("\t\treturn nil\n")
	dst.WriteString("\t}\n")
	fmt.Fprintf(dst, "\tout := make([]any, 0, %d)\n", width)
	fieldNames := declarationFieldNames(fields)
	for i, fld := range fields {
		fieldName := fieldNames[i]
		for _, line := range g.appendStackStructFieldLines(declName, fld, fieldName, "out", "value."+fieldName) {
			fmt.Fprintf(dst, "\t%s\n", line)
		}
	}
	dst.WriteString("\treturn out\n")
	dst.WriteString("}\n\n")
}

func (g *generator) appendStackStructFieldLines(declName string, fld field, fieldName, out, value string) []string {
	if g.tlbStructs[declName] {
		declared := g.typeForTLBNamed(fld.Type, true, exportedName(declName)+fieldName)
		if declared.Supported {
			return g.appendStackValueLinesForDeclaredType(fld.Type, declared, out, value)
		}
	}
	return g.appendStackValueLinesNamed(fld.Type, exportedName(declName)+fieldName, out, value)
}

func (g *generator) appendStackValueLinesNamed(typ abiType, suggestedName, out, value string) []string {
	info := g.typeForStackNamed(typ, suggestedName)
	if !info.Supported {
		return []string{fmt.Sprintf("%s = append(%s, %s)", out, out, value)}
	}
	if g.stackValueFlattens(typ) {
		return []string{fmt.Sprintf("%s = append(%s, %s...)", out, out, info.StackExpr(value))}
	}
	return []string{fmt.Sprintf("%s = append(%s, %s)", out, out, info.StackExpr(value))}
}

func (g *generator) appendStackValueLinesForDeclaredType(typ abiType, declared typeInfo, out, value string) []string {
	switch typ.Kind {
	case "AliasRef":
		decl, ok := g.aliases[typ.AliasName]
		if ok && aliasDecodesDirectlyThroughTarget(decl.Target) {
			return g.appendStackValueLinesForDeclaredType(decl.Target, declared, out, value)
		}
	case "StructRef":
		name := exportedName(typ.StructName)
		g.useStackStructEncoder(typ.StructName)
		arg := value
		if !strings.HasPrefix(declared.GoType, "*") {
			arg = "&" + value
		}
		return []string{fmt.Sprintf("%s = append(%s, stack%s(%s)...)", out, out, name, arg)}
	case "tensor":
		name := declaredStackTypeName(declared.GoType)
		info := g.tupleTypeForStack(typ, name)
		if !info.Supported {
			return []string{fmt.Sprintf("%s = append(%s, %s)", out, out, value)}
		}
		arg := value
		if !strings.HasPrefix(declared.GoType, "*") {
			arg = "&" + value
		}
		return []string{fmt.Sprintf("%s = append(%s, stack%s(%s)...)", out, out, name, arg)}
	case "union":
		name := declaredStackTypeName(declared.GoType)
		info := g.unionTypeForStack(typ, name)
		if !info.Supported {
			return []string{fmt.Sprintf("%s = append(%s, %s)", out, out, value)}
		}
		arg := value
		if !declared.Interface && !strings.HasPrefix(declared.GoType, "*") {
			arg = "&" + value
		}
		return []string{fmt.Sprintf("%s = append(%s, stack%s(%s)...)", out, out, name, arg)}
	case "nullable":
		if typ.Inner != nil && declaredUsesCompoundStackType(*typ.Inner, declared) {
			return g.appendNullableDeclaredStackValueLines(*typ.Inner, declared, out, value, typ.StackWidth, typ.StackTypeID)
		}
	case "remaining":
		if declared.GoType == "*cell.Cell" {
			return []string{fmt.Sprintf("%s = append(%s, %s)", out, out, value)}
		}
	}
	return g.appendStackValueLinesNamed(typ, declaredStackTypeName(declared.GoType), out, value)
}

func (g *generator) appendNullableDeclaredStackValueLines(inner abiType, declared typeInfo, out, value string, stackWidth, stackTypeID *int) []string {
	name := declaredStackTypeName(declared.GoType)
	encode := "nil"
	switch inner.Kind {
	case "AliasRef":
		if decl, ok := g.aliases[inner.AliasName]; ok && aliasDecodesDirectlyThroughTarget(decl.Target) {
			return g.appendNullableDeclaredStackValueLines(decl.Target, declared, out, value, stackWidth, stackTypeID)
		}
	case "StructRef":
		name = exportedName(inner.StructName)
		g.useStackStructEncoder(inner.StructName)
		if strings.HasPrefix(declared.GoType, "*") {
			encode = fmt.Sprintf("func(v %s) []any { return stack%s(v) }", declared.GoType, name)
		} else {
			encode = fmt.Sprintf("func(v %s) []any { return stack%s(&v) }", declared.GoType, name)
		}
	case "tensor":
		g.tupleTypeForStack(inner, name)
		encode = fmt.Sprintf("func(v %s) []any { return stack%s(v) }", declared.GoType, name)
	case "union":
		g.unionTypeForStack(inner, name)
		if declared.Interface || strings.HasPrefix(declared.GoType, "*") {
			encode = fmt.Sprintf("func(v %s) []any { return stack%s(v) }", declared.GoType, name)
		} else {
			encode = fmt.Sprintf("func(v %s) []any { return stack%s(&v) }", declared.GoType, name)
		}
	}
	if stackWidth != nil && stackTypeID != nil {
		if strings.HasPrefix(declared.GoType, "*") || declared.Interface {
			g.useHelper(helperWideNullableValue)
			return []string{fmt.Sprintf("%s = append(%s, stackWideNullableValue(%s, %d, int64(%d), %s)...)", out, out, value, *stackWidth, *stackTypeID, encode)}
		}
		g.useHelper(helperWideNullablePtr)
		return []string{fmt.Sprintf("%s = append(%s, stackWideNullablePtr(&%s, %d, int64(%d), %s)...)", out, out, value, *stackWidth, *stackTypeID, encode)}
	}
	return g.appendStackValueLinesNamed(abiType{Kind: "nullable", Inner: &inner}, name, out, value)
}

func (g *generator) writeGeneratedStackTupleHelpers(dst *bytes.Buffer) {
	// Synthetic tuple helpers are emitted lazily by raw/param helpers in this file.
}

func (g *generator) stackArrayType(typ abiType) typeInfo {
	if typ.Inner == nil {
		return unsupportedStack("arrayOf without inner type")
	}
	inner := g.typeForStack(*typ.Inner)
	if !inner.Supported {
		return unsupportedStack("arrayOf inner type: " + inner.Reason)
	}
	g.useHelper(helperStackArray)
	goType := "[]" + inner.GoType
	return typeInfo{
		GoType:    goType,
		Supported: true,
		Kind:      "array",
		StackExpr: func(value string) string {
			return fmt.Sprintf("stackArray(%s, func(v %s) any { return %s })", value, inner.GoType, g.stackValueItemExpr(*typ.Inner, "v"))
		},
		Zero: "nil",
	}
}

func (g *generator) nullableTypeForStack(typ abiType) typeInfo {
	return g.nullableTypeForStackNamed(typ, "")
}

func (g *generator) nullableTypeForStackNamed(typ abiType, suggestedName string) typeInfo {
	if typ.Inner == nil {
		return unsupportedStack("nullable without inner type")
	}
	inner := g.typeForStackNamed(*typ.Inner, suggestedName)
	if !inner.Supported {
		return inner
	}
	goType := inner.GoType
	if !inner.Interface {
		goType = nullableGoType(inner.GoType)
	}
	useValue := inner.Interface || strings.HasPrefix(inner.GoType, "*") || strings.HasPrefix(inner.GoType, "[]") || inner.GoType == "any"
	if typ.StackWidth != nil && typ.StackTypeID != nil {
		if useValue {
			g.useHelper(helperWideNullableValue)
		} else {
			g.useHelper(helperWideNullablePtr)
		}
		return typeInfo{
			GoType:    goType,
			Supported: true,
			Kind:      "nullable",
			Interface: inner.Interface,
			StackExpr: func(value string) string {
				encode := fmt.Sprintf("func(v %s) []any { return %s }", inner.GoType, g.stackValueSliceExprWithInfo(*typ.Inner, inner, "v"))
				if useValue {
					return fmt.Sprintf("stackWideNullableValue(%s, %d, int64(%d), %s)", value, *typ.StackWidth, *typ.StackTypeID, encode)
				}
				return fmt.Sprintf("stackWideNullablePtr(%s, %d, int64(%d), %s)", value, *typ.StackWidth, *typ.StackTypeID, encode)
			},
			Zero: "nil",
		}
	}
	if useValue {
		g.useHelper(helperNullableValue)
	} else {
		g.useHelper(helperNullablePtr)
	}
	return typeInfo{
		GoType:    goType,
		Supported: true,
		Kind:      "nullable",
		Interface: inner.Interface,
		StackExpr: func(value string) string {
			if useValue {
				return fmt.Sprintf("stackNullableValue(%s, func(v %s) any { return %s })", value, inner.GoType, inner.StackExpr("v"))
			}
			return fmt.Sprintf("stackNullablePtr(%s, func(v %s) any { return %s })", value, inner.GoType, inner.StackExpr("v"))
		},
		Zero: "nil",
	}
}

func (g *generator) stackCellOfType(typ abiType) typeInfo {
	if typ.Inner == nil {
		return unsupportedStack("cellOf without inner type")
	}
	if typ.Inner.Kind == "slice" {
		g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
		return typeInfo{
			GoType:    "*cell.Cell",
			Supported: true,
			Kind:      "cell",
			StackExpr: func(value string) string { return value },
			Zero:      "nil",
		}
	}
	inner := g.typeForTLB(*typ.Inner, true)
	if !inner.Supported {
		return unsupportedStack("cellOf inner type: " + inner.Reason)
	}
	g.useHelper(helperStackCell)
	return typeInfo{
		GoType:    inner.GoType,
		Supported: true,
		Kind:      "cellOf",
		StackExpr: func(value string) string {
			return fmt.Sprintf("mustStackCellOf(%s)", value)
		},
		StackErrExpr: func(value string) string {
			return fmt.Sprintf("stackCellOf(%s)", value)
		},
		StackErr: true,
		Zero:     inner.Zero,
	}
}

func (g *generator) stackMapType(typ abiType) typeInfo {
	info := g.mapTypeForTLB(typ, "")
	if !info.Supported {
		return unsupportedStack(info.Reason)
	}
	g.useHelper(helperStackCell)
	info.StackExpr = func(value string) string {
		return fmt.Sprintf("mustStackCellOf(%s)", value)
	}
	info.StackErrExpr = func(value string) string {
		return fmt.Sprintf("stackCellOf(%s)", value)
	}
	info.StackErr = true
	info.Zero = info.GoType + "{}"
	return info
}

func (g *generator) resultArrayType(typ abiType) typeInfo {
	if typ.Inner == nil {
		return unsupported("arrayOf without inner type")
	}
	if typ.Inner.Kind == "unknown" {
		return typeInfo{
			GoType:    "[]any",
			Supported: true,
			Kind:      "tupleAny",
			ResultDecode: func(target string, index uint, errReturn string) []string {
				return []string{
					fmt.Sprintf("%s, err %s result.Tuple(%d)", target, assignOp(target), index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
				}
			},
			Zero: "nil",
		}
	}

	inner := g.typeForResult(*typ.Inner)
	if !inner.Supported {
		return unsupported("arrayOf inner type: " + inner.Reason)
	}
	goType := "[]" + inner.GoType
	return typeInfo{
		GoType:    goType,
		Supported: true,
		Kind:      "array",
		ResultDecode: func(target string, index uint, errReturn string) []string {
			tuple := fmt.Sprintf("tuple%d", index)
			lines := []string{
				fmt.Sprintf("%s, err := result.Tuple(%d)", tuple, index),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
			}
			lines = append(lines, g.decodeStackArrayLines(*typ.Inner, target, tuple, errReturn, fmt.Sprintf("decoded%d", index))...)
			return lines
		},
		Zero: "nil",
	}
}

func (g *generator) resultCellOfType(typ abiType) typeInfo {
	if typ.Inner == nil {
		return unsupported("cellOf without inner type")
	}
	if typ.Inner.Kind == "slice" {
		return g.typeForResult(abiType{Kind: "cell"})
	}
	inner := g.typeForTLB(*typ.Inner, true)
	if !inner.Supported {
		return unsupported("cellOf inner type: " + inner.Reason)
	}
	g.useImport("github.com/xssnick/tonutils-go/tlb")
	g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
	return typeInfo{
		GoType:    inner.GoType,
		Supported: true,
		Kind:      "cellOf",
		ResultDecode: func(target string, index uint, errReturn string) []string {
			raw := fmt.Sprintf("cell%d", index)
			lines := []string{
				fmt.Sprintf("%s, err := result.Cell(%d)", raw, index),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
			}
			if assignOp(target) == ":=" {
				lines = append(lines, fmt.Sprintf("var %s %s", target, inner.GoType))
			}
			lines = append(lines,
				fmt.Sprintf("if err := tlb.Parse(&%s, %s); err != nil {", target, raw),
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
			)
			return lines
		},
		Zero: inner.Zero,
	}
}

func (g *generator) resultMapType(typ abiType) typeInfo {
	info := g.mapTypeForTLB(typ, "")
	if !info.Supported {
		return unsupported(info.Reason)
	}
	g.useImport("github.com/xssnick/tonutils-go/tlb")
	g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
	info.ResultDecode = func(target string, index uint, errReturn string) []string {
		raw := fmt.Sprintf("cell%d", index)
		lines := []string{
			fmt.Sprintf("%s, err := result.Cell(%d)", raw, index),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
		if assignOp(target) == ":=" {
			lines = append(lines, fmt.Sprintf("var %s %s", target, info.GoType))
		}
		lines = append(lines,
			fmt.Sprintf("if err := tlb.Parse(&%s, %s); err != nil {", target, raw),
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		)
		return lines
	}
	info.Zero = info.GoType + "{}"
	return info
}

func (g *generator) decodeStackArrayLines(inner abiType, target, tuple, errReturn, temp string) []string {
	innerInfo := g.typeForResult(inner)
	if !innerInfo.Supported {
		return []string{fmt.Sprintf("// TODO: unsupported stack array %s: %s.", target, innerInfo.Reason)}
	}
	item := temp + "Item"
	raw := temp + "Raw"
	decodeLines := g.rawResultDecodeLines(inner, item, raw, errReturn, item+"Value")
	rangeValue := raw
	if !linesReferenceIdentifier(decodeLines, raw) {
		rangeValue = "_"
	}
	lines := []string{
		fmt.Sprintf("%s %s make([]%s, 0, len(%s))", target, assignOp(target), innerInfo.GoType, tuple),
	}
	if rangeValue == "_" {
		lines = append(lines, fmt.Sprintf("for range %s {", tuple))
	} else {
		lines = append(lines, fmt.Sprintf("for _, %s := range %s {", rangeValue, tuple))
	}
	if rawResultDecodeNeedsDeclaredTarget(inner) {
		lines = append(lines, fmt.Sprintf("\tvar %s %s", item, innerInfo.GoType))
	}
	for _, line := range decodeLines {
		lines = append(lines, "\t"+line)
	}
	lines = append(lines,
		fmt.Sprintf("\t%s = append(%s, %s)", target, target, item),
		"}",
	)
	return lines
}

func linesReferenceIdentifier(lines []string, ident string) bool {
	for _, line := range lines {
		if strings.Contains(line, ident) {
			return true
		}
	}
	return false
}
