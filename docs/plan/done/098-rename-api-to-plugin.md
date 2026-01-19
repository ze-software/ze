# Plan: Rename `zebgp api` → `zebgp plugin`

**Status:** Ready for implementation
**Created:** January 2026
**Type:** Mechanical refactoring (no behavior change)

---

## Overview

Rename the command-line interface from `zebgp api <subcommand>` to `zebgp plugin <subcommand>`. This affects CLI usage, configuration files, documentation, and test infrastructure.

### Design Decision: Option A (Minimal Change)

| Component | Action | Reason |
|-----------|--------|--------|
| **CLI Command** | `zebgp api` → `zebgp plugin` | Clearer terminology |
| **Package Directory** | `internal/plugin/` → `internal/plugin/` | Match new command name |
| **Internal Functions** | `cmdAPI*` → `cmdPlugin*` | Consistency |
| **Config Blocks** | **KEEP** `api { ... }` | Refers to API protocol |
| **Config Types** | **KEEP** `APIBindings` | Refers to API protocol |
| **Config Schema** | **KEEP** `Field("api", ...)` | No config syntax change |

### Rationale

- **Config blocks** (`api my-process { ... }`) refer to the **API protocol** between ZeBGP and external processes
- **CLI command** (`zebgp api`) was confusing - users thought it meant "API" vs "plugin"
- **Package name** should match CLI command for clarity
- **No backward compatibility** - hard break per user decision

---

## 1. Command Line Interface (`cmd/zebgp/`)

### 1.1 main.go Changes

| Line | Current | New |
|------|---------|-----|
| 34 | `case "api":` | `case "plugin":` |
| 35 | `os.Exit(cmdAPI(os.Args[2:]))` | `os.Exit(cmdPlugin(os.Args[2:]))` |
| 95 | `api <subcommand>     API plugins (rr for route server)` | `plugin <subcommand>     Plugin system (rr for route server)` |

### 1.2 File Renames

```bash
git mv cmd/zebgp/api.go cmd/zebgp/plugin.go
git mv cmd/zebgp/api_rr.go cmd/zebgp/plugin_rr.go
git mv cmd/zebgp/api_persist.go cmd/zebgp/plugin_persist.go
```

### 1.3 Function Renames

| File | Function | New Function |
|------|-----------|--------------|
| `plugin.go` | `cmdAPI()` | `cmdPlugin()` |
| `plugin.go` | `apiUsage()` | `pluginUsage()` |
| `plugin_rr.go` | `cmdAPIRR()` | `cmdPluginRR()` |
| `plugin_persist.go` | `cmdAPIPersist()` | `cmdPluginPersist()` |

### 1.4 String Updates in plugin.go

| Line | Current | New |
|------|---------|-----|
| 24 | `"unknown api subcommand"` | `"unknown plugin subcommand"` |
| 31 | `Usage: zebgp api <subcommand>` | `Usage: zebgp plugin <subcommand>` |
| 33 | `API Subcommands:` | `Plugin Subcommands:` |
| 38 | `The api subcommands run as API processes` | `The plugin subcommands run as API processes` |
| 44 | `run "zebgp api rr";` | `run "zebgp plugin rr";` |
| 48 | `run "zebgp api persist";` | `run "zebgp plugin persist";` |

### 1.5 Comment Updates

| File | Line | Current | New |
|------|------|---------|-----|
| `plugin_rr.go` | 9 | `Run as Route Server API plugin` | `Run as Route Server plugin` |
| `plugin_persist.go` | 9 | `Run as route persistence API plugin` | `Run as route persistence plugin` |

---

## 2. Package Rename: `internal/plugin/` → `internal/plugin/`

### 2.1 Directory Rename

```bash
git mv internal/plugin internal/plugin
```

### 2.2 Package Declarations (66 files)

All files in `internal/plugin/` need package declaration updated:

```
package api → package plugin
```

**Files to update:**

#### internal/plugin/*.go (59 files)
```
capability_injection_test.go, command.go, command_test.go, commit.go,
commit_manager.go, commit_manager_test.go, commit_test.go,
config_delivery_test.go, decode.go, decode_test.go, errors.go,
filter.go, filter_parse_test.go, filter_test.go, forward.go,
forward_test.go, handler.go, handler_test.go, json.go, json_test.go,
message_receiver_test.go, mpwire.go, mpwire_test.go, nexthop.go,
nexthop_test.go, pending.go, pending_test.go, plugin.go, plugin_test.go,
process.go, process_test.go, raw.go, registration.go, registration_test.go,
registry.go, registry_sharing_test.go, registry_test.go, route.go,
route_keywords.go, route_parse_test.go, selector.go, selector_test.go,
server.go, server_test.go, session.go, session_test.go,
startup_coordinator.go, startup_test.go, text.go, text_test.go,
types.go, update_text.go, update_text_test.go, update_wire.go,
update_wire_test.go, wire_update.go, wire_update_split.go,
wire_update_split_test.go, wire_update_test.go
```

#### internal/plugin/rr/*.go (5 files)
```
peer.go, rib.go, rib_test.go, server.go, server_test.go
```

#### internal/plugin/persist/*.go (2 files)
```
event.go, persist.go
```

**Subpackages** (no change to package name):
- `internal/plugin/rr/` → `package rr` (keep)
- `internal/plugin/persist/` → `package persist` (keep)

### 2.3 Import Path Updates (18 files)

| File | Old Import | New Import |
|------|-----------|-----------|
| `cmd/zebgp/plugin_rr.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin/rr"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin/rr"` |
| `cmd/zebgp/plugin_persist.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin/persist"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin/persist"` |
| `cmd/zebgp/encode.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/config/bgp.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/config/loader.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/forward_split_test.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/mup_test.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/peer_test.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/peer.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/peersettings.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/reactor_batch_test.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/reactor_test.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/reactor.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/received_update_test.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/received_update.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/recent_cache_test.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/watchdog_test.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |
| `internal/reactor/session.go` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` | `"codeberg.org/thomas-mangin/zebgp/internal/plugin"` |

---

## 3. Test Data and Configuration Files

### 3.1 Directory Rename

```bash
git mv test/data/api test/data/plugin
```

This directory contains **110 files** (.conf, .ci, .run files for functional tests).

### 3.2 Config Content Changes

Only 1 file needs content update:

| File | Line | Current | New |
|------|------|---------|-----|
| `test/data/plugin/reconnect.conf` | 6 | `run "zebgp api persist";` | `run "zebgp plugin persist";` |

### 3.3 Config Blocks (UNCHANGED)

Config blocks `api { ... }` remain unchanged. These refer to API protocol, not CLI command:

```conf
peer 127.0.0.1 {
    api my-process {          # UNCHANGED - refers to API protocol
        receive { update; state; }
        send { update; }
    }
}
```

Internal config types remain unchanged:
- `APIBindings` (not `PluginBindings`)
- `PeerAPIBinding` (not `PeerPluginBinding`)
- `Field("api", ...)` (not `Field("plugin", ...)`)

---

## 4. Documentation Files

### 4.1 Spec File Renames

```bash
git mv docs/plan/spec-api-rr.md docs/plan/spec-plugin-rr.md
```

Other spec files refer to "api" as protocol concept (not CLI), keep names:
- `spec-api-capability-contract.md` - API capability contract (protocol)
- `spec-api-command-serial.md` - API command serialization
- `spec-api-plugin-commands.md` - Review: may rename to `spec-plugin-commands.md`
- `spec-api-sync.md` - API synchronization

### 4.2 Files with `zebgp api` CLI References (4 files)

| File | Changes |
|------|---------|
| `docs/plan/spec-plugin-rr.md` | All `zebgp api rr/persist` → `zebgp plugin rr/persist` |
| `docs/architecture/rib-transition.md` | Lines 257, 259: Task descriptions update |
| `docs/architecture/api/CAPABILITY_CONTRACT.md` | Lines 55, 180-181: Reference updates |

### 4.3 Files with `internal/plugin/` Path References (55 files)

Run systematic search and replace:

```bash
# Find all docs with internal/plugin references
grep -rl "internal/plugin" docs/ --include="*.md" | wc -l  # 55 files

# Categories:
# - docs/plan/done/*.md (historical specs - 40+ files)
# - docs/plan/*.md (active specs - 10+ files)
# - docs/architecture/*.md (architecture docs - 5+ files)
```

**Key files requiring manual review:**
- `docs/architecture/api/ARCHITECTURE.md`
- `docs/architecture/api/PROCESS_PROTOCOL.md`
- `docs/architecture/UPDATE_BUILDING.md`
- `docs/architecture/ENCODING_CONTEXT.md`
- `docs/architecture/overview.md`
- `docs/exabgp/EXABGP_CODE_MAP.md`

### 4.4 Documentation Notes

- Replace all `zebgp api` command-line references with `zebgp plugin`
- Replace all `internal/plugin/` directory references with `internal/plugin/`
- Keep `api { ... }` config block syntax in documentation
- Keep API protocol terminology in conceptual docs

---

## 5. Test Framework

### 5.1 Test Files

| File | Line | Current | New |
|------|------|---------|-----|
| `test/cmd/functional/main.go` | 57 | `case "encoding", "api":` | `case "encoding", "plugin":` |

### 5.2 Test Data Directory

The `test/data/api/` directory (110 files) is renamed in Phase 3. Functional test runner references updated in Phase 5.

---

## 6. 🧪 TDD Verification

This is a refactoring task. Existing tests validate no regression.

### Unit Tests
| Test Suite | Location | Validates |
|------------|----------|-----------|
| All internal/plugin tests | `internal/plugin/**/*_test.go` | Package functionality unchanged |
| Reactor tests | `internal/reactor/*_test.go` | Import path updates work |
| Config tests | `internal/config/*_test.go` | Config loading unchanged |

### Functional Tests
| Category | Location | Validates |
|----------|----------|-----------|
| API tests | `test/data/api/*.ci` | Plugin behavior unchanged |
| All 37 tests | `make functional` | End-to-end functionality |

### Verification Sequence
1. **Before changes**: `make test && make lint && make functional` (baseline)
2. **After each phase**: `go build ./...` (compilation)
3. **After all phases**: `make test && make lint && make functional` (regression)

---

## 7. Implementation Phases

### Phase 1: Package Rename (Foundation)
1. Rename directory: `git mv internal/plugin internal/plugin`
2. Update package declarations: 66 files (`package api` → `package plugin`)
3. Update import paths: 18 files
4. Verify compilation: `go build ./...`

### Phase 2: CLI Rename
5. Rename cmd files: `git mv cmd/zebgp/api.go cmd/zebgp/plugin.go` (etc.)
6. Update function names: `cmdAPI*` → `cmdPlugin*`, `apiUsage` → `pluginUsage`
7. Update main.go: Switch statement and function calls
8. Update strings: Help text, error messages, comments

### Phase 3: Test Data Directory Rename
9. Rename directory: `git mv test/data/api test/data/plugin`
10. Update content: `test/data/plugin/reconnect.conf`

### Phase 4: Documentation Updates
11. Rename spec: `git mv docs/plan/spec-api-rr.md docs/plan/spec-plugin-rr.md`
12. Update `zebgp api` → `zebgp plugin` in 4 docs
13. Update `internal/plugin/` → `internal/plugin/` in 55 docs

### Phase 5: Test Framework Updates
14. Update: `test/cmd/functional/main.go` (case statement)

### Phase 6: Verification
15. Run: `make test`
16. Run: `make lint`
17. Run: `make functional`
18. Grep verification: Ensure no old references remain

---

## 8. Verification Checklist

### Code Verification
- [ ] `grep -r "zebgp api" . --exclude-dir=.git` shows only intentional config block references
- [ ] `grep -r "cmdAPI" . --include="*.go"` returns no results
- [ ] `grep -r "apiUsage" . --include="*.go"` returns no results
- [ ] `grep -r '"codeberg.org/thomas-mangin/zebgp/internal/plugin"' . --include="*.go"` returns no results
- [ ] `grep -r '^package api' internal/plugin/` returns no results
- [ ] All Go files compile: `go build ./...`

### Test Verification
- [ ] All unit tests pass: `make test`
- [ ] All lint checks pass: `make lint`
- [ ] All functional tests pass: `make functional`

### CLI Verification
- [ ] `zebgp` shows `plugin <subcommand>` in help
- [ ] `zebgp plugin` shows subcommands (rr, persist)
- [ ] `zebgp plugin rr` runs without error
- [ ] `zebgp plugin persist` runs without error
- [ ] `zebgp plugin help` shows correct help text

### Documentation Verification
- [ ] No `zebgp api` command-line references in docs
- [ ] No `internal/plugin/` path references in docs (use `internal/plugin/`)
- [ ] Config examples use `zebgp plugin rr/persist`
- [ ] Config blocks still use `api { ... }` syntax

---

## 9. User Migration Guide

### Command Line Migration

**Before:**
```bash
zebgp api rr
zebgp api persist
zebgp api help
```

**After:**
```bash
zebgp plugin rr
zebgp plugin persist
zebgp plugin help
```

### Configuration Migration

**Before:**
```conf
process rr {
    run "zebgp api rr";
    encoder json;
}

process persist {
    run "zebgp api persist";
    encoder json;
}
```

**After:**
```conf
process rr {
    run "zebgp plugin rr";
    encoder json;
}

process persist {
    run "zebgp plugin persist";
    encoder json;
}
```

### Config Blocks (No Change Required)

```conf
peer 127.0.0.1 {
    api my-process {          # Still "api" - refers to API protocol
        receive { update; state; }
        send { update; }
    }
}
```

---

## 10. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Package rename breaks imports | High | Systematic update of all 18 import statements |
| Test failures | Medium | Comprehensive test suite (unit + functional) |
| Documentation inconsistency | Medium | Systematic review of all docs |
| Forgotten references | Low | Comprehensive grep search patterns |
| Config syntax confusion | Low | Keep `api { ... }` blocks, only change `run` command |
| Breaking change for users | High | Clear migration guide, hard break per user decision |

---

## 11. Search Patterns for Verification

```bash
# Check for old command-line references
grep -r "zebgp api" . --exclude-dir=.git --exclude-dir=node_modules

# Check for old function names
grep -r "cmdAPI" . --include="*.go"
grep -r "apiUsage" . --include="*.go"

# Check for old package imports
grep -r '"codeberg.org/thomas-mangin/zebgp/internal/plugin"' . --include="*.go"

# Check for old package declarations
grep -r "^package api" internal/plugin/

# Verify new package
grep -r "^package plugin" internal/plugin/

# Check for internal/plugin path references
grep -r "internal/plugin" docs/
```

---

## 12. File Change Summary

### By Category

| Category | Count | Details |
|----------|--------|---------|
| **Go Files (pkg)** | 66 | Package declarations (59 root + 5 rr + 2 persist) |
| **Go Files (imports)** | 18 | Import path updates |
| **Test Data Files** | 110 | Directory rename (no content changes except 1) |
| **Documentation Files** | 55 | `internal/plugin/` path references |
| **Test Framework Files** | 1 | `test/cmd/functional/main.go` |
| **Total** | **~250** | Files affected |

### By Operation

| Operation | Count | Details |
|-----------|--------|---------|
| **Directory Renames** | 2 | `internal/plugin/` → `internal/plugin/`, `test/data/api/` → `test/data/plugin/` |
| **File Renames** | 4 | 3 cmd files + 1 spec file |
| **Package Declarations** | 66 | `package api` → `package plugin` |
| **Import Paths** | 18 | `internal/plugin` → `internal/plugin` |
| **Doc Path Updates** | 55 | `internal/plugin/` → `internal/plugin/` in markdown |
| **Content Updates** | 5 | CLI strings, config file, test framework |

---

## 13. References

### Related Specs
- `docs/plan/spec-plugin-rr.md` - Plugin specifications (renamed)
- `docs/architecture/rib-transition.md` - Architecture with plugin references
- `docs/architecture/api/CAPABILITY_CONTRACT.md` - API capability contract

### Related Code
- `cmd/zebgp/` - Command-line interface
- `internal/plugin/` (formerly `internal/plugin/`) - Plugin implementation
- `internal/config/` - Configuration parsing
- `test/cmd/functional/` - Functional test framework

---

## 14. Completion

After implementation:

```bash
# Move plan to done/
LAST=$(command ls -1 docs/plan/done/ 2>/dev/null | sort -n | tail -1 | cut -c1-3)
test -z "$LAST" && LAST=0
NEXT=$(printf "%03d" $((LAST + 1)))
git mv docs/plan/rename-api-to-plugin.md docs/plan/done/${NEXT}-rename-api-to-plugin.md
```

Include moved plan in same commit as code changes.
