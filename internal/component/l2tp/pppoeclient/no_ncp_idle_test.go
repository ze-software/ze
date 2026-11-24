// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- no-NCP idle-period timer (RFC 1661 Section 3.7).
// Related: ipcp_independent.go -- syncNoNCPIdle, cancelNoNCPIdle, closeIdleLCP.
// Related: network_phase.go -- keepaliveLoop.

package pppoeclient

import (
	"bytes"
	"encoding/binary"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestClientNoNCPIdleTimerKeepsLCPUpAndClosesOnExpiry verifies owned row 20
// (RFC 1661 Section 3.7): closing the last Opened NCP does not immediately
// terminate LCP; LCP stays up under a bounded idle-period timer, cancels the
// timer if an NCP reopens, and transmits an LCP Terminate-Request only when
// the idle period expires.
//
// VALIDATES: RFC 1661 Section 3.7 no-NCP idle-period lifecycle across
// IPv4-only and dual-stack connected sessions.
// PREVENTS: immediate link teardown when the last NCP closes.
func TestClientNoNCPIdleTimerKeepsLCPUpAndClosesOnExpiry(t *testing.T) {
	t.Run("ipv4-only-stays-up-then-closes-on-idle-expiry", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w, s, done := ipcpIndependence(t, false, nil)
			s.noNCPIdleTimeout = 4 * time.Second
			s.lcpIdentifier = 20
			stop := make(chan struct{})
			defer close(stop)
			go keepaliveLoop(s, done, stop, slogutil.DiscardLogger())
			synctest.Wait()

			// Peer closes the only NCP (IPCP).
			w.send(ppp.ProtoIPCP, ppp.LCPTerminateRequest, 21, nil)
			ack := w.take(t, ppp.ProtoIPCP, ppp.LCPTerminateAck)
			if ack[3] != 21 {
				t.Fatalf("IPCP Terminate-Ack Identifier = %d, want 21", ack[3])
			}
			w.quiet(t)
			if workerEnded(done) {
				t.Fatal("closing the last NCP ended LCP immediately; RFC 1661 Section 3.7 requires keeping LCP up")
			}
			if len(w.modes) != 1 || w.modes[0] != pppkernel.NPModeDrop {
				t.Fatalf("IPv4 modes = %v, want [drop]", w.modes)
			}
			down := pendingResult(t, s)
			if down.LocalIP.IsValid() || down.PeerIP.IsValid() {
				t.Fatalf("IPv4 %v/%v still published after IPCP closed", down.LocalIP, down.PeerIP)
			}

			// Halfway through the idle period, LCP is still live and answers Echo-Requests.
			// sleep(timer): advance half of the no-NCP idle period inside synctest.
			time.Sleep(2 * time.Second)
			synctest.Wait()
			if workerEnded(done) {
				t.Fatal("LCP ended before the no-NCP idle period expired")
			}
			w.send(ppp.ProtoLCP, ppp.LCPEchoRequest, 55, []byte{0, 0, 0, 1})
			echo := w.take(t, ppp.ProtoLCP, ppp.LCPEchoReply)
			if echo[3] != 55 || binary.BigEndian.Uint32(echo[6:10]) != s.current.magic {
				t.Fatalf("unexpected Echo-Reply while idle: % x", echo)
			}

			// Remaining idle period elapses: client closes LCP with Terminate-Request.
			// sleep(timer): advance to the end of the no-NCP idle period inside synctest.
			time.Sleep(2 * time.Second)
			synctest.Wait()
			term := w.take(t, ppp.ProtoLCP, ppp.LCPTerminateRequest)
			if term[3] != 21 {
				t.Fatalf("LCP Terminate-Request Identifier = %d, want 21", term[3])
			}
			if !workerEnded(done) {
				t.Fatal("session stayed up after the no-NCP idle timer expired")
			}
		})
	})

	t.Run("ipv4-reopen-cancels-idle-timer", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w, s, done := ipcpIndependence(t, false, nil)
			s.noNCPIdleTimeout = 4 * time.Second
			stop := make(chan struct{})
			defer close(stop)
			go keepaliveLoop(s, done, stop, slogutil.DiscardLogger())
			synctest.Wait()

			w.send(ppp.ProtoIPCP, ppp.LCPTerminateRequest, 21, nil)
			w.take(t, ppp.ProtoIPCP, ppp.LCPTerminateAck)
			<-s.changes
			if workerEnded(done) {
				t.Fatal("closing IPCP ended LCP immediately")
			}

			// sleep(timer): wait part of the idle period before reopening IPCP.
			time.Sleep(2 * time.Second)
			synctest.Wait()

			// Peer reopens IPCP before the idle timer fires.
			peer := []byte{3, 6, 198, 51, 100, 1}
			w.send(ppp.ProtoIPCP, ppp.LCPConfigureRequest, 95, peer)
			req := w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
			if got := w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureAck); !bytes.Equal(got, exchangeFrame(ppp.ProtoIPCP, ppp.LCPConfigureAck, 95, peer)) {
				t.Fatalf("peer IPCP Ack = % x", got)
			}
			w.send(ppp.ProtoIPCP, ppp.LCPConfigureNak, req[3], []byte{3, 6, 198, 51, 100, 2})
			req = w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
			w.send(ppp.ProtoIPCP, ppp.LCPConfigureAck, req[3], req[6:])
			synctest.Wait()
			up := pendingResult(t, s)
			s.acknowledge(up.Seq)
			synctest.Wait()

			// Advance past the original 4-second deadline: idle timer was canceled.
			// sleep(timer): advance past the canceled idle deadline inside synctest.
			time.Sleep(4 * time.Second)
			synctest.Wait()
			w.quiet(t)
			if workerEnded(done) {
				t.Fatal("reopening IPCP did not cancel the no-NCP idle timer")
			}
			if len(w.modes) != 2 || w.modes[1] != pppkernel.NPModePass {
				t.Fatalf("IPv4 modes = %v, want [drop pass]", w.modes)
			}
		})
	})

	t.Run("dual-stack-last-ncp-close-arms-idle-and-ipv6cp-reopen-cancels", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var v6 []pppkernel.NPMode
			w, s, done := ipcpIndependence(t, true, &v6)
			s.noNCPIdleTimeout = 2 * time.Second
			stop := make(chan struct{})
			defer close(stop)
			go keepaliveLoop(s, done, stop, slogutil.DiscardLogger())
			synctest.Wait()

			// Close IPCP while IPv6CP stays Opened: idle timer MUST NOT arm.
			w.send(ppp.ProtoIPCP, ppp.LCPTerminateRequest, 21, nil)
			w.take(t, ppp.ProtoIPCP, ppp.LCPTerminateAck)
			<-s.changes
			// sleep(timer): exceed the idle duration while IPv6CP stays Opened.
			time.Sleep(3 * time.Second)
			synctest.Wait()
			if workerEnded(done) {
				t.Fatal("session ended while IPv6CP was still Opened")
			}

			// Now close IPv6CP too: no NCP remains, LCP stays up and idle timer arms.
			w.send(ppp.ProtoIPv6CP, ppp.LCPTerminateRequest, 33, nil)
			w.take(t, ppp.ProtoIPv6CP, ppp.LCPTerminateAck)
			<-s.changes
			if workerEnded(done) {
				t.Fatal("closing the last NCP (IPv6CP) ended LCP immediately")
			}

			// Reopen IPv6CP before the 2-second idle timer expires.
			// sleep(timer): wait half of the idle duration before reopening IPv6CP.
			time.Sleep(time.Second)
			synctest.Wait()
			peerID := ppp.IPv6CPInterfaceID{0, 0, 0, 0, 0, 0, 0, 2}
			w.send(ppp.ProtoIPv6CP, ppp.LCPConfigureRequest, 44, ipv6cpOption(peerID))
			v6Req := w.take(t, ppp.ProtoIPv6CP, ppp.LCPConfigureRequest)
			w.take(t, ppp.ProtoIPv6CP, ppp.LCPConfigureAck)
			w.send(ppp.ProtoIPv6CP, ppp.LCPConfigureAck, v6Req[3], v6Req[6:])
			synctest.Wait()
			<-s.changes

			// Advance past the canceled deadline: IPv6CP holds the link up.
			// sleep(timer): advance past the canceled idle deadline inside synctest.
			time.Sleep(3 * time.Second)
			synctest.Wait()
			if workerEnded(done) {
				t.Fatal("reopening IPv6CP did not cancel the no-NCP idle timer")
			}

			// Close IPv6CP again and let the idle timer expire.
			w.send(ppp.ProtoIPv6CP, ppp.LCPTerminateRequest, 45, nil)
			w.take(t, ppp.ProtoIPv6CP, ppp.LCPTerminateAck)
			<-s.changes
			if workerEnded(done) {
				t.Fatal("second IPv6CP close ended LCP immediately")
			}
			// sleep(timer): let the no-NCP idle period expire inside synctest.
			time.Sleep(2 * time.Second)
			synctest.Wait()
			w.take(t, ppp.ProtoLCP, ppp.LCPTerminateRequest)
			if !workerEnded(done) {
				t.Fatal("session stayed up after the idle timer expired")
			}
		})
	})

	t.Run("repeated-no-ncp-events-do-not-extend-idle-timer", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w, s, done := ipcpIndependence(t, false, nil)
			s.noNCPIdleTimeout = 4 * time.Second
			stop := make(chan struct{})
			defer close(stop)
			go keepaliveLoop(s, done, stop, slogutil.DiscardLogger())
			synctest.Wait()

			w.send(ppp.ProtoIPCP, ppp.LCPTerminateRequest, 1, nil)
			w.take(t, ppp.ProtoIPCP, ppp.LCPTerminateAck)
			<-s.changes

			// sleep(timer): advance 3s of the 4s idle period inside synctest.
			time.Sleep(3 * time.Second)
			synctest.Wait()
			// A second IPCP Terminate-Request in Stopped must not reset the timer.
			w.send(ppp.ProtoIPCP, ppp.LCPTerminateRequest, 2, nil)
			w.take(t, ppp.ProtoIPCP, ppp.LCPTerminateAck)
			if workerEnded(done) {
				t.Fatal("session ended before the 4s idle period elapsed")
			}

			// Only 1s remains of the original 4s idle period.
			// sleep(timer): advance the remaining 1s of the original idle period.
			time.Sleep(time.Second)
			synctest.Wait()
			w.take(t, ppp.ProtoLCP, ppp.LCPTerminateRequest)
			if !workerEnded(done) {
				t.Fatal("repeated IPCP Terminate-Request extended the no-NCP idle timer")
			}
		})
	})
}
