# Spec: 06 - Plugin Invocation Conventions

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/bgp/decode.go` - current plugin invocation
4. `internal/plugin/inprocess.go` - internal plugin registry

## Task

Implement three plugin invocation conventions based on naming syntax:

| Syntax | Method | Cost | Use Case |
|--------|--------|------|----------|
| `name` or `path/name` | Fork | High | Spawn subprocess, process isolation |
| `ze.<name>` | Internal | Medium | API call with goroutine, like engine calls plugins |
| `ze-<name>` | Direct | Low | Direct function call, synchronous, no goroutine |

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `cmd/ze/bgp/decode.go` - `invokePluginDecodeRequest` (line 388), `inProcessDecoders` (line 521)
- [x] `internal/plugin/inprocess.go` - `internalPluginRunners` (line 27), full plugin protocol runners

**Behavior to preserve:**
- Subprocess invocation via `os.Args[0]` with `bgp plugin <name> --decode`
- In-process decode via `inProcessDecoders` map (Direct mode)
- Auto fallback from subprocess to in-process for plain `name` syntax

**Behavior to change:**
- `ze.name` syntax → Internal mode (goroutine + pipe), no fallback
- `ze-name` syntax → Direct mode (sync in-process), no fallback
- `/path/to/prog` → Fork that path directly, no fallback
- `name` syntax → unchanged (subprocess with in-process fallback)

## Design

### Syntax Parsing

```
parsePluginName(input) → (name, mode)

"flowspec"       → ("flowspec", ModeFork)
"/path/to/prog"  → ("/path/to/prog", ModeFork)
"ze.flowspec"    → ("flowspec", ModeInternal)
"ze-flowspec"    → ("flowspec", ModeDirect)
```

### Invocation Logic

```
ModeFork:     spawn subprocess (exec)
ModeInternal: goroutine + pipe (like engine's internal plugin API)
ModeDirect:   direct function call, synchronous, blocking
```

### Method Comparison

| Aspect | Fork | Internal | Direct |
|--------|------|----------|--------|
| Process | New process | Same process | Same process |
| Concurrency | OS-level | Goroutine | None (blocking) |
| Communication | stdin/stdout pipes | Go channels/pipes | Function return |
| Isolation | Full | Memory shared | Memory shared |
| Use case | External plugins | Engine-style | CLI decode, tests |

### Affected Locations

| Location | Current | Change |
|----------|---------|--------|
| `pluginFamilyMap` values | `"flowspec"` | Support all three syntaxes |
| `pluginCapabilityMap` values | `"hostname"` | Support all three syntaxes |
| `--plugin` flag | Any string | Document three syntaxes |
| Config `plugin` blocks | Various | `ze.rib` = internal, `ze-rib` = direct, `rib` = fork |

### Examples

```
# Fork (subprocess)
ze bgp decode --plugin flowspec --nlri ipv4/flow ...
ze bgp decode --plugin /usr/local/bin/my-decoder --nlri ...

# Internal (goroutine + API)
ze bgp decode --plugin ze.flowspec --nlri ipv4/flow ...

# Direct (synchronous function call)
ze bgp decode --plugin ze-flowspec --nlri ipv4/flow ...
```

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/plugin-modes.md` - plugin CLI modes
- [ ] `internal/plugin/inprocess.go` - internal plugin registry

### Source Files
- [ ] `cmd/ze/bgp/decode.go` - decode invocation
- [ ] `internal/plugin/resolve.go` - plugin resolution

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParsePluginName` | `cmd/ze/bgp/decode_test.go` | Syntax parsing for all three modes | |
| `TestInvokeFork` | `cmd/ze/bgp/decode_test.go` | `name` spawns subprocess | |
| `TestInvokeInternal` | `cmd/ze/bgp/decode_test.go` | `ze.name` uses goroutine+pipe | |
| `TestInvokeDirect` | `cmd/ze/bgp/decode_test.go` | `ze-name` calls function directly | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Plugin name | Non-empty | Any valid name | Empty string | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-invoke-fork` | `test/decode/*.ci` | `bgpls` decodes via subprocess | |
| `plugin-invoke-internal` | `test/decode/*.ci` | `ze.bgpls` decodes via goroutine | |
| `plugin-invoke-direct` | `test/decode/*.ci` | `ze-bgpls` decodes via direct call | |

## Files to Modify

- `cmd/ze/bgp/decode.go` - add `parsePluginName()`, modify invocation logic
- `cmd/ze/bgp/decode_test.go` - add tests for new syntax

## Files to Create

- None (modifications only)

## Implementation Steps

1. **Write unit tests** for `parsePluginName` - FAIL
2. **Implement `parsePluginName`** - extract name and mode from syntax
3. **Run tests** - PASS
4. **Write unit tests** for invocation modes - FAIL
5. **Modify `invokePluginNLRIDecodeRequest`** - route by mode
6. **Run tests** - PASS
7. **Update documentation** - clarify three syntaxes
8. **Run `make verify`** - all tests pass

## Design Decisions

### Q1: Refactor `internalPluginRunners`?

**No.** Keep existing structure. The distinction:
- `internalPluginRunners` → used by Internal mode (goroutine + pipe wrapper)
- `inProcessDecoders` → used by Direct mode (already exists, sync in-process)

Add thin wrapper for Internal mode:
```
invokePluginInternal(name, request):
    runner = internalPluginRunners[name]
    create io.Pipe pair
    go runner(inR, outW)
    write request, read response
```

### Q2: Error for unknown plugin?

**Explicit error, no fallback for prefixed syntax:**

| Syntax | On unknown plugin |
|--------|-------------------|
| `ze.unknown` | Error: "internal plugin 'unknown' not registered" |
| `ze-unknown` | Error: "direct decoder 'unknown' not available" |
| `unknown` | Try subprocess → fallback to direct → nil (existing behavior) |

### Q3: Default mode in `pluginFamilyMap`?

**Keep plain names** (e.g., `"flowspec"` not `"ze-flowspec"`).
- Preserves backward compatibility
- Existing subprocess-with-fallback behavior useful for tests
- Tests can explicitly use `ze-flowspec` for max speed if needed

### Q4: Path detection?

**Any string containing `/` is a path** (matches shell conventions):
- `/usr/bin/plugin` → Fork with absolute path
- `./local-plugin` → Fork with relative path
- `../other/plugin` → Fork with relative path
- `flowspec` → Fork built-in (os.Args[0] bgp plugin flowspec)

## Checklist

### 🏗️ Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit behavior
- [ ] Minimal coupling
- [ ] Next-developer test

### 🧪 TDD
- [x] Tests written (TestParsePluginName, TestParsePluginNameBoundary, TestInvokePlugin*)
- [x] Tests FAIL (verified before implementation)
- [x] Implementation complete
- [x] Tests PASS (24 tests pass)
- [x] Feature code integrated
- [x] Functional tests verify end-user behavior (22/22 decode tests pass)

### Verification
- [x] `make lint` passes (0 issues)
- [ ] `make test` passes (pre-existing env failures in subsystem/reactor unrelated to this change)
- [x] `make functional` decode passes (22/22)

## Implementation Summary

### What Was Implemented
- `PluginMode` type with `ModeFork`, `ModeInternal`, `ModeDirect` constants
- `parsePluginName()` function to detect mode from syntax prefix
- `invokePluginInternal()` for ze.name syntax (goroutine + io.Pipe)
- `invokePluginPath()` for external binary paths with `--decode` flag
- Updated `invokePluginNLRIDecode()` to route by mode
- 5 new tests: Direct, Internal, Fork, ForkPath, ModeConsistency

### Code Paths Tested
| Syntax | Mode | Execution |
|--------|------|-----------|
| `ze-flowspec` | Direct | Sync in-process call |
| `ze.flowspec` | Internal | Goroutine + io.Pipe |
| `flowspec` | Fork | `ze bgp plugin flowspec --decode` subprocess |
| `/path/to/bin` | Fork | External binary with `--decode` flag |

### API Contract
- External plugins must accept `--decode` flag
- Protocol: stdin `decode nlri <family> <hex>` → stdout `decoded json <json>`

### Deviations from Plan
- Internal mode uses decode-only function (not full plugin protocol) to avoid startup deadlock
