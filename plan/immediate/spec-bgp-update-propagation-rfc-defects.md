# Spec: bgp-update-propagation-rfc-defects

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Six places where Ze accepts or re-advertises a BGP route in a way its RFC or
draft forbids, found by the strict re-read of
`spec-rfc-requirement-quote-hand-backfill`. An operator meets
each as a wire behaviour towards a peer: bits that must be zero are sent,
duplicates that must not be sent are sent, a malformed MUP route is kept, a
received link-local next hop reaches a peer off the link, and
two propagation rules whose reading the owner must settle. Each row was read
at its producer in HEAD on 2026-09-27.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 4271 Section 4.3 (RFC4271-4.3-3, 4.3-4) | "The lower-order four bits of the Attribute Flags octet are unused. They MUST be zero when sent and MUST be ignored when received." | `internal/core/bgp/attribute/opaque.go::NewOpaqueAttribute` (flags kept verbatim), `internal/core/bgp/attribute/attribute.go::WriteHeaderTo` (writes flags unmasked) | An unrecognised attribute received with low-order flag bits set is re-advertised with those bits set | unverified by a wire test |
| D2 | RFC 8092 Section 3 (RFC8092-3-1) | "Duplicate BGP Large Community values MUST NOT be transmitted. A receiving speaker MUST silently remove redundant BGP Large Community values from a BGP Large Community attribute." | `internal/component/bgp/message/rfc7606.go::validateLargeCommunityAttr` (length only), `internal/component/bgp/plugins/rib/ribout_entry.go::parseLargeCommunityWire` | Received duplicates are neither removed on receipt nor filtered on the forward path that reuses the received bytes; only the `LargeCommunities` encoder dedups | weak; lead not yet traced to the wire |
| D3 | draft-ietf-bess-mup-safi Section 3.1.3.1 (rows 3.1.3.1-6, -7, -10) | "the mandatory fields exceed the declared Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw." and "any TLV parsing error MUST result in Treat-as-withdraw" | `internal/core/bgp/nlri/nlrisplit/mup.go::SplitMUP` with `RecognizeNLRI` at ingress | Ingress frames the NLRI on its length and stores it; `ParseMUP`'s truncation and TLV errors are reached only by the decoder, so a malformed ST1 route is kept instead of withdrawn | weak, code gap with no `{gap}` marker |
| D4 | RFC 9234 Section 3.1 (RFC9234-3.1-1) | "Customer: MAY propagate any route learned from a Customer, or that is locally originated, to a Provider. All other routes MUST NOT be propagated." | `internal/component/bgp/plugins/role/otc.go::OTCEgressFilter` | Suppresses sources whose local role is customer, peer or rs-client; a route learned from an RS-Client (local role rs) is propagated to a Provider, Peer or RS. `TestOTCEgressFilter` subtest `src_role_rs_to_provider_accept` pins the same reading | weak; OWNER DECISION |
| D5 | RFC 7999 Section 3.1 (RFC7999-3.1-2) | "In a bilateral peering relationship, use of the BLACKHOLE community MUST be agreed upon by the two networks before advertising it." | `internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go::agreedSelector`; `internal/component/bgp/plugins/filter_community/config.go` `blackholeGuardToken` (default none) | The agreement gate covers origination by command only; a received BLACKHOLE route is re-advertised to a peer that never agreed, since the Section 3.2 propagation guard is off by default | weak; OWNER DECISION |
| D6 | RFC 2545 Section 3; draft-ietf-idr-linklocal-capability-06 Section 4 item 1 (DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2) when capability 77 is negotiated | RFC 2545: "The link-local address shall be included in the Next Hop field if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to." Draft: "If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop." | `internal/component/bgp/reactor/peer_forward_facts.go::precomputeNextHop` (next-hop auto and unchanged map to `nhModeNone`) and `applyFactsNextHop` (`nhModeNone` returns with no attribute 14 op); `reactor_api_forward.go::applyNextHopMod` (auto: no op) | A received 32-octet Global plus Link-Local next hop is forwarded to an internal peer as received, whether or not that peer is on the link. The `peerOnLink` gate in `link_scope.go::linkLocalNextHop` covers only Ze's own configured link-local under next-hop self or explicit | weak (the 4-2 tags test next-hop self only); read at the producer, not reproduced |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | One test per row, red against HEAD, over the forward path (not the codec alone) |
| Both polarities | D1: set bits cleared on send, clear bits unchanged. D2: duplicates removed, distinct values kept in order. D3: each malformed shape withdrawn, a well-formed route with an unknown TLV kept and propagated unchanged |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Real entry point | `.ci` tests: a peer sends the UPDATE, a second peer receives the re-advertisement |
| D6 cases | a received Global plus Link-Local next hop forwarded under next-hop auto to a multihop internal peer carries the global address only (red against HEAD); to an internal peer on the same link it may keep the link-local. The multihop EBGP twin (4-8, "Link-Local IPv6 next hops MUST NOT be included.") goes through the eBGP next-hop rewrite, which was not traced: the same test covers it |
| Interop | D1, D2 against FRR or BIRD as the receiving peer; D4 against a Role-capable FRR; D6 with FRR as a multihop iBGP receiver |

## Owner decisions

| Row | Question |
|-----|----------|
| D4 | Does an RS-Client-learned route count as "learned from a Customer" for propagation to a Provider, Peer or RS? The RFC sentence says it does not; code and test say it does |
| D5 | Does "advertising it" in Section 3.1 cover re-advertising a received BLACKHOLE route, and should the Section 3.2 guard default on? |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md`, `rfc/short/rfc8092.md`, `rfc/short/rfc9234.md`, `rfc/short/rfc7999.md`, `rfc/drafts/draft-ietf-bess-mup-safi.txt` Section 3.1.3.1

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/bgp/attribute/opaque.go` - flags preserved
- [ ] `internal/core/bgp/attribute/attribute.go` - `WriteHeaderTo`
- [ ] `internal/component/bgp/message/rfc7606.go` - `validateLargeCommunityAttr`
- [ ] `internal/core/bgp/nlri/nlrisplit/mup.go` - `SplitMUP`
- [ ] `internal/component/bgp/plugins/role/otc.go` - `OTCEgressFilter`
- [ ] `internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go` - `agreedSelector`

**Behavior to change:** D1 to D3; D4 and D5 after the owner rules.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a BGP UPDATE received from a peer

### Transformation Path
1. RFC 7606 validation and NLRI split
2. RIB insert and best path
3. egress filters (OTC, community guard) and the forward path to other peers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ filter plugins | egress filter calls with wire payload | No |

### Integration Points
- the zero-copy forward path and the per-peer egress filter chain

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE from peer A, re-advertised to peer B | → | forward path and egress filters | [to fill in design: `.ci`] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | unknown transitive attribute with flag bits 0x0F set | forwarded with those bits zero |
| AC-2 | LARGE_COMMUNITY with a repeated value | forwarded once per value |
| AC-3 | MUP ST1 route with truncated mandatory fields or a bad TLV | treat-as-withdraw |
| AC-4 | D4, D5 | as the owner rules |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| one failing-first test per row | beside each producer | D1 to D5 | [to fill in design] |

### Functional Tests
- [to fill in design]

## Files to Modify

- the producers in the defect table and their tests

## Implementation Steps

1. Ask the owner D4 and D5; 2. failing tests; 3. fixes; 4. discrimination and verdicts.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
