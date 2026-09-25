# Spec: release-evidence-gate

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-09-19 |

The historical Make matrix implementation landed at `d0e9d388c`, but the native
cutover retired its composite runner (`internal/le/completeness_record_test.go`,
`retiredProducers`). `./le verify evidence release-candidate` now runs only clean-clone
Docker verification: `Runner.Run` executes `ContainerScript`, whose final command
is `./le verify current mode full`. It does not run the category matrix below.
Native matrix composition, its execution evidence, independent review and closure
remain outstanding. The existing clean-clone action must retain its contract.

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
external infrastructure. Some functional suites also remain outside that default
gate. Restore the complete release-evidence composition through native actions,
with one discoverable matrix entry point and the outcomes specified below.
The original Make target was `ze-evidence-release-verify`; its removal did not
discharge this spec's matrix requirement.

### Inherited item: static-suite scheduling and capabilities

Rehomed here from the `fixit-sleeps-cli-harness` spec at its closure (the spec is gone;
its rows live in the retired deferral shard "ad-hoc-2026-07-27-2c83641a"). It was that spec's A-4
obligation ("wire the linux-gated static suite into an actually-run linux path, do NOT
drop the gate") and was the one live item left in it. The following table records
the unmet state observed on 2026-07-27, before the current native QEMU runner:

| Producer | Fact |
|----------|------|
| `test/static/static-show.ci`, `test/static/static-table-interface.ci` | both carry `option=needs-linux` |
| `internal/test/runner/record_parse.go` | on `GOOS != linux` the record gets a `SkipReason`, so they never run on the darwin dev host |
| `internal/le/functional/suites.go` (`all_suites`) | no `static`, so `./le verify current mode full` never runs it |
| `internal/le/qemu/alltests.go` (`fsuite` lines) | no `static`, so `./le test qemu run command "./le test qemu all-tests"` never runs it -- and that is the only automated Linux functional path (`.github/workflows/qemu-nightly.yml`) |
| `internal/le/functional/suites.go`, `internal/le/evidence/evidence.go` | the suite's only two invocation sites tree-wide, and `ze-evidence-release-verify` is invoked by no workflow |

The inherited requirement was to put `static` on an automated Linux execution path
and declare `caps=net-admin` for the two tests that create interfaces.

Source reconciliation on 2026-09-19: `vmSuites` in
`internal/le/qemu/alltests.go` now includes `static`, serially in the guest-root
namespace. `.github/workflows/qemu-nightly.yml` schedules `le test qemu all-tests`
with the Linux-only selection and skips only `web`. Both named `.ci` files now
declare `option=needs-linux:caps=net-admin`. Those changes satisfy the missing
caller and declaration portions of the inherited item. Execution evidence still
belongs in the capable-host rerun; source membership is no claim that it passed.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` test file format: embedded files, options, expectations and commands
- [x] The deployment-readiness-review record (retired with the learned corpus) - established ./le verify evidence release-candidate
  → Decision: Docker-based clean-clone verify is a permanent gate target
  → Constraint: ZE_SKIP_SUITES mechanism for container-incompatible suites

### Source Files
- [x] `internal/le/functional/suites.go` - shell runner pattern with continue-on-failure
  → Constraint: use same run_suite() pattern for category tracking
- [x] `internal/le/integration/gates.go` - all heavy test targets and ze-deployment-preflight
  → Constraint: preflight checks tools before starting, exits non-zero on missing
- [x] `internal/le/perfbench/actions.go` - `le perf run` and `le perf history-record` targets
  → Decision: le perf track --check already exits non-zero on regression
- [x] `internal/le/testchaos/actions.go` - chaos test targets
  → Constraint: chaos tests run in-process, no external infra needed
- [x] `internal/le/fuzz/actions.go` - fuzz targets with all corpora
  → Constraint: 48 fuzz targets, 10s each, ~8 min total
- [x] `internal/le/` native action tables - verify/all/all-test composition
  → Decision: ./le verify current mode full stays unchanged, new target sits alongside

**Key insights:**
- Shell runner pattern from ./le test functional gives continue-on-failure + summary
- le perf track --check with thresholds already exists, just needs a Make wrapper
- ze-deployment-preflight pattern exists for tooling checks
- Non-gated functional suites need platform-specific tooling (not available on macOS)

## Current Behavior (MANDATORY)

**Current producing-source evidence (2026-09-19):**
- `internal/le/evidence/evidence.go`, `Runner.Run` and `ContainerScript`: clean-clone Docker verification only.
- `internal/le/completeness_record_test.go`, `retiredProducers`: explicitly retires `ze-evidence-release-verify` as a Make loop.
- `internal/le/perfbench/bench.go`, `EvidenceRecord`: native benchmark, history append and regression-check composition.
- `internal/le/qemu/alltests.go`, `vmSuites`: automated Linux static-suite membership, as recorded above.

**Behavior to preserve:**
- `./le verify current mode full` retains its current producer-defined population.
- Existing category actions remain independently runnable.
- `./le verify evidence release-candidate` retains clean-clone Docker verification.
- The native setup probes and perf evidence action retain their existing contracts.

**Behaviour to change:**
- Restore the complete category composition, continue-after-failure behaviour,
  explicit skip accounting and final nonzero failure result through a native
  registered action. Its final command spelling must be settled during design
  and carried consistently into help, the tests and release-distribution.
- Keep Docker mandatory and QEMU absence explicitly reported under AC-1/AC-6.

## Data Flow (MANDATORY)

The native matrix composes existing test runners and produces release evidence.
The following path is required behaviour, not a claim about the clean-clone action.

### Entry Point
- The registered release-matrix action, whose final command spelling remains a design decision.
- No runtime data flow; this is build/test infrastructure

### Transformation Path
1. Preflight checks Docker (mandatory) and QEMU (advisory).
2. The native runner invokes every category's existing registered action in order.
3. It records each category's pass/fail/skip result and continues after failures.
4. It reports the complete population and exits nonzero if any executed category failed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Native matrix -> category actions | Existing registered runners | Not yet demonstrated for the restored composition |

### Integration Points
- The category table records required populations and historical Make locators. Resolve each to its current native action during matrix design; retired targets are not runnable integration points.

### Architectural Verification
- Architectural proof of the restored native composition remains outstanding.
- Category implementations must be reused, with no second protocol or test runner.

## Wiring Test (MANDATORY)

The native matrix requires invocation evidence through its registered command.
The historical Make dry-runs below remain dated evidence of the retired runner.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Registered release-matrix action | → | Native composition over existing category actions | A command-level run records every category, continues after an injected category failure and exits nonzero |
| `./le perf evidence-record` | → | `Bench.EvidenceRecord` | Existing perf evidence tests plus a capable-host regression-check run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Registered release-matrix action preflight | Checks Docker and QEMU, prints ok/missing per tool, exits nonzero if Docker is missing |
| AC-2 | Registered release-matrix action on a machine with Docker | Runs all categories in sequence, prints per-category PASS/FAIL/SKIP and a summary |
| AC-3 | One category fails | Remaining categories still run, summary shows which failed, exit code non-zero |
| AC-4 | `ZE_RELEASE_SKIP=interop,perf` on the matrix action | Named categories are skipped and shown as SKIPPED in the summary |
| AC-5 | `./le perf evidence-record` | Runs the Ze benchmark, appends history and runs `le perf track --check`, exiting nonzero on regression |
| AC-6 | Matrix action with no QEMU | QEMU category is skipped explicitly, others still run |
| AC-7 | Native action help | Shows the matrix entry point, preflight and perf evidence actions with their exact invocation syntax |
| AC-8 | All categories pass | Summary shows all green, exit code 0 |
| AC-9 | Scheduled `.github/workflows/qemu-nightly.yml` Linux run | `le test qemu all-tests` reaches the `static` suite, including `static-show.ci` and `static-table-interface.ci`, and records executed outcomes rather than silently omitting them |
| AC-10 | Static fixtures that create interfaces run on Linux without CAP_NET_ADMIN | Their `needs-linux:caps=net-admin` declarations produce an explicit capability skip; the privileged QEMU run executes them |

## 🧪 TDD Test Plan

The native composition needs command-level failure/skip accounting proof. Existing category tests remain with their producers.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Matrix result accounting | Native matrix owner, resolved during design | AC-2/AC-3/AC-4/AC-6/AC-8: omitted categories, stop-on-first-failure and hidden skips must fail the test | Outstanding |

### Functional Tests

No new `.ci` format is needed. The matrix's command-level tests must prove
category accounting; existing category suites keep their own behaviour tests.

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

## Files to Modify

- `internal/le/evidence/` - native matrix composition beside the preserved clean-clone action, with registered help and command tests
- `docs/functional-tests.md` - exact matrix invocation and its evidence population
- `plan/pre-release/spec-release-distribution.md` - consume the same registered matrix action without treating clean-clone verification as equivalent

## Files to Create

- Exact native source/test filenames remain to be settled during design; no new category runner is authorised.

## Implementation Steps

### Required release-matrix categories

Run in this order (fast/no-infra first, slow/heavy last):

| # | Category name | Action or historical locator | Infra |
|---|--------------|-------------|-------|
| 1 | verify | `./le verify current mode full` | None |
| 2 | chaos | `./le chaos selftest` | None |
| 3 | fuzz | `ze-fuzz-test` | None |
| 4 | interop | `./le test integration interop` | Docker |
| 5 | ipsec-interop | `./le test integration interop-ipsec` | Docker+privileged |
| 6 | l2tp-interop | `./le test deployment docker-l2tp-ppp-test` | Docker |
| 7 | functional-extra | static + traffic + vpp + l2tp-wire | Platform deps |
| 8 | perf | `ze-evidence-perf-record` | Docker |
| 9 | qemu | `ze-qemu-integration-test` | QEMU |
| 10 | vpp-deployment | `./le test deployment vpp-test` | Docker+privileged |
| 11 | live | `ze-live-test` | Docker+internet |

### Phase 1: Native matrix wiring

1. Resolve the exact registered matrix command and the current action for each category above.
2. Implement preflight, continue-after-failure, `ZE_RELEASE_SKIP` accounting and summary through existing native runners.
3. Preserve the independent clean-clone action and use `./le perf evidence-record` for the perf chain.
4. Prove the matrix through its command, including failure and skip cases, and keep AC-9/AC-10's scheduled Linux evidence.

### Phase 2: Documentation and capable-host evidence

1. Publish the exact entry point in native help and `docs/functional-tests.md`, then update release-distribution's caller.
2. Run every category on a capable host, retain the per-category output and finish the independent review.

### Critical Review Checklist

| Check | What to verify |
|-------|---------------|
| Completeness | All 11 categories wired, help entries added |
| Correctness | Each category calls the right existing target |
| Skip logic | ZE_RELEASE_SKIP comma-separated parsing works |
| Preflight | Docker check is mandatory, QEMU is advisory |
| Summary | Matches ./le test functional output style (PASS green, FAIL red, SKIP yellow) |
| Exit code | Non-zero if any category failed (not skipped) |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Native release matrix | Command-level output accounts for every required category |
| Preflight | Docker absence fails, QEMU absence is an explicit skip |
| Perf evidence | `./le perf evidence-record` runs the benchmark/history/check chain |
| Failure accounting | A failed category does not prevent later categories and makes the final result fail |
| Help and distribution caller | Both name the same registered matrix action |

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
- [ ] AC-1..AC-10 all demonstrated against the current native producers
- [ ] Native matrix invocation and failure/skip accounting demonstrated
- [ ] Native entry point, help and release-distribution caller integrated
- [ ] `./le verify worktree` passes (lint + all ze tests)

### Historical Make design evidence
- The original design reused the functional shell-runner pattern and composed existing targets.
- Its passing dry-runs do not prove the native matrix exists or executes.

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Native matrix command-level failure and skip proof recorded; the historical no-Go/Make-only exemption no longer applies

## Verification Evidence (2026-05-24)

| Check | Result | Evidence |
|-------|--------|----------|
| AC-1 preflight success | PASS (historical) | The 2026-05-24 preflight found Docker and `qemu-system-x86_64`, exited 0. This is not evidence for the current clean-clone command |
| AC-1 Docker missing | PASS (historical) | The retired `make ze-evidence-release-preflight` under `PATH="/usr/bin:/bin"` reported Docker missing and exited 1 |
| AC-2 category runner | PARTIAL | `make ZE_RELEASE_SKIP=verify ze-evidence-release-verify` ran the non-skipped matrix and printed per-category PASS/FAIL/SKIP plus summary |
| AC-3 continue after failure | PASS | `make MAKE=false ZE_RELEASE_SKIP=fuzz,interop,ipsec-interop,l2tp-interop,functional-extra,perf,qemu,vpp-deployment,live ze-evidence-release-verify` reported verify and chaos failures, then skipped remaining named categories and exited 1 |
| AC-4 explicit skip | PASS | `make MAKE=true ZE_RELEASE_SKIP=interop,perf ze-evidence-release-verify` reported `SKIPPED: interop perf` and exited 0 |
| AC-5 perf gate | PARTIAL | `make -n ze-evidence-perf-record` shows `ze-perf-bench PERF_DUT=ze`, history append, and a `track --check` of the perf tool (now `le perf track --check`); full run failed because current `cmd/ze/hub` does not build in Docker |
| AC-6 no QEMU skip | PASS | `make MAKE=true ZE_RELEASE_QEMU_BIN=definitely-not-qemu ZE_RELEASE_SKIP=interop,ipsec-interop,l2tp-interop,perf,vpp-deployment,live ze-evidence-release-verify` skipped qemu and exited 0 |
| AC-7 help output | PASS | `make help-test` shows `ze-evidence-release-preflight`, `ze-evidence-release-verify`, and `ze-evidence-perf-record` |
| AC-8 all categories pass | FAIL | Not demonstrated. `make ZE_RELEASE_SKIP=verify ze-evidence-release-verify` failed 7 of 10 attempted categories |
| Required final gate | FAIL | `./le verify current mode full` fails at `./le go lint run` on unrelated `cmd/ze/service` errcheck/modernize/unused issues and `internal/component/web/handler_config_test.go` gofmt |

## Unblock record (2026-07-10)

User instruction 2026-07-10: unblock. The 2026-05-24 blockers were re-verified
against current code (followup-wave impact review):

| 2026-05-24 blocker | Status today (verified firsthand) |
|--------------------|-----------------------------------|
| `wireManagedCommit` undefined breaks `go build ./cmd/ze` | resolved: defined `cmd/ze/hub/managed.go` (takes `audit.Recorder`), called `cmd/ze/hub/main.go` |
| `buildSessionModelFactory` call sites missing `audit.Recorder` | resolved: signature carries `recorder audit.Recorder` at `cmd/ze/hub/session_factory.go` |
| `./le verify current mode full` blocked at ./le go lint run (service/web lint reds) | to be proven by the next full `./le verify current mode full` (./le go lint run is a stage of it); a green run supersedes this row |

Additional post-wave corrections:
- Required Reading cites `internal/le/` native action tables for verify composition; `./le verify current mode full` is now
  at `internal/le/` native action tables and `_ze-verify-impl` carries a longer gate list
  (./le arch tier check, ze-iface-resolution-check, ./le plugin boundary check,
  ./le config ports check, ze-platform-vet, ./le cli grammar, ...).
- The evidence matrix categories predate wave-added heavy suites; the re-run should
  fold in `./le test deployment vpp-iface-test` (`internal/le/integration/gates.go`) and the new
  functional `.ci` (as112-dot/doh, exabgp-bridge-internal, mcp-get-sse,
  test/traffic 020-026) via their existing category targets.

Remaining work: restore the native matrix composition without changing the
clean-clone contract, then run it on a capable host (Docker, QEMU and required
privileges). Record fresh category results and AC-9/AC-10 evidence, complete
independent review, and only then proceed to closure.

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
