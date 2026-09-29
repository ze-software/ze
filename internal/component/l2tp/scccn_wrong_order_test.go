package l2tp

// Design: docs/architecture/wire/l2tp.md -- control connection establishment
//
// RFC 2661 Section 7.2.1 table rows: "wait-ctl-reply  Receive SCCCN  Send
// StopCCN Clean up  idle" and "established  Receive SCCRQ, SCCRP, SCCCN  Send
// StopCCN Clean up  idle". Section 7.1 names a message received in the wrong
// order as invalid, and says its receipt "should be logged appropriately and
// the control connection cleared to ensure recovery to a known state."
// Related: tunnel_fsm.go (handleSCCCN).

import (
	"bytes"
	"log/slog"
	"testing"
	"time"
)

// scccnBody returns an SCCCN body carrying the Message Type AVP alone, which
// a tunnel with no shared secret accepts in wait-ctl-conn.
func scccnBody() []byte {
	buf := make([]byte, 16)
	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgSCCCN))
	return buf[:off]
}

// RFC requirement: RFC2661-7.1-1 positive -- an SCCCN received out of order
// (in established with a session, and in wait-ctl-reply) is an invalid
// message: handleMessage logs it at warning level, sends one StopCCN, removes
// every session and leaves the tunnel closed.
// RFC requirement: RFC2661-7.1-1 negative -- the same SCCCN in wait-ctl-conn,
// the one state that awaits it, is valid: nothing is sent, nothing is logged
// at warning level, and the tunnel becomes established.
//
// VALIDATES: an out-of-order SCCCN clears the control connection.
// PREVENTS: handleSCCCN dropping a wrong-state SCCCN with a debug log.
//
// TestRFC2661OutOfOrderSCCCNClearsControlConnection drives handleMessage with
// one SCCCN body in each of the three states.
func TestRFC2661OutOfOrderSCCCNClearsControlConnection(t *testing.T) {
	now := time.Now()
	entry := RecvEntry{MessageType: uint16(MsgSCCCN), Payload: scccnBody(), MessageTypeMandatory: true}

	established, establishedLogs := tunnelWithSessionLogging(t)
	waitReply := newEstablishedTunnel(t, 0)
	waitReply.state = L2TPTunnelWaitCtlReply
	var waitReplyLogs bytes.Buffer
	waitReply.logger = slog.New(slog.NewTextHandler(&waitReplyLogs, nil))

	for _, tc := range []struct {
		name string
		tun  *L2TPTunnel
		logs *bytes.Buffer
	}{
		{"established", established, establishedLogs},
		{"wait-ctl-reply", waitReply, &waitReplyLogs},
	} {
		out := tc.tun.handleMessage(entry, now, TunnelDefaults{}, nil)
		if len(out) != 1 {
			t.Fatalf("%s: %d datagrams, want one StopCCN", tc.name, len(out))
		}
		if mt := sentMessageType(t, out[0].bytes); mt != MsgStopCCN {
			t.Fatalf("%s: answered with message type %d, want StopCCN", tc.name, mt)
		}
		if tc.tun.sessionCount() != 0 {
			t.Fatalf("%s: %d sessions survived", tc.name, tc.tun.sessionCount())
		}
		if tc.tun.state != L2TPTunnelClosed {
			t.Fatalf("%s: tunnel %s, want closed", tc.name, tc.tun.state)
		}
		if !bytes.Contains(tc.logs.Bytes(), []byte("level=WARN")) {
			t.Fatalf("%s: nothing logged at warning level: %q", tc.name, tc.logs.String())
		}
	}

	waitConn := newEstablishedTunnel(t, 0)
	waitConn.state = L2TPTunnelWaitCtlConn
	var waitConnLogs bytes.Buffer
	waitConn.logger = slog.New(slog.NewTextHandler(&waitConnLogs, nil))
	out := waitConn.handleMessage(entry, now, TunnelDefaults{}, nil)
	if len(out) != 0 {
		t.Fatalf("wait-ctl-conn: %d datagrams, want none", len(out))
	}
	if waitConn.state != L2TPTunnelEstablished {
		t.Fatalf("wait-ctl-conn: tunnel %s, want established", waitConn.state)
	}
	if bytes.Contains(waitConnLogs.Bytes(), []byte("level=WARN")) {
		t.Fatalf("wait-ctl-conn: valid SCCCN logged a warning: %q", waitConnLogs.String())
	}
}
