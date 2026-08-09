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

	PPPoESID    uint16
	ServiceName string

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
