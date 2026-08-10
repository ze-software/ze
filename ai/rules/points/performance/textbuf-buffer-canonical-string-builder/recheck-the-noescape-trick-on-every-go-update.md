---
kind: directive
level: MUST
stage:
---
**Go compiler review gate:** The `noescape` trick mirrors `strings.Builder`
in `$(go env GOROOT)/src/strings/builder.go`. On every Go compiler update, our
`noescape` + `inlineSlice` MUST be compared against the stdlib `copyCheck` +
`abi.NoEscape`. If the stdlib changes technique, ours MUST be updated to
match. It MUST be verified with `go build -gcflags='-m=2'` that `var b Buffer`
does not show `moved to heap`.
