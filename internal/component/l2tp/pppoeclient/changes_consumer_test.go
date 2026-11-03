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

// TestConnectedChangesReachConsumerNewest completes two real restarts while the
// interface worker is blocked. Only production code touches the notification slot.
func TestConnectedChangesReachConsumerNewest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newConnectedExchange()
		s, initial := w.session(t)
		stop, done := make(chan struct{}), make(chan struct{})
		defer close(stop)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		go keepaliveLoop(s, done, stop, logger)
		b := &changeConsumerBackend{release: make(chan struct{})}
		d := &changeConsumerDialer{session: iface.PPPoESession{
			UnitNum: 7, SessionID: 42, LocalIP: initial.localIP, PeerIP: initial.peerIP,
			NegMTU: initial.negMTU, Changes: s.changes, Done: done, Cleanup: func() {},
		}}
		c := iface.NewPPPoEClient(iface.PPPoEClientConfig{Name: "wan"}, d, b, logger)
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
		magic := w.openLCP(t, 0, initial.magic)
		w.openIPCP(t, initial.ipcpIdentifier, 0)
		w.openLCP(t, 0, magic)
		request := w.take(t, ppp.ProtoIPCP, 1)
		local := []byte{3, 6, 203, 0, 113, 2}
		peer := []byte{3, 6, 203, 0, 113, 1}
		w.send(ppp.ProtoIPCP, 3, request[3], local)
		request = w.take(t, ppp.ProtoIPCP, 1)
		if !bytes.Equal(request[6:], local) {
			t.Fatal("second local address not adopted")
		}
		w.send(ppp.ProtoIPCP, 1, 95, peer)
		ack := w.take(t, ppp.ProtoIPCP, 2)
		if !bytes.Equal(ack, exchangeFrame(ppp.ProtoIPCP, 2, 95, peer)) {
			t.Fatal("second peer not acknowledged")
		}
		w.send(ppp.ProtoIPCP, 2, request[3], request[6:])
		if w.passes() != 2 {
			t.Fatal("producer blocked behind pending result")
		}
		close(b.release)
		synctest.Wait()
		if len(b.pairs) != 2 {
			t.Fatalf("address installations = %v, want initial and newest", b.pairs)
		}
		if b.pairs[1] != [2]string{"203.0.113.2/32", "203.0.113.1/32"} {
			t.Fatalf("consumer applied %v", b.pairs[1])
		}
		if len(b.gateways) != 2 {
			t.Fatalf("gateway installations = %v", b.gateways)
		}
		if b.gateways[1] != "203.0.113.1" {
			t.Fatalf("consumer gateway = %s", b.gateways[1])
		}
		if len(b.mtus) != 2 {
			t.Fatalf("MTU writes = %v", b.mtus)
		}
		if b.mtus[1] != 1400 {
			t.Fatalf("consumer MTU = %d", b.mtus[1])
		}
		if len(s.changes) != 0 {
			t.Fatal("consumer left a result pending")
		}
	})
}
