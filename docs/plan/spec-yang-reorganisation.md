# Spec: YANG Schema Reorganisation

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/yang/modules/ze-hub-conf.yang` - core environment schema (daemon/log/debug)
4. `internal/plugins/bgp/schema/ze-bgp-conf.yang` - BGP config + environment augment
5. `internal/yang/modules/ze-plugin-conf.yang` - plugin config + environment augment
6. `internal/config/environment.go` - Environment struct and envOptions table
7. `internal/yang/loader.go` - YANG loader with `LoadEmbedded()`

## Task

Three related improvements to YANG schema organization:

1. **YANG split (DONE):** Move environment containers from monolithic `ze-hub-conf.yang` to owning subsystems using YANG `augment`
2. **Schema relocation:** Move YANG files to the component/infrastructure that owns them
3. **Init-based registration:** Replace hardcoded `LoadEmbedded()` file list with init()-based module registration
4. **Dead field cleanup:** Remove environment fields that are never consumed at runtime

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config parsing pipeline
  → Constraint: File → Tree → ResolveBGPTree() → map[string]any → PeersFromTree()
- [ ] `docs/architecture/hub-architecture.md` - hub coordination role
  → Constraint: hub is orchestrator, not owner of subsystem settings

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/yang/modules/ze-hub-conf.yang` (91L) — core environment schema (daemon/log/debug)
- [ ] `internal/plugins/bgp/schema/ze-bgp-conf.yang` (479L) — BGP config + environment augments
- [ ] `internal/plugins/bgp/schema/ze-bgp-api.yang` (~8K) — BGP RPC/notification definitions
- [ ] `internal/plugins/bgp/schema/embed.go` (7L) — embeds YANG files as string variables
- [ ] `internal/yang/modules/ze-plugin-conf.yang` (65L) — plugin config + api environment augment
- [ ] `internal/yang/loader.go` (101L) — `LoadEmbedded()` with hardcoded file list
- [ ] `internal/config/environment.go` (701L) — Environment struct and envOptions table
- [ ] `internal/config/yang_schema.go` (470L) — schema loading/aggregation
- [ ] `internal/config/editor/validator.go` (280L) — config editor validation
- [ ] `internal/config/editor/completer.go` (~500L) — config editor autocompletion
- [ ] `internal/config/loader.go` (~460L) — CreateReactorFromTree, environment consumption
- [ ] `cmd/ze/schema/main.go` (622L) — schema CLI, buildSchemaRegistry

### Phase 1: YANG Split (COMPLETED)

~~The `ze-hub-conf.yang` schema was a monolithic `environment {}` block. Containers have been split:~~

| Container | Moved to | Mechanism |
|-----------|----------|-----------|
| `daemon` | `ze-hub-conf.yang` (kept) | Core engine setting |
| `log` | `ze-hub-conf.yang` (kept) | Core engine setting |
| `debug` | `ze-hub-conf.yang` (kept) | Core engine setting |
| `tcp` | `ze-bgp-conf.yang` | `augment "/hub:environment"` |
| `bgp` | `ze-bgp-conf.yang` | `augment "/hub:environment"` |
| `cache` | `ze-bgp-conf.yang` | `augment "/hub:environment"` |
| `reactor` | `ze-bgp-conf.yang` | `augment "/hub:environment"` |
| `api` | `ze-plugin-conf.yang` | `augment "/hub:environment"` |

~~Reason for strikethrough: Phase 1 is implemented but not yet committed.~~

### Phase 2: Schema File Locations (TODO)

**Source files read:**
- [ ] `internal/plugins/bgp/schema/embed.go` (7L) — embeds ze-bgp-conf.yang and ze-bgp-api.yang
- [ ] `internal/plugins/bgp/schema/ze-bgp-conf.yang` (479L) — BGP config + environment augments
- [ ] `internal/plugins/bgp/schema/ze-bgp-api.yang` (~8K) — BGP RPC/notification definitions
- [ ] `internal/yang/modules/ze-plugin-conf.yang` (65L) — plugin config + api environment augment
- [ ] `internal/yang/loader.go` (101L) — `LoadEmbedded()` with hardcoded file list

**Current schema locations and consumers:**

| YANG Module | Current location | Consumers (Go packages) |
|-------------|-----------------|------------------------|
| `ze-hub-conf.yang` | `internal/yang/modules/` (embedded via `//go:embed modules`) | config, config/editor, yang — via `LoadEmbedded()` |
| `ze-bgp-conf.yang` | `internal/plugins/bgp/schema/` (embed.go) | config, config/editor, yang, ipc, cmd/ze/schema — via `bgpschema.ZeBGPConfYANG` |
| `ze-bgp-api.yang` | `internal/plugins/bgp/schema/` (embed.go) | yang, ipc, cmd/ze/schema — via `bgpschema.ZeBGPAPIYANG` |
| `ze-plugin-conf.yang` | `internal/yang/modules/` (embedded via `//go:embed modules`) | config — via `LoadEmbedded()` |
| `ze-extensions.yang` | `internal/yang/modules/` | all YANG consumers — via `LoadEmbedded()` |
| `ze-types.yang` | `internal/yang/modules/` | all YANG consumers — via `LoadEmbedded()` |

**Key observation:** `ze-bgp-conf.yang` is consumed by the config system, editor, and CLI — NOT by plugin infrastructure. It defines subsystem configuration, not plugin behavior. Same for `ze-bgp-api.yang`. These belong at the component level.

Similarly, `ze-plugin-conf.yang` defines plugin configuration consumed by the config system, not by plugin internals. It belongs with the plugin infrastructure.

**Behavior to preserve:**
- Config file syntax unchanged
- Environment variable format unchanged
- Config editor validation and autocompletion
- All existing tests pass

### Phase 3: Init-Based YANG Registration (TODO)

**Current loading pattern (scattered across codebase):**

| Location | What it does |
|----------|-------------|
| `yang/loader.go:LoadEmbedded()` | Hardcoded list: extensions, types, hub-conf, plugin-conf |
| `config/yang_schema.go` | Manually loads `bgpschema.ZeBGPConfYANG` via `AddModuleFromText()` |
| `config/editor/validator.go` | Manually loads `bgpschema.ZeBGPConfYANG` via `AddModuleFromText()` |
| `config/editor/completer.go` | Manually loads `bgpschema.ZeBGPConfYANG` via `AddModuleFromText()` |
| `cmd/ze/schema/main.go` | Manually loads `bgpschema.ZeBGPConfYANG` via `AddModuleFromText()` |
| `yang/loader_test_helper.go` | Manually loads `bgpschema.ZeBGPConfYANG` for tests |

**Problem:** Adding a new YANG module requires touching 5+ files. The `bgpschema.ZeBGPConfYANG` manual loading is scattered everywhere.

**Key insight:** goyang's `Parse()` does not require dependency order — only `Resolve()` does. So init()-registered modules can be loaded in any order before resolving.

**Proposed pattern:** Each YANG-owning package registers its content via `init()` into a central YANG module registry (same pattern as `internal/plugin/registry/`). The Loader gains `LoadRegistered()` to load all init()-registered modules.

### Phase 4: Unused Environment Field Cleanup (TODO)

**Runtime consumption audit (from tracing `internal/config/loader.go` CreateReactorFromTree):**

| Container | Field | Used at runtime? | Consumer | Action |
|-----------|-------|-------------------|----------|--------|
| `tcp` | `port` | YES | `loader.go:376` normalizeListenAddr | Keep |
| `tcp` | `attempts` | YES | `loader.go:415` reactor.Config.MaxSessions | Keep |
| `tcp` | `delay` | NO | Never read | Remove |
| `tcp` | `acl` | NO | Never read | Remove |
| `bgp` | `connection` | NO | Per-peer only, env level never read | Remove container |
| `bgp` | `openwait` | NO | Helper exists but never called | Remove container |
| `cache` | `attributes` | NO | Never read | Remove container |
| `reactor` | `speed` | NO | Validation exists, never read | Remove |
| `reactor` | `cache-ttl` | NO | Never read | Remove |
| `reactor` | `cache-max` | YES | `loader.go:418` reactor.Config.RecentUpdateMax | Keep |
| `api` | all 9 fields | NO | None wired to subsystems | Remove container |
| `debug` | `pprof` | YES | `loader.go:428-436` starts HTTP pprof server | Keep (already in hub) |

**Summary:** Only 4 of 14 YANG environment fields are consumed: tcp.port, tcp.attempts, reactor.cache-max, debug.pprof (debug is in hub, not augmented).

## Data Flow (MANDATORY)

### Entry Point
- Config file → tokenizer → parser → Tree → schema validation (YANG)
- YANG modules → Loader → goyang Parse → Resolve → entry tree → schema nodes

### Transformation Path
1. YANG files embedded or init()-registered → Loader collects all modules
2. `Resolve()` resolves imports/augments across modules → unified entry tree
3. Entry tree → `yangToNode()` → config Schema nodes (for parsing/validation)
4. Config file parsed against Schema → Tree → `LoadEnvironmentWithConfig()` → Environment struct
5. Environment struct fields read by reactor startup (`CreateReactorFromTree`)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG file → Loader | `LoadEmbedded()` or `AddModuleFromText()` or init() registry | [ ] |
| Loader → Schema | `yangToNode()` in `yang_schema.go` | [ ] |
| Schema → Parser | `NewParser(schema)` in config package | [ ] |
| Parser → Environment | `LoadEnvironmentWithConfig()` dispatches via `envOptions` table | [ ] |
| Environment → Reactor | `CreateReactorFromTree()` reads struct fields | [ ] |

### Integration Points
- `internal/config/yang_schema.go` — loads all YANG schemas, must discover new schema locations
- `internal/config/editor/validator.go` — uses YANG for validation
- `internal/config/editor/completer.go` — uses YANG for autocomplete
- `cmd/ze/schema/main.go` — schema CLI, builds registry from all YANG

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Design

### Phase 2: Schema Relocation

| YANG file | Current path | New path | Rationale |
|-----------|-------------|----------|-----------|
| `ze-bgp-conf.yang` | `internal/plugins/bgp/schema/` | `internal/component/bgp/schema/` | Subsystem config, not plugin-specific |
| `ze-bgp-api.yang` | `internal/plugins/bgp/schema/` | `internal/component/bgp/schema/` | Subsystem API, not plugin-specific |
| `embed.go` | `internal/plugins/bgp/schema/` | `internal/component/bgp/schema/` | Moves with YANG files |
| `ze-plugin-conf.yang` | `internal/yang/modules/` | `internal/plugin/schema/` | Plugin infra owns its config schema |

Impact: Update all `bgpschema` import paths (26+ references across codebase).

### Phase 3: YANG Module Registry

| Component | Purpose |
|-----------|---------|
| `internal/yang/registry.go` | Central registry: `RegisterModule(name, content)` + `RegisteredModules() []Module` |
| Per-schema `init()` | Each schema package registers in `init()` |
| `Loader.LoadRegistered()` | Loads all init()-registered modules |
| `LoadEmbedded()` | Shrinks to truly foundational: extensions + types only |

Registration sources (post-Phase 2):

| Module | Registers from | Via init() in |
|--------|---------------|---------------|
| `ze-extensions.yang` | `internal/yang/` | Stays in `LoadEmbedded()` (bootstrap) |
| `ze-types.yang` | `internal/yang/` | Stays in `LoadEmbedded()` (bootstrap) |
| `ze-hub-conf.yang` | `internal/yang/` | Stays in `LoadEmbedded()` (bootstrap, other modules augment it) |
| `ze-bgp-conf.yang` | `internal/component/bgp/schema/` | `schema/register.go` |
| `ze-bgp-api.yang` | `internal/component/bgp/schema/` | `schema/register.go` |
| `ze-plugin-conf.yang` | `internal/plugin/schema/` | `schema/register.go` |

After Phase 3, manually loading `bgpschema.ZeBGPConfYANG` via `AddModuleFromText()` in 5 files becomes unnecessary — the Loader loads everything from the registry.

### Phase 4: Dead Field Cleanup

After removing unused fields, the augmented environment containers simplify to:

| Container | Remaining fields | Location |
|-----------|-----------------|----------|
| `tcp` | port, attempts | `ze-bgp-conf.yang` augment |
| `reactor` | cache-max | `ze-bgp-conf.yang` augment |
| `bgp` | (removed entirely) | — |
| `cache` | (removed entirely) | — |
| `api` | (removed entirely) | — |

Also requires cleanup in `internal/config/environment.go` — remove corresponding struct fields and `envOptions` entries.

### Open Questions

1. Should `ze-hub-conf.yang` stay in `internal/yang/modules/` or move to its own package? It's truly core (bootstrap dependency), so staying in `yang/modules/` seems right.
2. For init()-based registration, should we topologically sort or rely on goyang's `Resolve()` handling any order?
   → Decision: rely on `Resolve()` — `Parse()` doesn't need deps loaded first.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config file with `environment { tcp { port 1179; } }` | → | YANG validation accepts tcp under bgp schema | `TestYANGValidationTCPInBGPSchema` |
| Config editor autocomplete for `environment.tcp.` | → | Completer finds tcp leaves from bgp schema | `TestEditorAutocompleteTCPFromBGPSchema` |
| `ze config check` with environment block | → | All schemas aggregated, validation passes | `test/parse/environment-split.ci` |
| YANG registry loading | → | Init-registered modules loaded and resolved | `TestYANGRegistryLoadsAllModules` |
| Schema import from `component/bgp/schema` | → | All consumers find schemas at new path | `TestBuildSchemaRegistry` (existing, path updated) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze-hub-conf.yang` loaded | Contains only daemon, log, debug containers |
| AC-2 | `ze-bgp-conf.yang` loaded | Contains environment augments for tcp, reactor (used fields only) |
| AC-3 | `ze-plugin-conf.yang` loaded | Contains plugin container (no api augment after cleanup) |
| AC-4 | Config file with all environment sections | Parses and validates correctly |
| AC-5 | Config editor autocomplete | All used environment leaves discoverable |
| AC-6 | `LoadEnvironmentWithConfig()` | Unchanged behavior for used fields |
| AC-7 | `ze-bgp-conf.yang` path | Located in `internal/component/bgp/schema/` |
| AC-8 | `ze-plugin-conf.yang` path | Located in `internal/plugin/schema/` |
| AC-9 | YANG module loading | Uses init()-based registry, no manual `AddModuleFromText()` for registered modules |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestYANGHubSchemaContainers` | `internal/yang/loader_test.go` | AC-1: hub schema has only daemon/log/debug | |
| `TestYANGBGPSchemaEnvironment` | `internal/yang/loader_test.go` | AC-2: bgp schema augments tcp/reactor | |
| `TestYANGRegistryLoadsAllModules` | `internal/yang/registry_test.go` | AC-9: registry-based loading works | |
| `TestEnvironmentLoadUnchanged` | `internal/config/environment_test.go` | AC-6: existing tests still pass | |
| `TestLoader_EmbeddedModules` | `internal/yang/loader_test.go` | Hub is embedded, plugin-conf resolved | Updated |
| `TestLoader_ZeHubModule` | `internal/yang/loader_test.go` | Hub loads via LoadEmbedded | Updated |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `environment-split-validation` | `test/parse/environment-split.ci` | Config with used environment sections validates | |

## Files to Modify
- `internal/plugins/bgp/schema/` → move to `internal/component/bgp/schema/` (embed.go, ze-bgp-conf.yang, ze-bgp-api.yang)
- `internal/yang/modules/ze-plugin-conf.yang` → move to `internal/plugin/schema/` with new embed.go
- `internal/yang/loader.go` — update `LoadEmbedded()`, add `LoadRegistered()`
- `internal/yang/registry.go` — new file: YANG module registry
- `internal/config/yang_schema.go` — remove manual `AddModuleFromText()` for bgp
- `internal/config/editor/validator.go` — remove manual `AddModuleFromText()` for bgp
- `internal/config/editor/completer.go` — remove manual `AddModuleFromText()` for bgp
- `internal/yang/loader_test_helper.go` — remove manual `AddModuleFromText()` for bgp
- `cmd/ze/schema/main.go` — remove manual `AddModuleFromText()` for bgp, update imports
- `internal/config/environment.go` — remove unused envOptions entries and struct fields
- All files importing `bgpschema` (26+) — update import path to component/bgp/schema

### Files to Create
- `internal/yang/registry.go` — YANG module registry
- `internal/plugin/schema/embed.go` — embeds ze-plugin-conf.yang
- `internal/component/bgp/schema/register.go` — init() registration for bgp YANG
- `internal/plugin/schema/register.go` — init() registration for plugin YANG

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema locations | Yes | Schema files moved |
| Config editor | Yes | Automatic if YANG loading correct |
| Schema CLI | Yes | Import paths updated |
| Environment struct | Yes | Dead fields removed |
| Functional test | Yes | `test/parse/environment-split.ci` |

## Implementation Steps

### Phase 1: YANG Split (DONE)
1. ~~Move environment containers from hub to owning subsystems using YANG augment~~
2. ~~Move `ze-hub-conf.yang` to `internal/yang/modules/` (now embedded core)~~
3. ~~Delete `internal/hub/schema/` package~~
4. ~~Update all Go consumers to remove `hubschema` imports~~
5. ~~Add `ze-hub-conf` and `ze-plugin-conf` to `coreModules` in schema CLI~~

### Phase 2: Schema Relocation
1. Move `internal/plugins/bgp/schema/` → `internal/component/bgp/schema/`
2. Move `internal/yang/modules/ze-plugin-conf.yang` → `internal/plugin/schema/` with new `embed.go`
3. Update `LoadEmbedded()` to load plugin-conf from registry instead of filesystem
4. Update all `bgpschema` import paths (26+ files)
5. Update scripts, docs, Makefile references

### Phase 3: Init-Based YANG Registration
1. Create `internal/yang/registry.go` — `RegisterModule(name, content)` + `RegisteredModules()`
2. Add `init()` in `internal/component/bgp/schema/` to register ze-bgp-conf and ze-bgp-api
3. Add `init()` in `internal/plugin/schema/` to register ze-plugin-conf
4. Add `Loader.LoadRegistered()` — iterates registry, calls `AddModuleFromText()` for each
5. Remove manual `AddModuleFromText("ze-bgp-conf.yang", ...)` from 5 files
6. Remove `bgpschema` imports from config, editor, schema CLI where no longer needed

### Phase 4: Dead Field Cleanup
1. Remove unused YANG leaves: tcp.delay, tcp.acl, reactor.speed, reactor.cache-ttl
2. Remove unused YANG containers: bgp, cache, api
3. Remove corresponding `envOptions` entries and struct fields in `environment.go`
4. Update environment tests
5. Verify `make test-all`

### Failure Routing

| Failure | Route To |
|---------|----------|
| Import cycle after schema move | Check dependency direction component→plugin |
| init() order issues | Verify goyang Parse() is order-independent |
| Missing module after registry | Check blank imports trigger init() |
| Environment test failures | Check envOptions table matches remaining fields |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `schema.Define()` merges | It replaces | Phase 1 implementation | Used augment instead |
| Hub schema could stay external | Plugin-conf imports it | Phase 1 implementation | Moved to embedded core |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Separate `environment` containers per module | `schema.Define()` replaces, not merges | YANG `augment` pattern |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- YANG `augment` is already established in the codebase (GR, hostname plugins use it)
- goyang `Parse()` is order-independent; only `Resolve()` needs all modules loaded
- The plugin registry already uses init()-based registration — YANG registry follows same pattern
- `ze-bgp-conf.yang` defines subsystem config consumed by config/editor/CLI, not by plugin infra
- 10 of 14 environment fields are dead code — never consumed at runtime

## Implementation Summary

### Phase 1 (DONE — not yet committed)
- `ze-hub-conf.yang` trimmed to daemon/log/debug, moved to `internal/yang/modules/`
- `ze-bgp-conf.yang` augments environment with tcp/bgp/cache/reactor
- `ze-plugin-conf.yang` augments environment with api
- `internal/hub/schema/` package deleted
- All `hubschema` imports removed from Go code
- `ze-hub-conf` and `ze-plugin-conf` added to `coreModules` in schema CLI
- `LoadEmbedded()` updated to include ze-hub-conf
- `loader_test.go` updated: hub now verified as embedded, stale comments fixed
- All tests pass (`make test-all` exit 0)

### Phases 2-4
- Not yet implemented

### Documentation Updates
- `.claude/rationale/memory.md` — updated YANG paths

### Deviations from Plan
- `chaos` container was not in the original YANG (mentioned in plan but didn't exist)

## Implementation Audit

### Phase 1 Audit
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Hub schema keeps daemon/log/debug | ✅ Done | `internal/yang/modules/ze-hub-conf.yang` | |
| BGP schema augments tcp/bgp/cache/reactor | ✅ Done | `internal/plugins/bgp/schema/ze-bgp-conf.yang:451-477` | |
| Plugin schema augments api | ✅ Done | `internal/yang/modules/ze-plugin-conf.yang:51-63` | |
| Hub schema package deleted | ✅ Done | `internal/hub/schema/` removed | |
| All hubschema imports removed | ✅ Done | yang_schema.go, validator.go, completer.go, loader_test_helper.go | |
| coreModules updated | ✅ Done | `cmd/ze/schema/main.go:38-42` | Added ze-hub-conf, ze-plugin-conf |
| Tests pass | ✅ Done | `make test-all` exit 0 | |

### Phases 2-4 Audit
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make test-all` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven
- [ ] Architecture docs updated
- [ ] Critical Review passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Quality Gates
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed
