// Design: plan/learned/956-ospf-2-wire.md -- Link State Request packet body codec
// RFC 2328 Appendix A.3.4: Link State Request packet.

package packet

import "github.com/ze-software/ze/internal/plugins/ospf/types"

// LSRequestEntry and LSReq are shared via the types leaf (one type across the engine,
// neighbor FSM, and both wire codecs); only the wire encode/decode below is v2-specific.
type LSRequestEntry = types.LSRequestEntry

// LSReq is the Link State Request packet body.
type LSReq = types.LSReq

// DecodeLSReq parses a Link State Request body.
func DecodeLSReq(body []byte) (LSReq, error) {
	if len(body)%types.LSRequestEntryLen != 0 {
		return LSReq{}, ErrLength
	}
	out := LSReq{Requests: make([]LSRequestEntry, 0, len(body)/types.LSRequestEntryLen)}
	off := 0
	for off < len(body) {
		rawType := readUint32(body, off)
		if rawType > 0xff {
			return LSReq{}, ErrUnknownLSAType
		}
		lt := types.LSType(rawType)
		if !lt.Known() {
			return LSReq{}, ErrUnknownLSAType
		}
		lsid, err := types.LinkStateIDFromBytes(body[off+4 : off+8])
		if err != nil {
			return LSReq{}, err
		}
		adv, err := types.RouterIDFromBytes(body[off+8 : off+12])
		if err != nil {
			return LSReq{}, err
		}
		out.Requests = append(out.Requests, LSRequestEntry{Type: lt, LinkStateID: lsid, AdvertisingRouter: adv})
		off += types.LSRequestEntryLen
	}
	return out, nil
}

// writeLSReq serializes the OSPFv2 LS Request body into buf at off (codec-layer encode).
func writeLSReq(r LSReq, buf []byte, off int) int {
	for _, req := range r.Requests {
		off += writeUint32(buf, off, uint32(req.Type))
		off += req.LinkStateID.WriteTo(buf, off)
		off += req.AdvertisingRouter.WriteTo(buf, off)
	}
	return off
}
