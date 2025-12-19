// Package capability implements BGP capabilities (RFC 5492).
package capability

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Errors
var (
	ErrShortRead = errors.New("capability: short read")
)

// Code represents a BGP capability code (IANA assigned).
type Code uint8

// Capability codes per IANA BGP Capability Codes registry.
const (
	CodeMultiprotocol   Code = 1  // RFC 4760
	CodeRouteRefresh    Code = 2  // RFC 2918
	CodeExtendedNextHop Code = 5  // RFC 8950
	CodeExtendedMessage Code = 6  // RFC 8654
	CodeGracefulRestart Code = 64 // RFC 4724
	CodeASN4            Code = 65 // RFC 6793
	CodeAddPath         Code = 69 // RFC 7911
	CodeFQDN            Code = 73 // RFC 8516
	CodeSoftwareVersion Code = 75 // draft-ietf-idr-software-version
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
	case CodeFQDN:
		return "FQDN(73)"
	case CodeSoftwareVersion:
		return "Software Version(75)"
	default:
		return fmt.Sprintf("Unknown(%d)", c)
	}
}

// Capability is the interface implemented by all BGP capabilities.
type Capability interface {
	Code() Code
	Pack() []byte
}

// AFI represents Address Family Identifier (RFC 4760).
type AFI uint16

// Address Family Identifiers.
const (
	AFIIPv4  AFI = 1
	AFIIPv6  AFI = 2
	AFIL2VPN AFI = 25
	AFIBGPLS AFI = 16388
)

// SAFI represents Subsequent Address Family Identifier (RFC 4760).
type SAFI uint8

// Subsequent Address Family Identifiers.
const (
	SAFIUnicast   SAFI = 1
	SAFIMulticast SAFI = 2
	SAFIMPLSLabel SAFI = 4
	SAFIEVPN      SAFI = 70
	SAFIVPN       SAFI = 128
	SAFIFlowSpec  SAFI = 133
)

// Family combines AFI and SAFI.
type Family struct {
	AFI  AFI
	SAFI SAFI
}

// Parse parses capability TLVs from optional parameters.
func Parse(data []byte) ([]Capability, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var caps []Capability
	offset := 0

	for offset < len(data) {
		if offset+2 > len(data) {
			return nil, ErrShortRead
		}

		code := Code(data[offset])
		length := int(data[offset+1])
		offset += 2

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
func parseCapability(code Code, data []byte) (Capability, error) {
	switch code {
	case CodeMultiprotocol:
		return parseMultiprotocol(data)
	case CodeASN4:
		return parseASN4(data)
	case CodeRouteRefresh:
		return &RouteRefresh{}, nil
	case CodeExtendedMessage:
		return &ExtendedMessage{}, nil
	case CodeAddPath:
		return parseAddPath(data)
	case CodeGracefulRestart:
		return parseGracefulRestart(data)
	default:
		// Unknown capability - preserve raw data
		return &Unknown{code: code, Data: append([]byte{}, data...)}, nil
	}
}

// Unknown represents an unrecognized capability.
type Unknown struct {
	code Code
	Data []byte
}

func (u *Unknown) Code() Code { return u.code }

func (u *Unknown) Pack() []byte {
	return packCapability(u.code, u.Data)
}

// packCapability creates a capability TLV.
func packCapability(code Code, data []byte) []byte {
	result := make([]byte, 2+len(data))
	result[0] = byte(code)
	result[1] = byte(len(data))
	copy(result[2:], data)
	return result
}

// Multiprotocol represents Multiprotocol Extensions (RFC 4760).
type Multiprotocol struct {
	AFI  AFI
	SAFI SAFI
}

func (m *Multiprotocol) Code() Code { return CodeMultiprotocol }

func (m *Multiprotocol) Pack() []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[0:2], uint16(m.AFI))
	data[2] = 0 // Reserved
	data[3] = byte(m.SAFI)
	return packCapability(CodeMultiprotocol, data)
}

func parseMultiprotocol(data []byte) (*Multiprotocol, error) {
	if len(data) < 4 {
		return nil, ErrShortRead
	}
	return &Multiprotocol{
		AFI:  AFI(binary.BigEndian.Uint16(data[0:2])),
		SAFI: SAFI(data[3]),
	}, nil
}

// ASN4 represents 4-Byte AS Number capability (RFC 6793).
type ASN4 struct {
	ASN uint32
}

func (a *ASN4) Code() Code { return CodeASN4 }

func (a *ASN4) Pack() []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, a.ASN)
	return packCapability(CodeASN4, data)
}

func parseASN4(data []byte) (*ASN4, error) {
	if len(data) < 4 {
		return nil, ErrShortRead
	}
	return &ASN4{
		ASN: binary.BigEndian.Uint32(data),
	}, nil
}

// RouteRefresh represents Route Refresh capability (RFC 2918).
type RouteRefresh struct{}

func (r *RouteRefresh) Code() Code { return CodeRouteRefresh }
func (r *RouteRefresh) Pack() []byte {
	return packCapability(CodeRouteRefresh, nil)
}

// ExtendedMessage represents Extended Message capability (RFC 8654).
type ExtendedMessage struct{}

func (e *ExtendedMessage) Code() Code { return CodeExtendedMessage }
func (e *ExtendedMessage) Pack() []byte {
	return packCapability(CodeExtendedMessage, nil)
}

// AddPathMode indicates send/receive capability for ADD-PATH.
type AddPathMode uint8

const (
	AddPathNone    AddPathMode = 0
	AddPathReceive AddPathMode = 1
	AddPathSend    AddPathMode = 2
	AddPathBoth    AddPathMode = 3
)

// AddPathFamily describes ADD-PATH support for one AFI/SAFI.
type AddPathFamily struct {
	AFI  AFI
	SAFI SAFI
	Mode AddPathMode
}

// AddPath represents ADD-PATH capability (RFC 7911).
type AddPath struct {
	Families []AddPathFamily
}

func (a *AddPath) Code() Code { return CodeAddPath }

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
type GracefulRestartFamily struct {
	AFI             AFI
	SAFI            SAFI
	ForwardingState bool
}

// GracefulRestart represents Graceful Restart capability (RFC 4724).
type GracefulRestart struct {
	RestartState bool   // Restart State (R) bit
	RestartTime  uint16 // Restart Time in seconds (12 bits)
	Families     []GracefulRestartFamily
}

func (g *GracefulRestart) Code() Code { return CodeGracefulRestart }

func (g *GracefulRestart) Pack() []byte {
	data := make([]byte, 2+len(g.Families)*4)

	// First 2 bytes: flags and restart time
	val := g.RestartTime & 0x0FFF
	if g.RestartState {
		val |= 0x8000
	}
	binary.BigEndian.PutUint16(data[0:2], val)

	// AFI/SAFI entries
	for i, f := range g.Families {
		offset := 2 + i*4
		binary.BigEndian.PutUint16(data[offset:], uint16(f.AFI))
		data[offset+2] = byte(f.SAFI)
		if f.ForwardingState {
			data[offset+3] = 0x80
		}
	}
	return packCapability(CodeGracefulRestart, data)
}

func parseGracefulRestart(data []byte) (*GracefulRestart, error) {
	if len(data) < 2 {
		return nil, ErrShortRead
	}

	flags := binary.BigEndian.Uint16(data[0:2])
	gr := &GracefulRestart{
		RestartState: (flags & 0x8000) != 0,
		RestartTime:  flags & 0x0FFF,
	}

	// Parse AFI/SAFI entries (4 bytes each)
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
			ForwardingState: (remaining[offset+3] & 0x80) != 0,
		}
	}

	return gr, nil
}
