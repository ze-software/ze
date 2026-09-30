// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- PPPoE client LCP negotiation
// Related: dialer.go -- pppoeSessionMTU, the Host input bound Dial applies
// Related: rfc2516_mru_ceiling_test.go -- the LCP half of the same bound
// RFC: rfc/short/rfc2516.md -- Section 7
//
// VALIDATES: RFC 2516 Section 7, "The Maximum-Receive-Unit (MRU) option MUST
// NOT be negotiated to a larger size than 1492." on the Host's input: the
// session MTU negotiateLCP proposes as the client's MRU defaults to 1492 and
// is never above it, whatever MTU the caller passes to Dial.
// METHOD: pppoeSessionMTU is called directly at the boundary values, its
// default drives negotiateLCP under synctest, and Dial is called with an MTU
// above the ceiling and an interface that does not exist.
// PREVENTS: a Dial caller that bypasses the iface config (which refuses an MTU
// above 1492) starting LCP with a Configure-Request for MRU 1500.

package pppoeclient

import (
	"errors"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestRFC2516HostSessionMTUDefaultsToThePPPoECeiling reads the MTU a session
// gets with no configured MTU, and the MRU its first Configure-Request names.
//
// RFC 2516 Section 7: "The Maximum-Receive-Unit (MRU) option MUST NOT be
// negotiated to a larger size than 1492."
//
// RFC requirement: RFC2516-x-9 positive -- with no MTU configured the Host
// session MTU is 1492, a configured 1492 or 1480 is kept, and negotiateLCP
// run with the default names MRU 1492 in the client's Configure-Request
// (dialer.go pppoeSessionMTU).
func TestRFC2516HostSessionMTUDefaultsToThePPPoECeiling(t *testing.T) {
	for _, tc := range []struct {
		configured int
		want       uint16
	}{
		{configured: 0, want: pppoeMRUCeiling},
		{configured: pppoeMRUCeiling, want: pppoeMRUCeiling},
		{configured: 1480, want: 1480},
	} {
		got, err := pppoeSessionMTU(tc.configured)
		if err != nil {
			t.Fatalf("pppoeSessionMTU(%d): %v", tc.configured, err)
		}
		if got != tc.want {
			t.Fatalf("pppoeSessionMTU(%d) = %d, want %d", tc.configured, got, tc.want)
		}
	}

	synctest.Test(t, func(t *testing.T) {
		mtu, err := pppoeSessionMTU(0)
		if err != nil {
			t.Fatalf("pppoeSessionMTU(0): %v", err)
		}
		log := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			var buf [ppp.MaxFrameBufLen]byte
			_, _ = negotiateLCP(log, frames, buf[:], sessionConfig{mtu: mtu}, clientMagic, stop, slog.Default()) //nolint:errcheck // stopped below; only the first request is read
		}()
		synctest.Wait()

		found := false
		for _, pkt := range log.lcpPackets(t) {
			if pkt.Code != ppp.LCPConfigureRequest {
				continue
			}
			found = true
			if got := requestedMRU(t, pkt.Data); got != pppoeMRUCeiling {
				t.Fatalf("the default session's Configure-Request names MRU %d, want %d", got, pppoeMRUCeiling)
			}
		}
		if !found {
			t.Fatal("the client sent no Configure-Request")
		}
		close(stop)
		<-done
	})
}

// TestRFC2516HostRefusesASessionMTUAboveThePPPoECeiling passes MTUs above 1492
// to the Host input bound and to Dial.
//
// RFC 2516 Section 7: "The Maximum-Receive-Unit (MRU) option MUST NOT be
// negotiated to a larger size than 1492."
//
// RFC requirement: RFC2516-x-9 negative -- a session MTU of 1493, 1500 or 9000
// is refused with errSessionMTURange, and Dial given MTU 1500 returns that
// refusal before it resolves the interface or opens a socket, so no LCP
// Configure-Request for an MRU above 1492 can be sent (dialer.go
// pppoeSessionMTU, Dial).
func TestRFC2516HostRefusesASessionMTUAboveThePPPoECeiling(t *testing.T) {
	for _, configured := range []int{pppoeMRUCeiling + 1, 1500, 9000} {
		got, err := pppoeSessionMTU(configured)
		if !errors.Is(err, errSessionMTURange) {
			t.Fatalf("pppoeSessionMTU(%d) = %d, %v; want errSessionMTURange", configured, got, err)
		}
	}

	var d Dialer
	cfg := iface.PPPoEClientConfig{SourceInterface: "ze-no-such-if0", MTU: 1500}
	_, err := d.Dial(cfg, make(chan struct{}), slog.Default())
	if !errors.Is(err, errSessionMTURange) {
		t.Fatalf("Dial with MTU 1500 returned %v, want errSessionMTURange before discovery", err)
	}
}
