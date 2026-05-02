# Go Standards

Rationale: `ai/rationale/go-standards.md`

## Required

- Go 1.21+ features (slog, generics)
- `golangci-lint` must pass
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Context as first param: `context.Context`
- Fail-early: propagate parse/config errors immediately, never silently default

## Logging: `log/slog` only

- Engine: `slogutil.Logger("subsystem")`
- Plugins: `slogutil.PluginLogger("name", level)`
- Per-subsystem: `ze.log.<path>=<level>` env vars (hierarchical, most-specific wins)
- Levels: `disabled`, `debug`, `info`, `warn`, `err`
- Config: `environment { log { level warn; bgp.routes debug; } }`
- Priority: CLI flag > env var > config > default (WARN)
- Debug logging is permanent — `logger.Debug()`, never `fmt.Printf`

## Dependencies

Never add new third-party imports (not already in `go.mod`) without asking the user first.

## Environment Variables: `internal/core/env` only

**BLOCKING:** All Ze environment variable access MUST use `env.Get()` / `env.Set()` or typed helpers. Never use `os.Getenv()` or `os.Setenv()` for Ze-specific vars.

| Getters | Use |
|---------|-----|
| `env.Get("ze.foo.bar")` | String lookup (case-insensitive, dot/underscore agnostic) |
| `env.GetInt("ze.foo", 0)` | Integer with default |
| `env.GetInt64("ze.foo", 0)` | Int64 with default |
| `env.GetBool("ze.foo", false)` | Boolean (true/false/1/0) with default |
| `env.IsEnabled("ze.foo")` | Enabling check (1/true/yes/on/enable/enabled) |
| `env.GetDuration("ze.foo", 5*time.Second)` | Duration with default |

| Setters | Use |
|---------|-----|
| `env.Set("ze.foo", "val")` | String (updates cache + os env) |
| `env.SetInt("ze.foo", 42)` | Integer |
| `env.SetBool("ze.foo", true)` | Boolean ("true"/"false") |

**Cache:** Built once from `os.Environ()` on first `Get()`. `Set*()` updates both cache and os env. Tests that use `os.Setenv` directly must call `env.ResetCache()`.

**Registration required:** Every env var must be registered via package-level `var _ = env.MustRegister(...)`. Calling `env.Get()` with an unregistered key aborts the process.

**Registration flags:**

| Flag | Meaning |
|------|---------|
| `Private: true` | Hidden from `ze env list` and autocomplete |
| `Secret: true` | Cleared from OS environment after first `Get()` (value stays in cache) |

**`os.Getenv` IS OK for:** System env vars (`HOME`, `PATH`, `XDG_*`, `NO_COLOR`, `USER`, `SSH_*`).

## Aliased Imports

When two packages in the module share the same name (e.g., `cmd/ze/iface/` and
`internal/component/iface/`), goimports cannot resolve which to use and silently removes
the import. Always use an aliased import in this case:
`ifacepkg "codeberg.org/thomas-mangin/ze/internal/component/iface"`.

Also: add import + usage in the same Edit call to prevent goimports from removing an
"unused" import between edits.

## Scripts: Python Only

Do not use shell/bash for scripts. Use Python. Shell scripts are fragile and hard to debug
for complex orchestration. Precedent: `test/interop/run.py`, `test/interop/interop.py`.

## Style patterns to prefer

Advisory, not a sweep. Adopt opportunistically the next time you are in
the relevant code.

- **Early return over nested else.** Handle the error / edge case and
  return immediately rather than wrapping the happy path in an else
  block. Deep nesting makes control flow hard to follow; a sequence of
  guard clauses followed by the main logic reads linearly. Applies to
  `if err != nil { return }`, validation guards, and nil checks alike.
- **Drain `time.NewTimer` on Stop.** If you use `time.NewTimer` directly
  and call `Stop()`, drain `.C` in a non-blocking select before `Reset()`,
  otherwise the next `Reset()` sees a stale firing and behaves wrong. The
  BGP FSM uses `clock.Timer` + `AfterFunc` (callback-based, no channel to
  drain) and needs no helper. The only `time.NewTimer` site in the BGP
  tree, `internal/component/bgp/plugins/rs/worker.go:379`, already has a
  `drainTimer` helper — promote it or copy the pattern when adding a new
  `time.NewTimer` site. Prefer `AfterFunc` where possible.
- **Slice type with methods beats a wrapping struct.** When the only
  state is `[]*T`, declare `type Foo []*T` with methods on the slice
  rather than `type Foo struct { items []*T }`. `Foo{}` is the empty
  value, `append(foo, x)` works, iteration works, JSON marshaling works,
  and you never write a `NewFoo`/`Items()` pair.
- **Family of narrow constructors beats a god `New(Config)`.** When most
  callers care about one axis of a type, give each common construction
  pattern its own named constructor (`NewFooWithPrefixes`,
  `NewFooWithProtocols`) rather than a functional-options pile or a
  config struct with half the fields nil.
- **Table-driven tests with `t.Run(name, ...)`.** When a test file has
  more than two similar cases, prefer a single `[]struct { name string;
  ... }` table iterated with `t.Run(tt.name, ...)`. Each case gets its
  own `-v` line, each case is self-contained, and you can focus one
  failing case.
- **Honest TODO comments over silent gaps.** When a function is
  incomplete — especially deep-comparison `equal()` methods that can pass
  superficially and silently mis-match on edge cases — write a visible
  `// TODO:` rather than a panic, a `//nolint`, or nothing. A reviewer
  should see the gap in a diff.

## Forbidden

- `panic()` for error handling. Allowed prefixes (enforced by `block-panic-error.sh`): `panic("BUG: ...")`, `panic("unreachable: ...")`, `panic("not implemented")`, `panic("unimplemented")`, `panic("TODO: ...")`, `panic("impossible: ...")`. Use `panic("BUG: <what>")` for programmer-error guards that must never fire at runtime. Any other `panic()` call is rejected at Write/Edit time (test files and `scripts/` excepted)
- `f, _ := func()` and `_, _ = func()` (ignoring errors). If you genuinely must discard, use `//nolint:errcheck // <why>` with a specific reason
- Global mutable state
- `init()` except registry patterns
- `log.Printf` (legacy log package)
- Silent defaults: `if x == "" { x = "0.0.0.0/0" }`
- `os.Getenv("ZE_*")` or `os.Getenv("ze.*")` -- use `env.Var()` instead
- `if end > x { end = x }` when clamping an int — use `end = min(end, x)` (Go 1.21+ built-in)
- `for i := 0; i < N; i++` when the body does not use `i` as anything but a counter — use `for range N` (Go 1.22+)
