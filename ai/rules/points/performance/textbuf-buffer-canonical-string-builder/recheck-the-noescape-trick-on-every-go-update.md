---
kind: directive
level:
stage:
---
**Go compiler review gate:** The `noescape` trick mirrors `strings.Builder`
in `$(go env GOROOT)/src/strings/builder.go`. On every Go compiler update,
compare our `noescape` + `inlineSlice` against the stdlib `copyCheck` +
`abi.NoEscape`. If the stdlib changes technique, update ours to match.
Verify with `go build -gcflags='-m=2'` that `var b Buffer` does not show
`moved to heap`.
