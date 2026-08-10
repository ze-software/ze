---
kind: directive
level: MUST
stage:
---
**After a Go update, you MUST:**
1. Read `$(go env GOROOT)/src/strings/builder.go`, find `copyCheck`.
2. Read `$(go env GOROOT)/src/internal/abi/escape.go`, find `NoEscape`.
3. Compare against `internal/core/textbuf/textbuf.go` `noescape` + `inlineSlice`.
4. If the stdlib changed technique, update ours to match.
5. Verify: `go build -gcflags='-m=2' -o bin/escape-test ./internal/core/textbuf/ 2>&1 | grep 'moved to heap'` SHOULD NOT show `b` escaping for stack-local Buffer usage.
