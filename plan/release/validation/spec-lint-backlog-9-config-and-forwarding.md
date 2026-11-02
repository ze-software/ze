# Spec: lint-backlog-9-config-and-forwarding

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | 5 findings of the full lint in config storage, the YANG CLI and the netlink interface backend, all in tests; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in
`internal/component/config/storage` and `config/yang/cli` (door
`ai/doors/config-yang.md`) and in `internal/plugins/iface/netlink` (door
`ai/doors/forwarding.md`). All five are in test files. By linter: modernize 3,
errcheck 1, misspell 1. Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/component/config/storage/store_reader_test.go` | 255 | errcheck |
| `internal/component/config/yang/cli/yang_refusal_test.go` | 16 | misspell |
| `internal/plugins/iface/netlink/address_watch_linux_test.go` | 31 | modernize |
| `internal/plugins/iface/netlink/neighbor_watch_integration_linux_test.go` | 37 | modernize |
| `internal/plugins/iface/netlink/neighbor_watch_linux_test.go` | 17 | modernize |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/config-yang.md` - config storage and the YANG CLI
  → Constraint: a test that writes config state uses its own store, never the developer's (`suite-shares-one-persistent-store`)
- [ ] `ai/doors/forwarding.md` - the netlink backend
  → Constraint: the netlink tests are Linux-only and some need `CAP_NET_ADMIN`; run them through `./le test integration iface` or the QEMU guest
- [ ] `docs/contributing/ze-go-style.md` - "The rules a tool enforces"

**Key insights:**
- The misspell in `yang_refusal_test.go` may be a quoted refusal message: if the producer's message is misspelled, fix the producer and the test together.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/yang/cli/yang_refusal_test.go` - the misspell site
- [ ] `internal/component/config/storage/store_reader_test.go` - the ignored error
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Every test's assertions.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| Linux integration runs | `./le test integration iface`, `./le test qemu all` (forwarding door, Gates) | Runs the netlink backend against a real kernel | Applies: the netlink tests' proof |
| Style rule table | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/component/config/storage/... ./internal/component/config/yang/cli/... ./internal/plugins/iface/netlink/..."`

### Transformation Path
1. Read each finding; fix it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Test edits | No |

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
| R-1 | The netlink tests cannot run on the host | The run skips for lack of capability | Run them in the QEMU guest |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the three packages | Zero findings |
| AC-2 | The config tests and the netlink tests (in the QEMU guest when the host lacks the capability) | Pass |
| AC-3 | Any `//nolint` added | Names its linter and its reason |

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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-9-config-and-forwarding.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. Fix the five findings.
3. Scoped lint zero; tests pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Test contracts | Storage tests use their own store, error checks retain assertions, and any refusal-text correction matches its producer. |
| Kernel proof | Netlink tests execute with the required capability, in the QEMU guest when needed. A skip is not AC-2 evidence (R-1). |
| Config evidence | Three scopes are lint-clean, config and netlink tests pass, and annotations state reasons (AC-1–3). |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the three packages>"` zero; `./le job run label config command go test -race ./internal/component/config/storage/... ./internal/component/config/yang/cli/...`; `./le test integration iface`. Full-run evidence belongs to the umbrella's CI run.
