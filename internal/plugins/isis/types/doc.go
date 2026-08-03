// Package types defines the pure IS-IS domain value types: SystemID, SourceID,
// LSPID, NET, AreaID, the two wide metric widths (Metric and PrefixMetric),
// SequenceNumber, RemainingLifetime and HoldingTime.
//
// The types come from ISO/IEC 10589 (the consolidated normative reference for
// IS-IS) section 1.4 (definitions) and section 6.2 (addressing model), and from
// RFC 5305 (wide metrics, TLV 22 / TLV 135) and RFC 5308 (IPv6 reachability,
// TLV 236).
//
// Each type provides:
//   - parse from a printable string (Parse*)
//   - parse from wire bytes (*FromBytes)
//   - format for display (String, allocation-light per ai/rules/performance.md)
//   - equality and, where semantically meaningful (LSPID, AreaID), ordering
//   - buffer-first byte serialization (WriteTo(buf, off) int) per
//     ai/rules/performance.md
//
// LEAF PACKAGE CONSTRAINT: this package is the bottom layer of the IS-IS
// component (types <- packet codec <- server runtime). It MUST import only the
// Go standard library plus Ze leaf helpers (internal/core/textbuf). It MUST NOT
// import anything from the IS-IS runtime (packet, transport, circuit,
// adjacency, lsdb, spf, the component root) nor from BGP-LS. The only
// correctness risk here is parse/format/compare/serialize round-trip fidelity,
// which is fully unit-testable without any network I/O.
package types
