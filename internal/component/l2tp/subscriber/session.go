// Design: docs/architecture/l2tp/subscriber-session-model.md -- shared subscriber session type

package subscriber

import (
	"net"
	"net/netip"
	"time"
)

type AccessType string

const (
	AccessPPPoE AccessType = "pppoe"
	AccessL2TP  AccessType = "l2tp"
)

type SessionState string

const (
	StateAuthenticating SessionState = "authenticating"
	StateConfiguring    SessionState = "configuring"
	StateActive         SessionState = "active"
	StateTerminating    SessionState = "terminating"
)

// Session is a read-only snapshot of a subscriber session. It is a value
// type with no pointers into transport-internal state.
type Session struct {
	ID              string
	AccessType      AccessType
	State           SessionState
	MAC             net.HardwareAddr
	Username        string
	AccessInterface string

	// PPPoESID and AccessIfIndex together identify a PPPoE session. A
	// PPPoE session id is unique per access interface and not per NAS, so
	// the interface index is half of the identity. Both are zero for L2TP.
	PPPoESID      uint16
	AccessIfIndex int
	ServiceName   string

	// TunnelID and SessionID identify an L2TP session. Both are zero for
	// PPPoE, which has no tunnel.
	TunnelID  uint16
	SessionID uint16
	PeerAddr  netip.AddrPort

	PppInterface  string
	NegotiatedMRU uint16
	AuthMethod    string

	IPv4Addr        netip.Addr
	IPv6InterfaceID [8]byte
	IPv6Prefix      netip.Prefix
	DNSPrimary      netip.Addr
	DNSSecondary    netip.Addr

	PoolName     string
	ServiceGroup string

	DownloadRate uint64
	UploadRate   uint64

	ActivatedAt   time.Time
	AcctSessionID string
}

// PPPKey returns the identifier pair the PPP driver carried for this session
// in ppp.EventIPRequest and ppp.EventAuthRequest. L2TP starts a PPP session
// under its tunnel and session ids; PPPoE starts one under the access
// interface index and the PPPoE session id (pppoe.InterfaceServer.handlePADR
// builds ppp.StartSession that way). These are transport-local driver
// coordinates, not a key for shared subscriber state.
//
// The pair is not unique across transports: an L2TP session and a PPPoE
// session can hold numerically equal pairs. A consumer that needs one
// identity for both transports MUST use ID, as the address pool and shared
// authorization metadata do.
//
// The branch on access type lives here rather than open-coded in each
// consumer because the overload is a property of the driver's request rather
// than of any one consumer. Callers needing numeric driver coordinates use
// this method; callers owning shared subscriber state use ID.
func (s *Session) PPPKey() (tunnelID, sessionID uint16) {
	if s.AccessType == AccessPPPoE {
		return uint16(s.AccessIfIndex), s.PPPoESID
	}
	return s.TunnelID, s.SessionID
}
