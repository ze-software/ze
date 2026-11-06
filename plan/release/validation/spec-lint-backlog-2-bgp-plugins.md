# Spec: lint-backlog-2-bgp-plugins

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | 22 findings of the full lint in the BGP plugins; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 2/3: source fixes; parent owns serialized verification |
| Handoff | - |
| Updated | 2026-11-02 |

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

AC-1 and AC-2 remain unverified. AC-3 has one linter-specific, reasoned
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
