// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 Link State Request packet body codec.
// RFC: rfc/short/rfc5340.md (§A.3.4 Link State Request packet)

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/v3/types"

// lsReqEntryLen is the 12-octet request entry. RFC 5340 §A.3.4 carries the
// genuinely 16-bit LS Type in the low half of a 32-bit slot: the leading two
// octets are reserved.
const lsReqEntryLen = 12

// LSReq entry field offsets (RFC 5340 §A.3.4).
const (
	lsReqTypeOff      = 2
	lsReqLSIDOff      = 4
	lsReqAdvRouterOff = 8
)

// LSRequestEntry is one Link State Request triple. RFC 5340 identifies a
// requested LSA by (LS Type, Link State ID, Advertising Router); the request
// carries no age, sequence, or checksum.
type LSRequestEntry struct {
	Type              types.LSType
	LinkStateID       types.LinkStateID
	AdvertisingRouter types.RouterID
}

// LSReq is the OSPFv3 Link State Request packet body.
type LSReq struct {
	Requests []LSRequestEntry
}

// DecodeLSReq parses a Link State Request body. Entries must align to 12 octets.
// Unknown LS Types are retained verbatim: a peer may request an LSA whose
// function code this router does not implement, and the request is still valid.
func DecodeLSReq(body []byte) (LSReq, error) {
	if len(body)%lsReqEntryLen != 0 {
		return LSReq{}, ErrLength
	}
	out := LSReq{Requests: make([]LSRequestEntry, 0, len(body)/lsReqEntryLen)}
	off := 0
	for off < len(body) {
		lt, err := types.LSTypeFromBytes(body, off+lsReqTypeOff)
		if err != nil {
			return LSReq{}, err
		}
		lsid, err := types.LinkStateIDFromBytes(body[off+lsReqLSIDOff : off+lsReqLSIDOff+types.IDLen])
		if err != nil {
			return LSReq{}, err
		}
		adv, err := types.RouterIDFromBytes(body[off+lsReqAdvRouterOff : off+lsReqAdvRouterOff+types.IDLen])
		if err != nil {
			return LSReq{}, err
		}
		out.Requests = append(out.Requests, LSRequestEntry{Type: lt, LinkStateID: lsid, AdvertisingRouter: adv})
		off += lsReqEntryLen
	}
	return out, nil
}

// EncodedLen returns the LS Request body length.
func (r LSReq) EncodedLen() int { return len(r.Requests) * lsReqEntryLen }

// WriteTo serializes the LS Request body into buf at off. The two leading
// reserved octets of every entry are written zero (RFC 5340 §A.3.4).
func (r LSReq) WriteTo(buf []byte, off int) int {
	for _, req := range r.Requests {
		buf[off] = 0
		buf[off+1] = 0
		off += 2
		off += req.Type.WriteTo(buf, off)
		off += req.LinkStateID.WriteTo(buf, off)
		off += req.AdvertisingRouter.WriteTo(buf, off)
	}
	return off
}
