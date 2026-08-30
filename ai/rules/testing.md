# Testing

**When:** writing, changing, or deleting any test, and before writing implementation code for new behavior
**Severity:** blocking
**Related:** completion, platform-linux, rfc-compliance

## Directives

- **Tests MUST exist and fail before implementation.**
- **Every user-facing behavior MUST have a functional test that exercises it through a user entry point. Unit tests (`_test.go`) prove internal logic. Functional tests (`.ci`, `.et`) prove the feature works end-to-end through the daemon. Both are required. Neither substitutes for the other.**
- **A red test means the CODE is wrong by default. MUST diagnose the failure and fix the source. MUST NOT weaken the test to make it green. MUST ask the user before deleting OR weakening any test code (`*_test.go`, `.ci`, `Test*`, `t.Run`, assertions, table entries). Exception: the user already explicitly requested it.**
- **A test that cannot run on every OS MUST either carry a build tag (`//go:build linux`) on its file, or skip (`t.Skip`) with a reason on the OSes where it cannot run. MUST NOT weaken the assertion to accept both outcomes.**
- **Every `time.sleep(` call in a `.ci` test MUST have an explanatory comment on the line directly above it, or trailing it on the same line. A bare sleep with no comment is rejected.**
- **A load-dependent failure is DIAGNOSED, and the outcome is always a fix. Load-dependence is the diagnosis (the test asserts on elapsed time instead of on state), and `ai/rules/completion.md` bans recording it as a `plan/known-failures/` shard, bans "passes in isolation" as a conclusion, and bans raising the timeout. MUST reproduce it with the stress reproducer, then fix the timing assumption.**

## Test-Driven Development

1. Write test with `VALIDATES:` and `PREVENTS:` comments
2. Run → MUST FAIL (paste output)
3. Minimum implementation
4. Run → MUST PASS (paste output)
5. Refactor while green

- **Table-driven:** `tests := []struct{...}` with `t.Run(tt.name, ...)`
- **Round-trip:** `original → packed → unpacked == original`
- **Fuzz (REQUIRED for wire format):** All external input parsing
- **Non-default params:** MUST test with non-default/non-zero values

All numeric ranges MUST test: last valid, first invalid below, first invalid above.

**Code that ships SHOULD reach the coverage target its row names.**

| Code Type | Target |
|-----------|--------|
| Wire format (pack/unpack) | 90%+ |
| Public functions | 100% |
| Error paths | 100% |

Every AC-N MUST have a test whose assertion directly verifies the AC's **expected behavior**, not just the mechanism used to achieve it.

| AC text says | Test MUST assert | Test MUST NOT assert |
|-------------|-----------------|---------------------|
| "rejected" / "not installed" | Route is absent from delivery / RIB | No error returned (mechanism) |
| "session torn down" | Connection closed + NOTIFICATION sent | NOTIFICATION struct returned (mechanism) |
| "warning logged" | Log entry exists (or counter incremented) | No teardown (absence of something) |
| "rejected at parse time" | Error returned with specific message | Generic error returned |

**The test:** MUST quote the AC expected behavior in the `VALIDATES:` comment. MUST read the test assertion. Does it verify that exact behavior? If the assertion would still pass with a stub implementation that does nothing, the test is invalid.

**Red flag:** A test MUST NOT assert the ABSENCE of an action ("no NOTIFICATION", "no error") as proof that a DIFFERENT action happened ("routes rejected"). Absence of X does not prove Y.

- If you debug something, MUST add a test so it's never re-investigated
- Implementation written before its test → MUST back-fill the test. Working product code MUST NOT be deleted to restore the ordering (`ai/rules/pre-release.md`)
- Test passes immediately → invalid test, MUST add failing assertion
- Claiming "done" without test output → MUST run it once, paste it

## Draft a Functional Test Before It Is Live (BLOCKING)

**A `.ci` MUST NOT be written or iterated on inside `test/<suite>/`, and a live
one MUST NOT be edited in place.** Copy it into `test/draft/<suite>/`, work
there, and move it back. That directory runs on every verify in the checkout,
including runs by OTHER sessions, who then have to work out whether your
half-written test is their regression. The incubator's contract is
`docs/functional-tests.md`.

**A draft is not a test yet, so the draft workflow MUST end in exactly two moves:
promote it into `test/<suite>/`, or delete it.** Leaving it in the incubator is
the third move, and it is the one that is refused. A draft proves no obligation,
claims no evidence, and appears in no coverage ledger, so a session that finds
one cannot tell abandoned scaffolding from work in progress.

**Deleting a draft needs no approval, and a command that mixes a draft with a
LIVE test is still refused.** The guards that protect tests do not protect
drafts: `bashTestDeletion` and `writeWeakening`
(`internal/le/hookruntime`) both exempt a path under the incubator, and only a
path under it.
**An `RFC requirement:` tag inside a draft is worth nothing until the file is
live**, which is why the tag guard skips it. Promoting the file is what turns the
tag into proof.

**A draft SHOULD be promoted early, because nothing in the incubator is gated.**
The accept-only check, the `time.sleep(` ratchet, and frame-length validation all
start applying only once the file is live.

## Test Code Is Held to One Standard

- **Test code is held to ONE standard: it MUST run, and it MUST be correct about the product.** The coverage targets above are for the code that ships. A test helper, a fixture builder, a `.ci` or `.et` script and the runners under `test/` need no coverage figure, no boundary sweep, and no test of their own. Spend that budget on the behavior under test, which is the only thing an operator ever meets.
- **A bug in test code that leads to NO TESTING is load-bearing, and it is fixed like product code.** A test the runner never selects, a skip that reports green, a harness that never reaches the code under test, a fixture that builds the wrong scenario, an assertion nothing evaluates: the suite claims coverage it does not have, and that claim is what the product is shipped on.
- **What else still applies is everything that decides what a test PROVES: it fails when the behavior breaks, it asserts the acceptance criterion rather than the mechanism, it never encodes a violation, and a gate still refuses what it exists to refuse.** Those are the sections around this one, and none of them is softened here. A defect in test-only code outside that set is a NOTE in review (`ai/rules/planning.md`, "Critical Review Is the Central Deliverable"), never a spec.
- **A tool that already carries tests keeps them.** Native Go tests beside packages under `internal/le/` exist because a gate that stops refusing is a product-visible failure. This point removes an obligation to ADD coverage over harness code; it removes no test that is there.

## Fix Code, Not Tests

**When a test fails, the CODE MUST be fixed to make it pass, and the test's
expectations MUST NOT be weakened or simplified to match broken code.** Tests are
ground truth. When an underlying mechanism changes (Unix sockets replaced by SSH,
for instance), the expectations stay and the replacement mechanism satisfies
them.

**Without explicit user authorization, test data (a golden file, expected output, a fixture, a `.ci` expectation) MUST NOT be modified to make a failing test pass.**
When output changes, the default assumption is that the code is
wrong, not the data. MUST ask the user before updating any test data, even when
the new output looks plausible.

## Test Deletion and Weakening

**Legitimate reasons a test MAY be deleted:** testing removed functionality, duplicating another test, fundamentally wrong, replacing with better coverage.
**Reasons that MUST NOT justify deletion:** failing and hard to fix, slow, "annoying", don't understand what it checks.

**Every weakening below MUST NOT be introduced. What the hook REFUSES is a
narrower set than what this rule forbids, and the difference is not permission.**

Refused on Edit / Write / MultiEdit to a test file (exit 2). Each is
one-directional: no innocent edit produces it.

- adding `t.Skip` / `t.Skipf` / `t.SkipNow` (the test stops running)
- commenting out assertions
- adding an `ignore` build tag (file dropped from the build)
- deleting a `Test`/`Fuzz`/`Benchmark` func
- emptying a `.ci` expectation's needle, so it matches anything
- introducing an assertion that cannot fail
- replacing a test's content with nothing

Reported and allowed through at EDIT time (exit 0, a notice, and the hook asks
for no row). Each is a COUNT falling, and a count cannot tell a deleted check
from three consolidated into one:

- removing assertions (any net drop)
- downgrading fatal assertions to non-fatal (`require` -> `assert`, `t.Fatal` -> `t.Error`)
- removing `t.Run` cases or table rows
- removing `.ci` `expect=` / `reject=` / `cmd=` lines

**The reported set MUST still be judged by the agent making the edit, and the
COMMIT that carries it MUST carry a row for it.** `weakened_problems`
(`internal/le/commit`) records every weakening kind, count drops
included, so the commit asks for a row the hook did not. Say in the row which
happened: the coverage moved, or it went.

**Not detected (by design, to avoid false positives):** changing an expected
value in place while the assertion structure stays (e.g. `Equal(t, 1, x)` ->
`Equal(t, 2, x)`). This is the one weakening the hook cannot see; MUST treat it with
the same discipline manually. Adjusting an expected value to match broken code is
the same violation as removing the assertion.

**A new issue in an area that already has tests MUST get a NEW test case or
function, and an existing test MUST NOT be repurposed to cover the new
behavior.** The old test verified a behavior that still needs coverage.

**The Wrong column MUST NOT be taken in any of these scenarios.**

| Scenario | Correct | Wrong |
|----------|---------|-------|
| New bug in `parsePeer`, existing `TestParsePeer` | Add `TestParsePeerRejectsEmpty` alongside `TestParsePeer` | Rewrite `TestParsePeer` to test the new edge case |
| Table-driven test, new case needed | Add a row to the table | Replace an existing row with the new case |
| Existing test fails because code changed | Fix the code so both old test and new test pass | Rewrite the old test to match the changed (broken) code |

**Why the hook cannot catch this:** the rewrite maintains the same structural
shape (same function count, same assertion count), so the mechanical check sees
no weakening. The coverage loss is semantic, not structural, so a passing
structural check MUST NOT be read as proof that nothing was weakened.

**Detection:** `/ze-review` step 0 runs the native structural weakening audit.
For semantic replacement, `/ze-review` step 7 (removed-behavior audit) MUST
verify that every assertion the diff replaces still has coverage elsewhere.
When reviewing a test edit that changes WHAT is asserted (not just adding new
assertions), ask: "is the old behavior still tested?"

**A row in `test/weakened.md` MUST be written for the ONE weakening in hand, and
the commit MUST carry that file.** The row is self-service: the agent that
weakened the test writes its own justification, so the only thing that makes it
safe is a human reading it. The file is replaced per commit: it holds the rows of
one change, and git history holds the rest.

**A legitimate weakening MUST have its row written in `test/weakened.md` BEFORE
the edit is made, and that row MUST name the test THIS edit weakens.** The
detector reads the file from disk, so a row written after the refusal buys
nothing until the edit is retried, and a row naming another test opens nothing.
The row's format is `docs/architecture/testing/test-health.md`.

**A row in `test/weakened.md` MUST state a real reason; writing one without a
reason is a violation.** The row unblocks the edit and leaves the audit trail,
and the commit MUST carry the file: `internal/le/commit` refuses a commit that
weakens a test without it, so no row is left behind in the working tree. Why the
file is replaced per commit rather than accumulated is
`docs/architecture/testing/test-health.md`.

## Functional Test Gate

When you add or change user-facing behavior, a corresponding functional test MUST
exist in the correct `test/` directory. "User-facing" means: reachable via CLI command,
config option, API call, web endpoint, plugin event, or wire protocol exchange.

**The functional test a change owes MUST take the form and live in the directory
its row names.** Each `test/<subdir>/` has its own runner and format, and they
are not interchangeable.

| Change type | Required functional test | Directory |
|-------------|------------------------|-----------|
| New/changed BGP wire behavior | `.ci` with `expect=bgp:` hex match | `test/encode/` or `test/decode/` |
| New/changed plugin behavior | `.ci` with API commands + expectations | `test/plugin/` |
| New/changed config option | `.ci` with parse success/failure | `test/parse/` |
| New/changed CLI subcommand | `.ci` with `cmd=foreground` + `expect=stdout` | `test/ui/` |
| New/changed web endpoint | `.ci` with HTTP expectations | `test/web/` |
| New/changed editor behavior | `.et` with input/expect directives | `test/editor/` |
| Config reload behavior | `.ci` with `action=sighup` | `test/reload/` |
| Managed/fleet behavior | `.ci` | `test/managed/` |
| Cross-component integration | `.ci` | `test/integration/` |
| Interoperability | `.ci` | `test/interop/` |

**A change fitting no row (a pure internal refactor with no user-visible effect)
owes no functional test, and anything short of certainty about that SHOULD be
answered by writing one.**

**Unit tests alone MAY stand without a functional test ONLY when the change
matches one of these rows. In every other case both kinds are required.**

| Condition | Example |
|-----------|---------|
| Pure internal logic with no user entry point | Helper function, data structure, algorithm |
| Existing functional test already covers the path | Bug fix where the `.ci` test already exercises the scenario |
| Wire encoding internals tested via round-trip | `pack -> unpack == original` in `_test.go`, AND a `.ci` encode test covers the message type |

1. Disable the producing function (the code the test exists to prove): an early
   `return`, a no-op, or `if true { return }` at the top of the function.
2. Re-run the test. It MUST flip to RED. If it still passes, the test does not gate
   on the feature: find the alternate delivery path and design it out (inject with no
   peers, remove the fallback store, use a genuinely-new peer instead of a reconnect),
   or the test is worthless: delete it, do not ship it.
3. Revert the mutation immediately and confirm the test is green again.

**A behavior that genuinely cannot be made to fail under mutation because it is
not observable end to end MUST be guarded by a UNIT test that inspects the
producing value directly, and the test comment MUST say so. A `.ci` that passes
with the feature disabled MUST NOT be kept.** The reactor suppressing a duplicate
announce, which makes per-peer targeting wire-indistinguishable, is the shape
this covers.

**A functional fixture MUST give distinct roles distinct identities, even where the
host lets them share one.** Loopback, one container and one process make it cheap
to hand two ends of a protocol session the same address, the same identifier or the
same port. The protocol forbids that, so a fixture built on it encodes a state no
deployment can reach, and every assertion it makes is about a machine that cannot
exist.

**The cost is paid later, by somebody else.** A guard that reasons about identity is
correct and still reddens such a fixture. The session that meets that red has to
prove the guard right before it can call the fixture wrong, and the cheap move at
that moment is to weaken the guard. That is the one move this point exists to stop.

**Give each role its own identity when you WRITE the fixture.** It costs one line
there and an investigation anywhere else.

**Each row below MUST NOT be given as a reason to ship without a functional test.**

| Pattern | Why it's wrong |
|---------|----------------|
| "Unit tests cover this" | Unit tests prove the function works in isolation. They do not prove the daemon exposes the feature to users. |
| "The wiring test passes" | Wiring proves reachability. Functional tests prove correct behavior through the full path. |
| "The `.ci` is green" | A test that passes with the feature DISABLED (false-pass) guards nothing. Mutation-verify it: disable the producing function, confirm the test goes red. |
| "I'll add the .ci test later" | Later never comes. The feature ships without end-to-end coverage. |
| "The behavior is too simple to need a functional test" | Simple behaviors break when config parsing, CLI dispatch, or plugin registration changes. The functional test catches that. |
| "There's no test infrastructure for this path" | Build the infrastructure or flag it as blocked. Do not skip the test. |

- `completion.md` requirement #2: both unit AND functional tests MUST exist per AC
- `completion.md` checks that code is reachable; this rule checks that the reachable path is tested end-to-end
- "Test-Driven Development" (above) governs the test-first cycle; this section governs test completeness at the feature level
- "No Throw-Away Tests" (below) has the directory table and iteration workflow; this section makes the directory mapping a gate

## RFC-Tagged Tests (BLOCKING)

**A test carrying an `RFC requirement: <id> <polarity>` tag MUST NOT be edited to
match the code.** It is the proof behind a public compliance claim in
`docs/features/rfc-status.md`, and `./le rfc check` counts it as that proof, so
such an edit retires the evidence while the claim stays up.

**The row that matches the situation MUST be followed.**

| Situation | Do |
|-----------|-----|
| A tagged test fails after your change | Fix YOUR code. The test is the requirement |
| You believe the test is genuinely wrong | STOP. Show the user the RFC text beside the test and ask. Do not edit first and explain after |
| The summary misquotes the RFC | Fix `rfc/short/rfcNNNN.md` (keep the id), then re-run `/ze-rfc-audit` |
| Reformat / comment / re-tag | Allowed, and the behavior stays unchanged |
| You added, moved, deleted, or re-tagged a tagged test (or an edit shifted its line) | Run `./le rfc index-update` and commit BOTH of its outputs in the SAME commit: `ai/RFC-REQUIREMENTS.md` and every changed file under `rfc/requirements/`. The per-RFC file records each test's `file:line`, and `./le rfc check` (both verify modes) fails on a stale index AND on a stale per-RFC file, so committing the index alone lands on the next session as a red gate |

**A tag MAY live only in a recognized carrier, and every carrier is declared once in `internal/le/rfc/carriers.go` and derived from there by the scanner, the HEAD baseline, the ledger and the ratchets. Evidence has two axes: KIND (which layer the test exercises) and TIER (whether anything executes it), and neither is declared by the test.** The carrier table and what each one earns is `docs/contributing/rfc-implementation-guide.md`.

- **SHOULD prefer a `.ci` over an interop binding** when a behavior is reachable from both: a `.ci` runs inside `./le verify current mode full` on every push, interop does not (owner decision, umbrella D3).
- A requirement whose ONLY evidence is nightly-tier is marked `**nightly-only**` on its ledger row and counted in its own rollup column: it is not merge-gate-proven, and the rollup deliberately never sums the two.
- **An interop tier is DERIVED; it MUST NOT be declared.** A native Go test under `internal/le/interoplab/` earns `interop/nightly` when a scheduled workflow names its registered `./le` runner. `internal/le/rfc.Carriers` derives that relation, so adding the job is the whole fix and deleting it removes the tier.
- **A scenario configuration directory is not an evidence carrier.** RFC tags belong in the native Go checker test that executes the assertion. A fixture name or configuration file cannot claim a tier.
- **A QEMU sibling is not that pipeline.** The registered QEMU actions execute their own Go packages and cannot justify an interop tier for a checker they never call.
- **Non-unit evidence is monotonic, per requirement and per tier.** Replacing a `.ci` binding with a unit tag, or with a nightly interop tag, fails `./le rfc check`, and no annotation satisfies it.

**A row in `test/weakened.md` MUST NOT be read as approval to change an
RFC-tagged test.** It is the agent's own justification, never the user's
approval. The `rfc-tagged-test` check runs BEFORE `test-weakening` precisely so
the weakening record cannot pre-empt it.
**Once the USER approves, what they approved MUST be written as one row in
`test/rfc-changed.md`, naming the test the change touches, and that file MUST be
committed with the change.** The hook reads the file from disk, so the row comes
first and the edit second.
**A justification explains ONE diff, so it MUST live with the commit and MUST NOT
be left in the test file forever.** That is why `test/weakened.md` is replaced per
commit: a permanent comment explains a change the reader of the test file can no
longer see.
**`rfc-test-change-approved:` is RETIRED (owner ruling, 2026-08-19) and a new one
MUST NOT be written.** No gate reads one. `writeWeakening`
(`internal/le/hookruntime/writeedit.go`) and the commit gate both read
`test/rfc-changed.md` instead. `test-asserts-nothing:` is NOT retired:
`escapeComment` (`internal/le/testhealth/collect.go`) still reads it.
**Before writing any justification an instruction hands you, MUST check whether
this repository already has a canonical home for that class of record.** A gate's
message states a constraint to satisfy; it does not decide where the record
belongs.

**Every gated requirement MUST have BOTH a positive and a negative test.** A
negative-only test passes when the code rejects everything, and a positive-only
test passes when it accepts everything; only the pair pins behavior to the
requirement.
**The assertion MUST name the EXACT outcome and MUST NOT assert a floor.**
`GreaterOrEqual(TreatAsWithdraw)` is also satisfied by `SessionReset`, so it
cannot fail when the implementation over-reacts.

## Back-Fill New Test Types (BLOCKING)

**A new test type, technique, or infrastructure (a fuzz target, a property test,
a mutation gate, a `-race` sweep, a clock-injection audit, a new `.ci` or `.et`
category, a QEMU harness) MUST be applied to the existing code it covers, in the
same work, not only to the code added alongside it.** Coverage that grows only
forward from the introduction date is the trap
(`plan/learned/RECURRING-PATTERNS.md`).

1. MUST name the applicable set: the package glob, symbol kind, or call-site pattern the new test type is meant to cover.
2. MUST back-fill that set, or record the uncovered remainder as explicit, tracked backlog (spec, handoff, or deferral table). MUST NOT leave it implicit.
3. SHOULD prefer a grep- or registry-driven audit that enumerates every applicable site over per-file judgement. `/ze-hunt` enumerates sites for grep-detectable patterns.

## Test Sensitivity Ratchets (BLOCKING)

**A test MUST NOT re-implement the logic it names, and no gate catches one that
does.** Such a test builds the production algorithm again inside itself, in a
local variable, then asserts on its own copy. It DOES assert, so the
assert-nothing detector never sees it, and it is green against the correct
implementation and the broken one alike.
**The tell is mechanical and it is the only one there is: the test names a
function it never calls.** Before writing a test, name the function under test
and check the body calls it. When reviewing one, MUST read what the assertion
reads FROM: a local the test itself filled is the defect, whatever the test is
called. A broad detector would also flag correct table-driven tests that build
local fixtures, so this stays a review obligation.

**`docs/features/test-health.md` MUST be read before the suite is claimed
healthy.** It is generated by `./le test-health update` and reports the
sensitivity counts alongside RFC proof density, mutation kill rate,
negative-test ratio, and technique adoption by package age. It is the answer to
"would a regression be caught", which no test count gives. The mechanism is
`docs/architecture/testing/test-health.md`.

## The Affected Population Is Not the Edited Population

**When a change alters what reaches a component at runtime, the tests you MUST re-check are the ones its new semantics can REACH, not only the ones it edited.** Delivery, wiring, subscription and permission are the four shapes of that change: each one moves a fixture onto a different code path while every line of that fixture stays as it was.

**Every gate in this repository scopes itself to the files the commit touched, so the reachable set is yours to find.** `./le commit audit` builds its population from `git diff --name-status`, and the lint, the weakening audit and the changed-file targets all read that same list. A fixture the change never opened is outside every one of them.

**A recorded discrimination proof MUST NOT be relied on after a change to what
reaches the component under test.** `ai/rules/interop-and-goal-validation.md`
proves a test could fail on the day it was proven, against the wiring of that
day. A change to what reaches a component moves a green test onto another rail
without touching one assertion, so the test still passes, no gate reddens, and
the recorded proof now describes code the test no longer runs.

**MUST derive the reachable set from the graph the change alters, then state that derivation and its count in the report.** A set derived from `git diff` is the edited set wearing another name, and it answers a question nobody asked.

**The derivation is one command when you name the graph first.** For a delivery change the graph is the config: the set was every fixture attaching a program whose feed changed, which `git grep -l 'attach process <name>' -- test/` returns in full. For a permission change it is every fixture whose peer sends through the rail you gated. Name the edge, then list the files that carry it.

**The audit that reads whether a test still enforces what it names already exists, so MUST run it over the reachable set rather than write a new one.** `/ze-rfc-audit` records a verdict per requirement, and `check_audit_freshness` (`internal/le/rfc/rfc.go`) invalidates that verdict when the tagged test changes. What is missing is a trigger and a scope: nothing routes a semantics change to that audit, and its population is edited files.

**A fixture carrying no RFC tag has no audit, so MUST read its header against its config yourself.** A header that names the rail, the plugin or the topology under test is the assertion the runner cannot check. When the change contradicts that header, the fixture is now testing something else and MUST be fixed in the same work, never left green.

**The tests you write for a change are written against its NEW contract, so they are green by construction and cannot tell you the change is safe.** The population that CAN go red is the one written against the OLD contract, and that is exactly the population you did not edit. "My tests pass" is therefore not evidence about a contract change. It is a restatement of what you just wrote down.

**So when a change alters what a function HANDS its callee, or what a shared artifact CLAIMS, you MUST run the whole suite for every file whose contract moved before claiming done, never only the cases you added.** Both of those are contracts, and neither is visible from a new test. A changed-file gate cannot help: it scopes to the diff, and the old-contract tests are outside it.

Measured on 2026-08-22. `clear_debt` (`internal/le/commit`) changed the argument its `GateRunner` receives from the repo root to the throwaway worktree the gate runs in. Four new tests were green, and six existing `TestDebtClear` cases were RED. Four failed because the fixture never committed, so there was no HEAD to materialize. Two were a genuine semantic break: their runners wrote to the ledger THROUGH that argument, and the directory a gate runs in had stopped being the directory the ledger lives in. An author running only their own tests would have shipped it.

Measured again on 2026-08-23, three times in ONE session, on one change. `show bgp rib` moved from a two-level envelope to flat rows. Each time the focused run was green and the pre-push gate was red.

- The tests written FOR the change passed, and `test/plugin/rib-pipe-filter.ci`, the one `.ci` the owner's ruling named, passed once updated. Five other `.ci` files parse that payload and were not run.
- Two `cmd/ze` tests each passed alone under a `-run` filter and panicked together in a package run, on a duplicate root registration.
- `internal/component/lg` was never edited at all, and its `extractRoutes` had a `routes` branch that silently captured the new shape and returned rows unnormalized. The looking-glass graph answered `No routes found`, which reads as a true answer about an empty RIB.

The third is the one this directive is for: no diff-scoped gate reaches a file the change never touched, and the author has no reason to open it.

**A sentence that has been wrong in two OPPOSITE directions is a surface nobody owns, and it MUST be given a test rather than corrected again.** The same commit found one: a shard header claimed the pass judged `over the committed code` while every gate judged the working tree, was corrected to claim the WORKING TREE, and this change made that false in turn. Correcting it a third time buys nothing. Asserting the current claim, with both wrong versions named in the test, is what stops a fourth.

**When a payload SHAPE changes, you MUST search for the NEW name as well as the old one.** Searching what you REMOVE finds code that stops working. It cannot find code that breaks because of what you ADD. An added key CAN land in a branch that already reads it for a different producer. That branch then handles the new payload wrongly and quietly. Nothing prompts this search, because the new name is yours and feels safe.

**For the OLD name, a hit is not yet a consumer: you MUST establish WHICH producer emits the key it matched.** One key name can have several producers, and only some of them are yours. A key name is not a producer.

**A consumer of a GENERIC payload is reached by neither search.** No search for the old key names it, because it never read that key. No list of the changed command's consumers names it either, because it consumes whatever payload it is handed. It is found only by searching the new key.

**The audit is owed AGAIN for the fix, and this is the half a reader will miss.** The natural reading of the rule above is "audit the consumers before you change the shape". But a repair to a shape change is itself a shape change. It lands in a function whose branches each carry a prior contract, so it earns its own pass over both populations.

**A repair that normalizes every element and drops the rest breaks the branches that were passing their elements through.** State each branch's prior contract instead: a branch passes through what it cannot normalize, and the branch that owns the new shape skips what does not carry it.

**The colliding branch does not fail, which is why it stays quiet.** It returns a valid empty result, and an empty answer reads as a true answer about empty state. Refusing would be loud and correct.

**The pre-push gate catches this and the focused tests do not.** A focused run covers the code you edited, and a shape change is defined by who READS it.

## No Throw-Away Tests

**Temporary test code MUST NOT be written.** Add a functional or unit test that
runs in CI.

**A test MUST go in the suite that runs its format.** Each `test/<subdir>/` has
its own runner and format, and they are not interchangeable: `test/parse/`
accepts only config-parse `.ci` files, so a BGP-plugin scenario placed there is
rejected and belongs in `test/plugin/`.
**Pure-logic, reactor-free code (an encoder, a parser, a state machine exercised
directly) MUST be tested in a Go unit test, never in a `.ci` directory.** A `.ci`
exists to prove a user entry point works end to end through the daemon.

## Native Test Actions

**Full rule: `ai/rules/platform-linux.md`** (build tags, virtual substitutes,
native action wiring, reference implementations). MUST read it before writing
any `//go:build linux` code.

**Any change to `//go:build linux` code MUST run this action.**

| Action | What it runs | When required |
|--------|--------------|---------------|
| `./le qemu all-tests` | Registered Linux integration packages in the runtime-kernel guest | Any change to `//go:build linux` code |

**A change matching a "when required" cell MUST run that action.** How the netns
actions obtain and drop their privilege is `docs/functional-tests.md`, "Netns
launch mode".

| Action | What it runs | When required |
|--------|--------------|---------------|
| `./le qemu netns-test suites firewall,policy,ospf,ospfv3` | Kernel-programming functional suites | Changes to nft, FIB, or OSPF kernel programming |
| `./le qemu run command '<focused Go test>'` | A focused capability-dependent package test | Changes to kernel log or other guest-only behavior |

**SHOULD prefer a knob that skips the work over an action that supplies the privilege.**
Use `ze.l2tp.disable-kernel-dataplane=true` when a test asserts only on the CLI
surface and never on the kernel's view. It is the WRONG move whenever the
privileged behaviour is the behaviour under test -- `show system kernel-log`
cannot be freed this way, and neither can
`test/l2tp/session-stopccn-cascade.ci`, which sets `skip-kernel-probe` and still
needs the data plane. `skip-kernel-probe` is a different knob and bypasses only
the modprobe.

**fakeOps pattern:** VPP backends MUST use a `vppOps` interface seam so the Apply
pipeline can be tested with a scripted fake without a running VPP daemon. The
`apply_test.go` files are `//go:build linux` (they import linux-only binapi
types) but do NOT need real VPP. They run in QEMU alongside the integration
tests. See `internal/plugins/traffic/vpp/apply_test.go` for the reference
pattern.

**Every row MUST be covered before a VPP backend merges.**

| Requirement | How |
|-------------|-----|
| Apply/Undo pipeline | `fakeOps` scripted tests in `apply_test.go` covering create, update, delete, partial-failure undo, and reconciliation |
| Translate functions | Pure-function unit tests in `translate_test.go` for every supported config shape |
| Verify/reject logic | `verify_test.go` asserting accepted configs pass and unsupported configs return clear errors |
| Registration side-effects | `register_test.go` confirming `init()` wires the backend into the correct registry |

**"VPP needs a real daemon" MUST NOT be given as a reason to skip a test, and
every VPP backend MUST ship with functional tests.** The `vppOps` interface seam
exists so Apply logic can be tested without VPP, and Translate and Verify are
pure functions with no VPP dependency at all. A new backend that cannot be tested
with the fakeOps pattern is a design problem to fix before merging, never a
deferral to log.

**A `./le verify current mode full` run MUST execute these stages, in order:**

1. **Lint** (full or changed-only depending on target)
2. **Cached full pass** (`go test` without `-race`): Go caches a verdict against the
   files the TEST BINARY OPENED. That is narrower than a source hash, and the
   difference decides whether a mutation proof means anything: a producer the test
   reaches through `exec` is not one of those files, so editing it leaves the verdict
   cached and the tool answers `ok (cached)` for a run that never happened.
   The pass uses `ze_core` plus the default-on feature tags from `feature-gates.txt`,
   matching the shipped `ze_core` feature set. It also runs the bare `ze_core`
   hub compile-out checks so absent-feature tests still execute.
   When nothing changed, this completes in under 1 second. Catches logic regressions
   across the entire codebase.
3. **Race pass on changed groups only** (`go test -race` on component groups containing
   modified `.go` files): catches data races in what you touched, without recompiling
   everything. Group detection uses `internal/le/changed/changed.go`.
4. **Functional tests** (13 suites via `ze-test`)
5. **ExaBGP compatibility**

## Iteration Workflow (BLOCKING)

**One change, one test, then scale.** MUST NOT bulk-modify test files or source files without validating the pattern on a single case first.

**Specific before generic.** For code changes, MUST start with the narrowest test
that can fail because of the changed file: direct Go test, matching `.ci`/`.et`
case, file-level test, feature test, or suite-local command. Then move outward
only after the narrower test passes. MUST NOT spend CPU on unaffected packages or
whole suites before proving the affected code path works.

**A numeric id is a position, not an identity (BLOCKING for anything you keep).**
The runner's one-based ordinal is an internal display position over a sorted
fixture population. Adding or renaming an earlier fixture silently renumbers
later rows. MUST use the stable scenario or Go test name in any verification
command, handover, gate subset, or evidence claim. Why a positional NAME is
stable and a positional number is not is
`docs/architecture/testing/runner-architecture.md`.

**A `ze.log.<subsystem>` key in a `.ci` test MUST name a real slog subsystem.**
An internal plugin's logger name is `CanonicalSubsystemName` of its registry name
(`internal/component/plugin/inprocess.go`), which turns every hyphen into a dot,
and `getLogEnv` (`internal/core/slogutil/slogutil.go`) splits the subsystem on
`.` only. So a plugin registered `bgp-adj-rib-in` reads `ze.log.bgp.adj.rib.in`;
`ze.log.bgp.adj-rib-in` matches no lookup, sets nothing, and leaves the level at
the WARN default -- with no error, which is why it has recurred three times. A
hyphen in the key is legitimate ONLY when that exact subsystem is declared
literally in Go (`slogutil.LazyLogger("bgp.filter.aspath-length")`). Enforced by
`checkLogSubsystemKeys` in `./le doc wiring`.

**Escalation ladder:** direct test -> file/feature test -> single package -> component group -> whole suite or `./le verify current mode full`. If any rung fails, MUST fix from that evidence and rerun the failed rung or a narrower failing test, not a wider suite.

`./le verify worktree` is the **final gate**, not a development tool. Use targeted commands and component groups during iteration.
On failure, `./le verify worktree` writes the compact index `tmp/ze-verify-failures.log`.
Read that file first. The next run MUST be the listed `Rerun` command for the
failed stage, or an even narrower single test/package from the detail log. If
multiple failures are listed, clear each one with its focused rerun. Only after
all focused reruns pass MAY you rerun the whole suite or gate as final
confirmation. The combined log is `tmp/ze-verify.log`, and automation can read
`tmp/ze-verify-failures.json`.

**Overlapping runs:** If a test run is failing, MUST kill it before starting another. MUST NOT run `./le verify worktree` twice concurrently.

**Understand before modifying:** Before bulk-editing `.ci` files or test files, MUST run one test and read its output to understand the format and expected behavior. Assumptions about test syntax cause cascading failures across every modified file.

## Timing Baseline

**Auto-timeout:** Per-test timeout = min(global, max(5s, 5x baseline avg)). A test that normally takes 500ms gets a 5s timeout instead of the default 15s. Catches hangs in seconds, not minutes. Explicit `.ci` `timeout=` overrides MUST win.

**Slow detection:** Tests exceeding 2x baseline are flagged in the summary output. MUST investigate before ignoring.

## Test Tools

- `ze-peer` MAY be used as a BGP test peer through the owning native fixture.
- `ze-test` is the internal functional runner. MUST launch its suites through `./le functional`, which prepares the isolated binaries and environment.

**A new test runner, test format, native action, or verification gate MUST update
its discovery paths in the SAME change**: `ai/INDEX.md` for the tool and for task
navigation when it changes task selection, this rule for required usage, and
`docs/architecture/testing/` or `docs/contributing/` for the operator
documentation. Which page owns what is `ai/rules/repo-maintenance.md`.

## Native Go Tooling

**A native tool MUST be reachable from a `./le` action or a compiled fixture
driver, and its callable Go producer MUST have a test.** A tool nothing invokes
never runs, and it reads as coverage while providing none.

**A native action table MUST have a package test that asserts its exact verb
population and which verbs write.** The bare `./le <area>` answer is the
developer inventory. A verb that disappears from both the table and the listing
otherwise leaves no command to fail.

**A compiled fixture registry MUST have a non-empty population test and reject
duplicate or unknown names.** `internal/test/fixture.Register`, `Names`, and
`Run` hold that contract. An exit code alone cannot distinguish every intended
case passing from no case being registered.

**First-party development or test tooling MUST NOT be added outside
`internal/le`, `internal/test`, or `internal/appliance`, and every command or
fixture MUST be registered in its native Go inventory.** A source file with no
caller is not a tool.

## Temporary Files

**A scratch file MUST go under the project's gitignored `tmp/`, and the system
`/tmp` MUST NOT be used.** A subfolder per debugging task
(`tmp/watchdog-debug/`) isolates artifacts from each other but not from a sibling
session, so it goes under this session's own directory unless the artifact has to
outlive the session.

**MUST write it under your session's own directory**: `dir=$(./le session scratch ensure)`
gives the `scratch/` subdirectory of `tmp/session/<YYYY-MM-DD>-<session-id>/`, so scratch
never collides with a sibling session (`ai/rules/commands.md`). Nothing removes
the live session's directory automatically. `./le session reap` removes only
directories whose owners are provably gone. A fixed name at the `tmp/` root is
the failure this replaces: it names the same file for every session in the
checkout. `bashScratch` and the Write/Edit scratch check in
`internal/le/hookruntime` refuse that path.

## Debugging Failures

Read the failure index before opening full logs or re-running.
After a suite or gate fails, the next test command MUST target
only the failing part: a single `.ci`/`.et` case, single Go test, single
package, or the stage-local `Rerun` command from the failure index. If there
are multiple failures, clear each one with its focused rerun. Only after all
focused reruns pass MAY you rerun the whole suite or gate as final
confirmation, except when the suite is the only available reproduction.

**Each failure group's `Rerun` command MUST be used for the smallest useful
scope, and its `Detail log` MUST NOT be opened before the group is chosen. A
test, verify, or build command MUST NOT be piped through `tail`.**

## Editor Tests (.et format)

`.et` files in `test/editor/` test the interactive TUI editor via headless simulation.
Infrastructure: `internal/component/cli/testing/` (parser, expect, headless, input, runner).
Run `./le functional editor`. Use focused Go tests under the compiled runner when iterating.

## OS-Specific Tests

**A test that cannot run on every OS MUST be gated by the row that fits it, and
its assertion MUST NOT be weakened to accept both outcomes.**

| Situation | Do |
|-----------|-----|
| Whole file is OS-specific | `//go:build linux` on the file |
| One test in a mixed file | `if runtime.GOOS != "linux" { t.Skip(...) }` at the top of that test |
| `.ci` / `.et` test | Split or gate in the runner; do not land an always-failing .ci |

## Reproducing Load-Dependent (Flaky-in-Full-Verify) Failures

**A crash is not the only reproduction.** By default only a CRASH signature
(panic / `DATA RACE` / runtime error) counts, and everything else is discarded
down to the last 500 bytes. An assertion flake (a test whose `expect=` pattern
is merely missed under load) exits non-zero with no crash signature, so
`--any-failure` MUST be passed, or the run reports "not reproduced" while quietly throwing the
evidence away.

**A no-build stress reproduction tests the isolated binary set it was given.** After
changing daemon source, MUST rebuild before trusting a verdict, otherwise a fixed
bug still "reproduces" against the stale binary. Run the owning
`./le functional <suite>` action once; `internal/le/functional.Prepare` rebuilds
the isolated daemon and runner pair.

- **MUST NOT loop `./le functional` / `./le verify worktree` to hunt a flake.**
  MUST use the stress reproducer against the suspected suite.
- **MUST static-clear the hypothesized site before trusting it.** MUST read the function
  that PRODUCES the crash (the reslice, the buffer allocation), not a byte-count
  inference (`ai/rules/evidence.md`). The `rsvpte-lsp` "cap-512
  share-registry" diagnosis in `plan/known-failures/` was inference from the
  5448-byte payload size and did not survive reading the producers: the send
  path is `json.Marshal` + `append` with no 512-cap buffer.
- **If it will not reproduce under stress AND the site is statically clear,**
  SHOULD suspect misattribution (the aggregator tagged another concurrent suite's crash
  to this one) or an already-landed fix, rather than "fixing" a phantom. That is
  the one case a shard MAY record, and only while you are still driving it
  (`ai/rules/completion.md`). It does not apply once you can name load as
  the cause: that is a mechanism, and it gets fixed.
- A genuine reproduction's log (`tmp/stress-repro/…`) carries the real stack:
  MUST attach it when filing or fixing the bug.

## Reactor Concurrency Code (BLOCKING)

**A change to `internal/component/bgp/reactor/session*.go`,
`forward_pool*.go`, `peer.go`, or any other reactor file that holds locks or
shares state across goroutines MUST run
`go test -race -count=20 ./internal/component/bgp/reactor/...` before it is
claimed done.** The standard `-race -count=1` unit run is not enough: a schedule
rare enough to need twenty runs has hidden a reactor race for weeks.

**The verification each row names MUST run before the change is claimed done.**

| Touched | Required verification |
|---------|----------------------|
| `session*.go` lock acquire/release, field assign | `go test -race -count=20 ./internal/component/bgp/reactor/...` |
| `forward_pool*.go` worker drain or buffer release | same repeated reactor race proof |
| New goroutine in reactor package | same repeated reactor race proof |
| Any reactor field shared between Run loop and other goroutines | same repeated reactor race proof |
| Reactor doc-only edits, log message changes | Not required |

**A passing `./le test-unit` MUST NOT be offered as proof that a reactor
concurrency change is race-free.** MUST paste the admitted
`go test -race -count=20 ./internal/component/bgp/reactor/...` output as the
evidence.

## Compiled Observer Failures (BLOCKING)

A compiled observer MUST NOT report an assertion failure only by printing a
line and then returning `nil`. `fixture.Observe` can still request a clean
daemon shutdown, so the daemon exit code does not prove the observer's
assertion. Return an error from the scenario.

**A failing observer MUST return an error.** `fixture.Run` passes it to
`fixture.ReportFailure`, which emits the `ZE-OBSERVER-FAIL` sentinel that
`checkObserverSentinel` in `internal/test/runner/runner_validate.go` detects.

**The left column MUST NOT be written; the right column is what replaces it.**

| Bad | Good |
|-----|------|
| `fmt.Fprintln(os.Stderr, "FAIL: ..."); return nil` | `return errors.New("reason")` |
| Relying on `expect=exit:code=0` to catch observer failures | Return an error and add an explicit assertion on the production result where possible |
| `time.Sleep(N)` then an informational line with no failure path | Use `fixture.Poll`; return an error when it exhausts |

**Equivalent positive assertions also work, and SHOULD be preferred.** The cmd-4 fix took the second
route: it asserted `expect=stderr:pattern=prefix-list accept` plus
`reject=stderr:pattern=prefix-list reject` on production log lines emitted by
`bgp-filter-prefix`. That is the strongest pattern because it verifies the
production code path, not the observer.

**The assertion pattern MUST match the row that describes the case, and
`expect=exit:code=0` alone MUST NOT be relied on.**

| Pattern | When to use |
|---------|------------|
| `expect=stderr:pattern=<production log line>` plus `reject=stderr:pattern=<wrong outcome>` | The plugin emits a decision log on every iteration. Preferred. |
| Return an error from the compiled observer | The observer computes something the engine cannot log directly |
| Rely on `expect=exit:code=0` alone | Forbidden: it does not prove the observer's assertion |

**`internal/test/fixture` owns the failure boundary, and its package tests MUST
prove that an unknown driver is refused and that a returned error reaches
`fixture.ReportFailure`.**

**Sleep ratchet: the total `time.sleep(` count across `test/**/*.ci` MUST NOT
increase.** The committed baseline lives in `test/.ci-sleep-baseline`, and
`./le doc wiring` fails when the count exceeds it. **A payload-predicate wait
MUST be used instead of a sleep**, because a sleep hides a real race:
`fixture.Poll` around `fixture.Dispatch` in a compiled observer, or a
`wait_until` / `dispatch_until` engine step. **A change that removes sleeps MUST
lower the baseline in the same change.**

## CI Sleep Justification

1. A blind sleep hides real races. A reader cannot tell whether it is safe (a
   bounded poll interval that already blocks on a real condition) or a guessed
   duration that will flake under load.
2. When a sleep is deliberately left un-converted, the reason (deliberate timer,
   a Linux-only effect verifiable only under QEMU, an effect with no queryable
   readiness signal) is knowledge that MUST live next to the code, not in a
   reviewer's head.

The sleep MUST be converted to a deterministic wait with `fixture.Poll`,
`fixture.Dispatch`, or an engine-step predicate from the Compiled Observer API
whenever a condition exists to wait on. Only when no such condition exists
does the sleep stay, and then it MUST be justified. See "Try a sync primitive
before you write a sleep" below for what trying means and how the comment records it.

**A `time.Sleep` MUST NOT be written until a synchronization primitive has been
tried and found not to fit.** Compiled fixtures use `fixture.Poll`,
`fixture.Dispatch`, SDK readiness callbacks, contexts, and runner engine-step
predicates. Peer-specific helpers poll counters such as `eor-sent` when a
scenario depends on bytes reaching the wire. A duration is what a test writes
when it cannot name the condition, so naming that condition is the work.

**The comment MUST declare which kind the sleep is, in the marker form
`// sleep(<kind>): <reason>`.** The kinds are the closed set the table above
names: `poll-interval`, `timer`, `timeout-under-test`, `needs-linux`,
`no-signal`.
A free-text `// settle` comment is insufficient because it names no mechanism a
reader can check.

**A `timer`, `timeout-under-test` or `no-signal` reason MUST name the mechanism
and where its period is set.** "The tracker pushes live carrier once a second"
is a reason a later reader can check and overturn. "Needs a moment" is not, and
it is the shape that turns a deliberate timer and a guessed duration into the
same line of code.

`fixture.Poll` takes a predicate, so it converts any state readback the fixture
can perform into a bounded wait. Its own timer is the synchronization helper,
not a delay before an unrelated assertion.

**Every sleep in a `.ci` test MUST carry its justification marker, in the form
`// sleep(<kind>): <reason>`.** Two producers enforce it: `./le doc wiring` at
gate time, and the Write/Edit hook at edit time. The closed set of kinds, what
each reason owes, and where the comment goes are
`docs/architecture/testing/ci-format.md`.

- The external-plugin refuse/warn sleeps wait on a reject-fence signal the daemon does not emit, so no deterministic wait exists for them yet and they MUST NOT be converted.

## Compiled Observer API (`internal/test/fixture`)

**SHOULD wait for route-server readiness through a compiled payload predicate.**
Poll the pushed state or `eor-sent` counters before shutdown. Do not replace the
barrier with a guessed delay or a synchronous one-shot query.

**`fixture.Poll` around `fixture.Dispatch` is the compiled payload-predicate
wait, and it SHOULD be used in place of a `time.Sleep` followed by a one-shot
assertion.** The test then blocks until the observed payload matches, within a
bounded attempt count. The API is `docs/architecture/testing/ci-format.md`.

## Mutation Testing

**A test that cannot fail is worse than no test, because it also spends the
attention that would have found the gap.** So before a test is offered as
evidence for a requirement, MUST establish that it would go red if the behavior
it names stopped happening. Reading the assertion is not that establishment: an
assertion can be true for a reason unrelated to the code under test.

**Five shapes recur, and each is invisible to a gate that checks only whether a
test EXISTS.** MUST check for them by name:

- A value asserted into memory that already held it. Any assertion that
  something is zero, empty, or absent is suspect when the buffer or structure
  was freshly allocated: deleting the code that writes the value changes
  nothing.
- The happy branch alone, where the justification for a missing case cites the
  very branch no test enters. When an annotation explains why one polarity is
  absent, that explanation names the case most worth writing.
- A property strictly weaker than the one the requirement states. "Not a
  constant" is not "unpredictable"; "present" is not "correct"; "non-empty" is
  not "complete". A test can only assert what it can observe, so where the
  requirement's own word is unobservable, MUST assert the structural fact the
  requirement depends on and say that is what is proven.
- One clause of a requirement that states two, with the tag claiming both. A
  requirement joined by "and" needs both halves asserted or the tag overstates.
- A negative confounded by a guard that fires first. When the input crafted to
  violate one rule also violates an earlier one, the earlier rule does the
  failing and the rule under test is never reached.

**Mutation is the cheap answer and MUST be preferred to argument.** Revert the
behavior in a throwaway copy and run the test. A verdict reached that way costs
one edit and one run, and it is the only kind that survives someone reading the
test differently later. Where a test and the code it checks share an
implementation, the mutation MUST change them together: a test that recomputes
the answer the same way agrees with any error the implementation makes.

**When a test is repaired this way, its coverage MUST rise rather than move.**
A rewrite that pins the new contract is worth more than a deletion, and it keeps
the requirement proven while the contract changes underneath it.

**Proving a test discriminates means breaking the mechanism on purpose, and in a
shared checkout that mutation MUST land in a file this session owns.** Every
other session reads the same working tree. A file under mutation is a file that
is deliberately wrong, and for as long as the window lasts anyone who builds,
lints or runs a suite gets that wrongness as their answer.

**Restoring from a snapshot does not make the window safe.** The snapshot holds
what the file looked like when it was taken, so an edit another session makes
inside the window is overwritten by the restore. The mutation is visible; the
loss is silent, and it lands in somebody else's work.

**When the only mutation point is a shared file, say so rather than taking the
window.** A discrimination claim that cannot be made safely is reported as not
made, with the file named and the reason given. That is a smaller cost than a
lost edit nobody can attribute, and `ai/rules/never-destroy-work.md` already
settles which of the two is acceptable.

**A build file, a manifest and a generated artifact are shared by default**, and
so is any source file another session's uncommitted work touches. Check before
mutating, not after restoring.

**A discrimination proof MUST state whether its re-run actually ran.** "It went
red when I mutated it" is not a sufficient claim. The report says which mutation
was applied, and whether the re-run was a real execution or a cached verdict.

The reason is mechanical rather than a matter of care. `go test` keys a cached
verdict on the files the TEST BINARY OPENED. A producer the test reaches through
`exec`, a compiler it invokes, or an interpreter it shells out to is not one of
those files, so mutating it changes no cache key and the tool answers `ok
(cached)` for a run that never happened. The standard proof, mutate the producer
then re-run and observe red, degrades silently into mutate the producer, re-run
nothing, observe the old green.

Which category a proof falls into decides what it owes:

- A mutation to PACKAGE SOURCE changes the cache key. Nothing further is owed.
- A mutation to a producer the test EXECS does not. Defeat the cache with
  `-count=1`, or drive the producer through a runner that keeps no Go cache, and
  say which was done.
- A `.ci`, `.et`, `.wb` or Docker run has no Go result cache at all. Say so rather
  than applying the caveat where it cannot apply.

The tell in the output is a bare `ok` with no duration. A real run reports one.

**A real re-run is only half of it. You MUST also verify that the MUTATION
APPLIED, between the patch and the run, with a diff that MUST come back non-empty
or a grep for the mutated text.** A patch that fails to apply leaves the test
running against unmodified source, so it passes, and the artifact of that attempt
is byte-identical to a successful proof. This is the worse half: a stale cached
verdict at least ran once against real code, while an unapplied mutation means
nothing was ever tested.

**Restore the file by copying back a pristine copy you saved first. You MUST NOT
reach for `git checkout --`, `git restore` or `git stash`: they are banned
outright, and a mutation proof is the moment the reflex to use one is strongest.**
Save the copy before the patch, restore with `cp`, and confirm the file is
byte-identical afterwards. In a shared checkout those verbs would discard another
session's uncommitted work in the same file, and the ban does not soften inside a
throwaway worktree, where the habit is formed and carried back out.

It defeats the habit that catches every other shape. Break it and watch it go red
fails silently when the break never landed, and the report then says truthfully
that the re-run was real while the proof is worth nothing. Everybody already
saves a copy of the file and restores it by hash, because the interesting moment
feels like the run. Confirming the change landed costs one command, and it is the
only thing that makes the run mean anything.

**Applying `-count=1` everywhere is not the answer and MUST NOT be treated as
one.** It disables the cache for every test in the run, and a gate that already
costs tens of minutes pays that in full. The obligation is to know which category
a proof is in, not to spend the cache to avoid thinking about it.

## Pre-Commit

**A commit owes NO verification pass (`ai/rules/pre-release.md`): the full gate
is owed before a PUSH. What a commit owes is the focused test for what it
changed, run once.**
**That focused test MUST run through a native action** -- `./le job run label
unit-pkg command go test PKG=<what you are changing>`, a component group
(`./le test-unit bgp`), or `./le test-unit`. **A bare `go test` MUST NOT be used
in its place**: `internal/le/gotoolchain.Toolchain` gives native actions the
repository build cache and the feature tags, and a shell run has neither.
