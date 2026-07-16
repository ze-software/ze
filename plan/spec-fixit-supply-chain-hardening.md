# Spec: fixit-supply-chain-hardening

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

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
  → Constraint: a new SCA stage is added by inserting one `mk("ze-vulncheck")` line in BOTH the `ze-verify-changed` and default branches; the Makefile comment (`Makefile:282-289`) warns stages live here, not in a Makefile `.PHONY` chain.
  → Decision: govulncheck is a scheduled/full-verify concern (network fetch of the vuln DB); place it in the default branch, and consider gating it behind availability rather than blocking `ze-verify-changed`.
- [ ] `.github/workflows/codeql.yml` - manual Go build (:92-95) and default query pack
  → Constraint: replace `go build ./...` with a build across the shipped tag set (`-tags 'ze_core ze_distro ze_appliance ...'`) so feature-gated code is analyzed; enabling `security-extended` (:84 comment) is the SCA-adjacent query knob.
- [ ] `ai/rules/appliance-dep-bumps.md` - why builddir modules are Dependabot-excluded and bumped by runbook
  → Constraint: do NOT bring builddir modules under Dependabot (a PR would fight the pin); add a **proactive review** cadence instead, not automated bumps.

**Key insights:**
- The verify gate is a registry of stages (`stagesForMode`), not hardcoded shell — the SCA stage registers there, honoring registration-over-hardcoding.
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
- **SCA gate:** `make ze-verify` -> `scripts/status/verify_run.go` -> `stagesForMode` produces the stage list -> a new `ze-vulncheck` stage runs `govulncheck ./...` (the real full-verify path CI invokes at `.woodpecker/verify.yml:19`).
- **CodeQL:** `push`/`pull_request`/`schedule` -> `codeql.yml` manual build compiles the module; today `./...` with no tags, changed to the shipped tag set.
- **Updater fix-marker:** `go mod vendor` rewrites `vendor/.../updater/updater.go`; a package test reads that file and asserts the four markers.

### Transformation Path
1. Verify run assembles stages via `stagesForMode`; each stage shells to `make <name>` and its exit code gates the run.
2. A new `ze-vulncheck` target `go install`s/ runs `govulncheck ./...`, scanning the module's dependency graph against the vuln DB; non-zero exit fails the gate.
3. CodeQL's manual build compiles the tagged surface, feeding the feature-gated packages into the analysis database.
4. The fix-marker test parses the vendored updater file and asserts the LimitReader/Body.Close/NoBody markers are present.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CI ↔ verify gate | `make ze-verify` runs `stagesForMode`; new SCA stage registers there | [ ] |
| Dependency graph ↔ vuln DB | `govulncheck ./...` fetches advisories and reports reachable vulns | [ ] |
| Re-vendor ↔ hardening | package test asserts the 4 updater fix markers survive `go mod vendor` | [ ] |
| Source tags ↔ pins | `go.mod` pseudo-versions replaced by tagged releases where published | [ ] |

### Integration Points
- `scripts/status/verify_run.go` (`stagesForMode`), `Makefile` (new `ze-vulncheck` target), `.github/workflows/codeql.yml` (tag set + query pack), the updater package test, `go.mod` (pins), `ai/rules/appliance-dep-bumps.md` (builddir review cadence).

### Architectural Verification
- [ ] No bypassed layers (SCA stage runs through `stagesForMode`, the same registry every other stage uses; not an ad-hoc shell step)
- [ ] No duplicated functionality (reuse the `mk(name)` stage helper and the existing updater test package)
- [ ] Registration over hardcoding — the SCA stage is registered in `stagesForMode`, discovered by the gate, not spelled inline in a Makefile chain (`ai/rules/discovery-updates.md`)

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
| `stagesForMode("ze-verify", ...)` | -> | list contains a `ze-vulncheck` stage | `TestStagesForModeIncludesVulncheck` |
| `go mod vendor` re-vendors updater | -> | vendored file retains the 4 hardening markers | `TestUpdaterHardeningMarkersPresent` |
| `codeql.yml` manual build | -> | build command carries the shipped `-tags` set | `TestCodeQLBuildUsesShippedTags` (workflow-string assertion) |

Concrete test: `TestStagesForModeIncludesVulncheck` in `scripts/status/verify_run_test.go` asserts the default-mode
slice returned by `stagesForMode` contains a stage named `ze-vulncheck`; `TestUpdaterHardeningMarkersPresent` in
`internal/appliance/updater/` reads `vendor/github.com/gokrazy/updater/updater.go` and asserts all four marker
literals are present (fails the unit gate if a re-vendor drops them).

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-verify` (or the scheduled CI job) | runs `govulncheck ./...` as a registered stage; a seeded known-vuln fixture makes it exit non-zero; `stagesForMode` lists `ze-vulncheck` |
| AC-2 | `codeql.yml` manual Go build | compiles the shipped tag set (`ze_core`/`ze_distro`/`ze_appliance` ...), so feature-gated `cmd/ze`/appliance code enters the CodeQL database |
| AC-3 | `go mod vendor` drops a hardening marker | `TestUpdaterHardeningMarkersPresent` fails; with all four markers present it passes (or: upstream PR merged, fork+script deleted, dep pinned to the fixed tag) |
| AC-4 | The six pseudo-version pins | each is moved to a tagged release where upstream publishes one; each remaining pseudo-version is documented with the reason (no tag) |
| AC-5 | Appliance builddir pins | a proactive review cadence is recorded in `ai/rules/appliance-dep-bumps.md` (not automated Dependabot) |
| AC-6 | GPLv2 `rtr7/kernel` in the shipped image | the need for a source-offer compliance sign-off is recorded (flag only; no legal adjudication in this spec) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStagesForModeIncludesVulncheck` | `scripts/status/verify_run_test.go` | AC-1 | |
| `TestUpdaterHardeningMarkersPresent` | `internal/appliance/updater/hardening_markers_test.go` | AC-3 | |
| `TestCodeQLBuildUsesShippedTags` | `scripts/status/verify_run_test.go` (or workflow lint) | AC-2 | |

### Functional Tests
Test infrastructure / CI-gate only; no user-facing runtime feature. The SCA gate is exercised by
`make ze-verify` running the new `ze-vulncheck` stage; CodeQL and Dependabot run in GitHub CI. No `.ci`
functional test applies. Opt-out justification: **test infrastructure only** (supply-chain gates, not product behavior).

## Files to Modify
- `scripts/status/verify_run.go` - register the `ze-vulncheck` stage in `stagesForMode` (non-test feature file)
- `Makefile` - add the `ze-vulncheck` target running `govulncheck ./...`
- `.github/workflows/codeql.yml` - build with the shipped tag set; optionally enable `security-extended`
- `go.mod` - move pseudo-version pins to tags where upstream publishes them
- `ai/rules/appliance-dep-bumps.md` - record the proactive builddir-pin review cadence and GPLv2 source-offer sign-off note
- `scripts/dev/reapply-updater-fixes.py` - only if upstreaming the PR lets us delete the fork + script

## Files to Create
- `internal/appliance/updater/hardening_markers_test.go` - asserts the 4 updater fix markers survive re-vendor

## Implementation Steps

### Implementation Phases
1. **Phase: SCA gate (MANDATORY FIRST)** — add the `ze-vulncheck` Makefile target (`govulncheck ./...`), register it in `stagesForMode` (default branch), add `golang.org/x/vuln` as a tool dep; confirm `make ze-verify` discovers and runs it.
   - Tests: `TestStagesForModeIncludesVulncheck`
   - Files: `Makefile`, `scripts/status/verify_run.go`, `go.mod`
2. **Phase: CodeQL tag set** — replace `go build ./...` with the shipped `-tags` build so feature-gated code is analyzed; assert the tag string in the workflow.
   - Tests: `TestCodeQLBuildUsesShippedTags`
3. **Phase: updater fix-marker guard** — add `TestUpdaterHardeningMarkersPresent`; decide upstream-PR-merge-and-delete vs keep-fork-with-guard.
4. **Phase: pin hygiene** — per-dep, check for a tag and move pins; document any that must stay pseudo-versioned; record the builddir review cadence + GPLv2 sign-off note.
5. **Full verification** — `make ze-test`, then `make ze-verify` (exercises the new SCA stage).
6. **Complete spec** — audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

### Discovery-Update Obligation (`ai/rules/discovery-updates.md`)
- Source of truth: `stagesForMode` in `scripts/status/verify_run.go` — the verify gate registry (registration over hardcoding).
- New make target `ze-vulncheck` documented where verify targets are listed; `ai/rules/appliance-dep-bumps.md` gains the review cadence.
- No new runtime dependency ships in the binary (`golang.org/x/vuln` is a build/CI tool), so no `ai/INDEX.md` component entry.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] SCA stage runs under `make ze-verify`
- [ ] Registration over hardcoding respected (SCA stage registered in `stagesForMode`)
- [ ] Discovery update done (`ai/rules/appliance-dep-bumps.md`, verify-target docs)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary cases (missing marker, missing stage, untagged build) present

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
