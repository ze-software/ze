# Ze Go Style

Ze runs on a router that nobody restarts. The peer on the other side of the
socket is hostile, and the operator who reads the log is not a Go developer.
This page is the standard for every line of Go in the repository.

The page has three parts. The rules a reader applies come first, because no tool
checks them. Then come the rules a tool enforces, listed in one table one link
away. Last come the places where Ze differs from standard Go. When this page
and a file in `ai/rules/` disagree, the rule file
wins.

Ze has three design goals, in this order: safety, performance, developer
experience. All three matter. The order decides only what gives way when two
goals pull apart. The question each goal asks: [go-style/goals.md](go-style/goals.md).

## The rules a reader applies

### Assertions, in a language that has none

**A peer MUST NOT be able to panic the daemon.** This is the most important
rule on this page. A malformed UPDATE is an operating error, and every parser
returns an error for one. Use `panic("BUG: <what>")` only for a state that a Ze
defect alone can produce, never for a state that arrives over a socket.

**The best check is the one that never runs.** When a type can make a bad state
impossible to write, use the type and delete the check.

**Pair the check.** Check each property on two code paths.

**Test the negative space.** Defects live where data crosses from valid to
invalid, so every wire parser carries a fuzz target and every boundary gets a
case.

Panic, error or type, and why each rule holds:
[go-style/assertions.md](go-style/assertions.md).

### A zero value is never an answer

Give the outcome a name and make
the caller branch on it.

Before you write a zero, nil, false or empty result, ask two questions. Can a
caller tell this from a failure? Does anything downstream rely on this value
being absent? If it does, that reliance is a guard, and a guard gets a name, a
comment, and a test. Full rule: `ai/rules/principles.md`.

**Code that cannot answer refuses; it does not default.** A lookup that missed,
an input nobody recognized, a resource that could not be read: each is an
error, a protocol discard with a counted reason, or an explicit Unknown, never
the type's zero or a guessed constant. When the signature has no way to say so
(`interfaceExists` returning `bool`, a label allocator returning `uint32`), the
signature is what changes.

A probe that cannot look answers Unknown, with the reason it could not look,
and never "absent". Its caller does not read Unknown, or its own failure to
look, as the absence of what it asked about. The caller decides what Unknown
means for its own action, and names the choice.

The five shapes the check refuses and the guard annotation:
[go-style/zero-values.md](go-style/zero-values.md).

### Types that cannot lie

A type that can hold an invalid value will hold one.

- A value from a known set is a typed numeric enum. Zero MUST mean
  `Unspecified`, so the Go zero value is never a valid state.
- An address is `netip.Addr` or `netip.Prefix`, never a string. A duration is
  `time.Duration`, never an integer of unstated units.
- A string exists only at a boundary: a YANG leaf, a CLI token, a JSON key, a log
  line, an error message. Convert once at that boundary, then pass the typed
  value inward.
- A question about a config file, Ze's or a peer daemon's, is answered from its
  parsed structure, never by matching its text. This holds in `le` tooling and
  test harnesses too: a text match misses the nested form and fires on a
  comment.
- `String()` is for a human. Never compare with it.

The construction, sum-type and lifecycle contract, including its new-code scope
and legacy migration limits, lives in
[`preserve-validity-from-construction-through-use`](../../ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md).
Enum switch coverage has its own scope and applies to both old and new code.

Examples, and the Go mechanisms of this section and the four below:
[go-style/types.md](go-style/types.md).

#### One state, one variant

A struct whose fields are valid only in some combinations is a sum type in
disguise. Separate variants remove those combinations because each variant
holds only its own payload.

Sealed-interface type switches are separate. Their consumers still list each
variant with no `default`, under the new-code rule above.

#### Every enum switch has a coverage policy

Classify each enum value switch by its input and purpose. A `default` handles
values outside the named set; it does not replace explicit named cases.

| Class | Input and purpose | Required handling |
|-------|-------------------|-------------------|
| C: closed, internal | A value only Ze produces | List every distinct named value explicitly. A retained `default` is only `panic("BUG: <what>")`, after tracing constructors, mutations and callers to prove that unvalidated external input cannot reach it. |
| O: open, from outside | A code from the wire, a file, config or a plugin | List every accessible distinct named value explicitly. Keep a safe, non-panicking unknown handler in `default`, with a comment that the set is open. |
| P: intentionally partial | A predicate, projection or independent handler for a legitimate subset | Put a normal `// <why this switch handles a subset>` comment immediately before an argument-free `//exhaustive:ignore` directly above the switch. Preserve the behavior for nonmatching values. |

A complete dispatcher or state/event transition table is C or O, even when some
named events do nothing. P is not an exemption for missing cases in such a
dispatcher. Do not use a blanket `nolint:exhaustive`, file exemption, cast or
untagged switch to avoid coverage.

#### Validated at construction

Validation at creation rejects invalid input before it becomes a domain value.
Encapsulation then protects that value: private fields prevent a caller in
another package from replacing data that the constructor checked.

#### One type per lifecycle state

**Ze wants this pattern, and it is the first design to reach for.** Go has no
tagged enum, but it has the half that matters: an operation that is a method of
the validated type, or that takes the validated type as its parameter, cannot be
called with anything else. When Ze controls the transition, write that type
instead of a state field and an `if` at the top of each operation.

In Go, write a progression of states (loaded, then resolved; parsed, then
validated) as distinct named types, each holding only its own methods.

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

Write the guard, handle it, and return.

Split a compound condition into nested branches. For each `if`, ask whether the
negative case also needs a branch.

One guard states one fact. `if a || b { return err }` is two guards written as
one: write `if a { return err }`, then `if b { return err }`.

State an invariant positively. `if index < length` is easy to read. With an
`else`, write `if c` rather than `if !c`, and swap the branches.

What `./le arch compound-guard check` refuses:
[go-style/control-flow.md](go-style/control-flow.md).

### Every error is handled

Ze has no discarded error, no silent default, and no fallback value invented at
the point of failure. Fail early: a config that does not parse stops the load,
and an absent value is an error,
never `0.0.0.0/0`. Do not discard an error into `_`, as in `f, _ := open()`:
no linter refuses that form, so the reader is the check. When a discard is
correct, write `//nolint:errcheck // <the reason>`.

An error that is caught and not returned is logged at WARN or above, never at
debug or info. Dropping an error is correct only where going on is correct.

A wrap adds only the context its error does not already carry. One layer owns
each segment, and the others pass the error through.

**An error that a registered operator handler returns is an `opererr.Error`.**
`opererr.New` and `opererr.Wrap` take what went wrong, why it matters, and the
command that fixes it.

The error-wrap check, and how an operator error reaches a response:
[go-style/errors.md](go-style/errors.md).

### Goroutines

Every goroutine is a long-lived worker with an owner and a stop path. A
long-lived goroutine reading from a channel is required. One goroutine for one
lifecycle (a process, a session, a peer) is permitted. One goroutine for one
event in a hot path, or `go func()` in a `for range` over events, is forbidden.

Create the channel and start the worker, enqueue on the hot path, close the
channel to stop. A type that owns a goroutine states that sequence in its doc
comment. Full rule: `ai/rules/goroutine-lifecycle.md`.

### Comments say why and state the contract

The code says what happens. The comment says why this, and not the obvious
alternative.

| Comment | Requirement |
|---------|-------------|
| The file header | `// Design:` first, then `// Detail:`, `// Overview:`, or `// Related:` to the sibling files a reader needs |
| A caller obligation | MUST, on both sides of the pair. `Stop` says "MUST call Wait after". `Wait` says "MUST be called after Stop" |
| Concurrency | "Safe for concurrent use", or the opposite. Silence is not an answer |
| A test | Opens with its goal and its method, so the next reader can skip it or trust it |

A comment that no longer matches the code is worse than no comment, so the
comments change in the same edit as the behavior. Full rules:
`ai/rules/go-standards.md` and `ai/rules/stale-comments.md`.

The form of every comment, and why the reason matters:
[go-style/comments.md](go-style/comments.md).

### Names

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
fields, then types, then methods.

Why a name matters, and the order when none is obvious:
[go-style/names.md](go-style/names.md).

### State that goes stale

- **Do not copy a variable or alias one.** Two names for one fact will disagree.
- **Reuse a cache or an artifact only on proof that it is complete.**
- **Build a large struct in place.**
- **Shrink the scope, and compute a value next to its use.**
- **Simplify the return type.** Prefer nothing to `bool`, `bool` to a value, a
  value to `(value, ok)`, and `(value, ok)` to `(value, error)`.
- **Zero the padding.**
- **Group an allocation with its release.**
- **Keep index, count and size apart.**

What counts as proof, and how each rule is written:
[go-style/stale-state.md](go-style/stale-state.md).

### The shape of a function

A function that fits on one screen can be read.

- Few parameters, a simple return type, and the logic between the braces.
- Keep the `switch` and `if` statements in the parent, and move branch-free
  fragments into helpers.
- One concern in one file. Past 1000 lines, look for a second concern.
- Pass each option at the call site rather than relying on a library default.
- Run at your own pace. When Ze meets an external system, it reads what has
  arrived and works on its own schedule.

When to split a file, and why each rule holds:
[go-style/function-shape.md](go-style/function-shape.md).

### Performance

Solve performance in design, where the large wins are and where nothing can be
measured yet.

Ze targets zero allocation on a wire path, because every allocation there is a
payment to the garbage collector. Full rule: `ai/rules/performance.md`.

**A copy is deliberate.** A copy on the wire path MUST match a trigger in
`docs/architecture/buffer-architecture.md`, "When a copy is deliberate". A copy
that fits none is a defect until that page names the new trigger.

The four resources, the compiler, and the buffer and pool table:
[go-style/performance.md](go-style/performance.md).

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

Adding a dependency, and where repository tooling lives:
[go-style/simplicity.md](go-style/simplicity.md).

## The rules a tool enforces

A finding from these tools names the rule, so the rule is not restated above.
`golangci-lint` runs in `./le go lint run` and in CI.

Each rule, its tool and what it refuses, and the other settings:
[go-style/tool-rules.md](go-style/tool-rules.md).

### By the numbers

Line length: 100 columns, advisory: two copies of the code fit side by side on
one screen. File length: 1000 lines is where you look for a second concern. It
is the only threshold.

## Where Ze differs from standard Go

A reader trained on standard Go defaults to the wrong approach in each row
below.

Every other difference:
[go-style/standard-go.md](go-style/standard-go.md).

### Encoding and wire

`func (t T) WriteTo(buf []byte, off int) int`, never
`func (t T) Marshal() ([]byte, error)`: zero allocations on a hot path; the
caller owns the buffer (`ai/rules/performance.md`).

### Architecture and registration

`init()` plus a registry plus a blank import, never direct imports between
packages: a small core discovers components and never imports them
(`ai/patterns/registration.md`).

### Config and schema

YANG schema as the sole source of truth, never struct tags plus
`json.Unmarshal`: schema-driven validation, migration, completion and diff
(`ai/rules/config.md`).

### Communication and IPC

JSON events down and text commands up, over pipes or `net.Pipe`, never gRPC or
HTTP between services: the plugin SDK is language-agnostic (Go, Python, Rust)
(`ai/rules/plugins.md`).

### Testing

Unit tests and `.ci` functional tests, both required, where standard Go holds
that unit tests prove correctness: a unit test proves the algorithm; a `.ci`
test proves a user can reach the feature (`ai/rules/completion.md`).

### CLI and commands

`<verb> <noun> <action> [<identifier>]`, never `command <identifier> [flags]`:
it removes identifier-keyword ambiguity (`ai/rules/cli.md`).

### Native tooling

A native Go package with a registered `./le` action, never ad-hoc scripts for
tooling: one typed implementation serves the local caller and CI
(`ai/rules/go-standards.md`).
