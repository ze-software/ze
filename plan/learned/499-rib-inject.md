# 499 -- RIB Inject/Withdraw

## Context

The looking glass graph visualization needs routes in the RIB to render AS path topologies. Without a live BGP session, there was no way to populate the RIB for testing or demonstration. The `update text` command sends UPDATEs to peers (outbound), not into the local RIB (inbound).

## Decisions

- **Direct RIB insertion** (over event fabrication): `rib inject` calls `peerRIB.Insert()` directly under `r.mu.Lock()`, same as `handleReceivedStructured` after wire parsing. Simpler, same end state.
- **Peer address as label** (over requiring live session): the peer address in `ribInPool` is just a map key. Injected routes use any address string, no TCP session needed.
- **attribute.Builder for wire encoding** (over manual byte construction): the existing Builder produces correct wire-format attribute bytes from text parameters.

## Consequences

- Routes injected via `rib inject` are indistinguishable from routes received via BGP. They appear in `rib show`, prefix-summary, graph, and CSV download.
- The peer address used for injection doesn't need to match any configured peer. This is intentional for testing.
- Only simple prefix families (IPv4/IPv6 unicast/multicast) are supported via `prefixToWire`. VPN, EVPN, etc. would need their own NLRI encoders.

## Gotchas

- `attribute.Origin` is a named type, not plain `uint8`. Must cast with `uint8(attribute.OriginIGP)` when passing to `Builder.SetOrigin()`.
- `PeerRIB` has `FamilyLen()` not `Count()` for per-family route counts.

## Files

- `internal/component/bgp/plugins/rib/rib_commands.go` -- injectRoute, withdrawRoute, parseASNList
- `internal/component/bgp/plugins/rib/rib.go` -- command registration
- `internal/component/bgp/plugins/rib/rib_test.go` -- 10 tests
- `internal/component/bgp/plugins/rib/protocol_test.go` -- updated command count
- `docs/guide/command-reference.md` -- documented syntax
