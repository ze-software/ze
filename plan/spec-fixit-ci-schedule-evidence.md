# Spec: fixit-ci-schedule-evidence

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-release-evidence-gate |
| Phase | - |
| Updated | 2026-07-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.woodpecker/verify.yml` - the push/pull_request gate (NOT the only pipeline: `perf-nightly.yml` predates this spec and is cron-only)
3. `mk/test-release.mk` - the `ze-release-evidence` matrix this spec schedules
4. `.claude/rules/planning.md` - workflow rules

## Task

**[BLOCKER]** CI runs ONLY `make ze-verify` (`.woodpecker/verify.yml:19`). The entire
Linux-integration / QEMU / fuzz / mutation / interop surface runs in NO automated
pipeline: a grep of `.woodpecker/` + `.github/workflows/` for qemu|integration|fuzz|mutation|interop
returns zero, and the only other workflow is `codeql.yml` (`pages.yml` was deleted on 2026-07-27 -- the gh-pages branch owns the website deploy; see plan/learned/1275-fixit-ci-green.md). Heavy suites
exist as make targets (`mk/test-integration.mk` 29KB, `mk/test-fuzz.mk`, `mk/test-mutation.mk`)
that nothing in CI invokes. `ai/rules/qemu-testing.md:3` calls QEMU tests BLOCKING/mandatory,
yet no gate enforces it. A regression in any heavy suite silently reaches main.

Add a scheduled (nightly cron) Woodpecker pipeline that invokes the evidence matrix -- at
minimum `ze-integration-test` + a representative `ze-qemu-integration-test` + `ze-fuzz-test`
(ideally interop/mutation too), advisory (non-blocking) at first -- converting "silently
reaches main" into next-day detection. It Depends on `spec-release-evidence-gate` for the
`ze-release-evidence` target (already at `mk/test-release.mk:83`); if that spec adds the CI
scheduling itself, this spec folds into it.

## Required Reading

### Source Files
- [ ] `.woodpecker/verify.yml` - the sole pipeline; runs `make ze-verify` on push + pull_request
  → Constraint: a second file in `.woodpecker/` is a separate pipeline; do not alter the fast `verify` gate's trigger or timeout.
  → Decision: use GitHub Actions `on: schedule` (cron lives in the workflow file); merging the workflow to the default branch CREATES the schedule (validation moved off Codeberg Woodpecker, 2026-07-20).
- [ ] `mk/test-release.mk` - `ze-release-evidence` composite (`:83`) and its category runner
  → Constraint: prefer invoking the existing composite / category targets over re-listing suites; registration over hardcoding.
- [ ] `Makefile:279` - `ze-verify`; `_ze-verify-impl` comment warns the live path is `scripts/status/verify_run.go` `stagesForMode()`
  → Constraint: this pipeline invokes heavy `make` targets directly (they are NOT part of the fast-verify stage list), so no `verify_run.go` edit is needed.
- [ ] `ai/rules/qemu-testing.md` - QEMU integration tests are BLOCKING/mandatory
  → Constraint: the scheduled pipeline is what actually enforces this rule; document that linkage.

**Key insights:**
- `spec-release-evidence-gate` is `in-progress` (not `ready`); its `ze-release-evidence` target already exists (`mk/test-release.mk:83`) but nothing schedules it -- this spec supplies the missing scheduler.
- Advisory-first keeps a red heavy suite from blocking main while the matrix is stabilized; flip to blocking once the categories are green on a capable host.
- `ze-integration-test` = `mk/test-integration.mk:106`; `ze-qemu-integration-test` = `:321`; `ze-fuzz-test` = `mk/test-fuzz.mk:12`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/status/verify_run.go` - `stagesForMode()` (:112) is the real stage list `make ze-verify` (hence CI) runs; the heavy integration/QEMU/fuzz/mutation suites are absent from it
- [ ] `scripts/evidence/qemu-run.py` - QEMU VM runner for the integration suite; exists but no pipeline invokes it
- [ ] `test/interop/run.py` - Docker-based BGP interop runner; exists but no pipeline invokes it
- [ ] `.woodpecker/verify.yml` - one `verify` step, `golang:1.26`, runs `make ze-verify` (line 19) on push/pull_request only
- [ ] `.github/workflows/codeql.yml` - CodeQL scan; does not run any ze test suite
- [x] `.github/workflows/pages.yml` - DELETED 2026-07-27; the gh-pages branch's own workflow owns the deploy (plan/learned/1275-fixit-ci-green.md)
- [ ] `mk/test-release.mk` - `ze-release-evidence` (:83) composes the heavy matrix with a continue-on-failure category runner and `ZE_RELEASE_SKIP`
- [ ] `mk/test-integration.mk` - `ze-integration-test` (:106), `ze-qemu-integration-test` (:321), other QEMU/deployment targets
- [ ] `mk/test-fuzz.mk` - `ze-fuzz-test` (:12) runs all fuzz corpora
- [ ] `mk/test-mutation.mk` - mutation-testing targets, unwired to CI

**Behavior to preserve:**
- `.woodpecker/verify.yml` stays exactly as-is: push/pull_request -> `make ze-verify`, fast gate unchanged.
- All existing make targets keep running independently and identically when invoked by hand.
- No change to `scripts/status/verify_run.go` `stagesForMode()`; the fast path is untouched.

**Behavior to change:**
- Add a new nightly-cron Woodpecker pipeline that invokes the evidence matrix (integration + QEMU + fuzz, advisory), so heavy-suite regressions are caught next-day instead of silently reaching main.

## Data Flow (MANDATORY)

### Entry Point
- A Woodpecker `cron` event (nightly, configured on the repo) triggers the new pipeline file in `.woodpecker/`. Real trigger; no runtime data buffers cross component boundaries.

### Transformation Path
1. Cron event fires the new `.woodpecker/<name>.yml` pipeline on the default branch.
2. The step provisions the toolchain (as `verify.yml` does) plus any heavy-suite deps (QEMU, Docker).
3. The step invokes the evidence make target(s). First cut (per the resolved open question): `ze-fuzz-test` + the non-QEMU `ze-integration-test`; ~~`ze-qemu-integration-test`~~ is a follow-up (2026-07-17). Later, once a QEMU-capable runner is confirmed, either add `ze-qemu-integration-test` or switch the step to the `ze-release-evidence` composite with a CI-appropriate `ZE_RELEASE_SKIP` (it self-skips QEMU/Docker categories when those tools are absent).
4. Per-category PASS/FAIL is surfaced; advisory-first means a FAIL is reported but does not block main.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Git event -> CI | GitHub Actions `on: schedule` (cron) schedules the pipeline | [ ] |
| CI -> make | pipeline step shell runs `make ze-<...>-test` (the same targets operators run) | [ ] |

### Integration Points
- `.woodpecker/` (new pipeline file), the existing evidence targets (`mk/test-release.mk`, `mk/test-integration.mk`, `mk/test-fuzz.mk`), and possibly a new `mk/` CI-subset target that names the representative fast-enough evidence categories.

### Architectural Verification
- [ ] No bypassed layers (the pipeline calls the same make targets operators use, not re-inlined suite logic)
- [ ] No duplicated functionality (reuse `ze-release-evidence` / category targets)
- [ ] Registration over hardcoding -- the matrix is expressed as existing make targets, not a hand-copied suite list in YAML (`ai/rules/plugin-self-containment.md`, `ai/rules/discovery-updates.md`)

## Risks & Assumptions

| ID | Assumption / Risk | Basis | If wrong / Mitigation |
|----|-------------------|-------|-----------------------|
| A-1 | The Woodpecker runner has (or can install) QEMU + Docker + privileged mode for the heavy targets | `ai/rules/qemu-testing.md`; `verify.yml` already apt-installs iproute2/nftables | If unavailable, scope the nightly to `ze-fuzz-test` + `ze-integration-test` non-QEMU subset and note QEMU as a follow-up host requirement |
| A-2 | A dedicated cron pipeline file does not disturb the fast `verify` gate | `.woodpecker/` treats each file as an independent pipeline; `verify.yml` untouched | If Woodpecker merges steps unexpectedly, isolate via distinct `when` blocks; verify with a `woodpecker lint` / config dry-run |
| A-3 | `ze-release-evidence` (`mk/test-release.mk:83`) is invocable from CI with a CI-suitable `ZE_RELEASE_SKIP` | target exists; spec-release-evidence-gate is in-progress | If the composite is too heavy for a nightly window, call the three representative category targets directly |
| R-1 | Heavy suites are currently red on some hosts (see release-evidence-gate evidence table) | that spec's 2026-07-10 record shows several FAIL categories | Advisory-first (non-blocking) so a known-red suite does not wedge main; flip to blocking only after a green baseline |
| R-2 | Nightly runtime exceeds the runner's budget | fuzz ~8 min + integration + QEMU | Start with a bounded subset (representative QEMU target only), expand once timing is known |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| scheduled (cron) event | -> | `.github/workflows/evidence-nightly.yml` runs the evidence subset | `scripts/dev/github_workflows_test.go` -- `TestEvidenceNightlyIsScheduled` (scheduled; push/pull_request absent) |
| scheduled event | -> | pipeline runs `ze-fuzz-test` AND `ze-integration-test` (AC-2 MET on GitHub) | `scripts/dev/github_workflows_test.go` -- `TestEvidenceNightlyRunsFuzzAndIntegration` (asserts BOTH present, `ze-qemu-integration-test` absent) |
| pipeline step | -> | invokes existing evidence make targets, not re-inlined suites | `scripts/dev/github_workflows_test.go` -- `TestWorkflowMakeTargetsExist` asserts every `make <target>` resolves to a real rule head |

> Internal / test-infrastructure only: no `.ci` functional test applies (N/A).
>
> **CORRECTION (2026-07-20): `evidence-pipeline-dryrun` DOES NOT EXIST.** Grep
> finds that name only inside this spec -- no script, make target or test bears
> it, and no `woodpecker`/`woodpecker-cli` binary is present, so the claimed
> "Woodpecker config lint" was never run. The REAL verification is
> `scripts/dev/github_workflows_test.go` (7 guards, each proven non-vacuous by
> mutation) plus `make -n` on the invoked targets. (Historical: the guards lived
> in `woodpecker_pipelines_test.go` until validation moved to GitHub Actions.)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | New `.github/workflows/evidence-nightly.yml` present | Declares an `on: schedule` (cron) trigger, push/pull_request absent; `verify.yml` runs push/pull_request -> `make ze-verify` |
| ~~AC-2~~ | ~~Config lint / `make -n` on the pipeline's command~~ | ~~Expands to `ze-integration-test`, `ze-qemu-integration-test`, and `ze-fuzz-test` (or `ze-release-evidence` with CI skip)~~ (superseded 2026-07-17 by the resolved open question: QEMU deferred to follow-up) |
| AC-2 | Config lint / `make -n` on the first pipeline's command | Expands to `ze-fuzz-test` and the non-QEMU `ze-integration-test` (target names, not copied recipes); `ze-qemu-integration-test` is NOT invoked in the first cut |
| AC-6 | A QEMU-capable runner is later confirmed to exist | Follow-up adds `ze-qemu-integration-test` (or switches the step to the `ze-release-evidence` composite, which self-skips QEMU/Docker when absent); until then AC-2's scope stands |
| AC-3 | One heavy category fails | Pipeline reports the failing category; advisory-first means main is not blocked; result is visible next-day |
| AC-4 | Fast gate | `.github/workflows/verify.yml` runs `make ze-verify` on push/pull_request with no added latency (no heavy suite) |
| AC-5 | Discovery | `ai/rules/qemu-testing.md` (and evidence docs) reference the scheduled pipeline as the enforcing gate |


## ✅ AC-2 RESOLVED -- validation moved to GitHub Actions (2026-07-20)

Thomas directed: "change the validation to run on github and not codeberg." That
supplies the explicit approval AC-2 was blocked on, and picks resolution (c) from
the choices below: a runner that grants the capabilities the integration suite
needs. The pipeline set was ported from `.woodpecker/` to `.github/workflows/`,
and `ze-integration-test` REJOINS the nightly. **AC-2 is met.**

Why GitHub unblocks it: the Woodpecker impasse was `privileged: true` -- a BLOCKING
linter error on the shared codeberg.org instance unless a server admin marks the
repo trusted, and the lint runs before the `when:` match so it discards EVERY
workflow, breaking `verify.yml` on every push. GitHub's `ubuntu-latest` runner
has no such gate: the job runs under `sudo` as root, which holds `CAP_NET_ADMIN`
and `CAP_NET_BIND_SERVICE` natively. So the step runs WITH the capabilities, not
without them -- which also disposes of the second hazard below.

The vacuous-green hazard is likewise gone: those six suites `t.Skipf` only when
capabilities are ABSENT (`internal/component/iface/integration_helpers_linux_test.go`,
`internal/plugins/as112/integration_linux_test.go`,
`.../freebind_integration_linux_test.go`). Under `sudo` on GitHub the capabilities
are present, so the suites actually run instead of skipping. The step is
advisory-first (`continue-on-error: true`) until a green baseline lets it flip to
blocking.

Superseded record (kept for history): before this move the implementation shipped
FUZZ-ONLY, and the earlier justification for that cut was circular (it leaned on
an implementer-authored `→ AUTONOMOUS DEFAULT` note, not user approval). Thomas's
instruction resolves it. The shape guard is now
`scripts/dev/github_workflows_test.go` (`TestEvidenceNightlyRunsFuzzAndIntegration`
asserts BOTH `ze-fuzz-test` and `ze-integration-test` are present, and that
`ze-qemu-integration-test` is NOT -- it still needs nested virt / KVM, AC-6).

## 🧪 TDD Test Plan

### Unit Tests

→ CORRECTION (2026-07-20): the skeleton said "N/A -- CI config, no Go code", and
that was wrong. A pipeline nobody can test is exactly how `.woodpecker/` rots: it
runs at night, and `make: *** No rule to make target` reads as an infrastructure
blip. `.ci` does not apply, but Go tests over the YAML do, and they are cheap.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifyWorkflowIsTheFastMergeGate` | `scripts/dev/github_workflows_test.go` | AC-4 (push + pull_request -> `make ze-verify`; no heavy/scheduled work) | GREEN |
| `TestEvidenceNightlyIsScheduled` | same | AC-1 (scheduled; push/pull_request ABSENT) | GREEN |
| `TestEvidenceNightlyRunsFuzzAndIntegration` | same | AC-2 (asserts BOTH `ze-fuzz-test` and `ze-integration-test` PRESENT; `ze-qemu-integration-test` ABSENT, AC-6) | GREEN |
| `TestEvidenceNightlyIsAdvisory` | same | AC-3 (every job carries `continue-on-error: true`) | GREEN |
| `TestPerfNightlyIsScheduled` | same | perf-nightly is scheduled-only, never a merge gate | GREEN |
| `TestWorkflowMakeTargetsExist` | same | every `make <target>` any workflow names resolves to a real rule head | GREEN |
| `TestValidationIsNotOnWoodpecker` | same | no `.woodpecker` pipeline remains (validation off Codeberg); non-vacuity proven by re-adding one | GREEN |

→ Constraint: string-based, not YAML-parsed, on purpose. `gopkg.in/yaml.v3` is an
INDIRECT dependency; importing it would promote it to direct and churn
`go.mod`/`go.sum` -- a shared file, and one
`plan/spec-fixit-supply-chain-hardening.md` is specifically about. `#` comments
are stripped before matching, so a commented-out command cannot satisfy a check.

### AC evidence (2026-07-20)

| AC | Evidence |
|----|----------|
| AC-1 | `.github/workflows/evidence-nightly.yml` `on: schedule` (cron). `TestEvidenceNightlyIsScheduled` also asserts `push`/`pull_request` are ABSENT from the `on:` block. Non-vacuity proven: adding a `push`/`pull_request` trigger fails it. |
| AC-2 | **MET (on GitHub) -- see the AC-2 RESOLVED banner above.** `evidence-nightly.yml` runs `make ze-fuzz-test` and `make ze-integration-test` by target NAME (not copied recipes), the integration job under `sudo` (root) so the six kernel suites (iface, fib, firewall, traffic, gtsm, as112) get CAP_NET_ADMIN / CAP_NET_BIND_SERVICE and actually run rather than skip. `TestEvidenceNightlyRunsFuzzAndIntegration` asserts both are present and `ze-qemu-integration-test` is absent; `TestWorkflowMakeTargetsExist` proves every `make <target>` in EVERY workflow resolves to a real rule head. |
| AC-3 | Both jobs (fuzz, integration) carry `continue-on-error: true`. `TestEvidenceNightlyIsAdvisory` iterates the jobs and fails if any job lacks it, so a blocking job fails it. Non-vacuity proven by removing one. |
| AC-4 | `.github/workflows/verify.yml` runs `make ze-verify` on push + pull_request only. `TestVerifyWorkflowIsTheFastMergeGate` pins push + pull_request + `make ze-verify`, that it never becomes schedule-triggered, and that no heavy suite (fuzz/integration/qemu/mutation/release-evidence) appears in it. |
| AC-5 | `ai/rules/qemu-testing.md` gained "What actually RUNS these suites": a suite -> pipeline -> blocking? table, the advisory-first rationale, and the explicit statement that `ze-qemu-integration-test` is STILL enforced by review alone. |
| AC-6 | Held open deliberately. `ze-qemu-integration-test` is absent AND `TestEvidenceNightlyRunsFuzzAndIntegration` FAILS if it is added, so the follow-up cannot land without also updating the guard. It stays out because GitHub-hosted runners do not reliably provide the nested virt / KVM it needs. |

### Citation corrections (2026-07-20)

- The Task section says "no cron config exists in the repo today". **STALE.**
  `.woodpecker/perf-nightly.yml` already exists and is `when: event: cron`
  (spec-fixit-perf-alloc-ci-gate AC-5). It was used as the template here, which
  is why this pipeline matches its shape. The audit's broader point stands: no
  cron job ran any INTEGRATION / QEMU / FUZZ suite.
- A-1 is now CONFIRMED, not assumed, by reading the producer
  (`mk/test-integration.mk:82-106`): the recipes state `CAP_NET_ADMIN` for iface
  (:83), fib (:87), firewall/nft (:91) and traffic/netlink (:95), and
  `CAP_NET_BIND_SERVICE/root` for as112 (:103). `ze-integration-test` (:106) is
  the aggregate of those six. The integration step is therefore
  `privileged: true` ON WOODPECKER. **UPDATE (2026-07-20, GitHub migration):** moot on GitHub -- the integration job runs under `sudo` as root, so `CAP_NET_ADMIN` / `CAP_NET_BIND_SERVICE` are present and the six suites RUN rather than `t.Skipf`. The step is PRESENT, not absent; the vacuous-green risk applied only to an UNPRIVILEGED run, which does not occur here.

### Functional Tests
Test infrastructure only; no user-facing features. The new pipeline invokes existing `.ci`-backed
suites (integration/QEMU/fuzz) that already carry their own functional coverage; no new `.ci` test
is added. **CORRECTION (2026-07-20): `evidence-pipeline-dryrun` DOES NOT EXIST.** Grep finds that name only inside this spec -- no script, make target or test bears it, and no `woodpecker`/`woodpecker-cli` binary is present, so the claimed "Woodpecker config lint" was never run. The REAL verification is `scripts/dev/github_workflows_test.go` (7 guards, each proven non-vacuous by mutation) plus `make -n` on the invoked targets. Cite that, not a name that resolves to nothing.

## Files to Modify
- `.github/workflows/evidence-nightly.yml` - NEW nightly-schedule workflow invoking the evidence matrix (primary feature file); Woodpecker removed
- `mk/test-release.mk` (or a new `mk/` CI-subset target) - only if a CI-tailored evidence subset target is needed
- `ai/rules/qemu-testing.md` - note the scheduled pipeline as the enforcing gate (discovery update)

## Implementation Steps

1. **Wiring (MANDATORY FIRST)** -- add `.github/workflows/evidence-nightly.yml` with an `on: schedule` trigger and jobs that install deps and run the representative evidence targets. Confirm via `scripts/dev/github_workflows_test.go` + `make -n` that the target names expand. Resolve A-1/A-3.
2. **Advisory-first** -- mark the heavy step non-blocking so a known-red suite (R-1) does not wedge main; record the intent to flip to blocking after a green baseline.
3. **Scope to runner budget** -- the GitHub nightly runs `ze-fuzz-test` (fuzz job) + the non-QEMU `ze-integration-test` (integration job, under `sudo` root), both advisory (`continue-on-error: true`). `ze-qemu-integration-test`, then interop/mutation, remain follow-ups once a KVM-capable runner is confirmed and timing is known (R-2, A-1, AC-6).
4. **Discovery update** -- reference the scheduled pipeline from `ai/rules/qemu-testing.md` and the evidence docs so future agents find the enforcing gate (`ai/rules/discovery-updates.md`).
5. **Verify** -- `make ze-verify` (fast gate unaffected) plus a config lint / dry-run of the new pipeline; if `spec-release-evidence-gate` lands CI scheduling first, fold this in instead of duplicating.
6. **Complete spec** -- audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete -- each row has a concrete verification name
- [ ] Fast `verify.yml` gate unchanged (push/pull_request -> `make ze-verify`)
- [ ] Registration over hardcoding respected (pipeline calls make targets, not copied suite lists)
- [ ] Discovery update done (`ai/rules/qemu-testing.md` references the scheduled gate)
- [ ] `make ze-test` passes (lint + all ze tests)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Coordinated with `spec-release-evidence-gate` (no duplicate scheduler)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] make ze-test

## Notes
- Skeleton captured from the 2026-07-16 repository audit. Depends on `spec-release-evidence-gate` for the `ze-release-evidence` target.
- Citation drift corrected vs the audit note: the depended-on spec is `in-progress` (not `ready`), and its `ze-release-evidence` target already EXISTS at `mk/test-release.mk:83`; what is missing is the scheduler, which this spec supplies.
- Open question for Thomas: does the Woodpecker runner have QEMU + privileged Docker for a nightly heavy run, or should the first scheduled pipeline be limited to `ze-fuzz-test` + the non-QEMU `ze-integration-test` subset until a capable runner exists?
  → AUTONOMOUS DEFAULT (2026-07-17): Limit the FIRST scheduled pipeline to `ze-fuzz-test` (pure Go fuzz, no privileges) + the non-QEMU `ze-integration-test` subset (`mk/test-integration.mk:106`), advisory / non-blocking. Do NOT include `ze-qemu-integration-test` (`mk/test-integration.mk:321`) in the first cut. Add the heavy QEMU run as a FOLLOW-UP gated on a QEMU-capable runner being confirmed to exist -- either `ze-qemu-integration-test` directly, or switch the step to the full `ze-release-evidence` composite (`mk/test-release.mk:83`), which already self-skips QEMU/Docker/interop categories via its `has_qemu`/`has_docker` probes (`run_if_qemu`/`run_if_docker`) and `ZE_RELEASE_SKIP`. Rationale: the repo documents no runner capability today (zero cron and zero QEMU config anywhere under `.woodpecker/`); the conservative scope guarantees the pipeline can actually run and start catching heavy-suite regressions next-day immediately, while QEMU (nested virt / KVM) is the single hardest, least-portable requirement and therefore the right thing to defer. GROUNDING NOTE: even the non-QEMU `ze-integration-test` subset requires CAP_NET_ADMIN / CAP_NET_BIND_SERVICE (`mk/test-integration.mk:87-105` recipe comments), so the pipeline step must run with those capabilities granted (privileged container or `cap_add`; this is what A-1 tracks); if the runner cannot grant them either, fall back to `ze-fuzz-test` alone for the first cut. Thomas: override if the runner already has QEMU + privileged Docker and you want the full heavy matrix on night one.
