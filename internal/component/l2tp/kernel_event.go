// Design: docs/research/l2tpv2-ze-integration.md -- kernel integration events
// Related: reactor.go -- produces events after FSM transitions

package l2tp

import "net/netip"

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

	// UDP socket fd for L2TP_CMD_TUNNEL_CREATE. Obtained from the
	// listener's SocketFD() method.
	socketFD int

	// Session parameters from ICCN/OCCN that affect kernel session creation.
	lnsMode    bool // true for LNS-side (handleICCN), false for LAC-side (handleOCCN)
	sequencing bool // from session.sequencingRequired
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
