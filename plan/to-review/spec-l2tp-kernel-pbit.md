# Spec: Kernel L2TP P-Bit for PPP LCP Echo

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-06-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `docs/architecture/wire/l2tp.md` - wire format and P-bit documentation
3. `rfc/short/rfc2661.md` - Section 3.1 (P bit definition)

## Task

Add kernel-level support for setting the L2TP P (Priority) bit on data
messages carrying PPP LCP Echo-Request/Echo-Reply frames. These packets are
used for CQM (Connection Quality Monitoring) and are time-sensitive:
delayed echo packets skew RTT measurements and inflate loss ratios.

The kernel's `l2tp_ppp` module constructs L2TP data headers internally.
Ze userspace cannot set P=1 on these packets. This spec defines a kernel
patch adding a new `L2TP_ATTR_PRIORITY_LCP` generic netlink attribute plus
the ze integration to use it, enabling per-session priority marking for
LCP echo traffic.

## Background

### What the RFC says

RFC 2661 Section 3.1: the P bit (bit 7 of the header flags word) applies to
data messages only. P=1 means "preferential treatment." Control messages
MUST have P=0. The RFC does not specify what preferential treatment means;
it is a hint to the network and processing layers.

### Current data path

1. Kernel ppp_generic module manages PPP state (LCP, IPCP, auth, echo)
2. PPP frames are passed to l2tp_ppp for encapsulation
3. l2tp_ppp builds the L2TP data header (flags, tunnel/session IDs, optional Ns/Nr)
4. The encapsulated packet is sent via the tunnel's UDP socket
5. The L2TP data header is always P=0; there is no mechanism to set P=1

### What ze sees

Ze receives echo RTT measurements via `ppp.EventEchoRTT` from the kernel's
PPP stack. Ze does not send or intercept PPP frames in userspace. The CQM
system (`cqm.go`, `observer.go`) aggregates RTT samples into 100-second
buckets and emits `ze_l2tp_lcp_echo_rtt_seconds` and
`ze_l2tp_lcp_echo_loss_ratio` metrics.

### Why P=1 matters for CQM

When intermediate equipment (switches, routers, shapers) between the LAC
and LNS is congested, bulk subscriber data and CQM echo packets compete for
the same queue. Echo packets delayed by congestion produce inflated RTT
measurements, masking the subscriber's actual link quality. P=1 tells
intermediate L2TP-aware equipment to preferentially forward these packets,
keeping RTT measurements accurate even under congestion.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/l2tp.md` - data message P-bit section
  -> Constraint: ze parses and encodes P bit correctly but kernel encapsulation does not expose it
- [ ] `docs/research/l2tpv2-implementation-guide.md` - S21 kernel L2TP subsystem

### RFC Summaries
- [ ] `rfc/short/rfc2661.md` - Section 3.1 P bit
  -> Constraint: P=1 is for data messages only. Control MUST be P=0.
- [ ] `rfc/short/rfc1661.md` - PPP LCP, Echo-Request (code 9), Echo-Reply (code 10)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/genl_linux.go` - kernel L2TP generic netlink attributes
  -> Constraint: no L2TP_ATTR for priority. Attributes defined: PwType, EncapType, ProtoVersion, ConnID, PeerConnID, SessionID, PeerSessionID, RecvSeq, SendSeq, LNSMode, FD, IPSAddr, IPDAddr, UDPSPort, UDPDPort.
- [ ] `internal/component/l2tp/pppox_linux.go` - PPPoL2TP socket options
  -> Constraint: socket options are RecvSeq, SendSeq, LNSMode. No priority option.
  -> Constraint: SOL_PPPOL2TP = 273 (x86_64/arm64).
- [ ] `internal/component/l2tp/header.go` - flagP=0x0100, WriteDataHeader, ParseMessageHeader
  -> Constraint: P bit already works in ze's wire layer.
- [ ] `internal/component/l2tp/reactor_kernel.go` - handleEchoRTT receives ppp.EventEchoRTT
  -> Constraint: ze observes echo RTT but does not generate echo packets.
- [ ] `internal/component/l2tp/cqm.go` - CQMBucket, addEcho, 100s aggregation
- [ ] `internal/component/l2tp/kernel_linux.go` - kernel session setup via genl + pppox

**Behavior to preserve:**
- Kernel session setup flow (genl create + pppox connect + setsockopt)
- CQM aggregation pipeline (echo RTT -> sample ring -> metrics)
- Default P=0 for all data messages when priority is not enabled

**Behavior to change:**
- Kernel sets P=1 in the L2TP data header for PPP frames with protocol=LCP (0xC021)
  and LCP code Echo-Request (9) or Echo-Reply (10)
- Ze sets a per-session flag during session setup to enable this behavior

## Design

### Approach: per-session generic netlink attribute

Add a new `L2TP_ATTR_PRIORITY_LCP` attribute (NLA_U8, 0 or 1) to the kernel's
`l2tp_netlink.c`. When set to 1 on a session, `l2tp_core.c` inspects outbound
PPP frames: if the PPP protocol field is 0xC021 (LCP), the P bit is set in
the L2TP data header. Marking all LCP (not just Echo-Request/Reply) is
acceptable: negotiation LCP (ConfReq/Ack/Nak/Rej) is one-time at session
start, while Echo-Request/Reply recurs throughout the session lifetime.

**Kernel changes (net/l2tp/):**
- `include/uapi/linux/l2tp.h`: add `L2TP_ATTR_PRIORITY_LCP` to the attribute enum
- `l2tp_core.h`: add `priority_lcp` bool to `struct l2tp_session`
- `l2tp_netlink.c`: parse `L2TP_ATTR_PRIORITY_LCP` in session create/modify;
  export in `l2tp_nl_session_fill` for dump/get
- `l2tp_core.c` or `l2tp_ppp.c`: in the xmit path, check `session->priority_lcp`
  and the PPP protocol field; set flagP in the header if matched

**Ze changes (internal/component/l2tp/):**
- `genl_linux.go`: add `l2tpAttrPriorityLCP` constant (value TBD by kernel patch)
- `genl_linux.go`: add the attribute to `genlCreateSession` when config enables it
- `config.go`: add `PriorityLCPEcho bool` parameter
- `kernel_linux.go`: feature-detect at session create; fall back gracefully on EINVAL
- YANG schema: add `priority-lcp-echo` leaf under l2tp session config
- Doctor check: probe kernel for attribute support at startup

### Rejected alternatives

| Alternative | Why rejected |
|-------------|-------------|
| PPPoL2TP socket option | Less discoverable; cannot be set on sessions created by `ip l2tp add session`; inconsistent with genl-based model |
| BPF/TC hook (no kernel patch) | Fragile offset calculations, runtime BPF dependency, CAP_BPF requirement, harder to test |
| Per-tunnel flag (mark all sessions) | Not all sessions need CQM; per-session gives operator control |
| Mark only Echo-Request/Reply (code 9/10) | Adds code-level inspection for negligible gain; LCP negotiation is one-time at session start, Echo is the recurring case |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Kernel l2tp xmit path has access to the PPP frame content before encapsulation | l2tp_ppp.c calls l2tp_xmit_skb which builds the header around the sk_buff | Cannot inspect PPP protocol field; would need a different hook point | Read l2tp_ppp.c xmit path | unvalidated |
| A-2 | PPP protocol field is at a fixed offset in the sk_buff at l2tp xmit time | PPP header is 2 bytes (protocol) or 1 byte (compressed) at sk_buff head | Offset calculation would be wrong for compressed protocol fields | Read ppp_generic.c output format | unvalidated |
| A-3 | Adding a conditional branch per packet on the xmit path has negligible performance impact | Branch is a single byte compare on a per-session bool flag; predicted taken (P=0 path) for most packets | Measurable throughput regression on high-PPS sessions | Benchmark with ze-perf | unvalidated |
| A-4 | linux-l2tp maintainers would accept a new attribute for priority marking | P bit is in the RFC but never exposed by the kernel; this is a legitimate gap | Patch rejected; fall back to Option C | Submit RFC patch to netdev | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Kernel patch takes months to land upstream | Review cycle on netdev list | Use Option C (BPF) as interim; feature-detect attribute availability at runtime |
| R-2 | PPP protocol field compression (RFC 1661 S5) changes the offset | Compressed protocol is 1 byte for values 0x00-0xFF; LCP=0xC021 is never compressed | Check ppp_generic output format |
| R-3 | Per-packet PPP inspection measurably impacts throughput | ze-perf benchmark shows >1% regression | Make the inspection conditional on the per-session flag (only enabled for sessions that need CQM) |

## Data Flow (MANDATORY)

### Entry Point
- Kernel ppp_generic sends PPP frames to l2tp_ppp for encapsulation
- l2tp_ppp builds L2TP data header (currently always P=0)

### Transformation Path
1. PPP frame with protocol field (2 bytes) at sk_buff head
2. l2tp_ppp calls l2tp_xmit_skb in l2tp_core.c
3. l2tp_core builds L2TP data header: flags, tunnel/session IDs, optional Ns/Nr
4. Change: if session->priority_lcp and PPP protocol == 0xC021, set P bit in flags
5. Encapsulated packet sent via tunnel UDP socket

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze -> Kernel | generic netlink L2TP_ATTR_PRIORITY_LCP at session create | [ ] |
| Kernel PPP -> Kernel L2TP | sk_buff passed from ppp_generic to l2tp_ppp | [ ] |

### Integration Points
- `genl_linux.go` genlCreateSession - add attribute
- `config.go` Parameters - add PriorityLCPEcho field
- YANG schema - add priority-lcp-echo leaf

### Architectural Verification
- [ ] No bypassed layers (attribute flows through genl to session struct to xmit)
- [ ] No unintended coupling (flag is per-session, not global)
- [ ] No duplicated functionality (extends existing session attributes)
- [ ] Zero-copy preserved (flag check only, no buffer copy)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG priority-lcp-echo leaf | -> | genl session create with L2TP_ATTR_PRIORITY_LCP | TestGenlCreateSessionPriorityLCP |

Note: wiring test requires kernel patch to be available. Feature-detect
allows graceful degradation when the attribute is unsupported.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Session created with priority-lcp-echo=true on patched kernel | L2TP_ATTR_PRIORITY_LCP=1 sent in genl session create |
| AC-2 | Session created with priority-lcp-echo=true on unpatched kernel | genl returns EINVAL/ENOMSG; ze logs warning and continues without priority |
| AC-3 | PPP LCP Echo-Request sent on priority-enabled session (kernel) | L2TP data header has P=1 |
| AC-4 | PPP LCP Echo-Reply sent on priority-enabled session (kernel) | L2TP data header has P=1 |
| AC-5 | Non-LCP PPP frame sent on priority-enabled session (kernel) | L2TP data header has P=0 (only LCP is marked) |
| AC-6 | Any PPP frame on priority-disabled session (kernel) | L2TP data header has P=0 (default) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestGenlCreateSessionPriorityLCP | internal/component/l2tp/genl_linux_test.go | AC-1: attribute included in genl message | |
| TestGenlCreateSessionPriorityLCPFeatureDetect | internal/component/l2tp/genl_linux_test.go | AC-2: graceful fallback on EINVAL | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| priority-lcp-echo | boolean (0/1) | 1 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Deferred | - | Requires patched kernel in test environment | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Deferred | - | Requires patched kernel + L2TP peer | P=1 accepted by peer | |

### Future (if deferring any tests)
- Functional and interop tests deferred until kernel patch is available
- Kernel-side tests would be in the kernel tree (net/l2tp/ selftests)

## Files to Modify

Ze integration (after kernel patch):
- `internal/component/l2tp/genl_linux.go` - add l2tpAttrPriorityLCP constant, include in session create
- `internal/component/l2tp/config.go` - add PriorityLCPEcho parameter
- `internal/component/l2tp/kernel_linux.go` - pass attribute in session setup, handle EINVAL fallback
- `internal/component/l2tp/yang/` - add priority-lcp-echo leaf

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] Yes | `internal/component/l2tp/yang/` |
| YANG validation constraints | [x] Yes | boolean leaf, type boolean |
| Doctor check for kernel support | [x] Yes | probe kernel for attribute support at startup |
| Prometheus counters/metrics | [ ] No | CQM metrics already exist |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - L2TP P-bit priority |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - priority-lcp-echo leaf |
| 7 | Wire format changed? | No (P bit already defined; kernel sets it) | - |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2661.md` - P bit now used for LCP echo |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/l2tp.md` - update kernel P-bit section |

## Files to Create

- Kernel patch (out-of-tree, not in ze repo)
- `internal/component/l2tp/genl_linux_test.go` additions for priority attribute

## Implementation Steps

### Implementation Phases

1. **Phase: Kernel patch** -- write and submit patch to netdev mailing list
   - Files: kernel net/l2tp/ (out of tree)
   - Verify: kernel selftests pass with new attribute

2. **Phase: Ze genl integration** -- add attribute constant and session create logic
   - Tests: TestGenlCreateSessionPriorityLCP
   - Files: genl_linux.go, kernel_linux.go
   - Verify: attribute sent in genl message

3. **Phase: Feature detection** -- graceful fallback on unpatched kernels
   - Tests: TestGenlCreateSessionPriorityLCPFeatureDetect
   - Files: kernel_linux.go
   - Verify: session created successfully without priority on old kernels

4. **Phase: Config + YANG** -- expose as configurable per-session option
   - Tests: config parse test
   - Files: config.go, yang/
   - Verify: YANG leaf parsed and passed through to genl

5. **Phase: Doctor check** -- probe kernel support at startup
   - Tests: doctor check unit test
   - Files: health.go or equivalent

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Attribute value matches kernel header definition |
| Feature detect | Unpatched kernel does not break session setup |
| Data flow | Config -> Parameters -> genl attribute -> kernel session struct -> xmit path |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| l2tpAttrPriorityLCP constant | grep genl_linux.go |
| Feature-detect fallback | test with mocked EINVAL response |
| YANG leaf | grep yang/ for priority-lcp-echo |
| Doctor check | ze doctor reports kernel support status |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | PriorityLCPEcho is boolean; no untrusted input |
| Kernel boundary | genl attribute is NLA_U8 with value 0 or 1; kernel validates |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Generic netlink attribute over setsockopt | setsockopt is simpler | genl is consistent with existing session attributes; works with `ip l2tp` tool; more discoverable |
| Generic netlink attribute over BPF hook | BPF needs no kernel patch | BPF adds complexity, runtime deps, and fragile offset calculations; kernel-native is cleaner |
| Per-session flag over per-tunnel | Per-tunnel marks ALL sessions | Not all sessions need CQM; per-session gives operator control |
| Mark all LCP over Echo-only | Mark only Echo-Request/Reply (code 9/10) | Code-level inspection adds complexity for negligible gain; LCP negotiation is one-time, Echo recurs |

## Known Limitations

- Kernel patch requires upstream coordination with linux-l2tp maintainers
- PPP protocol field compression (RFC 1661 S5) is not a concern for LCP
  (0xC021 > 0xFF, so it is never compressed), but a future extension to
  mark other protocol types would need to handle compression
- The xmit-path check adds a branch per packet; mitigated by per-session
  flag (only sessions with CQM enabled pay the cost)
