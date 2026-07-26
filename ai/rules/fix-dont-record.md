# Fix, Don't Record. Say It Short.

**When:** a test fails, a gate goes red, or you are about to write a problem down -- a `plan/known-failures/` shard, a deferral row, a commit body, a report, a learned summary
**Severity:** blocking
**Related:** no-parking, anti-rationalization, flaky-under-load, testing, ci-sleep-justification

## Directives

Two failures, one rule: writing a problem down instead of fixing it, and writing
at length instead of writing what matters. Both convert work into text.

## Load is never an explanation. It is the bug.

A test that passes on a quiet host and fails on a busy one is a BROKEN TEST. Load
did not break it. Load revealed that it asserts on elapsed time instead of on
state. **Fix the test so load cannot reach it.** Owner directive, 2026-07-26.

These are banned as a conclusion -- in a shard, a commit body, a report, or a
reply to the user:

| Banned | What it actually says |
|--------|-----------------------|
| "fails under load / on a loaded host" | the test waits a fixed time instead of waiting for the condition |
| "load average was ~11 vs ~2 earlier" | you measured the host instead of reading the test |
| "passes in isolation" | it depends on scheduling luck. That IS the defect, stated |
| "the failing set rotates, so it is not deterministic" | several tests share one timing assumption. Find it |
| "the contended-run detector did not trip" | that detector labels runs. It never absolves a test |
| "not reproducible, logged as non-deterministic" | you do not need a repro to fix a timing assumption. Read the test |

**There is no non-deterministic hatch for a load-sensitive test.** The
`no-parking.md` exception covers a failure whose MECHANISM you could not
determine. It does not cover one you already explained by naming the host's load:
that explanation is the diagnosis, and the fix follows from it directly.

## Making a test load-proof

Find what the test waits ON, and make it wait for the thing instead of for a
duration.

| Symptom | Fix |
|---------|-----|
| `time.Sleep` / `time.sleep(` then assert | poll the condition: `wait_until`, `dispatch_until`, `wait_for_event` (`test/scripts/ze_api.py`) |
| fixed deadline for startup, teardown or reconnect | wait on the readiness signal the daemon emits. If none exists, ADD one -- a missing signal is a product gap, not a test problem |
| "at most N events in a window" | count between two state transitions, not between two clock reads |
| assert immediately after a command returns | wait for the effect to be observable, then assert |
| the test genuinely needs a kernel-global surface to itself | `option=exclusive:group=<name>` (`internal/test/runner/record.go`), not a longer timeout |
| a timeout that is "generous enough" | generous is a synonym for unknown. Bound it by a condition |

Raising a timeout is not a fix. It moves the load level at which the test lies.

Replacing a sleep with a real wait routinely exposes a genuine data race in the
product. That is a feature of the technique, not a reason to avoid it.

`ai/rules/flaky-under-load.md` and `scripts/dev/stress-repro.py` exist to
DIAGNOSE such a failure, never to decide whether it deserves recording. A
deliberate timer that IS the behaviour under test stays, and says so in its
comment (`ai/rules/ci-sleep-justification.md`).

## Recording

`plan/known-failures/` is not a destination for a failure. It is the running log
of an investigation you are still driving, and it is empty the moment the
investigation ends.

Before writing any record of a problem, answer: **what did I fix?** If the answer
is "nothing yet", the record is not the deliverable and does not substitute for
one.

| Do not write | Do |
|--------------|-----|
| a shard for anything that reproduces, or that load explains | fix it |
| a shard as the outcome of a session | fix it, and delete the shard if one existed |
| "pre-existing" anywhere as a reason | fix it. It says when it started, not whose it is |
| the same failure in a shard, a commit body, a report and a summary | pick one place |

## Length is not evidence

A record earns its length from what a future reader must DO, never from what you
went through. Investigations are not narrated: the wrong hypotheses, the order
you tried things in, and how long it took are yours, not the reader's.

| Artifact | Contains | Budget |
|----------|----------|--------|
| Commit subject | what changed, imperative | one line |
| Commit body | the defect, its cause with `file:line`, what the fix does | under 15 lines. No investigation narrative, no "in sequence it was X, then Y" |
| Known-failure shard | the failing output, the repro command, the next step | under 20 lines |
| Report to the user | what is fixed, what is not, what proves it | shortest form that is complete |
| Learned summary | what a future reader needs that the code cannot tell them | per `ai/rules/planning.md` |

Banned in all of them: recounting dead ends, restating the same fact in
successive paragraphs, explaining why the previous attempt was wrong, and any
sentence about the difficulty of the work.

State the correction and move on. If a claim needs evidence, cite `file:line`
rather than retelling how you obtained it.

## Rationale

Both halves were paid for. A `plan/known-failures/` shard was written on
2026-07-26 for three functional tests that failed only on a loaded host; the
shard argued at length that the rotating failure set proved non-determinism,
when a rotating set across teardown-shaped tests is the signature of one shared
timing assumption -- a diagnosis, sitting unread inside its own record. The same
session produced commit bodies that narrated three superseded hypotheses about a
tc query before reaching the fix, so a reader had to discard two thirds of the
message to find what the commit did.
