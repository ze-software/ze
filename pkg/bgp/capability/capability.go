// Package capability implements BGP capabilities (RFC 5492).
//
// RFC 5492 Section 4 defines the Capabilities Optional Parameter format:
//
//	+------------------------------+
//	| Capability Code (1 octet)    |
//	+------------------------------+
//	| Capability Length (1 octet)  |
//	+------------------------------+
//	| Capability Value (variable)  |
//	~                              ~
//	+------------------------------+
//
// This package parses and encodes individual capability TLVs.
package capability

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Errors.
var (
	ErrShortRead = errors.New("capability: short read")
)

// Code represents a BGP capability code.
//
// RFC 5492 Section 4: Capability Code is a one-octet unsigned binary integer
// that unambiguously identifies individual capabilities.
//
// RFC 5492 Section 6: Capability Code values 1-63 are assigned via IETF Review,
// values 64-127 via First Come First Served, values 128-255 for Private Use.
type Code uint8

// Capability codes per IANA BGP Capability Codes registry.
//
// RFC 5492 Section 6: IANA maintains the registry for Capability Code values.
// Capability Code value 0 is reserved.
const (
	CodeMultiprotocol        Code = 1  // RFC 4760 Section 8
	CodeRouteRefresh         Code = 2  // RFC 2918 Section 2
	CodeExtendedNextHop      Code = 5  // RFC 8950
	CodeExtendedMessage      Code = 6  // RFC 8654 Section 3
	CodeGracefulRestart      Code = 64 // RFC 4724 Section 3
	CodeASN4                 Code = 65 // RFC 6793 Section 3
	CodeAddPath              Code = 69 // RFC 7911 Section 4
	CodeEnhancedRouteRefresh Code = 70 // RFC 7313 Section 3.1
	CodeFQDN                 Code = 73 // RFC 8516
	CodeSoftwareVersion      Code = 75 // draft-ietf-idr-software-version
)

// String returns human-readable capability code name.
func (c Code) String() string {
	switch c {
	case CodeMultiprotocol:
		return "Multiprotocol(1)"
	case CodeRouteRefresh:
		return "Route Refresh(2)"
	case CodeExtendedNextHop:
		return "Extended Next Hop(5)"
	case CodeExtendedMessage:
		return "Extended Message(6)"
	case CodeGracefulRestart:
		return "Graceful Restart(64)"
	case CodeASN4:
		return "ASN4(65)"
	case CodeAddPath:
		return "ADD-PATH(69)"
	case CodeEnhancedRouteRefresh:
		return "Enhanced Route Refresh(70)"
	case CodeFQDN:
		return "FQDN(73)"
	case CodeSoftwareVersion:
		return "Software Version(75)"
	default:
		return fmt.Sprintf("Unknown(%d)", c)
	}
}

// Capability is the interface implemented by all BGP capabilities.
//
// RFC 5492 Section 4: Each capability is encoded as a TLV (Type-Length-Value).
type Capability interface {
	Code() Code
	Pack() []byte
}

// AFI represents Address Family Identifier.
//
// RFC 4760 Section 3: Address Family Identifier (AFI) is a 16-bit value
// that identifies the address family (e.g., IPv4, IPv6).
// Values are assigned by IANA in the Address Family Numbers registry.
type AFI uint16

// Address Family Identifiers per IANA Address Family Numbers registry.
const (
	AFIIPv4  AFI = 1     // RFC 4760
	AFIIPv6  AFI = 2     // RFC 4760
	AFIL2VPN AFI = 25    // RFC 4761
	AFIBGPLS AFI = 16388 // RFC 7752
)

// SAFI represents Subsequent Address Family Identifier.
//
// RFC 4760 Section 6: SAFI is an 8-bit value that provides additional
// information about the type of NLRI being carried.
type SAFI uint8

// Subsequent Address Family Identifiers per IANA SAFI registry.
//
// RFC 4760 Section 6: SAFI values 1-2 defined in RFC 4760.
const (
	SAFIUnicast     SAFI = 1   // RFC 4760 Section 6
	SAFIMulticast   SAFI = 2   // RFC 4760 Section 6
	SAFIMPLSLabel   SAFI = 4   // RFC 8277 (labeled unicast)
	SAFIMcastVPN    SAFI = 5   // RFC 6514 (multicast VPN)
	SAFIVPLS        SAFI = 65  // RFC 4761 (VPLS)
	SAFIEVPN        SAFI = 70  // RFC 7432
	SAFIBGPLS       SAFI = 71  // RFC 7752 (BGP-LS)
	SAFIBGPLSVPN    SAFI = 72  // RFC 7752 (BGP-LS VPN)
	SAFIMPLS        SAFI = 128 // RFC 4364 (MPLS-labeled VPN, aka mpls-vpn)
	SAFIVPN         SAFI = 128 // RFC 4364 (alias for SAFIMPLS)
	SAFIFlowSpec    SAFI = 133 // RFC 8955
	SAFIFlowSpecVPN SAFI = 134 // RFC 8955 (FlowSpec VPN)
)

// Family combines AFI and SAFI.
//
// RFC 4760 Section 8: The <AFI, SAFI> tuple identifies the address family
// and sub-address family for multiprotocol capabilities.
type Family struct {
	AFI  AFI
	SAFI SAFI
}

// String returns human-readable family name (e.g., "ipv6/unicast").
func (f Family) String() string {
	return f.AFI.String() + "/" + f.SAFI.String()
}

// String returns human-readable AFI name.
func (a AFI) String() string {
	switch a {
	case AFIIPv4:
		return "ipv4"
	case AFIIPv6:
		return "ipv6"
	case AFIL2VPN:
		return "l2vpn"
	case AFIBGPLS:
		return "bgp-ls"
	default:
		return fmt.Sprintf("afi-%d", a)
	}
}

// String returns human-readable SAFI name.
func (s SAFI) String() string {
	switch s {
	case SAFIUnicast:
		return "unicast"
	case SAFIMulticast:
		return "multicast"
	case SAFIMPLSLabel:
		return "mpls-label"
	case SAFIMcastVPN:
		return "mcast-vpn"
	case SAFIVPLS:
		return "vpls"
	case SAFIEVPN:
		return "evpn"
	case SAFIBGPLS:
		return "bgp-ls"
	case SAFIBGPLSVPN:
		return "bgp-ls-vpn"
	case SAFIMPLS:
		return "mpls-vpn"
	case SAFIFlowSpec:
		return "flowspec"
	case SAFIFlowSpecVPN:
		return "flowspec-vpn"
	default:
		return fmt.Sprintf("safi-%d", s)
	}
}

// Parse parses capability TLVs from optional parameters.
//
// RFC 5492 Section 4: The Capabilities Optional Parameter contains one or more
// triples <Capability Code, Capability Length, Capability Value>.
//
// RFC 5492 Section 4: A BGP speaker MUST be prepared to accept multiple
// instances of a capability with the same Capability Code.
func Parse(data []byte) ([]Capability, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var caps []Capability
	offset := 0

	for offset < len(data) {
		// RFC 5492 Section 4: Capability Code (1 octet) + Capability Length (1 octet)
		if offset+2 > len(data) {
			return nil, ErrShortRead
		}

		code := Code(data[offset])    //nolint:gosec // Bounds checked above
		length := int(data[offset+1]) //nolint:gosec // Bounds checked above
		offset += 2

		// RFC 5492 Section 4: Capability Value is variable-length
		if offset+length > len(data) {
			return nil, ErrShortRead
		}

		capData := data[offset : offset+length]
		offset += length

		cap, err := parseCapability(code, capData)
		if err != nil {
			return nil, err
		}
		caps = append(caps, cap)
	}

	return caps, nil
}

// parseCapability parses a single capability by code.
//
// RFC 5492 Section 3: If a BGP speaker receives from its peer a capability
// that it does not itself support or recognize, it MUST ignore that capability.
func parseCapability(code Code, data []byte) (Capability, error) {
	switch code { //nolint:exhaustive // Unknown capabilities handled in default
	case CodeMultiprotocol:
		return parseMultiprotocol(data)
	case CodeASN4:
		return parseASN4(data)
	case CodeRouteRefresh:
		return &RouteRefresh{}, nil
	case CodeExtendedMessage:
		return &ExtendedMessage{}, nil
	case CodeEnhancedRouteRefresh:
		return &EnhancedRouteRefresh{}, nil
	case CodeExtendedNextHop:
		return parseExtendedNextHop(data)
	case CodeAddPath:
		return parseAddPath(data)
	case CodeGracefulRestart:
		return parseGracefulRestart(data)
	case CodeFQDN:
		return parseFQDN(data)
	case CodeSoftwareVersion:
		return parseSoftwareVersion(data)
	default:
		// RFC 5492 Section 3: Unrecognized capabilities MUST be ignored.
		// We preserve raw data for debugging/logging purposes.
		return &Unknown{code: code, Data: append([]byte{}, data...)}, nil
	}
}

// Unknown represents an unrecognized capability.
//
// RFC 5492 Section 3: Unrecognized capabilities are ignored but preserved
// for debugging/forwarding purposes.
type Unknown struct {
	code Code
	Data []byte
}

func (u *Unknown) Code() Code { return u.code }

func (u *Unknown) Pack() []byte {
	return packCapability(u.code, u.Data)
}

// packCapability creates a capability TLV.
//
// RFC 5492 Section 4: Each capability is encoded as:
//   - Capability Code (1 octet)
//   - Capability Length (1 octet)
//   - Capability Value (variable)
func packCapability(code Code, data []byte) []byte {
	result := make([]byte, 2+len(data))
	result[0] = byte(code)
	result[1] = byte(len(data))
	copy(result[2:], data)
	return result
}

// Multiprotocol represents Multiprotocol Extensions capability (RFC 4760).
//
// RFC 4760 Section 8: The Capability Value field is defined as:
//
//	+-------+-------+-------+-------+
//	|      AFI      | Res.  | SAFI  |
//	+-------+-------+-------+-------+
//
// The Capability Code is 1 and Capability Length is 4.
type Multiprotocol struct {
	AFI  AFI
	SAFI SAFI
}

func (m *Multiprotocol) Code() Code { return CodeMultiprotocol }

// Pack encodes the Multiprotocol capability.
//
// RFC 4760 Section 8: AFI (16 bit), Reserved (8 bit, set to 0), SAFI (8 bit).
func (m *Multiprotocol) Pack() []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[0:2], uint16(m.AFI))
	data[2] = 0 // RFC 4760 Section 8: Reserved, SHOULD be set to 0
	data[3] = byte(m.SAFI)
	return packCapability(CodeMultiprotocol, data)
}

// parseMultiprotocol parses a Multiprotocol Extensions capability.
//
// RFC 4760 Section 8: Capability Length is 4 bytes.
func parseMultiprotocol(data []byte) (*Multiprotocol, error) {
	if len(data) < 4 {
		return nil, ErrShortRead
	}
	return &Multiprotocol{
		AFI:  AFI(binary.BigEndian.Uint16(data[0:2])),
		SAFI: SAFI(data[3]), // RFC 4760 Section 8: Reserved byte at offset 2 is ignored
	}, nil
}

// ASN4 represents 4-Byte AS Number capability (RFC 6793).
//
// RFC 6793 Section 3: The capability code is 65 and length is 4.
// The Capability Value field contains the 4-octet AS number of the speaker.
//
// RFC 6793 Section 4.1: When a NEW BGP speaker processes an OPEN message
// from another NEW BGP speaker, it MUST use the AS number encoded in this
// capability in lieu of the "My Autonomous System" field of the OPEN message.
type ASN4 struct {
	ASN uint32
}

func (a *ASN4) Code() Code { return CodeASN4 }

// Pack encodes the ASN4 capability.
//
// RFC 6793 Section 3: The AS number is encoded as a 4-octet entity.
func (a *ASN4) Pack() []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, a.ASN)
	return packCapability(CodeASN4, data)
}

// parseASN4 parses a 4-Byte AS Number capability.
//
// RFC 6793 Section 3: Capability Length is 4 bytes.
func parseASN4(data []byte) (*ASN4, error) {
	if len(data) < 4 {
		return nil, ErrShortRead
	}
	return &ASN4{
		ASN: binary.BigEndian.Uint32(data),
	}, nil
}

// RouteRefresh represents Route Refresh capability (RFC 2918).
//
// RFC 2918 Section 2: Capability code 2 with Capability length 0.
// By advertising this capability, a BGP speaker conveys that it can
// receive and properly handle ROUTE-REFRESH messages.
type RouteRefresh struct{}

func (r *RouteRefresh) Code() Code { return CodeRouteRefresh }

// Pack encodes the Route Refresh capability.
//
// RFC 2918 Section 2: Capability length is 0 (no value field).
func (r *RouteRefresh) Pack() []byte {
	return packCapability(CodeRouteRefresh, nil)
}

// ExtendedMessage represents Extended Message capability (RFC 8654).
//
// RFC 8654 Section 3: Capability Code 6 with Capability Length 0.
// Allows BGP messages up to 65,535 octets (except OPEN and KEEPALIVE).
type ExtendedMessage struct{}

func (e *ExtendedMessage) Code() Code { return CodeExtendedMessage }

// Pack encodes the Extended Message capability.
//
// RFC 8654 Section 3: Capability length is 0 (no value field).
func (e *ExtendedMessage) Pack() []byte {
	return packCapability(CodeExtendedMessage, nil)
}

// EnhancedRouteRefresh represents Enhanced Route Refresh capability (RFC 7313).
//
// RFC 7313 Section 3.1: Capability Code 70 with Capability Length 0.
// Enables BoRR/EoRR markers for route refresh demarcation.
type EnhancedRouteRefresh struct{}

func (e *EnhancedRouteRefresh) Code() Code { return CodeEnhancedRouteRefresh }

// Pack encodes the Enhanced Route Refresh capability.
//
// RFC 7313 Section 3.1: Capability length is 0 (no value field).
func (e *EnhancedRouteRefresh) Pack() []byte {
	return packCapability(CodeEnhancedRouteRefresh, nil)
}

// AddPathMode indicates send/receive capability for ADD-PATH.
//
// RFC 7911 Section 4: The Send/Receive field indicates whether the sender is
// able to receive (1), send (2), or both (3) multiple paths for the <AFI, SAFI>.
type AddPathMode uint8

// RFC 7911 Section 4: Send/Receive field values.
const (
	AddPathNone    AddPathMode = 0 // Not valid per RFC 7911
	AddPathReceive AddPathMode = 1 // RFC 7911: able to receive multiple paths
	AddPathSend    AddPathMode = 2 // RFC 7911: able to send multiple paths
	AddPathBoth    AddPathMode = 3 // RFC 7911: able to both send and receive
)

// AddPathFamily describes ADD-PATH support for one AFI/SAFI.
//
// RFC 7911 Section 4: Each tuple is 4 octets: AFI (2) + SAFI (1) + Send/Receive (1).
type AddPathFamily struct {
	AFI  AFI
	SAFI SAFI
	Mode AddPathMode
}

// AddPath represents ADD-PATH capability (RFC 7911).
//
// RFC 7911 Section 4: Capability Code 69, variable length.
// The Capability Value consists of one or more <AFI, SAFI, Send/Receive> tuples.
type AddPath struct {
	Families []AddPathFamily
}

func (a *AddPath) Code() Code { return CodeAddPath }

// Pack encodes the ADD-PATH capability.
//
// RFC 7911 Section 4: Each AFI/SAFI tuple is encoded as:
// AFI (2 octets) + SAFI (1 octet) + Send/Receive (1 octet) = 4 octets.
func (a *AddPath) Pack() []byte {
	data := make([]byte, len(a.Families)*4)
	for i, f := range a.Families {
		offset := i * 4
		binary.BigEndian.PutUint16(data[offset:], uint16(f.AFI))
		data[offset+2] = byte(f.SAFI)
		data[offset+3] = byte(f.Mode)
	}
	return packCapability(CodeAddPath, data)
}

// parseAddPath parses an ADD-PATH capability.
//
// RFC 7911 Section 4: Capability value length must be a multiple of 4.
func parseAddPath(data []byte) (*AddPath, error) {
	if len(data)%4 != 0 {
		return nil, ErrShortRead
	}

	families := make([]AddPathFamily, len(data)/4)
	for i := range families {
		offset := i * 4
		families[i] = AddPathFamily{
			AFI:  AFI(binary.BigEndian.Uint16(data[offset:])),
			SAFI: SAFI(data[offset+2]),
			Mode: AddPathMode(data[offset+3]),
		}
	}
	return &AddPath{Families: families}, nil
}

// GracefulRestartFamily describes GR support for one AFI/SAFI.
//
// RFC 4724 Section 3: Each <AFI, SAFI, Flags> tuple is 4 octets.
// The Forwarding State (F) bit indicates forwarding state was preserved.
type GracefulRestartFamily struct {
	AFI             AFI
	SAFI            SAFI
	ForwardingState bool // RFC 4724: Forwarding State (F) bit
}

// GracefulRestart represents Graceful Restart capability (RFC 4724).
//
// RFC 4724 Section 3: Capability Code 64, variable length.
// First 2 octets contain Restart Flags (4 bits) and Restart Time (12 bits).
// Followed by zero or more <AFI, SAFI, Flags> tuples (4 octets each).
//
//	+--------------------------------------------------+
//	| Restart Flags (4 bits) | Restart Time (12 bits) |
//	+--------------------------------------------------+
//	| AFI (16 bits)          | SAFI (8 bits)          |
//	+--------------------------------------------------+
//	| Flags for AFI/SAFI (8 bits)                      |
//	+--------------------------------------------------+
type GracefulRestart struct {
	RestartState bool   // RFC 4724: Restart State (R) bit - speaker is restarting
	RestartTime  uint16 // RFC 4724: Restart Time in seconds (12 bits, max 4095)
	Families     []GracefulRestartFamily
}

func (g *GracefulRestart) Code() Code { return CodeGracefulRestart }

// Pack encodes the Graceful Restart capability.
//
// RFC 4724 Section 3: Restart Flags in high 4 bits, Restart Time in low 12 bits.
func (g *GracefulRestart) Pack() []byte {
	data := make([]byte, 2+len(g.Families)*4)

	// RFC 4724 Section 3: First 2 bytes contain flags (4 bits) and restart time (12 bits)
	val := g.RestartTime & 0x0FFF
	if g.RestartState {
		val |= 0x8000 // RFC 4724: R bit is the most significant bit
	}
	binary.BigEndian.PutUint16(data[0:2], val)

	// RFC 4724 Section 3: AFI/SAFI entries (4 bytes each)
	for i, f := range g.Families {
		offset := 2 + i*4
		binary.BigEndian.PutUint16(data[offset:], uint16(f.AFI))
		data[offset+2] = byte(f.SAFI)
		if f.ForwardingState {
			data[offset+3] = 0x80 // RFC 4724: F bit is the most significant bit
		}
	}
	return packCapability(CodeGracefulRestart, data)
}

// parseGracefulRestart parses a Graceful Restart capability.
//
// RFC 4724 Section 3: Minimum length is 2 bytes (flags + restart time).
func parseGracefulRestart(data []byte) (*GracefulRestart, error) {
	if len(data) < 2 {
		return nil, ErrShortRead
	}

	// RFC 4724 Section 3: Parse restart flags and time
	flags := binary.BigEndian.Uint16(data[0:2])
	gr := &GracefulRestart{
		RestartState: (flags & 0x8000) != 0, // R bit
		RestartTime:  flags & 0x0FFF,        // 12-bit restart time
	}

	// RFC 4724 Section 3: Parse AFI/SAFI entries (4 bytes each)
	remaining := data[2:]
	if len(remaining)%4 != 0 {
		return nil, ErrShortRead
	}

	gr.Families = make([]GracefulRestartFamily, len(remaining)/4)
	for i := range gr.Families {
		offset := i * 4
		gr.Families[i] = GracefulRestartFamily{
			AFI:             AFI(binary.BigEndian.Uint16(remaining[offset:])),
			SAFI:            SAFI(remaining[offset+2]),
			ForwardingState: (remaining[offset+3] & 0x80) != 0, // F bit
		}
	}

	return gr, nil
}

// ExtendedNextHopFamily describes extended next-hop support for one AFI/SAFI.
//
// RFC 8950 Section 4: Each tuple is 6 octets encoding the NLRI AFI/SAFI
// and the Next Hop AFI that can be used for that NLRI type.
type ExtendedNextHopFamily struct {
	NLRIAFI    AFI  // AFI of NLRI
	NLRISAFI   SAFI // SAFI of NLRI (encoded as 16-bit per RFC 8950)
	NextHopAFI AFI  // AFI of next-hop (typically IPv6 for IPv4 NLRI)
}

// ExtendedNextHop represents Extended Next Hop capability (RFC 8950).
//
// RFC 8950 Section 4: Capability Code 5, variable length.
// This capability allows advertising IPv4 NLRI with IPv6 next-hops,
// enabling IPv4 routing over IPv6-only infrastructure.
//
// Each tuple encodes: NLRI AFI (2) + NLRI SAFI (2) + Next Hop AFI (2) = 6 octets.
// Note: SAFI is encoded as 16 bits here, unlike RFC 4760 where it is 8 bits.
type ExtendedNextHop struct {
	Families []ExtendedNextHopFamily
}

func (e *ExtendedNextHop) Code() Code { return CodeExtendedNextHop }

// Pack encodes the Extended Next Hop capability.
//
// RFC 8950 Section 4: Each tuple is 6 octets (AFI + SAFI + Next Hop AFI, all 16-bit).
func (e *ExtendedNextHop) Pack() []byte {
	data := make([]byte, len(e.Families)*6)
	for i, f := range e.Families {
		offset := i * 6
		binary.BigEndian.PutUint16(data[offset:], uint16(f.NLRIAFI))
		binary.BigEndian.PutUint16(data[offset+2:], uint16(f.NLRISAFI)) // RFC 8950: SAFI as 16-bit
		binary.BigEndian.PutUint16(data[offset+4:], uint16(f.NextHopAFI))
	}
	return packCapability(CodeExtendedNextHop, data)
}

// parseExtendedNextHop parses an Extended Next Hop capability.
//
// RFC 8950 Section 4: Capability value length must be a multiple of 6.
func parseExtendedNextHop(data []byte) (*ExtendedNextHop, error) {
	if len(data)%6 != 0 {
		return nil, ErrShortRead
	}

	families := make([]ExtendedNextHopFamily, len(data)/6)
	for i := range families {
		offset := i * 6
		families[i] = ExtendedNextHopFamily{
			NLRIAFI:    AFI(binary.BigEndian.Uint16(data[offset:])),
			NLRISAFI:   SAFI(binary.BigEndian.Uint16(data[offset+2:])), //nolint:gosec // RFC 8950: SAFI encoded as 16-bit
			NextHopAFI: AFI(binary.BigEndian.Uint16(data[offset+4:])),
		}
	}
	return &ExtendedNextHop{Families: families}, nil
}

// FQDN represents FQDN capability (RFC 8516).
//
// RFC 8516 Section 3: Capability Code 73, variable length.
// This capability advertises the fully qualified domain name (hostname
// and domain name) of the BGP speaker.
//
// Format: Hostname Length (1) + Hostname (variable) + Domain Length (1) + Domain (variable).
type FQDN struct {
	Hostname   string
	DomainName string
}

func (f *FQDN) Code() Code { return CodeFQDN }

// Pack encodes the FQDN capability.
//
// RFC 8516 Section 3: Each string is prefixed with a 1-octet length field.
// Maximum length for each string is 255 bytes.
func (f *FQDN) Pack() []byte {
	hostLen := len(f.Hostname)
	domainLen := len(f.DomainName)

	// RFC 8516: Max 255 bytes each (1-octet length field)
	if hostLen > 255 {
		hostLen = 255
	}
	if domainLen > 255 {
		domainLen = 255
	}

	data := make([]byte, 1+hostLen+1+domainLen)
	data[0] = byte(hostLen)
	copy(data[1:1+hostLen], f.Hostname[:hostLen])
	data[1+hostLen] = byte(domainLen)
	copy(data[2+hostLen:], f.DomainName[:domainLen])

	return packCapability(CodeFQDN, data)
}

// parseFQDN parses an FQDN capability.
//
// RFC 8516 Section 3: Minimum length is 2 bytes (two length fields).
func parseFQDN(data []byte) (*FQDN, error) {
	if len(data) < 2 {
		return nil, ErrShortRead
	}

	hostLen := int(data[0])
	if len(data) < 1+hostLen+1 {
		return nil, ErrShortRead
	}

	hostname := string(data[1 : 1+hostLen])

	domainLen := int(data[1+hostLen])
	if len(data) < 2+hostLen+domainLen {
		return nil, ErrShortRead
	}

	domainName := string(data[2+hostLen : 2+hostLen+domainLen])

	return &FQDN{
		Hostname:   hostname,
		DomainName: domainName,
	}, nil
}

// SoftwareVersion represents Software Version capability.
//
// draft-ietf-idr-software-version: Capability Code 75, variable length.
// This capability advertises the software version of the BGP implementation.
//
// Format: Version Length (1) + Version String (variable).
type SoftwareVersion struct {
	Version string
}

func (s *SoftwareVersion) Code() Code { return CodeSoftwareVersion }

// Pack encodes the Software Version capability.
//
// draft-ietf-idr-software-version: Version string prefixed with 1-octet length.
// Maximum length is 255 bytes.
func (s *SoftwareVersion) Pack() []byte {
	verLen := len(s.Version)
	if verLen > 255 {
		verLen = 255
	}

	data := make([]byte, 1+verLen)
	data[0] = byte(verLen)
	copy(data[1:], s.Version[:verLen])

	return packCapability(CodeSoftwareVersion, data)
}

// parseSoftwareVersion parses a Software Version capability.
//
// draft-ietf-idr-software-version: Minimum length is 1 byte (length field).
func parseSoftwareVersion(data []byte) (*SoftwareVersion, error) {
	if len(data) < 1 {
		return nil, ErrShortRead
	}

	verLen := int(data[0])
	if len(data) < 1+verLen {
		return nil, ErrShortRead
	}

	return &SoftwareVersion{
		Version: string(data[1 : 1+verLen]),
	}, nil
}
