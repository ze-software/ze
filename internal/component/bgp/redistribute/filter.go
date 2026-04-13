// Design: docs/architecture/core-design.md -- redistribute ingress filter
// Related: bgp.go -- BGP source registration

package redistribute

import (
	"encoding/binary"

	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// IngressFilter is the redistribution ingress filter registered in the plugin registry.
// It checks whether the UPDATE's family and source (ibgp/ebgp) are accepted by the
// redistribution import rules. When no redistribution is configured, all routes pass.
func IngressFilter(src registry.PeerFilterInfo, payload []byte, _ map[string]any) (bool, []byte) {
	ev := redistribute.Global()
	if ev == nil {
		return true, nil // No redistribution configured: accept all.
	}

	fam := familyFromPayload(payload)
	source := "ebgp"
	if src.LocalAS == src.PeerAS {
		source = "ibgp"
	}

	route := redistribute.RedistRoute{
		Origin: "bgp",
		Family: fam,
		Source: source,
	}

	// Intra-BGP: importing protocol is empty to skip loop prevention.
	// Loop prevention applies when importing across protocols (e.g., OSPF -> BGP),
	// not when filtering within BGP (ibgp/ebgp source + family selection).
	return ev.Accept(route, ""), nil
}

// familyFromPayload extracts the address family from an UPDATE body.
// If MP_REACH_NLRI (attr 14) is present, returns its AFI/SAFI.
// Otherwise returns ipv4/unicast (RFC 4271 default).
func familyFromPayload(payload []byte) string {
	// Skip withdrawn routes length (2 bytes) + withdrawn routes.
	if len(payload) < 4 {
		return family.IPv4Unicast.String()
	}
	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrStart := 2 + wdLen
	if attrStart+2 > len(payload) {
		return family.IPv4Unicast.String()
	}
	attrTotalLen := int(binary.BigEndian.Uint16(payload[attrStart : attrStart+2]))
	attrs := payload[attrStart+2:]
	if len(attrs) < attrTotalLen {
		return family.IPv4Unicast.String()
	}
	attrs = attrs[:attrTotalLen]

	// Walk attributes looking for MP_REACH_NLRI (type code 14).
	const mpReachCode = 14
	off := 0
	for off+2 < len(attrs) {
		flags := attrs[off]
		code := attrs[off+1]

		var attrLen int
		var hdrLen int
		if flags&0x10 != 0 { // Extended length
			if off+4 > len(attrs) {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(attrs[off+2 : off+4]))
			hdrLen = 4
		} else {
			if off+3 > len(attrs) {
				break
			}
			attrLen = int(attrs[off+2])
			hdrLen = 3
		}
		if off+hdrLen+attrLen > len(attrs) {
			break
		}

		if code == mpReachCode && attrLen >= 3 {
			valStart := off + hdrLen
			afi := family.AFI(binary.BigEndian.Uint16(attrs[valStart : valStart+2]))
			safi := family.SAFI(attrs[valStart+2])
			return family.Family{AFI: afi, SAFI: safi}.String()
		}

		off += hdrLen + attrLen
	}

	return family.IPv4Unicast.String()
}
