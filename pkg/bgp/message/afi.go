package message

// AFI represents Address Family Identifier (RFC 4760).
type AFI uint16

// Address Family Identifiers
const (
	AFIIPv4  AFI = 1
	AFIIPv6  AFI = 2
	AFIL2VPN AFI = 25
	AFIBGPLS AFI = 16388
)

// SAFI represents Subsequent Address Family Identifier (RFC 4760).
type SAFI uint8

// Subsequent Address Family Identifiers
const (
	SAFIUnicast   SAFI = 1
	SAFIMulticast SAFI = 2
	SAFIMPLSLabel SAFI = 4
	SAFIEVPN      SAFI = 70
	SAFIVPN       SAFI = 128
	SAFIFlowSpec  SAFI = 133
)
