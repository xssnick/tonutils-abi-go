package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"path"
	"slices"
)

type generatedFile struct {
	body bytes.Buffer
}

type declarationRoles struct {
	tlbAliases   map[string]bool
	tlbEnums     map[string]bool
	tlbStructs   map[string]bool
	stackAliases map[string]bool
	stackEnums   map[string]bool
	stackStructs map[string]bool
}

func newDeclarationRoles() declarationRoles {
	return declarationRoles{
		tlbAliases:   map[string]bool{},
		tlbEnums:     map[string]bool{},
		tlbStructs:   map[string]bool{},
		stackAliases: map[string]bool{},
		stackEnums:   map[string]bool{},
		stackStructs: map[string]bool{},
	}
}

type generator struct {
	abi                  abiFile
	packageName          string
	aliases              map[string]declaration
	enums                map[string]declaration
	structs              map[string]declaration
	structInstantiations map[int]abiStructInstantiation
	aliasInstantiations  map[int]abiAliasInstantiation
	declarationRoles
	stackStructEncoders     map[string]bool
	stackResultDecoders     map[string]bool
	imports                 map[string]bool
	resultTypes             []string
	generatedTypes          []string
	generatedTypeSet        map[string]bool
	generatedTypeNames      map[string]string
	generatedHelperSet      map[string]bool
	genericInstanceNames    map[string]string
	genericInProgress       map[string]bool
	mapTypes                []mapSpec
	mapTypeNames            map[string]string
	mapTypeSet              map[string]bool
	names                   *nameAllocator
	contractName            string
	contractAPIName         string
	contractConstructorName string
	customSerializers       []CustomSerializer
	strict                  bool
	diagnostics             []Diagnostic
	helpers                 helperRegistry
}

type Options struct {
	PackageName string
	Strict      bool
}

type GenerationResult struct {
	Source            []byte
	Diagnostics       []Diagnostic
	CustomSerializers []CustomSerializer
}

func Generate(src io.Reader, opts Options) (GenerationResult, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return GenerationResult{}, fmt.Errorf("read ABI: %w", err)
	}
	return GenerateJSON(data, opts)
}

func GenerateFile(path string, opts Options) (GenerationResult, error) {
	abi, diagnostics, err := loadABIFile(path, opts.Strict)
	if err != nil {
		return GenerationResult{}, err
	}
	if opts.Strict && len(diagnostics) > 0 {
		return GenerationResult{Diagnostics: diagnostics}, newStrictDiagnosticError(diagnostics)
	}
	return generateABI(abi, opts, diagnostics)
}

func GenerateJSON(data []byte, opts Options) (GenerationResult, error) {
	abi, diagnostics, err := parseABIForGeneration(data, opts.Strict)
	if err != nil {
		return GenerationResult{}, err
	}
	if opts.Strict && len(diagnostics) > 0 {
		return GenerationResult{Diagnostics: diagnostics}, newStrictDiagnosticError(diagnostics)
	}
	return generateABI(abi, opts, diagnostics)
}

func generateABI(abi abiFile, opts Options, diagnostics []Diagnostic) (GenerationResult, error) {
	g := newGenerator(abi, opts.PackageName).withStrict(opts.Strict)
	g.diagnostics = append(g.diagnostics, diagnostics...)
	return g.Generate()
}

func newGenerator(abi abiFile, packageName string) *generator {
	g := &generator{
		abi:                  abi,
		packageName:          sanitizePackageName(packageName),
		aliases:              map[string]declaration{},
		enums:                map[string]declaration{},
		structs:              map[string]declaration{},
		structInstantiations: map[int]abiStructInstantiation{},
		aliasInstantiations:  map[int]abiAliasInstantiation{},
		declarationRoles:     newDeclarationRoles(),
		stackStructEncoders:  map[string]bool{},
		stackResultDecoders:  map[string]bool{},
		imports:              map[string]bool{},
		generatedTypeSet:     map[string]bool{},
		generatedTypeNames:   map[string]string{},
		generatedHelperSet:   map[string]bool{},
		genericInstanceNames: map[string]string{},
		genericInProgress:    map[string]bool{},
		mapTypeNames:         map[string]string{},
		mapTypeSet:           map[string]bool{},
		names:                newNameAllocator(),
		helpers:              newHelperRegistry(),
	}

	for _, decl := range abi.Declarations {
		switch decl.Kind {
		case "alias":
			g.aliases[decl.Name] = decl
		case "enum":
			g.enums[decl.Name] = decl
		case "struct":
			g.structs[decl.Name] = decl
		}
	}
	for _, inst := range abi.StructInstantiations {
		g.structInstantiations[inst.TypeIndex] = inst
	}
	for _, inst := range abi.AliasInstantiations {
		g.aliasInstantiations[inst.TypeIndex] = inst
	}

	return g
}

func (g *generator) withStrict(strict bool) *generator {
	g.strict = strict
	return g
}

func (g *generator) Generate() (GenerationResult, error) {
	file, err := g.analyze()
	if err != nil {
		return GenerationResult{
			Diagnostics:       append([]Diagnostic(nil), g.diagnostics...),
			CustomSerializers: append([]CustomSerializer(nil), g.customSerializers...),
		}, err
	}

	src, err := g.render(file)
	return GenerationResult{
		Source:            src,
		Diagnostics:       append([]Diagnostic(nil), g.diagnostics...),
		CustomSerializers: append([]CustomSerializer(nil), g.customSerializers...),
	}, err
}

func (g *generator) analyze() (*generatedFile, error) {
	file := &generatedFile{}

	g.monomorphizeGenerics()
	g.prepareNames()
	g.collectDeclarationRoles()
	g.writeContract(&file.body)
	g.writeDeclarations(&file.body)
	g.writeThrownErrors(&file.body)
	g.writeConstants(&file.body)
	g.writeMethods(&file.body)
	g.writeHelpers(&file.body)

	if err := g.strictDiagnosticError(); err != nil {
		return nil, err
	}
	return file, nil
}

func (g *generator) render(file *generatedFile) ([]byte, error) {
	var src bytes.Buffer
	src.WriteString("// Code generated by tonutils-abi-go; DO NOT EDIT.\n\n")
	fmt.Fprintf(&src, "package %s\n\n", g.packageName)
	g.pruneUnusedImports(file.body.Bytes())
	g.writeImports(&src)
	src.Write(file.body.Bytes())

	formatted, err := format.Source(src.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w\n%s", err, src.String())
	}

	return formatted, nil
}

func (g *generator) writeImports(dst *bytes.Buffer) {
	if len(g.imports) == 0 {
		return
	}

	std := make([]string, 0, len(g.imports))
	external := make([]string, 0, len(g.imports))
	for path := range g.imports {
		if isStdImport(path) {
			std = append(std, path)
		} else {
			external = append(external, path)
		}
	}
	slices.Sort(std)
	slices.Sort(external)

	dst.WriteString("import (\n")
	for _, path := range std {
		fmt.Fprintf(dst, "\t%q\n", path)
	}
	if len(std) > 0 && len(external) > 0 {
		dst.WriteString("\n")
	}
	for _, path := range external {
		fmt.Fprintf(dst, "\t%q\n", path)
	}
	dst.WriteString(")\n\n")
}

func (g *generator) useImport(path string) {
	g.imports[path] = true
}

func (g *generator) pruneUnusedImports(body []byte) {
	if len(g.imports) == 0 {
		return
	}

	file, err := parser.ParseFile(token.NewFileSet(), "", "package "+g.packageName+"\n\n"+string(body), parser.SkipObjectResolution)
	if err != nil {
		return
	}

	usedPackages := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := selector.X.(*ast.Ident); ok {
			usedPackages[ident.Name] = true
		}
		return true
	})

	for importPath := range g.imports {
		if !usedPackages[generatedImportPackage(importPath)] {
			delete(g.imports, importPath)
		}
	}
}

func generatedImportPackage(importPath string) string {
	if importPath == "math/big" {
		return "big"
	}
	return path.Base(importPath)
}

func (g *generator) writeTODO(dst *bytes.Buffer, indent string, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	g.addDiagnostic(Diagnostic{
		Code:    DiagnosticUnsupportedConstruct,
		Message: msg,
	})
	if g.strict {
		return
	}
	fmt.Fprintf(dst, "%s// TODO: %s\n", indent, msg)
}

func (g *generator) addDiagnostic(diagnostic Diagnostic) {
	g.diagnostics = append(g.diagnostics, diagnostic)
}

func (g *generator) strictDiagnosticError() error {
	if !g.strict || len(g.diagnostics) == 0 {
		return nil
	}
	return newStrictDiagnosticError(g.diagnostics)
}
