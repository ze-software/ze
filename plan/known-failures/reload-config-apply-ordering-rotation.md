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

## The next step, and it is a lead rather than a diagnosis

The runner reported that every expected message had arrived before the timeout.
Peer-action completion is therefore an investigation lead. The message alone
does not identify the blocked wait or exclude host load.

The same spec fixed one instance of exactly that shape in the same suite. A peer
block that queues an action which does not answer completion leaves the peer
never reporting done: `action=rewrite` did not answer completion, so the daemon's
shutdown NOTIFICATION was matched against an empty expectation list, and
`action=sighup` and `action=sigterm` already did answer it
(`internal/test/peer`, and the `action` table in
`docs/architecture/testing/ci-format.md`).

So the next step is to read which action arms `test/reload/config-apply-ordering-rotation.ci`
queues in its peer blocks, and to check each one against the arms that answer
completion. If one does not, this is the same defect and it is a fix rather than
a flake. That reading has not been done, which is why this is a shard and not a
journal row.
