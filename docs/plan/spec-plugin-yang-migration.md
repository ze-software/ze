# Spec: plugin-yang-migration

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-plugin-yang-discovery.md` - framework this depends on
4. `internal/plugin/gr/gr.go` - GR plugin to migrate
5. `internal/plugin/rib/rib.go` - RIB plugin to migrate

## Dependencies

**Requires:** `spec-plugin-yang-discovery.md` must be implemented first.

## Task

Migrate GR (Graceful Restart) and RIB plugins to have their own YANG schemas, enabling YANG-based auto-injection via the plugin discovery framework.

**Rationale:** All plugins should be self-describing via YANG. Currently GR and RIB use `declare conf` patterns but don't have YANG schemas. This creates inconsistency with new plugins (like hostname) that use YANG.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/capability-contract.md` - plugin capability injection
- [ ] `docs/architecture/config/yang-config-design.md` - YANG schema design

### Source Code
- [ ] `internal/plugin/gr/gr.go` - current GR plugin
- [ ] `internal/plugin/rib/rib.go` - current RIB plugin
- [ ] `internal/plugin/bgp/schema/ze-bgp.yang` - current schema with GR paths
- [ ] `cmd/ze/bgp/plugin_gr.go` - GR CLI entry
- [ ] `cmd/ze/bgp/plugin_rib.go` - RIB CLI entry

**Key insights:**
- GR uses `declare conf peer * capability graceful-restart:restart-time <restart-time:\d+>`
- RIB uses `declare conf peer * process * receive update` patterns
- Both need YANG for discovery framework consistency

## Current State

### GR Plugin

**Config paths in ze-bgp.yang:**
```
peer/capability/graceful-restart (container)
peer/capability/graceful-restart/restart-time (leaf uint16)
```

**Current declare conf:**
```
declare conf peer * capability graceful-restart:restart-time <restart-time:\d+>
```

### RIB Plugin

**Config paths (process binding):**
```
peer/process/NAME/receive/update
peer/process/NAME/send/update
```

**Current declare conf:**
```
declare conf peer * process * receive update
declare conf peer * process * send update
```

## Design

### GR YANG Schema (ze-gr.yang)

| Element | Value |
|---------|-------|
| module | `ze-gr` |
| namespace | `urn:ze:gr` |
| prefix | `gr` |
| augments | `/ze-bgp:bgp/ze-bgp:peer/ze-bgp:capability` |

**Schema structure:**

| Path | Type | Description |
|------|------|-------------|
| `capability/graceful-restart` | container | GR capability (RFC 4724) |
| `capability/graceful-restart/restart-time` | leaf uint16 | Restart time 0-4095 seconds |

### RIB YANG Schema (ze-rib.yang)

| Element | Value |
|---------|-------|
| module | `ze-rib` |
| namespace | `urn:ze:rib` |
| prefix | `rib` |
| augments | `/ze-bgp:bgp/ze-bgp:peer/ze-bgp:process` |

**Schema structure:**

| Path | Type | Description |
|------|------|-------------|
| `process/*/receive/update` | presence | Receive UPDATE messages |
| `process/*/send/update` | presence | Send UPDATE messages |

Note: RIB YANG describes which process config paths trigger RIB plugin, not RIB-specific config.

### CLI --yang Flag

Both plugins add `--yang` flag:

| Command | Output |
|---------|--------|
| `ze bgp plugin gr --yang` | ze-gr.yang content |
| `ze bgp plugin rib --yang` | ze-rib.yang content |

### Migration Path

1. Create plugin YANG files
2. Add `--yang` flag to CLI entries
3. Remove paths from ze-bgp.yang (moved to plugins)
4. Update plugins to embed and output YANG
5. Verify auto-injection works

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGRPluginYANG` | `internal/plugin/gr/gr_test.go` | --yang outputs valid YANG | |
| `TestRIBPluginYANG` | `internal/plugin/rib/rib_test.go` | --yang outputs valid YANG | |
| `TestGRAutoInjection` | `internal/config/loader_test.go` | GR config triggers injection | |
| `TestRIBAutoInjection` | `internal/config/loader_test.go` | RIB config triggers injection | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gr-yang-output` | `test/plugin/gr-yang.ci` | `ze bgp plugin gr --yang` works | |
| `rib-yang-output` | `test/plugin/rib-yang.ci` | `ze bgp plugin rib --yang` works | |
| `graceful-restart` | `test/plugin/graceful-restart.ci` | Existing GR test still passes | |
| `rib-reconnect` | `test/plugin/rib-reconnect.ci` | Existing RIB test still passes | |

## Files to Create

### GR Plugin
- `internal/plugin/gr/embed.go` - Embedded YANG
- `internal/plugin/gr/ze-gr.yang` - YANG schema

### RIB Plugin
- `internal/plugin/rib/embed.go` - Embedded YANG
- `internal/plugin/rib/ze-rib.yang` - YANG schema

## Files to Modify

### GR Plugin
- `cmd/ze/bgp/plugin_gr.go` - Add `--yang` flag
- `internal/plugin/gr/gr.go` - Add YANG getter method

### RIB Plugin
- `cmd/ze/bgp/plugin_rib.go` - Add `--yang` flag
- `internal/plugin/rib/rib.go` - Add YANG getter method

### Core Schema
- `internal/plugin/bgp/schema/ze-bgp.yang` - Remove graceful-restart container (moved to plugin)

## Implementation Steps

### Phase 1: GR Plugin YANG

1. **Write GR YANG tests** - `internal/plugin/gr/gr_test.go`
   - Test `--yang` outputs valid YANG
   → **Review:** Tests YANG content?

2. **Create ze-gr.yang** - `internal/plugin/gr/ze-gr.yang`
   - Augment capability container with graceful-restart
   → **Review:** Valid YANG? Matches current config?

3. **Add embed.go** - `internal/plugin/gr/embed.go`
   - Embed ze-gr.yang via `//go:embed`

4. **Add --yang flag** - `cmd/ze/bgp/plugin_gr.go`
   - Output embedded YANG when flag present
   → **Review:** Flag works?

5. **Run tests** - Verify PASS

### Phase 2: RIB Plugin YANG

6. **Write RIB YANG tests** - `internal/plugin/rib/rib_test.go`
   - Test `--yang` outputs valid YANG

7. **Create ze-rib.yang** - `internal/plugin/rib/ze-rib.yang`
   - Define process receive/send paths
   → **Review:** Triggers RIB injection correctly?

8. **Add embed.go** - `internal/plugin/rib/embed.go`

9. **Add --yang flag** - `cmd/ze/bgp/plugin_rib.go`

10. **Run tests** - Verify PASS

### Phase 3: Schema Migration

11. **Update ze-bgp.yang** - Remove graceful-restart from core
    - Paths now in ze-gr.yang
    → **Review:** Schema still valid? No broken references?

12. **Test auto-injection** - Verify plugins auto-inject
    - Config with graceful-restart → GR plugin injected
    - Config with process receive update → RIB plugin injected

### Phase 4: Verify

13. **Run existing tests** - Ensure no regression
    - `test/plugin/graceful-restart.ci`
    - `test/plugin/rib-reconnect.ci`
    - `test/plugin/rib-reconnect-simple.ci`

14. **Verify all** - `make verify`

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Separate YANG per plugin | Yes | Each plugin self-describes |
| Keep declare conf patterns | Yes | YANG for discovery, declare conf for config routing |
| RIB YANG describes triggers | Yes | RIB doesn't add new config, just claims process paths |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Migration
- [ ] ze-bgp.yang updated (GR paths removed)
- [ ] GR plugin has --yang flag
- [ ] RIB plugin has --yang flag
- [ ] Auto-injection works for both
