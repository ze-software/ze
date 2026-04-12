# Spec: gokrazy-0 -- Own DHCP and NTP (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/plugins/ifacedhcp/` -- DHCP plugin (all files)
4. `internal/component/iface/config.go` -- interface config parsing
5. `internal/component/iface/backend.go` -- Backend interface
6. `gokrazy/ze/config.json` -- gokrazy appliance config
7. Child specs: `spec-gokrazy-1-dhcp-wiring.md` through `spec-gokrazy-4-resilience.md`

## Task

Replace gokrazy's built-in DHCP and NTP services with ze's own implementations,
then exclude them from the gokrazy appliance build. All network configuration
and time synchronization flows through ze's config pipeline, giving operators a
single control plane.

**Goal:** a gokrazy appliance image where ze owns DHCP, DNS resolver config, NTP,
and the gokrazy `config.json` explicitly excludes `cmd/dhcp` and `cmd/ntp`.

### Context

Ze runs as a gokrazy appliance (`gokrazy/ze/config.json`). Today gokrazy's
default `GokrazyPackages` include its own DHCP client and NTP client. Ze has a
working DHCP plugin (`internal/plugins/ifacedhcp/`) with DHCPv4 and DHCPv6, but
it is not config-driven -- the YANG leaves exist but `config.go` does not parse
them. Ze has zero NTP code.

gokrazy's `WaitForClock: true` on the ze package config means ze blocks until
the clock is set. If we remove gokrazy's NTP, ze must set the clock itself or
the appliance hangs.

### Research Summary: gokrazy DHCP

| Feature | gokrazy | ze today |
|---------|---------|----------|
| DHCPv4 DORA | Yes (manual packets) | Yes (`insomniacslk/dhcp` library) |
| DHCPv6 | No | Yes (SARR + IA_NA + IA_PD) |
| Lease renewal T1/T2 | Yes | Yes |
| Hostname in DHCP | Yes | YANG leaf exists, not parsed |
| Client-ID | No | YANG leaf exists, not parsed |
| DNS from DHCP | Yes (writes `/tmp/resolv.conf`) | Payload has DNS, not applied |
| Default route from DHCP | Yes (via netlink) | Payload has Router, not applied |
| Link-state failover | Yes (deprioritize on carrier loss) | No |
| Route priority | Yes (eth=1, wlan=5, down=1024) | No |
| DNS conflict detection | Yes | No |

### Research Summary: gokrazy NTP

| Feature | gokrazy | ze today |
|---------|---------|----------|
| NTP client | Yes (`beevik/ntp`) | No |
| Server pool | Yes (0-3.gokrazy.pool.ntp.org) | No |
| Custom servers | Yes (CLI flag) | No |
| Settimeofday syscall | Yes | No |
| RTC write (`/dev/rtc0`) | Yes | No |
| Time persistence to disk | Yes (save on SIGTERM) | No |
| Boot time recovery | Yes (read saved time) | No |
| Sync interval | 1h success, 1s retry | No |
| Anti-thundering-herd jitter | Yes (0-250ms) | No |
| DHCP option 42 NTP servers | No | No |
| WaitForClock API | Yes | No |

### Scope

**In scope:**

| Area | Description |
|------|-------------|
| DHCP config wiring | Parse YANG leaves, drive ifacedhcp plugin from config |
| Default route from DHCP | Add route management to Backend, install gateway from lease |
| DNS from DHCP | Write `/tmp/resolv.conf` from lease |
| DHCP hostname/client-id | Pass through to DHCPv4 packets |
| NTP client plugin | Lightweight NTP, set system clock, RTC, persistence |
| NTP YANG config | `environment/ntp` with server list, enabled, interval |
| DHCP option 42 | Use DHCP-provided NTP servers |
| Gokrazy build change | Exclude `cmd/dhcp` and `cmd/ntp` from `GokrazyPackages` |
| Appliance docs | Update `docs/guide/appliance.md` |

**Out of scope:**

| Area | Reason |
|------|--------|
| DHCPv4 server | Ze is a client, not a DHCP server |
| Full NTP server | Ze syncs its own clock, does not serve time |
| DHCP relay | Not needed for appliance use case |
| WiFi management | No wlan support in ze |
| IPv6 SLAAC from DHCP | SLAAC is separate from DHCPv6, already has sysctl |

### Child Specs

| # | Spec | Status | What | Depends |
|---|------|--------|------|---------|
| 1 | `spec-gokrazy-1-dhcp-wiring.md` | design | Wire DHCP config parsing, default route, DNS, hostname | - |
| 2 | `spec-gokrazy-2-ntp.md` | design | NTP client, system time, RTC, time persistence, YANG | - |
| 3 | `spec-gokrazy-3-build.md` | skeleton | Gokrazy config change, QEMU test, docs | 1, 2 |
| 4 | `spec-gokrazy-4-resilience.md` | skeleton | Link-state failover, readiness gate | - |

**Dependency order:** 1 and 2 are independent (can run in parallel). 3 depends on
both 1 and 2. 4 is independent polish work.

### Decisions (agreed)

| Decision | Detail |
|----------|--------|
| NTP as ze plugin | Not a managed external process -- ze owns the clock |
| `/tmp/resolv.conf` for DNS | Same as gokrazy; gokrazy rootfs is read-only |
| Route management in Backend | Needed for DHCP default route installation |
| `beevik/ntp` library | Same as gokrazy, proven, small (needs user approval for new dep) |
| DHCP option 42 for NTP | Ze does what gokrazy doesn't -- discover NTP servers via DHCP |
| Time persistence path | `/perm/ze/timefile` on gokrazy (ext4 persistent partition) |
| DHCP flows through interface plugin | ifacedhcp depends on interface; config parsed in iface's OnConfigure |

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` -- interface management, DHCP plugin
  -> Constraint: ifacedhcp is a separate plugin from the interface plugin
- [ ] `docs/architecture/core-design.md` -- plugin model, event bus
  -> Constraint: plugins communicate via event bus, not direct calls
- [ ] `docs/guide/appliance.md` -- gokrazy build and deployment
  -> Constraint: rootfs read-only, persistent storage at /perm

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2131.md` -- DHCPv4 (existing)
  -> Constraint: DORA handshake, T1/T2 renewal timers
- [ ] `rfc/short/rfc2132.md` -- DHCP options (hostname, DNS, NTP servers)
  -> Constraint: option 12 hostname, option 6 DNS, option 42 NTP
- [ ] `rfc/short/rfc5905.md` -- NTPv4
  -> Constraint: SNTP subset sufficient for client-only mode

**Key insights:**
- ifacedhcp plugin already works but is not config-driven
- Backend interface has no route management -- must be extended
- DHCP payload already carries Router and DNS but nothing applies them
- gokrazy uses `WaitForClock: true` -- ze must set clock before starting

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ifacedhcp/register.go` -- registers as "iface-dhcp", no ConfigRoots, no YANG
- [ ] `internal/plugins/ifacedhcp/dhcp_linux.go` -- DHCPClient lifecycle, Start/Stop, event publishing
- [ ] `internal/plugins/ifacedhcp/dhcp_v4_linux.go` -- DHCPv4 DORA, address install, T1/T2 renewal
- [ ] `internal/plugins/ifacedhcp/dhcp_v6_linux.go` -- DHCPv6 SARR, IA_NA/IA_PD, address install
- [ ] `internal/component/iface/config.go` -- parseIfaceConfig, unitEntry has no DHCP fields
- [ ] `internal/component/iface/iface.go` -- DHCPPayload struct (has Router, DNS fields)
- [ ] `internal/component/iface/backend.go` -- Backend interface, no route methods
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` -- DHCP YANG leaves at lines 180-226
- [ ] `internal/component/config/system/schema/ze-system-conf.yang` -- no NTP config
- [ ] `gokrazy/ze/config.json` -- no GokrazyPackages field, defaults include dhcp+ntp

**Behavior to preserve:**
- Static IP interface configuration works unchanged
- DHCP plugin address installation via ReplaceAddressWithLifetime
- DHCP lease event publishing on event bus
- All existing tunnel, bridge, wireguard, VLAN config unchanged
- gokrazy build process unchanged (just different package list)

**Behavior to change:**
- `config.go` parses DHCP YANG leaves into unitEntry
- Interface plugin drives ifacedhcp based on config
- DHCP lease installs default route (new Backend method)
- DHCP lease writes DNS to `/tmp/resolv.conf`
- Hostname and client-id passed to DHCPv4 client
- New NTP plugin sets system clock
- `gokrazy/ze/config.json` gains explicit `GokrazyPackages` excluding dhcp/ntp

## Data Flow (MANDATORY)

### Entry Points

| Entry | Format | Component |
|-------|--------|-----------|
| Config file | YANG-modeled tree with `interface { ethernet { eth0 { unit 0 { dhcp { enabled true } } } } }` | iface plugin |
| DHCP lease ACK | Binary DHCPv4/v6 packet parsed by insomniacslk library | ifacedhcp plugin |
| NTP response | NTP packet parsed by beevik/ntp library | ntp plugin (new) |
| DHCP option 42 | NTP server IPs inside DHCP lease | ifacedhcp -> ntp (via event bus) |

### Transformation Path

1. Config parse: `parseIfaceSections` -> `parseUnits` reads `dhcp.enabled`, `dhcp.hostname`, `dhcp.client-id`
2. Interface plugin `OnConfigure`: for each unit with `dhcp.enabled=true`, starts ifacedhcp client
3. ifacedhcp performs DORA/SARR, receives lease with address, router, DNS, NTP servers
4. ifacedhcp installs address (existing), installs default route (new), writes resolv.conf (new)
5. ifacedhcp publishes lease event on event bus (existing) including NTP servers (new field)
6. NTP plugin receives lease event (or uses configured servers), queries NTP, sets system clock

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> iface plugin | SDK ConfigSection with "interface" root | [ ] |
| iface plugin -> ifacedhcp | iface plugin creates DHCPClient with parsed config | [ ] |
| ifacedhcp -> event bus | JSON lease event with Router, DNS, NTP fields | [ ] |
| event bus -> NTP plugin | NTP plugin subscribes to DHCP lease events for option 42 | [ ] |
| ifacedhcp -> Backend | Backend.AddRoute for default gateway, Backend.WriteResolvConf for DNS | [ ] |

### Integration Points
- `ifaceConfig` / `unitEntry` in `config.go` -- new DHCP fields
- `DHCPClient` constructor -- accepts hostname, client-id parameters
- `Backend` interface -- new `AddRoute`, `RemoveRoute` methods
- `DHCPPayload` struct -- new `NTPServers` field
- New NTP plugin registration in `internal/plugins/ntp/register.go`
- `ze-system-conf.yang` -- new `ntp` container under `environment`

### Architectural Verification
- [ ] No bypassed layers (DHCP flows through config -> plugin -> backend)
- [ ] No unintended coupling (NTP plugin receives NTP servers via event bus, not import)
- [ ] No duplicated functionality (extends existing ifacedhcp, does not recreate)
- [ ] Zero-copy preserved where applicable (not wire-encoding work, N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `dhcp { enabled true }` | -> | ifacedhcp starts on that interface | `test/plugin/dhcp-config.ci` |
| DHCP lease with Router field | -> | Backend.AddRoute installs default gateway | `test/plugin/dhcp-route.ci` |
| DHCP lease with DNS field | -> | `/tmp/resolv.conf` written | `test/plugin/dhcp-dns.ci` |
| `environment { ntp { enabled true } }` config | -> | NTP plugin queries servers, sets clock | `test/plugin/ntp-basic.ci` |
| DHCP lease with NTP servers (option 42) | -> | NTP plugin uses DHCP-provided servers | `test/plugin/ntp-dhcp.ci` |
| gokrazy image without cmd/dhcp and cmd/ntp | -> | Ze boots, gets IP, syncs clock | QEMU boot test (manual) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `dhcp { enabled true }` on eth0 unit 0 | ifacedhcp client starts for eth0 |
| AC-2 | Config with `dhcp { enabled false }` (default) | No DHCP client started |
| AC-3 | Config with `dhcp { hostname "ze-router" }` | Hostname sent in DHCPv4 DISCOVER/REQUEST |
| AC-4 | DHCP lease with Router option | Default route installed via Backend.AddRoute |
| AC-5 | DHCP lease with DNS option | `/tmp/resolv.conf` updated with nameserver |
| AC-6 | DHCP lease expires | Default route removed, resolv.conf cleared |
| AC-7 | Config with `ntp { enabled true; server { ... } }` | NTP plugin queries configured servers |
| AC-8 | NTP query succeeds | System clock set via Settimeofday |
| AC-9 | NTP query succeeds + `/dev/rtc0` exists | Hardware clock updated |
| AC-10 | Clean shutdown | Current time saved to persistence file |
| AC-11 | Boot with no RTC, persistence file exists | Clock set to saved time before NTP |
| AC-12 | DHCP lease with option 42 NTP servers | NTP plugin uses those servers |
| AC-13 | `GokrazyPackages` set in config.json | Image excludes gokrazy dhcp and ntp |
| AC-14 | QEMU boot of gokrazy image | Ze acquires IP via DHCP, syncs clock, starts BGP |
| AC-15 | Static IP config (no DHCP) | Works exactly as today, no regression |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseUnitDHCP` | `internal/component/iface/config_test.go` | DHCP leaves parsed into unitEntry | |
| `TestParseUnitDHCPDisabled` | `internal/component/iface/config_test.go` | DHCP disabled by default | |
| `TestDHCPPayloadNTPServers` | `internal/plugins/ifacedhcp/dhcp_test.go` | NTP servers extracted from option 42 | |
| `TestNTPQueryAndSet` | `internal/plugins/ntp/ntp_test.go` | NTP query returns time, clock set | |
| `TestNTPTimePersistence` | `internal/plugins/ntp/ntp_test.go` | Save/restore time from file | |
| `TestResolvConfWrite` | `internal/plugins/ifacedhcp/resolv_test.go` | DNS servers written correctly | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NTP sync interval | 60s-86400s | 86400 | 59 | 86401 |
| DHCPv6 PD length | 48-64 | 64 | 47 | 65 |
| NTP server count | 1-8 | 8 | 0 | 9 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dhcp-config-enabled` | `test/plugin/dhcp-config-enabled.ci` | DHCP enabled in config, client starts | |
| `dhcp-config-disabled` | `test/parse/dhcp-config-disabled.ci` | DHCP disabled (default), no client | |
| `ntp-basic` | `test/plugin/ntp-basic.ci` | NTP configured, clock query | |
| `static-ip-no-regression` | `test/parse/static-ip-no-regression.ci` | Static config still works | |

### Future (if deferring any tests)
- QEMU integration test for full gokrazy boot (requires QEMU infrastructure in CI)
- Link-state failover tests (spec-gokrazy-4)

## Files to Modify

- `internal/component/iface/config.go` -- parse DHCP leaves into unitEntry
- `internal/component/iface/iface.go` -- extend DHCPPayload with NTPServers field
- `internal/component/iface/backend.go` -- add AddRoute/RemoveRoute to Backend interface
- `internal/component/iface/dispatch.go` -- add AddRoute/RemoveRoute dispatch functions
- `internal/component/iface/register.go` -- start ifacedhcp from OnConfigure when DHCP enabled
- `internal/plugins/ifacedhcp/dhcp_linux.go` -- accept hostname/client-id params
- `internal/plugins/ifacedhcp/dhcp_v4_linux.go` -- send hostname/client-id, install route, write DNS, extract option 42
- `internal/plugins/ifacedhcp/dhcp_v6_linux.go` -- install route, write DNS
- `internal/plugins/ifacenetlink/netlink_linux.go` -- implement AddRoute/RemoveRoute
- `internal/component/config/system/schema/ze-system-conf.yang` -- add NTP container
- `gokrazy/ze/config.json` -- add GokrazyPackages field
- `docs/guide/appliance.md` -- update for ze-owned DHCP/NTP

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (NTP config) | [x] | `ze-system-conf.yang` |
| CLI commands/flags | [ ] | NTP status via show commands (child spec) |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test for DHCP config | [x] | `test/plugin/dhcp-config-enabled.ci` |
| Functional test for NTP | [x] | `test/plugin/ntp-basic.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- NTP client, DHCP-driven DNS/routes |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- NTP config, DHCP config |
| 3 | CLI command added/changed? | [ ] | N/A for umbrella |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- NTP plugin |
| 6 | Has a user guide page? | [x] | `docs/guide/appliance.md` -- updated for ze-owned DHCP/NTP |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc5905.md` -- NTPv4 SNTP subset |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- DHCP/NTP features |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/plugins/ntp/ntp.go` -- NTP client plugin
- `internal/plugins/ntp/ntp_linux.go` -- Settimeofday, RTC write (linux-only)
- `internal/plugins/ntp/register.go` -- plugin registration
- `internal/plugins/ntp/ntp_test.go` -- unit tests
- `internal/plugins/ifacedhcp/resolv_linux.go` -- resolv.conf writer (linux-only)
- `test/plugin/dhcp-config-enabled.ci` -- functional test
- `test/parse/dhcp-config-disabled.ci` -- parse test
- `test/plugin/ntp-basic.ci` -- functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + child specs |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Child specs in order: 1 (DHCP), 2 (NTP), 3 (build) |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: DHCP config wiring** (spec-gokrazy-1)
   - Parse DHCP YANG leaves, extend unitEntry, drive ifacedhcp from config
   - Add Backend.AddRoute/RemoveRoute, install default route from lease
   - Write DNS to /tmp/resolv.conf, pass hostname/client-id to DHCPv4
   - Tests: config parsing, route install, DNS write, functional tests
   - Verify: `make ze-verify`

2. **Phase: NTP client** (spec-gokrazy-2)
   - New NTP plugin with YANG config under `environment/ntp`
   - System clock set, RTC write, time persistence
   - DHCP option 42 integration (subscribe to lease events for NTP servers)
   - Tests: NTP query, time set, persistence, functional tests
   - Verify: `make ze-verify`

3. **Phase: gokrazy build** (spec-gokrazy-3)
   - Add `GokrazyPackages` to config.json excluding dhcp and ntp
   - Update appliance docs
   - QEMU boot test
   - Verify: `make ze-gokrazy` builds, QEMU boots successfully

4. **Phase: resilience** (spec-gokrazy-4, stretch)
   - Link-state failover
   - Readiness gate
   - Route priority config

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented in child spec |
| Correctness | DHCP config parsing matches YANG schema exactly |
| Naming | YANG leaves use kebab-case, JSON uses kebab-case |
| Data flow | Config -> iface plugin -> ifacedhcp, not bypassed |
| Rule: no-layering | No duplicate DHCP config path |
| Rule: integration-completeness | Every feature reachable from config file |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| DHCP config parsing works | `test/plugin/dhcp-config-enabled.ci` passes |
| NTP plugin registered | `grep "ntp" internal/component/plugin/all/all.go` |
| Default route installed from DHCP | Unit test for Backend.AddRoute |
| DNS written from DHCP | Unit test for resolv.conf writer |
| gokrazy config excludes dhcp/ntp | `grep GokrazyPackages gokrazy/ze/config.json` |
| Static IP not regressed | `test/parse/static-ip-no-regression.ci` passes |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | NTP server addresses validated before use |
| Resource exhaustion | NTP retry backoff prevents tight loops |
| File write safety | resolv.conf written atomically (write tmp + rename) |
| Time bounds | Reject NTP responses with absurd timestamps |
| DHCP option parsing | Bounded option lengths, no buffer overflows (library handles) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
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

- ifacedhcp is registered as a plugin with `Dependencies: []string{"interface"}` but has no
  ConfigRoots and no YANG -- it is not config-driven today
- The Backend interface has zero route management methods; adding AddRoute/RemoveRoute is a
  clean extension that all backends must implement
- DHCPPayload already has Router and DNS string fields but nothing acts on them
- The iface plugin's OnConfigure is the right place to start/stop DHCP clients based on config
- gokrazy's rootfs is read-only SquashFS; persistent storage at /perm (ext4); /tmp is tmpfs

## RFC Documentation

- RFC 2131: DHCPv4 protocol (existing implementation)
- RFC 2132: DHCP options -- option 12 (hostname), option 6 (DNS), option 42 (NTP servers)
- RFC 3315: DHCPv6 protocol (existing implementation)
- RFC 5905: NTPv4 -- SNTP subset sufficient for client-only mode

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

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
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-gokrazy-0-umbrella.md`
- [ ] Summary included in commit
