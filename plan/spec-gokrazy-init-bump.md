# Spec: gokrazy-init-bump

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/6 |
| Updated | 2026-07-22 |

Phase corrected 2026-07-22: the bump landed and is committed (`8dc8f389d`
"feat(appliance): bump gokrazy init 20260703 (clears CVE-2026-25680)" plus
`eae560cc6` x/net -> v0.56.0); builddir go.mods pin `gokrazy
v0.0.0-20260703061218-a4a45a20149d`, the new version is vendored under
`gokrazy/modcache/`, and no CVE workaround comment remains. The plausible
remaining item is AC-7 (recurrence guardrail) -- verify it before closing.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `mk/gokrazy.mk` (build + modcache mechanism), `docs/guide/appliance.md`
4. `gokrazy/modcache/.gitignore` (whitelist), the 7 `gokrazy/ze/builddir/*/go.mod` files

## Task

Bump the vendored gokrazy appliance init from `v0.0.0-20260218074004-791851666ca2`
(2026-02-18) to `v0.0.0-20260703061218-a4a45a20149d` (2026-07-03). The newer commit
requires `golang.org/x/net v0.56.0` natively, so bumping removes the stale
`x/net v0.38.0` manifest that trips Dependabot alert #26 (CVE-2026-25680,
`x/net/html` parser DoS) at its source, rather than masking it with a pin.

Scope (confirmed lean + recurrence guardrails):
1. Bump the version string in the 7 builddir modules and refresh their go.sums.
2. Remove the now-false `x/net` workaround comment + redundant explicit pin.
3. Re-vendor the committed init source under `gokrazy/modcache/`.
4. Refresh the two stale path references in `plan/spec-kernel-lockdown-hardening.md`.
5. (AC-7) Add a recurrence guardrail so the next stale-manifest alert is faster to handle.

Out of scope: top-level `go.mod` (it does not reference `github.com/gokrazy/gokrazy`).

## Required Reading

### Architecture Docs
- [ ] `mk/gokrazy.mk` - image build + `ze-gokrazy-deps` modcache population
  → Constraint: `gok` is pointed at a checked-in `GOMODCACHE` (`gokrazy/modcache/`) via `cmd/ze-gok/main.go`; `ze-gokrazy-deps` runs `go mod download all` per builddir (`mk/gokrazy.mk`). Bump builddir go.mods BEFORE running deps.
  → Constraint: `E2FS` is hardcoded to a homebrew path (`mk/gokrazy.mk`); on Linux the build needs `E2FS=/usr/sbin`.
  → Constraint: the pinned kernel handling (`mk/gokrazy.mk`) keys off the `rtr7/kernel` MODULE version, NOT the gokrazy version — bumping gokrazy does not touch the kernel pin.
- [ ] `cmd/ze-gok/main.go` - the gok wrapper
  → Constraint: defaults `GOMODCACHE` to `<wd>/gokrazy/modcache` then calls `gok.Context{}.Execute()`; the checked-in modcache IS the build's module source.
- [ ] `vendor/github.com/gokrazy/tools/packer/gotool.go` - how gok compiles appliance packages
  → Constraint: gok hardcodes `-mod=mod` (`:238,323,451`) and calls `go get`. It has ZERO vendor support — a builddir `vendor/` tree would be ignored. This is why the modcache exists and why "full go mod vendor" was rejected.
- [ ] `gokrazy/modcache/.gitignore` - what of the modcache is committed
  → Constraint: ignores `*`, whitelists only `github.com/gokrazy/gokrazy@*/**` (init source). The `@*` glob auto-whitelists the NEW version path on re-vendor. `docs/` and `website/` are re-ignored.
- [ ] `docs/guide/appliance.md` - appliance build/run user surface
  → Decision: appliance changes are verified via the QEMU suite in `mk/test-integration.mk`, not unit tests.

### RFC Summaries (MUST for protocol work)
- N/A — no protocol/wire behavior changes.

**Key insights:**
- The alert anchors to ONE committed upstream manifest: the vendored init `go.mod`. `vendor/` carries 0 dependency go.mods; the modcache whitelists only the init source.
- What actually compiles is already safe: the builddir main module pins `x/net v0.56.0` and MVS takes the max, so `v0.38.0` never builds. The bump makes the vendored manifest tell the truth AND lets us drop the pin.
- gok cannot consume `vendor/` trees (`-mod=mod` + `go get`), so remediation must stay inside the modcache design.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod` - requires gokrazy `@...20260218...` + explicit `x/net v0.56.0` with the workaround comment `// Pinned >= v0.55.0 ... upstream gokrazy pins v0.38.0`.
  → Constraint: the comment is a factual claim that becomes FALSE after the bump (upstream will pin 0.56.0).
- [ ] `gokrazy/ze/builddir/github.com/gokrazy/serial-busybox/go.mod` + `.../rtr7/kernel/go.mod` - `replace ... => github.com/gokrazy/gokrazy@...20260218...`.
  → Constraint: both `replace` targets must move to the new version.
- [ ] `gokrazy/modcache/github.com/gokrazy/gokrazy@...20260218.../go.mod` - the scanned manifest; line 24 `golang.org/x/net v0.38.0 // indirect`.
- [ ] `gokrazy/modcache/.../gokrazy.go`, `.../reboot_amd64.go` - pristine upstream (single import commit `86960d858`, no local patches).
  → Constraint: re-vendoring is a clean replace — no local patches to preserve.
- [ ] `plan/spec-kernel-lockdown-hardening.md` - references the OLD version path in two file anchors.
  → Constraint: `Status: design`, explicitly not scheduled; refreshing its path refs is a consistency fix, not a functional dependency.

**Behavior to preserve:**
- Appliance boots and the gokrazy init supervises ze + dhcp + ntp + heartbeat + randomd.
- `ze-gokrazy-deps` remains an offline-enabling one-time download; the modcache whitelist mechanism is unchanged.
- The kernel `replace`/modcache flow (`mk/gokrazy.mk`) is untouched.

**Behavior to change:**
- gokrazy init module version (7 builddir modules + committed source tree).
- Removal of the `x/net` pin + workaround comment (no longer needed).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build-time: `make ze-gokrazy` → `gok overwrite` reads the 7 builddir modules with `-mod=mod` against `GOMODCACHE=gokrazy/modcache`.
- Scan-time: GitHub dependency graph parses every committed `go.mod` as a manifest.

### Transformation Path
1. `ze-gokrazy-deps` runs `go mod download all` per builddir → populates `gokrazy/modcache/` (incl. extracted init source).
2. `gok` compiles each appliance package (`go build -mod=mod`) resolving versions via MVS across builddir go.mod + gokrazy go.mod.
3. Committed init source (whitelisted 60 files) is what GitHub scans and what feeds the kernel-lockdown design references.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Repo ↔ GitHub dep graph | committed `go.mod` files parsed as manifests | [ ] |
| builddir module ↔ modcache | `go mod download`/`-mod=mod` resolution | [ ] |
| gok ↔ Go toolchain | `go build -mod=mod`, `go get` | [ ] |

### Integration Points
- `cmd/ze-gok/main.go` sets `GOMODCACHE` → the checked-in modcache.
- `mk/gokrazy.mk` `ze-gokrazy-deps` / `ze-gokrazy` targets.

### Architectural Verification
- [ ] No bypassed layers (bump stays within the modcache/builddir design)
- [ ] No unintended coupling (top-level module untouched; kernel pin untouched)
- [ ] No duplicated functionality (no new pin; we remove one)
- [ ] Zero-copy preserved where applicable (N/A)
- [ ] Registration over hardcoding (N/A — no new runtime feature)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Upstream `@...20260703...` requires `x/net v0.56.0` natively | proxy `.mod` fetch (2026-07-03 version) + `main` go.mod both show `x/net v0.56.0` | bump doesn't clear the alert; must keep a pin | re-read the fetched `.mod` at implement time | confirmed — proxy .mod + new committed `go.mod:23` = `x/net v0.56.0` |
| A-2 | The committed init source is pristine upstream (no local patches) | single git commit `86960d858`; no patch markers in `reboot_amd64.go`/`gokrazy.go` | re-vendor clobbers local work | `git log` on the two files (done in research) | confirmed — single commit `86960d858`; `loadModules`/`KexecFileLoad` still present in new source |
| A-3 | `ze-gokrazy-deps` re-vendors the NEW version into a whitelisted path and the old tree can be `git rm`'d + pruned cleanly | `.gitignore` uses `@*` glob; deps runs `go mod download all` | dirty/partial modcache; build can't find init | run deps, `git status`, then `make ze-gokrazy` | confirmed — 58 removed/58 added, no stray files, old dir pruned + no reappearance, AC-5 build green |
| A-4 | The 4-month upstream init delta does not break appliance boot or the appliance API used by ze | pristine upstream, gokrazy API stable | appliance fails to boot or supervise | QEMU suite (integration/l2tp/pppoe) | PENDING — AC-6 not runnable on this host (no qemu/xl2tpd/pppd, non-root); R-6 realized |
| A-5 | Dropping the explicit `x/net v0.56.0` pin still yields `>= 0.56.0` via the new gokrazy require | new upstream go.mod requires 0.56.0; MVS max | a transitive dep pulls a lower x/net | inspect resolved `go list -m golang.org/x/net` per builddir after deps | confirmed — `go list` = `x/net v0.56.0` in gokrazy/dhcp/serial-busybox; kernel doesn't pull x/net |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Appliance boot regression from upstream init changes | QEMU boot hangs / init supervision failure | keep the old version reachable in git; revert the bump commit |
| R-2 | go.sum drift: hand-editing hashes produces checksum mismatch | `go build` "verifying module: checksum mismatch" | use `go mod download`/`go mod tidy`, never hand-edit hashes |
| R-3 | Re-vendor leaves a stale old-version dir on disk (untracked) that reappears in a future commit | `git status` shows old `@...20260218...` tree | explicitly `rm -rf` the old modcache dir + confirm `git status` clean |
| R-4 | Dropping the pin lets x/net regress below 0.56.0 | `go list -m golang.org/x/net` < 0.56.0 | re-add a minimal pin only if MVS resolves lower (A-5) |
| R-5 | Kernel-lockdown design spec left pointing at a dead path | grep for old version string in `plan/` | refresh the two refs (AC-4) |
| R-6 | Host lacks root/xl2tpd/pppd/PPPoL2TP → full appliance L2TP proof can't run here | `ze-check-qemu`/deployment test reports missing deps | user requires FULL L2TP proof (AC-6 blocks); provide setup + commands, keep spec OPEN until green elsewhere — no partial-done |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- For an appliance dep bump the "wiring" proof is that the rebuilt image boots and the init supervises services in QEMU. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-gokrazy` (rebuilt image, new init) | → | gokrazy init boots the image | `make ze-vpp-hugepages-qemu-test` (corrected at closure 2026-08-03: this row named `test/appliance/serial-login.ci`, which boots nothing -- `ai/rules/platform-linux.md` strikes it out of the proof table) |
| gokrazy image boot on the new init | → | init supervises ze + L2TP/PPP path works end-to-end | `ze-deployment-gokrazy-l2tp-ppp-test` (`scripts/evidence/effective-gokrazy-l2tp-ppp.py`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `grep -r 20260218074004 gokrazy/ plan/` after change | No tracked file references the old version string; all 7 builddir modules + committed source tree are at `...20260703061218-a4a45a20149d`. |
| AC-2 | Inspect the 6 builddir go.mods that had the pin | The `// Pinned >= v0.55.0 ... upstream gokrazy pins v0.38.0` comment is gone; the explicit `x/net` require is removed OR retained only if A-5 shows MVS resolves lower (documented). |
| AC-3 | `git status` after re-vendor + `ze-gokrazy-deps` | New `gokrazy@...20260703...` source tree tracked (whitelisted); old tree `git rm`'d; no stale old-version dir on disk; working tree clean of unintended modcache noise. |
| AC-4 | `grep 20260218074004 plan/spec-kernel-lockdown-hardening.md` | Zero hits; both anchors point at the new version path. |
| AC-5 | Rebuild: `make ze-gokrazy E2FS=/usr/sbin USER=.. PASS=..` | Image builds; every appliance package (init/ze/dhcp/ntp/heartbeat/randomd) compiles against the new gokrazy. |
| AC-6 | Boot the rebuilt gokrazy image in QEMU: `make ze-vpp-hugepages-qemu-test` + `ze-deployment-gokrazy-l2tp-ppp-test`; plus `ze-qemu-integration-test` for ze regression | Appliance boots and supervises services on the NEW init; L2TP/PPP works end-to-end; ze functional suite still green. Corrected at closure 2026-08-03: this row named `test/appliance/serial-login.ci` as the boot proof and that test boots nothing. |
| AC-7 | Recurrence guardrail present | BOTH: (a) `ai/rules/platform-linux.md` documents this bump procedure + an `ai/INDEX.md` pointer row; (b) `.github/dependabot.yml` adds a grouped, weekly `gomod` update entry. Caveat recorded: dependabot.yml does NOT suppress the always-on security scan — its value is earlier update PRs. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | boots the rebuilt appliance image | new gokrazy init → mounts, supervises ze/dhcp/ntp/heartbeat | `ze-qemu-integration-test` |
| 2 | GitHub rescans the default branch | dependency graph parses committed go.mods → no vulnerable x/net manifest → alert #26 auto-closes | manual: alert state after push (owner) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A | - | No new Go logic; a version bump has no unit surface | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
<!-- NOTE: the generic ze-qemu-* targets boot an Alpine VM with host-compiled ze
     binaries — they exercise ze, NOT the gokrazy init. The init-specific proof is
     booting the gokrazy IMAGE: make ze-vpp-hugepages-qemu-test (boot + kernel
     cmdline on the new init) + the gokrazy deployment evidence script (boot +
     supervise + L2TP). Corrected at closure 2026-08-03: this note named
     test/appliance/serial-login.ci, which boots nothing (ai/rules/platform-linux.md). -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-vpp-hugepages-qemu-test` | `mk/test-integration.mk` | gokrazy image built by `ze appliance build` boots in QEMU on the new init | UNRUN (AC-6) |
| `test/l2tp/handshake-full.ci` | `test/l2tp/` | L2TP control path intact (functional suite, regression) | |
| `test/pppoe/pppoe-basic.ci` | `test/pppoe/` | PPPoE path intact (functional suite, regression) | |

Driver targets: `ze-deployment-gokrazy-l2tp-ppp-test` (`scripts/evidence/effective-gokrazy-l2tp-ppp.py`, boots the gokrazy image + supervises + L2TP/PPP on the new init — the primary init proof) and `ze-qemu-integration-test` (Alpine VM functional-suite regression).

### Interop Tests (MANDATORY for protocol features)
- N/A — no wire protocol change.

### Future (if deferring any tests)
- None.

## Files to Modify
- `gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod` + `go.sum` - version + pin/comment removal
- `gokrazy/ze/builddir/github.com/gokrazy/serial-busybox/go.mod` + `go.sum` - replace target + pin/comment
- `gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod` + `go.sum` - replace target
- `gokrazy/ze/builddir/github.com/gokrazy/gokrazy/cmd/{dhcp,ntp,heartbeat,randomd}/go.mod` + `go.sum` - version + pin/comment
- `gokrazy/modcache/github.com/gokrazy/gokrazy@.../**` - re-vendored init source tree (old removed, new added)
- `plan/spec-kernel-lockdown-hardening.md` - refresh two path refs (AC-4)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | N/A | no config surface — build-time dependency bump |
| YANG validation | N/A | — |
| YANG custom validators | N/A | — |
| CLI commands/flags | N/A | no CLI change |
| CLI grammar | N/A | — |
| Editor autocomplete | N/A | — |
| Functional test for new RPC/API | N/A | no RPC; functional proof is the QEMU suite |
| Pipe completeness | N/A | — |
| Env var registration | N/A | — |
| Doctor check for runtime dependencies | N/A | no new runtime dependency; the appliance already depends on gokrazy init |
| Prometheus counters/metrics | N/A | — |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | version bump, no feature |
| 2 | Config syntax changed? | No | — |
| 3 | CLI command added/changed? | No | — |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | No | — |
| 6 | Has a user guide page? | No | `docs/guide/appliance.md` describes the build, not the pinned version |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior? | No | — |
| 10 | Test infrastructure changed? | No | reuses existing `ze-qemu-*` targets |
| 11 | Affects daemon comparison? | No | — |
| 12 | Internal architecture changed? | No | — |
| 13 | Route metadata keys? | No | — |
| 14 | Prometheus counters? | No | — |
| 15 | Registered plugin/event/command/inventory changed? | No | — |
| 16 | Any changed source file referenced by doc source anchors? | Yes | grep `docs/` for the version string / modcache anchors; update if any |
| 17 | Existing docs show examples for this area? | Yes | verify `docs/guide/appliance.md` has no pinned-version example that goes stale |

## Files to Create
- `ai/rules/platform-linux.md` - runbook for bumping a vendored gokrazy/modcache dependency (AC-7)
- `ai/INDEX.md` - add a pointer row so the runbook is discoverable (AC-7)
- `.github/dependabot.yml` - grouped weekly `gomod` update config (AC-7)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify — confirm version string locations |
| 3. Wiring phase | Wiring Test — the QEMU boot proof |
| 4. Implement | Phases below |
| 5. Full verification | `make ze-gokrazy E2FS=/usr/sbin` build + QEMU suite |
| 6-9. Critical review + fix | Critical Review Checklist |
| 10-12. Deliverables/security/docs | Checklists below |
| 13. /ze-review gate | Review Gate |
| 14. Close | Two-commit closure |

### Implementation Phases
1. **Phase: Manifest bump** — bump version in 7 builddir modules; refresh go.sums via `go mod download`/`go mod edit` (never hand-edit hashes, R-2).
   - Verify: `grep 20260218074004` empty in builddir go.mods; `go list -m golang.org/x/net` >= 0.56.0 (A-5).
2. **Phase: Pin/comment cleanup** — remove workaround comment + redundant pin (AC-2), keep only if A-5 fails.
   - Verify: comment gone; builds still resolve x/net >= 0.56.0.
3. **Phase: Re-vendor init source** — `ze-gokrazy-deps`; `git rm` old tree; `git add` new whitelisted tree; `rm -rf` stale old-version dir; confirm `git status` clean (AC-3, R-3).
4. **Phase: Refresh coupling** — update the two `plan/spec-kernel-lockdown-hardening.md` path refs (AC-4).
5. **Phase: AC-7 guardrail** — write `ai/rules/platform-linux.md` + `ai/INDEX.md` pointer + `.github/dependabot.yml` (grouped weekly gomod).
6. **Verification** — `make ze-gokrazy E2FS=/usr/sbin ...` build (AC-5) + boot proof + **full** `ze-deployment-gokrazy-l2tp-ppp-test` (AC-6, BLOCKS on green). If this host lacks root/xl2tpd/pppd/PPPoL2TP, provide setup + commands and keep the spec OPEN until the user confirms green elsewhere (R-6).
7. **Complete spec** — audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N demonstrated with evidence |
| Correctness | No tracked file references the old version; x/net resolves >= 0.56.0 |
| Data flow | Bump stays within modcache/builddir; top-level module + kernel pin untouched |
| Re-vendor hygiene | No stale modcache dir; `git status` clean; only intended files tracked |
| Rule: no-workarounds | The false pin comment is removed, not merely edited |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Version bumped everywhere | `grep -r 20260218074004 gokrazy/ plan/` → empty |
| Image builds | `make ze-gokrazy E2FS=/usr/sbin USER=.. PASS=..` exit 0 |
| QEMU green | three `ze-qemu-*` targets pass |
| Guardrail present | `ls` the AC-7 artifact |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Supply chain | New version hashes come from the module proxy via go.sum, not hand-typed |
| Vuln closed | Resolved x/net >= 0.56.0 (past CVE-2026-25680 fix in 0.55.0) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| go.sum checksum mismatch | Phase 1 — regenerate via go tooling |
| QEMU boot failure | R-1 — inspect init logs; consider revert |
| Old version dir reappears | Phase 3 — prune + re-check git status |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Full `go mod vendor` conversion | gok hardcodes `-mod=mod` + `go get`; zero vendor support (`gotool.go`) | version bump (this spec) |
| Own/edit the committed go.mod in place | user preferred a real upstream bump over a hand-owned manifest | version bump (this spec) |
| Dismiss alert #26 | leaves the stale manifest; may re-fire | version bump (this spec) |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

**Implementation progress (2026-07-10) — through AC-5 + AC-7; AC-6 pending capable host.**
- AC-1: bumped `...20260218074004...` → `...20260703061218-a4a45a20149d` in all 7 builddir modules; `git grep` of the old string is empty repo-wide.
- AC-2: removed the false pin comment + redundant `x/net v0.56.0` pin from 6 builddir go.mods; `go list -m x/net` still resolves 0.56.0 (A-5).
- AC-3: deleted 7 stale go.sums, `ze-gokrazy-deps` regenerated them clean, re-vendored (58 removed/58 added), pruned old modcache dir; old dir did not reappear after build.
- AC-4: refreshed the two `spec-kernel-lockdown-hardening.md` path refs; `loadModules`/`KexecFileLoad` still present in new source.
- AC-5: `make ze-gokrazy E2FS=/usr/sbin` → "Build complete!", `tmp/gokrazy/ze.img`. New committed `go.mod:23` = `x/net v0.56.0` (scanned manifest now carries the fix).
- AC-7: runbook `ai/rules/platform-linux.md` (+ regenerated `ai/rules/INDEX.md`, `ai/INDEX.md` pointer) and `.github/dependabot.yml` (weekly grouped gomod + github-actions).
- **AC-6 (BLOCKING) NOT done:** appliance QEMU boot + full L2TP proof unrunnable here (no qemu, no xl2tpd/pppd, non-root). Downstream review/closure paused; NOT committed; spec remains in-progress.
- Added (user request, related to AC-6 provisioning): `scripts/dev/dev-setup.py` now lists `xl2tpd` + `ppp` as optional Linux L2TP-evidence deps, so `make ze-setup` surfaces them; `ai/INDEX.md` updated. Verified: ruff clean, `dev_setup_drift_test.go` green (its regex only matches `appliance-*` `APPLIANCE_CHECKS` keys, not the tool lists), `ze-setup --check` now reports `xl2tpd`/`ppp` as optional.
- Fixed (Linux-support bug surfaced by AC-6): `mk/gokrazy.mk` hardcoded `E2FS := /opt/homebrew/...` (macOS), which a `:=` env var can't override, so the L2TP script's `make ze-gokrazy` failed the e2fsprogs guard on Linux. Now autodetects (`ifndef E2FS` → first of `/usr/sbin`,`/sbin`,`/usr/local/sbin`,homebrew Cellar carrying BOTH `mkfs.ext4` and `debugfs`); `make -n` resolves `E2FS=/usr/sbin` here. Corrected at closure 2026-08-03: the probe and the guard originally tested `mkfs.ext4` alone, which let a directory missing `debugfs` pass and fail later with its stderr discarded; the guard now names whichever tool is missing. Also corrected: an override is `make ... E2FS=/path`. `make ... E2FS=` (explicitly empty) does NOT resume autodetect, and the mechanism is not the obvious one. `ifndef` tests whether the variable expands to something NON-EMPTY, so the empty override still enters the block and the probe RUNS; its result is discarded because a command-line assignment beats the makefile's `:=`, `E2FS` stays empty, and the guard rejects it. Measured on GNU make with a two-line reproducer.

Learned insight: gok's checked-in modcache means a Dependabot alert on `gokrazy/modcache/**/go.mod` is a stale *vendored upstream manifest*, and `go mod download all` after a version bump regenerates a deleted go.sum from the new build list — pruning the old version string cleanly without hand-editing hashes.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Bump to newest upstream pseudo-version | own-the-go.mod edit; dismiss | user chose a real upstream update carrying the fix natively |
| Stay within modcache design | fork gokrazy/tools for `-mod=vendor` | gok has no vendor support; forking fights its design |

## Known Limitations
- Does not eliminate the general "committed upstream manifest gets scanned" class — gok's `-mod=mod` design requires the init go.mod present. AC-7 mitigates recurrence, does not remove the surface.

## Implementation Summary

### What Was Implemented
- The version bump itself, landed 2026-07-10 in `8dc8f389d` and `eae560cc6`: 7 builddir modules moved to `v0.0.0-20260703061218-a4a45a20149d`, the false `x/net` pin and its comment removed from 6 of them, and the committed init source re-vendored (58 files out, 58 in).
- AC-7 recurrence guardrail: `ai/rules/platform-linux.md` runbook, an `ai/INDEX.md` pointer row, and `.github/dependabot.yml` (weekly grouped `gomod` plus `github-actions`, scoped to the root module).
- `mk/gokrazy.mk` E2FS autodetection, so the appliance build works on Linux.
- At closure 2026-08-03: the E2FS probe and guard now require BOTH `mkfs.ext4` and `debugfs`, and the guard names whichever is missing and points at the distro package as well as homebrew.

### Bugs Found/Fixed
- `mk/gokrazy.mk` hardcoded a macOS homebrew `E2FS` path with `:=`, which an environment variable cannot override, so every Linux appliance build failed the e2fsprogs guard. Fixed by autodetection.
- The e2fsprogs guard checked one of the two tools the build uses, and named a macOS-only remedy. Fixed at closure; verified by rendering `make -n ze-gokrazy` and by exercising the guard's shell logic for empty, one-tool and both-tools directories.
- The spec cited `test/appliance/serial-login.ci` as the appliance boot proof in four places. That test boots nothing (`ai/rules/platform-linux.md` strikes it out of the proof table). Corrected in place.

### Documentation Updates
- `ai/rules/platform-linux.md` (the bump runbook) and the `ai/INDEX.md` pointer row, both landed with AC-7.
- `docs/guide/appliance.md` Prerequisites, updated at closure. It offered `brew install e2fsprogs` under a "(macOS)" heading and nothing else, which the Linux autodetect this spec added made incomplete. It now carries the Linux packages, states that BOTH `mkfs.ext4` and `debugfs` are needed and why, gives the search order, and records that an empty `E2FS=` is not an override. A source anchor on `mk/gokrazy.mk` was added with it.
- No `docs/` page quotes the gokrazy version, so the bump left no stale version claim: a grep of `docs/` for the old and the new version strings returns nothing.
- `make ze-doc-test` was NOT run at closure. A concurrent session is mid-restructure under `ai/`, and the doc gate reads the working tree.

### Deviations from Plan
- AC-6 is UNRUN and the spec closes with it unrun. It needs root, `/dev/ppp` and PPPoL2TP, which this host does not have. It is homed at `plan/spec-finish-appliance-qemu-evidence.md`, which lists it as a work item and exists to execute it.
- The boot proof named in the plan was wrong (see Bugs Found/Fixed). The proof AC-6 owes is `make ze-vpp-hugepages-qemu-test` plus `ze-deployment-gokrazy-l2tp-ppp-test`.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Bump the version string in 7 builddir modules, refresh go.sums | Done | `gokrazy/ze/builddir/**/go.mod` + `go.sum` | `git grep 20260218074004` returns hits only in this spec's prose |
| Remove the false `x/net` workaround comment and redundant pin | Done | the 5 gokrazy builddir go.mods | no `x/net` require survives in any tracked builddir manifest |
| Re-vendor the committed init source | Done | `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/` | one tracked version directory, and one on disk |
| Refresh the two stale path refs in the kernel-lockdown spec | Done | `plan/spec-kernel-lockdown-hardening.md`, its two modcache anchors | zero hits for the old string |
| AC-7 recurrence guardrail | Done | `ai/rules/platform-linux.md`, `ai/INDEX.md`, `.github/dependabot.yml` | the runbook is a 7-step procedure, not a stub |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `git grep 20260218074004` | only this spec's own prose matches |
| AC-2 | Done | the 7 builddir go.mods | the two surviving `replace` comments were re-checked and are true |
| AC-3 | Done | `git ls-files gokrazy/modcache/`, `ls gokrazy/modcache/github.com/gokrazy/` | one version directory, tracked and on disk; `git status gokrazy/` clean |
| AC-4 | Done | the two modcache anchors in `plan/spec-kernel-lockdown-hardening.md` | `loadModules` and `KexecFileLoad` exist in the new source |
| AC-5 | Done (2026-07-10), not reproducible now | recorded build output | `tmp/gokrazy/ze.img` is transient and gone. The corroborating fact still holds: the committed init `go.mod` requires `x/net v0.56.0` |
| AC-6 | **UNRUN** | nothing | needs root, `/dev/ppp`, PPPoL2TP. Homed at `plan/spec-finish-appliance-qemu-evidence.md`, live row in `plan/deferrals/gokrazy-init-bump.md` |
| AC-7 | Done | `ai/rules/platform-linux.md`, the `ai/INDEX.md` dependabot pointer row, `.github/dependabot.yml` | the caveat that dependabot.yml does not suppress security scanning is stated in the file header |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Unit tests | N-A | - | a version bump has no unit surface |
| `make ze-vpp-hugepages-qemu-test` | Not run | `mk/test-integration.mk` | AC-6; replaces the wrongly cited `serial-login.ci` |
| `ze-deployment-gokrazy-l2tp-ppp-test` | Not run | `scripts/evidence/effective-gokrazy-l2tp-ppp.py` | AC-6 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| 7 builddir `go.mod` + `go.sum` | Done | committed 2026-07-10 |
| `gokrazy/modcache/` re-vendor | Done | committed 2026-07-10 |
| `plan/spec-kernel-lockdown-hardening.md` | Done | committed 2026-07-10 |
| `ai/rules/platform-linux.md`, `ai/INDEX.md`, `.github/dependabot.yml` | Done | committed 2026-07-10 |
| `mk/gokrazy.mk` | Done | autodetect committed 2026-07-10; two-tool guard in the closing commit |

### Audit Summary
- **Total items:** 7 acceptance criteria
- **Done:** 6 (AC-1 to AC-5, AC-7)
- **Partial:** 0
- **Skipped:** 0
- **Unrun and homed:** 1 (AC-6, `plan/spec-finish-appliance-qemu-evidence.md`)
- **Changed:** the AC-6 proof target, recorded in Deviations

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| Full QEMU gokrazy L2TP appliance proof (AC-6) | deferred (live) | `plan/spec-finish-appliance-qemu-evidence.md`, which lists it in its Task section. The shard SURVIVES this closure and is not removed |
| Make the make-path credential injection fail closed (`debugfs` exits 0 on failure with its stderr discarded) | deferred (live) | `plan/spec-gokrazy-builddir-tmp-deferred-build-flow-unification.md`, work item added under "Added 2026-08-03". Found by the closure review |

No row anywhere in `plan/deferrals/` names `plan/spec-gokrazy-init-bump.md` as a
Destination, so this closure orphans nothing.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Clear Dependabot alert #26 at source (no stale x/net manifest) | grep + alert state | `grep -r "x/net v0.38.0" --include=go.mod` → empty on tracked files; alert #26 auto-closes after rescan |
| New init boots + supervises without regression | functional (QEMU appliance) | NOT MET at closure. `make ze-vpp-hugepages-qemu-test` + `ze-deployment-gokrazy-l2tp-ppp-test` on the rebuilt image are unrun; AC-6 is homed at `plan/spec-finish-appliance-qemu-evidence.md` |
| x/net resolves to the fixed version | resolution check | `go list -m golang.org/x/net` per builddir → `>= v0.56.0` |
| Recurrence made cheaper | artifact exists | AC-7 guardrail present (`.github/dependabot.yml` and/or runbook) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/gokrazy-init-bump-c4c78ddb-c47b-4f1a-a85d-5911d7c65455.md` |
| `review_gate.py check` | clean (1 code file, hashes match) |
| Reviewer lenses used | independent `/ze-review` subagent over three runs: logic and wiring, guard fail-closed behaviour, make semantics. The closure author verified every finding at source before acting on it |

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | The learned summary was an empty placeholder | `plan/learned/1329-gokrazy-init-bump.md` | Written |
| 2 | ISSUE | The spec cited `test/appliance/serial-login.ci` as the appliance boot proof in five places; that test boots nothing | Wiring Test, AC-6, Functional Tests, Goal Validation, TDD checklist | All five corrected to `make ze-vpp-hugepages-qemu-test` plus `ze-deployment-gokrazy-l2tp-ppp-test` |
| 3 | ISSUE | The e2fsprogs guard named a macOS-only remedy, and probed `/mkfs.ext4` when `E2FS` was empty | `mk/gokrazy.mk` `ze-gokrazy` guard | Rewritten: rejects an empty `E2FS`, names the distro packages and the override |
| 4 | NOTE | The deferral row's Reason contradicted its own Destination about whether the two appliance bugs were fixed | `plan/deferrals/gokrazy-init-bump.md` | Reason corrected |
| 5 | NOTE | `E2FS := $(shell ...)` forks a probe shell on every `make` invocation repo-wide | `mk/gokrazy.mk` | Recorded in the learned summary with the deferred-expansion remedy. Not fixed |

### Run 2
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 6 | ISSUE | Fail-open guard: the probe and the guard tested `mkfs.ext4` only, while the build also needs `debugfs`. The autodetect widened the hole, since it can select a directory on the strength of one tool | `mk/gokrazy.mk` probe and guard | Both now require BOTH tools, and the error names whichever is missing |
| 7 | NOTE | Learned summary bullets ran over the `METHODOLOGY.md` length budget | `plan/learned/1329-gokrazy-init-bump.md` | Tightened |
| 8 | NOTE | The corrected Functional Tests row left its Status cell blank, reading neutral rather than unrun | `plan/spec-gokrazy-init-bump.md` | Now reads `UNRUN (AC-6)` |
| 9 | NOTE | `debugfs` exits 0 on failure and its stderr is discarded at both credential-inject calls | `mk/gokrazy.mk` | Deferred with evidence to `plan/spec-gokrazy-builddir-tmp-deferred-build-flow-unification.md`. Measured on e2fsprogs 1.47.0 |

### Run 3
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 10 | ISSUE | The new comment stated the wrong `make` mechanism. It said `ifndef` is false for an empty command-line override. Measured: `ifndef` tests NON-EMPTINESS, so the probe RUNS and its result is then discarded, because a command-line assignment beats the makefile's `:=` | `mk/gokrazy.mk` comment, the learned summary, this spec's Design Insights | Reworded in all three, after reproducing the behaviour with a two-line makefile |
| 11 | ISSUE | Found by the closure author while verifying finding 10: the closure draft claimed no `docs/` source anchor named a changed file. The grep refuted it, and `docs/guide/appliance.md` Prerequisites offered `brew install` only, which this spec's Linux autodetect made incomplete | `docs/guide/appliance.md` | Rewritten with the Linux packages, the two-tool requirement, the search order and the `E2FS=` semantics, plus a `mk/gokrazy.mk` source anchor |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE outstanding. Findings 5 and 9 are recorded rather than fixed: 5 is a measured cost with a named remedy, 9 is a live deferral whose destination spec exists
- [ ] All NOTEs recorded above

## Pre-Commit Verification

All evidence below was re-collected on 2026-08-03, at closure, not carried over
from implementation.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `ai/rules/platform-linux.md` | Yes | `ls -1` lists it; the file is the AC-7 runbook |
| `.github/dependabot.yml` | Yes | `ls -1` lists it |
| `mk/gokrazy.mk` | Yes | `ls -1` lists it; carries the autodetect and the two-tool guard |
| `plan/spec-finish-appliance-qemu-evidence.md` | Yes | `ls -1` lists it; the AC-6 home |
| `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/` | Yes | the ONLY `gokrazy@` entry under `gokrazy/modcache/github.com/gokrazy/` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | No tracked file carries the old version string | `git grep -c 20260218074004` reports one file, `plan/spec-gokrazy-init-bump.md`, 8 hits, all prose |
| AC-2 | No `x/net` pin survives in a builddir manifest | `git grep golang.org/x/net -- 'gokrazy/ze/builddir/*go.mod'` returns nothing |
| AC-3 | One tracked modcache version tree, working tree clean | `git ls-files gokrazy/modcache/` = 60 files; `ls` shows one `gokrazy@` directory; `git status --porcelain gokrazy/` is empty |
| AC-4 | The kernel-lockdown anchors point at the new path | `grep -c 20260703061218 plan/spec-kernel-lockdown-hardening.md` = 2, and `grep -c 20260218074004` = 0 |
| AC-5 | The scanned manifest carries the fix | `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/go.mod` requires `golang.org/x/net v0.56.0`. The image itself is NOT re-verified: `tmp/gokrazy/ze.img` is transient and gone |
| AC-6 | UNRUN | no evidence exists, and none is claimed. Homed at `plan/spec-finish-appliance-qemu-evidence.md` |
| AC-7 | Guardrail present | `grep -c "package-ecosystem: gomod" .github/dependabot.yml` = 1; the runbook is 502 lines with a 7-step bump procedure; `ai/INDEX.md` carries a dependabot/modcache pointer row |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-gokrazy` (rebuilt image, new init) | `make ze-vpp-hugepages-qemu-test` | NOT verified. Read at closure: `test/appliance/serial-login.ci`, which the spec originally named here, asserts the offline argv[0] gate and boots nothing. The row was corrected rather than left to look satisfied |
| gokrazy image boot on the new init | `ze-deployment-gokrazy-l2tp-ppp-test` | NOT verified. Requires root, `/dev/ppp`, PPPoL2TP. This is AC-6 |
| `make ze-gokrazy` e2fsprogs guard | (no `.ci`; a Makefile guard) | Verified: `make -n ze-gokrazy` renders the guard, and its shell logic was exercised for empty `E2FS` (both tools reported missing), a directory holding only `mkfs.ext4` (` debugfs` reported), and `/usr/sbin` (nothing missing) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | the committed init `go.mod` requires `golang.org/x/net v0.56.0` |
| A-2 | confirmed | the re-vendored source is a clean upstream replace; no local patch markers |
| A-3 | confirmed | one tracked and one on-disk modcache version directory; `git status gokrazy/` clean |
| A-4 | **broken as stated, and unresolved** | the assumption was that the 4-month init delta does not break appliance boot. Nothing has tested it. It is NOT confirmed, and the spec closes saying so: the test that would resolve it is AC-6, homed at `plan/spec-finish-appliance-qemu-evidence.md` |
| A-5 | confirmed | no builddir manifest pins `x/net`, and the upstream require carries `v0.56.0` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| The AC-7 runbook documents this bump | `ai/rules/platform-linux.md` carries a 7-step "Appliance Dependency Bumps" procedure and strikes `test/appliance/serial-login.ci` out of the proof table | Yes |
| No `docs/` page pins the gokrazy version | a grep of `docs/` for the old and new version strings returns nothing | Yes |
| `.github/dependabot.yml` states its own limits | the file header says security-alert scanning is always-on and cannot be suppressed there, and that the builddir modules are deliberately unmanaged | Yes |
| A doc source anchor DOES point at a file this spec changed | `docs/guide/appliance.md` and `docs/functional-tests.md` both anchor on `mk/gokrazy.mk`, and `docs/guide/appliance.md` anchors on the builddir `go.mod` for "pinned gokrazy version". The first closure draft claimed no such anchor existed; the grep refuted it | Yes, after correction |
| `docs/guide/appliance.md` Prerequisites | It listed `brew install e2fsprogs` under "(macOS)" only, which this spec's Linux autodetect made incomplete. Updated at closure with the Linux packages, the two-tool requirement, the search order, and the `E2FS=` override semantics, carrying a new source anchor on `mk/gokrazy.mk` | Yes |
| `docs/guide/developer-setup.md` e2fsprogs row | Already says e2fsprogs provides `mkfs.ext4` AND `debugfs`, which agrees with the corrected guard. No edit needed | Yes |
| The builddir `go.mod` "pinned gokrazy version" anchor | The anchor names the file, and the page quotes no version number, so the bump leaves no stale claim behind it | Yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] Image build + QEMU suite pass
- [ ] `make ze-test` passes (lint + all ze tests) — proves the repo still builds/lints after the bump
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written — N/A for new Go logic (version bump); functional proof is the appliance boot .ci + gokrazy deployment test
- [ ] Tests FAIL (paste output) — N/A (no new unit test); pre-bump baseline captured instead
- [ ] Tests PASS (paste output) — appliance boot .ci + gokrazy deployment test green on the new init
- [ ] Boundary tests for all numeric inputs — N/A (no numeric inputs)
- [ ] Functional tests for end-to-end behavior — `make ze-vpp-hugepages-qemu-test` + gokrazy L2TP/PPP deployment (both unrun; AC-6 homed at `plan/spec-finish-appliance-qemu-evidence.md`)
- [ ] Interop tests for protocol features — N/A (no wire change)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-gokrazy-init-bump.md`
- [ ] **Commit A:** manifests + re-vendored source + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-gokrazy-init-bump.md`
