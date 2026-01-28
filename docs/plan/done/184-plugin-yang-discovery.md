# Spec: plugin-yang-discovery

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/process.go` - where plugins are started
4. `internal/plugin/inprocess.go` - internal plugin registry
5. `cmd/ze/bgp/server.go` - where `--plugin` flag is added

## Task

Create a plugin loading framework with `--plugin` CLI flag and `--yang` introspection that allows explicit plugin selection or auto-discovery.

**Rationale:** Plugins should be explicitly selected by the user, not auto-injected based on config content. Auto-discovery is opt-in via `--plugin auto`.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/api/capability-contract.md` - plugin capability injection
- [x] `docs/architecture/api/process-protocol.md` - 5-stage plugin startup

### Source Code
- [x] `internal/plugin/process.go` - current plugin process management
- [x] `internal/plugin/server.go` - plugin server, startup coordinator
- [x] `cmd/ze/bgp/server.go` - server command (--plugin flag)
- [x] `cmd/ze/main.go` - main command (--plugin discovery)

**Key insights:**
- Internal plugins run via goroutine + io.Pipe (no fork)
- External plugins fork via `exec.Command`
- Both use same stdin/stdout protocol with 5-stage startup

## Design

### CLI `--plugin` Flag

Add `--plugin` flag to `ze bgp server` command. Repeatable for multiple plugins.

| Usage | Effect |
|-------|--------|
| `ze bgp server config.conf` | No plugins (unless config declares them) |
| `ze bgp server --plugin ze.rib config.conf` | Load internal RIB plugin (in-process) |
| `ze bgp server --plugin ze.rib --plugin ze.gr config.conf` | Load multiple internal plugins |
| `ze bgp server --plugin ./myplugin config.conf` | Fork local binary |
| `ze bgp server --plugin "ze bgp plugin rr" config.conf` | Fork ze with args |
| `ze bgp server --plugin auto config.conf` | Auto-discover all available |

### Plugin Execution Model

| Type | Execution | Isolation |
|------|-----------|-----------|
| Internal (`ze.X`) | Goroutine + `io.Pipe` | Shared memory, crash = reactor crash |
| External (path/cmd) | Fork + `exec.Command` | Process isolation, can restart |

**Internal plugin execution:**
1. Create `io.Pipe()` for stdin and stdout
2. Start plugin as goroutine: `go rib.NewRIBManager(pipeReader, pipeWriter).Run()`
3. Reactor reads/writes to pipe ends (same protocol as external)
4. No code changes needed in plugin implementations

**Benefits of in-process:**
- Faster startup (no fork/exec overhead)
- Lower memory (shared address space)
- Simpler debugging

**Tradeoffs:**
- Plugin crash = reactor crash
- Cannot restart individual internal plugins

### Plugin Resolution Rules

| Pattern | Type | Action |
|---------|------|--------|
| `ze.X` | Internal | Run in-process via goroutine |
| `./path` | External | Fork binary at relative path |
| `/path` | External | Fork binary at absolute path |
| `auto` | Discovery | Call `ze --plugin`, load all discovered |
| `"cmd args"` | External | Split on whitespace, fork with args |

**Internal plugin registry:**

| Name | Implementation |
|------|----------------|
| `ze.rib` | `rib.NewRIBManager(in, out).Run()` |
| `ze.gr` | `gr.NewGRPlugin(in, out).Run()` |
| `ze.rr` | `rr.NewRouteServer(in, out).Run()` |

### Plugin Sources (All Additive)

Plugins can come from three sources, all combined:

| Source | Priority | Example |
|--------|----------|---------|
| CLI `--plugin` | 1 (highest) | `--plugin ze.rib` |
| Config `plugin {}` | 2 | `plugin { external rib { run "..."; } }` |
| Config `process {}` | 3 | `process foo { run "..."; }` |

Deduplication: Same plugin from multiple sources = one instance.

### Discovery CLI (`ze --plugin`)

For `--plugin auto` mode, discover available plugins:

| Command | Output |
|---------|--------|
| `ze --plugin` | List of plugin commands (one per line) |

Example output:
```
bgp plugin gr
bgp plugin rib
bgp plugin rr
```

### YANG Introspection (`--yang`)

Each plugin supports `--yang` flag to output its YANG schema:

| Command | Output |
|---------|--------|
| `ze bgp plugin rib --yang` | YANG module text (empty if none) |
| `ze bgp plugin gr --yang` | YANG module text (empty if none) |

Used by tooling to discover plugin capabilities and config schema.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolvePlugin` | `internal/plugin/resolve_test.go` | Resolution rules | ✅ |
| `TestInternalPluginRegistry` | `internal/plugin/resolve_test.go` | Registry lookup | ✅ |
| `TestMergeCliPlugins` | `internal/config/loader_test.go` | CLI + config merge | ✅ |
| `TestMergeCliPluginsInternal` | `internal/config/loader_test.go` | Internal flag set | ✅ |
| `TestAvailablePlugins` | `cmd/ze/main_test.go` | `ze --plugin` output | ✅ |
| `TestInternalPluginRunnerRegistry` | `internal/plugin/inprocess_test.go` | Runner lookup | ✅ |
| `TestGetInternalPluginRunner` | `internal/plugin/inprocess_test.go` | Runner functions | ✅ |
| `TestProcessInternalPlugin` | `internal/plugin/process_test.go` | In-process start | ✅ |
| `TestProcessInternalPluginUnknown` | `internal/plugin/process_test.go` | Unknown error | ✅ |
| `TestProcessInternalPluginStop` | `internal/plugin/process_test.go` | Clean shutdown | ✅ |
| `TestDeriveName` | `internal/plugin/inprocess_test.go` | Name derivation | ✅ |
| `TestDeriveNameEdgeCases` | `internal/plugin/inprocess_test.go` | Edge cases | ✅ |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Plugin name | 1-64 chars | 64 char name | empty string | 65 char name |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing plugin tests | `test/data/plugin/*.ci` | Plugin functional tests | ✅ (26 pass) |

## Files to Modify

### Phase 1: CLI Infrastructure
- `cmd/ze/main.go` - Added `--plugin` flag, uses `AvailableInternalPlugins()`
- `cmd/ze/main_test.go` - Tests for available plugins
- `cmd/ze/bgp/server.go` - Added `--plugin` flag (repeatable)
- `cmd/ze/bgp/plugin_rib.go` - Added `--yang` flag
- `cmd/ze/bgp/plugin_gr.go` - Added `--yang` flag
- `cmd/ze/bgp/plugin_rr.go` - Added `--yang` flag
- `internal/plugin/resolve.go` - Plugin resolution, uses `internalPluginRunners`
- `internal/plugin/resolve_test.go` - Resolution tests
- `internal/config/loader.go` - `LoadReactorFileWithPlugins()`, `mergeCliPlugins()`
- `internal/config/loader_test.go` - Merge tests

### Phase 2: In-Process Execution
- `internal/config/bgp.go` - Added `Internal bool` to `PluginConfig`
- `internal/plugin/types.go` - Added `Internal bool` to `PluginConfig`
- `internal/plugin/bgp/reactor/reactor.go` - Added `Internal bool` to `PluginConfig`
- `internal/plugin/process.go` - Split into `startInternal()`/`startExternal()`, fixed `Stop()`
- `internal/plugin/inprocess.go` - Registry + `InternalPluginRunner` type
- `internal/plugin/inprocess_test.go` - Registry and deriveName tests
- `internal/plugin/process_test.go` - Internal plugin tests

## Implementation Steps

### Phase 1: CLI Infrastructure ✅

1. ✅ Add `--plugin` to `ze bgp server`
2. ✅ Add plugin resolution (`internal/plugin/resolve.go`)
3. ✅ Add `ze --plugin` discovery
4. ✅ Add `--yang` to plugin commands
5. ✅ Add merge logic (`mergeCliPlugins`)

### Phase 2: In-Process Execution ✅

6. ✅ Create internal plugin registry (`internal/plugin/inprocess.go`)
7. ✅ Add `Internal` field to `PluginConfig` (3 locations)
8. ✅ Modify `Process.StartWithContext()` to handle internal plugins
9. ✅ Fix `Process.Stop()` to close stdin for internal plugins
10. ✅ Unit tests for internal plugin execution
11. ✅ Verify with `make verify`

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Explicit plugin selection | Yes | User controls what runs, no surprises |
| `ze.X` in-process | Yes | Faster startup, lower memory |
| External plugins fork | Yes | Process isolation for untrusted code |
| CLI + config additive | Yes | Flexibility, don't override config |
| Deduplication by name | Yes | Same plugin once, regardless of source |
| Same protocol in/out of process | Yes | No code changes in plugin implementations |
| Single registry | Yes | `internalPluginRunners` is source of truth |
| Stop closes stdin | Yes | Unblocks internal plugin read, causes exit |

## Implementation Summary

### What Was Implemented

1. **CLI `--plugin` flag** on `ze bgp server` - repeatable, supports multiple formats
2. **Plugin resolution** - `ze.X` (internal), `./path`, `/path`, `"cmd args"` (external)
3. **`ze --plugin` discovery** - lists available internal plugins
4. **`--yang` introspection** - on `ze bgp plugin X` commands (empty for now)
5. **In-process execution** - internal plugins run as goroutines with io.Pipe
6. **Clean shutdown** - Stop() closes stdin to unblock internal plugins

### Architecture

```
--plugin ze.rib
    ↓
mergeCliPlugins() → Internal=true, Run=""
    ↓
ProcessManager.Start() → NewProcess(config)
    ↓
Process.StartWithContext()
    ↓
if config.Internal:
    startInternal() → goroutine + io.Pipe
else:
    startExternal() → exec.Command + os pipes
```

### Bugs Found/Fixed

1. **Internal plugins didn't stop** - `Stop()` only cancelled context, but internal plugins don't respond to context cancellation. Fixed by closing stdin in `Stop()` for internal plugins.

### Design Insights

- Using `Process` for both internal and external plugins (rather than separate types) minimizes changes to server code
- io.Pipe provides same interface as os pipes, so plugin code doesn't need changes
- Single registry in `inprocess.go` avoids duplicate lists that must stay in sync

## Checklist

### Design
- [x] No premature abstraction
- [x] No speculative features
- [x] Single responsibility
- [x] Explicit behavior
- [x] Minimal coupling

### TDD
- [x] Tests written
- [x] Tests FAIL then PASS
- [x] Implementation complete
- [x] Tests PASS
- [x] Boundary tests cover numeric inputs
- [x] Feature code integrated into codebase
- [x] Functional tests verify end-user behavior

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (80 tests)
