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
// session: the access interface, subscriber MAC and owned transport borrower.
type realmSubscriber struct {
	ifindex int
	ifName  string
	mac     string
	channel callsink.RelayChannel
}

// relayDriver is the part of the PPP driver a realm handoff drives;
// *ppp.Driver implements it.
type relayDriver interface {
	QuiesceSession(request ppp.EventAuthRequest) error
	ReleaseSession(request ppp.EventAuthRequest) error
	RefuseAuth(request ppp.EventAuthRequest, message string) error
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
	subscriber func(ppp.EventAuthRequest) (realmSubscriber, bool)
	endSession func(ppp.EventAuthRequest)
	released   func(ppp.EventAuthRequest) error

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	mu    sync.Mutex
	calls map[pendingAuthKey]*realmHandoff
}

func newRealmRelay(logger *slog.Logger, driver relayDriver,
	subscriber func(ppp.EventAuthRequest) (realmSubscriber, bool), endSession func(ppp.EventAuthRequest), released func(ppp.EventAuthRequest) error,
) *realmRelay {
	return &realmRelay{
		logger:     logger,
		driver:     driver,
		subscriber: subscriber,
		endSession: endSession,
		released:   released,
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
	key := pendingAuthFor(req.Control)
	sub, live := r.subscriber(*req)
	// refuse-by-default: guards startPPPoEAuthDrain -- a session the server dropped is answered by the driver, which no longer holds it
	if !live {
		return false
	}

	h := &realmHandoff{
		relay:   r,
		key:     key,
		request: *req,
		ifindex: sub.ifindex,
		realm:   realm,
		events:  make(chan realmEvent, realmEventsMax),
		done:    make(chan struct{}),
	}
	call, accepted, err := sink.RelayRealm(callsink.RealmRequest{
		Realm:         realm,
		Username:      req.Username,
		Interface:     sub.ifName,
		SubscriberMAC: sub.mac,
		SessionID:     req.SessionID,
		Channel:       sub.channel,
		BeforeBridge:  h,
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
			"realm", realm, "ifindex", sub.ifindex, "session", req.SessionID, "error", err)
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
	previous := r.calls[key]
	r.calls[key] = h
	r.mu.Unlock()
	if previous != nil {
		previous.goneOnce.Do(func() { previous.post(realmEvent{kind: realmEventSubscriberGone}) })
	}
	r.wg.Go(h.run)
	return true
}

// subscriberGone tells the handoff of a PPPoE session that ended (PADT, PPP
// session down) to cancel its call. A bridged session reaches it only
// through PADT: the PPP driver released it, so no PPP session-down comes.
// A second report for the same session is dropped.
func (r *realmRelay) subscriberGone(attempt ppp.SessionAttempt) {
	r.mu.Lock()
	h := r.calls[pendingAuthKey{subscriberID: attempt.SubscriberID(), attempt: attempt}]
	r.mu.Unlock()
	if h == nil {
		return
	}
	h.goneOnce.Do(func() { h.post(realmEvent{kind: realmEventSubscriberGone}) })
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
// Bridged and one Ended, and the server one subscriber-gone (goneOnce).
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
	relay   *realmRelay
	key     pendingAuthKey
	request ppp.EventAuthRequest
	ifindex int
	realm   string
	events  chan realmEvent
	done    chan struct{}

	callMu   sync.Mutex
	call     callsink.RelayCall
	canceled bool
	// Preparation owns local retirement before fresh bridge channel capture.
	// It runs outside any original-transport borrow and retains its exact result.
	prepareMu     sync.Mutex
	prepareDone   bool
	prepareErr    error
	localReleased bool

	// goneOnce posts the one subscriber-gone event: PADT and the PPP
	// session-down can both report the same session.
	goneOnce sync.Once
}

// Quiesce stops the PPP driver's reader on the subscriber's channel and
// returns once it returned (callsink.RealmHandoff).
func (h *realmHandoff) Quiesce() error {
	h.prepareMu.Lock()
	defer h.prepareMu.Unlock()
	if h.localReleased {
		return h.prepareErr
	}
	return h.quiesce()
}

func (h *realmHandoff) quiesce() error {
	select {
	case <-h.done:
		return errRealmHandoffOver
	default:
	}
	return h.relay.driver.QuiesceSession(h.request)
}

// PrepareBridge MUST finish before the sink attaches its fresh native peers.
// The same Driver release path owns native seal confirmation and reporting;
// no file callback or access-table mutex is held across it.
func (h *realmHandoff) PrepareBridge() error {
	h.prepareMu.Lock()
	defer h.prepareMu.Unlock()
	if h.prepareDone {
		return h.prepareErr
	}
	h.prepareDone = true
	if err := h.quiesce(); err != nil {
		h.prepareErr = err
		return err
	}
	if err := h.relay.driver.ReleaseSession(h.request); err != nil {
		h.prepareErr = err
		return err
	}
	h.localReleased = true
	h.prepareErr = h.relay.released(h.request)
	if h.prepareErr != nil {
		h.reportBridgeReleaseFailure(h.prepareErr)
	}
	return h.prepareErr
}

func (h *realmHandoff) bridgePreparationResult() error {
	h.prepareMu.Lock()
	defer h.prepareMu.Unlock()
	if !h.prepareDone {
		return errors.New("pppoe: realm bridge bypassed local PPP release preparation")
	}
	return h.prepareErr
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

func (h *realmHandoff) setCall(call callsink.RelayCall) {
	h.callMu.Lock()
	h.call = call
	canceled := h.canceled
	h.callMu.Unlock()
	// Cancel runs outside callMu: after the bridge it tears the L2TP
	// session down, which reports Ended back into this handoff.
	if canceled && call != nil {
		call.Cancel()
	}
}

func (h *realmHandoff) cancelCall() {
	h.callMu.Lock()
	h.canceled = true
	call := h.call
	h.callMu.Unlock()
	if call != nil {
		call.Cancel()
	}
}

// refuse answers the subscriber's authentication with a failure naming the
// realm and closes its link; the PPP session then ends and the server sends
// PADT. A session already gone needs no answer.
func (h *realmHandoff) refuse(why string) {
	h.prepareMu.Lock()
	released := h.localReleased
	h.prepareMu.Unlock()
	if released {
		// Authentication has no local PPP owner after prepared retirement. The
		// existing exact admission teardown sends PADT and retains native errors.
		h.relay.endSession(h.request)
		return
	}
	message := "realm " + h.realm + ": " + why
	err := h.relay.driver.RefuseAuth(h.request, message)
	if err == nil {
		return
	}
	if errors.Is(err, ppp.ErrSessionNotFound) || errors.Is(err, ppp.ErrRequestExpired) {
		return
	}
	h.relay.logger.Warn("pppoe: realm relay refusal not delivered; the PPP session ends at its auth timeout",
		"realm", h.realm, "ifindex", h.ifindex, "session", h.request.SessionID, "error", err)
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
						"realm", h.realm, "session", h.request.SessionID, "error", ev.err)
					h.cancelCall()
					h.refuse("the L2TP bridge failed")
					return
				}
				if err := h.bridgePreparationResult(); err != nil {
					h.relay.logger.Warn("pppoe: realm relay bridged a session the PPP driver no longer held",
						"realm", h.realm, "session", h.request.SessionID, "error", err)
					h.cancelCall()
					h.refuse("the local PPP release was not confirmed")
					return
				}
				bridged = true
			case realmEventEnded:
				if bridged {
					// The L2TP session owned the subscriber's PPP: end the
					// PPPoE session as for a Service-Name relay.
					h.relay.endSession(h.request)
					return
				}
				h.refuse("the L2TP call ended before the bridge (" + ev.reason + ")")
				return
			case realmEventSubscriberGone:
				// Before the bridge the sink abandons the call; after it
				// the sink tears the bridged L2TP session down with CDN
				// (callsink.RelayCall). Either way the LNS learns.
				h.cancelCall()
				return
			case realmEventUnspecified:
				panic("BUG: realm relay event with no kind")
			}
		}
	}
}

var _ callsink.RealmHandoff = (*realmHandoff)(nil)
var _ callsink.BridgePreparation = (*realmHandoff)(nil)

// realmSubscriber returns what a realm request needs about the PPPoE
// exact admitted request, or false when its transport owner has departed.
func (s *Subsystem) realmSubscriber(request ppp.EventAuthRequest) (realmSubscriber, bool) {
	server, session, found := s.sessionForControl(request.Control, request.SessionID)
	if !found || !request.Active() {
		return realmSubscriber{}, false
	}
	return server.sessions.relaySubscriber(session, server.ifName)
}

// peerLeftSession is the servers' PADT report (InterfaceServer.peerLeft): a
// realm relay for the session cancels its call, which after the bridge
// tears the L2TP session down with CDN.
func (s *Subsystem) peerLeftSession(session *Session) {
	if s.realmRelay != nil {
		s.realmRelay.subscriberGone(session.Attempt)
	}
}

// endRelayedSession ends a PPPoE session whose L2TP call ended after the
// bridge: PADT to the subscriber, then the session's socket is closed.
func (s *Subsystem) endRelayedSession(request ppp.EventAuthRequest) {
	server, session, found := s.sessionForAttempt(request.Control.Attempt(), request.SessionID)
	if found {
		server.handleSessionDown(session)
	}
}

// relaySubscriber copies, under the table's lock, the fields a realm
// request carries for the captured admission.
func (st *SessionTable) relaySubscriber(expected *Session, ifName string) (realmSubscriber, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if expected == nil || st.sessions[expected.SID] != expected {
		return realmSubscriber{}, false
	}
	if expected.Opened == nil || expected.State == StateTeardown {
		return realmSubscriber{}, false
	}
	return realmSubscriber{ifindex: expected.AccessIfIndex, ifName: ifName, mac: expected.MAC.String(), channel: expected.Opened}, true
}

// releaseRelayedSession observes the successful no-Down Driver release. It
// relinquishes only protocol-worker ownership; native confirmation remains a
// separate terminal contract. The transport still owns its exact Opened.
func (s *Subsystem) releaseRelayedSession(request ppp.EventAuthRequest) error {
	server, session, found := s.sessionForControl(request.Control, request.SessionID)
	if !found {
		return errors.New("pppoe: released realm admission departed")
	}
	status, err := session.Attempt.CleanupStatus()
	if err != nil {
		return err
	}
	if !status.Admitted || !status.Finished {
		return errors.New("pppoe: realm PPP worker retirement was not confirmed")
	}
	table := server.sessions
	table.mu.Lock()
	if table.sessions[session.SID] != session || session.Attempt != request.Control.Attempt() || session.Control != request.Control {
		table.mu.Unlock()
		return errors.New("pppoe: released realm admission changed")
	}
	session.pppRelayReleased = true
	ending := session.State == StateTeardown
	table.mu.Unlock()
	if ending {
		server.closeAdmitted(session)
	}
	// Protocol retirement is handed back even while native release is pending,
	// so existing admission teardown can still run. Fresh channel capture needs
	// the stricter physical receipt as well and refuses until it is confirmed.
	if !status.Released {
		return errors.New("pppoe: realm PPP native release remains unconfirmed")
	}
	if status.Err != nil {
		return status.Err
	}
	return nil
}
