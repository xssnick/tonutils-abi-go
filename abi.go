package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type abiFile struct {
	SchemaVersion        string                   `json:"abi_schema_version"`
	ContractName         string                   `json:"contract_name"`
	UniqueTypes          []abiType                `json:"unique_types"`
	StructInstantiations []abiStructInstantiation `json:"struct_instantiations"`
	AliasInstantiations  []abiAliasInstantiation  `json:"alias_instantiations"`
	Declarations         []declaration            `json:"declarations"`
	GetMethods           []getMethod              `json:"get_methods"`
	IncomingMessages     []abiBodyRoot            `json:"incoming_messages"`
	IncomingExternal     []abiBodyRoot            `json:"incoming_external"`
	OutgoingMessages     []abiBodyRoot            `json:"outgoing_messages"`
	EmittedEvents        []abiBodyRoot            `json:"emitted_events"`
	Storage              *abiStorage              `json:"storage"`
	ThrownErrors         []thrownError            `json:"thrown_errors"`
	Constants            []abiConstant            `json:"constants"`
}

type declaration struct {
	Kind             string            `json:"kind"`
	Name             string            `json:"name"`
	Prefix           *prefix           `json:"prefix"`
	Fields           []field           `json:"fields"`
	Target           abiType           `json:"target_ty"`
	TargetTypeIndex  *int              `json:"target_ty_idx"`
	EncodedAs        *abiType          `json:"encoded_as"`
	EncodedAsIndex   *int              `json:"encoded_as_ty_idx"`
	Members          []enumMember      `json:"members"`
	TypeParams       []string          `json:"type_params"`
	TypeIndex        *int              `json:"ty_idx"`
	CustomPackUnpack *customPackUnpack `json:"custom_pack_unpack"`
}

type prefix struct {
	PrefixStr string  `json:"prefix_str"`
	PrefixNum *uint64 `json:"prefix_num"`
	PrefixLen int     `json:"prefix_len"`
}

type customPackUnpack struct {
	PackToBuilder   bool `json:"pack_to_builder"`
	UnpackFromSlice bool `json:"unpack_from_slice"`
}

type thrownError struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Code        int64  `json:"err_code"`
}

type abiConstant struct {
	Name  string             `json:"name"`
	Value abiConstExpression `json:"value"`
}

type abiConstExpression struct {
	Kind            string               `json:"kind"`
	Value           any                  `json:"v"`
	Hex             string               `json:"hex"`
	String          string               `json:"str"`
	Address         string               `json:"addr"`
	Items           []abiConstExpression `json:"items"`
	Fields          []abiConstExpression `json:"fields"`
	StructName      string               `json:"struct_name"`
	Inner           *abiConstExpression  `json:"inner"`
	CastTo          abiType              `json:"cast_to"`
	CastToTypeIndex *int                 `json:"cast_to_ty_idx"`
}

type field struct {
	Name      string  `json:"name"`
	Type      abiType `json:"ty"`
	TypeIndex *int    `json:"ty_idx"`
}

type enumMember struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type getMethod struct {
	TVMMethodID     int64       `json:"tvm_method_id"`
	Name            string      `json:"name"`
	Parameters      []parameter `json:"parameters"`
	ReturnType      abiType     `json:"return_ty"`
	ReturnTypeIndex *int        `json:"return_ty_idx"`
}

type parameter struct {
	Name      string  `json:"name"`
	Type      abiType `json:"ty"`
	TypeIndex *int    `json:"ty_idx"`
}

type abiBodyRoot struct {
	BodyType       abiType `json:"body_ty"`
	BodyTypeIndex  *int    `json:"body_ty_idx"`
	EventType      abiType `json:"event_ty"`
	EventTypeIndex *int    `json:"event_ty_idx"`
}

type abiStorage struct {
	StorageType      abiType `json:"storage_ty"`
	StorageTypeIndex *int    `json:"storage_ty_idx"`
}

type abiStructInstantiation struct {
	TypeIndex                   int       `json:"ty_idx"`
	StructName                  string    `json:"struct_name"`
	MonomorphicFieldTypeIndices []int     `json:"monomorphic_fields_ty_idx"`
	MonomorphicFields           []abiType `json:"-"`
}

type abiAliasInstantiation struct {
	TypeIndex                  int     `json:"ty_idx"`
	AliasName                  string  `json:"alias_name"`
	MonomorphicTargetTypeIndex int     `json:"monomorphic_target_ty_idx"`
	MonomorphicTarget          abiType `json:"-"`
}

type abiType struct {
	Kind            string           `json:"kind"`
	N               int              `json:"n"`
	AliasName       string           `json:"alias_name"`
	StructName      string           `json:"struct_name"`
	EnumName        string           `json:"enum_name"`
	NameT           string           `json:"name_t"`
	TypeArgs        []abiType        `json:"type_args"`
	TypeArgIndices  []int            `json:"type_args_ty_idx"`
	Inner           *abiType         `json:"inner"`
	InnerTypeIndex  *int             `json:"inner_ty_idx"`
	Items           []abiType        `json:"items"`
	ItemTypeIndices []int            `json:"items_ty_idx"`
	Variants        []abiTypeVariant `json:"variants"`
	Key             *abiType         `json:"k"`
	KeyTypeIndex    *int             `json:"key_ty_idx"`
	Value           *abiType         `json:"v"`
	ValueTypeIndex  *int             `json:"value_ty_idx"`
	StackTypeID     *int             `json:"stack_type_id"`
	StackWidth      *int             `json:"stack_width"`
	Index           *int             `json:"-"`
}

type abiTypeVariant struct {
	Type        abiType `json:"variant_ty"`
	TypeIndex   *int    `json:"variant_ty_idx"`
	PrefixNum   *uint64 `json:"prefix_num"`
	PrefixLen   int     `json:"prefix_len"`
	StackTypeID *int    `json:"stack_type_id"`
	StackWidth  *int    `json:"stack_width"`
}

func loadABIFile(path string, strict bool) (abiFile, []Diagnostic, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return abiFile{}, nil, fmt.Errorf("read ABI: %w", err)
	}

	return parseABIForGeneration(data, strict)
}

func parseABI(data []byte) (abiFile, error) {
	var abi abiFile
	if err := json.Unmarshal(data, &abi); err != nil {
		return abiFile{}, fmt.Errorf("parse ABI: %w", err)
	}
	return abi, nil
}

func parseABIForGeneration(data []byte, strict bool) (abiFile, []Diagnostic, error) {
	abi, err := parseABI(data)
	if err != nil {
		return abiFile{}, nil, err
	}
	diagnostics := abi.resolveTypeIndices()
	if !strict {
		return abi, diagnostics, nil
	}
	diagnostics = append(diagnostics, validateABISchema(data, abi)...)
	return abi, diagnostics, nil
}

func (abi *abiFile) resolveTypeIndices() []Diagnostic {
	if len(abi.UniqueTypes) == 0 {
		return nil
	}

	resolver := abiTypeIndexResolver{
		types:     abi.UniqueTypes,
		resolved:  map[int]abiType{},
		resolving: map[int]bool{},
	}
	var diagnostics []Diagnostic
	resolveInto := func(path string, target *abiType, idx *int) {
		if idx == nil || target == nil || target.Kind != "" {
			return
		}
		resolved, ok := resolver.resolve(*idx, path, &diagnostics)
		if ok {
			*target = resolved
		}
	}

	for i := range abi.UniqueTypes {
		resolved, ok := resolver.resolve(i, fmt.Sprintf("abi.unique_types[%d]", i), &diagnostics)
		if ok {
			abi.UniqueTypes[i] = resolved
		}
	}

	for i := range abi.StructInstantiations {
		inst := &abi.StructInstantiations[i]
		path := fmt.Sprintf("abi.struct_instantiations[%d]", i)
		inst.MonomorphicFields = resolver.resolveIndices(inst.MonomorphicFieldTypeIndices, path+".monomorphic_fields_ty_idx", &diagnostics)
	}

	for i := range abi.AliasInstantiations {
		inst := &abi.AliasInstantiations[i]
		path := fmt.Sprintf("abi.alias_instantiations[%d].monomorphic_target_ty_idx", i)
		if target, ok := resolver.resolve(inst.MonomorphicTargetTypeIndex, path, &diagnostics); ok {
			inst.MonomorphicTarget = target
		}
	}

	for i := range abi.Declarations {
		decl := &abi.Declarations[i]
		path := fmt.Sprintf("abi.declarations[%d]", i)
		resolveInto(path+".target_ty_idx", &decl.Target, decl.TargetTypeIndex)
		if decl.EncodedAs == nil && decl.EncodedAsIndex != nil {
			resolved, ok := resolver.resolve(*decl.EncodedAsIndex, path+".encoded_as_ty_idx", &diagnostics)
			if ok {
				decl.EncodedAs = &resolved
			}
		}
		for j := range decl.Fields {
			field := &decl.Fields[j]
			resolveInto(fmt.Sprintf("%s.fields[%d].ty_idx", path, j), &field.Type, field.TypeIndex)
		}
	}

	for i := range abi.GetMethods {
		method := &abi.GetMethods[i]
		path := fmt.Sprintf("abi.get_methods[%d]", i)
		resolveInto(path+".return_ty_idx", &method.ReturnType, method.ReturnTypeIndex)
		for j := range method.Parameters {
			param := &method.Parameters[j]
			resolveInto(fmt.Sprintf("%s.parameters[%d].ty_idx", path, j), &param.Type, param.TypeIndex)
		}
	}

	for i := range abi.IncomingMessages {
		root := &abi.IncomingMessages[i]
		resolveInto(fmt.Sprintf("abi.incoming_messages[%d].body_ty_idx", i), &root.BodyType, root.BodyTypeIndex)
	}
	for i := range abi.IncomingExternal {
		root := &abi.IncomingExternal[i]
		resolveInto(fmt.Sprintf("abi.incoming_external[%d].body_ty_idx", i), &root.BodyType, root.BodyTypeIndex)
	}
	for i := range abi.OutgoingMessages {
		root := &abi.OutgoingMessages[i]
		resolveInto(fmt.Sprintf("abi.outgoing_messages[%d].body_ty_idx", i), &root.BodyType, root.BodyTypeIndex)
	}
	for i := range abi.EmittedEvents {
		root := &abi.EmittedEvents[i]
		resolveInto(fmt.Sprintf("abi.emitted_events[%d].body_ty_idx", i), &root.BodyType, root.BodyTypeIndex)
		resolveInto(fmt.Sprintf("abi.emitted_events[%d].event_ty_idx", i), &root.EventType, root.EventTypeIndex)
	}
	if abi.Storage != nil {
		resolveInto("abi.storage.storage_ty_idx", &abi.Storage.StorageType, abi.Storage.StorageTypeIndex)
	}
	for i := range abi.Constants {
		resolver.resolveConstExpression(&abi.Constants[i].Value, fmt.Sprintf("abi.constants[%d].value", i), &diagnostics)
	}

	return diagnostics
}

type abiTypeIndexResolver struct {
	types     []abiType
	resolved  map[int]abiType
	resolving map[int]bool
}

func (r *abiTypeIndexResolver) resolve(index int, path string, diagnostics *[]Diagnostic) (abiType, bool) {
	if index < 0 || index >= len(r.types) {
		*diagnostics = append(*diagnostics, schemaDiagnostic(path, fmt.Sprintf("type index %d out of range", index)))
		return abiType{}, false
	}
	if typ, ok := r.resolved[index]; ok {
		return typ, true
	}
	if r.resolving[index] {
		*diagnostics = append(*diagnostics, schemaDiagnostic(path, fmt.Sprintf("recursive type index %d", index)))
		return abiType{}, false
	}

	r.resolving[index] = true
	typ := r.types[index]
	typ.Index = intPtr(index)
	r.resolveType(&typ, path, diagnostics)
	delete(r.resolving, index)
	r.resolved[index] = typ
	return typ, true
}

func (r *abiTypeIndexResolver) resolveType(typ *abiType, path string, diagnostics *[]Diagnostic) {
	if typ == nil {
		return
	}
	if typ.Inner == nil && typ.InnerTypeIndex != nil {
		if inner, ok := r.resolve(*typ.InnerTypeIndex, path+".inner_ty_idx", diagnostics); ok {
			typ.Inner = &inner
		}
	}
	if typ.Key == nil && typ.KeyTypeIndex != nil {
		if key, ok := r.resolve(*typ.KeyTypeIndex, path+".key_ty_idx", diagnostics); ok {
			typ.Key = &key
		}
	}
	if typ.Value == nil && typ.ValueTypeIndex != nil {
		if value, ok := r.resolve(*typ.ValueTypeIndex, path+".value_ty_idx", diagnostics); ok {
			typ.Value = &value
		}
	}
	if len(typ.TypeArgs) == 0 && len(typ.TypeArgIndices) > 0 {
		typ.TypeArgs = r.resolveIndices(typ.TypeArgIndices, path+".type_args_ty_idx", diagnostics)
	}
	if len(typ.Items) == 0 && len(typ.ItemTypeIndices) > 0 {
		typ.Items = r.resolveIndices(typ.ItemTypeIndices, path+".items_ty_idx", diagnostics)
	}
	if len(typ.Items) == 0 && len(typ.Variants) > 0 {
		for i, variant := range typ.Variants {
			switch {
			case variant.Type.Kind != "":
				item := variant.Type
				r.resolveType(&item, fmt.Sprintf("%s.variants[%d].variant_ty", path, i), diagnostics)
				typ.Variants[i].Type = item
				typ.Items = append(typ.Items, item)
			case variant.TypeIndex != nil:
				if item, ok := r.resolve(*variant.TypeIndex, fmt.Sprintf("%s.variants[%d].variant_ty_idx", path, i), diagnostics); ok {
					typ.Variants[i].Type = item
					typ.Items = append(typ.Items, item)
				}
			default:
				*diagnostics = append(*diagnostics, schemaDiagnostic(fmt.Sprintf("%s.variants[%d]", path, i), "variant type is missing"))
			}
		}
	}
	if typ.Inner != nil {
		r.resolveType(typ.Inner, path+".inner", diagnostics)
	}
	if typ.Key != nil {
		r.resolveType(typ.Key, path+".k", diagnostics)
	}
	if typ.Value != nil {
		r.resolveType(typ.Value, path+".v", diagnostics)
	}
	for i := range typ.TypeArgs {
		r.resolveType(&typ.TypeArgs[i], fmt.Sprintf("%s.type_args[%d]", path, i), diagnostics)
	}
	for i := range typ.Items {
		r.resolveType(&typ.Items[i], fmt.Sprintf("%s.items[%d]", path, i), diagnostics)
	}
}

func (r *abiTypeIndexResolver) resolveIndices(indices []int, path string, diagnostics *[]Diagnostic) []abiType {
	items := make([]abiType, 0, len(indices))
	for i, index := range indices {
		if item, ok := r.resolve(index, fmt.Sprintf("%s[%d]", path, i), diagnostics); ok {
			items = append(items, item)
		}
	}
	return items
}

func (r *abiTypeIndexResolver) resolveConstExpression(expr *abiConstExpression, path string, diagnostics *[]Diagnostic) {
	if expr == nil {
		return
	}
	if expr.CastTo.Kind == "" && expr.CastToTypeIndex != nil {
		if castTo, ok := r.resolve(*expr.CastToTypeIndex, path+".cast_to_ty_idx", diagnostics); ok {
			expr.CastTo = castTo
		}
	} else if expr.CastTo.Kind != "" {
		r.resolveType(&expr.CastTo, path+".cast_to", diagnostics)
	}
	if expr.Inner != nil {
		r.resolveConstExpression(expr.Inner, path+".inner", diagnostics)
	}
	for i := range expr.Items {
		r.resolveConstExpression(&expr.Items[i], fmt.Sprintf("%s.items[%d]", path, i), diagnostics)
	}
	for i := range expr.Fields {
		r.resolveConstExpression(&expr.Fields[i], fmt.Sprintf("%s.fields[%d]", path, i), diagnostics)
	}
}

func intPtr(v int) *int {
	return &v
}
