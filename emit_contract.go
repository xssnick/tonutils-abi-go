package main

import (
	"bytes"
	"fmt"
)

func (g *generator) writeContract(dst *bytes.Buffer) {
	contractName := g.contractName

	g.useImport("context")
	g.useImport("github.com/xssnick/tonutils-go/address")
	g.useImport("github.com/xssnick/tonutils-go/ton")

	apiName := g.contractAPIName
	fmt.Fprintf(dst, "type %s interface {\n", apiName)
	dst.WriteString("\tRunGetMethodByID(ctx context.Context, blockInfo *ton.BlockIDExt, addr *address.Address, methodID uint64, params ...any) (*ton.ExecutionResult, error)\n")
	dst.WriteString("}\n\n")

	fmt.Fprintf(dst, "type %s struct {\n", contractName)
	fmt.Fprintf(dst, "\tapi %s\n", apiName)
	dst.WriteString("\taddr *address.Address\n")
	dst.WriteString("}\n\n")

	fmt.Fprintf(dst, "func %s(api %s, addr *address.Address) *%s {\n", g.contractConstructorName, apiName, contractName)
	fmt.Fprintf(dst, "\treturn &%s{api: api, addr: addr}\n", contractName)
	dst.WriteString("}\n\n")

	fmt.Fprintf(dst, "func (c *%s) Address() *address.Address {\n", contractName)
	dst.WriteString("\treturn c.addr\n")
	dst.WriteString("}\n\n")
}
