// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- PPPoE client LCP negotiation
// Related: session.go -- negotiateLCP and clientLCPPolicy, which bound both MRU
//   directions by the session's configured MTU
// Related: lcp_reply_test.go -- the frame log and serverFrame harness
// RFC: rfc/short/rfc2516.md -- Section 7
//
// VALIDATES: RFC 2516 Section 7, "The Maximum-Receive-Unit (MRU) option MUST
// NOT be negotiated to a larger size than 1492." LCP negotiates an MRU in each
// direction: the client's own Configure-Request names what it receives, and the
// Access Concentrator's names what the client may send. The session runs with
// the configured MTU, which the iface config caps at 1492 and defaults to it.
// METHOD: negotiateLCP runs against a scripted Access Concentrator under
// synctest, and every frame the client writes is decoded from the log.
// PREVENTS: a client that acknowledges an Access Concentrator asking for an
// MRU of 1500, or adopts a 1500 suggested in a Configure-Nak.

package pppoeclient

import (
	"encoding/binary"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// pppoeMRUCeiling is the RFC 2516 Section 7 bound, and the MTU the iface
// config gives a PPPoE client by default and at most.
const pppoeMRUCeiling = 1492

// mruOption is one LCP MRU option carrying mru.
func mruOption(mru uint16) []byte {
	option := []byte{ppp.LCPOptMRU, 4, 0, 0}
	binary.BigEndian.PutUint16(option[2:], mru)
	return option
}

// requestedMRU answers the MRU option of an LCP Configure-Request's Data, and
// fails when the request carries none.
func requestedMRU(t *testing.T, data []byte) uint16 {
	t.Helper()
	opts, err := ppp.ParseLCPOptions(data)
	if err != nil {
		t.Fatalf("Configure-Request Data % x does not parse: %v", data, err)
	}
	for _, opt := range opts {
		if opt.Type == ppp.LCPOptMRU && len(opt.Data) == 2 {
			return binary.BigEndian.Uint16(opt.Data)
		}
	}
	t.Fatalf("Configure-Request Data % x carries no MRU", data)
	return 0
}

// mruNegotiation is one negotiateLCP run with the MTU a default PPPoE client
// session receives.
type mruNegotiation struct {
	log    *frameLog
	frames chan readFrame
	done   chan lcpResult
}

// startMRUNegotiation starts negotiateLCP and waits for its first
// Configure-Request. It MUST be called inside a synctest bubble.
func startMRUNegotiation(t *testing.T) *mruNegotiation {
	t.Helper()
	n := &mruNegotiation{log: &frameLog{}, frames: make(chan readFrame), done: make(chan lcpResult, 1)}
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		var buf [ppp.MaxFrameBufLen]byte
		result, err := negotiateLCP(n.log, n.frames, buf[:], sessionConfig{mtu: pppoeMRUCeiling}, clientMagic, stop, slog.Default())
		if err != nil {
			t.Errorf("negotiateLCP: %v", err)
		}
		n.done <- result
	}()
	synctest.Wait()
	return n
}

// lastRequest answers the client's most recent Configure-Request.
func (n *mruNegotiation) lastRequest(t *testing.T) ppp.LCPPacket {
	t.Helper()
	var last ppp.LCPPacket
	found := false
	for _, pkt := range n.log.lcpPackets(t) {
		if pkt.Code == ppp.LCPConfigureRequest {
			last, found = pkt, true
		}
	}
	if !found {
		t.Fatal("the client sent no Configure-Request")
	}
	return last
}

// send delivers frame to the client and waits until it has answered.
func (n *mruNegotiation) send(frame readFrame) {
	n.frames <- frame
	synctest.Wait()
}

// finish acknowledges the client's latest Configure-Request and answers the
// negotiated result.
func (n *mruNegotiation) finish(t *testing.T) lcpResult {
	t.Helper()
	request := n.lastRequest(t)
	n.send(serverFrame(ppp.LCPConfigureAck, request.Identifier, request.Data))
	return <-n.done
}

// TestRFC2516ClientNegotiatesMRUAtThePPPoECeiling runs one clean negotiation:
// both sides name 1492.
//
// RFC 2516 Section 7: "The Maximum-Receive-Unit (MRU) option MUST NOT be
// negotiated to a larger size than 1492."
//
// RFC requirement: RFC2516-x-9 positive -- with the MTU a PPPoE client session
// gets by default, the client's Configure-Request names MRU 1492, it acknowledges
// an Access Concentrator asking for MRU 1492, and LCP opens with 1492 in both
// directions (session.go negotiateLCP).
func TestRFC2516ClientNegotiatesMRUAtThePPPoECeiling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := startMRUNegotiation(t)
		if got := requestedMRU(t, n.lastRequest(t).Data); got != pppoeMRUCeiling {
			t.Fatalf("the client's Configure-Request names MRU %d, want %d", got, pppoeMRUCeiling)
		}

		n.send(serverFrame(ppp.LCPConfigureRequest, 77, mruOption(pppoeMRUCeiling)))
		code, opts, answered := clientReplyTo(t, n.log, 77)
		if !answered || code != ppp.LCPConfigureAck {
			t.Fatalf("the client answered MRU %d with code %d (%v), want Configure-Ack", pppoeMRUCeiling, code, opts)
		}

		result := n.finish(t)
		if result.peerMRU != pppoeMRUCeiling {
			t.Fatalf("negotiated send MRU %d, want %d", result.peerMRU, pppoeMRUCeiling)
		}
		if got := requestedMRU(t, n.lastRequest(t).Data); got != pppoeMRUCeiling {
			t.Fatalf("the acknowledged receive MRU is %d, want %d", got, pppoeMRUCeiling)
		}
	})
}

// TestRFC2516ClientRefusesAnMRUAboveThePPPoECeiling offers 1500 in each
// direction and reads what the client does with it.
//
// RFC 2516 Section 7: "The Maximum-Receive-Unit (MRU) option MUST NOT be
// negotiated to a larger size than 1492."
//
// RFC requirement: RFC2516-x-9 negative -- an Access Concentrator
// Configure-Request asking for MRU 1500 draws a Configure-Nak suggesting 1492,
// never an Ack, and LCP opens only at 1492; a Configure-Nak suggesting the
// client receive 1500 leaves the client's next Configure-Request at 1492
// (session.go negotiateLCP, clientLCPPolicy).
func TestRFC2516ClientRefusesAnMRUAboveThePPPoECeiling(t *testing.T) {
	t.Run("access concentrator asks to receive 1500", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			n := startMRUNegotiation(t)
			n.send(serverFrame(ppp.LCPConfigureRequest, 77, mruOption(1500)))
			code, opts, answered := clientReplyTo(t, n.log, 77)
			if !answered || code != ppp.LCPConfigureNak {
				t.Fatalf("the client answered MRU 1500 with code %d (%v), want Configure-Nak", code, opts)
			}
			if len(opts) != 1 || opts[0].Type != ppp.LCPOptMRU || binary.BigEndian.Uint16(opts[0].Data) != pppoeMRUCeiling {
				t.Fatalf("the Configure-Nak carries %v, want MRU %d alone", opts, pppoeMRUCeiling)
			}

			n.send(serverFrame(ppp.LCPConfigureRequest, 78, mruOption(pppoeMRUCeiling)))
			if code, _, _ := clientReplyTo(t, n.log, 78); code != ppp.LCPConfigureAck {
				t.Fatalf("the client answered the corrected MRU with code %d, want Configure-Ack", code)
			}
			if result := n.finish(t); result.peerMRU != pppoeMRUCeiling {
				t.Fatalf("negotiated send MRU %d, want %d", result.peerMRU, pppoeMRUCeiling)
			}
		})
	})

	t.Run("access concentrator suggests the client receive 1500", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			n := startMRUNegotiation(t)
			first := n.lastRequest(t)
			n.send(serverFrame(ppp.LCPConfigureNak, first.Identifier, mruOption(1500)))
			next := n.lastRequest(t)
			if next.Identifier == first.Identifier {
				t.Fatal("the client sent no new Configure-Request after the Configure-Nak")
			}
			if got := requestedMRU(t, next.Data); got != pppoeMRUCeiling {
				t.Fatalf("after a Nak suggesting 1500 the client requests MRU %d, want %d", got, pppoeMRUCeiling)
			}
			n.send(serverFrame(ppp.LCPConfigureRequest, 79, mruOption(pppoeMRUCeiling)))
			n.finish(t)
		})
	})
}
