// Design: docs/architecture/ospf/ospf-af-unify.md -- the LS Request body is an AF-neutral list of
// (type, link-state-id, advertising-router) triples, so it lives in the shared types leaf
// and both wire codecs produce/consume it. Only the wire encode/decode is version-specific.

package types

// LSRequestEntryLen is the on-wire width of one LS Request entry: 12 octets in both
// OSPFv2 (4-octet LS type) and OSPFv3 (2 reserved + 2-octet LS type).
const LSRequestEntryLen = 12

// LSRequestEntry is one Link State Request triple (RFC 2328 sec A.3.4 / RFC 5340).
type LSRequestEntry struct {
	Type              LSType
	LinkStateID       LinkStateID
	AdvertisingRouter RouterID
}

// LSReq is the Link State Request packet body.
type LSReq struct {
	Requests []LSRequestEntry
}

// EncodedLen returns the LS Request body length.
func (r LSReq) EncodedLen() int { return len(r.Requests) * LSRequestEntryLen }
