# Ze Go Style: a zero value is never an answer

The detail behind "A zero value is never an answer" in
[the Go style core](../ze-go-style.md), which states the rule. The paragraphs
below follow the order of the core's paragraphs.

A zero value is what every field holds before anybody writes to it, so it
cannot be told apart from a field nobody set. The dangerous form is the zero that behaves correctly: a missing branch and a
deliberate no-op produce the same silence, and the next change deletes the
behavior without touching a line that names it.

The sharpest case is a zero that guards without being a guard. A test that asks
*which value do I use* can also answer *is this allowed yet*, because the value
stays zero until the permission exists. `verifyRemoteAuth` gated an EAP peer's
AUTH payload on `sa.EAPMSK != [64]byte{}`, a test of which key to sign with. An
EAP method that derives no key makes the MSK legitimately zero, and the
accidental guard then becomes an authentication bypass. The fix asks the
exchange whether it succeeded, on purpose. `PeerResult` gained `Discarded` and
`Notified` for the same reason.

## The shapes the check refuses

`./le arch refuse-by-default check`, `./le go style check` and `./le commit create`
refuse five shapes on newly added or changed lines:

| Shape | Refused | Write instead |
|-------|---------|---------------|
| Unmatched switch value | A `default:` arm, or the return after a switch whose every case returns, answering a nil error | An error naming the value |
| Miss answered as a hit | A branch on `!ok`, on a nil registered slot or on a read error's `err != nil` answering `true`, a lone zero or a nil error; `return err == nil` from a read | An error naming what could not be found or read |
| ADD-PATH identifier compared with zero | `pathID != 0` and every comparison of a path identifier with `0` | The negotiated capability the route carries (`nlri.HasPathID`, `Route.AddPath`, `RawRoute.HasPathID`): zero is a valid identifier (RFC 7911 Section 2) |
| State from a literal | `StatusHealthy` in a function that measures nothing; a check that is one literal `return nil` or `return true`; a literal boolean in a `State`, `Status` or `Health` value, including a direct member assignment | Read the state; where it cannot be read, Unknown with the reason |
| Error dropped below WARN | A branch on `err != nil` (or the `else` of `err == nil`) whose only record is a `Debug` or `Info` log call, and which then falls through, continues, breaks or returns no error | Return the error, count it, or log it at WARN: Ze logs at WARN by default, so a debug line is no record (the MOBIKE route-source EPERM that stopped UPDATE_SA_ADDRESSES) |

A branch that records what it met before it answers (a field or element write,
a counter step, the error handed to an outer variable) or whose answer carries
the error is not refused. A direct `plugin.OperatorErrorResponse(...)` beside
a nil Go error carries the refusal in its response; the check resolves the
canonical package's import path and alias, including local shadowing. A parse's
own "no" (`strconv.Atoi`, `netip.ParseAddr`) is also accepted: the miss then
reaches a reader. A `false` beside the nil
error hands the miss on, and a bool function literal (an `ast.Inspect`
visitor, a yield, a poll predicate) steers its caller's loop, so neither is
refused either. A log line is no record: a WARN beside a default is still the
default. A branch that returns or records the error, logs it at WARN, or
panics hands it on, so the dropped-error shape does not refuse it, and a
branch the miss shape already refuses is not listed twice.

State member assignments use the same type-name suffixes as composite literals:
`State`, `Status` and `Health`. The check follows file-local receiver,
parameter, result and variable declarations, direct construction, builtin
`new`, pointer operations and copies of known values. Package initializer
dependencies are resolved regardless of declaration order. Each block keeps
its own bindings, so a config variable that shadows a state variable stays a
config variable. The check does not load imported packages or infer an unknown call
result or a nested member's type. A measured value assigned to a state member
is accepted.
<!-- source: internal/le/arch/refusebydefault/state_assignments.go -- stateAssignments, resolvePackageStates, Visit, assignment, stateValue, stateNamedType -->

## Naming the guard

Where the zero IS the answer another branch relies on, name that guard on the
shape's line or the line above it:

```go
// refuse-by-default: guards Stop -- a nil handler is the stopped state Stop leaves
if fn == nil {
	return nil
}
```

The annotation opens with `guards` and a Go name the file holds or imports,
or that another file of its package declares, then the reason. The guard is
the dependent, the caller or branch that relies on the zero, and never the
function the shape sits in: that name is always visible and records nothing,
so the check refuses it. One that names no guard, or a name the file cannot
see, is refused on its own line. The check lists every accepted shape with its
guard, so a reviewer reads each one. `./le arch refuse-by-default report` lists every
instance in the tree, refused and annotated.
<!-- source: internal/le/arch/refusebydefault/refusebydefault.go -- ScanFile, missOf, recordsMiss, answersPlausibly -->
<!-- source: internal/le/arch/refusebydefault/dropped.go -- droppedError, belowWarn, drops -->
<!-- source: internal/le/arch/refusebydefault/annotation.go -- parseGuard, namesEnclosing, addPackageNames -->
<!-- source: internal/le/arch/refusebydefault/report.go -- Check, Report, judgeFile -->

## A probe that cannot look

A capability probe that the sandbox refuses says nothing about the kernel
feature, and a directory the current user cannot read says nothing about the
file in it: read as absent, each one selects the wrong test bounds or rebuilds
what was already there.
