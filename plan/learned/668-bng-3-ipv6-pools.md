# Learned: spec-bng-3 -- IPv6 Address Pools and DHCPv6-PD

## What was built

IPv6 prefix delegation for BNG PPP sessions, in 4 phases:

1. **IPv6 prefix pool** (`l2tppool/pool_v6.go`): bitmap-backed pool allocating /N prefixes from a configured range, with PrefixHandler/PrefixReleaser interfaces and AuthMetadata IPv6 fields.
2. **RA builder** (`ppp/ra.go`): Router Advertisement with M+O flags and optional RDNSS.
3. **DHCPv6 codec** (`ppp/dhcpv6.go`): parse/build for Solicit/Advertise/Request/Reply/Renew/Release with IA_PD.
4. **IPv6Service + wiring** (`ppp/ipv6_service.go`, `*_linux.go`): per-session state machine orchestrating RA sender + DHCPv6-PD server, with route install/remove, prefix pool release callback, configurable lifetimes.

## Key decisions

- **Per-session DHCPv6 server, not relay.** BNG owns the prefix pool and assigns directly. Each pppN interface gets its own UDP :547 socket via SO_BINDTODEVICE + SO_REUSEADDR.
- **RA with M+O flags, no prefix info.** Subscriber learns to use DHCPv6 for addressing. RA carries no PIO (prefix information option).
- **DUID-LL from localInterfaceID.** Server DUID is derived from the IPv6CP-negotiated local interface identifier, unique per session.
- **Prefix release callback.** IPv6Service doesn't own the pool; it calls a `ReleasePrefix` callback on Release and Stop, so the pool plugin manages its own lifecycle.

## What surprised us

- **nil allocPrefix panic.** The initial wiring passed nil as the prefix allocator (pool not wired yet). HandleSolicit called it unconditionally. Caught in review, fixed with nil guard returning NoPrefixAvail.
- **Port 547 bind conflict.** Multiple sessions each binding [::]:547 with SO_BINDTODEVICE requires SO_REUSEADDR; without it, only the first session gets DHCPv6.
- **DUID slice aliasing.** `s.localInterfaceID[:]` in the DUID struct shares memory with the session field. Fixed with explicit copy.
- **unparam lint on test helpers.** Test builder functions with symmetric signatures (buildSolicit, buildRenew, buildRelease) flagged for always receiving iaid=1.

## Remaining integration work

- AC-5/AC-6: RADIUS-supplied prefix and named IPv6 pool (runtime integration)
- AC-12: DHCPv6 Information-Request (DNS-only, no PD)
- Functional tests: require full L2TP stack (`test/l2tp/ipv6-pd-*.ci`)
- Documentation updates: features.md, configuration.md, plugins.md

## Files

None recorded.
