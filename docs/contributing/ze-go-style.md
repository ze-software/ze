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

The construction, sum-type and lifecycle contract, including its new-code scope
and legacy migration limits, lives in
[`preserve-validity-from-construction-through-use`](../../ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md).
The sections below explain the Go mechanisms and their limits. Enum switch
coverage has its own scope and applies to both old and new code.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- construction and representation scope -->

#### One state, one variant

A struct whose fields are valid only in some combinations is a sum type in
disguise. With `done bool`, `result *T` and `reason string`, a caller can build
"done with a reason" or "not done with a result". Separate variants remove those
combinations because each variant holds only its own payload. This example
illustrates that separation; payload validity follows the construction contract
below.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- incompatible payload combinations -->

```go
type Outcome interface{ outcome() } // package marker; see the limits below

type Pending struct{}
type Done struct{ result Route }
type Failed struct{ reason error }

func (Pending) outcome() {}
func (Done) outcome()    {}
func (Failed) outcome()  {}
```

This pattern is often called a sealed interface. An unexported method prevents
another package from declaring that method directly, but embedding an existing
implementation or the interface can promote it into another type's method set.
The interface also admits nil, and pointer implementations can carry typed nils.
A boundary that accepts an `Outcome` therefore needs an admission contract for
these cases before an internal consumer relies on the known variants. The
[Go specification on struct types](https://go.dev/ref/spec#Struct_types) defines
method promotion; its [variables section](https://go.dev/ref/spec#Variables)
explains dynamic types, including typed nils.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- marker interfaces and nil contract -->

Within that contract, a consumer reads only the payload of the selected variant.
The type-switch coverage policy below remains a reader check; the interface
declaration alone proves neither exhaustive handling nor payload validity.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- representation validity -->

#### Every enum switch has a coverage policy

Classify each enum value switch by its input and purpose. A `default` handles
values outside the named set; it does not replace explicit named cases.

| Class | Input and purpose | Required handling |
|-------|-------------------|-------------------|
| C: closed, internal | A value only Ze produces | List every distinct named value explicitly. A retained `default` is only `panic("BUG: <what>")`, after tracing constructors, mutations and callers to prove that unvalidated external input cannot reach it. |
| O: open, from outside | A code from the wire, a file, config or a plugin | List every accessible distinct named value explicitly. Keep a safe, non-panicking unknown handler in `default`, with a comment that the set is open. |
| P: intentionally partial | A predicate, projection or independent handler for a legitimate subset | Put `//exhaustive:ignore // <why this switch handles a subset>` immediately before the switch. Preserve the behavior for nonmatching values. |

A complete dispatcher or state/event transition table is C or O, even when some
named events do nothing. P is not an exemption for missing cases in such a
dispatcher. Do not use a blanket `nolint:exhaustive`, file exemption, cast or
untagged switch to avoid coverage.

Preserve existing named behavior, including unsupported-value errors, sentinel
outcomes and valid zeros. This coverage migration does not impose invalid-zero
semantics on existing types: preserve an existing valid unnamed zero path
explicitly too. Inspect behavior after the switch, not only its arms. Do not
add protocol support or change masked-code recognition to make a case explicit.

Completeness is by distinct constant value: one named case covers same-valued
aliases. Within the enum's package, private members count; across package
boundaries, only exported members are required. Use named constants, not
literals, to satisfy named-member coverage.
<!-- source: vendor/github.com/nishanths/exhaustive/common.go -- checklist.add, checklist.found, exprConstVal -->

The `exhaustive` linter checks all unignored enum value switches, including
aliases, with `.golangci.yml` setting `default-signifies-exhaustive: false`.
A `default` never substitutes for a missing named member. Reviewers MUST still
apply C/O/P to old and new switches, including those whose named cases are
already complete: the linter does not prove input closure, judge subset reasons
or check default and post-switch behavior.
<!-- source: .golangci.yml -- exhaustive default-signifies-exhaustive -->
<!-- source: vendor/github.com/nishanths/exhaustive/switch.go -- switchChecker -->

Sealed-interface type switches are separate. Their consumers still list each
variant with no `default`, under the new-code rule above. `exhaustive` checks
enum value switches, not sealed-interface type-switch completeness; that
remains a reader check.
<!-- source: vendor/github.com/nishanths/exhaustive/switch.go -- switchChecker -->

#### Validated at construction

Validation at creation rejects invalid input before it becomes a domain value.
Encapsulation then protects that value: private fields prevent a caller in
another package from replacing data that the constructor checked. A raw
configuration or wire DTO can still contain invalid input until a parser or
constructor converts it to the validated type. An unconstrained data struct has
no such invariant to protect and needs no constructor ceremony.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- raw input and validated domain values -->

`NewX` and `ParseX` are ordinary function names. Go does not call them when a
caller declares a variable or uses a composite literal. The built-in `new(T)`
returns a pointer to a zero-initialized `T`; it performs no domain validation.
Even an exported struct with only private fields admits `var x T`, `T{}` and
`new(T)`. A named scalar such as `type Email string` also admits the conversion
`Email("junk")`. These are language rules, described under
[allocation](https://go.dev/ref/spec#Allocation),
[composite literals](https://go.dev/ref/spec#Composite_literals) and
[conversions](https://go.dev/ref/spec#Conversions).
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- constructor names and zero contract -->

A validated type can make its zero a legitimate value, as in this illustrative
API. Its parser rejects the values 1 and 2; a successful result or a zero value
then meets the same domain constraint.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- valid zero semantics -->

```go
type HoldTime struct{ seconds uint16 } // zero, or 3 and above

func ParseHoldTime(seconds uint16) (HoldTime, error)
```

When zero cannot be valid, the type's contract describes it as uninitialized
and gives operations a safe way to reject it. Nil pointers and typed-nil
interfaces need the same decision. This prevents the existence of a constructor
from being mistaken for proof that every value passed to an API used it. Existing
valid zeros and sentinel meanings remain unchanged by this guidance.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- zero and nil contract -->

Private fields alone do not protect referenced data. Go copies a slice, map or
pointer without copying the data it references, so a caller can still change
that data after validation. The ownership contract covers constructor inputs
and accessor results: immutable values, controlled ownership transfer, or a copy
where sharing cannot be made safe. Setters check a proposed change before
publishing it. Decoders can populate a raw value and publish the validated value
only after success, so a failed decode cannot corrupt an existing valid object.
The [representation rules](https://go.dev/ref/spec#Representation_of_values)
describe how values share underlying data.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- mutation decoder and ownership contract -->

Repeated internal checks become unnecessary only after the callers and all
producers establish that contract, including same-package writes and aliases.
Input types that already prove a constructor's preconditions need no repeated
validation either. This proof does not replace the paired wire-boundary checks
in "Assertions, in a language that has none", or the error handling for raw
external input.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- producer proof and boundary checks -->

#### One type per lifecycle state

**Ze wants this pattern, and it is the first design to reach for.** Rust attaches
an operation to the type a value holds, so a function that needs initialized
data cannot be called until the value is initialized; a tagged enum then
toggles between the states. Go has no tagged enum, but it has the half that
matters: an operation that is a method of the validated type, or that takes the
validated type as its parameter, cannot be called with anything else. When Ze
controls the transition, write that type instead of a state field and an `if`
at the top of each operation. A type turns a forgotten check into a compile
error; a runtime check finds it only on the path a test happens to run.

In Go, write a progression of states (loaded, then resolved; parsed, then
validated) as distinct named types, each holding only its own methods. Keep the
sealed interface of "One state, one variant" for a value that really is one of
several variants at runtime, such as an outcome, because a Go type switch is a
weaker tagged union than Rust's.

In this illustrative API, only a successful `Resolve` yields a `Schema`, and the
command tree accepts only a `Schema`, so no caller can build a tree from a
module set that failed its checks:

```go
type Loader struct{ /* modules added, not yet checked */ }
type Schema struct{ /* every module resolved and checked */ }

func (l *Loader) Resolve() (*Schema, error)
func BuildCommandTree(s *Schema) *command.Node
```
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- state-specific operations -->

Distinct state types restrict which operations a caller can name. For example,
a `ParsedConfig` has validation operations, while an apply API accepts only a
`ValidatedConfig`. This separates raw data from data that passed validation.
Go's method sets and parameter types enforce that distinction; the construction
contract above determines whether the accepted value is valid.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- state-specific operations -->

A transition that returns a new type does not consume the old value. Go leaves
old values and aliases usable, and resource handles can still refer to the same
mutable resource. An immutable builder can leave its old snapshot valid. A
resource with a one-way lifecycle instead needs controlled ownership and, where
aliases remain possible, shared runtime state that rejects stale operations.
The [Go value representation rules](https://go.dev/ref/spec#Representation_of_values)
explain the aliasing that a type transition cannot revoke.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- transitions and aliases -->

Distinct named types also give each state its own method set. A generic type
such as `Conn[State]` cannot specialize receiver methods for only the `Open`
instantiation: receiver type parameters are declarations, as specified under
[method declarations](https://go.dev/ref/spec#Method_declarations).
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- state-specific operations -->

These API types describe transitions Ze controls, such as setup and the move
from parsed to validated configuration. A peer-driven protocol state machine
still decides at runtime whether an event is permitted. A valid message can
arrive in the wrong state, so construction validation cannot replace that
decision or the safe error handling described under "Assertions, in a language
that has none".
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- peer-driven runtime validation -->

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
