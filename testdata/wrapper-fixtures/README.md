These ABI fixtures are copied from imported wrapper test contracts by compiling
them once with Acton:

```sh
acton compile <fixture>.tolk \
  --abi tugen/testdata/wrapper-fixtures/<name>.abi.json
```

They let the Go generator test against stable ABI inputs without recompiling
source fixtures during `go test`.

`small.tolk` is intentionally absent: the current Acton compiler rejects its
old `assert(1)` syntax before ABI emission.
