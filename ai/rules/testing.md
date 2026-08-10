# Testing

**When:** writing, changing, or deleting any test, and before writing implementation code for new behavior
**Severity:** blocking
**Related:** completion, platform-linux, rfc-compliance

## Directives

Rationale: `ai/rationale/testing.md`, `ai/rationale/tdd.md`, `ai/rationale/no-test-deletion.md`
Structural template: `ai/patterns/functional-test.md`

- **Tests MUST exist and fail before implementation.**
- **Every user-facing behavior MUST have a functional test that exercises it through a user entry point. Unit tests (`_test.go`) prove internal logic. Functional tests (`.ci`, `.et`) prove the feature works end-to-end through the daemon. Both are required. Neither substitutes for the other.**
- **A red test means the CODE is wrong by default. MUST diagnose the failure and fix the source. MUST NOT weaken the test to make it green. MUST ask the user before deleting OR weakening any test code (`*_test.go`, `.ci`, `Test*`, `t.Run`, assertions, table entries). Exception: the user already explicitly requested it.**
- **A test that cannot run on every OS MUST either carry a build tag (`//go:build linux`) on its file, or skip (`t.Skip`) with a reason on the OSes where it cannot run. MUST NOT weaken the assertion to accept both outcomes.**
- **Every `time.sleep(` call in a `.ci` test MUST have an explanatory comment on the line directly above it, or trailing it on the same line. A bare sleep with no comment is rejected.**
- **A load-dependent failure is DIAGNOSED, and the outcome is always a fix. Load-dependence is the diagnosis (the test asserts on elapsed time instead of on state), and `ai/rules/completion.md` bans recording it as a `plan/known-failures/` shard, bans "passes in isolation" as a conclusion, and bans raising the timeout. MUST reproduce it with the stress reproducer, then fix the timing assumption.**

## Test-Driven Development

### Cycle

1. Write test with `VALIDATES:` and `PREVENTS:` comments
2. Run → MUST FAIL (paste output)
3. Minimum implementation
4. Run → MUST PASS (paste output)
5. Refactor while green

### RFC-Enforcing Tests

A test that enforces an RFC obligation carries a third tag beside `VALIDATES:`/`PREVENTS:`:

```go
// RFC requirement: RFC7606-7.1-1 negative: ORIGIN length != 1 is treated as withdraw.
```

One id per line, polarity mandatory (`positive`/`negative`), placed INLINE at the table
case when one function covers many requirements. `make ze-rfc-check` binds it to
`rfc/short/*.md`; the tag is the only authored half, so it dies with the test. Once
tagged, the test may not change behavior without user approval (see "RFC-Tagged Tests"
below). Full rules: `ai/skills/ze-rfc.md`.

### Patterns

- **Table-driven:** `tests := []struct{...}` with `t.Run(tt.name, ...)`
- **Round-trip:** `original → packed → unpacked == original`
- **Fuzz (REQUIRED for wire format):** All external input parsing
- **Non-default params:** MUST test with non-default/non-zero values

### Boundary Testing (MANDATORY)

All numeric ranges MUST test: last valid, first invalid below, first invalid above.

| Range | Last Valid | Invalid Below | Invalid Above |
|-------|------------|---------------|---------------|
| Port 1-65535 | 65535 | 0 | 65536 |
| Hold time 0,3+ | 0, 3 | 1, 2 | N/A |
| Prefix IPv4 0-32 | 32 | N/A | 33 |
| Message len 19-4096 | 4096 | 18 | 4097 |

### Coverage

| Code Type | Target |
|-----------|--------|
| Wire format (pack/unpack) | 90%+ |
| Public functions | 100% |
| Error paths | 100% |

### AC-Linked Tests (BLOCKING)

Every AC-N MUST have a test whose assertion directly verifies the AC's **expected behavior**, not just the mechanism used to achieve it.

| AC text says | Test MUST assert | Test MUST NOT assert |
|-------------|-----------------|---------------------|
| "rejected" / "not installed" | Route is absent from delivery / RIB | No error returned (mechanism) |
| "session torn down" | Connection closed + NOTIFICATION sent | NOTIFICATION struct returned (mechanism) |
| "warning logged" | Log entry exists (or counter incremented) | No teardown (absence of something) |
| "rejected at parse time" | Error returned with specific message | Generic error returned |

**The test:** MUST quote the AC expected behavior in the `VALIDATES:` comment. MUST read the test assertion. Does it verify that exact behavior? If the assertion would still pass with a stub implementation that does nothing, the test is invalid.

**Red flag:** A test MUST NOT assert the ABSENCE of an action ("no NOTIFICATION", "no error") as proof that a DIFFERENT action happened ("routes rejected"). Absence of X does not prove Y.

### Rules (test-first)

- If you debug something, MUST add a test so it's never re-investigated
- Implementation before test exists → MUST delete impl, write test
- Test passes immediately → invalid test, MUST add failing assertion
- Claiming "done" without test output → MUST run it, paste it

## Draft a Functional Test Before It Is Live (BLOCKING)

Never write or iterate on a `.ci` inside `test/<suite>/`, and never edit a live
one in place. That directory runs on every `make ze-verify` in the checkout,
including runs by OTHER sessions, who then have to work out whether your
half-written test is their regression.

| Step | Command |
|------|---------|
| Write it in the incubator | `test/draft/<suite>/<name>.ci` |
| Run only drafts | `ze-test <suite> --draft -a` |
| Prove it under load | `scripts/dev/stress-repro.py "<suite> --draft" --test <id> --any-failure` |
| Promote when green | `mv test/draft/<suite>/<name>.ci test/<suite>/` |

`test/draft/` is gitignored and skipped by every repo-wide gate, so a draft
cannot redden anything for anyone. Changing an existing test is the same move:
copy it into the incubator, work there, `mv` it back. Full workflow: the
`/ze-test` skill, `test/draft/README.md`, `docs/functional-tests.md`.

**A draft is not a test yet, so the draft workflow MUST end in exactly two moves:
promote it into `test/<suite>/`, or delete it.** Leaving it in the incubator is
the third move, and it is the one that is refused. A draft proves no obligation,
claims no evidence, and appears in no coverage ledger, so a session that finds
one cannot tell abandoned scaffolding from work in progress.

Because a draft is not a test, the guards that protect tests do not protect
drafts, and deleting one needs no approval. `check_test_deletion`
(`.claude/hooks/pretool-bash.py`) exempts a command whose every named test path
is the incubator or sits under it. A test path is one carrying a `test/` segment
or a `_test.go` name, so a Go test counts. `c_test_weakening`
(`.claude/hooks/pretool-writeedit.py`) returns before both its weakening
heuristic and its RFC-tag branch for a file there. A command that mixes a draft
with a live test still blocks: the live one is the reason those guards exist.

An `RFC requirement:` tag inside a draft is worth nothing until the file is
live, which is why the tag guard skips it. Promoting the file is what turns the
tag into proof.

Nothing in the incubator is gated, so promote early: the accept-only check, the
`time.sleep(` ratchet, and frame-length validation only start applying once the
file is live.

## Test Code Is Held to One Standard

- **Test code is held to ONE standard: it MUST run, and it MUST be correct about the product.** The coverage targets above are for the code that ships. A test helper, a fixture builder, a `.ci` or `.et` script and the runners under `test/` need no coverage figure, no boundary sweep, and no test of their own. Spend that budget on the behavior under test, which is the only thing an operator ever meets.
- **A bug in test code that leads to NO TESTING is load-bearing, and it is fixed like product code.** A test the runner never selects, a skip that reports green, a harness that never reaches the code under test, a fixture that builds the wrong scenario, an assertion nothing evaluates: the suite claims coverage it does not have, and that claim is what the product is shipped on.
- **What else still applies is everything that decides what a test PROVES: it fails when the behavior breaks, it asserts the acceptance criterion rather than the mechanism, it never encodes a violation, and a gate still refuses what it exists to refuse.** Those are the sections around this one, and none of them is softened here. A defect in test-only code outside that set is a NOTE in review (`ai/rules/planning.md`, "Critical Review Is the Central Deliverable"), never a spec.
- **A tool that already carries tests keeps them.** The wired Python conventions below, and the Go tests over `scripts/dev`, exist because a gate that stops refusing is a product-visible failure. This point removes an obligation to ADD coverage over harness code; it removes no test that is there.

## Fix Code, Not Tests

When a test fails, fix the code to make the test pass. NEVER weaken or simplify test expectations to match broken code. Tests are ground truth. Even if an underlying mechanism changed (e.g., Unix sockets replaced by SSH), the test expectations stay and the replacement mechanism must satisfy them.

NEVER modify test data (golden files, expected output, fixtures, `.ci` expectations) to make a failing test pass without explicit user authorization. When output changes, the default assumption is that the code is wrong, not the test data. Ask the user before updating any test data, even if the new output looks plausible.

## Test Deletion and Weakening

**Legitimate reasons a test MAY be deleted:** testing removed functionality, duplicating another test, fundamentally wrong, replacing with better coverage.
**Reasons that MUST NOT justify deletion:** failing and hard to fix, slow, "annoying", don't understand what it checks.

### Mechanically blocked (c_test_weakening in pretool-writeedit.py)

Blocked on Edit / Write / MultiEdit to a test file (exit 2):

**These weakenings MUST NOT be introduced:**

- adding `t.Skip` / `t.Skipf` / `t.SkipNow` (the test stops running)
- removing assertions (any net drop, not only all-removed)
- downgrading fatal assertions to non-fatal (`require` -> `assert`, `t.Fatal` -> `t.Error`)
- commenting out assertions
- adding an `ignore` build tag (file dropped from the build)
- deleting a `Test`/`Fuzz`/`Benchmark` func, `t.Run` cases, or table rows
- removing `.ci` test lines

**Not detected (by design, to avoid false positives):** changing an expected
value in place while the assertion structure stays (e.g. `Equal(t, 1, x)` ->
`Equal(t, 2, x)`). This is the one weakening the hook cannot see; MUST treat it with
the same discipline manually. Adjusting an expected value to match broken code is
the same violation as removing the assertion.

### Test Rewrite as Replacement (BLOCKING)

When fixing a new issue that happens to touch an area with existing tests, ADD a
new test case or function for the new issue. Do not repurpose an existing test to
cover the new behavior. The old test verified a behavior that still needs coverage.

| Scenario | Correct | Wrong |
|----------|---------|-------|
| New bug in `parsePeer`, existing `TestParsePeer` | Add `TestParsePeerRejectsEmpty` alongside `TestParsePeer` | Rewrite `TestParsePeer` to test the new edge case |
| Table-driven test, new case needed | Add a row to the table | Replace an existing row with the new case |
| Existing test fails because code changed | Fix the code so both old test and new test pass | Rewrite the old test to match the changed (broken) code |

**Why the hook cannot catch this:** the rewrite maintains the same structural
shape (same function count, same assertion count), so the mechanical check sees
no weakening. The coverage loss is semantic, not structural, so a passing
structural check MUST NOT be read as proof that nothing was weakened.

**Detection:** `/ze-review` step 0 (`audit-test-relaxation.py`) flags structural
changes. For semantic replacement, `/ze-review` step 7 (removed-behavior audit)
MUST verify that every assertion the diff replaces still has coverage elsewhere.
When reviewing a test edit that changes WHAT is asserted (not just adding new
assertions), ask: "is the old behavior still tested?"

### Escape hatch (auditable)

When relaxation IS legitimate, document the reason on or above the changed line:

In the comment syntax the file itself uses: `//` in a `.go` test, `#` in a `.ci` or
`.et` scenario. Both are accepted on a `#` carrier, so the `# // test-relax:` already
in 315 scenarios keeps working.

```
// test-relax: <why this test/assertion no longer applies>
# test-relax: <why this test/assertion no longer applies>
```

The token unblocks the edit and leaves an audit trail. Review all relaxations with:
`git grep -n 'test-relax:' -- '*_test.go' '*.ci' '*.et'`. It must be `git grep`: plain
`grep` reads those globs as filenames after `--` and reports nothing, which looks
exactly like no relaxations. Using the token without a real reason is a violation.

## Functional Test Gate

### The Rule

When you add or change user-facing behavior, a corresponding functional test MUST
exist in the correct `test/` directory. "User-facing" means: reachable via CLI command,
config option, API call, web endpoint, plugin event, or wire protocol exchange.

### Required Test Type by Change

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

If the change does not fit any row (pure internal refactor, no user-visible effect),
no functional test is required. But if you are unsure, write one.

### When Unit Tests Alone Are Sufficient

Unit tests (`_test.go`) without a functional test are acceptable ONLY when:

| Condition | Example |
|-----------|---------|
| Pure internal logic with no user entry point | Helper function, data structure, algorithm |
| Existing functional test already covers the path | Bug fix where the `.ci` test already exercises the scenario |
| Wire encoding internals tested via round-trip | `pack -> unpack == original` in `_test.go`, AND a `.ci` encode test covers the message type |

In all other cases, both unit tests AND a functional test are required.

### Mechanical Check (MANDATORY before claiming done)

For every new or changed user-facing behavior in the diff:

```
# 1. Identify the feature's test directory from the table above
# 2. Check for a functional test covering the behavior
find test/<directory>/ -name "*.ci" -o -name "*.et" | xargs grep -l '<feature-keyword>'
```

If no functional test exists for a user-facing behavior, that is a BLOCKER.

### Mutation-Verify the Test Actually Gates (MANDATORY for behavior-guarding tests)

A functional test that EXISTS is not the same as one that GATES. A `.ci`/`.et` can
pass whether or not the feature works (a **false-pass**) when the observed effect
reaches the assertion by a path OTHER than the one under test. Real example: three
`redistribute-late-join*.ci` tests kept passing with the late-join replay
(`handleReplayBatch`) disabled, so the route reached the peer by some path other than
the replay: they guarded nothing and shipped green.

For every NEW or CHANGED `.ci`/`.et` that is meant to guard a SPECIFIC behavior:

1. Disable the producing function (the code the test exists to prove): an early
   `return`, a no-op, or `if true { return }` at the top of the function.
2. Re-run the test. It MUST flip to RED. If it still passes, the test does not gate
   on the feature: find the alternate delivery path and design it out (inject with no
   peers, remove the fallback store, use a genuinely-new peer instead of a reconnect),
   or the test is worthless: delete it, do not ship it.
3. Revert the mutation immediately and confirm the test is green again.

This is a MANUAL discipline. `make ze-mutation-test` / `ze-mutation-changed` (gomu,
see "Mutation Testing" below) mutates Go source and runs only `go test` UNIT tests: it
never executes `.ci`/`.et`, so it cannot catch a functional false-pass. Nothing else in
the pipeline does either.

If a test genuinely cannot be made to fail under mutation because the behavior is not
observable end-to-end (e.g. the reactor suppresses a duplicate announce, so per-peer
targeting is wire-indistinguishable), guard it with a UNIT test that inspects the
producing value directly, and say so in the test comment. Do NOT keep a `.ci` that
passes with the feature disabled.

### Common Violations

| Pattern | Why it's wrong |
|---------|----------------|
| "Unit tests cover this" | Unit tests prove the function works in isolation. They do not prove the daemon exposes the feature to users. |
| "The wiring test passes" | Wiring proves reachability. Functional tests prove correct behavior through the full path. |
| "The `.ci` is green" | A test that passes with the feature DISABLED (false-pass) guards nothing. Mutation-verify it: disable the producing function, confirm the test goes red. |
| "I'll add the .ci test later" | Later never comes. The feature ships without end-to-end coverage. |
| "The behavior is too simple to need a functional test" | Simple behaviors break when config parsing, CLI dispatch, or plugin registration changes. The functional test catches that. |
| "There's no test infrastructure for this path" | Build the infrastructure or flag it as blocked. Do not skip the test. |

### Relationship to Other Rules

- `completion.md` requirement #2: both unit AND functional tests MUST exist per AC
- `completion.md` checks that code is reachable; this rule checks that the reachable path is tested end-to-end
- "Test-Driven Development" (above) governs the test-first cycle; this section governs test completeness at the feature level
- "No Throw-Away Tests" (below) has the directory table and iteration workflow; this section makes the directory mapping a gate

## RFC-Tagged Tests (BLOCKING)

A test carrying an `RFC requirement: <id> <polarity>` tag is the proof behind a public
compliance claim in `docs/features/rfc-status.md`, and `make ze-rfc-check` counts it as
that proof. Editing it to match the code retires the evidence while the claim stays up.

| Situation | Do |
|-----------|-----|
| A tagged test fails after your change | Fix YOUR code. The test is the requirement |
| You believe the test is genuinely wrong | STOP. Show the user the RFC text beside the test and ask. Do not edit first and explain after |
| The summary misquotes the RFC | Fix `rfc/short/rfcNNNN.md` (keep the id), then re-run `/ze-rfc-audit` |
| Reformat / comment / re-tag | Allowed; behavior must be unchanged |
| You added, moved, deleted, or re-tagged a tagged test (or an edit shifted its line) | Run `make ze-rfc-index` and commit `ai/RFC-REQUIREMENTS.md` in the SAME commit. The ledger records each test's `file:line`, and `ze-rfc-check` (both verify modes) fails on a stale ledger, so a skipped regen lands on the next session as a cross-commit diff |

**Where a tag MAY live, and what it is worth: four carriers, declared once in `CARRIERS` (`scripts/dev/rfc_requirements.py`) and derived by the scanner, the HEAD baseline, the ledger and the ratchets. Evidence has two axes: KIND (which layer the test exercises) and TIER (whether anything executes it).**

| Carrier | Cell in the ledger | Executed by | Tier |
|---------|--------------------|-------------|------|
| `*_test.go` | `unit/verify` | `make ze-unit-test` | runs on every push |
| `*.ci` | `functional/verify` | `make ze-functional-test` | runs on every push, but ONLY from a suite that target actually runs: the tier is derived per-suite from `mk/test-functional.mk`'s own `all_suites=` line, so a `.ci` in a suite outside it (traffic, vrrp, ipsec, flow-export, static, vpp, chaos) earns no verify tier, and `test/draft/` is skipped entirely |
| `*.et` | `editor/verify` | `make ze-editor-test` | runs on every push, on the same earned-per-suite basis as `*.ci` |
| `test/interop/scenarios/*/check.py` | `interop/nightly` | `make ze-interop-test` | scheduled, ADVISORY |
| `test/ipsec-interop/scenarios/*/check.py` | `interop/nightly` | `make ze-ipsec-interop-test` | scheduled, ADVISORY |

- **SHOULD prefer a `.ci` over an interop binding** when a behavior is reachable from both: a `.ci` runs inside `ze-verify` on every push, interop does not (owner decision, umbrella D3).
- A requirement whose ONLY evidence is nightly-tier is marked `**nightly-only**` on its ledger row and counted in its own rollup column: it is not merge-gate-proven, and the rollup deliberately never sums the two.
- **An interop tier is DERIVED; it MUST NOT be declared.** A tree earns `interop/nightly` when a SCHEDULED workflow under `.github/workflows/` names its runner, which `scheduled_workflow_targets()` reads. So adding the job IS the whole fix and `CARRIERS` needs no edit, and deleting the job takes the tier away again rather than leaving a stale claim behind (`ai/rules/evidence.md`).
- **A tag in `test/l2tp-interop/`, `test/pppoe-interop/`, or any other `check.py` tree is REFUSED** with an error naming the file, because no scheduled workflow runs those suites and a tag nothing executes is an absence of evidence rather than weak evidence. The l2tp and pppoe labs need host kernel modules (`l2tp_ppp`, `pppoe`, `/dev/ppp`) that no runner is yet confirmed to provide, so the sequence stays wire, observe one green run, then the tier follows on its own.
- **A QEMU sibling is not that pipeline.** `ze-qemu-l2tp-ppp-test` and `ze-qemu-pppoe-accel-test` run `scripts/evidence/effective-*.py`, never the trees' `check.py`, so they execute no tagged carrier and cannot justify a tier for one.
- **Non-unit evidence is monotonic, per requirement and per tier.** Replacing a `.ci` binding with a unit tag, or with a nightly interop tag, fails `make ze-rfc-check`, and no annotation satisfies it.
- A `check.py` is TOKENIZED, not line-scanned: a `#` inside a docstring or string literal is not a comment and is not a tag, and an untokenizable `check.py` fails the scan closed.

`// test-relax:` does **not** authorize changing a tagged test: it is your own
justification, not the user's approval. Enforced by the `rfc-tagged-test` hook, which runs
before `test-weakening` precisely so the relax token cannot pre-empt it
(`ai/rules/repo-maintenance.md`). Once the USER approves, record what they approved:
`// rfc-test-change-approved: <date> <what and why>`.

Every gated requirement needs BOTH a positive and a negative test. A negative-only test
passes if the code rejects everything; a positive-only test passes if it accepts
everything. Only the pair pins behavior to the requirement. Assert the EXACT outcome, never
a floor: `GreaterOrEqual(TreatAsWithdraw)` is also satisfied by `SessionReset`, so it cannot
fail when the implementation over-reacts. See `ai/skills/ze-rfc.md`.

## Back-Fill New Test Types (BLOCKING)

When you introduce a new test type, technique, or infrastructure (fuzz target, property test, mutation gate, `-race` sweep, clock-injection audit, new `.ci`/`.et` category, QEMU harness), apply it to the existing code it covers, not only to the code added alongside it. Coverage that grows only forward from the introduction date is the trap (`plan/learned/RECURRING-PATTERNS.md`, "New test type added but not back-filled to existing code").

In the same work:

1. MUST name the applicable set: the package glob, symbol kind, or call-site pattern the new test type is meant to cover.
2. MUST back-fill that set, or record the uncovered remainder as explicit, tracked backlog (spec, handoff, or deferral table). MUST NOT leave it implicit.
3. SHOULD prefer a grep- or registry-driven audit that enumerates every applicable site over per-file judgement. `/ze-hunt` enumerates sites for grep-detectable patterns.

## Test Sensitivity Ratchets (BLOCKING)

A test that cannot fail, and a test file no target builds, both read as coverage
while providing none. Neither is visible in any count of tests, which is how the
published totals grew for years without either being noticed.

**No gate catches a test that re-implements the logic it names, and none is
planned.** Such a test builds the production algorithm again inside itself, in a
local variable, then asserts on its own copy. It DOES assert, so the
assert-nothing detector never sees it, and its name and `VALIDATES:` comment read
as coverage of the real thing. It is green against every implementation, the
correct one and the broken one alike.

Three of them stood in `internal/component/bgp/reactor/peer_test.go` until
2026-08-09, each building a local `familiesSent` map for RFC 4724 End-of-RIB
family tracking. `familiesSent` existed in no production code. One session read
them and was about to escalate a conformance violation that does not exist.

The tell is mechanical and it is the only one there is: **the test names a
function it never calls.** Before writing a test, name the function under test
and check the body calls it. When reviewing one, read what the assertion reads
from -- a local the test itself filled is the defect, whatever the test is
called. Widening the sensitivity ratchet to this class was tried and rejected:
every table-driven test builds local fixtures, so the detector would fire on
hundreds of correct tests, and a noisy detector gets switched off.

`make ze-test-sensitivity-check` (stage 10 of `make ze-verify`, both modes) counts
them and enforces committed floors in `test/health/sensitivity-baseline.json`. The
counts may only go DOWN, following the `test/.ci-sleep-baseline` convention.

| Detector | Fires when | Fix |
|----------|-----------|-----|
| assert-nothing | A `Test*` function has no reachable `Error`/`Fatal`/`Fail` call, no assertion-library call, no compile-time `var _ T = ...` assertion, and no `panic` | Add a real assertion, or annotate: `// test-asserts-nothing: <why the oracle is implicit>` |
| tag-orphan | A `_test.go` build constraint needs a `ze_*` tag that no `go test -tags` in `Makefile` or `mk/*.mk` supplies | Add the tag to a `go test` invocation, or delete the file |

Benchmarks and fuzz targets are deliberately exempt: a benchmark measures, and a
fuzz target delegates its oracle to the engine. Raising a floor is forbidden:
`make ze-test-health` only lowers one, so a regression cannot be laundered into
the baseline by regenerating.

`docs/features/test-health.md` (generated, `make ze-test-health`) reports these
alongside RFC proof density, mutation kill rate, negative-test ratio, and
technique adoption by package age. Read it before claiming the suite is healthy:
it is the answer to "would a regression be caught", which no test count gives.
Details: `test/health/README.md`.

| Target | Enforces | Notes |
|--------|----------|-------|
| `make ze-test-sensitivity-check` | The two ratchets, read from the tree | Stage 10 of `ze-verify`, both modes. Independent of the report |
| `make ze-test-health-check` | STRUCTURAL facts only: orphaned test files, unproven RFCs, metric statuses | Inside `ze-regen-check-readonly`. Volume counters are published, not gated, so adding a test does not force a regeneration |

The split is deliberate. Byte-gating the whole report charged a
regenerate-and-commit to ~60% of commits, and a check that fires that often for
cosmetic reasons gets routed around instead of read: the same "advisory gate
permanently red" failure the report is built to expose.

## No Throw-Away Tests

Never write temporary test code. Add functional or unit tests that run in CI.

| Situation | Location | Format |
|-----------|----------|--------|
| Valid config parses | `test/parse/` | `.ci` with `expect=exit:code=0` |
| Invalid config fails | `test/parse/` | `.ci` with `expect=exit:code=1` + `expect=stderr:contains=` |
| BGP encoding | `test/encode/` | Config + expectations |
| Plugin behavior | `test/plugin/` | Config + expectations |
| Wire decoding | `test/decode/` | stdin + cmd + `expect=json:` |
| Editor/TUI behavior | `test/editor/` | `.et` with `input=`/`expect=` directives |
| Internal logic | `internal/<pkg>/<file>_test.go` | Go test file |

Each `test/<subdir>/` has its own runner and format, and they are not interchangeable. `test/parse/` only accepts config-parse `.ci` files (config text + `expect=exit:code=`). Putting a BGP-plugin scenario there will be rejected; put it in `test/plugin/`. Pure-logic, reactor-free code (encoders, parsers, state machines exercised directly) belongs in Go unit tests (`internal/<pkg>/<file>_test.go`), not in any `.ci` directory: `.ci` tests exist to prove a user entry point works end-to-end through the daemon.

## Make Targets

### Component-Group Unit Tests

Test one logical area during development instead of all 349 packages:

| Target | Scope | Approx time |
|--------|-------|-------------|
| `make ze-test-bgp` | `./internal/component/bgp/...` (96 pkgs) | ~1:30 |
| `make ze-test-core` | `./internal/core/...` (26 pkgs) | ~30s |
| `make ze-test-plugins` | `./internal/plugins/...` (44 pkgs) | ~40s |
| `make ze-test-config` | `./internal/component/config/...` (13 pkgs) | ~20s |
| `make ze-test-cli` | `./internal/component/cli/...` (3 pkgs) | ~10s |
| `make ze-test-rest` | Everything not in a named group (~70 pkgs) | ~1:00 |
| `make ze-test-pkg PKG=<pattern>` | ONE package, or any pattern. `RUN=<regexp>` narrows, `RACE=0` drops `-race` while iterating | seconds |

All groups run with `-race`. Use the group matching your change during iteration.

### Verification Targets

| Target | Purpose |
|--------|---------|
| `make ze-verify` | Pre-commit gate: lint, changed-file wiring/doc/inventory, vet evidence, two-pass unit, functional, and ExaBGP |
| `make ze-verify-changed` | Changed-package lint/test plus wiring/doc/inventory, functional, and ExaBGP |
| `make ze-verify-wiring-docs` | Changed-file-aware wiring, documentation, command, and inventory gate |
| `make ze-unit-test` | All unit tests with `-race` under default-on feature tags, plus bare `ze_core` compile-out checks (~5 min) |
| `make ze-functional-test` | All 13 functional test suites |
| `make ze-lint` | 26 linters |
| `make ze-ci` | lint + unit + build |
| `make ze-fuzz-test` | Fuzz tests (10s per target) |
| `make ze-exabgp-test` | ExaBGP compatibility via `ze-test exabgp --all` |
| `make ze-test` | All tests including fuzz |
| `make ze-editor-test` | Editor `.et` tests (headless TUI) |
| `make ze-chaos-test` | Chaos unit + functional + integration + web |
| `make ze-race-reactor` | Stress race-test reactor (`-race -count=20`) -- REQUIRED when touching reactor concurrency code |
| `make ze-mutation-test` | Mutation testing via gomu on all non-excluded packages (advisory, slow) |
| `make ze-mutation-changed` | Incremental mutation testing on changed files only (advisory, fast) |
| `make ze-mutation-report` | Mutation testing with HTML report to `tmp/mutation-report.html` |
| `make ze-test-sensitivity-check` | Assert-nothing and tag-orphan ratchets (in `ze-verify`, both modes) |
| `make ze-test-health` | Regenerate `docs/features/test-health.md` + `test/health/latest.json` |
| `make ze-test-health-record` | Append one KPI sample to `test/health/history.ndjson` |

### Contended Run Verdicts

When `make ze-verify` runs on a loaded machine, the failure index may show
`VERIFY FAILURE INDEX (CONTENDED RUN)` with host load details. This means the
system had load > CPU count with concurrent ze-test or go-test processes.

How to read contended failures:
- `near_timeout` kind: the test consumed >80% of its timeout but the context
  deadline did not fire. This is CPU starvation, not a bug. Rerun on a quiet
  machine.
- `host-load` field in failure group JSON: load average, CPU count, and
  concurrent process counts at run start.
- Timing baseline updates are suppressed during contended runs to prevent
  slow-run pollution of the EMA.
- The project rejects retry-on-failure masking. Contended verdicts are for
  classification, not automatic retry.
<!-- source: internal/test/runner/hostload.go -- HostLoad, Contended, IsNearTimeout -->

### Linux-Only Tests (QEMU)

**Full rule: `ai/rules/platform-linux.md`** (build tags, virtual substitutes,
Makefile wiring, reference implementations). MUST read it before writing any
`//go:build linux` code.

| Target | What it runs | When required |
|--------|-------------|---------------|
| `make ze-qemu-integration-test` | iface, config/system, fib/kernel, firewall/nft, firewall/vpp, traffic/netlink in QEMU Alpine VM | Any change to `//go:build linux` code |

### Capability-Requiring `.ci` Tests (Linux host, per-test netns)

| Target | What it runs | When required |
|--------|-------------|---------------|
| `make ze-netns-test` | `firewall` `policy` `ospf` `ospfv3` suites under `ZE_TEST_NETNS=1` | Any change to nft/FIB/OSPF kernel programming |
| `make ze-netns-plugin-test` | `show-system-kernel-log`, which needs CAP_SYSLOG to read `/dev/kmsg` | Any change to `readKmsg` |

Both setcap a **throwaway** binary, run under `sudo` with a per-test network
namespace, assert the host's kernel state is byte-identical before and after,
and exit non-zero (never skip) when Linux, `sudo`, or `setcap` is missing.
Details: `docs/functional-tests.md` "Netns launch mode".

**SHOULD prefer a knob that skips the work over a target that supplies the privilege.**
Five L2TP `test/plugin` tests used to sit in the second target; they now set
`ze.l2tp.disable-kernel-dataplane=true`, build no kernel worker, and pass
unprivileged. That was right because each asserts on the CLI surface and never on
the kernel's view, so nothing was lost. It is the WRONG move whenever the
privileged behaviour is the behaviour under test -- `show system kernel-log`
cannot be freed this way, and neither can
`test/l2tp/session-stopccn-cascade.ci`, which sets `skip-kernel-probe` and still
needs the data plane. Note those are two DIFFERENT knobs:
`skip-kernel-probe` bypasses the modprobe only.
<!-- source: mk/test-integration.mk -- ze-netns-test, ze-netns-plugin-test -->

**fakeOps pattern:** VPP backends MUST use a `vppOps` interface seam so the Apply
pipeline can be tested with a scripted fake without a running VPP daemon. The
`apply_test.go` files are `//go:build linux` (they import linux-only binapi
types) but do NOT need real VPP. They run in QEMU alongside the integration
tests. See `internal/plugins/traffic/vpp/apply_test.go` for the reference
pattern.

### VPP Backend Testing Is Mandatory (BLOCKING)

Every VPP backend must ship with functional tests. No exceptions, no deferrals.

| Requirement | How |
|-------------|-----|
| Apply/Undo pipeline | `fakeOps` scripted tests in `apply_test.go` covering create, update, delete, partial-failure undo, and reconciliation |
| Translate functions | Pure-function unit tests in `translate_test.go` for every supported config shape |
| Verify/reject logic | `verify_test.go` asserting accepted configs pass and unsupported configs return clear errors |
| Registration side-effects | `register_test.go` confirming `init()` wires the backend into the correct registry |

"VPP needs a real daemon" is not a valid reason to skip tests. The `vppOps`
interface seam exists precisely so Apply logic can be tested without VPP.
Translate and Verify are pure functions with no VPP dependency at all. If a
new backend cannot be tested with the fakeOps pattern, that is a design
problem to fix before merging, not a deferral to log.

### Two-Pass Verification (how `ze-verify` works)

`ze-verify` uses a two-pass strategy to avoid recompiling all 349 packages with
`-race` every time:

**A `ze-verify` run MUST execute these stages, in order:**

1. **Lint** (full or changed-only depending on target)
2. **Cached full pass** (`go test` without `-race`): Go caches results by source hash.
   The pass uses `ze_core` plus the default-on feature tags from `feature-gates.txt`,
   matching the shipped `make ze` feature set. It also runs the bare `ze_core`
   hub compile-out checks so absent-feature tests still execute.
   When nothing changed, this completes in under 1 second. Catches logic regressions
   across the entire codebase.
3. **Race pass on changed groups only** (`go test -race` on component groups containing
   modified `.go` files): catches data races in what you touched, without recompiling
   everything. Group detection uses `scripts/dev/changed-groups.sh`.
4. **Functional tests** (13 suites via `ze-test`)
5. **ExaBGP compatibility**

Recorded full passes (`tmp/.ze-verify-duration.txt`) run 4-10 minutes;
a one-group change sits at the low end. Budget 10 minutes, not 2.

## Iteration Workflow (BLOCKING)

**One change, one test, then scale.** MUST NOT bulk-modify test files or source files without validating the pattern on a single case first.

**Specific before generic.** For code changes, MUST start with the narrowest test
that can fail because of the changed file: direct Go test, matching `.ci`/`.et`
case, file-level test, feature test, or suite-local command. Then move outward
only after the narrower test passes. MUST NOT spend CPU on unaffected packages or
whole suites before proving the affected code path works.

If a changed file has an associated test file, feature test, or suite test, run
that first. After it passes, run the next broader relevant scope, then the
remaining gate. Order is: direct test -> file/feature test -> package ->
component group -> whole suite or `ze-verify`.

| Step | Action | Command |
|------|--------|---------|
| 1 | Make the change in ONE file | Edit a single `.ci` or `.go` file |
| 2 | Run just that test | `ze-test bgp plugin N` or `go test -run TestName` |
| 3 | Investigate if it fails | Read output, understand the format, fix |
| 4 | Only then apply to remaining files | Repeat the pattern that worked |

**SHOULD use targeted test commands for development:**

| Scope | Command | Speed |
|-------|---------|-------|
| Single functional test | `ze-test bgp plugin N` or `ze-test ui N` | seconds |
| Resume functional suite | `ze-test bgp plugin --start N` or `ze-test ui --start N` | seconds to remaining suite |
| Single encode test | `ze-test bgp encode N` | seconds |
| Single editor test | `ze-test editor N` or `ze-test editor --pattern <name>` | seconds |
| Single ExaBGP compatibility test | `ze-test exabgp N` or `ze-test exabgp --start N` | seconds |
| Single Go test | `make ze-test-pkg PKG=./pkg/... RUN=TestName` | seconds |
| Single package | `make ze-test-pkg PKG=./internal/component/bgp/reactor/` | seconds |
| Component group | `make ze-test-bgp` (or core, plugins, config, cli, rest) | 10s-1:30 |
| All unit tests | `make ze-unit-test` | ~5 min |
| All editor tests | `make ze-editor-test` | ~30s |
| Pre-commit gate | `make ze-verify` | 4-10 min (see `tmp/.ze-verify-duration.txt`) |

**A numeric id is a position, not an identity (BLOCKING for anything you keep).**
`ze-test <suite> N` resolves `N` as a one-based ordinal over the sorted `.ci` glob,
so adding, renaming, or deleting an EARLIER file silently renumbers every test
after it. Ids are fine while you iterate inside one turn. They MUST NOT persist in
anything that outlives the turn -- a verification script, a gate subset, a
handover, a commit message claiming "8/8 green". A concurrent session added `.ci`
files mid-session and id 373 moved from `resolve-ping` to
`remove-private-as-replace-peer` while an id-driven script reported green for
tests it never ran.

| Use | Form |
|-----|------|
| Iterating right now, from a failure index you just read | `ze-test bgp plugin 145` |
| A script, a gate subset, a handover, a claim of evidence | `ze-test bgp plugin --pattern <name>` |

A positional selector matches a record's Nick, Name, or CIFile EXACTLY
(`indexRecordSelector`, `internal/test/runner/selection.go`), so passing names as
positional ids is as stable as `--pattern` and, unlike a substring pattern,
cannot widen. `scripts/evidence/netns_qemu.py` selects all four of its subsets by
name for exactly this reason, and its `assert_named` guard refuses to run a
subset that still carries a numeric selector -- a nick had already drifted there,
with firewall `"17"` resolving to `command-owner-firewall-root.ci` rather than to
any `017-*.ci`.

**MUST spell `--pattern` in full: `-p` is a DIFFERENT flag in most suites.** `-p` is `--parallel` (an int) for `ze-test bgp <type>`, `ze-test exabgp`, `ze-test vpp` and every `.ci` suite on the shared runner (`internal/test/cli/cmd_bgp.go`, `cmd_exabgp.go`, `cmd_vpp.go`, `ci_runner.go`), and `--pattern` (a string) only for `ze-test editor` and `ze-test web` (`cmd_editor.go`, `cmd_web.go`); `--pattern` itself has no short form anywhere. So `ze-test bgp plugin -p rfc7606-relay-one-field` is not a filtered run, it is a parse failure -- exit 2, no output, no tests -- and it reads as "nothing to report" rather than as an error.

**A `ze.log.<subsystem>` key in a `.ci` test MUST name a real slog subsystem.**
An internal plugin's logger name is `CanonicalSubsystemName` of its registry name
(`internal/component/plugin/inprocess.go`), which turns every hyphen into a dot,
and `getLogEnv` (`internal/core/slogutil/slogutil.go`) splits the subsystem on
`.` only. So a plugin registered `bgp-adj-rib-in` reads `ze.log.bgp.adj.rib.in`;
`ze.log.bgp.adj-rib-in` matches no lookup, sets nothing, and leaves the level at
the WARN default -- with no error, which is why it has recurred three times. A
hyphen in the key is legitimate ONLY when that exact subsystem is declared
literally in Go (`slogutil.LazyLogger("bgp.filter.aspath-length")`). Enforced by
`check_ci_log_subsystem_keys` in `make ze-verify-wiring-docs`.

**Escalation ladder:** direct test -> file/feature test -> single package -> component group -> whole suite or `ze-verify`. If any rung fails, MUST fix from that evidence and rerun the failed rung or a narrower failing test, not a wider suite.

`make ze-verify` is the **final gate**, not a development tool. Use targeted commands and component groups during iteration.
On failure, `make ze-verify` writes the compact index `tmp/ze-verify-failures.log`.
Read that file first. The next run MUST be the listed `Rerun` command for the
failed stage, or an even narrower single test/package from the detail log. If
multiple failures are listed, clear each one with its focused rerun. Only after
all focused reruns pass may you rerun the whole suite or gate as final
confirmation. The combined log is `tmp/ze-verify.log`, and automation can read
`tmp/ze-verify-failures.json`.

**Overlapping runs:** If a test run is failing, MUST kill it before starting another. MUST NOT run `make ze-verify` twice concurrently.

**Understand before modifying:** Before bulk-editing `.ci` files or test files, MUST run one test and read its output to understand the format and expected behavior. Assumptions about test syntax cause cascading failures across every modified file.

## Individual Commands

```bash
go test -race ./internal/component/bgp/message/... -v  # Single package
go test -race ./... -run TestName -v          # Single test
go test -race -cover ./...                    # Coverage
make ze-fuzz-one FUZZ=FuzzName TIME=30s       # Single fuzz target
```

## Timing Baseline

`ze-test` saves per-test timing to `tmp/test-timings.json` (rolling EMA, alpha=0.3).
After 3 samples, the baseline is used for two things:

**Auto-timeout:** Per-test timeout = min(global, max(5s, 5x baseline avg)). A test that normally takes 500ms gets a 5s timeout instead of the default 15s. Catches hangs in seconds, not minutes. Explicit `.ci` `timeout=` overrides MUST win.

**Slow detection:** Tests exceeding 2x baseline are flagged in the summary output. MUST investigate before ignoring.

## Test Tools

- `ze-peer` MAY be used as a BGP test peer (`--sink`, `--echo`, `--port`, `--asn`)
- `ze-test`: Test runner. Common suite syntax is `--list`, `--all`, `--start N`, `--pattern TEXT`, or positional `N...`; `--list` prints `N/TOTAL id name` with one-based ids, and runs print one completion line per test plus periodic progress.

When adding a test runner, test format, make target, or verification gate, update
`ai/rules/repo-maintenance.md` paths in the same change: `ai/INDEX.md` for the
tool, `ai/INDEX.md` (task navigation) if it changes task selection, this file for required
usage, and `docs/architecture/testing/` or `docs/contributing/` for detailed
operator documentation.

## Testing Python Tooling (scripts/)

There is no `pytest` and no `unittest discover` in this repo. A Python test that
nothing invokes never runs, and reads as coverage while providing none. Eight
`scripts/dev/*_test.py` files sat unexecuted this way until 2026-07-16. Use one of
the two wired conventions, never a bare test file plus hope:

| Your tool | Convention | Runs because |
|-----------|-----------|--------------|
| Has its own unit tests | Name them `<tool>_test.py` (unittest, with `unittest.main()`) and put them BESIDE the tool -- `scripts/dev/`, `test/scripts/`, or `test/perf/` | `TestPythonUnitTests` (`scripts/dev/python_tests_test.go`) globs `*_test.py` under EVERY root in `pythonTestRoots` and runs each. A new file in an existing root is picked up automatically; a Python tool in a NEW directory needs its root added there first, or its tests never run. Each root carries its own non-empty assertion so a root that stops contributing fails loudly rather than silently covering nothing |
| Wants fixture tests inside the script | Add a `--selftest` flag, then a small Go test that shells out to it | The pattern of `dep_audit.py`, `migrate_module.py`, `qemu-run.py`. See `scripts/dev/migrate_module_test.go` |

Both land inside `go test`, so `make ze-unit-test` covers them via `go list ./...`
and no make target is needed. `scripts/dev` and `scripts/evidence` are test-only Go
packages that exist for exactly this.

Do not add a `*_test.py` outside a directory covered by one of the above without
wiring it, and do not "fix" a discovery glob by replacing it with a hardcoded list:
the glob is what stops the next file from rotting.

## Temporary Files

Use project `tmp/` (gitignored) for scratch files, never `/tmp`.
A subfolder per debugging task (`tmp/watchdog-debug/`) isolates artifacts from each
other, but not from a sibling session: put it under your session's own directory
(below), unless the artifact must outlive the session.

**MUST write it under your session's own directory**: `dir=$(scripts/dev/session-scratch.sh)`
gives the `scratch/` subdirectory of `tmp/session/<YYYY-MM-DD>-<session-id>/`, so scratch
never collides with a sibling session (`ai/rules/commands.md`). Nothing removes it for you:
the date in the directory name is what lets the operator find it later, with
`make ze-clean-sessions BEFORE=<YYYY-MM-DD>`. A fixed name at
the `tmp/` ROOT is the failure this replaces: `tmp/` is keyed per checkout, so
`tmp/out.log` names the same file for every session in it.
A file at that root is refused: `check_scratch_path`
(`.claude/hooks/pretool-bash.py`) on a redirect, `c_scratch_path_we`
(`.claude/hooks/pretool-writeedit.py`) on Write and Edit.

The functional-test runner already writes there: its per-run and per-test working
directories (configs, sockets, daemon pid/ready files) root at
`sessionpath.DefaultScratchRoot()` / `EnsureScratchRoot(baseDir)` when a session is
active, instead of the unowned `$TMPDIR/ze-functional-*` they used before
(`internal/test/sessionpath`, `internal/test/runner/runner.go`). Off-session the
runner still uses the system temp dir, unchanged.

## Debugging Failures

Read the failure index before opening full logs or re-running.
After a suite or gate fails, the next test command MUST target
only the failing part: a single `.ci`/`.et` case, single Go test, single
package, or the stage-local `Rerun` command from the failure index. If there
are multiple failures, clear each one with its focused rerun. Only after all
focused reruns pass may you rerun the whole suite or gate as final
confirmation, except when the suite is the only available reproduction.

```bash
make ze-verify
# On failure, read:
tmp/ze-verify-failures.log
```

Use each group's `Rerun` command for the smallest useful scope. Open the
group's `Detail log` only after choosing the group. On success: one final pass
line plus fresh artifacts. Never `| tail`.

## Editor Tests (.et format)

`.et` files in `test/editor/` test the interactive TUI editor via headless simulation.
Infrastructure: `internal/component/cli/testing/` (parser, expect, headless, input, runner).
Run: `make ze-editor-test` or `bin/ze-test editor --all`; select by id/name with `bin/ze-test editor N`, and filter with `bin/ze-test editor --pattern <name>`.

### Directives

| Directive | Purpose | Example |
|-----------|---------|---------|
| `tmpfs=<path>:terminator=<TERM>` | Embedded config file | `tmpfs=test.conf:terminator=EOF` |
| `option=file:path=<name>` | Config file to load (required) | `option=file:path=test.conf` |
| `option=timeout:value=<dur>` | Test timeout (default 30s) | `option=timeout:value=10s` |
| `option=width:value=N` | Editor width (default 80) | `option=width:value=120` |
| `option=height:value=N` | Editor height (default 24) | `option=height:value=30` |
| `option=reload:mode=success\|fail` | Mock reload notifier | `option=reload:mode=success` |
| `option=monitor:ping=fake` | Deterministic ping monitor + fake PTR/origin resolvers (offline pipe-enrichment tests; see `internal/component/cli/testing/fake_monitor.go`) | `option=monitor:ping=fake` |
| `option=session:user=X:origin=Y` | Session identity | `option=session:user=alice:origin=ssh` |
| `session=<name>` | Switch to named session | `session=bob` |
| `input=type:text=<string>` | Type text | `input=type:text=show` |
| `input=<keyname>` | Press key | `input=enter`, `input=tab`, `input=up` |
| `input=ctrl:key=<char>` | Ctrl+key | `input=ctrl:key=c` |

**Named keys an `.et` test MAY press:** `tab`, `enter`, `esc`, `up`, `down`, `left`, `right`, `backspace`, `delete`, `home`, `end`, `pgup`, `pgdn`, `space`, `shift+tab`

### Expectations

| Type | Example | What it checks |
|------|---------|----------------|
| `expect=input:value=<text>` | `expect=input:value=show` | Text input buffer |
| `expect=input:empty` | | Input is empty |
| `expect=context:root` | | At root context |
| `expect=context:path=bgp.peer` | | At nested context |
| `expect=dirty:true\|false` | | Unsaved changes |
| `expect=error:none\|contains=<text>` | | Command error state |
| `expect=status:contains=<text>\|empty` | | Status message |
| `expect=mode:is=config\|operational` | | Editor mode |
| `expect=completion:contains=a,b` | | Tab completions include items |
| `expect=completion:empty\|count=N\|exact=a,b` | | Completion list state |
| `expect=ghost:text=<text>\|empty` | | Ghost text preview |
| `expect=content:contains=<text>` | | Config content |
| `expect=viewport:contains=<text>` | | Displayed output |
| `expect=dropdown:visible\|hidden` | | Dropdown shown |
| `expect=file:path=<rel>:contains=<text>` | | On-disk file content |
| `expect=file:path=<rel>:absent` | | File does not exist |
| `expect=timer:active\|inactive` | | Commit confirm timer |
| `expect=errors:count=N\|contains=<text>` | | Validation errors |
| `expect=warnings:count=N\|contains=<text>` | | Validation warnings |
| `expect=prompt:contains=<text>` | | Prompt text |

### When to use .et vs .ci vs Go tests

| Test need | Format | Why |
|-----------|--------|-----|
| TUI behavior (keystrokes, completions, history) | `.et` | Headless model simulates real TUI |
| BGP wire, config parsing, CLI commands | `.ci` | Process-level testing |
| Internal logic, persistence wiring | Go `_test.go` | Direct API access |

### Structure

Tests organized by concern in `test/editor/`: `commands/`, `completion/`, `lifecycle/`, `mode/`, `navigation/`, `pipe/`, `session/`, `validation/`, `workflow/`.

## OS-Specific Tests

| Situation | Do |
|-----------|-----|
| Whole file is OS-specific | `//go:build linux` on the file |
| One test in a mixed file | `if runtime.GOOS != "linux" { t.Skip(...) }` at the top of that test |
| `.ci` / `.et` test | Split or gate in the runner; do not land an always-failing .ci |

A darwin `FAIL` caused by a `_other.go` stub returning `ErrUnsupported`
is a test-setup bug, not a real failure. Keep the failure list
meaningful.

## Common Flaky Test Causes

| Symptom | Root Cause | Fix |
|---------|-----------|-----|
| Port reuse race in reactor tests | `Stop()` not waiting for cleanup | Ensure cleanup goroutines complete before returning |
| Completion test fails intermittently | Real bug, not flaky | Check `completeShowPath` includes YANG schema children |
| Inter-message timing in plugin tests | Sleep too tight under load | Increase inter-message delay or use synchronization |

Seven flake shapes have been seen here: locked-write with unlocked-read,
subscribe-before-broadcast, gate-handler queue state, barrier FIFO order,
cleanup-drains-work, a fixed port behind an SO_REUSEPORT gate, and colliding
test-fake pool IDs. Check each one against your test before investigating a new
race or isolation flake.

## Reproducing Load-Dependent (Flaky-in-Full-Verify) Failures

Some failures only surface under the scheduling and GC pressure of the full
~22-suite run (many concurrent `ze` daemons on all cores). Rerunning the single
suite never triggers them, and looping the full suite to hunt the bug is
impractical (minutes per run, low hit rate). The verify aggregator also
truncates the crashing daemon's goroutine stack to ~2 lines, so the crash site
is usually lost.

### Use the stress reproducer, not the full suite

`scripts/dev/stress-repro.py <suite>` recreates that pressure cheaply: CPU + GC
"burner" processes oversubscribe every core while many concurrent copies of one
suite loop, and it captures the FIRST failure's complete, untruncated output.

```
python3 scripts/dev/stress-repro.py rsvpte --iterations 80         # hunt + capture the stack
python3 scripts/dev/stress-repro.py rsvpte --race                  # data race self-reports its two accesses
python3 scripts/dev/stress-repro.py bgp --burners 32 --parallel 8  # more pressure
python3 scripts/dev/stress-repro.py "bgp plugin" --test 97 --any-failure  # sub-suite, one test, assertion flake
```

`<suite>` and `--test` are both split on whitespace, so a sub-suite and a
multi-token selector reach `ze-test` exactly as you would type them by hand.

**A crash is not the only reproduction.** By default only a CRASH signature
(panic / `DATA RACE` / runtime error) counts, and everything else is discarded
down to the last 500 bytes. An assertion flake (a test whose `expect=` pattern
is merely missed under load) exits non-zero with no crash signature, so
`--any-failure` MUST be passed, or the run reports "not reproduced" while quietly throwing the
evidence away.

It sets `GOTRACEBACK=all` so a panic dumps every goroutine (the one racing on
the corrupt buffer shows up next to the crasher), reuses the prebuilt
`bin/ze`/`bin/ze-test` via `ze.bin` + `ZE_TEST_NO_BUILD` (no rebuilds under
load), and writes the full capture to `tmp/stress-repro/<slug>-<ts>.log`. Exit
0 = reproduced, 1 = not reproduced, 2 = setup error.

**`ZE_TEST_NO_BUILD=1` means the run tests whatever `bin/ze` already is.** After
changing daemon source, MUST rebuild before trusting a verdict, otherwise a fixed
bug still "reproduces" against the stale binary. `bin/ze-test <suite> <test>`
once (no `ZE_TEST_NO_BUILD`) rebuilds both binaries.

### Rules (stress reproduction)

- **MUST NOT loop `make ze-functional-test` / `make ze-verify` to hunt a flake.**
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

When touching `internal/component/bgp/reactor/session*.go`, `forward_pool*.go`,
`peer.go`, or any other reactor file that holds locks or shares state across
goroutines, the standard `-race -count=1` unit run is **not enough**. The
bufReader/bufWriter races (`d5843235`, `8dffd422`) lived 47 days because the
schedule that triggered them was rare. Run `make ze-race-reactor` (`-race
-count=20`) before claiming the change done.

| Touched | Required verification |
|---------|----------------------|
| `session*.go` lock acquire/release, field assign | `make ze-race-reactor` |
| `forward_pool*.go` worker drain or buffer release | `make ze-race-reactor` |
| New goroutine in reactor package | `make ze-race-reactor` |
| Any reactor field shared between Run loop and other goroutines | `make ze-race-reactor` |
| Reactor doc-only edits, log message changes | Not required |

A passing `ze-unit-test` is NOT proof that a reactor concurrency change is
race-free. Paste the `ze-race-reactor` output as evidence.

## Observer-Exit Antipattern in `.ci` Tests (BLOCKING)

Python observer plugins inside `tmpfs=*.run` blocks MUST NOT use the
`dispatch(api, 'daemon shutdown') ; sys.exit(1)` pattern to signal failure.
The runner only watches ze's exit code, and ze has already exited 0 from the
clean shutdown by the time the observer's `sys.exit(1)` runs. The test passes
silently. The cmd-4 fix (`1fc98747`) removed three such false-positives.

**MUST use `runtime_fail` instead.** `test/scripts/ze_api.py` provides
`runtime_fail(message)` which emits the `ZE-OBSERVER-FAIL` sentinel that the
runner detects via `validateLogging` (`internal/test/runner/runner_validate.go`).

| Bad | Good |
|-----|------|
| `print('FAIL: ...', file=sys.stderr); sys.exit(1)` | `from ze_api import runtime_fail; runtime_fail('reason')` |
| Relying on `expect=exit:code=0` to catch observer failures | Adding explicit `expect=stderr:pattern=` on production logs the plugin emits |
| `time.sleep(N)` then "INFO: filter not called" with no failure path | `runtime_fail` if the expected event did not arrive |

**Equivalent positive assertions also work, and SHOULD be preferred.** The cmd-4 fix took the second
route: it asserted `expect=stderr:pattern=prefix-list accept` plus
`reject=stderr:pattern=prefix-list reject` on production log lines emitted by
`bgp-filter-prefix`. That is the strongest pattern because it verifies the
production code path, not the observer.

| Pattern | When to use |
|---------|------------|
| `expect=stderr:pattern=<production log line>` + `reject=stderr:pattern=<wrong outcome>` | Plugin emits a decision log on every iteration. **Preferred.** |
| `runtime_fail(...)` from observer when assertion fails | Observer must compute something the engine cannot log directly |
| Rely on `expect=exit:code=0` alone with a Python observer | Forbidden -- silent false positive |

Detection hook: `c_observer_sys_exit` in `.claude/hooks/pretool-writeedit.py`
(warns on Write/Edit of `.ci` files containing `tmpfs=*.run` Python with
`sys.exit(1)` and no `runtime_fail`).

**Sleep ratchet (BLOCKING):** the total `time.sleep(` count across
`test/**/*.ci` MUST NOT increase. The committed baseline lives in
`test/.ci-sleep-baseline`; `make ze-verify-wiring-docs` fails when the count
exceeds it. Use `ze_api` `wait_for_event` / `wait_for_shutdown` / `wait_until` /
`dispatch_until` (the payload-predicate waits, below) instead of sleeps (sleeps
hide real races). When your change removes sleeps, lower the baseline in the same
change. Known violations are tracked in `plan/known-failures/`
and MUST be migrated.

Every sleep the ratchet tolerates must also be justified: see "CI Sleep
Justification" below.

## CI Sleep Justification

This is the qualitative companion to the **Sleep ratchet** (above), which caps how
MANY sleeps exist: this section caps how many are unexplained. Two reasons:

1. A blind sleep hides real races. A reader cannot tell whether it is safe (a
   bounded poll interval that already blocks on a real condition) or a guessed
   duration that will flake under load.
2. When a sleep is deliberately left un-converted, the reason (deliberate timer,
   a Linux-only effect verifiable only under QEMU, an effect with no queryable
   readiness signal) is knowledge that MUST live next to the code, not in a
   reviewer's head.

Prefer converting the sleep to a deterministic wait
(`ze_api` `wait_until` / `wait_for_event` / `dispatch_until`, see
"Python Observer API" below). Only when conversion is not possible
does the sleep stay, and then it MUST be justified.

### What counts as justified

The comment must state which of these the sleep is:

| Kind | What the comment should say |
|------|-----------------------------|
| Bounded poll interval | Name the real condition the enclosing loop breaks/returns on ("poll interval; the loop above breaks when the nft table appears"). This is already a deterministic wait; the sleep is only its granularity. |
| Deliberate timer | The delay itself IS the behaviour under test ("the 3s verify hold IS the concurrency race window; do NOT convert"). |
| Timeout under test | The sleep waits out a fixed internal timeout that the test asserts ("the 5s vpp WaitConnected timeout IS the behaviour under test"). |
| needs-linux effect | A dataplane effect (tc/qdisc/nft/kernel FIB) with no readback in the driver, convertible only after a QEMU run ("needs-linux; no queryable signal that the qdisc was programmed"). |
| No readiness signal | The awaited effect exposes no queryable state to this driver ("backgrounded ze gets no ZE_READY_FILE marker; hold until OnConfigure emits the asserted log line"). |

Placement (mechanical): one `#` comment line directly above the sleep, indented to
match the sleep exactly (these are Python heredocs; wrong indentation is a syntax
error). No em dashes in the comment text.

### Enforcement

- **Blocking gate:** `check_ci_sleep_justification` in
  `scripts/dev/verify_wiring_docs.py`, run by `make ze-verify-wiring-docs` (and the
  inventory make gate). Scoped to CHANGED `.ci` files: a session MUST justify
  the sleeps in the tests it touches. Fails (exit 1) listing every unjustified
  `file:line`.
- **Edit-time nudge:** `c_ci_sleep_justification` in
  `.claude/hooks/pretool-writeedit.py` warns (non-blocking) when a Write/Edit of a
  `.ci` introduces a `time.sleep(` with no comment on the line above or trailing it.

### Related (CI sleep)

- `plan/spec-fixit-redistribute-establishment-stall.md` -- the redistribute establishment sleeps MUST NOT be converted until this P0 spec lands.
- The external-plugin refuse/warn sleeps wait on a reject-fence signal the daemon does not emit, so no deterministic wait exists for them yet.

## Python Observer API (`test/scripts/ze_api.py`)

Python plugins embedded in `.ci` tests via `tmpfs=*.py` can import `ze_api` for
the 5-stage plugin protocol and runtime assertions. Key functions:

| Function | Purpose |
|----------|---------|
| `ready()` | Complete all 5 stages, enter event loop (simple usage) |
| `send(cmd)` | Send a text command to the engine |
| `dispatch(api, cmd)` | Send command via API connection |
| `runtime_fail(msg)` | Signal assertion failure (replaces `sys.exit(1)`) |
| `wait_for_shutdown()` | Block until engine shuts down |
| `wait_for_event(timeout, predicate=None)` | Wait for the next event, or (with `predicate`) the first event whose decoded form satisfies it |
| `wait_until(predicate, attempts=20, delay=0.25)` | Poll an arbitrary `predicate()` (e.g. kernel FIB state) until true; returns bool |
| `dispatch_until(api, cmd, predicate, ...)` | Re-dispatch `cmd` until `predicate(result)` is true; returns the winning result dict (also `api.dispatch_until(cmd, predicate, ...)`) |
| `dispatch_until_done(cmd, ...)` | `dispatch_until` with the fixed `status=="done"` predicate |
| `run_rs_observer(expected_peers, forward_prefix=None)` | The standard route-server observer, one line: handshake, wait (event-driven) until every peer's EOR (and `forward_prefix`'s route, when given) is on the wire, then fire-and-forget shutdown. Load-robust successor to the `show bgp summary` `eor-sent` poll |
| `wait_rs_replayed(expected_peers, forward_prefix=None)` | The readiness half of `run_rs_observer`: block on the async event stream until N EORs (and optionally a route carrying `forward_prefix`) are sent. Returns bool |
| `shutdown_fire_and_forget()` | Send `request shutdown` without blocking on its RPC response (ze may close the connection before replying under load) |

**SHOULD prefer `run_rs_observer` for any route-server `.ci`.** The old copy-pasted
`all_peers_eor_sent` poll drove synchronous `show bgp summary` dispatch RPCs whose
30s TLS read could stall under load while the engine forwarded fine, stranding the
shutdown until the outer timeout killed ze. `run_rs_observer` waits on pushed events
instead (no request/response to stall on) and shuts down fire-and-forget.

`wait_until` / `dispatch_until` / `wait_for_event(predicate)` are the
payload-predicate waits: prefer them over `time.sleep` + a single-shot assert so
a test blocks exactly until the observed payload matches, not a guessed duration.

Full protocol usage: `API()` class with `declare_family()`, `declare_done()`,
`wait_for_config()`, `capability_done()`, `wait_for_registry()`, `ready()`.

Source: `test/scripts/ze_api.py` (docstring has examples).
<!-- source: test/scripts/ze_api.py -- wait_until, dispatch_until, dispatch_until_done, wait_for_event -->

First-class `.ci` engine steps have the symmetric declarative form
(`expect=output:matches=`/`absent=`/`json=`); see
`docs/architecture/testing/ci-format.md` "Engine Steps".

## Mutation Testing

Mutation testing uses [gomu](https://github.com/sivchari/gomu) to verify that
tests actually catch code changes. It modifies the AST (arithmetic, conditional,
logical, bitwise, branch, return value, error handling operators) and checks
whether the test suite detects each mutation. Advisory only, never gates
`ze-verify`.

gomu is vendored in `tools.go` and invoked via `go run`. No install needed.

| Target | Purpose |
|--------|---------|
| `make ze-mutation-test` | Full run on all non-excluded packages (slow) |
| `make ze-mutation-changed` | Incremental, changed files only (fast) |
| `make ze-mutation-report` | Full run with HTML report to `tmp/mutation-report.html` |

Tuning via environment: `GOMU_WORKERS` (default: `GO_TEST_PROCS`),
`GOMU_TIMEOUT` (default: 120s per test), `GOMU_THRESHOLD` (default: 0%).

gomu has no `--tags` support. Files with custom build tags (`ze_test`,
`ze_chaos`, `ze_perf`, `ze_analyze`) and `cmd/ze/` are excluded via
`.gomuignore`. Reports go to `tmp/` (gitignored).

## Pre-Commit

See `ai/rules/git-safety.md` for the full pre-commit workflow.

`make ze-verify` is the ONLY acceptable pre-commit verification. Not `go test`. Not any subset.
During development: `make ze-test-pkg PKG=<what you are changing>`, component groups
(`make ze-test-bgp`), and `make ze-unit-test` are fine for fast iteration. A BARE `go test`
is not: the Makefile exports `GOCACHE` to `cache/go-cache` into its own recipes only, so a
shell run uses `~/.cache/go-build`, rebuilds cold, and shares nothing with `ze-verify`. It
also drops the feature tags, which is the separate lie recorded above.
