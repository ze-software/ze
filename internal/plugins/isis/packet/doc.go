// Package packet is the IS-IS PDU and TLV wire codec: the protocol's
// serialization boundary. It parses received frames into PDU views and
// serializes PDU structs back to bytes.
//
// Layering (umbrella plan/spec-isis-0-umbrella.md): types (leaf) <- packet
// <- the IS-IS runtime (transport, circuit, adjacency, lsdb, spf). This
// package imports ONLY the domain types from internal/plugins/isis/types
// (plus the Go standard library and internal/core/textbuf for display). It
// contains no runtime, sockets, timers, LSDB, or FSM; those live in later
// children. It MUST NOT import the runtime, nor BGP-LS.
//
// Decode is lazy and zero-copy (ISO/IEC 10589 clause 7.3.14 unknown-TLV
// propagation): a PDU view holds the caller's byte slice plus offsets, and
// TLVs are iterated on demand via TLVIterator yielding (type, value-slice)
// without copying. Unknown TLVs are retained as opaque spans so the LSDB can
// re-flood them verbatim. The lifetime contract: a decoded view is valid only
// while the caller's backing slice is stable; isis-6 copies LSP bytes it
// retains.
//
// Encode is buffer-first (ai/rules/performance.md): every PDU and TLV writes
// into a caller-owned buffer via WriteTo(buf []byte, off int) int. The PDU
// Length field and the LSP Fletcher checksum are written by skip-and-backfill,
// never a Len()-then-WriteTo() double traversal. Human-readable rendering uses
// textbuf/AppendTo, never fmt.Sprintf (ai/rules/performance.md).
//
// The single highest-risk item is the ISO 8473 Fletcher checksum with its
// two-step adjustment (checksum.go): it is implemented and vector-tested
// before any runtime depends on this package.
package packet
