# Finishing Work

**When:** before claiming any work done, complete, or ready to commit, and whenever a defect, a red test, or a missing behavior blocks that claim
**Severity:** blocking
**Related:** planning, testing, interop-and-goal-validation, writing, evidence, rule-precedence

## Directives

**BLOCKING. ABSOLUTE PROHIBITION. Same level as git safety.**

**You may not claim work is done, complete, ready to commit, or ready for review while any in-scope acceptance criterion remains unimplemented.**

**When a defect blocks a goal the current work exists to achieve, you FIX the defect.** You do not park it, move it to `tmp/`, file it as a deferral, or offer to drop the deliverable.

**Interoperability and correctness are never "optional" and never a scope-reduction candidate.** A network daemon that another implementation rejects has failed at its only job.

**Recording a problem is not addressing it. Fix the root cause, always.** Writing a failure down (in `plan/known-failures/`, a spec, a learned summary, a deferral row, or a report to the user) changes nothing about the product. A record is a step *toward* a fix and never a substitute for one. When you find a red test, a hang, a wrong result, or a silent misbehavior, the deliverable is the FIX.

**Before changing code to make a symptom go away (failing test, rejected input, error, red gate, broken demo), write the Diagnosis first.** Editing to silence the symptom before the root cause is named is the defect, not the fix.

**If a user could experience a problem while trying to achieve a goal, implement the missing behavior at the source.** Do not bypass, mask, special-case, weaken a check, adjust a fixture, or route around the problem just to pass a test, demo, gate, or narrow scenario.

**Every exported function, type, or constant created by a spec implementation MUST have at least one caller in the running daemon.** "Library code with tests" is not done. "Tested but not wired" is not done.

**Every new feature MUST be proven to work integrated, not just in isolation.** Every feature needs at least one end-to-end test from its intended usage point.

**Before marking any spec done, complete a line-by-line audit comparing spec to implementation.**

**Never use phrases like "would you like me to", "want me to", "shall I", or "I can" before completing work.** Finish the task first, then report what was done.

**When you catch yourself explaining why a test, a gate, or a completion standard does not apply this time, the answer is always "no."**

## The Problem

Claude claims "done" or "ready to commit" while in-scope work remains unimplemented,
buried in a deferral list or silently dropped. This is the single most damaging
pattern in this project: the user trusts the completion claim, only to discover
later that 1/3 or 2/3 of the feature was never built.

## The Rule

The claim-done ban above has no synonyms. "Deferred" does not mean "done."
"Tracked in a plan/deferrals/ shard" does not mean "done." "Will be handled in
a follow-up" does not mean "done." If the spec lists it and you did not build
it, the work is not done.

## What "Done" Requires

Every single one of these must be true before you say "done":

| # | Requirement |
|---|-------------|
| 1 | Every acceptance criterion in the spec has working code |
| 2 | Every acceptance criterion has a unit test (`_test.go`) that exercises its logic |
| 3 | Every user-facing behavior has a functional test (`.ci`/`.et`) per `ai/rules/testing.md` |
| 4 | Protocol features have interop tests per `ai/rules/interop-and-goal-validation.md` |
| 5 | Goal Validation table filled with concrete evidence per goal |
| 6 | The code compiles and `make ze-verify` passes |
| 7 | No TODO, FIXME, or stub remains in the new code |
| 8 | No item was silently dropped from scope |
| 9 | Every function is reachable from a user entry point (wired, not just library) |

If ANY of these is false, you are not done. Say what remains and keep working.

## Banned Phrases When Work Remains

| Phrase | Why it is banned |
|--------|-----------------|
| "Ready to commit" | Implies nothing is left. If something is left, do not say this. |
| "Implementation complete" | Same. Not complete if items remain. |
| "Done, with the following deferred" | Contradicts itself. Done means nothing deferred. |
| "All core functionality implemented" | Redefining scope to exclude what you skipped. |
| "The remaining items are minor" | You do not decide what is minor. Implement them. |
| "Tests pass, ready for review" | Tests passing is step 10 of 12. Not the finish line. |
| "Should work for the common case" | Implement the uncommon cases too. |

## What To Do Instead

If you genuinely cannot complete an item (missing infrastructure, blocked by
another component, would require user decision):

1. **Say explicitly:** "I cannot complete X because Y. The work is NOT done."
2. **Keep the spec open.** Do not close it. Do not write a learned summary.
3. **List what works and what does not** in plain terms, no hedging.
4. **Ask the user** what they want to do about the incomplete items.

Never bury incomplete work in a deferral table and then present the task as finished.
The user reads "ready to commit" as "everything works." Honor that reading.

## Scope Reduction Requires Explicit User Approval

**ABSOLUTE PROHIBITION. No self-authorized scope reduction. No exceptions.**

If during implementation you discover that an acceptance criterion or deliverable
is harder than expected:

1. **Stop implementing.**
2. Tell the user: "AC #N is harder than expected because X. Do you want me to
   continue with it, or drop it from this spec?"
3. **Wait for an answer.** Do not proceed without it.
4. Only if the user explicitly says to drop it may you proceed without it.

You may NOT unilaterally decide an AC is "out of scope," "a follow-up," or
"better handled separately." That is scope reduction dressed as planning.

### Documentation Is Not a Substitute for Implementation

Writing "documented as known limitation" or "deferred to integration tests"
does NOT resolve a missing deliverable. It is scope reduction without permission.
The same applies to:

- "Requires infrastructure not available" -- attempt it first; ask only if genuinely blocked
- "Noted in the spec as a deviation" -- a deviation entry is tracking, not resolution
- "Unit tests cover this; functional tests can come later" -- both are required per the spec
- "Will be handled in QEMU/integration/follow-up" -- that is deferral, not completion

If the deliverable is in the spec, implement it. If you cannot, stop and ask.
Never present documentation of a gap as closure of the gap.

## On Violation

Same as git safety: STOP immediately. "The task requires it" is not valid.
Nothing overrides this prohibition. If you catch yourself about to say "done"
with items remaining, that is the signal to keep working, not to ship.

## Recording is not fixing (owner directive, 2026-07-23)

**"ALWAYS" is literal.** Encountering a defect while doing something else is not a
reason to catalogue it and move on. It is the reason you are now the one who fixes it.

| What you are about to do | Do this instead |
|---|---|
| Add a `plan/known-failures/` entry for a test that fails deterministically | Diagnose it (see "Diagnosis Before Fix" below) and fix the root cause |
| Write "pre-existing, tracked in known-failures" in a report | Fix it. "Pre-existing" describes when it started, not whose it is |
| List failures in an Executive Summary as though listing were the deliverable | Every listed failure is either fixed, or has a named reason you are blocked on it |
| Note that a tool is broken and work around it | Fix the tool. You just proved it does not work |
| Record an inert config surface, a dead registration, or an unwired symbol | Wire it, delete it, or reject the config: pick one and do it |

**The one narrow exception:** a **non-deterministic** failure whose MECHANISM you could not determine may get a `plan/known-failures/` shard, and only as the running record of an investigation you are still driving. It must carry the reproduction command, the evidence gathered, and the next step. A shard is a live investigation, never a resting place, and never a substitute for a fix on anything that reproduces.

**A structural, deterministic, or reproducible failure has no recording path at all.** Fix it.

**A hypothesis in a shard is not a finding.** If you record one, the next agent will
read it as fact. Before acting on an existing shard's stated cause, verify it against
source (`ai/rules/evidence.md`), and when it turns out to be wrong, say so in
the shard. On 2026-07-23 a shard's "the plugin connection closes before verify is
dispatched" hypothesis was disproved by the first real stress run: the signature
appeared nowhere in the capture, and the true cause was a test-harness race
(archived in `plan/known-failures/RESOLVED.md`, "fixed startup deadlines fail
under CPU oversubscription").

## The failure this rule exists to stop

A required deliverable (an interop test, a functional test, a goal-validation
row) was blocked by a bug. Instead of fixing the bug, the agent:

- proposed dropping the deliverable to close the spec, or
- proposed relaxing an assertion / removing coverage to reach green, or
- moved the unfinished work and the bug report into `tmp/` and called the rest done, or
- labelled the bug "pre-existing" and treated that as permission to leave it.

Every one of these is banned. The bug being pre-existing does not make it
someone else's problem: **the moment your work depends on that code path
working, the bug is in scope.** You found it because you are the first person
to exercise the path end to end. That is exactly the person who fixes it.

## The distinction from legitimate deferral

`ai/rules/planning.md` exists for genuinely separable, out-of-scope
future work. It is NOT a hatch for a blocker. Decide with one question:

**"Does the goal this work exists to achieve still hold if I leave this?"**

| Situation | Verdict |
|-----------|---------|
| The goal still works; this is a distinct, larger, separable feature | Deferral is legitimate. Home it per `planning.md`. |
| The goal does not work / a peer rejects the output / a required test cannot pass | NOT a deferral. Fix it now. Parking it is an invisible scope reduction with a polite name. |

If you are unsure which side you are on, you are on the "fix it" side. The cost
of over-fixing is some extra work; the cost of parking a real blocker is
shipping something that does not do what it claims.

**A defect you own is not a defect you fix this minute. When it does not block the goal, the order is: close the work in hand, then fix it.** "ALWAYS" governs WHETHER you fix it, never WHEN. Fixing it first is how finished work fails to land: the closing commit loses its single focus, the review loses its scope, and the gates that were green run again. Name it, home it per `planning.md`, close, then come back for it (`ai/rules/rule-precedence.md`).

## Banned moves

| Banned | Why |
|--------|-----|
| Offering the user "drop the interop / functional test" as an option | Reducing coverage to reach green is the failure, not a choice. Do not put it on the table. |
| Weakening or deleting a test so a red goes green | `ai/rules/testing.md`, and "No Workarounds For Missing Behavior" below. The test describes the behaviour; the code is what is wrong. |
| "Pre-existing defect, out of scope for this spec" when it blocks the goal | You are the entry point that reaches it. Fix it, or say plainly that you are stuck and why. Never quietly route around it. |
| Moving unfinished work or a bug report to `tmp/` and reporting the rest as complete | `tmp/` is not a destination. Parked is not delivered. |
| Marking a goal-validation row "N/A" or "blocked" to avoid the work | An empty goal validation for a completed feature is a false completion (`ai/rules/interop-and-goal-validation.md`). |

## When you genuinely cannot finish

Being blocked is allowed. Hiding it is not. If a fix is beyond the session
(needs hardware you lack, a decision only the owner can make, or is a deep
redesign), then:

1. State plainly that the goal is NOT met and why, with the evidence.
2. Keep the spec OPEN. Do not close it, do not claim the deliverable.
3. Do the fix if it is at all within reach before asking. Reach for the fix
   first; ask second.
4. If you must ask, ask "which way do you want this fixed", never "may I skip
   it". Scope reduction is the user's call to volunteer, never yours to propose.

## Verification of the goal

The goal is met when the real, user-visible path works against the real
counterpart: the peer daemon accepts the routes, the functional test passes
through the daemon, the interop scenario is green in the suite (not parked).
A passing unit test is necessary, never sufficient, for a goal that is about
interoperating with something outside this codebase.

## Load is never an explanation. It is the bug.

A test that passes on a quiet host and fails on a busy one is a BROKEN TEST. Load
did not break it. Load revealed that it asserts on elapsed time instead of on
state. **Fix the test so load cannot reach it.** Owner directive, 2026-07-26.

These are banned as a conclusion, in a shard, a commit body, a report, or a
reply to the user:

| Banned | What it actually says |
|--------|-----------------------|
| "fails under load / on a loaded host" | the test waits a fixed time instead of waiting for the condition |
| "load average was ~11 vs ~2 earlier" | you measured the host instead of reading the test |
| "passes in isolation" | it depends on scheduling luck. That IS the defect, stated |
| "the failing set rotates, so it is not deterministic" | several tests share one timing assumption. Find it |
| "the contended-run detector did not trip" | that detector labels runs. It never absolves a test |
| "not reproducible, logged as non-deterministic" | you do not need a repro to fix a timing assumption. Read the test |

**There is no non-deterministic hatch for a load-sensitive test.** The exception in "Recording is not fixing" covers a failure whose MECHANISM you could not determine. It does not cover one you already explained by naming the host's load: that explanation is the diagnosis, and the fix follows from it directly.

## Making a test load-proof

Find what the test waits ON, and make it wait for the thing instead of for a
duration.

| Symptom | Fix |
|---------|-----|
| `time.Sleep` / `time.sleep(` then assert | poll the condition: `wait_until`, `dispatch_until`, `wait_for_event` (`test/scripts/ze_api.py`) |
| fixed deadline for startup, teardown or reconnect | wait on the readiness signal the daemon emits. If none exists, ADD one: a missing signal is a product gap, not a test problem |
| "at most N events in a window" | count between two state transitions, not between two clock reads |
| assert immediately after a command returns | wait for the effect to be observable, then assert |
| the test genuinely needs a kernel-global surface to itself | `option=exclusive:group=<name>` (`internal/test/runner/record.go`), not a longer timeout |
| a timeout that is "generous enough" | generous is a synonym for unknown. Bound it by a condition |

Raising a timeout is not a fix. It moves the load level at which the test lies.

Replacing a sleep with a real wait routinely exposes a genuine data race in the
product. That is a feature of the technique, not a reason to avoid it.

`ai/rules/testing.md` and `scripts/dev/stress-repro.py` exist to
DIAGNOSE such a failure, never to decide whether it deserves recording. A
deliberate timer that IS the behaviour under test stays, and says so in its
comment (`ai/rules/testing.md`).

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

Enforced: `check_known_failure_load_excuses` in `scripts/dev/verify_wiring_docs.py`
(`make ze-verify-wiring-docs`, inside `make ze-verify`) fails a CHANGED
`plan/known-failures/` shard containing "under load", "loaded host", "load
average", "load-sensitive", "passes in isolation", "resource contention" or
"contended host". `README.md` and `RESOLVED.md` are exempt: the first states this
policy, the second is a verbatim archive of history and is not edited to satisfy
a present-day gate. The gate checks the excuse, not the existence of a shard:
a red whose mechanism is genuinely unknown still belongs there.

## Length is not evidence

A record earns its length from what a future reader must DO, never from what you
went through. Investigations are not narrated: the wrong hypotheses, the order
you tried things in, and how long it took are yours, not the reader's. State the
correction and move on.

Budgets, the citation rule, and what is banned in every artifact:
`ai/rules/writing.md`. A commit body is under 15 lines and a known-failure
shard is under 20.

## Anti-Rationalization

The answer is always "no."
Rationale: `ai/rationale/anti-rationalization.md`

### TDD

| Excuse | Answer |
|--------|--------|
| "Too simple to need a test" | Test it |
| "I'll write tests after" | Post-hoc tests validate implementation, not requirements |
| "TDD will slow me down" | Rework from bugs is slower |
| "Just a refactor" | Existing tests should pass. None exist? Write them first |

### Test Failures

| Excuse | Answer |
|--------|--------|
| "Transient" / "resource contention" | Investigate. A failure happened |
| "Only fails under load" / "passes in isolation" | That is the diagnosis, not an excuse: the test asserts on elapsed time. Make it wait on the condition (see "Load is never an explanation") |
| "Not related to our changes" | Fix it anyway. Include the fix in a separate commit script |
| "Passed on retry" | Retry is not evidence. Investigate the failure |
| "Timing-dependent" | Race condition. Fix it |
| "Pre-existing issue" | Fix it. "Pre-existing" says when it started, not whose it is. You are the entry point that reached it |

**Every test failure gets FIXED. BLOCKING.** Logging is not an alternative outcome (owner directive 2026-07-23; see "Recording is not fixing" above). A `plan/known-failures/` shard is the running record of an investigation you are still driving, never a place to leave a defect.

1. **Fix it** as a separate commit (not mixed with feature work). Do not block current work on a
   failure you didn't cause, but DO fix it in the same session after completing the primary task.
2. **A shard is allowed for ONE case only: a failure whose MECHANISM you could not
   determine.** Deterministic reds, structural gates, anything with a reproduction
   command, and anything host load explains are fixed, never sharded. When the exception
   does apply, add
   `plan/known-failures/<make-target>-<test-name>.md` with: failure output, the
   reproduction attempt and its result, evidence gathered, and the next step. Label a
   root cause you have not verified against source a HYPOTHESIS, so the next agent does
   not inherit it as fact.
3. **Mechanical check before session end:** every failure your session encountered is
   fixed, or is a non-reproducible one whose shard names the next step. An unfixed
   deterministic failure is a violation regardless of what was written down.

| Banned | Why |
|--------|-----|
| "Pre-existing, not my changes" | Acknowledging a failure without fixing it means the next session hits the same wall |
| "Known issue with the netlink API" | Known to whom? And "known" is not "fixed" |
| Mentioning a failure only in response text | Response text is ephemeral, and describing a bug does not fix it |
| "The only failures are..." (then moving on) | Enumeration without action is rationalization |
| "Tracked in `plan/known-failures/`" offered as the outcome | Tracking is not fixing. The product is still broken. See "Recording is not fixing" |
| Adding a shard for a failure that reproduces on demand | A reproduction command IS the start of the fix, not a substitute for it |

### Completion

| Excuse | Answer |
|--------|--------|
| "Should work" / "Probably fine" | Run it, paste output |
| "Tests passed earlier" | Run again now |
| "Only cosmetic differences" | Show diff, let user decide |
| "Library and interface only" | Feature is not done: library without wiring is dead code |
| "Wiring will be done in next commit" | One commit = code + tests + wiring + summary. No partial deliveries |
| "The .ci test requires infrastructure" | Then the feature is blocked, not done |
| "Unit tests prove it works" | Unit tests prove the algorithm. .ci tests prove the user can reach it |
| "SetAuthorizer is called somewhere" | Show the .ci test where a user command is denied. No test = no proof |
| "Consistent with how other plugins do it" | Other plugins missing tests is a gap, not a precedent |
| "No test infrastructure for this path" | Build the infra or flag as BLOCKER. Never downgrade to NOTE |
| "Out of scope for this review" | Missing coverage is never out of scope. Report as ISSUE |

### 3-Fix Rule

3 failed fixes: STOP. Report all 3 approaches. Question the mental model. Ask user.

### Posture

No performative agreement. Fix it, describe what changed, move on.
Assume your implementation report is optimistic. Re-read spec, re-run verification fresh.

## Diagnosis Before Fix

### The Diagnosis (write all five before any edit)

1. **Symptom** -- the exact failure, verbatim (error text, rejected input, failing assertion).
2. **Root cause** -- traced to the exact function where behavior diverges from intent, named as the file plus the symbol. Read the path; do not guess. If you cannot name it, you have not diagnosed it yet.
3. **Owning layer** -- which layer/component owns the correct fix.
4. **Two candidate fixes, labeled** -- at least one `[workaround]` and one `[source]`. Name what each changes and what each leaves broken for the next caller.
5. **Why not the workaround** -- one sentence on why the local edit is wrong.

Only after the five are written do you implement the `[source]` fix.

### When a check or validation rejects you

Ask the three-way question, not "how do I get past this":

- Is the **check** wrong? (the validation logic is incorrect)
- Is the **input** wrong? (you are doing the wrong thing)
- Is the check's **data/config** incomplete? (the check is right but its allowed-set / table / registry is missing an entry)

The third option is where most "I worked around it" bugs hide. Example: `update bgp irr` rejected because `update` was missing from the registry's allowed-verb set. The verb gate was correct; renaming the command was a workaround. The fix was adding `update` to the registry.

### Altitude

Always ask: am I fixing where the problem **is**, or where it **shows up**? A special case layered on shared infrastructure means the underlying mechanism should be generalized instead. See "No Workarounds For Missing Behavior" below.

### Trigger words (stop and write the Diagnosis)

"let me just rename / just skip / just special-case / just adjust the test / add a fallback / quick workaround". The word "just" is the tell. Stop, write the five, then fix the source.

## No Workarounds For Missing Behavior

A workaround is evidence that the feature, integration, validator, or test coverage is incomplete. The fix must make the user-visible goal work through the real entry point.

When tempted to work around a problem:

1. Name the user goal that should work.
2. Trace the code path that should provide it.
3. Implement the missing behavior at the owning layer.
4. Update affected callers and tests.
5. Verify the user-visible goal directly.

### Banned Fixes

| Banned | Why |
|--------|-----|
| Weakening or simplifying a test expectation | The test describes the required behavior. Broken code must change. |
| Special-casing only the failing fixture | Users can hit the same class of problem outside the fixture. |
| Skipping validation, errors, or unsupported inputs | Silent acceptance hides missing behavior and ships an operator trap. |
| Adding compatibility shims, aliases, or fallbacks instead of clean cutover | Ze has no released compatibility contract. Keep one real path. |
| Bypassing the owning layer from a caller | The next caller will fail the same way. Fix the owner. |
| Hiding a failure behind retries, sleeps, or broad catches | This masks the defect instead of proving the goal works. |

### Allowed Exception

A workaround is allowed only when the user explicitly asks for the workaround itself as the deliverable. In that case, name the limitation in the implementation notes and never present it as the real feature.

### Verification of the user-visible goal

Verification must exercise the user-visible goal, not just the workaround boundary. A unit test can prove internal logic, but the behavior is not complete until a functional, integration, or command-level check proves the user can reach the feature through the intended path.

## Wiring Completeness

The mechanical check behind requirement 9 of "What 'Done' Requires".

### The Principle: Wire First, Feature Second

Wiring is not a verification step at the end. It is the first implementation step.

1. **Design phase:** the spec's Wiring Test table names every entry point before implementation starts.
2. **Implementation phase:** `/ze-implement` step 4 creates the entry point skeleton and a failing wiring test before any feature code is written.
3. **Review phase:** `/ze-review` step 1 checks wiring before any other analysis.
4. **Completion phase:** the mechanical check below catches anything that slipped through.

If you find yourself checking wiring for the first time at completion, three earlier gates failed.

### Mechanical Check (MANDATORY before claiming done)

`make ze-verify` runs `make ze-verify-wiring-docs`. That changed-file
gate is blocking and checks:

- new exported Go symbols under `internal/` or `cmd/` have a non-test
  production reference in `internal/` or `cmd/`;
- command declaration changes run `make ze-validate-commands`;
- source-anchored documentation changes run doc drift and stale-anchor
  checks;
- plugin registration and generated inventory source changes run
  registry-backed inventory checks.

For manual review of a specific new exported symbol `Foo`, confirm it
is not only a definition plus tests:

```
grep -rn 'Foo' internal/ cmd/ --include="*.go" | grep -v "_test.go" | grep -v "plan/"
```

If the only hits are the definition and test files, the symbol is dead
code. Dead code is a BLOCKER, not a NOTE.

For multi-consumer data (route attributes, config fields, bus events),
grep all consumers: UI templates, graph rendering, functional tests,
CLI formatters. Changing the producer without updating consumers is
incomplete, not done.

### Common Violations

| Pattern | Why it's wrong |
|---------|----------------|
| "The caller will wire it later" | Later never comes. Other sessions see it as done. |
| "It's available for callers" | Available is not wired. No caller means no effect. |
| "The review said NOTE" | Reviews must flag unwired code as BLOCKER. |
| "The web UI doesn't need it" | If the feature produces data that a UI page renders, the UI must show it. |

### Where to Check

| New code in | Must be called from |
|-------------|---------------------|
| `internal/component/host/` | `cmd/ze/hub/main.go`, `loader_create.go`, `internal/component/cmd/show/system.go`, or `web/page_system.go` |
| `internal/component/config/system/` | `cmd/ze/hub/main.go` (startup + reload) |
| Any new metrics registration | `loader_create.go` telemetry block |
| Any new report bus emission | Verified via `show warnings` / `show errors` |

## Feature Integration Completeness

Every new feature MUST be proven to work integrated, not just in isolation.
Rationale: `ai/rationale/integration-completeness.md`

| Feature Type | Required Test |
|-------------|---------------|
| Injectable interface | Inject fake, verify component uses it |
| CLI flag | Flag changes program behavior |
| Config option | Option affects runtime behavior |
| YANG config leaf | Env var registered (`env.MustRegister`), appears in `ze env registered` |
| API/RPC | Caller reaches handler through real transport |
| Event/hook | Event fires, subscriber receives |
| Plugin capability | Engine dispatches to plugin correctly |
| Struct field | Field is read and affects a decision |

**Self-check:** "If I deleted all new code except tests, would any test fail because it tried to USE the feature through the intended path?" No, isolation only, rule violated.

### Functional `.ci` Test (BLOCKING)

**Every user-facing feature MUST have a `.ci` functional test** in `test/` that exercises the feature from the user's perspective: config file, ze launch, command/event, expected output. A Go unit test proves the algorithm; a `.ci` test proves a user can reach and use the feature.

| Feature Type | `.ci` Location | What the test does |
|-------------|----------------|-------------------|
| Config option | `test/parse/` | Config with option, ze parses without error |
| API/RPC command | `test/plugin/` | Config + peer, send command, verify wire/JSON output |
| Plugin behavior | `test/plugin/` | Config + plugin, trigger behavior, verify effect |
| CLI subcommand | `test/parse/` or `test/ui/` | Run subcommand, verify stdout/stderr/exit code |
| Wire encoding | `test/encode/` | Config with route, verify hex output |
| Wire decoding | `test/decode/` | Hex input, verify JSON output |

**A unit test is NOT a substitute for a `.ci` test.** Unit tests validate logic in isolation. `.ci` tests validate the feature is wired, reachable, and usable. Both are required.

**Deferrable:** advanced behavior (deterministic scheduler, fault injection, property testing, benchmarks).
**NOT deferrable:** one `.ci` test proving the feature works from the user's entry point.

### Wiring Tests (BLOCKING, NEVER deferrable)

A wiring test proves the feature is reachable from its intended entry point (config, CLI, event dispatch, plugin launch). It is the minimum proof that the feature is integrated, not just isolated. **For user-facing features, the wiring test MUST be a `.ci` functional test**, not a Go unit test.

| Banned | Why |
|--------|-----|
| "Deferred to next spec" | Next spec won't pick it up. Feature ships unwired. |
| "Requires infrastructure not yet built" | Then the feature is blocked, not done. |
| "Unit tests cover the logic" | Unit tests prove the algorithm, not the wiring. |
| "make ze-verify passes" | Passing tests that don't exercise the entry point prove nothing. |
| "Go test exercises the handler" | A Go test with mocked entry points is not a `.ci` test. |

**If the wiring test cannot be written, the feature is not done: it is blocked.**

Every spec MUST have a `## Wiring Test` table (see `plan/TEMPLATE.md`). Every row for a user-facing feature must name a `.ci` test file.

### Production Path Verification (BLOCKING)

Before modifying any handler, dispatcher, or protocol step: **grep for ALL implementations** of that function/protocol step in the codebase. Ze has multiple code paths for the same protocol (e.g., `subsystem.go` and `plugin/server/startup.go` both implement stage-1). Modifying one is not enough.

| Step | Action |
|------|--------|
| 1 | Grep for the protocol method/handler name across all `.go` files |
| 2 | List every implementation found |
| 3 | For each consumer of the feature: trace which implementation it actually calls |
| 4 | Modify (and test) the implementation the consumer uses, not just any implementation |

**One implementation found is not proof there's only one.** Finding *a* handler is not the same as finding *the* handler the feature's consumer calls.

## Implementation Audit

Rationale: `ai/rationale/implementation-audit.md`

### When to run the audit

Before: writing summary to `plan/learned/`, claiming "done", asking to commit.

### Process

1. Extract all requirements from spec: task items, AC-N assertions, TDD tests, files listed
2. Verify each with status: ✅ Done (file + symbol), ⚠️ Partial, ❌ Skipped, 🔄 Changed
3. Fill audit table in spec (template in `plan/TEMPLATE.md`)

### Approval Required

- ⚠️ Partial: document what's missing, ASK user
- ❌ Skipped: explain why, ASK user
- 🔄 Changed: document deviation (no approval needed if improvement)

### Cannot Mark Done Until

- [ ] Every Task requirement has a status
- [ ] Every AC-N has status + "Demonstrated By" evidence
- [ ] Every TDD test has a status
- [ ] Every file in plan has a status
- [ ] All Partial/Skipped have user approval
- [ ] Integration points verified (YANG, CLI, docs)
- [ ] Wiring Test table complete, every row has a test name, none deferred
- [ ] Audit Summary totals accurate

### Evidence Standards

| Claim | Acceptable Evidence | NOT Acceptable |
|-------|-------------------|----------------|
| Feature works | Test name + output | "make ze-verify passes" |
| Feature is wired in | Wiring test that exercises entry-to-feature path | Unit test with mock/fake entry point |
| AC-N done (wiring) | Functional test name exercising full path | Unit test in isolation |
| AC-N done (logic) | Unit test name + file, assertion matches AC text | "should work" |
| AC-N done (behavior) | Test asserts the AC's expected behavior directly | Test asserts mechanism (e.g., "no error" as proxy for "rejected") |

### AC Evidence Verification (BLOCKING)

For each AC-N, quote the expected behavior from the AC table, then name the test and its assertion. The assertion must verify the BEHAVIOR, not just the mechanism. See `ai/rules/testing.md` "AC-Linked Tests" for the full behavior-vs-mechanism table and mechanical check.

### Pre-Commit Verification (BLOCKING)

**Do NOT trust the audit.** After filling the audit, independently re-verify every item.
This is a separate section in the spec (see `plan/TEMPLATE-CLOSURE.md`, appended at
`/ze-close` step 1). It requires FRESH evidence:

| Table | What to verify | How |
|-------|---------------|-----|
| Files Exist | Every file from "Files to Create" | `ls -la <path>`, paste output |
| AC Verified | Every AC-N | grep, test output, or ls, NOT a copy from audit |
| Wiring Verified | Every wiring test row | Read the .ci file, confirm it tests the claimed path |
| Assumptions Resolved | Every A-N | `confirmed` or `broken` with evidence; `unvalidated` is not a final status |
| Documentation Verified | Every Yes/No in the Documentation checklist | The edited claim checked against source, or the grep proving no update was needed |

**EVERY table needs at least one evidence row.** `pre_commit_verification_gaps`
(`scripts/dev/commit_helper.py`) checks them one at a time and names the empty
ones on the closure commit. Each table is a separate obligation: a row in
`Files Exist` is not evidence for `AC Verified`. The old gate accepted a single
row anywhere in the section, and ~73% of `AC Verified` and ~75% of
`Wiring Verified` tables reached closure byte-identical to the template.

**NOT acceptable:** "Already checked in audit", "should work", empty cells.

### Red Flags

- AC-N with no test or evidence
- Can't find where feature was implemented
- TDD test from plan doesn't exist
- File from "Files to Create" wasn't created
- New RPCs without functional tests
- New CLI commands without usage text
- Learned summary admits incompleteness ("not wired", "infrastructure only")
- Commit message says "library and interface only"

## Don't Ask, Do

Never use phrases like "would you like me to", "want me to", "shall I",
or "I can" before completing work. Finish the task first, then report
what was done. The user delegated the work; asking for permission to
start it wastes a round-trip.

Exception: genuinely ambiguous scope or destructive actions that require
confirmation per the git safety rules.

Standing exceptions, where asking is MANDATORY and this rule does not apply:

- **RFC compliance.** When full RFC compliance and full testing of that compliance is reachable, IMPLEMENT it and prove it: that is not a question for Thomas (`ai/rules/rfc-compliance.md`, "Implement Full Compliance. Ask Thomas Only Before Doing LESS"). Asking is mandatory only when you are about to choose something NARROWER, and then the question is "which way do I fix it". Doing more never needs permission.
- **Deleting or overwriting user-visible or uncommitted work** (`ai/rules/never-destroy-work.md`).
- **Reducing the scope of a spec or dropping an acceptance criterion** (see "Scope Reduction Requires Explicit User Approval" above).

### Enforcement

This rule is hook-enforced. Breaking it costs a blocked Stop, not a note.

- `.claude/hooks/block-premature-stop.sh` scans the last assistant message against a phrase list and exits 2 on the first match. Exit 2 refuses the session an end and returns the turn to the model. The hook is live and first in the `Stop` array since 2026-07-31, after it sat on no event from 2026-06-29 (`41e5fa44f`).
- Two lists, and only one of them is unconditional. `PHRASES` covers ownership-dodging, premature handoff and permission-seeking, and it always blocks.
- `COMPLETION_PHRASES` covers `what next`, `what would you like` and `what do you want to do`. These join the scan ONLY when work remains, which the hook reads as a claimed spec still `in-progress` (the `OPEN_WORK` flag). Asking what to do next is not the same failure as asking permission to do what was already requested. `.claude/rules/session-start.md` REQUIRES the question once the original task is done. The phrases were split rather than deleted, so the same words still block while a spec is open.
- **The retry bound is scoped to this scan, and it disables nothing else.** When the harness sets `stop_hook_active`, the flag `STOP_RETRY` skips the scan loop alone. That bounds a refusal loop whose only escape is rewording. The spec-closure gate above it still blocks on a retry, because that gate has two escapes of its own: run commit B, or write `tmp/session/.closure-ack-<stem>`. Do not read a blocked stop as a licence to stop next turn. The hook also exits 0 on input it cannot parse.
- A banned phrase inside backticks or a closed fence is treated as QUOTED, not used, and does not block. Write about the phrases freely. Four guards keep that exemption from becoming a bypass. An unclosed fence is not a code block. A fence closes only on a run at least as long as the opener. The hook scans an all-markup message raw. Inline spans are stripped only on a line whose backticks balance, so one stray backtick cannot swallow a real request.
- Neither list is exhaustive, so a green Stop is not proof you followed this rule. Finish the work, then report.
- Fixtures: `python3 scripts/dev/hook-fixture-check.py --only delegation`. Full hook map: `ai/rules/repo-maintenance.md`.

## Rationale

The recurring failure behind "Diagnosis Before Fix" is jumping from symptom to the
nearest edit that silences it: rename the command so it stops being rejected, skip
or relax the test so it stops failing, special-case the one input that breaks. That
fixes where the problem *shows up*, not where it *is*. The cure is to change the
success criterion from "symptom gone" to "root cause named and fixed at the owning
layer", and to produce the diagnosis BEFORE touching code.

Both halves of "Fix, don't record" were paid for on 2026-07-26. A shard argued at
length that a rotating failure set proved non-determinism, when a rotating set
across teardown-shaped tests is the signature of one shared timing assumption. The
diagnosis was sitting unread inside its own record.
