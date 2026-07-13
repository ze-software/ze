# Spec: ownership-3-reactor-modes

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ownership-0-umbrella |
| Phase | 6/6 |
| Updated | 2026-07-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec + `spec-ownership-0-umbrella.md`
2. `internal/component/bgp/reactor/reactor.go` — `startAPIServer` (~1105), `externalServer` (309), `New()` (331)
3. `internal/component/bgp/config/loader_create.go` / `loader.go:107` (`LoadReactorWithPlugins`)
4. `internal/chaos/inprocess/runner.go` (standalone reactor consumer)
5. `internal/component/bgp/plugin/register.go` (~120-180, production injection)

## Task

Make the BGP reactor's ownership of the plugin `Server` an **explicit, constructed
mode** instead of one inferred at runtime from `r.api != nil` (`reactor.go:1112`).
Today `New()` returns a reactor that can go either way: in production the hub injects
its server (`SetPluginServerAny`) so `startAPIServer` borrows it; when nothing is
injected the reactor self-hosts (creates its own server, runs its own signal handler,
waits for plugin startup, validates families inline, starts peers inline). The
self-hosting regime is a legitimate, shipped mode — the **ze-chaos in-process
simulation** (`internal/chaos/inprocess/runner.go`) and the **integration test harness**
(`test/integration/integration_test.go`) — not vestigial cruft.

**Goal:** production construction is **borrow-only** (never silently self-hosts; starting
without an injected server is a clear error), and self-hosting is selected explicitly
(a `Config.Standalone` flag or a distinct `NewStandalone` constructor) by the simulation
and integration callers. `externalServer` becomes derived from the explicit mode, not
inferred from `r.api != nil`. No behavior change for either real path — only the
selection becomes intentional.

**OUT of scope:** removing the self-hosting mode (it is a real feature); the RS invariant
(P1); the Coordinator typing (P2); the `APIServer()`/`Dispatcher()` accessors (they
forward to `r.api` and stay).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `plan/learned/421-arch-9-plugin-manager-wiring.md` — two-phase startup rationale (Server created inside reactor; plugins start before subsystems)
  → Constraint: in the borrow path, peers are deferred to `StartPeers()` via a post-plugin-startup callback (register.go:173). Preserve this ordering.
- [ ] `plan/learned/533-bgp-boundary-cleanup.md` — hub has zero bgp imports
  → Constraint: the standalone flag must be threaded through the bgp loader, NOT by a hub→bgp import; `*Any` setters stay for cross-package injection.
- [ ] `docs/architecture/chaos-web-dashboard.md` — in-process chaos runner design
  → Constraint: the in-process sim wants ONE self-contained reactor (virtual clock, mock net); standalone mode must keep self-hosting the server, signals off (sim owns lifecycle), and inline peer start.

**Key insights:**
- Production is already borrow-only in effect; only the *selection* is implicit.
- `LoadReactorWithPlugins` (loader.go:107) is shared by production and ze-chaos — the standalone choice must be threaded through it, not hard-coded.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor.go` — `New()` seeds `pluginServerMaker = pluginserver.NewServer` (361); `startAPIServer` sets `externalServer = r.api != nil` (1112) and self-hosts in the `!externalServer` branch (1114-1145, StartWithContext 1159-1163); `!externalServer` also gates own signal handler (984), plugin-startup waits (1001), inline `validatePeerFamilies` (1022), abort cleanup guard (1225); `StartPeers()` (554) validates families (560) then starts peers — the production post-startup path
- [ ] `cmd/ze/hub/main.go` (441,453,502,868) — hub owns server + signals in production
- [ ] `internal/component/bgp/plugin/register.go` (144,149,172-178) — injects hub infra, registers deferred `StartPeers`
- [ ] `internal/component/bgp/config/loader.go:107` `LoadReactorWithPlugins` → `loader_create.go:187` `reactor.New` — shared construction path (production + ze-chaos)
- [ ] `internal/chaos/inprocess/runner.go` (150,176,182) — builds reactor via LoadReactorWithPlugins, sets ProcessSpawner but NOT a server → self-hosts; comment "creates listeners, starts API, starts peers"
- [ ] `test/integration/integration_test.go` (78-98) — `setupPeers` runs two self-hosting reactors peering over 127.0.0.1
- [ ] `internal/component/bgp/reactor/reactor_startup_test.go` (31,90,182) — override `pluginServerMaker` to inject server-creation failure for abort/cleanup coverage

**Behavior to preserve:**
- Production borrow-path startup ordering + deferred `StartPeers`.
- Standalone self-hosting behavior (server, no dup signals in sim, plugin waits, inline peers) for ze-chaos + integration.
- Startup-abort/cleanup guarantees (listener, cache scanner, context) — including the server-creation-failure trigger, which remains reachable in standalone mode.
- `make ze-plugin-boundary-check`, `make ze-tier-check` green.

**Behavior to change:**
- Selection of self-host vs borrow becomes explicit at construction; production borrow path errors (not self-hosts) if no server is injected before start.

## Data Flow (MANDATORY)

### Entry Point
- Production: `createReactorFromCoordinator` → `reactor.New(cfg)` (borrow mode) → `SetPluginServerAny` injects hub server → `StartWithContext`.
- Simulation/tests: explicit `Config.Standalone=true` / `NewStandalone(cfg)` → `StartWithContext` self-hosts.

### Transformation Path
1. Add explicit mode to reactor construction (`Config.Standalone` or `NewStandalone`).
2. `startAPIServer` branches on the explicit mode, not `r.api != nil`; borrow mode asserts `r.api != nil` and errors otherwise.
3. Thread the standalone choice through `LoadReactorWithPlugins` so ze-chaos selects it while production does not.
4. Update ze-chaos + integration callers to request standalone; migrate the abort tests to standalone construction.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| hub ↔ reactor | borrow via `SetPluginServerAny` (unchanged) | [ ] |
| bgp loader ↔ reactor | new explicit `Standalone` field on reactor `Config` | [ ] |
| chaos ↔ reactor | LoadReactorWithPlugins standalone param | [ ] |

### Integration Points
- `reactor.New`/`Config`, `startAPIServer`, `LoadReactorWithPlugins`, `chaos/inprocess/runner.go`, integration tests, reactor_startup_test.go.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The standalone regime has exactly two non-production consumers: ze-chaos in-process + integration harness (plus unit tests) | grep of `LoadReactorWithPlugins`/`reactor.New` non-test callers | A missed consumer breaks | re-grep at implement time | confirmed (umbrella research) |
| A-2 | Production family validation + signals do NOT depend on the `!externalServer` inline branches | validatePeerFamilies also in StartPeers:560 (prod); hub owns signals (main.go:868) | Removing inline branches drops prod behavior | verified in umbrella research | confirmed |
| A-3 | `LoadReactorWithPlugins` can carry a standalone flag without a hub→bgp import | it lives in bgp/config; hub never calls it | Threading forces a bad import | trace callers; keep flag inside bgp package | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|-----------|
| R-1 | Test migration changes startup ordering, masking/introducing races | reactor/integration tests flake under `-race` | mirror production ordering in standalone; run `-race` |
| R-2 | Borrow-path "error if no server" fires on a legit path that relied on implicit self-host | a real caller errors at start | audit every `reactor.New`/LoadReactorWithPlugins caller before flipping the default |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| production reactor built borrow-only, server injected, started | → | `startAPIServer` borrow branch | `TestReactorBorrowModeRequiresInjectedServer` |
| ze-chaos in-process sim runs a standalone reactor end to end | → | `Config.Standalone` self-host branch | `test/plugin/…` chaos sim scenario or existing chaos e2e |
| borrow mode without injected server errors clearly | → | `startAPIServer` borrow guard | `TestReactorBorrowModeErrorsWithoutServer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | reactor constructed in production (borrow) mode, `StartWithContext` called with no server injected | returns a clear error; never calls `pluginserver.NewServer` |
| AC-2 | reactor constructed with explicit standalone mode | self-hosts server + signals + plugin waits + inline peers exactly as the current `!externalServer` path |
| AC-3 | `externalServer` (or its replacement) | derived from the explicit constructed mode, not from `r.api != nil` |
| AC-4 | ze-chaos in-process sim + integration harness | pass using the explicit standalone constructor |
| AC-5 | server-creation failure in standalone mode | aborts startup and releases listener, cache scanner, and context (existing guarantee) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReactorBorrowModeErrorsWithoutServer` | `internal/component/bgp/reactor/reactor_startup_test.go` | AC-1 | |
| `TestReactorStandaloneSelfHosts` | `internal/component/bgp/reactor/reactor_startup_test.go` | AC-2 | |
| `TestExternalServerDerivedFromMode` | `internal/component/bgp/reactor/reactor_startup_test.go` | AC-3 | |
| (migrate) `TestStartWithContextCleansUpAfterAPIServerFailure` | same | AC-5 under standalone | |

### Functional Tests
<!-- .ci files that must still pass — prove the production (borrow) reactor path is unchanged. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-reactor-fastpath.ci` | `test/plugin/bgp-rs-reactor-fastpath.ci` | production reactor borrow path still forwards RS UPDATEs end to end | regression |
| `bgp-rs-reactor-fastpath-fallback.ci` | `test/plugin/bgp-rs-reactor-fastpath-fallback.ci` | borrow-path fallback to plugin forwarding still works | regression |
| (Go, not .ci) integration peering | `test/integration/integration_test.go` | two standalone reactors still peer after migration to explicit standalone mode | regression |

## Files to Modify
- `internal/component/bgp/reactor/reactor.go` — explicit mode; `startAPIServer` branch on mode; borrow guard
- `internal/component/bgp/config/loader.go`, `loader_create.go` — thread standalone flag
- `internal/chaos/inprocess/runner.go` — request standalone
- `test/integration/integration_test.go`, `internal/component/bgp/reactor/reactor_startup_test.go`, `cmd/ze/hub/infra_setup_test.go` — explicit standalone construction

## Files to Create
- (none expected; a small shared standalone-reactor test helper if useful)

## Implementation Steps

1. **Wiring:** add `Config.Standalone` (or `NewStandalone`); make `startAPIServer` branch on it; add borrow-mode guard + failing wiring tests.
2. **Thread the flag** through `LoadReactorWithPlugins`; production stays borrow, ze-chaos passes standalone.
3. **Migrate callers/tests** (chaos, integration, unit abort tests) to explicit standalone.
4. **Remove the `r.api != nil` inference**; derive `externalServer` from mode.
5. Functional/e2e verification; `make ze-verify`.
6. Complete audit tables + learned summary.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial) — independent adversarial pass (6 axes)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE (LOW, test-strength) | borrow-mode test asserted the error but not that the listener was released | `reactor_startup_test.go` (borrow test) | fixed: added `ListenAddr()==nil` / `ListenAddrs()` empty assertions |

### Fixes applied
- **#1:** `TestReactorBorrowModeErrorsWithoutServer` now asserts the listener is released on abort (`ListenAddr()==nil`), closing the test-strength gap.

### Run 2 (completeness verification)
Independent completeness pass: every reactor-start caller repo-wide classified (borrow-with-server / standalone / never-started); no missed borrow-without-server caller. **0 BLOCKER / 0 ISSUE.**

### Final status
- Run 1: 1 LOW test-strength ISSUE fixed. Run 2: completeness verified, **0 BLOCKER / 0 ISSUE**. **Review Gate satisfied** (recorded in Implementation Results → "ze-review"). `ze-tier-check` / `ze-plugin-boundary-check` / `golangci-lint` (0 issues) green; reactor `-race` green.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 demonstrated
- [ ] Wiring Test rows all have concrete tests
- [ ] `make ze-test` passes (lint + all ze tests)

### TDD
- [x] Tests written (AC-1/2/3 + 38 self-hosting tests migrated to Standalone)
- [x] Tests FAIL (before migration: 37 reactor tests errored with errBorrowModeNoServer)
- [x] Tests PASS (reactor + config + integration + chaos + hub all green)

## Implementation Results (2026-07-05)

### Assumptions resolved
- **A-1 corrected** — NOT two non-production self-hosting consumers but THREE: ze-chaos in-process, the integration harness (4 reactor.New sites), and `ze bgp --child` (childmode.go). The reactor's OWN unit suite also self-hosts pervasively: 38 tests needed explicit `Standalone: true` (blueprint estimated ~3). `ze bgp --child` is live/spawnable, so migrating it was required, not optional.
- **A-2 confirmed** — production borrow path unchanged: validatePeerFamilies runs in StartPeers; hub owns signals; the six `!externalServer` gates were logic-unchanged (only the source of externalServer moved to construction).
- **A-3 confirmed** — the `standalone` flag threads entirely inside `bgpconfig`; no hub->bgp import (`ze-tier-check`/`ze-plugin-boundary-check` green).

### AC evidence
- AC-1: `TestReactorBorrowModeErrorsWithoutServer` (borrow + no server -> errBorrowModeNoServer; pluginServerMaker never called; listener released on abort).
- AC-2: `TestReactorStandaloneSelfHosts` (standalone reaches the server maker) + the 4 migrated maker-override abort tests.
- AC-3: `TestExternalServerDerivedFromMode` (externalServer == !Standalone for both modes).
- AC-4: `test/integration` (2 standalone reactors peer) + `internal/chaos/inprocess` (in-process sim) green with the explicit standalone loaders.
- AC-5: production borrow path unchanged -- `cmd/ze/hub` tests green; functional `bgp-rs-reactor-fastpath{,-fallback}.ci` + `bgp-rs-fastpath-ebgp-shared.ci` pass end-to-end through the hub-injected-server borrow path; `go vet` clean; reactor `-race` green (R-1: startup ordering unchanged, only mode selection moved to construction).
- Gates: `ze-tier-check`, `ze-plugin-boundary-check`, `golangci-lint` (0 issues) green.

### Design chosen
`Config.Standalone bool` (default borrow) over an inverted `Borrow` flag: the default must be production-safe, so a caller that forgets the mode gets borrow (errors loudly without a server) rather than silently self-hosting -- the exact invariant the spec restores. `externalServer` derived once in New; runtime inference replaced by a borrow guard.

### ze-review
Two independent adversarial passes. First: 5/6 axes converged, 1 LOW test-strength finding (borrow test didn't assert listener release) -> fixed by adding `ListenAddr()==nil`/`ListenAddrs()` empty assertions. Completeness axis verified: every reactor-start caller repo-wide classified (borrow-with-server / standalone / never-started); no missed borrow-without-server caller.
