# Spec: set-with

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `internal/component/bgp/plugins/cmd/peer/peer.go` -- current HandleBgpPeerAdd handler
3. `internal/component/bgp/schema/ze-bgp-conf.yang` -- peer config YANG schema
4. `internal/component/plugin/types.go` -- DynamicPeerConfig struct

## Task

Rework `set bgp peer <ip> with <config>` to accept config-syntax inline args
matching the YANG peer config structure. Currently the handler accepts a flat
format (`asn 65001 local-as 65000`). It should accept the same key space as
the config file, using config-syntax paths.

**Design principle:** a user should be able to create a fully working peer
from the command line, with the same keys available in the config file.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go:287-425` -- HandleBgpPeerAdd
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang:82-155` -- peer-fields grouping
- [ ] `internal/component/plugin/types.go:72-80` -- DynamicPeerConfig struct
- [ ] `test/plugin/api-peer-add.ci` -- functional test

**Current handler accepts (flat key-value):**

| Key | Value | Required | Maps to |
|-----|-------|----------|---------|
| `asn` | uint32 | Yes | DynamicPeerConfig.PeerAS |
| `local-as` | uint32 | No | DynamicPeerConfig.LocalAS |
| `local-ip` | IP | No | DynamicPeerConfig.LocalAddress |
| `router-id` | IPv4 | No | DynamicPeerConfig.RouterID |
| `hold-time` | uint16 | No | DynamicPeerConfig.HoldTime |
| `connection` | both/passive/active | No | DynamicPeerConfig.Connection |

**Current syntax:** `set bgp peer 10.0.0.1 with asn 65001 local-as 65000`

**Behavior to preserve:**
- All validation (ASN range, hold-time range, connection enum)
- Error responses with specific messages
- Wildcard/glob selector rejection (must be specific IP)
- DynamicPeerConfig passed to reactor.AddDynamicPeer

**Behavior to change:**
- Parse config-syntax inline args instead of flat key-value pairs

## Design

### New Syntax

Config-syntax inline, matching the YANG structure:

| Config file syntax | CLI inline syntax |
|-------------------|-------------------|
| `remote { as 65001; }` | `remote as 65001` |
| `local { as 65000; ip 192.168.1.1; }` | `local as 65000 local ip 192.168.1.1` |
| `router-id 1.2.3.4;` | `router-id 1.2.3.4` |
| `hold-time 90;` | `hold-time 90` |
| `connection passive;` | `connection passive` |
| `description "my peer";` | `description "my peer"` |
| `link-local fe80::1;` | `link-local fe80::1` |
| `port 1179;` | `port 1179` |
| `group-updates disable;` | `group-updates disable` |

**Full example:**
`set bgp peer 10.0.0.1 with remote as 65001 local as 65000 local ip 192.168.1.1 router-id 1.2.3.4 hold-time 90 connection passive`

### Parsing Rules

The args after `with` are parsed as a sequence of config-path tokens:

| Token | Next token(s) | Action |
|-------|--------------|--------|
| `remote` | `as <N>` | Set PeerAS |
| `remote` | `ip <IP>` | Set remote IP (for named peers) |
| `local` | `as <N>` | Set LocalAS |
| `local` | `ip <IP>` | Set LocalAddress |
| `router-id` | `<IPv4>` | Set RouterID |
| `hold-time` | `<N>` | Set HoldTime |
| `connection` | `both/passive/active` | Set Connection |
| `description` | `"<text>"` | Set Description |
| `link-local` | `<IPv6>` | Set LinkLocal |
| `port` | `<N>` | Set Port |
| `group-updates` | `enable/disable` | Set GroupUpdates |

`remote` and `local` are container prefixes -- the next token determines which leaf
within the container is being set. Multiple leaves under the same container can appear
in sequence: `local as 65000 local ip 192.168.1.1`.

### DynamicPeerConfig Changes

| Field | Current | New | Notes |
|-------|---------|-----|-------|
| Address | netip.Addr | (unchanged) | From peer selector |
| PeerAS | uint32 | (unchanged) | `remote as` |
| LocalAS | uint32 | (unchanged) | `local as` |
| LocalAddress | netip.Addr | (unchanged) | `local ip` |
| RouterID | uint32 | (unchanged) | `router-id` |
| HoldTime | time.Duration | (unchanged) | `hold-time` |
| Connection | string | (unchanged) | `connection` |
| Description | - | string (new) | `description` |
| LinkLocal | - | netip.Addr (new) | `link-local` |
| Port | - | uint16 (new) | `port` |
| GroupUpdates | - | *bool (new) | `group-updates` (nil = default) |

### ~~Backward Compatibility~~ (SUPERSEDED: Ze has no users, no backward compatibility needed per rules/compatibility.md)

~~The old flat keys (`asn`, `local-as`, `local-ip`) should still work as aliases
for the config-syntax equivalents. This avoids breaking existing scripts.~~

Old flat keys removed. Only config-syntax keys accepted.

## Data Flow (MANDATORY)

### Entry Point
1. User types `set bgp peer 10.0.0.1 with remote as 65001 local as 65000`
2. Dispatcher extracts selector `10.0.0.1`, rebuilds to `set bgp peer with remote as 65001 local as 65000`
3. Matches registered `set bgp peer with` (wire method `ze-set:bgp-peer-with`)
4. Handler receives args `["remote", "as", "65001", "local", "as", "65000"]`
5. Parser walks args: `remote` -> `as` -> 65001, `local` -> `as` -> 65000
6. Builds DynamicPeerConfig, calls AddDynamicPeer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> YANG dispatch | `with` container in ze-cli-set-cmd.yang | [ ] |
| YANG -> RPC handler | ze:command maps to ze-set:bgp-peer-with | [ ] |
| Handler -> reactor | DynamicPeerConfig passed to AddDynamicPeer | [ ] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `set bgp peer 10.0.0.1 with remote as 65001` | Peer created with PeerAS=65001 |
| AC-2 | `set bgp peer 10.0.0.1 with remote as 65001 local as 65000 local ip 192.168.1.1` | Peer created with all local overrides |
| AC-3 | `set bgp peer 10.0.0.1 with remote as 65001 router-id 1.2.3.4 hold-time 180 connection passive` | All optional fields set |
| AC-4 | `set bgp peer 10.0.0.1 with remote as 65001 description "my peer"` | Description preserved |
| ~~AC-5~~ | ~~`set bgp peer 10.0.0.1 with asn 65001` (old alias)~~ | ~~Still works, same as `remote as 65001`~~ (SUPERSEDED: no aliases, old keys rejected as unknown) |
| AC-6 | `set bgp peer 10.0.0.1 with` (no config) | Error: missing required remote as |
| AC-7 | `set bgp peer 10.0.0.1 with remote as 65001 bogus-key value` | Error: unknown config key |
| AC-8 | `set bgp peer 10.0.0.1 with remote as 99999999999` | Error: ASN out of range |
| AC-9 | `set bgp peer * with remote as 65001` | Error: wildcard not allowed for peer creation |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestSetPeerWithRemoteAS | peer_ops_test.go | AC-1: minimal config-syntax peer creation | [ ] |
| TestSetPeerWithFullConfig | peer_ops_test.go | AC-3: all optional fields via config syntax | [ ] |
| TestSetPeerWithLocalOverrides | peer_ops_test.go | AC-2: local as + local ip | [ ] |
| TestSetPeerWithDescription | peer_ops_test.go | AC-4: description field | [ ] |
| TestSetPeerWithOldAliases | peer_ops_test.go | AC-5: asn/local-as/local-ip aliases | [ ] |
| TestSetPeerWithMissingRemoteAS | peer_ops_test.go | AC-6: error on no config | [ ] |
| TestSetPeerWithUnknownKey | peer_ops_test.go | AC-7: error on bogus key | [ ] |
| TestSetPeerWithASNOutOfRange | peer_ops_test.go | AC-8: validation | [ ] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| api-peer-add | test/plugin/api-peer-add.ci | Dispatch via config-syntax | [ ] |

## Files to Modify

- `internal/component/bgp/plugins/cmd/peer/peer.go` -- rework HandleBgpPeerAdd parser
- `internal/component/plugin/types.go` -- add Description, LinkLocal, Port, GroupUpdates to DynamicPeerConfig
- `internal/component/bgp/plugins/cmd/peer/peer_ops_test.go` -- update/add tests
- `test/plugin/api-peer-add.ci` -- update dispatch to config-syntax
- `test/plugin/api-peer-remove.ci` -- update dispatch to config-syntax

## Files to Create

(none -- all changes are to existing files)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `set bgp peer 10.0.0.1 with remote as 65001` via dispatch | -> | HandleBgpPeerAdd parser + AddDynamicPeer | test/plugin/api-peer-add.ci |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No (rework, not new) | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- update set peer syntax |
| 4 | API/RPC added/changed? | No (same wire method) | |
| 5-12 | Other | No | |

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Config-syntax parsing | All YANG peer-fields leaves have a corresponding parser case |
| Old aliases work | `asn`, `local-as`, `local-ip` still accepted |
| Validation preserved | ASN range, hold-time range, connection enum, IP format |
| Error messages clear | Each unknown/invalid key produces a specific error |
| DynamicPeerConfig complete | New fields propagated through AddDynamicPeer to peer creation |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Config-syntax parser works | Unit tests for each key |
| Old aliases preserved | TestSetPeerWithOldAliases |
| Full peer creation | .ci test |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | All numeric fields range-checked, IPs parsed safely |
| No injection | Description string safely stored (no shell/path expansion) |

## Implementation Phases

| Phase | What |
|-------|------|
| 1 | Add new fields to DynamicPeerConfig |
| 2 | Rework parser in HandleBgpPeerAdd for config-syntax with old aliases |
| 3 | Update tests (unit + .ci) |
| 4 | Update docs |

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

## Implementation Summary

### What Was Implemented

### Deviations from Plan

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
