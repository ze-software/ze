# Spec: bng-3 -- IPv6 Address Pools and DHCPv6-PD

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-bng-1-radius-attributes |
| Phase | 4/4 |
| Updated | 2026-05-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/plugins/l2tppool/pool.go` -- existing IPv4 bitmap pool
4. `internal/component/ppp/ipv6cp.go` -- IPv6CP implementation
5. `internal/component/ppp/ncp.go` -- NCP coordination, onNCPOpened
6. `internal/component/ppp/ip_events.go` -- EventIPRequest, AddressFamily
7. `internal/component/ppp/session_run.go` -- afterLCPOpen (where RA+DHCPv6 goroutines start)
8. `internal/component/l2tp/session_metadata.go` -- AuthMetadata (IPv6 fields)
9. `internal/component/l2tp/handler.go` -- RegisterPoolHandler / RegisterPrefixHandler

## Task

Extend Ze's BNG to support dual-stack subscribers via IPv6 address pools
and DHCPv6 Prefix Delegation (DHCPv6-PD) over PPP sessions.

The current l2tppool plugin handles IPv4 only. IPv6CP (RFC 5072) is
already implemented and negotiates Interface-Identifiers, but there is no
IPv6 address pool and no mechanism to delegate prefixes to subscribers.

For a dual-stack BNG:
1. **IPv6 address pool**: allocates /128 or /64 addresses for point-to-point links (analogous to IPv4 pool for IPCP)
2. **DHCPv6 Prefix Delegation**: delegates a /48, /52, /56, or /64 prefix to the subscriber CPE for downstream use
3. **RADIUS integration**: Framed-IPv6-Prefix (97), Delegated-IPv6-Prefix (123), Framed-IPv6-Pool (100)

DHCPv6-PD runs over the PPP link after IPv6CP establishes the link-local
interface. The BNG acts as DHCPv6 server (delegating router) on each pppN.
This is not a relay; it assigns prefixes from configured pools or
RADIUS-supplied values.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/l2tppool/pool.go` -- IPv4 bitmap pool architecture
- [ ] `internal/component/ppp/ipv6cp.go` -- IPv6CP option handling
- [ ] `internal/component/ppp/ncp.go` -- NCP coordination, onNCPOpened, IPv6CP flow
- [ ] `internal/component/ppp/session_run.go` -- session lifecycle after NCP
- [ ] `internal/component/ppp/ip_events.go` -- EventIPRequest with AddressFamilyIPv6
- [ ] `internal/component/l2tp/pppox_linux.go` -- pppN interface creation

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5072.md` -- IPv6CP (Interface-Identifier negotiation)
- [ ] `rfc/short/rfc3633.md` -- DHCPv6 Prefix Delegation
- [ ] `rfc/short/rfc3315.md` -- DHCPv6 (Stateful Address Configuration) -- relevant subset
- [ ] `rfc/short/rfc4818.md` -- RADIUS Delegated-IPv6-Prefix attribute (123)
- [ ] `rfc/short/rfc6911.md` -- RADIUS attrs for IPv6 access: Framed-IPv6-Prefix (97), Framed-IPv6-Pool (100)

**Key insights:**
- IPv6CP only negotiates Interface-Identifiers (64-bit); actual addressing is via RA or DHCPv6
- PPP link-local addresses are fe80::<interface-id>/10; global addressing needs separate mechanism
- DHCPv6-PD is the standard way ISPs delegate prefixes to CPE over PPP
- DHCPv6 runs as a separate protocol over the PPP link (protocol 0x0057 = IPv6)
- BNG must run a per-session DHCPv6 server, not a relay

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/l2tppool/pool.go` -- ipv4Pool struct, bitmap allocation, no IPv6 support
- [ ] `internal/component/ppp/ipv6cp.go` -- negotiates Interface-ID only; no address assignment
- [ ] `internal/component/ppp/ncp.go` -- IPv6CP driven to Opened, emits EventSessionIPAssigned with IPv6 family
- [ ] `internal/component/ppp/ip_events.go` -- EventIPRequest has Family field (IPv4/IPv6), PeerInterfaceID for IPv6

**Behavior to preserve:**
- IPv4 pool allocation unchanged
- IPv6CP Interface-Identifier negotiation unchanged
- PPP session lifecycle unchanged
- Existing IPv6CP tests pass

**Behavior to change:**
- Pool plugin gains IPv6 prefix pool (separate from IPv4 pool)
- DHCPv6 server runs per pppN interface for prefix delegation
- RADIUS Framed-IPv6-Prefix / Delegated-IPv6-Prefix / Framed-IPv6-Pool consumed
- IPv6 routes installed for delegated prefixes

## Data Flow (MANDATORY)

### Entry Point
- IPv6CP completes (Interface-IDs negotiated)
- DHCPv6 Solicit arrives on pppN from subscriber CPE
- RADIUS Access-Accept may contain Framed-IPv6-Prefix or Delegated-IPv6-Prefix

### Transformation Path

#### Link addressing (post-IPv6CP):
1. IPv6CP completes (Interface-IDs negotiated), NCP phase ends
2. afterLCPOpen: PPPIOCCONNECT, SetMRU, SetMTU, SetAdminUp (pppN now exists in kernel)
3. Set sysctls on pppN: accept_ra=0, autoconf=0, forwarding=1
4. Kernel auto-assigns link-local fe80::<local-interface-id> on pppN
5. Start RA goroutine: raw ICMPv6 socket + SO_BINDTODEVICE + ICMP6_FILTER(RS only) + join ff02::2; burst 5 RAs at 3s then periodic ~600s
6. Start DHCPv6 goroutine: UDP socket port 547 + SO_BINDTODEVICE + join ff02::1:2
7. BNG sends Router Advertisement on pppN (M+O flags set, no prefix info)
8. Subscriber sends DHCPv6 Solicit (IA_PD option) to ff02::1:2
9. DHCPv6 server checks AuthMetadata.DelegatedIPv6Prefix; if set, uses RADIUS prefix
10. Otherwise DHCPv6 server calls PrefixHandler to allocate from pool (respects FramedIPv6Pool for named pool selection)
11. BNG sends DHCPv6 Advertise with IA_PD
12. Subscriber sends DHCPv6 Request
13. BNG sends DHCPv6 Reply, installs route via IfaceBackend.AddRoute(pppN, prefix, fe80::<peer-id>, 0)
14. On session teardown: teardownNCPResources stops RA+DHCPv6 goroutines, RemoveRoute, releases prefix to pool

#### RADIUS-driven:
1. Access-Accept includes Delegated-IPv6-Prefix=2001:db8:abcd::/48
2. Stored in session attributes (from bng-1 spec)
3. When DHCPv6 Solicit arrives, use RADIUS-assigned prefix instead of pool

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPP session -> RA sender | Raw ICMPv6 socket (IPPROTO_ICMPV6) + SO_BINDTODEVICE on pppN; goroutine started from afterLCPOpen | [ ] |
| PPP session -> DHCPv6 server | UDP socket (AF_INET6 SOCK_DGRAM port 547) + SO_BINDTODEVICE on pppN; goroutine started from afterLCPOpen | [ ] |
| DHCPv6 server -> prefix pool | RegisterPrefixHandler direct call (same pattern as RegisterPoolHandler) | [ ] |
| RADIUS handler -> AuthMetadata | DelegatedIPv6Prefix, FramedIPv6Prefix, FramedIPv6Pool stored via StoreSessionMetadata | [ ] |
| DHCPv6 server -> AuthMetadata | LoadSessionMetadata; RADIUS-assigned prefix takes priority over pool | [ ] |
| DHCPv6 server -> kernel | IfaceBackend.AddRoute for delegated prefix via subscriber link-local | [ ] |

### Integration Points
- `l2tppool` plugin -- add IPv6 prefix pool alongside IPv4 pool; RegisterPrefixHandler (direct call, same pattern as RegisterPoolHandler)
- PPP session `afterLCPOpen` -- sysctl hardening, start RA + DHCPv6 goroutines after SetAdminUp
- PPP session `teardownNCPResources` -- stop RA + DHCPv6 goroutines, RemoveRoute, release prefix
- `IfaceBackend` interface -- existing AddRoute/RemoveRoute handle IPv6 (no changes needed)
- `AuthMetadata` -- add FramedIPv6Prefix, DelegatedIPv6Prefix, FramedIPv6Pool fields
- `l2tp/handler.go` -- add RegisterPrefixHandler alongside existing RegisterPoolHandler
- RADIUS handler -- extract Delegated-IPv6-Prefix (123), Framed-IPv6-Prefix (97), Framed-IPv6-Pool (100)
- EventBus -- session-down triggers prefix release (same pattern as IPv4 onSessionDown)

### Architectural Verification
- [ ] No bypassed layers (DHCPv6/RA run on kernel-side sockets on pppN after PPPIOCCONNECT; IPv6 data frames are kernel-handled per frame.go:25)
- [ ] No unintended coupling (DHCPv6 server is per-session goroutine owned by pppSession, no global state beyond pool)
- [ ] No duplicated functionality (extends existing pool plugin, reuses IfaceBackend.AddRoute/RemoveRoute)
- [ ] Zero-copy preserved where applicable (DHCPv6 codec uses buffer-first pattern)
- [ ] Goroutine lifecycle correct (RA+DHCPv6 started in afterLCPOpen, stopped in teardownNCPResources; same pattern as readFrames)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| IPv6CP completes on session | -> | RA sent on pppN with M+O flags | `TestRAOnIPv6CPOpen` |
| DHCPv6 Solicit received | -> | Prefix allocated from pool, Reply sent | `TestDHCPv6PDFromPool` |
| RADIUS provides Delegated-IPv6-Prefix | -> | DHCPv6 delegates RADIUS prefix, not pool | `TestDHCPv6PDFromRADIUS` |
| Session teardown | -> | Prefix released, route removed | `TestIPv6PrefixReleaseOnTeardown` |
| Pool exhausted | -> | DHCPv6 NoPrefixAvail status | `TestDHCPv6PDPoolExhausted` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IPv6 prefix pool configured; DHCPv6 Solicit with IA_PD | Prefix allocated from pool; DHCPv6 Reply with IA_PD sent |
| AC-2 | Delegated prefix /56 | Route for /56 installed pointing at subscriber link-local |
| AC-3 | Session teardown | Delegated prefix released to pool; route removed |
| AC-4 | Pool exhausted | DHCPv6 Reply with NoPrefixAvail status code |
| AC-5 | RADIUS provides Delegated-IPv6-Prefix | RADIUS prefix used instead of pool allocation |
| AC-6 | RADIUS provides Framed-IPv6-Pool="v6-gold" | Named IPv6 pool used for this session |
| AC-7 | DHCPv6 Renew received | Lease extended; same prefix maintained |
| AC-8 | DHCPv6 Release received | Prefix released; route removed |
| AC-9 | Multiple sessions, each gets unique prefix | No prefix collision between concurrent sessions |
| AC-10 | Configurable prefix lengths (/48, /52, /56, /60, /64) | Pool delegates configured length |
| AC-11 | RA sent after IPv6CP with M+O flags | Subscriber knows to use DHCPv6 |
| AC-12 | Subscriber sends DHCPv6 Information-Request (no PD) | BNG responds with DNS servers (if configured) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIPv6PrefixPoolAllocate` | `internal/plugins/l2tppool/pool_v6_test.go` | Allocate prefix from pool | |
| `TestIPv6PrefixPoolRelease` | `internal/plugins/l2tppool/pool_v6_test.go` | Release prefix back to pool | |
| `TestIPv6PrefixPoolExhausted` | `internal/plugins/l2tppool/pool_v6_test.go` | Full pool rejects | |
| `TestIPv6PrefixPoolVariableLengths` | `internal/plugins/l2tppool/pool_v6_test.go` | /48, /56, /64 all work | |
| `TestDHCPv6SolicitReply` | `internal/component/ppp/dhcpv6_test.go` | Solicit -> Advertise -> Request -> Reply |
| `TestDHCPv6Renew` | `internal/component/ppp/dhcpv6_test.go` | Renew extends lease | |
| `TestDHCPv6Release` | `internal/component/ppp/dhcpv6_test.go` | Release frees prefix | |
| `TestDHCPv6NoPrefixAvail` | `internal/component/ppp/dhcpv6_test.go` | Pool exhausted -> NoPrefixAvail | |
| `TestRAFlags` | `internal/component/ppp/ra_test.go` | RA has M+O set, no prefix info | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Prefix length | /48 - /64 | /64 | /47 (too large) | /65 (not useful) |
| Pool size | 1 - 65534 prefixes | 65534 | 0 | 65535+ |
| DHCPv6 T1 (preferred lifetime) | 1 - 4294967295 | 4294967295 | 0 (infinite) | N/A |
| DHCPv6 T2 (valid lifetime) | T1 - 4294967295 | 4294967295 | < T1 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipv6-pd-basic` | `test/l2tp/ipv6-pd-basic.ci` | Subscriber gets IPv6 prefix via DHCPv6-PD | |
| `ipv6-pd-radius` | `test/l2tp/ipv6-pd-radius.ci` | RADIUS assigns specific prefix | |

## Files to Modify

- `internal/plugins/l2tppool/l2tppool.go` -- register IPv6 prefix handler alongside IPv4 pool handler
- `internal/plugins/l2tppool/register.go` -- handle PrefixHandler registration, config parsing for IPv6 pools, onSessionDown prefix release
- `internal/plugins/l2tppool/schema/ze-l2tp-pool-conf.yang` -- IPv6 pool config (ipv6-pd container under pool)
- `internal/component/ppp/session.go` -- add ipv6SvcStop field to pppSession for RA+DHCPv6 goroutine cleanup
- `internal/component/ppp/session_run.go` -- afterLCPOpen: sysctl hardening + start RA+DHCPv6 goroutines; teardownNCPResources: stop goroutines + release prefix + remove route
- `internal/component/l2tp/session_metadata.go` -- add FramedIPv6Prefix, DelegatedIPv6Prefix, FramedIPv6Pool to AuthMetadata
- `internal/component/l2tp/handler.go` -- add RegisterPrefixHandler, PrefixHandler type

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema update | [x] | `internal/plugins/l2tppool/schema/ze-l2tp-pool-conf.yang` |
| CLI commands/flags | [x] | `show l2tp pool` extended for IPv6 |
| Functional test | [x] | `test/l2tp/ipv6-pd-basic.ci` |
| AuthMetadata IPv6 fields | [x] | `internal/component/l2tp/session_metadata.go` |
| PrefixHandler registration | [x] | `internal/component/l2tp/handler.go` |
| Sysctl hardening on pppN | [x] | `internal/component/ppp/session_run.go` (afterLCPOpen) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- IPv6 prefix delegation |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- IPv6 pool config |
| 3 | CLI command added/changed? | [ ] | |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- l2tp-pool IPv6 |
| 6 | Has a user guide page? | [ ] | |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [x] | RFC 3633, RFC 6911 |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- dual-stack BNG |
| 12 | Internal architecture changed? | [ ] | |

## Files to Create

- `internal/plugins/l2tppool/pool_v6.go` -- IPv6 prefix pool (bitmap-backed by prefix index)
- `internal/plugins/l2tppool/pool_v6_test.go` -- IPv6 pool tests
- `internal/component/ppp/dhcpv6.go` -- DHCPv6 message codec (parse/build, buffer-first) + server state machine logic
- `internal/component/ppp/dhcpv6_linux.go` -- UDP listener with SO_BINDTODEVICE, multicast join ff02::1:2, per-session goroutine
- `internal/component/ppp/dhcpv6_test.go` -- DHCPv6 codec + server tests
- `internal/component/ppp/ra.go` -- RA message building (M+O flags, RDNSS option)
- `internal/component/ppp/ra_linux.go` -- raw ICMPv6 socket with SO_BINDTODEVICE, ICMP6_FILTER(RS only), multicast join ff02::2, burst+periodic send
- `internal/component/ppp/ra_test.go` -- RA codec tests
- `test/l2tp/ipv6-pd-basic.ci` -- functional test
- `test/l2tp/ipv6-pd-radius.ci` -- RADIUS-assigned prefix test

## Implementation Steps

### Implementation Phases

1. **Phase: IPv6 prefix pool** -- bitmap-backed pool that allocates /N prefixes from a configured range
   - Tests: `TestIPv6PrefixPoolAllocate`, `TestIPv6PrefixPoolRelease`, `TestIPv6PrefixPoolExhausted`, `TestIPv6PrefixPoolVariableLengths`
   - Files: `pool_v6.go`, `pool_v6_test.go`, YANG schema
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Router Advertisement** -- send RA with M+O flags on pppN after IPv6CP opens
   - Tests: `TestRAFlags`
   - Files: `ra.go`, `ra_test.go`, `ncp.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: DHCPv6-PD server** -- per-session server handling Solicit/Request/Renew/Release on pppN
   - Tests: `TestDHCPv6SolicitReply`, `TestDHCPv6Renew`, `TestDHCPv6Release`, `TestDHCPv6NoPrefixAvail`
   - Files: `dhcpv6.go`, `dhcpv6_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Wiring** -- connect DHCPv6 server to prefix pool; route installation; teardown cleanup; RADIUS prefix support
   - Tests: wiring tests
   - Files: `session_run.go`, `l2tppool.go`
   - Verify: tests fail -> implement -> tests pass

5. **Functional tests** -> Create after feature works.
6. **Full verification** -> `make ze-verify`
7. **Complete spec** -> Fill audit tables, write learned summary, delete spec.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-12 has implementation with file:line |
| Correctness | DHCPv6 message format matches RFC 3633; RA flags correct per RFC 4861 |
| Naming | IPv6 pool config under `l2tp { pool { ipv6-pd { ... } } }` |
| Data flow | IPv6CP -> afterLCPOpen -> sysctl -> RA goroutine -> DHCPv6 goroutine -> Solicit -> PrefixHandler -> Reply -> AddRoute |
| Rule: goroutine-lifecycle | RA+DHCPv6 goroutines per session; started in afterLCPOpen, stopped in teardownNCPResources |
| Platform | Socket ops in _linux.go; codec/logic in plain .go (testable cross-platform) |
| Sysctl hardening | accept_ra=0, autoconf=0, forwarding=1 on pppN before starting RA/DHCPv6 |
| Multicast | DHCPv6 joins ff02::1:2; RA joins ff02::2; ICMP6_FILTER on RA socket |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| IPv6 prefix pool exists | `ls internal/plugins/l2tppool/pool_v6.go` |
| DHCPv6 codec + logic exists | `ls internal/component/ppp/dhcpv6.go` |
| DHCPv6 Linux socket layer exists | `ls internal/component/ppp/dhcpv6_linux.go` |
| RA codec exists | `ls internal/component/ppp/ra.go` |
| RA Linux socket layer exists | `ls internal/component/ppp/ra_linux.go` |
| AuthMetadata has IPv6 fields | `grep DelegatedIPv6Prefix internal/component/l2tp/session_metadata.go` |
| PrefixHandler registered | `grep RegisterPrefixHandler internal/component/l2tp/handler.go` |
| Tests pass | `go test ./internal/plugins/l2tppool/ -run TestIPv6Prefix` |
| `make ze-verify` passes | Run and check exit code |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | DHCPv6 messages from subscriber must be validated (length, option format, TLV bounds) |
| Resource exhaustion | One DHCPv6 + one RA goroutine per session; bounded by session count |
| Prefix overlap | Pool must never delegate overlapping prefixes |
| Spoofing | DHCPv6/RA only on pppN (point-to-point, SO_BINDTODEVICE); no spoofing risk |
| Sysctl hardening | accept_ra=0, autoconf=0 on pppN prevent subscriber-injected routes/addresses |
| ICMPv6 filtering | ICMP6_FILTER on RA socket accepts only Router Solicitation; drops all other ICMPv6 |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| DHCPv6 format wrong | Re-read RFC 3633; compare with pcap from working BNG |
| RA not triggering DHCPv6 | Check M+O flag handling in CPE |
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

### Decision Log

| # | Decision | Resolved | Rationale |
|---|----------|----------|-----------|
| 1 | DHCPv6/RA transport | Kernel-side UDP + ICMPv6 sockets on pppN | frame.go:25 declares ProtoIPv6 kernel-handled; after PPPIOCCONNECT IPv6 flows through kernel stack |
| 2 | DHCPv6 server lifecycle | Goroutine in afterLCPOpen, stopped in teardownNCPResources | Matches readFrames goroutine pattern; session goroutine owns what it creates |
| 3 | Prefix allocation path | RegisterPrefixHandler direct call on DHCPv6 Solicit | On-demand (not wasted if no Solicit); follows RegisterPoolHandler pattern |
| 4 | AuthMetadata IPv6 fields | FramedIPv6Prefix, DelegatedIPv6Prefix, FramedIPv6Pool | Mirrors IPv4 pattern; netip.Prefix carries address + prefix length |
| 5 | IfaceBackend extension | None needed | Existing AddRoute/RemoveRoute handle IPv6 CIDRs via string signatures |
| 6 | DHCPv6 codec | Hand-rolled minimal, buffer-first | Matches IPCP/IPv6CP codec pattern; ~200-300 lines; no external dep |
| 7 | IPv6 link-local | Kernel auto-assigns; ze sets accept_ra=0, autoconf=0, forwarding=1 | Kernel PPP driver uses IPv6CP Interface-ID; sysctls prevent subscriber route injection |
| 8 | Build tags | Codec in .go, socket/sysctl in _linux.go | Matches existing frame.go/frame_linux.go split |

### accel-ppp alignment (informational)

Verified against accel-ppp source (`accel-pppd/ipv6/dhcpv6.c`, `nd.c`):
- accel-ppp uses per-session sockets (not shared), same as ze's design
- DHCPv6: AF_INET6 SOCK_DGRAM + SO_BINDTODEVICE + join ff02::1:2 (identical)
- RA: AF_INET6 SOCK_RAW IPPROTO_ICMPV6 + SO_BINDTODEVICE + join ff02::2 (identical)
- RA burst: 5 initial RAs at 3s, then periodic ~600s with jitter
- ICMP6_FILTER on RA socket to only pass Router Solicitation
- Route install for delegated prefixes, remove on session teardown
- Hand-rolled DHCPv6 codec (~600 lines C header + packet parser)

## RFC Documentation

Add `// RFC 3633 Section 5.1` above DHCPv6-PD Solicit handling.
Add `// RFC 4861 Section 4.2` above RA flag setting.
Add `// RFC 6911 Section 3` above Framed-IPv6-Prefix RADIUS handling.

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/`
- [ ] Summary included in commit
