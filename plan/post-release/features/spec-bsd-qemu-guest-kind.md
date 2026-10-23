# Spec: bsd-qemu-guest-kind

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Adds a BSD platform or backend; release correctness of existing Linux support is not deferred. |
| Scope | tooling |
| Depends | - |
| Phase | 0/0 (Do not start until the owner approves) |
| Handoff | - |
| Updated | 2026-10-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`plan/post-release/features/spec-bsd-family-interface-backend.md` owes runtime proof on FreeBSD and
OpenBSD, and nothing in the test harness can boot either. `internal/le/test/qemu`
boots Alpine Linux guests only (`guest_linux.go`; `guestlabs_other.go` refuses off
Linux), and every GitHub job runs on `ubuntu-latest`. This spec adds a BSD guest
kind to the QEMU harness so that `test/bsd/*.ci` and the BSD interop scenarios run
inside a FreeBSD or OpenBSD guest, a macOS CI job for the darwin rows, and the rule
that says how BSD runtime tests run.

| Item | Notes |
|------|-------|
| FreeBSD guest image | Official VM image at the main spec's minimum version (Q4: FreeBSD 14), booted by the existing QEMU driver; isolation by VNET jails plus `epair(4)` |
| OpenBSD guest image | Minimum OpenBSD 7.5; isolation by rdomains plus `pair(4)` |
| Guest runner | Cross-compiled `ze` and test binaries copied in, `test/bsd/*.ci` run inside, results returned with the same plan/reach/count contract as `all-tests` |
| Interop peers | FRR, BIRD, OpenBGPD from packages inside the guest, or in a Linux guest on a shared QEMU network (open question 2) |
| CI | `vmactions/freebsd-vm` and `vmactions/openbsd-vm` for nightly, or self-booted; a `macos-latest` job for darwin rows (GitHub macOS runners have `sudo`) |
| Rule | `ai/rules/platform-bsd.md`, authored under `ai/rules/rule-format.md`: BSD runtime tests run in a guest, never natively; golden-byte tests run anywhere |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/qemu-integration.md` - guest kinds, entry points, `option=needs-linux`
  → Constraint: `all-tests` is a guest action; the BSD guest needs the same contract (plan, reach, count) rather than a parallel runner
- [ ] `ai/rules/platform-linux.md` - Linux-only tests via QEMU
  → Constraint: the BSD rule mirrors its structure; the two rules must not contradict on where `.ci` tests run
- [ ] `docs/architecture/testing/ci-workflows.md` - current workflows
  → Constraint: every job is `ubuntu-latest` today
- [ ] `ai/META-DECISIONS.md` - root only for the stress harness
  → Constraint: guest setup needs root inside the guest only, never on the host

**Key insights:**
- [to be filled at design]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/test/qemu/guest_linux.go` - Alpine guest boot
- [ ] `internal/le/test/qemu/guestlabs_other.go` - refuses guest labs off Linux
- [ ] `.github/workflows/qemu-nightly.yml` - nightly QEMU job

**Behavior to preserve:**
- Linux guest kinds and their tests unchanged.

**Behavior to change:**
- A `freebsd` and an `openbsd` guest kind exist and run `test/bsd/*.ci`.

## Open Questions

| # | Question | Recommendation |
|---|----------|----------------|
| 1 | Self-booted QEMU images or `vmactions/*-vm` in CI? | Self-booted for local runs, `vmactions` acceptable for nightly |
| 2 | Peers inside the BSD guest or in a Linux guest beside it? | Inside the guest from packages: one VM, fewer moving parts |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: `./le test qemu ...` with a BSD guest kind]
- [Format at entry]

### Transformation Path
1. [to be designed]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Host to guest | [serial console / ssh / 9p] | No |

### Integration Points
- [to be designed]

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | QEMU on the build hosts boots FreeBSD 14 and OpenBSD 7.5 images without host-specific acceleration | [to be checked] | [slow or no runs] | [boot test] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Image download size and boot time make the suite impractical locally | [boot time] | [cached image under `tmp/`] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test qemu` with the FreeBSD guest kind | → | [BSD guest boot] | [test name at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le test qemu` with the FreeBSD guest kind and one `test/bsd/*.ci` | Guest boots, test runs, result returned with plan/reach/count |
| AC-2 | Same on OpenBSD | Same |
| AC-3 | A `test/bsd/*.ci` run natively on a Linux or macOS host | Refused with the command that runs it in a guest |
| AC-4 | macOS CI job | Runs the darwin `test/bsd/` rows with `sudo` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to be designed] | `internal/le/test/qemu/` | [guest kind selection] | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [to be designed] | `test/bsd/` | [one trivial `.ci` boots and passes in each guest] | |

## Files to Modify
- `internal/le/test/qemu/` - BSD guest kind
- `.github/workflows/qemu-nightly.yml` - BSD nightly job
- `.github/workflows/verify.yml` - macOS job
- `docs/architecture/testing/qemu-integration.md` - BSD guest section
- `docs/functional-tests.md` - `test/bsd/`

## Files to Create
- `ai/rules/platform-bsd.md` - BSD runtime test rule
- `test/bsd/` - directory and a first `.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| [to be answered at design] | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/qemu-integration.md` |

## Implementation Steps

1. **Phase: Wiring** -- [to be designed]

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
