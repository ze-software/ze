# Spec: gokrazy-2 -- NTP Client Plugin

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-04-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `spec-gokrazy-0-umbrella.md` -- parent spec with research
3. `internal/component/config/system/schema/ze-system-conf.yang` -- system config YANG
4. `internal/plugins/ifacedhcp/register.go` -- plugin registration pattern
5. `internal/component/iface/iface.go` -- DHCPPayload struct (NTPServers field from spec-gokrazy-1)

## Task

Implement an NTP client plugin for ze. The plugin queries NTP servers, sets the
system clock via Settimeofday, writes to the hardware RTC when available, and
persists time to disk for recovery on devices without RTC. It subscribes to DHCP
lease events to discover NTP servers via option 42.

This is required before gokrazy's NTP can be removed from the appliance image.
gokrazy sets `WaitForClock: true` on ze, meaning ze does not start until the
clock is set. If ze owns NTP, the clock must be set before or during ze startup.

### Design: clock bootstrap on gokrazy

gokrazy's WaitForClock blocks ze until the gokrazy NTP has set the clock. If we
remove gokrazy's NTP, ze must set the clock itself. But ze blocks on WaitForClock.

**Solution:** the NTP plugin is an early-start internal plugin. It sets the clock
during its OnConfigure callback (before the main event loop). On gokrazy, we
remove WaitForClock from config.json since ze now owns the clock. The NTP plugin
runs as one of the first plugins (no dependencies except config), so it has time
to do an initial sync before BGP peers come up. Stale TLS certs and log timestamps
are acceptable for the few seconds before NTP sync completes.

**Alternative considered:** run a separate ze-ntp binary as a GokrazyPackage. Rejected
because it splits the config surface -- operators must configure NTP in two places.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- plugin lifecycle, event bus
  -> Constraint: plugins register via init(), start via 5-stage protocol
- [ ] `.claude/rules/plugin-design.md` -- registration fields, YANG, proximity
  -> Constraint: new plugin needs register.go, init(), YANG schema, ConfigRoots
- [ ] `.claude/rules/go-standards.md` -- new dependency approval
  -> Constraint: `beevik/ntp` is a new dependency, needs user approval
- [ ] `.claude/rules/goroutine-lifecycle.md` -- worker patterns
  -> Constraint: NTP sync is a long-lived goroutine, not per-event

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5905.md` -- NTPv4 specification
  -> Constraint: SNTP (client-only, no peer/server mode) is sufficient
  -> Constraint: client sends mode 3, expects mode 4 response

**Key insights:**
- SNTP subset is sufficient -- ze is a client, never a server
- beevik/ntp handles the protocol; ze manages lifecycle and clock setting
- Time persistence is critical for gokrazy (Raspberry Pi has no RTC)
- DHCP option 42 NTP servers arrive via event bus from ifacedhcp plugin

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/schema/ze-system-conf.yang` -- system config: host, domain, peeringdb, archive. No NTP.
- [ ] `internal/component/plugin/all/all.go` -- blank imports for all plugins
- [ ] `internal/plugins/ifacedhcp/register.go` -- example plugin registration pattern
- [ ] `gokrazy/ze/config.json` -- WaitForClock: true on ze package

**Behavior to preserve:**
- System config parsing for host, domain, peeringdb, archive
- Plugin registration and 5-stage startup protocol
- Event bus subscription patterns (ifacedhcp publishes, NTP subscribes)
- gokrazy build process

**Behavior to change:**
- ze-system-conf.yang gains ntp container under environment
- New ntp plugin registered in all.go
- DHCP lease events gain NTPServers field (done in spec-gokrazy-1)
- gokrazy config.json removes WaitForClock (done in spec-gokrazy-3)

## Data Flow (MANDATORY)

### Entry Point
- Config: `environment { ntp { enabled true; server { name pool0; address 0.pool.ntp.org } } }`
- DHCP lease event with NTPServers field (from option 42)

### Transformation Path
1. Config parsed by YANG schema walker into JSON tree
2. NTP plugin receives config via OnConfigure callback
3. NTP plugin extracts server list and settings
4. On startup: restore time from persistence file if exists (rough time)
5. NTP plugin starts sync goroutine
6. Sync goroutine queries NTP server, receives time response
7. If offset > threshold: call Settimeofday to set system clock
8. If `/dev/rtc0` exists: write time to hardware clock
9. Save current time to persistence file
10. Sleep for sync interval, repeat from step 6
11. On DHCP lease event with NTPServers: add those servers to pool

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> NTP plugin | SDK ConfigSection with "environment" root | [ ] |
| Event bus -> NTP plugin | Subscribe to interface/dhcp/lease-acquired events | [ ] |
| NTP plugin -> kernel | syscall.Settimeofday for clock, ioctl for RTC | [ ] |
| NTP plugin -> filesystem | Persistence file read/write | [ ] |

### Integration Points
- `ze-system-conf.yang` -- new ntp container under environment
- Plugin registry -- new "ntp" registration
- Event bus -- subscribe to DHCP lease events
- DHCPPayload.NTPServers -- source of DHCP-discovered NTP servers

### Architectural Verification
- [ ] No bypassed layers (config -> plugin -> NTP query -> clock set)
- [ ] No unintended coupling (NTP plugin does not import ifacedhcp directly)
- [ ] No duplicated functionality (no other time sync in ze)
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config `environment { ntp { enabled true } }` | -> | NTP plugin starts sync | `test/parse/ntp-config.ci` |
| Config `environment { ntp { enabled false } }` | -> | NTP plugin does not sync | `test/parse/ntp-disabled.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `ntp { enabled true; server { ... } }` | NTP plugin starts, queries servers |
| AC-2 | Config with `ntp { enabled false }` (or no ntp block) | NTP plugin does not start sync |
| AC-3 | NTP query returns valid time with offset > 100ms | System clock set via Settimeofday |
| AC-4 | NTP query returns valid time, `/dev/rtc0` exists | RTC updated |
| AC-5 | NTP query returns valid time | Time saved to persistence file |
| AC-6 | Boot with persistence file, no NTP yet | Clock set to saved time |
| AC-7 | Boot without persistence file, no RTC | Clock left as-is until NTP sync |
| AC-8 | NTP query fails | Retry after 1 second, log warning |
| AC-9 | NTP query succeeds | Next sync after configured interval (default 1 hour) |
| AC-10 | Multiple NTP servers configured | Servers queried with random selection |
| AC-11 | DHCP lease event with NTPServers | DHCP servers added to NTP server pool |
| AC-12 | Clean shutdown (SIGTERM) | Current time saved to persistence file |
| AC-13 | Config with invalid server address | Config rejected at parse time |
| AC-14 | NTP response with absurd timestamp (year < 2020 or > 2100) | Response rejected, retry |
| AC-15 | Sync interval configured as 300s | Sync happens every 300 seconds |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNTPConfigParse` | `internal/plugins/ntp/ntp_test.go` | Config parsed correctly | |
| `TestNTPConfigDisabled` | `internal/plugins/ntp/ntp_test.go` | Disabled by default | |
| `TestNTPTimePersistenceSave` | `internal/plugins/ntp/ntp_test.go` | Time saved to file | |
| `TestNTPTimePersistenceRestore` | `internal/plugins/ntp/ntp_test.go` | Time restored from file | |
| `TestNTPTimePersistenceMissing` | `internal/plugins/ntp/ntp_test.go` | Missing file handled gracefully | |
| `TestNTPTimePersistenceCorrupt` | `internal/plugins/ntp/ntp_test.go` | Corrupt file handled gracefully | |
| `TestNTPResponseValidation` | `internal/plugins/ntp/ntp_test.go` | Absurd timestamps rejected | |
| `TestNTPServerSelection` | `internal/plugins/ntp/ntp_test.go` | Random selection from pool | |
| `TestNTPDHCPServerMerge` | `internal/plugins/ntp/ntp_test.go` | DHCP servers added to pool | |
| `TestNTPSyncInterval` | `internal/plugins/ntp/ntp_test.go` | Custom interval respected | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Sync interval | 60s-86400s | 86400 | 59 | 86401 |
| Server count | 1-8 | 8 | 0 | 9 |
| Response year | 2020-2100 | 2100 | 2019 | 2101 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ntp-config` | `test/parse/ntp-config.ci` | NTP config parses correctly | |
| `ntp-disabled` | `test/parse/ntp-disabled.ci` | No NTP block, parses ok | |

### Future
- Live NTP test against real NTP server (requires network in CI)
- DHCP option 42 end-to-end test (requires DHCP server in CI)

## Files to Modify

- `internal/component/config/system/schema/ze-system-conf.yang` -- add ntp container
- `internal/component/plugin/all/all.go` -- blank import for ntp plugin

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (NTP config) | [x] | `ze-system-conf.yang` |
| CLI commands/flags | [ ] | N/A for now (stretch: `show ntp status`) |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test | [x] | `test/parse/ntp-config.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- NTP client |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- NTP config |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- NTP plugin |
| 6 | Has a user guide page? | [ ] | Could add docs/features/ntp.md |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc5905.md` -- SNTP subset |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- NTP support |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/plugins/ntp/ntp.go` -- NTP sync logic (query, validate, schedule)
- `internal/plugins/ntp/ntp_linux.go` -- Settimeofday, RTC write (linux build tag)
- `internal/plugins/ntp/persist.go` -- time persistence (save/restore)
- `internal/plugins/ntp/register.go` -- plugin registration
- `internal/plugins/ntp/ntp_test.go` -- unit tests
- `internal/plugins/ntp/schema/ze-ntp-conf.yang` -- NTP YANG schema (or add to ze-system-conf.yang)
- `test/parse/ntp-config.ci` -- functional test
- `test/parse/ntp-disabled.ci` -- functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist |
| 6. Fix issues | Fix every issue |
| 7. Re-verify | `make ze-verify` |
| 8. Repeat 5-7 | Max 2 passes |
| 9. Deliverables review | Deliverables Checklist |
| 10. Security review | Security Review Checklist |
| 11. Re-verify | `make ze-verify` |
| 12. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: YANG schema** -- add NTP config to system YANG
   - Add ntp container with enabled, interval, server list
   - Tests: parse tests
   - Verify: YANG validates

2. **Phase: Plugin skeleton** -- register NTP plugin
   - register.go with init(), ConfigRoots, YANG
   - Blank import in all.go
   - OnConfigure parses NTP config
   - Tests: plugin registration test
   - Verify: compiles

3. **Phase: Time persistence** -- save/restore time from file
   - persist.go: SaveTime(path), RestoreTime(path)
   - Write unix timestamp to file, read on startup
   - Tests: TestNTPTimePersistence*
   - Verify: tests fail -> implement -> pass

4. **Phase: NTP query and clock set** -- core sync logic
   - ntp.go: query server pool, validate response, compute offset
   - ntp_linux.go: Settimeofday, RTC write
   - Anti-thundering-herd jitter (0-250ms random before query)
   - Tests: TestNTPResponseValidation, TestNTPServerSelection
   - Verify: tests pass

5. **Phase: Sync goroutine** -- long-lived worker
   - OnConfigure starts sync goroutine if enabled
   - Goroutine: restore time -> initial sync -> sleep interval -> repeat
   - On shutdown: save time, stop goroutine
   - Tests: TestNTPSyncInterval
   - Verify: tests pass

6. **Phase: DHCP option 42 integration** -- subscribe to lease events
   - Subscribe to interface/dhcp/lease-acquired events
   - Extract NTPServers from DHCPPayload
   - Merge into server pool (DHCP servers have lower priority than configured)
   - Tests: TestNTPDHCPServerMerge
   - Verify: tests pass

7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- audit, learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Settimeofday called only when offset > threshold |
| Naming | YANG keys kebab-case, plugin name "ntp" |
| Data flow | Config -> OnConfigure -> sync goroutine -> clock set |
| Rule: goroutine-lifecycle | Sync is one long-lived goroutine, not per-event |
| Rule: go-standards | New dependency (beevik/ntp) approved by user |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| NTP plugin registered | `grep ntp internal/component/plugin/all/all.go` |
| YANG schema has ntp | `grep ntp internal/component/config/system/schema/ze-system-conf.yang` |
| Time persistence works | `TestNTPTimePersistence*` passes |
| Response validation | `TestNTPResponseValidation` passes |
| Config parse test | `test/parse/ntp-config.ci` passes |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | NTP server addresses validated as hostnames or IPs |
| Time bounds | Reject responses outside 2020-2100 range |
| Resource exhaustion | Retry backoff prevents tight loops (1s min) |
| Privilege | Settimeofday requires CAP_SYS_TIME (gokrazy grants it) |
| File safety | Persistence file written atomically |
| NTP amplification | ze is client-only, never responds to NTP queries |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| New dependency rejected | Ask user for approved alternative |
| Settimeofday EPERM | Check capabilities, document requirement |
| RTC ioctl fails | Log warning, continue without RTC (graceful) |
| 3 fix attempts fail | STOP. Ask user. |

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

- The NTP plugin must not create a circular dependency with DHCP. It subscribes to
  DHCP events via the event bus (loose coupling). If no DHCP lease arrives, it uses
  only the configured servers.
- gokrazy's WaitForClock must be removed from config.json when ze owns NTP. Otherwise
  ze blocks waiting for a clock that only ze can set. This is done in spec-gokrazy-3.
- Time persistence path on gokrazy is `/perm/ze/timefile` (ext4 persistent partition).
  On non-gokrazy systems, use a configurable path defaulting to `/var/lib/ze/timefile`.
- The 0-250ms jitter before NTP queries is important for gokrazy deployments where
  multiple devices boot simultaneously (e.g., after a power outage).

## RFC Documentation

- RFC 5905 Section 7: NTP on-wire protocol (mode 3 client request, mode 4 server response)
- RFC 5905 Section 10: SNTP subset -- client-only, no peer/server state machine
- RFC 5905 Section 7.3: Packet header format (timestamps, reference ID)

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests
- [ ] Functional tests

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-gokrazy-2-ntp.md`
- [ ] Summary included in commit
