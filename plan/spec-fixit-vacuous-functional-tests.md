# Spec: fixit-vacuous-functional-tests -- two `.ci` tests advertise coverage they do not provide

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `spec-fixit-ci-peer-block-silent-directives` |
| Phase | 1/1 |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-02-wire-edit-tail.md` |
| Updated | 2026-08-15 |

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
| No bypassed layers | No | fill during design: the replacement injection must reach the export gate through the real API rail |
| No unintended coupling | No | fill during design |
| No duplicated functionality | No | fill during design: check whether an existing `.ci` already proves the export gate before writing a new driver |
| Zero-copy preserved where applicable | N-A | test-side change |
| Registration over hardcoding (`ai/rules/plugins.md`) | N-A | no command, view, family, or handler added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The export-direction family filter actually works, and only its test is broken. | Nothing has proven it either way; the test that claimed to has been vacuous since it was written. | A real defect exists in the export gate and is in scope here (`ai/rules/completion.md`). | make the assertion live and observe | confirmed -- with the filter present the route never reaches the filtered peer; with it removed the peer receives it. No production defect found |
| A-2 | The `ipv4/flow` EoR is sent at initial sync regardless of export policy. | The hex asserted decodes to an MP_UNREACH with AFI 1, SAFI 133 and no NLRI, which is the EoR for that family. | The current assertion is not vacuous after all, and only the injection half of test 1 is broken. | remove the export filter from the config and re-run: if the test still passes, the assertion is vacuous | confirmed -- the marker survives because `writeUpdateGated` (`internal/component/bgp/reactor/session_write.go`) exempts it through `IsEndOfRIBAnyFamily`. Forcing that predicate false suppresses the marker, so the EoR assertion is load-bearing about the EXEMPTION, not about the filter |
| A-3 | A `tmpfs` plugin script is the right way to drive a live injection. | Other `.ci` tests in `test/plugin/` use one for API-driven scenarios. | Use whatever the corpus's working injection tests use. | read a working API-injection `.ci` first | confirmed -- `test/plugin/flowspec.ci` and `test/plugin/originated-nexthop-peer-own.ci` both drive the API from a `tmpfs=*.run` observer, and the rewrite follows that shape |

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
| `ze.log.bgp.server=info` is set and the server subsystem logs | → | DEBUG lines are suppressed | `test/plugin/logging-level-filter.ci`, with the rejection assertion moved to runner scope |

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

| Class | Sites | Verdict |
|-------|-------|---------|
| `reject=bgp:` inside a peer block | 11, in 9 files | Correctly homed. It is a real per-connection ze-peer directive (`internal/test/peer/reject.go`), and `validatePeerBlockRejects` (`internal/test/runner/peer_contract.go`) now refuses one whose connection carries no `expect=bgp` delivery |
| `reject=stderr:` inside a peer block | 1, `test/plugin/logging-level-filter.ci` | Live. The `default` arm of `validateOnePeerBlock` hands a line ze-peer does not claim to the runner's own parser, so it is read where it stands |
| Any other `reject=` inside a peer block | 0 | None remain, and a new one cannot be added silently: an unclaimed, unparseable line now fails the file at parse time |
| `cmd=` inside a peer block | 233, in 78 files | Narration. ze-peer records it as documentation and nobody executes it. 75 of the 78 files carry a live producer as well (a config `update { }`, a `tmpfs` observer, or an `action=`), so the narration is a comment beside a real injection |
| `cmd=api:` with NO live producer anywhere in the file | 3: `test/encode/bgpls.ci`, `test/encode/unknowncap.ci`, `test/plugin/text-handshake.ci` | Each names a producer that never runs, so each `cmd=api:` line is a wrong comment. None is vacuous: all three assert the initial-sync End-of-RIB for a configured family, which discriminates on the subject each states (BGP-LS capability negotiation, an ignored unknown capability, a completed text-mode plugin handshake). The assertions stand; the three narration lines misattribute their producer |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none new) | the subject is functional coverage, not a new algorithm | AC-1..AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-family-export-flowspec.ci` | `test/plugin/filter-family-export-flowspec.ci` | an operator's export family-filter suppresses a FlowSpec route while graceful-restart signalling survives | rewritten and green (3/3 isolated, PASS in the full plugin suite); both claims proven to fail under a reverted mechanism |
| `logging-level-filter.ci` | `test/plugin/logging-level-filter.ci` | an operator sets a subsystem log level and DEBUG output stops | green; the runner now reads the in-block `reject=stderr` line through `validateOnePeerBlock` (`internal/test/runner/peer_contract.go`) |

## Files to Modify
- `test/plugin/filter-family-export-flowspec.ci` - make the injection live and the suppression assertion able to fail
- `test/plugin/logging-level-filter.ci` - move the rejection assertion out of the peer block
- `docs/architecture/testing/ci-format.md` - state that `cmd=` and `reject=` inside a peer block are discarded, and what to use instead

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

## Known Limitations
- The audit in AC-4 covers `cmd=` and `reject=`. A full sweep of every runner-only directive misplaced into a peer block is wider than this spec, and belongs with the guard spec that owns the parser.

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
