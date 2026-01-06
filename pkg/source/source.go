// Package source provides a unified registry for message sources (peers, API processes, config).
// Source type is encoded in the ID value for self-describing compact storage.
package source

import "net/netip"

// SourceID is a self-describing identifier for a message source.
// The ID range encodes the source type:
//   - 0: config (singleton)
//   - 1-99999: peer
//   - 100000: reserved
//   - 100001+: api
//   - MaxUint32: invalid
type SourceID uint32

// ID range boundaries.
const (
	SourceIDConfig   SourceID = 0      // Singleton config source
	SourceIDPeerMin  SourceID = 1      // First peer ID
	SourceIDPeerMax  SourceID = 99999  // Last peer ID
	SourceIDReserved SourceID = 100000 // Reserved boundary
	SourceIDAPIMin   SourceID = 100001 // First API ID

	InvalidSourceID SourceID = 0xFFFFFFFF // Unset/invalid
)

// SourceType identifies the kind of message source.
type SourceType uint8

const (
	SourceUnknown SourceType = iota
	SourcePeer
	SourceAPI
	SourceConfig
)

// Type returns the source type encoded in this ID.
func (id SourceID) Type() SourceType {
	switch {
	case id == InvalidSourceID:
		return SourceUnknown
	case id == SourceIDConfig:
		return SourceConfig
	case id >= SourceIDPeerMin && id <= SourceIDPeerMax:
		return SourcePeer
	case id >= SourceIDAPIMin && id < InvalidSourceID:
		return SourceAPI
	default:
		return SourceUnknown
	}
}

// IsValid returns true if this is not the invalid/unset ID.
func (id SourceID) IsValid() bool {
	return id != InvalidSourceID
}

// IsPeer returns true if this ID is in the peer range.
func (id SourceID) IsPeer() bool {
	return id >= SourceIDPeerMin && id <= SourceIDPeerMax
}

// IsAPI returns true if this ID is in the API range.
func (id SourceID) IsAPI() bool {
	return id >= SourceIDAPIMin && id < InvalidSourceID
}

// IsConfig returns true if this is the config source ID.
func (id SourceID) IsConfig() bool {
	return id == SourceIDConfig
}

// String returns type:n format with 1-based numbering within each type.
// Examples: "config:1", "peer:1", "peer:42", "api:1", "api:5".
func (id SourceID) String() string {
	var n uint32
	switch {
	case id == SourceIDConfig:
		n = 1
	case id.IsPeer():
		n = uint32(id) // peer IDs already start at 1
	case id.IsAPI():
		n = uint32(id) - uint32(SourceIDAPIMin) + 1 // 100001->1, 100002->2
	default:
		return "unknown"
	}
	return id.Type().String() + ":" + uitoa(n)
}

// uitoa converts uint32 to string without fmt import.
func uitoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var buf [10]byte // max uint32 is 10 digits
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

// ParseSourceID converts "type:n" string to SourceID.
// Returns InvalidSourceID if format invalid or n out of range.
// n is 1-based within each type.
func ParseSourceID(s string) SourceID {
	// Find colon
	colon := -1
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			colon = i
			break
		}
	}
	if colon < 1 || colon >= len(s)-1 {
		return InvalidSourceID
	}

	typ := s[:colon]
	numStr := s[colon+1:]

	// Parse number with overflow protection
	var n uint32
	const maxUint32Div10 = 429496729 // MaxUint32 / 10
	for i := 0; i < len(numStr); i++ {
		c := numStr[i]
		if c < '0' || c > '9' {
			return InvalidSourceID
		}
		digit := uint32(c - '0')
		// Check overflow before multiply
		if n > maxUint32Div10 {
			return InvalidSourceID
		}
		n *= 10
		// Check overflow before add
		if n > 0xFFFFFFFF-digit {
			return InvalidSourceID
		}
		n += digit
	}
	if n == 0 {
		return InvalidSourceID // 1-based, so 0 is invalid
	}

	switch typ {
	case configStr:
		if n != 1 {
			return InvalidSourceID // only config:1 valid
		}
		return SourceIDConfig
	case "peer":
		if n > uint32(SourceIDPeerMax) {
			return InvalidSourceID
		}
		return SourceID(n) // peer IDs are 1-based internally
	case "api":
		// Check overflow before calculating ID
		maxN := uint32(InvalidSourceID) - uint32(SourceIDAPIMin)
		if n > maxN {
			return InvalidSourceID
		}
		id := SourceIDAPIMin + SourceID(n-1) // 1->100001, 2->100002
		return id
	default:
		return InvalidSourceID
	}
}

const (
	unknownStr = "unknown"
	configStr  = "config"
)

// String returns the human-readable name of the source type.
func (t SourceType) String() string {
	switch t {
	case SourceUnknown:
		return unknownStr
	case SourcePeer:
		return "peer"
	case SourceAPI:
		return "api"
	case SourceConfig:
		return configStr
	default:
		return unknownStr
	}
}

// Source contains metadata about a message source.
type Source struct {
	ID     SourceID
	Active bool

	// Peer-specific
	PeerIP netip.Addr
	PeerAS uint32

	// API-specific
	Name string
}

// Type returns the source type (derived from ID).
func (s Source) Type() SourceType {
	return s.ID.Type()
}

// String returns a formatted string identifying this source.
// Format: "config:1", "peer:<ip>", "api:<name>".
func (s Source) String() string {
	switch s.Type() {
	case SourceUnknown:
		return unknownStr
	case SourcePeer:
		return "peer:" + s.PeerIP.String()
	case SourceAPI:
		return "api:" + s.Name
	case SourceConfig:
		return "config:1"
	default:
		return unknownStr
	}
}
