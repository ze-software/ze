// Design: docs/architecture/l2tp/subscriber-session-model.md -- subscriber event namespace

package events

import (
	"net/netip"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/core/events"
)

const Namespace = "subscriber"

const (
	SessionUpEvent         = "session-up"
	SessionDownEvent       = "session-down"
	SessionIPAssignedEvent = "session-ip-assigned"
	SessionIPReleasedEvent = "session-ip-released"
	SessionRateChangeEvent = "session-rate-change"
	SessionAuthResultEvent = "session-auth-result"
)

type SessionUpPayload struct {
	Session subscriber.Session
}

type SessionDownPayload struct {
	Session         subscriber.Session
	Reason          string
	NetworkLifetime l2tpevents.NetworkLifetime `json:"network-lifetime,omitzero"`
}

type SessionIPAssignedPayload struct {
	Session subscriber.Session
	// Address is the address assigned by this notice. An IPv6CP interface ID
	// can update Session without assigning a routable host address here: a
	// notice with no Address is the IPv6 family's install, and the route
	// observer announces the IPv6 framed routes on it.
	Address         netip.Addr                 `json:"address,omitzero"`
	FramedRoutes    []l2tpevents.FramedRoute   `json:"framed-routes,omitempty"`
	NetworkLifetime l2tpevents.NetworkLifetime `json:"network-lifetime,omitzero"`
}

// SessionIPReleasedPayload reports that one family's NCP left Opened while the
// session stays up. Session is the registry entry with that family's fields
// already cleared (IPv4: the address and DNS; IPv6: the interface identifier).
// The route observer removes only that family's routes for NetworkLifetime;
// the next SessionIPAssigned of the family reinstalls them.
type SessionIPReleasedPayload struct {
	Session         subscriber.Session
	Family          l2tpevents.AddressFamily   `json:"family"`
	NetworkLifetime l2tpevents.NetworkLifetime `json:"network-lifetime,omitzero"`
}

// ClearReleasedFamily removes from sess the fields one family's NCP
// negotiated, so a registry entry holds only the families that are open.
// IPv4 clears the address and both DNS servers, which IPCP negotiates with
// it (RFC 1877); IPv6 clears the interface identifier. Both transports call
// it before they publish SessionIPReleased.
func ClearReleasedFamily(sess *subscriber.Session, family l2tpevents.AddressFamily) {
	switch family {
	case l2tpevents.AddressFamilyIPv4:
		sess.IPv4Addr = netip.Addr{}
		sess.DNSPrimary = netip.Addr{}
		sess.DNSSecondary = netip.Addr{}
	case l2tpevents.AddressFamilyIPv6:
		sess.IPv6InterfaceID = [8]byte{}
	case l2tpevents.AddressFamilyUnspecified:
		// No producer emits it; there is no family to clear.
	}
}

type SessionRateChangePayload struct {
	SessionID    string
	DownloadRate uint64
	UploadRate   uint64
}

type SessionAuthResultPayload struct {
	SessionID  string
	AccessType subscriber.AccessType
	Username   string
	Accept     bool
	Reason     string
}

var SessionUp = events.Register[*SessionUpPayload](Namespace, SessionUpEvent)
var SessionDown = events.Register[*SessionDownPayload](Namespace, SessionDownEvent)
var SessionIPAssigned = events.Register[*SessionIPAssignedPayload](Namespace, SessionIPAssignedEvent)
var SessionIPReleased = events.Register[*SessionIPReleasedPayload](Namespace, SessionIPReleasedEvent)
var SessionRateChange = events.Register[*SessionRateChangePayload](Namespace, SessionRateChangeEvent)
var SessionAuthResult = events.Register[*SessionAuthResultPayload](Namespace, SessionAuthResultEvent)
