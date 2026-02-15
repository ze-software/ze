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

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
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
	CodeRole                 Code = 9  // RFC 9234 Section 4.1
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
	case CodeRole:
		return "Role(9)"
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
	Len() int
	WriteTo(buf []byte, off int) int
}

// ConfigProvider is an optional interface for capabilities that provide
// configuration values for plugin delivery (Stage 2 of plugin protocol).
// Capabilities implement this to expose their config to plugins.
//
// Keys must be scoped to prevent collisions:
//   - "rfc<num>:<field>" for RFC-based capabilities
//   - "draft-<name>:<field>" for draft-based capabilities
//
// Example: FQDN capability returns {"draft-walton-bgp-hostname:hostname": "router1"}.
type ConfigProvider interface {
	// ConfigValues returns scoped key-value pairs for config delivery.
	ConfigValues() map[string]string
}

// AFI is an alias for nlri.AFI for backward compatibility.
// RFC 4760 Section 3: Address Family Identifier (AFI) is a 16-bit value.
type AFI = nlri.AFI

// Address Family Identifiers per IANA Address Family Numbers registry.
const (
	AFIIPv4  AFI = 1     // RFC 4760
	AFIIPv6  AFI = 2     // RFC 4760
	AFIL2VPN AFI = 25    // RFC 4761
	AFIBGPLS AFI = 16388 // RFC 7752
)

// SAFI is an alias for nlri.SAFI for backward compatibility.
// RFC 4760 Section 6: SAFI is an 8-bit value.
type SAFI = nlri.SAFI

// Subsequent Address Family Identifiers per IANA SAFI registry.
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

// Family is an alias for nlri.Family for backward compatibility.
// RFC 4760 Section 8: The <AFI, SAFI> tuple identifies the address family.
type Family = nlri.Family

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
	default: // RFC 5492 Section 3: Unrecognized capabilities MUST be ignored.
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

func (u *Unknown) Len() int { return 2 + len(u.Data) }

func (u *Unknown) WriteTo(buf []byte, off int) int {
	writeCapabilityTo(buf, off, u.code, len(u.Data))
	copy(buf[off+2:], u.Data)
	return u.Len()
}

// writeCapabilityTo writes a capability TLV header into buf at offset.
// Returns 2 (the header size). Caller writes value bytes after.
func writeCapabilityTo(buf []byte, off int, code Code, valueLen int) {
	buf[off] = byte(code)
	buf[off+1] = byte(valueLen) //nolint:gosec // Capability values are always <256 bytes
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

func (m *Multiprotocol) Len() int { return 6 } // 2 header + 4 value

func (m *Multiprotocol) WriteTo(buf []byte, off int) int {
	writeCapabilityTo(buf, off, CodeMultiprotocol, 4)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(m.AFI))
	buf[off+4] = 0 // RFC 4760 Section 8: Reserved
	buf[off+5] = byte(m.SAFI)
	return 6
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

func (a *ASN4) Len() int { return 6 } // 2 header + 4 value

func (a *ASN4) WriteTo(buf []byte, off int) int {
	writeCapabilityTo(buf, off, CodeASN4, 4)
	binary.BigEndian.PutUint32(buf[off+2:], a.ASN)
	return 6
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

func (r *RouteRefresh) Len() int { return 2 } // 2 header + 0 value

func (r *RouteRefresh) WriteTo(buf []byte, off int) int {
	writeCapabilityTo(buf, off, CodeRouteRefresh, 0)
	return 2
}

// ConfigValues implements ConfigProvider for plugin config delivery.
func (r *RouteRefresh) ConfigValues() map[string]string {
	return map[string]string{"rfc2918:enabled": "true"}
}

// ExtendedMessage represents Extended Message capability (RFC 8654).
//
// RFC 8654 Section 3: Capability Code 6 with Capability Length 0.
// Allows BGP messages up to 65,535 octets (except OPEN and KEEPALIVE).
type ExtendedMessage struct{}

func (e *ExtendedMessage) Code() Code { return CodeExtendedMessage }

func (e *ExtendedMessage) Len() int { return 2 } // 2 header + 0 value

func (e *ExtendedMessage) WriteTo(buf []byte, off int) int {
	writeCapabilityTo(buf, off, CodeExtendedMessage, 0)
	return 2
}

// ConfigValues implements ConfigProvider for plugin config delivery.
func (e *ExtendedMessage) ConfigValues() map[string]string {
	return map[string]string{"rfc8654:enabled": "true"}
}

// EnhancedRouteRefresh represents Enhanced Route Refresh capability (RFC 7313).
//
// RFC 7313 Section 3.1: Capability Code 70 with Capability Length 0.
// Enables BoRR/EoRR markers for route refresh demarcation.
type EnhancedRouteRefresh struct{}

func (e *EnhancedRouteRefresh) Code() Code { return CodeEnhancedRouteRefresh }

func (e *EnhancedRouteRefresh) Len() int { return 2 } // 2 header + 0 value

func (e *EnhancedRouteRefresh) WriteTo(buf []byte, off int) int {
	writeCapabilityTo(buf, off, CodeEnhancedRouteRefresh, 0)
	return 2
}

// ConfigValues implements ConfigProvider for plugin config delivery.
func (e *EnhancedRouteRefresh) ConfigValues() map[string]string {
	return map[string]string{"rfc7313:enabled": "true"}
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

func (a *AddPath) Len() int { return 2 + len(a.Families)*4 }

func (a *AddPath) WriteTo(buf []byte, off int) int {
	dataLen := len(a.Families) * 4
	writeCapabilityTo(buf, off, CodeAddPath, dataLen)
	for i, f := range a.Families {
		o := off + 2 + i*4
		binary.BigEndian.PutUint16(buf[o:], uint16(f.AFI))
		buf[o+2] = byte(f.SAFI)
		buf[o+3] = byte(f.Mode)
	}
	return 2 + dataLen
}

// ConfigValues implements ConfigProvider for plugin config delivery.
func (a *AddPath) ConfigValues() map[string]string {
	result := make(map[string]string)
	for _, f := range a.Families {
		if f.Mode == AddPathSend || f.Mode == AddPathBoth {
			result["rfc7911:send"] = "true"
		}
		if f.Mode == AddPathReceive || f.Mode == AddPathBoth {
			result["rfc7911:receive"] = "true"
		}
	}
	return result
}

// parseAddPath parses an ADD-PATH capability.
//
// RFC 7911 Section 4: Capability value length must be a multiple of 4.
// Each tuple is <AFI(2), SAFI(1), Send/Receive(1)> where Send/Receive
// values are: 1 (Receive), 2 (Send), 3 (Both).
// RFC 7911 Section 4: "If any other value is received, then the capability
// SHOULD be treated as not understood and ignored [RFC5492].".
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

func (g *GracefulRestart) Len() int { return 2 + 2 + len(g.Families)*4 }

func (g *GracefulRestart) WriteTo(buf []byte, off int) int {
	dataLen := 2 + len(g.Families)*4
	writeCapabilityTo(buf, off, CodeGracefulRestart, dataLen)

	val := g.RestartTime & 0x0FFF
	if g.RestartState {
		val |= 0x8000
	}
	binary.BigEndian.PutUint16(buf[off+2:], val)

	for i, f := range g.Families {
		o := off + 4 + i*4
		binary.BigEndian.PutUint16(buf[o:], uint16(f.AFI))
		buf[o+2] = byte(f.SAFI)
		if f.ForwardingState {
			buf[o+3] = 0x80
		} else {
			buf[o+3] = 0
		}
	}
	return 2 + dataLen
}

// ConfigValues implements ConfigProvider for plugin config delivery.
// Returns scoped keys per RFC 4724.
func (g *GracefulRestart) ConfigValues() map[string]string {
	return map[string]string{
		"rfc4724:restart-time": fmt.Sprintf("%d", g.RestartTime),
	}
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

func (e *ExtendedNextHop) Len() int { return 2 + len(e.Families)*6 }

func (e *ExtendedNextHop) WriteTo(buf []byte, off int) int {
	dataLen := len(e.Families) * 6
	writeCapabilityTo(buf, off, CodeExtendedNextHop, dataLen)
	for i, f := range e.Families {
		o := off + 2 + i*6
		binary.BigEndian.PutUint16(buf[o:], uint16(f.NLRIAFI))
		binary.BigEndian.PutUint16(buf[o+2:], uint16(f.NLRISAFI))
		binary.BigEndian.PutUint16(buf[o+4:], uint16(f.NextHopAFI))
	}
	return 2 + dataLen
}

// ConfigValues implements ConfigProvider for plugin config delivery.
func (e *ExtendedNextHop) ConfigValues() map[string]string {
	if len(e.Families) == 0 {
		return nil
	}
	return map[string]string{"rfc8950:enabled": "true"}
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

func (f *FQDN) Len() int {
	return 2 + 1 + min(len(f.Hostname), 255) + 1 + min(len(f.DomainName), 255)
}

func (f *FQDN) WriteTo(buf []byte, off int) int {
	hostLen := min(len(f.Hostname), 255)
	domainLen := min(len(f.DomainName), 255)
	dataLen := 1 + hostLen + 1 + domainLen
	writeCapabilityTo(buf, off, CodeFQDN, dataLen)
	buf[off+2] = byte(hostLen)
	copy(buf[off+3:], f.Hostname[:hostLen])
	buf[off+3+hostLen] = byte(domainLen)
	copy(buf[off+4+hostLen:], f.DomainName[:domainLen])
	return 2 + dataLen
}

// ConfigValues implements ConfigProvider for plugin config delivery.
// Returns scoped keys per draft-walton-bgp-hostname.
func (f *FQDN) ConfigValues() map[string]string {
	result := make(map[string]string)
	if f.Hostname != "" {
		result["draft-walton-bgp-hostname:hostname"] = f.Hostname
	}
	if f.DomainName != "" {
		result["draft-walton-bgp-hostname:domain"] = f.DomainName
	}
	return result
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

func (s *SoftwareVersion) Len() int { return 2 + 1 + min(len(s.Version), 255) }

func (s *SoftwareVersion) WriteTo(buf []byte, off int) int {
	verLen := min(len(s.Version), 255)
	dataLen := 1 + verLen
	writeCapabilityTo(buf, off, CodeSoftwareVersion, dataLen)
	buf[off+2] = byte(verLen)
	copy(buf[off+3:], s.Version[:verLen])
	return 2 + dataLen
}

// ConfigValues implements ConfigProvider for plugin config delivery.
func (s *SoftwareVersion) ConfigValues() map[string]string {
	if s.Version == "" {
		return nil
	}
	return map[string]string{"draft-ietf-idr-software-version:version": s.Version}
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

// ParseFromOptionalParams extracts capabilities from OPEN optional parameters.
// RFC 5492 Section 4: Optional Parameters contain type-length-value triples.
// Type 2 indicates the Capabilities Optional Parameter.
func ParseFromOptionalParams(optParams []byte) []Capability {
	var caps []Capability
	offset := 0

	for offset < len(optParams) {
		if offset+2 > len(optParams) {
			break
		}

		paramType := optParams[offset]       //nolint:gosec // G602 false positive: offset+2 bounds-checked above
		paramLen := int(optParams[offset+1]) //nolint:gosec // G602 false positive: offset+2 bounds-checked above
		offset += 2

		if offset+paramLen > len(optParams) {
			break
		}

		// RFC 5492: Capability parameter type is 2
		if paramType == 2 {
			parsed, err := Parse(optParams[offset : offset+paramLen])
			if err == nil {
				caps = append(caps, parsed...)
			}
		}
		offset += paramLen
	}

	return caps
}
