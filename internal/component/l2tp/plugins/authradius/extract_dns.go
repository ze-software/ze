// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- MS-Primary-DNS-Server and MS-Secondary-DNS-Server
// Related: extract.go -- extractAuthMetadata calls extractMSDNSServers
// Related: extract_vsa.go -- the other vendor-specific attributes read from an Access-Accept

package l2tpauthradius

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/radius"
)

// msDNSServerOctets is the IP-Address field length of both MS DNS attributes.
const msDNSServerOctets = 4

// extractMSDNSServers reads the subscriber's resolvers from an Access-Accept.
//
// Wire format of each, inside a Vendor-Specific attribute (RFC 2865
// Section 5.26) whose Vendor-Id is 311 (Microsoft):
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Vendor-Type  | Vendor-Length |           IP-Address
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	        IP-Address (cont)       |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
//	offset 0: Vendor-Type (28 primary, 29 secondary)
//	offset 1: Vendor-Length, 6
//	offset 2: IP-Address, 4 octets
//
// The first well-formed attribute of each type wins. One whose IP-Address is
// not four octets carries no address, so it is skipped and the slot stays
// the zero Addr, which the pool reads as "RADIUS named no resolver here".
func extractMSDNSServers(pkt *radius.Packet) (primary, secondary netip.Addr) {
	for _, raw := range pkt.FindAllAttr(radius.AttrVendorSpecific) {
		vendorID, vendorType, value, err := radius.DecodeVSA(raw)
		if err != nil {
			continue
		}
		if vendorID != radius.VendorMicrosoft {
			continue
		}
		if len(value) != msDNSServerOctets {
			continue
		}
		addr := netip.AddrFrom4([4]byte(value))
		switch vendorType {
		case radius.MSPrimaryDNSServer:
			// RFC 2548 Section 2.7.6: "The MS-Primary-DNS-Server Attribute is used to indicate the
			// address of the primary Domain Name Server (DNS) [16, 17] server to
			// be used by the PPP peer."
			if !primary.IsValid() {
				primary = addr
			}
		case radius.MSSecondaryDNSServer:
			// RFC 2548 Section 2.7.7: "The MS-Secondary-DNS-Server Attribute is used to indicate the
			// address of the secondary DNS server to be used by the PPP peer."
			if !secondary.IsValid() {
				secondary = addr
			}
		}
	}
	return primary, secondary
}
