package l2tp

// Design: docs/architecture/wire/l2tp.md -- control connection establishment
//
// RFC 2661 Section 7.2.1 table rows: "wait-ctl-conn  Receive SCCRP, SCCRQ
// Send StopCCN, Clean up  idle" and "established  Receive SCCRQ, SCCRP,
// SCCCN  Send StopCCN Clean up  idle". Section 7.1 counts a control message
// "received in an improper sequence" as invalid, and says its receipt "should
// be logged appropriately and the control connection cleared to ensure
// recovery to a known state."
// Related: tunnel_fsm.go (handleSCCRQ), tunnel_initiator.go (handleSCCRP),
// scccn_wrong_order_test.go (the SCCCN rows of the same table).

import (
	"bytes"
	"log/slog"
	"testing"
	"time"
)

// messageTypeOnlyBody returns a body carrying the Message Type AVP alone.
func messageTypeOnlyBody(mt MessageType) []byte {
	buf := make([]byte, 16)
	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(mt))
	return buf[:off]
}

// sccrpDefaults is what the peer's SCCRP advertises: a Host Name and framing,
// no shared secret, so a tunnel with no secret accepts it in wait-ctl-reply.
var sccrpDefaults = TunnelDefaults{HostName: "peer", FramingCapabilities: 0x3, RecvWindow: 16}

// sccrpBody returns a well-formed SCCRP body assigning peer tunnel ID 5.
func sccrpBody() []byte {
	buf := make([]byte, 256)
	return buf[:writeSCCRPBody(buf, 5, sccrpDefaults, nil, nil)]
}

// loggingTunnel answers a tunnel in state, logging to the returned buffer.
// In established it also holds one session.
func loggingTunnel(t *testing.T, state L2TPTunnelState) (*L2TPTunnel, *bytes.Buffer) {
	t.Helper()
	if state == L2TPTunnelEstablished {
		return tunnelWithSessionLogging(t)
	}
	tun := newEstablishedTunnel(t, 0)
	tun.state = state
	var logs bytes.Buffer
	tun.logger = slog.New(slog.NewTextHandler(&logs, nil))
	return tun, &logs
}

// RFC requirement: RFC2661-7.1-1 positive -- an SCCRQ or an SCCRP received in
// wait-ctl-conn or in established (with a session) is a message in an improper
// sequence: handleMessage logs it at warning level, sends one StopCCN, removes
// every session and leaves the tunnel closed.
func TestRFC2661OutOfOrderSCCRQOrSCCRPClearsControlConnection(t *testing.T) {
	now := time.Now()
	for _, msg := range []struct {
		name string
		mt   MessageType
		body []byte
	}{
		{"SCCRQ", MsgSCCRQ, messageTypeOnlyBody(MsgSCCRQ)},
		{"SCCRP", MsgSCCRP, sccrpBody()},
	} {
		for _, state := range []L2TPTunnelState{L2TPTunnelWaitCtlConn, L2TPTunnelEstablished} {
			tun, logs := loggingTunnel(t, state)
			entry := RecvEntry{MessageType: uint16(msg.mt), Payload: msg.body, MessageTypeMandatory: true}
			out := tun.handleMessage(entry, now, TunnelDefaults{}, nil)
			if len(out) != 1 {
				t.Fatalf("%s in %s: %d datagrams, want one StopCCN", msg.name, state, len(out))
			}
			if mt := sentMessageType(t, out[0].bytes); mt != MsgStopCCN {
				t.Fatalf("%s in %s: answered with message type %d, want StopCCN", msg.name, state, mt)
			}
			if n := tun.sessionCount(); n != 0 {
				t.Fatalf("%s in %s: %d sessions survived", msg.name, state, n)
			}
			if tun.state != L2TPTunnelClosed {
				t.Fatalf("%s in %s: tunnel %s, want closed", msg.name, state, tun.state)
			}
			if !bytes.Contains(logs.Bytes(), []byte("level=WARN")) {
				t.Fatalf("%s in %s: nothing logged at warning level: %q", msg.name, state, logs.String())
			}
		}
	}
}

// RFC requirement: RFC2661-7.1-1 negative -- each message in the one state
// that awaits it is in proper sequence and never clears the connection: an
// SCCRQ in idle is answered with an SCCRP (tunnel wait-ctl-conn), and an SCCRP
// in wait-ctl-reply is answered with an SCCCN (tunnel established); neither
// logs a warning nor sends a StopCCN.
func TestRFC2661InSequenceSCCRQAndSCCRPKeepControlConnection(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name  string
		state L2TPTunnelState
		entry RecvEntry
		sccrq *sccrqInfo
		reply MessageType
		after L2TPTunnelState
	}{
		{"SCCRQ in idle", L2TPTunnelIdle,
			RecvEntry{MessageType: uint16(MsgSCCRQ), Payload: messageTypeOnlyBody(MsgSCCRQ), MessageTypeMandatory: true},
			&sccrqInfo{HostName: "peer", FramingCapabilities: 0x3, RecvWindow: 16},
			MsgSCCRP, L2TPTunnelWaitCtlConn},
		{"SCCRP in wait-ctl-reply", L2TPTunnelWaitCtlReply,
			RecvEntry{MessageType: uint16(MsgSCCRP), Payload: sccrpBody(), MessageTypeMandatory: true},
			nil,
			MsgSCCCN, L2TPTunnelEstablished},
	} {
		tun, logs := loggingTunnel(t, tc.state)
		out := tun.handleMessage(tc.entry, now, sccrpDefaults, tc.sccrq)
		if len(out) != 1 {
			t.Fatalf("%s: %d datagrams, want one %d", tc.name, len(out), tc.reply)
		}
		if mt := sentMessageType(t, out[0].bytes); mt != tc.reply {
			t.Fatalf("%s: answered with message type %d, want %d", tc.name, mt, tc.reply)
		}
		if tun.state != tc.after {
			t.Fatalf("%s: tunnel %s, want %s", tc.name, tun.state, tc.after)
		}
		if bytes.Contains(logs.Bytes(), []byte("level=WARN")) {
			t.Fatalf("%s: in-sequence message logged a warning: %q", tc.name, logs.String())
		}
	}
}
