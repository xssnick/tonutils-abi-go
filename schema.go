package main

import (
	"encoding/json"
	"fmt"
)

func validateABISchema(data []byte, abi abiFile) []Diagnostic {
	var diagnostics []Diagnostic
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []Diagnostic{schemaDiagnostic("abi", "parse raw ABI object: "+err.Error())}
	}

	if abi.ContractName == "" {
		diagnostics = append(diagnostics, schemaDiagnostic("abi.contract_name", "required field is missing or empty"))
	}

	validateDeclarations(&diagnostics, raw["declarations"], abi.Declarations)
	validateGetMethods(&diagnostics, raw["get_methods"], abi.GetMethods)
	return diagnostics
}

func validateDeclarations(diagnostics *[]Diagnostic, raw json.RawMessage, declarations []declaration) {
	if isNull(raw) {
		return
	}
	rawItems, ok := rawArray(diagnostics, "abi.declarations", raw)
	if !ok {
		return
	}
	for i, decl := range declarations {
		path := fmt.Sprintf("abi.declarations[%d]", i)
		rawDecl, ok := rawObject(diagnostics, path, rawItems, i)
		if !ok {
			continue
		}
		if decl.Kind == "" {
			*diagnostics = append(*diagnostics, schemaDiagnostic(path+".kind", "required field is missing or empty"))
			continue
		}
		if decl.Name == "" {
			*diagnostics = append(*diagnostics, schemaDiagnostic(path+".name", "required field is missing or empty"))
		}

		switch decl.Kind {
		case "alias":
			if decl.Target.Kind == "" {
				*diagnostics = append(*diagnostics, schemaDiagnostic(path+".target_ty", "required for alias declarations"))
			} else if !isNull(rawDecl["target_ty"]) {
				validateABIType(diagnostics, path+".target_ty", rawDecl["target_ty"], decl.Target)
			}
		case "enum":
			if decl.EncodedAs == nil {
				*diagnostics = append(*diagnostics, schemaDiagnostic(path+".encoded_as", "required for enum declarations"))
			} else if !isNull(rawDecl["encoded_as"]) {
				validateABIType(diagnostics, path+".encoded_as", rawDecl["encoded_as"], *decl.EncodedAs)
			}
			validateEnumMembers(diagnostics, path+".members", rawDecl["members"], decl.Members)
		case "struct":
			validateFields(diagnostics, path+".fields", rawDecl["fields"], decl.Fields)
		default:
			*diagnostics = append(*diagnostics, schemaDiagnostic(path+".kind", fmt.Sprintf("unsupported declaration kind %q", decl.Kind)))
		}
	}
}

func validateFields(diagnostics *[]Diagnostic, path string, raw json.RawMessage, fields []field) {
	if isNull(raw) {
		return
	}
	rawItems, ok := rawArray(diagnostics, path, raw)
	if !ok {
		return
	}
	for i, field := range fields {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		rawField, ok := rawObject(diagnostics, itemPath, rawItems, i)
		if !ok {
			continue
		}
		if field.Name == "" {
			*diagnostics = append(*diagnostics, schemaDiagnostic(itemPath+".name", "required field is missing or empty"))
		}
		if field.Type.Kind == "" {
			*diagnostics = append(*diagnostics, schemaDiagnostic(itemPath+".ty", "required field is missing or empty"))
			continue
		}
		if !isNull(rawField["ty"]) {
			validateABIType(diagnostics, itemPath+".ty", rawField["ty"], field.Type)
		}
	}
}

func validateEnumMembers(diagnostics *[]Diagnostic, path string, raw json.RawMessage, members []enumMember) {
	if isNull(raw) {
		return
	}
	rawItems, ok := rawArray(diagnostics, path, raw)
	if !ok {
		return
	}
	for i, member := range members {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if _, ok := rawObject(diagnostics, itemPath, rawItems, i); !ok {
			continue
		}
		if member.Name == "" {
			*diagnostics = append(*diagnostics, schemaDiagnostic(itemPath+".name", "required field is missing or empty"))
		}
		if member.Value == "" {
			*diagnostics = append(*diagnostics, schemaDiagnostic(itemPath+".value", "required field is missing or empty"))
		}
	}
}

func validateGetMethods(diagnostics *[]Diagnostic, raw json.RawMessage, methods []getMethod) {
	if isNull(raw) {
		return
	}
	rawItems, ok := rawArray(diagnostics, "abi.get_methods", raw)
	if !ok {
		return
	}
	for i, method := range methods {
		path := fmt.Sprintf("abi.get_methods[%d]", i)
		rawMethod, ok := rawObject(diagnostics, path, rawItems, i)
		if !ok {
			continue
		}
		if method.Name == "" {
			*diagnostics = append(*diagnostics, schemaDiagnostic(path+".name", "required field is missing or empty"))
		}
		if method.ReturnType.Kind == "" {
			*diagnostics = append(*diagnostics, schemaDiagnostic(path+".return_ty", "required field is missing or empty"))
		} else if !isNull(rawMethod["return_ty"]) {
			validateABIType(diagnostics, path+".return_ty", rawMethod["return_ty"], method.ReturnType)
		}
		validateParameters(diagnostics, path+".parameters", rawMethod["parameters"], method.Parameters)
	}
}

func validateParameters(diagnostics *[]Diagnostic, path string, raw json.RawMessage, parameters []parameter) {
	if isNull(raw) {
		return
	}
	rawItems, ok := rawArray(diagnostics, path, raw)
	if !ok {
		return
	}
	for i, parameter := range parameters {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		rawParameter, ok := rawObject(diagnostics, itemPath, rawItems, i)
		if !ok {
			continue
		}
		if parameter.Name == "" {
			*diagnostics = append(*diagnostics, schemaDiagnostic(itemPath+".name", "required field is missing or empty"))
		}
		if parameter.Type.Kind == "" {
			*diagnostics = append(*diagnostics, schemaDiagnostic(itemPath+".ty", "required field is missing or empty"))
			continue
		}
		if !isNull(rawParameter["ty"]) {
			validateABIType(diagnostics, itemPath+".ty", rawParameter["ty"], parameter.Type)
		}
	}
}

func validateABIType(diagnostics *[]Diagnostic, path string, raw json.RawMessage, typ abiType) {
	rawType, ok := rawMap(diagnostics, path, raw)
	if !ok {
		return
	}
	if typ.Kind == "" {
		*diagnostics = append(*diagnostics, schemaDiagnostic(path+".kind", "required field is missing or empty"))
		return
	}

	switch typ.Kind {
	case "AliasRef":
		requireString(diagnostics, path+".alias_name", typ.AliasName)
	case "StructRef":
		requireString(diagnostics, path+".struct_name", typ.StructName)
	case "EnumRef":
		requireString(diagnostics, path+".enum_name", typ.EnumName)
	case "arrayOf", "nullable", "cellOf", "lispListOf":
		if typ.Inner == nil {
			*diagnostics = append(*diagnostics, schemaDiagnostic(path+".inner", "required for "+typ.Kind))
		} else if !isNull(rawType["inner"]) {
			validateABIType(diagnostics, path+".inner", rawType["inner"], *typ.Inner)
		}
	case "mapKV":
		if typ.Key == nil {
			*diagnostics = append(*diagnostics, schemaDiagnostic(path+".k", "required for mapKV"))
		} else if !isNull(rawType["k"]) {
			validateABIType(diagnostics, path+".k", rawType["k"], *typ.Key)
		}
		if typ.Value == nil {
			*diagnostics = append(*diagnostics, schemaDiagnostic(path+".v", "required for mapKV"))
		} else if !isNull(rawType["v"]) {
			validateABIType(diagnostics, path+".v", rawType["v"], *typ.Value)
		}
	case "tensor", "shapedTuple", "union":
		if !isNull(rawType["items"]) {
			validateABITypeItems(diagnostics, path+".items", rawType["items"], typ.Items)
		}
	default:
		validateABITypeArgs(diagnostics, path+".type_args", rawType["type_args"], typ.TypeArgs)
		if !isNull(rawType["items"]) {
			validateABITypeItems(diagnostics, path+".items", rawType["items"], typ.Items)
		}
	}
}

func validateABITypeArgs(diagnostics *[]Diagnostic, path string, raw json.RawMessage, items []abiType) {
	if isNull(raw) {
		return
	}
	validateABITypeItems(diagnostics, path, raw, items)
}

func validateABITypeItems(diagnostics *[]Diagnostic, path string, raw json.RawMessage, items []abiType) {
	if isNull(raw) {
		return
	}
	rawItems, ok := rawArray(diagnostics, path, raw)
	if !ok {
		return
	}
	for i, item := range items {
		if i >= len(rawItems) {
			*diagnostics = append(*diagnostics, schemaDiagnostic(fmt.Sprintf("%s[%d]", path, i), "raw item is missing"))
			continue
		}
		validateABIType(diagnostics, fmt.Sprintf("%s[%d]", path, i), rawItems[i], item)
	}
}

func requireString(diagnostics *[]Diagnostic, path string, value string) {
	if value == "" {
		*diagnostics = append(*diagnostics, schemaDiagnostic(path, "required field is missing or empty"))
	}
}

func rawArray(diagnostics *[]Diagnostic, path string, raw json.RawMessage) ([]json.RawMessage, bool) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		*diagnostics = append(*diagnostics, schemaDiagnostic(path, "expected array: "+err.Error()))
		return nil, false
	}
	return items, true
}

func rawObject(diagnostics *[]Diagnostic, path string, items []json.RawMessage, index int) (map[string]json.RawMessage, bool) {
	if index >= len(items) {
		*diagnostics = append(*diagnostics, schemaDiagnostic(path, "raw item is missing"))
		return nil, false
	}
	return rawMap(diagnostics, path, items[index])
}

func rawMap(diagnostics *[]Diagnostic, path string, raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		*diagnostics = append(*diagnostics, schemaDiagnostic(path, "expected object: "+err.Error()))
		return nil, false
	}
	return obj, true
}

func isNull(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	switch string(raw) {
	case "null", "":
		return true
	default:
		return false
	}
}

func schemaDiagnostic(subject string, message string) Diagnostic {
	return Diagnostic{
		Code:    DiagnosticSchema,
		Subject: subject,
		Message: message,
	}
}
