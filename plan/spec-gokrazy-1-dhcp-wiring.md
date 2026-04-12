# Spec: gokrazy-1 -- DHCP Config Wiring

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-04-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `spec-gokrazy-0-umbrella.md` -- parent spec with research
3. `internal/component/iface/config.go` -- interface config parsing (unitEntry at line 78)
4. `internal/plugins/ifacedhcp/dhcp_linux.go` -- DHCPClient constructor and lifecycle
5. `internal/plugins/ifacedhcp/dhcp_v4_linux.go` -- v4Payload, handleV4Lease
6. `internal/component/iface/backend.go` -- Backend interface (no route methods)
7. `internal/component/iface/register.go` -- OnConfigure flow

## Task

Wire the existing DHCP YANG config leaves through the config parsing pipeline so
the ifacedhcp plugin is driven from config. Add default route installation from
DHCP lease. Add DNS resolver config from DHCP lease. Pass hostname and client-id
to DHCPv4 packets.

Today the DHCP plugin works at the code level but a user cannot enable it from
config -- the YANG leaves exist but `config.go` does not parse them, and the
interface plugin's `OnConfigure` does not start DHCP clients.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` -- interface plugin design
  -> Constraint: ifacedhcp is a separate plugin, depends on interface plugin
- [ ] `.claude/rules/plugin-design.md` -- plugin registration patterns
  -> Constraint: plugins communicate via event bus, not direct import
- [ ] `.claude/patterns/config-option.md` -- config option pattern
  -> Constraint: YANG leaf -> config parsing -> application

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2131.md` -- DHCPv4 protocol
  -> Constraint: client sends hostname in DISCOVER (option 12)
- [ ] `rfc/short/rfc2132.md` -- DHCP options
  -> Constraint: option 6 DNS servers, option 12 hostname, option 42 NTP, option 61 client-id

**Key insights:**
- unitEntry (config.go:78) has no DHCP fields; needs dhcpConfig sub-struct
- DHCPClient constructor takes ifaceName, unit, eventBus, v4, v6 -- needs hostname, clientID
- v4Payload already extracts Router and DNS from ACK but nothing applies them
- Backend interface has no route methods; must be extended
- OnConfigure in register.go is where DHCP clients should be started

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/config.go:78-87` -- unitEntry struct: ID, VLANID, Addresses, Disable, IPv4, IPv6, MirrorIngress, MirrorEgress
  -> Constraint: no DHCP fields, no route fields
- [ ] `internal/component/iface/config.go:112-124` -- parseIfaceSections: finds "interface" section
- [ ] `internal/component/iface/config.go:400+` -- parseUnits: iterates unit map, builds unitEntry slice
- [ ] `internal/component/iface/register.go:105-138` -- OnConfigure: parses config, loads backend, calls applyConfig, starts monitor
- [ ] `internal/plugins/ifacedhcp/dhcp_linux.go:40-61` -- NewDHCPClient: requires ifaceName, unit, eventBus, v4, v6
- [ ] `internal/plugins/ifacedhcp/dhcp_v4_linux.go:156-188` -- handleV4Lease: installs address, publishes event
- [ ] `internal/plugins/ifacedhcp/dhcp_v4_linux.go:214-243` -- v4Payload: extracts Router, DNS from ACK options
- [ ] `internal/component/iface/backend.go:21-103` -- Backend interface: no AddRoute/RemoveRoute
- [ ] `internal/component/iface/iface.go:87-95` -- DHCPPayload: Name, Unit, Address, PrefixLength, Router, DNS, LeaseTime
- [ ] `internal/component/iface/schema/ze-iface-conf.yang:180-226` -- dhcp and dhcpv6 containers in interface-unit grouping

**Behavior to preserve:**
- Static IP address configuration (Addresses field in unitEntry)
- DHCP address installation via ReplaceAddressWithLifetime
- DHCP lease event publishing (TopicDHCPLeaseAcquired/Renewed/Expired)
- All other interface types (bridge, tunnel, wireguard, VLAN, etc.)
- The ifacedhcp plugin's existing Start/Stop lifecycle

**Behavior to change:**
- unitEntry gains DHCP config fields (enabled, hostname, client-id, v6 enabled, PD length, DUID)
- parseUnits reads DHCP container from each unit's JSON
- OnConfigure starts/stops DHCPClient instances based on DHCP config
- DHCPClient constructor accepts hostname and client-id
- DHCPv4 sends hostname (option 12) and client-id (option 61) in DISCOVER/REQUEST
- handleV4Lease installs default route from Router option
- handleV4Lease writes DNS servers to /tmp/resolv.conf
- handleV4Lease extracts NTP servers from option 42 into DHCPPayload
- On lease expiry, route and resolv.conf are cleaned up

## Data Flow (MANDATORY)

### Entry Point
- Config file with `interface { ethernet { eth0 { unit 0 { dhcp { enabled true; hostname "ze" } } } } }`
- Parsed by YANG schema walker into JSON tree

### Transformation Path
1. `parseIfaceSections` receives ConfigSection with root "interface"
2. `parseIfaceConfig` walks JSON tree per interface type (ethernet, dummy, etc.)
3. `parseUnits` iterates unit map -- **new:** reads `dhcp` and `dhcpv6` containers
4. `unitEntry` populated with **new** `DHCP *dhcpUnitConfig` field
5. `OnConfigure` in register.go iterates units -- **new:** for each unit with DHCP enabled, creates and starts DHCPClient
6. DHCPClient performs DORA/SARR, receives lease
7. `handleV4Lease` installs address (existing) + **new:** installs route, writes DNS
8. On config reload: units with DHCP toggled are started/stopped accordingly
9. On lease expiry: route removed, resolv.conf cleared (existing address removal stays)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config JSON -> unitEntry | parseUnits reads dhcp container | [ ] |
| unitEntry -> DHCPClient | OnConfigure passes hostname/clientID from config | [ ] |
| DHCPClient -> Backend | Backend.AddRoute for default gateway | [ ] |
| DHCPClient -> filesystem | Write /tmp/resolv.conf atomically | [ ] |
| DHCPClient -> event bus | Publish lease event with NTP servers | [ ] |

### Integration Points
- `parseUnits` in config.go -- extend to read dhcp/dhcpv6 containers
- `OnConfigure` in register.go -- start DHCPClient per enabled unit
- `NewDHCPClient` in dhcp_linux.go -- accept hostname, clientID parameters
- `nclient4.New` options -- WithOption for hostname (12) and client-id (61)
- `Backend` interface -- new AddRoute/RemoveRoute methods
- `ifacenetlink` backend -- implement AddRoute/RemoveRoute via vishvananda/netlink

### Architectural Verification
- [ ] No bypassed layers (config -> iface plugin -> ifacedhcp -> backend)
- [ ] No unintended coupling (ifacedhcp uses Backend via iface package functions)
- [ ] No duplicated functionality (extends existing DHCPClient, does not recreate)
- [ ] Zero-copy preserved where applicable (N/A -- config parsing, not wire encoding)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config `dhcp { enabled true }` | -> | DHCPClient.Start called | `test/parse/dhcp-config-enabled.ci` |
| Config with no dhcp block | -> | No DHCPClient started | `test/parse/dhcp-config-disabled.ci` |
| DHCP ACK with Router option | -> | Backend.AddRoute called | `internal/plugins/ifacedhcp/dhcp_v4_linux_test.go` |
| DHCP ACK with DNS option | -> | /tmp/resolv.conf written | `internal/plugins/ifacedhcp/resolv_linux_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `dhcp { enabled true }` on unit 0 | ifacedhcp client starts for that interface |
| AC-2 | Config with no dhcp block (default) | No DHCP client started, static IP works |
| AC-3 | Config with `dhcp { hostname "ze-router" }` | Option 12 sent in DHCPv4 DISCOVER and REQUEST |
| AC-4 | Config with `dhcp { client-id "ze:01" }` | Option 61 sent in DHCPv4 packets |
| AC-5 | DHCP ACK contains Router option | Default route 0.0.0.0/0 via router IP installed |
| AC-6 | DHCP ACK contains DNS option | `/tmp/resolv.conf` contains `nameserver <ip>` |
| AC-7 | DHCP ACK contains multiple DNS servers | All nameservers written to resolv.conf |
| AC-8 | DHCP lease expires | Default route removed, resolv.conf cleared |
| AC-9 | DHCP lease renewed with different router | Old route replaced with new route |
| AC-10 | Config reload: DHCP toggled off | DHCPClient stopped, address/route removed |
| AC-11 | Config reload: DHCP toggled on | DHCPClient started for that unit |
| AC-12 | Config with `dhcpv6 { enabled true }` | DHCPv6 client starts (existing v6 code) |
| AC-13 | Config with `dhcpv6 { pd { length 56 } }` | PD length passed to DHCPv6 client |
| AC-14 | DHCP ACK contains NTP servers (option 42) | DHCPPayload includes NTPServers field |
| AC-15 | Static IP config alongside DHCP (different units) | Both work independently |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseUnitDHCPEnabled` | `internal/component/iface/config_test.go` | dhcp.enabled parsed to unitEntry | |
| `TestParseUnitDHCPHostname` | `internal/component/iface/config_test.go` | dhcp.hostname parsed | |
| `TestParseUnitDHCPClientID` | `internal/component/iface/config_test.go` | dhcp.client-id parsed | |
| `TestParseUnitDHCPv6PD` | `internal/component/iface/config_test.go` | dhcpv6.pd.length parsed | |
| `TestParseUnitDHCPDisabledDefault` | `internal/component/iface/config_test.go` | DHCP disabled when no block | |
| `TestDHCPClientWithHostname` | `internal/plugins/ifacedhcp/dhcp_test.go` | Hostname passed to v4 client | |
| `TestResolvConfWrite` | `internal/plugins/ifacedhcp/resolv_linux_test.go` | Single DNS server written | |
| `TestResolvConfMultipleDNS` | `internal/plugins/ifacedhcp/resolv_linux_test.go` | Multiple DNS servers written | |
| `TestResolvConfClear` | `internal/plugins/ifacedhcp/resolv_linux_test.go` | resolv.conf emptied on expiry | |
| `TestV4PayloadNTPServers` | `internal/plugins/ifacedhcp/dhcp_v4_linux_test.go` | Option 42 extracted | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| DHCPv6 PD length | 48-64 (YANG range) | 64 | 47 | 65 |
| Unit ID | 0-4094 | 4094 | N/A | 4095 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dhcp-config-enabled` | `test/parse/dhcp-config-enabled.ci` | Config with DHCP enabled parses ok | |
| `dhcp-config-disabled` | `test/parse/dhcp-config-disabled.ci` | Config without DHCP parses ok | |
| `dhcp-static-coexist` | `test/parse/dhcp-static-coexist.ci` | Static + DHCP on different units | |

### Future
- Live DHCP test against real DHCP server (requires test infrastructure)

## Files to Modify

- `internal/component/iface/config.go` -- add dhcpUnitConfig struct, parse DHCP leaves in parseUnits
- `internal/component/iface/iface.go` -- extend DHCPPayload with NTPServers []string field
- `internal/component/iface/backend.go` -- add AddRoute, RemoveRoute to Backend interface
- `internal/component/iface/dispatch.go` -- add AddRoute, RemoveRoute package functions
- `internal/component/iface/register.go` -- start/stop DHCPClient from OnConfigure and OnConfigApply
- `internal/plugins/ifacedhcp/dhcp_linux.go` -- extend NewDHCPClient with DHCPConfig parameter
- `internal/plugins/ifacedhcp/dhcp_v4_linux.go` -- send hostname/client-id, install route, write DNS, extract option 42
- `internal/plugins/ifacedhcp/dhcp_v6_linux.go` -- write DNS from v6 lease
- `internal/plugins/ifacenetlink/netlink_linux.go` -- implement AddRoute/RemoveRoute

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (DHCP) | [ ] | Already exists in ze-iface-conf.yang |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [ ] | YANG-driven (automatic) |
| Functional test | [x] | `test/parse/dhcp-config-enabled.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- DHCP config-driven, DNS from DHCP, routes from DHCP |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- DHCP config section under interface unit |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | ifacedhcp extended, not new |
| 6 | Has a user guide page? | [x] | `docs/features/interfaces.md` -- DHCP section |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc2132.md` -- options 6, 12, 42, 61 |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- DHCP features |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/plugins/ifacedhcp/resolv_linux.go` -- resolv.conf writer
- `internal/plugins/ifacedhcp/resolv_linux_test.go` -- resolv.conf tests
- `test/parse/dhcp-config-enabled.ci` -- functional test
- `test/parse/dhcp-config-disabled.ci` -- functional test
- `test/parse/dhcp-static-coexist.ci` -- functional test

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

1. **Phase: Config parsing** -- parse DHCP YANG leaves into unitEntry
   - Add dhcpUnitConfig struct to config.go
   - Extend parseUnits to read dhcp/dhcpv6 containers
   - Tests: TestParseUnitDHCP* in config_test.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Backend route methods** -- add AddRoute/RemoveRoute to Backend
   - Extend Backend interface
   - Implement in ifacenetlink backend
   - Add dispatch functions
   - Tests: unit test for route add/remove
   - Verify: compiles, tests pass

3. **Phase: DHCPClient config params** -- pass hostname, client-id, PD length
   - Extend NewDHCPClient to accept DHCPConfig struct
   - Pass hostname as option 12, client-id as option 61 to nclient4
   - Pass PD length to nclient6
   - Tests: TestDHCPClientWithHostname
   - Verify: tests pass

4. **Phase: Route and DNS from lease** -- install default route, write resolv.conf
   - handleV4Lease calls Backend.AddRoute for Router option
   - handleV4Lease writes DNS to /tmp/resolv.conf via atomic write
   - Extract option 42 NTP servers into DHCPPayload
   - On expiry: remove route, clear resolv.conf
   - Tests: TestResolvConf*, TestV4PayloadNTPServers
   - Verify: tests pass

5. **Phase: OnConfigure integration** -- start/stop DHCP from config
   - In register.go OnConfigure: iterate units, create DHCPClient for enabled
   - In OnConfigApply: diff previous config, start/stop as needed
   - Tests: functional tests (parse)
   - Verify: `make ze-verify`

6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- audit, learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | parseUnits reads dhcp/dhcpv6 JSON keys matching YANG leaf names |
| Naming | JSON keys match YANG: `client-id`, `hostname`, `enabled` |
| Data flow | Config -> parseUnits -> dhcpUnitConfig -> OnConfigure -> DHCPClient |
| Rule: no-layering | No second DHCP config path, only the parsed one |
| Rule: sibling-audit | AddRoute added to all backends (netlink + any mock) |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| dhcpUnitConfig struct exists | `grep dhcpUnitConfig internal/component/iface/config.go` |
| DHCP parsing in parseUnits | `grep "dhcp" internal/component/iface/config.go` |
| Backend.AddRoute method | `grep AddRoute internal/component/iface/backend.go` |
| resolv.conf writer | `ls internal/plugins/ifacedhcp/resolv_linux.go` |
| DHCPPayload.NTPServers field | `grep NTPServers internal/component/iface/iface.go` |
| Functional test passes | `test/parse/dhcp-config-enabled.ci` |
| Static IP not regressed | `test/parse/dhcp-config-disabled.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Hostname validated (RFC 1123), client-id length bounded |
| File write safety | resolv.conf: write to tmp file then rename (atomic) |
| Path traversal | resolv.conf path is a constant, not configurable |
| DNS injection | Nameserver IPs validated as IP addresses, not arbitrary strings |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read config.go current parsing |
| Lint failure | Fix inline |
| Functional test fails | Check AC |
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

- ifacedhcp registers with `Dependencies: []string{"interface"}` -- it starts after
  the interface plugin. The interface plugin should create DHCPClient instances in
  OnConfigure and pass them the event bus. The ifacedhcp plugin's own RunEngine is
  currently a no-op SDK shell; the real work happens in DHCPClient goroutines.
- The DHCPClient constructor needs to be extended but the lifecycle (Start/Stop)
  stays the same. The OnConfigure handler holds references to active clients and
  stops them on reconfigure.
- resolv.conf must be written atomically to avoid partial reads by other processes.
  Write to `/tmp/resolv.conf.tmp`, then rename to `/tmp/resolv.conf`.

## RFC Documentation

- RFC 2131 Section 2: DHCPv4 client operation
- RFC 2132 Section 3.14: option 12 hostname
- RFC 2132 Section 3.8: option 6 DNS servers
- RFC 2132 Section 8.3: option 42 NTP servers
- RFC 2132 Section 9.14: option 61 client identifier

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
- [ ] Write learned summary to `plan/learned/NNN-gokrazy-1-dhcp-wiring.md`
- [ ] Summary included in commit
