// Design: docs/architecture/isis/isis-3-l2-transport.md -- ISO multicast MAC selection
//
// IS-IS on a LAN (and, per the umbrella Frame-addressing contract, also on
// point-to-point circuits) addresses PDUs to ISO/IEC 10589 reserved multicast
// MAC groups, NOT to a learned neighbor unicast MAC. The level selects the
// group on send; AllISs is additionally accepted on receive.
//
// ISO/IEC 10589 (and the research guide sec 6 "Circuit Types and Network
// Model"): AllL1ISs = 01:80:c2:00:00:14, AllL2ISs = 01:80:c2:00:00:15,
// AllISs = 09:00:2b:00:00:05.

package transport

// Level is the IS-IS routing level a frame is associated with. It selects the
// destination multicast group on send. A circuit configured for both levels
// sends to both groups (one frame each); the transport does not invent an L1L2
// combined group.
type Level uint8

// IS-IS routing levels (ISO/IEC 10589). LevelNone is the zero value and is not
// a valid send target; callers select L1, L2, or both explicitly.
const (
	LevelNone Level = 0
	Level1    Level = 1
	Level2    Level = 2
)

// String renders the level for logs and metrics (lowercase, matching the
// umbrella metrics label convention `level` = `l1`|`l2`).
func (l Level) String() string {
	switch l {
	case Level1:
		return "l1"
	case Level2:
		return "l2"
	default:
		return "none"
	}
}

// MACLen is the length of an Ethernet/ISO MAC address in octets.
const MACLen = 6

// ISO multicast destination MAC groups (ISO/IEC 10589; research guide sec 6).
// These are fixed 6-octet values carried big-endian on the wire.
var (
	// AllL1ISs is the destination group for Level-1 PDUs (01:80:c2:00:00:14).
	AllL1ISs = [MACLen]byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x14}
	// AllL2ISs is the destination group for Level-2 PDUs (01:80:c2:00:00:15).
	AllL2ISs = [MACLen]byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x15}
	// AllISs is the legacy "all intermediate systems" group (09:00:2b:00:00:05).
	// Ze sends to the level-specific groups but ACCEPTS frames addressed to
	// AllISs on receive (umbrella Frame-addressing contract).
	AllISs = [MACLen]byte{0x09, 0x00, 0x2b, 0x00, 0x00, 0x05}
)

// MulticastMACForLevel returns the ISO multicast destination MAC for the given
// level: AllL1ISs for Level1, AllL2ISs for Level2. The boolean is false for any
// other level (including LevelNone), which has no single send group -- a
// dual-level circuit sends to both groups by calling this once per level.
//
// ISO/IEC 10589 sec 6 / research guide sec 6: IS-IS packets are sent to the
// appropriate multicast address based on the PDU type and level.
func MulticastMACForLevel(l Level) ([MACLen]byte, bool) {
	switch l {
	case Level1:
		return AllL1ISs, true
	case Level2:
		return AllL2ISs, true
	default:
		return [MACLen]byte{}, false
	}
}

// IsISMulticastMAC reports whether dst is one of the three ISO multicast groups
// IS-IS uses (AllL1ISs, AllL2ISs, AllISs). The receive path uses this to accept
// frames the local node should process; frames to any other group are ignored
// by the higher layers (level/area enforcement is isis-5).
func IsISMulticastMAC(dst [MACLen]byte) bool {
	return dst == AllL1ISs || dst == AllL2ISs || dst == AllISs
}
