# Go Standards

**When:** writing Go in Ze: naming, env access, logging, imports, typed-vs-string choices, external commands, or a compatibility shim
**Severity:** blocking
**Related:** config, cli, performance, repo-maintenance, architecture

## Directives

**`docs/contributing/ze-go-style.md` MUST be read at the START of every session, before any code (owner directive, 2026-08-18).** It names every place Ze diverges from standard Go, and it carries the one obligation no rule file repeats: a peer MUST NOT be able to panic the daemon, so `panic("BUG:")` marks only a state a Ze defect reaches and a malformed message from a socket returns an error. Where the guide and a rule file disagree, the rule file wins.

**Guard with early returns, state the invariant POSITIVELY, and test ONE fact per guard: a compound `if a || b` or `if a && b && !c` MUST be split.** A happy path MUST NOT be wrapped in an `else`, `if index < length` MUST be preferred to its negation, and a compound test that earns a name MUST get one rather than sit inline.

**These Go patterns MUST NOT be written:** `panic()` for error handling, a discarded error (`f, _ :=`, `_, _ =`) without `//nolint:errcheck // <why>`, global mutable state, `init()` outside a registry pattern, `log.Printf`, a silent default such as `if x == "" { x = "0.0.0.0/0" }`, `os.Getenv("ZE_*")`, `os.Exit()` in a handler, a function whose name joins two responsibilities with `And`, `if end > x { end = x }` where `min` works, and `for i := 0; i < N; i++` where the body never uses `i`.

**Logging MUST go through `slogutil.Logger("subsystem")` in the engine and `slogutil.PluginLogger("name", level)` in a plugin; `fmt.Printf` and the `log` package MUST NOT be used, and debug logging is permanent rather than temporary.** Levels are `disabled`, `debug`, `info`, `warn`, `err`, set per subsystem by the hierarchical `ze.log.<path>` env var or the `environment { log { ... } }` config, with a CLI flag beating an env var beating config beating the WARN default.

**Every Ze environment variable MUST be reached through `internal/core/env` and MUST be registered with a package-level `var _ = env.MustRegister(...)`; `env.Get()` on an unregistered key aborts the process.** `os.Getenv` and `os.Setenv` MAY be used only for a genuine system variable such as `PATH`, `HOME` or `NO_COLOR`, a test that calls `os.Setenv` directly MUST call `env.ResetCache()`, and `ai/rules/config.md` decides whether the setting belongs in YANG instead.

**A hot path and a component seam MUST carry typed numeric identity, an enum, a registered ID, a bitset or a packed integer, and MUST NOT carry a string.** A typed enum MUST make its zero value invalid so an unset field cannot pass for a real one, a map on such a path MUST be keyed by the numeric type, the string MUST be parsed ONCE where it enters the component, an accepted string key MUST be bound to a constant, and conversion back to string MUST happen only at the wire sink or the human sink. The boundaries where a string IS correct are in `docs/contributing/go-conventions.md`.

**Shipped Ze code MUST NOT run an external binary: no `exec.Command`, no `exec.CommandContext`, no shell.** Ze runs on an appliance image carrying no shell utilities, so a fork depends on a binary the environment lacks and answers with a second implementation's opinion where Ze holds its own; take the native path (`vishvananda/netlink`, a hand-built netlink request, `os.ReadFile` over `/proc` and `/sys`, `x/sys/unix`). Thomas authorises every exception and it MUST carry a row in `ai/allowed-system-commands.md` before the code lands, so an agent that believes a fork is unavoidable MUST state the case and STOP. `_test.go`, `test/`, `internal/test/` and `internal/le/` drive a developer machine and are outside this rule; everything Ze ships under `cmd/`, `internal/` and `pkg/` is inside it.

**Ze has never been released and has no users, so compat code, deprecation shims, fallbacks and "keep the old name working" MUST NOT be written anywhere; when something needs to change, change it.** The one frozen surface is the plugin API contract external authors compile against (`pkg/plugin/` types, the JSON event and text command protocol, anything re-exported for plugin consumption): its signatures and documented semantics MUST NOT break once released, while its implementation and every other `internal/` package stay free to change forever. ExaBGP format awareness MUST live only in `ze exabgp plugin` and `ze config migrate`, never in engine code.
