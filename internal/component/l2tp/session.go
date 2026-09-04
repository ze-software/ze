// Design: docs/research/l2tpv2-implementation-guide.md -- S10 session state machines
// Related: tunnel.go -- L2TPTunnel owns the session map
// Related: tunnel_fsm.go -- handleMessage dispatches session-scoped messages

package l2tp

import (
	"crypto/rand"
	"encoding/binary"
	"net/netip"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// L2TPSessionState enumerates the session FSM states from RFC 2661 S10.
// The set covers both incoming and outgoing call flows.
type L2TPSessionState uint8

// Session FSM states.
const (
	L2TPSessionIdle         L2TPSessionState = iota // no call
	L2TPSessionWaitTunnel                           // LAC: waiting for tunnel to establish (deferred)
	L2TPSessionWaitReply                            // sent ICRQ or OCRQ, waiting for peer reply
	L2TPSessionWaitConnect                          // LNS incoming: sent ICRP, waiting ICCN; LNS outgoing: got OCRP, waiting OCCN
	L2TPSessionWaitCSAnswer                         // LAC outgoing: sent OCRP, waiting bearer answer
	L2TPSessionEstablished                          // call connected
)

// String returns the lowercase name of the state. Used in logs.
func (s L2TPSessionState) String() string {
	switch s {
	case L2TPSessionIdle:
		return "idle"
	case L2TPSessionWaitTunnel:
		return "wait-tunnel"
	case L2TPSessionWaitReply:
		return "wait-reply"
	case L2TPSessionWaitConnect:
		return "wait-connect"
	case L2TPSessionWaitCSAnswer:
		return "wait-cs-answer"
	case L2TPSessionEstablished:
		return bucketStateEstablishedStr
	}
	return stateUnknown
}

// L2TPSession carries one call's state within a tunnel. NOT safe for
// concurrent use; only the reactor goroutine accesses its fields (same
// guarantee as L2TPTunnel).
//
// RFC 2661 Section 10: each session has its own state machine, but all
// session control messages are sequenced by the tunnel's ReliableEngine.
//
// Fields are added incrementally as each FSM handler is implemented.
// Phase 6 (PPP) consumes the proxy LCP/auth fields.
type L2TPSession struct {
	localSID  uint16
	remoteSID uint16
	state     L2TPSessionState

	// createdAt is the time.Now() when the session was allocated
	// (handleICRQ on LNS side, handleOCRQ on LAC side). Used by the
	// CLI snapshot to report uptime. Immutable after creation.
	createdAt time.Time

	// assignedAddr is the peer IP address the PPP NCP layer negotiated
	// (IPCP for IPv4, IPv6CP interface-ID for IPv6). Zero-valued until
	// EventSessionIPAssigned arrives. Populated by handlePPPEvent in
	// the reactor; the RouteObserver uses this to inject the /32 or
	// /128 into the protocol RIB.
	//
	// Caller MUST hold the owning reactor's tunnelsMu.
	assignedAddr netip.Addr

	// username is the PPP-authenticated peer identity. Populated from
	// proxyAuthenName on ICCN (LAC->LNS proxy auth) or from the auth
	// plugin response (spec-l2tp-8). Empty until populated.
	//
	// Caller MUST hold the owning reactor's tunnelsMu.
	username string

	// Connection parameters captured from ICCN/OCCN.
	txConnectSpeed     uint32
	rxConnectSpeed     uint32
	framingType        uint32
	sequencingRequired bool

	// Proxy LCP state captured from ICCN (phase 6 PPP engine consumes these).
	// RFC 2661 Section 18.
	proxyInitialRecvLCPConfReq []byte
	proxyLastSentLCPConfReq    []byte
	proxyLastRecvLCPConfReq    []byte

	// Proxy authentication captured from ICCN (phase 6 PPP engine consumes these).
	// RFC 2661 Section 18.
	proxyAuthenType      uint16
	proxyAuthenName      string
	proxyAuthenChallenge []byte
	proxyAuthenID        uint8
	proxyAuthenResponse  []byte

	// WEN error counters (RFC 2661 S19.1). Updated on each WEN received.
	callErrors CallErrorsValue

	// SLI ACCM values (RFC 2661 S19.2). Updated on each SLI received.
	accm ACCMValue

	// kernelSetupNeeded is set to true by handleICCN/handleOCCN when the
	// session transitions to L2TPSessionEstablished. The reactor checks
	// this flag after Process() returns and enqueues a kernelSetupEvent.
	// Phase 5 kernel integration.
	kernelSetupNeeded bool

	// lnsMode records whether this session was established via handleICCN
	// (LNS side, true) or handleOCCN (LAC side, false). Used by the
	// kernel worker to set L2TP_ATTR_LNS_MODE.
	lnsMode bool

	// callingNumber is the Calling Number AVP (RFC 2661 Section 4.4.3) the
	// peer sent on the ICRQ, or the value ze put on an ICRQ it originated.
	// It is what RADIUS accounting reports as Calling-Station-Id (RFC 2865
	// Section 5.31). Empty when neither side named one, and an empty value
	// sends no attribute (RFC 2865 Section 5).
	callingNumber string

	// pppInterface is the kernel pppN interface name (e.g. "ppp0").
	// Set by handleKernelSuccess when the PPP session is started.
	// Used by SessionUp EventBus event so the shaper can apply TC.
	pppInterface string

	// pppoeChannelFD is the relayed PPPoE session's kernel PPP channel fd for
	// a LAC-relayed incoming call (spec-followup-l2tp-call AC-3). The reactor
	// bridges it to the pppol2tp channel (A-4) at kernel setup. Zero for
	// sessions not originating from a PPPoE relay.
	pppoeChannelFD int

	// RADIUS timeout cancellation. Set by handleSessionUp when the
	// RADIUS Access-Accept carried Session-Timeout or Idle-Timeout.
	// Canceled on session teardown. nil when no timeout is active.
	sessionTimeoutCancel func()
	idleTimeoutCancel    func()

	// callResult carries the outcome of an operator-initiated call
	// (spec-followup-l2tp-call AC-4) back to the blocking RPC. Set when a
	// pendingCall with a result channel is placed on this session
	// (placePendingCallLocked); nil for peer-initiated sessions and
	// fire-and-forget (config-driven relay) calls. Signaled exactly once by
	// resolveCall: success when the session establishes, failure when it is
	// torn down before establishing. Buffered (cap 1) so the reactor never
	// blocks even if the RPC caller already timed out.
	callResult chan callOutcome

	fsmHistory *fsmHistoryRing
}

// resolveCall delivers the call outcome to a waiting RPC exactly once and
// clears the channel so a later teardown signal is a no-op. Non-blocking:
// callResult is buffered (cap 1) and a caller that already timed out simply
// leaves the value unread. No-op when callResult is nil (peer-initiated or
// fire-and-forget session). Caller MUST hold the owning reactor's tunnelsMu.
func (s *L2TPSession) resolveCall(o callOutcome) {
	if s.callResult == nil {
		return
	}
	s.callResult <- o
	s.callResult = nil
}

// State returns the session's current FSM state.
func (s *L2TPSession) State() L2TPSessionState { return s.state }

func (s *L2TPSession) transition(to L2TPSessionState, trigger string) {
	from := s.state
	s.state = to
	if s.fsmHistory != nil {
		s.fsmHistory.append(FSMTransition{
			Timestamp: time.Now(),
			From:      from.String(),
			To:        to.String(),
			Trigger:   trigger,
		})
	}
}

// LocalSID returns the session ID we assigned. Immutable after creation.
func (s *L2TPSession) LocalSID() uint16 { return s.localSID }

// RemoteSID returns the peer's Assigned Session ID.
func (s *L2TPSession) RemoteSID() uint16 { return s.remoteSID }

// CreatedAt returns the wall-clock time the session was allocated.
// Caller MUST hold the owning reactor's tunnelsMu.
func (s *L2TPSession) CreatedAt() time.Time { return s.createdAt }

// AssignedAddr returns the peer IP negotiated via IPCP / IPv6CP, or a
// zero netip.Addr when no NCP has completed yet.
// Caller MUST hold the owning reactor's tunnelsMu.
func (s *L2TPSession) AssignedAddr() netip.Addr { return s.assignedAddr }

// Username returns the PPP-authenticated peer identity, or "" when
// authentication is not yet complete or was not required.
// Caller MUST hold the owning reactor's tunnelsMu.
func (s *L2TPSession) Username() string { return s.username }

// maxAllocRetries caps the session ID collision retry loop to prevent
// infinite spinning when the ID space is exhausted.
const maxAllocRetries = 100

// allocateSessionID picks a random non-zero uint16 that is not already
// in use by this tunnel's session map. Returns 0 if the ID space is
// exhausted after maxAllocRetries attempts.
//
// RFC 2661: Session ID 0 is reserved and never assigned.
func (t *L2TPTunnel) allocateSessionID() uint16 {
	var buf [2]byte
	for range maxAllocRetries {
		if _, err := rand.Read(buf[:]); err != nil {
			return 0
		}
		sid := binary.BigEndian.Uint16(buf[:])
		if sid == 0 {
			continue
		}
		if _, exists := t.sessions[sid]; !exists {
			return sid
		}
	}
	return 0
}

// addSession inserts a session into the tunnel's session map.
// Caller MUST have verified that localSID is not already present.
func (t *L2TPTunnel) addSession(s *L2TPSession) {
	if t.sessions == nil {
		t.sessions = make(map[uint16]*L2TPSession)
	}
	t.sessions[s.localSID] = s
}

// sessionDown is one session a teardown removed from the tunnel, held on the
// tunnel until the reactor can emit (l2tp, session-down) for it after it
// releases tunnelsMu.
//
// The username is captured at removal because it lives only on the session
// struct the removal drops: a read taken after the map is cleared returns an
// empty string, which reaches RADIUS as a Stop record with no User-Name.
type sessionDown struct {
	// localTID travels with the event because a single drain can carry
	// sessions from more than one tunnel: a tie-breaker loss discards a
	// tunnel whose local TID is not the one the inbound packet named.
	localTID uint16
	localSID uint16
	username string
	cause    l2tpevents.TerminateCause
}

// removeSession is the ONE place a session leaves a tunnel, so it performs
// every obligation a removal carries: it cancels the session and idle
// timers, drops the session metadata, queues the kernel teardown when kernel
// resources exist, fails a call still waiting on an RPC, and queues the
// (l2tp, session-down) the reactor emits after it unlocks.
//
// cause is why this session ended, and it reaches RADIUS accounting as the
// Acct-Terminate-Cause of RFC 2866 Section 5.10. Every caller MUST name one
// that is true of its own path: a peer CDN names what the CDN Result Code
// says, a tunnel that went away names Lost Service, and an error ze detected
// names NAS Error.
//
// Removing a session the tunnel does not hold is a no-op and queues nothing.
func (t *L2TPTunnel) removeSession(sid uint16, cause l2tpevents.TerminateCause) {
	sess := t.sessions[sid]
	if sess == nil {
		return
	}
	cancelSessionTimeouts(sess)
	ClearSessionMetadata(t.localTID, sid)
	if sess.state == L2TPSessionEstablished {
		t.pendingKernelTeardowns = append(t.pendingKernelTeardowns, kernelTeardownEvent{
			localTID: t.localTID,
			localSID: sid,
		})
	}
	// AC-4: an operator call torn down before it established fails its RPC.
	// No-op once the session established (resolveCall already fired success).
	sess.resolveCall(callOutcome{err: errCallTornDown})
	t.pendingSessionDowns = append(t.pendingSessionDowns, sessionDown{
		localTID: t.localTID,
		localSID: sid,
		username: sess.username,
		cause:    cause,
	})
	delete(t.sessions, sid)
}

// drainPendingTeardowns returns everything the removals queued on this tunnel
// and resets both lists: the kernel teardown events for the kernel worker,
// and one sessionDown for every session removed.
//
// The two lists differ deliberately. A kernel teardown exists only for a
// session that reached established, because only that session holds kernel
// resources. A sessionDown exists for every session removed, because the
// subscribers of the event decide for themselves whether they hold state for
// it.
//
// Caller MUST hold tunnelsMu, and MUST emit the session-downs after it
// unlocks (emitSessionDowns) so a subscriber cannot re-enter the reactor
// under the lock.
func (t *L2TPTunnel) drainPendingTeardowns() ([]kernelTeardownEvent, []sessionDown) {
	teardowns := t.pendingKernelTeardowns
	t.pendingKernelTeardowns = nil
	downs := t.pendingSessionDowns
	t.pendingSessionDowns = nil
	return teardowns, downs
}

// sessionCount returns the number of active sessions on this tunnel.
func (t *L2TPTunnel) sessionCount() int {
	return len(t.sessions)
}

// lookupSession finds a session by our local session ID (the ID in the
// header of inbound packets). Returns nil if not found.
func (t *L2TPTunnel) lookupSession(localSID uint16) *L2TPSession {
	return t.sessions[localSID]
}

// lookupSessionByRemote finds a session by the peer's assigned session ID.
// Used to detect duplicate ICRQ/OCRQ from a malicious peer sending the
// same Assigned Session ID with different Ns values. Linear scan is
// acceptable: session counts per tunnel are small (typically < 100).
func (t *L2TPTunnel) lookupSessionByRemote(remoteSID uint16) *L2TPSession {
	for _, s := range t.sessions {
		if s.remoteSID == remoteSID {
			return s
		}
	}
	return nil
}

// clearSessions removes every session from the tunnel and returns the ones it
// removed, for the caller's logging. Used whenever the tunnel itself ends:
// RFC 2661 Section 6.4 says a StopCCN implicitly clears all active sessions.
//
// cause carries the obligation removeSession states, and it is the same for
// every session here because they all end for the one reason the tunnel did.
func (t *L2TPTunnel) clearSessions(cause l2tpevents.TerminateCause) []*L2TPSession {
	if len(t.sessions) == 0 {
		return nil
	}
	result := make([]*L2TPSession, 0, len(t.sessions))
	for _, s := range t.sessions {
		result = append(result, s)
	}
	// The sessions are collected before the first removal, so the walk never
	// reads the map removeSession is deleting from.
	for _, s := range result {
		t.removeSession(s.localSID, cause)
	}
	t.sessions = nil
	return result
}
