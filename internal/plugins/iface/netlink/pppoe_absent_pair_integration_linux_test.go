//go:build integration && linux

// Design: docs/features/interfaces.md -- Idempotent exact P2P cleanup.
// Related: addr_p2p_exact_linux_test.go -- Missing pairs never reach Delete.

package ifacenetlink

import (
	"io"
	"log/slog"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/test/userns"
)

type absentPairDialer struct{ session iface.PPPoESession }

func (d *absentPairDialer) Dial(iface.PPPoEClientConfig, <-chan struct{}, *slog.Logger) (iface.PPPoESession, error) {
	return d.session, nil
}

// absentPairBackend removes the original pair between initial setup and change
// application. Only the production backend decides whether later removal succeeds.
type absentPairBackend struct {
	iface.Backend
	adds    int
	ready   chan struct{}
	removed chan error
}

func (b *absentPairBackend) AddAddressP2P(device, local, peer string) error {
	b.adds++
	if b.adds == 2 {
		if err := b.Backend.RemoveAddressP2P(device, "192.0.2.2/32", "192.0.2.1/32"); err != nil {
			return err
		}
	}
	err := b.Backend.AddAddressP2P(device, local, peer)
	if b.adds == 1 {
		close(b.ready)
	}
	return err
}

func (b *absentPairBackend) RemoveAddressP2P(device, local, peer string) error {
	err := b.Backend.RemoveAddressP2P(device, local, peer)
	b.removed <- err
	return err
}

// TestPPPoEClientAlreadyAbsentPair keeps the real runSession alive when its old
// pair disappeared, and independently reads the surviving replacement from Linux.
func TestPPPoEClientAlreadyAbsentPair(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	b, err := newNetlinkBackend()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := b.CreateDummy("ppp7"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.DeleteInterface("ppp7"); err != nil {
			t.Error(err)
		}
	})
	changes := make(chan iface.PPPoEResult, 1)
	var cleanups atomic.Int32
	d := &absentPairDialer{session: iface.PPPoESession{
		UnitNum: 7, SessionID: 42, NegMTU: 1492,
		LocalIP: netip.MustParseAddr("192.0.2.2"), PeerIP: netip.MustParseAddr("192.0.2.1"),
		Changes: changes, Done: make(chan struct{}), Cleanup: func() { cleanups.Add(1) },
	}}
	recorder := &absentPairBackend{Backend: b, ready: make(chan struct{}), removed: make(chan error, 1)}
	c := iface.NewPPPoEClient(iface.PPPoEClientConfig{Name: "wan", NoDefaultRoute: true}, d, recorder, slog.New(slog.NewTextHandler(io.Discard, nil)))
	c.Start()
	t.Cleanup(c.Stop)
	select {
	case <-recorder.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("initial address not installed")
	}
	changes <- iface.PPPoEResult{LocalIP: netip.MustParseAddr("192.0.2.3"), PeerIP: netip.MustParseAddr("192.0.2.1"), NegMTU: 1492}
	select {
	case err := <-recorder.removed:
		if err != nil {
			t.Fatalf("already absent pair ended change application: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not attempt old-pair cleanup")
	}
	if cleanups.Load() != 0 {
		t.Fatal("absent pair released the session")
	}
	link, err := netlink.LinkByName("ppp7")
	if err != nil {
		t.Fatal(err)
	}
	addresses, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 1 {
		t.Fatalf("surviving addresses = %v", addresses)
	}
	want := exactRemovalPair(t, "192.0.2.3/32", "192.0.2.1/32")
	if !addresses[0].Equal(*want) {
		t.Fatalf("replacement local lost: %v", addresses)
	}
	if !addresses[0].PeerEqual(*want) {
		t.Fatalf("replacement peer lost: %v", addresses)
	}
}
