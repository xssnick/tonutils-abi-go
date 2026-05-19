package main

import (
	"fmt"
	"strings"
)

type DiagnosticCode string

const (
	DiagnosticUnsupportedConstruct DiagnosticCode = "unsupported_construct"
	DiagnosticSchema               DiagnosticCode = "schema"
)

type Diagnostic struct {
	Code    DiagnosticCode
	Subject string
	Message string
}

func (d Diagnostic) String() string {
	if d.Subject == "" {
		return d.Message
	}
	if d.Message == "" {
		return d.Subject
	}
	return d.Subject + ": " + d.Message
}

type DiagnosticError struct {
	Strict      bool
	Diagnostics []Diagnostic
}

type CustomSerializer struct {
	TypeName               string
	LoadFromCellSetterName string
	ToCellSetterName       string
}

func (e *DiagnosticError) Error() string {
	if len(e.Diagnostics) == 0 {
		return "strict mode: diagnostics failed"
	}
	if len(e.Diagnostics) == 1 {
		return fmt.Sprintf("strict mode: %s", e.Diagnostics[0])
	}

	lines := make([]string, 0, len(e.Diagnostics))
	for _, diagnostic := range e.Diagnostics {
		lines = append(lines, diagnostic.String())
	}
	return "strict mode: diagnostics failed:\n- " + strings.Join(lines, "\n- ")
}

func newStrictDiagnosticError(diagnostics []Diagnostic) error {
	return &DiagnosticError{
		Strict:      true,
		Diagnostics: append([]Diagnostic(nil), diagnostics...),
	}
}
