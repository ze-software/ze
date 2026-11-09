# Spec: lint-backlog-2-bgp-plugins

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | 22 findings of the full lint in the BGP plugins; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3: closure |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in the BGP plugins
(door `ai/doors/bgp-plugins.md`): filter_modify 9, role 4, filter_family 2,
filter_prefix 2, filter_remove_private_as 2, filter_irr 1, filter_path_asn 1,
gr 1. By linter: goconst 9, misspell 6, unparam 3, exhaustive 1, modernize 1,
gocritic 1, nilnil 1. Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/component/bgp/plugins/filter_family/evaluator_test.go` | 22 | misspell |
| `internal/component/bgp/plugins/filter_family/evaluator_test.go` | 26 | misspell |
| `internal/component/bgp/plugins/filter_irr/irr_snapshot_test.go` | 153 | exhaustive |
| `internal/component/bgp/plugins/filter_modify/config.go` | 236 | goconst |
| `internal/component/bgp/plugins/filter_modify/config.go` | 238 | goconst |
| `internal/component/bgp/plugins/filter_modify/config.go` | 240 | goconst |
| `internal/component/bgp/plugins/filter_modify/match.go` | 85 | modernize |
| `internal/component/bgp/plugins/filter_modify/rfc4271_modify_test.go` | 432 | misspell |
| `internal/component/bgp/plugins/filter_modify/rfc4271_modify_test.go` | 435 | misspell |
| `internal/component/bgp/plugins/filter_modify/rib_evaluator.go` | 46 | goconst |
| `internal/component/bgp/plugins/filter_modify/rib_evaluator.go` | 47 | goconst |
| `internal/component/bgp/plugins/filter_modify/rib_evaluator.go` | 48 | goconst |
| `internal/component/bgp/plugins/filter_path_asn/bgp_condition_log_test.go` | 30 | unparam |
| `internal/component/bgp/plugins/filter_prefix/bgp_condition_log_test.go` | 28 | unparam |
| `internal/component/bgp/plugins/filter_prefix/bgp_condition_test.go` | 197 | gocritic |
| `internal/component/bgp/plugins/filter_remove_private_as/filter_remove_private_as_test.go` | 103 | misspell |
| `internal/component/bgp/plugins/filter_remove_private_as/filter_remove_private_as_test.go` | 106 | misspell |
| `internal/component/bgp/plugins/gr/gr_state.go` | 106 | unparam |
| `internal/component/bgp/plugins/role/config_tx.go` | 135 | nilnil |
| `internal/component/bgp/plugins/role/decode.go` | 64 | goconst |
| `internal/component/bgp/plugins/role/role.go` | 38 | goconst |
| `internal/component/bgp/plugins/role/upstream.go` | 89 | goconst |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/bgp-plugins.md` - plugin boundaries and gates
  → Constraint: a plugin can run forked; a goconst constant stays in the plugin's own package, never imported from another plugin's implementation
  → Constraint: run the packages with `-tags ze_bgp`
- [ ] `docs/contributing/ze-go-style.md` - "The rules a tool enforces", "No `nil, nil` answer"
  → Constraint: a `(pointer, error)` function returns one or the other, never neither
- [ ] `ai/META-DECISIONS.md` - "Code and dependencies"
  → Constraint: the misspellings in `rfc4271_modify_test.go` may sit inside an RFC quote, which stays byte-exact and takes a reasoned `//nolint:misspell`

**Key insights:**
- The `nilnil` in `role/config_tx.go` is an answer shape the style guide refuses: decide the sentinel or the typed absence before changing callers.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/role/config_tx.go` - the `nilnil` return
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` - the `unparam` parameter
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Every filter verdict and the RFC 9234 role behavior.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| Style rule table | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |
| Plugin import ownership | `./le arch tier check` | Refuses one plugin importing another's implementation | Applies: shared constants stay per plugin |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/component/bgp/plugins/filter_family/... ./internal/component/bgp/plugins/filter_irr/... ./internal/component/bgp/plugins/filter_modify/... ./internal/component/bgp/plugins/filter_path_asn/... ./internal/component/bgp/plugins/filter_prefix/... ./internal/component/bgp/plugins/filter_remove_private_as/... ./internal/component/bgp/plugins/gr/... ./internal/component/bgp/plugins/role/..."`

### Transformation Path
1. Read each finding; fix, or annotate with the reason the linter is wrong there.
2. A real defect gets a red test first.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Source edits inside each plugin | No |

### Integration Points
- None beyond the packages listed.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | Source audit: role fixes landed in 56bc606e5c; the other 18 finding shapes remain. Parent's baseline lint was interrupted by server restarts; no fresh lint verdict is available | broken |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Replacing `nil, nil` changes what a caller sees | A role test or `.ci` goes red | Trace every caller first; keep their outcome |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the plugins | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the eight plugin packages | Zero findings |
| AC-2 | The package tests under `-tags ze_bgp -race` | Pass |
| AC-3 | Any `//nolint` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | `internal/component/bgp/plugins/...` | AC-2 | |

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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-2-bgp-plugins.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. Fix per plugin, one commit per plugin or two.
3. Scoped lint zero; package tests pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Role absence | Every caller preserves its outcome after the nilnil repair in `role/config_tx.go` (R-1). |
| Plugin contracts | Constants stay within their owning plugin, filter verdicts stay intact, and RFC quotes remain byte-exact. |
| Plugin evidence | All eight scopes are lint-clean, tagged race tests pass, and annotations state their reasons (AC-1–3). |

## Source audit and implementation handoff

The source audit found four role findings already addressed by 56bc606e5c.
The remaining 18 finding shapes were present before these edits. This is
source evidence, not a current lint result: the parent's baseline run was
interrupted by server restarts. Role files are owned by another session and
were not edited by this phase.

The remaining source fixes are written: community field constants stay in
filter_modify; its bracket guard uses bytes.HasPrefix; the three conformance
comments use US spelling; the logging helpers lose unused input/output;
the labeled-prefix fixture appends into its own slice variable. No RFC quote
was changed. The affected guide sections keep the same documented behavior.

The IRR snapshot test now fails on FilterWithdraw as well as modify and
unspecified. Diagnosis: the saved exhaustive finding exposed a missing enum
case in TestIRRSnapshotSwap, which let that verdict pass without an assertion.
The owning layer is the test. The source fix adds the verdict to the existing
failure arm; suppressing exhaustive would leave that blind spot in place.
Existing assertions are preserved and no product behavior changes.

GR's familyRetained was used only by tests. It moved unchanged into
gr_state_helpers_test.go, retaining its generic peer argument and a reasoned
unparam annotation: assertions keep the queried peer explicit although current
fixtures share one address. Its RFC-tagged callers and their claims were not
edited. No RFC-test judgment is requested by this patch.

At this handoff AC-1 and AC-2 were unverified; closure records their runs. AC-3 has one linter-specific, reasoned
annotation, pending the parent's lint run. No tests, lint or builds ran in
this implementation phase; the main session owns serialized verification,
independent review and committing. The exact commands and changed-file digest
are in the session's per-spec state file.

### Mistake Log — Wrong Assumptions

| Assumption | Evidence | Correction |
|------------|----------|------------|
| A-1: all 22 CI findings still need edits | The role source and 56bc606e5c already carry its four fixes | Implement the remaining source findings; retain all eight scopes for acceptance verification |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Eight plugin scopes cleared | Record zero scoped lint findings and tagged race-test results (AC-1–2). |
| Preserved role outcomes | Trace callers of the nilnil return and cite role-test results, with reasons for annotations (R-1, AC-3). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Role admission | The replacement for nilnil preserves each caller's rejection and absence handling. |
| Filter decisions | Mechanical edits preserve filter verdicts and safe unknown handling without cross-plugin implementation imports. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the eight packages>"` zero; `./le job run label bgp-plugins command go test -race -tags ze_bgp <the eight packages>`. Full-run evidence belongs to the umbrella's CI run.
- [ ] The eight packages belong to the `./le test unit bgp` group; the closure evidence is the scoped tagged race run above, not that whole group.

---

## Implementation Summary

### What Was Implemented
- `f23e06435c` ("bgp plugins: clear mechanical lint findings") carries the 18
  non-role findings. filter_modify: `communityAttr`, `largeCommunityAttr` and
  `extendedCommunityAttr` are declared in `filter_modify.go` and read by
  `communitySetLeaves` and `parseMatchBlock` (`config.go`) and by
  `matchKeyFields` (`rib_evaluator.go`); `matchCommunity.in` (`match.go`)
  tests the bracket with `bytes.HasPrefix`. misspell: US spellings in three
  conformance-test comments (filter_family, filter_modify,
  filter_remove_private_as). unparam: `runLogged` (filter_path_asn) loses the
  direction every caller passed as `import`; `runFromPeer` (filter_prefix)
  loses the outcome no caller read. gocritic:
  `TestBGPMatchLabeledAllocatesNothing` appends into its own `attrs`.
  exhaustive: `TestIRRSnapshotSwap` now fails on `sdk.FilterWithdraw` too. gr:
  `grStateManager.familyRetained`, called only by tests, moved unchanged into
  `gr_state_helpers_test.go` with a reasoned `//nolint:unparam`.
- The four role findings landed earlier in `56bc606e5c` (Spec-work
  filter-wire-0-umbrella): `roleKeyword` in `role.go`, read by `decode.go` and
  `otc_filter.go`; `policySection` in `config_tx.go` answers
  `(body, delivered, err)`, so no two-value `nil, nil` remains there.

### Bugs Found/Fixed
- `TestIRRSnapshotSwap` let a `FilterWithdraw` verdict pass without an
  assertion. The exhaustive finding exposed it, and the test now fails on it.
  This was a test gap; no product behavior changed.

### Documentation Updates
- None. The changes turn literals into constants, simplify test helpers and
  move a test-only query, so no documented behavior moved. The source anchors
  that name changed files (`docs/guide/configuration.md`, `docs/guide/plugins.md`,
  `docs/guide/redistribution.md`, `docs/guide/route-policy.md`,
  `docs/guide/graceful-restart.md`, `docs/architecture/core-design.md`,
  `docs/architecture/bgp/egress-attribute-rules.md`,
  `docs/features/bgp-protocol.md`) name symbols whose behavior is unchanged.
  None names `familyRetained`: `git grep familyRetained -- ':!*.go'` hits only
  this spec. `./le spec citation anchors` printed nothing and exited 0.

### Deviations from Plan
- One commit for all eight plugins, not one per plugin or two: the edits are
  mechanical and small (13 files, +52/-41).
- The role findings were not edited here, because `56bc606e5c` had already
  fixed them.
- Closure fixed one finding the CI list did not hold: `1e65b6d4bb` (11-06)
  declared `type llgrSession` in `gr/gr_filter.go` after its first method,
  and the scoped lint reported gocritic `typeDefFirst`. The type now sits
  above `lostLLGRStale` (Review Gate, Run 2).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1: all 22 CI findings still needed edits | `56bc606e5c` had already cleared the four role findings | Source audit of `role/` | Implemented the other 18; kept all eight scopes in the acceptance runs |
| approach | `f23e06435c` landed with no `Spec-work:` trailer | It was recovered from an overlapping session that held no claim on this spec | Closure triage, `git show f23e06435c` | Closure commit A carries the trailer and names `f23e06435c` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| filter_family misspell x2 | Done | `filter_family/evaluator_test.go` `TestFamilyFilterConformance` comment | honors, serialized |
| filter_irr exhaustive | Done | `filter_irr/irr_snapshot_test.go` `TestIRRSnapshotSwap` | FilterWithdraw fails the test |
| filter_modify goconst x6 | Done | `filter_modify/filter_modify.go` const block; `config.go` `communitySetLeaves`, `parseMatchBlock`; `rib_evaluator.go` `matchKeyFields` | Constants stay in filter_modify |
| filter_modify modernize | Done | `filter_modify/match.go` `matchCommunity.in` | `bytes.HasPrefix` |
| filter_modify misspell x2 | Done | `filter_modify/rfc4271_modify_test.go` `TestModifyConformance` comment | No RFC quote touched |
| filter_path_asn unparam | Done | `filter_path_asn/bgp_condition_log_test.go` `runLogged` | Direction fixed to import |
| filter_prefix unparam | Done | `filter_prefix/bgp_condition_log_test.go` `runFromPeer` | No caller read the outcome |
| filter_prefix gocritic | Done | `filter_prefix/bgp_condition_test.go` `TestBGPMatchLabeledAllocatesNothing` | appendAssign |
| filter_remove_private_as misspell x2 | Done | `filter_remove_private_as_test.go` `TestRemovePrivateASConformance` comment | |
| gr unparam | Changed | `gr/gr_state_helpers_test.go` `familyRetained` | Test-only helper with a reasoned nolint |
| role nilnil and goconst x3 | Done | `role/config_tx.go` `policySection`; `role/role.go` `roleKeyword` | By `56bc606e5c` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `./le go lint run scope` over the eight trees, 2026-11-09, after the `gr_filter.go` move: exit 0, `0 issues.` in the host and the GOOS=linux integration flavors | Log `tmp/session/2026-11-09-222501/scratch/job-lint2-f92ee8da.log`; run from a session build of le |
| AC-2 | Done | `./le job run label lint2-race quiet command go test -race -count=1 -tags ze_bgp` over the eight trees, 2026-11-09: exit 0, all eight packages `ok` | Log `tmp/session/2026-11-09-222501/scratch/job-lint2-race-170ffd5b.log`; the yang subpackages hold no tests. gr re-run after the `gr_filter.go` move: `ok` (11.3s), log `job-lint2-gr-81ccb5b9.log` in the same scratch directory |
| AC-3 | Done | `gr/gr_state_helpers_test.go` `familyRetained` | The only nolint added names unparam and its reason |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The existing package tests | Done | `internal/component/bgp/plugins/{filter_family,filter_irr,filter_modify,filter_path_asn,filter_prefix,filter_remove_private_as,gr,role}` | The AC-2 run |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| The 13 files of `f23e06435c` | Done | The Task table files, plus `filter_modify.go` (constants) and `gr_state_helpers_test.go` (moved helper) |
| `role/config_tx.go`, `decode.go`, `role.go`, `upstream.go` | Done | By `56bc606e5c`; `upstream.go` needed no edit there |
| `gr/gr_filter.go` | Done | Closure: `llgrSession` declared before its methods |

### Audit Summary
- **Total items:** 11 requirement rows, 3 ACs
- **Done:** 10 requirement rows, AC-1, AC-2, AC-3
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the gr helper moved to test code; see Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The 22 CI lint findings in the BGP plugins are cleared | Scoped lint | The scoped lint over the eight trees, 2026-11-09: `0 issues.` in both flavors, exit 0 (18 findings by `f23e06435c`, 4 by `56bc606e5c`, and the later `typeDefFirst` by the closure move) |
| Filter verdicts and the RFC 9234 role behavior are preserved | Package tests | The tagged race run of the eight packages, 2026-11-09, exit 0 with every package `ok` (filter_family, filter_irr, filter_modify, filter_path_asn, filter_prefix, filter_remove_private_as, gr, role); it includes the RFC 9494 GR tests that call `familyRetained` and the role RFC 9234 tests |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | All 22 findings are fixed: 18 here and 4 by `56bc606e5c` | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/lint-backlog-2-bgp-plugins-49da3d3d-44f5-443f-bc7a-73b9535a17f9.md` (verdict CLEAN, rounds 2) over `gr/gr_filter.go`, this spec and the umbrella row hunk |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | /ze-review inline over `git show f23e06435c`: size, wiring, removed behavior, logic, performance, style, RFC tags |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | No BLOCKER or ISSUE | - | - |

### Run 1
- 0 BLOCKER, 0 ISSUE. Size: 13 files, +52/-41, in proportion to 18 findings.
- Wiring: the new constants have readers in `config.go` and `rib_evaluator.go`.
  `familyRetained` had no non-test caller before the move
  (`git grep familyRetained -- '*.go'` hits only `_test.go` files).
- Removed behavior: `len(list) == 0 || list[0] != '['` equals
  `!bytes.HasPrefix(list, []byte{'['})`. Every `runLogged` caller passed
  `"import"`, and the assertions still read `direction=import`. Both
  `runFromPeer` callers discard the result. The IRR change adds a failure arm.
- Performance: `[]byte{'['}` does not escape `bytes.HasPrefix`, so the match
  path stays allocation-free.
- Style: the one new Go file holds a moved method with no `panic`; the nolint
  names its linter and reason.
- RFC tags: no tagged unit or claim changed, and the RFC 9494 tests call
  `familyRetained` with its body unchanged. `./le rfc check` names no record
  over a file this diff changed; its reds (RFC9234-5-9 negative-less,
  interoplab, ike, config records) are other work.
- NOTE: `./le repo check` (env.go `SetInt`/`SetBool`, dhcpprobe/step.go) and
  `./le test weakened check base f23e06435c~1` (ike `negAuthResponse`) report
  only other sessions' work.

### Run 2
- Over the closure edit to `gr/gr_filter.go`: 0 BLOCKER, 0 ISSUE.
- Diagnosis: the closure's scoped lint reported gocritic `typeDefFirst` on
  `type llgrSession`, which `1e65b6d4bb` placed after its first method,
  `lostLLGRStale`. The owning layer is the file's layout. `[workaround]` a
  `//nolint:gocritic`; `[source]` move the declaration, taken, because the
  finding is about layout and a move fixes it with no behavior change.
- The diff is a pure move: 15 lines out, the same 15 lines in above
  `lostLLGRStale`, with every removed line matched by an added one. No
  function body changed, so no RFC 9494 test unit fingerprint moved;
  `rfc/audit/rfc9494/RFC9494-4.3-3.json` names `gr_filter.go` only in prose.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/gr/gr_state_helpers_test.go` | Yes | `git grep -n familyRetained` hits its `familyRetained` method |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Zero scoped lint findings | Scoped lint log ends `0 issues.` for the host run and for the GOOS=linux integration run |
| AC-2 | Tagged race tests pass | `go test -race -count=1 -tags ze_bgp` under `./le job run`, 2026-11-09: eight `ok` lines, exit 0 |
| AC-3 | Every nolint names its linter and reason | `gr_state_helpers_test.go` `familyRetained`: `//nolint:unparam // Keep the queried peer explicit in retention assertions; ...` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le go lint run scope` over the eight packages | None: a lint run | Yes: exit 0, 0 issues in both flavors |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | `56bc606e5c` cleared the role findings; Mistake Log row 1 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 16, source anchors | The anchors name `parseModifyDef(s)`, `buildSets`, `buildCommunitySets`, `matchCond`, `medRemoveFor`, `evaluateModify`, `compileRouteEvaluator`, `parseRIBSets` and GR session functions, all with unchanged behavior; none names `familyRetained` | Yes |
