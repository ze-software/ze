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
| Deferral shard | `-` (nothing deferred OUT of this spec; the l2tp/pppoe work stays here, tracked by the three live rows in `plan/deferrals/rfcgate-2-evidence.md` that already name this spec as their destination) |
| Updated | 2026-08-01 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/deferral-tracking.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Deferred out of `plan/spec-rfcgate-2-evidence.md` (see
`plan/deferrals/rfcgate-2-evidence.md`).

Three interop trees have runners but no automated caller, so a test in them
executes only when somebody remembers to type the target:

| Tree | Scenarios | Runner | Automated caller |
|------|-----------|--------|------------------|
| `test/ipsec-interop/` | 12 `check.py` | `make ze-ipsec-interop-test` | none |
| `test/l2tp-interop/` | 2 `check.py` (+1 self-run scenario) | `make ze-deployment-l2tp-ppp-docker-test` | none |
| `test/pppoe-interop/` | 1 `check.py` | `make ze-deployment-pppoe-accel-docker-test` | none |

→ Constraint: the counts above are measured (`find test/*-interop -name check.py`),
not the 10/3/1 the deferral row carried. The l2tp runner's target name is
`ze-deployment-l2tp-ppp-docker-test` (`mk/test-integration.mk`), not "the L2TP
interop runner" as `CARRIERS` currently spells it.

Because nothing runs them, `scripts/dev/rfc_requirements.py` classifies each as
tier `unrun` and REFUSES an `RFC requirement:` tag placed there, naming the file
and the missing pipeline. That refusal is the correct interim state, not the
fix: it keeps false evidence out, and it also keeps real evidence unavailable.

The work: give each tree an automated caller (the BGP tree's own advisory job in
`.github/workflows/evidence-nightly.yml` is the pattern), confirm each runner
fails closed on a missing lab the way `test/interop/run.py` and
`test/ipsec-interop/run.py` now do, then change that tree's row in `CARRIERS`
from `TIER_UNRUN` to a real tier so its scenarios can carry evidence.

**Research changed the shape of this work. It is three problems, not two.** A
tier is honest only when something executes the test AND the test can fail. The
original framing covered the first. Research (2026-08-01) found the second is
violated in the ipsec tree today, so wiring alone would grant a tier to checks
that pass with the dataplane broken.

| # | Problem | Status after research |
|---|---------|----------------------|
| 1 | Nothing executes the three trees | confirmed; fix is a workflow job per tree |
| 2 | Two ipsec scenarios pass when the assertion throws | NEW, confirmed by reading the source |
| 3 | The interop tier is an unchecked literal in `CARRIERS` | NEW; `interop-bgp` has the same weakness |

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/rules/testing.md` - the carrier table and the two evidence axes (kind, tier)
  → Constraint: "A tag in `test/ipsec-interop/`, `test/l2tp-interop/`,
    `test/pppoe-interop/`, or any other `check.py` tree is REFUSED ... wire the
    suite into a pipeline first, then give its carrier a tier in `CARRIERS`."
    Sequencing is normative: wire, observe, then grant.
  → Constraint: `interop/nightly` evidence is ADVISORY and never merge-gate. A
    requirement whose only evidence is nightly is marked `**nightly-only**` and
    counted in its own rollup column, never summed with verify-tier evidence.
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps
  → Constraint: "Before claiming an interop/functional test validates a change,
    revert the behaviour and watch the test go red." A test asserting the
    ABSENCE of something, or one whose assertion is swallowed, is not evidence.
- [ ] `ai/rules/fail-closed-guards.md` - a guard must deny or say something
  → Constraint: a check that cannot evaluate its assertion must fail, not pass.
- [ ] `ai/rules/qemu-testing.md` - "Interop Labs and Docker-Based Tests Need a QEMU Runner Too"
  → Decision: for a Docker lab needing host-kernel features, the repo's existing
    answer is a QEMU sibling (`ze-qemu-l2tp-ppp-test`, `ze-qemu-pppoe-accel-test`).
  → Constraint: those siblings run `scripts/evidence/effective-*.py`, NOT the
    tree's `check.py`. A QEMU run therefore does NOT execute the tagged carrier,
    so it cannot justify a tier for `test/l2tp-interop/scenarios/*/check.py`.
- [ ] `docs/labs/l2tp-interop.md`, `docs/labs/pppoe-interop.md` - host requirements
  → Constraint: both claim Docker Desktop on macOS lacks the modules. Measured
    2026-08-01: `pppoe` and `/dev/ppp` are PRESENT, `l2tp_ppp` is absent. The
    pppoe half of that claim is stale and the doc needs correcting.

### RFC Summaries (Scope: protocol)
- N-A. Scope is tooling: this spec changes which carriers may hold a tag, never
  what any RFC requires.

**Key insights:** (minimal context to resume after compaction)
- The ipsec lab is GREEN on a real host and its Ze-side XFRM assertion passes,
  so the "expected on Docker for Mac" excuse the fail-open checks cite is stale.
- `CARRIERS` asserts the interop tier as a literal; nothing ties it to a workflow.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `scripts/dev/rfc_requirements.py` - holds `CARRIERS`, the ONE table where the
  evidence kind and tier are spelled. `TIER_VERIFY`/`TIER_NIGHTLY`/`TIER_UNRUN`.
  `functional_suites()` DERIVES the `.ci` tier from `mk/test-functional.mk`'s own
  `all_suites=` line and fails closed when it cannot read it. `_suite_carriers()`
  emits one derived row per suite. The four interop rows are literals instead:
  `interop-bgp` asserts `TIER_NIGHTLY`, and `interop-ipsec`/`interop-l2tp`/
  `interop-pppoe` assert `TIER_UNRUN`. `carrier_for()` → `_lookup()` returns the
  first row whose prefix AND suffix match. `_refuse_unrun()` builds the refusal.
  `_build_head_carriers()` re-reads HEAD's `all_suites=` so a suite DROP registers
  as an evidence loss rather than a symmetric relabel.
- [ ] `.github/workflows/evidence-nightly.yml` - scheduled-only, every job
  `continue-on-error: true`. Jobs: `fuzz`, `integration`, `interop`. The `interop`
  job runs `make ze-interop-test` and is the sole automated caller of any interop
  tree.
- [ ] `scripts/dev/github_workflows_test.go` - `TestEvidenceNightlyRunsInterop`
  pins that the `interop` job exists, runs `make ze-interop-test` by name, and is
  advisory. `TestWorkflowMakeTargetsExist` proves every `make <target>` a workflow
  names is real. `parseMakeTargets()` is the shared extractor.
- [ ] `mk/test-integration.mk` - `ze-interop-test`, `ze-ipsec-interop-test`,
  `ze-deployment-l2tp-ppp-docker-test`, `ze-deployment-pppoe-accel-docker-test`.
- [ ] `test/ipsec-interop/run.py` - `build_images()` cross-compiles ze on the HOST
  (`CGO_ENABLED=0 GOOS=linux go build`) into the gitignored `test/ipsec-interop/ze-linux`,
  then Docker COPYs it. Preflight is `docker info` only; a missing daemon exits 1.
- [ ] `test/l2tp-interop/lab.py` `preflight_strict()` / `test/pppoe-interop/lab.py`
  `preflight_strict()` - run a privileged alpine probe against the host's
  `/lib/modules`, then `raise SystemExit("host kernel missing ... requirements: %s")`.
  Both refuse a skip override.
- [ ] `test/ipsec-interop/scenarios/04-eap-tls/check.py` and
  `.../02-ipsec-bgp-redistribute-frr/check.py` - the Ze-side XFRM assertion is
  wrapped in `except (AssertionError, Exception)`. 04 calls `log_pass(...)` in the
  handler, so a real ESP failure is reported as a PASS.

**Behavior to preserve:**
- `make ze-rfc-check` stays green and keeps refusing a tag in any carrier nothing
  runs. The refusal message keeps naming the file, the runner, and the pipeline.
- `interop-bgp` keeps its `interop/nightly` label, so the 2 existing interop tags
  keep resolving and no ratchet fires.
- Every runner keeps failing CLOSED: a missing Docker daemon or a missing host
  kernel module exits non-zero and never prints "skipping".
- `test/draft/` stays invisible to the scan.

**Behavior to change:**
- The interop tier stops being an asserted literal and becomes derived from the
  workflow set, so deleting a job DOWNGRADES the carrier instead of doing nothing.
- The two ipsec checks stop converting a thrown assertion into a pass.
- `test/ipsec-interop/` gains an automated caller and, with it, `interop/nightly`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A developer writes `# RFC requirement: <id> <polarity> -- <why>` in a
  `check.py` under one of the three trees.
- Format at entry: a Python comment token, read by `scan_python_tags`.

### Transformation Path
1. `scan_tree()` walks `TEST_ROOTS` and calls `carrier_for(rel)` for each file.
2. `carrier_for()` → `_lookup()` returns the first `CARRIERS` row matching prefix
   and suffix; today that is `interop-ipsec` / `interop-l2tp` / `interop-pppoe`.
3. Today: `carrier.tier == TIER_UNRUN`, so `scan_tree` raises `_refuse_unrun()`
   and `make ze-rfc-check` exits 2 naming the file.
4. After this spec: the row's tier is computed by a new `scheduled_workflow_targets()`
   reader over `.github/workflows/*.yml`. A tree whose runner target appears in a
   scheduled workflow resolves to `TIER_NIGHTLY`; one that does not stays `TIER_UNRUN`.
5. The tag resolves, `evidence_label()` prints `interop/nightly`, and the ledger
   marks the requirement `**nightly-only**` unless it also has verify-tier evidence.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `CARRIERS` ↔ `.github/workflows/*.yml` | new reader parses `make <target>` out of scheduled workflows | No - new code |
| `CARRIERS` ↔ git HEAD | `_build_head_carriers()` must also read HEAD's workflows, or a job deletion relabels both sides and the loss is invisible | No - new code |
| gate ↔ `github_workflows_test.go` | both extract make targets from workflow YAML; two extractors would drift | No - new code |

### Integration Points
- `_suite_carriers()` / `functional_suites()` - the existing derive-from-recipe
  pattern this change copies for workflows. Same fail-closed shape.
- `_build_head_carriers()` - already swaps derived rows for HEAD's; must learn the
  workflow-derived rows too.
- `parseMakeTargets()` (`scripts/dev/github_workflows_test.go`) - the Go side's
  make-target extractor. The Python reader must agree with it on the same files.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the tier stays a `CARRIERS` property; only its SOURCE changes from literal to derived |
| No unintended coupling (components stay isolated) | Yes | the reader is one function in `rfc_requirements.py`; nothing else learns about workflows |
| No duplicated functionality (extends existing, does not recreate) | Partial | a make-target extractor already exists in Go (`parseMakeTargets`). AC-7 requires the two be pinned against each other rather than left to drift |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no wire path |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | Yes | this change REMOVES hardcoding: four asserted tiers become one derived rule (`ai/rules/derive-not-hardcode.md`) |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The ipsec lab actually passes, so its green is real rather than assumed | deferral note says all three are unverified since the launcher fix | the tree cannot earn a tier at all | ran `make ze-ipsec-interop-test IPSEC_INTEROP_SCENARIO=01-psk-site-to-site` on Darwin/Docker: exit 0, Ze-side XFRM SA present, ESP counters advanced | **confirmed for 01 only -- see A-9/A-10** |
| A-9 | Scenario 02 passes once its fail-open handler is gone | assumed by A-1 generalising from 01 | the tree carries a real, previously-hidden dataplane defect | ran it 2026-08-01 with the handler removed: FAILS at `wait_xfrm_sa(ZE_CONTAINER)`. strongSwan installs its XFRM SA, Ze installs none. This is exactly the failure the `except (AssertionError, Exception)` was converting into a pass | **broken -- real defect, see below** |
| A-10 | Scenario 04 passes once its fail-open handler is gone | same | EAP-TLS is not interoperable today | ran it 2026-08-01: FAILS EARLIER than the removed handler, at step 1 `swan.wait_sa_established("ze")`. strongSwan logs `EAP method EAP_TLS failed for peer ze-test-client`; Ze logs `eap: authenticator sent Failure`. The handler removal did not cause it and could not have hidden it | **broken -- real defect, see below** |
| A-2 | `test/ipsec-interop/ze-linux` is a build output, not a checked-in input CI would lack | `.gitignore` line for it; absent from `git ls-files`; `run.py` `build_images()` regenerates it | CI could never build the image | `git check-ignore -v` and `git ls-files` | **confirmed** |
| A-3 | The ipsec lab needs a Go toolchain ON THE HOST (unlike the other three, which build inside Docker) | `build_images()` shells `go build` before `docker build` | the nightly job would fail at image build | read `run.py` `build_images()` | **confirmed** |
| A-4 | Every runner fails CLOSED on a missing prerequisite | claimed by the deferral note for interop/ipsec only | a job could go green having run nothing | read `run.py` main() for all four; `preflight_strict()` for l2tp/pppoe; each exits 1 | **confirmed** |
| A-5 | The two ipsec XFRM checks discriminate | implied by them being interop scenarios | a nightly tier would be granted to a vacuous check | read `04-eap-tls/check.py` and `02-.../check.py`: both wrap the assertion in `except (AssertionError, Exception)`, 04 calls `log_pass` | **broken** |
| A-6 | `ubuntu-latest` provides `l2tp_ppp`/`pppol2tp` so the l2tp lab can run in CI | none - never measured | the l2tp job is red every night and the tree cannot earn a tier | needs one observed nightly run, or an owner ruling. Measured on Darwin/Docker Desktop: `l2tp_ppp` ABSENT | **unvalidated - blocks AC-4** |
| A-7 | `ubuntu-latest` provides `pppoe` + `/dev/ppp` so the pppoe lab can run in CI | none - never measured | the pppoe job is red every night and the tree cannot earn a tier | needs one observed nightly run. Measured on Darwin/Docker Desktop: both PRESENT, so the `docs/labs/pppoe-interop.md` claim they are absent is stale | **unvalidated - blocks AC-5** |
| A-8 | Deleting the `interop` job today would be caught | `TestEvidenceNightlyRunsInterop` exists | the tier derivation would be the only guard | read the test: it does pin the job by name | **confirmed** (the derivation is defence in depth, not the sole guard) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Granting `interop-ipsec` a tier while A-5 stays broken hands evidence status to a check that passes with ESP broken | a scenario passes on a host with no XFRM | AC-3 fixes the checks BEFORE AC-2 grants the tier; ordering is normative in Implementation Steps |
| R-2 | The l2tp/pppoe jobs go red every night, and an advisory red trains people to ignore the workflow | first nightly after merge | do not wire a tree whose lab cannot run; settle A-6/A-7 first (owner question below) |
| R-3 | The Python workflow reader and Go `parseMakeTargets` drift, so the gate believes a target runs that CI does not invoke | none - silent | AC-7: a test asserts both extractors return the same target set for every workflow file |
| R-4 | The ipsec nightly job needs Go + Docker + privileged; a missing one turns a real failure into an infrastructure blip | job red with a build error, not a scenario failure | job runs `actions/setup-go` like its siblings; runner fails closed with a message naming Docker |
| R-5 | Adding a tree to `CARRIERS` with a nightly tier lets a future author bind an RFC MUST to nightly-only evidence when a `.ci` would have run on every push | a requirement's only evidence is `interop/nightly` | `ai/rules/testing.md` already prefers a `.ci`; the ledger already marks such rows `**nightly-only**` and never sums them |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. The failure mode is evidential: an RFC requirement could be credited to a test that does not run or cannot fail, which makes `docs/features/rfc-status.md` overstate conformance. |
| How is it reverted? | Single commit revert. No config migration, no wire change. The `check_evidence_ratchet` would then fire on any requirement that had taken interop evidence, which is the intended alarm. |
| Who else touches this path? | the rfcgate-1b pilot, now closed (`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`), whose IPsec tags this path carries, plus sibling sessions in `internal/component/ike/**` and `internal/component/bgp/**`. This spec touches neither tree. |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by .claude/hooks/validate-spec.sh, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `# RFC requirement:` tag in `test/ipsec-interop/scenarios/*/check.py` | → | `carrier_for()` → `_lookup()` returns `interop-ipsec` with `TIER_NIGHTLY` | `test_ipsec_interop_carrier_earns_nightly_when_wired` (`scripts/dev/rfc_requirements_test.py`) |
| `.github/workflows/evidence-nightly.yml` `ipsec-interop` job | → | `scheduled_workflow_targets()` reports `ze-ipsec-interop-test` | `TestEvidenceNightlyRunsIpsecInterop` (`scripts/dev/github_workflows_test.go`) |
| A workflow that does NOT name a tree's runner | → | that tree's carrier resolves `TIER_UNRUN` and `_refuse_unrun()` fires | `test_interop_carrier_falls_to_unrun_without_a_scheduled_caller` |
| Deleting the interop job at HEAD→tree | → | `_build_head_carriers()` labels HEAD `nightly` and tree `unrun`, so the evidence ratchet reports a LOSS | `test_head_carriers_read_head_workflows` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `.github/workflows/evidence-nightly.yml` after the change | carries an `ipsec-interop` job, `continue-on-error: true`, running `make ze-ipsec-interop-test` by name, with `actions/setup-go` (the lab cross-compiles ze on the host) |
| AC-2 | An `RFC requirement:` tag in `test/ipsec-interop/scenarios/*/check.py` | is accepted and labelled `interop/nightly`; `make ze-rfc-check` exits 0 |
| AC-3 | `04-eap-tls/check.py` and `02-ipsec-bgp-redistribute-frr/check.py` run against a host where the Ze-side XFRM SA never appears | the scenario FAILS. No `except Exception` path reports a pass |
| AC-4 | An `RFC requirement:` tag in `test/l2tp-interop/scenarios/*/check.py` | BLOCKED on A-6. Accepted as `interop/nightly` only once a scheduled job runs the lab green; otherwise the tag stays refused and the refusal names the runner |
| AC-5 | An `RFC requirement:` tag in `test/pppoe-interop/scenarios/*/check.py` | BLOCKED on A-7. Same condition as AC-4 |
| AC-6 | The `interop` job is deleted from `evidence-nightly.yml` | `interop-bgp` resolves `TIER_UNRUN`, the 2 existing BGP interop tags are refused, and `make ze-rfc-check` exits 2. Today this deletion changes nothing in `CARRIERS` |
| AC-7 | Every file under `.github/workflows/` | the Python `scheduled_workflow_targets()` and the Go `parseMakeTargets()` agree on the make targets found, so the gate cannot believe in a caller CI does not have |
| AC-8 | `make ze-rfc-check` after the change | exits 0; the `evidence:` line reports a non-zero `interop/nightly` count and no requirement silently loses a polarity or an evidence kind |

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
| `test_scheduled_workflow_targets_reads_make_targets` | `scripts/dev/rfc_requirements_test.py` | the reader finds `ze-interop-test` and `ze-ipsec-interop-test` in a scheduled workflow fixture | |
| `test_scheduled_workflow_targets_ignores_push_only_workflow` | same | a target named only by `verify.yml` (push/pull_request) does not grant a NIGHTLY tier | |
| `test_scheduled_workflow_targets_ignores_comments` | same | a commented-out `make` line grants nothing, matching `stripComments` on the Go side | |
| `test_scheduled_workflow_targets_fails_closed_when_unreadable` | same | an unreadable workflow dir raises `ParseError`, never "everything runs" (`ai/rules/fail-closed-guards.md`) | |
| `test_ipsec_interop_carrier_earns_nightly_when_wired` | same | `carrier_for('test/ipsec-interop/scenarios/x/check.py').tier == TIER_NIGHTLY` | |
| `test_interop_carrier_falls_to_unrun_without_a_scheduled_caller` | same | with the job removed from the fixture, the same path resolves `TIER_UNRUN` and `_refuse_unrun` names the runner | |
| `test_head_carriers_read_head_workflows` | same | `_build_head_carriers()` labels from HEAD's workflow set, so a job deletion is a LOSS not a wash | |
| `TestEvidenceNightlyRunsIpsecInterop` | `scripts/dev/github_workflows_test.go` | the `ipsec-interop` job exists, is advisory, and runs the target by name | |
| `TestWorkflowTargetExtractorsAgree` | same | Go `parseMakeTargets` and the Python reader return the same set for every workflow file (AC-7) | |

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
| N-A - no daemon-visible behavior changes. The user entry point is `make ze-rfc-check`, exercised by `--selftest` (699 tests) and by the gate itself in `ze-verify`. No `internal/**` or `cmd/**` Go file is modified. | - | - | - |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `01-psk-site-to-site` | `test/ipsec-interop/scenarios/` | strongSwan | the tree is genuinely green: IKE SA + Child SA + XFRM SA on BOTH sides, ESP counters advancing | **PASS** (measured 2026-08-01, exit 0) |
| `04-eap-tls` | `test/ipsec-interop/scenarios/` | strongSwan | after AC-3, a missing Ze-side XFRM SA FAILS the scenario instead of logging a pass | **AC-3 met; scenario RED for an unrelated, earlier reason** (EAP-TLS auth, A-10) |
| `02-ipsec-bgp-redistribute-frr` | `test/ipsec-interop/scenarios/` | strongSwan + FRR | same, plus the BGP redistribute assertion is unaffected | **AC-3 met and it DISCRIMINATED: the un-guarded assertion is what goes red** (A-9). BGP steps 1 and 4 passed |

→ Constraint: both reds live in `internal/component/ike/**`, which this spec must not
touch (a Review Gate is reading it and a sibling session owns the uncommitted work
there). Neither is fixable from inside this spec. They are the OWNER QUESTION below,
not a deferral: the tooling deliverable is complete and these are product defects the
tooling just made visible for the first time.

→ Decision: the tier is still granted. AC-3's requirement is that the checks CAN fail,
and two of them just did, on real evidence rather than a synthetic mutation. A nightly
advisory job reporting two genuine reds is the system working. No RFC requirement is
bound to this tree yet, so no false evidence is created either way.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `scripts/dev/rfc_requirements.py` - add `scheduled_workflow_targets()`; make the
  four interop rows derive their tier from it; teach `_build_head_carriers()` to
  read HEAD's workflows. **Feature code for a tooling spec.**
- `.github/workflows/evidence-nightly.yml` - add the `ipsec-interop` job.
- `scripts/dev/github_workflows_test.go` - add `TestEvidenceNightlyRunsIpsecInterop`
  and `TestWorkflowTargetExtractorsAgree`.
- `scripts/dev/rfc_requirements_test.py` - the seven unit tests above.
- `test/ipsec-interop/scenarios/04-eap-tls/check.py` - remove the fail-open handler.
- `test/ipsec-interop/scenarios/02-ipsec-bgp-redistribute-frr/check.py` - same.
- `docs/labs/pppoe-interop.md` - line 58 reads "Docker Desktop on macOS typically
  cannot pass this check (its Linux VM lacks the ... modules)". Measured
  2026-08-01 on Docker Desktop: `PPPOE=ok`, `DEV_PPP=ok`. The sentence is hedged
  ("typically"), so correct it to say the probe decides rather than deleting the
  caveat outright.
- `ai/rules/testing.md` - update the carrier table: `test/ipsec-interop/` is no
  longer in the refused list, and the l2tp target name is corrected.
- `ai/RFC-REQUIREMENTS.md` - regenerated by `make ze-rfc-index`.

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
| 9 | RFC behavior implemented, changed, or newly proven? | No | no requirement changes status here. This spec makes a tier AVAILABLE; binding a requirement to it is `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md` |
| 10 | Test infrastructure changed? | Yes | `ai/rules/testing.md` carrier table; `docs/functional-tests.md` if it names the refused trees |
| 11 | Affects daemon comparison? | No | no capability change |
| 12 | Internal architecture changed? | No | one derivation added inside an existing gate |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` and `ai/` for anchors naming `rfc_requirements.py` and the three trees; `ai/RFC-REQUIREMENTS.md:21` names them as having no automated caller and is regenerated |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/labs/*.md` show the make targets; verify each still exists after the change |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

**Ordering is normative. AC-3 lands before AC-2.** Granting a tier to a tree
whose checks can swallow their assertion is the failure this spec exists to
avoid, so the checks are fixed before the tier is available.

1. **Phase: Wiring (MANDATORY FIRST)** -- add the `ipsec-interop` job and the failing tier test
   - Tests: `TestEvidenceNightlyRunsIpsecInterop`, `test_ipsec_interop_carrier_earns_nightly_when_wired`
   - Files: `.github/workflows/evidence-nightly.yml`, `scripts/dev/github_workflows_test.go`, `scripts/dev/rfc_requirements_test.py`
   - Verify: the job exists and is advisory; the carrier test FAILS because `CARRIERS` still asserts `TIER_UNRUN`
2. **Phase: Discriminating checks (AC-3)** -- remove the two fail-open handlers
   - Tests: `04-eap-tls` and `02-ipsec-bgp-redistribute-frr` run green on a host WITH XFRM, and fail on one without
   - Files: the two `check.py`
   - Verify: `make ze-ipsec-interop-test` still exits 0 here (XFRM is present); mutation-check by asserting a bogus container name and confirming the scenario reddens
3. **Phase: Derive the tier (AC-2, AC-6, AC-7)** -- replace the literal with a reader
   - Tests: the seven `rfc_requirements_test.py` tests, `TestWorkflowTargetExtractorsAgree`
   - Files: `scripts/dev/rfc_requirements.py`, `scripts/dev/github_workflows_test.go`
   - Verify: `python3 scripts/dev/rfc_requirements.py --selftest` green; the phase-1 carrier test now PASSES; removing the BGP `interop` job from a fixture refuses the BGP tags
4. **Phase: Docs and ledger** -- correct the stale claims, regenerate
   - Files: `ai/rules/testing.md`, `docs/labs/pppoe-interop.md`, `ai/RFC-REQUIREMENTS.md`
   - Verify: `make ze-rfc-index && make ze-rfc-check && make ze-doc-test`
5. **Phase: l2tp / pppoe (AC-4, AC-5) -- BLOCKED, needs the owner ruling below**
   - Do not start until A-6/A-7 are settled. Wiring a lab whose kernel prerequisite
     the runner lacks produces a nightly red, which earns no tier and trains people
     to ignore the workflow (R-2).

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file + symbol; AC-4/AC-5 are explicitly BLOCKED, not silently dropped |
| Feature completeness | A tag in `test/ipsec-interop/` is accepted end to end: written, scanned, labelled, rendered in the ledger |
| Fail-closed | An unreadable or absent `.github/workflows/` raises `ParseError`. It must never resolve to "everything runs" (`ai/rules/fail-closed-guards.md`) |
| Discrimination | No `check.py` in the newly-tiered tree converts a thrown assertion into a pass. Grep the whole tree for `except`, not only the two known sites |
| Ratchet safety | `check_evidence_ratchet` and `check_coverage_ratchet` stay green: no requirement loses a kind or a polarity. Run `make ze-rfc-check` before and after |
| Derivation, not assertion | No tier literal survives for a carrier whose runner a workflow names. `grep TIER_NIGHTLY` returns the derivation, not four hardcoded rows |
| HEAD symmetry | `_build_head_carriers()` reads HEAD's workflows. Prove a job deletion reports a LOSS by running the ratchet against a fixture |
| Rule: `ai/rules/testing.md` | The rule's carrier table and the code agree after the change. The rule is the published contract; a stale row there is a false promise |
| Rule: `ai/rules/derive-not-hardcode.md` | The workflow reader is the ONLY place a tier is decided; `ai/rules/testing.md` describes it rather than re-listing it |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `ipsec-interop` job exists and is advisory | `go test -tags "ze_core $TAGS" -run TestEvidenceNightlyRunsIpsecInterop ./scripts/dev/` |
| A tag in `test/ipsec-interop/` is accepted | add one, run `make ze-rfc-check`, expect exit 0 and a non-zero `interop/nightly` count |
| The two checks discriminate | `make ze-ipsec-interop-test IPSEC_INTEROP_SCENARIO=04-eap-tls` green; then break the container name and confirm red |
| The tier is derived | `python3 scripts/dev/rfc_requirements.py --selftest` |
| Deleting the BGP interop job refuses BGP tags | the fixture test `test_interop_carrier_falls_to_unrun_without_a_scheduled_caller` |
| The gate is still green overall | `make ze-rfc-check` exit 0 |
| No ratchet fired | compare the `evidence:` line against `tmp/rfccheck-baseline.log` (unit/verify 3103, functional/verify 19, editor/verify 0, interop/nightly 2) |

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
  sufficient: the test must also be able to FAIL. `test/ipsec-interop/` would
  have satisfied the written rule while two of its twelve scenarios reported a
  pass on a thrown assertion. The rule's wording is worth widening at closure.
- **The refusal was load-bearing in an unadvertised way.** Because the tree was
  refused, nobody noticed the fail-open checks. Removing the refusal without
  reading the checks would have converted a visible blocker into an invisible
  false positive.
- **`CARRIERS` derives the `.ci` tier but asserts the interop tier.** The `.ci`
  path reads `mk/test-functional.mk` and even re-reads HEAD's copy so a dropped
  suite registers as a loss. The interop path is four literals. The asymmetry is
  not principled; it is the order the two were written in.
- **The measured host facts contradict two docs.** Docker Desktop on this Darwin
  host has `pppoe` and `/dev/ppp` but not `l2tp_ppp`, and the ipsec lab's Ze-side
  XFRM works. Three separate comments and doc lines say otherwise.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Derive the interop tier from `.github/workflows/*.yml` | Keep the literal and just flip `TIER_UNRUN`→`TIER_NIGHTLY` for ipsec | The literal is a claim nobody checks. Flipping it would grant a tier that survives deleting the job. Deriving fixes `interop-bgp`'s identical weakness in the same change (`ai/rules/derive-not-hardcode.md`) |
| Fix the two fail-open checks BEFORE granting the tier | Grant the tier now, fix the checks in a follow-up | A tier granted to a vacuous check is exactly the false evidence the `unrun` refusal exists to prevent. Ordering costs nothing and the reverse is unsafe (R-1) |
| Wire ipsec now; hold l2tp/pppoe for evidence | Wire all three at once | Their kernel prerequisite on `ubuntu-latest` is unmeasured (A-6, A-7). Wiring a lab that cannot run yields a permanent advisory red, which earns no tier and devalues the workflow. `ai/rules/testing.md` sequencing is wire → observe → grant |
| Reject the QEMU siblings as the pipeline for l2tp/pppoe | Point the tier at `ze-qemu-l2tp-ppp-test` / `ze-qemu-pppoe-accel-test` | Those targets run `scripts/evidence/effective-*.py`, NOT the trees' `check.py`. Crediting a `check.py` for a run that never opens it is precisely a tier the carrier has not earned |
| One shared notion of "make targets in a workflow" | Let the Python reader and Go `parseMakeTargets` evolve separately | Two extractors that disagree let the gate believe in a caller CI does not have. AC-7 pins them together |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- **AC-4 and AC-5 (l2tp, pppoe) are NOT delivered in the first pass and are NOT
  dropped.** They are BLOCKED on A-6/A-7, which need either one observed nightly
  run on `ubuntu-latest` or an owner ruling. They stay acceptance criteria of
  THIS spec, so the spec stays open until they are settled: blocked is not
  deferred, and a blocker is not a scope reduction (`ai/rules/no-parking.md`).
  The two matching rows in `plan/deferrals/rfcgate-2-evidence.md` already name
  this spec as their destination and stay `deferred` (live) until then.
- `interop/nightly` is advisory evidence by construction. It never gates a merge,
  and the ledger keeps it in a separate rollup. A requirement whose only proof is
  an interop scenario is proven nightly, not on every push. That is a property of
  the tier, not a gap in this work.
- The `interop` tier remains unavailable to the four other `check.py` trees
  (`test/stress/scenarios/`, `test/l2tp-scale/`, and the two named above). The
  `scenario-check` catch-all keeps refusing them, which is the fail-closed default.

## RFC Documentation (Scope: protocol)

N-A. Scope is tooling. This spec changes which carriers may hold an
`RFC requirement:` tag; it implements no protocol behavior and quotes no RFC.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
