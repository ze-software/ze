# Native catalog observer: independent actual-delta review

Verdict: **REFUSE native execution** pending the two fixture repairs below.
Source review only. No formatter, compiler, test, native process, or approval
command ran. The reviewer authored none of the observer.

## P1 — Partial startup can lose the descendant's cleanup owner

`TestNativeCatalogCompilerTool` starts the TERM-ignoring child, installs
`defer child.finish(t)`, and then connects and transfers its gate receipt.
The child's own ready packet is a separate transfer. `readPeer` can therefore
add the gate to `p.handles` before the descendant's receipt has been accepted
and bound. A second-packet or later readiness failure enters fixture cleanup.

`nativeCatalogProbe.finish` first kills every recorded handle, including that
gate. It closes the control connections only afterward. SIGKILL bypasses the
gate's deferred `child.finish`, while the untransferred descendant is absent
from the controller's ownership list. That child can continue until its own
five-minute socket deadline. The inverse partial-start case can leave the Go
builder/gate without a transferred handle when the controller kills only the
direct native entry.

This is a fixture lifetime defect, not evidence about product cancellation.
Pidfs inode equality correctly prevents PID-reuse signaling; it does not join
a successfully started child whose ownership has not yet been transferred.

The source repair must keep an owner through startup and failed handoffs.
Controller closure/abort must be observed throughout helper startup. The
helper must close and join its direct child before the controller can force
that helper's termination while the child remains untransferred. Transfer or
explicitly settle child ownership before publishing final readiness. Merely
moving connection closure earlier without a bounded acknowledgement/join is
insufficient: it would still race the subsequent gate kill. Preserve all
product assertions before this fixture fallback and never introduce a numeric
PID/group sweep.

MAIN needs a controlled partial-start discriminator before the actual native
roots: interrupt the handoff after a child has started but before its receipt
is accepted, then require bounded child/owner join, socket closure, and the
unaffected foreign sentinel. The current draft must fail that probe.

## P2 — The foreign sentinel can expire during permitted native work

`TestNativeCatalogPeer` gives every role a five-minute connection deadline.
The foreign role starts before flag resolution and native launch.
`awaitCompiler` subsequently permits five minutes of readiness, and the normal
root permits another minute after releasing the compiler. A valid readiness
near its bound can therefore encounter a foreign sentinel that has already
exited. Its failure would be attributed to the product although the native
operation stayed within its declared bounds.

Tie this fixture role's lifetime to the controller's bounded case lifecycle
and abort/EOF protocol. Do not change the five-second cancellation oracle,
five-minute build/readiness bounds, or minute normal-return bound to repair
the fixture.

## Preserved proof and reviewed producer

The canonical file still matches `before/catalog_cancel_test.go.txt`.
`HEAD` has no such tracked path; the available original is the preserved
uncommitted carrier. The actual diff changes imports and appends new code;
all its old functions and assertions remain unchanged. No RFC-tagged unit or
existing expectation is changed, so no weakening/RFC approval row is owed.

| New unit | Verdict | Reason |
|----------|---------|--------|
| `TestNativeCatalogBuildCancellation` | REFUSE execution | Its useful stronger native oracles do not make the partial-start cleanup safe. |
| `TestNativeCatalogBuildCompletes` | REFUSE execution | It shares that cleanup defect and can outlive the foreign sentinel's earlier deadline. |

The direct native entry has one Wait owner and no fixture group cleanup before
the product assertions. `finishedBy` checks the recorded return timestamp; raw
signal death cannot become the required ordinary cancellation exit. The test
retains owned-process state, socket EOF, the unreleased gate, and positive
foreign-sentinel checks. Its intended source path is registered `runDrift` ->
`Drift` -> `BuildDaemon` -> `runCatalogProcess`; these producers root the build
and command in the native signal context and preserve failure verdicts.

The scoped compiler selector, rejection of inherited conflicting instrumentation,
and target-only fresh `-trimpath` input preserve the declared diagnostic boundary.
The normal oracle consumes native issues JSON and rejects the live-catalog
generation/parse error. It claims the build phase only; it supplies no separate
catalog-exec marker or proof for the other native routes.

After repairs, review their actual delta and the controlled handoff probe.
Only MAIN may format and execute. Linux pidfs support, actual compiler argv,
named-binary identity, and native output still require runtime confirmation.
