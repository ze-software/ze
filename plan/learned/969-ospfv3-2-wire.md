# 969 - OSPFv3 packet + LSA codec (spec-ospfv3-2)

## Context

Second implementation child of the OSPFv3 (RFC 5340) umbrella. Created
`internal/plugins/ospfv3/packet/` -- the wire codec that every later OSPFv3 child
(transport, ISM/NSM, LSDB, SPF, ABR, ASBR, NSSA, auth, CLI) frames bytes through.
It encodes/decodes the 16-byte common header, the 5 packet types, the 20-byte LSA
header, the 8 base LSA bodies, the IPv6 prefix encoding, and the two checksums,
consuming the `ospfv3/types` leaf package and sharing no code with the OSPFv2
codec.

## Decisions

- **A SEPARATE codec, mirroring OSPFv2's buffer-first conventions, not sharing its
  code.** `WriteTo(buf, off) int` + `EncodedLen()`, lazy `RawBytes` retention with
  byte-for-byte re-flood, opaque passthrough of unknown LS Types, length-validated
  decode, typed sentinel errors (`ErrShortBuffer`/`ErrTruncated`/`ErrLength`/...).
  The OSPFv3 wire contract differs enough (16-byte header, no AuType, IPv6
  pseudo-header packet checksum, 24-bit Options, address-free Router/Network LSAs,
  IPv6 prefix encoding) that sharing would leak version detail.
- **The packet checksum takes the IPv6 src/dst.** OSPFv3 uses the IPv6 upper-layer
  checksum over the pseudo-header (src16 + dst16 + len32 + zero24 + nextHeader=89)
  plus the packet, so `PacketChecksum(src, dst, pkt)` and `VerifyPacketChecksum`
  take the addresses transport supplies; `Packet.WriteTo` leaves the checksum field
  zero (the codec cannot know the addresses) and transport finalizes it. This also
  cleanly covers the RFC 7166 auth-trailer case (header checksum omitted) with no
  API change. The LSA Fletcher checksum is byte-identical to OSPFv2 (over
  `lsa[2:length]`, non-zero), re-implemented here per the no-cross-version rule.
- **One shared prefix helper for both carriage forms.** Inter-Area-Prefix /
  AS-External / NSSA inline `PrefixLength + PrefixOptions + 16-bit field` then the
  address; Link / Intra-Area-Prefix use the repeating 4-octet-header entry. The
  16-bit field is a Metric (Intra-Area-Prefix), Reserved (Link), or the Referenced
  LS Type (AS-External). `types.PrefixLength.ByteLen/ValidatePadding` enforce the
  RFC 5340 padded-word rule and zero padding past the prefix length.

## Gotchas

- **A round-trip test of a wire-WRONG-but-self-consistent encoding passes green
  (review BLOCKER).** The first implementation laid the AS-External E/F/T flags at
  body offset 6 and truncated the Referenced LS Type to 8 bits at offset 7. RFC
  5340 §A.4.7 actually puts the flags in byte 0 (sharing the 32-bit word with the
  24-bit Metric) and the Referenced LS Type in a 16-bit field at offset 6. Every
  `*RoundTrip` test passed because encode and decode agreed on the wrong layout --
  the spec table AND the research digest had the same error, so nothing caught it
  until an independent review compared against the RFC. The decisive tell: an
  8-bit field cannot hold a 16-bit referenced type like `0x2003`. Fix: flags into
  byte 0, `ReferencedLSType` widened to `types.LSType` (16-bit) at offset 6, and a
  hardcoded **golden vector** (`TestOSPFv3ExternalGoldenWire`) that locks the exact
  RFC bytes. **Lesson: every wire codec needs at least one golden/hardcoded-bytes
  vector per structure, not only round-trips; round-trips prove encode==decode, not
  encode==RFC.** Added golden vectors for the common header, a prefix, and the
  AS-External body.
- **The Fletcher-255 checksum treats 0x00 and 0xff as equivalent** (255 mod 255 ==
  0). A "flip a body byte" tamper test that happens to land on a 0x00 (or 0xff)
  byte does NOT invalidate the checksum -- correctly, but it makes the test flaky on
  byte choice. Scan for a byte that is neither before flipping.
- **The external body's inlined 16-bit field IS the Referenced LS Type**, not a
  per-prefix `Field16`; lift it into the top-level `ReferencedLSType` and zero the
  stored `Prefix.Field16` so the embedded prefix compares equal to a freshly-built
  one.

## Verification anchors

- `go test -race ./internal/plugins/ospfv3/...` clean (packet + types); `go vet` 0.
  Golden vectors: `TestOSPFv3CommonHeaderGolden`, `TestOSPFv3PrefixGolden`,
  `TestOSPFv3ExternalGoldenWire`. Round-trips per packet/LSA; bounds/truncation
  (`TestOSPFv3ExternalTruncated`, iterator + count guards); checksums
  (`TestOSPFv3LSAChecksum` with header + deep-body tamper, `TestOSPFv3PacketChecksum`
  with wrong src/dst). Leaf import guard `TestOSPFv3PacketNoRuntimeImports`.
- All 19 ACs mapped to passing tests; `make ze-tier-check` 0 (leaf isolation).
- **Still unvalidated (A-2/A-5, deferred to interop):** an FRR `ospf6d`-captured
  checksum and AS-External vector. The layout now follows RFC 5340 §A.4.7 and is
  golden-locked, but a real capture is the final interop proof when ospfv3-3/-5/-6
  land.
- Next OSPFv3 target (umbrella): `spec-ospfv3-3-ipv6-transport.md` (raw IPv6
  proto-89 sockets, multicast membership, link-local source, receive loop) --
  Linux-specific, needs QEMU integration tests.

## Files

None recorded.
