// Design: docs/research/l2tpv2-implementation-guide.md -- Section 11.3 sending
// Related: tunnel_fsm.go -- Process collects queued and fresh control messages
// Related: rfc2661_tunnel_test.go -- wire delivery and ownership helpers
// RFC 2661 Section 5.8 -- see rfc/short/rfc2661.md

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestTunnelProcessQueuedSendBeforeFreshReply opens the real send window with
// an ICRQ's piggyback ACK and checks both response identities in emission order.
// RFC 2661 Section 5.8: "Each subsequent message is sent with the next increment of the sequence number."
func TestTunnelProcessQueuedSendBeforeFreshReply(t *testing.T) {
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	defaults := TunnelDefaults{HostName: "ze-test", FramingCapabilities: 0x3, RecvWindow: 4}

	// RFC 2661 Section 5.8. The initial CWND admits only this first ICRP.
	first := deliver(t, tun, wrapControl(buildICRQ(500, 1000), tun.localTID, 0, 0, 0), now, defaults)
	firstHeader, firstBody := findWire(t, first, MsgICRP)
	require.Equal(t, uint16(0), firstHeader.Ns)
	require.Equal(t, uint16(500), firstHeader.SessionID)
	firstSession := tun.lookupSessionByRemote(500)
	require.NotNil(t, firstSession)
	require.Equal(t, firstSession.localSID, avpU16(t, firstBody, AVPAssignedSessionID))
	require.Equal(t, uint16(1), tun.engine.CWND())
	require.Equal(t, 1, tun.engine.Outstanding())

	// RFC 2661 Section 5.8. Without an ACK the second ICRP must stay queued.
	queued := deliver(t, tun, wrapControl(buildICRQ(501, 1001), tun.localTID, 0, 1, 0), now, defaults)
	require.Len(t, queued, 1)
	queuedHeader, queuedBody := controlWire(t, queued[0].bytes)
	require.Empty(t, queuedBody, "a full window permits only the ZLB acknowledgement")
	require.Equal(t, uint16(1), queuedHeader.Ns, "a queued ICRP must not consume Ns")
	require.Equal(t, uint16(1), tun.engine.CWND())
	require.Equal(t, 1, tun.engine.Outstanding())
	require.Len(t, tun.engine.sendQueue, 1)
	queuedSession := tun.lookupSessionByRemote(501)
	require.NotNil(t, queuedSession)

	// RFC 2661 Section 5.8. This real ACK drains the queued reply before the
	// current ICRQ creates another reply; both fit the now-open CWND2.
	out := deliver(t, tun, wrapControl(buildICRQ(502, 1002), tun.localTID, 0, 2, 1), now, defaults)
	defer ackAll(t, tun, now)
	require.Len(t, out, 2)
	require.Equal(t, uint16(2), tun.engine.CWND())
	require.Equal(t, 2, tun.engine.Outstanding())
	require.Empty(t, tun.engine.sendQueue)
	freshSession := tun.lookupSessionByRemote(502)
	require.NotNil(t, freshSession)

	for index, expected := range []struct {
		ns        uint16
		remoteSID uint16
		localSID  uint16
	}{
		{1, 501, queuedSession.localSID},
		{2, 502, freshSession.localSID},
	} {
		// RFC 2661 Sections 3.1 and 5.8. Check the complete response identity,
		// so changing only header sequence numbers cannot satisfy ordering.
		header, body := controlWire(t, out[index].bytes)
		require.Equal(t, MsgICRP, wireMessageType(t, body))
		require.Equal(t, expected.ns, header.Ns, "response %d is out of send order", index)
		require.Equal(t, expected.remoteSID, header.SessionID)
		require.Equal(t, expected.localSID, avpU16(t, body, AVPAssignedSessionID))
		require.Equal(t, tun.peerAddr, out[index].to)
	}
}
