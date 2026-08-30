---
kind: directive
level: MUST
stage:
---
**On every Go compiler update, `noescape` and `inlineSlice` MUST be compared
against the stdlib `copyCheck` and `abi.NoEscape` in
`$(go env GOROOT)/src/strings/builder.go`, and MUST be updated to match when
the stdlib changes technique.** It MUST then be verified with
`go build -gcflags='-m=2'` that `var b Buffer` does not show `moved to heap`.
Why the trick is there is `docs/architecture/textbuf-string-building.md`.
