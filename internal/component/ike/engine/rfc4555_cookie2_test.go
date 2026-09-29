// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- encrypted MOBIKE exchanges
// Related: mobike_test.go -- mbInitiator, mbReceive, mbDecrypt, mbRequireNotify
// VALIDATES: the COOKIE2 data Ze sends in a MOBIKE request is drawn from the system
// cryptographic random source (RFC 4555 Section 4.2.5).
// PREVENTS: a COOKIE2 built from a counter, a timestamp or a fixed-seed generator, which
// the exchange responder could predict.

package engine

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// patternReader answers an endless stream whose octet i is seed + 7*i, so two readers
// with different seeds never answer the same first 32 octets.
type patternReader struct {
	seed byte
	next int
}

func (r *patternReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.seed + byte(7*r.next)
		r.next++
	}
	return len(p), nil
}

// TestRFC4555Cookie2IsTheRandomSourceOutput proves the COOKIE2 data of a MOBIKE request
// is the first octets the system random source answers, request after request.
//
// Goal: "unpredictable to the exchange responder" cannot be measured on one cookie, and
// two cookies that merely differ also come from a counter. What can be measured is where
// the octets come from. Method: crypto/rand.Reader is replaced, for the duration of each
// startMobikeRequest call only, with a pattern stream of a different seed. Each request's
// COOKIE2 MUST equal the first 32 octets of its own stream, so the cookie is the random
// source's output and carries nothing derived from the SA, the time or the previous
// cookie. No test in this package calls t.Parallel, and the reader is restored after each
// call.
//
// RFC 4555 Section 4.2.5: "The data associated with this notification MUST be between 8
// and 64 octets in length (inclusive), and MUST be chosen by the exchange initiator in a
// way that is unpredictable to the exchange responder."
//
// RFC requirement: RFC4555-4.2.5-1 positive -- two MOBIKE requests under two random streams (seeds 0x11 and 0xa3) each carry a 32 octet COOKIE2 equal to the first 32 octets of their own stream.
func TestRFC4555Cookie2IsTheRandomSourceOutput(t *testing.T) {
	saved := rand.Reader
	t.Cleanup(func() { rand.Reader = saved })
	log := slogutil.DiscardLogger()

	for _, seed := range []byte{0x11, 0xa3} {
		f := mbInitiator(t)
		want := make([]byte, 32)
		if _, err := (&patternReader{seed: seed}).Read(want); err != nil {
			t.Fatalf("patternReader: %v", err)
		}

		rand.Reader = &patternReader{seed: seed}
		err := f.ps.startMobikeRequest(f.local, f.myTr, true, log)
		rand.Reader = saved
		if err != nil {
			t.Fatalf("seed %#02x: start address update: %v", seed, err)
		}

		request := mbReceive(t, f.peerTr)
		inner := mbDecrypt(t, f.peer, request.Data)
		cookie := mbRequireNotify(t, inner, wire.NotifyCookie2).NotificationData
		if len(cookie) < 8 || len(cookie) > 64 {
			t.Fatalf("seed %#02x: COOKIE2 is %d octets, want 8 to 64", seed, len(cookie))
		}
		if !bytes.Equal(cookie, want) {
			t.Fatalf("seed %#02x: COOKIE2 = %x, want the first 32 octets of the random stream %x",
				seed, cookie, want)
		}
	}
}
