// Design: docs/architecture/l2tp/subscriber-session-model.md -- shared subscriber session type

package subscriber

import (
	"net"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/subscriberid"
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
	// L2TPBirth is absent for other access types and omitted from their JSON.
	L2TPBirth       subscriberid.L2TPBirth `json:"l2tp-birth,omitzero"`
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

// PPPKey returns transport-local diagnostic labels. RADIUS uses the session
// label for NAS-Port after it has found the subscriber by canonical identity.
// L2TP labels are wire tunnel/session IDs; PPPoE labels are the low 16 bits of
// its access index and its wire SID. Labels can collide, including across access
// interfaces, and MUST NOT select a PPP worker, admission or subscriber.
// Canonical Session.ID identifies shared state; exact SessionAttempt and
// SessionControl capabilities identify the corresponding PPP admission.
func (s *Session) PPPKey() (tunnelID, sessionID uint16) {
	if s.AccessType == AccessPPPoE {
		return uint16(s.AccessIfIndex), s.PPPoESID
	}
	return s.TunnelID, s.SessionID
}
