# Ze Go Style

Ze runs on a router that nobody restarts. A defect drops a session, blackholes a
prefix, or leaks a buffer that grows for a year. The peer on the other side of
the socket is not friendly, and the operator reading the log is not a Go
developer.

Ze code is written to that standard. This page is the working standard for every
line of Go in the repository. It has two companions. `writing-style.md` is the
standard for every word. `ze-python-style.md` explains where external Python
references remain valid while all first-party tooling stays in Go.

This page carries the reasoning. The blocking mechanical detail lives in
`ai/rules/`, and each section names the rule that owns it. When this page and a
rule file disagree, the rule file wins.

## Why have style

Another word for style is design.

Ze has three design goals, in this order.

| # | Goal | The question it asks |
|---|------|----------------------|
| 1 | Safety | What happens when this input is hostile, this peer is dead, or this buffer is full? |
| 2 | Performance | What does this cost for each UPDATE, for each peer, for each second? |
| 3 | Developer experience | Can the next reader change this code without a second file open? |

All three matter. The order decides one thing only: what gives way when two
goals pull apart.

Style serves the goals. A rule earns its place on this page when it makes Ze
safer, faster, or easier to change. Readability is the floor, not the target.

## Simplicity is the hard revision

Simplicity is not a smaller answer to the problem. It is the same answer with
less machinery.

The simplest fully correct design is usually the hardest one to find, so budget
thinking time for it. The first shape that works is rarely the simplest one. A
large diff is more often the cheap answer than the good one.

Simplicity governs the shape of the answer. It never governs how much of the
problem the answer solves. Cutting correctness to reach a smaller diff is scope
reduction, and scope reduction is banned.

Short is not simple either. A dense expression that the reader must simulate
fails this standard exactly as a five-file framework fails it. Write the version
that is boring to read.

Full rule: `ai/rules/simplicity.md` (blocking).

## Zero technical debt

Code, like steel, is cheaper to change while it is hot. A defect found in design
costs an hour. The same defect found in production costs a week, and it costs it
to somebody else.

So Ze fixes what it finds, when it finds it. A memory copy in a hot path does
not enter the tree with a comment that promises a later fix. Neither does an
unbounded queue, and neither does an algorithm that grows with the square of the
peer count.

Two habits carry this.

| Habit | Rule |
|-------|------|
| When you replace X with Y, delete X first, then write Y. Never keep both | `ai/rules/no-layering.md` |
| Ze has never been released, so no compatibility shim exists anywhere. When something needs to change, change it | `ai/rules/go-standards.md` |

The one frozen surface is the plugin API, and only after the first release. Its
implementation stays free to change. Its contract does not.

## Safety

### Control flow a reader can simulate

Write the guard, handle it, and return. A sequence of guard clauses followed by
the main logic reads from top to bottom. A happy path wrapped in an `else` block
does not.

Split a compound condition into nested branches. A reader who meets
`if a && b && !c` must hold three facts at once to decide whether every case is
covered. Ask, for each `if`, whether the negative case also needs a branch or a
guard.

State an invariant positively. `if index < length` is easy to read. `if index >=
length` states the failure of the invariant, and the reader must invert it.

Recursion over a structure that an external peer controls is forbidden, because
the peer chooses the depth. Recursion over a bounded internal structure is
permitted, and the bound belongs in a comment above the function.

### A limit on everything

Everything in reality has a limit, so state the limit in the code.

| Thing | Bound |
|-------|-------|
| A loop over external input | The message length, checked before the loop starts |
| A pool | A fixed slot count, and one maximum buffer size for every slot |
| A queue or a channel | A declared depth, and a defined behavior when it is full |
| A retry | A count and a maximum backoff |
| A cache | An entry count or a byte budget |

A loop that cannot end, the reactor loop for example, is correct. Say so in a
comment at the top of the loop, so the reader knows the absence of a bound is a
decision.

### Types that cannot lie

A type that can hold an invalid value will hold one.

- Use a typed numeric enum for a value from a known set. Zero MUST mean
  `Unspecified`, so the Go zero value is never a valid state.
- Use `netip.Addr` and `netip.Prefix` for an address, never a string.
- Use `time.Duration` for a duration, never an integer of unstated units.
- Use a string only at a boundary: a YANG leaf, a CLI token, a JSON key, a log
  line, an error message. Convert once at that boundary, then pass the typed
  value inward.
- Use `String()` for a human. Never compare with it.

Full rule: `ai/rules/go-standards.md`, "Prefer Typed Numeric Over String".

Storing an address as a string and parsing it back for a comparison is the
common shape of this failure.

| Anti-pattern | Fix |
|-------------|-----|
| `type Foo struct { Addr string }` then `net.ParseIP(a.Addr).Compare(...)` | `type Foo struct { Addr netip.Addr }` then `a.Addr.Compare(b.Addr)` |
| Formatting to a string, storing it, parsing it back to compare | Parse once at construction, store the typed value, format only for display |
| `compareAddrs(a.PeerAddr, b.PeerAddr)` with string parsing inside | `a.PeerIP.Compare(b.PeerIP)` over a `netip.Addr` field |

When a struct genuinely needs both forms, the typed one for comparison and the
string one for a map key or JSON, it stores both and parses once at
construction.

```go
type Candidate struct {
    PeerAddr string     // for map keys, JSON, interning
    PeerIP   netip.Addr // for comparison (zero-alloc)
}

// At construction:
c.PeerAddr = peerAddr
c.PeerIP, _ = netip.ParseAddr(peerAddr)
```

### A zero value is never an answer

The section above asks a type not to hold an invalid value. A zero value is the
harder case, because it is the value every field holds before anybody writes to
it, so it cannot be told apart from a field nobody set.

**The dangerous form is the zero that behaves correctly.** A missing branch and
a deliberate no-op produce the same silence, so a reader cannot tell whether the
case was handled or forgotten, and the next change deletes the behavior without
touching a line that mentions it. Give the outcome a name and make the caller
branch on it.

| The accident | Why it holds today | What breaks it |
|--------------|--------------------|----------------|
| An all-zero result means "drop this packet", because the caller sends only when a field is non-nil | the wire behavior is right | a second outcome that also sends nothing, now indistinguishable from a bug |
| An empty set means "the peer offered nothing", read as "we failed to read the peer" | both are rare | the first peer that legitimately offers nothing |
| A search returning no hits is read as absence | the corpus was complete | the first query that fails rather than finds nothing |

**The sharpest case is a zero that is guarding without being a guard.** A test
written to ask *which value do I use* can, by accident, also answer *is this
allowed yet* — because the value happens to stay zero until the moment
authorization is granted. Nothing names the second job, no test covers it, and
the comment above it describes only the first. Change the type so a legitimate
zero appears, and the guard is gone with no line deleted and no test red.

Ze has paid for this three times in one package. `PeerResult` needed an explicit
`Discarded`, because a silent drop and an unwritten branch were the same value.
`PeerResult` needed `Notified` beside its message, because a Notification whose
message is legitimately empty is still a Notification. And `verifyRemoteAuth`
gated an EAP peer's AUTH payload on `sa.EAPMSK != [64]byte{}`, a test asking
which key to sign with, which happened to reject an AUTH arriving before the EAP
exchange succeeded — until a method that derives no key made the MSK legitimately
zero, at which point the accidental guard would have become an authentication
bypass. The replacement asks the exchange whether it succeeded, on purpose.

So, before writing a zero, nil, false or empty as a result, ask two questions.
Can a caller tell this from a failure? And is anything downstream relying on this
value being absent? If the second answer is yes, that reliance is a guard, and a
guard gets a name, a comment and a test.

Full rule: `ai/rules/principles.md` (blocking).

### Assertions, in a language that has none

An assertion detects a programmer error. An error return handles an operating
error. The two are different, and Go gives no keyword for the first one, so Ze
uses three tools.

| The failure | The tool |
|-------------|----------|
| A programmer error that MUST NOT happen at runtime | `panic("BUG: <what>")` |
| An operating error that a running system produces | An error return, wrapped with `fmt.Errorf("context: %w", err)` |
| A design property that holds before the program runs | `var _ Iface = (*T)(nil)`, or a test |

The allowed panic prefixes are `BUG`, `unreachable`, `not implemented`,
`unimplemented`, `TODO`, and `impossible`. Any other `panic()` is refused when
the file is written.
<!-- source: internal/le/hookruntime/writeedit.go -- writeGoPatterns -->

Three habits make the tools earn their cost.

**A peer MUST NOT be able to panic the daemon.** A malformed UPDATE is an
operating error, and every parser returns an error for one. Use `panic("BUG:")`
for a state that only a Ze defect can produce, never for a state that arrives
over a socket. This is the single most important line on this page.

**Pair the check.** For each property you want to enforce, find two code paths
that can carry a check. Validate an attribute when it is parsed from the wire,
and validate it again when it is written back to the wire. A bug that survives
one check rarely survives both, and the pair also documents the property twice.

**Test the negative space.** A test that feeds valid data proves the code reads
valid data. Interesting defects live where data crosses from valid to invalid,
so every wire parser carries a fuzz target and every boundary gets a case.
<!-- source: ai/rules/testing.md -- Boundary Testing, Fuzz -->

A fuzz target proves that defects are present. It never proves that they are
absent. So build the mental model first, write the checks that encode it, then
let the fuzzer find the gap between the model and the code.

### Memory

Ze forwards millions of UPDATE messages. Go gives you a garbage collector, and
every allocation on that path is a payment to it. Ze targets zero allocation on
a wire path.

| Rule | Detail |
|------|--------|
| The caller owns the buffer | A callee writes into `buf[off:]` and returns the byte count. It allocates nothing |
| A pool replaces `make` | A wire-facing path takes its buffer from a bounded pool. `make` stays for a fixed-size header and for a one-shot allocation at startup |
| The pool shape follows the goroutine shape | One sequential goroutine takes a ring. Concurrent goroutines take a `sync.Pool` seeded for the peak |
| Every buffer in a pool has the same maximum size | Variable-sized allocation defeats the pool |
| A copy is deliberate | Ze has four reasons to copy. A copy that fits none of them is a defect until somebody names the fifth reason |

Full rule: `ai/rules/performance.md`.

### Every error is handled

An analysis of production failures in distributed data-intensive systems found
one dominant cause. Most catastrophic failures came from the handling of errors
that the software had already detected.

> "Specifically, we found that almost all (92%) of the catastrophic system
> failures are the result of incorrect handling of non-fatal errors explicitly
> signaled in software."

So: no discarded error, no silent default, no fallback value invented at the
point of failure. `f, _ := open()` is refused. When a discard is genuinely
correct, write `//nolint:errcheck // <the reason>` and give the reason.

Fail early. A config that does not parse stops the load. A value that is absent
is an error, never `0.0.0.0/0`.

### Goroutines

Every goroutine is a long-lived worker with an owner and a stop path.

| Pattern | Status |
|---------|--------|
| A long-lived goroutine reading from a channel | Required |
| One goroutine for one lifecycle: a process, a session, a peer | Permitted |
| One goroutine for one event in a hot path | Forbidden |
| `go func()` inside a `for range` over events | Forbidden |

The shape is: create the channel and start the worker, enqueue on the hot path,
close the channel to stop. When a type owns a goroutine, its doc comment states
the sequence, and `Stop` says that the caller MUST call `Wait` after it.

Full rule: `ai/rules/goroutine-lifecycle.md`.

### The shape of a function

A function that fits on one screen can be read. A function that needs a scroll
must be remembered instead, and memory is where defects hide.

- A good function is the inverse of an hourglass: few parameters, a simple
  return type, and a lot of logic between the braces.
- Centralize control flow. When you split a large function, keep the `switch`
  and the `if` statements in the parent, and move the branch-free fragments into
  helpers. One function owns the control flow, and the rest ignore it.
- Centralize state. The parent holds the state in local variables, and a helper
  computes what changes rather than applying the change itself. A leaf function
  is pure.
- One concern in one file. Past 1000 lines, look for a second concern. Split
  only when the separation is right, because a forced split that scatters one
  concern across three files is worse than one long cohesive file.

Full rule: `ai/rules/go-standards.md`, "File Modularity".

### Run at your own pace

When Ze meets an external system, Ze does not act at the moment of each external
event. Ze reads what has arrived, then works on its own schedule.

This keeps control flow inside Ze, which is a safety property. It also lets Ze
batch, which is a performance property. And it makes the work for each unit of
time possible to bound, which is the first rule of this section.

### Always say why

Never forget to say why. An explained decision gives the reader the criteria to
judge it, and the reader who has the criteria can change the code safely.

Code is not documentation of itself. The code says what happens. The comment
says why that, and not the obvious alternative.

### Pass the option at the call site

Write the option out rather than relying on a default from a library. The call
then states its own behavior, and a change of default in the library cannot
change Ze in silence.

## Performance

> "The lack of back-of-the-envelope performance sketches is the root of all
> evil."

**Solve performance in design, because that is where the large wins are.** In
the design phase you cannot measure or profile, which is exactly why the
sketches matter. After implementation the fix is harder and the gain is smaller.

**Sketch the four resources.** Network, disk, memory, and CPU, each with its
bandwidth and its latency. A sketch is cheap, and a rough sketch lands close to
the best design.

**Weight each resource by how often you touch it.** The order network, disk,
memory, CPU is the order of raw cost. A memory cache miss that happens a
thousand times costs more than one disk write, so count the accesses before you
pick the target.

**Separate the control plane from the data plane.** A session negotiation runs
once for each peer, and an UPDATE runs millions of times. The control plane can
afford a check that the data plane cannot, and the line between them is what
lets Ze carry both.

**Batch.** Batching amortizes the cost of every one of the four resources. It is
the same answer for a system call, a disk write, a lock, and a wake-up.

**Let the CPU sprint.** Give it a large piece of work with a predictable shape.
Do not make it change lanes for each message.

**Be explicit. Do not depend on the compiler.** Extract a hot loop into a
standalone function that takes primitive arguments and no receiver. The compiler
then has no struct fields to prove it can cache in registers, and a reader can
see a redundant computation.

Go adds its own costs, and each one has a cheaper form.

| Cost | The cheaper form |
|------|------------------|
| `fmt.Sprintf` on a hot path | `textbuf.Buffer`, or `strconv.Append*` into a buffer you own |
| String concatenation with `+` | One `textbuf.Buffer` |
| `strings.Join(parts, " ")` | One buffer, with a separator written between the parts |
| A `map[string]V` on a hot path | A numeric or typed-enum key, parsed once at the boundary |
| A value that escapes to the heap | An out pointer from the caller |

<!-- source: internal/core/textbuf/textbuf.go -- Buffer -->

Full rule: `ai/rules/performance.md`.

## Developer experience

> "There are only two hard things in Computer Science: cache invalidation,
> naming things, and off-by-one errors."

### Names

**Get the nouns and the verbs right.** A great name captures what a thing is or
what it does, and it proves that the author understood the domain. Take the time
to find it.

**Describe what the value is, never its Go type.** `famStr`, `levelStr`, and
`addrStr` name the type. `family`, `level`, and `addr` name the value. When two
variables hold one concept in two forms, separate them by meaning: `afiName` for
the name, and `afi` for the numeric code.

**Put the qualifier last, by descending significance.** Write `latencyMsMax`
rather than `maxLatencyMs`. Then `latencyMsMin` lines up beside it, and every
latency name sorts together.

**Infuse a name with meaning.** `logger` is correct and dull. `peerLog` and
`wireLog` tell the reader which subsystem writes the line.

**Give related names the same length.** As arguments to a copy, `source` and
`target` beat `src` and `dst`, because `sourceOffset` and `targetOffset` then
line up in every slice expression that follows. Code that lines up is code the
eye can check.

**Give the helper the name of its caller.** `writeUpdate` and `writeUpdateBody`
show the call history in the file listing. A reader who greps one finds the
other.

**Order the file for the first read.** The entry point goes first. In a type,
the order is fields, then types, then methods. When no order is obviously right,
sort alphabetically and take the benefit of names that start with the concept.

**Do not overload a name.** Ze keeps `delete` for config, `clear` for counters,
and `remove` for a route. One name that carries two meanings costs the reader a
guess on every page.

**Name for the document, not only for the compiler.** A noun goes straight into
a sentence, an email, or a heading. A present participle must be rephrased
first. `peer.pipeline` beats `peer.preparing`, and it composes into
`pipelineMax`.

Go casing, package naming, and the package glossary: `docs/contributing/go-conventions.md`.

### Comments

| Comment | Requirement |
|---------|-------------|
| The file header | `// Design:` first, then `// Detail:`, `// Overview:`, or `// Related:` to the sibling files that a reader needs |
| A caller obligation | State it with MUST, on both sides of the pair. `Stop` says "MUST call Wait after". `Wait` says "MUST be called after Stop" |
| Concurrency | Say "Safe for concurrent use", or say the opposite. Silence is not an answer |
| A test | Open with the goal and the method, so the next reader can skip it or trust it |
| Any comment | Sentences. A capital letter, a space after the slashes, and a full stop |

A comment that no longer matches the code is worse than no comment. When you
change behavior, the comments that described the old behavior change in the same
edit.

Full rules: `ai/rules/go-standards.md` and `ai/rules/stale-comments.md`.

### State that goes stale

Most defects come from a gap in time or in space between where a value is
checked and where it is used.

- **Do not copy a variable, and do not take an alias to one.** Two names for one
  fact will disagree.
- **Pass a large struct as a pointer.** Go copies a value argument at every
  call. The linter reports a value parameter above its size threshold.
  <!-- source: .golangci.yml -- gocritic hugeParam sizeThreshold -->
- **Build a large struct in place.** Give the constructor an out pointer, so the
  value is written where it will live. This holds the pointers stable and
  removes the intermediate copy.
- **Shrink the scope.** Fewer variables in scope means fewer variables to
  confuse.
- **Compute a value next to where it is used.** Do not declare it early, and do
  not leave it alive after the last read.
- **Simplify the return type.** Each extra dimension is a branch at every call
  site, and it propagates up the chain.

  | Prefer | Over |
  |--------|------|
  | Nothing | `bool` |
  | `bool` | A value |
  | A value | `(value, ok)` |
  | `(value, ok)` | `(value, error)` |

- **Zero the padding.** A buffer that is written short and sent long leaks
  whatever the last user left in it. It also breaks the deterministic output
  that Ze tests depend on.
- **Group an allocation with its release.** A blank line before the acquisition
  and a blank line after the `defer` makes a missing `defer` visible in a diff.

### Off-by-one

An index, a count, and a size are three different types that Go spells the same
way.

| From | To | Operation |
|------|----|-----------|
| Index | Count | Add one. An index starts at zero, and a count starts at one |
| Count | Size | Multiply by the unit |

This is the second reason to put the unit in the name. `octets`, `entries`, and
`slots` each answer the question that `n` leaves open.

Show the intent of a division. When a division can round, write which way it
rounds, so the reader knows that you thought about the case.

### By the numbers

| Setting | Value |
|---------|-------|
| Formatting | `gofmt` and `goimports`. Ze imports come last, under the local prefix |
| Linting | `golangci-lint` MUST pass. Do not disable a linter. Fix the finding |
| A `//nolint` | Carries the specific reason on the same line |
| Indentation | Tabs, because `gofmt` writes tabs. Every other file type is spaces, and `.editorconfig` at the repository root carries the width for each one |
| Line length | 100 columns is the target, and it is advisory. The number is physical: two copies of the code fit beside each other on one screen |
| File length | 1000 lines is the point at which you look for a second concern. It is the only threshold |
| Test file length | No threshold. A table of cases grows with coverage |
| Function length | One screen is the target, and it is advisory |

<!-- source: .golangci.yml -- linters, formatters -->
<!-- source: .editorconfig -- indentation per file type -->

### Dependencies

Ze takes no new third-party import until you ask Thomas and he agrees. A dependency
carries a supply-chain risk, a safety risk, a performance risk, and an install
cost. For infrastructure that other software runs on, every one of those costs
is multiplied down the stack.

The dependencies that Ze has are vendored in the repository, so a build never
reaches the network.

### Tooling

A tool has a cost. A small standard toolbox is easier to operate than a shelf of
specialist instruments, each with its own manual.

> "The right tool for the job is often the tool you are already using -- adding
> new tools has a higher cost than many people appreciate"

Ze writes repository tooling in Go under `internal/le/`. A new development
workflow is an `./le <area> <action>` backed by a callable Go package, and its
fixtures live under that package's `testdata/` directory or
`internal/test/fixture`. Python remains relevant only when documentation or
interoperability concerns an external Python program.
<!-- source: internal/le/register.go -- native tooling composition root -->

## Where Ze differs from standard Go

Ze differs from a typical Go project in specific, load-bearing ways. A reader
trained on standard Go patterns defaults to the wrong approach in each of the
rows below. Each row names the standard approach, the Ze approach, the rule
that governs it, and the reason.

### Encoding and wire

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `func (t T) Marshal() ([]byte, error)` | `func (t T) WriteTo(buf []byte, off int) int` | `ai/rules/performance.md` | Zero allocations on a hot path; the caller owns the buffer |
| `bytes.Buffer` or `append` in helpers | Pre-allocated pooled buffers, sliced inward | `ai/rules/performance.md` | Bounded memory, no GC pressure |
| `make([]byte, n)` for variable-length wire data | Pool-backed buffers of one fixed maximum size | `ai/rules/performance.md` | Block accounting can release a block whole |
| A helper allocating its own scratch | The caller passes the buffer down and the callee writes into it | `ai/rules/performance.md` | One allocation at the outermost scope, not N in sub-functions |
| `sync.Pool` only for reuse | `sync.Pool` for multi-goroutine scratch, a ring for a single goroutine | `ai/rules/performance.md` | The pool shape follows the goroutine shape |
| Parse into structs eagerly | Lazy iterators over raw byte slices (`Next()`) | `ai/rules/architecture.md` | N to zero-until-needed, not N to one |
| `fmt.Sprintf` for formatting | `textbuf.Buffer` (128-byte stack inline) or `strconv.Append*` | `ai/rules/performance.md` | Sprintf allocates two to three times; textbuf allocates once |
| `strings.Join(parts, " ")` | One `textbuf.Buffer` with `.Byte(' ')` separators | `ai/rules/performance.md` | Removes the intermediate `[]string` and the final join |

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

## Where Ze differs from TigerStyle

The differences are the places where Go, or Ze's own history, gives a different
answer. Each one is deliberate.

| Subject | TigerStyle | Ze |
|---------|-----------|-----|
| Case | `snake_case` for everything | Go casing. `gofmt` and the standard library set it, and fighting them costs more than it returns |
| Function length | A hard limit of 70 lines | One screen, advisory. No gate counts the lines |
| Line length | A hard limit of 100 columns | 100 columns, advisory |
| Indentation | 4 spaces | Tabs, written by `gofmt` |
| Allocation | No dynamic allocation after startup | Zero allocation on a wire path, through bounded pools. Go has a garbage collector, so the target is the hot path rather than the whole program |
| Repository tooling | Zig | Go actions under `internal/le/` |
| Dependencies | Zero, apart from the toolchain | Vendored Go modules. A new one needs Thomas to agree first |
| Assertion density | At least two for each function | Ze counts nothing. `panic("BUG:")` marks a state that a Ze defect alone can reach, and a peer never reaches one |

## Lineage

This standard follows TigerStyle, the coding standard of TigerBeetle. It is
restated for Go, for a routing daemon, and for this repository, and the examples
are Ze's own.

Source: `https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md`

The quotations on this page come from that document. It credits Rivacindela
Hudsoni for the sketching line, Phil Karlton for the hard things in computer
science, and John Carmack for the tools. The failure statistic comes from "Simple
Testing Can Prevent Most Critical Failures", published at OSDI 2014.
