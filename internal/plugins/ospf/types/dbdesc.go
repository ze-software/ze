// Design: docs/architecture/ospf/ospf-af-unify.md -- the Database Description body carries the same
// logical fields in OSPFv2 and OSPFv3, so the struct is shared via the types leaf. The
// wire fixed-layout differs between versions (RFC 2328 A.3.3 vs RFC 5340 A.3.3) and is
// encoded/decoded by the codec.

package types

// DBDesc is the Database Description packet body. Options is 32-bit to hold both the
// OSPFv2 8-bit and OSPFv3 24-bit Options widths.
type DBDesc struct {
	InterfaceMTU uint16
	Options      Options
	Flags        uint8
	DDSequence   uint32
	Headers      []LSAHeader
	// AFBit carries the OSPFv3 AF-bit (RFC 5838 §2.4), which the neutral 8-bit Options
	// superset cannot represent. OSPFv3-only: the v6 codec sets it from the received
	// 24-bit Options; the OSPFv2 codec leaves it false. The engine's AF-bit adjacency
	// gate (§2.5/§2.6) reads it for non-default address families, mirroring Hello.AFBit.
	AFBit bool
}
