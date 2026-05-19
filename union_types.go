package main

import (
	"bytes"
	"fmt"
	"strings"
)

type unionVariantInfo struct {
	Index       int
	Name        string
	BoxName     string
	GoType      string
	TLBTag      string
	IsNull      bool
	StackTypeID int
	StackWidth  int
}

func (g *generator) unionTypeForTLB(typ abiType, suggestedName string) typeInfo {
	variants, ok := g.unionVariantsForTLB(typ, suggestedName)
	if !ok {
		return unsupported("union contains unsupported TLB variant")
	}
	name := g.ensureUnionType(typ, suggestedName, variants, true, false)
	if interfaceVariants, ok := g.unionInterfaceVariants(typ, suggestedName); ok {
		return typeInfo{
			GoType:    name,
			TLBTag:    unionAllowedListTag(interfaceVariants),
			Supported: true,
			Kind:      "union",
			Interface: true,
			Zero:      "nil",
		}
	}
	return typeInfo{
		GoType:    name,
		TLBTag:    ".",
		Supported: true,
		Kind:      "union",
		Zero:      name + "{}",
	}
}

func (g *generator) unionTypeForStack(typ abiType, suggestedName string) typeInfo {
	variants, ok := g.unionVariantsForStack(typ, suggestedName)
	if !ok {
		return unsupportedStack("union contains unsupported stack variant")
	}
	name := g.ensureUnionType(typ, suggestedName, variants, false, true)
	if _, ok := g.unionInterfaceVariants(typ, suggestedName); ok {
		return typeInfo{
			GoType:    name,
			Supported: true,
			Kind:      "union",
			Interface: true,
			StackExpr: func(value string) string {
				return fmt.Sprintf("stack%s(%s)", name, value)
			},
			Zero: "nil",
		}
	}
	return typeInfo{
		GoType:    "*" + name,
		Supported: true,
		Kind:      "union",
		StackExpr: func(value string) string {
			return fmt.Sprintf("stack%s(%s)", name, value)
		},
		Zero: "nil",
	}
}

func (g *generator) unionTypeForResult(typ abiType, suggestedName string) typeInfo {
	variants, ok := g.unionVariantsForStack(typ, suggestedName)
	if !ok {
		return unsupported("union contains unsupported stack variant")
	}
	name := g.ensureUnionType(typ, suggestedName, variants, false, true)
	if _, ok := g.unionInterfaceVariants(typ, suggestedName); ok {
		return typeInfo{
			GoType:    name,
			Supported: true,
			Kind:      "union",
			Interface: true,
			ResultDecode: func(target string, index uint, errReturn string) []string {
				return []string{
					fmt.Sprintf("tuple%d, err := result.Tuple(%d)", index, index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
					fmt.Sprintf("%s, err %s decode%sStackUnion(tuple%d)", target, assignOp(target), name, index),
					"if err != nil {",
					fmt.Sprintf("\treturn %s, err", errReturn),
					"}",
				}
			},
			Zero: "nil",
		}
	}
	return typeInfo{
		GoType:    "*" + name,
		Supported: true,
		Kind:      "union",
		ResultDecode: func(target string, index uint, errReturn string) []string {
			return []string{
				fmt.Sprintf("tuple%d, err := result.Tuple(%d)", index, index),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
				fmt.Sprintf("%s, err %s decode%sStackUnion(tuple%d)", target, assignOp(target), name, index),
				"if err != nil {",
				fmt.Sprintf("\treturn %s, err", errReturn),
				"}",
			}
		},
		Zero: "nil",
	}
}

func (g *generator) unionVariantsForTLB(typ abiType, suggestedName string) ([]unionVariantInfo, bool) {
	if len(typ.Items) == 0 {
		return nil, false
	}
	name := g.generatedName("Union", suggestedName, typ)
	var variants []unionVariantInfo
	usedSuffixes := map[string]bool{}
	for i, item := range typ.Items {
		suffix := unionVariantSuffix(item, i, usedSuffixes)
		variant := unionVariantInfo{
			Index:   i,
			Name:    name + suffix,
			BoxName: unexportedName(name+suffix) + "Box",
		}
		if isNullABIType(item) {
			variant.IsNull = true
			variants = append(variants, variant)
			continue
		}
		info := g.typeForTLB(item, true)
		if !info.Supported || info.TLBTag == "" {
			return nil, false
		}
		variant.GoType = info.GoType
		variant.TLBTag = info.TLBTag
		variants = append(variants, variant)
	}
	return variants, true
}

func (g *generator) unionVariantsForStack(typ abiType, suggestedName string) ([]unionVariantInfo, bool) {
	if len(typ.Items) == 0 {
		return nil, false
	}
	if typ.StackWidth == nil || len(typ.Variants) != len(typ.Items) {
		return nil, false
	}
	name := g.generatedName("Union", suggestedName, typ)
	var variants []unionVariantInfo
	usedSuffixes := map[string]bool{}
	for i, item := range typ.Items {
		abiVariant := typ.Variants[i]
		if abiVariant.StackTypeID == nil || abiVariant.StackWidth == nil {
			return nil, false
		}
		suffix := unionVariantSuffix(item, i, usedSuffixes)
		variant := unionVariantInfo{
			Index:       i,
			Name:        name + suffix,
			StackTypeID: *abiVariant.StackTypeID,
			StackWidth:  *abiVariant.StackWidth,
		}
		if isNullABIType(item) {
			variant.IsNull = true
			variants = append(variants, variant)
			continue
		}
		info := g.typeForResult(item)
		if !info.Supported {
			return nil, false
		}
		variant.GoType = info.GoType
		variants = append(variants, variant)
	}
	return variants, true
}

func (g *generator) unionInterfaceVariants(typ abiType, suggestedName string) ([]unionVariantInfo, bool) {
	if len(typ.Items) == 0 {
		return nil, false
	}
	var variants []unionVariantInfo
	for i, item := range typ.Items {
		if item.Kind != "StructRef" || len(item.TypeArgs) > 0 {
			return nil, false
		}
		decl, ok := g.structs[item.StructName]
		if !ok || decl.Prefix == nil {
			return nil, false
		}
		goType := exportedName(item.StructName)
		if goType == "" {
			return nil, false
		}
		variants = append(variants, unionVariantInfo{
			Index:  i,
			GoType: goType,
		})
	}
	return variants, true
}

func (g *generator) unionInterfaceStackVariants(typ abiType, suggestedName string) ([]unionVariantInfo, bool) {
	variants, ok := g.unionInterfaceVariants(typ, suggestedName)
	if !ok {
		return nil, false
	}
	if typ.StackWidth == nil || len(typ.Variants) != len(typ.Items) {
		return nil, false
	}
	name := g.generatedName("Union", suggestedName, typ)
	usedSuffixes := map[string]bool{}
	for i := range variants {
		abiVariant := typ.Variants[i]
		if abiVariant.StackTypeID == nil || abiVariant.StackWidth == nil {
			return nil, false
		}
		variants[i].Name = name + unionVariantSuffix(typ.Items[i], i, usedSuffixes)
		variants[i].StackTypeID = *abiVariant.StackTypeID
		variants[i].StackWidth = *abiVariant.StackWidth
	}
	return variants, true
}

func (g *generator) ensureUnionType(typ abiType, suggestedName string, variants []unionVariantInfo, includeTLB, includeStack bool) string {
	name := g.generatedName("Union", suggestedName, typ)
	if g.generatedTypeSet[name] {
		return name
	}

	tlbVariants, tlbOK := g.unionVariantsForTLB(typ, suggestedName)
	stackVariants, stackOK := g.unionVariantsForStack(typ, suggestedName)
	includeTLB = includeTLB || tlbOK
	includeStack = includeStack || stackOK
	if includeTLB {
		variants = tlbVariants
	} else if includeStack {
		variants = stackVariants
	}

	var b bytes.Buffer
	if interfaceVariants, ok := g.unionInterfaceVariants(typ, suggestedName); ok {
		g.writeInterfaceUnionType(&b, name, interfaceVariants, includeTLB)
		if includeStack && stackOK {
			for _, item := range typ.Items {
				g.markStackType(item, map[string]bool{})
			}
			if stackInterfaceVariants, ok := g.unionInterfaceStackVariants(typ, suggestedName); ok {
				g.writeInterfaceStackUnionMethods(&b, name, stackInterfaceVariants, typ.Items)
			}
		}
		g.addGeneratedType(name, b.String())
		return name
	}

	g.useImport("fmt")
	fmt.Fprintf(&b, "type %sVariant uint8\n\n", name)
	fmt.Fprintf(&b, "const (\n")
	for _, variant := range variants {
		fmt.Fprintf(&b, "\t%s %sVariant = %d\n", variant.Name, name, variant.Index)
	}
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "type %s struct {\n", name)
	fmt.Fprintf(&b, "\tVariant %sVariant\n", name)
	b.WriteString("\tValue any\n")
	b.WriteString("}\n\n")

	for _, variant := range variants {
		constructor := g.unionConstructorName("New"+variant.Name, "New"+name)
		if variant.IsNull {
			fmt.Fprintf(&b, "func %s() *%s {\n", constructor, name)
			fmt.Fprintf(&b, "\treturn &%s{Variant: %s}\n", name, variant.Name)
			b.WriteString("}\n\n")
			continue
		}
		fmt.Fprintf(&b, "func %s(value %s) *%s {\n", constructor, variant.GoType, name)
		fmt.Fprintf(&b, "\treturn &%s{Variant: %s, Value: value}\n", name, variant.Name)
		b.WriteString("}\n\n")
	}
	writeUnionAccessors(&b, name, variants)

	if includeTLB && tlbOK {
		g.writeTLBUnionMethods(&b, name, tlbVariants)
	}
	if includeStack && stackOK {
		for _, item := range typ.Items {
			g.markStackType(item, map[string]bool{})
		}
		g.writeStackUnionMethods(&b, name, stackVariants, typ.Items)
	}

	g.addGeneratedType(name, b.String())
	return name
}

func (g *generator) writeInterfaceUnionType(dst *bytes.Buffer, name string, variants []unionVariantInfo, includeTLB bool) {
	method := unionMarkerMethod(name)
	fmt.Fprintf(dst, "type %s interface {\n", name)
	fmt.Fprintf(dst, "\t%s()\n", method)
	dst.WriteString("}\n\n")

	for _, variant := range variants {
		fmt.Fprintf(dst, "func (%s) %s() {}\n\n", variant.GoType, method)
	}

	for _, variant := range variants {
		constructor := g.unionConstructorName(unionConstructorName(name, variant.GoType), "New"+name)
		fmt.Fprintf(dst, "func %s(value %s) %s {\n", constructor, variant.GoType, name)
		dst.WriteString("\treturn value\n")
		dst.WriteString("}\n\n")
	}

	if !includeTLB {
		return
	}
	g.useImport("github.com/xssnick/tonutils-go/tlb")
	dst.WriteString("func init() {\n")
	for _, variant := range variants {
		fmt.Fprintf(dst, "\ttlb.Register(%s{})\n", variant.GoType)
	}
	dst.WriteString("}\n\n")
}

func (g *generator) writeInterfaceStackUnionMethods(dst *bytes.Buffer, name string, variants []unionVariantInfo, items []abiType) {
	g.useImport("fmt")
	g.useDecodeStackInt()
	stackWidth := 0
	for _, variant := range variants {
		if variant.StackWidth+1 > stackWidth {
			stackWidth = variant.StackWidth + 1
		}
	}
	fmt.Fprintf(dst, "func stack%s(union %s) []any {\n", name, name)
	dst.WriteString("\tif union == nil {\n")
	dst.WriteString("\t\treturn nil\n")
	dst.WriteString("\t}\n")
	dst.WriteString("\tswitch value := union.(type) {\n")
	for _, variant := range variants {
		info := g.typeForStack(items[variant.Index])
		fmt.Fprintf(dst, "\tcase %s:\n", variant.GoType)
		fmt.Fprintf(dst, "\t\tout := make([]any, 0, %d)\n", stackWidth)
		for i := 0; i < stackWidth-1-variant.StackWidth; i++ {
			dst.WriteString("\t\tout = append(out, nil)\n")
		}
		if info.Supported {
			for _, line := range g.appendStackValueLines(items[variant.Index], "out", "&value") {
				fmt.Fprintf(dst, "\t\t%s\n", line)
			}
		} else {
			dst.WriteString("\t\tout = append(out, value)\n")
		}
		fmt.Fprintf(dst, "\t\tout = append(out, int64(%d))\n", variant.StackTypeID)
		dst.WriteString("\t\treturn out\n")
		fmt.Fprintf(dst, "\tcase *%s:\n", variant.GoType)
		dst.WriteString("\t\tif value == nil {\n")
		dst.WriteString("\t\t\treturn nil\n")
		dst.WriteString("\t\t}\n")
		fmt.Fprintf(dst, "\t\tout := make([]any, 0, %d)\n", stackWidth)
		for i := 0; i < stackWidth-1-variant.StackWidth; i++ {
			dst.WriteString("\t\tout = append(out, nil)\n")
		}
		if info.Supported {
			for _, line := range g.appendStackValueLines(items[variant.Index], "out", "value") {
				fmt.Fprintf(dst, "\t\t%s\n", line)
			}
		} else {
			dst.WriteString("\t\tout = append(out, value)\n")
		}
		fmt.Fprintf(dst, "\t\tout = append(out, int64(%d))\n", variant.StackTypeID)
		dst.WriteString("\t\treturn out\n")
	}
	dst.WriteString("\tdefault:\n")
	dst.WriteString("\t\treturn nil\n")
	dst.WriteString("\t}\n")
	dst.WriteString("}\n\n")

	fmt.Fprintf(dst, "func decode%sStackUnion(values []any) (%s, error) {\n", name, name)
	fmt.Fprintf(dst, "\tif len(values) < %d {\n", stackWidth)
	fmt.Fprintf(dst, "\t\treturn nil, fmt.Errorf(\"%s stack union expects %d items, got %%d\", len(values))\n", name, stackWidth)
	dst.WriteString("\t}\n")
	fmt.Fprintf(dst, "\trawVariant, err := decodeStackInt(values[%d])\n", stackWidth-1)
	dst.WriteString("\tif err != nil {\n")
	dst.WriteString("\t\treturn nil, err\n")
	dst.WriteString("\t}\n")
	dst.WriteString("\tswitch rawVariant.Uint64() {\n")
	for _, variant := range variants {
		fmt.Fprintf(dst, "\tcase %d:\n", variant.StackTypeID)
		if variant.IsNull {
			fmt.Fprintf(dst, "\t\treturn %s{}, nil\n", variant.GoType)
			continue
		}
		fmt.Fprintf(dst, "\t\tvar value *%s\n", variant.GoType)
		start := stackWidth - 1 - variant.StackWidth
		for _, line := range g.stackDecodeLines(items[variant.Index], "value", "values", start, "nil", unexportedName(variant.GoType)+"Value") {
			fmt.Fprintf(dst, "\t\t%s\n", line)
		}
		dst.WriteString("\t\treturn value, nil\n")
	}
	dst.WriteString("\tdefault:\n")
	fmt.Fprintf(dst, "\t\treturn nil, fmt.Errorf(\"unknown %s stack union variant %%d\", rawVariant.Uint64())\n", name)
	dst.WriteString("\t}\n")
	dst.WriteString("}\n\n")
}

func (g *generator) writeTLBUnionMethods(dst *bytes.Buffer, name string, variants []unionVariantInfo) {
	g.useImport("github.com/xssnick/tonutils-go/tlb")
	g.useImport("github.com/xssnick/tonutils-go/tvm/cell")
	prefixBits := unionPrefixBits(len(variants))

	for _, variant := range variants {
		if variant.IsNull {
			continue
		}
		fmt.Fprintf(dst, "type %s struct {\n", variant.BoxName)
		fmt.Fprintf(dst, "\tValue %s `tlb:%q`\n", variant.GoType, variant.TLBTag)
		dst.WriteString("}\n\n")
	}

	fmt.Fprintf(dst, "func (u *%s) LoadFromCell(loader *cell.Slice) error {\n", name)
	if prefixBits > 0 {
		fmt.Fprintf(dst, "\tprefix, err := loader.LoadUInt(%d)\n", prefixBits)
		dst.WriteString("\tif err != nil {\n")
		dst.WriteString("\t\treturn err\n")
		dst.WriteString("\t}\n")
	} else {
		dst.WriteString("\tprefix := uint64(0)\n")
	}
	dst.WriteString("\tswitch prefix {\n")
	for _, variant := range variants {
		fmt.Fprintf(dst, "\tcase %d:\n", variant.Index)
		if variant.IsNull {
			fmt.Fprintf(dst, "\t\tu.Variant = %s\n", variant.Name)
			dst.WriteString("\t\tu.Value = nil\n")
			dst.WriteString("\t\treturn nil\n")
			continue
		}
		fmt.Fprintf(dst, "\t\tvar box %s\n", variant.BoxName)
		dst.WriteString("\t\tif err := tlb.LoadFromCell(&box, loader); err != nil {\n")
		dst.WriteString("\t\t\treturn err\n")
		dst.WriteString("\t\t}\n")
		fmt.Fprintf(dst, "\t\tu.Variant = %s\n", variant.Name)
		dst.WriteString("\t\tu.Value = box.Value\n")
		dst.WriteString("\t\treturn nil\n")
	}
	dst.WriteString("\tdefault:\n")
	fmt.Fprintf(dst, "\t\treturn fmt.Errorf(\"unknown %s union prefix %%d\", prefix)\n", name)
	dst.WriteString("\t}\n")
	dst.WriteString("}\n\n")

	fmt.Fprintf(dst, "func (u %s) ToCell() (*cell.Cell, error) {\n", name)
	dst.WriteString("\tb := cell.BeginCell()\n")
	dst.WriteString("\tswitch u.Variant {\n")
	for _, variant := range variants {
		fmt.Fprintf(dst, "\tcase %s:\n", variant.Name)
		if prefixBits > 0 {
			fmt.Fprintf(dst, "\t\tif err := b.StoreUInt(%d, %d); err != nil {\n", variant.Index, prefixBits)
			dst.WriteString("\t\t\treturn nil, err\n")
			dst.WriteString("\t\t}\n")
		}
		if variant.IsNull {
			dst.WriteString("\t\treturn b.EndCell(), nil\n")
			continue
		}
		writeUnionValueCast(dst, variant.GoType)
		fmt.Fprintf(dst, "\t\tc, err := tlb.ToCell(%s{Value: value})\n", variant.BoxName)
		dst.WriteString("\t\tif err != nil {\n")
		dst.WriteString("\t\t\treturn nil, err\n")
		dst.WriteString("\t\t}\n")
		dst.WriteString("\t\tif err := b.StoreBuilder(c.ToBuilder()); err != nil {\n")
		dst.WriteString("\t\t\treturn nil, err\n")
		dst.WriteString("\t\t}\n")
		dst.WriteString("\t\treturn b.EndCell(), nil\n")
	}
	dst.WriteString("\tdefault:\n")
	fmt.Fprintf(dst, "\t\treturn nil, fmt.Errorf(\"unknown %s union variant %%d\", u.Variant)\n", name)
	dst.WriteString("\t}\n")
	dst.WriteString("}\n\n")
}

func (g *generator) writeStackUnionMethods(dst *bytes.Buffer, name string, variants []unionVariantInfo, items []abiType) {
	g.useImport("fmt")
	g.useDecodeStackInt()
	stackWidth := 0
	for _, variant := range variants {
		if variant.StackWidth+1 > stackWidth {
			stackWidth = variant.StackWidth + 1
		}
	}
	fmt.Fprintf(dst, "func stack%s(union *%s) []any {\n", name, name)
	dst.WriteString("\tif union == nil {\n")
	dst.WriteString("\t\treturn nil\n")
	dst.WriteString("\t}\n")
	dst.WriteString("\tswitch union.Variant {\n")
	for _, variant := range variants {
		fmt.Fprintf(dst, "\tcase %s:\n", variant.Name)
		fmt.Fprintf(dst, "\t\tout := make([]any, 0, %d)\n", stackWidth)
		for i := 0; i < stackWidth-1-variant.StackWidth; i++ {
			dst.WriteString("\t\tout = append(out, nil)\n")
		}
		if variant.IsNull {
			fmt.Fprintf(dst, "\t\tout = append(out, int64(%d))\n", variant.StackTypeID)
			dst.WriteString("\t\treturn out\n")
			continue
		}
		writeUnionStackValueCast(dst, variant.GoType)
		info := g.typeForStack(items[variant.Index])
		if info.Supported {
			for _, line := range g.appendStackValueLines(items[variant.Index], "out", "value") {
				fmt.Fprintf(dst, "\t\t%s\n", line)
			}
		} else {
			dst.WriteString("\t\tout = append(out, value)\n")
		}
		fmt.Fprintf(dst, "\t\tout = append(out, int64(%d))\n", variant.StackTypeID)
		dst.WriteString("\t\treturn out\n")
	}
	dst.WriteString("\tdefault:\n")
	dst.WriteString("\t\treturn nil\n")
	dst.WriteString("\t}\n")
	dst.WriteString("}\n\n")

	fmt.Fprintf(dst, "func decode%sStackUnion(values []any) (*%s, error) {\n", name, name)
	fmt.Fprintf(dst, "\tif len(values) < %d {\n", stackWidth)
	fmt.Fprintf(dst, "\t\treturn nil, fmt.Errorf(\"%s stack union expects %d items, got %%d\", len(values))\n", name, stackWidth)
	dst.WriteString("\t}\n")
	fmt.Fprintf(dst, "\trawVariant, err := decodeStackInt(values[%d])\n", stackWidth-1)
	dst.WriteString("\tif err != nil {\n")
	dst.WriteString("\t\treturn nil, err\n")
	dst.WriteString("\t}\n")
	fmt.Fprintf(dst, "\tout := &%s{}\n", name)
	dst.WriteString("\tswitch rawVariant.Uint64() {\n")
	for _, variant := range variants {
		fmt.Fprintf(dst, "\tcase %d:\n", variant.StackTypeID)
		fmt.Fprintf(dst, "\t\tout.Variant = %s\n", variant.Name)
		if variant.IsNull {
			dst.WriteString("\t\tout.Value = nil\n")
			dst.WriteString("\t\treturn out, nil\n")
			continue
		}
		fmt.Fprintf(dst, "\t\tvar value %s\n", variant.GoType)
		start := stackWidth - 1 - variant.StackWidth
		for _, line := range g.stackDecodeLines(items[variant.Index], "value", "values", start, "nil", unexportedName(variant.Name)+"Value") {
			fmt.Fprintf(dst, "\t\t%s\n", line)
		}
		dst.WriteString("\t\tout.Value = value\n")
		dst.WriteString("\t\treturn out, nil\n")
	}
	dst.WriteString("\tdefault:\n")
	fmt.Fprintf(dst, "\t\treturn nil, fmt.Errorf(\"unknown %s stack union type id %%d\", rawVariant.Uint64())\n", name)
	dst.WriteString("\t}\n")
	dst.WriteString("}\n\n")
}

func writeUnionValueCast(dst *bytes.Buffer, goType string) {
	if strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") || goType == "any" {
		fmt.Fprintf(dst, "\t\tvalue, ok := u.Value.(%s)\n", goType)
		dst.WriteString("\t\tif !ok {\n")
		fmt.Fprintf(dst, "\t\t\treturn nil, fmt.Errorf(\"union value has type %%T, want %s\", u.Value)\n", goType)
		dst.WriteString("\t\t}\n")
		return
	}
	fmt.Fprintf(dst, "\t\tvalue, ok := u.Value.(%s)\n", goType)
	dst.WriteString("\t\tif !ok {\n")
	fmt.Fprintf(dst, "\t\t\tif ptr, ok := u.Value.(*%s); ok && ptr != nil {\n", goType)
	dst.WriteString("\t\t\t\tvalue = *ptr\n")
	dst.WriteString("\t\t\t} else {\n")
	fmt.Fprintf(dst, "\t\t\t\treturn nil, fmt.Errorf(\"union value has type %%T, want %s\", u.Value)\n", goType)
	dst.WriteString("\t\t\t}\n")
	dst.WriteString("\t\t}\n")
}

func writeUnionStackValueCast(dst *bytes.Buffer, goType string) {
	if strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") || goType == "any" {
		fmt.Fprintf(dst, "\t\tvalue, ok := union.Value.(%s)\n", goType)
		dst.WriteString("\t\tif !ok {\n")
		if stackPointerTypeCanUseValue(goType) {
			fmt.Fprintf(dst, "\t\t\tif raw, ok := union.Value.(%s); ok {\n", strings.TrimPrefix(goType, "*"))
			dst.WriteString("\t\t\t\tvalue = &raw\n")
			dst.WriteString("\t\t\t} else {\n")
			dst.WriteString("\t\t\t\treturn nil\n")
			dst.WriteString("\t\t\t}\n")
		} else {
			dst.WriteString("\t\t\treturn nil\n")
		}
		dst.WriteString("\t\t}\n")
		return
	}
	fmt.Fprintf(dst, "\t\tvalue, ok := union.Value.(%s)\n", goType)
	dst.WriteString("\t\tif !ok {\n")
	fmt.Fprintf(dst, "\t\t\tif ptr, ok := union.Value.(*%s); ok && ptr != nil {\n", goType)
	dst.WriteString("\t\t\t\tvalue = *ptr\n")
	dst.WriteString("\t\t\t} else {\n")
	dst.WriteString("\t\t\t\treturn nil\n")
	dst.WriteString("\t\t\t}\n")
	dst.WriteString("\t\t}\n")
}

func writeUnionAccessors(dst *bytes.Buffer, name string, variants []unionVariantInfo) {
	for _, variant := range variants {
		suffix := strings.TrimPrefix(variant.Name, name)
		if suffix == "" {
			suffix = "Variant"
		}
		fmt.Fprintf(dst, "func (u *%s) Is%s() bool {\n", name, suffix)
		fmt.Fprintf(dst, "\treturn u != nil && u.Variant == %s\n", variant.Name)
		dst.WriteString("}\n\n")
		if variant.IsNull {
			continue
		}
		fmt.Fprintf(dst, "func (u *%s) As%s() (%s, bool) {\n", name, suffix, variant.GoType)
		fmt.Fprintf(dst, "\tvar zero %s\n", variant.GoType)
		dst.WriteString("\tif u == nil || !u.Is" + suffix + "() {\n")
		dst.WriteString("\t\treturn zero, false\n")
		dst.WriteString("\t}\n")
		writeUnionAccessorCast(dst, variant.GoType)
		dst.WriteString("}\n\n")
	}
}

func writeUnionAccessorCast(dst *bytes.Buffer, goType string) {
	fmt.Fprintf(dst, "\tvalue, ok := u.Value.(%s)\n", goType)
	dst.WriteString("\tif ok {\n")
	dst.WriteString("\t\treturn value, true\n")
	dst.WriteString("\t}\n")
	if strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") || goType == "any" {
		dst.WriteString("\treturn zero, false\n")
		return
	}
	fmt.Fprintf(dst, "\tif ptr, ok := u.Value.(*%s); ok && ptr != nil {\n", goType)
	dst.WriteString("\t\treturn *ptr, true\n")
	dst.WriteString("\t}\n")
	dst.WriteString("\treturn zero, false\n")
}

func stackPointerTypeCanUseValue(goType string) bool {
	if !strings.HasPrefix(goType, "*") {
		return false
	}
	switch goType {
	case "*address.Address", "*big.Int", "*cell.Cell", "*cell.Slice", "*cell.Builder":
		return false
	}
	return true
}

func unionPrefixBits(n int) uint {
	if n <= 1 {
		return 0
	}
	bits := uint(0)
	for x := n - 1; x > 0; x >>= 1 {
		bits++
	}
	return bits
}

func unionAllowedListTag(variants []unionVariantInfo) string {
	types := make([]string, 0, len(variants))
	for _, variant := range variants {
		types = append(types, variant.GoType)
	}
	return "[" + strings.Join(types, ",") + "]"
}

func unionMarkerMethod(name string) string {
	return "is" + name
}

func (g *generator) unionConstructorName(name, fallback string) string {
	if g.names == nil {
		return exportedName(name)
	}
	return g.names.uniquePackage(name, fallback)
}

func unionConstructorName(unionName, variantGoType string) string {
	suffix := strings.TrimPrefix(variantGoType, unionName)
	if suffix == "" || suffix == variantGoType {
		suffix = variantGoType
		unionParts := splitName(unionName)
		variantParts := splitName(variantGoType)
		i := 0
		for i < len(unionParts) && i < len(variantParts) && strings.EqualFold(unionParts[i], variantParts[i]) {
			i++
		}
		if i > 0 && i < len(variantParts) {
			suffix = strings.Join(variantParts[i:], "_")
		}
	}
	return "New" + unionName + exportedName(suffix)
}

func unionVariantSuffix(item abiType, index int, used map[string]bool) string {
	name := semanticNameForType(item)
	switch name {
	case "", "Value", "Choice":
		name = fmt.Sprintf("Variant%d", index+1)
	}
	return uniqueExportedName(name, fmt.Sprintf("Variant%d", index+1), used)
}
