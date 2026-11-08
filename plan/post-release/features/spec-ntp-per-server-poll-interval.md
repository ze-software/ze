# Spec: ntp-per-server-poll-interval

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P3 |
| Release reason | RFC 5905 is optional in the fttx-edge profile; the client ships with one global interval, so per-server polling is post-release work |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator wants to poll a local stratum-1 server often and a public pool
rarely. Ze's NTP client has one `interval` leaf (60 to 86400 seconds) and
queries every server in the same round, so the operator cannot set a poll
interval per server.

This spec adds a per-server minimum and maximum poll exponent, as RFC 5905
defines them, and schedules each server on its own poll timer. It also gives
`plan/post-release/features/spec-ntp-client-kiss-of-death.md` the per-server
poll state its RATE handling (its AC-3) needs: a RATE kiss lengthens that
server's interval, bounded by the maximum this spec configures.

VyOS reference: T8882 `44d216875` (`~/Code/github.com/vyos/vyos-1x`) added
`minpoll`, `maxpoll`, `presend` and `extfield-f323` per server. VyOS accepts
0 to 24 for the exponents; RFC 5905 bounds them at 4 to 17 (AC-3). `presend`
and `extfield-f323` are chrony options with no RFC behind them and are not in
this spec.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/ntp.md` or the NTP section of the services guide (find with `ai/INDEX.md`) - what the operator writes today

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5905.md` - summary exists; RFC 5905 is optional in fttx-edge
  → Constraint: RFC 5905 Section 5: "A client sends messages to each server with a poll interval of 2^tau seconds, as determined by the poll exponent tau.  In NTPv4, tau ranges from 4 (16 s) to 17 (36 h)."
  → Constraint: RFC 5905 Section 7.2, MINPOLL 4 and MAXPOLL 17

**Key insights:**
- The poll interval is per server in RFC 5905, so a global interval is a simplification, not the protocol's model.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ntp/ntp.go` - `syncWorker.run`: phase 2 retries every second until a sync succeeds, phase 3 sleeps `IntervalSec` then calls `doSync`; `doSync` queries every server in turn with `ntpQueryFn`, keeps `serverState` per address (reach register, last query, last error)
- [ ] `internal/plugins/ntp/yang/ze-ntp-conf.yang` - `environment ntp { enabled; interval 60..86400; max-step; slew-threshold; persist-path; server <name> { address; } }`

**Behavior to preserve:**
- The global `interval` stays the default for a server that sets no exponents
- The initial sync retry, the step and slew rules, the absurd-year check

**Behavior to change:**
- Each server polls on its own interval between its configured minimum and maximum

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Sync worker | `internal/plugins/ntp/ntp.go` `syncWorker.run`, `doSync` | One goroutine, one global timer, all servers per round | Extends: the same worker keeps one timer per server; no goroutine per server (`ai/rules/goroutine-lifecycle.md`) |
| Per-server state | `internal/plugins/ntp/ntp.go` `serverState`, `getOrCreatePeer` | Reach, last query, last error | Extends: adds the current poll exponent and next due time |
| Kiss-o'-Death spec | `plan/post-release/features/spec-ntp-client-kiss-of-death.md` | Plans RATE handling that lengthens one server's interval | Shares the per-server interval this spec adds; whichever lands first adds the state |
| DHCP-learned servers | `internal/plugins/ntp/ntp.go` `addDHCPServers` | Servers from DHCP option 42 | Extends: they take the default exponents |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `environment ntp server <name> { minpoll <n>; maxpoll <n>; }` (leaf names fixed at design)

### Transformation Path
1. `parseNTPConfig` reads the exponents and validates minimum not above maximum
2. The worker starts each server at its minimum exponent and schedules the next query at 2^tau seconds
3. A selection round uses each server's latest sample; a server not yet due keeps its last sample's age

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to NTP server | NTPv4 client mode, the Poll field carries tau | No |

### Changed-Wire Census
**Search evidence:** to write at design. The NTP request's Poll field is written by `github.com/beevik/ntp`; whether its `QueryOptions` lets Ze set it must be checked in `vendor/`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to fill at design] | | | | |

### Integration Points
- `ntpQueryFn`, `serverState`, the NTP status command

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to fill at design |
| No unintended coupling | No | plugin-local change |
| No duplicated functionality | No | one worker |
| Zero-copy preserved where applicable | No | N-A |
| Registration over hardcoding, outbound | No | N-A, no new command |
| Registration over hardcoding, inbound | No | to fill at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `beevik/ntp` lets the caller set the Poll field | not yet read | Ze sends a constant Poll the server may rate-limit against | read `vendor/github.com/beevik/ntp` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Per-server timers make a reload or stop wait on a long sleep | reload test hangs | one select over the stop channel and the earliest due time |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The clock syncs too rarely or floods a server |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-ntp-client-kiss-of-death.md` |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with two servers at different exponents | → | per-server schedule in `syncWorker.run` | `ntp-per-server-poll.ci` (planned, `test/plugin/`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Server A minimum and maximum 4, server B minimum and maximum 10 | Over 1100 seconds of fake clock, A is queried about every 16 s and B about every 1024 s; the status command shows each server's current interval | None |
| AC-2 | A server with no exponents | Polls at the global `interval`, as today | None |
| AC-3 | Exponent 3 or 18 | Refused at parse, naming the RFC 5905 range 4 to 17 | None |
| AC-4 | Minimum above maximum | Refused, naming both values | None |
| AC-5 | Stop or reload while the next query is far away | The worker ends within one query timeout | None |
| AC-6 | Status command from SSH, REST, gRPC, MCP and web | Same per-server poll fields on every surface | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPerServerPollSchedule` | `internal/plugins/ntp/ntp_test.go` | AC-1, AC-2 with an injected clock and `ntpQueryFn` | |
| `TestPollExponentRange` | `internal/plugins/ntp/ntp_test.go` | AC-3, AC-4 | |
| `TestStopDuringLongPoll` | `internal/plugins/ntp/reload_test.go` | AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| minpoll | 4-17 | 4, 17 | 3 | 18 |
| maxpoll | 4-17 | 4, 17 | 3 | 18 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ntp-per-server-poll` | `test/plugin/` | AC-1, AC-6 | |
| `ntp-poll-range` | `test/parse/` | AC-3, AC-4 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| existing chrony test | `internal/plugins/ntp/ntp_chrony_docker_test.go` | chrony | A query at the configured interval is answered and the Poll field chrony logs matches | |

## Files to Modify
- `internal/plugins/ntp/ntp.go` - per-server schedule
- `internal/plugins/ntp/yang/ze-ntp-conf.yang` - two leaves per server

## Files to Create
- the functional tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-ntp-conf.yang` |
| YANG validation constraints | Yes | range 4..17 |
| YANG custom validators | Yes | minimum not above maximum |
| CLI commands/flags | Yes | status gains fields |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | Yes | automatic for typed leaves |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | N-A | existing command |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new dependency |
| Prometheus counters/metrics | Yes | per-server poll exponent gauge |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed | Yes | the NTP guide page |
| 9 | RFC behavior | Yes | `rfc/short/rfc5905.md` rows on the poll interval |

## Implementation Steps
1. **Phase: Wiring** - leaves parsed, schedule stub, wiring test red
2. **Phase: Scheduler** - AC-1 to AC-5
3. **Phase: Status and metric** - AC-6

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Range 4 to 17 | 0 to 24 as VyOS and chrony accept | RFC 5905 Section 5 bounds tau there |

## Known Limitations
- NTS is `plan/post-release/features/spec-ntp-nts-client.md`.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional plugin`, `./le test functional parse`, `./le job run label ntp command go test -tags ze_ntp ./internal/plugins/ntp/`
