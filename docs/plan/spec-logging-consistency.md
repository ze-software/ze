# Spec: logging-consistency

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/go-standards.md` - current logging docs
4. `internal/slogutil/slogutil.go` - logging implementation
5. `internal/trace/trace.go` - trace system being migrated

## Task

Unify Ze logging system for complete consistency:
1. Plugins check environment variables (not just CLI flags)
2. Add hierarchical logging (base level + specific overrides)
3. New env var convention: `ze.log.<package-path>`
4. Migrate trace system to slog (delete internal/trace/)
5. Update documentation to reflect all subsystems

**Out of scope (separate specs per package):**
- Add logging to core BGP packages (fsm, message, attribute, nlri, rib, capability, reactor, rr)

## Problem Statement

### Inconsistency 1: Plugins Ignore Env Vars

Engine subsystems use `slogutil.Logger()` which reads env vars.
Plugin processes use `slogutil.LoggerWithLevel()` which only reads CLI `--log-level` flag.

This means:
- Engine: `ze.log.server=debug ze bgp server ...` ✅ works
- Plugin: `ze.log.gr=debug ze bgp server ...` ❌ ignored

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
- Old env var format `ze.log.bgp.<sub>` needs migration to `ze.log.<path>` convention

### Inconsistency 4: Two Logging Systems

`internal/trace/` uses unstructured printf-style logging:
- Env var: `ze.bgp.debug.trace=config,routes,session,fsm`
- Output: `[TRACE 15:04:05] cat: msg`
- No levels, no structured fields, stderr only

Should migrate to slog for consistency and structured logging.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/go-standards.md` - current logging documentation

### Source Code
- [ ] `internal/slogutil/slogutil.go` - Logger(), LoggerWithLevel() implementation
- [ ] `internal/trace/trace.go` - trace system to be migrated
- [ ] `cmd/ze/bgp/plugin_gr.go` - current plugin logging pattern
- [ ] `cmd/ze/bgp/plugin_rib.go` - current plugin logging pattern

## Design Decisions

### Subsystem Naming Convention

**Rule:** `ze.log.` + simplified package path (without `internal/plugin/`)

| Package | Subsystem | Env Var |
|---------|-----------|---------|
| `plugin/server.go` | `server` | `ze.log.server` |
| `plugin/startup_coordinator.go` | `coordinator` | `ze.log.coordinator` |
| `plugin/filter.go` | `filter` | `ze.log.filter` |
| `plugin/process.go` | `process` | `ze.log.process` |
| `plugin/subscribe.go` | `subscribe` | `ze.log.subscribe` |
| `plugin/bgp/reactor/peer.go` | `bgp.reactor.peer` | `ze.log.bgp.reactor.peer` |
| `plugin/bgp/reactor/session.go` | `bgp.reactor.session` | `ze.log.bgp.reactor.session` |
| `plugin/bgp/reactor/reactor.go` | `bgp.reactor` | `ze.log.bgp.reactor` |
| `plugin/bgp/fsm/` | `bgp.fsm` | `ze.log.bgp.fsm` |
| `plugin/bgp/message/` | `bgp.message` | `ze.log.bgp.message` |
| `plugin/bgp/attribute/` | `bgp.attribute` | `ze.log.bgp.attribute` |
| `plugin/bgp/nlri/` | `bgp.nlri` | `ze.log.bgp.nlri` |
| `plugin/bgp/capability/` | `bgp.capability` | `ze.log.bgp.capability` |
| `plugin/bgp/rib/` | `bgp.rib` | `ze.log.bgp.rib` |
| `plugin/gr/` | `gr` | `ze.log.gr` |
| `plugin/rr/` | `rr` | `ze.log.rr` |
| `plugin/rib/` | `rib` | `ze.log.rib` |
| `config/` | `config` | `ze.log.config` |

**New subsystem (from trace migration):**

| Package | Subsystem | Env Var |
|---------|-----------|---------|
| `plugin/bgp/reactor/` (Routes) | `bgp.routes` | `ze.log.bgp.routes` |

**Special env vars (not subsystems):**

| Env Var | Purpose |
|---------|---------|
| `ze.log.relay` | Plugin stderr relay level (debug/info/warn/err/disabled) |
| `ze.log.bgp.backend` | Output: stderr (default), stdout, syslog |
| `ze.log.bgp.destination` | Syslog address (when backend=syslog) |

### Hierarchical Logging Levels

Base level sets default for all subsystems, specific overrides:

| Env Var | Effect |
|---------|--------|
| `ze.log=info` | All subsystems at INFO |
| `ze.log.bgp=debug` | All bgp.* at DEBUG |
| `ze.log.bgp.reactor=debug` | All bgp.reactor.* at DEBUG |
| `ze.log.bgp.fsm=warn` | FSM at WARN (overrides base) |

### Priority Order (Same as ExaBGP)

Highest to lowest:
1. CLI flag `--log-level` (plugin processes only)
2. Most specific env var (dot notation): `ze.log.bgp.fsm`
3. Most specific env var (underscore): `ze_log_bgp_fsm`
4. Parent env var (dot): `ze.log.bgp`
5. Parent env var (underscore): `ze_log_bgp`
6. ... up to `ze.log` / `ze_log`
7. Default: disabled

Dot notation always takes precedence over underscore at same level.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoggerCLIOverridesEnv` | `internal/slogutil/slogutil_test.go` | CLI flag takes precedence over env | |
| `TestLoggerSpecificOverridesParent` | `internal/slogutil/slogutil_test.go` | `ze.log.bgp.fsm` overrides `ze.log.bgp` | |
| `TestLoggerParentLevel` | `internal/slogutil/slogutil_test.go` | `ze.log.bgp=debug` enables all bgp.* | |
| `TestLoggerRootLevel` | `internal/slogutil/slogutil_test.go` | `ze.log=debug` enables all subsystems | |
| `TestLoggerDotOverridesUnderscore` | `internal/slogutil/slogutil_test.go` | Dot notation wins over underscore | |
| `TestLoggerUnderscoreFallback` | `internal/slogutil/slogutil_test.go` | Underscore works when dot not set | |
| `TestLoggerDisabledCLIFallsBackToEnv` | `internal/slogutil/slogutil_test.go` | CLI "disabled" uses env var | |
| `TestLoggerInvalidLevel` | `internal/slogutil/slogutil_test.go` | Invalid level string = disabled | |
| `TestLoggerEmptyEnvAndCLI` | `internal/slogutil/slogutil_test.go` | Both empty = disabled | |
| `TestLoggerCreated` | `internal/slogutil/slogutil_test.go` | Logger not discard when env set | |
| `TestRelayLevel` | `internal/slogutil/slogutil_test.go` | `ze.log.relay` controls relay output | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | N/A | Logging is internal infrastructure, no end-user functional test | |

## Files to Modify

### Step 1: Hierarchical Logging + Plugin Consistency
- `internal/slogutil/slogutil.go` - new env var convention, hierarchical lookup, PluginLogger()
- `internal/slogutil/slogutil_test.go` - add 11 new unit tests
- `cmd/ze/bgp/plugin_gr.go` - use PluginLogger()
- `cmd/ze/bgp/plugin_rib.go` - use PluginLogger()

### Step 2: Update Existing Loggers
- `internal/plugin/server.go` - update subsystem name to `server`
- `internal/plugin/startup_coordinator.go` - update to `coordinator`
- `internal/plugin/filter.go` - update to `filter`
- `internal/plugin/process.go` - update to `process`, update relay logic
- `internal/plugin/subscribe.go` - update to `subscribe`
- `internal/plugin/bgp/reactor/peer.go` - update to `bgp.reactor.peer`
- `internal/plugin/bgp/reactor/session.go` - update to `bgp.reactor.session`
- `internal/test/runner/record.go` - update to `test.record`
- `internal/test/runner/runner.go` - update to `test.runner`

### Step 3: Migrate Trace to slog
- `internal/config/loader.go` - add logger, convert trace calls
- `internal/plugin/bgp/reactor/peer.go` - convert trace calls (logger exists)
- `internal/plugin/bgp/reactor/session.go` - convert trace calls (logger exists)
- `internal/plugin/bgp/reactor/reactor.go` - add logger, convert trace calls
- Delete `internal/trace/` package after migration

### Step 4: Documentation
- `.claude/rules/go-standards.md` - rewrite logging section with new convention

## Implementation Steps

### Step 1: Hierarchical Logging + Plugin Consistency

1. **Write unit tests** for hierarchical logging and PluginLogger()
   - All 11 tests from TDD Test Plan
   → **Review:** Edge cases covered?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Tests fail for the RIGHT reason?

3. **Implement hierarchical getLogEnv()**
   - Check most specific env var first: `ze.log.bgp.fsm`
   - Walk up parent path: `ze.log.bgp` → `ze.log`
   - At each level, check dot notation then underscore
   → **Review:** Simplest solution?

4. **Implement PluginLogger(subsystem, cliLevel string)**
   - CLI flag takes precedence if != "disabled"
   - Otherwise use hierarchical env var lookup
   → **Review:** Follows existing patterns?

5. **Run tests** - Verify PASS (paste output)

6. **Update plugin_gr.go and plugin_rib.go** to use PluginLogger()

7. **Verify** - `make lint && make test`

### Step 2: Update Existing Loggers

1. **Update all existing Logger() calls** to use new subsystem names
   - `Logger("server")` → `Logger("server")` (no change needed)
   - `Logger("peer")` → `Logger("bgp.reactor.peer")`
   - `Logger("session")` → `Logger("bgp.reactor.session")`
   - etc.

2. **Update IsPluginRelayEnabled()** to use `ze.log.relay` with level support

3. **Verify** - `make lint && make test`

### Step 3: Migrate Trace to slog

1. **Files to migrate:**
   - `internal/config/loader.go` (5 trace calls) - add `Logger("config")`
   - `internal/plugin/bgp/reactor/peer.go` (~60 trace calls) - logger exists
   - `internal/plugin/bgp/reactor/session.go` (7 trace calls) - logger exists
   - `internal/plugin/bgp/reactor/reactor.go` (7 trace calls) - add logger

2. **Trace category → slog subsystem mapping**

   | trace category | slog subsystem |
   |----------------|----------------|
   | `trace.Config` | `config` |
   | `trace.Routes` | `bgp.routes` (new) |
   | `trace.Session` | `bgp.reactor.session` |
   | `trace.FSM` | `bgp.reactor.peer` (FSM is per-peer) |

3. **Add routes logger** to reactor/peer.go for Routes category (`bgp.routes`)

4. **Convert each call** - replace printf-style with structured key-value pairs

5. **Delete internal/trace/** after all usages migrated

6. **Verify** - `make lint && make test && make functional`

### Step 4: Documentation Update

1. **Rewrite logging section** in go-standards.md with new `ze.log.<path>` convention

2. **Update env var table** with all subsystems including hierarchical base levels

3. **Document trace removal** - remove any trace references from docs

4. **Verify** - Review docs for completeness

### Final Verification

- `make lint && make test && make functional` (paste output)
- Verify no trace imports remain: `grep -r "internal/trace" internal/`
- Verify old env var format not referenced: `grep -r "ze\.log\.bgp\.\(server\|coordinator\|filter\|plugin\)" .`

## Checklist

### 🏗️ Design
- [ ] No premature abstraction (using existing slogutil pattern)
- [ ] No speculative features (only adding what's needed)
- [ ] Single responsibility (each logger for one subsystem)
- [ ] Explicit behavior (env var + CLI flag priority documented)
- [ ] Minimal coupling (loggers are package-local)

### 🧪 TDD
- [ ] Tests written (11 unit tests)
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Feature code integrated
- [ ] Functional tests N/A (internal logging)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Existing Loggers Updated
- [ ] All Logger() calls use new simplified subsystem names
- [ ] plugin_gr.go uses PluginLogger()
- [ ] plugin_rib.go uses PluginLogger()
- [ ] IsPluginRelayEnabled() replaced with relay level check

### Trace Migration
- [ ] config/loader.go converted
- [ ] reactor/peer.go converted
- [ ] reactor/session.go converted
- [ ] reactor/reactor.go converted
- [ ] internal/trace/ deleted
- [ ] No trace imports remain

### Documentation
- [ ] go-standards.md rewritten with new convention
- [ ] Env var table complete with all subsystems
- [ ] Hierarchical logging documented
- [ ] Old `ze.log.bgp.*` references removed

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
