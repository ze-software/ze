# Spec: RSVP routing and interface integration (RFC 2205 Sections 2.8, 3.8, 3.9, 3.10, 3.11)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Required correctness of shipped configuration, routing or forwarding behavior; existing partial implementations and remaining proof are retained. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-09 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Own the shipped RSVP routing/interface behavior and all required repairs: RFC2205-2-11/2-12, 3-6, 3-21/3-23, the implemented unicast portion of 3-33, and 3-37/3-38/3-39/3-40/3-42. Preserve the already implemented native transport and RSVP_HOP handling, and require the outstanding native forwarding proofs. The explicit 2026-10-08 defect decision is unchanged. General multicast/interface selection alone has the separate absent-feature owner.

## Acceptance Criteria

The original criterion ids are retained below for this file’s slice only. Cross-cutting criteria are partitioned by the ownership table; the companion owns the other portion, not a duplicate implementation.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Every release-owned requirement named in Task | Its required routing/interface behavior is implemented and proven by the native forwarding scenarios; existing partial carriage is preserved, not counted as full proof. |

## Implementation Steps

Complete and prove the source Task’s required defect fixes and implemented carriage through native forwarding scenarios. Do not replace them with multicast feature work.

## Work Not Done

The release-scope and spec-split rules in `ai/META-DECISIONS.md` approve this scope separation, not completion or additional product implementation. Original dependencies, design decisions and evidence requirements remain binding within each slice.

| Original obligation | Implementation owner | Boundary |
|---------------------|----------------------|----------|
| RFC2205-2-11/2-12, 3-6/3-21/3-23, unicast 3-33 and 3-37/3-38/3-39/3-40/3-42 | `plan/release/fixes/spec-rsvp-routing-and-interface-integration.md` | Shipped routing/interface repairs and native proof |
| RFC2205-3-33: general multicast/interface-selection extension | `plan/post-release/features/spec-rsvp-multicast-interface-selection.md` | Absent extension, with original authorization limit retained |

## Files to Modify

- `internal/plugins/rsvpte/engine.go` - the original listed producer for routing/interface defects and existing RSVP_HOP handling.
- `internal/plugins/rsvpte/routing.go` - release-owned route resolution named by the source Task.
- `internal/plugins/rsvpte/transport_linux.go` - existing native routing, interface metadata, Router Alert carriage and PATH transmission named by the source Task.

This list preserves the source authorization limits and outstanding native proof; it does not authorize the separate general multicast/interface-selection extension.

## Owned journal rows
<!-- Every open journal row this spec will fix, one line each; `./le commit create`
     refuses an open row no spec lists (plan/journal/README.md, "Owned rows"). -->
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `green-that-could-not-have-been-red.md` | 2026-10-30 | `TestIntegrationRSVPPathCarriage` cannot tell the IP TTL from Send_TTL (RFC2205-3-40) |

## Historical source record

The original design, Task, criteria, implementation plan, decisions and dated evidence follow intact apart from heading depth. They are history and design detail, not a second current implementation owner. The current Task, Acceptance Criteria and ownership table above select which slice this file owns. Recorded completion stays recorded completion; outstanding work is not discharged by relocation.

### Task

The started Linux transport now queries native routes and interface addresses, receives Router Alert traffic with source, destination, interface and TTL metadata, and sends PATH/PathTear with separate protocol identities and next-hop selection. `engine.go::handlePacket` derives the previous RSVP hop from RSVP_HOP rather than the end-to-end IP source. These source changes still require Main's native forwarding scenarios. The table distinguishes implemented carriage from remaining protocol gaps. RFC2205-2-12, 3-6 and 3-21 are unmet MUSTs in RSVP that Ze implements, so under `ai/rules/rfc-compliance.md` they are defects this spec fixes, not gaps moved elsewhere. The table does not by itself authorize implementing the other listed requirements in this completion phase.

-> Decision (owner, 2026-10-08): RFC2205-2-12, 3-6 and 3-21 are defects in implemented RSVP that this spec fixes, not gaps moved elsewhere (`ai/rules/rfc-compliance.md`).

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC2205-2-11 | "RSVP must therefore provide correct protocol operation even when two RSVP-capable routers are joined by an arbitrary \"cloud\" of non-RSVP routers." (§2) | `engine.go::handlePacket` uses RSVP_HOP for the previous signaling hop; arbitrary non-RSVP-cloud operation remains unverified and non-RSVP-hop detection remains backlog |
| RFC2205-2-12 | "If the destination address does not match any local interface and the message is not a Path or PathTear, the message must be forwarded without further processing by this node." (§2) | Defect this spec fixes (owner, 2026-10-08): `engine.go::handlePacket` has no general non-local-message forwarding branch |
| RFC2205-3-6 | "RSVP must not forward (according to the rules of Section 3.9) Path messages that arrive on an incoming interface different from that provided by routing." (§3) | Defect this spec fixes (owner, 2026-10-08): `Packet.IfIndex` is available, but the engine does not compare it with the expected incoming route |
| RFC2205-3-21 | "However, this must not trigger sending a message out the interface through which M arrived (which could happen if the implementation simply triggered an immediate refresh of all state for the session)." (§3) | Defect this spec fixes (owner, 2026-10-08): the general session/interface loop-prevention rule is not established by the unicast next-hop path |
| RFC2205-3-23 | "Forwarding of RSVP messages must avoid looping." (§3) | Transit PATH processing bounds Send_TTL; general interface-based loop prevention remains a separate gap |
| RFC2205-3-33 | "The RSVP process must determine which case holds by examining the path state, to decide which incoming interface to use for sending Resv messages." (§3) | `engine.go::sendResv` selects the previous hop from path state; `transport_linux.go::Send` queries its native output route. General multicast/interface selection is not claimed |
| RFC2205-3-37 | "To forward Path and PathTear messages, an RSVP process must be able to query the routing process(s) for routes." (§3) | `routing.go::resolveExplicitPath` and `resolveImplicitPath` call `Transport.ResolveRoute`; `transport_linux.go::ResolveRoute` queries the native FIB |
| RFC2205-3-38 | "RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses." (§3) | `transport_linux.go::LocalAddresses` reads native addresses, link state and MTU; the engine refreshes this view before signaling |
| RFC2205-3-39 | "Packets received for IP protocol 46 but not addressed to the node must be diverted to the RSVP program for processing, without being forwarded." (§3) | `transport_linux.go::openSockets` registers a wildcard protocol-46 socket with IP_ROUTER_ALERT; native transit diversion must be exercised |
| RFC2205-3-40 | "On a router or multi-homed host, the identity of the interface (real or virtual) on which a diverted message is received, as well as the IP source address and IP TTL with which it arrived, must also be available to the RSVP process." (§3) | `transport_linux.go::receivedIPv4Packet` decodes source, destination, TTL and IP_PKTINFO into `Packet`; missing interface metadata is rejected |
| RFC2205-3-42 | "RSVP must be able to specify the IP source address and IP TTL to be used when sending Path messages." (§3) | `transport_linux.go::SendPath` writes source and Send_TTL into the IPv4 header and pins the selected output interface |

<!-- The sections below were added on 2026-10-08 so the spec passes the native
     spec validator when the owner decision above was recorded. Their bodies
     are owed by the next implementation pass on this spec, from
     plan/templates/TEMPLATE.md; nothing in them is a design claim yet. -->

### Required Reading

#### Architecture Docs
- [ ] `docs/architecture/rsvpte/mpls-rsvp-te.md` - the design page the RSVP-TE code declares

#### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2205.md` - the rows in the requirement table above

### Current Behavior (MANDATORY)

**Source files read:** (named by the requirement table above; read before the next edit)
- [ ] `internal/plugins/rsvpte/engine.go` - `handlePacket`, `sendResv`
- [ ] `internal/plugins/rsvpte/routing.go` - `resolveExplicitPath`, `resolveImplicitPath`

### Data Flow (MANDATORY)

#### Entry Point
- Not yet written: owed by the next implementation pass on this spec.

#### Transformation Path
1. Not yet written.

#### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel → engine | Router Alert protocol-46 socket, `Packet` with interface metadata | No |

#### Integration Points
- Not yet written.

### Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Not yet written | → | `handlePacket` forwarding and incoming-interface checks | Not yet written |

### 🧪 TDD Test Plan

#### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Not yet written | | RFC2205-2-12, 3-6, 3-21 | |

### Files to Modify
- `internal/plugins/rsvpte/engine.go` - the three defects above

### Implementation Steps
1. Not yet written.

### Checklist

#### Goal Gates (MUST pass)
- [ ] `./le verify worktree` passes

#### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
