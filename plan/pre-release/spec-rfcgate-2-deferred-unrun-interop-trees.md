# Spec: rfcgate-2-deferred-unrun-interop-trees

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-01 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Deferred out of `plan/spec-rfcgate-2-evidence.md` (see
the retired deferral shard "rfcgate-2-evidence").

The IPsec, L2TP and PPPoE automated callers now exist. This spec retains the
execution and discrimination evidence review before any closure; the presence
of a job or a derived tier is not a successful scenario run.

| Native carrier | Runner | Scheduled caller |
|---|---|---|
| `internal/le/interoplab/ipsec/` | `./le integration interop-ipsec` | `ipsec-interop` |
| `internal/le/interoplab/l2tp/` | `./le deployment docker-l2tp-ppp-test` | `l2tp-interop` |
| `internal/le/interoplab/pppoe/` | `./le deployment docker-pppoe-accel-test` | `pppoe-interop` |

All callers are in `.github/workflows/evidence-nightly.yml`.
`scheduledActionsFrom` and `interopCarriers` in `internal/le/rfc/carriers.go`
derive `interop/nightly` from those scheduled native actions; an action without
a scheduled caller resolves to `unrun`. The old Python `check.py` paths are
historical locations, not the current tagged-test implementation.

The workflow deliberately removed `continue-on-error: true` on 2026-09-02.
It remains outside the merge gate, but a failed job must report failure.
Restoring hidden failures would violate this contract.

The 2026-08-01 observations below remain historical evidence. Current source
returns XFRM assertion errors from `checkIPsecBGPRedistributeFRR`, while
`checkEAPTLS` now proves an explicit TLS 1.2 refusal after a real handshake and
requires no XFRM state on either peer. That refusal scenario cannot replace the
positive EAP/ESP proof the original plan sought. Both proof directions and
current runner results must be reconciled before closure.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
- [ ] `ai/rules/testing.md` - the carrier table and the two evidence axes (kind, tier)
  → Constraint: a native interop action without a scheduled caller remains
    `unrun`; its tags are refused. Scheduling grants a carrier tier, while
    scenario execution and discrimination establish the claimed proof.
  → Constraint: `interop/nightly` evidence is ADVISORY and never merge-gate. A
    requirement whose only evidence is nightly is marked `**nightly-only**` and
    counted in its own rollup column, never summed with verify-tier evidence.
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps
  → Constraint: "Before claiming an interop/functional test validates a change,
    revert the behaviour and watch the test go red." A test asserting the
    ABSENCE of something, or one whose assertion is swallowed, is not evidence.
- [ ] `ai/rules/evidence.md` - a guard must deny or say something
  → Constraint: a check that cannot evaluate its assertion must fail, not pass.
- [ ] `ai/rules/platform-linux.md` - "Interop Labs and Docker-Based Tests Need a QEMU Runner Too"
  → Decision: for a Docker lab needing host-kernel features, the repo's existing
    answer is a QEMU sibling (`ze-qemu-l2tp-ppp-test`, `ze-qemu-pppoe-accel-test`).
  → Constraint: a platform run can prove a tagged carrier only when it invokes
    that carrier's native checker. A similarly named deployment target is
    insufficient evidence on its own.
- [ ] `docs/labs/l2tp-interop.md`, `docs/labs/pppoe-interop.md` - host requirements
  → Constraint: the 2026-08-01 host observation found `pppoe` and `/dev/ppp`
    present but `l2tp_ppp` absent on that Darwin/Docker Desktop host. Compare
    current documentation with the native preflight; do not generalise that
    dated observation to a different host.

### RFC Summaries (Scope: protocol)
- N-A. Scope is tooling: this spec changes which carriers may hold a tag, never
  what any RFC requires.

**Key insights:**
- Native scheduled callers and tier derivation exist for all three trees.
- A negative EAP-TLS refusal and a positive ESP assertion prove different
  behaviours. Preserve both obligations when reconciling the migrated scenarios.

## Current Behavior (MANDATORY)

**Source files read for the 2026-09-19 reconciliation:**
- `internal/le/rfc/carriers.go`: `carriers` reads the scheduled action map;
  `interopCarriers` grants nightly tier only to a scheduled native action.
  `CarrierFor` recognises the native `.go` prefixes.
- `.github/workflows/evidence-nightly.yml`: the three named jobs invoke their
  native runners and set up Go. Failures are visible.
- `internal/le/interoplab/ipsec/checkers.go`: `checkIPsecBGPRedistributeFRR`
  returns failures from both peers' XFRM checks; `checkEAPTLS` requires handshake
  and refusal facts before checking that both peers have zero XFRM state.

The Python fail-open handlers described in the original 2026-08-01 research
were retired with the native migration. Their dated red runs below remain
evidence about that tree, not a pass or a failure of the current native suite.

**Behavior to preserve:**
- The RFC gate continues refusing a tag in any carrier with no scheduled caller.
  No current green gate is claimed here.
- The existing BGP interop evidence keeps its nightly classification while its
  scheduled caller remains present; retain its actual tagged population.
- Every runner keeps failing CLOSED: a missing Docker daemon or a missing host
  kernel module exits non-zero and never prints "skipping".
- `test/draft/` stays invisible to the scan.

**Completion work:**
- Review the delivered workflow and carrier derivation against the ACs below.
- Record current runner results, prerequisite failures and discriminating
  breaks; retain every unresolved positive EAP/ESP proof obligation.
- Do not re-add the delivered callers or hand-assign their tiers.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An `RFC requirement:` Go comment in a native checker under
  `internal/le/interoplab/ipsec/`, `l2tp/` or `pppoe/`.

### Transformation Path
1. `scheduledWorkflowActions` reads `.github/workflows/`, and
   `scheduledActionsFrom` derives actions from scheduled workflows.
2. `interopCarriers` maps each native checker tree to its declared action.
   A scheduled action grants `nightly`; an absent one leaves `unrun`.
3. `CarrierFor` resolves the native Go carrier before the tooling exclusion,
   and the scanner reads its tags. An unrun tag is refused.
4. The ledger reports `interop/nightly` separately from verify-tier evidence.
   A successful tag scan does not certify a scenario run.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Carrier ↔ workflow | `scheduledWorkflowActions`, `scheduledActionsFrom`, `interopCarriers` | Source present; runtime proof still owed |
| Current carrier ↔ HEAD baseline | each side must read its own workflow snapshot | Regression proof still owed before closure |
| Native action ↔ scenario checker | the runner must invoke the checker carrying the tag | Current suite execution still owed |

### Integration Points
- `internal/le/rfc/carriers.go` owns action parsing and tier assignment.
- Native interop packages own executable assertions; workflow callers own scheduling.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes at source | the tier is derived by `interopCarriers` from scheduled actions |
| No unintended coupling | Yes at source | workflow parsing remains in the RFC carrier producer |
| No duplicated functionality | Required | use the native action identity rather than restoring a Python make-target reader |
| Zero-copy preserved where applicable | N-A | tooling, no wire-path implementation here |
| Registration over hardcoding | Required | declared native trees map to actions; tiers remain derived from the workflow |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
The following assumption outcomes are the 2026-08-01 record. They do not
certify the migrated native runners. AC-4/AC-5 still require current hosted
execution evidence, but their callers are no longer absent.

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The ipsec lab actually passes, so its green is real rather than assumed | deferral note says all three are unverified since the launcher fix | the tree cannot earn a tier at all | ran `./le integration interop-ipsec IPSEC_INTEROP_SCENARIO=psk-site-to-site` on Darwin/Docker: exit 0, Ze-side XFRM SA present, ESP counters advanced | **confirmed for psk-site-to-site only -- see A-9/A-10** |
| A-9 | Scenario ipsec-bgp-redistribute-frr passes once its fail-open handler is gone | assumed by A-1 generalising from psk-site-to-site | the tree carries a real, previously-hidden dataplane defect | ran it 2026-08-01 with the handler removed: FAILS at `wait_xfrm_sa(ZE_CONTAINER)`. strongSwan installs its XFRM SA, Ze installs none. This is exactly the failure the `except (AssertionError, Exception)` was converting into a pass | **broken -- real defect, see below** |
| A-10 | Scenario eap-tls passes once its fail-open handler is gone | same | EAP-TLS is not interoperable today | ran it 2026-08-01: FAILS EARLIER than the removed handler, at step 1 `swan.wait_sa_established("ze")`. strongSwan logs `EAP method EAP_TLS failed for peer ze-test-client`; Ze logs `eap: authenticator sent Failure`. The handler removal did not cause it and could not have hidden it | **broken -- real defect, see below** |
| A-2 | `test/interop-ipsec/ze-linux` is a build output, not a checked-in input CI would lack | `.gitignore` line for it; absent from `git ls-files`; `run.py` `build_images()` regenerates it | CI could never build the image | `git check-ignore -v` and `git ls-files` | **confirmed** |
| A-3 | The ipsec lab needs a Go toolchain ON THE HOST (unlike the other three, which build inside Docker) | `build_images()` shells `go build` before `docker build` | the nightly job would fail at image build | read `run.py` `build_images()` | **confirmed** |
| A-4 | Every runner fails CLOSED on a missing prerequisite | claimed by the deferral note for interop/ipsec only | a job could go green having run nothing | read `run.py` main() for all four; `preflight_strict()` for l2tp/pppoe; each exits 1 | **confirmed** |
| A-5 | The two ipsec XFRM checks discriminate | implied by them being interop scenarios | a nightly tier would be granted to a vacuous check | read `eap-tls/check.py` and `ipsec-bgp-redistribute-frr/check.py`: both wrap the assertion in `except (AssertionError, Exception)`, eap-tls calls `log_pass` | **broken** |
| A-6 | `ubuntu-latest` provides `l2tp_ppp`/`pppol2tp` so the l2tp lab can run in CI | none - never measured | the l2tp job is red every night and the tree cannot earn a tier | needs one observed nightly run, or an owner ruling. Measured on Darwin/Docker Desktop: `l2tp_ppp` ABSENT | **unvalidated - blocks AC-4** |
| A-7 | `ubuntu-latest` provides `pppoe` + `/dev/ppp` so the pppoe lab can run in CI | none - never measured | the pppoe job is red every night and the tree cannot earn a tier | needs one observed nightly run. Measured on Darwin/Docker Desktop: both PRESENT, so the `docs/labs/pppoe-interop.md` claim they are absent is stale | **unvalidated - blocks AC-5** |
| A-8 | Deleting the `interop` job today would be caught | `TestEvidenceNightlyRunsInterop` exists | the tier derivation would be the only guard | read the test: it does pin the job by name | **confirmed** (the derivation is defence in depth, not the sole guard) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A scheduled checker passes with broken ESP | a positive scenario passes without its required XFRM state | AC-3 requires discriminating native checks; an available tier cannot substitute for that proof |
| R-2 | A scheduled L2TP/PPPoE job lacks a kernel prerequisite | hosted run reports a preflight failure | retain the visible failure and obtain the required execution proof; never hide it with continue-on-error |
| R-3 | The carrier parser credits an action the workflow does not run | tag accepted without its scheduled runner | AC-7 checks native action identity, comments, trigger scope and unreadable workflows |
| R-4 | The ipsec nightly job needs Go + Docker + privileged; a missing one turns a real failure into an infrastructure blip | job red with a build error, not a scenario failure | job runs `actions/setup-go` like its siblings; runner fails closed with a message naming Docker |
| R-5 | Adding a tree to `CARRIERS` with a nightly tier lets a future author bind an RFC MUST to nightly-only evidence when a `.ci` would have run on every push | a requirement's only evidence is `interop/nightly` | `ai/rules/testing.md` already prefers a `.ci`; the ledger already marks such rows `**nightly-only**` and never sums them |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. The failure mode is evidential: an RFC requirement could be credited to a test that does not run or cannot fail, which makes `docs/features/rfc-status.md` overstate conformance. |
| How is it reverted? | Single commit revert. No config migration, no wire change. The `check_evidence_ratchet` would then fire on any requirement that had taken interop evidence, which is the intended alarm. |
| Who else touches this path? | the rfcgate-1b pilot, now closed, whose IPsec tags this path carries, plus sibling sessions in `internal/component/ike/**` and `internal/component/bgp/**`. This spec touches neither tree. |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A tag in a native IPsec checker | → | `CarrierFor` resolves the `interop-ipsec` row derived by `interopCarriers` | native tag/runner proof required by AC-2 |
| Scheduled versus push-only workflow | → | `scheduledActionsFrom` | `TestOnlyAScheduledWorkflowGrantsANightlyTier` |
| Unreadable workflow directory | → | `scheduledWorkflowActions` refuses the read | `TestAWorkflowDirectoryTheGateCannotReadIsRefused` |
| Workflow native action syntax | → | `nativeActionsIn` | `TestNativeActionsInWorkflowCommands` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scheduled IPsec job runs | It invokes `./le integration interop-ipsec` with its required host toolchain; a runner or scenario failure fails the job. No `continue-on-error: true` masks it |
| AC-2 | A valid requirement tag in an executed native IPsec checker | The tag resolves as `interop/nightly`, the recorded scenario result names the checker that ran, and its discriminating break fails that checker |
| AC-3 | A positive IPsec scenario cannot obtain the required Ze-side XFRM state | It fails. `ipsec-bgp-redistribute-frr` and the positive EAP/ESP proof must propagate failed assertions. The migrated `eap-tls` TLS 1.2 refusal is recorded separately and cannot discharge a positive ESP proof |
| AC-4 | The L2TP caller and native checker run on the hosted Linux runner | A dated run demonstrates the required kernel prerequisites and scenario assertions; valid tags resolve as `interop/nightly`. Scheduling alone is insufficient completion evidence |
| AC-5 | The PPPoE caller and native checker run on the hosted Linux runner | The same proof as AC-4 is recorded for PPPoE, including its kernel prerequisites |
| AC-6 | A scheduled interop action is removed from the workflow snapshot | Its carrier becomes `unrun`, its tags are refused, and comparison with HEAD reports lost evidence without relabelling both sides from the new workflow |
| AC-7 | Native workflow action parsing receives scheduled, push-only, commented or unreadable input | Only an executed native action in a scheduled workflow grants nightly tier; comments and push-only jobs grant none, and unreadable/empty workflow sources fail closed |
| AC-8 | The final proof inventory and RFC gate are reconciled | Every original carrier/proof obligation is accounted for, no evidence kind or polarity is silently lost, and all remaining suite failures are explicit. Closure still owes the required clean final gate and current execution evidence |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->

N-A. Scope is tooling: the consumer is a developer binding an RFC requirement to
a test, and that path is covered by the Wiring Test table above.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOnlyAScheduledWorkflowGrantsANightlyTier` | `internal/le/rfc/tags_test.go` | scheduled versus push-only tier derivation | Existing; not run in this reconciliation |
| `TestNativeActionsInWorkflowCommands` | same | native action identity | Existing; not run |
| `TestAWorkflowDirectoryTheGateCannotReadIsRefused` | same | fail-closed workflow reads | Existing; not run |
| `TestEAPTLSNegativeHandshakeIsProvenAndFailClosed` | `internal/le/interoplab/ipsec/ipsec_test.go` | refusal proof requires a real handshake | Existing; not run |
| `TestXFRMStatePropagatesCommandFailure` | same | failed kernel read is not empty success | Existing; not run |
| HEAD/current workflow-loss proof | `internal/le/rfc/` | AC-6, including each side's own workflow snapshot | Review required |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A - this change introduces no numeric input | - | - | - | - |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Native RFC gate and lab entry points | `internal/le/rfc/`, `internal/le/interoplab/` | The contributor invokes the native action and receives its actual result | Current execution evidence owed |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Current native positive EAP/ESP proof | `internal/le/interoplab/ipsec/` | strongSwan | Reconcile the positive proof with the original EAP scenario before closure; the TLS 1.2 refusal below cannot replace it | Unfinished evidence review |
| `psk-site-to-site` | `test/interop-ipsec/scenarios/` | strongSwan | the tree is genuinely green: IKE SA + Child SA + XFRM SA on BOTH sides, ESP counters advancing | **PASS** (measured 2026-08-01, exit 0) |
| `eap-tls` | `test/interop-ipsec/scenarios/` | strongSwan | after AC-3, a missing Ze-side XFRM SA FAILS the scenario instead of logging a pass | **AC-3 met; scenario RED for an unrelated, earlier reason** (EAP-TLS auth, A-10) |
| `ipsec-bgp-redistribute-frr` | `test/interop-ipsec/scenarios/` | strongSwan + FRR | same, plus the BGP redistribute assertion is unaffected | **AC-3 met and it DISCRIMINATED: the un-guarded assertion is what goes red** (A-9). BGP steps 1 and 4 passed |

The three scenario rows dated 2026-08-01 above preserve the original runs.
They do not certify the native migration. Current product failures remain
owned until fixed or transferred to a named live spec under the normal scope
rules. Scheduling a truthful red job does not turn it green or complete this
spec's execution evidence.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/le/rfc/carriers.go` and its tests, only if the AC review finds a defect in native action parsing or tier derivation.
- `.github/workflows/evidence-nightly.yml`, only for a measured runner defect; retain all delivered callers and visible job failures.
- `internal/le/interoplab/ipsec/`, `l2tp/`, `pppoe/` and their fixtures, only for measured checker/runner defects or missing original proofs.
- `docs/labs/pppoe-interop.md`, `docs/labs/l2tp-interop.md`, `ai/rules/testing.md` and `docs/functional-tests.md` - correct any remaining claim that disagrees with the native preflight or carrier owner. The Darwin module observations are dated evidence.
- `ai/RFC-REQUIREMENTS.md` - regenerate through the native writer when implementation evidence changes.

## Files to Create
- None. Every file this spec needs already exists.

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling change; no config surface |
| YANG validation constraints | N-A | no YANG leaf added |
| YANG custom validators | N-A | no YANG leaf added |
| CLI commands/flags | N-A | no CLI surface; entry point is an existing make target |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no YANG leaf added |
| Functional test for new RPC/API | N-A | no RPC or API added |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | no env var added |
| Doctor check for runtime dependencies | N-A | the new dependency (Docker, host kernel modules) belongs to a CI runner, not to the ze daemon. `ze doctor` describes daemon readiness |
| Prometheus counters/metrics | N-A | no runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | no protocol change |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | evidence tiering is developer-facing tooling |
| 2 | Config syntax changed? | No | no config touched |
| 3 | CLI command added/changed? | No | no command touched |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/labs/pppoe-interop.md` - the stale module claim |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no requirement changes status here. This spec makes a tier AVAILABLE; binding a requirement to it is the rfcgate-1b RFC 7296 pilot spec |
| 10 | Test infrastructure changed? | Yes | `ai/rules/testing.md` carrier table; `docs/functional-tests.md` if it names the refused trees |
| 11 | Affects daemon comparison? | No | no capability change |
| 12 | Internal architecture changed? | No | one derivation added inside an existing gate |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` and `ai/` for anchors naming `rfc_requirements.py` and the three trees; `ai/RFC-REQUIREMENTS.md` names them as having no automated caller and is regenerated |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/labs/*.md` show the make targets; verify each still exists after the change |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. Reconcile each AC with the delivered native producer and existing tests.
   Preserve the original positive EAP/ESP obligation alongside the current
   negative TLS 1.2 scenario; name any missing proof before implementation.
2. Exercise the current IPsec, L2TP and PPPoE runners on the required Linux
   host, including missing-prerequisite refusal and discriminating assertion
   failures. Record the scenario and revision behind every result.
3. Prove workflow removal downgrades the carrier and the HEAD comparison sees
   the evidence loss. Use native action identities and the current Go scanner.
4. Correct only measured gaps, preserve failure visibility, and reconcile
   carrier documentation and evidence. Any unresolved obligation stays open
   here or receives an approved live destination before closure.

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All three carriers have current execution proof; every original scenario obligation is preserved |
| Feature completeness | Native tags are scanned, classified and backed by the checker the scheduled action executes |
| Fail-closed | Missing workflow or lab prerequisites refuse the run, and no failed assertion is converted to success |
| Discrimination | Both the positive XFRM proof and the negative EAP-TLS refusal fail under their own meaningful breaks |
| Ratchet safety | Evidence-loss checks compare the current tree with HEAD's own workflow population |
| Derivation | No native interop tier is hand-assigned independently of a scheduled action |
| Failure visibility | Advisory jobs remain outside merge requirements but report their own failure |
| Rule: `ai/rules/testing.md` | The rule's carrier table and the code agree after the change. The rule is the published contract; a stale row there is a false promise |
| Rule: `ai/rules/evidence.md` | The workflow reader is the ONLY place a tier is decided; `ai/rules/testing.md` describes it rather than re-listing it |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| Scheduled callers retain visible failures | workflow review plus current job results |
| Native tags resolve to the right runner | carrier tests and requirement-to-checker inventory |
| Original positive and negative scenarios discriminate | current scenario runs and recorded meaningful breaks |
| Job removal removes its tier and reports evidence loss | AC-6 regression evidence |
| Final evidence and documentation agree | native RFC gate and documentation review at closure |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | `.github/workflows/*.yml` is repo-controlled, not untrusted input. The reader must still not `eval` or shell out on its contents; parse as text like the Go side does |
| Fail open | The whole point. A reader that returns an empty set on error would silently downgrade every carrier to `unrun` (loud, safe) - but one that returns "all targets" on error would upgrade every carrier (silent, unsafe). Assert the direction in a test |
| Privileged containers in CI | The ipsec lab runs `--privileged`. That is on a hosted ephemeral runner against repo-controlled Dockerfiles, and is the same posture the BGP job already has with `--cap-add NET_ADMIN` |
| Error leakage | The refusal message names files and make targets, never secrets |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

- **A tier has two preconditions, and the rule only names one.** `ai/rules/testing.md`
  says a tag is evidence when something EXECUTES the test. Necessary, not
  sufficient: the test must also be able to FAIL. `test/interop-ipsec/` would
  have satisfied the written rule while two of its twelve scenarios reported a
  pass on a thrown assertion. The rule's wording is worth widening at closure.
- **The refusal was load-bearing in an unadvertised way.** Because the tree was
  refused, nobody noticed the fail-open checks. Removing the refusal without
  reading the checks would have converted a visible blocker into an invisible
  false positive.
- The original 2026-08-01 carrier table asserted interop tiers as literals.
  `interopCarriers` now derives them from scheduled native actions. The old
  asymmetry records the motivation for the change.
- **The measured host facts contradict two docs.** Docker Desktop on this Darwin
  host has `pppoe` and `/dev/ppp` but not `l2tp_ppp`, and the ipsec lab's Ze-side
  XFRM works. Three separate comments and doc lines say otherwise.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Derive the interop tier from `.github/workflows/*.yml` | Keep the literal and just flip `TIER_UNRUN`→`TIER_NIGHTLY` for ipsec | The literal is a claim nobody checks. Flipping it would grant a tier that survives deleting the job. Deriving fixes `interop-bgp`'s identical weakness in the same change (`ai/rules/evidence.md`) |
| Fix the two fail-open checks BEFORE granting the tier | Grant the tier now, fix the checks in a follow-up | A tier granted to a vacuous check is exactly the false evidence the `unrun` refusal exists to prevent. Ordering costs nothing and the reverse is unsafe (R-1) |
| The original L2TP/PPPoE hold is superseded by delivered scheduled callers | Restore the missing-caller blocker | Current workflow source contains both actions; AC-4/AC-5 retain the independent execution-proof obligation |
| A platform run must execute the tagged native checker | Credit a similarly named deployment action without tracing it | Runner identity and scenario execution must agree |
| One native action parser supplies tier derivation | Restore paired Python/Go make-target extractors | The migration retired the Python path; AC-7 preserves its intended agreement with the executed action |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- AC-4 and AC-5 remain unfinished until the current hosted L2TP/PPPoE results
  and discrimination evidence are recorded. Their scheduled callers are
  delivered; this reconciliation records no fresh execution or pass.
- `interop/nightly` is advisory evidence by construction. It never gates a merge,
  and the ledger keeps it in a separate rollup. A requirement whose only proof is
  an interop scenario is proven nightly, not on every push. That is a property of
  the tier, not a gap in this work.
- Unscheduled native interop trees remain `unrun`. Retired `check.py` paths
  cannot stand in for an executable native checker or establish current proof.

## RFC Documentation (Scope: protocol)

N-A. Scope is tooling. This spec changes which carriers may hold an
`RFC requirement:` tag; it implements no protocol behavior and quotes no RFC.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
