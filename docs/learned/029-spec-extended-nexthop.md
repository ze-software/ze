# 029 — Extended Next-Hop (RFC 8950)

## Objective

Implement RFC 8950 extended next-hop encoding so IPv4 NLRI can be sent with IPv6 next-hops via MP_REACH_NLRI when the capability is negotiated.

## Decisions

- Extended next-hop is stored in `Negotiated` as `extendedNextHop map[extendedNHKey]AFI` where the key is `(nlriAFI, nlriSAFI)` — mirrors ExaBGP's tuple approach.
- When encoding: if the prefix is IPv4 and next-hop is IPv6 and extended-NH is negotiated for that family, use `MP_REACH_NLRI` with AFI=1 instead of the traditional NEXT_HOP attribute + inline NLRI.

## Patterns

- Capability negotiation lives in `capability/negotiated.go`; the `Negotiate()` function was extended to process `*ExtendedNextHop` capability objects.
- Route builder checks `NegotiatedFamilies.IPv4UnicastExtNH` before choosing encoding path.

## Gotchas

- The original bug was that `buildStaticRouteUpdate` always used inline IPv4 NLRI for IPv4 prefixes, ignoring any extended next-hop negotiation.
- MP_REACH_NLRI for RFC 8950 uses AFI=1, SAFI=1 (NOT AFI=2), with next-hop length 16 (IPv6).

## Files

- `internal/bgp/capability/negotiated.go` — added ExtendedNextHop negotiation
- `internal/reactor/peer.go` — `NegotiatedFamilies.IPv4UnicastExtNH`, `buildMPReachNLRIExtNHUnicast`
