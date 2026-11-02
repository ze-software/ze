# Spec: lint-backlog-1-bgp-engine

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | 53 findings of the full lint in the BGP engine packages; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in the BGP engine
(door `ai/doors/bgp-engine.md`): reactor 39, policychain 8, config 4, cli 1,
redistribute 1. By linter: exhaustive 10, modernize 10, misspell 8, errcheck 8,
goimports 8, gocritic 5, nilerr 2, goconst 1, gosec 1. Line numbers are the CI
run's; regenerate the list with the scoped run first, since other sessions edit
the reactor.

| File | Line | Linter |
|------|------|--------|
| `internal/component/bgp/cli/register.go` | 76 | goconst |
| `internal/component/bgp/config/dynamic_group_parity_test.go` | 190 | modernize |
| `internal/component/bgp/config/dynamic_group_parity_test.go` | 191 | modernize |
| `internal/component/bgp/config/external_filters.go` | 150 | modernize |
| `internal/component/bgp/config/templated_chain.go` | 108 | modernize |
| `internal/component/bgp/policychain/chain.go` | 643 | misspell |
| `internal/component/bgp/policychain/external.go` | 111 | misspell |
| `internal/component/bgp/policychain/external_intents_test.go` | 150 | gocritic |
| `internal/component/bgp/policychain/match.go` | 139 | gocritic |
| `internal/component/bgp/policychain/prefix_overlay.go` | 139 | gocritic |
| `internal/component/bgp/policychain/sets.go` | 291 | misspell |
| `internal/component/bgp/policychain/sets.go` | 396 | modernize |
| `internal/component/bgp/policychain/sets_attributes.go` | 63 | modernize |
| `internal/component/bgp/reactor/bfd_delay_open_release_race_test.go` | 79 | errcheck |
| `internal/component/bgp/reactor/bfd_delay_open_release_race_test.go` | 105 | errcheck |
| `internal/component/bgp/reactor/capture_replay.go` | 242 | gosec |
| `internal/component/bgp/reactor/filter_chain.go` | 613 | exhaustive |
| `internal/component/bgp/reactor/filter_library.go` | 273 | modernize |
| `internal/component/bgp/reactor/filter_library_text.go` | 89 | misspell |
| `internal/component/bgp/reactor/filter_message_test.go` | 9 | goimports |
| `internal/component/bgp/reactor/filter_message_test.go` | 26 | modernize |
| `internal/component/bgp/reactor/filter_message_test.go` | 50 | misspell |
| `internal/component/bgp/reactor/filter_nlri_delta.go` | 208 | modernize |
| `internal/component/bgp/reactor/filter_nlri_delta.go` | 306 | nilerr |
| `internal/component/bgp/reactor/filter_nlri_delta.go` | 312 | nilerr |
| `internal/component/bgp/reactor/filter_nlri_delta_test.go` | 5 | goimports |
| `internal/component/bgp/reactor/filter_ordered.go` | 170 | gocritic |
| `internal/component/bgp/reactor/filter_ordered_test.go` | 4 | goimports |
| `internal/component/bgp/reactor/filter_unavailable_test.go` | 89 | modernize |
| `internal/component/bgp/reactor/forward_body.go` | 639 | exhaustive |
| `internal/component/bgp/reactor/forward_pool_barrier_test.go` | 47 | errcheck |
| `internal/component/bgp/reactor/forward_pool_barrier_test.go` | 201 | errcheck |
| `internal/component/bgp/reactor/forward_publication_ownership_test.go` | 150 | errcheck |
| `internal/component/bgp/reactor/forward_validation.go` | 633 | exhaustive |
| `internal/component/bgp/reactor/loop_policy_binding_test.go` | 9 | goimports |
| `internal/component/bgp/reactor/peer.go` | 2520 | exhaustive |
| `internal/component/bgp/reactor/peer_run_test.go` | 746 | errcheck |
| `internal/component/bgp/reactor/peer_run_test.go` | 888 | errcheck |
| `internal/component/bgp/reactor/peer_run_test.go` | 894 | errcheck |
| `internal/component/bgp/reactor/policy_final_test.go` | 6 | goimports |
| `internal/component/bgp/reactor/reactor_iface_test.go` | 230 | misspell |
| `internal/component/bgp/reactor/reactor_notify.go` | 390 | exhaustive |
| `internal/component/bgp/reactor/reactor_notify.go` | 433 | exhaustive |
| `internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go` | 8 | goimports |
| `internal/component/bgp/reactor/rfc4271_ibgp_aspath_export_test.go` | 38 | gocritic |
| `internal/component/bgp/reactor/rfc6996_remove_private_test.go` | 4 | goimports |
| `internal/component/bgp/reactor/rfc6996_remove_private_test.go` | 84 | misspell |
| `internal/component/bgp/reactor/rfc8212_default_reject_test.go` | 11 | goimports |
| `internal/component/bgp/reactor/rfc8277_label_rsrv.go` | 108 | exhaustive |
| `internal/component/bgp/reactor/session_accept_own.go` | 31 | exhaustive |
| `internal/component/bgp/reactor/session_bfd_strict.go` | 422 | exhaustive |
| `internal/component/bgp/reactor/session_handlers.go` | 368 | exhaustive |
| `internal/component/bgp/redistribute/consumer.go` | 120 | misspell |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/bgp-engine.md` - the engine's boundaries and traps
  → Constraint: an untagged package test does not link BGP; run the packages with `-tags ze_bgp`
  → Constraint: received bytes are borrowed and encoding is buffer-first; a modernize rewrite must not add an allocation on a hot path
- [ ] `docs/contributing/ze-go-style.md` - "The rules a tool enforces", the `exhaustive` C/O/P classification
  → Constraint: each enum switch is classified C, O or P; a P switch takes `//exhaustive:ignore // <why>`, never a blanket nolint
- [ ] `ai/META-DECISIONS.md` - "Code and dependencies"
  → Constraint: an RFC quote in a comment stays byte-exact; a misspell inside one gets a reasoned `//nolint:misspell`, never an edit

**Key insights:**
- The two `nilerr` findings in `filter_nlri_delta.go` return nil beside a live error: read each as a possible dropped error before any annotation.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/filter_nlri_delta.go` - the two `nilerr` sites
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - two enum switches `exhaustive` reports
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Every wire and FSM behavior; an exhaustive fix keeps each switch's existing named outcomes and its unknown-value handling.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope over every build flavor | Applies: the evidence for this child |
| Style rule table and C/O/P | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies: every fix follows it |
| Lint contract journal rows | `plan/journal/lint-contract-not-applied.md`, the 2026-10-23 reactor row (owned by `plan/post-release/engineering/spec-gates-report-cause-and-progress.md`) | Records reactor findings found in a scoped run | Applies: a finding here that row names is marked `fixed:` in the same commit |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/component/bgp/reactor/... ./internal/component/bgp/policychain/... ./internal/component/bgp/config/... ./internal/component/bgp/cli/... ./internal/component/bgp/redistribute/..."`

### Transformation Path
1. Read each finding at its line; decide fix, or a reasoned annotation where the linter is wrong.
2. A finding that is a real defect gets a red test, then the fix.

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
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted; findings added or gone | The scoped run before the first edit | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An exhaustive fix changes an FSM or notify outcome | A reactor or functional test goes red | Keep each named outcome; add only the missing named cases with their current behavior |
| R-2 | Another session holds uncommitted reactor edits | `git status` shows reactor files dirty | Commit only this child's hunks (`./le commit create` `hunks`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the five packages | Zero findings |
| AC-2 | The package tests under `-tags ze_bgp -race` | Pass |
| AC-3 | Any `//nolint` or `//exhaustive:ignore` added | Names its linter and its reason |
| AC-4 | A `nilerr`, `errcheck` or `gosec` finding that is a real defect | Fixed with a test that was red before the fix |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | `internal/component/bgp/...` | AC-2 | |
| A red-first test per real defect | the package of the producer | AC-4 | |

## Files to Modify
- The files in the Task table, under `internal/component/bgp/reactor`, `policychain`, `config`, `cli` and `redistribute`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, CLI, env, doctor, metrics, BGP family surface | N-A | Lint fixes; no surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-15, 17 | Feature, config, CLI, API, plugin, wire, SDK, RFC, test infrastructure, comparison, architecture, metadata, metrics, inventory, examples | No | Lint fixes change no documented behavior |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-1-bgp-engine.md`; a renamed symbol updates its anchor |

## Implementation Steps

1. Scoped lint; record the current list in the spec.
2. Mechanical findings (goimports, misspell, modernize, goconst, gocritic).
3. errcheck, nilerr, gosec: read each; real defects red-first.
4. exhaustive: classify C/O/P per switch, then fix.
5. Scoped lint zero; package tests pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| FSM outcomes | C/O/P classification preserves named, unknown and post-switch outcomes in the reactor (R-1). |
| Error paths | Each real nilerr, errcheck or gosec defect has red-first proof (AC-4). |
| Engine evidence | All five scopes are lint-clean and tagged race tests pass, with reasoned annotations and byte-exact RFC quotes (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Five engine scopes cleared | Record zero scoped lint findings and the tagged race-test results from the TDD checklist (AC-1–2). |
| Error-path repairs | Cite red-first tests for real defects and inspect reasons on each new annotation (AC-3–4). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Peer-controlled states | Exhaustive fixes preserve safe unknown handling and introduce no peer-reachable panic. |
| Dropped errors and ownership | Nilerr and errcheck repairs retain error handling, while mechanical rewrites preserve borrowed-buffer lifetimes. |
| Capture paths | Any gosec annotation establishes why the capture path is trusted. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the five packages>"` zero; `./le job run label bgp-engine command go test -race -tags ze_bgp <the five packages>`. Full-run evidence belongs to the umbrella's CI run.
