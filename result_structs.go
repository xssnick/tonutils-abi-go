package main

import (
	"bytes"
	"fmt"
	"strings"
)

func (g *generator) writeStackResultDecoders(dst *bytes.Buffer) {
	written := map[string]bool{}
	for {
		progress := false
		for _, decl := range g.abi.Declarations {
			if decl.Kind != "struct" || !g.stackResultDecoders[decl.Name] || written[decl.Name] {
				continue
			}
			written[decl.Name] = true
			if !g.stackStructResultSupported(decl) {
				continue
			}
			g.writeStackResultDecoder(dst, decl)
			progress = true
		}
		if !progress {
			return
		}
	}
}

func (g *generator) writeStackStructEncoders(dst *bytes.Buffer) {
	written := map[string]bool{}
	for {
		progress := false
		for _, decl := range g.abi.Declarations {
			if decl.Kind != "struct" || !g.stackStructEncoders[decl.Name] || written[decl.Name] {
				continue
			}
			written[decl.Name] = true
			if !g.stackStructEncoderSupported(decl) {
				continue
			}
			g.writeStackTupleHelpers(dst, decl.Name, decl.Fields)
			progress = true
		}
		if !progress {
			return
		}
	}
}

func (g *generator) stackStructResultSupported(decl declaration) bool {
	for _, fld := range decl.Fields {
		if !g.typeForResult(fld.Type).Supported {
			return false
		}
		if _, ok, _ := g.stackWidth(fld.Type); !ok {
			return false
		}
	}
	return true
}

func (g *generator) stackStructEncoderSupported(decl declaration) bool {
	for _, fld := range decl.Fields {
		if !g.typeForStack(fld.Type).Supported {
			return false
		}
	}
	return true
}

func (g *generator) writeStackResultDecoder(dst *bytes.Buffer, decl declaration) {
	name := exportedName(decl.Name)
	if name == "" {
		return
	}
	g.useImport("fmt")
	totalWidth := 0
	for _, fld := range decl.Fields {
		width, ok, _ := g.stackWidth(fld.Type)
		if ok {
			totalWidth += width
		}
	}

	fmt.Fprintf(dst, "func decode%sResult(values []any) (*%s, error) {\n", name, name)
	fmt.Fprintf(dst, "\tif len(values) < %d {\n", totalWidth)
	fmt.Fprintf(dst, "\t\treturn nil, fmt.Errorf(\"%s result tuple expects %d items, got %%d\", len(values))\n", name, totalWidth)
	dst.WriteString("\t}\n")
	fmt.Fprintf(dst, "\tout := &%s{}\n", name)
	offset := 0
	fieldNames := declarationFieldNames(decl.Fields)
	var body []string
	for i, fld := range decl.Fields {
		fieldName := fieldNames[i]
		temp := unexportedName(name + fieldName)
		body = append(body, g.stackDecodeStructFieldLines(decl, fld, fieldName, "out."+fieldName, "values", offset, "nil", temp)...)
		width, ok, _ := g.stackWidth(fld.Type)
		if ok {
			offset += width
		}
	}
	if decodeLinesNeedErrVar(body) {
		dst.WriteString("\tvar err error\n")
	}
	for _, line := range body {
		fmt.Fprintf(dst, "\t%s\n", line)
	}
	dst.WriteString("\treturn out, nil\n")
	dst.WriteString("}\n\n")
}

func (g *generator) stackDecodeStructFieldLines(decl declaration, fld field, fieldName, target, values string, offset int, errReturn, temp string) []string {
	if g.tlbStructs[decl.Name] {
		declared := g.typeForTLBNamed(fld.Type, true, exportedName(decl.Name)+fieldName)
		if declared.Supported {
			return g.stackDecodeLinesForDeclaredType(fld.Type, declared, target, values, offset, errReturn, temp)
		}
	}
	return g.stackDecodeLinesNamed(fld.Type, exportedName(decl.Name)+fieldName, target, values, offset, errReturn, temp)
}

func (g *generator) stackDecodeLinesForDeclaredType(typ abiType, declared typeInfo, target, values string, offset int, errReturn, temp string) []string {
	switch typ.Kind {
	case "AliasRef":
		decl, ok := g.aliases[typ.AliasName]
		if !ok {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: unknown alias %s.", target, typ.AliasName)}
		}
		if aliasDecodesDirectlyThroughTarget(decl.Target) {
			return g.stackDecodeLinesForDeclaredType(decl.Target, declared, target, values, offset, errReturn, temp)
		}
	case "StructRef":
		width, ok, reason := g.stackWidth(typ)
		if !ok {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, reason)}
		}
		name := exportedName(typ.StructName)
		g.useStackResultDecoder(typ.StructName)
		tmp := temp + "Struct"
		lines := []string{
			fmt.Sprintf("%s, err := decode%sResult(%s[%d:%d])", tmp, name, values, offset, offset+width),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
		lines = append(lines, assignDecodedDeclaredValue(target, tmp, declared)...)
		return lines
	case "tensor":
		width, ok, reason := g.stackWidth(typ)
		if !ok {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, reason)}
		}
		name := declaredStackTypeName(declared.GoType)
		info := g.tupleTypeForStack(typ, name)
		if !info.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, info.Reason)}
		}
		tmp := temp + "TupleStruct"
		lines := []string{
			fmt.Sprintf("%s, err := decode%sStackTuple(%s[%d:%d])", tmp, name, values, offset, offset+width),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
		lines = append(lines, assignDecodedDeclaredValue(target, tmp, declared)...)
		return lines
	case "union":
		width, ok, reason := g.stackWidth(typ)
		if !ok {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, reason)}
		}
		name := declaredStackTypeName(declared.GoType)
		info := g.unionTypeForResult(typ, name)
		if !info.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, info.Reason)}
		}
		tmp := temp + "Union"
		lines := []string{
			fmt.Sprintf("%s, err := decode%sStackUnion(%s[%d:%d])", tmp, name, values, offset, offset+width),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
		lines = append(lines, assignDecodedDeclaredValue(target, tmp, declared)...)
		return lines
	case "nullable":
		if typ.Inner != nil && declaredUsesCompoundStackType(*typ.Inner, declared) {
			return g.stackDecodeNullableDeclaredType(*typ.Inner, declared, target, values, offset, errReturn, temp, typ.StackWidth, typ.StackTypeID)
		}
	case "remaining":
		if declared.GoType == "*cell.Cell" {
			g.useDecodeStackSlice()
			tmp := temp + "Slice"
			return []string{
				fmt.Sprintf("%s, err := decodeStackSlice(%s[%d])", tmp, values, offset),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
				fmt.Sprintf("%s, err = %s.ToCell()", target, tmp),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
			}
		}
	}
	return g.stackDecodeLinesNamed(typ, declaredStackTypeName(declared.GoType), target, values, offset, errReturn, temp)
}

func (g *generator) stackDecodeNullableDeclaredType(inner abiType, declared typeInfo, target, values string, offset int, errReturn, temp string, stackWidth, stackTypeID *int) []string {
	if stackWidth == nil || stackTypeID == nil {
		source := fmt.Sprintf("%s[%d]", values, offset)
		lines := []string{fmt.Sprintf("if %s == nil {", source), fmt.Sprintf("\t%s = nil", target), "} else {"}
		for _, line := range g.rawResultDecodeLinesNamed(inner, declaredStackTypeName(declared.GoType), target, source, errReturn, temp+"Inner") {
			lines = append(lines, "\t"+line)
		}
		lines = append(lines, "}")
		return lines
	}
	g.useDecodeStackInt()
	typeID := temp + "TypeID"
	lines := []string{
		fmt.Sprintf("%s, err := decodeStackInt(%s[%d])", typeID, values, offset+*stackWidth-1),
		"if err != nil {",
		fmt.Sprintf("\treturn %s, err", errReturn),
		"}",
		fmt.Sprintf("if %s.Sign() == 0 {", typeID),
		fmt.Sprintf("\t%s = nil", target),
		fmt.Sprintf("} else if %s.Uint64() != %d {", typeID, *stackTypeID),
		fmt.Sprintf("\treturn %s, fmt.Errorf(\"unexpected nullable stack type id %%d\", %s.Uint64())", errReturn, typeID),
		"} else {",
	}
	for _, line := range g.stackDecodeLinesForDeclaredType(inner, declared, target, values, offset, errReturn, temp+"Inner") {
		lines = append(lines, "\t"+line)
	}
	lines = append(lines, "}")
	return lines
}

func declaredUsesCompoundStackType(typ abiType, declared typeInfo) bool {
	switch typ.Kind {
	case "StructRef", "tensor", "shapedTuple", "union":
		return true
	case "AliasRef":
		return true
	default:
		return false
	}
}

func declaredStackTypeName(goType string) string {
	return strings.TrimPrefix(goType, "*")
}

func assignDecodedDeclaredValue(target, value string, declared typeInfo) []string {
	if declared.Interface || strings.HasPrefix(declared.GoType, "*") || strings.HasPrefix(declared.GoType, "[]") || declared.GoType == "any" {
		return []string{fmt.Sprintf("%s = %s", target, value)}
	}
	return []string{fmt.Sprintf("%s = *%s", target, value)}
}

func directDecodeLines(target, call, errReturn string) []string {
	return []string{
		fmt.Sprintf("%s, err = %s", target, call),
		"if err != nil {",
		fmt.Sprintf("\treturn %s, err", errReturn),
		"}",
	}
}

func decodeLinesNeedErrVar(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(line, ", err = ") {
			return true
		}
	}
	return false
}

func (g *generator) rawResultDecodeLines(typ abiType, target, source, errReturn, temp string) []string {
	return g.rawResultDecodeLinesNamed(typ, "", target, source, errReturn, temp)
}

func (g *generator) rawResultDecodeLinesNamed(typ abiType, suggestedName, target, source, errReturn, temp string) []string {
	info := g.typeForResultNamed(typ, suggestedName)
	if !info.Supported {
		return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, info.Reason)}
	}
	if width, ok, _ := g.stackWidth(typ); ok && width > 1 {
		g.useDecodeStackTuple()
		tuple := temp + "Tuple"
		lines := []string{
			fmt.Sprintf("%s, err := decodeStackTuple(%s)", tuple, source),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
		lines = append(lines, g.stackDecodeLinesNamed(typ, suggestedName, target, tuple, 0, errReturn, temp)...)
		return lines
	}

	switch typ.Kind {
	case "void":
		return []string{fmt.Sprintf("%s = struct{}{}", target)}
	case "null", "nullLiteral":
		return []string{fmt.Sprintf("%s = nil", target)}
	case "int", "varuintN", "varintN":
		g.useDecodeStackInt()
		return directDecodeLines(target, fmt.Sprintf("decodeStackInt(%s)", source), errReturn)
	case "coins":
		g.useImport("github.com/xssnick/tonutils-go/tlb")
		g.useDecodeStackInt()
		return []string{
			fmt.Sprintf("%s, err := decodeStackInt(%s)", temp, source),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
			fmt.Sprintf("%s = tlb.FromNanoTON(%s)", target, temp),
		}
	case "uintN", "intN":
		g.useDecodeStackInt()
		raw := temp + "Int"
		lines := []string{
			fmt.Sprintf("%s, err := decodeStackInt(%s)", raw, source),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
		goType := intGoType(typ.N, typ.Kind == "intN")
		if goType == "*big.Int" {
			return directDecodeLines(target, fmt.Sprintf("decodeStackInt(%s)", source), errReturn)
		}
		read := "Uint64"
		if typ.Kind == "intN" {
			read = "Int64"
		}
		lines = append(lines, fmt.Sprintf("%s = %s(%s.%s())", target, goType, raw, read))
		return lines
	case "bool":
		g.useDecodeStackBool()
		return directDecodeLines(target, fmt.Sprintf("decodeStackBool(%s)", source), errReturn)
	case "string":
		g.useDecodeStackString()
		return directDecodeLines(target, fmt.Sprintf("decodeStackString(%s)", source), errReturn)
	case "address", "addressExt", "addressOpt", "addressAny":
		g.useDecodeStackAddress()
		return directDecodeLines(target, fmt.Sprintf("decodeStackAddress(%s)", source), errReturn)
	case "bitsN":
		g.useDecodeStackBits()
		return directDecodeLines(target, fmt.Sprintf("decodeStackBits(%s, %d)", source, typ.N), errReturn)
	case "bytesN":
		g.useDecodeStackBits()
		return directDecodeLines(target, fmt.Sprintf("decodeStackBits(%s, %d)", source, typ.N*8), errReturn)
	case "cell":
		g.useDecodeStackCell()
		return directDecodeLines(target, fmt.Sprintf("decodeStackCell(%s)", source), errReturn)
	case "cellOf":
		if typ.Inner == nil || typ.Inner.Kind == "slice" {
			g.useDecodeStackCell()
			return directDecodeLines(target, fmt.Sprintf("decodeStackCell(%s)", source), errReturn)
		}
		inner := g.typeForTLB(*typ.Inner, true)
		if !inner.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: cellOf inner type: %s.", target, inner.Reason)}
		}
		g.useImport("github.com/xssnick/tonutils-go/tlb")
		g.useDecodeStackCell()
		return []string{
			fmt.Sprintf("%s, err := decodeStackCell(%s)", temp+"Cell", source),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
			fmt.Sprintf("if err := tlb.Parse(&%s, %sCell); err != nil {", target, temp),
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
	case "slice", "remaining":
		g.useDecodeStackSlice()
		return directDecodeLines(target, fmt.Sprintf("decodeStackSlice(%s)", source), errReturn)
	case "builder":
		g.useDecodeStackBuilder()
		return directDecodeLines(target, fmt.Sprintf("decodeStackBuilder(%s)", source), errReturn)
	case "arrayOf":
		if typ.Inner != nil && typ.Inner.Kind == "unknown" {
			g.useDecodeStackTuple()
			return directDecodeLines(target, fmt.Sprintf("decodeStackTuple(%s)", source), errReturn)
		}
		if typ.Inner == nil {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: arrayOf without inner type.", target)}
		}
		lines := []string{
			fmt.Sprintf("%s, err := decodeStackTuple(%s)", temp+"Tuple", source),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
		g.useDecodeStackTuple()
		lines = append(lines, g.decodeStackArrayLines(*typ.Inner, target, temp+"Tuple", errReturn, temp)...)
		return lines
	case "lispListOf":
		if typ.Inner == nil {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: lispListOf without inner type.", target)}
		}
		return g.lispListDecodeLines(*typ.Inner, target, source, errReturn, temp)
	case "nullable":
		if typ.Inner == nil {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: nullable without inner type.", target)}
		}
		inner := g.typeForResult(*typ.Inner)
		if !inner.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, inner.Reason)}
		}
		lines := []string{fmt.Sprintf("if %s == nil {", source), fmt.Sprintf("\t%s = nil", target), "} else {"}
		if inner.Interface || isDirectlyNullableResultType(inner.GoType) {
			for _, line := range g.rawResultDecodeLines(*typ.Inner, target, source, errReturn, temp+"Inner") {
				lines = append(lines, "\t"+line)
			}
		} else {
			lines = append(lines, fmt.Sprintf("\tvar %s %s", temp+"Value", inner.GoType))
			for _, line := range g.rawResultDecodeLines(*typ.Inner, temp+"Value", source, errReturn, temp+"Inner") {
				lines = append(lines, "\t"+line)
			}
			lines = append(lines, fmt.Sprintf("\t%s = &%s", target, temp+"Value"))
		}
		lines = append(lines, "}")
		return lines
	case "AliasRef":
		decl, ok := g.aliases[typ.AliasName]
		if !ok {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: unknown alias %s.", target, typ.AliasName)}
		}
		if aliasDecodesDirectlyThroughTarget(decl.Target) {
			return g.rawResultDecodeLinesNamed(decl.Target, typ.AliasName, target, source, errReturn, temp)
		}
		base := g.typeForResult(decl.Target)
		if !base.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, base.Reason)}
		}
		aliasName := exportedName(typ.AliasName)
		lines := []string{}
		if rawResultDecodeNeedsDeclaredTarget(decl.Target) {
			lines = append(lines, fmt.Sprintf("var %s %s", temp+"Alias", base.GoType))
		}
		lines = append(lines, g.rawResultDecodeLines(decl.Target, temp+"Alias", source, errReturn, temp+"Base")...)
		lines = append(lines, fmt.Sprintf("%s = %s", target, aliasConversionExpr(aliasName, base, temp+"Alias")))
		return lines
	case "EnumRef":
		decl, ok := g.enums[typ.EnumName]
		if !ok || decl.EncodedAs == nil {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: unknown enum %s.", target, typ.EnumName)}
		}
		enumName := exportedName(typ.EnumName)
		lines := []string{fmt.Sprintf("var %s %s", temp+"Enum", intGoType(decl.EncodedAs.N, decl.EncodedAs.Kind == "intN"))}
		lines = append(lines, g.rawResultDecodeLines(*decl.EncodedAs, temp+"Enum", source, errReturn, temp+"Base")...)
		lines = append(lines, fmt.Sprintf("%s = %s(%s)", target, enumName, temp+"Enum"))
		return lines
	case "StructRef":
		name := exportedName(typ.StructName)
		g.useStackResultDecoder(typ.StructName)
		if width, ok, _ := g.stackWidth(typ); ok && width == 1 {
			return []string{
				fmt.Sprintf("%sValues := []any{%s}", temp, source),
				fmt.Sprintf("%s, err = decode%sResult(%sValues)", target, name, temp),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
			}
		}
		g.useDecodeStackTuple()
		return []string{
			fmt.Sprintf("%s, err := decodeStackTuple(%s)", temp+"Tuple", source),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
			fmt.Sprintf("%s, err = decode%sResult(%s)", target, name, temp+"Tuple"),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
	case "tensor", "shapedTuple":
		info := g.tupleTypeForStack(typ, suggestedName)
		if !info.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, info.Reason)}
		}
		name := strings.TrimPrefix(info.GoType, "*")
		g.useDecodeStackTuple()
		return []string{
			fmt.Sprintf("%s, err := decodeStackTuple(%s)", temp+"Tuple", source),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
			fmt.Sprintf("%s, err = decode%sStackTuple(%s)", target, name, temp+"Tuple"),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
	case "mapKV":
		info := g.resultMapType(typ)
		if !info.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, info.Reason)}
		}
		g.useImport("github.com/xssnick/tonutils-go/tlb")
		g.useDecodeStackCell()
		return []string{
			fmt.Sprintf("%s, err := decodeStackCell(%s)", temp+"Cell", source),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
			fmt.Sprintf("if err := tlb.Parse(&%s, %sCell); err != nil {", target, temp),
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
	case "union":
		info := g.unionTypeForResult(typ, suggestedName)
		if !info.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, info.Reason)}
		}
		name := strings.TrimPrefix(info.GoType, "*")
		g.useDecodeStackTuple()
		return []string{
			fmt.Sprintf("%s, err := decodeStackTuple(%s)", temp+"Tuple", source),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
			fmt.Sprintf("%s, err = decode%sStackUnion(%s)", target, name, temp+"Tuple"),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
		}
	case "unknown":
		return []string{fmt.Sprintf("%s = %s", target, source)}
	}

	return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, info.Reason)}
}

func rawResultDecodeNeedsDeclaredTarget(typ abiType) bool {
	switch typ.Kind {
	case "arrayOf":
		return typ.Inner == nil || typ.Inner.Kind == "unknown"
	case "lispListOf":
		return false
	default:
		return true
	}
}

func (g *generator) stackDecodeLines(typ abiType, target, values string, offset int, errReturn, temp string) []string {
	return g.stackDecodeLinesNamed(typ, "", target, values, offset, errReturn, temp)
}

func (g *generator) stackDecodeLinesNamed(typ abiType, suggestedName, target, values string, offset int, errReturn, temp string) []string {
	width, ok, reason := g.stackWidth(typ)
	if !ok {
		return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, reason)}
	}
	if width == 0 && typ.Kind != "StructRef" {
		return nil
	}

	source := fmt.Sprintf("%s[%d]", values, offset)
	switch typ.Kind {
	case "AliasRef":
		decl, ok := g.aliases[typ.AliasName]
		if !ok {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: unknown alias %s.", target, typ.AliasName)}
		}
		if aliasDecodesDirectlyThroughTarget(decl.Target) {
			return g.stackDecodeLinesNamed(decl.Target, typ.AliasName, target, values, offset, errReturn, temp)
		}
		base := g.typeForResult(decl.Target)
		if !base.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, base.Reason)}
		}
		aliasName := exportedName(typ.AliasName)
		baseTarget := temp + "Alias"
		lines := []string{}
		if rawResultDecodeNeedsDeclaredTarget(decl.Target) {
			lines = append(lines, fmt.Sprintf("var %s %s", baseTarget, base.GoType))
		}
		lines = append(lines, g.stackDecodeLines(decl.Target, baseTarget, values, offset, errReturn, temp+"Base")...)
		lines = append(lines, fmt.Sprintf("%s = %s", target, aliasConversionExpr(aliasName, base, baseTarget)))
		return lines
	case "EnumRef":
		decl, ok := g.enums[typ.EnumName]
		if !ok || decl.EncodedAs == nil {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: unknown enum %s.", target, typ.EnumName)}
		}
		enumName := exportedName(typ.EnumName)
		baseTarget := temp + "Enum"
		lines := []string{fmt.Sprintf("var %s %s", baseTarget, intGoType(decl.EncodedAs.N, decl.EncodedAs.Kind == "intN"))}
		lines = append(lines, g.stackDecodeLines(*decl.EncodedAs, baseTarget, values, offset, errReturn, temp+"Base")...)
		lines = append(lines, fmt.Sprintf("%s = %s(%s)", target, enumName, baseTarget))
		return lines
	case "StructRef":
		name := exportedName(typ.StructName)
		g.useStackResultDecoder(typ.StructName)
		return directDecodeLines(target, fmt.Sprintf("decode%sResult(%s[%d:%d])", name, values, offset, offset+width), errReturn)
	case "tensor":
		info := g.tupleTypeForStack(typ, suggestedName)
		if !info.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, info.Reason)}
		}
		name := strings.TrimPrefix(info.GoType, "*")
		return directDecodeLines(target, fmt.Sprintf("decode%sStackTuple(%s[%d:%d])", name, values, offset, offset+width), errReturn)
	case "union":
		info := g.unionTypeForResult(typ, suggestedName)
		if !info.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, info.Reason)}
		}
		name := strings.TrimPrefix(info.GoType, "*")
		return directDecodeLines(target, fmt.Sprintf("decode%sStackUnion(%s[%d:%d])", name, values, offset, offset+width), errReturn)
	case "nullable":
		if typ.Inner == nil {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: nullable without inner type.", target)}
		}
		if typ.StackWidth == nil || typ.StackTypeID == nil {
			return g.rawResultDecodeLines(typ, target, source, errReturn, temp)
		}
		inner := g.typeForResult(*typ.Inner)
		if !inner.Supported {
			return []string{fmt.Sprintf("// TODO: unsupported stack value %s: %s.", target, inner.Reason)}
		}
		g.useDecodeStackInt()
		typeID := temp + "TypeID"
		lines := []string{
			fmt.Sprintf("%s, err := decodeStackInt(%s[%d])", typeID, values, offset+width-1),
			"if err != nil {",
			fmt.Sprintf("\treturn %s, err", errReturn),
			"}",
			fmt.Sprintf("if %s.Sign() == 0 {", typeID),
			fmt.Sprintf("\t%s = nil", target),
			fmt.Sprintf("} else if %s.Uint64() != %d {", typeID, *typ.StackTypeID),
			fmt.Sprintf("\treturn %s, fmt.Errorf(\"unexpected nullable stack type id %%d\", %s.Uint64())", errReturn, typeID),
			"} else {",
		}
		if inner.Interface || isDirectlyNullableResultType(inner.GoType) {
			for _, line := range g.stackDecodeLinesNamed(*typ.Inner, suggestedName, target, values, offset, errReturn, temp+"Inner") {
				lines = append(lines, "\t"+line)
			}
		} else {
			valueTarget := temp + "Value"
			lines = append(lines, fmt.Sprintf("\tvar %s %s", valueTarget, inner.GoType))
			for _, line := range g.stackDecodeLinesNamed(*typ.Inner, suggestedName, valueTarget, values, offset, errReturn, temp+"Inner") {
				lines = append(lines, "\t"+line)
			}
			lines = append(lines, fmt.Sprintf("\t%s = &%s", target, valueTarget))
		}
		lines = append(lines, "}")
		return lines
	default:
		return g.rawResultDecodeLines(typ, target, source, errReturn, temp)
	}
}

func (g *generator) lispListDecodeLines(inner abiType, target, source, errReturn, temp string) []string {
	innerInfo := g.typeForResult(inner)
	if !innerInfo.Supported {
		return []string{fmt.Sprintf("// TODO: unsupported stack value %s: lispListOf inner type: %s.", target, innerInfo.Reason)}
	}

	g.useDecodeStackLispList()
	items := temp + "Items"
	raw := temp + "Raw"
	item := temp + "Item"
	decodeLines := g.rawResultDecodeLines(inner, item, raw, errReturn, item+"Value")
	rangeValue := raw
	if !linesReferenceIdentifier(decodeLines, raw) {
		rangeValue = "_"
	}
	lines := []string{
		fmt.Sprintf("%s, err := decodeStackLispList(%s)", items, source),
		"if err != nil {",
		fmt.Sprintf("\treturn %s, err", errReturn),
		"}",
		fmt.Sprintf("%s %s make([]%s, 0, len(%s))", target, assignOp(target), innerInfo.GoType, items),
	}
	if rangeValue == "_" {
		lines = append(lines, fmt.Sprintf("for range %s {", items))
	} else {
		lines = append(lines, fmt.Sprintf("for _, %s := range %s {", rangeValue, items))
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
