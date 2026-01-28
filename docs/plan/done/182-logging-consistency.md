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
2. Add hierarchical logging (base level + specific overrides)
3. New env var convention: `ze.log.<package-path>`
4. Migrate trace system to slog (delete internal/trace/)
5. Update documentation to reflect all subsystems
6. **Config file support:** Allow setting log levels via `environment { }` block

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

### Inconsistency 5: Config File Cannot Set Log Levels (TODO)

The `environment { }` block in config files can set various settings, but the new
`ze.log.<subsystem>=<level>` logging cannot be configured from config files.

**Current state:**
- OS env vars work: `ze.log.bgp.routes=debug ze bgp server config.conf`
- Config file does NOT work for new logging

**Required:** Add config file support using the new hierarchical format:

```
environment {
    log {
        # Base level for all subsystems (default: warn)
        level warn;

        # Specific subsystem overrides
        bgp.routes debug;
        bgp.reactor.peer info;
        config debug;

        # Output destination
        backend stderr;        # stderr | stdout | syslog
        destination localhost; # syslog address (when backend=syslog)

        # Plugin stderr relay level
        relay warn;
    }
}
```

**Implementation approach:**
1. Parse `environment { log { } }` block with new syntax
2. Call `os.Setenv()` for each `ze.log.*` setting BEFORE loggers are created
3. Priority: OS env var > config file > default (WARN)

**Why os.Setenv:** Loggers are created at package init time via `var logger = slogutil.Logger("subsystem")`.
The config file is parsed later. By setting OS env vars early in `main()`, the existing `slogutil.Logger()`
code works unchanged - it just sees the env vars set from config.

**Current environment block system:**

The existing `environment { }` block is parsed into the `Environment` struct:
- `internal/config/environment.go` - defines `Environment`, `LogEnv`, `TCPEnv`, etc.
- `internal/config/bgp.go` - `ExtractEnvironment()` parses config tree → `map[string]map[string]string`
- `internal/config/loader.go` - `LoadEnvironmentWithConfig()` merges: defaults → config block → OS env

Flow:
1. `LoadReactorFile()` parses config file
2. `ExtractEnvironment(tree)` extracts `environment { }` block values
3. `CreateReactorWithDir()` calls `LoadEnvironmentWithConfig(envValues)`
4. `LoadEnvironmentWithConfig()` builds `Environment` struct with priority: defaults < config < OS env

The `log { }` section currently uses old ExaBGP-style boolean flags:

| Old Syntax | Type | Description |
|------------|------|-------------|
| `level DEBUG;` | string | Syslog level |
| `routes true;` | bool | Log received routes |
| `configuration true;` | bool | Log config parsing |

**New syntax needed:**

| New Syntax | Maps To |
|------------|---------|
| `level warn;` | `ze.log=warn` |
| `bgp.routes debug;` | `ze.log.bgp.routes=debug` |
| `backend syslog;` | `ze.log.backend=syslog` |

## Required Reading

### Architecture Docs
- [x] `.claude/rules/go-standards.md` - current logging documentation
- [ ] `docs/architecture/config/syntax.md` - config syntax (environment block)

### Source Code
- [x] `internal/slogutil/slogutil.go` - Logger(), PluginLogger() implementation
- [x] `cmd/ze/bgp/plugin_gr.go` - plugin logging pattern
- [x] `cmd/ze/bgp/plugin_rib.go` - plugin logging pattern
- [ ] `internal/config/environment.go` - current environment block parsing
- [ ] `internal/config/loader.go` - LoadEnvironmentWithConfig() flow

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
- [x] No premature abstraction (using existing slogutil pattern)
- [x] No speculative features (only adding what's needed)
- [x] Single responsibility (each logger for one subsystem)
- [x] Explicit behavior (env var + CLI flag priority documented)
- [x] Minimal coupling (loggers are package-local)

### 🧪 TDD
- [x] Tests written (15 unit tests for hierarchical/plugin/relay)
- [x] Tests FAIL (functions didn't exist)
- [x] Implementation complete
- [x] Tests PASS (all 35 slogutil tests pass)
- [x] Feature code integrated
- [x] Functional tests N/A (internal logging)

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Existing Loggers Updated
- [x] All Logger() calls use new subsystem names (bgp.reactor.peer, bgp.reactor.session, test.runner, test.record)
- [x] plugin_gr.go uses PluginLogger()
- [x] plugin_rib.go uses PluginLogger()
- [x] IsPluginRelayEnabled() replaced with RelayLevel() check

### Trace Migration
- [x] config/loader.go converted (5 calls → configLogger)
- [x] reactor/peer.go converted (~60 calls → routesLogger/peerLogger)
- [x] reactor/session.go converted (7 calls → sessionLogger)
- [x] reactor/reactor.go converted (7 calls → reactorLogger/routesLogger)
- [x] internal/trace/ deleted
- [x] No trace imports remain

### Documentation
- [x] go-standards.md rewritten with new ze.log.<path> convention
- [x] Env var table complete with all subsystems
- [x] Hierarchical logging documented (priority order)
- [x] Old `ze.log.bgp.*` references removed

### Config File Support
- [x] ApplyLogConfig() function added to slogutil
- [x] Unit tests for config file support (8 tests)
- [x] Integration in LoadReactorWithConfig() after ExtractEnvironment()
- [x] Priority: OS env var > config file > default
- [x] LazyLogger() for deferred logger creation
- [x] All 12 package-level loggers converted to lazy initialization
- [x] Invalid level/backend validation with warnings
- [x] YANG schema updated with ze:allow-unknown-fields extension
- [x] Functional test for config file log settings

## Implementation Summary

### What Was Implemented
- Hierarchical env var lookup: ze.log.bgp.fsm → ze.log.bgp → ze.log
- PluginLogger() for plugins (CLI flag OR env var fallback)
- RelayLevel() replacing IsPluginRelayEnabled() for level-aware relay
- Migrated all trace calls to slog (config, reactor, peer, session)
- Deleted internal/trace/ package
- Updated docs with new convention
- **Config file support:** ApplyLogConfig() maps config `log { }` block to env vars
- **LazyLogger():** Defers logger creation until first use, allowing config file settings to work for all loggers

### Config File Syntax
```
environment {
    log {
        level warn;            # ze.log=warn (base level)
        bgp.routes debug;      # ze.log.bgp.routes=debug
        config info;           # ze.log.config=info
        backend stderr;        # ze.log.backend=stderr
        destination localhost; # ze.log.destination=localhost (syslog)
        relay warn;            # ze.log.relay=warn
    }
}
```

### Lazy Loggers
All engine loggers use `LazyLogger()` for deferred creation:
- configLogger, logger (server), stderrLogger, filterLogger
- coordinatorLogger, subscribeLogger, reactorLogger, routesLogger
- peerLogger, sessionLogger, logger (test.runner), recordLogger

This allows config file settings to be applied before first logger use.

### Bugs Found/Fixed
- Package-level loggers ignored config file (fixed with LazyLogger)

### Design Insights
- Hierarchical lookup is clean: split subsystem by ".", walk from specific to root
- PluginLogger() enables env var control for plugins without CLI changes
- routesLogger needed in both reactor.go and peer.go (declared in both)
- Config file support uses os.Setenv() to integrate with existing hierarchical lookup
- LazyLogger uses sync.Once for thread-safe deferred creation
- applyLogConfigTo() internal function enables thread-safe testing

### Deviations from Plan
- Added routesLogger to peer.go (not just reactor.go) since peer.go has many route operations
- RelayLevel returns (slog.Level, bool) instead of just bool for level-aware filtering
- Added LazyLogger to fix package-level logger limitation (not in original plan)
