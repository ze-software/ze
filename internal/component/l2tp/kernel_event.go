// Design: docs/research/l2tpv2-ze-integration.md -- kernel integration events
// Related: reactor.go -- produces events after FSM transitions

package l2tp

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/callsink"
)

// kernelSetupEvent requests the kernel worker to create kernel resources
// for a newly established L2TP session. Produced by the reactor when a
// session transitions to L2TPSessionEstablished.
type kernelSetupEvent struct {
	// Tunnel identification.
	localTID  uint16
	remoteTID uint16
	peerAddr  netip.AddrPort

	// Session identification.
	localSID  uint16
	remoteSID uint16
	session   *L2TPSession // identity retained across asynchronous setup

	// UDP socket fd for L2TP_CMD_TUNNEL_CREATE. Obtained from the
	// listener's SocketFD() method.
	socketFD int

	// Session parameters from ICCN/OCCN that affect kernel session creation.
	lnsMode    bool // true for LNS-side (handleICCN), false for LAC-side (handleOCCN)
	sequencing bool // from session.sequencingRequired

	// channel is the relayed PPPoE transport owner. The worker borrows its
	// descriptor only inside the bridge ioctl, excluding transport closure
	// until that ioctl returns. Nil deliberately means no PPPoE relay.
	channel callsink.RelayChannel
	// beforeBridge must finish before PPP channel attachment. Nil means the
	// originating call has no unused local lease to release.
	beforeBridge callsink.BridgePreparation

	// relay is the exact accepted call owner (nil for ordinary sessions).
	// Only a realm call quiesces local PPP before bridging.
	relay *relayCall

	// RFC 2661 Section 4.4.5: proxy LCP AVPs from ICCN. Empty slices when
	// the peer omitted the AVPs. Carried verbatim through the kernel
	// worker into the success event so PPP can short-circuit LCP.
	proxyInitialRecvLCPConfReq []byte
	proxyLastSentLCPConfReq    []byte
	proxyLastRecvLCPConfReq    []byte
}

// kernelTeardownEvent requests the kernel worker to destroy kernel
// resources for a session that was torn down (CDN or StopCCN). Produced
// by the reactor when removeSession or clearSessions is called.
type kernelTeardownEvent struct {
	localTID uint16
	localSID uint16
	// relay pins the call that owned this removal; nil is an ordinary call.
	relay *relayCall
}

// kernelSetupFailed is sent from the kernel worker to the reactor when
// kernel resource creation fails partway through. The reactor uses this
// to send a CDN to the peer for the affected session.
type kernelSetupFailed struct {
	localTID uint16
	localSID uint16
	// relay preserves the failed setup's owner across asynchronous reporting.
	relay *relayCall
	err   error
}

// kernelSetupSucceeded is sent from the kernel worker to the reactor
// after setupSession completes. Carries the fds the PPP driver needs to
// run a per-session goroutine, plus the session metadata so the reactor
// can build a ppp.StartSession without re-locking the tunnel map.
//
// The reactor's run loop dispatches this to ppp.Driver.SessionsIn().
type kernelSetupSucceeded struct {
	localTID   uint16
	localSID   uint16
	lnsMode    bool
	sequencing bool
	fds        pppSessionFDs
	// owner identifies this exact setup, not merely a reused tunnel/session ID.
	owner   *pppSessionFDs
	session *L2TPSession

	// RFC 2661 Section 4.4.5: proxy LCP AVPs sourced from L2TPSession at the
	// time the kernelSetupEvent was enqueued. Empty when the peer omitted
	// them; PPP runs the full LCP negotiation.
	proxyInitialRecvLCPConfReq []byte
	proxyLastSentLCPConfReq    []byte
	proxyLastRecvLCPConfReq    []byte
}
