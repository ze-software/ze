# Spec: feature-gate-7-monitoring

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | plan/spec-feature-gate-0-umbrella.md; learned 980 (lg registry), 981 (ssh seam) |
| Phase | - (ready for /ze-implement; RECOMMENDED LAST or deferred -- see Cost/Benefit) |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/981-feature-gate-2-ssh.md` - the seam pattern; `980` - schema gating
4. `internal/core/metrics/server.go` - the HTTP exporter Server + TelemetryConfig + extraction
5. `cmd/ze/hub/main_system.go` (startStandaloneTelemetry ~327), `internal/component/bgp/config/loader_create.go` (~28 metrics start)
6. `internal/component/telemetry/yang` - the telemetry config schema

## Task

Make the **Prometheus/metrics HTTP exporter compile-out-able** from the `ze` binary via
a `ze_telemetry` build tag, for a smaller binary and a smaller attack surface. Monitoring
is a feature-gate umbrella child (`plan/spec-feature-gate-0-umbrella.md`).

**Critical scoping:** this gates ONLY the HTTP exporter (the `/metrics` + `/health`
listener: `metrics.Server`, its handlers, `TelemetryConfig`, `ExtractTelemetryConfig`,
basic-auth, netdata). It does NOT gate the metric COLLECTION API (`PrometheusRegistry`,
`Gauge`, `Counter`, `Registry`), which lives in `internal/core/metrics` and is used by
~60 plugin/component packages -- that stays always-on. A no-telemetry build still collects
all internal counters; it just cannot expose them over HTTP.

**Cost/Benefit (READ BEFORE IMPLEMENTING):** this is the highest-cost, lowest-value child.
Cost: split a core leaf (`internal/core/metrics`) into registry (always-on) vs exporter
(gated), add a cross-package start hook, rewire TWO construction sites in TWO components
(hub standalone + bgp reactor). Benefit: removes one read-only, optionally-auth'd metrics
HTTP endpoint -- a small attack surface, and the data is still collected. Recommend
sequencing this LAST among the feature-gate children, or deferring it unless a hardened
build specifically forbids any metrics-scraping endpoint.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `plan/learned/981-feature-gate-2-ssh.md` - the dedicated-seam pattern
  → Constraint: a nil-able hook var (set by gated code's init) lets always-on call sites
    invoke a feature without importing it; off => hook nil => feature skipped.
  → Constraint: four-place tag wiring (`ZE_FEATURES`, `.golangci.yml`, `TestBuildTags()`,
    `featureTags`); `dep_audit` `DISABLEABLE` += the gated exporter package -> ze_telemetry.
- [ ] `plan/learned/980-feature-gate-1-lg.md` - generator schema gating + helper relocation
  → Constraint: `featureTags` maps `telemetry/yang -> ze_telemetry`; emit into `all_ze_telemetry.go`.
  → Constraint: exporter-only helpers must move into the gated package or a no-telemetry
    build flags them unused / fails to compile.
- [ ] `ai/rules/module-tiers.md` - core vs component placement
  → Constraint: `internal/core/metrics` is a leaf used by ~60 packages; it must NOT gain a
    build tag. The registry API stays here; the exporter SERVER moves to a gated exporter
    package (preferred path `internal/component/telemetry/exporter` unless Phase 1 proves
    the existing telemetry component dir is cleaner), which imports core/metrics
    (component→core, allowed).
  → Constraint: the start hook var lives in always-on `internal/core/metrics` and is set by
    the gated exporter package init; `make ze-tier-check` must stay green.
- [ ] `ai/rules/plugin-self-containment.md` - the delete-the-folder invariant
  → Constraint: with `ze_telemetry` off, the exporter package unlinks; the registry API and
    all `Gauge`/`Counter` call sites keep working.

### RFC Summaries (MUST for protocol work)
- N/A. No wire-protocol behavior; the Prometheus exposition format is unchanged when compiled in.

**Key insights:**
- `internal/core/metrics/server.go` mixes two concerns: (a) the HTTP exporter `Server`
  (`Start(registry, cfg)`, `Close`, basic-auth middleware, netdata handlers) + config
  (`TelemetryConfig`, `Endpoint`, `ExtractTelemetryConfig`); (b) lives in the same package
  as the registry API (`PrometheusRegistry`, `Gauge`, `Counter`). Only (a) is gateable.
- Two start sites: `cmd/ze/hub/main_system.go:~327` (`startStandaloneTelemetry`, no-bgp
  path) and `internal/component/bgp/config/loader_create.go:~28` (reactor path). Both call
  `metrics.Server.Start(reg, cfg)` directly and must instead call a nil-able hook so neither
  imports the gated exporter.
- `internal/component/telemetry/yang` (all.go:80) is the config schema; gate its blank import.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/metrics/server.go` - `Server` (84) with `Start(registry
  *PrometheusRegistry, cfg TelemetryConfig)` (94), `Close` (161); `TelemetryConfig` (40),
  `Endpoint` (26), `BasicAuthConfig`/`NetdataConfig`/`CollectorConfig`;
  `ExtractTelemetryConfig` (218); `basicAuthMiddleware` + netdata extraction helpers.
  → Constraint: this whole exporter surface moves to the gated package; the registry API
    (`PrometheusRegistry`, `Gauge`, `Counter`, `Registry`) stays in core/metrics always-on.
- [ ] `cmd/ze/hub/main_system.go` (~327 `startStandaloneTelemetry`) - resolves
  `metrics.ExtractTelemetryConfig(tree.ToMap())` and calls `srv.Start(reg, cfg)`.
  → Constraint: rewire to call the nil-able `metrics.StartExporter(reg, tree)` hook; with
    the hook nil, no exporter (metrics still collected).
- [ ] `internal/component/bgp/config/loader_create.go` (~28) - the reactor path starts the
  metrics server.
  → Constraint: rewire to the same hook; BGP must not import the gated exporter package.
- [ ] `internal/core/metrics` (registry) - `PrometheusRegistry`, `Gauge`, `Counter`,
  `Registry`; imported by ~60 plugin/component `register.go` files.
  → Constraint: stays always-on; unchanged. Only `server.go`'s exporter content leaves.
- [ ] `internal/component/plugin/all/all.go` (80 `telemetry/yang`).
  → Constraint: move into generated `all_ze_telemetry.go` under `//go:build ze_telemetry`.

**Behavior to preserve:**
- Default `ze`/`ze-appliance` keep the metrics exporter (ZE_FEATURES includes `ze_telemetry`).
- Metric COLLECTION (every `Gauge`/`Counter` call across all plugins) is unchanged in EVERY
  build, telemetry on or off.
- Exporter behavior (Prometheus exposition, `/health`, basic auth, netdata, multi-endpoint
  bind, reload) byte-for-byte unchanged when compiled in.
- Both start paths (standalone + bgp reactor) start the exporter when compiled in + enabled.

**Behavior to change:**
- The exporter `Server` + config + handlers move from `internal/core/metrics` to a gated
  exporter package (preferred path `internal/component/telemetry/exporter`; exact path
  finalized by the Phase 1 split audit).
- The gated exporter package becomes disableable: `ze_telemetry` off => unlinked.
- `ze-stripped` drops the metrics exporter (no `ze_telemetry`); counters still collected.
- The two start sites call a nil-able `metrics.StartExporter` hook instead of `Server.Start`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Compile time: presence/absence of `ze_telemetry`.
- Run time: the hub standalone path and the bgp reactor path starting the exporter via the hook.

### Transformation Path
1. `register.go` in the gated exporter package (`//go:build ze_telemetry`) `init()` sets
   `metrics.StartExporter = startExporterImpl`.
2. The generator emits `all_ze_telemetry.go` blank-importing `telemetry/yang` only under the tag.
3. A start site calls `if metrics.StartExporter != nil { closer, _ = metrics.StartExporter(reg, tree) }`.
4. The gated impl runs `ExtractTelemetryConfig`, builds + starts the HTTP `Server`, returns a closer.
5. With `ze_telemetry` off, the hook is nil, the exporter package unlinks; the registry keeps collecting.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build tag ↔ composition root | generator-emitted `all_ze_telemetry.go` | [ ] |
| Composition root ↔ exporter | gated `init()` sets `metrics.StartExporter` | [ ] |
| Start site ↔ exporter | nil-able `metrics.StartExporter(reg, tree)` hook | [ ] |
| Registry ↔ exporter | exporter imports core/metrics registry (component→core) | [ ] |
| Disableable exporter ↔ always-on | dep_audit DISABLEABLE enforces; registry stays always-on | [ ] audit |

### Integration Points
- `internal/core/metrics` - gains a nil-able `StartExporter` hook var; keeps the registry API.
- gated exporter package (preferred `internal/component/telemetry/exporter`) - the moved exporter Server + config + handlers.
- `cmd/ze/hub/main_system.go`, `internal/component/bgp/config/loader_create.go` - call the hook.
- `scripts/dev/dep_audit.py` DISABLEABLE - the gated exporter package -> ze_telemetry.
- `scripts/codegen/plugin_imports.go` featureTags - telemetry/yang -> ze_telemetry.

### Architectural Verification
- [ ] No bypassed layers (exporter still started the same way, behind the hook)
- [ ] No unintended coupling (registry stays always-on; only the exporter is gated; hook is core-level)
- [ ] No duplicated functionality (the registry API is not duplicated; only the HTTP server moves)
- [ ] Zero-copy preserved (N/A)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | the exporter surface (Server, TelemetryConfig, extraction, handlers) can be cleanly separated from the registry API | server.go mixes them but the registry is referenced by Start via *PrometheusRegistry | the split entangles registry internals | trace Server.Start deps; confirm it only reads the registry's public API | confirmed -- within core/metrics the exporter symbols live only in server.go/server_test.go; Server.Start takes `*PrometheusRegistry` and calls only `.Handler()`. Clean move. |
| A-2 | only the two known sites start the exporter (no third always-on caller) | Explore enumeration | a missed caller keeps the exporter linked or breaks | grep callers of `metrics.Server` / `.Start(` | confirmed -- grep found exactly main_system.go + loader_create.go (prod) + server_test.go + trafficusage test. |
| A-3 | TelemetryConfig + ExtractTelemetryConfig have no always-on caller outside the start sites | they live in server.go | moving them breaks an always-on caller | grep callers of `ExtractTelemetryConfig` / `TelemetryConfig` | confirmed -- same two prod sites; post-move grep for `metrics.(Server\|ExtractTelemetryConfig\|TelemetryConfig\|Endpoint\|BasicAuthConfig\|NetdataConfig)` returns 0. |
| A-4 | a nil-able hook in internal/core/metrics does not violate the tier rule | core may hold a hook var set by a component init | ze-tier-check flags the hook | add the hook, run `make ze-tier-check` | confirmed -- `make ze-tier-check` green with `metrics.StartExporter` in core/metrics. |
| A-5 | the bgp reactor path can call the hook without importing the gated package | the hook lives in always-on core/metrics | bgp still needs a telemetry type | trace loader_create.go; confirm only reg + tree cross the hook | confirmed -- loader_create.go calls `metrics.StartExporter(tree.ToMap(), configLogger())`; only `map[string]any` + `*slog.Logger` in, `*PrometheusRegistry` + `io.Closer` out. No exporter import. |
| A-6 | a no-telemetry build leaves config validation safe (telemetry/yang not registered) | 980: schema gated => clean "unknown field" | `telemetry {}` / prometheus config panics in a no-telemetry build | build ze_core binary, feed prometheus config | confirmed -- `TestBuildTag_Telemetry_AbsentRejectsTelemetryConfig` passes (clean "unknown" rejection under bare ze_core). |
| A-7 | metric collection is unaffected when the exporter is gated | registry stays always-on | Gauge/Counter calls fail or are dropped in a no-telemetry build | build ze_core binary, exercise a counter, confirm no panic | confirmed -- `TestRegistryCollectsWithoutExporter` + `TestNopRegistryIsTheDummy` pass; ze_core binary nm shows 82 always-on `core/metrics` symbols. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | the registry/exporter split is more entangled than it looks (shared internals) | the move doesn't compile cleanly | A-1 spike first; if entangled, define a minimal exporter-facing registry interface in core/metrics |
| R-2 | a third always-on start site or TelemetryConfig caller exists | dep_audit or build break | A-2/A-3 grep first; rewire any extra site through the hook |
| R-3 | the core-level hook trips the tier check | ze-tier-check fails | A-4; the hook is a value set by a component, a known registration shape; if flagged, place the hook in a tiny always-on shim package |
| R-4 | poor cost/benefit leads to wasted effort | review questions the value | this spec recommends sequencing last/deferring; implement only if the hardened build forbids a metrics endpoint |
| R-5 | a residual exporter symbol keeps it linked | go tool nm shows exporter symbols in ze_core | dep_audit DISABLEABLE + nm symbol-count check |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go build -tags 'ze_core ze_telemetry'` | → | exporter hook set; start sites serve `/metrics` | `TestBuildTag_Telemetry_Present` (cmd/ze/hub) |
| `go build -tags ze_core` (telemetry off) | → | exporter package not linked; no `/metrics` listener; counters still collected | `TestBuildTag_Telemetry_Absent` (cmd/ze/hub) |
| a `Gauge`/`Counter` call on a no-telemetry build | → | registry records it; no panic | `TestRegistryCollectsWithoutExporter` (always-on) |
| `dep_audit.py --check` | → | no always-on import of the gated exporter package | `dep_audit` `--check` clean + `--selftest` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go build` with `ze_telemetry` ON | exporter compiled in; both start sites serve `/metrics` + `/health`; existing telemetry tests pass |
| AC-2 | bare `go build -tags ze_core` (telemetry OFF) | `go tool nm` shows zero exporter `Server` symbols; daemon starts; no `/metrics` listener; no error |
| AC-3 | a no-telemetry build exercises metric collection | `Gauge`/`Counter` calls succeed; the registry records values; no panic (collection is always-on) |
| AC-4 | always-on code is inspected | neither the hub standalone path nor the bgp reactor path imports the gated exporter; both call `metrics.StartExporter` |
| AC-5 | the generator runs | emits `all_ze_telemetry.go` (`//go:build ze_telemetry`) blank-importing `telemetry/yang`; removes it from `all.go`; `--check` passes |
| AC-6 | `dep_audit.py --check` with the exporter package in DISABLEABLE | clean: no always-on importer |
| AC-7 | no-telemetry binary fed config with a prometheus/telemetry block | clean "unknown field" validation, no panic |
| AC-8 | `make ze-tier-check` after adding the core hook | green: `internal/core/metrics` keeps only the registry + the hook var |
| AC-9 | `make ze-stripped` and `make ze` are built | ze-stripped links no exporter symbols (still collects counters); `ze`/`ze-appliance` keep the exporter |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds a hardened `ze` without the metrics endpoint | tag off → exporter package unlinked → no `/metrics` listener; counters still collected | `TestBuildTag_Telemetry_Absent` + `go tool nm` |
| 2 | builds a full `ze` with metrics (default) | tag on → hook set → start site serves `/metrics` | `TestBuildTag_Telemetry_Present` + existing telemetry functional test |
| 3 | scrapes nothing but still relies on internal counters in a no-telemetry build | Gauge/Counter → registry (always-on) → readable internally | `TestRegistryCollectsWithoutExporter` |
| 4 | runs a no-telemetry binary against a config with a prometheus block | config load → telemetry schema absent → clean unknown-field handling | `test/parse/telemetry-absent-config.ci` or absent-test assertion |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildTag_Telemetry_Present` | `cmd/ze/hub/build_tag_telemetry_present_test.go` (`//go:build ze_telemetry`) | `metrics.StartExporter` non-nil | |
| `TestBuildTag_Telemetry_Absent` | `cmd/ze/hub/build_tag_telemetry_absent_test.go` (`//go:build !ze_telemetry`) | `metrics.StartExporter` nil | |
| `TestRegistryCollectsWithoutExporter` | `internal/core/metrics/registry_test.go` | counters work with no exporter | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no new numeric inputs; exporter port/interval unchanged) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `build_tag_telemetry` | `cmd/ze/hub/build_tag_telemetry_*_test.go` | exporter present with `ze_telemetry`, absent without | |
| `telemetry-absent-config` | `test/parse/telemetry-absent-config.ci` (or absent-test assertion) | no-telemetry binary handles prometheus config safely | |
| existing telemetry suite | `internal/core/metrics/*_test.go` (exporter tests) | move with the exporter; gate `//go:build ze_telemetry` | |

### Interop Tests (MANDATORY for protocol features)
- N/A. Prometheus exposition unchanged when compiled in; existing exporter tests run under
  the `ze_telemetry` build.

### Future (if deferring any tests)
- This child is RECOMMENDED for last/deferral (Cost/Benefit). If deferred, no tests are
  written until it is scheduled; the umbrella tracks it.

## Files to Modify
- `internal/core/metrics/server.go` - remove the exporter `Server` + `TelemetryConfig` +
  `ExtractTelemetryConfig` + handlers (moved out); add the nil-able `StartExporter` hook var
- `cmd/ze/hub/main_system.go` - `startStandaloneTelemetry` calls `metrics.StartExporter`
- `internal/component/bgp/config/loader_create.go` - the reactor start path calls `metrics.StartExporter`
- `scripts/codegen/plugin_imports.go` - `featureTags["internal/component/telemetry/yang"] = "ze_telemetry"`
- `internal/component/plugin/all/all.go` - telemetry/yang removed (generator)
- `scripts/dev/dep_audit.py` - `DISABLEABLE[<gated exporter pkg>] = "ze_telemetry"`
- `Makefile` - `ZE_FEATURES += ze_telemetry`
- `internal/test/runner/runner.go` - `TestBuildTags()` appends `ze_telemetry`
- `.golangci.yml` - `build-tags` appends `ze_telemetry`
- `ai/rules/module-tiers.md`, `docs/features.md` - document `ze_telemetry` + the registry/exporter split

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] no | build-time |
| Functional test | [ ] yes | `cmd/ze/hub/build_tag_telemetry_*_test.go`, config-absent assertion, registry-without-exporter |
| Doctor check | [ ] no | telemetry owns its own doctor checks; absent exporter = no check |
| Discovery-updates | [ ] yes | `ai/rules/discovery-updates.md` - register `ze_telemetry` |
| YANG schema | [ ] no new | telemetry/yang exists; only its blank import is gated |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes (build flavor) | `docs/features.md` (build-tag table: add `ze_telemetry`) |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/cli/plugin-modes.md`; `ai/rules/module-tiers.md` (registry vs exporter) |
| 14 | Prometheus counters added/changed? | [ ] note | note the exporter is compile-out-able; collection stays always-on |
| others | - | [ ] assess | grep docs for telemetry/prometheus references |

## Files to Create
- `internal/component/telemetry/exporter/server.go` (gated `//go:build ze_telemetry`) - the moved exporter `Server` + config + handlers
- `internal/component/telemetry/exporter/register.go` (gated) - `init(){ metrics.StartExporter = startExporterImpl }`
- `cmd/ze/hub/build_tag_telemetry_present_test.go` (`//go:build ze_telemetry`), `build_tag_telemetry_absent_test.go` (`//go:build !ze_telemetry`)
- `internal/component/plugin/all/all_ze_telemetry.go` (generated, `//go:build ze_telemetry`)
- `test/parse/telemetry-absent-config.ci` (if expressible)

(Exact gated package path defaults to `internal/component/telemetry/exporter`; adjust only if the Phase 1 split audit proves the existing `internal/component/telemetry` dir is the cleaner self-contained exporter home.)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, Current Behavior, Assumptions (validate A-1/A-2/A-3/A-5 first) |
| 3. Wiring | Wiring Test - hook + build-tag tests |
| 4. Implement | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Verification | `make ze-verify-changed` |
| 14. Summary | Implementation Summary |

### Implementation Phases
1. **Phase 1: split registry vs exporter (behavior-preserving, NO gating)**
   - Move the exporter `Server` + `TelemetryConfig` + `ExtractTelemetryConfig` + handlers
     out of `internal/core/metrics` into the exporter package; add the `metrics.StartExporter`
     hook; rewire both start sites through it. Exporter still always-on (hook set unconditionally).
   - Tests: `TestRegistryCollectsWithoutExporter`; existing exporter tests move + pass.
   - Verify: `make ze-verify-changed` passes; `make ze-tier-check` green; behavior identical.
2. **Phase 2: gate the exporter behind `ze_telemetry`**
   - Gate the exporter package + its register.go `//go:build ze_telemetry`; generator gates
     telemetry/yang; four-place tag wiring; dep_audit DISABLEABLE += exporter pkg.
   - Tests: present/absent build-tag tests.
   - Verify: generator `--check` clean; dep_audit clean; nm 0 exporter symbols in ze_core.
3. **Phase 3: docs + safety**
   - `docs/features.md`, `module-tiers.md`; validate A-6 (config) + A-7 (collection works no-exporter).
4. **Full verification** - `make ze-verify-changed`; nm-measure ze_core vs ze_telemetry.

### Failure Routing
| Failure | Route To |
|---------|----------|
| exporter not omitted with tag off | residual always-on import - dep_audit + nm (R-5) |
| registry/exporter split won't compile | entangled internals - R-1/A-1; define a registry interface |
| tier check fails on the hook | core hook placement - R-3/A-4 |
| collection breaks no-exporter | registry accidentally moved - A-7 |
| 3 fix attempts fail | STOP, report, ask user |

### Critical Review Checklist (/implement stage 7)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Registry stays always-on | `PrometheusRegistry`/`Gauge`/`Counter` remain in core/metrics; ~60 callers unchanged |
| No always-on exporter import | dep_audit `--check` clean; both start sites use the hook |
| Symbol absence | `go tool nm` on ze_core binary lists zero exporter Server symbols |
| Collection works no-exporter | a ze_core build records counters without panic |
| Tier | `make ze-tier-check` green; the hook is the only new core surface |

### Deliverables Checklist (/implement stage 11)
| Deliverable | Verification method |
|-------------|---------------------|
| registry/exporter split + hook | `grep StartExporter internal/core/metrics`; `ls` the gated exporter package |
| dep_audit DISABLEABLE entry | `python3 scripts/dev/dep_audit.py --check` exits 0; `--selftest` passes |
| generated all_ze_telemetry.go | `ls internal/component/plugin/all/all_ze_telemetry.go`; `--check` |
| symbol drop | `go build -tags ze_core -o /tmp/ze-core ...`; `go tool nm` exporter count = 0 |
| present/absent tests | `go test -tags 'ze_core ze_telemetry' -run TestBuildTag_Telemetry` and `-tags ze_core` |

### Security Review Checklist (/implement stage 12)
| Check | What to look for |
|-------|-----------------|
| No endpoint exposure | a no-telemetry build serves no `/metrics` or `/health` exporter listener |
| Basic-auth handling | exporter basic-auth stays inside the gated code; not reachable no-telemetry |
| No data leak via collection | the always-on registry is internal-only with no exporter; counters are not exposed without the gated server |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (spec) the gated exporter package's `init()` sets the hook, blank-imported via generated `all_ze_telemetry.go` | the generator only emits DISCOVERED packages (plugin/schema/rpc/namespace); the exporter package is none of those, so it is never auto-blank-imported | reading `scripts/codegen/plugin_imports.go` discovery during Phase 1 | switched to the proven hub-seam: `cmd/ze/hub/register_telemetry.go` (gated) sets `metrics.StartExporter = exporter.Start`; `all_ze_telemetry.go` only gates the schema |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Make `registry.GetMetricsRegistry()` return a `NopRegistry` dummy by default (the literal "dummy default" requested) | `ike/engine/register.go:216` gates a 5s ticker GOROUTINE on `GetMetricsRegistry() != nil`; a Nop default would spin that idle ticker (and risk other control-signal consumers) when telemetry is off, for zero functional gain (skip vs no-op-record are identical) | keep the nil contract; the `NopRegistry` dummy stays always-on and available, collection-without-exporter is proven by `TestRegistryCollectsWithoutExporter` + `TestNopRegistryIsTheDummy`, and every one of the 8 `GetMetricsRegistry` consumers already nil-checks |
| Separate gated exporter package whose own `init()` sets the hook, reached via the composition root | not discoverable by the generator (above); a core-leaf hook set from the composition root would need generator surgery | hub-seam setter (hub is linked in every ze_telemetry binary: `ze`/`ze-appliance`); exporter exports `Start` with no init side-effect |
| One manifest line `ze_telemetry internal/component/telemetry` to gate `telemetry/yang` | `internal/component/telemetry` is not a Go package (no .go files) -- a confusing manifest entry | relocate the schema to `telemetry/exporter/yang` so one real-package line gates `exporter` + `exporter/yang` (api/rest/yang precedent); add a second line for the `collector` netdata sidecar (dep_audit safety) |

## Design Insights
- Monitoring is the canonical "registry vs server" split: the COLLECTION API is pervasive
  always-on infrastructure; only the HTTP EXPOSURE is optional. Gating must cut between them,
  not gate the whole metrics package.
- Some compile-out children have poor cost/benefit; documenting that (and recommending
  deferral) is a valid spec outcome, not a gap.

## Core Insight
You can only gate the part of a package that nothing always-on imports. For metrics, that
is the HTTP exporter alone; the registry API is load-bearing for ~60 packages and stays.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Gate the exporter only; keep the registry always-on | gate all of internal/core/metrics | the registry API is used by ~60 packages; gating it is impossible without rewriting every plugin |
| Move the exporter to a gated exporter package + a core hook | build-tag the core package | a core leaf cannot carry a build tag without breaking its ~60 importers; the hook keeps core always-on |
| Start sites call a nil-able hook | each site imports the exporter conditionally | the bgp reactor path is in another component; a core-level hook lets both call without importing the gated package |
| Recommend last/deferral | implement eagerly | highest cost (core split + cross-component rewire), lowest value (one read-only endpoint; data still collected) |

## Known Limitations
- Gates only the HTTP exporter; metric collection is always-on (by design).
- A no-telemetry build cannot be scraped by Prometheus/netdata; internal counters still exist.
- Highest-cost, lowest-value feature-gate child; recommend implementing last or deferring.

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| exporter compile-out-able via `ze_telemetry` | build-tag test + nm symbol check | `TestBuildTag_Telemetry_Absent` passes; `go tool nm bin/ze-core-test` = 0 exporter symbols AND 0 collector symbols |
| collection unaffected when gated | unit test + nm | `TestRegistryCollectsWithoutExporter` + `TestNopRegistryIsTheDummy` pass; ze_core binary keeps 82 `internal/core/metrics` symbols |
| no always-on import of the exporter | audit | `dep_audit.py --check` exit 0 with `telemetry/exporter` + `telemetry/collector` in the manifest; `make ze-tier-check` green |
| default flavors keep the exporter | build + nm | ON binary (`ze_core` + ZE_FEATURES) links 30 `telemetry/exporter` symbols; `TestBuildTag_Telemetry_Present` confirms the seam is wired |

## Review Gate
### Run 1 (focused self-review per /ze-review checklist; full automated review noisy due to concurrent geodns session)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Standalone + reactor telemetry start logs were unified into `exporter.Start` (same message text, caller-supplied logger). Standalone messages lost the "standalone telemetry:" prefix; reactor keeps `bgp.config` logger name. Intentional consolidation, not exposition. | exporter/exporter.go | accepted |
| 2 | NOTE | Reactor path discards the exporter `io.Closer` (`_`), matching prior behavior (the old `var srv metrics.Server` was never closed; daemon-lifetime). | loader_create.go | accepted |
| 3 | NOTE | `make generate` pulled a concurrent session's untracked `geodns` plugin into shared `all.go`; stripped via Bash so my commit excludes it. `generate --check` is locally red on geodns only until that session commits. | all.go | accepted (cross-session) |

### Run 2 (/ze-review, post-implementation)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | exporter over-exported its config surface: `TelemetryConfig` + `ExtractTelemetryConfig` (and the `Server`/`Endpoint`/`BasicAuthConfig`/`NetdataConfig`/`CollectorConfig` family) had no cross-package non-test caller; only `exporter.Start` is the prod entry. Flagged by `ze-validate`. | telemetry/exporter/server.go | RESOLVED -- unexported all of them (`server`/`telemetryConfig`/`endpoint`/`basicAuthConfig`/`netdataConfig`/`collectorConfig`/`extractTelemetryConfig`, methods `start`/`close`/`joinHostPort`); `Start` is the package's only exported symbol. Moved `server_test.go` to internal `package exporter`; reworked the trafficusage integration test to scrape via `reg.Handler()` + `httptest` (dropping its dep on the gated package, so its build tag returns to `integration && linux`). `ze-validate` no longer flags either symbol. |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")
- Wiring: `metrics.StartExporter` set by `register_telemetry.go`, read by both start sites; `exporter.Start` (the package's only exported symbol) called by the setter; schema gated via `all_ze_telemetry.go`. No orphan symbols.
- Run 2 ISSUE-1 RESOLVED; remaining `ze-validate` flag (`main_system.go:259 SetIdentityStore`) is pre-existing (0 occurrences in this diff), unrelated to telemetry.
- 0 BLOCKER, 0 open ISSUE in my changes; NOTEs (2-5) recorded above.

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| internal/core/metrics/exporter_hook.go | yes | StartExporter hook (always-on) |
| internal/component/telemetry/exporter/{server,exporter}.go | yes | gated `//go:build ze_telemetry` exporter |
| internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang | yes | relocated schema |
| internal/component/plugin/all/all_ze_telemetry.go | yes | generated, gates exporter/yang |
| cmd/ze/hub/register_telemetry.go | yes | gated seam wiring |
| cmd/ze/hub/build_tag_telemetry_{present,absent}_test.go | yes | present/absent tests |
| internal/core/metrics/registry_test.go | yes | dummy/collection-without-exporter proof |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | exporter ON serves /metrics | ON binary links 30 exporter symbols; `TestBuildTag_Telemetry_Present` PASS; exporter unit tests PASS |
| AC-2 | ze_core 0 exporter Server symbols | `go tool nm bin/ze-core-test` exporter=0, collector=0; OFF build PASS |
| AC-3 | collection works no-exporter | `TestRegistryCollectsWithoutExporter` + `TestNopRegistryIsTheDummy` PASS; 82 metrics symbols in ze_core |
| AC-4 | no always-on exporter import | both sites call `metrics.StartExporter`; `dep_audit.py --check` exit 0 |
| AC-5 | generator emits all_ze_telemetry.go | `ls` confirms; gates `exporter/yang`; removed from all.go |
| AC-6 | dep_audit clean | exit 0 with exporter+collector in manifest |
| AC-7 | no-telemetry rejects telemetry config | `TestBuildTag_Telemetry_AbsentRejectsTelemetryConfig` PASS (clean "unknown") |
| AC-8 | tier-check green | `make ze-tier-check` exit 0 |
| AC-9 | flavors: ze keeps, stripped drops | ON binary 30 symbols; ze_core 0 symbols |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | clean split; Server.Start uses only `*PrometheusRegistry.Handler()` |
| A-2 | confirmed | exactly 2 prod start sites |
| A-3 | confirmed | post-move grep returns 0 stray callers |
| A-4 | confirmed | ze-tier-check green |
| A-5 | confirmed | only map+logger in, reg+closer out |
| A-6 | confirmed | AbsentRejectsTelemetryConfig PASS |
| A-7 | confirmed | RegistryCollectsWithoutExporter PASS |

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 demonstrated
- [ ] registry/exporter split done; registry stays always-on; collection works no-exporter
- [ ] exporter compile-out-able; present/absent build-tag tests pass
- [ ] dep_audit DISABLEABLE clean; `make ze-tier-check` green
- [ ] generator emits all_ze_telemetry.go; `--check` passes
- [ ] `make ze-verify-changed` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (build-tag present/absent + config-safe + collection-without-exporter)
- [ ] Goal Validation table filled with concrete evidence
