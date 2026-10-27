// Design: docs/architecture/behavior/fsm.md -- the BGP FSM the strict-mode draft revises
// RFC: rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt -- Sections 5 to 9
// Overview: session_handlers.go -- the OPEN and KEEPALIVE rails this file gates
// Related: peer_bfd.go -- the BFD subscriber that raises the six events
//
// The wire half of draft-ietf-idr-bgp-bfd-strict-mode. The FSM package owns
// the state changes and the ConnectRetryCounter (fsm/fsm.go); everything the
// draft's action lists spell as "sends a KEEPALIVE message", "sends a
// NOTIFICATION message" or "drops the TCP connection" is here, because the FSM
// "only handles state transitions; message sending is external" (fsm.go).
//
// Two entry points. advanceAfterOpen is the Section 8.5.5 fork the OPEN rails
// take once the peer's OPEN is validated and the capabilities are negotiated.
// handleBFDEvent is what the peer's BFD subscriber, its config-reload path and
// the BfdHoldTimer all call to deliver one of FSM events 30 to 35.
package reactor

import (
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// bfdStateReader answers the draft's bfd.SessionState session attribute
// (Section 3, quoting RFC 5880 Section 6.8.1), and WHEN the session entered
// that state.
//
// The time is what draft-ietf-idr-bgp-bfd-strict-mode Section 10 measures: "the
// BFD session has been Up for the desired amount of time". A session that came
// Up ten seconds ago has already served a 300 ms hold-down, and only the entry
// time can say so. Zero means the entry time is unknown.
//
// The last return says whether a BFD session exists at all: false means the
// peer asked for BFD and no session is open, which is NOT the same fact as a
// session that is Down, and the two have different answers in bfdStrictHolds.
type bfdStateReader func() (state api.State, since time.Time, live bool)

// setBFDStateReader wires the peer's BFD session state into the session, so the
// OPEN rail can read the draft's bfd.SessionState attribute. The peer calls it
// once per connection cycle (peer_run.go). A session with no reader runs no
// strict-mode wait, which is every session of a peer that configured no BFD.
func (s *Session) setBFDStateReader(read bfdStateReader) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bfdState = read
}

// applyBFDStrictNegotiation records the draft's two session attributes on the
// FSM, after the OPEN exchange has settled both of them.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 3, attribute 16 (BfdEnabled): "A
// boolean value that is TRUE when BFD is configured and enabled for this BGP
// session." Section 3, attribute 20 (BfdStrictNegotiated): "A boolean value
// that is TRUE when the BFD strict-mode feature capability has been
// successfully negotiated for this BGP session."
//
// The FSM tests the two together in every clause, so it stores the conjunction.
func (s *Session) applyBFDStrictNegotiation() {
	bfd := s.bfdOptIn()
	s.mu.RLock()
	negotiated := s.negotiated
	s.mu.RUnlock()

	enabled := bfd != nil && bfd.Enabled
	strict := enabled && negotiated != nil && negotiated.BFDStrictMode
	s.fsm.SetBFDStrict(strict)

	if !strict {
		return
	}
	s.timers.SetBfdHoldTime(time.Duration(bfd.HoldTime) * time.Second)
	s.timers.SetBfdHoldDown(time.Duration(bfd.HoldDown) * time.Millisecond)

	// FSM Event 34, BfdHoldTimerExpires. Without this registration the timer
	// fires into a nil callback and the three Event 34 arms in fsm.go are
	// unreachable from the wire, which is a HANG: RFC 4271's own HoldTimer
	// bounds the wait only while the negotiated hold time is non-zero, and
	// ResetHoldTimer returns without arming anything when it is zero. The
	// BfdHoldTimer is the whole bound in that case, which is what draft
	// Section 3 attribute 19 says it is for: "Hold timer used when the BGP
	// HoldTime has been negotiated to zero to ensure the BGP session terminates
	// if the associated BFD session does not enter the Up state."
	s.timers.OnBfdHoldTimerExpires(s.raiseBFDHoldTimerExpired)

	// The hold-down expiry re-enters the transition it was holding, so one
	// function owns each release whichever path reaches it.
	s.timers.OnBfdHoldDownTimerExpires(s.releaseBFDHoldDown)
}

// raiseBFDHoldTimerExpired delivers FSM Event 34, BfdHoldTimerExpires. It is
// the timer callback, and the only producer of that event.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 8.5.3, the whole action list:
// "sends a NOTIFICATION message with the error code Cease (6) and error subcode
// BFD Down (10), drops the TCP connection, releases all BGP resources,
// increments the ConnectRetryCounter, ... and changes its state to Idle."
// handleBFDEvent performs the wire half; the corresponding Connect, Active or
// OpenSent FSM handler owns the counter and state change (Sections 8.3.3/8.4.3/8.5.3).
func (s *Session) raiseBFDHoldTimerExpired() {
	sessionLogger().Warn("bfd strict-mode: BfdHoldTimer expired waiting for the BFD session to come up",
		"peer", s.settings.Address,
		"bfd-hold-time", s.timers.BfdHoldTime())
	if err := s.handleBFDEvent(fsm.EventBfdHoldTimerExpires); err != nil {
		sessionLogger().Debug("bfd hold-timer teardown failed",
			"peer", s.settings.Address, "err", err)
	}
}

// releaseBFDHoldDown runs when the BFD session has been Up for the whole
// hold-down interval of draft-ietf-idr-bgp-bfd-strict-mode Section 10. It
// completes whichever transition the interval was holding: the OpenConfirm
// KEEPALIVE that was withheld, or the OpenSentConfirmedBfdUpPending release.
// It MUST enter without openMu: handleKeepalive and handleBFDEvent acquire
// that lock themselves to serialize their pre-establishment action lists.
func (s *Session) releaseBFDHoldDown() {
	s.bfdHoldDownObserved.Store(true)
	sessionLogger().Info("bfd strict-mode: hold-down interval observed, establishing",
		"peer", s.settings.Address, "hold-down", s.timers.BfdHoldDown())

	if s.bfdHoldDownKeepalive.Swap(false) {
		if err := s.handleKeepalive(); err != nil {
			sessionLogger().Debug("bfd hold-down keepalive release failed",
				"peer", s.settings.Address, "err", err)
		}
		return
	}
	if err := s.handleBFDEvent(fsm.EventBfdUp); err != nil {
		sessionLogger().Debug("bfd hold-down release failed",
			"peer", s.settings.Address, "err", err)
	}
}

// bfdHoldDownPending answers whether the transition to Established must wait
// out the hold-down interval, arming the timer for whatever is LEFT of it.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 10: "If both the local and remote
// BGP speakers include the BFD Strict-Mode Capability, the BGP state machine is
// permitted to transition to the Established state from the OpenConfirm state
// after the locally configured BFD hold-down interval is observed. That is, the
// BFD session has been Up for the desired amount of time."
//
// Two things follow from that sentence, and an earlier version of this code got
// both wrong by gating the OPEN rail instead.
//
// The interval is measured from when the BFD session came UP, not from when the
// BGP session noticed. Section 7 has ze open the BFD session before the FSM
// starts and keep it open across a teardown, so on a connect-retry the session
// is usually Up again well before the new OPEN arrives; measuring from the OPEN
// would restart a wait the link has already served, and gating the OPEN rail at
// all let the common case skip the interval entirely.
//
// And the gate belongs on the transition to ESTABLISHED, which is where the
// draft puts it, so the timing of the OPEN cannot bypass it. That is what makes
// the damping real for the case it exists for: a link that flaps tears the BGP
// session down (Section 8.7.2), the BFD session stays open, and the next
// attempt must serve the interval before it establishes again.
//
// It answers true only when it armed a timer, so a zero interval, an interval
// already served, and an unknown entry time each fall through to the release
// rather than stalling the session on a timer that will never fire
// (ai/rules/principles.md: a value that is silently wrong must not be reachable).
func (s *Session) bfdHoldDownPending() bool {
	if s.bfdHoldDownObserved.Load() {
		return false
	}
	interval := s.timers.BfdHoldDown()
	if interval <= 0 {
		return false
	}

	s.mu.RLock()
	read := s.bfdState
	s.mu.RUnlock()
	if read == nil {
		return false
	}
	state, since, live := read()
	if !live || state != api.StateUp {
		// Not Up, so there is no Up time to measure. The session is held by
		// bfdStrictHolds rather than by the interval.
		return false
	}
	if since.IsZero() {
		// The entry time is unknown, so the whole interval is owed: a wait that
		// is too long is safe, and skipping it is the thing this damps.
		return s.timers.StartBfdHoldDownTimer()
	}
	// s.clock, not time.Since: the timer this arms runs on the same injected
	// clock, so measuring the elapsed part on the wall clock makes the pair
	// disagree the moment a test drives one of them, and makes a fake-clock case
	// depend on how much real time its own setup took.
	if remaining := interval - s.clock.Now().Sub(since); remaining > 0 {
		return s.timers.StartBfdHoldDownTimerFor(remaining)
	}
	s.bfdHoldDownObserved.Store(true)
	return false
}

// raiseBFDStrictConfigChanged delivers FSM Event 35, BfdStrictConfigChanged,
// after a reload changed this peer's strict-mode configuration.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 4, Event 35 MUST NOT: "If
// BfdEnabled is FALSE, this event MUST NOT occur. When BFD has been disabled,
// the local system will trigger a BfdAdminDown event instead." This function is
// the only producer of the event, so the guard lives here.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 8.1 gives the reason the event
// exists: "If BFD strict-mode configuration is changed once the BGP FSM has
// started executing, but has not reached the Established state, the session is
// reset to the Idle state to ensure consistent behavior. I.e., no unexpected
// timers are running, and the BGP session's transition to Established is not
// lingering on a pending event".
func (s *Session) raiseBFDStrictConfigChanged() {
	bfd := s.bfdOptIn()

	event := fsm.EventBfdStrictConfigChanged
	if bfd == nil || !bfd.Enabled {
		event = fsm.EventBfdAdminDown
	}
	if err := s.handleBFDEvent(event); err != nil {
		sessionLogger().Debug("bfd strict-mode config change event failed",
			"peer", s.settings.Address, "event", event.String(), "err", err)
	}
}

// bfdStrictConfigChanged reports whether the strict-mode configuration of a
// peer differs between two settings, which is the condition
// draft-ietf-idr-bgp-bfd-strict-mode Section 4 Event 35 names: "The
// configuration for the BFD strict configuration for the BGP session has been
// changed."
//
// A peer that gained or lost its whole bfd block changed it, and so did one
// whose block stayed and whose strict, hold-time, hold-down or enabled leaf
// moved. hold-down is in that list because it is strict-mode configuration by
// the draft's own Section 10 and nothing else reads it: a reload that changed
// only the damping interval would otherwise restart the peer with no Event 35
// and no NOTIFICATION saying why.
func bfdStrictConfigChanged(current, next *PeerSettings) bool {
	if current == nil || next == nil {
		return false
	}
	a, b := current.BFD, next.BFD
	if a == nil || b == nil {
		return (a == nil) != (b == nil)
	}
	return a.Strict != b.Strict || a.HoldTime != b.HoldTime ||
		a.HoldDown != b.HoldDown || a.Enabled != b.Enabled
}

// bfdStrictHolds reports whether the strict-mode wait applies right now.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 8.5.5: "If BfdEnabled is TRUE, and
// BfdStrictNegotiated is TRUE, and bfd.SessionState is neither Up nor
// AdminDown". The first two are the flag the FSM holds; the third is read live,
// because the BFD session of a strict peer is opened before the FSM starts
// (Section 7) and can have reached Up before the peer's OPEN arrived.
//
// A strict session whose BFD session does not exist HOLDS. Nothing else is
// safe: the operator asked for a BGP session gated on a forwarding-path check,
// and no check is running, so establishing would deliver the opposite of what
// was configured. The doctor check and the config validator are what stop a
// peer reaching this state silently.
func (s *Session) bfdStrictHolds() bool {
	if !s.fsm.BFDStrict() {
		return false
	}
	s.mu.RLock()
	read := s.bfdState
	s.mu.RUnlock()
	if read == nil {
		return true
	}
	state, _, live := read()
	if !live {
		return true
	}
	return state != api.StateUp && state != api.StateAdminDown
}

// advanceAfterOpen handles the shared negotiated strict-mode decision for
// Event 19 and DelayOpen Event 20 (draft Sections 8.3.5, 8.4.5 and 8.5.5).
// Both OPEN rails call it with openMu held, serializing the decision and entry
// with BFD releases. The session sends OPEN before withholding KEEPALIVE.
func (s *Session) advanceAfterOpen(conn net.Conn) error {
	delayed := s.timers.IsDelayOpenTimerRunning()
	if delayed {
		if err := s.sendOpen(conn); err != nil {
			return err
		}
	}
	if s.bfdStrictHolds() {
		if delayed {
			return s.fsm.EnterDelayOpenBfdUpPending()
		}
		if err := s.fsm.EnterBfdUpPending(); err != nil {
			return err
		}
		// "sets the HoldTimer according to the negotiated value" is in BOTH
		// branches of Section 8.5.5, above the fork, so it runs here too.
		s.timers.ResetHoldTimer()
		if s.timers.HoldTime() == 0 {
			s.timers.StartBfdHoldTimer()
		}
		sessionLogger().Info("bfd strict-mode: holding BGP session in OpenSent until BFD is up",
			"peer", s.settings.Address,
			"bfd-hold-time", s.timers.BfdHoldTime())
		return nil
	}
	if delayed {
		if err := s.fsm.Event(fsm.EventBGPOpenWithDelayOpenTimer); err != nil {
			return err
		}
		return s.sendKeepalive(conn)
	}

	if err := s.fsm.Event(fsm.EventBGPOpen); err != nil {
		return err
	}
	if err := s.sendKeepalive(conn); err != nil {
		return err
	}
	s.timers.ResetHoldTimer()
	return nil
}

// handleBFDEvent delivers one of the six draft events and performs its wire
// actions, including the withheld KEEPALIVE in Sections 8.3.1, 8.4.1 and 8.5.1.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 9: "When BGP sessions are closed
// according to the procedures in this document, the session SHOULD be
// terminated with a NOTIFICATION message with the Cease Code (6) and the 'BFD
// Down' Subcode (10); see [RFC9384]." Configuration changes instead use Other
// Configuration Change (6) in Sections 8.5.4/8.6.3, or close silently in
// Connect/Active (Sections 8.3.4/8.4.4). Second OPEN uses FSM Error in handleOpen.
//
// The teardown clauses share teardownSession: seal, stop timers, optionally
// notify, drop TCP and wake Run. The FSM event applies the clause's own
// ConnectRetryCounter rule.
//
// Ordering on the advancing path: the KEEPALIVE goes out BEFORE the FSM event,
// which is the order Section 8.5.1 writes and the order the wire needs. A
// session that reached Established first would let an UPDATE overtake the
// KEEPALIVE that confirms the OPEN.
func (s *Session) handleBFDEvent(event fsm.Event) error {
	// An Up arriving between the state snapshot and strict Event 20 entry
	// must release that entry, not disappear in the non-pending Connect state.
	s.openMu.Lock()
	defer s.openMu.Unlock()
	state := s.fsm.State()
	pending := s.fsm.BfdSubState()

	if subcode, reason, tear := s.bfdTeardown(event, state); tear {
		sessionLogger().Info("bfd strict-mode: closing BGP session",
			"peer", s.settings.Address,
			"state", state.String(),
			"event", event.String(),
			"reason", reason)
		if event == fsm.EventBfdStrictConfigChanged &&
			(state == fsm.StateConnect || state == fsm.StateActive) {
			// Sections 8.3.4/8.4.4 drop TCP without a NOTIFICATION.
			return s.teardownSession(nil, event)
		}
		return s.teardown(subcode, reason, event)
	}

	// Sections 8.3.1, 8.4.1 and 8.5.1 send the withheld KEEPALIVE
	// before leaving a pending sub-state. The FSM chooses the next state.
	advancing := pending != fsm.SubStateNone &&
		(event == fsm.EventBfdUp || event == fsm.EventBfdAdminDown || event == fsm.EventBfdDisabled)
	// draft-ietf-idr-bgp-bfd-strict-mode Section 10 gates the transition to
	// ESTABLISHED, and only ONE of the two release paths goes there: Section
	// 8.5.1's OpenSentConfirmedBfdUpPending branch, where the peer's KEEPALIVE
	// already arrived. The OpenSentBfdUpPending branch goes to OpenConfirm, and
	// the interval is served at that state's own KEEPALIVE instead
	// (handleKeepalive), so no session serves it twice.
	if advancing && pending == fsm.SubStateOpenSentConfirmedBfdUpPending && s.bfdHoldDownPending() {
		sessionLogger().Info("bfd strict-mode: BFD is up, observing the hold-down interval before establishing",
			"peer", s.settings.Address,
			"hold-down", s.timers.BfdHoldDown())
		return nil
	}
	if advancing {
		// The wait is over however it ended, so the interval stops with it: a
		// timer left armed here fires later on an Established session and
		// re-enters a release nothing is waiting for.
		s.timers.StopBfdHoldDownTimer()
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()
		if err := s.sendKeepalive(conn); err != nil {
			return err
		}
	}

	err := s.fsm.Event(event)

	// draft-ietf-idr-bgp-bfd-strict-mode Section 8.5.1: "sets a KeepaliveTimer
	// (via the text below)". On the OpenSentConfirmedBfdUpPending path no
	// KEEPALIVE is received in OpenConfirm, so handleKeepalive never runs and
	// this is the only place the two send-side timers start.
	if advancing && s.fsm.State() == fsm.StateEstablished {
		s.timers.StartKeepaliveTimer()
		s.startSendHoldTimer()
	}
	return err
}

// bfdTeardown answers whether the draft's clause for this event in this state
// closes the BGP session, and with which Cease subcode. Kept apart from
// handleBFDEvent so the table of clauses reads as a table.
//
// Every "false" here is a clause that says the local system "stays in" its
// state, or that the event "is ignored". The FSM holds the same table and is
// what actually declines to move; this one decides only what reaches the wire.
func (s *Session) bfdTeardown(event fsm.Event, state fsm.State) (uint8, string, bool) {
	strict := s.fsm.BFDStrict()

	//exhaustive:ignore // Only BFD events requiring wire teardown are selected here; FSM.Event handles transitions.
	switch event {
	case fsm.EventBfdDown:
		// draft-ietf-idr-bgp-bfd-strict-mode Section 8.7.2 closes an
		// Established session unconditionally: that is the RFC 5882 Section
		// 4.2 failure detector ze has always run. Sections 8.5.2 and 8.6.2
		// close an OpenSent or OpenConfirm one only "if BfdEnabled is TRUE,
		// and BfdStrictNegotiated is TRUE"; otherwise the local system "stays
		// in the OpenSent State". Sections 8.3.2 and 8.4.2 ignore it outright
		// in Connect and Active.
		if state == fsm.StateEstablished {
			return message.NotifyCeaseBFDDown, "BFD detected forwarding path down", true
		}
		if strict && (state == fsm.StateOpenSent || state == fsm.StateOpenConfirm) {
			return message.NotifyCeaseBFDDown, "BFD strict-mode: session went down before establishment", true
		}
		return 0, "", false

	case fsm.EventBfdHoldTimerExpires:
		// draft-ietf-idr-bgp-bfd-strict-mode Sections 8.3.3, 8.4.3 and 8.5.3.
		// The draft writes no clause for the other three states, and its
		// Section 4 Event 34 note says the timer SHOULD only run in these.
		//
		// Section 10's logging SHOULD is NOT this close: it names a close "due
		// to hold timer expiration", whose producer is the OnHoldTimerExpires
		// closure in NewSession (session.go), and that is where the line is.
		// This close has its own line in handleBFDEvent below.
		if state == fsm.StateConnect || state == fsm.StateActive || state == fsm.StateOpenSent {
			return message.NotifyCeaseBFDDown, "BFD strict-mode: BfdHoldTimer expired waiting for BFD up", true
		}
		return 0, "", false

	case fsm.EventBfdStrictConfigChanged:
		// draft-ietf-idr-bgp-bfd-strict-mode Sections 8.5.4 and 8.6.3 send "an
		// error code Cease (6), error subcode Other Configuration Change (6)".
		// Sections 8.3.4 and 8.4.4 close without a NOTIFICATION;
		// handleBFDEvent selects that silent teardown even with live TCP.
		// Section 8.7.3 ignores the event in Established.
		if state == fsm.StateIdle || state == fsm.StateEstablished {
			return 0, "", false
		}
		return message.NotifyCeaseOtherConfigChange, "BFD strict-mode configuration changed", true
	}
	return 0, "", false
}
