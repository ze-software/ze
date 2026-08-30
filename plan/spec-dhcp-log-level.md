# Spec: dhcp-log-level

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/dhcpserver/register.go` - `ConfigureEngineLogger`
4. `internal/plugins/dhcpserver/config.go` - `parseConfig`
5. `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang`

## Task

Ze's DHCP server (its own IPv4 Go implementation) logs at a fixed verbosity: the
plugin receives a single injected `slog` logger and its call sites use hardcoded
`Debug`/`Info` levels. ~~There is no operator control over how chatty the server is,
so debugging a lease problem in the field means either drowning in debug output
elsewhere or having none.~~ (Superseded 2026-07-10: operator control over the
dhcpserver logger's level ALREADY exists -- see Post-wave corrections below. The
injected logger's level is set from the hierarchical `ze.log.<subsystem>` env
lookup and is runtime-adjustable via the log plugin's `log-set` command. The
premise of this spec must be re-established or the spec re-scoped before it can
go ready.)

Add a `log-level` (or equivalent verbosity) config leaf to the DHCP server so the
operator can raise or lower DHCP logging independently, e.g. `set service
dhcp-server log-level debug|info|warning|error`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/provisioning/dhcp-server.md` - the DHCP server, which serves LAN leases per RFC 2131 and RFC 2132
- [ ] `ai/rules/config.md` - YANG leaf vs env var decision.
  → Constraint: a per-service runtime log verbosity is operator-facing config; YANG leaf, not env var.
- [ ] `ai/rules/plugins.md` - dhcpserver owns its config surface.
  → Constraint: the level maps to the plugin's own logger; no central logging switch.

**Key insights:**
- The plugin already owns its logger via `ConfigureEngineLogger`; this feature sets that logger's level from config rather than adding new logging plumbing.
- Level is a bounded enum, so YANG native `enumeration` validation is sufficient.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dhcpserver/register.go` - `ConfigureEngineLogger` stores an injected `slog.Logger` into `loggerPtr` (register.go); log call sites use fixed `Debug`/`Info`. ~~No level control.~~ (Superseded 2026-07-10: the injected logger carries an operator-controlled level; see Post-wave corrections.)
- [ ] `internal/plugins/dhcpserver/config.go` - `parseConfig` (config.go) reads only `enabled` (:88), `listen-interface` (:92), `shared-network` (:111), `pxe` (:365); no `log`/`verbosity`/`log-level` leaf is parsed.
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - service config schema; no logging leaf today.

### Post-wave corrections (2026-07-10)

Citations re-verified: register.go, the parseConfig surface, and the YANG
schema are all current (no drift). However, re-reading the PRODUCER of the
injected logger contradicts the spec's premise:

- `ConfigureEngineLogger` receives `CanonicalSubsystemName(name)` from the
  plugin host (`internal/component/plugin/inprocess.go`) and calls
  `slogutil.Logger(loggerName)` (register.go).
- `slogutil.Logger` (`internal/core/slogutil/slogutil.go`) resolves the
  boot-time level via the hierarchical env lookup `ze.log.<subsystem>` ->
  parents -> `ze.log` (`getLogEnv`, slogutil.go; default WARN) and
  registers a runtime-adjustable `slog.LevelVar` in `levelRegistry`
  (slogutil.go, stores at :190 and :202).
- `slogutil.SetLevel` (slogutil.go) changes a registered subsystem's
  level at RUNTIME, and the log plugin exposes it as an operator command:
  `ze-bgp:log-set` -> `handleLogSet` -> `slogutil.SetLevel`
  (`internal/plugins/log/cmd/handlers.go`, SetLevel call at :112);
  `ze-bgp:log-levels` (:17) lists current levels.

Consequence: DHCP log verbosity is ALREADY operator-controllable (env var at
boot, `log-set` at runtime) when the plugin runs in-process. The proposed
per-service YANG `log-level` leaf would duplicate that surface and needs a
reconciliation decision before this spec can be promoted:

- Option (a): drop the new leaf; re-scope the spec to documenting and/or
  functionally testing the existing `ze.log.<dhcp subsystem>` + `log-set`
  path for the DHCP server (possibly none of the current design survives).
- Option (b): keep a service-local YANG leaf and define precedence between the
  leaf, the `ze.log.*` env hierarchy, and runtime `SetLevel`
  (`ai/rules/config.md` governs this YANG-vs-env decision).

This is a scope/design decision that requires the user; the spec stays in
`design` until it is made. A-1 is meanwhile effectively answered: the injected
logger IS levelable at runtime through its registered `LevelVar`.

**Behavior to preserve:**
- Default logging behaviour when the leaf is absent must match today's output (choose the current effective level as the default).
- No change to lease/pool/handler logic; this is purely observability.

**Behavior to change:**
- Add a `log-level` leaf; apply it to the plugin's logger so DHCP log verbosity is operator-controlled.

## Data Flow (MANDATORY)

### Entry Point
- Config: new `log-level` leaf under the DHCP service container in `ze-dhcp-server-conf.yang`.
- Parsed by `parseConfig` (`internal/plugins/dhcpserver/config.go`) into the DHCP config struct.

### Transformation Path
1. YANG `log-level <enum>` parsed by `parseConfig` into a config field.
2. On config apply, the level is translated to an `slog.Level` and applied to the plugin's logger (the one set via `ConfigureEngineLogger`).
3. Existing `Debug`/`Info`/etc. call sites are now filtered by the configured level.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ dhcpserver | YANG `log-level` → config struct via `parseConfig` | [ ] |
| Config ↔ logger | level applied to the plugin's `slog` logger | [ ] |

### Integration Points
- `parseConfig` (`config.go`) - read the new leaf.
- `ConfigureEngineLogger` / `loggerPtr` (`register.go`) - apply the level to the logger.

### Architectural Verification
- [ ] No bypassed layers (config via `parseConfig`)
- [ ] No unintended coupling (only the dhcpserver logger is affected)
- [ ] No duplicated functionality (reuse existing logger; set its level)
- [ ] Registration over hardcoding — level is dhcpserver-local config, no central logging switch added.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The injected `slog.Logger` level can be set/leveled at runtime | `loggerPtr.Store` holds a `*slog.Logger` (register.go) | may need a leveled handler wrapper | read `slogutil.Logger` during audit | unvalidated |
| A-2 | A bounded enum (debug/info/warning/error) covers operator needs | standard log levels | operator wants numeric levels | design confirmation with user | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Changing the shared logger affects other plugins' output | other components' logs change verbosity | apply level only to the dhcpserver logger instance, not the global default |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set service dhcp-server log-level debug` | → | `parseConfig` reads leaf, level applied to logger | `test/plugin/dhcp-log-level.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `log-level debug` | DHCP server emits debug-level logs |
| AC-2 | `log-level error` | only error-and-above DHCP logs emitted |
| AC-3 | leaf absent | logging matches current default behaviour |
| AC-4 | invalid level value | config verify rejects (enum) |
| AC-5 | level applied | only the dhcpserver logger is affected, not other plugins |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | raises DHCP verbosity to debug a lease issue | config → `parseConfig` → logger level set → debug logs appear | `test/plugin/dhcp-log-level.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDHCPLogLevelParse` | `internal/plugins/dhcpserver/config_test.go` | enum → config field | |
| `TestDHCPLogLevelAppliedToLogger` | `internal/plugins/dhcpserver/register_test.go` | level applied to the plugin logger | |
| `TestDHCPLogLevelDefault` | `internal/plugins/dhcpserver/config_test.go` | absent leaf → current default | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| log-level | enum (debug/info/warning/error) | error | N/A (enum) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dhcp-log-level` | `test/plugin/dhcp-log-level.ci` | configured level changes DHCP log verbosity | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - observability only, no wire change | - | - | - | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/plugins/dhcpserver/config.go` - parse `log-level`
- `internal/plugins/dhcpserver/register.go` - apply level to the plugin logger
- `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - add `log-level` enum leaf

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `ze-dhcp-server-conf.yang` `log-level` enum; `ai/rules/config.md`, `ai/rules/config.md` |
| YANG validation constraints | [ ] yes | `enumeration` (debug/info/warning/error) |
| CLI grammar | [ ] yes | `ai/rules/cli.md` |
| Functional test for new behaviour | [ ] yes | `test/plugin/dhcp-log-level.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |

## Files to Create
- `test/plugin/dhcp-log-level.ci` - functional test
- (unit tests extend existing dhcpserver test files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add `log-level` YANG enum + `parseConfig` field (unused); failing `test/plugin/dhcp-log-level.ci`.
2. **Phase: Apply level** — translate to `slog.Level` and set it on the dhcpserver logger.
   - Tests: `TestDHCPLogLevelAppliedToLogger`, `TestDHCPLogLevelDefault`
3. **Functional test**
4. **Full verification** → `./le verify current mode full`
5. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | default preserved; only dhcpserver logger affected |
| YANG validation | `enumeration` used, not bare string |
| Registration over hardcoding | dhcpserver-local, no central logging switch |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| level parsed + applied | `go test ./internal/plugins/dhcpserver -run LogLevel` |
| functional | `test/plugin/dhcp-log-level.ci` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Log content | raising to debug must not log secrets (client MACs are already logged; no new PII) |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (enum)
- [ ] Functional tests for end-to-end behavior
