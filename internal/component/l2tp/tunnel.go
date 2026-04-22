// Design: docs/research/l2tpv2-implementation-guide.md -- tunnel state machine (S9)
// Related: tunnel_fsm.go -- Process / handleMessage / handleSCCRQ
// Related: reactor.go -- owns the tunnel map and calls newTunnel

package l2tp

import (
	"log/slog"
	"net/netip"
	"time"
)

// L2TPTunnelState enumerates the four tunnel FSM states from RFC 2661 S9.
// Idle is the initial and terminal state; Closed is a transient state
// during which the reliable engine still serves ZLB ACKs for retransmitted
// StopCCNs (retention window).
type L2TPTunnelState uint8

// FSM states.
const (
	L2TPTunnelIdle         L2TPTunnelState = iota // no control connection
	L2TPTunnelWaitCtlReply                        // LAC side: sent SCCRQ, waiting SCCRP
	L2TPTunnelWaitCtlConn                         // LNS side: sent SCCRP, waiting SCCCN
	L2TPTunnelEstablished                         // three-way complete
	L2TPTunnelClosed                              // teardown in progress, engine retains
)

// String returns the lowercase RFC name of the state. Used in logs.
func (s L2TPTunnelState) String() string {
	switch s {
	case L2TPTunnelIdle:
		return "idle"
	case L2TPTunnelWaitCtlReply:
		return "wait-ctl-reply"
	case L2TPTunnelWaitCtlConn:
		return "wait-ctl-conn"
	case L2TPTunnelEstablished:
		return "established"
	case L2TPTunnelClosed:
		return "closed"
	}
	return stateUnknown
}

// L2TPTunnel carries one control connection's state. NOT safe for
// concurrent use; only the reactor goroutine accesses its fields.
type L2TPTunnel struct {
	localTID  uint16
	remoteTID uint16         // peer's Assigned Tunnel ID
	peerAddr  netip.AddrPort // last known peer addr:port (updated per RFC S24.19)
	state     L2TPTunnelState
	engine    *ReliableEngine
	logger    *slog.Logger

	// Captured from the peer's SCCRQ. Used for logging and for future
	// phases that may enforce bearer/framing policy.
	peerHostName   string
	peerFraming    uint32
	peerBearer     uint32
	peerRecvWindow uint16

	// ourChallenge is the 16-byte random Challenge value we emitted in
	// SCCRP when peer authentication was requested (sccrq.ChallengePresent
	// + non-empty SharedSecret). Used to verify the peer's Challenge
	// Response AVP in SCCCN (RFC 2661 S4.2 / S5.1.2.3). Nil when we did
	// not challenge the peer, or has been cleared after successful SCCCN
	// verification.
	//
	// Caller MUST hold the owning reactor's tunnelsMu. Mutated by the FSM
	// during handleSCCRQ/handleSCCCN.
	ourChallenge []byte

	// tieBreaker is the 8-byte Tie Breaker AVP value captured from the
	// peer's SCCRQ when present (RFC 2661 S9.5). Used by the reactor when
	// a second SCCRQ arrives from the same peer address to decide which
	// tunnel keeps and which is torn down. Nil when the SCCRQ carried no
	// Tie Breaker AVP.
	//
	// Caller MUST hold the owning reactor's tunnelsMu. Set once by
	// handleSCCRQ and read by locateTunnelLocked's tie-breaker path.
	tieBreaker []byte

	// lastActivity records the time.Now() of the most recent inbound
	// control message from the peer. Used by the reactor to decide when
	// to send a HELLO keepalive (AC-12). Updated on every Process call
	// that delivers at least one message. Zero until the first delivery.
	//
	// Caller MUST hold the owning reactor's tunnelsMu.
	lastActivity time.Time

	// createdAt records the time the tunnel object was allocated (SCCRQ
	// receive on LNS side, SCCRQ send on LAC side). Used by the CLI
	// snapshot to report uptime. Immutable after newTunnel.
	createdAt time.Time

	// sessions maps our locally-assigned Session ID to the session state.
	// Nil until the first session is created. Keyed by local SID (the ID
	// in the header of inbound session-scoped messages).
	//
	// Caller MUST hold the owning reactor's tunnelsMu.
	sessions map[uint16]*L2TPSession

	// maxSessions is the per-tunnel session limit (0 = unbounded).
	// Copied from config at tunnel creation time.
	maxSessions uint16

	// pendingKernelTeardowns collects sessions that were removed from the
	// tunnel and had kernel resources. The reactor drains this after
	// Process() returns and enqueues kernelTeardownEvents. Phase 5.
	pendingKernelTeardowns []kernelTeardownEvent
}

// newTunnel constructs a tunnel in the idle state with a pre-wired
// reliable engine. Caller owns the tunnel map entry; this function only
// produces the value.
func newTunnel(localTID, remoteTID uint16, peer netip.AddrPort, cfg ReliableConfig, logger *slog.Logger, now time.Time) *L2TPTunnel {
	cfg.LocalTunnelID = localTID
	cfg.PeerTunnelID = remoteTID
	return &L2TPTunnel{
		localTID:  localTID,
		remoteTID: remoteTID,
		peerAddr:  peer,
		state:     L2TPTunnelIdle,
		engine:    NewReliableEngine(cfg),
		logger:    logger.With("local-tid", localTID, "peer", peer.String()),
		createdAt: now,
	}
}

// CreatedAt returns the time the tunnel object was allocated.
// Caller MUST hold the owning reactor's tunnelsMu.
func (t *L2TPTunnel) CreatedAt() time.Time { return t.createdAt }

// State returns the tunnel's current FSM state.
//
// Caller MUST hold the owning reactor's tunnelsMu (or be inside the
// reactor goroutine's dispatch path). Reading without the lock races
// with FSM transitions driven by subsequent inbound packets.
func (t *L2TPTunnel) State() L2TPTunnelState { return t.state }

// LocalTID returns the tunnel ID we assigned to the peer. Immutable
// after newTunnel so no lock is required.
func (t *L2TPTunnel) LocalTID() uint16 { return t.localTID }

// RemoteTID returns the peer's Assigned Tunnel ID from its SCCRQ/SCCRP.
// Immutable after newTunnel so no lock is required.
func (t *L2TPTunnel) RemoteTID() uint16 { return t.remoteTID }

// PeerAddr returns the last known peer address:port.
//
// Caller MUST hold the owning reactor's tunnelsMu (or be inside the
// reactor goroutine). The field is updated on every inbound datagram
// to track RFC 2661 S24.19 source-port variation.
func (t *L2TPTunnel) PeerAddr() netip.AddrPort { return t.peerAddr }

// Reaper gap (phase 3 known limitation):
// A tunnel that reaches L2TPTunnelWaitCtlConn but never receives SCCCN
// (peer disappears, peer requires Challenge we cannot answer until
// phase 4, etc.) has no retransmit schedule once the engine's 5
// SCCRP retransmits exhaust -- the engine calls for teardown but
// phase 3 does not act on it. The entry persists until the subsystem
// stops. Phase 5's timer/reaper closes this by acting on the engine's
// TeardownRequired signal. Documented in plan/spec-l2tp-3-tunnel.md
// Deferred Items.
