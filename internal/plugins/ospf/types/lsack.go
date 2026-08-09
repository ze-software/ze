// Design: docs/architecture/ospf/ospf-af-unify.md -- the LS Acknowledgment body is an AF-neutral list
// of LSA headers, so it lives in the shared types leaf and both wire codecs (ospf/packet,
// ospfv3/packet) produce/consume it. Only the wire encode/decode is version-specific.

package types

// LSAck is the Link State Acknowledgment packet body: the LSA headers being acknowledged
// (RFC 2328 sec A.3.6 / RFC 5340 sec A.3.6).
type LSAck struct {
	Headers []LSAHeader
}

// EncodedLen returns the LS Ack body length. Each acknowledged header is LSAHeaderLen
// octets on the wire (20 in both OSPFv2 and OSPFv3).
func (a LSAck) EncodedLen() int { return len(a.Headers) * LSAHeaderLen }
