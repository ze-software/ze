# Ze Go Style: the rules a tool enforces

The detail behind "The rules a tool enforces" and "By the numbers" in
[the Go style core](../ze-go-style.md).

The edit-time check is `writeGoPatterns`, which delegates to the shared
diff-scoped AST engine (`internal/le/go/style`) also used by `./le commit create`
(`CheckAddedGoStyle`) and the fast, subagent-callable `./le go style check`.
Panic calls and banned Go patterns are judged on the complete proposed file,
comparing newly added lines against what the file already held so comments,
string literals and untouched legacy lines never false-positive.
<!-- source: .golangci.yml -- revive rules -->
<!-- source: internal/le/go/style/style.go -- CheckFormat, CheckAdded -->
<!-- source: internal/le/go/style/actions.go -- CheckTreePaths, CheckChangedTree -->
<!-- source: internal/le/hookruntime/writeedit.go -- writeGoPatterns -->

| Rule | Tool | What it refuses |
|------|------|-----------------|
| Formatting and local import grouping | `gofmt`, `goimports`, `./le go style check`, `./le commit create` | Unformatted Go files, or `github.com/ze-software/ze` imports mixed with non-local groups |
| Guard, handle, return | revive `early-return`, `indent-error-flow`, `superfluous-else` | An `else` that a guard makes unnecessary |
| One fact per guard | `./le arch compound-guard check`, `./le go style check`, `./le commit create` | A newly added terminating `if a \|\| b` |
| Refuse, never default | `./le arch refuse-by-default check`, `./le go style check`, `./le commit create` | On a changed line: an unmatched switch value or a miss answered as a value, an ADD-PATH identifier compared with zero, a state answered from a literal, an error dropped with only a log line below WARN, or a `refuse-by-default:` annotation that names no guard or names the function it sits in (see "A zero value is never an answer") |
| One layer owns each error prefix | `./le arch error-wrap check`, `./le go style check`, `./le commit create` | On a changed line: a `fmt.Errorf` whose segment before `%w` (verbs compared as one mark) is already carried by the error it wraps (see "Every error is handled") |
| State an invariant positively | ruleguard `negatedElse` | `if !c` or `if a != b` with an `else`. `err != nil` and the failure form of a bound stay allowed |
| Only a named panic | `writeGoPatterns` | Each `panic(` without its own literal `BUG`, `unreachable`, `not implemented`, `unimplemented`, `TODO` or `impossible` prefix, and a malformed actual call even with a permitted prefix. Comments and quoted examples are not calls; one allowed call never exempts another. Peer reachability remains a reader check (see "Assertions, in a language that has none") |
| No unchecked error | `errcheck` (type assertions too, `check-blank: false`), `forcetypeassert`, `nilerr`, `errorlint` | A call whose error result is ignored, an unchecked assertion, `return nil` beside a live `err`, an error compared with `==` or wrapped without `%w`. A blank discard `f, _ := open()` passes: see "Every error is handled" |
| No `nil, nil` answer | `nilnil` | A `(pointer, error)` function that returns neither |
| Every enum value handled | `exhaustive`, globally, including aliases; `default` does not satisfy named-member coverage | Missing enum members in any unignored value switch, with or without a `default`; not sealed-interface type-switch omissions. C/O/P provenance and fallback policy remain reader checks |
| A `//nolint` names its linter and its reason | `nolintlint`, `gostyle` (`writeGoPatterns`, `./le commit create`, `./le go style check`) | Bare `//nolint`, or `//nolint:x` with no `// reason` |
| No legacy logger | `forbidigo`, `gostyle` | `log.Print*`, `log.Fatal*`, `log.Panic*`: use `slog` |
| No allocating formatter | `gostyle` | `fmt.Sprintf`, `fmt.Fprintf`, `fmt.Printf` (outside `cmd/` and `hookruntime/` for `Fprintf`), `strconv.FormatInt`, `strconv.FormatUint`: use `textbuf.Buffer` |
| No direct Ze env access | ruleguard `zeEnvAccess`, `gostyle` | `os.Getenv`, `os.LookupEnv` or `os.Setenv` on a `ZE_*` or `ze.*` key in shipped product code outside `internal/core/env` |
| Never compare with `String()` | `gostyle` | Newly added `x.String() == y` or `x.String() != y` |
| No anonymous goroutine | `gostyle` | `go func(` |
| No exit outside entrypoints | ruleguard `noHandlerOsExit`, `gostyle` | `os.Exit(` outside entrypoints, `pprof*.go`, `env`, `crashlog` and test fixtures; `init()` registration failures use `panic("BUG: ...")` |
| Registration only in `register*.go` | `gostyle` | An `init()` outside `register*.go`, `init.go` or `all*.go` |
| No switch dispatch | `gostyle` | `switch args[0]` |
| No fake `BufHandle` | ruleguard `noFakeBufHandle`, `gostyle` | `BufHandle{..., Buf: make(...)}` |
| One responsibility per function | `writeGoPatterns` (warning) | An exported function named `...And...` |
| Pass a large struct by pointer | gocritic `hugeParam` (288 bytes), `rangeValCopy` (160 bytes) | A value parameter or range copy above the size |
| `slices.Sort` | ruleguard `modernSort` | `sort.Strings`, `sort.Ints`, `sort.Float64s`, which compare through `sort.Interface` |
| `crashlog.Exec` | ruleguard `crashlogExec` | An `execve` past the crash-capture flush |
| `commandtest` in tests only | ruleguard `commandtestInTestsOnly` | `commandtest.Args` or `commandtest.Must` outside a `_test.go` file: a validated value the model never judged |
| A comment is a sentence | `godot` | A declaration comment with no full stop |
| US spelling | `misspell` | UK spellings (except the owner-approved `licence` spelling) |
| Modern Go | `modernize`, `intrange` | A pre-generics or pre-range-over-int idiom |
<!-- source: .golangci/ruleguard/modern.go -- negatedElse, zeEnvAccess -->

The other enabled linters (`govet`, `staticcheck`, `unused`, `gosec`,
`unconvert`, `unparam`, `nakedret`, `prealloc`, `noctx`, `bodyclose`, `dupl`,
`goconst`, `tparallel`, `wastedassign`, `ineffassign`, gocritic's diagnostic,
style and performance tags) teach through their own messages.

## By the numbers

The settings the core does not list:

| Setting | Value |
|---------|-------|
| Linting | `golangci-lint` MUST pass. Do not disable a linter. Fix the finding |
| Formatting | `gofmt` and `goimports`. Ze imports come last, under the local prefix |
| A `//nolint` | Names the linter and carries the reason on the same line |
| Indentation | Tabs, because `gofmt` writes tabs. Every other file type is spaces, and `.editorconfig` carries the width |
| Test file length | No threshold. A table of cases grows with coverage |
| Function length | One screen, advisory. No gate counts the lines |
| Assertion density | Ze counts nothing. A `panic("BUG:")` marks a state that a Ze defect alone can reach |

<!-- source: .golangci.yml -- linters, formatters -->
<!-- source: .golangci.yml -- gocritic hugeParam sizeThreshold -->
<!-- source: .editorconfig -- indentation per file type -->

A Ze idiom that no linter knows goes in `.golangci/ruleguard/modern.go`, which
gocritic's `ruleguard` loads. It then reaches every place the linter reaches,
with no new gate. Give each rule a `Report` line that states the cost and a
`Suggest` line that gives the replacement. `--fix` does not edit imports, so a
mass rewrite runs `gofmt -r` and then `goimports`. No path gate checks a
reference into that dot directory, so when you rename the file, correct this
page by hand.
