# OSPFv2 packet and LSA codec

`internal/plugins/ospf/packet` is the OSPFv2 serialization boundary. The byte
layouts are in `docs/architecture/wire/ospf.md`. This file holds the decisions
behind them.

## Decisions

- **The codec follows the package shape of `internal/plugins/isis/packet`, not
  its semantics.** Common header, per-body codecs, buffer-first writes, lazy
  decoded views, JSON for offline diagnostics, fuzz coverage. No IS-IS code is
  imported.
- **Write is skip-and-backfill.** `Packet.WriteTo` and `LSA.WriteTo` write the
  header with zero length and zero checksum, then backfill the length and the
  checksum after the body is serialized.
  <!-- source: internal/plugins/ospf/packet/header.go -- Packet, WriteTo -->
  <!-- source: internal/plugins/ospf/packet/lsa.go -- LSA, WriteTo -->
- **The packet layer owns checksum RANGES, the types package owns the
  ALGORITHMS.** The packet checksum is `InternetChecksumPair(packet[:16],
  packet[24:])`. The LSA checksum is `FletcherChecksum(lsa[2:])`.
  <!-- source: internal/plugins/ospf/packet/checksum.go -- PacketChecksum, FinalizeLSAChecksum -->
- **The codec never reinterprets a Link State ID.** The Network-LSA Link State
  ID stays the DR interface address from the common LSA header. SPF interprets
  it later.
  <!-- source: internal/plugins/ospf/packet/lsa_network.go -- DecodeNetworkLSA -->
- **A decoded opaque LSA is raw first.** Types 9, 10 and 11 decoded from the
  wire keep `RawBytes` authoritative, so a re-flood is byte-for-byte. A
  constructed opaque LSA can carry `Opaque` body data and recompute the
  checksum.
  <!-- source: internal/plugins/ospf/packet/lsa_opaque.go -- OpaqueTypeOf, OpaqueIDOf -->
- **Counts are bounded before allocation.** The LS Update count is untrusted
  input. It is checked against the maximum possible number of 20-byte LSA
  headers before any `[]LSA` is allocated.
  <!-- source: internal/plugins/ospf/packet/lsupdate.go -- DecodeLSUpdate -->

## Traps

- **Two durable bug classes for OSPF parsers**, both found by fuzzing and
  review: allocation driven by an untrusted count, and raw passthrough disabled
  by accident when a typed body is set. Test both in any new OSPF codec.
- Runtime code must not parse packet bytes itself. It calls this codec, then
  applies area, checksum, authentication and neighbor-state policy in the
  runtime packages.
