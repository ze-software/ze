// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- One pending complete result.
// Related: connected_exchange_test.go -- The real restart producer's peer script.

package pppoeclient

import (
	"bytes"
	"io"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/rtproto"
)

type changeConsumerDialer struct{ session iface.PPPoESession }

func (d *changeConsumerDialer) Dial(iface.PPPoEClientConfig, <-chan struct{}, *slog.Logger) (iface.PPPoESession, error) {
	return d.session, nil
}

// changeConsumerBackend is owned by the interface worker. The test reads its
// records only after synctest.Wait. Release holds initial setup before receive.
type changeConsumerBackend struct {
	iface.Backend
	release  chan struct{}
	mtus     []int
	pairs    [][2]string
	gateways []string
}

func (b *changeConsumerBackend) SetAdminUp(string) error { <-b.release; return nil }
func (b *changeConsumerBackend) SetMTU(_ string, mtu int) error {
	b.mtus = append(b.mtus, mtu)
	return nil
}
func (b *changeConsumerBackend) AddAddressP2P(_, local, peer string) error {
	b.pairs = append(b.pairs, [2]string{local, peer})
	return nil
}
func (b *changeConsumerBackend) RemoveAddressP2P(string, string, string) error { return nil }
func (b *changeConsumerBackend) AddRoute(_, _, gateway string, _ int, _ rtproto.Proto) error {
	b.gateways = append(b.gateways, gateway)
	return nil
}
func (b *changeConsumerBackend) RemoveRoute(string, string, string, int, rtproto.Proto) error {
	return nil
}

// TestConnectedChangesReachConsumerNewest holds the real interface worker in
// its initial setup while the session publishes twice: the IPv4 withdrawal an
// IPCP Configure-Request in Opened publishes (Seq 1), which nobody waits on,
// then the reopened IPCP result (Seq 2). Method: release the worker and check
// it applied only the initial and the newest result, once each (pairs,
// gateways, MTUs), reported only Seq 2 applied, left the slot empty, and that
// IPv4 passed only after that report. Only production code touches the slot.
func TestConnectedChangesReachConsumerNewest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		stop, done := make(chan struct{}), make(chan struct{})
		defer close(stop)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		go keepaliveLoop(s, done, stop, logger)
		b := &changeConsumerBackend{release: make(chan struct{})}
		// The interface worker is the only caller; the test reads reported
		// only after synctest.Wait.
		var reported []uint64
		d := &changeConsumerDialer{session: iface.PPPoESession{
			UnitNum: 7, SessionID: 42, LocalIP: initial.localIP, PeerIP: initial.peerIP,
			NegMTU: initial.negMTU, Changes: s.changes, Done: done, Cleanup: func() {},
			Applied: func(seq uint64) {
				reported = append(reported, seq)
				s.acknowledge(seq)
			},
		}}
		c := iface.NewPPPoEClient(iface.PPPoEClientConfig{Name: "wan", Timing: testDialTiming}, d, b, logger)
		c.Start()
		// Always release the setup gate before joining the interface worker.
		defer func() {
			select {
			case <-b.release:
			default:
				close(b.release)
			}
			c.Stop()
		}()
		synctest.Wait()
		local := []byte{3, 6, 203, 0, 113, 2}
		peer := []byte{3, 6, 203, 0, 113, 1}
		// RFC 1661 Section 4.1, Opened: RCR takes tld, scr, sca.
		w.send(ppp.ProtoIPCP, ppp.LCPConfigureRequest, 95, peer)
		if len(s.changes) != 1 {
			t.Fatalf("IPCP leaving Opened published %d results, want the withdrawal", len(s.changes))
		}
		request := w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
		ack := w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureAck)
		if !bytes.Equal(ack, exchangeFrame(ppp.ProtoIPCP, ppp.LCPConfigureAck, 95, peer)) {
			t.Fatal("peer not acknowledged")
		}
		w.send(ppp.ProtoIPCP, ppp.LCPConfigureNak, request[3], local)
		request = w.take(t, ppp.ProtoIPCP, ppp.LCPConfigureRequest)
		if !bytes.Equal(request[6:], ipcpRequestFor(local)) {
			t.Fatal("new local address not adopted")
		}
		w.send(ppp.ProtoIPCP, ppp.LCPConfigureAck, request[3], request[6:])
		if len(s.changes) != 1 {
			t.Fatalf("pending results = %d, want the newest alone", len(s.changes))
		}
		if w.passes() != 0 {
			t.Fatalf("IPv4 passed while the interface worker was held: %v", w.modes)
		}
		close(b.release)
		synctest.Wait()
		if len(reported) != 1 || reported[0] != 2 {
			t.Fatalf("results reported applied = %v, want only the newest, Seq 2", reported)
		}
		if w.passes() != 1 {
			t.Fatalf("IPv4 did not pass once the newest result was applied: %v", w.modes)
		}
		if len(b.pairs) != 2 {
			t.Fatalf("address installations = %v, want initial and newest", b.pairs)
		}
		if b.pairs[0] != [2]string{"192.0.2.2/32", "192.0.2.1/32"} {
			t.Fatalf("initial setup applied %v", b.pairs[0])
		}
		if b.pairs[1] != [2]string{"203.0.113.2/32", "203.0.113.1/32"} {
			t.Fatalf("consumer applied %v", b.pairs[1])
		}
		if len(b.gateways) != 2 {
			t.Fatalf("gateway installations = %v", b.gateways)
		}
		if b.gateways[0] != "192.0.2.1" {
			t.Fatalf("initial gateway = %s", b.gateways[0])
		}
		if b.gateways[1] != "203.0.113.1" {
			t.Fatalf("consumer gateway = %s", b.gateways[1])
		}
		// IPCP alone renegotiated, so the newest result keeps the initial MTU
		// and the worker writes it once, at setup.
		if len(b.mtus) != 1 {
			t.Fatalf("MTU writes = %v, want the initial one alone", b.mtus)
		}
		if b.mtus[0] != int(initial.negMTU) {
			t.Fatalf("initial MTU = %d, want %d", b.mtus[0], initial.negMTU)
		}
		if len(s.changes) != 0 {
			t.Fatal("consumer left a result pending")
		}
	})
}
