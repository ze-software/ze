# Spec: release-evidence-gate

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `mk/test-integration.mk` - existing heavy test targets
3. `mk/test-functional.mk` - shell runner pattern (lines 48-86)
4. `mk/perf.mk` - perf bench/track targets
5. `Makefile` - ze-verify, ze-all, ze-all-test composition

## Task

Turn release evidence into a product gate. The default `ze-verify` gate excludes
integration, interop, stress, live, deployment, and QEMU tests because they need
external infrastructure. Some shipped functional suites are also non-gated (static,
traffic, vpp, l2tp-wire). Add a `ze-release-evidence` target that runs the full
evidence matrix while keeping `ze-verify` fast.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/656-deployment-readiness-review.md` - established ze-release-check
  → Decision: Docker-based clean-clone verify is a permanent gate target
  → Constraint: ZE_SKIP_SUITES mechanism for container-incompatible suites

### Source Files
- [ ] `mk/test-functional.mk:48-86` - shell runner pattern with continue-on-failure
  → Constraint: use same run_suite() pattern for category tracking
- [ ] `mk/test-integration.mk` - all heavy test targets and ze-deployment-preflight
  → Constraint: preflight checks tools before starting, exits non-zero on missing
- [ ] `mk/perf.mk` - ze-perf-bench and ze-perf-track targets
  → Decision: ze-perf track --check already exits non-zero on regression
- [ ] `mk/test-chaos.mk` - chaos test targets
  → Constraint: chaos tests run in-process, no external infra needed
- [ ] `mk/test-fuzz.mk` - fuzz targets with all corpora
  → Constraint: 48 fuzz targets, 10s each, ~8 min total
- [ ] `Makefile:178-194` - verify/all/all-test composition
  → Decision: ze-verify stays unchanged, new target sits alongside

**Key insights:**
- Shell runner pattern from ze-functional-test gives continue-on-failure + summary
- ze-perf track --check with thresholds already exists, just needs a Make wrapper
- ze-deployment-preflight pattern exists for tooling checks
- Non-gated functional suites need platform-specific tooling (not available on macOS)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/interop/run.py` - BGP interop test runner, Docker-based, runs scenarios against FRR/BIRD/GoBGP
- [ ] `test/perf/run.py` - perf benchmark runner, Docker-based, runs all DUTs with results to JSON
- [ ] `scripts/evidence/effective-verify.sh` - clean-clone Docker verify, ZE_SKIP_SUITES, 1200s timeout
- [ ] `scripts/evidence/qemu-run.py` - QEMU VM runner for integration tests on macOS

**Behavior to preserve:**
- `ze-verify` stays fast (~2 min), unchanged: lint + vet + unit(2-pass) + functional(12) + exabgp
- `ze-all` stays as ze-verify + chaos-verify
- `ze-all-test` stays as ze-test + chaos-verify
- All existing individual targets keep working independently
- `ze-release-check` (Docker clean-clone) stays as-is
- `ze-deployment-preflight` stays as-is (deployment-specific checks)

**Behavior to change:**
- Add `ze-release-evidence` composite target in new `mk/test-release.mk`
- Add `ze-perf-gate` target (bench + regression check)
- Add `ze-release-evidence-preflight` target (broader than deployment-preflight)

## Data Flow (MANDATORY)

N/A: This spec adds Makefile targets only. No data enters, transforms, or crosses
component boundaries. The targets compose existing test runners.

### Entry Point
- `make ze-release-evidence` invoked by operator from command line
- No runtime data flow; this is build/test infrastructure

### Transformation Path
1. Preflight check: verify Docker available (mandatory), QEMU available (advisory)
2. Shell runner iterates categories, calling existing Make targets in sequence
3. Each category returns exit code; runner tracks pass/fail/skip per category
4. Summary printed at end with colored output matching ze-functional-test style

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Make → shell | Inline shell in recipe, same as ze-functional-test | [ ] |
| Shell → Make sub-targets | `$(MAKE) ze-interop-test` etc. | [ ] |

### Integration Points
- Calls existing targets: ze-verify, ze-chaos-test, ze-fuzz-test, ze-interop-test, ze-ipsec-interop-test, ze-deployment-l2tp-ppp-docker-test, ze-static-test, ze-traffic-test, ze-vpp-test, ze-l2tp-wire-test, ze-perf-gate (new), ze-qemu-integration-test, ze-deployment-vpp-test, ze-live-test

### Architectural Verification
- [ ] No bypassed layers (calls existing targets, does not duplicate their logic)
- [ ] No unintended coupling (new file included from Makefile, no cross-dependencies)
- [ ] No duplicated functionality (composes, does not reimplement)
- [ ] Zero-copy preserved where applicable (N/A, no data buffers)

## Wiring Test (MANDATORY)

N/A: No Go code, no runtime entry points. Verification is via `make -n` dry-run
and `make help-test` output checks.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-release-evidence` | → | `mk/test-release.mk` recipe | `make -n ze-release-evidence` dry-run shows all sub-targets |
| `make ze-perf-gate` | → | `mk/test-release.mk` recipe | `make -n ze-perf-gate` dry-run shows bench + track |
| `make ze-release-evidence-preflight` | → | `mk/test-release.mk` recipe | `make ze-release-evidence-preflight` prints ok/missing |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-release-evidence-preflight` | Checks Docker, QEMU, prints ok/missing per tool, exits non-zero if Docker missing |
| AC-2 | `make ze-release-evidence` on a machine with Docker | Runs all categories in sequence, prints per-category PASS/FAIL/SKIP, summary at end |
| AC-3 | One category fails | Remaining categories still run, summary shows which failed, exit code non-zero |
| AC-4 | `ZE_RELEASE_SKIP=interop,perf make ze-release-evidence` | Named categories are skipped, shown as SKIPPED in summary |
| AC-5 | `make ze-perf-gate` | Runs ze-perf-bench then ze-perf track --check on ze results, exits non-zero on regression |
| AC-6 | `make ze-release-evidence` with no QEMU | QEMU category skipped (not failed), others still run |
| AC-7 | `make help-test` | Shows ze-release-evidence and ze-perf-gate in help output |
| AC-8 | All categories pass | Summary shows all green, exit code 0 |

## 🧪 TDD Test Plan

No Go code; no unit tests. Verification via Make dry-run and manual inspection.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A | N/A | No Go code in this spec | N/A |

### Functional Tests

N/A: This is build infrastructure (Makefile targets). No new user-facing features,
no .ci tests needed. The targets call existing test suites that already have their
own .ci coverage.

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

## Files to Modify

- `mk/test-release.mk` - new file with release evidence targets
- `Makefile` - include test-release.mk, add help-test entries

## Files to Create

- `mk/test-release.mk` - release evidence gate targets

## Implementation Steps

### Categories in ze-release-evidence

Run in this order (fast/no-infra first, slow/heavy last):

| # | Category name | Make target | Infra |
|---|--------------|-------------|-------|
| 1 | verify | `ze-verify` | None |
| 2 | chaos | `ze-chaos-test` | None |
| 3 | fuzz | `ze-fuzz-test` | None |
| 4 | interop | `ze-interop-test` | Docker |
| 5 | ipsec-interop | `ze-ipsec-interop-test` | Docker+privileged |
| 6 | l2tp-interop | `ze-deployment-l2tp-ppp-docker-test` | Docker |
| 7 | functional-extra | static + traffic + vpp + l2tp-wire | Platform deps |
| 8 | perf | `ze-perf-gate` | Docker |
| 9 | qemu | `ze-qemu-integration-test` | QEMU |
| 10 | vpp-deployment | `ze-deployment-vpp-test` | Docker+privileged |
| 11 | live | `ze-live-test` | Docker+internet |

### Phase 1: Create mk/test-release.mk

1. Header comment with quick reference
2. `.PHONY` declarations for all new targets
3. `ze-release-evidence-preflight`: check Docker (mandatory), QEMU (optional), print status
4. `ze-perf-gate`: depends on ze-perf, runs bench for ze DUT then track --check
5. `ze-release-evidence`: shell runner with run_category() function, ZE_RELEASE_SKIP support, summary

### Phase 2: Wire into Makefile

1. Add `include mk/test-release.mk` to the include block
2. Add help-test entries for ze-release-evidence, ze-perf-gate, ze-release-evidence-preflight

### Critical Review Checklist

| Check | What to verify |
|-------|---------------|
| Completeness | All 11 categories wired, help entries added |
| Correctness | Each category calls the right existing target |
| Skip logic | ZE_RELEASE_SKIP comma-separated parsing works |
| Preflight | Docker check is mandatory, QEMU is advisory |
| Summary | Matches ze-functional-test output style (PASS green, FAIL red, SKIP yellow) |
| Exit code | Non-zero if any category failed (not skipped) |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `mk/test-release.mk` exists | `ls mk/test-release.mk` |
| Preflight target works | `make ze-release-evidence-preflight` |
| Perf gate target works | `make -n ze-perf-gate` |
| Evidence target works | `make -n ze-release-evidence` |
| Makefile includes test-release.mk | `grep 'test-release.mk' Makefile` |
| Help entries present | `make help-test` shows new targets |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] `make -n ze-release-evidence` shows correct target expansion
- [ ] Feature code integrated (`mk/test-release.mk`, `Makefile`)
- [ ] `make ze-test` passes (lint + all ze tests)

### Design
- [ ] No premature abstraction
- [ ] Follows ze-functional-test shell runner pattern
- [ ] Minimal coupling (calls existing targets, no new Go code)

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] N/A: no Go code, verification via Make dry-run
