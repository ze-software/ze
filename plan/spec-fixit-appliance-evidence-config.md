# Spec: fixit-appliance-evidence-config

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/init/main.go` (`daemonRunning`, `runInit` active-config write), `scripts/evidence/effective-gokrazy-l2tp-ppp.py`, `mk/gokrazy.mk`

## Task

Two independent bugs in the appliance build/config flow, both surfaced by
`ze-deployment-gokrazy-l2tp-ppp-test` while verifying `spec-gokrazy-init-bump`
(AC-6) and `spec-iface-absent-link-graceful` (AC-3). They block a green gokrazy
L2TP appliance run on this host and are unrelated to the gokrazy bump or the
interface graceful-skip fix (both of which are done + proven).

**Bug 1 — `daemonRunning` false-positive vs a non-ze sshd.** `ze init --force`
refuses to replace the DB when a daemon is "running", but the check
(`daemonRunning`, `internal/plugins/init/main.go:419-437`) reads the DB's stored
SSH host:port and merely **dials TCP** to it — any listener answers. When the
appliance SSH is configured on `0.0.0.0:22`, the probe hits the **host's own
sshd** and false-reports a running daemon, aborting the fresh init. A build then
silently reuses a stale seed DB. (Worked around at the build layer in
`mk/gokrazy.mk` by deleting the seed DB before `ze init --force`; the real fix is
here.)

**Bug 2 — `ze init` active config shadows the appliance template.** `ze init`
writes discovered interfaces to the **active** config key
(`zefs.KeyFileActive.Key("ze.conf")`, `internal/plugins/init/main.go:279`). A
`GOKRAZY_TEMPLATE` provided at build time is written to a **separate** template
key (`file/template/ze.conf`) that the appliance only consults on a first boot
with no active config. So the template is **never applied**: the appliance boots
the init-written active config (build-host interfaces, SSH, DHCP) and any
template-only settings (web, l2tp) never take effect. Observed: the L2TP
appliance never starts its web server, so the evidence harness times out.

Not covered here: the gokrazy bump and the iface graceful-skip fix (separate, done).

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/init/main.go` - `daemonRunning` + `runInit`
  → Constraint: `daemonRunning` (`:419-437`) proves "running" only by dialing the stored SSH host:port; it must instead confirm a **ze** daemon (protocol handshake or a ze-specific marker), not any TCP listener.
  → Constraint: `runInit` writes discovered interfaces to `zefs.KeyFileActive.Key("ze.conf")` (`:279`) — the ACTIVE config, which shadows `file/template/ze.conf` on boot.
- [ ] `scripts/evidence/effective-gokrazy-l2tp-ppp.py` - the evidence harness
  → Constraint: `build_image` runs `make ze-gokrazy ... GOKRAZY_TEMPLATE=<l2tp cfg>`; the template must actually become the appliance's effective config for web/l2tp to start.
- [ ] `mk/gokrazy.mk` - the appliance build
  → Constraint: the credential step already deletes the stale seed DB (this spec's Bug-1 workaround); a real Bug-1 fix in `daemonRunning` lets that deletion be removed.

### RFC Summaries (MUST for protocol work)
- N/A — no protocol/wire change.

**Key insights:**
- Bug 1 and Bug 2 are independent; either alone breaks a repeatable gokrazy appliance evidence run.
- The interface graceful-skip fix already lets the appliance boot past a missing NIC; these two bugs are why web/l2tp still don't come up in the evidence test.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/init/main.go` - `daemonRunning` dials SSH host:port; `runInit` writes `KeyFileActive`.
  → Constraint: preserve the intent of the daemon guard (don't clobber a DB a live ze is using) while removing the non-ze false positive.

**Behavior to preserve:**
- `ze init` still refuses to replace a DB a live **ze** daemon is actively using.
- Normal (no-template) appliance builds still get a working discovered active config.

**Behavior to change:**
- `daemonRunning` must not treat an arbitrary TCP listener (e.g. host sshd) as a ze daemon.
- A build-time template must become the appliance's effective config (web/l2tp enabled).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Build: `make ze-gokrazy USER/PASS GOKRAZY_TEMPLATE=...` → `ze init` + `ze data write file/template/ze.conf`.
- Boot: appliance loads `file/active/ze.conf` (init-written) — template ignored.

### Transformation Path
1. `ze init --force` → `daemonRunning` (dials SSH port) → false positive → abort OR (post-workaround) fresh DB.
2. `runInit` → `EmitConfig(discovered)` → `WriteFile(KeyFileActive, ...)` (active config).
3. `ze data write file/template/ze.conf` → template file (separate key).
4. Appliance boot → uses active config → template never applied.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| build host ↔ seed DB | `ze init` / `ze data write` | [ ] |
| seed DB ↔ appliance boot | active vs template config keys | [ ] |

### Integration Points
- `daemonRunning`, `runInit` (`internal/plugins/init/main.go`).
- `mk/gokrazy.mk` template handling; `effective-gokrazy-l2tp-ppp.py` build.

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable (N/A)
- [ ] Registration over hardcoding (N/A)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A ze daemon can be distinguished from a generic TCP listener via a cheap probe (handshake/banner or a ze-specific PID/lock) | `daemonRunning` currently only dials | can't safely fix Bug 1 without a real ze liveness signal | prototype the probe against a live ze + a bare sshd | unvalidated |
| A-2 | Making the build-time template the effective (active) config yields web/l2tp on boot without breaking normal discovery | template has `dhcp-auto true`; the appliance discovers its own NIC at boot | web/l2tp still absent, or discovery regressions | rebuild L2TP image + boot in QEMU | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stricter `daemonRunning` misses a genuinely-running ze and clobbers a live DB | live ze DB replaced under it | require a positive ze-protocol response, default to "running" on ambiguity |
| R-2 | Template-as-active-config drops interface discovery the appliance needs | appliance has no usable NIC config | keep `dhcp-auto` in the template; merge discovery + template rather than replace |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze init --force` with a host sshd on the config'd SSH port | → | `daemonRunning` returns false (not a ze daemon) | `test/appliance/serial-login.ci` (appliance still builds/boots) + a new `daemonRunning` unit test |
| gokrazy image built with a template | → | appliance boots with the template's web/l2tp effective | `ze-deployment-gokrazy-l2tp-ppp-test` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `daemonRunning(db)` where the stored SSH port is served by a non-ze listener (e.g. host sshd) | Returns false; `ze init --force` proceeds. A genuinely-running ze still returns true. |
| AC-2 | Appliance image built with `GOKRAZY_TEMPLATE=<web+l2tp cfg>` | The appliance's effective boot config enables web + l2tp (template applied, not shadowed by the init active config). |
| AC-3 | `ze-deployment-gokrazy-l2tp-ppp-test` end to end | ze web server starts, L2TP listener binds, and a real xl2tpd/pppd session completes (unblocks gokrazy-init-bump AC-6 + iface-absent-link AC-3). Then the `mk/gokrazy.mk` seed-DB-delete workaround can be removed. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds + boots an appliance image with a config template | build → template becomes effective config → web/l2tp start | `ze-deployment-gokrazy-l2tp-ppp-test` |
| 2 | runs `ze init` on a host with sshd on :22 | daemonRunning correctly ignores non-ze sshd → fresh DB | `test/appliance/serial-login.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDaemonRunningIgnoresNonZeListener` | `internal/plugins/init/main_test.go` | AC-1: a non-ze TCP listener on the SSH port is not treated as a ze daemon | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/appliance/serial-login.ci` | `test/appliance/` | appliance still builds + boots to serial login after the daemonRunning fix | |

Integration proof (AC-2/AC-3): `ze-deployment-gokrazy-l2tp-ppp-test` — the appliance boots with the template's web/l2tp effective and completes an L2TP/PPP session.

### Interop Tests (MANDATORY for protocol features)
- N/A — no wire protocol change.

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/plugins/init/main.go` - `daemonRunning` (real ze liveness probe) + the template/active-config precedence.
- `mk/gokrazy.mk` and/or `scripts/evidence/effective-gokrazy-l2tp-ppp.py` - make a build-time template the effective config; remove the seed-DB-delete workaround once Bug 1 is fixed.
- `internal/plugins/init/main_test.go` - AC-1 unit test.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | N/A | no config surface change |
| YANG validation | N/A | — |
| YANG custom validators | N/A | — |
| CLI commands/flags | N/A | — |
| CLI grammar | N/A | — |
| Editor autocomplete | N/A | — |
| Functional test for new RPC/API | N/A | covered by serial-login.ci + the L2TP evidence test |
| Pipe completeness | N/A | — |
| Env var registration | N/A | — |
| Doctor check for runtime dependencies | N/A | no new runtime dependency |
| Prometheus counters/metrics | N/A | — |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | bug fixes |
| 2 | Config syntax changed? | No | — |
| 3 | CLI command added/changed? | No | — |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | No | — |
| 6 | Has a user guide page? | No | `docs/guide/appliance.md` (build flow) if template semantics change |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior? | No | — |
| 10 | Test infrastructure changed? | Yes | `mk/gokrazy.mk` comment + this test flow if the seed-DB workaround is removed |
| 11 | Affects daemon comparison? | No | — |
| 12 | Internal architecture changed? | Yes | note the init active-vs-template config precedence |
| 13 | Route metadata keys? | No | — |
| 14 | Prometheus counters? | No | — |
| 15 | Registered plugin/event/command/inventory changed? | No | — |
| 16 | Any changed source file referenced by doc source anchors? | Yes | grep `docs/` for `init/main.go` anchors |
| 17 | Existing docs show examples for this area? | No | — |

## Files to Create
- None (extends existing init + build flow).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify + assumptions |
| 3. Wiring phase | Wiring Test |
| 4. Implement | Phases below |
| 5. Full verification | unit + the L2TP evidence test |
| 6-9. Critical review | Critical Review Checklist |
| 10-12. Deliverables/security/docs | Checklists |
| 13. /ze-review gate | Review Gate |
| 14. Close | Two-commit closure |

### Implementation Phases
1. **Phase: daemonRunning real liveness** — replace the bare TCP dial with a ze-specific liveness probe; unit test AC-1.
2. **Phase: template precedence** — make a build-time template the appliance's effective config (merge template + boot-time discovery), so web/l2tp start.
3. **Phase: remove the workaround** — drop the `mk/gokrazy.mk` seed-DB delete once Bug 1 is fixed.
4. **Verification** — the gokrazy L2TP evidence test goes green.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1/2/3 demonstrated |
| Correctness | daemonRunning still catches a real ze; template applied on boot |
| Data flow | init config precedence correct; no discovery regression |
| Rule: no-workarounds | Bug 1 fixed at source; the Makefile workaround removed |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| daemonRunning fix | `go test ./internal/plugins/init/ -run DaemonRunning` |
| Template applied | L2TP appliance boots with web/l2tp |
| Evidence green | `ze-deployment-gokrazy-l2tp-ppp-test` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| DB clobber safety | a stricter daemonRunning must not replace a DB a live ze is using |
| Probe safety | the liveness probe must not act on untrusted responses |

### Failure Routing
| Failure | Route To |
|---------|----------|
| daemonRunning still false-positives | Phase 1 |
| template still shadowed | Phase 2 |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Delete the seed DB in mk/gokrazy.mk | unblocks the build but is a workaround for the daemonRunning false-positive | fix daemonRunning (Bug 1) then remove the delete |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

Surfaced while verifying `spec-gokrazy-init-bump` (AC-6) and
`spec-iface-absent-link-graceful` (AC-3) via `ze-deployment-gokrazy-l2tp-ppp-test`
on a Proxmox-style host (build host has `ens18`+`docker0`, guest NIC is `eth0`,
host sshd on `:22`). The interface graceful-skip fix let the appliance boot past
the missing `ens18`; these two config-flow bugs are why web/l2tp still don't come
up. See `plan/spec-iface-absent-link-graceful.md` Design Insights for the boot log.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| File both bugs as one fixit spec | one spec each | they share the init/build config flow and one integration proof |

## Known Limitations
- Until implemented, the gokrazy L2TP appliance evidence test cannot go fully green on a host with sshd on :22; the interface fix is verified only up to "appliance boots + interface config applied".

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| daemonRunning no longer false-positives on a non-ze listener | unit | `TestDaemonRunningIgnoresNonZeListener` |
| Build-time template becomes effective config | integration | L2TP appliance boots with web/l2tp |
| Full gokrazy L2TP evidence green | integration | `ze-deployment-gokrazy-l2tp-ppp-test` passes |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | | | |

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written — `TestDaemonRunningIgnoresNonZeListener`
- [ ] Tests FAIL (paste output) — before the fix
- [ ] Tests PASS (paste output) — after the fix
- [ ] Boundary tests for all numeric inputs — N/A
- [ ] Functional tests for end-to-end behavior — `test/appliance/serial-login.ci` + L2TP evidence
- [ ] Interop tests for protocol features — N/A
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-fixit-appliance-evidence-config.md`
- [ ] **Commit A:** init fix + build flow + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-fixit-appliance-evidence-config.md`
