### `./le functional reload` config-apply-ordering-rotation -- one timeout on 2026-09-08, NOT reproduced since

Observed ONCE, in five `./le functional reload` runs over the phase-1 tree of
`spec-config-apply-ordering-covers-every-root`. The test took 30.0s against a
2.2s average for that run and the runner reported:

> all expected messages received but test still timed out

The shard exists because the spec that found it closed on 2026-09-12 and is no
longer in the tree. `ai/rules/completion.md` allows a failure to be RECORDED only
where it was actively tried and could not be reproduced, and only with the
attempt and the next step on the record. Both are below.

## The attempt

| Setting | Value |
|---------|-------|
| Tool | `./le functional reload`, the action that owns the suite |
| Runs started | 6, on 2026-09-08 |
| Runs that reached a test | 2. The other four never built, because another session's uncommitted work left the checkout unable to build the isolated binaries |
| Later run | 1, after the phase-4 comment edit |
| Result | **not reproduced.** The test passed at 7.2s, 4.0s and 4.0s |
| Capture | `tmp/session/2026-09-08-cbc36cee-41ac-4afd-8b71-1bae841964d9/scratch/rotation-repro-{1..6}.log` (scratch, not durable) |
| Today | It passed again at 2.5s in the closure run, `.../scratch/closure-functional-reload.log`, in a suite that went 44 of 44 |

Three completed runs are too few to call it gone. All three are well under the
30s deadline, and none of them is near it.

## What was NOT changed

The spec's phase 4 edited this file's COMMENT header only, to stop it claiming
address operations it does not emit. No behavior of the test moved, so the
timeout is neither removed nor hidden by that work. The daemon-side code the test
exercises did change across the spec, so a fourth run is owed on the current
tree rather than on the phase-1 one.

## The next step, and it is a lead rather than a diagnosis

The message is the useful part: the runner had every expectation it was waiting
for and still timed out. That is not an elapsed-time assertion and it is not a
busy host. It is the shape of a test whose COMPLETION signal is separate from its
expectations.

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
