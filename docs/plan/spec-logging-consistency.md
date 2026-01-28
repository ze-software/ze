# Spec: logging-consistency

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/go-standards.md` - current logging docs
4. `internal/slogutil/slogutil.go` - logging implementation

## Task

Unify Ze logging system for complete consistency:
1. Plugins check environment variables (not just CLI flags)
2. Add logging to core BGP packages that lack it
3. Update documentation to reflect all subsystems

## Problem Statement

### Inconsistency 1: Plugins Ignore Env Vars

Engine subsystems use `slogutil.Logger()` which reads `ze.log.bgp.<subsystem>`.
Plugin processes use `slogutil.LoggerWithLevel()` which only reads CLI `--log-level` flag.

This means:
- Engine: `ze.log.bgp.server=debug ze bgp server ...` ✅ works
- Plugin: `ze.log.bgp.gr=debug ze bgp server ...` ❌ ignored

### Inconsistency 2: Core Packages Lack Logging

| Package | LOC | Has Logging |
|---------|-----|-------------|
| fsm | 1,554 | ❌ |
| message | 13,578 | ❌ |
| attribute | 10,893 | ❌ |
| nlri | 12,260 | ❌ |
| rib (internal) | 6,875 | ❌ |
| capability | 2,303 | ❌ |
| reactor | 20,537 | ⚠️ partial (2/11 files) |
| rr | 1,356 | ❌ |

### Inconsistency 3: Documentation Incomplete

- Only 4 subsystems documented (server, coordinator, filter, plugin)
- 5 engine subsystems missing (subscribe, peer, session, record, runner)
- Typo in example: `ze.bgp.log.server` should be `ze.log.bgp.server`

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/go-standards.md` - current logging documentation

### Source Code
- [ ] `internal/slogutil/slogutil.go` - Logger(), LoggerWithLevel() implementation
- [ ] `cmd/ze/bgp/plugin_gr.go` - current plugin logging pattern
- [ ] `cmd/ze/bgp/plugin_rib.go` - current plugin logging pattern

## Design Decisions

### Subsystem Naming Convention

Use dot notation for hierarchical subsystems:

| Subsystem | Env Var | Purpose |
|-----------|---------|---------|
| server | `ze.log.bgp.server` | Plugin server |
| coordinator | `ze.log.bgp.coordinator` | Startup coordinator |
| filter | `ze.log.bgp.filter` | Route filtering |
| plugin | `ze.log.bgp.plugin` | stderr relay |
| subscribe | `ze.log.bgp.subscribe` | Subscriptions |
| peer | `ze.log.bgp.peer` | Peer management |
| session | `ze.log.bgp.session` | BGP sessions |
| gr | `ze.log.bgp.gr` | Graceful restart plugin |
| rib | `ze.log.bgp.rib` | RIB plugin |
| bgp.fsm | `ze.log.bgp.bgp.fsm` | FSM state machine |
| bgp.message | `ze.log.bgp.bgp.message` | Message parsing |
| bgp.attribute | `ze.log.bgp.bgp.attribute` | Attribute handling |
| bgp.nlri | `ze.log.bgp.bgp.nlri` | NLRI parsing |
| bgp.capability | `ze.log.bgp.bgp.capability` | Capability negotiation |
| bgp.reactor | `ze.log.bgp.bgp.reactor` | Reactor orchestration |
| bgp.rr | `ze.log.bgp.bgp.rr` | Route reflector |

### Plugin Logging Priority

CLI flag overrides env var (explicit user intent):
1. Check env var `ze.log.bgp.<subsystem>`
2. If CLI flag provided and not "disabled", CLI wins
3. Otherwise env var wins

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoggerWithEnvFallback` | `internal/slogutil/slogutil_test.go` | Env var read when CLI is disabled | |
| `TestLoggerCLIOverridesEnv` | `internal/slogutil/slogutil_test.go` | CLI flag takes precedence | |
| `TestLoggerEnvOnlyNoFlag` | `internal/slogutil/slogutil_test.go` | Works with env var alone | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | N/A | Logging is internal, no functional test needed | |

## Files to Modify

### Step 1: Plugin Consistency
- `internal/slogutil/slogutil.go` - add `LoggerWithEnvFallback()`
- `cmd/ze/bgp/plugin_gr.go` - use new function
- `cmd/ze/bgp/plugin_rib.go` - use new function

### Step 2: Core Package Logging
- `internal/plugin/bgp/fsm/fsm.go` - add logger
- `internal/plugin/bgp/message/update.go` - add logger (main entry point)
- `internal/plugin/bgp/attribute/builder.go` - add logger
- `internal/plugin/bgp/nlri/iterator.go` - add logger
- `internal/plugin/bgp/rib/store.go` - add logger
- `internal/plugin/bgp/capability/negotiated.go` - add logger
- `internal/plugin/bgp/reactor/reactor.go` - add logger
- `internal/plugin/rr/server.go` - add logger

### Documentation
- `.claude/rules/go-standards.md` - update subsystem table, fix typo

## Implementation Steps

### Step 1: Plugin Logging Consistency

1. **Write unit tests** for new `LoggerWithEnvFallback()` function
   - Test env var alone works
   - Test CLI flag overrides env var
   - Test CLI "disabled" falls back to env var
   → **Review:** Edge cases covered?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Tests fail for the RIGHT reason?

3. **Implement** `LoggerWithEnvFallback(subsystem, cliLevel string)` in slogutil.go
   - Check env var first via `getLogEnv(subsystem)`
   - If cliLevel provided and != "disabled", use cliLevel
   - Otherwise use env var level
   → **Review:** Simplest solution? Follows existing patterns?

4. **Run tests** - Verify PASS (paste output)

5. **Update plugin_gr.go and plugin_rib.go** to use new function

6. **Verify** - `make lint && make test`

### Step 2: Core Package Logging

For each package (fsm, message, attribute, nlri, rib, capability, reactor, rr):

1. **Add package-level logger** at top of main file:
   ```
   var logger = slogutil.Logger("bgp.<package>")
   ```

2. **Add strategic debug logging** for:
   - Entry points (function start with key parameters)
   - Error conditions (before returning errors)
   - State changes (FSM transitions, route updates)
   - Important decisions (capability negotiation outcomes)

3. **Verify** - `make lint && make test`

### Step 3: Documentation Update

1. **Fix typo** in go-standards.md line 30: `ze.bgp.log.server` → `ze.log.bgp.server`

2. **Update env var table** with all subsystems (engine + plugin + core)

3. **Verify** - Review docs for completeness

### Final Verification

- `make lint && make test && make functional` (paste output)

## Checklist

### 🏗️ Design
- [ ] No premature abstraction (using existing slogutil pattern)
- [ ] No speculative features (only adding what's needed)
- [ ] Single responsibility (each logger for one subsystem)
- [ ] Explicit behavior (env var + CLI flag priority documented)
- [ ] Minimal coupling (loggers are package-local)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Feature code integrated
- [ ] Functional tests N/A (internal logging)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] go-standards.md updated with all subsystems
- [ ] Typo fixed
- [ ] Env var table complete

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented
-

### Bugs Found/Fixed
-

### Design Insights
-

### Deviations from Plan
-
