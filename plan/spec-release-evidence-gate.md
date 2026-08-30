# Spec: release-evidence-gate

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/6 |
| Updated | 2026-07-22 |

Phase note (was in the Phase cell; moved 2026-07-22): implementation landed --
`ze-evidence-release-verify` is defined at `internal/le/evidence/evidence.go` and wired in the
the native action tables under `internal/le/` (commit `d0e9d388c` "test: add release evidence gate runner"). The
outstanding step is the full evidence-matrix verification re-run.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/le/integration/gates.go` - existing heavy test targets
3. `internal/le/functional/suites.go` - shell runner pattern (lines 48-86)
4. `internal/le/perfbench/actions.go` - perf bench/track targets
5. `internal/le/` native action tables - ./le verify current mode full, ze-verify-all, ze-test-all composition

## Task

Turn release evidence into a product gate. The default `./le verify current mode full` gate excludes
integration, interop, stress, live, deployment, and QEMU tests because they need
external infrastructure. Some shipped functional suites are also non-gated (static,
traffic, vpp, l2tp-wire). Add a `ze-evidence-release-verify` target that runs the full
evidence matrix while keeping `./le verify current mode full` fast.

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
| `internal/le/functional/suites.go` (`all_suites`) | no `static`, so `./le verify current mode full` never runs it |
| `internal/le/qemu/alltests.go` (`fsuite` lines) | no `static`, so `./le qemu run command "./le qemu all-tests"` never runs it -- and that is the only automated Linux functional path (`.github/workflows/qemu-nightly.yml`) |
| `internal/le/functional/suites.go`, `internal/le/evidence/evidence.go` | the suite's only two invocation sites tree-wide, and `ze-evidence-release-verify` is invoked by no workflow |

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
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` test file format: embedded files, options, expectations and commands
- [x] The deployment-readiness-review record (retired with the learned corpus) - established ./le evidence release-candidate
  → Decision: Docker-based clean-clone verify is a permanent gate target
  → Constraint: ZE_SKIP_SUITES mechanism for container-incompatible suites

### Source Files
- [x] `internal/le/functional/suites.go` - shell runner pattern with continue-on-failure
  → Constraint: use same run_suite() pattern for category tracking
- [x] `internal/le/integration/gates.go` - all heavy test targets and ze-deployment-preflight
  → Constraint: preflight checks tools before starting, exits non-zero on missing
- [x] `internal/le/perfbench/actions.go` - ze-perf-bench and ze-perf-history-record targets
  → Decision: ze-perf track --check already exits non-zero on regression
- [x] `internal/le/testchaos/actions.go` - chaos test targets
  → Constraint: chaos tests run in-process, no external infra needed
- [x] `internal/le/fuzz/actions.go` - fuzz targets with all corpora
  → Constraint: 48 fuzz targets, 10s each, ~8 min total
- [x] `internal/le/` native action tables - verify/all/all-test composition
  → Decision: ./le verify current mode full stays unchanged, new target sits alongside

**Key insights:**
- Shell runner pattern from ./le functional gives continue-on-failure + summary
- ze-perf track --check with thresholds already exists, just needs a Make wrapper
- ze-deployment-preflight pattern exists for tooling checks
- Non-gated functional suites need platform-specific tooling (not available on macOS)

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `test/interop/run.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - BGP interop test runner, Docker-based, runs scenarios against FRR/BIRD/GoBGP
- [x] `test/perf/run.py` (retired; now `internal/test/perfrunner`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - perf benchmark runner, Docker-based, runs all DUTs with results to JSON
- [x] `internal/le/deployment/` - clean-clone Docker verify, ZE_SKIP_SUITES, 1200s timeout
- [x] `internal/le/qemu/run.go` - QEMU VM runner for integration tests on macOS

**Behavior to preserve:**
- `./le verify current mode full` stays fast (~2 min), unchanged: lint + vet + unit(2-pass) + functional(12) + exabgp
- `ze-verify-all` stays as ./le verify current mode full + chaos-verify
- `ze-test-all` stays as ze-test + chaos-verify
- All existing individual targets keep working independently
- `./le evidence release-candidate` (Docker clean-clone) stays as-is
- `ze-deployment-preflight` stays as-is (deployment-specific checks)

**Behavior to change:**
- Add `ze-evidence-release-verify` composite target in new `internal/le/evidence/evidence.go`
- Add `ze-evidence-perf-record` target (bench + regression check)
- Add `ze-evidence-release-preflight` target (broader than deployment-preflight)

## Data Flow (MANDATORY)

N/A: This spec adds the native action tables under `internal/le/` targets only. No data enters, transforms, or crosses
component boundaries. The targets compose existing test runners.

### Entry Point
- `./le evidence release-candidate` invoked by operator from command line
- No runtime data flow; this is build/test infrastructure

### Transformation Path
1. Preflight check: verify Docker available (mandatory), QEMU available (advisory)
2. Shell runner iterates categories, calling existing Make targets in sequence
3. Each category returns exit code; runner tracks pass/fail/skip per category
4. Summary printed at end with colored output matching ./le functional style

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Make → shell | Inline shell in recipe, same as ./le functional | [x] |
| Shell → Make sub-targets | `$(MAKE) ./le integration interop` etc. | [x] |

### Integration Points
- Calls existing targets: ./le verify current mode full, ze-chaos-test, ze-fuzz-test, ./le integration interop, ./le integration interop-ipsec, ./le deployment docker-l2tp-ppp-test, ze-functional-static-test, ze-functional-traffic-test, ze-functional-vpp-test, ze-functional-l2tp-wire-test, ze-evidence-perf-record (new), ze-qemu-integration-test, ./le deployment vpp-test, ze-live-test

### Architectural Verification
- [x] No bypassed layers (calls existing targets, does not duplicate their logic)
- [x] No unintended coupling (new file included from the native action tables under `internal/le/`, no cross-dependencies)
- [x] No duplicated functionality (composes, does not reimplement)
- [x] Zero-copy preserved where applicable (N/A, no data buffers)

## Wiring Test (MANDATORY)

N/A: No Go code, no runtime entry points. Verification is via `make -n` dry-run
and `make help-test` output checks.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le evidence release-candidate` | → | `internal/le/evidence/evidence.go` recipe | `make -n ze-evidence-release-verify` dry-run shows all sub-targets |
| `./le perf-bench record` | → | `internal/le/evidence/evidence.go` recipe | `make -n ze-evidence-perf-record` dry-run shows bench + track |
| `./le evidence release-candidate` | → | `internal/le/evidence/evidence.go` recipe | `./le evidence release-candidate` prints ok/missing |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le evidence release-candidate` | Checks Docker, QEMU, prints ok/missing per tool, exits non-zero if Docker missing |
| AC-2 | `./le evidence release-candidate` on a machine with Docker | Runs all categories in sequence, prints per-category PASS/FAIL/SKIP, summary at end |
| AC-3 | One category fails | Remaining categories still run, summary shows which failed, exit code non-zero |
| AC-4 | `ZE_RELEASE_SKIP=interop,perf ./le evidence release-candidate` | Named categories are skipped, shown as SKIPPED in summary |
| AC-5 | `./le perf-bench record` | Runs ze-perf-bench then ze-perf track --check on ze results, exits non-zero on regression |
| AC-6 | the retired `ze-evidence-release-verify` (current: `./le evidence release-candidate`) with no QEMU | QEMU category skipped (not failed), others still run |
| AC-7 | `make help-test` | Shows ze-evidence-release-verify and ze-evidence-perf-record in help output |
| AC-8 | All categories pass | Summary shows all green, exit code 0 |

## 🧪 TDD Test Plan

No Go code; no unit tests. Verification via Make dry-run and manual inspection.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A | N/A | No Go code in this spec | N/A |

### Functional Tests

N/A: This is build infrastructure (the native action tables under `internal/le/` targets). No new user-facing features,
no .ci tests needed. The targets call existing test suites that already have their
own .ci coverage.

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

## Files to Modify

- `internal/le/evidence/evidence.go` - new file with release evidence targets
- `internal/le/` native action tables - include test-release.mk, add help-test entries

## Files to Create

- `internal/le/evidence/evidence.go` - release evidence gate targets

## Implementation Steps

### Categories in ze-evidence-release-verify

Run in this order (fast/no-infra first, slow/heavy last):

| # | Category name | Make target | Infra |
|---|--------------|-------------|-------|
| 1 | verify | `./le verify current mode full` | None |
| 2 | chaos | `ze-chaos-test` | None |
| 3 | fuzz | `ze-fuzz-test` | None |
| 4 | interop | `./le integration interop` | Docker |
| 5 | ipsec-interop | `./le integration interop-ipsec` | Docker+privileged |
| 6 | l2tp-interop | `./le deployment docker-l2tp-ppp-test` | Docker |
| 7 | functional-extra | static + traffic + vpp + l2tp-wire | Platform deps |
| 8 | perf | `ze-evidence-perf-record` | Docker |
| 9 | qemu | `ze-qemu-integration-test` | QEMU |
| 10 | vpp-deployment | `./le deployment vpp-test` | Docker+privileged |
| 11 | live | `ze-live-test` | Docker+internet |

### Phase 1: Create internal/le/evidence/evidence.go

1. Header comment with quick reference
2. `.PHONY` declarations for all new targets
3. `ze-evidence-release-preflight`: check Docker (mandatory), QEMU (optional), print status
4. `ze-evidence-perf-record`: depends on `ze-perf-build`, runs the benchmark for the Ze DUT, then runs `ze-perf track --check`
5. `ze-evidence-release-verify`: shell runner with run_category() function, ZE_RELEASE_SKIP support, summary

### Phase 2: Wire into the native action tables under `internal/le/`

1. Add `include internal/le/evidence/evidence.go` to the include block
2. Add help-test entries for ze-evidence-release-verify, ze-evidence-perf-record, ze-evidence-release-preflight

### Critical Review Checklist

| Check | What to verify |
|-------|---------------|
| Completeness | All 11 categories wired, help entries added |
| Correctness | Each category calls the right existing target |
| Skip logic | ZE_RELEASE_SKIP comma-separated parsing works |
| Preflight | Docker check is mandatory, QEMU is advisory |
| Summary | Matches ./le functional output style (PASS green, FAIL red, SKIP yellow) |
| Exit code | Non-zero if any category failed (not skipped) |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/le/evidence/evidence.go` exists | `ls internal/le/evidence/evidence.go` |
| Preflight target works | `./le evidence release-candidate` |
| Perf gate target works | `make -n ze-evidence-perf-record` |
| Evidence target works | `make -n ze-evidence-release-verify` |
| the native action tables under `internal/le/` includes test-release.mk | `grep 'test-release.mk' the native action tables under `internal/le/`` |
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
- [x] Feature code integrated (`internal/le/evidence/evidence.go`, `internal/le/` native action tables)
- [ ] `./le verify worktree` passes (lint + all ze tests)

### Design
- [x] No premature abstraction
- [x] Follows ./le functional shell runner pattern
- [x] Minimal coupling (calls existing targets, no new Go code)

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [x] N/A: no Go code, verification via Make dry-run

## Verification Evidence (2026-05-24)

| Check | Result | Evidence |
|-------|--------|----------|
| AC-1 preflight success | PASS | `./le evidence release-candidate` found Docker and `qemu-system-x86_64`, exited 0 |
| AC-1 Docker missing | PASS | `PATH="/usr/bin:/bin" the retired make ze-evidence-release-preflight (current: ./le evidence release-candidate)` reported Docker missing and exited 1 |
| AC-2 category runner | PARTIAL | `make ZE_RELEASE_SKIP=verify ze-evidence-release-verify` ran the non-skipped matrix and printed per-category PASS/FAIL/SKIP plus summary |
| AC-3 continue after failure | PASS | `make MAKE=false ZE_RELEASE_SKIP=fuzz,interop,ipsec-interop,l2tp-interop,functional-extra,perf,qemu,vpp-deployment,live ze-evidence-release-verify` reported verify and chaos failures, then skipped remaining named categories and exited 1 |
| AC-4 explicit skip | PASS | `make MAKE=true ZE_RELEASE_SKIP=interop,perf ze-evidence-release-verify` reported `SKIPPED: interop perf` and exited 0 |
| AC-5 perf gate | PARTIAL | `make -n ze-evidence-perf-record` shows `ze-perf-bench PERF_DUT=ze`, history append, and `bin/ze-perf track --check`; full run failed because current `cmd/ze/hub` does not build in Docker |
| AC-6 no QEMU skip | PASS | `make MAKE=true ZE_RELEASE_QEMU_BIN=definitely-not-qemu ZE_RELEASE_SKIP=interop,ipsec-interop,l2tp-interop,perf,vpp-deployment,live ze-evidence-release-verify` skipped qemu and exited 0 |
| AC-7 help output | PASS | `make help-test` shows `ze-evidence-release-preflight`, `ze-evidence-release-verify`, and `ze-evidence-perf-record` |
| AC-8 all categories pass | FAIL | Not demonstrated. `make ZE_RELEASE_SKIP=verify ze-evidence-release-verify` failed 7 of 10 attempted categories |
| Required final gate | FAIL | `./le verify current mode full` fails at `./le verify lint run` on unrelated `cmd/ze/service` errcheck/modernize/unused issues and `internal/component/web/handler_config_test.go` gofmt |

## Unblock record (2026-07-10)

User instruction 2026-07-10: unblock. The 2026-05-24 blockers were re-verified
against current code (followup-wave impact review):

| 2026-05-24 blocker | Status today (verified firsthand) |
|--------------------|-----------------------------------|
| `wireManagedCommit` undefined breaks `go build ./cmd/ze` | resolved: defined `cmd/ze/hub/managed.go` (takes `audit.Recorder`), called `cmd/ze/hub/main.go` |
| `buildSessionModelFactory` call sites missing `audit.Recorder` | resolved: signature carries `recorder audit.Recorder` at `cmd/ze/hub/session_factory.go` |
| `./le verify current mode full` blocked at ./le verify lint run (service/web lint reds) | to be proven by the next full `./le verify current mode full` (./le verify lint run is a stage of it); a green run supersedes this row |

Additional post-wave corrections:
- Required Reading cites `internal/le/` native action tables for verify composition; `./le verify current mode full` is now
  at `internal/le/` native action tables and `_ze-verify-impl` carries a longer gate list
  (./le tier check, ze-iface-resolution-check, ./le plugin boundary check,
  ./le port-defaults check, ze-platform-vet, ./le cli-grammar, ...).
- The evidence matrix categories predate wave-added heavy suites; the re-run should
  fold in `./le deployment vpp-iface-test` (`internal/le/integration/gates.go`) and the new
  functional `.ci` (as112-dot/doh, exabgp-bridge-internal, mcp-get-sse,
  test/traffic 020-026) via their existing category targets.

Remaining work: re-run `./le evidence release-candidate` on a capable host (Docker +
QEMU + privileged), record fresh per-category results, fill the Review Gate, close.

Blocked failures from the (superseded) 2026-05-24 release evidence run:

| Category | Result | Cause |
|----------|--------|-------|
| verify | SKIPPED | User instructed to skip tests that cannot run; `./le verify current mode full` is blocked by unrelated service/web lint failures |
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
