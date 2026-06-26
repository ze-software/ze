# Spec: feature-gate-5-mcp

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
3. `plan/learned/980-feature-gate-1-lg.md` - the construction-registry pattern this mirrors
4. `cmd/ze/hub/service_registry.go`, `service_lg.go`, `register_lg.go`, `listener_migrate.go` (SetMCP)
5. `cmd/ze/hub/mcp.go` (MCPServerHandle, startMCPServer, mcpConfigToStreamable), `main_servers.go` (serverCommandLister)
6. `internal/component/mcp/streamable.go` (NewStreamable, Streamable)

## Task

Make the **MCP (Model Context Protocol) server compile-out-able** from the `ze` binary
via a `ze_mcp` build tag, for a smaller binary and a smaller attack surface. MCP is a
feature-gate umbrella child (`plan/spec-feature-gate-0-umbrella.md`).

MCP fits the **construction registry** (lg/web's pattern), NOT a bespoke seam: the
hub-side `MCPServerHandle` already implements `Reconfigurable` (`Addresses`/`Reconfigure`)
+ `Shutdown` and is migrator-registered via `SetMCP`, exactly like web/lg. It has no
reactor or reload-closure coupling. The wrinkles are that the factory needs richer
generic inputs than lg's (command metadata, the audit recorder, the resolved auth
config), and API already reuses MCP's command lister. Split command metadata into an
always-on generic builder first; the gated MCP file only converts that metadata into
`zemcp.CommandInfo`/`ParamInfo`. MCP's authentication is internal (bearer /
bearer-list / OAuth), with no AAA-package dependency; `audit.Recorder` stays always-on
(other surfaces use it).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint:. -->
- [ ] `plan/learned/980-feature-gate-1-lg.md` - the construction-registry pattern
  → Constraint: registry in `package hub` (`service_registry.go`): `Service`
    (`Reconfigurable`+`Name`+`Shutdown`), `ServiceDeps`, `registerService`/`buildServices`/
    `registerBuiltService`. The factory + adapter live in `service_<x>.go`
    (`//go:build ze_<x>`); registration `init()` in `register_<x>.go`.
  → Constraint: four-place tag wiring -- `ZE_FEATURES`, `.golangci.yml`, `TestBuildTags()`,
    `featureTags` (generator). Generator gates `mcp/yang -> ze_mcp` into `all_ze_mcp.go`.
  → Constraint: feature-only helpers (MCP server construction and zemcp adapters) move
    INTO the gated file or a no-mcp build flags them U1000-unused / fails to compile.
    Shared command metadata stays always-on so API does not depend on `ze_mcp`.
- [ ] `plan/learned/981-feature-gate-2-ssh.md` - the DISABLEABLE gate
  → Constraint: `dep_audit` `DISABLEABLE` += `internal/component/mcp -> ze_mcp`; `--check`
    flags any always-on (untagged, non-test) importer. Note the ze-chaos orchestrator
    imports mcp -- verify it is NOT classed always-on by dep_audit (A-5).
- [ ] `cmd/ze/hub/service_lg.go` / `listener_migrate.go` - the adapter + migrator wiring
  → Constraint: `SetMCP` currently takes `*MCPServerHandle` (a hub type that moves into
    the gated file); widen it to `Reconfigurable` (as `SetWeb`/`SetLG` did) and route
    "mcp" in `registerBuiltService`. The `mcp` migrator field is already `Reconfigurable`.
- [ ] `ai/rules/plugin-self-containment.md` - the delete-the-folder invariant
  → Constraint: with `ze_mcp` off, the mcp blank import drops and the package unlinks; no
    mcp spelling in always-on hub files.

### RFC Summaries (MUST for protocol work)
- N/A. MCP is JSON-over-HTTP; compile-out is a composition/build-tag change with no wire
  behavior change when compiled in.

**Key insights:**
- MCP's always-on importers are: the hub (`main.go:44`, `main_servers.go:30`, `mcp.go:21`)
  and `all.go:71` (`mcp/yang` blank import). The chaos orchestrator imports mcp but is
  ze-chaos-only (not production always-on). audit is a dependency MCP keeps, and audit
  stays always-on (used by ssh/web/api too).
- The construction is concentrated: `mcp.go` (`mcpConfigToStreamable`, `startMCPServer`,
  the `MCPServerHandle` type with `Addresses`/`Reconfigure`/`Shutdown`) plus
  `main_servers.go:156` `serverCommandLister` (builds `zemcp.CommandLister`/`CommandInfo`).
  All of it moves into a gated `service_mcp.go`.
- The factory inputs are all GENERIC-typed (resolvable always-on): the resolved MCP
  config (addrs, token, TLS cert/key, auth mode, bearer list, OAuth), the MCP-surface
  command dispatcher (`serverDispatcherWithSurface(apiServer, audit.MCP)`), the plugin
  server + YANG loader needed to build the command lister, and the `audit.Recorder`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/mcp.go` - `mcpConfigToStreamable` (32), `startMCPServer` (202) building
  the `MCPServerHandle` (177: Server `*http.Server` + Handler `*zemcp.Streamable` + bound
  listeners), `NewStreamable` (212), `Addresses` (334), `Shutdown` (348), `Reconfigure`
  (366), `serveMCP` (265).
  → Constraint: `MCPServerHandle` already satisfies `Reconfigurable`+`Shutdown`; an
    adapter only needs `Name()=="mcp"`. The whole file moves into the gated service file.
- [ ] `cmd/ze/hub/main_servers.go` / `api.go` - `serverCommandLister` currently returns
  `zemcp.CommandLister`, and `apiCommandLister` reuses it.
  → Constraint: extract an always-on generic command metadata builder first. MCP wraps it
    in a gated `zemcp.CommandLister`; API wraps the same generic metadata in `api.CommandSource`.
- [ ] `cmd/ze/hub/main.go` - imports zemcp (44); `Run`/`run`/`runYANGConfig` carry
  `mcpAddr`/`mcpToken` params; MCP listen/token resolution (344-381); `mcpDispatch`
  (617); `StreamableConfig` (788); `startMCPServer` (795); `lm.SetMCP(mcpSrv)` (798);
  shutdown `mcpSrv.Shutdown` (1036).
  → Constraint: the CLI-flag/env/config resolution stays always-on (no zemcp type needed
    once resolved to plain values); the StreamableConfig construction + startMCPServer +
    the zemcp import move out; `SetMCP` widens to `Reconfigurable`.
- [ ] `cmd/ze/hub/listener_migrate.go` - `SetMCP(s *MCPServerHandle)` (64); `mcp`
  field already `Reconfigurable`; ReloadListeners handles mcp (93-99) via
  `zeconfig.ExtractMCPConfig`.
  → Constraint: widen `SetMCP` to `Reconfigurable`; the migration path is otherwise
    unchanged (mcp is reconfigurable, unlike gnmi).
- [ ] `internal/component/mcp/streamable.go` - `NewStreamable(cfg) (*Streamable, error)`;
  `StreamableConfig` (dispatch, commands, token, auth mode, bearer list, oauth, audit).
  Auth is internal (no AAA package import).
  → Constraint: only the resolved config values + the dispatch/commands funcs + audit
    recorder cross into the factory; all generic-typed.
- [ ] `internal/component/plugin/all/all.go` (71 `mcp/yang`).
  → Constraint: move into generated `all_ze_mcp.go` under `//go:build ze_mcp`.

**Behavior to preserve:**
- Default `ze`/`ze-appliance` keep MCP (ZE_FEATURES includes `ze_mcp`).
- MCP behavior (tools listing, dispatch, bearer/bearer-list/OAuth auth, session TTL/limits,
  audit logging, listener migration on reload) byte-for-byte unchanged when compiled in.
- `audit.Recorder` and the MCP audit surface stay always-on.
- listener discovery/default handling for the OTHER services.

**Behavior to change:**
- `internal/component/mcp` becomes a disableable feature: `ze_mcp` off => unlinked.
- `ze-stripped` drops MCP (no `ze_mcp`).
- `SetMCP` takes `Reconfigurable`; no always-on file names a zemcp type or `*MCPServerHandle`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Compile time: presence/absence of `ze_mcp`.
- Run time: the hub building MCP via `buildServices(deps)`.

### Transformation Path
1. `register_mcp.go` (`//go:build ze_mcp`) `init()` calls `registerService("mcp", buildMCPService)`.
2. The generator emits `all_ze_mcp.go` blank-importing `mcp/yang` only under `ze_mcp`.
3. The hub resolves MCP config + the command-source inputs into `ServiceDeps` (new MCP
   fields / sub-struct, all generic types) and calls `buildServices(deps)`.
4. `buildMCPService` wraps the generic command metadata source as a gated `zemcp.CommandLister`,
   builds the Streamable, the `MCPServerHandle`, starts listeners, returns an `mcpService` adapter.
5. `registerBuiltService` routes the built mcp service into `ListenerMigrator.SetMCP`.
6. With `ze_mcp` off, the mcp package is unimported, uncompiled, unlinked.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build tag ↔ composition root | generator-emitted `all_ze_mcp.go` | [ ] |
| Composition root ↔ registry | `register_mcp.go` init() → `registerService("mcp", ...)` | [ ] |
| Registry ↔ hub | `buildServices(deps)`; no direct `zemcp` import always-on | [ ] |
| mcp service ↔ migrator | `registerBuiltService` → `SetMCP(Reconfigurable)` | [ ] |
| Disableable mcp ↔ always-on | dep_audit DISABLEABLE enforces | [ ] audit |

### Integration Points
- `ServiceDeps` (extended) - MCP resolved config + command-source inputs (generic types).
- `Reconfigurable` / `ListenerMigrator` (existing) - mcp routed via `SetMCP`.
- `audit.Recorder` (always-on) - passed into the factory.
- `scripts/dev/dep_audit.py` DISABLEABLE - `internal/component/mcp -> ze_mcp`.
- `scripts/codegen/plugin_imports.go` featureTags - `mcp/yang -> ze_mcp`.

### Architectural Verification
- [ ] No bypassed layers (mcp still built via registry + Reconfigurable + migrator)
- [ ] No unintended coupling (only generic types cross into the factory)
- [ ] No duplicated functionality (reuse the registry; audit stays shared always-on)
- [ ] Zero-copy preserved (N/A - composition/build change only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | MCP's only production always-on importers are the hub + all.go (mcp/yang) | Explore map | a missed always-on importer keeps mcp linked | re-grep importers of `internal/component/mcp` (excluding ze-chaos) | **confirmed** -- grep: production importers are `main.go`, `main_servers.go`, `mcp.go` (all hub) + `all.go` (mcp/yang); rest are `_test.go` or `internal/chaos/*` |
| A-2 | all factory inputs are generic-typed (no zemcp type needed in always-on main.go) | StreamableConfig fields resolve from config; command metadata can be plain hub values | main.go or API still needs a zemcp type | enumerate ServiceDeps MCP fields; confirm none are zemcp types | **confirmed** -- factory inputs are `[]string` addrs, `string` token, `zeconfig.MCPListenConfig`, a generic dispatch func, a neutral `commandMeta` source, and `audit.Recorder`; none are zemcp types |
| A-3 | command metadata can be built once in always-on hub code and adapted separately for API and MCP | apiCommandLister already reuses serverCommandLister | API breaks when `serverCommandLister` moves behind ze_mcp | split generic builder; build api and mcp adapters from it; compile ze_api without ze_mcp | **confirmed** -- introduced neutral `commandMetaSource()` (command_meta.go, always-on); API adapts it directly, MCP wraps it as `zemcp.CommandLister` in the gated file |
| A-4 | a no-mcp build leaves config validation safe (mcp/yang not registered) | 980: schema gated => clean "unknown field" | `mcp {}` config panics in a no-mcp build | build ze_core binary, feed `mcp {}` config | confirmed -- bare `ze_core` build rejects `mcp {}` as unknown field, no panic (same as web/lg/gnmi gating) |
| A-5 | dep_audit does NOT class internal/chaos/orchestrator as always-on (so its zemcp import won't trip DISABLEABLE) | Explore: chaos orchestrator is ze-chaos-only | adding mcp to DISABLEABLE flags the chaos orchestrator | run `dep_audit.py --check` after adding mcp; inspect whether orchestrator is scanned | **BROKEN** -- `disableable_violations` text-scans the whole repo and WOULD flag `internal/chaos/{orchestrator,mcp}` (untagged direct imports of `internal/component/mcp`). Fixed by R-2 fallback: exclude non-production trees (`internal/chaos/`, `internal/test/`) from the disableable scan. See Mistake Log. |
| A-6 | listener discovery/default handling tolerates an absent mcp schema/service | listener services are schema-discovered; mcp has a builtin listener default | parse-time conflict detection panics/misfires | build ze_core binary, run listener-conflict path and config with no mcp schema | confirmed -- registry skips an unregistered mcp factory; `knownListenerServices`/listener defaults unchanged for other services |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | ServiceDeps grows several MCP-specific fields, bloating the generic struct | service_registry.go becomes mcp-aware | use a nested `MCP` sub-struct of generic-typed fields; keep `ServiceDeps` readable |
| R-2 | the chaos orchestrator's zemcp import trips DISABLEABLE | dep_audit `--check` fails on internal/chaos | A-5 first; if flagged, gate the orchestrator import or refine dep_audit's always-on definition |
| R-3 | a residual zemcp symbol (CommandInfo metadata) keeps mcp linked | go tool nm shows mcp symbols in ze_core | dep_audit DISABLEABLE + nm symbol-count check in the absent test |
| R-4 | the MCP-surface dispatcher (`serverDispatcherWithSurface`) couples the factory to apiServer | factory can't be built at buildServices time | the dispatcher is a hub func producing a generic func value; build it always-on and pass the func in |
| R-5 | API command listing breaks because it reused MCP's lister | ze_api without ze_mcp fails to compile or loses command metadata | split the generic command metadata builder before moving the zemcp adapter |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go build -tags 'ze_core ze_mcp'` | → | mcp factory registered; hub builds mcp | `TestBuildTag_MCP_Present` (cmd/ze/hub) |
| `go build -tags ze_core` (mcp off) | → | mcp package not linked; daemon starts without mcp | `TestBuildTag_MCP_Absent` (cmd/ze/hub) |
| registry has a registered mcp factory | → | hub builds mcp via `buildServices`, not `startMCPServer` directly | `TestServiceRegistry_BuildsMCP` (gated `ze_mcp`) |
| `dep_audit.py --check` | → | no production always-on import of `internal/component/mcp` | `dep_audit` `--check` clean + `--selftest` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go build` with `ze_mcp` ON | mcp compiled in, registered, started; existing mcp tests pass; tools/dispatch/auth work; listener migration works |
| AC-2 | bare `go build -tags ze_core` (mcp OFF) | `go tool nm` shows zero `internal/component/mcp` symbols; daemon starts without mcp; no error |
| AC-3 | always-on hub code is inspected | no always-on file imports `internal/component/mcp` or names `*MCPServerHandle`; `SetMCP` takes `Reconfigurable` |
| AC-4 | the generator runs | emits `all_ze_mcp.go` (`//go:build ze_mcp`) blank-importing `mcp/yang`; removes it from `all.go`; `--check` passes |
| AC-5 | `dep_audit.py --check` with mcp in DISABLEABLE | clean: no production always-on importer (chaos orchestrator not flagged) |
| AC-6 | no-mcp binary fed config containing `mcp { ... }` | clean "unknown field" validation, no panic |
| AC-7 | a no-mcp build exercises listener discovery/default handling | port-conflict detection for other services works; no panic from the absent mcp schema/service |
| AC-8 | `make ze-stripped` and `make ze` are built | ze-stripped links no mcp symbols; `ze`/`ze-appliance` keep MCP |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds a hardened `ze` without mcp | tag off → mcp blank import dropped → package unlinked → no mcp listener | `TestBuildTag_MCP_Absent` + `go tool nm` symbol check |
| 2 | builds a full `ze` with mcp (default) | tag on → factory registered → hub builds mcp via registry → MCP listens | `TestBuildTag_MCP_Present` + existing mcp functional tests |
| 3 | reloads listener config on an mcp build | `ReloadListeners` → `SetMCP(Reconfigurable)` → mcp `Reconfigure` | existing listener-migration test (mcp routed via registry) |
| 4 | runs a no-mcp binary against a config with `mcp {}` | config load → mcp schema absent → clean unknown-field handling | `test/parse/mcp-absent-config.ci` or absent-test assertion |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestServiceRegistry_BuildsMCP` | `cmd/ze/hub/service_mcp_test.go` (`//go:build ze_mcp`) | hub builds mcp via registry, not direct startMCPServer | |
| `TestBuildTag_MCP_Present` | `cmd/ze/hub/build_tag_mcp_present_test.go` (`//go:build ze_mcp`) | mcp factory registered | |
| `TestBuildTag_MCP_Absent` | `cmd/ze/hub/build_tag_mcp_absent_test.go` (`//go:build !ze_mcp`) | mcp factory not registered | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no new numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `build_tag_mcp` | `cmd/ze/hub/build_tag_mcp_*_test.go` | mcp present with `ze_mcp`, absent without | |
| `mcp-absent-config` | `test/parse/mcp-absent-config.ci` (or absent-test assertion) | no-mcp binary handles `mcp {}` config safely | |
| existing mcp suite | `cmd/ze/hub/mcp_test.go`, `mcp_keyperm_test.go` | gate `//go:build ze_mcp`; pass under the ze_mcp unit build | |

### Interop Tests (MANDATORY for protocol features)
- N/A. MCP wire behavior unchanged when compiled in; the existing MCP functional tests run
  under the `ze_mcp` build.

### Future (if deferring any tests)
- None. MCP is fully in scope for this spec.

## Files to Modify
- `cmd/ze/hub/mcp.go` - move MCP-only construction (`mcpConfigToStreamable`, `startMCPServer`,
  `MCPServerHandle`, `serveMCP`) into `service_mcp.go` (`//go:build ze_mcp`)
- `cmd/ze/hub/main_servers.go` / `api.go` - split command metadata: keep the generic
  traversal always-on for API; move only the `zemcp.CommandInfo` adapter into `service_mcp.go`
- `cmd/ze/hub/main.go` - keep MCP flag/env/config resolution always-on (plain values);
  move StreamableConfig construction + startMCPServer into the factory; remove the zemcp import
- `cmd/ze/hub/listener_migrate.go` - `SetMCP` takes `Reconfigurable`
- `cmd/ze/hub/service_registry.go` - `ServiceDeps` gains a nested `MCP` sub-struct (resolved
  config + command-source inputs, all generic types); `registerBuiltService` routes "mcp" → `SetMCP`
- `scripts/codegen/plugin_imports.go` - `featureTags["internal/component/mcp/yang"] = "ze_mcp"`
- `internal/component/plugin/all/all.go` - mcp/yang removed (generator)
- `scripts/dev/dep_audit.py` - `DISABLEABLE["internal/component/mcp"] = "ze_mcp"`
- `Makefile` - `ZE_FEATURES += ze_mcp`
- `internal/test/runner/runner.go` - `TestBuildTags()` appends `ze_mcp`
- `.golangci.yml` - `build-tags` appends `ze_mcp`
- `cmd/ze/hub/mcp_test.go`, `mcp_keyperm_test.go` - gate `//go:build ze_mcp`
- `ai/rules/module-tiers.md`, `docs/features.md` - document `ze_mcp`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] keep | `--mcp-listen`/`--mcp-token` flag parsing stays always-on (resolves to plain values) |
| Functional test | [ ] yes | `cmd/ze/hub/build_tag_mcp_*_test.go`, config-absent assertion |
| Doctor check | [ ] no | mcp owns its own doctor check; absent mcp = no check registered |
| Discovery-updates | [ ] yes | `ai/rules/discovery-updates.md` - register `ze_mcp` |
| YANG schema | [ ] no new | mcp/yang exists; only its blank import is gated |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes (build flavor) | `docs/features.md` (build-tag table: add `ze_mcp`) |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/cli/plugin-modes.md` (registry services) |
| 15 | Runtime inventory changed? | [ ] no | mcp tools unchanged when compiled in |
| others | - | [ ] assess | grep docs for mcp references |

## Files to Create
- `cmd/ze/hub/service_mcp.go` (`//go:build ze_mcp`) - `buildMCPService` factory + `mcpService` adapter + moved mcp.go contents + zemcp command-lister adapter
- `cmd/ze/hub/register_mcp.go` (`//go:build ze_mcp`) - `init(){ registerService("mcp", buildMCPService) }`
- `cmd/ze/hub/build_tag_mcp_present_test.go` (`//go:build ze_mcp`), `build_tag_mcp_absent_test.go` (`//go:build !ze_mcp`)
- `internal/component/plugin/all/all_ze_mcp.go` (generated, `//go:build ze_mcp`)
- `test/parse/mcp-absent-config.ci` (if expressible)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, Current Behavior, Assumptions (validate A-1/A-2/A-3/A-5 first) |
| 3. Wiring | Wiring Test - registry + build-tag tests |
| 4. Implement | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Verification | `make ze-verify-changed` |
| 14. Summary | Implementation Summary |

### Implementation Phases
1. **Phase 1: registry-ize mcp behind `ze_mcp`**
   - Move mcp.go + the zemcp command-lister adapter into `service_mcp.go`; add
     `mcpService` adapter + `buildMCPService`; `register_mcp.go` registers it. Extract
     the generic command metadata builder first so API still works without `ze_mcp`.
     Extend `ServiceDeps` with the MCP sub-struct; route "mcp" in `registerBuiltService`;
     widen `SetMCP`; resolve MCP config to plain values in always-on main.go; remove the
     zemcp import from always-on files.
   - Tests: `TestServiceRegistry_BuildsMCP`, present/absent build-tag tests.
   - Verify: `ze_mcp` build identical; ze_core build drops mcp.
2. **Phase 2: schema gating + tag wiring + audit**
   - generator `featureTags` += mcp/yang; regenerate `all_ze_mcp.go`; four-place tag
     wiring; dep_audit DISABLEABLE += mcp (validate A-5 chaos orchestrator).
   - Verify: generator `--check` clean; dep_audit clean; nm 0 mcp symbols in ze_core.
3. **Phase 3: docs + config safety**
   - `docs/features.md`, `module-tiers.md`; validate A-4 (`mcp {}` config) + A-6 on ze_core.
4. **Full verification** - `make ze-verify-changed`; nm-measure ze_core vs ze_mcp.

### Failure Routing
| Failure | Route To |
|---------|----------|
| mcp not omitted with tag off | residual always-on mcp import - dep_audit + nm (R-3) |
| chaos orchestrator flagged by dep_audit | A-5/R-2 - gate the orchestrator import or refine dep_audit scope |
| factory can't build at buildServices time | dispatcher/lister coupling - R-4/A-3 |
| config panics in no-mcp build | schema absence - A-4 |
| 3 fix attempts fail | STOP, report, ask user |

### Critical Review Checklist (/implement stage 7)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| No always-on mcp import | dep_audit `--check` clean; `grep -rl internal/component/mcp` shows only gated/test/chaos files |
| SetMCP widening | no always-on file names `*MCPServerHandle` or a zemcp type |
| Symbol absence | `go tool nm` on ze_core binary lists zero mcp symbols |
| audit still always-on | a no-mcp build still has audit + the API/ssh audit surfaces |
| Test gating | mcp-requiring tests gated; default ze_core unit suite passes |

### Deliverables Checklist (/implement stage 11)
| Deliverable | Verification method |
|-------------|---------------------|
| mcp factory + registration | `ls cmd/ze/hub/service_mcp.go register_mcp.go`; `grep registerService.*mcp` |
| dep_audit DISABLEABLE entry | `python3 scripts/dev/dep_audit.py --check` exits 0; `--selftest` passes |
| generated all_ze_mcp.go | `ls internal/component/plugin/all/all_ze_mcp.go`; `--check` |
| symbol drop | `go build -tags ze_core -o /tmp/ze-core ...`; `go tool nm` mcp count = 0 |
| present/absent tests | `go test -tags 'ze_core ze_mcp' -run TestBuildTag_MCP` and `-tags ze_core` |

### Security Review Checklist (/implement stage 12)
| Check | What to look for |
|-------|-----------------|
| No auth bypass | gating mcp removes the MCP endpoint + its bearer/OAuth surface; a no-mcp build exposes no MCP |
| Token/OAuth handling | bearer list + OAuth verification stay inside the gated code; not reachable no-mcp |
| Audit retained | auth-failure audit logging path for OTHER surfaces unaffected by removing mcp |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-5: dep_audit would NOT flag chaos's mcp import (chaos is "ze-chaos-only") | dep_audit's `disableable_violations` is a build-tag-agnostic text scan over the whole repo; it skips only the feature's own tree, `_test.go`, and tag-gated files. `internal/chaos/{orchestrator,mcp}` import `internal/component/mcp` untagged, so they WOULD be flagged. | grep: `internal/chaos/orchestrator/run.go:29`, `internal/chaos/mcp/tools.go:12`, `internal/chaos/orchestrator/cli.go:35` import zemcp; ze-chaos builds `-tags ze_chaos` (no ze_mcp), reaching the orchestrator only under `//go:build ze_chaos` in `cmd/ze/ze_chaos_run.go`. | mcp is the first disableable feature chaos imports. Resolved by R-2 fallback: refined `disableable_violations` to skip non-production trees (`internal/chaos/`, `internal/test/`) -- they are not in the production `ze` daemon, mirroring the engine gate's `NON_FEATURE_PREFIXES`. Added a selftest fixture. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- MCP is the cleanest registry fit after lg: its handle is already Reconfigurable +
  migrator-registered. The only work beyond lg is widening ServiceDeps for the
  command-lister source and moving the zemcp-typed metadata builders into the gated file.
- "audit stays always-on" recurs (ssh, mcp): cross-cutting infra a feature USES must not
  move behind the feature's tag, or other features lose it.

## Core Insight
A service whose hub handle already implements `Reconfigurable`+`Shutdown` is a registry
fit; the work is only severing the always-on construction helpers that name its types.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Construction registry (like web/lg), not a seam | ssh-style seam | MCPServerHandle is already Reconfigurable + migrator-registered; no reactor/reload coupling |
| ServiceDeps nested `MCP` sub-struct | flat MCP fields (like LGAddrs/LGTLS) | MCP needs several inputs (config + command source + audit); a sub-struct keeps ServiceDeps readable |
| audit.Recorder stays always-on, passed into the factory | move audit behind ze_mcp | audit is shared by ssh/web/api; only the resolved recorder crosses into the mcp factory |
| Widen SetMCP to Reconfigurable | keep `*MCPServerHandle` | MCPServerHandle moves into the gated file; always-on migrator must not name it |

## Known Limitations
- MCP compile-out is independent of the other feature children.
- A no-mcp build has no MCP/agent interface; management is via ssh CLI / web / API / gnmi.

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| mcp compile-out-able via `ze_mcp` | build-tag test + nm symbol check | `TestBuildTag_MCP_Absent` + `TestBuildTag_MCP_AbsentBinaryDropsMCPSymbols` PASS; `go tool nm` on a bare `ze_core` binary = **0** `internal/component/mcp` symbols; the `ze_core ze_mcp` binary = **342** |
| no production always-on import of mcp | audit | `dep_audit.py --check` exits 0 with `ze_mcp internal/component/mcp` in the manifest; `--selftest` OK (non-prod chaos importer excluded) |
| listener migration works when compiled in | functional test | `bin/ze-test bgp parse --pattern mcp` 4/4 PASS; `--pattern listener` 15/15 PASS (mcp-multi-listener, listener-conflict-* all green); unit `TestServiceRegistry_BuildsMCP` binds a real listener via the registry |
| default flavors keep mcp | build | `ZE_FEATURES` derives `ze_mcp` from the manifest so `ze`/`ze-appliance` link mcp; `ze-stripped` (`ze_core ze_ssh`) and bare `ze_core` drop it |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | MCP now shuts down via the `builtServices` deferred loop (after `apiServer.Stop`) instead of an explicit pre-stop call -- identical to how web/lg already behave; `http.Server.Shutdown` drains in-flight requests | `cmd/ze/hub/main.go` shutdown | accepted: consistent with web/lg registry services; no behavior regression |
| 2 | DISCARDED (pre-existing) | `ze-validate` flags 12 exported hub symbols (Reconfigurable, ListenerMigrator, Set*, ServiceDeps, ServiceFactory, MCPServerHandle) as lacking a cross-package caller | `listener_migrate.go`, `service_registry.go`, `service_mcp.go` | all present at HEAD (introduced by lg/web children); package-internal hub infra with same-package callers; `ze-verify` does not run validate.py |
| 3 | DISCARDED (other session) | `audit-test-relaxation` flags `internal/appliance/cmd_kernel_test.go` | internal/appliance | belongs to spec-kernel-build-consolidation (another session), not this spec |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE (the two findings above are pre-existing/other-session, not introduced by this spec)
- [x] All NOTEs recorded above (NOTE 1: shutdown ordering, accepted)

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/hub/service_mcp.go` | yes | `//go:build ze_mcp`; buildMCPService + mcpService + mcpCommandLister + moved mcp.go construction |
| `cmd/ze/hub/register_mcp.go` | yes | `//go:build ze_mcp`; `registerService("mcp", buildMCPService, SetMCP)` |
| `cmd/ze/hub/command_meta.go` | yes | always-on neutral commandMeta + commandMetaSource + generic helpers |
| `cmd/ze/hub/build_tag_mcp_present_test.go` / `_absent_test.go` | yes | present (`ze_mcp`) + absent (`!ze_mcp`, incl. config-reject + nm symbol check) |
| `cmd/ze/hub/service_mcp_test.go` | yes | `//go:build ze_mcp`; BuildsMCP + NotConfigured + MCPCommandLister conversion |
| `internal/component/plugin/all/all_ze_mcp.go` | yes | generated, `//go:build ze_mcp`, blank-imports `mcp/yang` |
| `cmd/ze/hub/mcp.go` | DELETED | content split into command_meta.go + service_mcp.go |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | mcp compiled in works | `ze_core ze_mcp` build OK; `bin/ze-test bgp parse --pattern mcp` 4/4 PASS; `TestServiceRegistry_BuildsMCP` PASS |
| AC-2 | bare ze_core drops mcp | `go tool nm` 0 `internal/component/mcp` symbols; `TestBuildTag_MCP_AbsentBinaryDropsMCPSymbols` PASS |
| AC-3 | no always-on mcp import; SetMCP widened | `dep_audit.py --check` clean; `SetMCP(s Reconfigurable)`; no always-on file names `*MCPServerHandle`/zemcp (grep) |
| AC-4 | generator emits all_ze_mcp.go | `ls all_ze_mcp.go`; generator `--check` current; `mcp/yang` removed from all.go |
| AC-5 | dep_audit clean (chaos not flagged) | `--check` exit 0; `--selftest` OK with the new non-prod fixture |
| AC-6 | no-mcp binary safe on `mcp {}` config | `environment { mcp {} }` -> "unknown field in environment: mcp", no panic; `TestBuildTag_MCP_AbsentRejectsMCPConfig` PASS |
| AC-7 | listener handling for other services | `bin/ze-test bgp parse --pattern listener` 15/15 PASS |
| AC-8 | ze keeps mcp, ze-stripped drops it | `ZE_FEATURES` includes `ze_mcp` (manifest-derived); `ze-stripped`=`ze_core ze_ssh` (no mcp) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | grep: production importers are hub-only (main.go/main_servers.go/mcp.go) + all.go (mcp/yang) |
| A-2 | confirmed | ServiceDeps.MCP fields are `[]string`/`string`/`zeconfig.MCPListenConfig`/generic func/`audit.Recorder` -- no zemcp type always-on |
| A-3 | confirmed | neutral `commandMetaSource` (command_meta.go); API + MCP adapt it independently; ze_core build (no ze_mcp) compiles API fine |
| A-4 | confirmed | `mcp {}` rejected cleanly on a no-mcp binary, no panic |
| A-5 | **broken** -> resolved | dep_audit WOULD have flagged `internal/chaos/*`; fixed by excluding `internal/chaos/`, `internal/test/` (DISABLEABLE_NONPROD_PREFIXES). See Mistake Log. |
| A-6 | confirmed | listener parse suite 15/15 PASS; absent mcp schema does not break other services |

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 demonstrated
- [ ] mcp registry-ized; no always-on mcp import; SetMCP widened
- [ ] mcp compile-out-able; present/absent build-tag tests pass
- [ ] dep_audit DISABLEABLE clean (chaos orchestrator not flagged)
- [ ] generator emits all_ze_mcp.go; `--check` passes
- [ ] `make ze-verify-changed` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (build-tag present/absent + config-safe)
- [ ] Goal Validation table filled with concrete evidence
