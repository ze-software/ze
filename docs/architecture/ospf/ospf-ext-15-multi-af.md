# OSPFv3 multiple address families

RFC 5838 maps an address family to an OSPFv3 Instance-ID range: IPv6 unicast
0 to 31, IPv6 multicast 32 to 63, IPv4 unicast 64 to 95, IPv4 multicast 96 to
127. Ze spawns one unified-engine instance per configured address family, each
with its own LSDB, topology and route table. The RFC 5340 codec is not
re-opened. See `docs/architecture/wire/ospfv3.md` for the wire view.

## Decisions

- **An engine SET owns one engine per address family**, replacing the single v6
  engine. Each engine gets its own metrics and config at spawn, deduplicated by
  name under an address-family label.
  <!-- source: internal/plugins/ospf/register_multiaf.go -- v6EngineSet -->
  <!-- source: internal/plugins/ospf/multiaf.go -- addressFamily -->
- **The engine carries its address family and a multi-AF flag.** The engine
  constructor takes the codec AND the address family.
- **Each address family's routes are installed into the matching RIB family**,
  and the installed-routes metric gains an address-family label.
  <!-- source: internal/plugins/ospf/spf/install.go -- NewInstallerFamily -->
- **The AF-bit is set in Hello and Database Description only (RFC 5838 Section
  2.4), and it is CHECKED in both.** A Hello-only gate lets a peer that sets the
  bit in Hello and not in DD reach Full on a non-default address family. The
  default family, IPv6 unicast, does not require the bit, which preserves
  backward compatibility.
  <!-- source: internal/plugins/ospf/v3/types/options.go -- Options -->

## Constraints on callers

- A per-address-family feature attaches to the engine SET, never to a single v6
  engine. IPsec had to hook its installer into the per-engine lifecycle for this
  reason.
- Prefix decoding and the forwarding-address decode are address-family aware: 4
  bytes for an IPv4 family, 16 for an IPv6 family.

## Traps

- **An unconditionally wired IPv4-over-OSPFv3 injector silently DROPS OSPFv2
  redistribution.** With the injector always returning the v4-over-v3 wrapper,
  the lookup never falls back to the OSPFv2 engine, so connected, BGP and static
  routes reached no Type-5 LSA when no IPv4-unicast family was configured. The
  wrapper reports whether its address-family engine exists, and the lookup falls
  back when it does not. The check is evaluated per injection, so it stays
  correct across a runtime family add or remove.
  <!-- source: internal/plugins/ospf/redistribute/consumer.go -- injectorFor -->
