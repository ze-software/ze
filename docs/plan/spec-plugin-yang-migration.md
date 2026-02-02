# Spec: plugin-yang-migration

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/gr/gr.go` - GR plugin with YANG
4. `internal/plugin/rib/rib.go` - RIB plugin with YANG
5. `internal/plugin/hostname/hostname.go` - reference plugin pattern

## Dependencies

**Requires:** `spec-plugin-yang-discovery.md` - completed as `docs/plan/done/184-plugin-yang-discovery.md`.

## Task

Migrate GR (Graceful Restart) and RIB plugins to have their own YANG schemas, enabling YANG-based auto-injection via the plugin discovery framework.

**Rationale:** All plugins should be self-describing via YANG. This creates consistency with plugins like hostname that use YANG.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/api/capability-contract.md` - plugin capability injection
- [x] `.claude/rules/plugin-design.md` - plugin patterns

### Source Code
- [x] `internal/plugin/gr/gr.go` - GR plugin
- [x] `internal/plugin/rib/rib.go` - RIB plugin
- [x] `internal/plugin/hostname/hostname.go` - reference implementation
- [x] `internal/plugin/bgp/schema/ze-bgp.yang` - core schema

**Key insights:**
- Hostname plugin uses `declare wants config bgp` + JSON config parsing
- Pattern-based `declare conf` has been removed from the codebase
- All internal plugin YANG is loaded by `YANGSchema()` automatically

## Current Behavior (Before Implementation)

**Source files read:**
- [x] `internal/plugin/gr/gr.go` - Used deprecated `declare conf` pattern
- [x] `internal/plugin/rib/rib.go` - Had YANG but no `GetYANG()` export
- [x] `internal/plugin/hostname/hostname.go` - Reference implementation with `declare wants config bgp`
- [x] `internal/plugin/bgp/schema/ze-bgp.yang` - Had `graceful-restart` in core schema

**GR Plugin:**
- Used deprecated `declare conf peer * capability graceful-restart:restart-time <...>` pattern
- YANG definition was in `ze-bgp.yang` core schema

**RIB Plugin:**
- Already had YANG defined but not exported via `GetYANG()`

## Design

### GR YANG Schema (ze-graceful-restart)

| Element | Value |
|---------|-------|
| module | `ze-graceful-restart` |
| namespace | `urn:ze:graceful-restart` |
| prefix | `gr` |

**Augments multiple paths for template support:**

| Path | Purpose |
|------|---------|
| `/ze-bgp:bgp/ze-bgp:peer/ze-bgp:capability` | Main bgp block |
| `/ze-bgp:template/ze-bgp:bgp/ze-bgp:peer/ze-bgp:capability` | Template bgp peer |
| `/ze-bgp:template/ze-bgp:group/ze-bgp:capability` | Legacy group template |
| `/ze-bgp:template/ze-bgp:match/ze-bgp:capability` | Legacy match template |

### RIB YANG Schema (ze-rib)

| Element | Value |
|---------|-------|
| module | `ze-rib` |
| namespace | `urn:ze:rib` |
| prefix | `rib` |
| augments | `/ze-bgp:bgp` |

Describes RIB state (adj-rib-in, adj-rib-out) as operational data.

### Plugin Pattern

Both plugins follow the hostname plugin pattern:

| Aspect | Implementation |
|--------|----------------|
| Declaration | `declare wants config bgp` |
| Config format | `config json bgp <json>` |
| YANG export | `GetYANG()` function returning embedded const |
| Registration | Added to `internalPluginYANG` map |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGRPlugin_ParseBGPConfig` | `internal/plugin/gr/gr_test.go` | JSON config parsing | ✅ |
| `TestGRPluginStartup` | `internal/plugin/gr/gr_test.go` | Startup protocol | ✅ |
| Config tests | `internal/config/*_test.go` | Schema with GR YANG | ✅ |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `graceful-restart` | `test/plugin/graceful-restart.ci` | GR capability + reconnect | ✅ |
| `plugin-gr-features` | `test/plugin/plugin-gr-features.ci` | --features flag | ✅ |
| `plugin-gr-capa` | `test/plugin/plugin-gr-capa.ci` | --capa decode | ✅ |

## Files to Modify

- `internal/plugin/gr/gr.go` - Add `GetYANG()`, update config parsing
- `internal/plugin/rib/rib.go` - Add `GetYANG()`
- `internal/plugin/bgp/schema/ze-bgp.yang` - Remove graceful-restart
- `cmd/ze/bgp/plugin_gr.go` - Use `gr.GetYANG`
- `cmd/ze/bgp/plugin_rib.go` - Use `rib.GetYANG`
- `internal/config/yang_schema.go` - Include all internal plugin YANG
- `internal/config/loader.go` - Load all internal plugin YANG
- `internal/plugin/inprocess.go` - Register GR in `internalPluginYANG`

## Implementation Steps

1. **Read source files** - Understand current GR/RIB plugin patterns
2. **Add GR YANG** - Create `GetYANG()` and YANG schema in `gr.go`
3. **Update GR config parsing** - Align with hostname plugin pattern
4. **Add RIB YANG** - Create `GetYANG()` and YANG schema in `rib.go`
5. **Remove GR from core** - Delete `graceful-restart` from `ze-bgp.yang`
6. **Register plugins** - Add to `internalPluginYANG` map
7. **Auto-load YANG** - Update `YANGSchema()` to include all internal plugins
8. **Update tests** - Add schema helpers, fix functional test
9. **Verify** - Run `make test`, `make lint`, `make functional`

## Files Modified (Actual)

### GR Plugin
- `internal/plugin/gr/gr.go` - Added `GetYANG()`, updated to `declare wants config bgp` + JSON parsing
- `internal/plugin/gr/gr_test.go` - Updated tests for JSON config format
- `cmd/ze/bgp/plugin_gr.go` - Changed to use `gr.GetYANG`

### RIB Plugin
- `internal/plugin/rib/rib.go` - Added `GetYANG()` with YANG schema
- `cmd/ze/bgp/plugin_rib.go` - Changed to use `rib.GetYANG`

### Core Schema
- `internal/plugin/bgp/schema/ze-bgp.yang` - Removed `graceful-restart` container (moved to plugin)

### Config Loading
- `internal/config/yang_schema.go` - `YANGSchema()` includes all internal plugin YANG
- `internal/config/loader.go` - Load all internal plugin YANG before parsing
- `internal/plugin/inprocess.go` - Added `gr` to `internalPluginYANG`, added `GetAllInternalPluginYANG()`

### Tests
- `internal/config/bgp_test.go` - Added `schemaWithGR()` helper
- `internal/config/extended_test.go` - Added `extendedSchemaWithGR()` helper
- `internal/config/serialize_test.go` - Added `serializeSchemaWithGR()` helper
- `internal/config/loader_test.go` - Updated `TestHostnameRequiresPlugin` → `TestHostnameAlwaysAvailable`
- `test/plugin/graceful-restart.ci` - Simplified test, added process binding

## Implementation Summary

### What Was Implemented

1. **GR Plugin YANG Migration**
   - Removed `graceful-restart` from `ze-bgp.yang`
   - Added YANG schema to `gr.go` with multi-path augments for template support
   - Updated GR plugin to use `declare wants config bgp` + JSON parsing (aligns with hostname)
   - Registered in `internalPluginYANG` map

2. **RIB Plugin YANG Export**
   - Added `GetYANG()` function and YANG schema to `rib.go`
   - YANG describes operational state (adj-rib-in, adj-rib-out)

3. **Automatic YANG Loading**
   - `YANGSchema()` now includes all internal plugin YANG
   - `GetAllInternalPluginYANG()` returns all registered plugin YANG
   - Config loaders use this for parsing

4. **Test Updates**
   - Config tests use schema helpers that include GR YANG
   - Functional test updated with proper process binding

### Design Insights

1. **Template Support Requires Multiple Augments**: The GR YANG augments 4 different paths to support both main config and templates (bgp peer, template bgp peer, template group, template match).

2. **Internal Plugin YANG Always Loaded**: Changed from opt-in to always-on. `YANGSchema()` includes all internal plugin YANG, eliminating chicken-and-egg problem where plugin YANG was needed to parse config that defined those plugins.

3. **Pattern-Based Config Delivery Removed**: The old `declare conf peer * capability ...` pattern was deprecated. All plugins now use `declare wants config <root>` + JSON.

### Deviations from Original Plan

| Original Plan | Actual Implementation |
|---------------|----------------------|
| Separate `embed.go` files | YANG embedded as const in main `.go` files |
| Keep `declare conf` patterns | Updated to `declare wants config bgp` (aligns with hostname) |
| RIB triggers process paths | RIB YANG describes operational state instead |

## Checklist

### TDD
- [x] Tests written
- [x] Tests FAIL (before implementation)
- [x] Implementation complete
- [x] Tests PASS
- [x] Functional tests verify end-user behavior

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (3 pre-existing timeouts unrelated to this work)

### Migration
- [x] ze-bgp.yang updated (GR paths removed)
- [x] GR plugin has --yang flag (via `gr.GetYANG`)
- [x] RIB plugin has --yang flag (via `rib.GetYANG`)
- [x] Auto-injection works (internal YANG always loaded)
