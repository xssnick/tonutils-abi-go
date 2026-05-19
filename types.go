package main

import "fmt"

type typeInfo struct {
	GoType       string
	TLBTag       string
	Supported    bool
	Reason       string
	Kind         string
	Bits         int
	Interface    bool
	StackExpr    func(string) string
	StackErrExpr func(string) string
	StackErr     bool
	ResultDecode func(target string, index uint, errReturn string) []string
	Zero         string
}

func (g *generator) intType(bits int, signed bool, tlb bool) typeInfo {
	goType := intGoType(bits, signed)
	if goType == "*big.Int" {
		g.useImport("math/big")
	}

	info := typeInfo{
		GoType:    goType,
		TLBTag:    fmt.Sprintf("## %d", bits),
		Supported: true,
		Kind:      "int",
		Bits:      bits,
		Zero:      zeroValue(goType),
	}
	if tlb {
		return info
	}

	info.StackExpr = func(name string) string {
		if goType == "*big.Int" {
			return name
		}
		return fmt.Sprintf("%s(%s)", goType, name)
	}
	return info
}

func unsupported(reason string) typeInfo {
	return typeInfo{
		GoType:    "any",
		Supported: false,
		Reason:    reason,
		StackExpr: func(name string) string { return name },
		Zero:      "nil",
	}
}

func unsupportedStack(reason string) typeInfo {
	info := unsupported(reason)
	info.StackExpr = func(name string) string { return name }
	return info
}
