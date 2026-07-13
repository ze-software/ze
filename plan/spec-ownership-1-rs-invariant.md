# Spec: ownership-1-rs-invariant

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ownership-0-umbrella |
| Phase | 6/6 |
| Updated | 2026-07-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec + `spec-ownership-0-umbrella.md`
2. `internal/component/bgp/reactor/forward_rs.go` (`reactorForwardRS`)
3. `internal/component/bgp/reactor/reactor_notify.go` (~586, the RSFastPath gate)
4. `internal/component/bgp/plugins/rs/{server_withdrawal.go,server_handlers.go,server_forward.go,register.go}`
5. `plan/learned/663-rs-gap-0-structural-forwarding.md` (why the reactor-native path exists)

## Task

Restore the project invariant "delete the plugin folder and the feature vanishes" for
**route-server (RS) forwarding**, WITHOUT regressing the reactor-native fast-path
performance that was deliberately built (`learned/663-rs-gap-0`). Today RS forwarding is
**split**: the reactor natively forwards RS UPDATEs on the hot path
(`reactorForwardRS`, `forward_rs.go:81-490`, gated only by the per-peer
`RSFastPath` setting at `reactor_notify.go:586`), while `internal/component/bgp/plugins/rs/`
owns the correctness-critical lifecycle (peer-down withdrawal map, replay-on-peer-up,
delivery to export-filtered peers). Deleting `plugins/rs/` would leave the hot forwarding
path alive in the reactor but silently break withdrawals, replay, and filtered-peer
delivery — and the enabling config leaves live in **core** BGP YANG, not the plugin.

**Goal:** make the reactor's RS fast path a capability that the `rs` plugin **owns and
activates** (via registration), so that with the plugin absent the fast path is inert and
no RS forwarding happens (fast or slow) — while, with the plugin present, the fast path
runs exactly as today. Bring the RS config surface under the plugin's ownership so its
removal also removes the config. Net: single ownership, invariant restored, zero
performance regression.

**OUT of scope:** the reactor-modes cleanup (P3); the Coordinator typing (P2); any change
to the RS forwarding *algorithm* or its perf characteristics.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `plan/learned/663-rs-gap-0-structural-forwarding.md` — the reactor-native RS forwarding perf design
  → Constraint: forwarding must stay on the source read goroutine with direct `bufWriter` writes and one `RetainN` per UPDATE; do not reintroduce per-peer retains, sync.Map hops, or string allocation on the forward path.
- [ ] `docs/architecture/core-design.md` (rs-gap-0 sections, ~570-640) — documented RS dispatch
  → Constraint: `reactorForwardRS` fires once per received UPDATE before caching+StructuredEvent dispatch; withdrawal extraction happens before forwarding while the cache buffer is alive.
- [ ] `ai/rules/plugin-self-containment.md` — the "delete the folder" invariant this spec restores
  → Constraint: no plugin spelling in generic/central packages; the enabling config + activation must be plugin-owned.
- [ ] `ai/rules/config-surface.md`, `ai/patterns/config-option.md` — for relocating/owning the `rs-fast-path`/`rs-client` leaves
  → Constraint: moving YANG leaves is a config-compat concern; preserve existing config acceptance or provide migration.

**Key insights:**
- The reactor code is fine; the defect is that it activates from a *core* per-peer setting with no dependency on the plugin's presence.
- Restoring the invariant = gating the fast path on plugin-owned registration + plugin-owned config, not deleting the reactor code.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_rs.go` — `reactorForwardRS` (81-490) iterates `r.peers`, writes directly to each dest peer's TCP `bufWriter` via `tryDirectWriteNoFlush` (28-70); returns `skipped` peers with `exportFilters` (111-114)
- [ ] `internal/component/bgp/reactor/reactor_notify.go` (586-592) — gate `kept && hasPeer && peer.settings.RSFastPath && msgType==UPDATE`; sets `msg.ReactorForwarded=true` (590) and `msg.FastPathSkipped=skipped` (592)
- [ ] `internal/component/bgp/types/rawmessage.go` (29-36) — `ReactorForwarded` / `FastPathSkipped` flags the plugin reads
- [ ] `internal/component/bgp/plugins/rs/server_withdrawal.go` (64-77) — `processForward` switches: not-handled → `batchForwardUpdate`; skipped>0 → `batchForwardUpdateSkipped`; else → `releaseCache`; ALWAYS updates withdrawal map (79-87)
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` (80-93 peer-down withdrawal; 149-183 replay-on-peer-up), `server_forward.go` (19-83 filtered-peer forward)
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` (751-757 `rs-fast-path`, 518-525 `rs-client`) — enabling config in **core** BGP YANG
- [ ] `internal/component/bgp/reactor/peersettings.go` (312-317 `RSFastPath`), `config.go` (218-221), `bgp/config/peers.go` (375-377) — where the setting is parsed
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` (309) — comment calls the RS plugin "(always loaded)", encoding the hidden dependency

**Behavior to preserve:**
- Exact fast-path forwarding performance and semantics (direct writes, RetainN, bucketing).
- Filtered-peer delivery, peer-down withdrawal, replay-on-peer-up (plugin lifecycle).
- Existing RS functional tests pass unchanged.

**Behavior to change:**
- Reactor fast path activates ONLY when the `rs` plugin has registered its forwarding capability; RS config surface becomes plugin-owned.

## Data Flow (MANDATORY)

### Entry Point
- Received UPDATE on a peer session read goroutine → `notifyMessageReceiver` → RS fast-path gate (`reactor_notify.go:586`).

### Transformation Path
1. `rs` plugin, at registration, activates the reactor RS-forwarding capability (new registration seam) and owns the `rs-fast-path`/`rs-client` config.
2. Reactor gate additionally requires the capability to be active (plugin present); absent → no native forwarding.
3. `reactorForwardRS` runs unchanged when active; sets `ReactorForwarded`/`FastPathSkipped`.
4. Plugin `processForward` handles skipped peers + withdrawal map + replay (unchanged).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| rs plugin ↔ reactor | new registration that activates the fast-path capability (no reactor→rs import) | [ ] |
| config ↔ rs feature | `rs-fast-path`/`rs-client` leaves owned by the rs plugin's YANG | [ ] |
| reactor hot path ↔ dest peers | direct `bufWriter` writes (unchanged) | [ ] |

### Integration Points
- `reactor_notify.go` gate, `forward_rs.go` (unchanged internals), `plugins/rs/register.go` (new activation), `plugins/rs/yang/` (config ownership), `reactor/peersettings.go` (RSFastPath sourced from plugin-owned config).

## Alternatives (decide at DESIGN gate)

- **B1 — Plugin-activated capability (recommended).** The reactor exposes an RS-forwarding
  capability that is inert unless a plugin activates it at registration; `rs` activates it.
  The gate at `reactor_notify.go:586` also checks activation. Deleting `plugins/rs/` → no
  activation → fast path inert → no RS forwarding. Reactor code + perf unchanged.
- **B2 — Move the whole fast path into the plugin.** Relocate `reactorForwardRS` into
  `plugins/rs/`. Cleanest ownership, but the plugin would need the reactor's peer/session
  internals and direct `bufWriter` access — likely regresses perf and pierces encapsulation.
  Rejected unless B1 proves infeasible.
- **B3 — Config-only ownership.** Move just the YANG leaves to the plugin; leave activation
  keyed on the setting. Weakest: the reactor code still runs if a stray config sets the
  (now plugin-owned) leaf via another path. Insufficient alone.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The reactor can gate the fast path on a plugin-activated capability without importing `plugins/rs` | reactor already reads `RSFastPath`; capability can be a bool/registration set via existing seams | Forces a reactor→rs import (tier violation) | design the activation via registry/`filterapi`-style seam; `make ze-tier-check` | **confirmed** — `filterapi` value seam; `ze-tier-check`/`ze-plugin-boundary-check` green |
| A-2 | Moving `rs-fast-path`/`rs-client` YANG leaves to the plugin preserves config acceptance | plugin YANG augments core (rs plugin already augments `route-server`) | Existing configs fail to parse | round-trip existing RS configs; interop configs audit | **corrected** — acceptance preserved (both parse tests pass unchanged), but "removal removes the surface" holds at COMPILE time only: the validation schema is a union of all registered modules, so the plugin block is not required at runtime (see Results) |
| A-3 | No non-rs consumer reads `RSFastPath` semantics | grep shows only reactor RS path + capability negotiation (session_negotiate.go:129) | Gating breaks another feature | grep all `RSFastPath` readers | **confirmed** — only the forward gate, PATHS-LIMIT suppression (session_negotiate.go:129), and dynamic-group propagation (reactor_dynamic.go:130) |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|-----------|
| R-1 | Gating adds a per-UPDATE check on the hot path | perf regression in `ze-perf` RS shape | make activation a single cached bool read, set once at startup |
| R-2 | Config relocation breaks interop fixtures | interop RS scenarios fail to load | audit `test/interop` + `test/plugin` RS configs; provide compat |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| RS peer receives UPDATE with rs plugin loaded | → | `reactorForwardRS` active via plugin capability | `bgp-rs-reactor-fastpath.ci` still green |
| rs plugin absent | → | fast path inert (no native RS forwarding) | `TestRSFastPathInertWithoutPlugin` (unit) |
| RS config present but plugin absent | → | config surface absent/inert | `TestRSConfigOwnedByPlugin` (unit/schema) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | rs plugin loaded, RS peers configured, UPDATE received | reactor fast path forwards exactly as today (perf + semantics unchanged); `bgp-rs-reactor-fastpath*.ci` pass |
| AC-2 | `plugins/rs/` not registered | `reactorForwardRS` is never invoked; no RS forwarding occurs (fast or slow) |
| AC-3 | `rs-fast-path`/`rs-client` config | owned by the rs plugin's YANG; removing the plugin removes the config surface |
| AC-4 | peer-down / peer-up / export-filtered peers with plugin loaded | withdrawal, replay, filtered-peer delivery unchanged |
| AC-5 | `ze-perf` RS benchmark shape | no measurable regression vs baseline in `test/perf/results/` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRSFastPathInertWithoutPlugin` | `internal/component/bgp/reactor/forward_rs_test.go` | AC-2 | |
| `TestRSFastPathActiveWithPlugin` | `internal/component/bgp/plugins/rs/server_test.go` | AC-1 | |
| `TestRSConfigOwnedByPlugin` | `internal/component/bgp/plugins/rs/` schema test | AC-3 | |

### Functional Tests
<!-- .ci functional tests exercising the full RS path. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-reactor-fastpath.ci` | `test/plugin/bgp-rs-reactor-fastpath.ci` | RS fast path forwards with plugin loaded | regression |
| `bgp-rs-reactor-fastpath-fallback.ci` | `test/plugin/bgp-rs-reactor-fastpath-fallback.ci` | export-filtered peers fall back to plugin forwarding | regression |
| `bgp-rs-fastpath-ebgp-shared.ci` | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | eBGP shared-path RS forwarding | regression |
| `bgp-rs-fastpath-ibgp-identity.ci` | `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` | iBGP identity forwarding | regression |
| `rs-plugin-absent-no-forward.ci` (new) | `test/plugin/` | with rs plugin unloaded, RS peers receive nothing | new |

## Files to Modify
- `internal/component/bgp/reactor/reactor_notify.go` — gate fast path on plugin-activated capability
- `internal/component/bgp/plugins/rs/register.go` — activate the capability at registration
- `internal/component/bgp/reactor/peersettings.go` / `config.go` — source `RSFastPath` from plugin-owned config
- `internal/component/bgp/yang/ze-bgp-conf.yang` + `plugins/rs/yang/` — relocate `rs-fast-path`/`rs-client` ownership
- `internal/component/bgp/reactor/peer_initial_sync.go` — remove the "(always loaded)" assumption comment/behavior

## Files to Create
- `internal/component/bgp/reactor/forward_rs_test.go` (if absent) — inert-without-plugin coverage
- `test/plugin/rs-plugin-absent-no-forward.ci` — invariant proof

## Implementation Steps

1. **Wiring:** add the reactor RS-forwarding capability + activation seam; failing wiring tests (AC-2).
2. **Activate from rs plugin** register.go; gate `reactor_notify.go` on activation.
3. **Relocate config ownership** of `rs-fast-path`/`rs-client` to the plugin; preserve acceptance.
4. **Regression:** all `bgp-rs-*fastpath*.ci` green; add invariant `.ci`.
5. **Perf check** against `test/perf/results/` (AC-5).
6. Audit tables + learned summary.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial) — two independent adversarial passes (full-diff, different lenses)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | augment-path coverage gap: nothing guarded that all six relocated `rs-*` augment paths survive | `plugins/rs/yang/ze-rs-conf.yang` | fixed: added `test/parse/bgp-rs-augment-paths.ci` (sensitivity-proven — dropping any augment → rejected unknown field) |
| 2 | NOTE | gate enforced only at the reactor `&&`; intent not documented for defense-in-depth | `reactor_notify.go:586` | acknowledged: defense-in-depth comment added |
| 3 | NOTE | new `filterapi` reactor-capability seam not cataloged | `ai/patterns/registration.md` | fixed: seam documented in the registration pattern catalog |

### Fixes applied
- **#1:** `test/parse/bgp-rs-augment-paths.ci` guards all six augment paths; proven to reject if any is dropped.
- **#2/#3:** clarity comment at the gate + `ai/patterns/registration.md` entry for the filterapi capability seam.

### Run 2 (closure re-review)
Second independent adversarial pass over the full diff (different lens) reached the same result: **0 BLOCKER, 0 ISSUE.**

### Final status
- Run 1: 1 ISSUE fixed (augment-path guard), 2 NOTEs addressed. Run 2: 0 BLOCKER / 0 ISSUE. **Review Gate satisfied** (recorded in Implementation Results → "ze-review convergence"). Pre-existing `ze-validate` unused-export findings in reactor.go/filterapi.go predate this change and are not addressed here.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 demonstrated
- [ ] Wiring Test rows all have concrete tests
- [ ] `make ze-test` passes (lint + all ze tests)

### TDD
- [x] Tests written
- [x] Tests FAIL (capability funcs undefined; gate/inert assertions red before impl)
- [x] Tests PASS (filterapi + reactor + rs unit green; 13 RS `.ci` green)

## Implementation Results (2026-07-04)

### Design chosen
B1 (plugin-activated capability) via the existing BGP-owned `filterapi` leaf seam,
mirroring how filter plugins register at `init()` (e.g. `bgp-gr`). The rs plugin's
`init()` calls `filterapi.EnableRSForwarding()`; the reactor caches
`filterapi.RSForwardingEnabled()` once in `New()` and the per-UPDATE gate adds a
single short-circuiting `&& r.rsForwardingEnabled`. No reactor→`plugins/rs` import;
no plugin name spelled in central code (a value, not a name). Config surface moved
by relocating the `rs-client` (session) and `rs-fast-path` (behavior) YANG leaves
into `plugins/rs/yang/ze-rs-conf.yang` as augments (3 session + 3 behavior paths,
mirroring `filter_irr`); the reactor keeps parsing them from the merged tree
because the fields are reactor-owned and hot-path-consumed.

### Files changed
- `internal/component/bgp/filterapi/filterapi.go` (+`EnableRSForwarding`/`RSForwardingEnabled`, snapshot/restore/reset)
- `internal/component/bgp/filterapi/filterapi_test.go` (capability tests)
- `internal/component/bgp/reactor/reactor.go` (field + cache in `New`)
- `internal/component/bgp/reactor/reactor_notify.go` (gate `&& r.rsForwardingEnabled`)
- `internal/component/bgp/reactor/forward_rs_test.go` (`TestNewReadsRSForwardingCapability`, `TestRSFastPathGateRespectsCapability`)
- `internal/component/bgp/reactor/peer_initial_sync.go` (drop "(always loaded)" assumption comment)
- `internal/component/bgp/plugins/rs/register.go` (activate capability at `init()`)
- `internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang` (adopt the two leaves as augments)
- `internal/component/bgp/plugins/rs/config_ownership_test.go` (AC-3 ownership guard)
- `internal/component/bgp/yang/ze-bgp-conf.yang` (remove the two leaves)
- `test/parse/bgp-rs-augment-paths.ci` (ze-review: guards all 6 augment paths; fails if any is dropped)
- `ai/patterns/registration.md` (ze-review: catalog the filterapi reactor-capability seam)

### ze-review convergence
Two independent adversarial passes (full-diff, different lenses) reached 0 BLOCKER / 0 ISSUE. Findings fixed: augment-path coverage gap → `bgp-rs-augment-paths.ci` (sensitivity-proven: dropping any augment → rejected unknown field); gate-only-enforcement clarity → defense-in-depth comment at `reactor_notify.go:586`; new-seam not cataloged → `ai/patterns/registration.md`. Pre-existing `ze-validate` unused-export findings in reactor.go/filterapi.go are out of scope (predate this change; surfaced only because the files are in the diff).

### AC evidence
- AC-1: `bgp-rs-reactor-fastpath{,-fallback}.ci`, `bgp-rs-fastpath-{ebgp-shared,ibgp-identity}.ci` PASS; `TestRSFastPathGateRespectsCapability/capability_active_forwards`.
- AC-2: `TestRSFastPathGateRespectsCapability/capability_inactive_inert`, `TestNewReadsRSForwardingCapability`, `filterapi.TestRSForwardingDefaultDisabled` (the reactor test binary does not link the rs plugin → capability false → gate inert). Structural: `filterapi.EnableRSForwarding` has exactly one caller, `plugins/rs/register.go`.
- AC-3: `TestRSConfigOwnedByPlugin` (leaves in `ze-rs-conf`, absent from `ze-bgp-conf`, wired via augment); live `ze config validate` accepts both leaves.
- AC-4: `rs-ipv4-withdrawal`, `rs-ipv6-processing`, `bgp-rs-replaying-gate`, `bgp-rs-mod-copy`, `plugin-rs-features`, `rs-backpressure`, `bgp-rs-asn4-transcode` PASS (withdrawal/replay/filtered-delivery lifecycle preserved).
- AC-5: gate adds one cached-bool `&&` first in the chain (no allocation, no map hop, short-circuits when off) → no measurable regression by construction. Full `ze-perf` RS-shape benchmark not run.
- Gates: `ze-tier-check`, `ze-plugin-boundary-check`, `golangci-lint` on changed packages all green; reactor `-race -count=3` on RS/delivery tests green.

### Deviations from spec / mapping
1. **The two parse tests were NOT updated** (`test/parse/bgp-rs-client-config.ci`, `bgp-dynamic-peer-group.ci`). The mapping premise that moving `rs-client` breaks them was wrong: `ze config validate` assembles its schema from ALL init()-registered YANG modules unconditionally (`config/yang_schema.go` → `LoadRegistered`), so a plugin augment is accepted whether or not the config declares the plugin. Both tests pass unchanged. (This flips A-2.)
2. **AC-3 "removing the plugin removes the config surface" is a COMPILE-time invariant** (delete the folder / drop the `plugin/all` blank import → the module's `init()` no longer registers → leaves vanish from the schema), not a runtime one. Symmetric with the compile-time capability activation. The planned runtime `rs-plugin-absent-no-forward.ci` was therefore NOT created (a standard `ze` binary always links the rs plugin; config-omission cannot make it inert). AC-2's absent-proof is the reactor unit test, per the spec's own Wiring Test table.
3. `forward_rs_test.go` already existed (spec "Files to Create" was stale); the inert test was added there. The AC-1 "active" case lives in the reactor gate test (faithful to the gate) rather than in `plugins/rs/server_test.go`.
