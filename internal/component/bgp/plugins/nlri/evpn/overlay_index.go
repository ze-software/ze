// Design: docs/architecture/wire/nlri-evpn.md -- explicit EVPN route origination
// Related: encode.go -- route-command keyword parser that selects the index
// Related: plugin.go -- NLRI-only encoder that selects the index
// RFC: rfc/short/rfc9136.md

package evpn

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// fieldOverlayIndex names the IP Prefix route keyword that selects the
// route's Overlay Index. It is an encode keyword only: a decoded route reports
// the ESI and gateway fields, from which RFC 9136 Section 3.2 derives the index.
const fieldOverlayIndex = "overlay-index"

// overlayIndex is the Overlay Index an originated IP Prefix route carries.
// The operator selects it; Ze checks the ESI and GW IP fields against it.
type overlayIndex uint8

const (
	// overlayIndexUnspecified means the route named no overlay-index keyword,
	// so it carries no Overlay Index: both fields are held to zero, as for
	// overlayIndexNone.
	overlayIndexUnspecified overlayIndex = iota
	// overlayIndexNone is RFC 9136 Table 1's "None": the BGP next hop is the
	// actual next hop and no recursive resolution happens.
	overlayIndexNone
	// overlayIndexESI makes the ESI field the Overlay Index.
	overlayIndexESI
	// overlayIndexGatewayIP makes the GW IP field the Overlay Index.
	overlayIndexGatewayIP
)

// Keyword values of the overlay-index route keyword.
const (
	overlayIndexNameNone      = "none"
	overlayIndexNameESI       = "esi"
	overlayIndexNameGatewayIP = "gateway-ip"
)

// parseOverlayIndex converts the overlay-index keyword value to its index.
func parseOverlayIndex(value string) (overlayIndex, error) {
	switch strings.ToLower(value) {
	case overlayIndexNameNone:
		return overlayIndexNone, nil
	case overlayIndexNameESI:
		return overlayIndexESI, nil
	case overlayIndexNameGatewayIP:
		return overlayIndexGatewayIP, nil
	}
	return overlayIndexUnspecified, fmt.Errorf("invalid overlay-index %q: use none, esi or gateway-ip", value)
}

// checkOverlayIndex checks the ESI and GW IP fields of an IP Prefix route
// against the Overlay Index the route was asked to carry, before any byte is
// written.
//
// RFC 9136 Section 3.1, IP Prefix route NLRI (IPv4 Length 34, IPv6 Length 58),
// octet offsets after the route type and Length octets:
//
//	+---------------------------------------+
//	|  RD (8 octets)                        |  0-7
//	+---------------------------------------+
//	|  Ethernet Segment Identifier (10)     |  8-17
//	+---------------------------------------+
//	|  Ethernet Tag ID (4 octets)           |  18-21
//	+---------------------------------------+
//	|  IP Prefix Length (1 octet)           |  22
//	+---------------------------------------+
//	|  IP Prefix (4 or 16 octets)           |  23-26 or 23-38
//	+---------------------------------------+
//	|  GW IP Address (4 or 16 octets)       |  27-30 or 39-54
//	+---------------------------------------+
//	|  MPLS Label (3 octets)                |  31-33 or 55-57
//	+---------------------------------------+
//
// RFC 9136 Section 3.1: "The Ethernet Segment Identifier MUST be a non-zero
// 10-octet identifier if the ESI is used as an Overlay Index (see the
// definition of "Overlay Index" in Section 3.2).  It MUST be all bytes zero
// otherwise."
//
// RFC 9136 Section 3.1: "The GW IP Address field is a 4- or 16-octet field
// (IPv4 or IPv6) and will encode a valid IP address as an Overlay Index for
// the IP prefixes.  The GW IP field MUST be all bytes zero if it is not used
// as an Overlay Index."
//
// RFC 9136 Section 3.2: "If either the ESI or the GW IP are non-zero, then the
// non-zero one is the Overlay Index". So a non-zero field is emitted only when
// the operator selected that field as the index.
//
// The label is not checked: RFC 9136 Section 3.1 says the label "SHOULD be
// zero if a recursive resolution based on an Overlay Index is used".
func checkOverlayIndex(index overlayIndex, esi [10]byte, gateway netip.Addr) error {
	esiSet := esi != [10]byte{}
	gatewaySet := gateway.IsValid() && !gateway.IsUnspecified()
	switch index {
	case overlayIndexUnspecified:
		if esiSet {
			return fmt.Errorf("ip-prefix esi %s is non-zero, which makes the ESI the route's Overlay Index "+
				"(RFC 9136 Section 3.2), but no overlay-index was named; "+
				"add overlay-index esi, or remove esi (RFC 9136 Section 3.1)", formatESIForJSON(ESI(esi)))
		}
		if gatewaySet {
			return fmt.Errorf("ip-prefix gateway %s is non-zero, which makes the GW IP the route's Overlay Index "+
				"(RFC 9136 Section 3.2), but no overlay-index was named; "+
				"add overlay-index gateway-ip, or remove gateway (RFC 9136 Section 3.1)", gateway)
		}
	case overlayIndexNone:
		if esiSet {
			return fmt.Errorf("overlay-index none needs an all-zero ESI, but esi is %s (RFC 9136 Section 3.1); "+
				"remove esi, or set overlay-index esi", formatESIForJSON(ESI(esi)))
		}
		if gatewaySet {
			return fmt.Errorf("overlay-index none needs an all-zero GW IP, but gateway is %s (RFC 9136 Section 3.1); "+
				"remove gateway, or set overlay-index gateway-ip", gateway)
		}
	case overlayIndexESI:
		if !esiSet {
			return errors.New("overlay-index esi needs a non-zero ESI, but esi is all zero (RFC 9136 Section 3.1); " +
				"set esi to the Ethernet Segment the prefix resolves through, or remove overlay-index")
		}
		if gatewaySet {
			return fmt.Errorf("overlay-index esi needs an all-zero GW IP, but gateway is %s (RFC 9136 Section 3.1); "+
				"remove gateway, or set overlay-index gateway-ip and remove esi", gateway)
		}
	case overlayIndexGatewayIP:
		if !gatewaySet {
			return errors.New("overlay-index gateway-ip needs a non-zero GW IP, but gateway is absent or zero " +
				"(RFC 9136 Section 3.1); set gateway to the tenant address the prefix resolves through, or remove overlay-index")
		}
		if esiSet {
			return fmt.Errorf("overlay-index gateway-ip needs an all-zero ESI, but esi is %s (RFC 9136 Section 3.1); "+
				"remove esi, or set overlay-index esi and remove gateway", formatESIForJSON(ESI(esi)))
		}
	default:
		return fmt.Errorf("unknown overlay index %d; use none, esi or gateway-ip", index)
	}
	return nil
}
