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

**Source files to read:**
- [ ] `cmd/ze/bgp/decode.go` - `invokePluginNLRIDecodeRequest`, `inProcessDecoders`
- [ ] `internal/plugin/inprocess.go` - `internalPluginRunners`, plugin registration
- [ ] `internal/plugin/resolve.go` - `IsInternalPlugin`, plugin resolution
- [ ] Config files using `ze.rib`, `ze.hostname` syntax

**Behavior to preserve:**
- Subprocess invocation via `os.Args[0]` with `bgp plugin <name> --decode`
- In-process decode via `inProcessDecoders` map
- Auto fallback from subprocess to in-process (for tests)

**Behavior to change:**
- `ze.name` syntax → force in-process only (no subprocess attempt)
- `ze/name` syntax → force subprocess only (no fallback)
- `name` syntax → unchanged (subprocess with in-process fallback)
- Config `ze.rib` currently treated as internal → should use `ze/rib` for subprocess

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

## Open Questions

1. Should `internalPluginRunners` in `inprocess.go` be renamed/refactored to support internal vs direct distinction?
2. What error should `ze.unknown` or `ze-unknown` return if plugin not available?
3. For tests: which mode should be default in `pluginFamilyMap`? (`ze-` direct would be fastest)
4. Should path detection (`/path/to/prog`) work for any path or only absolute paths?

## Checklist

### 🏗️ Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit behavior
- [ ] Minimal coupling
- [ ] Next-developer test

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Feature code integrated
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes
