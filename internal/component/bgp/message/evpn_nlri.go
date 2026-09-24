// Design: docs/architecture/wire/nlri-evpn.md -- received Ethernet A-D wire format
// RFC: rfc/short/rfc7432.md -- Section 7.1; rfc/short/rfc7606.md -- Sections 3 and 5.3
package message

// ValidEVPNNLRILengths checks framing and the fixed Ethernet A-D payload length.
// Type 1 has one three-octet label field, not a variable MPLS label stack.
// Unknown types are handled separately by the RFC 7606 Section 5.4 filter.
func ValidEVPNNLRILengths(nlris []byte, addPath bool) bool {
	for len(nlris) != 0 {
		if addPath {
			if len(nlris) < 4 {
				return false
			}
			nlris = nlris[4:]
		}
		if len(nlris) < 2 {
			return false
		}
		length := int(nlris[1])
		if length+2 > len(nlris) {
			return false
		}
		if nlris[0] == 1 {
			if length != 25 {
				return false
			}
		}
		nlris = nlris[2+length:]
	}
	return true
}
