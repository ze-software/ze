# Ze Go Style

Ze runs on a router that nobody restarts. The peer on the other side of the
socket is hostile, and the operator who reads the log is not a Go developer.
This page is the standard for every line of Go in the repository.

The page has three parts. The rules a reader applies come first, because no tool
checks them. One table then lists the rules a tool enforces. Last come the places
where Ze differs from standard Go. The history and the reasoning behind the
standard are in `ze-go-style-background.md`. `writing-style.md` is the standard
for every word. When this page and a file in `ai/rules/` disagree, the rule file
wins.

Ze has three design goals, in this order: safety, performance, developer
experience. All three matter. The order decides only what gives way when two
goals pull apart.

| # | Goal | The question it asks |
|---|------|----------------------|
| 1 | Safety | What happens when this input is hostile, this peer is dead, or this buffer is full? |
| 2 | Performance | What does this cost for each UPDATE, for each peer, for each second? |
| 3 | Developer experience | Can the next reader change this code without a second file open? |

## The rules a reader applies

### Assertions, in a language that has none

**A peer MUST NOT be able to panic the daemon.** This is the most important
rule on this page. A malformed UPDATE is an operating error, and every parser
returns an error for one. Use `panic("BUG: <what>")` only for a state that a Ze
defect alone can produce, never for a state that arrives over a socket.

| The failure | The tool |
|-------------|----------|
| A programmer error that MUST NOT happen at runtime | `panic("BUG: <what>")` |
| An operating error that a running system produces | An error return, wrapped with `fmt.Errorf("context: %w", err)` |
| A design property that holds before the program runs | A type that cannot hold the bad state, `var _ Iface = (*T)(nil)`, or a test |

**The best check is the one that never runs.** When a type can make a bad state
impossible to write, use the type and delete the check. A check that runs can be
forgotten, but a state that cannot be written needs no check. "Types that cannot
lie" shows the forms.

**Pair the check.** Check each property on two code paths. Validate an
attribute when it is parsed from the wire, and again when it is written back. A
bug that survives one check rarely survives both.

**Test the negative space.** Defects live where data crosses from valid to
invalid, so every wire parser carries a fuzz target and every boundary gets a
case. A fuzz target proves that defects are present, never that they are absent.
Build the mental model first, write the checks that encode it, then let the
fuzzer find the gap.
<!-- source: ai/rules/testing.md -- Boundary Testing, Fuzz -->

### A zero value is never an answer

A zero value is what every field holds before anybody writes to it, so it
cannot be told apart from a field nobody set. The dangerous form is the zero
that behaves correctly: a missing branch and a deliberate no-op produce the same
silence, and the next change deletes the behavior without touching a line that
names it. Give the outcome a name and make the caller branch on it.

The sharpest case is a zero that guards without being a guard. A test that asks
*which value do I use* can also answer *is this allowed yet*, because the value
stays zero until the permission exists. `verifyRemoteAuth` gated an EAP peer's
AUTH payload on `sa.EAPMSK != [64]byte{}`, a test of which key to sign with. An
EAP method that derives no key makes the MSK legitimately zero, and the
accidental guard then becomes an authentication bypass. The fix asks the
exchange whether it succeeded, on purpose. `PeerResult` gained `Discarded` and
`Notified` for the same reason.

Before you write a zero, nil, false or empty result, ask two questions. Can a
caller tell this from a failure? Does anything downstream rely on this value
being absent? If it does, that reliance is a guard, and a guard gets a name, a
comment, and a test. Full rule: `ai/rules/principles.md`.

### Types that cannot lie

A type that can hold an invalid value will hold one.

- A value from a known set is a typed numeric enum. Zero MUST mean
  `Unspecified`, so the Go zero value is never a valid state.
- An address is `netip.Addr` or `netip.Prefix`, never a string. A duration is
  `time.Duration`, never an integer of unstated units.
- A string exists only at a boundary: a YANG leaf, a CLI token, a JSON key, a log
  line, an error message. Convert once at that boundary, then pass the typed
  value inward.
- `String()` is for a human. Never compare with it.

The common failure is an address stored as a string and parsed back for each
comparison. When a struct needs both forms, it stores both and parses once at
construction:

```go
type Candidate struct {
    PeerAddr string     // for map keys, JSON, interning
    PeerIP   netip.Addr // for comparison (zero-alloc)
}
```

Full rule: `ai/rules/go-standards.md`, "Prefer Typed Numeric Over String".

The four rules below apply to code written from 2026-10-04. Older code does not
need a rewrite to meet them.

#### One state, one variant

A struct whose fields are valid only in some combinations is a sum type in
disguise. With `done bool`, `result *T` and `reason string`, a caller can build
"done with a reason" or "not done with a result", and each of those is a defect
nobody wrote on purpose. Write one type for each state. Each type holds only the
data that its state has, and a sealed interface joins them:

```go
type Outcome interface{ outcome() } // sealed: only this package can add a state

type Pending struct{}
type Done struct{ Result Route }
type Failed struct{ Reason error }

func (Pending) outcome() {}
func (Done) outcome()    {}
func (Failed) outcome()  {}
```

The consumer uses a type switch with one arm for each state, and each arm reads
only that state's data.

#### A switch over a closed set has no default

A switch over a typed enum or a sealed interface lists every value and has no
`default` arm. When a value is added, every switch that lists the old values
becomes a place to update. A `default` arm takes the new value without a word,
and gives it a behavior that nobody chose. A `default` is correct only when the
set is open, for example a code read from the wire. Its comment then says that
the set is open.

The `exhaustive` linter accepts a `default` as complete, because older code
relies on that. So for new code, the reader is the check.
<!-- source: .golangci.yml -- exhaustive default-signifies-exhaustive -->

#### Validated at construction

A type that carries a validation has an unexported field and one constructor
that returns `(T, error)`. A plain named type such as `type Email string` lets
any package write `Email("junk")`, so it guarantees nothing. When the
constructor is the only way to build a value, the code that receives the value
does not check it again:

```go
type HoldTime struct{ seconds uint16 } // zero, or 3 and above

func ParseHoldTime(seconds uint16) (HoldTime, error)
```

This is not the paired check in "Assertions, in a language that has none". That
check pairs the read from the wire with the write back to it. It does not
validate again a value that a type can only hold when the value is valid.

#### One type per lifecycle state

When Ze itself moves a value through a lifecycle, give each state its own type,
and put an operation only on the state where it is permitted. `Dial` returns a
`*Conn`. Only `*Conn` has `Send`, so a call to `Send` before `Dial` does not
compile. A transition takes the old value and returns the new one, so the old
state cannot be used after the transition.

Go cannot attach a method to one instance of a generic type, so `Conn[Open]` is
not a choice. Use distinct types.

This rule is only for transitions that Ze controls: setup, builders, and config
that goes from parsed to validated. A state that a peer changes, for example the
BGP FSM, is runtime data. A peer can send any message in any state, so the code
must check the state at runtime and the type cannot carry it.

### A limit on everything

Everything in reality has a limit, so state the limit in the code.

| Thing | Bound |
|-------|-------|
| A loop over external input | The message length, checked before the loop starts |
| A pool | A fixed slot count, and one maximum buffer size for every slot |
| A queue or a channel | A declared depth, and a defined behavior when it is full |
| A retry | A count and a maximum backoff |
| A cache | An entry count or a byte budget |

Recursion over a structure that a peer controls is forbidden, because the peer
chooses the depth. Recursion over a bounded internal structure is permitted, and
the bound goes in a comment above the function. A loop that cannot end, the
reactor loop for example, says so in a comment at its top, so the reader knows
the missing bound is a decision.

### Control flow a reader can simulate

Write the guard, handle it, and return. Guard clauses followed by the main logic
read from top to bottom, and a happy path inside an `else` does not.

Split a compound condition into nested branches. A reader who meets
`if a && b && !c` must hold three facts at once to decide whether every case is
covered. For each `if`, ask whether the negative case also needs a branch.

One guard states one fact. `if a || b { return err }` is two guards written as
one: write `if a { return err }`, then `if b { return err }`. The split changes
no meaning, because `||` stops at the first true operand and the body leaves
either way. `./le arch compound-guard check` refuses a top-level `||` condition
with no `else` and a body that ends in `return`, `continue`, `break` or `goto`,
on the lines your unpushed commits and working tree changed. It does not judge
`&&`, an `else`, or a body that falls through, because none of those splits
without a design decision. How it chooses the lines:
`ze-go-style-background.md`.
<!-- source: internal/le/arch/compoundguard/compoundguard.go -- Check -->

State an invariant positively. `if index < length` is easy to read.
`if index >= length` states the failure of the invariant, and the reader must
invert it. With an `else`, write `if c` rather than `if !c`, and swap the
branches.

### Every error is handled

Most catastrophic failures in distributed systems come from the handling of an
error the software had already detected. So Ze has no discarded error, no silent
default, and no fallback value invented at the point of failure. Fail early: a
config that does not parse stops the load, and an absent value is an error,
never `0.0.0.0/0`. Do not discard an error into `_`, as in `f, _ := open()`:
no linter refuses that form, so the reader is the check. When a discard is
correct, write `//nolint:errcheck // <the reason>`.

### Goroutines

Every goroutine is a long-lived worker with an owner and a stop path.

| Pattern | Status |
|---------|--------|
| A long-lived goroutine reading from a channel | Required |
| One goroutine for one lifecycle: a process, a session, a peer | Permitted |
| One goroutine for one event in a hot path, or `go func()` in a `for range` over events | Forbidden |

Create the channel and start the worker, enqueue on the hot path, close the
channel to stop. A type that owns a goroutine states that sequence in its doc
comment. Full rule: `ai/rules/goroutine-lifecycle.md`.

### Comments say why and state the contract

The code says what happens. The comment says why this, and not the obvious
alternative. A reader who has the criteria for a decision can change the code
safely.

| Comment | Requirement |
|---------|-------------|
| The file header | `// Design:` first, then `// Detail:`, `// Overview:`, or `// Related:` to the sibling files a reader needs |
| A caller obligation | MUST, on both sides of the pair. `Stop` says "MUST call Wait after". `Wait` says "MUST be called after Stop" |
| Concurrency | "Safe for concurrent use", or the opposite. Silence is not an answer |
| A test | Opens with its goal and its method, so the next reader can skip it or trust it |
| Any comment | Sentences: a capital letter, a space after the slashes, and a full stop |

A comment that no longer matches the code is worse than no comment, so the
comments change in the same edit as the behavior. Full rules:
`ai/rules/go-standards.md` and `ai/rules/stale-comments.md`.

### Names

A great name captures what a thing is or does, and it proves that the author
understood the domain. Take the time to find it.

| Rule | Instead of | Write |
|------|-----------|-------|
| Name the value, never its Go type. Two forms of one concept differ by meaning | `famStr`, `levelStr` | `family`, `level`; `afiName` beside `afi` |
| Put the qualifier last, by descending significance, so related names sort together | `maxLatencyMs` | `latencyMsMax` beside `latencyMsMin` |
| Give the name meaning: say which subsystem | `logger` | `peerLog`, `wireLog` |
| Give related names the same length, so expressions line up | `src`, `dst` | `source`, `target` |
| Give a helper the name of its caller | `writeBody` | `writeUpdateBody` beside `writeUpdate` |
| Never overload a name | `delete` for a counter | `delete` config, `clear` counters, `remove` a route |
| Prefer a noun to a participle: it goes straight into a sentence | `peer.preparing` | `peer.pipeline`, then `pipelineMax` |

Order a file for the first read: the entry point first, and in a type the
fields, then types, then methods. When no order is obviously right, sort
alphabetically. Go casing and the package glossary: `go-conventions.md`.

### State that goes stale

Most defects come from a gap in time or in space between where a value is
checked and where it is used.

- **Do not copy a variable or alias one.** Two names for one fact will disagree.
- **Build a large struct in place.** The constructor takes an out pointer, so
  the value is written where it lives and no intermediate copy exists.
- **Shrink the scope, and compute a value next to its use.** Do not declare it
  early or keep it alive after its last read.
- **Simplify the return type.** Each extra dimension is a branch at every call
  site. Prefer nothing to `bool`, `bool` to a value, a value to `(value, ok)`,
  and `(value, ok)` to `(value, error)`.
- **Zero the padding.** A buffer written short and sent long leaks what the last
  user left in it, and it breaks deterministic test output.
- **Group an allocation with its release.** A blank line before the acquisition
  and after the `defer` makes a missing `defer` visible in a diff.
- **Keep index, count and size apart.** An index plus one is a count, and a
  count times the unit is a size. Put the unit in the name: `octets`, `entries`
  and `slots` answer what `n` leaves open. When a division can round, write
  which way.

### The shape of a function

A function that fits on one screen can be read. A function that needs a scroll
must be remembered, and memory is where defects hide.

- Few parameters, a simple return type, and the logic between the braces.
- Keep the `switch` and `if` statements in the parent, and move branch-free
  fragments into helpers. The parent holds the state in locals, and a helper
  computes the change rather than applying it. A leaf function is pure.
- One concern in one file. Past 1000 lines, look for a second concern. Split
  only when the separation is right: one concern scattered over three files is
  worse than one long file. Full rule: `ai/rules/go-standards.md`, "File
  Modularity".
- Pass each option at the call site rather than relying on a library default.
  The call then states its behavior, and a changed default cannot change Ze in
  silence.
- Run at your own pace. When Ze meets an external system, it reads what has
  arrived and works on its own schedule. Control stays inside Ze, the work can
  batch, and the work for each unit of time has a bound.

### Performance

Solve performance in design, where the large wins are and where nothing can be
measured yet.

- **Sketch the four resources:** network, disk, memory, CPU, each with its
  bandwidth and its latency. Weight each by how often you touch it: a thousand
  cache misses cost more than one disk write.
- **Separate the control plane from the data plane.** A session negotiation runs
  once for each peer, and an UPDATE runs millions of times. The control plane
  can afford a check that the data plane cannot.
- **Batch,** for a system call, a disk write, a lock, and a wake-up alike, and
  give the CPU a large piece of work with a predictable shape.
- **Do not depend on the compiler.** Extract a hot loop into a function that
  takes primitive arguments and no receiver, so no struct field has to be
  proved cacheable and a redundant computation is visible.

Ze targets zero allocation on a wire path, because every allocation there is a
payment to the garbage collector.

| Rule | Detail |
|------|--------|
| The caller owns the buffer | A callee writes into `buf[off:]` and returns the byte count. It allocates nothing |
| A pool replaces `make` | A wire path takes its buffer from a bounded pool. `make` stays for a fixed-size header and a one-shot allocation at startup |
| The pool shape follows the goroutine shape | One sequential goroutine takes a ring. Concurrent goroutines take a `sync.Pool` seeded for the peak |
| One maximum size for every buffer in a pool | A variable size defeats the pool |
| A copy is deliberate | A copy on the wire path MUST match a trigger in `docs/architecture/buffer-architecture.md`, "When a copy is deliberate". A copy that fits none is a defect until that page names the new trigger |
| A string on a hot path is built once | Concatenation with `+`, `strings.Join`, and a `map[string]V` key each cost an allocation. Use one `textbuf.Buffer` and a typed key parsed at the boundary |
| A value that escapes to the heap | The caller passes an out pointer |

<!-- source: internal/core/textbuf/textbuf.go -- Buffer -->

Full rule: `ai/rules/performance.md`.

### Simplicity and debt

The simplest fully correct design is usually the hardest to find, so budget the
thinking for it. Simplicity cuts machinery and never correctness: an answer that
solves less of the problem is scope reduction, which is banned. Short is not
simple either, so write the version that is boring to read. Full rule:
`ai/rules/simplicity.md`.

Fix what you find when you find it. A hot-path copy, an unbounded queue, or an
algorithm quadratic in the peer count never enters with a promise of a later
fix. When you replace X with Y, delete X first (`ai/rules/no-layering.md`). Ze
is unreleased, so no compatibility shim exists (`ai/rules/go-standards.md`).
Only the plugin API contract freezes, and only after the first release.

No new third-party import enters until Thomas agrees, because each one adds a
supply-chain, safety, performance and install cost. Existing dependencies are
vendored, so a build never reaches the network.

Repository tooling is Go under `internal/le/`: an `./le <area> <action>` backed
by a callable package, with fixtures under its `testdata/` or
`internal/test/fixture`. Python stays only where an external Python program is
the subject.
<!-- source: internal/le/register.go -- native tooling composition root -->

## The rules a tool enforces

A finding from these tools names the rule, so the rule is not restated above.
`golangci-lint` runs in `./le go lint run` and in CI. The edit-time check is
`writeGoPatterns`, which refuses a `Write` or `Edit` of shipped Go (not
`_test.go`) that matches a pattern.
<!-- source: .golangci.yml -- revive rules -->
<!-- source: internal/le/hookruntime/writeedit.go -- writeGoPatterns -->

| Rule | Tool | What it refuses |
|------|------|-----------------|
| Guard, handle, return | revive `early-return`, `indent-error-flow`, `superfluous-else` | An `else` that a guard makes unnecessary |
| One fact per guard | `./le arch compound-guard check` | A terminating `if a \|\| b` on a changed line |
| State an invariant positively | ruleguard `negatedElse` | `if !c` or `if a != b` with an `else`. `err != nil` and the failure form of a bound stay allowed |
| Only a named panic | `writeGoPatterns` | A `panic(` in content that holds no panic with a `BUG`, `unreachable`, `not implemented`, `unimplemented`, `TODO` or `impossible` prefix. One allowed panic admits every other panic in the same content, so a peer-reachable panic stays a reader check (see "Assertions, in a language that has none") |
| No unchecked error | `errcheck` (type assertions too, `check-blank: false`), `forcetypeassert`, `nilerr`, `errorlint` | A call whose error result is ignored, an unchecked assertion, `return nil` beside a live `err`, an error compared with `==` or wrapped without `%w`. A blank discard `f, _ := open()` passes: see "Every error is handled" |
| No `nil, nil` answer | `nilnil` | A `(pointer, error)` function that returns neither |
| Every enum value handled | `exhaustive` (a `default` counts, so new code with no `default` is a reader check: see "A switch over a closed set has no default") | A `switch` over an enum that skips a value |
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
| A comment is a sentence | `godot` | A declaration comment with no full stop |
| US spelling | `misspell` | UK spellings |
| Modern Go | `modernize`, `intrange` | A pre-generics or pre-range-over-int idiom |
<!-- source: .golangci/ruleguard/modern.go -- negatedElse -->

The other enabled linters (`govet`, `staticcheck`, `unused`, `gosec`,
`unconvert`, `unparam`, `nakedret`, `prealloc`, `noctx`, `bodyclose`, `dupl`,
`goconst`, `tparallel`, `wastedassign`, `ineffassign`, gocritic's diagnostic,
style and performance tags) teach through their own messages.

### By the numbers

| Setting | Value |
|---------|-------|
| Formatting | `gofmt` and `goimports`. Ze imports come last, under the local prefix |
| Linting | `golangci-lint` MUST pass. Do not disable a linter. Fix the finding |
| A `//nolint` | Names the linter and carries the reason on the same line |
| Indentation | Tabs, because `gofmt` writes tabs. Every other file type is spaces, and `.editorconfig` carries the width |
| Line length | 100 columns, advisory: two copies of the code fit side by side on one screen |
| File length | 1000 lines is where you look for a second concern. It is the only threshold |
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

## Where Ze differs from standard Go

A reader trained on standard Go defaults to the wrong approach in each row
below. The buffer and pool rules are in "Performance" above.

### Encoding and wire

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `func (t T) Marshal() ([]byte, error)` | `func (t T) WriteTo(buf []byte, off int) int` | `ai/rules/performance.md` | Zero allocations on a hot path; the caller owns the buffer |
| `bytes.Buffer`, `append`, or `make` in a helper | A pooled buffer of one fixed maximum size, passed down by the caller | `ai/rules/performance.md` | One allocation at the outermost scope, and block accounting can release a block whole |
| Parse into structs eagerly | Lazy iterators over raw byte slices (`Next()`) | `ai/rules/architecture.md` | N to zero-until-needed, not N to one |
| `fmt.Sprintf`, `strings.Join` | `textbuf.Buffer` (128-byte stack inline) or `strconv.Append*` | `ai/rules/performance.md` | Sprintf allocates two to three times; textbuf allocates once |

### Architecture and registration

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Direct imports between packages | `init()` plus a registry plus a blank import | `ai/patterns/registration.md` | A small core discovers components and never imports them |
| Constructor injection | Registry lookup at runtime, such as `Registration.InProcessNLRIDecoder` through the family index | `ai/rules/plugins.md` | A plugin is removable by dropping its blank import |
| `os.Getenv("FOO")` | `env.Get("ze.foo")` through `internal/core/env` | `ai/rules/go-standards.md` | Caching, registration, dot and underscore agnostic, secret clearing |
| `log.Printf` or `logrus` | `slog` through `slogutil.Logger("subsystem")` | `ai/rules/go-standards.md` | Per-subsystem levels set by env var |
| Shared types by direct import | Cross-boundary payloads are value types only | `ai/rules/plugins.md` | No pointer fields cross a plugin or component boundary |

<!-- source: internal/core/env/env.go -- Get -->
<!-- source: internal/core/slogutil/slogutil.go -- Logger -->
<!-- source: internal/component/plugin/registry/registry.go -- Registration.InProcessNLRIDecoder -->

### Config and schema

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Struct tags plus `json.Unmarshal` | YANG schema as the sole source of truth | `ai/rules/config.md` | Schema-driven validation, migration, completion and diff |
| A config version field | No version numbers; machine-transformable migration | `ai/rules/config.md` | YANG evolution handles schema change |
| Silent defaults for missing fields | Fail on an unknown key and suggest the closest valid one | `ai/rules/config.md` | Explicit beats implicit |
| `interface{}` for flexible config | `map[string]any` through one canonical pipeline | `ai/rules/repo-maintenance.md` | File to Tree to `ResolveBGPTree` to `map[string]any` to `PeersFromTree` |

### Communication and IPC

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| gRPC or HTTP between services | JSON events down and text commands up, over pipes or `net.Pipe` | `ai/rules/plugins.md` | The plugin SDK is language-agnostic (Go, Python, Rust) |
| Direct function calls for synchronous work | DirectBridge for typed in-process calls | `ai/rules/plugins.md` | Skips JSON serialization for internal plugins |
| Channel-based pub/sub | EventBus with typed handles (`events.Register[T]`) | `ai/rules/plugins.md` | Type-safe registered event types, no raw `bus.Subscribe` |

<!-- source: internal/core/events/typed.go -- Register -->

### Testing

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `go test ./...` for verification | `./le verify worktree` (two-pass, plus functional and exabgp stages) | `ai/rules/testing.md` | Cached full run, race on the changed groups |
| Unit tests prove correctness | Unit tests and `.ci` functional tests, both required | `ai/rules/completion.md` | A unit test proves the algorithm; a `.ci` test proves a user can reach the feature |
| `testify/assert` | The standard library `testing` package | (convention) | No test framework dependencies |
| `go test -race` once | `go test -race -count=20 ./internal/component/bgp/reactor/...` for reactor code | `ai/rules/testing.md` | A rare schedule needs repeated runs to surface |

### CLI and commands

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `cobra` or `flag` | YANG-modeled dispatch with RPC handlers | `ai/patterns/cli-command.md` | One schema serves CLI, web, config and completion |
| `command <identifier> [flags]` | `<verb> <noun> <action> [<identifier>]` | `ai/rules/cli.md` | Removes identifier-keyword ambiguity |
| Format the output as a string | Return structured data and format through pipe operators | `ai/rules/cli.md` | `\| json`, `\| table`, `\| match`, `\| resolve` |
| Hardcode help text | Derive it from the registry or schema | `ai/rules/evidence.md` | One source of truth, no stale enumerations |

### Native tooling

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Ad-hoc scripts for tooling | A native Go package with a registered `./le` action | `ai/rules/go-standards.md` | One typed implementation serves the local caller and CI |
| `/tmp` for scratch files | The per-session directory from `./le session scratch ensure` | `ai/rules/commands.md` | Concurrent sessions never share a name |
| A bare staging verb followed by a bare commit | `./le commit create`, then the generated script | `ai/rules/git-safety.md` | The declared file population is checked before staging |
