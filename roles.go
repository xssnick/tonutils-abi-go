package main

func (g *generator) collectDeclarationRoles() {
	hasTLBRoots := false
	if g.abi.Storage != nil && !isEmptyABIType(g.abi.Storage.StorageType) {
		hasTLBRoots = true
		g.markTLBType(g.abi.Storage.StorageType, map[string]bool{})
	}
	for _, root := range g.abi.IncomingMessages {
		if !isEmptyABIType(root.BodyType) {
			hasTLBRoots = true
			g.markTLBType(root.BodyType, map[string]bool{})
		}
	}
	for _, root := range g.abi.IncomingExternal {
		if !isEmptyABIType(root.BodyType) {
			hasTLBRoots = true
			g.markTLBType(root.BodyType, map[string]bool{})
		}
	}
	for _, root := range g.abi.OutgoingMessages {
		if !isEmptyABIType(root.BodyType) {
			hasTLBRoots = true
			g.markTLBType(root.BodyType, map[string]bool{})
		}
	}
	for _, root := range g.abi.EmittedEvents {
		switch {
		case !isEmptyABIType(root.EventType):
			hasTLBRoots = true
			g.markTLBType(root.EventType, map[string]bool{})
		case !isEmptyABIType(root.BodyType):
			hasTLBRoots = true
			g.markTLBType(root.BodyType, map[string]bool{})
		}
	}

	for _, method := range g.abi.GetMethods {
		for _, param := range method.Parameters {
			g.markStackType(param.Type, map[string]bool{})
		}
		g.markStackType(method.ReturnType, map[string]bool{})
	}

	if !hasTLBRoots && len(g.abi.GetMethods) == 0 {
		for _, decl := range g.abi.Declarations {
			if len(decl.TypeParams) > 0 {
				continue
			}
			switch decl.Kind {
			case "alias":
				g.tlbAliases[decl.Name] = true
			case "enum":
				g.tlbEnums[decl.Name] = true
			case "struct":
				g.tlbStructs[decl.Name] = true
			}
		}
	}
}

func (g *generator) markTLBType(typ abiType, seen map[string]bool) {
	switch typ.Kind {
	case "AliasRef":
		if typ.AliasName == "" || seen["alias:"+typ.AliasName] {
			return
		}
		g.tlbAliases[typ.AliasName] = true
		decl, ok := g.aliases[typ.AliasName]
		if !ok {
			return
		}
		seen["alias:"+typ.AliasName] = true
		g.markTLBType(decl.Target, seen)
		delete(seen, "alias:"+typ.AliasName)
	case "EnumRef":
		if typ.EnumName == "" {
			return
		}
		g.tlbEnums[typ.EnumName] = true
		if decl, ok := g.enums[typ.EnumName]; ok && decl.EncodedAs != nil {
			g.markTLBType(*decl.EncodedAs, seen)
		}
	case "StructRef":
		if typ.StructName == "" || seen["struct:"+typ.StructName] {
			return
		}
		g.tlbStructs[typ.StructName] = true
		decl, ok := g.structs[typ.StructName]
		if !ok {
			return
		}
		if decl.CustomPackUnpack != nil {
			return
		}
		seen["struct:"+typ.StructName] = true
		for _, fld := range decl.Fields {
			g.markTLBType(fld.Type, seen)
		}
		delete(seen, "struct:"+typ.StructName)
	case "arrayOf", "nullable", "cellOf", "lispListOf":
		if typ.Inner != nil {
			g.markTLBType(*typ.Inner, seen)
		}
	case "mapKV":
		if typ.Key != nil {
			g.markTLBType(*typ.Key, seen)
		}
		if typ.Value != nil {
			g.markTLBType(*typ.Value, seen)
		}
	case "tensor", "shapedTuple", "union":
		for _, item := range typ.Items {
			g.markTLBType(item, seen)
		}
	}
	for _, arg := range typ.TypeArgs {
		g.markTLBType(arg, seen)
	}
}

func (g *generator) markStackType(typ abiType, seen map[string]bool) {
	switch typ.Kind {
	case "AliasRef":
		if typ.AliasName == "" || seen["alias:"+typ.AliasName] {
			return
		}
		g.stackAliases[typ.AliasName] = true
		decl, ok := g.aliases[typ.AliasName]
		if !ok {
			return
		}
		seen["alias:"+typ.AliasName] = true
		g.markStackType(decl.Target, seen)
		delete(seen, "alias:"+typ.AliasName)
	case "EnumRef":
		if typ.EnumName == "" {
			return
		}
		g.stackEnums[typ.EnumName] = true
		if decl, ok := g.enums[typ.EnumName]; ok && decl.EncodedAs != nil {
			g.markStackType(*decl.EncodedAs, seen)
		}
	case "StructRef":
		if typ.StructName == "" || seen["struct:"+typ.StructName] {
			return
		}
		g.stackStructs[typ.StructName] = true
		decl, ok := g.structs[typ.StructName]
		if !ok {
			return
		}
		seen["struct:"+typ.StructName] = true
		for _, fld := range decl.Fields {
			g.markStackType(fld.Type, seen)
		}
		delete(seen, "struct:"+typ.StructName)
	case "arrayOf", "nullable", "lispListOf":
		if typ.Inner != nil {
			g.markStackType(*typ.Inner, seen)
		}
	case "cellOf":
		if typ.Inner != nil && typ.Inner.Kind != "slice" {
			g.markTLBType(*typ.Inner, seen)
		}
		return
	case "mapKV":
		if typ.Key != nil {
			g.markStackType(*typ.Key, seen)
		}
		if typ.Value != nil {
			g.markStackType(*typ.Value, seen)
		}
	case "tensor", "shapedTuple", "union":
		for _, item := range typ.Items {
			g.markStackType(item, seen)
		}
	}
	for _, arg := range typ.TypeArgs {
		g.markStackType(arg, seen)
	}
}

func isEmptyABIType(typ abiType) bool {
	return typ.Kind == "" && typ.N == 0 && typ.AliasName == "" && typ.StructName == "" && typ.EnumName == "" &&
		typ.NameT == "" && len(typ.TypeArgs) == 0 && typ.Inner == nil && len(typ.Items) == 0 &&
		typ.Key == nil && typ.Value == nil
}
