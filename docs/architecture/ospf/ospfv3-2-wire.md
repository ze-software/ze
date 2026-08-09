# OSPFv3 packet and LSA codec

`internal/plugins/ospf/v3/packet` encodes and decodes the 16-byte common header,
the five packet types, the 20-byte LSA header, the base LSA bodies, the IPv6
prefix encoding and the two checksums. The byte layouts are in
`docs/architecture/wire/ospfv3.md`.

## Decisions

- **A separate codec that mirrors the OSPFv2 conventions and shares none of its
  code.** Buffer-first `WriteTo(buf, off) int` with `EncodedLen()`, lazy raw-byte
  retention for byte-for-byte re-flood, opaque passthrough of an unknown LS Type,
  length-validated decode and typed sentinel errors. The OSPFv3 wire contract
  differs in the 16-byte header, the absent AuType, the IPv6 pseudo-header
  checksum, the 24-bit Options, the address-free Router and Network LSAs and the
  prefix encoding. Sharing would leak version detail.
  <!-- source: internal/plugins/ospf/v3/packet/header.go -- Packet, WriteTo -->
  <!-- source: internal/plugins/ospf/v3/packet/lsa.go -- LSA -->
- **The packet checksum takes the IPv6 source and destination.** OSPFv3 uses the
  IPv6 upper-layer checksum over the pseudo-header plus the packet, so the
  checksum function takes the addresses that the transport supplies.
  `Packet.WriteTo` leaves the checksum field zero, because the codec does not
  know the addresses, and the transport finalizes it. This also covers the RFC
  7166 authentication-trailer case with no API change. The LSA Fletcher checksum
  is byte-identical to OSPFv2 and is re-implemented here under the
  no-cross-version rule.
  <!-- source: internal/plugins/ospf/v3/packet/checksum.go -- PacketChecksum, VerifyPacketChecksum, FinalizePacketChecksum -->
- **One shared prefix helper covers both carriage forms.** Inter-Area-Prefix,
  AS-External and NSSA carry the prefix inline as PrefixLength, PrefixOptions
  and a 16-bit field, then the address. Link and Intra-Area-Prefix use the
  repeating 4-octet-header entry. The 16-bit field is a Metric, a Reserved
  field, or the Referenced LS Type, depending on the body.
  <!-- source: internal/plugins/ospf/v3/packet/prefix.go -- Prefix, decodePrefix, decodeInlinePrefix -->

## Traps

- **A round-trip test of a wire-wrong but self-consistent encoding passes
  green.** The first AS-External implementation put the E, F and T flags at body
  offset 6 and truncated the Referenced LS Type to 8 bits at offset 7. RFC 5340
  Appendix A.4.7 puts the flags in byte 0, sharing the 32-bit word with the
  24-bit metric, and the Referenced LS Type in a 16-bit field at offset 6. Every
  round-trip test passed because encode and decode agreed on the wrong layout.
  The decisive tell was that an 8-bit field cannot hold a 16-bit referenced type
  such as `0x2003`. **Every wire codec carries at least one golden hardcoded-byte
  vector per structure. A round-trip proves encode equals decode, never encode
  equals RFC.**
  <!-- source: internal/plugins/ospf/v3/packet/lsa_external.go -- ExternalLSA -->
- **The Fletcher-255 checksum treats 0x00 and 0xff as equivalent**, because 255
  mod 255 is 0. A tamper test that flips a byte which happens to be 0x00 or 0xff
  does not invalidate the checksum. Scan for a byte that is neither before
  flipping.
- **The external body's inline 16-bit field IS the Referenced LS Type**, not a
  per-prefix field. It is lifted to the top level, and the stored per-prefix
  field is zeroed so the embedded prefix compares equal to a freshly built one.
