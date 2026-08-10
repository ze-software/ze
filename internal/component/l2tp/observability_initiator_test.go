package l2tp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestObservability_InitiatorTunnelState -- AC-5.
//
// VALIDATES: a dialed tunnel is snapshot-visible with the correct initiator
// state name (wait-ctl-reply) via both LookupTunnel and Snapshot, and the FSM
// history records the idle -> wait-ctl-reply transition -- the observability
// the CLI (show l2tp tunnels) and metrics consume.
func TestObservability_InitiatorTunnelState(t *testing.T) {
	ln, r, _, stop := buildLogReactor(t)
	defer stop()
	client := newClient(t, ln)
	defer client.Close()

	localTID, err := r.Dial(DialTarget{Remote: clientAddrPort(t, client)})
	require.NoError(t, err)
	_ = readDatagram(t, client) // drain the SCCRQ

	ts, ok := r.LookupTunnel(localTID)
	require.True(t, ok)
	require.Equal(t, "wait-ctl-reply", ts.State, "initiator tunnel renders its state name")

	snap := r.Snapshot()
	require.Equal(t, 1, snap.TunnelCount)
	require.Equal(t, "wait-ctl-reply", snap.Tunnels[0].State)

	hist := r.TunnelFSMHistory(localTID)
	require.NotEmpty(t, hist)
	sawWaitCtl := false
	for _, tr := range hist {
		if tr.To == "wait-ctl-reply" {
			sawWaitCtl = true
		}
	}
	require.True(t, sawWaitCtl, "tunnel FSM history records wait-ctl-reply")
}

// TestObservability_InitiatorSessionState -- AC-5.
//
// VALIDATES: an initiated (auto-OCRQ) session is snapshot-visible with the
// wait-reply state name and a matching StateNum, and its FSM history records
// the transition -- so metrics (ze_l2tp_session_state gauge, Set from
// StateNum) and the CLI report initiator sessions correctly.
func TestObservability_InitiatorSessionState(t *testing.T) {
	ln, r, logs, stop := buildLogReactor(t)
	defer stop()
	timer := newTunnelTimer(r.tickCh, r.updateCh)
	require.NoError(t, timer.Start())
	defer timer.Stop()
	client := newClient(t, ln)
	defer client.Close()

	localTID, err := r.PlaceOutgoingCall(DialTarget{Remote: clientAddrPort(t, client)},
		callParams{callSerial: 7, framingType: 1, calledNumber: "5551234"})
	require.NoError(t, err)

	sccrq := readDatagram(t, client)
	client.Send(t, buildSCCRPForSCCRQ(t, sccrq, 909))
	waitForLog(t, logs, "OCRQ sent; session wait-reply (LNS outgoing)")

	// The auto-placed outgoing call is a wait-reply session on this tunnel.
	tun := r.tunnelByLocalID(localTID)
	require.NotNil(t, tun)
	r.tunnelsMu.Lock()
	var localSID uint16
	for _, s := range tun.sessions {
		localSID = s.localSID
	}
	r.tunnelsMu.Unlock()
	require.NotZero(t, localSID)

	ss, ok := r.LookupSession(localSID)
	require.True(t, ok)
	require.Equal(t, "wait-reply", ss.State, "initiator session renders its state name")
	require.Equal(t, int(L2TPSessionWaitReply), ss.StateNum, "StateNum drives the metrics gauge")

	hist := r.SessionFSMHistory(localSID)
	require.NotEmpty(t, hist, "session FSM history records initiator transitions")
}
