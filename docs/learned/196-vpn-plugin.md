# 196 — VPN Plugin

## Objective

Create VPN (IPv4/IPv6 VPN) family plugins by moving VPN-specific types out of `nlri` and into the plugin package.

## Decisions

- `RouteDistinguisher` stays in `nlri` — shared by 6+ families (EVPN, VPN, BGP-LS, FlowSpec VPN, etc.); moving it to VPN plugin would force those families to import VPN.
- Only `IPVPN` struct moves to the plugin; consumers of `RouteDistinguisher` are unaffected.

## Patterns

- Same ownership rule as EVPN: shared types live in `nlri`, family-specific types live in the plugin.

## Gotchas

- None.

## Files

- `internal/plugins/vpn/` — IPVPN type, decode/encode
- `internal/bgp/nlri/` — RouteDistinguisher remains
