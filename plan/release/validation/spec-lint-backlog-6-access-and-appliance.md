# Spec: lint-backlog-6-access-and-appliance

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | 25 findings of the full lint in IKE, L2TP and the appliance; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4: closure; CI-list fixes landed in `85a33a3887`, `4abd85f6f9` and `34549af4be`; later findings in `7eb86b64a2`; the last 13 rangeValCopy findings in the closure commit |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in IKE and L2TP
(door `ai/doors/access-security.md`) and in the appliance build tooling and
`internal/component/config/system` (door `ai/doors/appliance.md`). By linter:
gosec 10 (all `internal/appliance`), exhaustive 7, errcheck 6, gocritic 1,
modernize 1. Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/appliance/cache.go` | 383 | gosec |
| `internal/appliance/cache.go` | 470 | gosec |
| `internal/appliance/cache.go` | 513 | gosec |
| `internal/appliance/cache.go` | 572 | gosec |
| `internal/appliance/cache.go` | 693 | gosec |
| `internal/appliance/cmd_init.go` | 125 | gosec |
| `internal/appliance/cmd_init.go` | 451 | gosec |
| `internal/appliance/cmd_initrd.go` | 169 | gosec |
| `internal/appliance/cmd_kernel.go` | 658 | gosec |
| `internal/appliance/cmd_kernel.go` | 712 | gosec |
| `internal/component/config/system/doctor.go` | 280 | exhaustive |
| `internal/component/ike/dataplane/rfc3948_policy_owner_test.go` | 377 | errcheck |
| `internal/component/ike/engine/cp_close_test.go` | 46 | errcheck |
| `internal/component/ike/engine/cp_close_test.go` | 81 | errcheck |
| `internal/component/ike/engine/dpd_retransmit_test.go` | 74 | errcheck |
| `internal/component/ike/engine/dpd_retransmit_test.go` | 118 | errcheck |
| `internal/component/ike/engine/eap_auth.go` | 151 | exhaustive |
| `internal/component/ike/engine/mobike_route_linux.go` | 29 | gocritic |
| `internal/component/ike/engine/mobike_route_linux_test.go` | 42 | modernize |
| `internal/component/ike/engine/reconcile_test.go` | 251 | exhaustive |
| `internal/component/ike/engine/rfc4301_sad_lifetime_action_test.go` | 117 | errcheck |
| `internal/component/l2tp/metrics.go` | 189 | exhaustive |
| `internal/component/l2tp/session_fsm.go` | 612 | exhaustive |
| `internal/component/l2tp/session_fsm.go` | 719 | exhaustive |
| `internal/component/l2tp/session_fsm.go` | 837 | exhaustive |

## Required Reading

### Architecture Docs
- [x] `ai/doors/access-security.md` - IKE, L2TP, the subscriber model
  → Constraint: EAP-TLS compiles only against the patched `crypto/tls`; build and test through `./le`, never a plain `go test` of a package that links `internal/core/eap`
  → Constraint: a datagram that fails to decrypt must not kill a live SA; an exhaustive fix in `eap_auth.go` keeps every outcome
- [x] `ai/doors/appliance.md` - the build host tooling
  → Constraint: secrets are never in process arguments; a gosec G204 fix keeps argv free of them
  → Constraint: `internal/appliance` is build-host tooling behind `ze_setup`
- [x] `docs/contributing/ze-go-style.md` - the `exhaustive` C/O/P classification
  → Constraint: transition tables stay C/O; the three cited L2TP switches extract AVP subsets and are P

**Key insights:**
- The three cited `session_fsm.go` switches extract ICRQ, ICCN and OCRQ fields after generic AVP validation. They already carry reasoned partial-switch annotations; no transition changes are needed.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/component/l2tp/session_fsm.go` - three enum switches `exhaustive` reports
- [x] `internal/appliance/cache.go` - five gosec sites
- [x] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Every IKE, L2TP and appliance behavior; FSM transitions unchanged.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope over every flavor (it selects the patched GOROOT) | Applies: the evidence for this child |
| Style rule table and C/O/P | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |
| Reasoned gosec annotations | 752d8f19ef (`judge.go`, `timestamp.go`) | A trusted path read carries `//nolint:gosec // <why trusted>` | Extends: the same form where an appliance path or argv is trusted |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/component/ike/... ./internal/component/l2tp/... ./internal/appliance/... ./internal/component/config/system/..."`

### Transformation Path
1. Read each finding; fix, or annotate with the reason the linter is wrong there.
2. A real defect gets a red test first.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Source edits inside the packages | No |

### Integration Points
- None beyond the packages listed.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | Source audit of every listed site; parent prohibited another baseline lint | broken: all seven exhaustive sites already carry annotations |

### Wrong assumptions

The source audit found all seven exhaustive findings already addressed. The three
L2TP findings are AVP projections, not transition tables. Keep those annotations
and the existing transitions. Saved local lint logs did not supply the exact CI
diagnostics; appliance permission IDs were checked against gosec's
`NewMkdirPerms` and `NewFilePerms` in its vendored `rules/fileperms.go` and the
ID mapping in `rules/rulelist.go`.

### Implementation handoff (2026-11-02)

- Written, unverified: eight G301 directory-mode reasons and two G302 reasons
  for owner-only directory traversal; all modes and paths are unchanged.
- Written, unverified: `routeSourceTo` iterates routes by index; the MOBIKE
  namespace fixture uses the Go 1.27 promoted-field literal.
- Written, unverified: six errcheck sites now assert the normal loop outcome
  or explain the ignored result on an already-failing path.
- Independent `ze-test-judge` approved the annotation-only edit to unit
  `internal/component/ike/engine.TestRFC4301SADLifetimeHardTimeTerminatesTheSA`
  in `rfc4301_sad_lifetime_action_test.go`. Its timeout branch closes `stopCh`,
  joins `done`, then fails unconditionally. The approved annotation is applied;
  inputs, assertions and the RFC claim are unchanged. Discrimination hashes strip
  this comment, so no new discrimination record is owed. Audit unit hashes retain
  comments (`freshness.go` `unitSHAs`), so the coordinator owes an independent
  rejudge stamp for `RFC4301-4.4.2.1-11`; plain reseal refuses a stale unit.
- No tests, lint, builds or commits ran in this phase. The parent serializes
  AC-1 and AC-2. `git diff --check` passed for the nine edited Go files.
- No product defect was identified that needs a red-first implementation queue.
  A failure of the strengthened error assertions must be diagnosed before a fix.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A lint edit changes an L2TP transition | An L2TP `.ci` or unit test goes red | No L2TP edit is needed: the cited AVP projections already carry partial-switch reasons |
| R-2 | A gosec fix on a path read breaks the appliance cache | `./le test functional appliance` red | Annotate trusted paths instead of rewriting them |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the four trees | Zero findings |
| AC-2 | The package tests with their tags (`ze_ike`, `ze_l2tp`, `ze_setup`) under `-race` | Pass |
| AC-3 | Any `//nolint` or `//exhaustive:ignore` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | the changed packages | AC-2 | |

## Files to Modify
- The files in the Task table.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, CLI, env, doctor, metrics, BGP family surface | N-A | Lint fixes; no surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-15, 17 | Feature, config, CLI, API, plugin, wire, SDK, RFC, test infrastructure, comparison, architecture, metadata, metrics, inventory, examples | No | Lint fixes change no documented behavior |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-6-access-and-appliance.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. IKE and L2TP: errcheck, exhaustive (C/O), gocritic, modernize.
3. Appliance and config/system: gosec, exhaustive.
4. Scoped lint zero; tests pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Protocol outcomes | L2TP transition tables stay C/O; the cited AVP projections stay P. IKE EAP outcomes remain intact (R-1). |
| Appliance safety | Gosec annotations establish path and argv trust, preserve cache behavior, and leave secrets out of arguments (R-2). |
| Access evidence | Four trees are lint-clean, tagged race tests use the patched GOROOT, and an FSM change has L2TP functional proof (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Access and appliance scopes cleared | Record zero scoped lint findings and tagged race-test results through the patched GOROOT (AC-1–2). |
| Preserved transitions and cache | Cite L2TP functional results if its FSM changes, and inspect appliance path annotations (R-1–2, AC-3). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Authentication and sessions | EAP and L2TP switch edits preserve rejection, unknown-state handling and existing session transitions. |
| Appliance paths and secrets | Gosec annotations establish path and argv trust without exposing secrets or bypassing cache checks. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the four trees>"` zero; `./le job run label access command go test -race -tags <tags> <changed packages>`; `./le test functional l2tp` if the FSM changed. Full-run evidence belongs to the umbrella's CI run.
- [ ] The changed packages (`ike/engine`, `ike/dataplane`, `internal/appliance`) belong to no named `./le test unit` group other than `all`; the closure evidence is the scoped tagged race run above, not `./le test unit all`. The L2TP FSM did not change, so `./le test functional l2tp` is not owed.

---

## Implementation Summary

### What Was Implemented
- `85a33a3887` ("lint(access): annotate appliance modes and check IKE join
  errors"):
  - gosec: two G302 reasons on the `secretsDirPerm` (0o700) chmods in
    `runInit` and `initOneFromBatch` (`cmd_init.go`), and three G301 reasons
    on the `cacheDirPerm` (0o755) `MkdirAll` calls in
    `defaultInitrdMakeBuild` (`cmd_initrd.go`), `defaultKernelBuild` and
    `downloadFirmwareBlob` (`cmd_kernel.go`).
  - errcheck: `TestLeaseReleasedWhenRunEstablishedExits` now fails on a
    non-nil `runEstablished` error, and `TestLeaseReleasedOnResponderDeadBranch`
    fails unless `runResponder` answers `errStopped` (`cp_close_test.go`).
    `TestDprUnansweredProbeIsRetransmitted` fails on a non-nil `maintainSA`
    error after stop (`dpd_retransmit_test.go`). Three joins on an
    already-failing path carry a reasoned `//nolint:errcheck`:
    `TestDprSilentPeerStillTimesOut`, `TestRFC4301SADLifetimeHardTimeTerminatesTheSA`
    and `TestPolicyOwnerHoldsTheSelectorAcrossTheKernelDelete`.
  - gocritic: `routeSourceTo` (`mobike_route_linux.go`) ranges by index.
  - modernize: `mobikeRouteNetns` (`mobike_route_linux_test.go`) sets the
    promoted `Name` field of `netlink.Dummy`.
  - The `RFC4301-4.4.2.1-11` and `RFC3948-5.2-1` audit unit hashes are
    restamped over these comment-only edits to their tagged units.
- `34549af4be` carries the five G301 reasons in `internal/appliance/cache.go`
  (lines 392, 479, 522, 581 and 708).
- `4abd85f6f9` ("lint: keep exhaustive directive arguments empty") placed the
  seven exhaustive reasons on the line above an empty `//exhaustive:ignore`:
  `doctor.go` (platform switch), `eapMethodType` (`eap_auth.go`),
  `mutateForTest` (`reconcile_test.go`), the session-state metric switch
  (`metrics.go`) and the ICRQ, ICCN and OCRQ AVP projections
  (`session_fsm.go`).

### Interim commit (2026-11-10): later findings in the four trees
- The closure's scoped lint found about 100 new findings in the four trees,
  from other sessions' commits after 2026-11-02. Three fix agents cleared
  them (28 IKE files, 27 L2TP files, 13 appliance and config/system files),
  and the interim commit lands them while the spec stays open.
  - They are mechanical: rangeValCopy by index (`installIKEBypass`,
    `installChildDataplane`, `rollbackChildPolicies`, `installSPDPolicies`,
    and the rest), unparam (`writeAVPProxyAuthenID` always writes the M-bit
    clear, which RFC 2661 Section 4.4.5 requires and every caller already
    passed; `cgroup2Root`, `reconcilePeers`, `removeByPeerExcept`), goconst,
    misspell, typeDefFirst moves (`ikeEngineState`), and reasoned errcheck
    directives.
  - `injection_lifetime_proof_test.go` drops a `defer connection.Close()` that
    could never run before the descendant's `os.Exit(0)`.
- Evidence (main thread):
  - `go test -race` with every feature tag over l2tp, the appliance and
    config/system: 20 packages `ok` (`job-lint6-l2tp-app-f0fc5b18.log`).
  - The same over ike, l2tp and kernelbuilder: 10 `ok`
    (`job-lint6-ike2-02d19dff.log`).
  - ike/engine after the last pass: `ok` (`job-lint6-ike3-886ba6d4.log`).
  - `./le repo compiles check`: exit 0.
- RFC records:
  - Resealed, because the edits shifted these verdicts: rfc1661 5.4-5;
    rfc2661 4.1-2, 4.1-3, 9-1, 10-1, 10-2; rfc3948 5.1-1; rfc4301 4.4.1-3,
    4.4.1-10, 4.4.2-2; rfc5176 3.3-1, 3.4-4, 3.5-1.
  - `RFC2661-4.1-1` was rejudged `enforced` by this closure agent, which did
    not author the change: `TestAVPCatalogRoundTrip` lost only the `false`
    argument, and its reserved-bit assertion is unchanged.
- Left after the interim commit: 13 gocritic rangeValCopy findings in
  `ike/engine`. Thomas decided they are fixed before release, the production
  ones included, so the earlier routing of `unmatched.go` and
  `unmatched_xfrm_interface.go` to
  `plan/post-release/features/spec-ipsec-spd-final-discard-vpp-ospf.md` is
  withdrawn. The closure commit fixes all 13 (next section).

### Closure commit (2026-11-10): the last 13 rangeValCopy findings
- A fix agent edited six `ike/engine` files:
  - `unmatched.go`: `installUnmatched` and `removeUnmatched` range the
    catch-all policies by index and pass `*p`, so the same `SPParams` reach
    `InstallPolicy` and `RemovePolicyParams`.
  - `unmatched_xfrm_interface.go` and `apply.go`: the plan map and
    `ikeEngineState.xfrmEntries` hold `*xfrmInterfaceEntry`.
    `planXFRMInterfaceEntries` builds each entry once, from the by-index
    element of a local `interfaceSPDCatchAll` or `xfrmInterfaceExemptions`
    slice. No code changes an entry after it is built:
    `installXFRMInterfaceEntries` stores the plan's pointer, and
    `pruneXFRMInterfaceEntries` only reads and deletes, so a plan and the
    installed set may share one.
  - The test helpers (`liveExemptions`, `liveCatchAll`,
    `liveInterfaceCatchAll`, `installedPolicyScope`, `guardRecorder.deleteRule`)
    range by key or index, with the same values read.
- RFC records:
  - `RFC4301-4.4.1-3` positive (`TestApplyInstallsInterfaceSPDCatchAllAndGuard`)
    and negative (`TestInterfaceSPDCatchAllInstallFailureFailsTheApply`) were
    re-recorded by the main thread from hand-written breaks, both observed red:
    `range catchAll[:0]` in `planXFRMInterfaceEntries`, and the dropped
    `noFinalEntry[entry.iface] = err` in `installXFRMInterfaceEntries`. They
    are the stale records' breaks on the loops' new form. This closure agent,
    which wrote neither the fix nor the breaks, judged the break text under
    `ze-test-judge`: each removes exactly what its claim names (no interface
    final entry planned; a refused interface catch-all reported applied). The
    rejudge of 4.4.1-3 was stamped.
  - The fix agent resealed `RFC7296-1.3.3-4` and `RFC7296-2.9-1` over the
    `installedPolicyScope` edit, and re-judged `RFC4301-5.2-1` and
    `RFC4301-5.2-9`: unimplemented, and they stand. Its reseal of
    `RFC7296-2.7-1` pins another session's uncommitted
    `internal/le/interoplab/ipsec/checkers.go`, so it stays out of this
    commit and lands with that edit.
- Tests (main thread): `ike/engine` with every ze tag, all pass but
  `TestEkuPeerCertificateIsNotJudgedByTLSExtendedKeyUsage`, which another
  session is editing (`remote_cert_eku_test.go`, RFC 4945 EKU work, not in this
  commit). Log `job-lint6-ike4-886ba6d4.log`.

### Bugs Found/Fixed
- None in product code. The three strengthened joins were test gaps: a loop
  that returned an unexpected error on stop passed before.

### Documentation Updates
- None. Every edit is a comment, a lint directive, a test assertion or a
  by-index loop with the same result. `./le spec citation anchors spec
  plan/release/validation/spec-lint-backlog-6-access-and-appliance.md` printed
  nothing and exited 0. The one page anchoring a changed producer,
  `docs/architecture/ike/ipsec-7-ikev2-engine.md` (`routeSourceTo`), describes
  the route lookup, which is unchanged.

### Deviations from Plan
- The seven exhaustive sites and the five `cache.go` sites were not edited in
  this spec's commit: `4abd85f6f9` and `34549af4be` had already landed them.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1: the CI list needed an edit at every site | The seven exhaustive sites were already annotated, and `34549af4be` carried the `cache.go` reasons | Source audit and `git blame` | Implemented the rest in `85a33a3887`; kept all four trees in the acceptance runs |
| approach | `85a33a3887` landed unlinted and untested before a host shutdown | Its body names the owed runs | Closure triage | Closure runs the scoped lint and the tagged race runs |
| scope | The interim commit routed the `unmatched.go` and `unmatched_xfrm_interface.go` rangeValCopy findings to a post-release IPsec spec | Findings in the four trees are this spec's; moving them was a scope change only Thomas decides, and he kept them before release | Thomas's decision at closure | The closure commit fixes all 13 |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `cache.go` gosec x5 | Done | `internal/appliance/cache.go` lines 392, 479, 522, 581, 708 | By `34549af4be`; G301 reasons, mode unchanged |
| `cmd_init.go` gosec x2 | Done | `runInit`, `initOneFromBatch` | G302 reasons |
| `cmd_initrd.go` gosec | Done | `defaultInitrdMakeBuild` | G301 reason |
| `cmd_kernel.go` gosec x2 | Done | `defaultKernelBuild`, `downloadFirmwareBlob` | G301 reasons; the CI lines 658 and 712 drifted to 698 and 757 |
| exhaustive x7 | Done | `doctor.go`, `eap_auth.go` `eapMethodType`, `reconcile_test.go` `mutateForTest`, `metrics.go`, `session_fsm.go` x3 | By `4abd85f6f9`; reason on the line above each empty directive |
| errcheck x6 | Done | `cp_close_test.go` x2, `dpd_retransmit_test.go` x2, `rfc4301_sad_lifetime_action_test.go`, `rfc3948_policy_owner_test.go` | Three assert, three carry a reasoned nolint on an already-failing path |
| gocritic | Done | `mobike_route_linux.go` `routeSourceTo` | rangeValCopy |
| modernize | Done | `mobike_route_linux_test.go` `mobikeRouteNetns` | Promoted field |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done (committed tree) | `./le go lint run scope` over `ike/...`, `l2tp/...`, `internal/appliance/...`, `config/system/...`, 2026-11-10 (`job-lint6f-21412b75.log`): the 13 rangeValCopy findings are gone; freebsd, distro and appliance flavors 0 issues | The 22 findings left in the other flavors (14 hugeParam, 5 intrange, 3 goconst) all come from another session's uncommitted RFC 4301 DF and ICMP-selector work (`ike/ipsec` edits, untracked `icmp_selector_test.go`, `rfc4301_icmp_selector_test.go`, `outer_header.go`); that session owes its own lint |
| AC-2 | Done | `./le job run label lint6-race quiet command go test -race -count=1 -tags "ze_core <every ze_* tag in feature-gates.txt>"` over `ike/engine`, `ike/dataplane`, `internal/appliance`, 2026-11-10: exit 0; engine `ok` (242s), dataplane `ok` (63s), appliance `ok` (248s) | Log `tmp/session/2026-11-09-222501/scratch/job-lint6-race-e9b268bd.log`; the strengthened joins in `cp_close_test.go` and `dpd_retransmit_test.go` passed |
| AC-3 | Done | `git show 85a33a3887` and the `cache.go` lines: every `//nolint` names gosec or errcheck and a reason; each `//exhaustive:ignore` has its reason on the line above | The exhaustive analyzer rejects text on its directive line (`4abd85f6f9`) |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The existing package tests | Done | `internal/component/ike/{engine,dataplane}`, `internal/appliance` | The AC-2 run, three `ok` lines |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| The 25 Task-table sites | Done | 9 files by `85a33a3887`, `cache.go` by `34549af4be`, 5 files by `4abd85f6f9` |

### Audit Summary
- **Total items:** 8 requirement rows, 3 ACs
- **Done:** 8 requirement rows, AC-1 (over the committed tree), AC-2, AC-3
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The 25 CI lint findings in IKE, L2TP, the appliance and config/system are cleared | Scoped lint | `job-lint6f-21412b75.log`, 2026-11-10: no finding from committed code in the four trees; the 22 left belong to another session's uncommitted RFC 4301 work |
| IKE, L2TP and appliance behavior is preserved | Package tests | The tagged race run of the three changed packages, 2026-11-10: exit 0, `ike/engine`, `ike/dataplane` and `internal/appliance` all `ok` (log `job-lint6-race-e9b268bd.log`); the interim commit's runs over l2tp, the appliance, config/system and kernelbuilder (`job-lint6-l2tp-app-f0fc5b18.log`, `job-lint6-ike2-02d19dff.log`); after the closure fixes, `ike/engine` with every ze tag (`job-lint6-ike4-886ba6d4.log`): every test passes except `TestEkuPeerCertificateIsNotJudgedByTLSExtendedKeyUsage`, another session's in-flight RFC 4945 EKU work in `remote_cert_eku_test.go`, outside this commit |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | Every Task-table finding is fixed | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/lint-backlog-6-access-and-appliance-49da3d3d-44f5-443f-bc7a-73b9535a17f9.md` (83 files: the six `ike/engine` files, six RFC records, the umbrella and the IPsec spec of the closure commit, plus the HEAD blobs of the interim commit's other 69 Go files, which Run 2 reviewed and no later commit changed) |
| `./le spec review check` | `review_gate: OK (6 code files, clean, hashes match ...)` |
| Rounds | 3 |
| Reviewer lenses used | /ze-review inline over `git show 85a33a3887` and the `cache.go` and exhaustive sites: wiring, removed behavior, logic, test strength, security, style, RFC records |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | No BLOCKER or ISSUE | - | - |

### Run 1
- 0 BLOCKER, 0 ISSUE. Size: 9 code files, +26/-12, in proportion to 13 findings.
- Logic: `maintainSA` (`established.go`) returns nil on `<-ps.stopCh`, and
  `runResponder` (`fsm.go`) returns `errStopped` there, so the new assertions
  state the producers' contract. `routeSourceTo` returns the same `route.Src`
  slice by index as by value.
- Test strength: three joins now fail on an unexpected error; the three
  annotated joins sit on a path that ends in an unconditional `t.Fatal`.
  `./le test weakened check base 85a33a3887~1` names only another session's
  DDoS detector tests. An independent `ze-test-judge` approved the
  `rfc4301_sad_lifetime_action_test.go` edit.
- Security: no mode, path or argv changed; 0o755 cache directories hold public
  build artifacts and 0o700 is the secrets directory. No secret enters argv.
- Style: no new `panic`, no new function; each directive names its linter and
  reason.
- RFC records: `./le rfc check` names neither `RFC4301-4.4.2.1-11` nor
  `RFC3948-5.2-1`; its IKE reds (RFC3748-4-5, RFC7296-2.4-3, RFC3948-1-1,
  RFC7296-1.3.1-1, RFC7296-2.20-1, RFC7296-2.21.2-3) judge
  `handleAuthRequest`, `handleResponderEAP` and the unprotected-handshake
  tests, none of which this diff touched.
- NOTE: `./le repo check` reports only other sessions' findings
  (`SetInt`/`SetBool`, `DirtyManifest`/`Fingerprint`, a DHCP probe parse).

### Run 2 (interim commit)
- Over the fix agents' edits in the 68 interim files: 0 BLOCKER, 0 ISSUE.
- IKE engine sources:
  - `apply.go` moves `ikeEngineState` unchanged above its first method.
  - `bypass.go`, `child.go` and `spd_policy.go` range by index and pass `*p`,
    so the same values reach `InstallPolicy` and `removeZePolicy`.
    `rollbackChildPolicies` still walks `installed` backwards.
  - `table.go` `removeByPeerExcept` lost a count no caller read.
  - `reconcile.go` `reconcilePeers` lost the parameters that every test
    caller passed as nil.
- `cgroup2Root` reads its one constant path. The orderedInstall cleanup now
  reads through a pointer into its slice, and its two callers do not change
  the slice afterwards.
- `writeAVPProxyAuthenID`: every caller passed `false`, so the bytes on the
  wire are unchanged. The RFC 2661 Section 4.4.5 quote is byte-exact.
- `injection_lifetime_proof_test.go`: the removed `defer connection.Close()`
  sat before `os.Exit(0)`, so it never ran. The new type-assertion check in
  `acceptInjectionLifetime` adds a failure arm, and no assertion is weaker.
- NOTE: every edit to an RFC-tagged unit is mechanical, and its record was
  resealed or rejudged above.

### Run 3 (closure commit)
- Over the six `ike/engine` files of the closure commit, by this closure
  agent, which authored none of them: 0 BLOCKER, 0 ISSUE.
- Logic: `installUnmatched` and `removeUnmatched` pass `*p`, the same value the
  range copy held. `planXFRMInterfaceEntries` sets `SPD` on the element of a
  slice it owns, as it set it on the copy before, then builds the entry from
  `*p`. Holding `*xfrmInterfaceEntry` is safe because nothing writes through
  it: `installXFRMInterfaceEntries` reads `entry.params` and stores the
  pointer, `pruneXFRMInterfaceEntries` reads and deletes, and the failed-read
  path copies the held pointer into the plan. No package outside `ike/engine`
  names the type or the map.
- Tests: the helpers read the same map values by key and the same slice
  elements by index; no assertion changed.
- RFC records: the two hand-written `RFC4301-4.4.1-3` breaks remove exactly
  the behavior each claim names (`ze-test-judge` reading, recorded above).
- Style: no new function, no `panic`, no directive; the `xfrmInterfaceEntry`
  comment now states the pointer and never-mutated contract.
- NOTE: the map-range helpers in the tests still copy one `SPParams` per
  key, since a map value is not addressable; the lint accepts it and the
  cost is a test's.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/engine/mobike_route_linux.go` | Yes | `grep -n "route := &routes\[i\]"` hits `routeSourceTo` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Zero scoped lint findings | `job-lint6f-21412b75.log`: freebsd, distro, appliance 0 issues; the other flavors' 22 findings are another session's uncommitted files |
| AC-2 | Tagged race tests pass | `go test -race -count=1` with every feature tag under `./le job run`, 2026-11-10: three `ok` lines, exit 0 |
| AC-3 | Every directive names its linter and reason | `grep -n "nolint:gosec // G30[12]" internal/appliance/*.go` lists ten sites, each with its reason |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le go lint run scope` over the four trees | None: a lint run | Yes, `job-lint6f-21412b75.log` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | `4abd85f6f9` and `34549af4be` had cleared twelve sites; Mistake Log row 1 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 16, source anchors | `ipsec-7-ikev2-engine.md` anchors `routeSourceTo`, whose route choice is unchanged; `./le spec citation anchors` printed nothing | Yes |
