package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func intGoType(bits int, signed bool) string {
	if bits <= 0 {
		return "*big.Int"
	}
	if bits <= 8 {
		if signed {
			return "int8"
		}
		return "uint8"
	}
	if bits <= 16 {
		if signed {
			return "int16"
		}
		return "uint16"
	}
	if bits <= 32 {
		if signed {
			return "int32"
		}
		return "uint32"
	}
	if bits <= 64 {
		if signed {
			return "int64"
		}
		return "uint64"
	}
	return "*big.Int"
}

func nullableGoType(goType string) string {
	if strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") || goType == "any" {
		return goType
	}
	return "*" + goType
}

func varIntKind(kind string) string {
	switch kind {
	case "varuintN":
		return "varuint"
	case "varintN":
		return "varint"
	default:
		return kind
	}
}

func zeroValue(goType string) string {
	switch goType {
	case "any":
		return "nil"
	case "string":
		return `""`
	default:
		if isNilGoType(goType) {
			return "nil"
		}
		return "0"
	}
}

func isNilGoType(goType string) bool {
	return goType == "any" || strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") || strings.HasPrefix(goType, "map[")
}

func assignOp(target string) string {
	if strings.Contains(target, ".") {
		return "="
	}
	return ":="
}

func prefixTag(p prefix) string {
	if strings.HasPrefix(p.PrefixStr, "0x") {
		return "#" + strings.TrimPrefix(p.PrefixStr, "0x")
	}
	if strings.HasPrefix(p.PrefixStr, "0b") {
		return "$" + strings.TrimPrefix(p.PrefixStr, "0b")
	}
	if p.PrefixNum != nil && p.PrefixLen > 0 {
		if p.PrefixLen%4 == 0 {
			return fmt.Sprintf("#%0*x", p.PrefixLen/4, *p.PrefixNum)
		}
		bits := strconv.FormatUint(*p.PrefixNum, 2)
		if len(bits) < p.PrefixLen {
			bits = strings.Repeat("0", p.PrefixLen-len(bits)) + bits
		}
		return "$" + bits
	}
	return p.PrefixStr
}

func callArgSuffix(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return ", " + strings.Join(args, ", ")
}

func sanitizePackageName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "wrappers"
	}

	var b strings.Builder
	for i, r := range name {
		switch {
		case r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)):
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsDigit(r):
			b.WriteRune('_')
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 || goKeywords[b.String()] {
		return "wrappers"
	}
	return b.String()
}

func isStdImport(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

func exportedName(name string) string {
	parts := splitName(name)
	if len(parts) == 0 {
		return ""
	}

	var b strings.Builder
	for _, part := range parts {
		upper := strings.ToUpper(part)
		if commonInitialisms[upper] {
			b.WriteString(upper)
			continue
		}
		runes := []rune(part)
		b.WriteString(strings.ToUpper(string(runes[:1])))
		if len(runes) > 1 {
			b.WriteString(strings.ToLower(string(runes[1:])))
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	if unicode.IsDigit(rune(out[0])) {
		out = "X" + out
	}
	if goKeywords[out] {
		out += "_"
	}
	return out
}

func unexportedName(name string) string {
	exp := exportedName(name)
	if exp == "" {
		return ""
	}
	for _, initialism := range commonInitialismList {
		if strings.HasPrefix(exp, initialism) {
			return strings.ToLower(initialism) + exp[len(initialism):]
		}
	}
	return strings.ToLower(exp[:1]) + exp[1:]
}

func splitName(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	var parts []string
	var current []rune
	runes := []rune(name)
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			if len(current) > 0 {
				parts = appendNameParts(parts, string(current))
				current = current[:0]
			}
			continue
		}

		if len(current) > 0 {
			prev := current[len(current)-1]
			next := rune(0)
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if shouldSplitNamePart(prev, r, next) {
				parts = appendNameParts(parts, string(current))
				current = current[:0]
			}
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		parts = appendNameParts(parts, string(current))
	}
	return parts
}

func shouldSplitNamePart(prev, cur, next rune) bool {
	switch {
	case unicode.IsDigit(prev) && unicode.IsLetter(cur):
		return true
	case unicode.IsLetter(prev) && unicode.IsDigit(cur):
		return true
	case unicode.IsLower(prev) && unicode.IsUpper(cur):
		return true
	case unicode.IsUpper(prev) && unicode.IsUpper(cur) && next != 0 && unicode.IsLower(next):
		return true
	default:
		return false
	}
}

func appendNameParts(parts []string, raw string) []string {
	if raw == "" {
		return parts
	}
	for _, part := range splitLowercaseInitialismPrefix(raw) {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func splitLowercaseInitialismPrefix(part string) []string {
	if part == "" || part != strings.ToLower(part) {
		return []string{part}
	}
	for _, initialism := range commonInitialismList {
		if len(initialism) < 3 {
			continue
		}
		prefix := strings.ToLower(initialism)
		if strings.HasPrefix(part, prefix) && len(part) > len(prefix) {
			return append([]string{prefix}, splitName(part[len(prefix):])...)
		}
	}
	return []string{part}
}

func uniqueExportedName(name, fallback string, used map[string]bool) string {
	name = exportedName(name)
	if name == "" {
		name = exportedName(fallback)
	}
	if name == "" {
		name = "Field"
	}
	base := name
	for i := 2; used[name]; i++ {
		name = numberedExportedName(base, i)
	}
	used[name] = true
	return name
}

func numberedExportedName(base string, index int) string {
	if base == "" {
		return fmt.Sprintf("Field%d", index)
	}
	runes := []rune(base)
	last := runes[len(runes)-1]
	if unicode.IsDigit(last) {
		return fmt.Sprintf("%sValue%d", base, index)
	}
	return fmt.Sprintf("%s%d", base, index)
}

func declarationFieldNames(fields []field) []string {
	used := map[string]bool{}
	names := make([]string, 0, len(fields))
	for i, fld := range fields {
		names = append(names, uniqueExportedName(fld.Name, fmt.Sprintf("Field%d", i+1), used))
	}
	return names
}

func tupleFieldNames(items []abiType) []string {
	return semanticFieldNames(items, "Item")
}

func resultFieldNames(items []abiType) []string {
	return semanticFieldNames(items, "Value")
}

func semanticFieldNames(items []abiType, fallbackPrefix string) []string {
	used := map[string]bool{}
	names := make([]string, 0, len(items))
	for i, item := range items {
		name := semanticNameForType(item)
		if name == "" || name == "Value" {
			name = fmt.Sprintf("%s%d", fallbackPrefix, i+1)
		}
		names = append(names, uniqueExportedName(name, fmt.Sprintf("%s%d", fallbackPrefix, i+1), used))
	}
	return names
}

func tupleFieldName(index int) string {
	return fmt.Sprintf("Item%d", index+1)
}

func semanticNameForType(typ abiType) string {
	switch typ.Kind {
	case "AliasRef":
		return exportedName(typ.AliasName)
	case "StructRef":
		return exportedName(typ.StructName)
	case "EnumRef":
		return exportedName(typ.EnumName)
	case "nullable":
		if typ.Inner == nil {
			return "Maybe"
		}
		inner := semanticNameForType(*typ.Inner)
		if inner == "" || inner == "Value" {
			return "Maybe"
		}
		if strings.HasPrefix(inner, "Maybe") {
			return inner
		}
		return "Maybe" + inner
	case "arrayOf", "lispListOf":
		if typ.Inner == nil {
			return "Items"
		}
		inner := semanticNameForType(*typ.Inner)
		if inner == "" || inner == "Value" {
			return "Items"
		}
		return inner + "List"
	case "cellOf":
		if typ.Inner == nil {
			return "Cell"
		}
		inner := semanticNameForType(*typ.Inner)
		if inner == "" || inner == "Value" {
			return "Cell"
		}
		return inner + "Cell"
	case "mapKV":
		return "Dict"
	case "tensor", "shapedTuple":
		return "Tuple"
	case "union":
		return "Choice"
	case "uintN":
		return fmt.Sprintf("Uint%d", typ.N)
	case "intN":
		return fmt.Sprintf("Int%d", typ.N)
	case "varuintN":
		return fmt.Sprintf("VarUint%d", typ.N)
	case "varintN":
		return fmt.Sprintf("VarInt%d", typ.N)
	case "bitsN":
		return fmt.Sprintf("Bits%d", typ.N)
	case "bytesN":
		return fmt.Sprintf("Bytes%d", typ.N)
	case "address":
		return "Address"
	case "addressExt":
		return "ExternalAddress"
	case "addressOpt":
		return "Address"
	case "addressAny":
		return "AnyAddress"
	case "bool":
		return "Bool"
	case "builder":
		return "Builder"
	case "cell":
		return "Cell"
	case "coins":
		return "Coins"
	case "int":
		return "Int"
	case "remaining", "slice":
		return "Slice"
	case "string":
		return "String"
	case "void":
		return "Void"
	case "null", "nullLiteral":
		return "Null"
	default:
		return "Value"
	}
}

type nameAllocator struct {
	packageNames    map[string]bool
	receiverMethods map[string]bool
}

func newNameAllocator() *nameAllocator {
	return &nameAllocator{
		packageNames:    map[string]bool{},
		receiverMethods: map[string]bool{},
	}
}

func (a *nameAllocator) reservePackage(name string) {
	if name != "" {
		a.packageNames[name] = true
	}
}

func (a *nameAllocator) reserveReceiverMethod(name string) {
	if name != "" {
		a.receiverMethods[name] = true
	}
}

func (a *nameAllocator) packageNameUsed(name string) bool {
	return a != nil && a.packageNames[name]
}

func (a *nameAllocator) uniquePackage(name, fallback string) string {
	return a.uniqueExported(name, fallback, a.packageNames)
}

func (a *nameAllocator) uniqueReceiverMethod(name, fallback string) string {
	return a.uniqueExported(name, fallback, a.receiverMethods)
}

func (a *nameAllocator) uniqueExported(name, fallback string, used map[string]bool) string {
	name = exportedName(name)
	if name == "" {
		name = exportedName(fallback)
	}
	if name == "" {
		name = "Value"
	}
	base := name
	for i := 2; used[name]; i++ {
		name = numberedExportedName(base, i)
	}
	used[name] = true
	return name
}

func (g *generator) prepareNames() {
	if g.names == nil {
		g.names = newNameAllocator()
	}
	for _, decl := range g.abi.Declarations {
		g.names.reservePackage(exportedName(decl.Name))
	}
	g.contractName = g.names.uniquePackage(g.abi.ContractName, "Contract")
	g.contractAPIName = g.names.uniquePackage("ContractAPI", "ContractAPI")
	g.contractConstructorName = g.names.uniquePackage("New"+g.contractName, "NewContract")
	g.names.reserveReceiverMethod("Address")
}

func uniqueName(name string, used map[string]bool) string {
	if name == "" {
		name = "arg"
	}
	if goKeywords[name] {
		name += "_"
	}
	base := name
	for i := 2; used[name]; i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	used[name] = true
	return name
}

func uniqueParamName(name string, fallback string, used map[string]bool) string {
	if name == "" {
		name = fallback
	}
	if goPredeclared[name] {
		name += "Arg"
	}
	return uniqueName(name, used)
}

func (g *generator) uniqueGetMethodName(name string) string {
	return g.names.uniqueReceiverMethod(getMethodReceiverName(name), "RunMethod")
}

func getMethodReceiverName(name string) string {
	base := exportedName(name)
	if base == "" {
		return "RunMethod"
	}
	if strings.HasPrefix(base, "RunMethod") {
		return base
	}
	return "RunMethod" + base
}

func (g *generator) uniqueTypeName(name string) string {
	return g.names.uniquePackage(name, "Result")
}

var commonInitialismList = []string{
	"HTTPS",
	"HTTP",
	"JSON",
	"DNS",
	"NFT",
	"TON",
	"ABI",
	"API",
	"BOC",
	"TLB",
	"TVM",
	"URI",
	"URL",
	"ID",
}

var commonInitialisms = map[string]bool{
	"ABI":   true,
	"API":   true,
	"BOC":   true,
	"DNS":   true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"JSON":  true,
	"NFT":   true,
	"TLB":   true,
	"TON":   true,
	"TVM":   true,
	"URI":   true,
	"URL":   true,
}

var goKeywords = map[string]bool{
	"break":       true,
	"default":     true,
	"func":        true,
	"interface":   true,
	"select":      true,
	"case":        true,
	"defer":       true,
	"go":          true,
	"map":         true,
	"struct":      true,
	"chan":        true,
	"else":        true,
	"goto":        true,
	"package":     true,
	"switch":      true,
	"const":       true,
	"fallthrough": true,
	"if":          true,
	"range":       true,
	"type":        true,
	"continue":    true,
	"for":         true,
	"import":      true,
	"return":      true,
	"var":         true,
}

var goPredeclared = map[string]bool{
	"any":        true,
	"append":     true,
	"bool":       true,
	"byte":       true,
	"cap":        true,
	"close":      true,
	"comparable": true,
	"complex":    true,
	"complex64":  true,
	"complex128": true,
	"copy":       true,
	"delete":     true,
	"error":      true,
	"false":      true,
	"float32":    true,
	"float64":    true,
	"imag":       true,
	"int":        true,
	"int8":       true,
	"int16":      true,
	"int32":      true,
	"int64":      true,
	"iota":       true,
	"len":        true,
	"make":       true,
	"new":        true,
	"nil":        true,
	"panic":      true,
	"print":      true,
	"println":    true,
	"real":       true,
	"recover":    true,
	"rune":       true,
	"string":     true,
	"true":       true,
	"uint":       true,
	"uint8":      true,
	"uint16":     true,
	"uint32":     true,
	"uint64":     true,
	"uintptr":    true,
}
