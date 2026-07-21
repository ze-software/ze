# Spec: fixit-supply-chain-hardening

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `scripts/status/verify_run.go` - `stagesForMode` (:112), the live verify gate
4. `.github/workflows/codeql.yml`, `scripts/dev/reapply-updater-fixes.py`, `ai/rules/appliance-dep-bumps.md`

## Task

**[HIGH]** Ze has **no dependency-vulnerability scanning gate**. `govulncheck` appears in the
tree only inside a vendored third-party Makefile (`vendor/github.com/prometheus/procfs/Makefile.common`);
`golang.org/x/vuln` is not even in `go.sum`. The verify gate (`scripts/status/verify_run.go`
`stagesForMode`, driven by `make ze-verify` at `Makefile:279-280`, run in CI by `.woodpecker/verify.yml:19`)
has no SCA stage. CodeQL (`.github/workflows/codeql.yml`) runs the **default** query pack (first-party
taint analysis, not software-composition/CVE scanning) AND its manual Go build is a bare `go build ./...`
(`codeql.yml:92-95`) with **no `-tags`**, so the large feature-gated surface behind `//go:build ze_core` /
`ze_appliance` (most of `cmd/ze` and the appliance) is never compiled and therefore never analyzed.
Detection today rests entirely on advisory-only Dependabot (`.github/dependabot.yml`). Folded MEDIUM/LOW
findings (same root cause: no automated supply-chain gate):
- The `gokrazy/updater` hard-fork's security hardening (two `io.LimitReader(resp.Body, 1<<20)` DoS caps +
  `defer resp.Body.Close()`) is re-applied to the vendored copy by a **manual** script after every
  `go mod vendor` (`scripts/dev/reapply-updater-fixes.py`); **no gate verifies the fix markers persist**,
  and the upstream PR (`scripts/dev/gokrazy-updater-upstream.patch`) is unmerged.
- Six direct deps are pinned to pseudo-versions, not tags (`go.mod:12,15,19,30,38,39`).
- The appliance builddir tree is hand-pinned and excluded from Dependabot (`dependabot.yml:8-11,14-15`),
  bumped only reactively via `ai/rules/appliance-dep-bumps.md`.
- LOW (flag, do not adjudicate): the appliance ships a GPLv2 Linux kernel (`rtr7/kernel`,
  `gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod:5`); a source-offer compliance sign-off is unrecorded.

This is the **absence of a scanner**, not a known CVE. Do not invent CVEs. Wire the missing gates.

## Required Reading

### Architecture Docs
- [ ] `scripts/status/verify_run.go` - `stagesForMode(mode, makeCmd)` (:112-158): the live stage list; each stage is a `make <target>` (`mk` helper :113)
  → Constraint: ~~a new SCA stage is added by inserting one `mk("ze-vulncheck")` line in BOTH the `ze-verify-changed` and default branches;~~ (SUPERSEDED 2026-07-17 by the SCHEDULED default: `stagesForMode` is NOT modified for govulncheck; this reading remains the reference for HOW an inline stage WOULD be added, useful if Thomas overrides to inline.) the Makefile comment (`Makefile:282-289`) warns stages live here, not in a Makefile `.PHONY` chain.
  → Decision: govulncheck is a scheduled/full-verify concern (network fetch of the vuln DB); ~~place it in the default branch, and consider gating it behind availability rather than blocking `ze-verify-changed`.~~ (SUPERSEDED 2026-07-17) run it as a scheduled CI job (`.github/workflows/govulncheck.yml`), NOT in `stagesForMode` at all, so neither `ze-verify` nor `ze-verify-changed` is blocked. See the AUTONOMOUS DEFAULT resolution in Notes.
- [ ] `.github/workflows/codeql.yml` - manual Go build (:92-95) and default query pack
  → Constraint: replace `go build ./...` with a build across the shipped tag set (`-tags 'ze_core ze_distro ze_appliance ...'`) so feature-gated code is analyzed; enabling `security-extended` (:84 comment) is the SCA-adjacent query knob.
- [ ] `ai/rules/appliance-dep-bumps.md` - why builddir modules are Dependabot-excluded and bumped by runbook
  → Constraint: do NOT bring builddir modules under Dependabot (a PR would fight the pin); add a **proactive review** cadence instead, not automated bumps.

**Key insights:**
- The verify gate is a registry of stages (`stagesForMode`), not hardcoded shell. ~~the SCA stage registers there, honoring registration-over-hardcoding.~~ (SUPERSEDED 2026-07-17: the SCA scan runs as a scheduled workflow, top-level CI config like `codeql.yml`, NOT a `stagesForMode` entry; this is the deliberate placement, not a hardcoding violation.)
- `reapply-updater-fixes.py` is idempotent and self-documents its four fix markers; a marker-assertion test is the cheap durable guard, upstreaming the PR is the durable fix.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/status/verify_run.go` - `stagesForMode` (:112) has no SCA stage; stages: lint, tier, rfc, iface, plugin-boundary, config-coercion, fs-persistence, port-defaults, platform-vet, wiring-docs, hook, unit, functional, exabgp
  → Constraint: no `govulncheck` / vuln stage in either branch.
- [ ] `.github/workflows/codeql.yml` - `go build ./...` (:95), default pack, `build-mode: manual` for go (:47)
  → Constraint: bare build with no `-tags` excludes `ze_core`/`ze_appliance` code from the CodeQL database.
- [ ] `scripts/dev/reapply-updater-fixes.py` - re-applies 4 fixes to `vendor/github.com/gokrazy/updater/updater.go`; markers: `io.LimitReader(resp.Body, 1<<20)` (x2), `defer resp.Body.Close()`, `http.NoBody`
  → Constraint: run manually after `go mod vendor`; nothing asserts the markers survive a re-vendor.
- [ ] `go.mod` - pseudo-version pins at :12 (charmbracelet/ssh), :15 (insomniacslk/dhcp), :19 (packetcap/go-pcap), :30 (wgctrl), :38 (gokrazy/tools), :39 (gokrazy/updater); no `golang.org/x/vuln`
  → Constraint: pseudo-versions are legal but bypass tagged-release review; move to tags only where upstream publishes them.
- [ ] `.github/dependabot.yml` - root gomod + github-actions only; builddir excluded by design (:8-11)
- [ ] `internal/appliance/updater/body_leak_test.go`, `updater_test.go` - existing updater package tests (home for a fix-marker assertion)

**Behavior to preserve:**
- The existing verify stages, their ordering, and `ze-verify` semantics; the appliance builddir pinning model (do not automate it).
- The four updater hardening fixes and the manual re-vendor workflow until the upstream PR merges.

**Behavior to change:**
- Add an SCA (govulncheck) stage/CI job; build CodeQL with the shipped tag set; add a fix-marker assertion; move pins to tags where possible; schedule a builddir-pin review; record the GPLv2 source-offer sign-off need.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **SCA gate:** ~~`make ze-verify` -> `scripts/status/verify_run.go` -> `stagesForMode` produces the stage list -> a new `ze-vulncheck` stage runs `govulncheck ./...` (the real full-verify path CI invokes at `.woodpecker/verify.yml:19`).~~ (SUPERSEDED 2026-07-17 by the SCHEDULED default) A scheduled CI event (`schedule: cron` in `.github/workflows/govulncheck.yml`) -> `make ze-vulncheck` -> `govulncheck ./...` scans the module graph against the vuln DB. This runs OUTSIDE the inline `make ze-verify` / `.woodpecker/verify.yml:19` push path so a transient advisory does not block commits; `make ze-vulncheck` remains runnable on demand.
- **CodeQL:** `push`/`pull_request`/`schedule` -> `codeql.yml` manual build compiles the module; today `./...` with no tags, changed to the shipped tag set.
- **Updater fix-marker:** `go mod vendor` rewrites `vendor/.../updater/updater.go`; a package test reads that file and asserts the four markers.

### Transformation Path
1. Verify run assembles stages via `stagesForMode`; each stage shells to `make <name>` and its exit code gates the run.
2. A new `ze-vulncheck` target `go install`s/ runs `govulncheck ./...`, scanning the module's dependency graph against the vuln DB; non-zero exit fails ~~the gate~~ the scheduled CI job (`.github/workflows/govulncheck.yml`), NOT the inline `ze-verify` run (SCHEDULED default, 2026-07-17).
3. CodeQL's manual build compiles the tagged surface, feeding the feature-gated packages into the analysis database.
4. The fix-marker test parses the vendored updater file and asserts the LimitReader/Body.Close/NoBody markers are present.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ~~CI ↔ verify gate~~ Scheduled CI ↔ SCA scan (2026-07-17) | ~~`make ze-verify` runs `stagesForMode`; new SCA stage registers there~~ scheduled `.github/workflows/govulncheck.yml` runs `make ze-vulncheck`; NOT wired into `stagesForMode`/inline `ze-verify` | [ ] |
| Dependency graph ↔ vuln DB | `govulncheck ./...` fetches advisories and reports reachable vulns | [ ] |
| Re-vendor ↔ hardening | package test asserts the 4 updater fix markers survive `go mod vendor` | [ ] |
| Source tags ↔ pins | `go.mod` pseudo-versions replaced by tagged releases where published | [ ] |

### Integration Points
- `scripts/status/verify_run.go` (`stagesForMode`), `Makefile` (new `ze-vulncheck` target), `.github/workflows/codeql.yml` (tag set + query pack), the updater package test, `go.mod` (pins), `ai/rules/appliance-dep-bumps.md` (builddir review cadence).

### Architectural Verification
- [ ] No bypassed layers (~~SCA stage runs through `stagesForMode`, the same registry every other stage uses; not an ad-hoc shell step~~ SUPERSEDED 2026-07-17: the scheduled workflow calls `make ze-vulncheck`, a real make target, not an inline shell blob duplicated in YAML)
- [ ] No duplicated functionality (reuse the `mk(name)` stage helper where relevant and the existing updater test package; the scheduled workflow calls the single `make ze-vulncheck` target rather than re-spelling `govulncheck ./...`)
- [ ] Registration over hardcoding — ~~the SCA stage is registered in `stagesForMode`, discovered by the gate, not spelled inline in a Makefile chain~~ (SUPERSEDED 2026-07-17) the SCA scan is a top-level scheduled CI workflow (`.github/workflows/govulncheck.yml`), the same class of config as `codeql.yml`; it is deliberately NOT a `stagesForMode` entry (so the inline dev loop stays unblocked) and adds no per-feature switch to a core/shared package (`ai/rules/discovery-updates.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `govulncheck ./...` stage is wireable as one `mk("ze-vulncheck")` entry in `stagesForMode` | each stage is `make <name>` via `mk` (:113-119) | needs a bespoke stage struct or CI-only job | add the stage, run `make ze-verify` | unvalidated (resolve during implement) |
| A-2 | The four updater fix markers are stable literals a test can assert on | markers in `reapply-updater-fixes.py` (:121-140) + `gokrazy-updater-upstream.patch` | upstream merges the PR first, deleting the fork (then AC-3 becomes "delete script + pin fixed tag") | grep the vendored file for markers | confirmed by reading the script + patch |
| A-3 | Some of the six pseudo-version deps have upstream tags to move to | `go.mod:12,15,19,30,38,39` are all `v0.0.0-<date>-<hash>` | those deps publish no tags; document and keep the pseudo-version | check the proxy/`go list -m -versions` per dep | unvalidated (per-dep, resolve during implement) |
| A-4 | govulncheck adds bounded runtime and needs network access to the vuln DB | govulncheck fetches `vuln.go.dev` | offline/air-gapped CI can't run it inline; make it scheduled or availability-gated | time a run; check CI network policy | unvalidated (resolve during implement) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | govulncheck reports a live reachable vuln in a current dep | first run exits non-zero with a GO-XXXX advisory | Treat as a real finding: bump the dep (root) or follow `appliance-dep-bumps.md` (builddir); this is the gate working, not a spec failure |
| R-2 | CodeQL tag-set build increases analysis time / breaks the manual build | codeql job slower or red | Pin the minimal shipped tag set that covers the appliance surface; keep `security-extended` optional |
| R-3 | An SCA stage blocking `ze-verify-changed` slows every commit | contributor friction on the fast path | Place govulncheck in the default (full) branch or a scheduled CI job only, not `ze-verify-changed` |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| ~~`stagesForMode("ze-verify", ...)`~~ | ~~->~~ | ~~list contains a `ze-vulncheck` stage~~ | ~~`TestStagesForModeIncludesVulncheck`~~ |
| `.github/workflows/govulncheck.yml` scheduled job (SCHEDULED default, 2026-07-17) | -> | scheduled workflow invokes `make ze-vulncheck` (`govulncheck ./...`); no `stagesForMode` entry | `TestGovulncheckScheduledWorkflow` (workflow-string assertion) |
| `go mod vendor` re-vendors updater | -> | vendored file retains the 4 hardening markers | `TestUpdaterHardeningMarkersPresent` |
| `codeql.yml` manual build | -> | build command carries the shipped `-tags` set | `TestCodeQLBuildUsesShippedTags` (workflow-string assertion) |

~~Concrete test: `TestStagesForModeIncludesVulncheck` in `scripts/status/verify_run_test.go` asserts the default-mode
slice returned by `stagesForMode` contains a stage named `ze-vulncheck`;~~ (superseded 2026-07-17 by the SCHEDULED
default) Concrete test: `TestGovulncheckScheduledWorkflow` in `scripts/status/verify_run_test.go` (or a workflow-lint
test) reads `.github/workflows/govulncheck.yml` and asserts it declares a `schedule:` trigger and invokes the
govulncheck run (`make ze-vulncheck` / `govulncheck ./...`), and that no `stagesForMode` branch contains a
`ze-vulncheck` stage (proving the inline dev loop is not blocked); `TestUpdaterHardeningMarkersPresent` in
`internal/appliance/updater/` reads `vendor/github.com/gokrazy/updater/updater.go` and asserts all four marker
literals are present (fails the unit gate if a re-vendor drops them).

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ~~`make ze-verify` (or the scheduled CI job)~~ the SCHEDULED CI job (`.github/workflows/govulncheck.yml`, 2026-07-17 default) | runs `govulncheck ./...` (via `make ze-vulncheck`); a seeded known-vuln fixture makes it exit non-zero; ~~`stagesForMode` lists `ze-vulncheck`~~ `stagesForMode` does NOT list `ze-vulncheck` (inline `ze-verify` stays unblocked) and the scheduled workflow declares a `schedule:` trigger |
| AC-2 | `codeql.yml` manual Go build | compiles the shipped tag set (`ze_core`/`ze_distro`/`ze_appliance` ...), so feature-gated `cmd/ze`/appliance code enters the CodeQL database |
| AC-3 | `go mod vendor` drops a hardening marker | `TestUpdaterHardeningMarkersPresent` fails; with all four markers present it passes (or: upstream PR merged, fork+script deleted, dep pinned to the fixed tag) |
| AC-4 | The six pseudo-version pins | each is moved to a tagged release where upstream publishes one; each remaining pseudo-version is documented with the reason (no tag) |
| AC-5 | Appliance builddir pins | a proactive review cadence is recorded in `ai/rules/appliance-dep-bumps.md` (not automated Dependabot) |
| AC-6 | GPLv2 `rtr7/kernel` in the shipped image | the need for a source-offer compliance sign-off is recorded (flag only; no legal adjudication in this spec) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| ~~`TestStagesForModeIncludesVulncheck`~~ `TestGovulncheckScheduledWorkflow` (SCHEDULED default, 2026-07-17) | `scripts/status/verify_run_test.go` (or workflow lint) | AC-1 | |
| `TestUpdaterHardeningMarkersPresent` + `TestUpdaterHardeningMarkersDetectRegression` | `internal/appliance/updater_hardening_markers_test.go` (DONE 2026-07-19) | AC-3 | PASS |
| `TestCodeQLBuildUsesShippedTags` | `scripts/status/verify_run_test.go` (or workflow lint) | AC-2 | |

### Functional Tests
Test infrastructure / CI-gate only; no user-facing runtime feature. The SCA gate is exercised by
~~`make ze-verify` running the new `ze-vulncheck` stage~~ the scheduled `.github/workflows/govulncheck.yml`
job running `make ze-vulncheck` (SCHEDULED default, 2026-07-17), with `make ze-vulncheck` also runnable on
demand; CodeQL and Dependabot run in GitHub CI. No `.ci`
functional test applies. Opt-out justification: **test infrastructure only** (supply-chain gates, not product behavior).

## Files to Modify
- ~~`scripts/status/verify_run.go` - register the `ze-vulncheck` stage in `stagesForMode` (non-test feature file)~~
  (SUPERSEDED 2026-07-17: SCHEDULED default means govulncheck is NOT a `stagesForMode` stage; no change to `stagesForMode`. `scripts/status/verify_run_test.go` still gains `TestGovulncheckScheduledWorkflow`.)
- `Makefile` - add the `ze-vulncheck` target running `govulncheck ./...` (on-demand target; called by the scheduled workflow, NOT by `ze-verify`)
- `.github/workflows/codeql.yml` - build with the shipped tag set; optionally enable `security-extended`
- `go.mod` - move pseudo-version pins to tags where upstream publishes them
- `ai/rules/appliance-dep-bumps.md` - record the proactive builddir-pin review cadence and GPLv2 source-offer sign-off note
- `scripts/dev/reapply-updater-fixes.py` - only if upstreaming the PR lets us delete the fork + script

## Files to Create
- ~~`internal/appliance/updater/hardening_markers_test.go`~~ `internal/appliance/updater_hardening_markers_test.go`
  (CORRECTED 2026-07-19: the `internal/appliance/updater/` subpackage does NOT exist in the tree; the vendored
  updater is consumed via `internal/appliance/cmd_push.go`, so the guard lives in package `appliance`.
  It gates the 3 DoS-hardening markers -- `io.LimitReader(resp.Body, 1<<20)` x2, `http.NoBody`, `defer resp.Body.Close()`;
  the 4th script rewrite, `slices.Contains`, is a cosmetic refactor and is deliberately NOT gated. Also found the
  vendored copy had already regressed (lost LimitReader + NoBody); restored via `reapply-updater-fixes.py`, and fixed
  three latent bugs in that script: dropped trailing newline, wrong `slices` import position, and a missing final gofmt
  normalization pass with a fail-loud anchor check.)
- `.github/workflows/govulncheck.yml` - NEW scheduled (`schedule: cron`) CI workflow that runs `make ze-vulncheck` (`govulncheck ./...`); mirrors the `codeql.yml` cron precedent, does NOT run on push/PR blocking the dev loop (SCHEDULED default, 2026-07-17)

## Implementation Steps

### Implementation Phases
1. **Phase: SCA gate (MANDATORY FIRST)** — add the `ze-vulncheck` Makefile target (`govulncheck ./...`), ~~register it in `stagesForMode` (default branch),~~ (SUPERSEDED 2026-07-17: SCHEDULED default) create `.github/workflows/govulncheck.yml` (`schedule: cron`) that runs `make ze-vulncheck`, add `golang.org/x/vuln` as a tool dep; confirm the scheduled workflow runs govulncheck and that `stagesForMode` is unchanged (inline `ze-verify` not blocked).
   - Tests: ~~`TestStagesForModeIncludesVulncheck`~~ `TestGovulncheckScheduledWorkflow`
   - Files: `Makefile`, `.github/workflows/govulncheck.yml`, `scripts/status/verify_run_test.go`, `go.mod`
2. **Phase: CodeQL tag set** — replace `go build ./...` with the shipped `-tags` build so feature-gated code is analyzed; assert the tag string in the workflow.
   - Tests: `TestCodeQLBuildUsesShippedTags`
3. **Phase: updater fix-marker guard** — add `TestUpdaterHardeningMarkersPresent`; ~~decide upstream-PR-merge-and-delete vs keep-fork-with-guard.~~ RESOLVED 2026-07-17: keep-fork-with-guard (self-contained); upstream merge stays a tracked follow-up (see Notes AUTONOMOUS DEFAULT #2).
4. **Phase: pin hygiene** — per-dep, check for a tag and move pins; document any that must stay pseudo-versioned; record the builddir review cadence + GPLv2 sign-off note.
5. **Full verification** — `make ze-test`, then `make ze-verify` ~~(exercises the new SCA stage)~~ (SCHEDULED default: `ze-verify` does NOT run govulncheck; separately run `make ze-vulncheck` on demand and confirm the scheduled `.github/workflows/govulncheck.yml` invokes it).
6. **Complete spec** — audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

### Discovery-Update Obligation (`ai/rules/discovery-updates.md`)
- ~~Source of truth: `stagesForMode` in `scripts/status/verify_run.go` — the verify gate registry (registration over hardcoding).~~ (SUPERSEDED 2026-07-17) Source of truth for the SCA scan: the scheduled workflow `.github/workflows/govulncheck.yml` (top-level CI config, sibling of `codeql.yml`); `stagesForMode` stays the registry for the inline gate but gains no govulncheck entry.
- New make target `ze-vulncheck` documented where verify targets are listed (on-demand target, not an inline `ze-verify` stage); `ai/rules/appliance-dep-bumps.md` gains the review cadence.
- No new runtime dependency ships in the binary (`golang.org/x/vuln` is a build/CI tool), so no `ai/INDEX.md` component entry.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] ~~SCA stage runs under `make ze-verify`~~ (SUPERSEDED 2026-07-17) govulncheck runs in the scheduled CI job (`.github/workflows/govulncheck.yml`), NOT under inline `make ze-verify`; `stagesForMode` unchanged
- [ ] Registration over hardcoding respected (~~SCA stage registered in `stagesForMode`~~ the scheduled workflow is top-level CI config like `codeql.yml`, not a per-feature switch in a core/shared package; no `stagesForMode` change needed. SCHEDULED default, 2026-07-17)
- [ ] Discovery update done (`ai/rules/appliance-dep-bumps.md`, verify-target docs)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary cases (missing marker, ~~missing stage~~ missing/mis-triggered scheduled workflow, untagged build) present

## Pre-Commit Verification

| Item | Verified | Evidence |
|------|----------|----------|
| AC-1 target + scheduled workflow | yes | `make -n ze-vulncheck` expands to `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`; `go run ...@latest -version` prints Go/Scanner v1.6.0/DB (works despite `vendor/`); `govulncheck.yml` has `schedule:` cron + `workflow_dispatch`, no push/PR, runs `make ze-vulncheck` |
| AC-1 test non-vacuous | yes | `TestGovulncheckScheduledWorkflow` PASS; asserts schedule trigger, `make ze-vulncheck`, no push/pull_request, no `stagesForMode` vulncheck entry |
| AC-2 tag combos compile | yes | `go vet -tags 'ze_core ze_distro ze_gnmi ze_grpc ze_isis ze_ldp ze_lg ze_mcp ze_ospf ze_rest ze_rsvpte ze_ssh ze_telemetry ze_vrrp ze_web' ./...` exit 0; combos mirror `Makefile:139/143/147` |
| AC-2 test + drift guard | yes | `TestCodeQLBuildUsesShippedTags` PASS; reads `feature-gates.txt` and fails if any ze_ tag is missing from `codeql.yml` |
| AC-4 pin claims | yes | proxy `@v/list` empty for all 6 direct pins (charmbracelet/ssh, insomniacslk/dhcp, packetcap/go-pcap, wireguard/wgctrl, gokrazy/tools, gokrazy/updater); documented in `appliance-dep-bumps.md`; zero go.mod/go.sum/vendor diff |
| AC-5 / AC-6 docs | yes | `appliance-dep-bumps.md` proactive-review-cadence + GPLv2 `rtr7/kernel` source-offer sign-off sections present |
| Lint / vet | yes | `go vet ./scripts/status/` exit 0; `make ze-lint-changed` "0 issues" |
| Independent review | yes | 0 BLOCKER, 0 ISSUE (see Review Gate Run 2) |

## Review Gate

**All ACs implemented; spec closing 2026-07-21.**

### AC status (2026-07-21 — final)
| AC | Status | Evidence |
|----|--------|----------|
| AC-1 (govulncheck scheduled job) | **DONE** | `Makefile:270-272` `ze-vulncheck` (`go run golang.org/x/vuln/cmd/govulncheck@latest ./...`, on-demand, NOT in `stagesForMode`); `.github/workflows/govulncheck.yml` (schedule cron + workflow_dispatch, no push/PR, runs `make ze-vulncheck`); `TestGovulncheckScheduledWorkflow`. `go run ...@latest` verified to run despite the vendor dir. |
| AC-2 (CodeQL shipped tag set) | **DONE** | `.github/workflows/codeql.yml` builds `ze_core ze_distro $(ZE_FEATURES)`, `ze_core ze_appliance $(ZE_FEATURES)`, `ze_setup` (mirrors bin/ze, bin/ze-appliance, bin/ze-setup; 13 feature tags literal). `TestCodeQLBuildUsesShippedTags` + feature-gates.txt drift guard. All combos verified to compile across `./...`. |
| AC-3 (updater marker guard) | **DONE** | `internal/appliance/updater_hardening_markers_test.go` (prior commit 7a54527d0). |
| AC-4 (pin -> tag hygiene) | **DONE** | All 6 direct pseudo-version pins have NO upstream semver tag (proxy `@v/list` empty for each); documented in `ai/rules/appliance-dep-bumps.md` (no go.mod change — conservative, build-safe). |
| AC-5 (builddir review cadence) | **DONE** | `ai/rules/appliance-dep-bumps.md` "Proactive review cadence (builddir pins)". |
| AC-6 (GPLv2 source-offer sign-off) | **DONE** | `ai/rules/appliance-dep-bumps.md` GPLv2 `rtr7/kernel` source-offer note (flag-only, UNRESOLVED). |

### Run 1 (AC-3 slice, 2026-07-19)
Independent reviewer, 2 passes; Pass 2 CONFIRMED 0 BLOCKER, 0 ISSUE over the AC-3 delta.

### Run 2 (closure — AC-1/2/4/5/6, independent adversarial review, 2026-07-21)
Independent subagent review of the AC-1/2/4/5/6 changeset. **Verdict: CLEAN — 0 BLOCKER,
0 ISSUE, 2 NOTE.** NOTE-1 (codeql combos omitted `$(ZE_FEATURES)`) was ADDRESSED after the
review: the 13 feature tags were added to both `ze_core` combos and a feature-gates.txt drift
guard added to `TestCodeQLBuildUsesShippedTags`; the expansion was verified to compile across
`./...` (`go vet` exit 0) and the guard proven non-vacuous. NOTE-2 (em dashes) was pre-existing
content, not introduced here. Verified in the review: workflow shape (schedule, no push/PR,
`make ze-vulncheck`), the 3 tag combos compile, all 6 pins have no upstream tag (proxy-checked),
tests non-vacuous, YAML parses.

Gate satisfied: last run 0 BLOCKER, 0 ISSUE.
- Verification: `internal/appliance/` lints 0 issues; `go vet` clean; `gofmt -l` clean
  on both changed files; both guard tests PASS. (`make ze-lint-changed` reports 4
  issues, ALL in sibling-owned files outside this slice: `macvlan.go`,
  `show_linux.go`, `ping_test.go`, `reconcile.go`.)
- Drain recipe (parked, not committed): `tmp/drain-fixit-supply-chain-hardening.md`.
  Learned summary: `plan/learned/1195-fixit-supply-chain-hardening.md` (NNN contended;
  run `scripts/dev/learned_numbers.py --fix` at drain).

## Notes
- Skeleton captured from the 2026-07-16 repository audit. Finding = the **absence** of a dependency-vulnerability
  scanner and marker-persistence gate, not a known CVE. `govulncheck` verified absent from first-party build files
  (only `vendor/github.com/prometheus/procfs/Makefile.common`); `golang.org/x/vuln` absent from `go.sum`.
- Verified citations: `Makefile:279-280` (ze-verify -> verify_run.go), `stagesForMode` :112-158, `codeql.yml:92-95`
  (`go build ./...`, no tags), `reapply-updater-fixes.py:121-140` (4 markers), `go.mod:12,15,19,30,38,39` (pseudo-versions),
  `dependabot.yml:8-11` (builddir excluded), `rtr7/kernel/go.mod:5` (GPLv2 kernel).
- Open question for /ze-spec deepening: should govulncheck block `ze-verify` inline, or run as a scheduled CI job
  (R-3, A-4)? And should the updater fix be durably resolved by merging `gokrazy-updater-upstream.patch` upstream
  (deleting the fork + script) rather than guarding the fork with a marker test?
  - → AUTONOMOUS DEFAULT (2026-07-17), resolving BOTH open questions above (APPEND-ONLY; supersedes the
    `stagesForMode`-stage wiring struck through below in Required Reading, Data Flow, Wiring Test, AC-1, TDD,
    Files, Implementation Phase 1, and Checklist Goal Gates):
    1. **govulncheck placement -> SCHEDULED CI JOB, not an inline `ze-verify` stage.** Add govulncheck as an
       on-demand `make ze-vulncheck` target (`govulncheck ./...`) invoked by a NEW scheduled workflow
       `.github/workflows/govulncheck.yml` (`schedule: cron`, mirroring the `codeql.yml:19-20` cron precedent),
       and deliberately do NOT register it in `stagesForMode`. Rationale: it must NOT block the inline
       `ze-verify` / pre-commit loop (`.woodpecker/verify.yml:19` runs the full `make ze-verify` on every push,
       so any `stagesForMode` entry gates every commit); the vuln DB is a network fetch (A-4) and a transient or
       false-positive advisory (R-1, R-3) must not wedge the dev loop. This mirrors the perf-alloc-gate split
       (`plan/spec-fixit-perf-alloc-ci-gate.md`): deterministic/bounded checks register inline in `stagesForMode`;
       host/network-dependent checks (Docker there, vuln DB here) run scheduled. Keeping the `ze-vulncheck` make
       target lets a developer run it on demand and gives the scheduled workflow a single source of truth for the
       invocation. Thomas: override to inline (add `mk("ze-vulncheck")` to the default `stagesForMode` branch) if
       you want every full `ze-verify` to fail on a new advisory. [STAKES: scope]
    2. **updater fix -> KEEP THE FORK + MARKER-GUARD TEST now; upstreaming stays a tracked follow-up.** Adopt the
       self-contained option: land `TestUpdaterHardeningMarkersPresent` (AC-3 primary path) so a re-vendor that
       drops a marker fails the unit gate. Do NOT gate this spec on merging `scripts/dev/gokrazy-updater-upstream.patch`
       upstream: that depends on the gokrazy maintainers and is outside the implementer's control. It remains the
       durable fix, tracked by the patch plus `scripts/dev/reapply-updater-fixes.py`; when upstream merges, AC-3's
       alternate path applies (delete fork + script, pin the fixed tag). Rationale: conservative smaller/self-contained
       default the implementer can complete deterministically today. Thomas: override if you hold upstream commit
       rights and prefer to delete the fork now. [STAKES: scope]
