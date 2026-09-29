// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- network phase: keepalive and CHAP re-challenge
//
// VALIDATES: RFC 1994 Section 4.1 in the network phase. After authentication
// the authenticator may challenge again at any time; negotiateIPCP and then
// keepaliveLoop answer every Challenge with a Response carrying its Identifier
// and the MD5 value, answer nothing else, and end on a Failure for that Response.
// PREVENTS: a client that answers a Challenge only during the Authentication
// phase and drops the re-challenge, so the authenticator ends the link.

package pppoeclient

import (
	"bytes"
	"io"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// runNetworkPhase starts keepaliveLoop for a CHAP-authenticated session
// inside the current synctest bubble, feeds it the frames, and returns the
// writer and the done channel. The caller ends the loop by closing stop and
// MUST read the writer only after done is closed.
func runNetworkPhase(t *testing.T, stop chan struct{}, feed ...readFrame) (*recordingRWC, chan struct{}) {
	t.Helper()
	w := &recordingRWC{}
	frames := make(chan readFrame)
	done := make(chan struct{})
	chap := networkPhaseCHAPFor(ppp.ProtoCHAP, sessionConfig{username: "user", password: "pass"})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go keepaliveLoop(w, frames, 1, chap, done, stop, logger)
	for _, f := range feed {
		frames <- f
	}
	synctest.Wait()
	return w, done
}

// sessionEnded reports whether keepaliveLoop has closed done.
func sessionEnded(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// VALIDATES: a Challenge (Identifier 0x77) received after authentication gets
// one CHAP Response: Code 2, Identifier 0x77, Value-Size 16, the MD5 of
// Identifier, secret and Challenge value, and the Name. The matching Success
// leaves the session running.
// METHOD: keepaliveLoop runs in a synctest bubble against a recording writer.
//
// RFC requirement: RFC1994-4.1-4 positive -- in the network phase, a Challenge with Identifier 0x77 received by keepaliveLoop is answered with one CHAP packet with Code 2 (Response), Identifier 0x77, and Value MD5(0x77, secret, challenge value).
func TestRFC1994NetworkPhaseChallengeAnswered(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		value := []byte{9, 8, 7, 6}
		stop := make(chan struct{})
		w, done := runNetworkPhase(t, stop,
			chapChallengeFrame(0x77, append([]byte{byte(len(value))}, value...)),
			readFrame{data: chapReplyFrame(t, 3, 0x77, "")},
		)
		if sessionEnded(done) {
			t.Fatal("session ended after the re-challenge's Success")
		}
		close(stop)
		<-done

		got := chapPacketsWritten(t, w)
		if len(got) != 1 {
			t.Fatalf("client wrote %d CHAP packets after a network-phase Challenge, want 1: %+v", len(got), got)
		}
		resp := got[0]
		if resp.Code != 2 {
			t.Fatalf("Code = %d, want 2 (Response)", resp.Code)
		}
		if resp.Identifier != 0x77 {
			t.Fatalf("Identifier = %#x, want 0x77", resp.Identifier)
		}
		want := chapMD5Response(0x77, "pass", value)
		if len(resp.Data) != 1+len(want)+len("user") || int(resp.Data[0]) != len(want) {
			t.Fatalf("Response data = % x, want Value-Size 16, the digest and the Name", resp.Data)
		}
		if !bytes.Equal(resp.Data[1:1+len(want)], want[:]) {
			t.Fatalf("Value = % x, want % x", resp.Data[1:1+len(want)], want[:])
		}
		if name := string(resp.Data[1+len(want):]); name != "user" {
			t.Fatalf("Name = %q, want %q", name, "user")
		}
	})
}

// VALIDATES: in the network phase, CHAP packets that are not a Challenge (a
// reflected Response, Code 2, and an unassigned Code 5) make keepaliveLoop
// transmit nothing and leave the session running.
//
// RFC requirement: RFC1994-4.1-4 negative -- in the network phase, CHAP packets with Code 2 and Code 5 (not a Challenge) make keepaliveLoop transmit no CHAP packet.
func TestRFC1994NetworkPhaseNoResponseWithoutChallenge(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		notChallenge := func(code, id uint8) readFrame {
			var buf [ppp.MaxFrameBufLen]byte
			off := ppp.WriteFrame(buf[:], 0, ppp.ProtoCHAP, nil)
			off += ppp.WriteLCPPacket(buf[:], off, code, id, []byte{4, 1, 2, 3, 4})
			return readFrame{data: buf[:off]}
		}
		stop := make(chan struct{})
		w, done := runNetworkPhase(t, stop, notChallenge(2, 0x77), notChallenge(5, 0x78))
		if sessionEnded(done) {
			t.Fatal("session ended on a CHAP packet that is not a Challenge")
		}
		close(stop)
		<-done
		if got := chapPacketsWritten(t, w); len(got) != 0 {
			t.Fatalf("client wrote CHAP packets %+v for packets that are not a Challenge", got)
		}
	})
}

// VALIDATES: a Failure for the Response to a network-phase Challenge ends
// the session (RFC 1994 Section 4.2: the authenticator sends Failure and
// SHOULD terminate the link, so the client stops using it at once), while a
// Failure carrying another Identifier is ignored.
func TestNetworkPhaseCHAPFailureEndsSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stop := make(chan struct{})
		defer close(stop)
		_, done := runNetworkPhase(t, stop,
			chapChallengeFrame(0x77, []byte{4, 1, 2, 3, 4}),
			readFrame{data: chapReplyFrame(t, 4, 0x76, "stale")},
		)
		if sessionEnded(done) {
			t.Fatal("a Failure for another Identifier ended the session")
		}
	})
	synctest.Test(t, func(t *testing.T) {
		stop := make(chan struct{})
		defer close(stop)
		_, done := runNetworkPhase(t, stop,
			chapChallengeFrame(0x77, []byte{4, 1, 2, 3, 4}),
			readFrame{data: chapReplyFrame(t, 4, 0x77, "")},
		)
		if !sessionEnded(done) {
			t.Fatal("a Failure for the re-challenge's Response left the session running")
		}
	})
}

// runIPCPWindow starts negotiateIPCP for a CHAP-authenticated session inside
// the current synctest bubble, feeds it the frames, and returns the writer
// and the channel that carries negotiateIPCP's error once it returns.
func runIPCPWindow(t *testing.T, stop chan struct{}, feed ...readFrame) (*recordingRWC, chan error) {
	t.Helper()
	w := &recordingRWC{}
	frames := make(chan readFrame)
	done := make(chan error, 1)
	chap := networkPhaseCHAPFor(ppp.ProtoCHAP, sessionConfig{username: "user", password: "pass"})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go ipcpWindowWorker(w, frames, chap, stop, logger, done)
	for _, f := range feed {
		frames <- f
	}
	synctest.Wait()
	return w, done
}

// ipcpWindowWorker runs negotiateIPCP once and reports its error on done.
func ipcpWindowWorker(w io.Writer, frames <-chan readFrame, chap *networkPhaseCHAP, stop <-chan struct{}, logger *slog.Logger, done chan<- error) {
	var buf [ppp.MaxFrameBufLen]byte
	_, err := negotiateIPCP(w, frames, buf[:], 1, chap, stop, logger)
	done <- err
}

// ipcpEnded reports whether negotiateIPCP has returned, and its error.
func ipcpEnded(done <-chan error) (bool, error) {
	select {
	case err := <-done:
		return true, err
	default:
		return false, nil
	}
}

// VALIDATES: a Challenge (Identifier 0x55) received while IPCP negotiates, which
// is already the Network-Layer Protocol phase, gets one CHAP Response: Code 2,
// Identifier 0x55, Value-Size 16, the MD5 of Identifier, secret and Challenge
// value, and the Name. IPCP negotiation keeps running.
// METHOD: negotiateIPCP runs in a synctest bubble against a recording writer.
//
// RFC requirement: RFC1994-4.1-4 positive -- while IPCP negotiates, a Challenge with Identifier 0x55 received by negotiateIPCP is answered with one CHAP packet with Code 2 (Response), Identifier 0x55, and Value MD5(0x55, secret, challenge value).
func TestRFC1994IPCPWindowChallengeAnswered(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		value := []byte{5, 4, 3, 2, 1}
		stop := make(chan struct{})
		w, done := runIPCPWindow(t, stop, chapChallengeFrame(0x55, append([]byte{byte(len(value))}, value...)))
		if ended, err := ipcpEnded(done); ended {
			t.Fatalf("IPCP negotiation ended on a Challenge: %v", err)
		}
		got := chapPacketsWritten(t, w)
		close(stop)
		<-done

		if len(got) != 1 {
			t.Fatalf("client wrote %d CHAP packets after a Challenge during IPCP, want 1: %+v", len(got), got)
		}
		resp := got[0]
		if resp.Code != 2 {
			t.Fatalf("Code = %d, want 2 (Response)", resp.Code)
		}
		if resp.Identifier != 0x55 {
			t.Fatalf("Identifier = %#x, want 0x55", resp.Identifier)
		}
		want := chapMD5Response(0x55, "pass", value)
		if len(resp.Data) != 1+len(want)+len("user") || int(resp.Data[0]) != len(want) {
			t.Fatalf("Response data = % x, want Value-Size 16, the digest and the Name", resp.Data)
		}
		if !bytes.Equal(resp.Data[1:1+len(want)], want[:]) {
			t.Fatalf("Value = % x, want % x", resp.Data[1:1+len(want)], want[:])
		}
		if name := string(resp.Data[1+len(want):]); name != "user" {
			t.Fatalf("Name = %q, want %q", name, "user")
		}
	})
}

// VALIDATES: while IPCP negotiates, CHAP packets that are not a Challenge (a
// reflected Response, Code 2, and an unassigned Code 5) make negotiateIPCP
// transmit no CHAP packet and keep negotiating.
//
// RFC requirement: RFC1994-4.1-4 negative -- while IPCP negotiates, CHAP packets with Code 2 and Code 5 (not a Challenge) make negotiateIPCP transmit no CHAP packet.
func TestRFC1994IPCPWindowNoResponseWithoutChallenge(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		notChallenge := func(code, id uint8) readFrame {
			var buf [ppp.MaxFrameBufLen]byte
			off := ppp.WriteFrame(buf[:], 0, ppp.ProtoCHAP, nil)
			off += ppp.WriteLCPPacket(buf[:], off, code, id, []byte{4, 1, 2, 3, 4})
			return readFrame{data: buf[:off]}
		}
		stop := make(chan struct{})
		w, done := runIPCPWindow(t, stop, notChallenge(2, 0x55), notChallenge(5, 0x56))
		if ended, err := ipcpEnded(done); ended {
			t.Fatalf("IPCP negotiation ended on a CHAP packet that is not a Challenge: %v", err)
		}
		got := chapPacketsWritten(t, w)
		close(stop)
		<-done
		if len(got) != 0 {
			t.Fatalf("client wrote CHAP packets %+v for packets that are not a Challenge", got)
		}
	})
}

// VALIDATES: a Failure for the Response to a Challenge answered during IPCP
// ends the negotiation with an error, as keepaliveLoop ends the session (RFC
// 1994 Section 4.2), while a Failure carrying another Identifier is ignored.
func TestIPCPWindowCHAPFailureEndsNegotiation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stop := make(chan struct{})
		defer close(stop)
		_, done := runIPCPWindow(t, stop,
			chapChallengeFrame(0x55, []byte{4, 1, 2, 3, 4}),
			readFrame{data: chapReplyFrame(t, 4, 0x54, "stale")},
		)
		if ended, err := ipcpEnded(done); ended {
			t.Fatalf("a Failure for another Identifier ended IPCP: %v", err)
		}
	})
	synctest.Test(t, func(t *testing.T) {
		stop := make(chan struct{})
		defer close(stop)
		_, done := runIPCPWindow(t, stop,
			chapChallengeFrame(0x55, []byte{4, 1, 2, 3, 4}),
			readFrame{data: chapReplyFrame(t, 4, 0x55, "")},
		)
		ended, err := ipcpEnded(done)
		if !ended || err == nil {
			t.Fatalf("a Failure for the Response left IPCP running (ended %v, error %v)", ended, err)
		}
	})
}
