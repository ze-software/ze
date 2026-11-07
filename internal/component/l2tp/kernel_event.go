// Design: docs/research/l2tpv2-ze-integration.md -- kernel integration events
// Related: reactor.go -- produces events after FSM transitions

package l2tp

import (
	"errors"
	"fmt"
	"net/netip"
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

	// pppoeChannelFD is the relayed PPPoE session's pppox socket fd for a
	// LAC-relayed incoming call (spec-followup-l2tp-call AC-3, A-4). When
	// non-zero and lnsMode is false, the kernel worker bridges the pppol2tp
	// channel to this PPPoE channel (PPPIOCBRIDGECHAN) so PPP frames flow
	// directly between subscriber and LNS. Zero for non-relayed sessions.
	pppoeChannelFD int

	// realm is the realm relay a LAC incoming call carries (nil otherwise).
	// The kernel worker asks its handoff to quiesce the subscriber's PPP
	// before the bridge, and reports the bridge result to it.
	realm *realmCall

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
}

// kernelSetupFailed is sent from the kernel worker to the reactor when
// kernel resource creation fails partway through. The reactor uses this
// to send a CDN to the peer for the affected session.
type kernelSetupFailed struct {
	localTID uint16
	localSID uint16
	err      error
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

// errRealmCallCancelled is the setup failure of a realm call the subscriber
// abandoned before the bridge completed (before quiesce, or while the
// kernel worker bridged it: realmCall.markBridged); the session is then
// torn down with CDN.
var errRealmCallCancelled = errors.New("l2tp: realm call abandoned by the subscriber before the bridge completed")

// quiesceRealmSubscriber asks the PPPoE side to stop reading the subscriber's
// channel before PPPIOCBRIDGECHAN, so only one reader owns the channel at any
// instant. It returns an error when the subscriber left, and the caller then
// tears the session down instead of bridging.
func quiesceRealmSubscriber(rc *realmCall) error {
	if rc.isCancelled() {
		return errRealmCallCancelled
	}
	if err := rc.handoff.Quiesce(); err != nil {
		return fmt.Errorf("l2tp: realm %q subscriber could not be quiesced: %w", rc.realm, err)
	}
	return nil
}
