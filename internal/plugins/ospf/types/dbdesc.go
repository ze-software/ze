// Design: plan/spec-ospf-af-unify.md -- the Database Description body carries the same
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
}
