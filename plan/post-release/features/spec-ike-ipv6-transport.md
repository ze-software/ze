# Spec: IKE over IPv6

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | IKEv2 runs only over IPv4: an IPv6 peer or an AAAA-only name cannot be reached, and commit accepts both. Post-release feature; the commit refusal half is the defect to settle first. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every IKE socket Ze opens is `udp4` (`internal/component/ike/transport/udp.go`
resolves and listens with `udp4`), and the connect cycle resolves the configured
`remote-address` with `udp4` (`engine/cycle_addr.go` `resolveRemoteFn`). An IPv6
literal or a name with only AAAA records is therefore unreachable, and commit
refuses neither. This spec carries a second socket family through the transport
and the listener set (`engine/apply.go`), so a peer configured over IPv6
negotiates, and until it lands, commit refuses an IPv6 `remote-address` or
`local-address` with the fix in the message.

The split came from the closure of spec-home-cpe-4-ike-dynamic-wan, which made a
DNS-named remote work over IPv4 and recorded IPv6 as a known limitation.

## Required Reading

### Architecture Docs
- [ ] <doc> - <why>

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/transport/udp.go` - resolves and listens with `udp4`
- [ ] `internal/component/ike/engine/cycle_addr.go` - `resolveRemoteFn` resolves `remote-address` with `udp4`

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | How this design extends or replaces it |
|-----------|-------|----------------------------------------|
| <mechanism> | <file> | <extend / replace> |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- <entry>

### Transformation Path
1. <step>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|

### Integration Points
- <point>

### Architectural Verification
- [ ] <check>

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | <answer> |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| <entry> | → | <code> | <test> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | <input> | <expected> |
| AC-2 | a Child SA over an IPv6 outer header on UDP 4500 with no NAT, whose inbound state carries the ESP-in-UDP template, receiving bare ESP whole and fragmented, inside and outside its selectors (moved from AC-8 of `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md`) | the SA installs instead of being refused; inside the selectors the inner packet is delivered, outside it is dropped by the SPD catch-all; the ECN field of the outer traffic class reaches the inner header; `Udp6InCsumErrors` does not move |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|

## Files to Modify
- <file>

## Files to Create
- <file>

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|

## Implementation Steps

1. <phase>

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|

### Failure Routing
| Failure | Route To |
|---------|----------|

## Dual-form receipt over IPv6

Moved here on 2026-11-12 from `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md`
(its AC-8, now AC-2 here). Today `InstallSA` refuses an SA that asks for both ESP forms over
IPv6, and `MigrateTunnel` refuses a MOBIKE move to IPv6 endpoints
(`ike/dataplane/xfrm_linux.go` `InstallSA`, `xfrm_migrate_linux.go`
`validateXFRMMigration`), each naming this gap. `installChildDataplane`
(`ike/engine/child.go`) sets `AcceptBothESPForms` on every inbound state, and `newChildSA`
(`ike/engine/create_child.go`) sets `UDPEncap` on port 4500 with or without a NAT. So the
first IPv6 Child SA on port 4500 this spec negotiates hits that refusal, and the items
below are owed in the same work. The IPv6 variant of the tc fast path already exists and
is tested (`espFormFastPathIPv6`, `TestESPFormRewriteProgram`); nothing writes its map yet.

| # | Owed | Evidence |
|---|------|----------|
| 1 | A raw `ip6:50` reader beside the `ip4:50` one (`espFormListenNetwork`), carrying its own per-socket bypass set with `IPV6_XFRM_POLICY` and an `AF_INET6` selector | The IPv6 raw receive checks the inbound policy as the IPv4 one does: `rawv6_rcv` (`net/ipv6/raw.c`, v7.0) opens with `if (!xfrm6_policy_check(sk, XFRM_POLICY_IN, skb)) {`. `xfrm_sk_policy_lookup` (`net/xfrm/xfrm_policy.c`) skips a socket policy of another family (`if (pol->family != family) {`), so the `AF_INET` exemption `espFormBypassInboundPolicy` sets never covers an IPv6 copy |
| 2 | `writeESPForm6`: the IPv6 and UDP headers, with the UDP checksum computed over the pseudo-header, and a computed zero written as `0xFFFF` | RFC 8200 Section 8.1: "That is, whenever originating a UDP packet, an IPv6 node must compute a UDP checksum over the packet and the pseudo-header, and, if that computation yields a result of zero, it must be changed to hex FFFF for placement in the UDP header." and "IPv6 receivers must discard UDP packets containing a zero checksum and should log the error." The same section's zero-checksum mode for tunnel ports is the alternative to weigh at design |
| 3 | The UDP ports passed into the builder, before the checksum | The IPv4 complete path writes the ports after building (`espFormReceiver.run`), which is valid only because the IPv4 UDP checksum is zero. The IPv6 checksum covers the ports, so a port written afterwards invalidates it |
| 4 | One rate limiter shared by both readers | `espFormLimiter` is not safe for concurrent use and each `run` holds its own (`espform.go`). Two readers with one each would double the 5000/s bound `espFormRate` sets for the userspace path |
| 5 | The traffic class copied into the rebuilt header, for ECN | RFC 7296 Section 2.24 (`RFC7296-2.24-2`), as the IPv4 path does with the TOS (`writeESPForm`, `TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE`). An IPv6 raw socket is expected to hand over the payload without the IPv6 header (RFC 3542 Section 3, whose text is not yet under `rfc/full/`; confirm it in `rawv6_rcv` first), so the class comes from the control message `IPV6_RECVTCLASS` enables (`net/ipv6/ipv6_sockglue.c`) |
| 6 | `InstallSA` and `MigrateTunnel` accept IPv6, refuse a mixed pair (peer and local of different families), and refuse a link-local IPv6 local address | A link-local destination needs a strict lookup bound to the input interface: `rt6_need_strict` (`include/net/ip6_route.h`) is true for `IPV6_ADDR_MULTICAST`, `IPV6_ADDR_LINKLOCAL` and `IPV6_ADDR_LOOPBACK`. The complete path's datagram enters on `ze-espform`, so that strict lookup with `ze-espform` as `iif` answers the null route and the datagram is dropped. `newESPFormFastPathValue6` already refuses an IPv4-mapped address |
| 7 | The IPv6 inner policy check judged by the local route's `DST_NOPOLICY`, not by the receiving device | IPv6 never reads the receiving device's `disable_policy` for this: `__xfrm_policy_check2` (`include/net/xfrm.h`) returns early on `skb_dst(skb)->flags & DST_NOPOLICY`, which `fib6_info_dst_flags` (`net/ipv6/route.c`) sets from `rt->dst_nopolicy` and `addrconf_set_nopolicy` (`net/ipv6/addrconf.c`) sets on the routes of a device's addresses. So the device holding the SA's local address decides, and `ze-espform`'s own IPv6 `disable_policy` does not. The IPv4 sysctls `espFormTUNSysctls` writes have no IPv6 counterpart yet, and `disable_ipv6` 0 is owed on `ze-espform` so it receives IPv6 at all |
| 8 | The kernel test rows, on a veth between two network namespaces: addresses added with `IFA_F_NODAD`, a `NUD_PERMANENT` neighbour, the sender's socket at `IPV6_PMTUDISC_DONT` so a large datagram fragments at the source, `Ip6ReasmOKs` read from `/proc/thread-self/net/snmp6` to prove the fragment rows reassembled, and `Udp6InCsumErrors` unchanged to prove the checksum | The IPv4 rows of `TestRFC4301BareESPOnTemplatedSAIsPolicyChecked` (`ike/engine/rfc4301_espform_policy_integration_linux_test.go`) are the shape: production SPD (`installUnmatched`, then `installIKEBypass`), drops counted as `XfrmInPolBlock` +1 and `XfrmInNoPols` +0, the fast path's counter (`Rewritten6` here) advanced by the whole rows only |
| 9 | A `CHECKSUM_COMPLETE` IPv6 skb is served by the complete path | The fast path leaves it byte-identical, because it cannot write a valid checksum for it (`espFormFastPathIPv6`); items 1 to 5 carry it |

## Design Insights

<insight>

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-11-07 | IKE over IPv6, every socket udp4 |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- <limitation>

## RFC Documentation (Scope: protocol)

<quotes owed>

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le test ipsec` and `./le test integration interop-ipsec` pass for the IPv6 peer

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
