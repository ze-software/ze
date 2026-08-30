# Go Standards

**When:** writing Go in Ze: naming, env access, context, logging, imports, errors, API contracts, typed-vs-string choices, file layout and cross-references, compatibility shims, or a Go compiler bump
**Severity:** blocking
**Related:** config, cli, performance, repo-maintenance, architecture

## Directives

- **You MUST read `docs/contributing/ze-go-style.md` at the START of EVERY session, before any code (owner directive, 2026-08-18).** `.claude/rules/session-start.md` step 2 carries it as a blocking checklist item.
- **The three-trigger gate this point used to state is WITHDRAWN.** It read the guide only before a Go design decision, a review, or an argument about how Ze code is written, and told you not to open it for an ordinary edit. It was set to save context and it cost more than it saved: a session can write Go all day, meet none of those three triggers, and never learn that Ze guards with early returns, splits a compound condition, and states an invariant positively. Measured 2026-08-18, on four `internal/` files in one session.
- That page carries the reasoning behind these rules. It states Ze's three design goals in their order: safety, performance, and developer experience.
- It covers control flow, limits, assertions in a language that has none, memory, errors, goroutines, and the shape of a function. It also covers names, comments, duplicated state, off-by-one errors, and the numbers.
- **The one line on that page with no rule file behind it: a peer MUST NOT be able to panic the daemon.** `panic("BUG:")` marks a state that only a Ze defect reaches. A malformed message from a socket is an operating error, and every parser returns an error for one.
- **When the guide and a rule file disagree, the rule file wins.** The guide explains. A rule file obliges.
- The guide is adapted from TigerStyle, the coding standard of TigerBeetle. Its last table names every deliberate divergence.

- **Guard with EARLY RETURNS: handle the edge case and return, and never wrap the happy path in an `else`.** A sequence of guard clauses followed by the main logic reads top to bottom; a happy path inside an `else` does not. Applies to `if err != nil { return }`, validation guards and nil checks alike.
- **One fact per guard. A compound condition MUST be split.** `if a || b` and `if a && b && !c` make a reader hold two or three facts at once to decide whether every case is covered, and the answer is usually that one of them was never considered. Write one `if` per question and ask, for each, whether the negative case needs a branch of its own.
- **State the invariant POSITIVELY.** `if index < length` reads directly. `if index >= length` states the failure of the invariant and makes the reader invert it before they can check it.
- **Name a compound test rather than inlining it.** Two exit codes tested inline are a condition; `isCheckIgnoreAnswer(code)` is a sentence. The name is where the reason lives, and a reviewer checks a name against its call site far faster than they re-derive a boolean.
- **Every non-exempt `.go` file MUST carry a `// Design:` header.** The native `./le consistency` action reports missing headers from `checkDesignRefs` in `internal/le/consistency/consistency.go`. Exempt: tests, generated files, registration leaves, and vendor code.
- **This obliges; `docs/contributing/ze-go-style.md` explains.** That page's "Control flow a reader can simulate" carries the reasoning. When the two disagree, this file wins.
- **The one-fact rule was guidance-only until 2026-08-18 and that cost real code.** `timedOut` in `internal/component/ike/engine/dpd.go` shipped with a two-fact condition and `ignoredNames` in `internal/le/vendorweb/check.go` decided its error case on three. The native edit checks do not replace reading this rule.

- Go 1.21+ features (slog, generics)
- `golangci-lint` MUST pass
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Context as first param: `context.Context`
- Code MUST NOT strip `ctx context.Context` parameters from function signatures. "Clean unused context" means remove dead `import "context"` lines only. Parameters stay even if the current body doesn't use ctx (propagation, cancellation, future use).
- Fail-early: code MUST propagate parse/config errors immediately, and MUST NOT silently default

- Engine: `slogutil.Logger("subsystem")`
- Plugins: `slogutil.PluginLogger("name", level)`
- Per-subsystem: `ze.log.<path>=<level>` env vars (hierarchical, most-specific wins)
- Levels: `disabled`, `debug`, `info`, `warn`, `err`
- Config: `environment { log { level warn; bgp.routes debug; } }`
- Priority: CLI flag > env var > config > default (WARN)
- Debug logging is permanent: code MUST use `logger.Debug()`, and MUST NOT use `fmt.Printf`

**A third-party import that is not already in `go.mod` MUST NOT be added without asking the user first.**

**Every Ze environment variable access MUST use `internal/core/env`. `os.Getenv` and `os.Setenv` MUST NOT be used for a Ze-specific variable.** The accessors and the registration flags are in `docs/contributing/go-conventions.md`. `os.Getenv` stays correct for a genuine system variable such as `PATH`.

**Before adding an env var you MUST read `ai/rules/config.md`:** it decides whether the setting belongs in YANG config instead, and it fixes the naming, because the env var path mirrors the YANG path.

**Cache:** Built once from `os.Environ()` on first `Get()`. `Set*()` updates both cache and os env. Tests that use `os.Setenv` directly MUST call `env.ResetCache()`.

**Registration required:** every env var MUST be registered via package-level `var _ = env.MustRegister(...)`. Calling `env.Get()` with an unregistered key aborts the process.

**`os.Getenv` MAY be used for:** System env vars (`HOME`, `PATH`, `XDG_*`, `NO_COLOR`, `USER`, `SSH_*`).

- When two packages in the module share the same name (e.g., `internal/component/iface/cli/` and `internal/component/iface/`), goimports cannot resolve which to use and silently removes the import. You MUST use an aliased import in this case: `ifacepkg "github.com/ze-software/ze/internal/component/iface"`.
- You MUST add import + usage in the same Edit call to prevent goimports from removing an "unused" import between edits.

**First-party tooling MUST live in a native Go package under `internal/le/` and MUST register its `./le` action. A new Python or shell helper MUST NOT be added.** The root `le` and `ze` POSIX launchers are the intentional entry points.

**These style patterns are advisory. They SHOULD be adopted opportunistically the next time you are in the relevant code, and MUST NOT be applied as a sweep.**

- **Early return over nested else.** Handle the error / edge case and return immediately rather than wrapping the happy path in an else block. Deep nesting makes control flow hard to follow; a sequence of guard clauses followed by the main logic reads linearly. Applies to `if err != nil { return }`, validation guards, and nil checks alike.
- **Drain `time.NewTimer` on Stop.** If you use `time.NewTimer` directly and call `Stop()`, drain `.C` in a non-blocking select before `Reset()`, otherwise the next `Reset()` sees a stale firing and behaves wrong. The BGP FSM uses `clock.Timer` + `AfterFunc` (callback-based, no channel to drain) and needs no helper. The only `time.NewTimer` site in the BGP tree, `internal/component/bgp/plugins/rs/worker.go`, already has a `drainTimer` helper: promote it or copy the pattern when adding a new `time.NewTimer` site. Prefer `AfterFunc` where possible.
- **Slice type with methods beats a wrapping struct.** When the only state is `[]*T`, declare `type Foo []*T` with methods on the slice rather than `type Foo struct { items []*T }`. `Foo{}` is the empty value, `append(foo, x)` works, iteration works, JSON marshaling works, and you never write a `NewFoo`/`Items()` pair.
- **Family of narrow constructors beats a god `New(Config)`.** When most callers care about one axis of a type, give each common construction pattern its own named constructor (`NewFooWithPrefixes`, `NewFooWithProtocols`) rather than a functional-options pile or a config struct with half the fields nil.
- **Table-driven tests with `t.Run(name, ...)`.** When a test file has more than two similar cases, prefer a single `[]struct { name string; ... }` table iterated with `t.Run(tt.name, ...)`. Each case gets its own `-v` line, each case is self-contained, and you can focus one failing case.
- **Honest TODO comments over silent gaps.** When a function is incomplete (especially deep-comparison `equal()` methods that can pass superficially and silently mis-match on edge cases), write a visible `// TODO:` rather than a panic, a `//nolint`, or nothing. A reviewer SHOULD see the gap in a diff.

**Every exported struct field that reaches JSON output MUST carry a `json:"kebab-name"` tag.** Keys are lowercase kebab-case, matching the YANG leaf or the config tree key. The full rules and the attribute table are in `ai/rules/cli.md`.

**Code MUST NOT write these forbidden Go patterns:**
- `panic()` for error handling. The native Write/Edit gate in `internal/le/hookruntime/writeedit.go` blocks a new `panic()` call. Return an error from operating paths; reserve `panic("BUG: <what>")` for a programmer-error invariant only where the owning rule permits it
- `f, _ := func()` and `_, _ = func()` (ignoring errors). If you genuinely MUST discard, use `//nolint:errcheck // <why>` with a specific reason
- Global mutable state
- `init()` except registry patterns
- `log.Printf` (legacy log package)
- Silent defaults: `if x == "" { x = "0.0.0.0/0" }`
- `os.Getenv("ZE_*")` or `os.Getenv("ze.*")` -- use `env.Var()` instead
- `if end > x { end = x }` when clamping an int, use `end = min(end, x)` (Go 1.21+ built-in)
- `for i := 0; i < N; i++` when the body does not use `i` as anything but a counter, use `for range N` (Go 1.22+)

## Naming

**"Ze" is "The" with a French accent, and it MUST be used wherever "the" works grammatically.** Which spelling each surface takes is in `docs/contributing/go-conventions.md`.

**Config-specific naming, for a YANG leaf, an env var key, or its Go struct field, MUST follow `ai/rules/config.md`.**

**A variable or constant MUST name what the value IS. It MUST NOT encode its Go type.** `famStr`, `levelStr` and `addrStr` name the type; `family`, `level` and `addr` name the value. The banned patterns and their fixes are in `docs/contributing/go-conventions.md`.

**When you create a NEW package, you MUST pick the term from the glossary whose definition matches your concern. You MUST NOT coin a synonym.** The glossary is in `docs/contributing/go-conventions.md`, verified against each package's own doc comment.
**The glossary documents the packages that exist and forces no rename.** An existing protocol keeps its vocabulary, and a rename needs an explicit user-approved shortlist.

## Prefer Typed Numeric Over String

**A hot path MUST carry typed numeric identity: an enum, a registered ID, a bitset, or a packed integer. It MUST NOT carry a string identity.** Across a component or engine seam the same rule holds, plus the pointer restrictions in `ai/rules/repo-maintenance.md`.
**The surfaces, the boundaries where a string IS correct, the acceptable `map[string]V` cases, and the anti-patterns with their fixes are in `docs/contributing/go-conventions.md`.**

**A typed enum MUST make its zero value invalid, so an unset field cannot pass for a real one. A hot-path comparison MUST be against the typed constant, never against a string literal.**

**A conversion to string MUST happen once, at the wire sink or the human sink, and MUST NOT happen on the path between them.**

**BLOCKING on hot paths.** When a map is keyed by a value from a known set, code MUST use a numeric key type (`uint8`, `uint16`, `int`, typed enum), not `string`.

**A hot-path map MUST be keyed by a numeric or typed value. The string MUST be parsed ONCE where it enters the component, and the typed value MUST be passed to every internal helper.**

**Where a string key is accepted, it MUST be bound to a constant rather than spelled at each use.**

**Before adding a `string` field that crosses a component seam, or that sits on a hot path, you MUST answer all three:**
- Is this value one of a closed set? Then it MUST be a typed enum
- Is it read more than once per message? Then it MUST be parsed at the boundary and carried typed
- Does the receiving side compare it against a literal? Then the literal MUST become a constant and the field MUST become its type

## API Contracts in Comments

**A function with a caller obligation MUST document it in its godoc, and MUST state it on BOTH sides of a pair.** The trigger is any function where skipping a step causes a resource leak, a deadlock, a panic, or silent misbehavior. The comment each lifecycle pattern owes is in `docs/contributing/go-conventions.md`.

- Use "MUST" (not "SHOULD") for obligations that cause bugs when violated.
- Place the obligation on both sides of the pair: the function that creates the obligation AND the function that fulfills it.

- [ ] Every resource-acquiring function MUST name how to release it
- [ ] Every multi-step lifecycle MUST be documented on the type
- [ ] Every "call B after A" MUST appear in both A's and B's comments

## File Modularity

**Each `.go` source file MUST hold exactly one concern: a cohesive group of types and functions serving one responsibility.** The size thresholds and what to do at each one are in `docs/contributing/ze-go-style.md`, "The shape of a function".
**A split MUST be made only when the separation is RIGHT.** A forced mechanical split that scatters one concern across files is worse than one large cohesive file, which is why the post-edit size warning is non-blocking. Read it as a prompt to check cohesion, never as an order to cut.
**A `_test.go` file is NOT subject to a line-count threshold.** Tests grow with coverage and table-driven cases, and splitting them adds navigation cost without improving production code.

**You MUST judge file size against 1000 lines, the only threshold** (Thomas, 2026-08-01).

**Before creating a file, you MUST ask "one concern?" Before adding to one, you MUST ask "belongs to this file's concern?" Past 1000 lines, you MUST check for multiple concerns.**

- **Tool:** `go build -o bin/go_extract ./internal/le/goextract/goextract.go && bin/go_extract <source.go> <dest.go> <symbol1> [symbol2 ...]` moves named declarations (with doc comments) to dest, runs `goimports` on both. Note: `goimports` cannot resolve aliased imports; you MUST add those manually to the new file.
- Zero semantic effect: Go compiles all files in a package together
- File-local types move with their functions
- Shared test helpers stay in base `_test.go`
- `goimports` handles import cleanup
- You MUST name the file after its concern: `reactor_announce.go`, `session_handlers.go`
- New files: you MUST copy `// Design:` from original, and review the topic annotation (see "Design Document References" below)
- All resulting files: you MUST add `// Related:` to siblings (see "File Cross-References" below)

**Size alone MUST NOT be a reason to split a file that is:**
- Large but single coherent concern (capability registry, pool internals)
- CLI file with one-function-per-subcommand
- Dependency chain where dispatcher references all implementations

## Design Document References

**Every non-test, non-generated `.go` file MUST carry a `// Design:` comment, and it MUST be the first comment in the file.** The format, the line ordering, the situation table and the exempt set are in `docs/contributing/go-conventions.md`.

- **Format:** you MUST write `// Design: docs/architecture/core-design.md -- topic annotation`
- Topic annotations SHOULD be used over section numbers (they survive restructuring).

- The `// Design:` line MUST be the first comment in every file. Only compiler directives (`//go:build`) MAY precede it.
- `// Package` doc comments MUST go after the header block, not before it.

## File Cross-References

**Every cross-reference MUST have a back-reference: if A references B, B MUST reference A.** The three directional keywords, the pairing table, and when to add or not add one are in `docs/contributing/go-conventions.md`.

**A file MAY skip a cross-reference when it is:**
- Standalone in package (no strong coupling to siblings)
- Only related through package's public API
- Relationship is obvious from filename alone (see "Not a Directory Listing" below)

**A file header is not a directory listing. A `// Detail:` line MUST point at a relationship a reader would otherwise miss, and it MUST NOT reproduce `ls`.**

**Rule of thumb:** if removing the `// Detail:` line would leave a reader unable to find important code, you SHOULD keep it. If they would find it anyway by scanning filenames, you SHOULD drop it. Aim for 3-5 references maximum per hub file.

## External Commands

**Ze code MUST NOT run an external binary.** No `exec.Command`, no `exec.CommandContext`, no shell. Ze is a network operating system: it runs on an appliance image carrying no shell utilities, inside a network namespace carrying nothing, and on a router nobody restarts. A fork buys three defects and one convenience. It depends on a binary the environment does not carry. It answers with a second implementation's opinion about the kernel, where Ze holds its own. And it reports nothing exactly when the environment is minimal and an operator most needs an answer.

**Thomas authorises every exception, and it MUST carry a row in `ai/allowed-system-commands.md` before the code lands.** The register is the only authority: a fork whose command names no row there is a defect whatever the comment beside it says. An agent that believes a fork is unavoidable MUST state the case and STOP. It MUST NOT add its own row, and it MUST NOT ask whether to skip the Go path -- the question is which Go path to take.

**Take the native path, and it nearly always exists.** Links, routes, addresses and generic families come from `vishvananda/netlink`; a family it does not wrap is still reachable by building the request by hand, as `internal/le/deployment/l2tpdiag.go` does for L2TP, which the library does not support at all. `/proc` and `/sys` answer through `os.ReadFile`, a device node through `os.Stat`, and the rest through `x/sys/unix`. A fork that only formats something Ze already knows MUST be replaced by stating Ze's own view, because a second view can disagree with it.

**Test code and native developer tooling are outside this rule.** A `_test.go` anywhere, `test/`, `internal/test/`, and `internal/le/` drive a developer machine or CI runner, where the toolchain is present by construction and calling it is often the point: a test MAY run what it needs to set up or observe. What is governed is what Ze ships and runs on an appliance: non-test product code under `cmd/`, `internal/`, and `pkg/`. A diagnostic Ze ships is Ze, and it runs where no toolchain exists.

## No Backwards Compatibility

**Ze has never been released and has no users. Compat code, comments, shims and fallbacks MUST NOT be written anywhere, the plugin API included. When something needs to change, change it.**

**Code under `internal/` is not user-exposed, so it MAY be changed freely, forever. No shims, no deprecation layers, and no "keep the old name working".**

**The only exception is the plugin API**, the surface that external plugin authors compile against (`pkg/plugin/` SDK types, the JSON event / text command protocol between core and plugins, and anything re-exported for plugin consumption). Once released, that surface MUST NOT break. Everything else under `internal/` remains free to change.

**The plugin API's IMPLEMENTATION MAY change freely. Only its CONTRACT is frozen once released:** the signatures, the protocol shape, and the documented semantics.

**ExaBGP format awareness MUST live only in the external tools `ze exabgp plugin` and `ze config migrate`. Engine code MUST carry none.**

## Go Compiler Upgrade Checklist

**After a Go update, you MUST:**
1. Read `$(go env GOROOT)/src/strings/builder.go`, find `copyCheck`.
2. Read `$(go env GOROOT)/src/internal/abi/escape.go`, find `NoEscape`.
3. Compare against `internal/core/textbuf/textbuf.go` `noescape` + `inlineSlice`.
4. If the stdlib changed technique, update ours to match.
5. Verify: `go build -gcflags='-m=2' -o bin/escape-test ./internal/core/textbuf/ 2>&1 | grep 'moved to heap'` SHOULD NOT show `b` escaping for stack-local Buffer usage.
