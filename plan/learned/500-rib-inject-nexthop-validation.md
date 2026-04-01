# 500 -- RIB Inject Next-Hop Validation

## Context

The `rib inject` command hardcoded IPv4-only next-hop validation, rejecting all IPv6 next-hops. This was wrong for two reasons: injected routes for unknown peers (no session) should accept any valid IP since there are no capabilities to check, and real peers with RFC 8950 extended-nexthop negotiated should accept IPv6 next-hops for the negotiated families.

## Decisions

- **Capability-aware fallback** (over always-reject or always-accept): unknown peers accept any valid IP with a warning log. Known peers check ExtendedNextHopFor(family) from their EncodingContext.
- **Store ContextID in PeerMeta** (over secondary index in Registry): rib_structured.go already reads SourceCtxID from WireUpdate. Adding a field to PeerMeta is one line. The context registry is keyed by ID, not peer address -- adding a reverse index would be a larger change for no benefit.
- **Stale ContextID is not a bug**: the registry never removes entries by design. Old IDs remain valid. Peers reconnecting with different capabilities get new IDs. Investigated and confirmed safe.

## Consequences

- IPv6 next-hops now work for injected routes (unknown peers) and real peers with RFC 8950.
- PeerMeta has a new ContextID field populated from structured events. JSON-path events don't set it (ContextID stays 0, treated as unknown).
- The validation is fully programmatic via map lookup -- no hardcoded family switch. Adding new families to ExtendedNextHop capability automatically enables IPv6 nhop for those families.

## Gotchas

- `attribute.Builder.SetNextHop` only handles IPv4 (type code 3). Accepted IPv6 next-hops are not stored in the wire attributes. For RIB injection use cases (graph visualization, testing) this is acceptable -- the route exists in the RIB without a wire next-hop.
- `bgpctx.NewEncodingContext(nil, nil, DirectionRecv)` creates a valid context with no capabilities -- useful for tests.

## Files

- `internal/component/bgp/plugins/rib/rib.go` -- ContextID field in PeerMeta, bgpctx import
- `internal/component/bgp/plugins/rib/rib_commands.go` -- validateIPv6NextHop method, updated nhop block
- `internal/component/bgp/plugins/rib/rib_structured.go` -- store ContextID from SourceCtxID
- `internal/component/bgp/plugins/rib/rib_test.go` -- 3 new tests (unknown peer, no capability, context without ExtNH)
