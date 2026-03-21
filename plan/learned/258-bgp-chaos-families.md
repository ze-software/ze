# 258 — BGP Chaos Families

## Objective

Phase 4 of ze-bgp-chaos: add multi-family route generation support (IPv6, VPN, EVPN, FlowSpec) alongside existing IPv4 unicast.

## Decisions

- `UpdateBuilder.BuildUnicast()` auto-detects IPv6 prefixes via address family — no separate IPv6 sender method needed.
- Validation model unchanged: `netip.Prefix` handles IPv4+IPv6 unicast; non-unicast families use count-based tracking only.
- `cmd/ze-bgp-chaos/` can import plugin packages directly — import restriction applies only to `internal/component/plugin/`, `internal/component/bgp/`, `internal/component/config/`, and `cmd/ze/`.
- Receive-side MP_REACH_NLRI parsing for non-unicast families left out of scope — chaos tool read loop parses IPv4 unicast NLRI only.
- Go 1.20+ `[6]byte(slice[2:])` slice-to-array conversion used directly for EVPN MAC bytes.

## Patterns

- Auto-detect IP version: check prefix address family at send time rather than branching by family type.
- Count-based validation for non-unicast: when prefix structure differs per family, track route counts rather than prefix equality.

## Gotchas

- `evpn.RDType` does not exist — it is `nlri.RDType`; the evpn package does not re-export it.
- Receive-side MP_REACH parsing for non-unicast families is out of scope for the chaos tool.

## Files

- `cmd/ze-bgp-chaos/scenario/routes_ipv6.go`, `routes_vpn.go`, `routes_evpn.go`, `routes_flowspec.go` — family-specific generators
- `cmd/ze-bgp-chaos/scenario/generator.go` — `assignFamilies()`, `buildFamilyPool()` with weighted probability per optional family
- `cmd/ze-bgp-chaos/scenario/config.go` — per-peer family blocks in Ze config
- `cmd/ze-bgp-chaos/peer/sender.go` — `BuildVPNRoute`, `BuildEVPNRoute`, `BuildFlowSpecRoute`, `BuildEOR(family)`
- `cmd/ze-bgp-chaos/peer/session.go` — multi-family Multiprotocol capabilities in OPEN
- `cmd/ze-bgp-chaos/main.go` — `--families` / `--exclude-families` wiring
