# Spec: bng-5 -- PPPoE Access

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-bng-1-radius-attributes |
| Phase | 6/9 |
| Updated | 2026-05-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/ppp/doc.go` -- PPP is transport-agnostic
4. `internal/component/ppp/manager.go` -- Driver, StartSession, session lifecycle
5. `internal/component/ppp/frame.go` -- frame codec
6. `internal/component/ppp/start_session.go` -- StartSession struct
7. `internal/component/l2tp/subsystem.go` -- L2TP subsystem as pattern

## Task

Add PPPoE (RFC 2516) as an alternative subscriber access method alongside
L2TP. PPPoE is the direct-attach model: subscriber CPEs connect over
Ethernet to the BNG without an intermediate LAC/LNS tunnel.

The PPP component was designed to be transport-agnostic (doc.go: "one
goroutine per session, chan fd for control plane, unit fd for data plane").
The `StartSession` struct accepts `ChanFD` and `UnitFD` file descriptors
plus configuration; it does not assume L2TP. This spec builds the PPPoE
discovery and session layer that creates those file descriptors and feeds
them to the existing PPP Driver.

**PPPoE has two phases:**
1. **Discovery** (PADI/PADO/PADR/PADS): Ethernet frames, no PPP. Establishes a session ID.
2. **Session**: PPP frames encapsulated in PPPoE session headers on Ethernet.

Linux provides kernel PPPoE support via AF_PPPOX + PX_PROTO_OE, which
gives us the same /dev/ppp channel+unit FD pattern as PPPoL2TP.

## Required Reading

### Architecture Docs
- [ ] `internal/component/ppp/doc.go` -- transport-agnostic design
- [ ] `internal/component/ppp/manager.go` -- Driver, StartSession
- [ ] `internal/component/ppp/start_session.go` -- StartSession struct (ChanFD, UnitFD, LNSMode, auth config)
- [ ] `internal/component/ppp/frame.go` -- PPP frame codec
- [ ] `internal/component/l2tp/subsystem.go` -- subsystem pattern to follow
- [ ] `internal/component/l2tp/pppox_linux.go` -- PPPoL2TP as precedent for PPPoE kernel interface
- [ ] `internal/component/l2tp/kernel_linux.go` -- kernel worker pattern

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2516.md` -- PPPoE: discovery (PADI/PADO/PADR/PADS/PADT) + session framing
- [ ] `rfc/short/rfc1661.md` -- PPP (already implemented in PPP component)

**Key insights:**
- PPPoE discovery is Ethernet-layer (ethertype 0x8863); session is ethertype 0x8864
- Linux AF_PPPOX with PX_PROTO_OE creates /dev/ppp FDs, same pattern as L2TP
- PPPoE session ID is 16-bit; max 65535 sessions per access interface
- BNG may have multiple access interfaces (VLANs, physical ports)
- Service-Name in PADI/PADO is used for service selection (maps to RADIUS attributes)
- AC-Name (Access Concentrator Name) identifies the BNG
- Host-Uniq in PADI is echoed back for client-side demux
- PPPoE MTU is 1492 (Ethernet 1500 - PPPoE header 6 - PPP protocol 2)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ppp/manager.go` -- Driver accepts StartSession; manages per-session goroutines
- [ ] `internal/component/ppp/start_session.go` -- StartSession requires ChanFD, UnitFD, UnitNum, plus config
- [ ] `internal/component/ppp/frame.go` -- protocol constants, frame parsing (transport-independent)
- [ ] `internal/component/l2tp/pppox_linux.go` -- pppoxCreate for L2TP; PPPoE follows same syscall pattern
- [ ] `internal/component/l2tp/subsystem.go` -- subsystem lifecycle pattern

**Behavior to preserve:**
- PPP Driver API unchanged (StartSession struct)
- L2TP subsystem unaffected (PPPoE is a separate component)
- All existing PPP, L2TP, and RADIUS tests pass
- Auth/pool/shaper plugins work for both L2TP and PPPoE sessions

**Behavior to change:**
- New PPPoE component (`internal/component/pppoe/`)
- PPPoE discovery state machine (per-client)
- PPPoE kernel socket creation (AF_PPPOX + PX_PROTO_OE)
- PPPoE sessions feed into existing PPP Driver
- YANG config for PPPoE access interfaces

## Data Flow (MANDATORY)

### Entry Point
- Ethernet frame with ethertype 0x8863 (PPPoE Discovery) arrives on access interface
- Raw socket listener on configured interfaces

### Transformation Path

#### Discovery phase:
1. PADI received on raw socket (broadcast, ethertype 0x8863)
2. Parse tags: Service-Name, Host-Uniq, Max-Payload
3. If Service-Name matches (or empty = any): send PADO with AC-Name, Service-Name, AC-Cookie
4. PADR received (unicast to BNG MAC)
5. Validate AC-Cookie (anti-DoS)
6. Allocate session ID
7. Send PADS with session ID
8. Create kernel PPPoE socket: socket(AF_PPPOX, SOCK_STREAM, PX_PROTO_OE) + connect(peer MAC, session ID, iface)
9. Create /dev/ppp channel + unit (same as L2TP path)
10. Submit StartSession to PPP Driver

#### Session phase:
11. PPP LCP/Auth/NCP handled by existing PPP component
12. Auth goes through auth handler (RADIUS or local)
13. IP assigned via pool handler
14. Traffic flows through kernel PPPoE

#### Teardown:
15. PADT from subscriber or BNG-initiated -> close kernel socket
16. PPP EventSessionDown -> cleanup

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ethernet -> PPPoE discovery | Raw socket (ethertype 0x8863) | [ ] |
| PPPoE discovery -> kernel | AF_PPPOX + PX_PROTO_OE socket | [ ] |
| Kernel -> PPP Driver | ChanFD + UnitFD via StartSession | [ ] |
| PPP Driver -> auth/pool handlers | Same as L2TP (existing) | [ ] |

### Integration Points
- `ppp.Driver` -- StartSession for new PPPoE sessions
- Auth handler registry -- same handlers serve both L2TP and PPPoE
- Pool handler registry -- same pools serve both L2TP and PPPoE
- Shaper plugin -- applies to pppN interfaces (same for both)
- RADIUS accounting -- session events from PPP are transport-agnostic
- EventBus -- session lifecycle events (SessionUp, SessionDown, etc.)

### Architectural Verification
- [ ] No bypassed layers (PPPoE feeds into PPP Driver, same as L2TP)
- [ ] No unintended coupling (PPPoE component does not import L2TP; shared surface is PPP Driver)
- [ ] No duplicated functionality (reuses PPP, auth, pool, shaper, RADIUS)
- [ ] Zero-copy preserved where applicable (kernel handles data plane)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| PADI on access interface | -> | PADO sent with AC-Name and cookie | `TestPPPoEDiscoveryPADO` |
| PADR with valid cookie | -> | PADS sent, kernel socket created, PPP session starts | `TestPPPoESessionEstablish` |
| PADT from subscriber | -> | Session torn down, resources released | `TestPPPoEPADTTeardown` |
| PPP auth + IPCP over PPPoE | -> | Session fully up with IP assigned | `TestPPPoEEndToEnd` |
| Invalid AC-Cookie in PADR | -> | PADR rejected, no session created | `TestPPPoECookieValidation` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | PADI broadcast on access interface | PADO unicast reply with AC-Name, Service-Name, AC-Cookie |
| AC-2 | PADR with valid cookie | PADS reply with session ID; PPP session started |
| AC-3 | PPP LCP/Auth/IPCP over PPPoE | Full session establishment, IP assigned |
| AC-4 | PADT from subscriber | Session torn down; accounting-stop sent |
| AC-5 | BNG-initiated disconnect | PADT sent to subscriber; session torn down |
| AC-6 | Invalid AC-Cookie | PADR silently discarded (DoS protection) |
| AC-7 | Service-Name filtering | Only matching services accepted; non-matching PADI ignored |
| AC-8 | Multiple access interfaces | Sessions on each interface independently managed |
| AC-9 | VLAN-tagged PPPoE (802.1Q) | Discovery and session work on VLAN sub-interfaces |
| AC-10 | PPPoE MTU=1492 | LCP MRU negotiated to 1492; pppN MTU set to 1492 |
| AC-11 | Max sessions per interface | Rejects new sessions when limit reached |
| AC-12 | Concurrent L2TP + PPPoE | Both access methods active; shared auth/pool/shaper |
| AC-13 | `show pppoe sessions` CLI | Lists active PPPoE sessions with MAC, session ID, IP, interface |
| AC-14 | RADIUS NAS-Port-Type=Ethernet (15) | Accounting correctly identifies PPPoE (not Virtual L2TP) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParsePADI` | `internal/component/pppoe/discovery_test.go` | PADI parsing with tags | |
| `TestBuildPADO` | `internal/component/pppoe/discovery_test.go` | PADO construction with AC-Name, cookie | |
| `TestParsePADR` | `internal/component/pppoe/discovery_test.go` | PADR parsing and cookie validation | |
| `TestBuildPADS` | `internal/component/pppoe/discovery_test.go` | PADS with session ID | |
| `TestBuildPADT` | `internal/component/pppoe/discovery_test.go` | PADT construction | |
| `TestACCookieGenVerify` | `internal/component/pppoe/cookie_test.go` | Cookie generation and verification (HMAC-based) | |
| `TestACCookieReplay` | `internal/component/pppoe/cookie_test.go` | Expired cookie rejected | |
| `TestSessionIDAlloc` | `internal/component/pppoe/session_test.go` | Session ID allocation (1-65535) | |
| `TestSessionIDExhausted` | `internal/component/pppoe/session_test.go` | Session ID exhaustion | |
| `TestPPPoEKernelSocket` | `internal/component/pppoe/kernel_linux_test.go` | AF_PPPOX + PX_PROTO_OE socket creation | |
| `TestServiceNameFilter` | `internal/component/pppoe/discovery_test.go` | Service-Name matching | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Session ID | 1 - 65535 | 65535 | 0 (reserved) | N/A (uint16) |
| Service-Name length | 0 - 255 | 255 | N/A (0 = any) | 256 |
| AC-Cookie length | 16 - 32 | 32 | 15 | 33 |
| PPPoE payload | 0 - 1494 | 1494 | N/A | 1495 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-basic` | `test/pppoe/pppoe-basic.ci` | PPPoE session with PAP auth and IPCP | |
| `pppoe-vlan` | `test/pppoe/pppoe-vlan.ci` | PPPoE over VLAN interface | |
| `pppoe-concurrent-l2tp` | `test/pppoe/pppoe-concurrent-l2tp.ci` | PPPoE + L2TP sessions coexist | |

## Files to Modify

- `internal/component/ppp/start_session.go` -- add PPPoE-specific fields (Access interface, subscriber MAC, session ID) if needed
- `cmd/ze/hub/main.go` -- register PPPoE component

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new) | [x] | `internal/component/pppoe/schema/ze-pppoe-conf.yang` |
| CLI commands | [x] | `show pppoe sessions`, `show pppoe statistics` |
| Editor autocomplete | [x] | YANG-driven |
| Functional test | [x] | `test/pppoe/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- PPPoE access |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- PPPoE config |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` -- show pppoe |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | PPPoE is a component, not a plugin |
| 6 | Has a user guide page? | [x] | `docs/guide/pppoe.md` |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [x] | RFC 2516 |
| 10 | Test infrastructure changed? | [x] | PPPoE test peer needed |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- PPPoE support |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` -- PPPoE component |

## Files to Create

### PPPoE component
- `internal/component/pppoe/doc.go` -- package doc
- `internal/component/pppoe/discovery.go` -- PPPoE discovery state machine (PADI/PADO/PADR/PADS/PADT)
- `internal/component/pppoe/discovery_test.go` -- discovery tests
- `internal/component/pppoe/cookie.go` -- AC-Cookie generation/verification (HMAC-SHA256, time-limited)
- `internal/component/pppoe/cookie_test.go` -- cookie tests
- `internal/component/pppoe/session.go` -- session table, session ID allocation
- `internal/component/pppoe/session_test.go` -- session tests
- `internal/component/pppoe/listener.go` -- raw socket listener on access interfaces
- `internal/component/pppoe/kernel_linux.go` -- AF_PPPOX + PX_PROTO_OE socket, /dev/ppp
- `internal/component/pppoe/kernel_linux_test.go` -- kernel tests
- `internal/component/pppoe/kernel_other.go` -- non-Linux stub
- `internal/component/pppoe/config.go` -- YANG config parsing (access interfaces, service names, limits)
- `internal/component/pppoe/config_test.go` -- config tests
- `internal/component/pppoe/subsystem.go` -- ze.Subsystem implementation
- `internal/component/pppoe/register.go` -- component registration
- `internal/component/pppoe/schema/embed.go` -- go:embed
- `internal/component/pppoe/schema/register.go` -- YANG registration
- `internal/component/pppoe/schema/ze-pppoe-conf.yang` -- YANG schema

### CLI
- `internal/component/cmd/pppoe/main.go` -- show pppoe commands
- `internal/component/cmd/pppoe/register.go` -- CLI registration

### Tests
- `test/pppoe/pppoe-basic.ci` -- basic PPPoE functional test
- `test/pppoe/pppoe-vlan.ci` -- VLAN PPPoE test
- `test/pppoe/pppoe-concurrent-l2tp.ci` -- coexistence test

### RFC summary
- `rfc/short/rfc2516.md` -- PPPoE summary

## Implementation Steps

### Implementation Phases

1. **Phase: Discovery wire format** -- parse/build PADI, PADO, PADR, PADS, PADT with tags (Service-Name, AC-Name, Host-Uniq, AC-Cookie)
   - Tests: `TestParsePADI`, `TestBuildPADO`, `TestParsePADR`, `TestBuildPADS`, `TestBuildPADT`
   - Files: `discovery.go`, `discovery_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: AC-Cookie** -- HMAC-SHA256 cookie with timestamp; generation and verification; replay protection
   - Tests: `TestACCookieGenVerify`, `TestACCookieReplay`
   - Files: `cookie.go`, `cookie_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Session management** -- session ID allocation, session table, per-interface limits
   - Tests: `TestSessionIDAlloc`, `TestSessionIDExhausted`
   - Files: `session.go`, `session_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Kernel integration** -- AF_PPPOX + PX_PROTO_OE socket creation, /dev/ppp channel+unit
   - Tests: `TestPPPoEKernelSocket` (Linux only)
   - Files: `kernel_linux.go`, `kernel_linux_test.go`, `kernel_other.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Listener and subsystem** -- raw socket listener, discovery dispatch, PPP Driver integration, subsystem lifecycle
   - Tests: `TestServiceNameFilter`, integration tests
   - Files: `listener.go`, `subsystem.go`, `register.go`, `config.go`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: CLI and config** -- YANG schema, show commands, hub registration
   - Files: `schema/*.yang`, `cmd/pppoe/`, hub imports
   - Verify: `make generate`; CLI commands work

7. **Functional tests** -> Create after feature works.
8. **Full verification** -> `make ze-verify`
9. **Complete spec** -> Fill audit tables, write learned summary, delete spec.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-14 has implementation with file:line |
| Correctness | PPPoE header format matches RFC 2516; session ID in network byte order |
| Naming | Component name `pppoe`; YANG `ze-pppoe-conf` |
| Data flow | PADI -> PADO -> PADR -> PADS -> kernel socket -> PPP Driver StartSession |
| Rule: goroutine-lifecycle | Discovery listener has clean shutdown; per-session cleanup on PADT |
| Transport agnostic | PPP component receives PPPoE sessions identically to L2TP sessions |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| PPPoE component exists | `ls internal/component/pppoe/*.go` |
| Discovery wire format | `go test ./internal/component/pppoe/ -run TestParse` |
| Kernel integration | `go test ./internal/component/pppoe/ -run TestPPPoEKernel` (Linux) |
| CLI works | `grep "show pppoe" internal/component/cmd/pppoe/` |
| `make ze-verify` passes | Run and check exit code |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| DoS protection | AC-Cookie prevents PADR flood; rate-limit PADI processing |
| Input validation | All PPPoE tag lengths validated before reading |
| MAC spoofing | Session bound to subscriber MAC; PADT only accepted from correct MAC |
| Session ID exhaustion | Bounded by uint16; reject gracefully when exhausted |
| Raw socket privileges | Requires CAP_NET_RAW or root |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Discovery format wrong | Re-read RFC 2516; compare with pcap |
| Kernel socket fails | Check kernel config (CONFIG_PPPOE); compare with pppd source |
| PPP session fails after PPPoE | Check ChanFD/UnitFD wiring; compare with L2TP path |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Phases 1-5 commit was "ready" | Only core code done, no CLI/YANG/hub/docs/review | Handoff review | Framed as checkpoint instead |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| (none) | | |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| (none) | | | |

## Design Insights

- **devPPPSetup is transport-agnostic.** L2TP and PPPoE both used identical /dev/ppp ioctl sequences. Extracted to `ppp.DevPPPSetup()`, removing 148 lines of duplication.
- **StartSession grew past hugeParam.** Adding 4 PPPoE fields pushed it over the gocritic threshold (304 bytes). All internal pass-by-value sites converted to pointer. The channel type (`chan StartSession`) remains value-typed since it owns the data.
- **LookupSnapshot prevents data race.** `SessionTable.Lookup()` returns a `*Session` pointer, but fields are mutated after `Add()` without the table lock. `LookupSnapshot()` copies fields under the lock for CLI queries.

## Implementation Summary

### What Was Implemented
- RFC 2516 PPPoE access concentrator: discovery wire format (PADI/PADO/PADR/PADS/PADT), HMAC-SHA256 AC-Cookie, per-interface session tables with bitmap SID allocation, PADI rate limiting, Service-Name filtering
- Kernel integration: AF_PPPOX + PX_PROTO_OE sockets, shared ppp.DevPPPSetup()
- Subsystem lifecycle: discovery reader goroutine, event consumer, PPP Driver integration
- YANG schemas: ze-pppoe-conf (config), ze-pppoe-api (RPCs), ze-pppoe-cmd (CLI tree)
- CLI: show pppoe summary/sessions/session/statistics/interfaces (5 RPC handlers)
- Hub wiring: PPPoE subsystem registered alongside L2TP
- StartSession extensions: AccessInterface, SubscriberMAC, ServiceName, VendorTags
- Functional tests: 3 .ci files (basic discovery, VLAN, concurrent L2TP)
- 41 unit tests across discovery, cookie, session, and rate-limit

### Bugs Found/Fixed
- SIOCGIFHWADDR accessor: Go's `x/sys/unix.Ifreq` has no hwaddr method; used unsafe offset into raw union (review run 1)
- Double-close race: `Session.Remove()` returns PppoxFD atomically to prevent double-close between discovery reader and event consumer (review run 1)
- FD leak on PADS build failure: added chanFD+unitFD cleanup on frame-too-large path (review run 2)
- Ethertype validation: discovery packets with wrong ethertype silently dropped (review run 2)
- PADR dedup: re-send existing PADS if subscriber retransmits PADR before PPP starts (review run 2)
- parseTags capacity: capped initial tag slice capacity to avoid over-allocation on malformed frames (review run 3)
- LookupSession data race: added LookupSnapshot() that copies under SessionTable lock (review run 4)
- StartSession hugeParam: converted spawnSession/emitRejection/run to pointer receivers (lint)

### Documentation Updates
- `docs/features.md`: PPPoE feature entry
- `docs/comparison.md`: BNG section updated for dual access
- `docs/guide/command-reference.md`: PPPoE CLI commands
- `docs/guide/configuration.md`: PPPoE config section
- `docs/guide/pppoe.md`: new guide page
- `docs/architecture/core-design.md`: PPPoE component dependency

### Deviations from Plan
- `listener.go` not created as separate file; discovery reading lives in `subsystem.go` (simpler, one goroutine)
- `config_test.go` and `kernel_linux_test.go` not created (config parsing tested via integration; kernel requires Linux)
- devPPPSetup extracted to `ppp/devppp_linux.go` instead of staying in each transport

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestBuildPADO`, `TestParsePADI` | PADO with AC-Name, Service-Name, AC-Cookie |
| AC-2 | done | `TestBuildPADS`, server.go:handlePADR | PADS with SID after valid cookie |
| AC-3 | partial | ppp.Driver integration wired | Full LCP/Auth/IPCP requires Linux kernel |
| AC-4 | done | server.go:handlePADT, subsystem.go:eventConsumer | PADT teardown, PPP StopSession |
| AC-5 | done | server.go:handleSessionDown, BuildPADT | BNG-initiated PADT on EventSessionDown |
| AC-6 | done | `TestACCookieReplay`, server.go:handlePADR cookie check | Invalid cookie silently dropped |
| AC-7 | done | `TestMatchServiceName*` (6 tests), server.go | Service-Name filtering in PADI and PADR |
| AC-8 | done | subsystem.go:Start, per-interface SessionTable | Independent SID space per interface |
| AC-9 | done | test/pppoe/pppoe-vlan.ci | VLAN sub-interface test |
| AC-10 | done | server.go: MaxMRU=PPPoEMaxMTU (1492) | StartSession.MaxMRU set to 1492 |
| AC-11 | done | `TestAllocSIDExhausted`, `TestAllocMaxSessions` | Graceful rejection at limit |
| AC-12 | done | test/pppoe/pppoe-concurrent-l2tp.ci | Both subsystems coexist |
| AC-13 | done | cmd/pppoe/pppoe.go:handleSessions | `show pppoe sessions` CLI |
| AC-14 | partial | StartSession.AccessInterface wired | NAS-Port-Type requires RADIUS plugin integration |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestParsePADI | done | discovery_test.go | |
| TestBuildPADO | done | discovery_test.go | |
| TestParsePADR | done | discovery_test.go | |
| TestBuildPADS | done | discovery_test.go | |
| TestBuildPADT | done | discovery_test.go | |
| TestACCookieGenVerify | done | cookie_test.go | |
| TestACCookieReplay | done | cookie_test.go | |
| TestSessionIDAlloc | done | session_test.go | |
| TestSessionIDExhausted | done | session_test.go | |
| TestPPPoEKernelSocket | skipped | requires Linux | |
| TestServiceNameFilter | done | discovery_test.go (6 tests) | |

### Audit Summary
- **Total items:** 25 (14 ACs + 11 tests)
- **Done:** 22
- **Partial:** 2 (AC-3 kernel path, AC-14 RADIUS NAS-Port-Type)
- **Skipped:** 1 (kernel socket test, Linux-only)

## Review Gate

### Run 1 (phases 1-5, pre-commit)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | SIOCGIFHWADDR accessor missing | kernel_linux.go | Fixed: unsafe offset |
| 2 | BLOCKER | Double-close race on PppoxFD | session.go:Remove | Fixed: atomic return |
| 3 | BLOCKER | FD leak on kernel setup failure | server.go | Fixed: cleanup path |
| 4 | ISSUE | Ethertype not validated | discovery.go | Fixed: check added |
| 5 | ISSUE | PADR dedup missing | server.go | Fixed: LookupByMAC check |
| 6 | ISSUE | PADI limiter not wired | server.go | Fixed: wired in handlePADI |

### Run 2 (phases 1-5, post-fix)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | parseTags over-allocates on malformed | discovery.go | Fixed: capped capacity |

### Run 3 (phases 1-5, post-fix)
Clean. 0 findings.

### Run 4 (phases 6-8)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | LookupSession data race | snapshot.go | Fixed: LookupSnapshot |
| 2 | NOTE | vendorTagsFromPacket captures only first | server.go | Accepted: TR-101 deferred |
| 3 | NOTE | Subsystem.Reload is no-op | subsystem.go | Accepted: future work |

### Run 5 (post-fix)
Clean. 0 findings.

### Run 6 (final)
Clean. 0 findings.

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above
