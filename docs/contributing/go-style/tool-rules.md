# Ze Go Style: the rules a tool enforces

The detail behind "The rules a tool enforces" and "By the numbers" in
[the Go style core](../ze-go-style.md).

The edit-time check is
`writeGoPatterns`, which refuses a `Write`, `Edit` or `MultiEdit` of shipped Go
(not `_test.go`) that matches a pattern. Panic calls are judged in the complete
proposed content, not in a replacement fragment that may be inside a quote.
<!-- source: .golangci.yml -- revive rules -->
<!-- source: internal/le/hookruntime/writeedit.go -- writeGoPatterns -->

| Rule | Tool | What it refuses |
|------|------|-----------------|
| Guard, handle, return | revive `early-return`, `indent-error-flow`, `superfluous-else` | An `else` that a guard makes unnecessary |
| One fact per guard | `./le arch compound-guard check` | A terminating `if a \|\| b` on a changed line |
| Refuse, never default | `./le arch refuse-by-default check` | On a changed line: an unmatched switch value or a miss answered as a value, an ADD-PATH identifier compared with zero, a state answered from a literal, an error dropped with only a log line below WARN, or a `refuse-by-default:` annotation that names no guard or names the function it sits in (see "A zero value is never an answer") |
| One layer owns each error prefix | `./le arch error-wrap check` | On a changed line: a `fmt.Errorf` whose segment before `%w` (verbs compared as one mark) is already carried by the error it wraps (see "Every error is handled") |
| State an invariant positively | ruleguard `negatedElse` | `if !c` or `if a != b` with an `else`. `err != nil` and the failure form of a bound stay allowed |
| Only a named panic | `writeGoPatterns` | Each `panic(` without its own literal `BUG`, `unreachable`, `not implemented`, `unimplemented`, `TODO` or `impossible` prefix, and a malformed actual call even with a permitted prefix. Comments and quoted examples are not calls; one allowed call never exempts another. Peer reachability remains a reader check (see "Assertions, in a language that has none") |
| No unchecked error | `errcheck` (type assertions too, `check-blank: false`), `forcetypeassert`, `nilerr`, `errorlint` | A call whose error result is ignored, an unchecked assertion, `return nil` beside a live `err`, an error compared with `==` or wrapped without `%w`. A blank discard `f, _ := open()` passes: see "Every error is handled" |
| No `nil, nil` answer | `nilnil` | A `(pointer, error)` function that returns neither |
| Every enum value handled | `exhaustive`, globally, including aliases; `default` does not satisfy named-member coverage | Missing enum members in any unignored value switch, with or without a `default`; not sealed-interface type-switch omissions. C/O/P provenance and fallback policy remain reader checks |
| A `//nolint` names its linter and its reason | `nolintlint`, `writeGoPatterns` | `//nolint`, or `//nolint:x` with no `// reason` |
| No legacy logger | `forbidigo`, `writeGoPatterns` | `log.Print*`, `log.Fatal*`, `log.Panic*`: use `slog` |
| No allocating formatter | `writeGoPatterns` | `fmt.Sprintf`, `fmt.Fprintf`, `fmt.Printf`, `strconv.FormatInt`, `strconv.FormatUint`: use `textbuf.Buffer` |
| No anonymous goroutine | `writeGoPatterns` | `go func(` |
| No exit from a handler | `writeGoPatterns` | `os.Exit(` outside `main.go` and `register.go` |
| Registration only in `register*.go` | `writeGoPatterns` | An `init()` that registers, subscribes or hooks in another file |
| No switch dispatch | `writeGoPatterns` | `switch args[0]` |
| One responsibility per function | `writeGoPatterns` (warning) | An exported function named `...And...` |
| Pass a large struct by pointer | gocritic `hugeParam` (288 bytes), `rangeValCopy` (160 bytes) | A value parameter or range copy above the size |
| `slices.Sort` | ruleguard `modernSort` | `sort.Strings`, `sort.Ints`, `sort.Float64s`, which compare through `sort.Interface` |
| `crashlog.Exec` | ruleguard `crashlogExec` | An `execve` past the crash-capture flush |
| `commandtest` in tests only | ruleguard `commandtestInTestsOnly` | `commandtest.Args` or `commandtest.Must` outside a `_test.go` file: a validated value the model never judged |
| A comment is a sentence | `godot` | A declaration comment with no full stop |
| US spelling | `misspell` | UK spellings |
| Modern Go | `modernize`, `intrange` | A pre-generics or pre-range-over-int idiom |
<!-- source: .golangci/ruleguard/modern.go -- negatedElse -->

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
