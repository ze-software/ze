// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 packet and LSA wire codec.

// Package packet is the OSPFv3 (RFC 5340) packet and LSA wire codec: the
// protocol's serialization boundary. It parses received OSPFv3 payloads (raw
// IPv6 proto-89 datagrams, after the IPv6 header has been stripped) into packet
// and LSA views, and serializes packet/LSA structs back to bytes.
//
// Layering (plan/spec-ospfv3-0-umbrella.md): types (leaf) <- packet <- the
// OSPFv3 runtime (transport, interface ISM, neighbor NSM, LSDB, SPF). This
// package imports only internal/plugins/ospf/v3/types plus the Go standard
// library; it shares NO code with the OSPFv2 codec. The OSPFv3 wire contract
// differs enough that sharing would leak the divergences:
//
//   - a 16-octet common header with an Instance ID and no AuType/Authentication
//     field (the OSPFv2 24-octet header has neither),
//   - an IPv6 upper-layer (pseudo-header) packet checksum that binds the IPv6
//     source and destination, not the OSPFv2 over-the-packet Internet checksum,
//   - a 24-bit Options field,
//   - a 20-octet LSA header with a 16-bit LS Type and no Options byte,
//   - address-free Router-LSAs and Network-LSAs (IPv6 addresses live in the
//     Link-LSA and Intra-Area-Prefix-LSA),
//   - and the RFC 5340 IPv6 prefix encoding.
//
// Decode is zero-copy: decoded packets and LSAs retain slices owned by the
// caller, and LSA body parsing is available on demand. A decoded view is valid
// only while the caller's backing slice remains stable; LSDB code copies
// retained LSA bytes before storing.
//
// Encode is buffer-first: WriteTo(buf, off) methods write into a caller-provided
// buffer, skip the Packet Length / Checksum and LSA Length / Checksum fields,
// and backfill them last. Unknown LS Types retain and re-emit their raw spans
// verbatim so flooding never re-marshals a received LSA.
package packet
