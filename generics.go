package main

import (
	"fmt"
	"hash/fnv"
	"strings"
)

type genericEnv map[string]abiType

func (g *generator) monomorphizeGenerics() {
	if len(g.abi.Declarations) == 0 {
		return
	}

	if g.abi.Storage != nil {
		g.abi.Storage.StorageType = g.normalizeGenericType(g.abi.Storage.StorageType, nil)
	}
	for i := range g.abi.IncomingMessages {
		g.abi.IncomingMessages[i].BodyType = g.normalizeGenericType(g.abi.IncomingMessages[i].BodyType, nil)
	}
	for i := range g.abi.IncomingExternal {
		g.abi.IncomingExternal[i].BodyType = g.normalizeGenericType(g.abi.IncomingExternal[i].BodyType, nil)
	}
	for i := range g.abi.OutgoingMessages {
		g.abi.OutgoingMessages[i].BodyType = g.normalizeGenericType(g.abi.OutgoingMessages[i].BodyType, nil)
	}
	for i := range g.abi.EmittedEvents {
		g.abi.EmittedEvents[i].BodyType = g.normalizeGenericType(g.abi.EmittedEvents[i].BodyType, nil)
		g.abi.EmittedEvents[i].EventType = g.normalizeGenericType(g.abi.EmittedEvents[i].EventType, nil)
	}
	for i := range g.abi.GetMethods {
		for j := range g.abi.GetMethods[i].Parameters {
			g.abi.GetMethods[i].Parameters[j].Type = g.normalizeGenericType(g.abi.GetMethods[i].Parameters[j].Type, nil)
		}
		g.abi.GetMethods[i].ReturnType = g.normalizeGenericType(g.abi.GetMethods[i].ReturnType, nil)
	}

	for i := 0; i < len(g.abi.Declarations); i++ {
		decl := g.abi.Declarations[i]
		if len(decl.TypeParams) > 0 {
			continue
		}
		g.normalizeGenericDeclaration(&decl, nil)
		g.abi.Declarations[i] = decl
	}

	g.rebuildDeclarationMaps()
}

func (g *generator) normalizeGenericDeclaration(decl *declaration, env genericEnv) {
	switch decl.Kind {
	case "alias":
		decl.Target = g.normalizeGenericType(decl.Target, env)
		decl.TargetTypeIndex = nil
	case "enum":
		if decl.EncodedAs != nil {
			encodedAs := g.normalizeGenericType(*decl.EncodedAs, env)
			decl.EncodedAs = &encodedAs
			decl.EncodedAsIndex = nil
		}
	case "struct":
		for i := range decl.Fields {
			decl.Fields[i].Type = g.normalizeGenericType(decl.Fields[i].Type, env)
			decl.Fields[i].TypeIndex = nil
		}
	}
	decl.TypeIndex = nil
}

func (g *generator) normalizeGenericType(typ abiType, env genericEnv) abiType {
	typ.TypeArgs = append([]abiType(nil), typ.TypeArgs...)
	typ.Items = append([]abiType(nil), typ.Items...)
	typ.Variants = append([]abiTypeVariant(nil), typ.Variants...)

	switch typ.Kind {
	case "":
		return typ
	case "genericT":
		if env != nil {
			if bound, ok := env[typ.NameT]; ok {
				return g.normalizeGenericType(bound, env)
			}
		}
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: "generic type " + typ.NameT + " is not bound",
		})
		return typ
	}

	if len(typ.TypeArgs) > 0 {
		for i := range typ.TypeArgs {
			typ.TypeArgs[i] = g.normalizeGenericType(typ.TypeArgs[i], env)
		}
		typ.TypeArgIndices = nil
	}

	switch typ.Kind {
	case "AliasRef":
		if len(typ.TypeArgs) > 0 {
			if typ.Index != nil {
				if inst, ok := g.aliasInstantiations[*typ.Index]; ok {
					return abiType{Kind: "AliasRef", AliasName: g.instantiateCompilerGenericAlias(typ.AliasName, typ.TypeArgs, inst)}
				}
			}
			return abiType{Kind: "AliasRef", AliasName: g.instantiateGenericAlias(typ.AliasName, typ.TypeArgs)}
		}
	case "StructRef":
		if len(typ.TypeArgs) > 0 {
			if typ.Index != nil {
				if inst, ok := g.structInstantiations[*typ.Index]; ok {
					return abiType{Kind: "StructRef", StructName: g.instantiateCompilerGenericStruct(typ.StructName, typ.TypeArgs, inst)}
				}
			}
			return abiType{Kind: "StructRef", StructName: g.instantiateGenericStruct(typ.StructName, typ.TypeArgs)}
		}
	}

	if typ.Inner != nil {
		inner := g.normalizeGenericType(*typ.Inner, env)
		typ.Inner = &inner
		typ.InnerTypeIndex = nil
	}
	if typ.Key != nil {
		key := g.normalizeGenericType(*typ.Key, env)
		typ.Key = &key
		typ.KeyTypeIndex = nil
	}
	if typ.Value != nil {
		value := g.normalizeGenericType(*typ.Value, env)
		typ.Value = &value
		typ.ValueTypeIndex = nil
	}
	for i := range typ.Items {
		typ.Items[i] = g.normalizeGenericType(typ.Items[i], env)
	}
	typ.ItemTypeIndices = nil
	for i := range typ.Variants {
		typ.Variants[i].Type = g.normalizeGenericType(typ.Variants[i].Type, env)
		typ.Variants[i].TypeIndex = nil
	}
	return typ
}

func (g *generator) instantiateGenericAlias(name string, args []abiType) string {
	decl, ok := g.aliases[name]
	if !ok {
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: "unknown generic alias " + name,
		})
		return name
	}
	if len(decl.TypeParams) == 0 {
		return name
	}
	if len(decl.TypeParams) != len(args) {
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: fmt.Sprintf("generic alias %s expects %d type arguments, got %d", name, len(decl.TypeParams), len(args)),
		})
		return name
	}

	key := "alias:" + name + ":" + mapTypeSignature(abiType{TypeArgs: args})
	concreteName := g.genericInstanceName(key, name, args)
	if g.genericInProgress[key] {
		return concreteName
	}
	if _, exists := g.aliases[concreteName]; exists {
		return concreteName
	}

	g.genericInProgress[key] = true
	concrete := decl
	concrete.Name = concreteName
	concrete.TypeParams = nil
	concrete.TypeIndex = nil
	concrete.Target = g.normalizeGenericType(decl.Target, bindGenericArgs(decl.TypeParams, args))
	concrete.TargetTypeIndex = nil
	g.abi.Declarations = append(g.abi.Declarations, concrete)
	g.aliases[concreteName] = concrete
	delete(g.genericInProgress, key)
	return concreteName
}

func (g *generator) instantiateGenericStruct(name string, args []abiType) string {
	decl, ok := g.structs[name]
	if !ok {
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: "unknown generic struct " + name,
		})
		return name
	}
	if len(decl.TypeParams) == 0 {
		return name
	}
	if len(decl.TypeParams) != len(args) {
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: fmt.Sprintf("generic struct %s expects %d type arguments, got %d", name, len(decl.TypeParams), len(args)),
		})
		return name
	}

	key := "struct:" + name + ":" + mapTypeSignature(abiType{TypeArgs: args})
	concreteName := g.genericInstanceName(key, name, args)
	if g.genericInProgress[key] {
		return concreteName
	}
	if _, exists := g.structs[concreteName]; exists {
		return concreteName
	}

	genericEnv := bindGenericArgs(decl.TypeParams, args)
	g.genericInProgress[key] = true
	concrete := decl
	concrete.Name = concreteName
	concrete.TypeParams = nil
	concrete.TypeIndex = nil
	concrete.Fields = append([]field(nil), decl.Fields...)
	for i := range concrete.Fields {
		concrete.Fields[i].Type = g.normalizeGenericType(concrete.Fields[i].Type, genericEnv)
		concrete.Fields[i].TypeIndex = nil
	}
	g.abi.Declarations = append(g.abi.Declarations, concrete)
	g.structs[concreteName] = concrete
	delete(g.genericInProgress, key)
	return concreteName
}

func (g *generator) instantiateCompilerGenericAlias(name string, args []abiType, inst abiAliasInstantiation) string {
	decl, ok := g.aliases[name]
	if !ok {
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: "unknown generic alias " + name,
		})
		return name
	}
	if len(decl.TypeParams) == 0 {
		return name
	}
	if len(decl.TypeParams) != len(args) {
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: fmt.Sprintf("generic alias %s expects %d type arguments, got %d", name, len(decl.TypeParams), len(args)),
		})
		return name
	}
	if !genericInstantiationMatchesTemplate(inst.AliasName, name) {
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: fmt.Sprintf("generic alias instantiation mismatch: %s uses %s", name, inst.AliasName),
		})
		return name
	}

	key := "alias:" + name + ":" + mapTypeSignature(abiType{TypeArgs: args})
	concreteName := g.genericInstanceName(key, name, args)
	if g.genericInProgress[key] {
		return concreteName
	}
	if _, exists := g.aliases[concreteName]; exists {
		return concreteName
	}

	g.genericInProgress[key] = true
	concrete := decl
	concrete.Name = concreteName
	concrete.TypeParams = nil
	concrete.TypeIndex = nil
	concrete.Target = g.normalizeGenericType(inst.MonomorphicTarget, nil)
	concrete.TargetTypeIndex = nil
	g.abi.Declarations = append(g.abi.Declarations, concrete)
	g.aliases[concreteName] = concrete
	delete(g.genericInProgress, key)
	return concreteName
}

func (g *generator) instantiateCompilerGenericStruct(name string, args []abiType, inst abiStructInstantiation) string {
	decl, ok := g.structs[name]
	if !ok {
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: "unknown generic struct " + name,
		})
		return name
	}
	if len(decl.TypeParams) == 0 {
		return name
	}
	if len(decl.TypeParams) != len(args) {
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: fmt.Sprintf("generic struct %s expects %d type arguments, got %d", name, len(decl.TypeParams), len(args)),
		})
		return name
	}
	if !genericInstantiationMatchesTemplate(inst.StructName, name) {
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: fmt.Sprintf("generic struct instantiation mismatch: %s uses %s", name, inst.StructName),
		})
		return name
	}
	if len(decl.Fields) != len(inst.MonomorphicFields) {
		g.addDiagnostic(Diagnostic{
			Code:    DiagnosticUnsupportedConstruct,
			Message: fmt.Sprintf("generic struct %s expects %d monomorphic fields, got %d", name, len(decl.Fields), len(inst.MonomorphicFields)),
		})
		return name
	}

	key := "struct:" + name + ":" + mapTypeSignature(abiType{TypeArgs: args})
	concreteName := g.genericInstanceName(key, name, args)
	if g.genericInProgress[key] {
		return concreteName
	}
	if _, exists := g.structs[concreteName]; exists {
		return concreteName
	}

	g.genericInProgress[key] = true
	concrete := decl
	concrete.Name = concreteName
	concrete.TypeParams = nil
	concrete.TypeIndex = nil
	concrete.Fields = append([]field(nil), decl.Fields...)
	for i := range concrete.Fields {
		concrete.Fields[i].Type = g.normalizeGenericType(inst.MonomorphicFields[i], nil)
		concrete.Fields[i].TypeIndex = nil
	}
	g.abi.Declarations = append(g.abi.Declarations, concrete)
	g.structs[concreteName] = concrete
	delete(g.genericInProgress, key)
	return concreteName
}

func bindGenericArgs(params []string, args []abiType) genericEnv {
	env := make(genericEnv, len(params))
	for i, param := range params {
		env[param] = args[i]
	}
	return env
}

func genericInstantiationMatchesTemplate(instantiationName, templateName string) bool {
	if instantiationName == "" || instantiationName == templateName {
		return true
	}
	base, _, ok := strings.Cut(instantiationName, "<")
	return ok && strings.TrimSpace(base) == templateName
}

func (g *generator) genericInstanceName(key, templateName string, args []abiType) string {
	if name, ok := g.genericInstanceNames[key]; ok {
		return name
	}

	base := templateName
	if exportedName(base) == "" {
		base = "generic"
	}
	parts := []string{base}
	for _, arg := range args {
		parts = append(parts, genericTypeNamePart(arg))
	}

	name := strings.Join(parts, "_")
	goName := exportedName(name)
	if len(goName) > 96 {
		name = base + "_generic_" + shortTypeHash(key)
		goName = exportedName(name)
	}
	if goName == exportedName(base) {
		name += "_instance"
		goName = exportedName(name)
	}
	for i, candidate := 2, name; g.typeNameTaken(goName); i++ {
		candidate = fmt.Sprintf("%s_%d", name, i)
		candidateGoName := exportedName(candidate)
		if !g.typeNameTaken(candidateGoName) {
			name = candidate
			break
		}
		goName = candidateGoName
	}

	g.genericInstanceNames[key] = name
	return name
}

func genericTypeNamePart(typ abiType) string {
	switch typ.Kind {
	case "":
		return "any"
	case "AliasRef":
		return typ.AliasName + genericTypeArgsNamePart(typ.TypeArgs)
	case "StructRef":
		return typ.StructName + genericTypeArgsNamePart(typ.TypeArgs)
	case "EnumRef":
		return typ.EnumName
	case "genericT":
		return typ.NameT
	case "uintN":
		return fmt.Sprintf("uint%d", typ.N)
	case "intN":
		return fmt.Sprintf("int%d", typ.N)
	case "varuintN":
		return fmt.Sprintf("varuint%d", typ.N)
	case "varintN":
		return fmt.Sprintf("varint%d", typ.N)
	case "bitsN":
		return fmt.Sprintf("bits%d", typ.N)
	case "bytesN":
		return fmt.Sprintf("bytes%d", typ.N)
	case "arrayOf":
		return "array_of_" + genericInnerTypeNamePart(typ.Inner)
	case "nullable":
		return "maybe_" + genericInnerTypeNamePart(typ.Inner)
	case "cellOf":
		return genericInnerTypeNamePart(typ.Inner) + "_cell"
	case "lispListOf":
		return "lisp_list_of_" + genericInnerTypeNamePart(typ.Inner)
	case "mapKV":
		return "map_" + genericPtrTypeNamePart(typ.Key) + "_to_" + genericPtrTypeNamePart(typ.Value)
	case "tensor", "shapedTuple":
		return "tuple_" + genericTypeItemsNamePart(typ.Items)
	case "union":
		return "union_" + genericTypeItemsNamePart(typ.Items)
	case "address":
		return "address"
	case "addressExt":
		return "address_ext"
	case "addressOpt":
		return "address_opt"
	case "addressAny":
		return "address_any"
	case "bool":
		return "bool"
	case "builder":
		return "builder"
	case "callable":
		return "callable"
	case "cell":
		return "cell"
	case "coins":
		return "coins"
	case "int":
		return "int"
	case "null":
		return "null"
	case "remaining":
		return "remaining"
	case "slice":
		return "slice"
	case "string":
		return "string"
	case "unknown":
		return "unknown"
	case "void":
		return "void"
	default:
		return typ.Kind
	}
}

func genericTypeArgsNamePart(args []abiType) string {
	if len(args) == 0 {
		return ""
	}
	return "_" + genericTypeItemsNamePart(args)
}

func genericTypeItemsNamePart(items []abiType) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, genericTypeNamePart(item))
	}
	return strings.Join(parts, "_")
}

func genericInnerTypeNamePart(inner *abiType) string {
	if inner == nil {
		return "any"
	}
	return genericTypeNamePart(*inner)
}

func genericPtrTypeNamePart(typ *abiType) string {
	if typ == nil {
		return "any"
	}
	return genericTypeNamePart(*typ)
}

func shortTypeHash(value string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return fmt.Sprintf("%08x", h.Sum32())
}

func (g *generator) rebuildDeclarationMaps() {
	g.aliases = map[string]declaration{}
	g.enums = map[string]declaration{}
	g.structs = map[string]declaration{}
	for _, decl := range g.abi.Declarations {
		switch decl.Kind {
		case "alias":
			g.aliases[decl.Name] = decl
		case "enum":
			g.enums[decl.Name] = decl
		case "struct":
			g.structs[decl.Name] = decl
		}
	}
}
