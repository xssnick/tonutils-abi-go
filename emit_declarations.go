package main

import (
	"bytes"
	"fmt"
	"strings"
)

func (g *generator) writeDeclarations(dst *bytes.Buffer) {
	for _, decl := range g.abi.Declarations {
		switch decl.Kind {
		case "alias":
			g.writeAlias(dst, decl)
		case "enum":
			g.writeEnum(dst, decl)
		case "struct":
			g.writeStruct(dst, decl)
		default:
			name := exportedName(decl.Name)
			if name == "" {
				name = decl.Name
			}
			g.writeTODO(dst, "", "declaration %s has unsupported kind %s.", name, decl.Kind)
			dst.WriteString("\n")
		}
	}
}

func (g *generator) writeAlias(dst *bytes.Buffer, decl declaration) {
	switch {
	case g.tlbAliases[decl.Name]:
		g.writeTLBAlias(dst, decl)
	case g.stackAliases[decl.Name]:
		g.writeStackAlias(dst, decl)
	}
}

func (g *generator) writeTLBAlias(dst *bytes.Buffer, decl declaration) {
	name := exportedName(decl.Name)
	if name == "" {
		return
	}
	if len(decl.TypeParams) > 0 {
		g.writeTODO(dst, "", "alias %s has generic parameters and is not generated yet.", name)
		dst.WriteString("\n")
		return
	}

	if decl.CustomPackUnpack != nil {
		if !customPackUnpackEnabled(decl.CustomPackUnpack) {
			g.writeTODO(dst, "", "alias %s is not generated yet: %s.", name, customPackUnpackReason(decl.CustomPackUnpack))
			dst.WriteString("\n")
			return
		}
		info := g.customValueTypeForTLB(decl.Target, decl.Name+"Value")
		if !info.Supported {
			g.writeTODO(dst, "", "alias %s is not generated yet: custom value type: %s.", name, info.Reason)
			dst.WriteString("\n")
			return
		}
		fmt.Fprintf(dst, "type %s struct {\n", name)
		fmt.Fprintf(dst, "\tValue %s `tlb:%q`\n", info.GoType, "-")
		dst.WriteString("}\n\n")
		g.writeCustomPackUnpackMethods(dst, name, decl.CustomPackUnpack)
		return
	}

	info := g.typeForTLBNamed(decl.Target, false, decl.Name)
	if !info.Supported {
		g.writeTODO(dst, "", "alias %s is not generated yet: %s.", name, info.Reason)
		dst.WriteString("\n")
		return
	}
	if info.Kind == "map" && decl.Target.Kind == "AliasRef" {
		fmt.Fprintf(dst, "type %s = %s\n\n", name, info.GoType)
		return
	}
	if info.Kind == "map" {
		return
	}
	if aliasShouldPreserveTargetType(info.Kind) {
		if info.GoType == name {
			return
		}
		fmt.Fprintf(dst, "type %s = %s\n\n", name, info.GoType)
		return
	}

	fmt.Fprintf(dst, "type %s %s\n\n", name, info.GoType)
}

func (g *generator) writeStackAlias(dst *bytes.Buffer, decl declaration) {
	name := exportedName(decl.Name)
	if name == "" {
		return
	}
	if len(decl.TypeParams) > 0 {
		g.writeTODO(dst, "", "alias %s has generic parameters and is not generated yet.", name)
		dst.WriteString("\n")
		return
	}

	info := g.typeForResultNamed(decl.Target, decl.Name)
	if !info.Supported {
		g.writeTODO(dst, "", "stack alias %s is not generated yet: %s.", name, info.Reason)
		dst.WriteString("\n")
		return
	}
	if (info.Kind == "tuple" || info.Kind == "tupleStruct" || info.Kind == "union") && strings.TrimPrefix(info.GoType, "*") == name {
		return
	}
	if info.Kind == "struct" && strings.HasPrefix(info.GoType, "*") {
		fmt.Fprintf(dst, "type %s = %s\n\n", name, strings.TrimPrefix(info.GoType, "*"))
		return
	}
	if decl.Target.Kind == "AliasRef" && aliasShouldPreserveTargetType(info.Kind) {
		fmt.Fprintf(dst, "type %s = %s\n\n", name, info.GoType)
		return
	}

	fmt.Fprintf(dst, "type %s %s\n\n", name, info.GoType)
}

func aliasShouldPreserveTargetType(kind string) bool {
	switch kind {
	case "map", "tensor", "tupleStruct", "union", "lispList", "struct":
		return true
	default:
		return false
	}
}

func (g *generator) writeEnum(dst *bytes.Buffer, decl declaration) {
	if !g.tlbEnums[decl.Name] && !g.stackEnums[decl.Name] {
		return
	}

	name := exportedName(decl.Name)
	if name == "" {
		return
	}
	if decl.EncodedAs == nil {
		g.writeTODO(dst, "", "enum %s has no encoded_as and is not generated yet.", name)
		dst.WriteString("\n")
		return
	}

	if decl.CustomPackUnpack != nil {
		if !customPackUnpackEnabled(decl.CustomPackUnpack) {
			g.writeTODO(dst, "", "enum %s is not generated yet: %s.", name, customPackUnpackReason(decl.CustomPackUnpack))
			dst.WriteString("\n")
			return
		}
		info := g.customValueTypeForTLB(*decl.EncodedAs, decl.Name+"Value")
		if !info.Supported {
			g.writeTODO(dst, "", "enum %s is not generated yet: custom value type: %s.", name, info.Reason)
			dst.WriteString("\n")
			return
		}
		fmt.Fprintf(dst, "type %s struct {\n", name)
		fmt.Fprintf(dst, "\tValue %s `tlb:%q`\n", info.GoType, "-")
		dst.WriteString("}\n\n")
		g.writeCustomPackUnpackMethods(dst, name, decl.CustomPackUnpack)
		if len(decl.Members) == 0 {
			return
		}
		fmt.Fprintf(dst, "var (\n")
		usedNames := map[string]bool{}
		for _, member := range decl.Members {
			memberName := exportedName(member.Name)
			if memberName == "" {
				continue
			}
			valueName := uniqueName(name+memberName, usedNames)
			if info.GoType == "*big.Int" {
				g.useHelper(helperBigIntLiteral)
			}
			fmt.Fprintf(dst, "\t%s = %s{Value: %s}\n", valueName, name, customEnumValueExpr(info.GoType, member.Value))
		}
		dst.WriteString(")\n\n")
		return
	}

	info := g.typeForTLB(*decl.EncodedAs, false)
	if !info.Supported || info.Kind == "bits" {
		g.writeTODO(dst, "", "enum %s is not generated yet: unsupported encoding %s.", name, decl.EncodedAs.Kind)
		dst.WriteString("\n")
		return
	}

	if info.GoType == "*big.Int" {
		g.useHelper(helperBigIntLiteral)
		fmt.Fprintf(dst, "type %s = *big.Int\n\n", name)
	} else {
		fmt.Fprintf(dst, "type %s %s\n\n", name, info.GoType)
	}
	if len(decl.Members) == 0 {
		return
	}

	blockKeyword := "const"
	if info.GoType == "*big.Int" {
		blockKeyword = "var"
	}
	fmt.Fprintf(dst, "%s (\n", blockKeyword)
	usedNames := map[string]bool{}
	for _, member := range decl.Members {
		memberName := exportedName(member.Name)
		if memberName == "" {
			continue
		}
		valueName := uniqueName(name+memberName, usedNames)
		if info.GoType == "*big.Int" {
			fmt.Fprintf(dst, "\t%s %s = tugenMustBigInt(%q)\n", valueName, name, member.Value)
			continue
		}
		fmt.Fprintf(dst, "\t%s %s = %s\n", valueName, name, member.Value)
	}
	dst.WriteString(")\n\n")
}

func (g *generator) writeStruct(dst *bytes.Buffer, decl declaration) {
	switch {
	case g.tlbStructs[decl.Name]:
		g.writeTLBStruct(dst, decl)
	case g.stackStructs[decl.Name]:
		g.writeStackStruct(dst, decl)
	}
}

func (g *generator) writeTLBStruct(dst *bytes.Buffer, decl declaration) {
	name := exportedName(decl.Name)
	if name == "" {
		return
	}
	if len(decl.TypeParams) > 0 {
		g.writeTODO(dst, "", "struct %s has generic parameters and is not generated yet.", name)
		dst.WriteString("\n")
		return
	}

	if decl.CustomPackUnpack != nil {
		if !customPackUnpackEnabled(decl.CustomPackUnpack) {
			g.writeTODO(dst, "", "struct %s is not generated yet: %s.", name, customPackUnpackReason(decl.CustomPackUnpack))
			dst.WriteString("\n")
			return
		}
		fmt.Fprintf(dst, "type %s struct {\n", name)
		dst.WriteString("\tValue any `tlb:\"-\"`\n")
		dst.WriteString("}\n\n")
		g.writeCustomPackUnpackMethods(dst, name, decl.CustomPackUnpack)
		return
	}

	fmt.Fprintf(dst, "type %s struct {\n", name)
	if decl.Prefix != nil {
		g.useImport("github.com/xssnick/tonutils-go/tlb")
		fmt.Fprintf(dst, "\t_ tlb.Magic `tlb:%q`\n", prefixTag(*decl.Prefix))
	}

	fieldNames := declarationFieldNames(decl.Fields)
	for i, fld := range decl.Fields {
		fieldName := fieldNames[i]

		info := g.typeForTLBNamed(fld.Type, true, name+fieldName)
		if !info.Supported {
			g.writeTODO(dst, "\t", "unsupported TLB field %s: %s.", fieldName, info.Reason)
			continue
		}
		if info.Kind == "remaining" && i != len(decl.Fields)-1 {
			g.writeTODO(dst, "\t", "unsupported TLB field %s: remaining must be the last field.", fieldName)
			continue
		}

		fmt.Fprintf(dst, "\t%s %s `tlb:%q`\n", fieldName, info.GoType, info.TLBTag)
	}
	dst.WriteString("}\n\n")
}

func (g *generator) writeCustomPackUnpackMethods(dst *bytes.Buffer, name string, custom *customPackUnpack) {
	g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
	loadHook := unexportedName(name) + "LoadFromCell"
	toCellHook := unexportedName(name) + "ToCell"
	loadSetter := "Set" + name + "LoadFromCell"
	toCellSetter := "Set" + name + "ToCell"
	g.addCustomSerializer(name, custom, loadSetter, toCellSetter)

	fmt.Fprintf(dst, "func (v *%s) LoadFromCell(loader *cell.Slice) error {\n", name)
	if custom != nil && custom.UnpackFromSlice {
		fmt.Fprintf(dst, "\treturn %s(v, loader)\n", loadHook)
	} else {
		fmt.Fprintf(dst, "\tpanic(%q)\n", name+" has no custom unpack_from_slice")
	}
	dst.WriteString("}\n\n")
	if custom != nil && custom.UnpackFromSlice {
		fmt.Fprintf(dst, "var %s = func(value *%s, loader *cell.Slice) error {\n", loadHook, name)
		fmt.Fprintf(dst, "\tpanic(%q)\n", loadSetter+" must be called before decoding "+name)
		dst.WriteString("}\n\n")
		fmt.Fprintf(dst, "func %s(fn func(value *%s, loader *cell.Slice) error) {\n", loadSetter, name)
		dst.WriteString("\tif fn == nil {\n")
		fmt.Fprintf(dst, "\t\tpanic(%q)\n", loadSetter+" got nil")
		dst.WriteString("\t}\n")
		fmt.Fprintf(dst, "\t%s = fn\n", loadHook)
		dst.WriteString("}\n\n")
	}

	fmt.Fprintf(dst, "func (v %s) ToCell() (*cell.Cell, error) {\n", name)
	if custom != nil && custom.PackToBuilder {
		fmt.Fprintf(dst, "\treturn %s(&v)\n", toCellHook)
	} else {
		fmt.Fprintf(dst, "\tpanic(%q)\n", name+" has no custom pack_to_builder")
	}
	dst.WriteString("}\n\n")
	if custom != nil && custom.PackToBuilder {
		fmt.Fprintf(dst, "var %s = func(value *%s) (*cell.Cell, error) {\n", toCellHook, name)
		fmt.Fprintf(dst, "\tpanic(%q)\n", toCellSetter+" must be called before encoding "+name)
		dst.WriteString("}\n\n")
		fmt.Fprintf(dst, "func %s(fn func(value *%s) (*cell.Cell, error)) {\n", toCellSetter, name)
		dst.WriteString("\tif fn == nil {\n")
		fmt.Fprintf(dst, "\t\tpanic(%q)\n", toCellSetter+" got nil")
		dst.WriteString("\t}\n")
		fmt.Fprintf(dst, "\t%s = fn\n", toCellHook)
		dst.WriteString("}\n\n")
	}
}

func (g *generator) addCustomSerializer(name string, custom *customPackUnpack, loadSetter, toCellSetter string) {
	if custom == nil {
		return
	}
	info := CustomSerializer{TypeName: name}
	if custom.UnpackFromSlice {
		info.LoadFromCellSetterName = loadSetter
	}
	if custom.PackToBuilder {
		info.ToCellSetterName = toCellSetter
	}
	g.customSerializers = append(g.customSerializers, info)
}

func (g *generator) writeStackStruct(dst *bytes.Buffer, decl declaration) {
	name := exportedName(decl.Name)
	if name == "" {
		return
	}
	if len(decl.TypeParams) > 0 {
		g.writeTODO(dst, "", "result struct %s has generic parameters and is not generated yet.", name)
		dst.WriteString("\n")
		return
	}

	fmt.Fprintf(dst, "type %s struct {\n", name)
	fieldNames := declarationFieldNames(decl.Fields)
	for i, fld := range decl.Fields {
		fieldName := fieldNames[i]

		info := g.typeForResultNamed(fld.Type, name+fieldName)
		if !info.Supported {
			g.writeTODO(dst, "\t", "unsupported stack field %s: %s.", fieldName, info.Reason)
			continue
		}

		fmt.Fprintf(dst, "\t%s %s\n", fieldName, info.GoType)
	}
	dst.WriteString("}\n\n")
}
