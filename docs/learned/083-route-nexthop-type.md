# 083 — Route NextHop Type

## Objective

Replace the dual-field pattern (`NextHop netip.Addr` + `NextHopSelf bool`) with a unified `RouteNextHop` type across all route-origination structs.

## Decisions

- Explicit addresses bypass validation in `resolveNextHop()`: explicit = intentional override, user is responsible for correctness. Returns the address as-is even if invalid.
- Resolution at peer level, not parser level: peer has negotiated capabilities (Extended NH, local address) needed to validate "self" policy; the parser cannot know these at parse time.
- `NextHopUnset` (zero value) is invalid: forces callers to explicitly set policy, prevents accidental zero-value use.
- Cross-family next-hop (e.g., IPv6 address for IPv4 NLRI) is valid when Extended NH capability negotiated — `canUseNextHopFor()` checks `sendCtx.ExtendedNextHopFor(family)`.

## Gotchas

- BGP session has ONE local address (IPv4 OR IPv6, not both) — `NextHopSelf` resolves to `settings.LocalAddress`, which may not match the NLRI family if Extended NH is not negotiated.
- Spec was created retrospectively to document a completed implementation.

## Files

- `internal/plugin/nexthop.go` — RouteNextHop, NextHopPolicy, constructors
- `internal/reactor/peer.go` — resolveNextHop(), canUseNextHopFor(), error vars
- `internal/plugin/types.go`, `internal/reactor/peersettings.go` — dual-field pattern removed
