// Design: docs/research/l2tpv2-implementation-guide.md -- S10 session state machines
// Related: tunnel.go -- L2TPTunnel owns the session map
// Related: tunnel_fsm.go -- handleMessage dispatches session-scoped messages

package l2tp

import (
	"crypto/rand"
	"encoding/binary"
	"net/netip"
	"time"
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
		return "established"
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

	// pppInterface is the kernel pppN interface name (e.g. "ppp0").
	// Set by handleKernelSuccess when the PPP session is started.
	// Used by SessionUp EventBus event so the shaper can apply TC.
	pppInterface string
}

// State returns the session's current FSM state.
func (s *L2TPSession) State() L2TPSessionState { return s.state }

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

// removeSession removes a session from the tunnel's maps. If the session
// had reached established state (kernel resources may exist), a teardown
// event is queued for the reactor to send to the kernel worker.
func (t *L2TPTunnel) removeSession(sid uint16) {
	sess := t.sessions[sid]
	if sess != nil && sess.state == L2TPSessionEstablished {
		t.pendingKernelTeardowns = append(t.pendingKernelTeardowns, kernelTeardownEvent{
			localTID: t.localTID,
			localSID: sid,
		})
	}
	delete(t.sessions, sid)
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

// clearSessions removes all sessions from the tunnel. Used during
// StopCCN processing. Returns the sessions that were active (for CDN
// generation by the caller). Established sessions are queued for kernel
// teardown.
func (t *L2TPTunnel) clearSessions() []*L2TPSession {
	if len(t.sessions) == 0 {
		return nil
	}
	result := make([]*L2TPSession, 0, len(t.sessions))
	for _, s := range t.sessions {
		if s.state == L2TPSessionEstablished {
			t.pendingKernelTeardowns = append(t.pendingKernelTeardowns, kernelTeardownEvent{
				localTID: t.localTID,
				localSID: s.localSID,
			})
		}
		result = append(result, s)
	}
	t.sessions = nil
	return result
}
