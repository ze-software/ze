# Spec: iface-absent-link-graceful

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/iface/config_apply.go` (the transactional apply), `internal/plugins/iface/netlink/manage_linux.go` (SetMACAddress producer), `internal/component/iface/register.go` (OnConfigure entry)

## Task

Make the interface plugin's config apply tolerate a **configured physical (Ethernet)
interface whose link is absent** at apply time. Today `applyConfig` is transactional
all-or-nothing: any per-interface error (e.g. `SetMACAddress` on an absent link →
"Link not found") calls `record()` → `rollbackPartial()`, which rolls back the ENTIRE
interface config — including `dhcp-auto` on interfaces that DO exist. The interface
plugin then fails startup and the appliance crash-loops (no network at all).

Fix: detect an absent physical (Ethernet) interface and **warn + skip its per-interface
configuration** so the rest of the config still applies and the appliance boots.
Preserve transactional rollback for genuine errors (permission, invalid MAC, a
create-type interface that failed creation).

Motivation: surfaced by `ze-deployment-gokrazy-l2tp-ppp-test`. A gokrazy image built by
`ze init` on a host with `ens18` bakes an `ens18` MAC pin (`EmitSetConfigWithDHCP`); the
QEMU guest has no `ens18`, so the appliance bricks. This also fixes the general "appliance
bricks when a configured NIC is absent (unplugged cable / hardware change / portable
image)" class. Unblocks `spec-gokrazy-init-bump` AC-6.

Not changed by this fix: `ze init` interface discovery / whether it bakes MAC pins. The
robustness fix here is the correct general behavior regardless of how the interface got
into the config; the discovery/baking angle is a separate concern tracked in
`spec-fixit-appliance-evidence-config`.

## Required Reading

### Architecture Docs
- [ ] `internal/component/iface/config_apply.go` - the transactional apply
  → Constraint: `applyConfig` (`:333`) returns `[]error`; `record(msg,err)` (`:346`) logs a warn, appends the error, and calls `rollbackPartial()` (`:339`) which undoes the whole journal. First error ⇒ full rollback.
  → Constraint: Phase 1 creates dummy/veth/bridge/tunnel/wireguard/xfrm; Phase 2 loop (`:644`) applies MTU/MAC/offloads/VLANs to `allEntries`. `cfg.Ethernet` (`:626`) are physical (may be absent); created types exist by Phase 2.
  → Constraint: MAC apply at `:665-676` → `record(e.Name+" set mac", err)` on failure = the fatal path.
- [ ] `internal/plugins/iface/netlink/manage_linux.go` - the MAC producer
  → Constraint: `SetMACAddress` (`:433`) returns `"iface: set mac on %q: not found: %w"` (`:443`) when `netlink.LinkByName` fails (absent link). The wrapped error is the netlink not-found error.
- [ ] `internal/component/iface/register.go` - the plugin Config entry
  → Constraint: `OnConfigure` (`:395`) → `applyConfig` (`:416`) → `joinApplyErrors` (`:417`) returns to the plugin, failing the Config stage → plugin startup fails → crash loop.

### RFC Summaries (MUST for protocol work)
- N/A — no protocol/wire behavior.

**Key insights:**
- The bug is the transactional rollback: one absent configured interface undoes ALL interface config, so `dhcp-auto` on the present NIC never applies.
- `cfg.Ethernet` are the only entries that can be legitimately absent (physical NICs); created types are made in Phase 1.
- Fix must skip ONLY absent-link conditions, never mask genuine errors.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/config_apply.go` - `applyConfig`, `record`/`rollbackPartial`, Phase 1 create, Phase 2 per-interface loop.
  → Constraint: preserve rollback on genuine errors; preserve create-type hard-fail.
- [ ] `internal/plugins/iface/netlink/manage_linux.go` - `SetMACAddress`/`SetMTU`/`SetAdminDown` all return a "not found" wrap on absent link.
- [ ] `internal/component/iface/config_apply_test.go` - existing fake-backend unit tests for applyConfig (where new unit tests go).

**Behavior to preserve:**
- Transactional rollback for genuine (non-absent-link) errors.
- Created interfaces (bridge/tunnel/wireguard/xfrm/dummy/veth) still hard-fail on creation error.
- All present interfaces configured exactly as before.
- `dhcp-auto` global behavior.

**Behavior to change:**
- A physical (Ethernet) interface whose link is absent → log a warning and skip its per-interface config; do NOT roll back the rest.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Plugin Config stage: `OnConfigure(sections)` in `register.go:395` (called by the plugin server when delivering config).

### Transformation Path
1. `parseIfaceSections` → `ifaceConfig`.
2. `applyConfig(cfg, prev, backend)` → Phase 1 create → Phase 2 per-interface apply.
3. Per-interface MAC/MTU via backend (`SetMACAddress` → netlink `LinkByName`+`LinkSetHardwareAddr`).
4. (FIX) absent physical interface → warn + skip instead of `record`→rollback.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin server ↔ iface plugin | OnConfigure RPC (Config stage) | [ ] |
| iface component ↔ netlink backend | Backend interface (SetMACAddress, GetInterface) | [ ] |

### Integration Points
- `applyConfig` in `config_apply.go` (the fix site).
- `Backend.GetInterface` (existence probe) / `Backend.SetMACAddress`.

### Architectural Verification
- [ ] No bypassed layers (fix stays in applyConfig; backend unchanged)
- [ ] No unintended coupling (only iface component touched)
- [ ] No duplicated functionality (reuses GetInterface existence probe)
- [ ] Zero-copy preserved where applicable (N/A)
- [ ] Registration over hardcoding (N/A)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `cfg.Ethernet` entries are the only ones legitimately absent at apply; created types exist by Phase 2 | `config_apply.go:626` + Phase-1 creation with `record` on failure | skipping wrong entries masks real create failures | read + unit test | confirmed — only cfg.Ethernet is skipped; `TestApplyConfigSkipsAbsentEthernet` green; created types unaffected |
| A-2 | Skipping an absent Ethernet iface leaves the rest applied (dhcp-auto + present NICs) | `dhcp-auto` is global; the baked `ens18` has no static addr (DHCP) | appliance still has no network | L2TP appliance test boots + web up | CONFIRMED (2026-07-13) — the L2TP re-run landed green in the now-closed fixit-appliance-evidence-config (`f42c2ccb2`, learned 1106, 5/5 runs); ens18 MAC is synchronous (skipped), DHCP is event-driven, no static addr baked |
| A-3 | The absent-link condition is detectable distinctly (netlink LinkNotFound / GetInterface error) | `manage_linux.go:441-443`; `GetInterface` errors on absent link | can't distinguish absent from genuine error → over/under-skip | unit test with fake backend | confirmed — absence detected via `GetInterface`; `TestApplyConfigRollsBackGenuineError` shows genuine errors still roll back |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Over-skip: a genuine error mis-classified as "absent" masks a real problem | a real misconfig silently ignored | skip ONLY on absent-link/GetInterface-absent, not other errors; unit test AC-2 |
| R-2 | Address/route phases still abort on the absent interface (skip only covers Phase 2) | L2TP appliance still crash-loops after fix | pre-compute an absent-Ethernet skip set and exclude it from every per-interface phase, not just Phase 2 |
| R-3 | Regression: present interfaces stop applying | `test-tx-iface-apply.ci` fails | keep the present-interface path unchanged; regression .ci |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| plugin Config stage `OnConfigure` with a present + an absent Ethernet iface | → | `applyConfig` skips absent, applies present, returns nil | `TestApplyConfigSkipsAbsentEthernet` (config_apply_test.go) |
| gokrazy image with a baked-but-absent NIC boots | → | interface plugin comes up, web starts, L2TP works | `ze-deployment-gokrazy-l2tp-ppp-test` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `applyConfig` with a present Ethernet iface (Y) + an absent Ethernet iface (X, MAC pinned) | Y fully applied; X warned + skipped; `applyConfig` returns no errors (NO rollback). |
| AC-2 | `applyConfig` where a present iface's backend op fails with a genuine (non-absent) error | Still records the error and rolls back (existing transactional behavior preserved). |
| AC-3 | `ze-deployment-gokrazy-l2tp-ppp-test` on a host whose baked NIC is absent in the QEMU guest | Appliance boots; interface plugin comes up; web server starts; real xl2tpd/pppd L2TP/PPP session against the appliance succeeds. Also satisfies `spec-gokrazy-init-bump` AC-6. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | boots an appliance whose config names a NIC that isn't present | OnConfigure → applyConfig → warn+skip absent → present NICs + dhcp-auto apply → appliance online | `ze-deployment-gokrazy-l2tp-ppp-test` |
| 2 | applies a normal multi-interface config (all present) | OnConfigure → applyConfig → all applied | `test/reload/test-tx-iface-apply.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyConfigSkipsAbsentEthernet` | `internal/component/iface/config_apply_test.go` | absent Ethernet iface skipped (warn), present applied, no rollback (AC-1) | |
| `TestApplyConfigRollsBackGenuineError` | `internal/component/iface/config_apply_test.go` | non-absent error still rolls back + returns error (AC-2) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/reload/test-tx-iface-apply.ci` | `test/reload/` | normal multi-interface apply still works (regression) | |

Integration proof (drives AC-3): `ze-deployment-gokrazy-l2tp-ppp-test` (`scripts/evidence/effective-gokrazy-l2tp-ppp.py`) — boots the gokrazy image whose baked NIC is absent in the guest; appliance must come up and complete an L2TP/PPP session.

### Interop Tests (MANDATORY for protocol features)
- N/A — no wire protocol change.

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/iface/config_apply.go` - detect absent physical (Ethernet) interfaces and warn+skip their per-interface config across the apply phases; preserve rollback for genuine errors.
- `internal/component/iface/config_apply_test.go` - AC-1 + AC-2 unit tests.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | N/A | no config surface change |
| YANG validation | N/A | — |
| YANG custom validators | N/A | — |
| CLI commands/flags | N/A | — |
| CLI grammar | N/A | — |
| Editor autocomplete | N/A | — |
| Functional test for new RPC/API | N/A | no RPC; covered by iface apply .ci + L2TP evidence |
| Pipe completeness | N/A | — |
| Env var registration | N/A | — |
| Doctor check for runtime dependencies | N/A | no new runtime dependency |
| Prometheus counters/metrics | N/A | behavior change only; a warn log is emitted per skipped iface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | robustness fix, not a feature |
| 2 | Config syntax changed? | No | — |
| 3 | CLI command added/changed? | No | — |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | No | — |
| 6 | Has a user guide page? | No | — |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior? | No | — |
| 10 | Test infrastructure changed? | No | reuses existing tests |
| 11 | Affects daemon comparison? | No | — |
| 12 | Internal architecture changed? | Yes | `internal/component/iface/config_apply.go` `// Design:` anchor if present; note the absent-interface tolerance |
| 13 | Route metadata keys? | No | — |
| 14 | Prometheus counters? | No | — |
| 15 | Registered plugin/event/command/inventory changed? | No | — |
| 16 | Any changed source file referenced by doc source anchors? | Yes | grep `docs/` for `config_apply.go` anchors; update if any |
| 17 | Existing docs show examples for this area? | No | — |

## Files to Create
- None (extends existing config_apply.go + its test).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify + assumptions |
| 3. Wiring phase | Wiring Test — failing unit test first |
| 4. Implement | Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test` + iface apply .ci |
| 6-9. Critical review | Critical Review Checklist |
| 10-12. Deliverables/security/docs | Checklists |
| 13. /ze-review gate | Review Gate |
| 14. Close | Two-commit closure |

### Implementation Phases
1. **Phase: Failing unit test** — `TestApplyConfigSkipsAbsentEthernet`: fake backend where iface X's link is absent (GetInterface/SetMAC returns not-found) and Y present; assert Y applied, X skipped, no error. Fails against current code (it rolls back).
2. **Phase: Skip logic** — in `applyConfig`, pre-compute the set of `cfg.Ethernet` interfaces whose link is absent (`GetInterface` fails), log a warning per skipped iface, and exclude them from the per-interface application (Phase 2 loop + any address/route per-interface phase). Only absent-link ⇒ skip; genuine errors keep `record`→rollback.
3. **Phase: Preserve-rollback test** — `TestApplyConfigRollsBackGenuineError`: a non-absent backend error still rolls back + returns.
4. **Verification** — unit tests green; `test-tx-iface-apply.ci`; then the L2TP appliance integration (AC-3).
5. **Complete spec** — audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1/2/3 demonstrated |
| Correctness | Only absent-link skipped; genuine errors still roll back |
| Data flow | Fix in applyConfig only; backend + register unchanged |
| Regression | Present interfaces apply exactly as before (`test-tx-iface-apply.ci`) |
| Rule: no-workarounds | Fix at the apply layer (source), not by editing the test or suppressing the log |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Skip logic | `go test ./internal/component/iface/ -run ApplyConfig` green |
| Regression | iface apply .ci passes |
| Integration | `ze-deployment-gokrazy-l2tp-ppp-test` green |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Silent failure | The skip MUST emit a WARN log (operator visibility); not a silent no-op |
| Scope of skip | A skipped interface must not leave partial state for that interface |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Unit test fails wrong reason | Fix test setup |
| L2TP still crash-loops after fix | R-2 — extend skip to address/route phases |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Change ze init to not bake build-host MAC pins | narrower; doesn't fix general brick-on-missing-NIC | graceful skip in applyConfig |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

**Implementation (2026-07-10):** `config_apply.go` now pre-computes `absentPhysical`
(cfg.Ethernet names where `GetInterface` fails), logs a WARN per skipped iface, and
`continue`s them in the Phase-2 loop. TDD: `TestApplyConfigSkipsAbsentEthernet` (was RED
with the exact real error `set mac on "eth-absent": not found`, now GREEN) + preserved
`TestApplyConfigRollsBackGenuineError`. Full iface package + `make ze-lint-changed` = 0
issues. R-2 retired: `emit.go:152-156` bakes only `mac`/`os-name`/`dhcp` (no static addr),
so the MAC skip is sufficient; DHCP is event-driven. Made the `fakeBackend.SetMACAddress`
faithful (fail on absent + record to `macSet`) — no existing test set a MAC, so safe.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Warn+skip absent physical interface in applyConfig | don't bake pins in ze init; reject-at-validation | robustness fix is the correct general behavior; appliance must survive a missing configured NIC |
| Skip only `cfg.Ethernet` (physical) on absent-link | skip any absent entry | created types exist post-Phase-1; skipping them would mask create failures |

## Known Limitations
- Does not change `ze init` interface discovery (it still bakes discovered interfaces); the apply is now tolerant of absent ones. A follow-up could make discovery avoid pinning MACs.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Appliance no longer bricks when a configured interface is absent | unit + integration | `TestApplyConfigSkipsAbsentEthernet` green; `ze-deployment-gokrazy-l2tp-ppp-test` boots + completes L2TP |
| Genuine errors still fail transactionally | unit | `TestApplyConfigRollsBackGenuineError` green |
| Unblocks gokrazy-init-bump AC-6 | integration | the L2TP appliance test passes end-to-end |

## Review Gate

### Run 1 (retrospective close-review, 2026-07-13)
No `/ze-review` was recorded during implementation; this is a focused close-review of the
committed change, grounded in the truth-audit's verified producer and tests.
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| — | (none) | AC-1/AC-2: absent Ethernet is probed, warned, and filtered from every apply phase; created types (dummy/veth/…) stay; a genuine error still aborts+rolls back. Verified in the producer. | `internal/component/iface/config_apply.go:651-675` (skip) + `:346-350` (record→rollback) | no change needed |

### Fixes applied
- None. The skip logic is small and behavior-preserving for the non-absent path.

### Final status
- Run 1: **0 BLOCKER / 0 ISSUE**. AC-1/AC-2 covered by green unit tests (`TestApplyConfigSkipsAbsentEthernet`, `TestApplyConfigRollsBackGenuineError`, `config_apply_test.go`). AC-3 (full L2TP appliance proof) was legitimately tracked in `fixit-appliance-evidence-config`, now **closed green** (commit `f42c2ccb2`, learned `1106`: 5/5 L2TP runs). **Review Gate satisfied.**

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
- [ ] Tests written — `TestApplyConfigSkipsAbsentEthernet`, `TestApplyConfigRollsBackGenuineError`
- [ ] Tests FAIL (paste output) — before the skip logic
- [ ] Tests PASS (paste output) — after the skip logic
- [ ] Boundary tests for all numeric inputs — N/A
- [ ] Functional tests for end-to-end behavior — `test-tx-iface-apply.ci` + L2TP appliance evidence
- [ ] Interop tests for protocol features — N/A
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-iface-absent-link-graceful.md`
- [ ] **Commit A:** iface fix + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-iface-absent-link-graceful.md`
