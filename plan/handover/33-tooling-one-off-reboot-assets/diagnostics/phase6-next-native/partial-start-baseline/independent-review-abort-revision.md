# Controlled startup abort revision: independent review

**CLEAN source review for MAIN to execute only
`TestNativeCatalogPartialHandoffCleanup`.** This is not approval of either
refused native observer root or evidence that the old cleanup defect is fixed.
No formatter, compiler, test, runtime process, or approval command ran here.

The reviewed `startup-abort-revision.patch` exactly matches the actual delta
from `refused-v2/catalog_cancel_test.go.txt`. Canonical source still matches
the preserved before snapshot; the ordinary native observer still matches
`refused-v1`. The actual old `nativeCatalogProbe.finish` and protected oracle
are unchanged.

## Earlier refusal addressed

The controller's absolute startup deadline now reaches both surrogate and
helper. A delayed helper cannot begin a fresh relative startup interval.
The sole safety reader starts before preparation. EOF, expiry, or an invalid
grant records abort under the same mutex used by `startChild` to check state,
perform Start, and install its actual child owner. An abort therefore either
prevents that fork or obtains the installed owner and joins its existing Wait.
No owner can disappear between the fork and registration.

The controlled hashing path checks that state between bounded 32 KiB reads
and again at completion. No child exists during initial hashing; expired or
aborted preparation cannot subsequently pass the guarded Start. These checks
apply only to the controlled seam, leaving ordinary native hashing unchanged.

Neither supervisor nor surrogate discards its direct-child Wait bridge after
a shorter give-up timer. Both retain it through actual settlement within the
unchanged helper/surrogate/supervisor process bounds of 40/45/50 seconds.
The helper's close defer closes the socket, joins its lifecycle reader, and
retains any child Wait before return. A pending Wait is not converted to a
successful cleanup claim.

## Reader and oracle boundary

`readControl` is the only reader of the safety connection. `publishChild`
receives its one result through a buffered one-message channel; it no longer
competes for socket reads. The reader drops the mutex before child cleanup or
Wait, so the helper's deferred close/join cannot deadlock on that mutex.
Both cleanup paths observe the same `nativeCatalogChild` Wait worker; neither
adds a second `Cmd.Wait` or releases its handle concurrently.

A timely grant is accepted only after an actual child owner exists. It ends
the startup-abort reader. That reader does not become a post-grant watchdog
which could clean the survivor before the old-finish oracle. If startup aborts
instead, the helper never publishes the ordinary gate packet, so MAIN must
classify the resulting setup failure separately from the intended semantic RED.

The original protected-path safety transfer, post-oracle exact-handle killing,
subreaper adoption, and sequential adopted reaping remain unchanged. The prior
review's verified pidfs/SCM_RIGHTS and amd64/arm64 prctl ABI boundary still
applies. The five-second cancellation, five-minute product/readiness, and
minute normal-return bounds were not increased.

## Execution scope

No original assertion or RFC-tagged unit is weakened; no approval row is owed.
MAIN may format this selected baseline source and run the exact one-minute
controlled-root command in `HANDOFF.md`, after its current CPU job ends.
Runtime must establish the intended old survivor/socket/join-ack failure,
the live foreign sentinel, and completed independent safety reaping. Compile,
startup, hashing, transfer, or pidfs failures do not establish that RED.

The full native P1/P2 repairs and native execution remain refused and owed.
