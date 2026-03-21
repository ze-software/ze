# Spec: rpki-5-wiring

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-rpki-0-umbrella |
| Phase | - |
| Updated | 2026-03-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/rpki/rpki.go` - plugin entry point (needs OnConfigure + WantsConfig)
4. `internal/component/bgp/plugins/gr/gr.go` - model for OnConfigure callback
5. `test/plugin/adj-rib-in-query.ci` - model for Python dispatch-command testing

## Task

Wire the bgp-rpki plugin into the config pipeline and prove it works end-to-end with exhaustive functional tests. Currently the plugin is library code: unit tests pass but no user can reach the feature through config/CLI. This spec closes that gap.

**Parent spec:** `spec-rpki-0-umbrella.md`

**What's broken today:**
- No `OnConfigure` callback: RPKI config (`rpki { cache-server { ... } }`) is never parsed
- No `WantsConfig` in SDK Registration: engine never delivers config to the plugin
- No RTR sessions started: `rp.sessions` always empty
- No functional `.ci` test: feature not proven reachable from user entry point
- Validation gate in adj-rib-in has no `.ci` test either

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - plugin config delivery, 5-stage protocol
  -> Constraint: Config delivered in Stage 2 via OnConfigure callback
  -> Decision: WantsConfig declares which config roots the plugin needs
- [ ] `docs/functional-tests.md` - .ci test format, process orchestration
  -> Constraint: cmd=background starts processes, cmd=foreground runs ze
  -> Decision: Python plugins can call dispatch-command for inter-plugin testing

### RFC Summaries
- [ ] `rfc/short/rfc8210.md` - RTR protocol (session lifecycle for mock server)
  -> Constraint: Reset Query -> Cache Response -> Prefix PDUs -> End of Data
- [ ] `rfc/short/rfc6811.md` - validation states (Valid, Invalid, NotFound)
  -> Constraint: Three states determine accept/reject behavior

**Key insights:**
- Config flows: YANG schema -> config parser -> Stage 2 OnConfigure -> plugin parses JSON sections
- Functional tests use Python plugins calling `dispatch-command` to interact with other plugins
- The `ze-peer` test helper validates BGP wire bytes and supports `send-default-route` option
- RPKI testing needs a mock RTR server (TCP, serves fixed VRPs via RTR protocol)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - plugin entry point, missing OnConfigure
- [ ] `internal/component/bgp/plugins/rpki/rtr_session.go` - RTR session, never started
- [ ] `internal/component/bgp/plugins/rpki/roa_cache.go` - ROA cache, never populated from config
- [ ] `internal/component/bgp/plugins/rpki/register.go` - registration has ConfigRoots but no WantsConfig in Run()
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - validation gate exists, tested in unit tests only
- [ ] `test/plugin/adj-rib-in-query.ci` - model for Python plugin dispatch-command testing

**Behavior to preserve:**
- All existing unit tests (63 tests across adj-rib-in and rpki packages)
- Plugin registration (21 plugins including bgp-rpki)
- Validation gate mechanics (pending/accept/reject/timeout)
- ROA cache VRP storage and lookup
- RTR PDU parsing

**Behavior to change:**
- RPKIPlugin.RunRPKIPlugin: add OnConfigure callback, add WantsConfig to Registration
- RPKIPlugin: parse config, start RTR sessions from cache-server list
- Add mock RTR server test infrastructure for functional tests

## Data Flow (MANDATORY)

### Entry Point
- Config file containing `rpki { cache-server 127.0.0.1 { port 3323 } }` under `bgp { }`
- RTR TCP connection to mock cache server providing VRPs

### Transformation Path
1. Config parsed by YANG-aware parser into JSON tree
2. Engine delivers config to bgp-rpki via Stage 2 OnConfigure
3. Plugin parses JSON, extracts cache-server list
4. Plugin starts RTR session goroutines per cache-server
5. RTR sessions connect, receive VRPs, populate ROACache
6. BGP UPDATE events arrive, plugin validates each prefix against cache
7. Plugin issues accept-routes/reject-routes to adj-rib-in

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config parser -> plugin | JSON via OnConfigure callback | [ ] |
| Plugin -> RTR server | TCP, RFC 8210 PDU format | [ ] |
| RTR -> ROACache | VRP structs via ApplyDelta | [ ] |
| Plugin -> adj-rib-in | DispatchCommand text commands | [ ] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `rpki { cache-server { ... } }` | -> | Plugin starts, RTR session connects | `test/plugin/rpki-cache-connect.ci` |
| BGP UPDATE with origin AS matching ROA | -> | Route accepted with state=Valid | `test/plugin/rpki-validate-accept.ci` |
| BGP UPDATE with origin AS not matching ROA | -> | Route rejected (Invalid) | `test/plugin/rpki-validate-reject.ci` |
| BGP UPDATE with no ROA coverage | -> | Route accepted with state=NotFound | `test/plugin/rpki-validate-notfound.ci` |
| Config without rpki plugin loaded | -> | Routes flow through unchanged | `test/plugin/rpki-passthrough.ci` |
| RTR cache sends updated VRPs | -> | Re-validation triggers | `test/plugin/rpki-cache-update.ci` |
| Multiple prefixes in one UPDATE | -> | Each validated independently | `test/plugin/rpki-multi-prefix.ci` |
| AS_PATH with AS_SET as final segment | -> | Origin=NONE, result per ROA coverage | `test/plugin/rpki-as-set.ci` |
| Validation timeout (RPKI plugin slow) | -> | Route promoted after timeout (fail-open) | `test/plugin/rpki-timeout.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with rpki cache-server, mock RTR serves VRPs | RTR session connects, ROA cache populated, validation active |
| AC-2 | UPDATE with AS 65001, ROA covers prefix with AS 65001 | Route accepted, validation-state=Valid (1) |
| AC-3 | UPDATE with AS 65001, ROA covers prefix with AS 65002 | Route rejected (not installed) |
| AC-4 | UPDATE with AS 65001, no ROA covers prefix | Route accepted, validation-state=NotFound (2) |
| AC-5 | Config without rpki plugin | Routes stored immediately, no pending state |
| AC-6 | RTR cache sends new VRP mid-session | Affected routes re-validated on next UPDATE |
| AC-7 | UPDATE with 3 prefixes, mixed ROA coverage | Each prefix gets independent accept/reject |
| AC-8 | AS_PATH ends with AS_SET | Origin=NONE, covered prefixes -> Invalid |
| AC-9 | Validation timeout (30s, shortened for test) | Pending route promoted with state=NotValidated |
| AC-10 | ROA with maxLength=24, route is /25 | Route rejected (exceeds maxLength) |

## Test Plan

### Mock RTR Server

A lightweight Go test tool (`cmd/ze-rtr-mock/`) that:
- Listens on TCP port
- Accepts RTR Reset Query
- Sends Cache Response + configured VRP set + End of Data
- Supports incremental updates via Serial Query
- Configurable VRP set via command-line flags

| Flag | Purpose |
|------|---------|
| `--port` | TCP listen port |
| `--vrp` | VRP entry: `prefix,maxlen,asn` (repeatable) |
| `--serial` | Initial serial number |

### Functional Tests (.ci files)

Each test follows this pattern:
1. Start mock RTR server with specific VRPs (`cmd=background`)
2. Start ze-peer as BGP test peer (`cmd=background`)
3. Start ze with RPKI config pointing to mock RTR (`cmd=foreground`)
4. Python test plugin sends UPDATE via dispatch-command
5. Validate: ze-peer receives or does not receive the route

| Test File | VRPs | UPDATE | Expected |
|-----------|------|--------|----------|
| `rpki-validate-accept.ci` | 10.0.0.0/8,24,65001 | AS 65001, 10.0.1.0/24 | Route forwarded to peer (Valid) |
| `rpki-validate-reject.ci` | 10.0.0.0/8,24,65001 | AS 65999, 10.0.1.0/24 | Route NOT forwarded (Invalid) |
| `rpki-validate-notfound.ci` | 10.0.0.0/8,24,65001 | AS 65001, 192.168.0.0/24 | Route forwarded (NotFound) |
| `rpki-passthrough.ci` | (no rpki plugin) | AS 65001, 10.0.1.0/24 | Route forwarded immediately |
| `rpki-multi-prefix.ci` | 10.0.0.0/8,24,65001 | 3 prefixes: valid, invalid, notfound | 2 forwarded, 1 blocked |
| `rpki-as-set.ci` | 10.0.0.0/8,24,65001 | AS_SET {65001}, 10.0.1.0/24 | Route NOT forwarded (AS_SET -> Invalid) |
| `rpki-maxlength.ci` | 10.0.0.0/8,24,65001 | AS 65001, 10.0.1.0/25 | Route NOT forwarded (/25 > maxLen /24) |
| `rpki-timeout.ci` | (no RTR server) | AS 65001, 10.0.1.0/24 | Route forwarded after timeout (fail-open) |
| `rpki-cache-connect.ci` | 10.0.0.0/8,24,65001 | (status query only) | `rpki status` shows VRP count > 0 |

### Unit Test Additions

| Test | File | Validates |
|------|------|-----------|
| `TestParseRPKIConfig` | `rpki/rpki_test.go` | Config JSON parsed into cache-server list |
| `TestHandlePDUAllTypes` | `rpki/rtr_session_test.go` | All 8 PDU type handlers tested |
| `TestValidationWorker` | `rpki/rpki_test.go` | Channel-based async worker processes requests |

## Files to Create

- `cmd/ze-rtr-mock/main.go` - mock RTR cache server for testing
- `test/plugin/rpki-validate-accept.ci` - Valid route forwarded
- `test/plugin/rpki-validate-reject.ci` - Invalid route blocked
- `test/plugin/rpki-validate-notfound.ci` - NotFound route forwarded
- `test/plugin/rpki-passthrough.ci` - No RPKI plugin, routes flow through
- `test/plugin/rpki-multi-prefix.ci` - Mixed validation per prefix
- `test/plugin/rpki-as-set.ci` - AS_SET origin yields Invalid
- `test/plugin/rpki-maxlength.ci` - maxLength exceeded
- `test/plugin/rpki-timeout.ci` - Fail-open timeout
- `test/plugin/rpki-cache-connect.ci` - RTR connection + VRP count
- `internal/component/bgp/plugins/rpki/rpki_config.go` - Config parsing (OnConfigure)

## Files to Modify

- `internal/component/bgp/plugins/rpki/rpki.go` - Add OnConfigure, WantsConfig, session startup
- `internal/component/bgp/plugins/rpki/rtr_session.go` - May need adjustments for test scenarios

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] Already done | `schema/ze-rpki.yang` |
| Config parsing | [x] Yes - OnConfigure | `rpki.go` + `rpki_config.go` |
| RTR session startup | [x] Yes - from config | `rpki.go` |
| Mock RTR server | [x] Yes - test tool | `cmd/ze-rtr-mock/main.go` |
| Functional tests | [x] Yes - 9 .ci tests | `test/plugin/rpki-*.ci` |

## Implementation Steps

1. **Build mock RTR server** (`cmd/ze-rtr-mock/`) - TCP server, serve VRPs
2. **Add OnConfigure + WantsConfig** to rpki plugin - parse config, start sessions
3. **Write rpki-cache-connect.ci** - prove RTR connection works from config
4. **Write rpki-validate-accept.ci** - prove Valid route forwarded
5. **Write rpki-validate-reject.ci** - prove Invalid route blocked
6. **Write rpki-validate-notfound.ci** - prove NotFound route forwarded
7. **Write rpki-passthrough.ci** - prove no overhead without rpki
8. **Write remaining .ci tests** - multi-prefix, as-set, maxlength, timeout
9. **Run `make ze-verify`** - all tests pass
10. **Deep review** - fix any findings

### Failure Routing

| Failure | Route To |
|---------|----------|
| Mock RTR server won't build | Step 1 (fix Go build) |
| OnConfigure not called | Check WantsConfig in Registration |
| RTR session won't connect | Check mock server port, TCP dial |
| Route not forwarded | Check validation state, DispatchCommand |
| Route forwarded when should be blocked | Check ROA cache population, Validate() |
| Timeout test flaky | Increase timeout, check sweep interval |

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
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]
