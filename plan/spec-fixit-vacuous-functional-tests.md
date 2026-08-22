# Spec: fixit-vacuous-functional-tests -- two `.ci` tests advertise coverage they do not provide

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `spec-fixit-ci-peer-block-silent-directives` |
| Phase | 1/1 |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` |
| Updated | 2026-08-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two functional tests state a subject in their header and never prove it. Both
were found by an independent review on 2026-08-02 and both were re-read and
confirmed the same day. A test that exists but cannot fail is worse than an
absent one, because it reads as coverage on every audit
(`ai/rules/testing.md`, "Test Sensitivity Ratchets").

### Test 1: `test/plugin/filter-family-export-flowspec.ci`

Its header claims it proves two things: that an export-direction family filter
removes `ipv4/flow` from originated routes, and that the per-family End-of-RIB
marker survives the same gate. It proves neither.

| Finding | Detail |
|---------|--------|
| The injections are inert | Both route injections are `cmd=api:` lines placed inside the `stdin=peer` block. `cmd=` is documentation-only to ze-peer, so nothing is ever announced |
| No plugin drives the API | The file has no `tmpfs` plugin script, so nothing else injects either |
| The rejection assertion is dead | `reject=bgp:conn=1:pattern=01180A010003` is dropped, for both reasons in `spec-fixit-ci-peer-block-silent-directives` |
| The one live assertion proves nothing about the filter | `expect=bgp:conn=1:seq=1:hex=...900F0003000185` is an MP_UNREACH_NLRI (type 15) with AFI 1 and SAFI 133 and no NLRI, which is the `ipv4/flow` End-of-RIB. Ze sends that at initial sync for every configured family, filter or no filter |

So the test passes on a daemon that has no export filter at all. The FlowSpec
route-reflector case it names is unproven, and so is the EoR exemption, because
the EoR would appear either way.

### Test 2: `test/plugin/logging-level-filter.ci`

Its subject is that `ze.log.bgp.server=info` filters out DEBUG lines. The only
assertion of that subject is `reject=stderr:pattern=level=DEBUG`, and it sits
inside the `stdin=peer` block where the runner never sees it. The two live
assertions are `expect=bgp:` hex matches for an UPDATE and an EoR, which say
nothing about log levels.

The irony is worth recording: the same file carries a comment explaining that
`option=env` must live OUTSIDE the peer block because the runner consumes it, and
then places a runner-consumed `reject=stderr:` inside the block.

### Sequencing

**Test 2 shares its root cause with `spec-fixit-ci-peer-block-silent-directives`
and must be sequenced AFTER it.** That spec adds a runner guard that hard-errors
on `reject=` inside a peer block. Once the guard fires, this test goes red on its
own and the fix is forced rather than remembered. Fixing the line here first
would remove the evidence the guard exists to produce.

Test 1 needs the same guard for its dead `reject=bgp:` line, and separately needs
a live injection path, which the guard does not give it.

Neither test may be deleted to reach green (`ai/rules/testing.md`). Both
name real behavior. The behavior needs proving, and if proving it exposes a
defect in the export-filter gate, that defect is in scope
(`ai/rules/completion.md`).

Source: `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md`, rows 4 and 5.

## Required Reading

- [ ] `ai/rules/interop-and-goal-validation.md` - "Prove the test discriminates"
  → Constraint: revert the behavior under test and confirm the test goes RED. Both tests fail that check today.
- [ ] `ai/rules/testing.md` - "Mutation-Verify the Test Actually Gates"
  → Constraint: disable the producing function; a test that still passes guards nothing.
- [ ] `ai/rules/testing.md` - a red or vacuous test is fixed, never weakened
  → Constraint: the header states the subject; the assertions must be made to match it, not the reverse.
- [ ] `docs/architecture/testing/ci-format.md` - which directives each block consumes
  → Constraint: `cmd=` and `reject=` are runner-only, so a peer block is the wrong home for both.

**Key insights:**
- An End-of-RIB assertion is a trap in any filter test: ze emits one per configured family at initial sync regardless of policy.
- `cmd=api:` inside a peer block is the same class of silent drop as `reject=`, and no guard covers it yet.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/peer/expect.go` - `consumes` returns true only for `expect=bgp` and a fixed `action=` set, which is why `cmd=` and `reject=` vanish inside a peer block
- [ ] `internal/test/runner/record_parse.go` - `parseReject` has no `bgp` case; the peer-block loop is where the drop happens
- [ ] `internal/component/bgp/plugins/filter_family/` - the export-filter gate test 1 claims to prove; read it before writing the replacement assertion

**Behavior to preserve:** the two tests keep their stated subjects and their headers. `filter-family-export-flowspec.ci` keeps asserting that the per-family EoR survives the gate, because that IS a real requirement even though the current assertion cannot fail.

**Behavior to change:** the assertions, so that each one can fail. No production behavior is targeted, but a defect uncovered by a now-live assertion is fixed here.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A `.ci` file read by the functional-test runner, which splits it into a peer block, a config block, and runner directives.

### Transformation Path
1. The runner parses the file and routes each line by its block.
2. Lines inside `stdin=peer` go to ze-peer, which keeps only what `consumes` accepts.
3. Everything ze-peer does not consume is discarded with no diagnostic.
4. The runner starts ze-peer and ze, and compares only the assertions that survived step 2.
5. The test reports pass, having never evaluated the discarded lines.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner ↔ ze-peer | the peer block, filtered by `consumes` | Yes, read on 2026-08-02 |
| Runner ↔ ze | the config block plus `cmd=foreground` | Yes |
| Test author ↔ harness | a directive with no acknowledgement path | Yes, this is the defect |

### Integration Points
- `spec-fixit-ci-peer-block-silent-directives` - supplies the guard that makes test 2 fail honestly. This spec depends on it.
- `internal/component/bgp/plugins/filter_family/` - the subject of test 1.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | The injection uses the real API rail. The observer calls `api.send('peer * update text ... nlri ipv4/flow add ...')` from `test/plugin/filter-family-export-flowspec.ci`, so the route enters through the plugin API and reaches egress through `writeUpdateGated` (`internal/component/bgp/reactor/session_write.go`). No test hook writes to a peer socket, and the suppression is read back from the daemon's own per-peer counters |
| No unintended coupling | Yes | The diff is test-side only: two `.ci` files and one doc section. The runner behaviour both tests depend on -- `ClaimLine` (`internal/test/peer/expect.go`) answering `ClaimRunner`, and `validateOnePeerBlock` (`internal/test/runner/peer_contract.go`) parsing the line where it stands -- came from the `Depends` spec and is not touched here |
| No duplicated functionality | Yes | No other `.ci` proves the export direction. `filter-family-import-teardown.ci` and `filter-family-import-remove.ci` both configure `import [ ... ]`, and `test/parse/filter-family-config.ci` only parses the config block. `filter-family-export-flowspec.ci` is the single export-direction test |
| Zero-copy preserved where applicable | N-A | test-side change |
| Registration over hardcoding (`ai/rules/plugins.md`) | N-A | no command, view, family, or handler added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The export-direction family filter actually works, and only its test is broken. | Nothing has proven it either way; the test that claimed to has been vacuous since it was written. | A real defect exists in the export gate and is in scope here (`ai/rules/completion.md`). | make the assertion live and observe | confirmed -- with the filter present the route never reaches the filtered peer; with it removed the peer receives it. No production defect found |
| A-2 | The `ipv4/flow` EoR is sent at initial sync regardless of export policy. | The hex asserted decodes to an MP_UNREACH with AFI 1, SAFI 133 and no NLRI, which is the EoR for that family. | The current assertion is not vacuous after all, and only the injection half of test 1 is broken. | remove the export filter from the config and re-run: if the test still passes, the assertion is vacuous | confirmed -- the marker survives because `writeUpdateGated` (`internal/component/bgp/reactor/session_write.go`) exempts it through `IsEndOfRIBAnyFamily`. Forcing that predicate false suppresses the marker, so the EoR assertion is load-bearing about the EXEMPTION, not about the filter |
| A-3 | A `tmpfs` plugin script is the right way to drive a live injection. | Other `.ci` tests in `test/plugin/` use one for API-driven scenarios. | Use whatever the corpus's working injection tests use. | read a working API-injection `.ci` first | confirmed -- `test/plugin/flowspec-announce.ci` and `test/plugin/originated-nexthop-peer-own.ci` both drive the API from a `tmpfs=*.run` observer, and the rewrite follows that shape |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The live assertion goes red and the temptation is to relax it back. | A failing assertion that "looks wrong". | The header states the requirement. A red means the daemon is wrong (`ai/rules/testing.md`). |
| R-2 | Test 2 is fixed before the guard lands, so the guard ships with no test that would have caught the class. | This spec's work starts before its `Depends` spec. | Honour the sequencing. The guard fires first, then this test is fixed. |
| R-3 | Other `.ci` tests carry the same inert `cmd=api:` pattern and are equally vacuous. | The corpus scan finds more. | Audit for `cmd=` inside a peer block in the same pass, and report the count. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing at runtime. The cost is already paid: two audits' worth of false coverage. |
| How is it reverted? | Single commit revert, unless a production defect is uncovered and fixed alongside. |
| Who else touches this path? | Any session working the export-filter gate or the `.ci` peer-block parser. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator configures an export family-filter and a route of that family is originated | → | the export gate drops the route before egress | `test/plugin/filter-family-export-flowspec.ci`, rewritten so it fails when the filter is removed |
| Initial sync completes for a configured family | → | the End-of-RIB marker bypasses the export gate | `test/plugin/filter-family-export-flowspec.ci`, with the filter removed as the discriminating control |
| `ze.log.bgp.server=info` is set and the server subsystem logs | → | DEBUG lines are suppressed | `test/plugin/logging-level-filter.ci`. The rejection assertion stays where it always stood, inside the peer block; the `Depends` spec's runner guard is what makes the runner read it |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `filter-family-export-flowspec.ci` with the export filter removed from its config | The test FAILS. It passes today, which is the defect |
| AC-2 | `filter-family-export-flowspec.ci` unchanged | A FlowSpec route is genuinely announced toward the peer and genuinely suppressed, proven by an assertion that would fail if it were delivered |
| AC-3 | `logging-level-filter.ci` with the level set to debug instead of info | The test FAILS |
| AC-4 | The `.ci` corpus | Every other test carrying `cmd=` or `reject=` inside a peer block is listed, with a verdict per site |
| AC-5 | Any production defect uncovered by AC-1 or AC-2 | Fixed in this spec, not deferred |

### AC-4: the corpus audit, with a verdict per class

Measured over every `.ci` whose block is named on a `cmd=...:exec=ze-peer ...:stdin=<name>` line.
Counts re-derived on 2026-08-17 against the current tree; the earlier run of the
same scan read 233/78, 11/9 and 3, and the corpus moved under it.

Re-derived a third time by the closure pass on 2026-08-22 at HEAD `50468ee34`,
and every row below matched: 12 sites in 10 files, 1, 0, and 230 sites in 77
files. The 233/78 reading is the same scan with `test/draft/` included:
`test/draft/plugin/mup4-debug.ci` carries the extra 3 sites, and a draft is not
run, so the live corpus is the 230/77 figure the table states.

| Class | Sites | Verdict |
|-------|-------|---------|
| `reject=bgp:` inside a peer block | 12, in 10 files | Correctly homed. It is a real per-connection ze-peer directive (`internal/test/peer/reject.go`), and `validatePeerBlockRejects` (`internal/test/runner/peer_contract.go`) now refuses one whose connection carries no `expect=bgp` delivery |
| `reject=stderr:` inside a peer block | 1, `test/plugin/logging-level-filter.ci` | Live. The `default` arm of `validateOnePeerBlock` hands a line ze-peer does not claim to the runner's own parser, so it is read where it stands |
| Any other `reject=` inside a peer block | 0 | None remain, and a new one cannot be added silently: an unclaimed, unparseable line now fails the file at parse time |
| `cmd=` inside a peer block | 230, in 77 files, every one of them a `cmd=api` | Narration. ze-peer records it as documentation and nobody executes it. 75 of the 77 files carry a live producer as well (a config `update { }`, a `tmpfs` observer, or an `action=`), so the narration is a comment beside a real injection |
| `cmd=api:` with NO live producer anywhere in the file | 2: `test/encode/bgpls-encode.ci`, `test/encode/unknown-capability-encode.ci` | Each names a producer that never runs, so each `cmd=api:` line is a wrong comment. Neither is vacuous: both assert the initial-sync End-of-RIB for a configured family, which discriminates on the subject each states. `bgpls.ci` asserts the marker for AFI 16388 SAFI 71, which ze emits only for a family it negotiated, so it proves BGP-LS negotiation; `unknowncap.ci` asserts the ipv4/unicast marker after its peer sends an unknown capability, so it proves the OPEN was not refused. The assertions stand; the two narration lines misattribute their producer. `test/plugin/text-handshake.ci` was the third site and has since gained a live producer |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none new) | the subject is functional coverage, not a new algorithm | AC-1..AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-family-export-flowspec.ci` | `test/plugin/filter-family-export-flowspec.ci` | an operator's export family-filter suppresses a FlowSpec route while graceful-restart signalling survives | rewritten and green (3/3 isolated, PASS in the full plugin suite); both claims proven to fail under a reverted mechanism. Re-proven on 2026-08-22 in a detached worktree at HEAD `6dc0b403c`: PASS in 6.3s, RED with the peer's `filter { export [ ... ] }` block removed (the wire rejection fires on `01180A0100` and the observer reads `updates-sent=2`), RED again with the `!update.IsEndOfRIBAnyFamily()` term dropped from `writeUpdateGated` (the filtered peer never receives the marker), PASS once the term is restored |
| `logging-level-filter.ci` | `test/plugin/logging-level-filter.ci` | an operator sets a subsystem log level and DEBUG output stops | green; the runner now reads the in-block `reject=stderr` line through `validateOnePeerBlock` (`internal/test/runner/peer_contract.go`). Re-proven on 2026-08-22 at HEAD `6dc0b403c`: PASS in 4.1s, RED in 4.4s with `value=info` changed to `value=debug`, reporting `reject=stderr pattern found: level=DEBUG` over three lines that all carry `subsystem=bgp.server` |

## Files to Modify
- `test/plugin/filter-family-export-flowspec.ci` - make the injection live and the suppression assertion able to fail
- `test/plugin/logging-level-filter.ci` - record the non-vacuity evidence in the file. The rejection assertion needed no move: the `Depends` spec's runner guard made the line live where it already stood, which is the same outcome by a different route than this spec planned
- `docs/architecture/testing/ci-format.md` - state that `cmd=` and `reject=` inside a peer block are discarded, and what to use instead. Not edited: "What a ze-peer block may carry" already says who reads each directive

## Files to Create
- `test/draft/plugin/` - draft both rewrites here first and promote when green (`ai/rules/testing.md`, "Draft a Functional Test Before It Is Live")

## Implementation Steps

1. Wait for `spec-fixit-ci-peer-block-silent-directives` to land its guard. Confirm test 2 is red.
2. Validate A-2 by removing the export filter from test 1's config and re-running. Record the result before changing anything.
3. Draft the rewrite of test 1 under `test/draft/plugin/`. Prove it fails with the filter removed.
4. Fix test 2 by moving the rejection assertion to runner scope. Prove it fails at debug level.
5. Audit the corpus for AC-4 and report the count with a verdict per site.
6. Promote both drafts.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Both tests discriminate, proven by the reverted-behavior run, not by a green bar |
| Correctness | The EoR assertion is no longer load-bearing on its own; the discriminating assertion is the suppression |
| Rule: `ai/rules/testing.md` | No assertion was removed to reach green; every removal carries a stated reason |
| Rule: `ai/rules/completion.md` | A defect uncovered by the now-live assertion is fixed here, not filed |
| Registration over hardcoding | N-A |

### Deliverables Checklist

<!-- Added by the closure pass on 2026-08-22. The spec was written without the
     three template checklists, and `/ze-close` verifies against them, so they
     are filled here rather than left absent. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `test/plugin/filter-family-export-flowspec.ci` discriminates on the export gate | `ze-test bgp plugin --pattern filter-family-export-flowspec` PASS, and FAIL with the peer's `filter { export [ ... ] }` block deleted |
| The same file discriminates on the End-of-RIB exemption | The same run FAILS with the `!update.IsEndOfRIBAnyFamily()` term dropped from `writeUpdateGated` (`internal/component/bgp/reactor/session_write.go`) |
| `test/plugin/logging-level-filter.ci` discriminates on the log level | `ze-test bgp plugin --pattern logging-level-filter` PASS, and FAIL with `option=env:var=ze.log.bgp.server:value=info` set to `debug` |
| The AC-4 corpus table matches the tree | Re-derive the four counts from the peer-block scan and compare against the table |
| Every surviving `reject=` literal in the corpus has a producer | The two sweeps of Known Limitations, with the residue triaged site by site |
| `test/weakened.md` carries a row for the one removal | `make ze-test-weakened-check` and `python3 scripts/dev/check_weakened_tests.py test/plugin/task-forbidden.ci` |

### Security Review Checklist

<!-- Added by the closure pass on 2026-08-22. -->
| Check | What to look for |
|-------|-----------------|
| Untrusted input | None reachable. The diff changes comment text and deletes one assertion in a `.ci` file. No production code path is touched |
| Assertion removed from a security surface | `test/plugin/task-forbidden.ci` guards the MCP task-support annotation, which is an authorization-shaped surface. The removed line asserted the absence of a refusal string; the retained positive pair asserts the tool was DISPATCHED and answered inline, which is the stronger claim on the same surface |
| A guard left with no test | `lookupTaskSupport` (`internal/component/mcp/streamable_tools.go`) keeps its coverage: `task-call ze_show_bgp` in the same file proves the `required` half, and the `probe` rows prove the `forbidden` half |
| Information leakage | None. No error text, log line, or CLI output changes |

### Documentation Update Checklist (BLOCKING)

<!-- Added by the closure pass on 2026-08-22. Every No is backed by the
     `make ze-repository-check` anchor scan plus a grep of `docs/` for the
     changed paths. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Nothing ships to an operator |
| 2 | Config syntax changed? | No | No YANG, parser, or resolver edit |
| 3 | CLI command added/changed? | No | No handler edit |
| 4 | API/RPC added/changed? | No | No RPC type or method edit |
| 5 | Plugin added/changed? | No | No registration edit |
| 6 | Has a user guide page? | No | The subject is the test corpus |
| 7 | Wire format changed? | No | The EoR exemption was mutated and restored, never changed |
| 8 | Plugin SDK/protocol changed? | No | No `pkg/plugin` edit |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No `rfc/` row moves; no tagged test changed |
| 10 | Test infrastructure changed? | No | The runner is untouched. `docs/functional-tests.md` describes the directives, and no directive's meaning changed |
| 11 | Affects daemon comparison? | No | No capability claim moves |
| 12 | Internal architecture changed? | No | No component boundary moves |
| 13 | Route metadata keys added/changed? | No | None touched |
| 14 | Prometheus counters added/changed? | No | None touched |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None touched |
| 16 | Any changed source file referenced by existing doc source anchors? | No | `make ze-repository-check` is green, and it is the check that reads the anchors |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No syntax an example could show has changed |

## Known Limitations
- **The corpus sweep covered the LITERAL half only, and it is not a gate.** Two
  mechanical passes ran on 2026-08-22. The first read all 616
  `reject=stderr:pattern=` and `reject=stdout:pattern=` sites in the non-draft
  `.ci` and `.et` corpus, reduced them to 107 distinct non-boilerplate literals,
  and asked whether each longest literal run appears in `internal`, `pkg`,
  `cmd`, `scripts` or `api` outside an assertion line. It flagged 10. Nine were
  false positives of one blind spot: the needle is assembled by a format string,
  so `refusing to start API` has a producer (`checkMgmtListeners`,
  `cmd/ze/hub/mgmt_guard.go`, `refusing to start %s`, with
  `service: "API"` at `cmd/ze/hub/main.go`) that no literal grep can see. One
  was real and is fixed. 615 such sites remain, which is the arithmetic of the
  one removal. The same blind spot works the other way: a dead literal whose
  text happens to be a substring of an unrelated source line reads as covered.
- **The observer half is largely unswept.** The second pass read only the 24
  sites where a quoted literal of at least four characters sits to the left of
  `not in` inside a `require(...)` or `assert(...)` call. It flagged three, all
  false positives, each a value the config supplies and the product echoes back.
  The `.ci` and `.et` corpus holds 278 `require`/`assert` observer assertions,
  so 254 were not judged: their needle is a variable or an expression, and only
  per-site analysis can say whether the product can produce it.
- **No gate enforces that a `reject=` literal has a producer.** Both passes were
  scratch scripts run once. Nothing in `ze-precommit-verify` repeats them, so a
  new dead assertion lands unchallenged. A reader of this spec MUST NOT take the
  corpus as clean.
- The audit in AC-4 covers `cmd=` and `reject=`. A full sweep of every runner-only directive misplaced into a peer block is wider than this spec, and belongs with the guard spec that owns the parser.
- AC-3 proves its subject by mutation, not by an in-test control. `logging-level-filter.ci` asserts the ABSENCE of `level=DEBUG` and holds no assertion that `bgp.server` logged anything at all, so a build that stopped logging that subsystem would pass it. The discrimination is real and it is measured elsewhere: the `value=debug` copy of the file FAILS on DEBUG lines carrying `subsystem=bgp.server`, so the rejection is answered by a site that fires in this scenario. An in-test positive control would need a stable INFO-level line to match, which is a second assertion on log TEXT for no gain in what the file discriminates (`ai/rules/simplicity.md`). The mutation is recorded in the file's own NON-VACUITY header, where the next reader meets it.
- `logging-level-filter.ci` is load-flaky, and this spec is not its cause. Measured 4 FAIL / 4 PASS in 8 sequential runs, failing at load average 38 and passing at 16, with every expected message received and the run still hitting the 10s wall. Both timeout directives predate this work (`84312fe01`). Recorded as one more occurrence of `plan/journal/gate-verdict-depends-on-the-machine.md`, which now holds 23 rows. The class earns a deliberate pass of its own; raising the number here is banned (`ai/rules/testing.md`).
- `filter-family-export-flowspec.ci` is load-flaky too, on the same mechanism as its sibling and for the same reason: a fixed poll budget against a daemon the host is not scheduling. Its observer holds `wait_peer_eor_sent(expected_peers=2)` (`test/scripts/ze_api.py`), which is 40 attempts at 0.25s, so it gives up 10s after `ready()`. Measured on 2026-08-22: one FAIL at 13:19 on a busy machine, both peers holding the marker on the wire and neither counted, the whole run hitting the 30s wall; then three consecutive PASS in the same tree 35 minutes later at load average 7.17, 4.4s to 4.6s each. The same file passes at HEAD in a detached worktree in 6.3s, so the tree is not the variable. Recorded as one more occurrence of `plan/journal/gate-verdict-depends-on-the-machine.md`. Not tuned: raising the attempt count is the move `ai/rules/testing.md` bans, and the class earns a deliberate pass of its own.
  The first reading of that FAIL blamed another session's uncommitted `pkg/plugin/rpc` answer-encoding work for breaking the rail the observer reads. The closure pass settles the withdrawal on stronger evidence than the implementation phase had: that work is COMMITTED at HEAD `50468ee34`, and the file passes 4 of 4 in a clean detached worktree at that commit, in 3.0s to 4.1s at load average 5.35. The in-flight diff was never the variable, and it is no longer in flight.

## Implementation Summary

### What Was Implemented

The two tests this spec names were rewritten by the 2026-08-17 session and
landed in `06c95f65d`, with the runner guard they rely on in `b452c9221`. This
spec's remaining work was proof and sweep, and it produced one further fix.

- `test/plugin/task-forbidden.ci`: removed `reject=stderr:pattern=does not
  support task-augmented calls`, which no current build can emit, and replaced
  it with a comment naming what the claim now rests on.
- `plan/journal/green-that-could-not-have-been-red.md`: one row for that dead
  assertion and the sweep that found it.
- `plan/journal/gate-verdict-depends-on-the-machine.md`: one row for the
  `filter-family-export-flowspec` load red, and for the wrong first reading of
  it.
- `test/weakened.md`: the row that accepts the one removal.

### Bugs Found/Fixed

| Bug | Where | Covered by |
|-----|-------|-----------|
| A `reject=` asserting the absence of a phrase no build writes | `test/plugin/task-forbidden.ci` | the positive pair that stays: `probe status=200 code=ok` with `"resultType":"complete"` and no taskId, re-run PASS at HEAD `50468ee34` in 2.7s |
| The removal's own comment said the phrase "could not fire on any build" | `test/plugin/task-forbidden.ci`, and the journal row | corrected by the closure pass. A 2026-07-29 build DID emit it, recorded in `plan/deferrals/mcp2026-1-stateless-core.md`. The claim that holds is about the CURRENT build |
| No production defect was found | -- | A-1 and A-2 both confirmed at the producer |

### Documentation Updates

None. The Documentation Update Checklist above answers all 17 rows No, and
`make ze-repository-check` is green, which is the check that reads the source
anchors. `make ze-doc-verify` was not run because no doc changed.

### Deviations from Plan

- The spec was written without the three template checklists (Deliverables,
  Security Review, Documentation Update). `/ze-close` verifies against them, so
  the closure pass ADDED and filled them rather than stopping. Stopping would
  have left finished, green work unlanded over a template gap, and every row is
  filled with a check that was actually run.
- Implementation steps 1 to 6 were executed by the 2026-08-17 session. This
  phase re-proved the result against a tree that had moved rather than
  re-executing them.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | A `filter-family-export-flowspec` FAIL was read as another session's uncommitted `pkg/plugin/rpc` work breaking the answer rail | It was load. The rail was intact: the sibling `originated-nexthop-peer-own.ci` read the same rail and passed in the same tree | Three consecutive PASS in the same dirty tree 35 minutes later, then 4 of 4 in a clean worktree at the commit that carries the rpc work | Withdrawn by its own author, confirmed by the closure pass, and recorded in `plan/journal/gate-verdict-depends-on-the-machine.md` |
| assumption | The removal's comment asserted the dead phrase "could not fire on any build" | It fired on a 2026-07-28 to 2026-07-29 build. The cutover to the server-directed model replaced the refusal with a revert | The closure pass grepped the phrase corpus-wide and met `plan/deferrals/mcp2026-1-stateless-core.md`, which quotes the refusal verbatim | The comment and the journal row both corrected to say "this build" and to name when it was live |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Test 1 keeps its subject and gains an assertion that can fail | Done | `test/plugin/filter-family-export-flowspec.ci` | The observer injects through `api.send`; suppression is read back from the daemon's per-peer counters |
| Test 2's rejection assertion becomes live | Done | `test/plugin/logging-level-filter.ci`, `validateOnePeerBlock` (`internal/test/runner/peer_contract.go`) | The line never moved. The `Depends` spec's guard made it live where it stood |
| Neither test is deleted to reach green | Done | both files | Both keep their headers and their stated subjects |
| A defect uncovered by a now-live assertion is fixed here | Done | no production defect surfaced | A-1 and A-2 both confirmed |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `ze-test bgp plugin --pattern filter-family-export-flowspec` FAILS with the peer's `filter { export [ ... ] }` block deleted | Re-measured at HEAD `50468ee34`: `ZE-PEER-REJECTED ... 01180A0100` on the wire, and the observer reads `updates-sent: 2` for `filtered-peer` |
| AC-2 | Done | The same file PASSES unmutated, and FAILS with the EoR exemption dropped | Unmutated PASS 3.0s. With `!update.IsEndOfRIBAnyFamily()` removed from `writeUpdateGated` (`internal/component/bgp/reactor/session_write.go`), the filtered peer receives no `900F0003000185` frame at all while the unfiltered peer receives it |
| AC-3 | Done | `ze-test bgp plugin --pattern logging-level-filter` FAILS with `value=info` set to `debug` | Re-measured at HEAD: `reject=stderr pattern found: level=DEBUG`, over three lines all carrying `subsystem=bgp.server` |
| AC-4 | Done | The corpus table in Acceptance Criteria, re-derived a third time on 2026-08-22 | 12/10, 1, 0 and 230/77, every row matching |
| AC-5 | Done | No production defect uncovered | Both mutations are test-side or a one-term revert of a correct guard |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `filter-family-export-flowspec.ci` | Done | `test/plugin/filter-family-export-flowspec.ci` | PASS 4 of 4 at HEAD `50468ee34`, RED under two independent mutations |
| `logging-level-filter.ci` | Done | `test/plugin/logging-level-filter.ci` | PASS at HEAD, RED under the level mutation |
| `task-forbidden.ci` | Done | `test/plugin/task-forbidden.ci` | PASS 2.7s at HEAD after the removal |
| (no new unit test) | Done | -- | The subject is functional coverage, not a new algorithm |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `test/plugin/filter-family-export-flowspec.ci` | Done | Rewritten in `06c95f65d`; unchanged by this phase |
| `test/plugin/logging-level-filter.ci` | Done | NON-VACUITY header added in `06c95f65d`; unchanged by this phase |
| `docs/architecture/testing/ci-format.md` | Changed | Not edited. "What a ze-peer block may carry" already states who reads each directive |
| `test/draft/plugin/` | Changed | Used by the 2026-08-17 session and not needed by this phase |
| `test/plugin/task-forbidden.ci` | Done | Added by this phase; not in the original file list |

### Audit Summary
- **Total items:** 18
- **Done:** 16
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (both recorded in Deviations and in Files from Plan)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| `filter-family-export-flowspec.ci` proves the export-direction family filter removes `ipv4/flow` from originated routes | functional, mutation-proven | PASS 3.0s at HEAD `50468ee34`. Deleting the peer's `filter { export [ ... ] }` block makes it FAIL: the route reaches the filtered peer, `ZE-PEER-REJECTED` fires on `01180A0100`, and the daemon's own counter reads `updates-sent: 2` |
| The same file proves the per-family End-of-RIB survives that gate | functional, mutation-proven | Dropping `!update.IsEndOfRIBAnyFamily()` from `writeUpdateGated` makes it FAIL. The filtered peer's transcript then holds only the OPEN, with no `900F0003000185` frame, while the unfiltered peer still receives one |
| `logging-level-filter.ci` proves `ze.log.bgp.server=info` suppresses DEBUG | functional, mutation-proven | PASS 2.5s at HEAD. Setting the level to `debug` makes it FAIL with `reject=stderr pattern found: level=DEBUG` over three `subsystem=bgp.server` lines |
| The `.ci` corpus carries no other test of this class | mechanical audit | AC-4's four rows re-derived on 2026-08-22 and matching, plus two literal sweeps whose coverage and whose LIMIT are stated in Known Limitations. One survivor found and fixed |
| No production defect hides behind the vacuous tests | source reading plus mutation | A-1 and A-2 both confirmed at `writeUpdateGated`. Neither mutation exposed a defect; each is a revert of a correct guard |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `test/plugin/filter-family-export-flowspec.ci` is VACUOUS (2026-08-02, independent review) | done | Set to `done` in `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` with the two mutations that redden it |
| `test/plugin/logging-level-filter.ci` has never enforced its own subject (2026-08-02, independent review) | done | Set to `done` in the same shard. The guard from `spec-fixit-ci-peer-block-silent-directives` made the line live where it stood |
| The other six rows in that shard | deferred / done, unchanged | Three remain `deferred` and are homed at `spec-wire-edit-2-deferred-ci-substitution`, `spec-wire-edit-3-deferred-ac9-dead-code` and `spec-fixit-fwdpool-backpressure-timing`. The shard therefore SURVIVES this closure and is not removed |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-vacuous-functional-tests-ebf5d9b3-b158-40df-bba0-32a51591883e.md` |
| `review_gate.py check` | clean (1 code file, hashes match) |
| Rounds | 1 |
| Reviewer lenses used | test discrimination (does each assertion redden under a mutation, re-measured at the current HEAD), removed-behavior audit over the one deletion, and record-against-source (every claim in the spec's own prose checked at the producing function) |

The 2026-08-17 entry that stood here was NOT reused. It judged a tree four days
and seven commits old, and two of those commits changed `pkg/plugin/rpc` and
`test/scripts/ze_api.py`, which are on the rail this spec's evidence reads. The
pass below is fresh and is independent of every author involved: the `.ci`
rewrites came from the 2026-08-17 session, the sweep and the `task-forbidden`
fix from the 2026-08-22 implementation phase, and this reviewer wrote none of
them.

Two commits carry the evidence, because the shared checkout moved under the
review. Every mutation proof ran in a detached worktree at `50468ee34`. All
three files were then re-run to PASS in a second detached worktree at
`1ed6b74e3`, which is HEAD at commit time: `task-forbidden` 4.4s,
`logging-level-filter` 2.9s, `filter-family-export-flowspec` 3.5s. The mutation
proofs were not repeated there, and the reason is checkable rather than
asserted: `git diff --name-only 50468ee34..1ed6b74e3` over
`internal/component/bgp/`, `internal/test/`, `test/scripts/` and
`internal/component/mcp/` returns one path, `exclusive_group_test.go`, which no
binary in these runs links.

Three lenses were run rather than one, because the diff touches a test that pins
an authorization-shaped surface (the MCP task-support annotation).

One round, and the loop stopped there deliberately. Both findings are RECORD
defects, not product defects, and `ai/rules/planning.md` states that a false
statement in a spec's own closure prose never earns another round.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The removal's comment claimed the phrase "could not fire on any build". A 2026-07-29 build refused a task-augmented call with that exact text, quoted verbatim in `plan/deferrals/mcp2026-1-stateless-core.md`. The comment also said `callTool` "writes only `unknown tool: ` on that path", which is the fallthrough for an unknown tool and not the forbidden path at all | `test/plugin/task-forbidden.ci`, and the same sentence in `plan/journal/green-that-could-not-have-been-red.md` | Both rewritten to say the phrase cannot fire on THIS build, to name when it was live, and to name what `callTool` actually reads: `lookupTaskSupport` for `TaskSupportRequired` only, with no `TaskSupportForbidden` branch |
| 2 | ISSUE | The spec carried no Deliverables, Security Review, or Documentation Update checklist, which `/ze-close` steps 1, 2 and 4 verify against | this spec | All three added and filled, each row backed by a check that was run rather than by a judgement |

NOTE, recorded and not acted on: the sweep that found the one dead assertion has
a blind spot it cannot close, and nine of its ten flags were false because of it.
It greps for a literal, so a needle a format string assembles reads as absent and
a needle that is a substring of an unrelated line reads as present. The boundary
is stated in Known Limitations rather than fixed, because fixing it means
building a gate, which is a different spec.

NOTE: `test/plugin/attach-process-reload` is red at HEAD for a cause another
session owns, recorded in `plan/journal/green-that-could-not-have-been-red.md`.
It is excluded from this closure's evidence and nothing here depends on it.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/filter-family-export-flowspec.ci` | Yes | Read at 217 lines during the AC-1 mutation; `cmd=foreground:seq=3:exec=ze --plugin ze.bgp-filter-family ...` at its tail |
| `test/plugin/logging-level-filter.ci` | Yes | Read during the AC-3 mutation; `option=env:var=ze.log.bgp.server:value=info` at line 36 |
| `test/plugin/task-forbidden.ci` | Yes | Read in full, 179 lines after the edit |
| `test/weakened.md` | Yes | `make ze-test-weakened-check` reports "test/weakened.md parses (1 row(s))" |
| `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` | Yes | Read in full; 3 rows still `deferred`, so it is NOT removed |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The file FAILS with the export filter removed | Run at HEAD `50468ee34` in a detached worktree, exit 1 at 30.0s: `ZE-OBSERVER-FAIL: destination peer 127.0.0.1 is 'connecting' ... 'updates-sent': 2`, and ze-peer reports `ZE-PEER-REJECTED: received bytes that reject=bgp:pattern=01180A0100 forbids` |
| AC-2 | The file PASSES unchanged and its EoR assertion is load-bearing | PASS 3.0s unmutated. With the exemption dropped, exit 1: the filtered peer's transcript holds OPEN only, no `900F0003000185`, while `127.0.0.2` receives both frames |
| AC-3 | The file FAILS at debug level | Exit 1 at 2.6s: `reject=stderr pattern found: level=DEBUG`, three DEBUG lines all `subsystem=bgp.server` (`OnMessageReceived`, two `OnPeerStateChange`) |
| AC-4 | The corpus table matches the tree | Re-derived on 2026-08-22 from the peer-block scan: `reject=bgp:` 12 sites in 10 files, `reject=stderr:` 1, any other `reject=` 0, `cmd=` 230 sites in 77 files all `cmd=api`. Every cell matches |
| AC-5 | No production defect | Both mutations restored and the file returns to PASS in 4.1s, so each was a revert of a correct guard rather than a discovery |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| An operator configures an export family-filter and a route of that family is originated | `test/plugin/filter-family-export-flowspec.ci` | Yes. Read the file: the observer calls `api.send('peer * update text ... nlri ipv4/flow add ...')`, so the route enters through the plugin API and reaches egress through `writeUpdateGated`. Suppression is asserted twice, on the wire (`reject=bgp:pattern=01180A0100`) and from the daemon (`assert_peer_received_only_eor`) |
| Initial sync completes for a configured family | `test/plugin/filter-family-export-flowspec.ci` | Yes. `expect=bgp` on the filtered peer asserts `900F0003000185`, and dropping the exemption removes that frame from that peer only |
| `ze.log.bgp.server=info` is set and the server subsystem logs | `test/plugin/logging-level-filter.ci` | Yes. `option=env` sits outside the peer block where the runner consumes it; `reject=stderr:pattern=level=DEBUG` sits inside it and is read by the `default` arm of `validateOnePeerBlock` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | The export gate works. With the filter present the route never reaches the filtered peer; deleting the filter delivers it and reddens the file. No production defect |
| A-2 | confirmed | The `ipv4/flow` EoR crosses the gate because `writeUpdateGated` exempts it through `IsEndOfRIBAnyFamily`. Forcing that predicate out suppresses the marker for the filtered peer alone, so the assertion is load-bearing about the EXEMPTION |
| A-3 | confirmed | The rewrite drives its injection from a `tmpfs=*.run` observer, the shape `test/plugin/flowspec-announce.ci` and `test/plugin/originated-nexthop-peer-own.ci` already use |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No doc anchor points at a changed file | `python3 scripts/dev/spec_doc_anchors.py plan/spec-fixit-vacuous-functional-tests.md` prints nothing, exit 0 | Yes |
| `docs/functional-tests.md` names `task-forbidden.ci` only in a list of the eight task tests | Read at line 1268. The removal of one `reject=` changes no claim on that page | Yes |
| No stale source anchor anywhere | `make ze-repository-check` exits 0, and it is the check that reads the anchors | Yes |

## Core Insight

An assertion can be born live and DIE without anyone touching it. The dead
`reject=` in `task-forbidden.ci` was correct on the day it was written: ze
refused a task-augmented call to a forbidden tool with exactly that sentence.
The cutover to the server-directed model replaced the refusal with a revert, the
sentence stopped existing, and the assertion kept passing because a `reject=`
passes hardest when its needle cannot occur. Nothing in the repository asks
whether a `reject=` literal still has a producer, so the decay is silent and
one-way. The vacuity class this spec was written for is therefore not only about
tests written wrong: it is about tests that were written right and were then
outlived by their subject.

The pass was independent of the author: the two `.ci` rewrites were written by
the 2026-08-17 session and landed in `06c95f65d`, and the runner guard they rely
on in `b452c9221`. The reviewer re-derived the AC-4 corpus counts from the
current tree rather than reading them out of the spec, and they matched.

One round, and the loop stopped there on purpose. Every finding was in the
RECORD, not in the product. `ai/rules/planning.md` ("How each review round is
scoped and when it ends") states that a false statement in a spec's own closure
prose is not a reason for another round, so the five were fixed in one edit.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | No `## Review Gate` section, which `review_gate_problems` (`scripts/dev/commit_helper.py`) refuses the closure commit without | this spec | this section, and the artifact it cites |
| 2 | ISSUE | The test is load-flaky: 4 FAIL / 4 PASS in 8 sequential runs, failing at load average 38 and passing at 16, every expected message received and the run still hitting the 10s wall. Not this spec's doing -- both timeout directives exist at `84312fe01` | `test/plugin/logging-level-filter.ci` | one row in `plan/journal/gate-verdict-depends-on-the-machine.md`, the 23rd of that class. Not tuned: raising the number is banned (`ai/rules/testing.md`) and the class earns a deliberate pass of its own |
| 3 | ISSUE | No non-vacuity rationale in the file itself. The discrimination evidence lived only in a scratch state file due for deletion, while the sibling `filter-family-export-flowspec.ci` carries a NON-VACUITY header | `test/plugin/logging-level-filter.ci` | a NON-VACUITY header naming the mutation that reddens it, why a runner directive is read inside a peer block, and what the file does not prove. Comment-only: `ClaimLine` (`internal/test/peer/expect.go`) and `validateOnePeerBlock` (`internal/test/runner/peer_contract.go`) both skip a `#` line |
| 4 | ISSUE | Three `fill during design` cells left in the Architectural Verification table | this spec, Data Flow | filled from the shipped design, each cell naming the producing function or the sibling test it was checked against |
| 5 | ISSUE | The spec misdescribed its own shipped fix: it said the rejection assertion was moved out of the peer block. The line never moved. The `Depends` spec's runner guard made it live where it stood | this spec, Files to Modify and the AC-3 Wiring Test row | both rows corrected. The code is right and is unchanged |

NOTE, recorded and not acted on (`ai/rules/simplicity.md`): AC-3's absence
assertion discriminates on the mechanism, because a DEBUG site fires on every
session establishment, but the file holds no in-test control proving
`bgp.server` logged at all. It is in Known Limitations above and in the file's
own header. Adding a positive control would assert on log TEXT for no gain in
what the file discriminates.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] `make ze-precommit-verify` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Each test proven to FAIL when its subject behavior is reverted

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output from the reverted-behavior run)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
