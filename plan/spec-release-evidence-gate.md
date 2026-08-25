# Spec: release-evidence-gate

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/6 |
| Updated | 2026-07-22 |

Phase note (was in the Phase cell; moved 2026-07-22): implementation landed --
`ze-evidence-release-verify` is defined at `mk/test-release.mk` and wired in the
Makefile (commit `d0e9d388c` "test: add release evidence gate runner"). The
outstanding step is the full evidence-matrix verification re-run.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `mk/test-integration.mk` - existing heavy test targets
3. `mk/test-functional.mk` - shell runner pattern (lines 48-86)
4. `mk/perf-bench.mk` - perf bench/track targets
5. `Makefile` - ze-precommit-verify, ze-verify-all, ze-test-all composition

## Task

Turn release evidence into a product gate. The default `ze-precommit-verify` gate excludes
integration, interop, stress, live, deployment, and QEMU tests because they need
external infrastructure. Some shipped functional suites are also non-gated (static,
traffic, vpp, l2tp-wire). Add a `ze-evidence-release-verify` target that runs the full
evidence matrix while keeping `ze-precommit-verify` fast.

### Inherited item: the `static` suite currently runs NOWHERE (2026-07-27)

Rehomed here from the `fixit-sleeps-cli-harness` spec at its closure (the spec is gone;
its rows live in `plan/deferrals/ad-hoc-2026-07-27-2c83641a.md`). It was that spec's A-4
obligation ("wire the linux-gated static suite into an actually-run linux path, do NOT
drop the gate") and was the one live item left in it. Verified unmet by an independent
check, with the tree having moved AGAINST it since:

| Producer | Fact |
|----------|------|
| `test/static/static-show.ci`, `test/static/static-table-interface.ci` | both carry `option=needs-linux` |
| `internal/test/runner/record_parse.go` | on `GOOS != linux` the record gets a `SkipReason`, so they never run on the darwin dev host |
| `mk/test-functional.mk` (`all_suites`) | no `static`, so `make ze-precommit-verify` never runs it |
| `scripts/evidence/qemu-all-tests.sh` (`fsuite` lines) | no `static`, so `make ze-qemu-needs-linux-test` never runs it -- and that is the only automated Linux functional path (`.github/workflows/qemu-nightly.yml`) |
| `mk/test-functional.mk`, `mk/test-release.mk` | the suite's only two invocation sites tree-wide, and `ze-evidence-release-verify` is invoked by no workflow |

So a rewrite fixed a real defect in those tests and left them behind a gate no runner
honors. They are not skipped honestly; they are simply never reached. Either add `static`
to the QEMU functional list, or make `ze-evidence-release-verify` an invoked path -- this spec's
own subject.

Carry with it: both tests run `ip link add` in `setup.py` (`004:26-30`, `005:28-35`) while
declaring `option=needs-linux` with no `caps=net-admin`. `record_parse.go`
documents that exact shape as fail-open: on an unprivileged Linux host they hang or fail
rather than skipping honestly.

## Required Reading

### Architecture Docs
- [x] The deployment-readiness-review record (retired with the learned corpus) - established ze-evidence-release-candidate-check
  → Decision: Docker-based clean-clone verify is a permanent gate target
  → Constraint: ZE_SKIP_SUITES mechanism for container-incompatible suites

### Source Files
- [x] `mk/test-functional.mk` - shell runner pattern with continue-on-failure
  → Constraint: use same run_suite() pattern for category tracking
- [x] `mk/test-integration.mk` - all heavy test targets and ze-deployment-preflight
  → Constraint: preflight checks tools before starting, exits non-zero on missing
- [x] `mk/perf-bench.mk` - ze-perf-bench and ze-perf-history-record targets
  → Decision: ze-perf track --check already exits non-zero on regression
- [x] `mk/test-chaos.mk` - chaos test targets
  → Constraint: chaos tests run in-process, no external infra needed
- [x] `mk/test-fuzz.mk` - fuzz targets with all corpora
  → Constraint: 48 fuzz targets, 10s each, ~8 min total
- [x] `Makefile:178-194` - verify/all/all-test composition
  → Decision: ze-precommit-verify stays unchanged, new target sits alongside

**Key insights:**
- Shell runner pattern from ze-functional-test gives continue-on-failure + summary
- ze-perf track --check with thresholds already exists, just needs a Make wrapper
- ze-deployment-preflight pattern exists for tooling checks
- Non-gated functional suites need platform-specific tooling (not available on macOS)

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `test/interop/run.py` - BGP interop test runner, Docker-based, runs scenarios against FRR/BIRD/GoBGP
- [x] `test/perf/run.py` - perf benchmark runner, Docker-based, runs all DUTs with results to JSON
- [x] `scripts/evidence/effective-verify.sh` - clean-clone Docker verify, ZE_SKIP_SUITES, 1200s timeout
- [x] `scripts/evidence/qemu-run.py` - QEMU VM runner for integration tests on macOS

**Behavior to preserve:**
- `ze-precommit-verify` stays fast (~2 min), unchanged: lint + vet + unit(2-pass) + functional(12) + exabgp
- `ze-verify-all` stays as ze-precommit-verify + chaos-verify
- `ze-test-all` stays as ze-test + chaos-verify
- All existing individual targets keep working independently
- `ze-evidence-release-candidate-check` (Docker clean-clone) stays as-is
- `ze-deployment-preflight` stays as-is (deployment-specific checks)

**Behavior to change:**
- Add `ze-evidence-release-verify` composite target in new `mk/test-release.mk`
- Add `ze-evidence-perf-record` target (bench + regression check)
- Add `ze-evidence-release-preflight` target (broader than deployment-preflight)

## Data Flow (MANDATORY)

N/A: This spec adds Makefile targets only. No data enters, transforms, or crosses
component boundaries. The targets compose existing test runners.

### Entry Point
- `make ze-evidence-release-verify` invoked by operator from command line
- No runtime data flow; this is build/test infrastructure

### Transformation Path
1. Preflight check: verify Docker available (mandatory), QEMU available (advisory)
2. Shell runner iterates categories, calling existing Make targets in sequence
3. Each category returns exit code; runner tracks pass/fail/skip per category
4. Summary printed at end with colored output matching ze-functional-test style

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Make → shell | Inline shell in recipe, same as ze-functional-test | [x] |
| Shell → Make sub-targets | `$(MAKE) ze-interop-test` etc. | [x] |

### Integration Points
- Calls existing targets: ze-precommit-verify, ze-chaos-test, ze-fuzz-test, ze-interop-test, ze-interop-ipsec-test, ze-deployment-docker-l2tp-ppp-test, ze-functional-static-test, ze-functional-traffic-test, ze-functional-vpp-test, ze-functional-l2tp-wire-test, ze-evidence-perf-record (new), ze-qemu-integration-test, ze-deployment-vpp-test, ze-live-test

### Architectural Verification
- [x] No bypassed layers (calls existing targets, does not duplicate their logic)
- [x] No unintended coupling (new file included from Makefile, no cross-dependencies)
- [x] No duplicated functionality (composes, does not reimplement)
- [x] Zero-copy preserved where applicable (N/A, no data buffers)

## Wiring Test (MANDATORY)

N/A: No Go code, no runtime entry points. Verification is via `make -n` dry-run
and `make help-test` output checks.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-evidence-release-verify` | → | `mk/test-release.mk` recipe | `make -n ze-evidence-release-verify` dry-run shows all sub-targets |
| `make ze-evidence-perf-record` | → | `mk/test-release.mk` recipe | `make -n ze-evidence-perf-record` dry-run shows bench + track |
| `make ze-evidence-release-preflight` | → | `mk/test-release.mk` recipe | `make ze-evidence-release-preflight` prints ok/missing |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-evidence-release-preflight` | Checks Docker, QEMU, prints ok/missing per tool, exits non-zero if Docker missing |
| AC-2 | `make ze-evidence-release-verify` on a machine with Docker | Runs all categories in sequence, prints per-category PASS/FAIL/SKIP, summary at end |
| AC-3 | One category fails | Remaining categories still run, summary shows which failed, exit code non-zero |
| AC-4 | `ZE_RELEASE_SKIP=interop,perf make ze-evidence-release-verify` | Named categories are skipped, shown as SKIPPED in summary |
| AC-5 | `make ze-evidence-perf-record` | Runs ze-perf-bench then ze-perf track --check on ze results, exits non-zero on regression |
| AC-6 | `make ze-evidence-release-verify` with no QEMU | QEMU category skipped (not failed), others still run |
| AC-7 | `make help-test` | Shows ze-evidence-release-verify and ze-evidence-perf-record in help output |
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

### Categories in ze-evidence-release-verify

Run in this order (fast/no-infra first, slow/heavy last):

| # | Category name | Make target | Infra |
|---|--------------|-------------|-------|
| 1 | verify | `ze-precommit-verify` | None |
| 2 | chaos | `ze-chaos-test` | None |
| 3 | fuzz | `ze-fuzz-test` | None |
| 4 | interop | `ze-interop-test` | Docker |
| 5 | ipsec-interop | `ze-interop-ipsec-test` | Docker+privileged |
| 6 | l2tp-interop | `ze-deployment-docker-l2tp-ppp-test` | Docker |
| 7 | functional-extra | static + traffic + vpp + l2tp-wire | Platform deps |
| 8 | perf | `ze-evidence-perf-record` | Docker |
| 9 | qemu | `ze-qemu-integration-test` | QEMU |
| 10 | vpp-deployment | `ze-deployment-vpp-test` | Docker+privileged |
| 11 | live | `ze-live-test` | Docker+internet |

### Phase 1: Create mk/test-release.mk

1. Header comment with quick reference
2. `.PHONY` declarations for all new targets
3. `ze-evidence-release-preflight`: check Docker (mandatory), QEMU (optional), print status
4. `ze-evidence-perf-record`: depends on `ze-perf-build`, runs the benchmark for the Ze DUT, then runs `ze-perf track --check`
5. `ze-evidence-release-verify`: shell runner with run_category() function, ZE_RELEASE_SKIP support, summary

### Phase 2: Wire into Makefile

1. Add `include mk/test-release.mk` to the include block
2. Add help-test entries for ze-evidence-release-verify, ze-evidence-perf-record, ze-evidence-release-preflight

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
| Preflight target works | `make ze-evidence-release-preflight` |
| Perf gate target works | `make -n ze-evidence-perf-record` |
| Evidence target works | `make -n ze-evidence-release-verify` |
| Makefile includes test-release.mk | `grep 'test-release.mk' Makefile` |
| Help entries present | `make help-test` shows new targets |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [x] `make -n ze-evidence-release-verify` shows correct target expansion
- [x] Feature code integrated (`mk/test-release.mk`, `Makefile`)
- [ ] `make ze-standard-test` passes (lint + all ze tests)

### Design
- [x] No premature abstraction
- [x] Follows ze-functional-test shell runner pattern
- [x] Minimal coupling (calls existing targets, no new Go code)

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [x] N/A: no Go code, verification via Make dry-run

## Verification Evidence (2026-05-24)

| Check | Result | Evidence |
|-------|--------|----------|
| AC-1 preflight success | PASS | `make ze-evidence-release-preflight` found Docker and `qemu-system-x86_64`, exited 0 |
| AC-1 Docker missing | PASS | `PATH="/usr/bin:/bin" make ze-evidence-release-preflight` reported Docker missing and exited 1 |
| AC-2 category runner | PARTIAL | `make ZE_RELEASE_SKIP=verify ze-evidence-release-verify` ran the non-skipped matrix and printed per-category PASS/FAIL/SKIP plus summary |
| AC-3 continue after failure | PASS | `make MAKE=false ZE_RELEASE_SKIP=fuzz,interop,ipsec-interop,l2tp-interop,functional-extra,perf,qemu,vpp-deployment,live ze-evidence-release-verify` reported verify and chaos failures, then skipped remaining named categories and exited 1 |
| AC-4 explicit skip | PASS | `make MAKE=true ZE_RELEASE_SKIP=interop,perf ze-evidence-release-verify` reported `SKIPPED: interop perf` and exited 0 |
| AC-5 perf gate | PARTIAL | `make -n ze-evidence-perf-record` shows `ze-perf-bench PERF_DUT=ze`, history append, and `bin/ze-perf track --check`; full run failed because current `cmd/ze/hub` does not build in Docker |
| AC-6 no QEMU skip | PASS | `make MAKE=true ZE_RELEASE_QEMU_BIN=definitely-not-qemu ZE_RELEASE_SKIP=interop,ipsec-interop,l2tp-interop,perf,vpp-deployment,live ze-evidence-release-verify` skipped qemu and exited 0 |
| AC-7 help output | PASS | `make help-test` shows `ze-evidence-release-preflight`, `ze-evidence-release-verify`, and `ze-evidence-perf-record` |
| AC-8 all categories pass | FAIL | Not demonstrated. `make ZE_RELEASE_SKIP=verify ze-evidence-release-verify` failed 7 of 10 attempted categories |
| Required final gate | FAIL | `make ze-standard-test` fails at `ze-lint` on unrelated `cmd/ze/service` errcheck/modernize/unused issues and `internal/component/web/handler_config_test.go` gofmt |

## Unblock record (2026-07-10)

User instruction 2026-07-10: unblock. The 2026-05-24 blockers were re-verified
against current code (followup-wave impact review):

| 2026-05-24 blocker | Status today (verified firsthand) |
|--------------------|-----------------------------------|
| `wireManagedCommit` undefined breaks `go build ./cmd/ze` | resolved: defined `cmd/ze/hub/managed.go` (takes `audit.Recorder`), called `cmd/ze/hub/main.go` |
| `buildSessionModelFactory` call sites missing `audit.Recorder` | resolved: signature carries `recorder audit.Recorder` at `cmd/ze/hub/session_factory.go` |
| `make ze-standard-test` blocked at ze-lint (service/web lint reds) | to be proven by the next full `make ze-precommit-verify` (ze-lint is a stage of it); a green run supersedes this row |

Additional post-wave corrections:
- Required Reading cites `Makefile:178-194` for verify composition; `ze-precommit-verify` is now
  at `Makefile:276` and `_ze-verify-impl` carries a longer gate list
  (ze-tier-check, ze-iface-resolution-check, ze-plugin-boundary-check,
  ze-port-defaults-check, ze-platform-vet, ze-cli-grammar-check, ...).
- The evidence matrix categories predate wave-added heavy suites; the re-run should
  fold in `ze-deployment-vpp-iface-test` (`mk/test-integration.mk`) and the new
  functional `.ci` (as112-dot/doh, exabgp-bridge-internal, mcp-get-sse,
  test/traffic 020-026) via their existing category targets.

Remaining work: re-run `make ze-evidence-release-verify` on a capable host (Docker +
QEMU + privileged), record fresh per-category results, fill the Review Gate, close.

Blocked failures from the (superseded) 2026-05-24 release evidence run:

| Category | Result | Cause |
|----------|--------|-------|
| verify | SKIPPED | User instructed to skip tests that cannot run; `make ze-standard-test` is blocked by unrelated service/web lint failures |
| chaos | PASS | Release evidence run passed this category |
| fuzz | PASS | Release evidence run passed this category |
| interop | FAIL | 24 interop scenarios passed, 11 failed |
| ipsec-interop | FAIL | Linux cross-build failed: `buildSessionModelFactory` call sites missing `audit.Recorder` argument |
| l2tp-interop | FAIL | Host kernel missing PPPoL2TP requirements |
| functional-extra | FAIL | `ze-functional-static-test`, `ze-functional-traffic-test`, `ze-functional-vpp-test`, and `ze-functional-l2tp-wire-test` failed because `cmd/ze/hub` does not build |
| perf | FAIL | Docker build for Ze image failed because `cmd/ze/hub` does not build |
| qemu | FAIL | QEMU integration failures in `internal/component/iface` and `internal/plugins/firewall/nft` |
| vpp-deployment | FAIL | `go build ./cmd/ze` failed: `wireManagedCommit` undefined |
| live | PASS | RPKI live tests passed; ASPA live test skipped by test because stayrtr did not serve ASPA records |
