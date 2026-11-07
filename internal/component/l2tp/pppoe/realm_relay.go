// Design: docs/architecture/l2tp/subscriber-session-model.md -- PPPoE auth drain, realm relay decision
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 4.4.5 (LAC proxy LCP and authentication)
// Related: drain.go -- the auth drain that asks for a realm relay first
// Related: ../ppp/relay_handoff.go -- QuiesceSession, ReleaseSession, RefuseAuth

package pppoe

import (
	"errors"
	"log/slog"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/callsink"
)

// errRealmHandoffOver is Quiesce's answer once the PPPoE side abandoned the
// call: the sink then tears the call down instead of bridging.
var errRealmHandoffOver = errors.New("pppoe: realm relay abandoned: the subscriber left or the PPPoE subsystem stopped")

// realmSubscriber is what a realm request needs to know about the PPPoE
// session: the access interface, the subscriber's MAC and its pppox socket.
type realmSubscriber struct {
	ifName  string
	mac     string
	pppoxFD int
}

// relayDriver is the part of the PPP driver a realm handoff drives;
// *ppp.Driver implements it.
type relayDriver interface {
	QuiesceSession(tunnelID, sessionID uint16) error
	ReleaseSession(tunnelID, sessionID uint16) error
	RefuseAuth(tunnelID, sessionID uint16, message string) error
}

var _ relayDriver = (*ppp.Driver)(nil)

// realmRelay is the PPPoE subsystem's side of the realm relays it placed.
// The auth drain calls decide; each accepted relay gets one realmHandoff,
// whose goroutine lives for that relay's lifecycle and ends with it.
// stopAll MUST be called once the PPP driver stopped; it cancels every
// relay still placing its call and waits for every handoff goroutine.
//
// Safe for concurrent use.
type realmRelay struct {
	logger     *slog.Logger
	driver     relayDriver
	subscriber func(ifindex int, sid uint16) (realmSubscriber, bool)
	endSession func(ifindex int, sid uint16)

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	mu    sync.Mutex
	calls map[pendingAuthKey]*realmHandoff
}

func newRealmRelay(logger *slog.Logger, driver relayDriver,
	subscriber func(int, uint16) (realmSubscriber, bool), endSession func(int, uint16),
) *realmRelay {
	return &realmRelay{
		logger:     logger,
		driver:     driver,
		subscriber: subscriber,
		endSession: endSession,
		stop:       make(chan struct{}),
		calls:      make(map[pendingAuthKey]*realmHandoff),
	}
}

// realmRelayConfigured reports whether the registered call sink holds any
// realm relay binding, so a new PPPoE session records its LCP exchange for
// it (ppp.StartSession.CaptureProxyLCP).
func realmRelayConfigured() bool {
	sink := callsink.Lookup()
	// refuse-by-default: guards handlePADR -- no sink is no L2TP subsystem, so no realm binding exists
	if sink == nil {
		return false
	}
	return sink.RealmRelayConfigured()
}

// realmFromUsername returns the realm of a PPP username: the text after its
// last '@'. A name with no '@', or nothing after it, has no realm.
func realmFromUsername(username string) (string, bool) {
	at := strings.LastIndexByte(username, '@')
	if at < 0 {
		return "", false
	}
	if at == len(username)-1 {
		return "", false
	}
	return username[at+1:], true
}

// proxyAuthFrom converts the authentication exchange the driver reported
// into the form the Proxy Authen AVPs carry. MS-CHAPv2 and no-auth have no
// kind here: the sink refuses a matched realm with AuthKindUnspecified, so
// such a subscriber is refused rather than authenticated locally.
func proxyAuthFrom(req *ppp.EventAuthRequest) callsink.ProxyAuth {
	auth := callsink.ProxyAuth{Identifier: req.Identifier, Challenge: req.Challenge, Response: req.Response}
	switch req.Method {
	case ppp.AuthMethodCHAPMD5:
		auth.Kind = callsink.AuthKindCHAP
	case ppp.AuthMethodPAP:
		auth.Kind = callsink.AuthKindPAP
		auth.Challenge = nil
	case ppp.AuthMethodNone, ppp.AuthMethodMSCHAPv2:
		auth.Kind = callsink.AuthKindUnspecified
	}
	return auth
}

// decide asks the call sink to relay req by its realm, before any auth
// handler sees the credential. It returns true when the request is handled
// here (relayed, or refused because its realm matched and the call could not
// start), and false when the subscriber authenticates locally: no realm, no
// sink, no binding for the realm, or a session the server no longer holds.
func (r *realmRelay) decide(req *ppp.EventAuthRequest) bool {
	if req.ProxyLCP == nil {
		return false
	}
	realm, hasRealm := realmFromUsername(req.Username)
	// refuse-by-default: guards startPPPoEAuthDrain -- a name with no realm authenticates locally (AC-5)
	if !hasRealm {
		return false
	}
	sink := callsink.Lookup()
	// refuse-by-default: guards startPPPoEAuthDrain -- with no L2TP subsystem no realm can be relayed
	if sink == nil {
		return false
	}
	key := pendingAuthKey{ifindex: int(req.TunnelID), sessionID: req.SessionID}
	sub, live := r.subscriber(key.ifindex, key.sessionID)
	// refuse-by-default: guards startPPPoEAuthDrain -- a session the server dropped is answered by the driver, which no longer holds it
	if !live {
		return false
	}

	h := &realmHandoff{
		relay:  r,
		key:    key,
		realm:  realm,
		events: make(chan realmEvent, realmEventsMax),
		done:   make(chan struct{}),
	}
	call, accepted, err := sink.RelayRealm(callsink.RealmRequest{
		Realm:         realm,
		Username:      req.Username,
		Interface:     sub.ifName,
		SubscriberMAC: sub.mac,
		SessionID:     req.SessionID,
		ChannelFD:     sub.pppoxFD,
		LCP: callsink.ProxyLCP{
			InitialReceived: req.ProxyLCP.InitialReceived,
			LastSent:        req.ProxyLCP.LastSent,
			LastReceived:    req.ProxyLCP.LastReceived,
		},
		Auth:    proxyAuthFrom(req),
		Handoff: h,
	})
	if err != nil {
		r.logger.Warn("pppoe: realm relay refused the subscriber; it is not authenticated locally",
			"realm", realm, "ifindex", key.ifindex, "session", key.sessionID, "error", err)
		h.refuse("the L2TP call could not start")
		close(h.done)
		return true
	}
	if !accepted {
		close(h.done)
		return false
	}
	h.setCall(call)
	r.mu.Lock()
	r.calls[key] = h
	r.mu.Unlock()
	r.wg.Go(h.run)
	return true
}

// subscriberGone tells the handoff of a PPPoE session that ended (PADT, PPP
// session down) to abandon its call.
func (r *realmRelay) subscriberGone(ifindex int, sid uint16) {
	r.mu.Lock()
	h := r.calls[pendingAuthKey{ifindex: ifindex, sessionID: sid}]
	r.mu.Unlock()
	if h == nil {
		return
	}
	h.post(realmEvent{kind: realmEventSubscriberGone})
}

// stopAll cancels every relay still in flight and waits for every handoff
// goroutine. MUST be called after the PPP driver stopped.
func (r *realmRelay) stopAll() {
	r.stopOnce.Do(func() { close(r.stop) })
	r.wg.Wait()
}

// forget drops a finished handoff from the table.
func (r *realmRelay) forget(h *realmHandoff) {
	r.mu.Lock()
	if r.calls[h.key] == h {
		delete(r.calls, h.key)
	}
	r.mu.Unlock()
}

type realmEventKind uint8

const (
	realmEventUnspecified realmEventKind = iota
	realmEventBridged
	realmEventEnded
	realmEventSubscriberGone
)

// realmEventsMax bounds a handoff's queue: the sink posts at most one
// Bridged and one Ended, and the server one subscriber-gone.
const realmEventsMax = 3

type realmEvent struct {
	kind   realmEventKind
	err    error
	reason string
}

// realmHandoff implements callsink.RealmHandoff for one relayed PPPoE
// session. Quiesce runs in the sink's goroutine; Bridged and Ended only
// queue an event for run, which owns every driver call that can block.
type realmHandoff struct {
	relay  *realmRelay
	key    pendingAuthKey
	realm  string
	events chan realmEvent
	done   chan struct{}

	callMu   sync.Mutex
	call     callsink.RealmCall
	canceled bool
}

// Quiesce stops the PPP driver's reader on the subscriber's channel and
// returns once it returned (callsink.RealmHandoff).
func (h *realmHandoff) Quiesce() error {
	select {
	case <-h.done:
		return errRealmHandoffOver
	default:
	}
	return h.relay.driver.QuiesceSession(uint16(h.key.ifindex), h.key.sessionID)
}

// Bridged queues the bridge result (callsink.RealmHandoff). Never blocks.
func (h *realmHandoff) Bridged(err error) {
	h.post(realmEvent{kind: realmEventBridged, err: err})
}

// Ended queues the end of the L2TP call (callsink.RealmHandoff). Never blocks.
func (h *realmHandoff) Ended(reason string) {
	h.post(realmEvent{kind: realmEventEnded, reason: reason})
}

func (h *realmHandoff) post(ev realmEvent) {
	select {
	case h.events <- ev:
	case <-h.done:
	}
}

func (h *realmHandoff) setCall(call callsink.RealmCall) {
	h.callMu.Lock()
	defer h.callMu.Unlock()
	h.call = call
	if h.canceled && call != nil {
		call.Cancel()
	}
}

func (h *realmHandoff) cancelCall() {
	h.callMu.Lock()
	defer h.callMu.Unlock()
	h.canceled = true
	if h.call != nil {
		h.call.Cancel()
	}
}

// refuse answers the subscriber's authentication with a failure naming the
// realm and closes its link; the PPP session then ends and the server sends
// PADT. A session already gone needs no answer.
func (h *realmHandoff) refuse(why string) {
	message := "realm " + h.realm + ": " + why
	err := h.relay.driver.RefuseAuth(uint16(h.key.ifindex), h.key.sessionID, message)
	if err == nil {
		return
	}
	if errors.Is(err, ppp.ErrSessionNotFound) {
		return
	}
	h.relay.logger.Warn("pppoe: realm relay refusal not delivered; the PPP session ends at its auth timeout",
		"realm", h.realm, "ifindex", h.key.ifindex, "session", h.key.sessionID, "error", err)
}

// run is the handoff's lifecycle: it ends when the call failed, the
// subscriber left, the L2TP call ended, or the subsystem stopped.
func (h *realmHandoff) run() {
	defer h.relay.forget(h)
	defer close(h.done)
	bridged := false
	for {
		select {
		case <-h.relay.stop:
			h.cancelCall()
			return
		case ev := <-h.events:
			switch ev.kind {
			case realmEventBridged:
				if ev.err != nil {
					h.relay.logger.Warn("pppoe: realm relay bridge failed; refusing the subscriber",
						"realm", h.realm, "session", h.key.sessionID, "error", ev.err)
					h.cancelCall()
					h.refuse("the L2TP bridge failed")
					return
				}
				if err := h.relay.driver.ReleaseSession(uint16(h.key.ifindex), h.key.sessionID); err != nil {
					h.relay.logger.Warn("pppoe: realm relay bridged a session the PPP driver no longer held",
						"realm", h.realm, "session", h.key.sessionID, "error", err)
				}
				bridged = true
			case realmEventEnded:
				if bridged {
					// The L2TP session owned the subscriber's PPP: end the
					// PPPoE session as for a Service-Name relay.
					h.relay.endSession(h.key.ifindex, h.key.sessionID)
					return
				}
				h.refuse("the L2TP call ended before the bridge (" + ev.reason + ")")
				return
			case realmEventSubscriberGone:
				if !bridged {
					h.cancelCall()
				}
				return
			case realmEventUnspecified:
				panic("BUG: realm relay event with no kind")
			}
		}
	}
}

var _ callsink.RealmHandoff = (*realmHandoff)(nil)

// realmSubscriber returns what a realm request needs about the PPPoE
// session sid on the interface ifindex, or false when no server or session
// holds it any more.
func (s *Subsystem) realmSubscriber(ifindex int, sid uint16) (realmSubscriber, bool) {
	s.mu.Lock()
	srv := s.servers[ifindex]
	s.mu.Unlock()
	if srv == nil {
		return realmSubscriber{}, false
	}
	return srv.sessions.relaySubscriber(sid, srv.ifName)
}

// endRelayedSession ends a PPPoE session whose L2TP call ended after the
// bridge: PADT to the subscriber, then the session's socket is closed.
func (s *Subsystem) endRelayedSession(ifindex int, sid uint16) {
	s.mu.Lock()
	srv := s.servers[ifindex]
	s.mu.Unlock()
	if srv == nil {
		return
	}
	srv.handleSessionDown(sid)
}

// relaySubscriber copies, under the table's lock, the fields a realm
// request carries for session sid.
func (st *SessionTable) relaySubscriber(sid uint16, ifName string) (realmSubscriber, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	sess := st.sessions[sid]
	if sess == nil {
		return realmSubscriber{}, false
	}
	if sess.PppoxFD < 0 {
		return realmSubscriber{}, false
	}
	return realmSubscriber{ifName: ifName, mac: sess.MAC.String(), pppoxFD: sess.PppoxFD}, true
}
