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
	// can update Session without assigning a routable host address here.
	Address         netip.Addr                 `json:"address,omitzero"`
	FramedRoutes    []l2tpevents.FramedRoute   `json:"framed-routes,omitempty"`
	NetworkLifetime l2tpevents.NetworkLifetime `json:"network-lifetime,omitzero"`
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
var SessionRateChange = events.Register[*SessionRateChangePayload](Namespace, SessionRateChangeEvent)
var SessionAuthResult = events.Register[*SessionAuthResultPayload](Namespace, SessionAuthResultEvent)
