// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- client LCP reply correlation.
package pppoeclient

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestClientLCPReplyCorrelation drives the actual negotiator with a peer
// request followed by forged replies and then an exact Ack.
//
// RFC requirement: RFC1661-5.2-3 positive -- an exact Ack of the latest Identifier opens the client LCP.
// RFC requirement: RFC1661-5.2-3 negative -- an otherwise exact Ack with a different Identifier leaves LCP waiting.
// RFC requirement: RFC1661-5.2-4 positive -- an Ack echoing every sent option opens LCP.
// RFC requirement: RFC1661-5.2-4 negative -- missing, modified, or reordered Ack options cannot open LCP.
// RFC requirement: RFC1661-5.3-7 negative -- a Nak with a different Identifier emits no replacement request.
// RFC requirement: RFC1661-5.4-3 negative -- a Reject with a different Identifier emits no replacement request.
// RFC requirement: RFC1661-5.4-4 negative -- modified or reordered Reject options emit no replacement request.
// MUTATION: bypass ValidateLCPReply in negotiateLCP; forged replies open LCP or change the request.
func TestClientLCPReplyCorrelation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		done := make(chan error, 1)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			_, err := negotiateLCP(w, frames, buf[:], sessionConfig{mtu: 1492}, clientMagic, stop, slog.Default())
			done <- err
		}()
		synctest.Wait()
		request := w.lcpPackets(t)[0]
		frames <- serverFrame(ppp.LCPConfigureRequest, 77, nil)
		synctest.Wait()
		modified := append([]byte(nil), request.Data...)
		modified[3] ^= 1
		reordered := append(append([]byte(nil), request.Data[4:]...), request.Data[:4]...)
		for _, frame := range []readFrame{
			serverFrame(ppp.LCPConfigureAck, request.Identifier+1, request.Data),
			serverFrame(ppp.LCPConfigureAck, request.Identifier, nil),
			serverFrame(ppp.LCPConfigureAck, request.Identifier, modified),
			serverFrame(ppp.LCPConfigureAck, request.Identifier, reordered),
			serverFrame(ppp.LCPConfigureNak, request.Identifier+1, modified[:4]),
			serverFrame(ppp.LCPConfigureReject, request.Identifier+1, request.Data[:4]),
			serverFrame(ppp.LCPConfigureReject, request.Identifier, modified[:4]),
			serverFrame(ppp.LCPConfigureReject, request.Identifier, reordered),
		} {
			frames <- frame
			synctest.Wait()
			select {
			case err := <-done:
				t.Fatalf("invalid reply completed LCP: % x, error %v", frame.data, err)
			default:
			}
			if len(w.frames) != 2 {
				t.Fatalf("invalid reply emitted extra frames: % x", w.frames)
			}
		}
		frames <- serverFrame(ppp.LCPConfigureAck, request.Identifier, request.Data)
		if err := <-done; err != nil {
			t.Fatalf("exact Ack: %v", err)
		}
	})
}

// TestClientLCPRejectRemovesOnlyRejectedOptions keeps each valid rejection
// through the next retransmission and completes on an exact Ack of that request.
//
// RFC requirement: RFC1661-5.4-3 positive -- a matching Reject creates a replacement request.
// RFC requirement: RFC1661-5.4-4 positive -- an unchanged ordered subset of the request is accepted as a Reject.
// RFC requirement: RFC1661-5.4-5 positive -- rejected MRU and Magic-Number options are absent from the replacement and its retransmission.
// RFC requirement: RFC1661-5.4-5 negative -- an option the peer did not reject remains byte-identical.
// RFC requirement: RFC1661-5.1-3 positive -- the replacement request after a valid Reject has a different Identifier.
// MUTATION: restore rejected options or retain the old Identifier; the replacement request differs from the expected wire bytes.
func TestClientLCPRejectRemovesOnlyRejectedOptions(t *testing.T) {
	for _, rejected := range []string{"MRU", "Magic", "all"} {
		t.Run(rejected, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := &frameLog{}
				frames := make(chan readFrame)
				stop := make(chan struct{})
				defer close(stop)
				done := make(chan error, 1)
				go func() {
					var buf [ppp.MaxFrameBufLen]byte
					_, err := negotiateLCP(w, frames, buf[:], sessionConfig{mtu: 1492}, clientMagic, stop, slog.Default())
					done <- err
				}()
				synctest.Wait()
				first := w.lcpPackets(t)[0]
				var reject, want []byte
				switch rejected {
				case "MRU":
					reject, want = first.Data[:4], first.Data[4:]
				case "Magic":
					reject, want = first.Data[4:], first.Data[:4]
				case "all":
					reject = first.Data
				}
				frames <- serverFrame(ppp.LCPConfigureReject, first.Identifier, reject)
				synctest.Wait()
				packets := w.lcpPackets(t)
				if len(packets) != 2 {
					t.Fatalf("requests = %d, want 2", len(packets))
				}
				next := packets[1]
				if next.Identifier == first.Identifier || !bytes.Equal(next.Data, want) {
					t.Fatalf("replacement = %+v, want a new Identifier and % x", next, want)
				}
				// sleep(timer): drive the production LCP retransmission timer.
				time.Sleep(3 * time.Second)
				synctest.Wait()
				if len(w.frames) != 3 || !bytes.Equal(w.frames[1], w.frames[2]) {
					t.Fatalf("retransmission changed the outstanding request: % x", w.frames)
				}
				// Appended Nak suggestions cannot resurrect rejected options.
				frames <- serverFrame(ppp.LCPConfigureNak, next.Identifier, reject)
				synctest.Wait()
				packets = w.lcpPackets(t)
				if len(packets) != 4 || !bytes.Equal(packets[3].Data, want) {
					t.Fatalf("Nak resurrected a rejected option: %+v", packets)
				}
				next = packets[3]
				frames <- serverFrame(ppp.LCPConfigureRequest, 77, nil)
				frames <- serverFrame(ppp.LCPConfigureAck, next.Identifier, next.Data)
				if err := <-done; err != nil {
					t.Fatalf("replacement Ack: %v", err)
				}
			})
		})
	}
}

// TestClientLCPNakChangesIdentifierAndDropsStaleReply accepts a matching Nak
// and refuses the old Ack before opening on the replacement request's Ack.
//
// RFC requirement: RFC1661-5.3-7 positive -- a matching Nak emits a new request with a changed Identifier.
// RFC requirement: RFC1661-5.1-3 negative -- a valid reply cannot leave the replacement request with the previous Identifier.
// MUTATION: keep lcpID unchanged after a valid Nak; the request reuses its old Identifier.
func TestClientLCPNakChangesIdentifierAndDropsStaleReply(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		done := make(chan error, 1)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			_, err := negotiateLCP(w, frames, buf[:], sessionConfig{mtu: 1492}, clientMagic, stop, slog.Default())
			done <- err
		}()
		synctest.Wait()
		first := w.lcpPackets(t)[0]
		frames <- serverFrame(ppp.LCPConfigureNak, first.Identifier, []byte{ppp.LCPOptMRU, 4, 5, 0})
		synctest.Wait()
		packets := w.lcpPackets(t)
		if len(packets) != 2 || packets[1].Identifier == first.Identifier {
			t.Fatalf("Nak did not create a new request: %+v", packets)
		}
		frames <- serverFrame(ppp.LCPConfigureRequest, 77, nil)
		frames <- serverFrame(ppp.LCPConfigureAck, first.Identifier, first.Data)
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatalf("stale Ack completed LCP: %v", err)
		default:
		}
		frames <- serverFrame(ppp.LCPConfigureAck, packets[1].Identifier, packets[1].Data)
		if err := <-done; err != nil {
			t.Fatalf("replacement Ack: %v", err)
		}
	})
}

// TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue reflects the client's own
// Magic Nak back to it, then observes the new request and the subsequent Echo.
//
// RFC requirement: RFC1661-6.4-7 positive -- receiving the Magic-Number just sent in a Nak creates a fresh nonzero proposal.
// RFC requirement: RFC1661-6.4-7 negative -- the replacement cannot reuse the previous Magic-Number.
// MUTATION: omit generateDifferentMagic in the client Nak branch; the old value reappears.
func TestClientLCPMagicNakRedrawsAndUsesNegotiatedValue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		defer close(stop)
		done := make(chan error, 1)
		go func() {
			var buf [ppp.MaxFrameBufLen]byte
			result, err := negotiateLCP(w, frames, buf[:], sessionConfig{mtu: 1492}, clientMagic, stop, slog.Default())
			if err == nil {
				sendEchoReply(w, buf[:], ppp.LCPPacket{Identifier: 0x51}, result.localMagic)
			}
			done <- err
		}()
		synctest.Wait()
		first := w.lcpPackets(t)[0]
		frames <- serverFrame(ppp.LCPConfigureRequest, 77, []byte{ppp.LCPOptMagic, 6, 0, 0, 0, 0})
		synctest.Wait()
		nak := w.lcpPackets(t)[1]
		if nak.Code != ppp.LCPConfigureNak {
			t.Fatalf("zero Magic reply = %+v, want Nak", nak)
		}
		frames <- serverFrame(ppp.LCPConfigureNak, first.Identifier, nak.Data)
		synctest.Wait()
		packets := w.lcpPackets(t)
		if len(packets) != 3 {
			t.Fatalf("frames = %d, want request, Nak, request", len(packets))
		}
		next := packets[2]
		opts, err := ppp.ParseLCPOptions(next.Data)
		if err != nil {
			t.Fatal(err)
		}
		var magic uint32
		for _, opt := range opts {
			if opt.Type == ppp.LCPOptMagic {
				magic = binary.BigEndian.Uint32(opt.Data)
			}
		}
		if magic == 0 || magic == clientMagic {
			t.Fatalf("new Magic = %#x, want fresh nonzero value", magic)
		}
		frames <- serverFrame(ppp.LCPConfigureRequest, 78, nil)
		frames <- serverFrame(ppp.LCPConfigureAck, next.Identifier, next.Data)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		packets = w.lcpPackets(t)
		echo := packets[len(packets)-1]
		if echo.Code != ppp.LCPEchoReply || len(echo.Data) < 4 || binary.BigEndian.Uint32(echo.Data[:4]) != magic {
			t.Fatalf("Echo = %+v, want newly negotiated Magic %#x", echo, magic)
		}
	})
}

// TestClientLCPMRUNakBounds adopts acceptable suggestions and keeps the current
// MRU when a peer suggests a value outside the configured receive bounds.
func TestClientLCPMRUNakBounds(t *testing.T) {
	for _, tc := range []struct {
		suggestion uint16
		want       uint16
	}{
		{ppp.MinFrameLen - 1, 1492},
		{ppp.MinFrameLen, ppp.MinFrameLen},
		{1492, 1492},
		{1493, 1492},
	} {
		synctest.Test(t, func(t *testing.T) {
			w := &frameLog{}
			frames := make(chan readFrame)
			stop := make(chan struct{})
			defer close(stop)
			done := make(chan error, 1)
			go func() {
				var buf [ppp.MaxFrameBufLen]byte
				_, err := negotiateLCP(w, frames, buf[:], sessionConfig{mtu: 1492}, clientMagic, stop, slog.Default())
				done <- err
			}()
			synctest.Wait()
			first := w.lcpPackets(t)[0]
			var option [4]byte
			option[0], option[1] = ppp.LCPOptMRU, 4
			binary.BigEndian.PutUint16(option[2:], tc.suggestion)
			frames <- serverFrame(ppp.LCPConfigureNak, first.Identifier, option[:])
			synctest.Wait()
			packets := w.lcpPackets(t)
			if len(packets) != 2 {
				t.Fatalf("requests = %d, want 2", len(packets))
			}
			next := packets[1]
			if len(next.Data) < 4 || binary.BigEndian.Uint16(next.Data[2:4]) != tc.want {
				t.Fatalf("MRU Nak %d produced % x, want MRU %d", tc.suggestion, next.Data, tc.want)
			}
			frames <- serverFrame(ppp.LCPConfigureRequest, 77, nil)
			frames <- serverFrame(ppp.LCPConfigureAck, next.Identifier, next.Data)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestClientLCPAuthenticationAlternatives requires the client's reply to agree
// with the authentication methods its next phase can execute.
func TestClientLCPAuthenticationAlternatives(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		code uint8
	}{
		{"PAP", []byte{0xc0, 0x23}, ppp.LCPConfigureAck},
		{"CHAP-MD5", []byte{0xc2, 0x23, 5}, ppp.LCPConfigureAck},
		{"CHAP without algorithm", []byte{0xc2, 0x23}, ppp.LCPConfigureNak},
		{"MS-CHAP-v2", []byte{0xc2, 0x23, 0x81}, ppp.LCPConfigureNak},
		{"unknown protocol", []byte{0, 0}, ppp.LCPConfigureNak},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := append([]byte{ppp.LCPOptAuthProto, byte(len(tc.data) + 2)}, tc.data...)
			log := runClientLCP(t, serverFrame(ppp.LCPConfigureRequest, 0x61, options))
			code, reply, ok := clientReplyTo(t, log, 0x61)
			if !ok || code != tc.code {
				t.Fatalf("authentication reply = %d (present=%v), want %d", code, ok, tc.code)
			}
			if tc.code == ppp.LCPConfigureNak {
				if len(reply) != 1 || !bytes.Equal(reply[0].Data, []byte{0xc2, 0x23, 5}) {
					t.Fatalf("alternative = %+v, want CHAP-MD5", reply)
				}
			}
		})
	}
}
