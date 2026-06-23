// Package packet is the OSPFv2 packet and LSA wire codec: the protocol's
// serialization boundary. It parses received OSPF payloads (after the IP header
// has been stripped) into packet and LSA views, and serializes packet/LSA structs
// back to bytes.
//
// Layering (plan/spec-ospf-0-umbrella.md): types (leaf) <- packet <- the OSPF
// runtime (transport, interface FSM, neighbor FSM, LSDB, SPF). This package
// imports only internal/plugins/ospf/types plus the Go standard library. It
// contains no sockets, timers, goroutines, LSDB, or FSM.
//
// Decode is zero-copy: decoded packets and LSAs retain slices owned by the
// caller, and LSA body parsing is available on demand. The lifetime contract is
// the same as internal/plugins/isis/packet: a decoded view is valid only while
// the caller's backing slice remains stable; later LSDB code copies retained LSA
// bytes.
//
// Encode is buffer-first: WriteTo methods write into a caller-provided buffer,
// skip length/checksum fields, and backfill Packet Length, packet checksum, LSA
// Length, and LSA Fletcher checksum last. Unknown opaque LSAs retain and
// re-emit their raw spans verbatim.
package packet
