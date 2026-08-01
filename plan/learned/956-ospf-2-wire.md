# 956 -- OSPFv2 wire codec: mirror IS-IS shape, not IS-IS semantics

## Context

`spec-ospf-2-wire.md` added the pure OSPFv2 packet and LSA codec in
`internal/plugins/ospf/packet`, plus an offline `ze ospf-decode` command and
`test/ospf-wire` fixtures. The implementation intentionally follows the package
shape of `internal/plugins/isis/packet`: common header, per-body codecs,
buffer-first writes, lazy decoded views, JSON for offline diagnostics, and fuzz
coverage.

## Decisions

- **Keep packet decode as the only public entry point for the CLI.**
  `ze ospf-decode` calls `packet.DecodePacket` and renders `Packet.ToJSON`.
  Runtime children can consume more granular exported APIs as they are wired in.
- **Use buffer-first skip and backfill.** `Packet.WriteTo` and `LSA.WriteTo`
  write headers with zero length/checksum fields, then backfill length and the
  correct checksum after body serialization.
- **Apply checksum ranges in the packet layer, own algorithms in types.** Packet
  checksum uses `InternetChecksumPair(packet[:16], packet[24:])`; LSA checksum
  uses `FletcherChecksum(lsa[2:])`.
- **Do not reinterpret LS IDs in the codec.** Network-LSA Link State ID remains
  the DR interface address from the common LSA header. SPF interprets it later.
- **Decoded opaque LSAs are raw first.** Type 9/10/11 LSAs decoded from the wire
  leave `RawBytes` authoritative so re-flood passthrough is byte-for-byte. A
  constructed opaque LSA can still use `Opaque` body data and recompute checksum.
- **Bound counts before allocation.** LS Update count is untrusted input. It is
  checked against the maximum possible number of 20-byte LSA headers before any
  `[]LSA` allocation.
- **Use a public capture for decode evidence.** The `ospf-lsupdate-frr.ci` file
  uses Wireshark's `ospf-ls-update-with-41-lsas.pcap` payload. The filename
  follows the spec, but the source issue does not identify the daemon as FRR.

## Consequences

- Future runtime code should not parse packet bytes itself. It should call this
  codec, then enforce instance-level area, checksum/auth, and neighbor-state
  policy in later OSPF packages.
- Exported packet APIs will trip `make ze-validate` until transport, instance,
  flooding, and SPF children consume them. Do not add artificial callers or
  deprecated aliases just to silence the check.
- Fuzz and review found two durable bug classes for OSPF parsers: untrusted count
  allocation and raw passthrough accidentally disabled by setting a typed body.
  New OSPFv3 codecs should test both from the start.

## Files

None recorded.
