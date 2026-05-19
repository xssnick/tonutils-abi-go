package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type generatedThrownError struct {
	VarName string
	Message string
	Code    int64
}

func (g *generator) hasThrownErrors() bool {
	return len(g.abi.ThrownErrors) > 0
}

func (g *generator) writeThrownErrors(dst *bytes.Buffer) {
	if !g.hasThrownErrors() {
		return
	}

	specs := g.thrownErrorSpecs()
	if len(specs) == 0 {
		return
	}

	g.useImport("errors")

	dst.WriteString("var (\n")
	for _, spec := range specs {
		fmt.Fprintf(dst, "\t%s = errors.New(%q)\n", spec.VarName, spec.Message)
	}
	dst.WriteString(")\n\n")

	dst.WriteString("func mapContractError(err error) error {\n")
	dst.WriteString("\tif err == nil {\n")
	dst.WriteString("\t\treturn nil\n")
	dst.WriteString("\t}\n")
	dst.WriteString("\tvar execErr ton.ContractExecError\n")
	dst.WriteString("\tif errors.As(err, &execErr) {\n")
	dst.WriteString("\t\tif mapped := contractErrorByExitCode(execErr.Code); mapped != nil {\n")
	dst.WriteString("\t\t\treturn errors.Join(mapped, err)\n")
	dst.WriteString("\t\t}\n")
	dst.WriteString("\t}\n")
	dst.WriteString("\treturn err\n")
	dst.WriteString("}\n\n")

	g.writeContractErrorByExitCode(dst, specs)
}

func (g *generator) thrownErrorSpecs() []generatedThrownError {
	specs := make([]generatedThrownError, 0, len(g.abi.ThrownErrors))
	for _, thrown := range g.abi.ThrownErrors {
		name := thrownErrorVarName(thrown)
		if name == "" {
			continue
		}
		name = g.names.uniquePackage(name, "ErrCode")
		specs = append(specs, generatedThrownError{
			VarName: name,
			Message: thrownErrorMessage(thrown),
			Code:    thrown.Code,
		})
	}
	return specs
}

func (g *generator) writeContractErrorByExitCode(dst *bytes.Buffer, specs []generatedThrownError) {
	byCode := map[int32][]string{}
	var codes []int32
	for _, spec := range specs {
		code, ok := thrownErrorInt32Code(spec.Code)
		if !ok {
			continue
		}
		if _, exists := byCode[code]; !exists {
			codes = append(codes, code)
		}
		byCode[code] = append(byCode[code], spec.VarName)
	}

	dst.WriteString("func contractErrorByExitCode(code int32) error {\n")
	dst.WriteString("\tswitch code {\n")
	for _, code := range codes {
		fmt.Fprintf(dst, "\tcase %d:\n", code)
		names := byCode[code]
		if len(names) == 1 {
			fmt.Fprintf(dst, "\t\treturn %s\n", names[0])
			continue
		}
		fmt.Fprintf(dst, "\t\treturn errors.Join(%s)\n", strings.Join(names, ", "))
	}
	dst.WriteString("\tdefault:\n")
	dst.WriteString("\t\treturn nil\n")
	dst.WriteString("\t}\n")
	dst.WriteString("}\n\n")
}

func thrownErrorVarName(thrown thrownError) string {
	base := exportedName(thrown.Name)
	if base == "" {
		base = "Code" + thrownErrorCodeName(thrown.Code)
	}
	if !strings.HasPrefix(base, "Err") {
		base = "Err" + base
	}
	return base
}

func thrownErrorMessage(thrown thrownError) string {
	name := strings.TrimSpace(thrown.Name)
	description := strings.TrimSpace(thrown.Description)
	code := strconv.FormatInt(thrown.Code, 10)
	switch {
	case name != "" && description != "":
		return fmt.Sprintf("%s: %s (exit code %s)", name, description, code)
	case description != "":
		return fmt.Sprintf("%s (exit code %s)", description, code)
	case name != "":
		return fmt.Sprintf("%s (exit code %s)", name, code)
	default:
		return fmt.Sprintf("contract exit code %s", code)
	}
}

func thrownErrorCodeName(code int64) string {
	if code < 0 {
		magnitude := uint64(-(code + 1)) + 1
		return "Negative" + strconv.FormatUint(magnitude, 10)
	}
	return strconv.FormatInt(code, 10)
}

func thrownErrorInt32Code(code int64) (int32, bool) {
	const (
		minInt32 = -1 << 31
		maxInt32 = 1<<31 - 1
	)
	if code < minInt32 || code > maxInt32 {
		return 0, false
	}
	return int32(code), true
}
