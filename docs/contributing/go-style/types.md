# Ze Go Style: types that cannot lie

The detail behind "Types that cannot lie" and its four subsections in
[the Go style core](../ze-go-style.md), which states each rule. The paragraphs
below follow the order of the core's paragraphs.

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

The sections below explain the Go mechanisms and their limits.
<!-- source: ai/rules/points/go-standards/directives/preserve-validity-from-construction-through-use.md -- construction and representation scope -->

## One state, one variant

With `done bool`, `result *T` and `reason string`, a caller can build
"done with a reason" or "not done with a result". This example
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

## Every enum switch has a coverage policy

The rationale MUST be on its own preceding comment line. A trailing comment is
parsed as directive arguments and rejected:

```go
// Only events with a defined metric contribute to this projection.
//exhaustive:ignore
switch event {
```
<!-- source: vendor/github.com/nishanths/exhaustive/passes/exhaustive/exhaustive.go -- parseDirectives -->

Preserve existing named behavior, including unsupported-value errors, sentinel
outcomes and valid zeros. This coverage migration does not impose invalid-zero
semantics on existing types: preserve an existing valid unnamed zero path
explicitly too. Inspect behavior after the switch, not only its arms. Do not
add protocol support or change masked-code recognition to make a case explicit.

Completeness is by distinct constant value: one named case covers same-valued
aliases. Within the enum's package, private members count; across package
boundaries, only exported members are required. Use named constants, not
literals, to satisfy named-member coverage.
<!-- source: vendor/github.com/nishanths/exhaustive/passes/exhaustive/check.go -- checkExprSwitch, needValues -->

The `exhaustive` linter checks all unignored enum value switches, including
aliases, with `.golangci.yml` setting `default-signifies-exhaustive: false`.
A `default` never substitutes for a missing named member. Reviewers MUST still
apply C/O/P to old and new switches, including those whose named cases are
already complete: the linter does not prove input closure, judge subset reasons
or check default and post-switch behavior.
<!-- source: .golangci.yml -- exhaustive default-signifies-exhaustive -->
<!-- source: vendor/github.com/nishanths/exhaustive/passes/exhaustive/check.go -- checkExprSwitch -->

`exhaustive` checks
enum value switches, not sealed-interface type-switch completeness; that
remains a reader check.
<!-- source: vendor/github.com/nishanths/exhaustive/passes/exhaustive/check.go -- checkExprSwitch, checkTypeSwitch -->

## Validated at construction

A raw configuration or wire DTO can still contain invalid input until a parser or
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

## One type per lifecycle state

Rust attaches an operation to the type a value holds, so a function that needs
initialized data cannot be called until the value is initialized; a tagged enum
then toggles between the states. A type turns a forgotten check into a compile
error; a runtime check finds it only on the path a test happens to run.

Keep the sealed interface of "One state, one variant" for a value that really is
one of several variants at runtime, such as an outcome, because a Go type switch
is a weaker tagged union than Rust's.

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
