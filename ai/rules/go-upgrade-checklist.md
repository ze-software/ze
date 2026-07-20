# Go Compiler Upgrade Checklist

**When:** Every Go compiler version bump (go.mod `go` directive change or toolchain update).
**Severity:** advisory

## textbuf.noescape vs strings.Builder

`internal/core/textbuf/textbuf.go` uses a `noescape` function identical to
the technique `strings.Builder` uses via `abi.NoEscape` to prevent
self-referential slices from escaping to the heap.

On every Go update:

1. Read `$(go env GOROOT)/src/strings/builder.go`, find `copyCheck`.
2. Read `$(go env GOROOT)/src/internal/abi/escape.go`, find `NoEscape`.
3. Compare against `internal/core/textbuf/textbuf.go` `noescape` + `inlineSlice`.
4. If the stdlib changed technique, update ours to match.
5. Verify: `go build -gcflags='-m=2' -o bin/escape-test ./internal/core/textbuf/ 2>&1 | grep 'moved to heap'`
   should NOT show `b` escaping for stack-local Buffer usage.

If the Go team removes `NoEscape` or changes escape analysis to see through
the `uintptr` round-trip, the inline array optimization breaks and
`var b Buffer` reverts to heap allocation. This is not a correctness bug
(the code still works), but a performance regression.
