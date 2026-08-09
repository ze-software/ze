// Design: docs/architecture/ospf/ospf-af-unify.md -- the LSA common header is address-family-neutral
// data (all fields are shared types), so it lives in the types leaf and is shared by the
// engine, LSDB, neighbor FSM, SPF, and both wire codecs (ospf/packet, ospfv3/packet).
// Only the WIRE encode/decode is version-specific and stays in the codec packages.

package types

// LSAHeader is the common OSPF LSA header (RFC 2328 Appendix A.4.1: 20 bytes for
// OSPFv2; RFC 5340 Appendix A.4.2 for OSPFv3). Length includes this header. The fields
// are address-family-neutral; the LS Type width difference (OSPFv2 8-bit vs OSPFv3
// 16-bit scope-typed) is handled by the version-specific codec when it fills Type.
type LSAHeader struct {
	Age               LSAge
	Options           Options
	Type              LSType
	LinkStateID       LinkStateID
	AdvertisingRouter RouterID
	Sequence          LSSequenceNumber
	Checksum          uint16
	Length            uint16
}

// Key returns the LSDB identity tuple for this LSA header.
func (h LSAHeader) Key() LSAKey {
	return LSAKey{Type: h.Type, LinkStateID: h.LinkStateID, AdvertisingRouter: h.AdvertisingRouter}
}
