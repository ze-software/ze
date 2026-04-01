# Spec: listener-4-migrate

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-listener-1-yang, spec-listener-5-log |
| Phase | - |
| Updated | 2026-04-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/migration/migrate.go` - existing migration pipeline
4. `cmd/ze/exabgp/main.go` - ExaBGP migrate subcommand
5. `exabgp/environment/config.py` - ExaBGP source reference for env file format

## Task

Add migration transformations for the listener and log changes, and implement ExaBGP environment file migration. Two tools affected: `ze config migrate` (Ze format upgrades) and `ze exabgp migrate --env` (ExaBGP env file conversion).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config parsing pipeline, how migration transformations integrate
  -> Constraint: migrations operate on the parsed config tree, not raw text
- [ ] `docs/architecture/config/environment.md` - environment configuration, what sections exist
  -> Constraint: environment block extraction must reflect post-migration state
- [ ] `spec-listener-0-umbrella.md` - umbrella spec, ExaBGP migration tool section
  -> Decision: mapping tables are defined in umbrella; this spec implements them

### RFC Summaries (MUST for protocol work)
- [ ] N/A -- no new RFC behavior

**Key insights:**
- Migration transformations operate on the parsed config tree, not raw text
- `ze config migrate` has an existing pipeline in `internal/component/config/migration/`
- `ze exabgp migrate` currently handles config files only; `--env` is a new flag for INI env files
- ExaBGP env files are INI format (Python configparser) with `[exabgp.<section>]` sections
- Log topic mapping depends on spec-listener-5-log verifying Ze subsystem equivalents exist

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/migration/migrate.go` - existing transformation pipeline, ordered list of transformations applied to config tree
- [ ] `cmd/ze/exabgp/main.go` - ExaBGP subcommands including migrate; accepts config file path, outputs Ze format
- [ ] `exabgp/environment/config.py` - ExaBGP source: INI format with `[exabgp.<section>]` sections, `key = value` entries, Python configparser

**Behavior to preserve:**
- Existing `ze config migrate` transformations continue to work in order
- `ze exabgp migrate` config file conversion unchanged when `--env` is not provided
- `ze config migrate --list` shows all transformations including new ones
- Migration output format (Ze config syntax)

**Behavior to change:**

### ze config migrate -- new transformations

| Transformation | Detects | Applies |
|---------------|---------|---------|
| host-to-ip | leaf named "host" or "address" in listener containers | Rename to "ip" |
| listener-to-list | single ip + port in service container | Convert to `list server { key name; }` with entry named "default" |
| remove-bgp-listen | bgp > listen as a string value | Remove leaf, emit warning: Ze uses per-peer connection > local |
| remove-tcp-port | environment > tcp > port exists | Remove leaf, emit warning: Ze uses per-peer config, default port 179 |
| remove-env-bgp-connect | environment > bgp > connect exists | Remove leaf, emit warning |
| remove-env-bgp-accept | environment > bgp > accept exists | Remove leaf, emit warning |
| log-booleans-to-subsystems | environment > log > (topic) as boolean | Convert to subsystem level syntax (true to debug, false to disabled), remove boolean leaf |

### ze exabgp migrate --env -- new flag

Add --env flag that reads an ExaBGP INI environment file and merges settings into the Ze output.

ExaBGP env is INI format (Python configparser). Sections are `[exabgp.<section>]`, keys are `option = value`. Source: `exabgp/environment/config.py` in the ExaBGP repo.

Only non-default values emitted. Default values skipped (Ze has own YANG defaults).

Mapping -- listener-related (ExaBGP legacy, no Ze equivalent):

| ExaBGP key | Ze output | Notes |
|-----------|-----------|-------|
| tcp.bind (space-separated IP list) | Comment: Ze uses per-peer connection > local > ip | No global listen in Ze |
| tcp.port | Comment: Ze uses per-peer connection > local > port, default 179 | No global listen in Ze |

Mapping -- environment settings (value carries to Ze config):

| ExaBGP section | Ze output path | Keys |
|---------------|---------------|------|
| daemon | environment daemon | pid, user, daemonize, drop, umask |
| cache | environment cache | attributes |
| reactor | environment reactor | speed |
| debug | environment debug | memory, configuration, selfcheck, route, defensive, rotate, timing |
| tcp (remaining) | environment tcp | attempts, delay, acl |
| bgp (remaining) | environment bgp | openwait |

Mapping -- logging (requires spec-listener-5-log):

| ExaBGP key | Ze output |
|-----------|-----------|
| log.level = WARNING | environment log level warn |
| log.destination = syslog | environment log destination syslog |
| log.all = true | environment log level debug |
| log.enable = false | environment log level disabled |
| log.short = true | environment log short true |
| log.(topic) = true | environment log (ze-subsystem) debug |
| log.(topic) = false | environment log (ze-subsystem) disabled |

Topic mapping (verified by spec-listener-5-log):

| ExaBGP topic | Ze subsystem |
|-------------|-------------|
| configuration | bgp.configuration |
| reactor | bgp.reactor |
| daemon | bgp.daemon |
| processes | bgp.processes |
| network | bgp.network |
| statistics | bgp.statistics |
| packets | bgp.packets |
| rib | bgp.rib |
| message | bgp.message |
| timers | bgp.timers |
| routes | bgp.routes |
| parser | bgp.parser |

Mapping -- comment-only:

| ExaBGP key | Comment |
|-----------|---------|
| tcp.once | deprecated, use tcp.attempts = 1 |
| bgp.passive | in Ze, set per-peer: connection remote accept false |
| cache.nexthops | deprecated, always enabled in Ze |
| debug.pdb | Python debugger, not applicable to Ze |
| api.* | ExaBGP API, Ze uses YANG RPC over plugin sockets |
| profile.* | Python profiling, not applicable to Ze |
| pdb.* | Python debugger, not applicable to Ze |

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- `ze config migrate`: Ze config file (parsed into config tree)
- `ze exabgp migrate --env`: ExaBGP INI environment file (raw text, parsed by INI reader)

### Transformation Path

**ze config migrate:**
1. Config file parsed into tree
2. Migration pipeline applies transformations in order
3. New transformations detect old patterns (host leaf, string listen, tcp.port, bgp.connect/accept, log booleans)
4. Each transformation modifies the tree in-place
5. Modified tree serialized back to Ze config format

**ze exabgp migrate --env:**
1. ExaBGP env file read as INI (section/key/value triples)
2. Each section.key looked up in mapping tables
3. Listener-related keys (tcp.bind, tcp.port) produce bgp listen blocks
4. Environment keys produce environment block entries
5. Log keys produce environment log entries with level mapping
6. Comment-only keys produce Ze config comments
7. Merged with config file migration output (if both --env and config file provided)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| INI file -> Ze config tree | ExaBGP env parser reads INI, produces Ze config entries | [ ] |
| Migration pipeline -> config tree | Transformations modify tree nodes in-place | [ ] |
| Config tree -> serialized output | Existing serialization, unchanged | [ ] |

### Integration Points
- `internal/component/config/migration/migrate.go` - register new transformations in the pipeline
- `cmd/ze/exabgp/main.go` - add --env flag, call env parser, merge results
- Existing ExaBGP config migration - env output merges with config output

### Architectural Verification
- [ ] No bypassed layers (transformations use the migration pipeline, not ad-hoc tree manipulation)
- [ ] No unintended coupling (env parser is standalone, does not import config internals)
- [ ] No duplicated functionality (extends existing migration pipeline and ExaBGP migrate command)
- [ ] Zero-copy preserved where applicable (N/A -- config migration, not wire encoding)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config migrate` on config with host leaf | -> | host-to-ip transformation | `test/parse/config-migrate-host-to-ip.ci` |
| `ze config migrate` on config with string bgp listen | -> | bgp-listen-container transformation | `test/parse/config-migrate-bgp-listen.ci` |
| `ze config migrate` on config with tcp.port | -> | tcp-port-to-listen transformation | `test/parse/config-migrate-tcp-port.ci` |
| `ze config migrate` on config with bgp.connect/accept | -> | remove-env-bgp transformation | `test/parse/config-migrate-remove-env-bgp.ci` |
| `ze config migrate` on config with log booleans | -> | log-booleans-to-subsystems transformation | `test/parse/config-migrate-log-booleans.ci` |
| `ze exabgp migrate --env` with env file | -> | ExaBGP env parser + merge | `test/parse/exabgp-migrate-env.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ze config migrate on config with "host" in web container | Renamed to "ip" |
| AC-2 | ze config migrate on config with bgp listen "0.0.0.0:179" | Removed with warning: Ze uses per-peer connection > local |
| AC-3 | ze config migrate on config with environment tcp port 1179 | Removed with warning: Ze uses per-peer config, default 179 |
| AC-4 | ze config migrate on config with environment bgp connect false | Removed with warning |
| AC-5 | ze config migrate on config with log.packets true | Converted to bgp.packets debug |
| AC-6 | ze exabgp migrate --env exabgp.env config.conf | Env file parsed, tcp.port/bind emit comments (no global listen in Ze) |
| AC-7 | ze exabgp migrate --env with log.packets = true | Output contains bgp.packets debug in log section |
| AC-8 | ze exabgp migrate --env with bgp.passive = true | Output contains comment about per-peer setting |
| AC-9 | ze exabgp migrate --env with debug.pdb = true | Output contains comment about Python-only |
| AC-10 | ze config migrate --list | Shows new transformations in list |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHostToIPTransformation` | `internal/component/config/migration/listener_test.go` | host leaf renamed to ip in listener containers | |
| `TestBGPListenContainerTransformation` | `internal/component/config/migration/listener_test.go` | string bgp listen converted to container with ip + port | |
| `TestTCPPortToListenTransformation` | `internal/component/config/migration/listener_test.go` | tcp.port value moved to bgp listen port, tcp.port removed | |
| `TestRemoveEnvBGPConnect` | `internal/component/config/migration/listener_test.go` | bgp.connect removed with warning | |
| `TestRemoveEnvBGPAccept` | `internal/component/config/migration/listener_test.go` | bgp.accept removed with warning | |
| `TestLogBooleansToSubsystems` | `internal/component/config/migration/listener_test.go` | boolean log topics converted to subsystem level syntax | |
| `TestParseExaBGPEnv` | `internal/exabgp/migration/env_test.go` | INI file parsed into section/key/value triples | |
| `TestEnvListenerMapping` | `internal/exabgp/migration/env_test.go` | tcp.bind + tcp.port produce bgp listen blocks | |
| `TestEnvEnvironmentMapping` | `internal/exabgp/migration/env_test.go` | daemon, cache, reactor, debug, tcp, bgp keys mapped to environment paths | |
| `TestEnvLogMapping` | `internal/exabgp/migration/env_test.go` | log topics mapped to Ze subsystem levels | |
| `TestEnvCommentOnly` | `internal/exabgp/migration/env_test.go` | unsupported keys produce comments | |
| `TestEnvDefaultsSkipped` | `internal/exabgp/migration/env_test.go` | default values not emitted | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| tcp.port value | 1-65535 | 65535 | 0 | 65536 |
| bgp listen port parsed from string | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-migrate-host-to-ip` | `test/parse/config-migrate-host-to-ip.ci` | User runs ze config migrate on config with host leaf, gets ip | |
| `config-migrate-bgp-listen` | `test/parse/config-migrate-bgp-listen.ci` | User runs ze config migrate on config with string bgp listen, gets container | |
| `config-migrate-tcp-port` | `test/parse/config-migrate-tcp-port.ci` | User runs ze config migrate on config with tcp.port, port moved to bgp listen | |
| `config-migrate-remove-env-bgp` | `test/parse/config-migrate-remove-env-bgp.ci` | User runs ze config migrate on config with bgp.connect, setting removed with warning | |
| `config-migrate-log-booleans` | `test/parse/config-migrate-log-booleans.ci` | User runs ze config migrate on config with log booleans, converted to subsystem levels | |
| `exabgp-migrate-env` | `test/parse/exabgp-migrate-env.ci` | User runs ze exabgp migrate --env with env file, gets Ze config with listen blocks and log levels | |

### Future (if deferring any tests)
- Property test: round-trip migration (migrate old config, parse result, compare semantics)

## Files to Modify
- `internal/component/config/migration/migrate.go` - add new transformations to pipeline
- `cmd/ze/exabgp/main.go` - add --env flag to cmdMigrate

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A |
| CLI commands/flags | [x] | `cmd/ze/exabgp/main.go` -- add --env flag |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- ze config migrate new transformations, ze exabgp migrate --env |
| 2 | Config syntax changed? | [ ] | N/A (migration handles old syntax, not new syntax) |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` -- ze exabgp migrate --env flag |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/migration.md` -- ExaBGP env file migration, ze config migrate new transformations |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `internal/component/config/migration/listener.go` - host-to-ip, bgp-listen, tcp-port, log-boolean transformations
- `internal/exabgp/migration/env.go` - ExaBGP env file parser and Ze config mapper
- `internal/exabgp/migration/env_test.go` - unit tests for env parser and mapper
- `test/parse/config-migrate-host-to-ip.ci`
- `test/parse/config-migrate-bgp-listen.ci`
- `test/parse/config-migrate-tcp-port.ci`
- `test/parse/config-migrate-remove-env-bgp.ci`
- `test/parse/config-migrate-log-booleans.ci`
- `test/parse/exabgp-migrate-env.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Ze config migrate transformations** -- host-to-ip, bgp-listen-container, tcp-port-to-listen, remove-env-bgp-connect, remove-env-bgp-accept, log-booleans-to-subsystems
   - Tests: TestHostToIPTransformation, TestBGPListenContainerTransformation, TestTCPPortToListenTransformation, TestRemoveEnvBGPConnect, TestRemoveEnvBGPAccept, TestLogBooleansToSubsystems
   - Files: internal/component/config/migration/listener.go, internal/component/config/migration/migrate.go
   - Verify: tests fail -> implement -> tests pass
2. **Phase: ExaBGP env file INI parser** -- parse INI format into section/key/value triples
   - Tests: TestParseExaBGPEnv
   - Files: internal/exabgp/migration/env.go
   - Verify: tests fail -> implement -> tests pass
3. **Phase: ExaBGP env to Ze config mapper** -- listener, environment, log, and comment-only mappings
   - Tests: TestEnvListenerMapping, TestEnvEnvironmentMapping, TestEnvLogMapping, TestEnvCommentOnly, TestEnvDefaultsSkipped
   - Files: internal/exabgp/migration/env.go
   - Verify: tests fail -> implement -> tests pass
4. **Phase: Wire --env flag into ze exabgp migrate** -- add flag, call parser, merge output
   - Tests: TestEnvListenerMapping (end-to-end via CLI)
   - Files: cmd/ze/exabgp/main.go
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Functional tests** -- create all .ci test files
   - Tests: all functional tests from TDD Plan
   - Files: test/parse/config-migrate-*.ci, test/parse/exabgp-migrate-env.ci
   - Verify: all .ci tests pass
6. **Phase: Full verification** -- make ze-verify
   - Verify: all tests pass including lint

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Port parsing handles edge cases (no port in string, port 0, port > 65535). Log boolean mapping correct (true -> debug, false -> disabled). Warning messages for removed settings are clear |
| Naming | Transformation names match the table (host-to-ip, bgp-listen-container, tcp-port-to-listen, etc.) |
| Data flow | Transformations registered in correct order in pipeline. Env parser output merges cleanly with config migration output |
| Rule: no-layering | No dual-format support; old formats are transformed, not kept alongside new |
| Rule: compatibility | No compat shims for old config formats (Ze unreleased) |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| host-to-ip transformation registered | `grep "host-to-ip" internal/component/config/migration/migrate.go` |
| bgp-listen-container transformation registered | `grep "bgp-listen" internal/component/config/migration/migrate.go` |
| tcp-port-to-listen transformation registered | `grep "tcp-port" internal/component/config/migration/migrate.go` |
| remove-env-bgp transformations registered | `grep "remove-env-bgp" internal/component/config/migration/migrate.go` |
| log-booleans-to-subsystems transformation registered | `grep "log-booleans" internal/component/config/migration/migrate.go` |
| --env flag on ze exabgp migrate | `grep "env" cmd/ze/exabgp/main.go` |
| ExaBGP env parser exists | `ls internal/exabgp/migration/env.go` |
| All 6 .ci functional tests exist | `ls test/parse/config-migrate-*.ci test/parse/exabgp-migrate-env.ci` |
| ze config migrate --list shows new transformations | functional test output |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Port values from INI file validated (1-65535). Malformed INI lines rejected with clear error |
| Path traversal | --env flag path does not allow reading arbitrary files beyond what the user specifies (standard file open, no special concern) |
| Error leakage | Migration warnings do not expose filesystem paths or internal state beyond the config being migrated |
| Resource exhaustion | Large or malformed INI files do not cause unbounded memory allocation (set reasonable line/file size limits) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

N/A -- no new RFC behavior. Migration tools are Ze-internal.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-listener-4-migrate.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
