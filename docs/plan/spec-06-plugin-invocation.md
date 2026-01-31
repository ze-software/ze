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
| `name` | Auto | Variable | Default - subprocess with in-process fallback |
| `ze.name` | In-process | Low | Direct function call, no IPC overhead |
| `ze/name` | Subprocess | High | Explicit process isolation, like calling a program |

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

"flowspec"    → ("flowspec", ModeAuto)
"ze.flowspec" → ("flowspec", ModeInProcess)
"ze/flowspec" → ("flowspec", ModeSubprocess)
```

### Invocation Logic

```
ModeAuto:       subprocess() || inprocess()   # current behavior
ModeInProcess:  inprocess() only              # fail if not available
ModeSubprocess: subprocess() only             # fail if subprocess fails
```

### Affected Locations

| Location | Current | Change |
|----------|---------|--------|
| `pluginFamilyMap` values | `"flowspec"` | Support all three syntaxes |
| `pluginCapabilityMap` values | `"hostname"` | Support all three syntaxes |
| `--plugin` flag | Any string | Document three syntaxes |
| Config `plugin` blocks | `ze.rib` | Clarify: `ze.rib` = in-process, `ze/rib` = subprocess |

### Config Migration

Current config uses `ze.rib` which will now mean in-process. If users want subprocess:
- Old: `run "ze bgp plugin rib"` or `plugin ze.rib`
- New: `plugin ze/rib` for subprocess, `plugin ze.rib` for in-process

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
| `TestInvokeInProcessOnly` | `cmd/ze/bgp/decode_test.go` | `ze.name` uses in-process only | |
| `TestInvokeSubprocessOnly` | `cmd/ze/bgp/decode_test.go` | `ze/name` uses subprocess only | |
| `TestInvokeAutoFallback` | `cmd/ze/bgp/decode_test.go` | `name` falls back to in-process | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Plugin name | Non-empty | Any valid name | Empty string | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-invoke-inprocess` | `test/decode/*.ci` | `ze.bgpls` decodes via in-process | |
| `plugin-invoke-subprocess` | `test/decode/*.ci` | `ze/bgpls` decodes via subprocess | |

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

1. Should `internalPluginRunners` in `inprocess.go` also respect this syntax?
2. What error should `ze.unknown` return if plugin not available in-process?
3. Should we add `ze.` and `ze/` variants to `pluginFamilyMap` or handle dynamically?

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
