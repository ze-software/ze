### `./le test functional reload` config-apply-ordering-rotation: one recorded timeout, later attempts passed

Observed ONCE, in five `./le test functional reload` runs over the phase-1 tree of
`spec-config-apply-ordering-covers-every-root`. The test took 30.0s against a
2.2s average for that run and the runner reported:

> all expected messages received but test still timed out

This September timeout is separate from the July 26 entry under the same test
name in `RESOLVED.md`. That entry records commits `834f92629` and `62dcfcacd`
repairing a BGP Identifier claim race that caused an OPEN rejection. The
September observation reached all expected messages, and this shard neither
reopens the July repair nor attributes the later timeout to it.

The shard exists because the spec that found it closed on 2026-09-12 and is no
longer in the tree. `ai/rules/completion.md` allows a failure to be RECORDED only
where it was actively tried and could not be reproduced, and only with the
attempt and the next step on the record. Both are below.

## The attempt

| Setting | Value |
|---------|-------|
| Tool | `./le test functional reload`, the action that owns the suite |
| Runs started | 6, on 2026-09-08 |
| Runs that reached a test | 2. The other four never built, because another session's uncommitted work left the checkout unable to build the isolated binaries |
| Later run | 1, after the phase-4 comment edit |
| Result | **not reproduced.** The test passed at 7.2s, 4.0s and 4.0s |
| Capture | `tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/rotation-repro-{1..6}.log` (scratch, not durable) |
| Closure run, 2026-09-12 | It passed again at 2.5s in `.../scratch/closure-functional-reload.log`, in a suite that went 44 of 44 |

The record contains four completed passing attempts, including the closure run.
All were below the 30s deadline. They do not establish that the timeout was fixed.

## What was NOT changed

The spec's phase 4 edited the COMMENT header of
`test/reload/config-apply-ordering-rotation.ci` to stop it claiming address
operations it does not emit. No behavior of the test moved in that edit.
Daemon-side code did change across the spec, and the closure pass above records
one later-tree result. The timeout's mechanism remains unknown.

## The peer-action lead is ruled out (2026-11-14)

This shard used to end on one lead: a peer block that queues an action which
does not answer completion never reports done, and the reading that would test
it had not been done. It has now been done at the producer, and the lead does
not apply to this test.

| Checked | Result |
|---------|--------|
| Which actions the test queues | `action=rewrite` then `action=sighup` on conn 3, `action=sigterm` on conn 6 |
| Do they answer completion today | Yes, all three: each arm of `runMessageLoop` (`internal/test/peer/peer.go`) ends on `p.checker.Completed()` |
| When rewrite's completion landed | `9b07614064`, 2026-09-11, three days after the timeout |
| Could that fix explain this timeout | No. It matters only when the rewrite is the last queued item. Here the sighup follows it, so the peer was never complete after the rewrite on any version, and the sighup and sigterm arms already answered |

## What the timeout message proved: only that the first batch finished

The message quoted at the top carried almost no information. The runner of
that day compared the peer's count of received frames with the number of
`(conn, seq)` keys the test declares (`connSeqToIndex`,
`internal/test/runner/record_parse.go`). This test has seven keys, and seven
frames are printed once the first batch of three connections completes. So
"all expected messages received" meant "batch 1 is done", never "the exchange
is done". `7340942afc` (2026-10-21) deleted the count claim for that reason: a
timeout now prints the peer's frames.

What the 30.0s does establish: the test peer was still alive at the deadline.
The runner waits on the peer; it stops `ze` afterwards with a SIGTERM and a
kill after 10s (`ownedProcess.stop`, `TeardownGraceTimeout`). A `ze` that hangs
on SIGTERM cannot produce a 30s timeout alone.

## Where the peer can still be waiting after batch 1

No wait in this test is on elapsed time: `pauseForSignal` returns at once under
`conn_map`, and every other wait is on socket state. So this is not the
host-load diagnosis `README.md` refuses. The peer waits, in order, on:

1. `waitBatchClosed` (`internal/test/peer/peer_connmap.go`): the daemon closes
   connections 1 to 3 after the SIGHUP. Bounded only by the test's context.
2. `acceptConnMapBatch`: three new connections, each with an OPEN and the
   daemon's KEEPALIVE.
3. Batch 2: an UPDATE and an End-of-RIB on each of connections 4, 5 and 6.
   They are ordered by the router-id of the daemon's OPEN, so conn 6 is the
   peer with router-id 9.10.11.12.

One landed fix touches the reads in steps 1 and 3. It is a candidate, not an
explanation. Before `fe781e1c7d` (2026-10-22) the peer's `ReadMessage`
discarded a partial header or body when its 100ms read deadline fired, and the
retry then parsed mid-frame bytes as a header. A frame the daemon wrote in two
parts more than 100ms apart could stall a read for good. Nothing shows the
September run hit it: the captures under `tmp/session/2026-09-08-*` are gone.

## What the next occurrence must keep

The mechanism is unknown and nothing is left to read, so this stays a shard.
One capture would diagnose it:

| Capture | Why |
|---------|-----|
| The peer's whole stdout | Its progress lines name the wait it died in: `waiting for the daemon to close all 3 connections`, `conn=N closed by the daemon`, `conn=N still open when the test ended`, `waiting for next mapped connection batch`, `new connection from`, `sending SIGHUP`, `sending SIGTERM` |
| The daemon's stderr with reload and peer logging at debug | Shows whether the two changed peers were closed and redialled |
| The runner's verbose output, saved | The frames the timeout report prints since `7340942afc` |

One tooling gap makes that capture weaker than it could be. At the deadline the
runner kills the peer's process group with SIGKILL (`ownedProcess.cancel`,
`internal/test/runner/process_owner.go`), so the peer never prints the line
that names the connection and the expectation it still owed. A SIGTERM, a
short grace and then the kill would let it print that. This is not done.

Reading the capture: a peer that stops in `waitBatchClosed` or in a batch 2
read makes this a daemon defect in the reload path, to be fixed. If the test
never times out again on a tree after 2026-10-22, the partial-frame read is
the best reading and still not a proof.
