# Ze Go Style

Ze runs on a router that nobody restarts. The peer on the other side of the
socket is hostile, and the operator who reads the log is not a Go developer.
This page is the standard for every line of Go in the repository.

The rules a reader applies come first, because no tool checks them. Then come
the rules a tool enforces (plus first-draft linter traps), and last where Ze
differs from standard Go. When this page and a file in `ai/rules/` disagree,
the rule file wins.

Ze has three design goals, in order: safety, performance, developer experience.
The question each goal asks: [go-style/goals.md](go-style/goals.md).

## Ze Go cheat sheet (write this on the first draft)

| Need | Write | Never |
|------|-------|-------|
| Build text / format ints | `var tb textbuf.Buffer; tb.Str(k).Byte(':').Int(n).String()` | `fmt.Sprintf`, `strconv.FormatInt`, `+` on hot path |
| Operator handler error | `opererr.New(what, why, fix)` / `plugin.OperatorErrorResponse(...)` | `errors.New`, `fmt.Errorf` returned to CLI/RPC |
| Log in engine / plugin | `slogutil.Logger("sub")` / `slogutil.PluginLogger("name", lvl)` | `log.Printf`, `fmt.Printf`, `Debug` for dropped error |
| Ze env variable | `var _ = env.MustRegister(...)` + `env.Get("ze.foo")` | `os.Getenv("ZE_*")` |
| Intentional partial enum switch | `// <reason>` on its own line, then `//exhaustive:ignore` | trailing comment on `//exhaustive:ignore` or `nolint:exhaustive` |
| Intentional zero guard | `// refuse-by-default: guards <CallerSym> -- <why>` | naming the enclosing function, or bare `return nil` |
| Wire encoding | `func (t T) WriteTo(buf []byte, off int) int` | `Marshal() ([]byte, error)`, `make([]byte)` per item |
| Fast self-check before handback | `./le go style check path <file>` + `gopls check <file>` | waiting for main-thread lint or CI |

## The rules a reader applies

### Assertions, in a language that has none

**A peer MUST NOT be able to panic the daemon.** A malformed message from a
socket is an operating error and returns an error. Use `panic("BUG: <what>")`
only for a state that a Ze defect alone can produce. Startup registration
failures in `init()` (such as `registry.Register`,
`diagnostic.RegisterDoctorCheck`, backend or validator registration) can only
result from a compile-time Ze defect and MUST call
`panic("BUG: <pkg>: ...: " + err.Error())`, never `os.Exit()`.

**The best check is the one that never runs.** When a type makes a bad state
impossible to write, use the type and delete the check. **Pair the check** on
two code paths (parse and encode, validate and apply). **Test the negative
space**: every wire parser carries a fuzz target and every boundary gets a case.

Panic, error or type, and why each rule holds:
[go-style/assertions.md](go-style/assertions.md).

### A zero value is never an answer

Give the outcome a name and make the caller branch on it. Before returning a
zero, nil, false or empty result, ask whether a caller can tell it from a
failure and whether anything downstream relies on absence (if so, that is a
guard: name it, comment it, test it; full rule: `ai/rules/principles.md`).

**Code that cannot answer refuses; it does not default.** A lookup miss,
unrecognized input or unreadable resource is an error, a counted protocol
discard, or an explicit `Unknown` (never zero or a guessed constant). Change
the signature when it cannot express refusal. A probe that cannot look answers
`Unknown` with its reason, never "absent", and the caller decides explicitly.

The five shapes `./le arch refuse-by-default check` refuses and the guard
annotation: [go-style/zero-values.md](go-style/zero-values.md).

### Types that cannot lie

A type that can hold an invalid value will hold one.

- A value from a known set is a typed numeric enum with zero = `Unspecified`.
- An address is `netip.Addr` or `netip.Prefix`; a duration is `time.Duration`.
- A string lives only at a boundary (YANG, CLI, JSON, log, error). Convert once
  at that boundary and pass the typed value inward.
- Answer config questions from parsed structure, never text matching (including
  in `le` and test harnesses).
- `String()` is for a human. Never compare with it.

Construction, sum-type and lifecycle scope live in
[`preserve-validity-from-construction-through-use`](../../ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md).
Enum switch coverage applies to both old and new code. Examples and Go
mechanisms: [go-style/types.md](go-style/types.md).

#### One state, one variant

A struct whose fields are valid only in some combinations is a sum type in
disguise. Split it into variants holding only their own payloads. Sealed-interface
type switches list every variant with no `default`.

#### Every enum switch has a coverage policy

A `default` handles values outside the named set; it never replaces explicit
named cases.

| Class | Input and purpose | Required handling |
|-------|-------------------|-------------------|
| C: closed, internal | Value only Ze produces | List every distinct named value. A retained `default` is only `panic("BUG: <what>")` after proving external input cannot reach it. |
| O: open, from outside | Wire, file, config or plugin code | List every accessible distinct named value. Keep a safe, non-panicking `default` with a comment that the set is open. |
| P: intentionally partial | Predicate, projection or subset handler | Put `// <why subset>` immediately before an argument-free `//exhaustive:ignore` above the switch. Preserve nonmatching behavior. |

A complete dispatcher or state/event transition table is C or O, even for no-op
events. Never use `nolint:exhaustive`, file exemptions, casts or untagged
switches to bypass coverage.

#### Validated at construction

Validate at creation before building a domain value. Use private fields so
callers in other packages cannot break invariants after construction.

#### One type per lifecycle state

**Ze wants this pattern first.** Attach operations as methods of the validated
type (or accept only that type as parameter) so invalid state calls fail to
compile. Write Ze-controlled progressions (loaded -> resolved, parsed ->
validated) as distinct named types instead of a state field plus runtime `if`s.

### A limit on everything

State every bound in code:

| Thing | Bound |
|-------|-------|
| Loop over external input | Message length, checked before the loop |
| Pool | Fixed slot count and one max buffer size per slot |
| Queue or channel | Declared depth and defined full behavior |
| Retry | Count and maximum backoff |
| Cache | Entry count or byte budget |

Recursion over peer-controlled input is forbidden. Bounded internal recursion
names its bound in a comment; an intentional infinite loop (e.g. reactor) says
so at its top.

### Control flow a reader can simulate

Write the guard, handle it, and return. Split compound conditions into nested
branches and check whether the negative case needs handling.

One guard states one fact: split `if a || b { return err }` into `if a { return err }`
then `if b { return err }`. State invariants positively (`if index < length`);
with an `else`, write `if c` rather than `if !c` and swap the branches.

What `./le arch compound-guard check` refuses:
[go-style/control-flow.md](go-style/control-flow.md).

### Every error is handled

No discarded errors, no silent defaults, no invented fallbacks. Fail early on
parse/config errors. Do not discard an error into `_` (`f, _ := open()`); when
a discard is genuinely correct, write `//nolint:errcheck // <reason>`.

An error caught and not returned MUST be logged at WARN or above (never Debug
or Info). A wrap adds only context the inner error lacks—one layer owns each
`: `-separated prefix. **An error returned by a registered operator handler is
an `opererr.Error`** (`opererr.New`/`opererr.Wrap`: what, why, exact fix).

Error-wrap check and operator-error responses:
[go-style/errors.md](go-style/errors.md).

### Goroutines

Every goroutine is a long-lived worker with an owner and a stop path (or one
per session/peer lifecycle). Never spawn `go func()` per event or inside a
`for range`. Create the channel, start the worker, enqueue on the hot path,
close the channel to stop, and document that sequence on the owning type. Full
rule: `ai/rules/goroutine-lifecycle.md`.

### Comments say why and state the contract

Code says what; comments say why this and not the obvious alternative.

| Comment | Requirement |
|---------|-------------|
| File header | `// Design:` first, then `// Detail:`, `// Overview:`, or `// Related:` to sibling files |
| Caller obligation | MUST on both sides (`Stop`: "MUST call Wait after"; `Wait`: "MUST be called after Stop") |
| Concurrency | "Safe for concurrent use" or "NOT safe for concurrent use" |
| Test | Opens with `VALIDATES:` / `PREVENTS:` (goal and method) |

Update or delete comments in the same edit when behavior changes
(`ai/rules/go-standards.md`, `ai/rules/stale-comments.md`). Form details:
[go-style/comments.md](go-style/comments.md).

### Names

| Rule | Instead of | Write |
|------|-----------|-------|
| Name the value, never its Go type | `famStr`, `levelStr` | `family`, `level`; `afiName` beside `afi` |
| Qualifier last, descending significance | `maxLatencyMs` | `latencyMsMax` beside `latencyMsMin` |
| Name the subsystem | `logger` | `peerLog`, `wireLog` |
| Match helper name to caller | `writeBody` | `writeUpdateBody` beside `writeUpdate` |
| Never overload a verb | `delete` for a counter | `delete` config, `clear` counters, `remove` route |
| Prefer a noun to a participle | `peer.preparing` | `peer.pipeline`, `pipelineMax` |

Order files for the first read: entry point first; in a type, fields, then
types, then methods. More on names and ordering:
[go-style/names.md](go-style/names.md).

### State that goes stale

- **Do not copy or alias a variable** (two names for one fact drift).
- **Reuse a cache/artifact only on a completion mark or content identity** (never size or mtime).
- **Shrink scope and compute values next to their use.**
- **Simplify return types**: prefer nothing -> `bool` -> value -> `(value, ok)` -> `(value, error)`.
- **Group an allocation with its `defer` release.**
- **Keep index, count and size apart** (put the unit in the name: `octets`, `entries`, `slots`).

Proof rules, in-place construction, and zeroing padding:
[go-style/stale-state.md](go-style/stale-state.md).

### The shape of a function

A function that fits on one screen can be read.

- Few parameters, simple return type, logic between the braces.
- Keep `switch` and `if` branching in the parent; push branch-free logic into pure helpers.
- One concern per file; past 1000 lines, look for a second concern.
- Pass options explicitly at the call site instead of relying on library defaults.
- Run at your own pace: read what arrived from external systems and process on Ze's schedule.

When to split a file and why each rule holds:
[go-style/function-shape.md](go-style/function-shape.md).

### Performance

Solve performance in design and target zero allocation on wire paths
(`ai/rules/performance.md`). **A copy is deliberate**: every wire-path copy
MUST match a trigger in `docs/architecture/buffer-architecture.md`, "When a
copy is deliberate".

The four resources, compiler proofs (`-gcflags=-m`/`-S`), and buffer/pool rules:
[go-style/performance.md](go-style/performance.md).

### Simplicity and debt

Budget thinking for the simplest fully correct design (`ai/rules/simplicity.md`).
Simplicity cuts machinery, never correctness or scope. Fix hot-path copies,
unbounded queues, or quadratic loops when you find them. Delete X before
writing Y (`ai/rules/no-layering.md`), and write no compatibility shims
(`ai/rules/go-standards.md`).

Dependency approval and repository tooling:
[go-style/simplicity.md](go-style/simplicity.md).

## The rules a tool enforces

Run `./le go style check path <file>` (fast AST check, allowed in subagents)
before handback and `./le go lint run scope "<pkg>"` in the main thread. Full
tool table and settings: [go-style/tool-rules.md](go-style/tool-rules.md).

**Top first-draft linter traps** (avoid slow lint retries):
- `godot`: end every top-level declaration comment with a full stop (except a line ending in a quoted RFC `"`).
- `misspell`: US English (`neighbor`, `behavior`, `canceling`), except the owner-approved `licence`.
- `gocritic`: define types before their methods (`typeDefFirst`), use `switch` over 3+ `if/else if` (`ifElseChain`), combine adjacent same-type params (`a, b uint8`, `paramTypeCombine`), pass structs > 288B by pointer (`hugeParam`), no `defer` inside loops (`deferInLoop`).
- `errorlint` / `modernize`: use `errors.Is`/`errors.As`/`errors.AsType` (never `==` or `err.(*T)`), `%w` wrapping, `slices.Sort`, `min`/`max`, `for range n`.
- `goconst`: extract string literals repeated 3+ times (len >= 3) into a `const`.
- `goimports`: group `github.com/ze-software/ze/...` imports last after a blank line.

### By the numbers

Line length: 100 columns, advisory. File length: 1000 lines is where you look
for a second concern (the only threshold).

## Where Ze differs from standard Go

A reader trained on standard Go defaults to the wrong approach below. Every
other difference: [go-style/standard-go.md](go-style/standard-go.md).

### Encoding and wire

`func (t T) WriteTo(buf []byte, off int) int`, never `Marshal() ([]byte, error)`:
zero allocations on a hot path; the caller owns the buffer (`ai/rules/performance.md`).

### Architecture and registration

`init()` plus a registry plus a blank import, never direct imports between
packages (`ai/patterns/registration.md`).

### Config and schema

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Struct tags plus `json.Unmarshal` | YANG schema as the sole source of truth | `ai/rules/config.md` | Schema-driven validation, migration, completion and diff |
| A config version field | No version numbers; machine-transformable migration | `ai/rules/config.md` | YANG evolution handles schema change |
| Silent defaults for missing fields | Fail on an unknown key and suggest the closest valid one | `ai/rules/config.md` | Explicit beats implicit |
| `interface{}` for flexible config | `map[string]any` through one canonical pipeline | `ai/rules/repo-maintenance.md` | File to Tree to `ResolveBGPTree` to `map[string]any` to `PeersFromTree` |

### Communication and IPC

JSON events down and text commands up over pipes or `net.Pipe`, never gRPC or
HTTP between internal services (`ai/rules/plugins.md`).

### Testing

Unit tests and `.ci` functional tests are both required: unit tests prove the
algorithm; `.ci` tests prove a user reaches the feature (`ai/rules/completion.md`).

### CLI and commands

`<verb> <noun> <action> [<identifier>]`, never `command <identifier> [flags]`
(`ai/rules/cli.md`).

### Native tooling

A native Go package with a registered `./le` action, never ad-hoc scripts
(`ai/rules/go-standards.md`).

## Check before finishing a Go change

Empirical top defects from `plan/journal/`:
1. **Wired end-to-end?** Registered in `register.go` / `all.go` and reachable from the real entry point, not only unit-tested.
2. **Mirror path guarded?** Every guard added on parse/validate/send has its partner on encode/apply/receive.
3. **Canonical helper used?** Called Ze's existing helper (`opererr`, `textbuf`, `env`, `slogutil`, `crashlog.Exec`) instead of open-coding a copy.
4. **No overloaded zero?** No zero value carries two meanings ("unset" vs "disabled/valid").
5. **Comments and docs current?** Updated every comment and doc page whose behavior changed.
