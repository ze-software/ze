# 063 — AFI/SAFI Map-Based Refactor

## Objective

Eliminate data duplication between NegotiatedFamilies (25 bool fields) and EncodingContext (maps), consolidate the Family type to the nlri package, and separate "what was negotiated" from "how to encode".

## Decisions

- `nlri.Family` is the canonical type; `capability.Family` becomes a type alias (`type Family = nlri.Family`) for backward compatibility
- `NegotiatedCapabilities` (replaces NegotiatedFamilies) uses `map[nlri.Family]bool` — O(1) lookup, no field explosion when adding new families
- `NegotiatedCapabilities` holds only: which families are enabled + ExtendedMessage bool; everything else (ASN4, AddPath, ExtNH) lives exclusively in EncodingContext
- `ExtendedNextHop` changed from `map[Family]bool` to `map[Family]AFI` — stores the next-hop AFI (e.g., AFIIPv6 for IPv4 prefix with IPv6 NH), richer information
- `Families()` returns sorted slice via `FamilyLess()` comparator — deterministic ordering for EOR sending and test reproducibility

## Patterns

- Three separate packages (nlri, capability, context) each had their own Family type; consolidating to one source-of-truth prevents drift
- Adding a new BGP family now requires: one constant in nlri package, no field additions anywhere else

## Gotchas

- ASN4, AddPath, ExtNH were stored in BOTH NegotiatedFamilies and EncodingContext — any fix to one left the other stale; this was a reliability hazard

## Files

- `internal/bgp/nlri/nlri.go` — canonical Family type, pre-computed constants, FamilyLess()
- `internal/bgp/capability/capability.go` — Family alias
- `internal/bgp/context/context.go` — EncodingContext with nlri.Family keys
- `internal/reactor/peer.go` — NegotiatedCapabilities replaces NegotiatedFamilies
