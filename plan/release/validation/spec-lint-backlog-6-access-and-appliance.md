# Spec: lint-backlog-6-access-and-appliance

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | 25 findings of the full lint in IKE, L2TP and the appliance; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4: closure open; CI-list fixes landed in `85a33a3887`, `4abd85f6f9` and `34549af4be`; later in-scope findings in the interim commit; scoped lint pending |
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
- Still open. The scoped lint cannot settle while another IKE session edits
  `ike/engine`:
  - gocritic rangeValCopy in `rfc4301_unmatched_xfrm_interface_test.go`,
    `rfc7296_child_rekey_initiator_answer_test.go` and `source_guard_test.go`.
  - Whatever the lint's cap of ten findings per linter still hides.
  - `unmatched.go` and `unmatched_xfrm_interface.go` belong to
    `plan/post-release/features/spec-ipsec-spd-final-discard-vpp-ospf.md`.

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
| AC-1 | OWED | The main thread's scoped lint over the four trees | Filled from its log |
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
- **Done:** 8 requirement rows, AC-2, AC-3 (AC-1 waits for the scoped lint)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The 25 CI lint findings in IKE, L2TP, the appliance and config/system are cleared | Scoped lint | OWED: the scoped lint over the four trees |
| IKE, L2TP and appliance behavior is preserved | Package tests | The tagged race run of the three changed packages, 2026-11-10: exit 0, `ike/engine`, `ike/dataplane` and `internal/appliance` all `ok` (log `job-lint6-race-e9b268bd.log`); L2TP and config/system carry comment-only edits |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | Every Task-table finding is fixed | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | recorded at closure |
| `./le spec review check` | recorded at closure |
| Rounds | 1 |
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

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/engine/mobike_route_linux.go` | Yes | `grep -n "route := &routes\[i\]"` hits `routeSourceTo` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Zero scoped lint findings | OWED |
| AC-2 | Tagged race tests pass | `go test -race -count=1` with every feature tag under `./le job run`, 2026-11-10: three `ok` lines, exit 0 |
| AC-3 | Every directive names its linter and reason | `grep -n "nolint:gosec // G30[12]" internal/appliance/*.go` lists ten sites, each with its reason |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le go lint run scope` over the four trees | None: a lint run | OWED |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | `4abd85f6f9` and `34549af4be` had cleared twelve sites; Mistake Log row 1 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 16, source anchors | `ipsec-7-ikev2-engine.md` anchors `routeSourceTo`, whose route choice is unchanged; `./le spec citation anchors` printed nothing | Yes |
