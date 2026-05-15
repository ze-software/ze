# Spec: cpe-1-pppoe-client

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/7 |
| Updated | 2026-05-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/pppoe/schema/ze-pppoe-conf.yang` - existing PPPoE AC (server) schema
4. `internal/component/iface/schema/ze-iface-conf.yang` - interface config model
5. `internal/component/iface/backend.go` - Backend interface (CreateTunnel, CreateWireguardDevice pattern)

## Task

Add PPPoE **client** support to Ze as a new interface kind. Ze currently has a PPPoE access concentrator (server) in `internal/component/pppoe/` for BNG use cases. The CPE/home-router use case requires Ze to act as a PPPoE client: dial out over a physical Ethernet interface, authenticate via PAP or CHAP, negotiate LCP/IPCP/IPv6CP, and present the resulting PPP session as a routable interface with server-assigned addresses.

Modeled as an interface kind (like tunnel, wireguard, bridge) because the result is a network interface with addresses, MTU, and routing participation.

**Motivation:** VyOS home.conf uses `interfaces pppoe pppoe0 { authentication { ... }; source-interface eth2; mtu 1492; no-default-route }`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component isolation, registration pattern
  → Constraint: PPPoE client must not collide with PPPoE AC (server) namespace in `internal/component/pppoe/`
- [ ] `internal/component/iface/backend.go` - Backend interface with Create* methods and kind dispatch
  → Decision: Add CreatePPPoEClient method to Backend interface following CreateWireguardDevice pattern
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` - interface choice with cases: ethernet, dummy, veth, bridge, tunnel, wireguard, loopback
  → Decision: Add `pppoe-client` case to the interface choice
- [ ] `internal/component/pppoe/schema/ze-pppoe-conf.yang` - PPPoE AC server config (avoid duplication)
  → Constraint: Client and AC are independent; client lives in iface component, AC stays in pppoe component

### RFC Summaries (MUST for protocol work)
- [ ] RFC 2516 - PPPoE discovery (PADI/PADO/PADR/PADS) and session framing
  → Constraint: Must implement full discovery state machine
- [ ] RFC 1661 - PPP LCP negotiation (MRU, authentication protocol, magic number)
  → Constraint: Must negotiate MRU and auth method before network phase
- [ ] RFC 1332 - PPP IPCP (IPv4 address assignment from server)
  → Constraint: Client requests 0.0.0.0 to get server-assigned address
- [ ] RFC 5072 - PPP IPv6CP (interface-ID negotiation)
  → Constraint: Optional; only if server supports IPv6

**Key insights:**
- PPPoE client creates a virtual ppp interface bound to a physical source interface
- Authentication credentials come from config, marked ze:sensitive
- Addresses assigned by server via IPCP/IPv6CP, not configured statically
- MTU constrained by PPPoE overhead (max 1492 for standard 1500 MTU Ethernet)
- Optional default route via the PPP interface

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` - Backend interface: CreateDummy, CreateVeth, CreateBridge, CreateTunnel, CreateWireguardDevice, ConfigureWireguardDevice
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` - interface kinds defined as YANG choice cases
- [ ] `internal/component/pppoe/schema/ze-pppoe-conf.yang` - PPPoE AC (server) schema with ac-name, service-name, cookie-timeout, per-interface config
- [ ] `internal/component/ppp/ppp.go` - PPP protocol framing (shared with AC if exists)

**Behavior to preserve:**
- Existing PPPoE AC (server) component in `internal/component/pppoe/` unchanged
- Backend interface dispatch pattern (kind -> Create* method)
- YANG schema structure for interface kinds (choice with cases)
- Existing interface lifecycle (discover, create, configure, monitor)

**Behavior to change:**
- Add `pppoe-client` case to interface kind choice in ze-iface-conf.yang
- Extend Backend interface with PPPoE client creation method
- New PPPoE discovery + PPP negotiation state machine for client role

## Data Flow (MANDATORY)

### Entry Point
- Config commit containing `interface { pppoe-client pppoe0 { source-interface eth2; authentication { ... } } }`

### Transformation Path
1. Config parsed via YANG schema, validated (source-interface must exist, credentials present)
2. Interface dispatcher recognizes `pppoe-client` kind, calls Backend.CreatePPPoEClient
3. Backend opens raw Ethernet socket on source-interface for PPPoE discovery
4. PPPoE discovery state machine: PADI -> PADO -> PADR -> PADS (session ID assigned)
5. LCP negotiation: MRU, auth protocol selection, magic number exchange
6. Authentication: PAP or CHAP-MD5 using configured credentials
7. IPCP/IPv6CP negotiation: address assignment from peer (server)
8. Kernel ppp interface created, assigned addresses applied via Backend.AddAddress
9. Default route installed (unless no-default-route flag set)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> iface component | config tree extraction, kind dispatch | [ ] |
| iface -> kernel | netlink for ppp device, raw AF_PACKET socket for PPPoE | [ ] |
| PPPoE discovery -> PPP session | Ethernet encapsulation per RFC 2516 | [ ] |

### Integration Points
- `internal/component/iface/backend.go` - Backend interface extension for PPPoE client create/teardown
- `internal/plugins/connected/` - ppp interface addresses redistributed via connected plugin
- `internal/plugins/static/` - default route integration when no-default-route is not set

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (PPPoE client isolated from PPPoE AC server)
- [ ] No duplicated functionality (reuse PPP framing from `internal/component/ppp/` if available)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config commit with pppoe-client block | → | iface kind dispatch + Backend.CreatePPPoEClient | `test/pppoe/client.ci` |
| PPPoE discovery on source-interface | → | PADI/PADO/PADR/PADS state machine | `TestPPPoEDiscoveryStateMachine` |
| LCP + authentication | → | MRU negotiation + PAP/CHAP exchange | `TestLCPNegotiation` |
| IPCP address assignment | → | kernel interface address apply | `TestIPCPAddressAssignment` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with pppoe-client, source-interface, and auth credentials | Config parses and validates; source-interface must be an existing Ethernet interface |
| AC-2 | PPPoE discovery on source-interface | PADI sent, PADO received, PADR sent, PADS received; session ID assigned |
| AC-3 | LCP negotiation phase | MRU negotiated, authentication method agreed (PAP or CHAP) |
| AC-4 | PAP or CHAP-MD5 authentication | Credentials sent per agreed method, auth-ack received from server |
| AC-5 | IPCP negotiation | IPv4 address assigned by server, applied to ppp interface in kernel |
| AC-6 | IPv6CP negotiation (optional) | Interface-ID negotiated when server advertises IPv6 support |
| AC-7 | MTU configuration | PPPoE interface created with configured MTU (default 1492) |
| AC-8 | `no-default-route` presence flag | When set: no default route installed; when absent: default route via ppp interface |
| AC-9 | Session teardown (config delete or interface disable) | PADT sent to server, kernel ppp interface removed cleanly |
| AC-10 | Session loss (PADT from server, keepalive timeout) | Automatic reconnection with exponential backoff |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPPPoEClientConfigParse` | `internal/component/iface/pppoe_client_test.go` | Config extraction: source-interface, auth, MTU, no-default-route | |
| `TestPPPoEDiscoveryStateMachine` | `internal/component/iface/pppoe_client_test.go` | PADI/PADO/PADR/PADS state transitions | |
| `TestLCPNegotiation` | `internal/component/iface/pppoe_client_test.go` | MRU and auth protocol negotiation | |
| `TestCHAPAuthentication` | `internal/component/iface/pppoe_client_test.go` | CHAP-MD5 challenge/response | |
| `TestPAPAuthentication` | `internal/component/iface/pppoe_client_test.go` | PAP authenticate-request/ack | |
| `TestIPCPAddressAssignment` | `internal/component/iface/pppoe_client_test.go` | IPv4 address negotiation from 0.0.0.0 | |
| `TestPPPoEReconnectBackoff` | `internal/component/iface/pppoe_client_test.go` | Exponential backoff on session loss | |
| `TestPPPoEConfigValidation` | `internal/component/iface/pppoe_client_test.go` | Rejects missing source-interface, missing credentials | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MTU | 68-1492 | 1492 | 67 | 1493 |
| speed (source Ethernet MTU) | 68-16000 | 16000 | 67 | 16001 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-pppoe-client` | `test/pppoe/client.ci` | PPPoE client dials, authenticates, gets address, routes traffic | |

## Files to Modify
- `internal/component/iface/schema/ze-iface-conf.yang` - add pppoe-client case to interface kind choice
- `internal/component/iface/backend.go` - extend Backend interface with CreatePPPoEClient method

## Files to Create
- `internal/component/iface/pppoe_client.go` - PPPoE discovery + PPP negotiation state machine
- `internal/component/iface/pppoe_client_linux.go` - Linux raw socket + kernel ppp device creation
- `internal/component/iface/pppoe_client_test.go` - unit tests
- `test/pppoe/client.ci` - functional test with veth pair PPPoE server mock

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist |
| 7-9. Fix cycle | Fix issues, re-verify, max 2 passes |
| 10. Deliverables | Deliverables Checklist |
| 11. Security | Security Review Checklist |

### Implementation Phases

1. **Phase: YANG schema** - Add pppoe-client case to ze-iface-conf.yang with source-interface, authentication, MTU, no-default-route
   - Tests: `TestPPPoEClientConfigParse`, `TestPPPoEConfigValidation`
   - Files: `ze-iface-conf.yang`
   - Verify: config parse tests fail -> implement schema -> tests pass

2. **Phase: PPPoE discovery** - PADI/PADO/PADR/PADS state machine on raw AF_PACKET socket
   - Tests: `TestPPPoEDiscoveryStateMachine`
   - Files: `pppoe_client.go`, `pppoe_client_linux.go`
   - Verify: discovery test fails -> implement -> passes

3. **Phase: LCP/Auth** - LCP negotiation (MRU, auth protocol) + PAP/CHAP authentication
   - Tests: `TestLCPNegotiation`, `TestCHAPAuthentication`, `TestPAPAuthentication`
   - Files: `pppoe_client.go`
   - Verify: negotiation tests fail -> implement -> pass

4. **Phase: IPCP/IPv6CP** - Address negotiation and kernel interface setup
   - Tests: `TestIPCPAddressAssignment`
   - Files: `pppoe_client.go`, `pppoe_client_linux.go`
   - Verify: tests fail -> implement -> pass

5. **Phase: Lifecycle** - Reconnection on session loss, PADT handling, config delete cleanup
   - Tests: `TestPPPoEReconnectBackoff`
   - Files: `pppoe_client.go`

6. **Phase: Backend wiring** - Extend Backend interface, wire into iface dispatcher
   - Files: `backend.go`

7. **Phase: Functional tests** - End-to-end with veth pair and PPPoE server mock
   - Tests: `test/pppoe/client.ci`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-1 through AC-10 have implementation with file:line |
| Correctness | PPPoE discovery state machine matches RFC 2516 Section 5 exactly |
| Naming | `pppoe-client` interface kind does not collide with `pppoe` AC component |
| Data flow | Discovery on source-interface raw socket, session on kernel ppp device |
| Rule: no-layering | No duplication with existing PPPoE AC code paths |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG schema with pppoe-client case | `grep pppoe-client internal/component/iface/schema/ze-iface-conf.yang` |
| Backend interface extension | `grep CreatePPPoEClient internal/component/iface/backend.go` |
| PPPoE client state machine | `ls internal/component/iface/pppoe_client.go` |
| Linux backend | `ls internal/component/iface/pppoe_client_linux.go` |
| Unit tests pass | `go test ./internal/component/iface/ -run PPPoE` |
| Functional test | `ls test/pppoe/client.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Source-interface must exist and be Ethernet type; credentials required |
| Credential handling | Password marked ze:sensitive in YANG, $9$ encoded in config file |
| Session security | AC-Cookie validation per RFC 2516 Section 5.5 |
| Resource limits | Max reconnection backoff capped; discovery timeout prevents infinite loop |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 2516/1661 and source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: PADI/PADO/PADR/PADS state transitions (RFC 2516 Section 5), LCP negotiation (RFC 1661 Section 5), IPCP option 3 (RFC 1332 Section 3.3), CHAP challenge format (RFC 1994 Section 4).

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/component/iface/*`)
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added (RFC 2516, 1661, 1332, 5072)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling
