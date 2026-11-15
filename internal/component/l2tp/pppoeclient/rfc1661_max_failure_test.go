// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- bounded peer-option negotiation.
// Related: session.go -- negotiateLCP counts the Configure-Naks it sends.
// RFC 1661 Sections 4.6 and 5.4 -- see rfc/short/rfc1661.md.

package pppoeclient

import (
	"bytes"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestRFC1661ClientMaxFailureConvertsFurtherNak observes configured budgets
// of two and five Naks, then a Reject containing the peer's original MRU.
// MUTATION: replace the configured Max-Failure threshold with a fixed five.
// RFC requirement: RFC1661-4.6-4 positive -- the client sends exactly the configured two or five Configure-Naks, then rejects the original unacceptable MRU instead of sending another Nak.
func TestRFC1661ClientMaxFailureConvertsFurtherNak(t *testing.T) {
	for _, maximum := range []int{2, 5} {
		t.Run(strconv.Itoa(maximum), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := &timedLCPWriter{}
				frames := make(chan readFrame, 1)
				stop := make(chan struct{})
				done := startConfiguredLCP(t, w, frames, stop, clientMaxFailureConfig(maximum))
				defer func() { close(stop); <-done }()
				synctest.Wait()
				bad := []byte{ppp.LCPOptMRU, 4, 0x07, 0xd0}
				for sent := 0; sent <= maximum; sent++ {
					id := uint8(40 + sent)
					frames <- serverFrame(ppp.LCPConfigureRequest, id, bad)
					synctest.Wait()
					reply := clientMaxFailureReply(t, w, id)
					want := ppp.LCPConfigureNak
					if sent == maximum {
						want = ppp.LCPConfigureReject
					}
					if reply.Code != want {
						t.Fatalf("reply %d Code = %d, want %d", sent+1, reply.Code, want)
					}
					if reply.Code == ppp.LCPConfigureReject {
						if !bytes.Equal(reply.Data, bad) {
							t.Fatalf("Reject options = %x, want original MRU %x", reply.Data, bad)
						}
					}
				}
			})
		})
	}
}

// TestRFC1661ClientMaxFailureAckRenewsBudget contrasts an acceptable request
// after five Naks: its Ack renews the budget, allowing a fresh Nak afterward.
// MUTATION: omit renewal of the Nak count after a successful Configure-Ack.
// RFC requirement: RFC1661-4.6-4 negative -- a sent Configure-Ack renews the client's consecutive-Nak budget, so the next unacceptable request receives a Nak rather than a Reject.
func TestRFC1661ClientMaxFailureAckRenewsBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &timedLCPWriter{}
		frames := make(chan readFrame, 1)
		stop := make(chan struct{})
		done := startConfiguredLCP(t, w, frames, stop, clientMaxFailureConfig(5))
		defer func() { close(stop); <-done }()
		synctest.Wait()
		bad := []byte{ppp.LCPOptMRU, 4, 0x07, 0xd0}
		for sent := range 5 {
			id := uint8(50 + sent)
			frames <- serverFrame(ppp.LCPConfigureRequest, id, bad)
			synctest.Wait()
			if reply := clientMaxFailureReply(t, w, id); reply.Code != ppp.LCPConfigureNak {
				t.Fatalf("reply %d Code = %d, want Nak", sent+1, reply.Code)
			}
		}
		frames <- serverFrame(ppp.LCPConfigureRequest, 60, nil)
		synctest.Wait()
		if reply := clientMaxFailureReply(t, w, 60); reply.Code != ppp.LCPConfigureAck {
			t.Fatalf("acceptable request Code = %d, want Ack", reply.Code)
		}
		frames <- serverFrame(ppp.LCPConfigureRequest, 61, bad)
		synctest.Wait()
		if reply := clientMaxFailureReply(t, w, 61); reply.Code != ppp.LCPConfigureNak {
			t.Fatalf("first reply after Ack Code = %d, want fresh Nak", reply.Code)
		}
	})
}

// clientMaxFailureConfig supplies validated timing explicitly. The schema's
// default itself is proven through the iface configuration boundary tests.
func clientMaxFailureConfig(maximum int) sessionConfig {
	return sessionConfig{name: "pppoe0", mtu: 1492, lcpRestartTimer: 3 * time.Second,
		lcpMaxConfigure: 10, lcpMaxFailure: maximum}
}

// clientMaxFailureReply reads the completed reply after the bubble has blocked
// the negotiation worker on its next input. The recorder lock protects copies.
func clientMaxFailureReply(t *testing.T, w *timedLCPWriter, id uint8) ppp.LCPPacket {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.frames) == 0 {
		t.Fatal("no LCP response")
	}
	frame := w.frames[len(w.frames)-1]
	if len(frame) < 6 {
		t.Fatalf("short response %x", frame)
	}
	proto, payload, _, err := ppp.ParseFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if proto != ppp.ProtoLCP {
		t.Fatalf("response protocol = %#x, want LCP", proto)
	}
	reply, err := ppp.ParseLCPPacket(payload)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Identifier != id {
		t.Fatalf("response Identifier = %d, want %d", reply.Identifier, id)
	}
	return reply
}
