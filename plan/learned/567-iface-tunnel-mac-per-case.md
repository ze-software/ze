# 567 -- iface-tunnel-mac-per-case

## Context

The YANG grouping split (learned/566) introduced `interface-common` and `interface-l2`,
but left `list tunnel` on `interface-l2` even though L3 tunnel cases (gre, ipip, sit,
ip6tnl, ipip6) have no MAC address. This meant the YANG schema advertised a
`mac-address` leaf on all 8 tunnel kinds, even though the kernel ignores it on L3
kinds. The spec moved `mac-address` into the two L2 case containers only (gretap,
ip6gretap), tightening the schema to match hardware reality.

## Decisions

- Switched `list tunnel` from `uses interface-l2` to `uses interface-common` (drops
  list-level mac-address).
- Added `leaf mac-address` with the same type/validator inside `container gretap` and
  `container ip6gretap` case blocks.
- `parseTunnelEntry` clears any list-level `MACAddress` (defense-in-depth, since YANG
  now rejects it), then reads from `matchedCase` for `IsBridgeable()` kinds only.
- Phase 2 `SetMACAddress` was already gated on non-empty `MACAddress`, so no change needed.
- Chose not to add a Go-level rejection for mac-address on L3 case containers; YANG
  schema enforcement is sufficient.

## Consequences

- L3 tunnel configs can no longer include a `mac-address` leaf (YANG rejects it).
  No existing configs used this, confirmed by grep of test/ and .ci files.
- `interface-l2` grouping description updated to remove the tunnel mention.
- Future tunnel kinds that need MAC (unlikely) add it to their case container,
  following the gretap pattern.

## Gotchas

- `parseIfaceEntry` still reads mac-address from the list-level map for non-tunnel
  kinds (ethernet, dummy, veth, bridge). The `entry.MACAddress = ""` clear in
  `parseTunnelEntry` is specific to tunnels and must not be applied generically.
- No existing .ci test or config sets mac-address on any tunnel kind, so the risk
  of breaking existing configs was zero.

## Files

- `internal/component/iface/schema/ze-iface-conf.yang` -- grouping + case changes
- `internal/component/iface/config.go` -- parseTunnelEntry mac-address handling
- `internal/component/iface/config_test.go` -- TestParseTunnelGretapMAC, TestParseTunnelGreNoMAC
- `docs/features/interfaces.md` -- L2 tunnel MAC note
