# tonutils-abi-go

`tonutils-abi-go` generates Go wrappers from Tolk ABI JSON files.

Generated wrappers are built on top of
[`github.com/xssnick/tonutils-go`](https://github.com/xssnick/tonutils-go) and
include typed contract get-methods, TLB structs, dictionaries, constants, and
contract exit-code errors.

## Install

Install the CLI with Go:

```bash
go install github.com/xssnick/tonutils-abi-go@latest
```

## CLI Usage

Generate a wrapper from an ABI file:

```bash
tonutils-abi-go -abi ./path/to/Contract.abi.json -out ./contract_gen.go -package mycontract
```

If `-out` is omitted, generated Go code is written to stdout:

Useful flags:

- `-abi`: path to Tolk ABI JSON.
- `-out`: output Go file. Stdout is used when empty.
- `-package`: generated Go package name. Default is `wrappers`.
- `-strict`: fail generation when unsupported ABI constructs would otherwise be emitted as TODO diagnostics.

The CLI writes human-readable logs to stderr. Successful file generation prints
the absolute output path:

```text
15:21:38 INF generated file file=/abs/path/jetton_v2_minter_gen.go
```

Warnings are also printed when generation leaves TODO diagnostics or when a type
has custom pack/unpack hooks that must be assigned manually:

```text
15:21:38 WRN custom pack/unpack must be assigned manually functions="SetMyTypeLoadFromCell, SetMyTypeToCell" type=MyType
```

## Generated Code

For a contract named `Minter`, the wrapper exposes:

```go
type ContractAPI interface {
    RunGetMethodByID(ctx context.Context, blockInfo *ton.BlockIDExt, addr *address.Address, methodID uint64, params ...any) (*ton.ExecutionResult, error)
}

func NewMinter(api ContractAPI, addr *address.Address) *Minter
func (c *Minter) Address() *address.Address
func (c *Minter) RunMethodGetJettonData(ctx context.Context, block *ton.BlockIDExt) (*JettonDataReply, error)
```
And other methods for each get method and const, each struct for serialization will also be generated.

Use it with any API client that implements `ContractAPI`, including
`*ton.APIClient` from `tonutils-go`:

```go
addr := address.MustParseAddr("EQ...")
minter := wrappers.NewMinter(api, addr)

data, err := minter.RunMethodGetJettonData(ctx, block)
if err != nil {
    return err
}
fmt.Println(data.TotalSupply)
```

## Sending Messages

Incoming/internal messages are generated as regular Go structs with TLB tags.
Create the struct, serialize it with `tlb.ToCell`, and pass the resulting cell
as the wallet message body.

Example: send a jetton transfer message generated from a wallet ABI:

```go
package main

import (
    "context"
    "time"

    "github.com/xssnick/tonutils-go/address"
    "github.com/xssnick/tonutils-go/tlb"
    "github.com/xssnick/tonutils-go/ton/wallet"
    "github.com/xssnick/tonutils-go/tvm/cell"

    "your/module/wrappers"
)

func sendJettonTransfer(
    ctx context.Context,
    w *wallet.Wallet,
    jettonWallet *address.Address,
    recipient *address.Address,
    responseAddress *address.Address,
) error {
    body, err := tlb.ToCell(wrappers.AskToTransfer{
        QueryID:           uint64(time.Now().UnixNano()),
        JettonAmount:      tlb.MustFromDecimal("10.755", 6),
        TransferRecipient: recipient,
        SendExcessesTo:    responseAddress,
        CustomPayload:     nil,
        ForwardTONAmount:  tlb.MustFromTON("0.01"),
        ForwardPayload:    wrappers.ForwardPayloadRemainder(cell.BeginCell().EndCell()),
    })
    if err != nil {
        return err
    }

    return w.Send(ctx, wallet.SimpleMessage(jettonWallet, tlb.MustFromTON("0.05"), body), true)
}
```

The same pattern works for any generated incoming message type:

```go
body, err := tlb.ToCell(wrappers.SomeGeneratedMessage{
    QueryID: 1,
})
if err != nil {
    return err
}

msg := wallet.SimpleMessage(contractAddress, tlb.MustFromTON("0.1"), body)
return w.Send(ctx, msg, true)
```

## Deploying Contracts

For deployment, decode compiled code from a base64 BOC, serialize the generated
storage struct into a data cell, and pass both cells to the wallet deploy helper.

```go
package main

import (
    "context"
    "encoding/base64"

    "github.com/xssnick/tonutils-go/address"
    "github.com/xssnick/tonutils-go/tlb"
    "github.com/xssnick/tonutils-go/ton/wallet"
    "github.com/xssnick/tonutils-go/tvm/cell"

    "your/module/wrappers"
)

func deployMinter(
    ctx context.Context,
    w *wallet.Wallet,
    codeBOCBase64 string,
    walletCodeBOCBase64 string,
    admin *address.Address,
) (*address.Address, error) {
    code, err := cellFromBase64BOC(codeBOCBase64)
    if err != nil {
        return nil, err
    }
    walletCode, err := cellFromBase64BOC(walletCodeBOCBase64)
    if err != nil {
        return nil, err
    }

    data, err := tlb.ToCell(wrappers.MinterStorage{
        TotalSupply:      tlb.MustFromDecimal("1000000", 6),
        AdminAddress:     admin,
        NextAdminAddress: nil,
        JettonWalletCode: walletCode,
        MetadataURI:      "https://example.com/jetton.json",
    })
    if err != nil {
        return nil, err
    }

    addr, _, _, err := w.DeployContractWaitTransaction(
        ctx,
        tlb.MustFromTON("0.05"),
        cell.BeginCell().EndCell(),
        code,
        data,
    )
    if err != nil {
        return nil, err
    }
    return addr, nil
}

func cellFromBase64BOC(raw string) (*cell.Cell, error) {
    boc, err := base64.StdEncoding.DecodeString(raw)
    if err != nil {
        return nil, err
    }
    return cell.FromBOC(boc)
}
```

Generated files may import `tonutils-go`; run this in the target module after
generation:

```bash
go mod tidy
```

## Programmatic Usage

The generator can also be used as a library:

```go
package main

import (
    "os"

    "github.com/xssnick/tonutils-abi-go/gen"
)

func main() {
    result, err := gen.GenerateFile("Counter.abi.json", gen.Options{
        PackageName: "wrappers",
        Strict:      true,
    })
    if err != nil {
        panic(err)
    }

    for _, diagnostic := range result.Diagnostics {
        println(diagnostic.String())
    }

    if err := os.WriteFile("counter_gen.go", result.Source, 0o644); err != nil {
        panic(err)
    }
}
```

For stream or in-memory ABI JSON:

```go
result, err := gen.Generate(reader, gen.Options{PackageName: "wrappers"})
result, err := gen.GenerateJSON(data, gen.Options{PackageName: "wrappers"})
```

Strict failures are returned as `*gen.DiagnosticError`. Non-strict schema and
unsupported-construct details are available in `result.Diagnostics`.

If `result.CustomSerializers` is not empty, the generated code contains setter
functions for manual serializers. Call the listed functions during init or
application setup before using the corresponding generated types.

## Supported ABI Features

The generator currently supports:

- contract wrapper types with `RunGetMethodByID`;
- typed get-method parameters and results;
- TLB aliases, enums, structs, and generic instantiations;
- constants as `Const...` functions;
- contract exit-code errors and error mapping;
- integers, coins, bools, strings, bits, bytes, addresses, cells, slices, builders, arrays, nullable types, tensors, shaped tuples, unions, and lisp lists;
- `mapKV` dictionaries with fixed-width flat keys;
- `cellOf` stack values decoded through `tlb.Parse`;
- custom pack/unpack hooks with explicit generated setter functions.

Unsupported constructs are reported as diagnostics and, unless `-strict` is set,
preserved in generated code as TODO comments where possible.
