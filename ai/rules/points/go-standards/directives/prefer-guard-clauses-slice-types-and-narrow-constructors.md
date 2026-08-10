---
kind: directive
level: SHOULD
stage:
---
- **Early return over nested else.** Handle the error / edge case and return immediately rather than wrapping the happy path in an else block. Deep nesting makes control flow hard to follow; a sequence of guard clauses followed by the main logic reads linearly. Applies to `if err != nil { return }`, validation guards, and nil checks alike.
- **Drain `time.NewTimer` on Stop.** If you use `time.NewTimer` directly and call `Stop()`, drain `.C` in a non-blocking select before `Reset()`, otherwise the next `Reset()` sees a stale firing and behaves wrong. The BGP FSM uses `clock.Timer` + `AfterFunc` (callback-based, no channel to drain) and needs no helper. The only `time.NewTimer` site in the BGP tree, `internal/component/bgp/plugins/rs/worker.go`, already has a `drainTimer` helper: promote it or copy the pattern when adding a new `time.NewTimer` site. Prefer `AfterFunc` where possible.
- **Slice type with methods beats a wrapping struct.** When the only state is `[]*T`, declare `type Foo []*T` with methods on the slice rather than `type Foo struct { items []*T }`. `Foo{}` is the empty value, `append(foo, x)` works, iteration works, JSON marshaling works, and you never write a `NewFoo`/`Items()` pair.
- **Family of narrow constructors beats a god `New(Config)`.** When most callers care about one axis of a type, give each common construction pattern its own named constructor (`NewFooWithPrefixes`, `NewFooWithProtocols`) rather than a functional-options pile or a config struct with half the fields nil.
- **Table-driven tests with `t.Run(name, ...)`.** When a test file has more than two similar cases, prefer a single `[]struct { name string; ... }` table iterated with `t.Run(tt.name, ...)`. Each case gets its own `-v` line, each case is self-contained, and you can focus one failing case.
- **Honest TODO comments over silent gaps.** When a function is incomplete (especially deep-comparison `equal()` methods that can pass superficially and silently mis-match on edge cases), write a visible `// TODO:` rather than a panic, a `//nolint`, or nothing. A reviewer SHOULD see the gap in a diff.
