# Spec: feature-gate-6-api

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | plan/spec-feature-gate-0-umbrella.md; learned 980 (lg registry), 981 (ssh seam) |
| Phase | 1/3 |
| Updated | 2026-06-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/981-feature-gate-2-ssh.md` - the dedicated-seam pattern this mirrors
4. `cmd/ze/hub/api.go` (startAPIServers), `cmd/ze/hub/listener_migrate.go` (SetREST/SetGRPC)
5. `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go`
6. `internal/component/api` (ConfigSessionManager - stays always-on)

## Task

Make the **REST and gRPC API servers compile-out-able** from the `ze` binary via a
`ze_api` build tag, for a smaller binary and a smaller attack surface. API is a
feature-gate umbrella child (`plan/spec-feature-gate-0-umbrella.md`).

REST and gRPC are the lowest-entanglement services: nothing always-on imports
`internal/component/api/rest` or `internal/component/api/grpc` except `cmd/ze/hub`
(`api.go` builds both in `startAPIServers`; `listener_migrate.go` names the two server
types in `SetREST`/`SetGRPC`). They are built **as a pair** (shared `apiCfg`, engine,
session manager) and wired to **two** migrator slots, so they fit a **dedicated seam**
(ssh/gnmi's pattern) better than the one-factory-one-service construction registry. They
share one YANG module (`api/yang`, the `api {}` block) and one construction function, so
the gate is **combined `ze_api`** (both rest and grpc together).

The parent package `internal/component/api` (`ConfigSessionManager`, shared types) stays
always-on: gNMI and the API servers all use it; only the two SERVER subpackages are gated.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `plan/learned/981-feature-gate-2-ssh.md` - the dedicated-seam pattern
  → Constraint: an always-on `*_infra.go` declares opaque handles + a nil-able build hook
    + a setter; the gated `service_<x>.go` holds the impl; `register_<x>.go` installs it.
  → Constraint: only GENERIC types cross the seam; never `rest.RESTServer`/
    `apigrpc.GRPCServer`. The hub's always-on files must not import the server packages.
  → Constraint: four-place tag wiring (`ZE_FEATURES`, `.golangci.yml`, `TestBuildTags()`,
    `featureTags`); `dep_audit` `DISABLEABLE` += BOTH `api/rest` and `api/grpc -> ze_api`.
- [ ] `plan/learned/980-feature-gate-1-lg.md` - generator schema gating + Reconfigurable widening
  → Constraint: `featureTags` maps `api/yang -> ze_api`; emit into `all_ze_api.go`.
  → Constraint: widen `SetREST`/`SetGRPC` from the concrete server types to
    `Reconfigurable` (as `SetLG`/`SetWeb` did) so `listener_migrate.go` drops the
    apigrpc/rest imports. The `rest`/`grpc` migrator fields are ALREADY `Reconfigurable`.
- [ ] `cmd/ze/hub/api.go` - `startAPIServers` builds rest (when `RESTOn`) and grpc (when
  `GRPCOn`) together, returning both handles.
  → Constraint: rest+grpc are constructed in one call; the seam returns both handles so
    the migrator can keep them as separate slots with independent conflict detection.
- [ ] `ai/rules/plugin-self-containment.md` - the delete-the-folder invariant
  → Constraint: with `ze_api` off, both server packages unlink; no rest/grpc spelling in
    always-on hub files. The parent `api` package is NOT disableable (ConfigSessionManager).

### RFC Summaries (MUST for protocol work)
- N/A. Compile-out is a composition/build-tag change; REST/gRPC behavior is unchanged
  when compiled in.

**Key insights:**
- Always-on importers of the SERVER packages: only `cmd/ze/hub/api.go` (builds them) and
  `cmd/ze/hub/listener_migrate.go` (`SetREST`/`SetGRPC` type signatures; imports at lines
  12-13). No blank import in `all.go` for the servers (only `api/yang` at all.go:12).
- `internal/component/api` (parent) holds `ConfigSessionManager`, used by gNMI's
  `buildGNMISessionManager` (api.go:449) and the servers. It stays always-on; only
  `api/rest` and `api/grpc` are gated.
- REST and gRPC share `api/yang` (one `api {}` config module covering both) and one
  `startAPIServers`, so combined `ze_api` is the natural granularity; per-server
  `ze_rest`/`ze_grpc` would require splitting both the YANG module and startAPIServers
  (A-4 / Key Design Decision).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/api.go` - `startAPIServers(apiCfg, ...)` (66): builds `rest.NewRESTServer`
  (111) when `cfg.RESTOn`, `apigrpc.NewGRPCServer` (138) when `cfg.GRPCOn`; each `.Start()`
  binds listeners; returns the handles. `buildGNMISessionManager` (449) returns
  `*api.ConfigSessionManager` (parent package, always-on).
  → Constraint: the rest/grpc construction moves into the gated impl; `startAPIServers`
    becomes the seam impl. `buildGNMISessionManager` / ConfigSessionManager stay always-on.
- [ ] `cmd/ze/hub/main.go` - calls `startAPIServers` (873); wires the returned handles to
  `lm.SetREST`/`lm.SetGRPC`; shutdown on exit.
  → Constraint: the API config resolution (`zeconfig.ExtractAPIConfig`, RESTOn/GRPCOn)
    stays always-on (plain values); the build + the apigrpc/rest imports move out.
- [ ] `cmd/ze/hub/listener_migrate.go` - `SetREST(s *rest.RESTServer)` (67),
  `SetGRPC(s *apigrpc.GRPCServer)` (70); imports apigrpc + rest (12-13); `rest`/`grpc`
  fields already `Reconfigurable`; ReloadListeners handles them (101-113).
  → Constraint: widen both setters to `Reconfigurable`; drop the two imports. Migration
    logic is otherwise unchanged (both are reconfigurable, separate slots).
- [ ] `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go` -
  `NewRESTServer`/`NewGRPCServer`; each has Start/Shutdown/Addresses/Reconfigure +
  a private `*ListenerDiff` copy.
  → Constraint: each already satisfies `Reconfigurable`+Shutdown; the seam returns them
    as `Reconfigurable` handles.
- [ ] `internal/component/plugin/all/all.go` (12 `api/yang`).
  → Constraint: move into generated `all_ze_api.go` under `//go:build ze_api`.

**Behavior to preserve:**
- Default `ze`/`ze-appliance` keep REST + gRPC (ZE_FEATURES includes `ze_api`).
- REST + gRPC behavior (endpoints, TLS, auth, listener migration on reload, the
  RESTOn/GRPCOn independent enable) byte-for-byte unchanged when compiled in.
- `internal/component/api` (`ConfigSessionManager`) stays always-on; gNMI keeps using it.
- listener discovery/default handling for the OTHER services.

**Behavior to change:**
- `internal/component/api/rest` and `internal/component/api/grpc` become disableable:
  `ze_api` off => both unlinked.
- `ze-stripped` drops the API servers (no `ze_api`).
- `SetREST`/`SetGRPC` take `Reconfigurable`; listener_migrate.go names no server type.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Compile time: presence/absence of `ze_api`.
- Run time: the hub building the API servers via the seam hook.

### Transformation Path
1. `register_api.go` (`//go:build ze_api`) `init()` calls `setAPIInfra(apiBuildImpl)`.
2. The generator emits `all_ze_api.go` blank-importing `api/yang` only under `ze_api`.
3. The hub resolves `apiCfg` (RESTOn/GRPCOn, listens, TLS) + engine + session manager into
   `apiBuildInputs` (generic types) and calls `if apiBuild != nil { rH, gH, stop = apiBuild(&inputs) }`.
4. The gated impl runs `startAPIServers`, returns the rest + grpc handles as `Reconfigurable`
   plus a shutdown func.
5. The hub wires `lm.SetREST(rH)` / `lm.SetGRPC(gH)` (widened to `Reconfigurable`).
6. With `ze_api` off, the hook is nil, the blank import drops, both server packages unlink.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build tag ↔ composition root | generator-emitted `all_ze_api.go` | [ ] |
| Composition root ↔ seam | `register_api.go` init() → `setAPIInfra(...)` | [ ] |
| Seam ↔ hub | `apiBuild(&apiBuildInputs)` returns Reconfigurable handles | [ ] |
| Handles ↔ migrator | `lm.SetREST/SetGRPC(Reconfigurable)` | [ ] |
| Disableable api/rest + api/grpc ↔ always-on | dep_audit DISABLEABLE enforces | [ ] audit |

### Integration Points
- `cmd/ze/hub/api_infra.go` (new, always-on) - the seam.
- `cmd/ze/hub/service_api.go` (new, gated) - the impl (moved startAPIServers rest/grpc build).
- `internal/component/api` (parent) - stays always-on (ConfigSessionManager).
- `scripts/dev/dep_audit.py` DISABLEABLE - api/rest + api/grpc -> ze_api.
- `scripts/codegen/plugin_imports.go` featureTags - api/yang -> ze_api.

### Architectural Verification
- [ ] No bypassed layers (servers still built + migrator-wired the same way, behind the seam)
- [ ] No unintended coupling (only generic types cross the seam; parent api stays always-on)
- [ ] No duplicated functionality (reuse the seam shape; servers keep their private ListenerDiff)
- [ ] Zero-copy preserved (N/A - composition/build change only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | only the hub imports api/rest + api/grpc (no other always-on importer) | Explore enumeration | a missed importer keeps a server linked | grep importers of api/rest and api/grpc | **confirmed** -- only `cmd/ze/hub/api.go` + `listener_migrate.go` imported them; both severed |
| A-2 | the parent `internal/component/api` is needed always-on (ConfigSessionManager) and must NOT be gated | gnmi's buildGNMISessionManager uses it | gating the parent breaks gnmi/other users | grep importers of `internal/component/api` (parent, not /rest //grpc) | **confirmed** -- parent imported by gnmi, web, hub; kept always-on (base api/yang + parent pkg stay); `ze_core ze_gnmi` (no api) builds + passes |
| A-3 | apiBuildInputs are all generic (no rest/grpc/apigrpc type in always-on main.go) | apiCfg + engine + sessions are always-on types | main.go still needs a server type | enumerate apiBuildInputs fields | **confirmed** -- apiBuildInputs fields are zeconfig/pluginserver/storage/authz/aaa/audit; apiShared is parent-api types; no rest/grpc type always-on |
| A-4 | api/yang is one module covering both rest+grpc, so combined ze_api is correct | all.go:12 single api/yang import | per-feature split (ze_rest/ze_grpc) is required | confirm api/yang covers both; user confirms combined granularity | **BROKEN (superseded)** -- user chose per-server `ze_rest`/`ze_grpc`; api/yang split into base + 2 transport modules via Ze container-merge. See Mistake Log + Deviations. |
| A-5 | a no-api build leaves config validation safe (schema not registered) | 980: schema gated => clean "unknown field" | config panics in a no-transport build | build ze_core binary, feed rest/grpc config | **confirmed** -- `environment { api-server { rest {} } }` / `{ grpc {} }` rejected as unknown on a no-transport build, no panic (TestBuildTag_REST/GRPC_AbsentRejects*) |
| A-6 | listener discovery/default handling tolerates absent rest+grpc schema/services | listener services are schema-discovered; api-rest/api-grpc have builtin listener defaults | conflict detection panics/misfires | build ze_core binary, run listener-conflict path | **confirmed** -- listener parse suite 15/15 PASS (incl listener-conflict-api); doctor/config suites pass under shipped tags |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | user wanted per-server granularity (ze_rest/ze_grpc), not combined ze_api | review feedback | A-4 documents the combined choice + what a split needs (yang + startAPIServers split); confirm at implementation |
| R-2 | the parent api package gets accidentally gated, breaking gnmi | gnmi build fails with api off | DISABLEABLE lists ONLY api/rest + api/grpc, never the parent; A-2 validates |
| R-3 | a residual server symbol keeps api linked | go tool nm shows rest/grpc symbols in ze_core | dep_audit DISABLEABLE (both pkgs) + nm symbol-count check |
| R-4 | the seam returning two handles + a shutdown is awkward vs the registry | code review friction | the pair-construction + two-slot migration justify the seam over the registry (Key Design Decision) |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go build -tags 'ze_core ze_api'` | → | api seam installed; hub builds rest+grpc | `TestBuildTag_API_Present` (cmd/ze/hub) |
| `go build -tags ze_core` (api off) | → | api/rest + api/grpc not linked; daemon starts without them | `TestBuildTag_API_Absent` (cmd/ze/hub) |
| reload on an api build | → | `SetREST`/`SetGRPC` handles `Reconfigure` | existing api listener-migration test |
| `dep_audit.py --check` | → | no always-on import of api/rest or api/grpc | `dep_audit` `--check` clean + `--selftest` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go build` with `ze_api` ON | rest + grpc compiled in, built via the seam, started per RESTOn/GRPCOn; existing api tests pass; migration works |
| AC-2 | bare `go build -tags ze_core` (api OFF) | `go tool nm` shows zero `api/rest` and `api/grpc` symbols; daemon starts without them; no error |
| AC-3 | always-on hub code is inspected | no always-on file imports api/rest or api/grpc; `SetREST`/`SetGRPC` take `Reconfigurable` |
| AC-4 | gNMI is built (ze_gnmi on, ze_api off) | `internal/component/api` parent (ConfigSessionManager) still linked and usable; gnmi works |
| AC-5 | the generator runs | emits `all_ze_api.go` (`//go:build ze_api`) blank-importing `api/yang`; removes it from `all.go`; `--check` passes |
| AC-6 | `dep_audit.py --check` with api/rest + api/grpc in DISABLEABLE | clean: no always-on importer |
| AC-7 | no-api binary fed config containing `api { ... }` | clean "unknown field" validation, no panic |
| AC-8 | `make ze-stripped` and `make ze` are built | ze-stripped links no api server symbols; `ze`/`ze-appliance` keep REST + gRPC |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds a hardened `ze` without REST/gRPC | tag off → api blank import dropped → server packages unlinked → no api listeners | `TestBuildTag_API_Absent` + `go tool nm` |
| 2 | builds a full `ze` with the API (default) | tag on → seam installed → startAPIServers builds rest+grpc → listeners up | `TestBuildTag_API_Present` + existing api functional tests |
| 3 | builds a no-api `ze` that still serves gNMI | api off, gnmi on → parent api package (ConfigSessionManager) still linked | gnmi functional test under `ze_core ze_gnmi` (api off) |
| 4 | runs a no-api binary against a config with `api {}` | config load → api schema absent → clean unknown-field handling | `test/parse/api-absent-config.ci` or absent-test assertion |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildTag_API_Present` | `cmd/ze/hub/build_tag_api_present_test.go` (`//go:build ze_api`) | api seam hook non-nil | |
| `TestBuildTag_API_Absent` | `cmd/ze/hub/build_tag_api_absent_test.go` (`//go:build !ze_api`) | api seam hook nil | |
| `TestAPISeamBuildsBoth` | `cmd/ze/hub/service_api_test.go` (`//go:build ze_api`) | seam returns rest+grpc handles per RESTOn/GRPCOn | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no new numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `build_tag_api` | `cmd/ze/hub/build_tag_api_*_test.go` | api present with `ze_api`, absent without | |
| `api-absent-config` | `test/parse/api-absent-config.ci` (or absent-test assertion) | no-api binary handles `api {}` config safely | |
| gnmi-without-api | existing gnmi functional test under `ze_core ze_gnmi` | parent api package still usable with servers gated | |

### Interop Tests (MANDATORY for protocol features)
- N/A. REST/gRPC behavior unchanged when compiled in; existing api functional tests run
  under the `ze_api` build.

### Future (if deferring any tests)
- Per-server `ze_rest`/`ze_grpc` split is out of scope (combined `ze_api`); noted in A-4.

## Files to Modify
- `cmd/ze/hub/api.go` - move the rest/grpc construction (the RESTOn/GRPCOn branches of
  `startAPIServers`) into `service_api.go`; keep `buildGNMISessionManager` + ConfigSessionManager always-on
- `cmd/ze/hub/main.go` - keep apiCfg resolution always-on; call the seam; wire handles via
  widened `SetREST`/`SetGRPC`; remove apigrpc/rest imports
- `cmd/ze/hub/listener_migrate.go` - `SetREST`/`SetGRPC` take `Reconfigurable`; drop the two imports
- `scripts/codegen/plugin_imports.go` - `featureTags["internal/component/api/yang"] = "ze_api"`
- `internal/component/plugin/all/all.go` - api/yang removed (generator)
- `scripts/dev/dep_audit.py` - `DISABLEABLE["internal/component/api/rest"] = "ze_api"`, `DISABLEABLE["internal/component/api/grpc"] = "ze_api"`
- `Makefile` - `ZE_FEATURES += ze_api`
- `internal/test/runner/runner.go` - `TestBuildTags()` appends `ze_api`
- `.golangci.yml` - `build-tags` appends `ze_api`
- any hub test naming rest/grpc server types - gate `//go:build ze_api`
- `ai/rules/module-tiers.md`, `docs/features.md` - document `ze_api`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] keep | api listen flags resolve always-on to plain values |
| Functional test | [ ] yes | `cmd/ze/hub/build_tag_api_*_test.go`, config-absent assertion, gnmi-without-api |
| Doctor check | [ ] no | api owns its own doctor checks; absent api = no check registered |
| Discovery-updates | [ ] yes | `ai/rules/discovery-updates.md` - register `ze_api` |
| YANG schema | [ ] no new | api/yang exists; only its blank import is gated |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes (build flavor) | `docs/features.md` (build-tag table: add `ze_api`) |
| 4 | API/RPC added/changed? | [ ] note | `docs/architecture/api/*` - note REST/gRPC are compile-out-able via `ze_api` |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/cli/plugin-modes.md` (seam services) |
| others | - | [ ] assess | grep docs for api/rest, api/grpc references |

## Files to Create
- `cmd/ze/hub/api_infra.go` (always-on) - opaque handles, `apiBuildInputs`, nil-able `apiBuild`, `setAPIInfra`
- `cmd/ze/hub/service_api.go` (`//go:build ze_api`) - `apiBuildImpl` (moved rest/grpc construction)
- `cmd/ze/hub/register_api.go` (`//go:build ze_api`) - `init(){ setAPIInfra(apiBuildImpl) }`
- `cmd/ze/hub/build_tag_api_present_test.go` (`//go:build ze_api`), `build_tag_api_absent_test.go` (`//go:build !ze_api`)
- `internal/component/plugin/all/all_ze_api.go` (generated, `//go:build ze_api`)
- `test/parse/api-absent-config.ci` (if expressible)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, Current Behavior, Assumptions (validate A-1/A-2/A-3 first) |
| 3. Wiring | Wiring Test - seam + build-tag tests |
| 4. Implement | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Verification | `make ze-verify-changed` |
| 14. Summary | Implementation Summary |

### Implementation Phases
1. **Phase 1: seam + impl move**
   - Add `api_infra.go` (seam); move the rest/grpc construction into `service_api.go`
     (gated); `register_api.go` installs the hook; main.go calls the seam + widened
     SetREST/SetGRPC; remove apigrpc/rest imports from always-on files.
   - Tests: `TestBuildTag_API_Present/Absent`, `TestAPISeamBuildsBoth`.
   - Verify: `ze_api` build identical; ze_core build compiles without the servers.
2. **Phase 2: schema gating + tag wiring + audit**
   - generator `featureTags` += api/yang; regenerate `all_ze_api.go`; four-place tag
     wiring; dep_audit DISABLEABLE += api/rest + api/grpc (NOT the parent, A-2).
   - Verify: generator `--check` clean; dep_audit clean; nm 0 rest/grpc symbols in ze_core;
     gnmi-without-api still builds and works.
3. **Phase 3: docs + config safety**
   - `docs/features.md`, `module-tiers.md`; validate A-5 (`api {}` config) + A-6 on ze_core.
4. **Full verification** - `make ze-verify-changed`; nm-measure ze_core vs ze_api.

### Failure Routing
| Failure | Route To |
|---------|----------|
| api not omitted with tag off | residual always-on server import - dep_audit + nm (R-3) |
| gnmi breaks with api off | the parent api package got gated - R-2/A-2 |
| config panics in no-api build | schema absence - A-5 |
| 3 fix attempts fail | STOP, report, ask user |

### Critical Review Checklist (/implement stage 7)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Parent vs server | DISABLEABLE lists ONLY api/rest + api/grpc; the parent api package is untouched |
| No always-on server import | dep_audit `--check` clean; `grep -rl internal/component/api/rest` shows only gated/test |
| SetREST/SetGRPC widening | no always-on file names a server type |
| Symbol absence | `go tool nm` on ze_core binary lists zero rest/grpc server symbols |
| gnmi-without-api | a ze_gnmi (api off) build links the parent api package and works |

### Deliverables Checklist (/implement stage 11)
| Deliverable | Verification method |
|-------------|---------------------|
| api seam + impl + registration | `ls cmd/ze/hub/api_infra.go service_api.go register_api.go` |
| dep_audit DISABLEABLE entries (both) | `python3 scripts/dev/dep_audit.py --check` exits 0; `--selftest` passes |
| generated all_ze_api.go | `ls internal/component/plugin/all/all_ze_api.go`; `--check` |
| symbol drop | `go build -tags ze_core -o /tmp/ze-core ...`; `go tool nm` rest+grpc count = 0 |
| present/absent tests | `go test -tags 'ze_core ze_api' -run TestBuildTag_API` and `-tags ze_core` |

### Security Review Checklist (/implement stage 12)
| Check | What to look for |
|-------|-----------------|
| No auth bypass | gating api removes REST + gRPC endpoints + their auth surfaces; a no-api build exposes neither |
| TLS/token handling | API TLS + auth stays inside the gated code; not reachable no-api |
| Parent api safety | ConfigSessionManager (still always-on) has no residual rest/grpc exposure when servers gated |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-4: combined `ze_api` is the right granularity (rest+grpc share one YANG module + one startAPIServers) | The user wants per-encoding compile-out (gRPC without REST or vice-versa). REST and gRPC are independent encodings with independent server packages; the shared `api-server` YANG splits because Ze merges same-named containers across modules. | User request mid-implementation ("break ze-api into multiple plugins"). R-1 risk materialized. | Reworked from combined `ze_api` to per-server `ze_rest` + `ze_grpc`: ze-api-conf.yang keeps the always-on `api-server { token }` base; `internal/component/api/rest/yang` (gated ze_rest) and `internal/component/api/grpc/yang` (gated ze_grpc) contribute the `rest{}`/`grpc{}` containers. Two seam hooks (restBuild/grpcBuild) share an always-on engine/session builder. |

### Deviations from Plan
| Plan said | Reality | Why |
|-----------|---------|-----|
| Combined `ze_api` (single tag, rest+grpc together); per-server split deferred (A-4) | Per-server `ze_rest` + `ze_grpc` implemented | User chose the full per-server split: independent compile-out per encoding, each transport's config rejected when its code is off (YANG split via Ze's container-merge). |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- API is the cleanest compile-out target (only the hub names the server types), but it
  forces the "parent stays, subpackages gate" distinction: ConfigSessionManager in the
  parent `api` package is shared infra and must not move behind `ze_api`.
- Pair-constructed services (rest+grpc from one startAPIServers, two migrator slots) are a
  seam fit, not a registry fit: the registry's one-factory-one-service shape would either
  merge the two slots or duplicate the shared construction.

## Core Insight
Gate the leaf SERVER subpackages, never the shared parent: `api/rest` + `api/grpc` are
disableable, but `api` (ConfigSessionManager) is always-on infra other features depend on.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Dedicated seam, not the construction registry | registry with two factories | rest+grpc are built together (shared cfg/engine/sessions) and wired to two migrator slots; a seam returning both handles preserves that without merging slots or duplicating construction |
| Combined `ze_api` (rest+grpc) | per-server `ze_rest`/`ze_grpc` | they share one `api/yang` module and one `startAPIServers`; splitting needs a YANG-module + construction split (A-4). Confirm with user if per-server is wanted |
| Parent `internal/component/api` stays always-on | gate the whole api tree | ConfigSessionManager is used by gnmi + servers; only the two server subpackages are disableable (A-2) |
| Widen SetREST/SetGRPC to Reconfigurable | keep concrete server types | the migrator fields are already Reconfigurable; widening drops the always-on apigrpc/rest imports |

## Known Limitations
- Combined `ze_api`: cannot build rest-only or grpc-only without a follow-up YANG +
  construction split.
- A no-api build has no REST/gRPC management; management is via ssh CLI / web / gnmi / mcp.
- The parent `api` package is not compile-out-able (shared ConfigSessionManager).

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| REST compile-out-able via `ze_rest`, gRPC via `ze_grpc` (independent) | build-tag tests + nm symbol matrix | `go tool nm`: `ze_core` rest=0 grpc=0; `ze_core ze_rest` rest=119 grpc=**0** (gRPC dropped); `ze_core ze_grpc` rest=**0** grpc=79 (REST dropped). present/absent tests pass for both |
| gRPC-without-REST and REST-without-gRPC build | compile | `go vet` passes for `ze_core ze_rest`, `ze_core ze_grpc`, and `ze_core ze_rest ze_grpc` |
| no always-on import of the server packages | audit | `dep_audit.py --check` clean (manifest-derived DISABLEABLE: api/rest + api/grpc) |
| gnmi still works with both transports off | compile + tests | `ze_core ze_gnmi` builds; parent api + base api-server schema stay always-on |
| each transport's config rejected when off | functional test | `TestBuildTag_REST_AbsentRejectsRESTConfig`, `TestBuildTag_GRPC_AbsentRejectsGRPCConfig`; api parse suite 3/3 + listener 15/15 PASS with both on |
| default flavors keep the API | build | `ZE_FEATURES` derives `ze_rest`+`ze_grpc`; `ze`/`ze-appliance` link both; `ze-stripped`/`ze_core` drop both |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER (fixed) | YANG split broke committed config tests that import only the base `api/yang` while testing rest/grpc config (rest/grpc schemas relocated to gated packages) | `loader_extract_test.go`, `api_schema_check_test.go`, `all_schemas_test.go` | fixed: added blank imports of `api/rest/yang` + `api/grpc/yang` to those tests (schemas relocated, not removed). All config tests pass. |
| 2 | NOTE | doctor tests (`TestCheckListeners_API`, `TestCollectSchemaListeners_SSH*`) fail under bare `ze_core` because they rely on build-tag-registered schemas | `internal/component/doctor` | not a regression: these are gate-dependent (ssh gated since 981; api now gated). The unit suite runs shipped tags (`GO_TEST_TAGS` = manifest gates) where they pass. |
| 3 | NOTE | combined-ze_api shutdown ordering (deferred) carried over: REST/gRPC shut down via `apiShutdowns` after `apiServer.Stop` | `main.go` | unchanged from prior behavior; in-flight requests drained by http.Server.Shutdown / grpc graceful stop |
| 4 | DISCARDED (pre-existing) | `ze-validate` flags pre-existing hub exports lacking cross-package callers | hub | present at HEAD; `ze-verify` does not run validate.py |

### Final status
- [x] Review complete: 0 BLOCKER remaining (finding 1 fixed), NOTEs recorded
- [x] All NOTEs recorded above

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/hub/api_infra.go` | yes | always-on seam: apiBuildInputs, apiShared, apiServerHandle, restBuild/grpcBuild, setRESTInfra/setGRPCInfra |
| `cmd/ze/hub/service_rest.go` + `register_rest.go` | yes | `//go:build ze_rest` REST factory + hook install |
| `cmd/ze/hub/service_grpc.go` + `register_grpc.go` | yes | `//go:build ze_grpc` gRPC factory + hook install |
| `internal/component/api/rest/yang/ze-rest-conf.yang` (+gen) | yes | gated rest{} schema; all_ze_rest.go blank-imports it |
| `internal/component/api/grpc/yang/ze-grpc-conf.yang` (+gen) | yes | gated grpc{} schema; all_ze_grpc.go blank-imports it |
| `internal/component/api/yang/ze-api-conf.yang` | yes | reduced to always-on base `api-server { token }` |
| build_tag_rest_*_test.go, build_tag_grpc_*_test.go, service_rest_test.go, service_grpc_test.go | yes | present/absent + seam tests per transport |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | transports compiled in work | api parse 3/3, listener 15/15 PASS; present + seam tests PASS |
| AC-2 | bare ze_core drops servers | `go tool nm` ze_core rest=0 grpc=0; AbsentBinaryDrops* tests PASS |
| AC-3 | no always-on server import; SetREST/SetGRPC widened | dep_audit clean; api/rest only in service_rest.go, api/grpc only in service_grpc.go; setters take Reconfigurable |
| AC-4 (per-server) | gnmi works with both transports off | `ze_core ze_gnmi` builds; parent api + base schema stay |
| AC-5 | generator emits gated all_ze_rest/grpc.go | `ls`; generator `--check` current; base api/yang stays in all.go |
| AC-6 | dep_audit clean | `--check` exit 0; `--selftest` OK |
| AC-7 | per-transport config rejected when off | rest{}/grpc{} -> unknown on no-transport build, no panic |
| AC-8 | independent compile-out (the new goal) | nm matrix: ze_rest=REST-only, ze_grpc=gRPC-only; ZE_FEATURES has both for ze/ze-appliance |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | only hub api.go + listener_migrate.go imported the servers; both severed |
| A-2 | confirmed | parent api + base api/yang always-on; gnmi-without-api builds |
| A-3 | confirmed | apiBuildInputs/apiShared are generic/parent-api types only |
| A-4 | **broken -> superseded** | per-server ze_rest/ze_grpc chosen (user); see Mistake Log + Deviations |
| A-5 | confirmed | rest{}/grpc{} rejected as unknown on no-transport build |
| A-6 | confirmed | listener parse 15/15; config/doctor suites pass under shipped tags |

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 demonstrated
- [ ] api seam created; servers moved; SetREST/SetGRPC widened; parent api untouched
- [ ] rest+grpc compile-out-able; present/absent build-tag tests pass
- [ ] dep_audit DISABLEABLE clean (both server pkgs)
- [ ] generator emits all_ze_api.go; `--check` passes
- [ ] `make ze-verify-changed` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (build-tag present/absent + config-safe + gnmi-without-api)
- [ ] Goal Validation table filled with concrete evidence
