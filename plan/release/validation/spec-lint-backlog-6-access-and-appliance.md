# Spec: lint-backlog-6-access-and-appliance

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | 25 findings of the full lint in IKE, L2TP and the appliance; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

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
- [ ] `ai/doors/access-security.md` - IKE, L2TP, the subscriber model
  → Constraint: EAP-TLS compiles only against the patched `crypto/tls`; build and test through `./le`, never a plain `go test` of a package that links `internal/core/eap`
  → Constraint: a datagram that fails to decrypt must not kill a live SA; an exhaustive fix in `eap_auth.go` keeps every outcome
- [ ] `ai/doors/appliance.md` - the build host tooling
  → Constraint: secrets are never in process arguments; a gosec G204 fix keeps argv free of them
  → Constraint: `internal/appliance` is build-host tooling behind `ze_setup`
- [ ] `docs/contributing/ze-go-style.md` - the `exhaustive` C/O/P classification
  → Constraint: the L2TP session FSM switches are transition tables (C or O), never P

**Key insights:**
- The three `session_fsm.go` findings are the L2TP session state machine: each missing named state is listed with its current behavior, never a new transition.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/session_fsm.go` - three enum switches `exhaustive` reports
- [ ] `internal/appliance/cache.go` - five gosec sites
- [ ] `.golangci.yml` - the linter settings

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
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | The scoped run before the first edit | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An exhaustive arm added to the L2TP FSM changes a transition | An L2TP `.ci` or unit test goes red | List each missing state with the behavior the `default` gave it |
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
| Protocol outcomes | L2TP transition tables stay C/O and preserve default outcomes. IKE EAP outcomes remain intact (R-1). |
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
