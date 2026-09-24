//go:build linux

// Design: docs/architecture/wire/l2tp.md -- RFC 2661 lower-layer conformance coverage
//
// RFC-tagged units retain bare linux so rfc discriminate-record can select
// them. They also run through the registered l2tp QEMU integration package.
//
// Proves the boundary Ze owns for the RFC 2661 obligations the Linux l2tp_ppp
// module and the Linux UDP stack perform on state Ze installs: the kernel
// session is requested only once the L2TP session is established, it carries
// the sequencing and LNS-mode flags the module reads, and the control socket
// keeps UDP checksums enabled.

package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// emptyAVP writes an AVP with no value, the shape of Sequencing Required.
func emptyAVP(attr AVPType) func(buf []byte, off int) int {
	return func(buf []byte, off int) int {
		return WriteAVPBytes(buf, off, true, 0, attr, nil)
	}
}

// kernelSetupsFor collects the kernel setup events a tunnel holds, with a
// kernel worker installed so the collection is not short-circuited.
func kernelSetupsFor(t *testing.T, r *l2tpReactor, tun *L2TPTunnel) []kernelSetupEvent {
	t.Helper()
	r.tunnelsMu.Lock()
	defer r.tunnelsMu.Unlock()
	setups, _, _ := r.collectKernelEventsLocked(tun)
	return setups
}

// reactorWithWorker returns an unstarted reactor holding a fake kernel worker.
func reactorWithWorker(t *testing.T) (*UDPListener, *l2tpReactor) {
	t.Helper()
	ln, r, stop := newUnstartedReactor(t)
	t.Cleanup(stop)
	fake := &fakeKernelOps{}
	errCh := make(chan kernelSetupFailed, 4)
	successCh := make(chan kernelSetupSucceeded, 4)
	w := newKernelWorker(fake.ops(), errCh, successCh, r.logger)
	w.Start()
	t.Cleanup(w.Stop)
	r.setKernelWorker(w, errCh, successCh)
	return ln, r
}

// registerTunnel makes a tunnel built by the FSM fixtures visible to the
// reactor's kernel collection.
func registerTunnel(r *l2tpReactor, tun *L2TPTunnel) {
	r.tunnelsMu.Lock()
	r.tunnelsByLocalID[tun.localTID] = tun
	r.tunnelsByPeer[peerKey{addr: tun.peerAddr, tid: tun.remoteTID}] = tun
	r.tunnelsMu.Unlock()
}

// TestKernelSessionRequestedOnlyOnceEstablished drives an incoming call on an
// LNS tunnel and reads, after the ICRQ and after the ICCN, whether a kernel
// session setup is requested.
//
// RFC requirement: RFC2661-5.0-2 positive — after the ICCN establishes the session, one kernel session setup carrying its IDs is requested, which is the state Linux l2tp_ppp tunnels PPP frames on.
// RFC requirement: RFC2661-5.0-2 negative — after the ICRQ alone, with the session not yet established, no kernel session setup is requested, so no PPP frame can be tunneled before establishment.
func TestKernelSessionRequestedOnlyOnceEstablished(t *testing.T) {
	_, r := reactorWithWorker(t)
	now := time.Now()
	tun := newEstablishedTunnel(t, 4)
	registerTunnel(r, tun)

	ns := tun.engine.nextRecvSeq
	nr := tun.engine.nextSendSeq
	outs := deliver(t, tun, wrapControl(buildICRQ(9, 1), tun.localTID, 0, ns, nr), now, TunnelDefaults{})
	_, icrpBody := findWire(t, outs, MsgICRP)
	sid := avpU16(t, icrpBody, AVPAssignedSessionID)
	require.Empty(t, kernelSetupsFor(t, r, tun), "no kernel session before the ICCN establishes the session")

	deliver(t, tun, wrapControl(buildICCN(64000, 1), tun.localTID, sid, ns+1, nr+1), now, TunnelDefaults{})
	setups := kernelSetupsFor(t, r, tun)
	require.Len(t, setups, 1, "one kernel session once the session is established")
	require.Equal(t, sid, setups[0].localSID)
	require.Equal(t, uint16(9), setups[0].remoteSID)
	require.Equal(t, tun.localTID, setups[0].localTID)
}

// TestSequencingRequiredReachesKernelSession delivers an ICCN with and without
// the Sequencing Required AVP and reads the sequencing flag of the kernel
// session setup each earns, the flag pppSetupReal turns into
// PPPOL2TP_SO_SENDSEQ and PPPOL2TP_SO_RECVSEQ.
//
// RFC requirement: RFC2661-4.4.4-5 positive — an ICCN carrying Sequencing Required produces a kernel session setup with sequencing set, so Linux l2tp_ppp sends and expects sequence numbers on the data channel.
// RFC requirement: RFC2661-4.4.4-5 negative — an ICCN without Sequencing Required produces a kernel session setup with sequencing clear, so sequence numbers are not forced on the data channel.
// RFC requirement: RFC2661-5.4-1 positive — with Sequencing Required present at session setup the kernel session is created with sequencing set for its whole life.
// RFC requirement: RFC2661-5.4-1 negative — without the AVP the kernel session is created with sequencing clear.
func TestSequencingRequiredReachesKernelSession(t *testing.T) {
	cases := []struct {
		name       string
		iccn       []byte
		sequencing bool
	}{
		{"Sequencing Required present", bodyOf(MsgICCN, u32AVP(AVPTxConnectSpeed, 64000), u32AVP(AVPFramingType, 1), emptyAVP(AVPSequencingRequired)), true},
		{"Sequencing Required absent", buildICCN(64000, 1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, r := reactorWithWorker(t)
			now := time.Now()
			tun := newEstablishedTunnel(t, 4)
			registerTunnel(r, tun)

			ns := tun.engine.nextRecvSeq
			nr := tun.engine.nextSendSeq
			outs := deliver(t, tun, wrapControl(buildICRQ(9, 1), tun.localTID, 0, ns, nr), now, TunnelDefaults{})
			_, icrpBody := findWire(t, outs, MsgICRP)
			sid := avpU16(t, icrpBody, AVPAssignedSessionID)
			deliver(t, tun, wrapControl(tc.iccn, tun.localTID, sid, ns+1, nr+1), now, TunnelDefaults{})

			setups := kernelSetupsFor(t, r, tun)
			require.Len(t, setups, 1)
			require.Equal(t, tc.sequencing, setups[0].sequencing)
		})
	}
}

// TestLACKernelSessionFollowsPeerSequencing reads the LNS-mode flag of the
// kernel session setup for a LAC-side call and for an LNS-side call. Linux
// l2tp_ppp adapts to the peer's sequence numbers only on a session without
// PPPOL2TP_SO_LNSMODE, which pppSetupReal sets from that flag.
//
// RFC requirement: RFC2661-5.4-2 positive — a LAC-side session produces a kernel session setup with LNS mode clear and sequencing clear, so Linux l2tp_ppp stops sending sequence numbers when the peer's data messages carry none.
// RFC requirement: RFC2661-5.4-2 negative — an LNS-side session produces a kernel session setup with LNS mode set, so the adaptation the RFC binds to the LAC is never installed on the LNS.
// RFC requirement: RFC2661-5.4-3 positive — a LAC-side session produces a kernel session setup with LNS mode clear, so Linux l2tp_ppp begins sending sequence numbers when the peer's data messages carry them.
// RFC requirement: RFC2661-5.4-3 negative — an LNS-side session produces a kernel session setup with LNS mode set, so the LNS never follows the peer.
func TestLACKernelSessionFollowsPeerSequencing(t *testing.T) {
	_, r := reactorWithWorker(t)
	now := time.Now()
	lac := newEstablishedTunnel(t, 4)
	registerTunnel(r, lac)
	lsid, _ := lac.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, slog.Default())
	deliver(t, lac, wrapControl(bodyOf(MsgICRP, u16AVP(AVPAssignedSessionID, 55)), lac.localTID, lsid, 0, 1), now, TunnelDefaults{})
	setups := kernelSetupsFor(t, r, lac)
	require.Len(t, setups, 1, "the LAC requests its kernel session after the ICRP")
	require.False(t, setups[0].lnsMode, "a LAC session leaves the kernel free to follow the peer")
	require.False(t, setups[0].sequencing, "a LAC session forces no sequence numbers")

	lns := newEstablishedTunnel(t, 5)
	lns.localTID = lac.localTID + 1
	registerTunnel(r, lns)
	lnsSessionEstablished(t, lns, now, 9)
	setups = kernelSetupsFor(t, r, lns)
	require.Len(t, setups, 1)
	require.True(t, setups[0].lnsMode, "an LNS session is installed in LNS mode")
}

// TestControlSocketKeepsUDPChecksums reads SO_NO_CHECK on the UDP socket the
// listener opens, the socket every control message and, through
// L2TP_ATTR_FD, every kernel data session is sent on.
//
// RFC requirement: RFC2661-8.1-2 positive — the listener's UDP socket reads SO_NO_CHECK as 0, so the Linux UDP stack computes a checksum on every control and data message Ze sends.
func TestControlSocketKeepsUDPChecksums(t *testing.T) {
	logger := slog.Default()
	ln := newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), logger)
	require.NoError(t, ln.Start(t.Context()))
	t.Cleanup(func() { _ = ln.Stop() })

	fd, err := ln.SocketFD()
	require.NoError(t, err)
	noCheck, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_NO_CHECK)
	require.NoError(t, err)
	require.Equal(t, 0, noCheck, "UDP checksums stay enabled on the L2TP socket")
}
