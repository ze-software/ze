# Spec: bgp-orf-prefix

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release: Outbound Route Filtering with the Address Prefix ORF-Type, sent and received. RFC 5291 and RFC 5292 are listed by no release profile, so no summary is written or enrolled for them until Thomas brings them into scope (`ai/META-DECISIONS.md`). |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

## Task

With Outbound Route Filtering (ORF), a BGP speaker pushes its inbound prefix
filter to its peer inside a ROUTE-REFRESH message, and the peer applies it as an
outbound filter. The routes the speaker would drop are never sent, which saves
the peer's update generation and the link for a customer with a large upstream
feed and a small accepted set.

Ze neither advertises the ORF capability nor reads ORF entries. A ROUTE-REFRESH
that carries them is parsed as a plain refresh and the entries are ignored.

### VyOS reference

VyOS `protocols bgp neighbor <n> address-family <af> capability orf
prefix-list send|receive`; T9360 (vyos-1x `f6b3ee397`) renders FRR's
`orf prefix-list both` when both are set. FRR: `bgp_route_refresh_receive`
(receive side, `ORF_TYPE_PREFIX`) and `bgp_capability_send` in
`bgpd/bgp_packet.c`. BIRD implements no ORF, so FRR is the interop peer.

### RFC text read (`rfc/full/rfc5291.txt`, `rfc/full/rfc5292.txt`)

- RFC 5291 Section 6: "A BGP speaker that implements the Outbound Route Filtering Capability MUST support the BGP ROUTE-REFRESH message, as defined in [BGP-RR]."
- RFC 5291 Section 6: "If, on the other hand, for a given AFI/SAFI the intersection between these two sets is empty, the speaker MUST follow normal BGP procedures."
- RFC 5291 Section 6: "The speaker MUST re-advertise all the routes that have been affected by the ORF entries carried in the message, but MAY also re-advertise the routes that have not been affected by the ORF entries carried in the message."
- RFC 5291 Section 6: "ORF entries with either REMOVE or REMOVE-ALL cannot remove locally configured outbound route filters."
- RFC 5291 Section 6: "After a BGP speaker makes changes to the ORF entries previously sent to a peer, the speaker MUST send to the peer the updated ORF entries with either (a) When-to-refresh set to IMMEDIATE, or (b) When-to-refresh set to DEFER followed by a plain ROUTE-REFRESH message."
- RFC 5291 Section 4 defines the ORF entry encoding inside ROUTE-REFRESH, Section 5 the capability.
- RFC 5292 Section 2: "The value of the ORF-Type for the Address Prefix ORF-Type is 64." Section 4 defines the matching rule.

## Required Reading

- [ ] `rfc/full/rfc5291.txt`, `rfc/full/rfc5292.txt` in full
- [ ] `docs/architecture/api/capability-contract.md` - how a capability plugin injects into OPEN
- [ ] `docs/guide/bgp-policy.md` - per-peer export chains, where a received ORF becomes a member
- [ ] `rfc/short/rfc2918.md`, `rfc/short/rfc7313.md` - the refresh this rides on

## Current Behavior

- [ ] `internal/component/bgp/message/routerefresh.go` - `UnpackRouteRefresh` reads AFI, subtype and SAFI from the first 4 octets and ignores the rest of the body, so ORF entries are dropped in silence.
- [ ] `internal/component/bgp/plugins/route_refresh/register.go` - `bgp-route-refresh` declares capability codes 2 and 70 only; nothing declares code 3 (ORF).

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Route refresh plugin and message codec | `route_refresh` plugin, `message.RouteRefresh` | Capabilities 2 and 70; refresh request and BoRR/EoRR | Extends: capability 3 and the ORF entry list after the 4-octet header |
| Capability injection | `rpc.CapabilityDecl`, `Registration.CapabilityCodes` | A plugin declares and decodes its capability | Reused for the ORF capability per AFI/SAFI |
| Per-peer export chain and declared insertions | reactor policy chain, `rpc.FilterInsertion` | Runs export members on each route | Extends: a received prefix ORF is a peer-scoped export member placed after the locally configured members, which it can never remove (Section 6) |
| Prefix-list match type | `filter_prefix` plugin | Matches prefixes with ge/le | Reused: the send side derives ORF entries from the peer's import prefix-list; the receive side's matching follows RFC 5292 Section 4 |
| Re-advertisement on refresh | bgp-rib Adj-RIB-Out replay on ROUTE-REFRESH | Re-sends the family on a plain refresh | Extends: IMMEDIATE re-sends after the ORF changes; DEFER waits for the next refresh |

## Data Flow

### Entry Point
Config per peer and family: ORF prefix send, receive, or both. Received ROUTE-REFRESH carrying ORF entries.

### Transformation Path
1. OPEN carries capability 3 with the configured AFI/SAFI and ORF-Type 64 send/receive bits.
2. On receive: entries ADD, REMOVE, REMOVE-ALL update the per-peer ORF; IMMEDIATE re-sends the family through the export chain plus the ORF member.
3. On send: the import prefix-list becomes ORF entries in a ROUTE-REFRESH, and a change to that list sends updated entries.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire to plugin | ROUTE-REFRESH body to the route refresh plugin | No |
| Plugin to reactor | export member verdicts through `filter-update` | No |

### Integration Points
- OPEN capability negotiation; ROUTE-REFRESH decode; export chain; Adj-RIB-Out replay.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer config with ORF receive, peer sends ORF entries | → | ORF member in the peer's export chain | [functional `.ci` with a hex ROUTE-REFRESH, named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ORF receive configured | OPEN carries capability 3 for the family with ORF-Type 64 and the receive bit; hex asserted |
| AC-2 | Peer sends ROUTE-REFRESH with prefix ORF entries, IMMEDIATE | Ze re-sends the family; routes the ORF denies are withdrawn or never sent; hex and JSON asserted |
| AC-3 | DEFER, then a plain ROUTE-REFRESH | No re-send until the plain refresh, then one re-send with the ORF applied |
| AC-4 | REMOVE-ALL | The ORF is cleared; locally configured export members still apply |
| AC-5 | An ORF entry with an unrecognized field value | The whole ORF received for that family is removed (Section 6) |
| AC-6 | Capability intersection empty for a family | Normal BGP procedures, no deferral of the initial advertisement |
| AC-7 | ORF send configured with an import prefix-list | Ze sends the entries after session up and again when the list changes, with IMMEDIATE or DEFER plus refresh |
| AC-8 | Session reset | The received ORF is forgotten (lifetime is the session) |
| AC-9 | Interop: Ze and FRR with `orf prefix-list both` | Each side honors the other's ORF; proven red with the ORF member reverted |

## Risks & Assumptions

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | A peer floods ORF entries | A configured per-peer entry limit, refused beyond it with a NOTIFICATION or a log line the design chooses from RFC 5291 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | `internal/component/bgp/message/routerefresh_test.go` | ORF entry encode and decode, RFC 5291 Section 4 layout | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named at design] | `test/encode/`, `test/decode/` | Capability and ROUTE-REFRESH bytes | |

## Files to Modify

- `internal/component/bgp/message/routerefresh.go`
- `internal/component/bgp/plugins/route_refresh/` (capability, YANG, receive handling)
- `docs/guide/bgp-policy.md`, `docs/architecture/wire/` route refresh page

## Implementation Steps

1. Tests first for the wire layout.
2. Capability, receive side, export member, then send side.
3. FRR interop scenario.

### Integration Checklist
- [ ] Ze still interoperates with peers that do not advertise ORF

### Documentation Update Checklist
- [ ] Guide section for ORF

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le test functional encode`, `./le test functional decode`, `./le test integration interop`
