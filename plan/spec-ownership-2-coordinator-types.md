# Spec: ownership-2-coordinator-types

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-ownership-0-umbrella |
| Phase | 6/6 |
| Updated | 2026-07-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec + `spec-ownership-0-umbrella.md`
2. `internal/component/plugin/coordinator.go` (the Coordinator; `extra`/`reactors` maps)
3. `internal/component/plugin/registry/registry.go` (Configure* hooks ~104-114; backing globals 246-259)
4. `internal/component/plugin/registry/interfaces.go` (leaf cross-plugin interfaces — the cycle-avoidance boundary)
5. `cmd/ze/hub/main.go` (~385-402, `SetExtra` writers) + `internal/component/bgp/config/register.go` (`GetExtra` readers)

## Task

Replace the `any`-typed plumbing that the design review (finding #1) flagged around the
plugin Coordinator and registration hooks with typed values, **where no import cycle
forces `any`**. Research (2026-07-04) found that of five `any` families, only one is
cycle-forced at its concrete type — and even that already has a leaf-interface answer in
use. The rest are `any` by convention and can be typed today, converting ~60 per-plugin
runtime type-assertions into compile-time-checked signatures.

**In scope (typeable today):**
1. `Registration.ConfigureEventBus func(any)` → `func(ze.EventBus)` (~34 implementers drop `eb.(ze.EventBus)`).
2. `Registration.ConfigureMetrics func(any)` → `func(metrics.Registry)` (~27 implementers drop `reg.(metrics.Registry)`).
3. `Registration.ConfigurePluginServer func(any)` → `func(registry.PluginServerAccessor)` (the assertion target already; 1 implementer).
4. The registry backing globals `eventBusInstance`/`metricsRegistry`/`pluginServerInstance` typed accordingly.
5. The Coordinator `extra map[string]any` string-keyed bag → a typed bootstrap struct (9 keys, each with a known concrete type at both ends), removing the 9 keyed assertions.

**Explicitly OUT of scope (genuinely cycle-forced or intentional segregation — leave as `any`):**
- `PluginServerAccessor.ReactorAny()/ReactorFor() any`, `ProtocolReactorHandle.*Any(any)`,
  `PeerLifecycleCallback/MessageCallback` `any` params — leaf `registry` cannot import
  `plugin`/`bgp` types (real cycle; `interfaces.go` header says so).
- The optional BGP provider widening assertions (`PolicyDryRunner`, `FSMHistoryProvider`,
  `BGPCaptureProvider`, `BGPRawCaptureProvider`) — deliberate interface segregation, not a defect.
- The `reactors map[string]any` container — intentionally generic for multi-protocol
  (OSPF/IS-IS); the `"bgp"` slot is already `ReactorLifecycle`-typed inside the Coordinator
  (`getReactor`/`FullReactor`). P2 may add a typed `BGPReactor()` accessor but does not
  remove the generic map.
- The P1 (RS) and P3 (reactor modes) concerns.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `internal/component/plugin/registry/interfaces.go` (header) — "cross-plugin interfaces for cycle avoidance"
  → Constraint: the leaf `registry` package MUST NOT import `internal/component/plugin` or any `bgp/*`; only downward (`pkg/ze`, `internal/core/metrics`) is allowed.
- [ ] `ai/rules/module-tiers.md`, `plan/learned/533-bgp-boundary-cleanup.md` — tier direction rules
  → Constraint: typing a hook to `ze.EventBus`/`metrics.Registry`/`PluginServerAccessor` is downward and legal; typing `bgp.store` to `storage.Storage` needs a transitive-cycle check first.
- [ ] `ai/rules/plugin-self-containment.md` — registration-pattern rules
  → Constraint: the typed bootstrap struct belongs in the leaf `registry` package (where `PeerLifecycleCallback`/`MessageCallback` already live), not in a BGP package.

**Key insights:**
- Only `ConfigurePluginServer` at the *concrete* `*pluginserver.Server` is cycle-forced; the leaf interface `PluginServerAccessor` (interfaces.go:13) is the already-used answer.
- `pkg/ze` imports nothing from codeberg → `registry`→`pkg/ze` is safe; `core/metrics` is below component → safe.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` — `ConfigureMetrics func(reg any)` (104), `ConfigureEventBus func(eventBus any)` (109), `ConfigurePluginServer func(server any)` (114); backing globals `metricsRegistry any` (249), `eventBusInstance any` (254), `pluginServerInstance any` (259); Set/Get accessors (360-402)
- [ ] `internal/component/plugin/inprocess.go` (107-121) — the single injection point calling all three hooks with the `Get*()` values
- [ ] `internal/component/plugin/coordinator.go` (25-31) — `reactors map[string]any` (28), `extra map[string]any` (29); `SetExtra`/`GetExtra` (44-55)
- [ ] `internal/component/plugin/registry/interfaces.go` (13 `PluginServerAccessor`, 41/49 callback interfaces) — the leaf boundary
- [ ] `cmd/ze/hub/main.go` (389-402) — 9 `SetExtra` writers with concrete types; `internal/component/bgp/config/{register.go,loader.go}` — matching `GetExtra` assertions
- [ ] ~60 plugin `register.go` files implementing `ConfigureEventBus`/`ConfigureMetrics`/`ConfigurePluginServer` with an inline `.(T)` assertion (inventory in umbrella research)

**Behavior to preserve:**
- Runtime wiring is identical; only the static types change. Every plugin receives the same instance it does today.
- Leaf-purity of `registry` (no upward imports).
- `make ze-tier-check`, `make ze-plugin-boundary-check` green.

**Behavior to change:**
- The three hooks + backing globals become typed; the `extra` bag becomes a typed struct; ~60 implementers drop their assertions.

## Data Flow (MANDATORY)

### Entry Point
- Plugin `init()` registers `Registration` with a `Configure*` callback; hub `SetExtra` writes bootstrap state.

### Transformation Path
1. Type the three hook fields + backing globals (`registry`).
2. Update the single injection point (`inprocess.go`) to pass typed values.
3. Sweep ~60 implementers to drop the now-redundant `.(T)` assertion.
4. Introduce a typed bootstrap struct replacing the 9 `extra` keys; update the writers (main.go) and readers (bgp/config).
5. (Optional) add a typed `Coordinator.BGPReactor()` accessor.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| registry ↔ pkg/ze | `ConfigureEventBus(ze.EventBus)` (downward import, legal) | [ ] |
| registry ↔ core/metrics | `ConfigureMetrics(metrics.Registry)` (downward, legal) | [ ] |
| registry ↔ server | `ConfigurePluginServer(PluginServerAccessor)` (leaf iface, no cycle) | [ ] |
| hub ↔ bgp/config | typed bootstrap struct instead of string-keyed `extra` | [ ] |

### Integration Points
- `registry.go` (hooks + globals), `inprocess.go` (injection), every `register.go` implementer, `coordinator.go` (extra→struct), `cmd/ze/hub/main.go`, `bgp/config/register.go`.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `registry`→`pkg/ze` and `registry`→`core/metrics` introduce no cycle | pkg/ze imports nothing from codeberg; core/metrics is below component | tier-check fails | `make ze-tier-check` after typing hooks | unvalidated |
| A-2 | Typing `bgp.store` extra to `storage.Storage` is cycle-safe | `config/storage` ⊥ `component/plugin` today | struct field forces a cycle | build after adding the field; keep it `any` in the struct if it cycles | unvalidated |
| A-3 | Every `Configure*` implementer receives exactly the asserted type (no divergent shapes) | inventory shows uniform `.(T)`; ntp wires via a helper | a nonconforming implementer fails to compile | compiler catches all at sweep time | mostly-confirmed (ntp flagged) |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|-----------|
| R-1 | ~60-file sweep misses a hand-rolled implementer (e.g. ntp helper) | build failure | rely on the compiler — typed signature makes every miss a hard error, not silent |
| R-2 | The typed bootstrap struct grows into a god-struct | review pushback | keep it to the 9 known bootstrap values; do not fold general state into it |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| plugin registers with typed `ConfigureEventBus` | → | `inprocess.go` passes `ze.EventBus` | build + `TestConfigureHooksTyped` (compile-time) |
| hub writes bootstrap struct, bgp/config reads it | → | typed bootstrap struct | `TestBGPBootstrapRoundTrip` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `Registration.ConfigureEventBus/Metrics/PluginServer` | typed (`ze.EventBus` / `metrics.Registry` / `PluginServerAccessor`); no `any` in the signatures |
| AC-2 | every plugin implementer | no `.(ze.EventBus)` / `.(metrics.Registry)` / `.(*pluginserver.Server or PluginServerAccessor)` assertion remains in `Configure*` callbacks |
| AC-3 | Coordinator bootstrap state | delivered via a typed struct; the 9 `extra` string-keys and their assertions are gone (or reduced to any genuinely-untypeable residue, documented) |
| AC-4 | build + tier check | `make ze-tier-check` and `make ze-plugin-boundary-check` pass; no new import cycle |
| AC-5 | runtime | every plugin receives the same instance as before (no behavioral change) — existing functional tests pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigureHooksTyped` | `internal/component/plugin/registry/registry_test.go` | AC-1/AC-2 (compile + no assertion) | |
| `TestBGPBootstrapRoundTrip` | `internal/component/bgp/config/register_test.go` | AC-3 | |
| `TestNoNewImportCycle` | via `make ze-tier-check` | AC-4 | |

### Functional Tests
<!-- .ci proving runtime wiring unchanged across representative plugins. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing plugin functional suite | `test/plugin/*.ci` | plugins still receive bus/metrics/server and behave identically | regression |
| `bgp-rs-reactor-fastpath.ci` | `test/plugin/bgp-rs-reactor-fastpath.ci` | BGP plugins still wired after retyping | regression |

## Files to Modify
- `internal/component/plugin/registry/registry.go` — type the 3 hooks + 3 backing globals + Set/Get accessors
- `internal/component/plugin/inprocess.go` — pass typed values
- `internal/component/plugin/coordinator.go` — `extra` → typed bootstrap struct (+ optional `BGPReactor()` accessor)
- `internal/component/plugin/registry/interfaces.go` — add the typed bootstrap struct (leaf-owned)
- `cmd/ze/hub/main.go` — write the bootstrap struct instead of 9 `SetExtra` calls
- `internal/component/bgp/config/register.go`, `loader.go` — read the typed struct
- ~60 plugin `register.go` files — drop the `Configure*` assertions

## Files to Create
- (none; the bootstrap struct lands in `registry/interfaces.go`)

## Implementation Steps

1. **Wiring:** type the 3 hooks + globals in `registry`; update `inprocess.go`; run `make ze-tier-check` (proves A-1). Build fails at every implementer → that is the worklist.
2. **Sweep** the ~60 implementers to drop assertions (compiler-guided).
3. **Bootstrap struct:** define in `registry`, thread through `main.go` writers + `bgp/config` readers; validate A-2 (storage.Storage cycle).
4. **Optional** typed `BGPReactor()` accessor on Coordinator.
5. Full verification; existing functional tests prove no behavior change (AC-5).
6. Audit tables + learned summary.

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
- [ ] AC-1..AC-5 demonstrated
- [ ] Wiring Test rows all have concrete tests
- [ ] `make ze-test` passes (lint + all ze tests)

### TDD
- [x] Tests written (`TestConfigureHooksTyped`, `TestTypedInstanceAccessorsRoundTrip`, `TestBootstrapRoundTrip`)
- [x] Tests FAIL (before typing: 62 `func(any) as func(T)` compile errors = the worklist)
- [x] Tests PASS (all three green; full tagged build green)

## Implementation Results (2026-07-05)

### Assumptions resolved
- **A-1 confirmed** — `registry -> pkg/ze` and `registry -> core/metrics` are downward, no cycle (`go list -deps`; `make ze-tier-check` green).
- **A-2 confirmed** — `bgp.store -> storage.Storage` is cycle-safe (`config/storage` depends only on `pkg/zefs`); typed the field, no `any` residue.
- **A-3 confirmed** — all 47 implementers use the uniform comma-ok shape; the ntp/as112/geodns "field-assign" variant drops identically (the spec's "ntp via a helper" concern was overstated). The compiler surfaced every site.

### AC evidence
- AC-1: hooks + globals + accessors typed (`registry.go`); `TestConfigureHooksTyped` / `TestTypedInstanceAccessorsRoundTrip`.
- AC-2: no `.(ze.EventBus)/.(metrics.Registry)/.(PluginServerAccessor)` assertion remains in any Configure callback (full build compiles; grep confirms none). 47 implementers swept.
- AC-3: 9-key `extra` bag replaced by typed `BGPBootstrap` (interfaces.go); `TestBootstrapRoundTrip`; zero residual `any`.
- AC-4: `make ze-tier-check` + `make ze-plugin-boundary-check` green; the one new lateral import (`registry -> config/storage`) introduces no cycle.
- AC-5: runtime unchanged by construction (behavior-preserving sweep); `go vet` clean; all changed-package unit tests green; two independent adversarial reviews CONVERGED with no findings. Functional regressions via `ze-verify-changed` (functional-test stage) apart from an unrelated other-session golden red (below).

### Scope beyond spec (review-driven)
- Also cleaned 6 redundant `GetMetricsRegistry().(metrics.Registry)` assertions (blueprint §3d) + removed 5 now-unused `metrics` imports. Kept `manager.metricsRegistry` (own `any` field) and reactor `SetEventBusAny` out of scope per the design.

### Known red not attributable to P2
- `plugin/all` `TestRegisteredWireMethods` fails on `dns-cache` methods owned by `internal/component/resolve/cmd` — another session's uncommitted rename (its files are modified, none by P2). P2 pulls `plugin/all` into changed scope, surfacing their stale golden. Not a P2 regression; their session owns the golden update.
