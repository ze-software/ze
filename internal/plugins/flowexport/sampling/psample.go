// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- psample constants and parser

package sampling

import "github.com/mdlayher/netlink"

// psample netlink attribute types (kernel include/uapi/linux/psample.h).
const (
	psampleAttrIIfIndex   = 1
	psampleAttrOrigSize   = 3
	psampleAttrSampleRate = 6
	psampleAttrData       = 7
)

// parsePsampleMessage extracts a SampledPacket from the raw netlink
// attribute bytes of a psample genetlink message.
func parsePsampleMessage(data []byte) (SampledPacket, error) {
	ad, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return SampledPacket{}, err
	}

	var pkt SampledPacket
	for ad.Next() {
		switch ad.Type() {
		case psampleAttrIIfIndex:
			pkt.IfIndex = ad.Uint32()
		case psampleAttrOrigSize:
			pkt.OrigSize = ad.Uint32()
		case psampleAttrSampleRate:
			pkt.Rate = ad.Uint32()
		case psampleAttrData:
			raw := ad.Bytes()
			pkt.Header = make([]byte, len(raw))
			copy(pkt.Header, raw)
		}
	}

	return pkt, ad.Err()
}
