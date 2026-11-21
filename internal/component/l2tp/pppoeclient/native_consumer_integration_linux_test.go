//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- original native consumer
// Related: scripted_ac_integration_linux_test.go -- two actual wire epochs

package pppoeclient

import (
	"context"
	"io"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

type observedNativeDialer struct {
	dialer   Dialer
	sessions chan iface.PPPoESession
}

func (d *observedNativeDialer) Dial(cfg iface.PPPoEClientConfig, stop <-chan struct{}, logger *slog.Logger) (iface.PPPoESession, error) { //nolint:gocritic // hugeParam: exact public dial boundary.
	session, err := d.dialer.Dial(cfg, stop, logger)
	if err == nil {
		d.sessions <- session
	}
	return session, err
}

// TestNativeCPEConsumerKeepsOriginalAcrossNameReplacement drives BOTH actual
// producers: real Dial binds its original CreatedUnit and iface's real worker
// applies both wire epochs. No fake writer, numeric-unit reconstruction or
// current-backend fallback is involved. A replacement with the configured name
// is a negative control; full prefixes and default-route metric are inspected.
// VALIDATES: AC-1 and AC-8 of spec-home-cpe-1-logical-interface-name through
// actual named PPP creation, both actual consumer epochs and the surviving
// adversarial same-name dummy. No logical fixture supplies the kernel name.
func TestNativeCPEConsumerKeepsOriginalAcrossNameReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	restart := make(chan struct{})
	peerDone := make(chan error, 1)
	go scriptedACRenegotiation(ctx, peer, restart, peerDone)
	joined := false
	t.Cleanup(func() {
		cancel()
		if !joined {
			if err := <-peerDone; err != nil {
				t.Error(err)
			}
		}
	})
	dialer := &observedNativeDialer{sessions: make(chan iface.PPPoESession, 1)}
	client := iface.NewPPPoEClient(iface.PPPoEClientConfig{Name: "wan0", SourceInterface: "zec0", MTU: 1400,
		RoutePriority: 73, Timing: testDialTiming, Username: "subscriber", AuthSecret: "secret"}, dialer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	client.Start()
	t.Cleanup(func() {
		if err := client.Stop(); err != nil {
			t.Error(err)
		}
	})
	var session iface.PPPoESession
	select {
	case session = <-dialer.sessions:
	case <-ctx.Done():
		t.Fatal("real consumer never received a negotiated original")
	}
	if session.NativeWriter == nil {
		t.Fatal("real Dial omitted original writer")
	}
	original, err := netlink.LinkByName("wan0")
	if err != nil {
		t.Fatal(err)
	}
	if original.Type() != "ppp" {
		t.Fatalf("configured key did not create a real PPP device: %s", original.Type())
	}
	if original.Attrs().Name != "wan0" {
		t.Fatalf("actual configured name: %s", original.Attrs().Name)
	}
	index := original.Attrs().Index
	waitNativeCPEAddress(t, ctx, index, netip.PrefixFrom(session.LocalIP, 32))
	if original, err = netlink.LinkByIndex(index); err != nil {
		t.Fatal(err)
	}
	if original.Attrs().MTU != 1400 || original.Attrs().RawFlags&unix.IFF_UP == 0 {
		t.Fatalf("initial actual consumer link state: %+v", original.Attrs())
	}
	if err := netlink.LinkSetName(original, "heldwan0"); err != nil {
		t.Fatal(err)
	}
	replacement := &netlink.Dummy{Name: "wan0", MTU: 9000}
	if err := netlink.LinkAdd(replacement); err != nil {
		t.Fatal(err)
	}
	close(restart)
	waitNativeCPEAddress(t, ctx, index, netip.MustParsePrefix("198.51.100.2/32"))
	waitNativeCPERoute(t, ctx, original)
	untouched, err := netlink.LinkByName("wan0")
	if err != nil {
		t.Fatal(err)
	}
	if untouched.Attrs().Index == index || untouched.Attrs().MTU != 9000 {
		t.Fatalf("replacement acquired original mutation: %+v", untouched.Attrs())
	}
	addresses, err := netlink.AddrList(untouched, netlink.FAMILY_ALL)
	if err != nil || len(addresses) != 0 {
		t.Fatalf("replacement address effects: %+v, %v", addresses, err)
	}
	select {
	case err := <-peerDone:
		joined = true
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("scripted wire epochs did not complete")
	}
	if err := client.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := netlink.LinkByIndex(index); err == nil {
		t.Fatal("cleanup returned before original unit physically retired")
	}
	if _, err := netlink.LinkByName("wan0"); err != nil {
		t.Fatalf("original cleanup changed replacement: %v", err)
	}
}

func waitNativeCPERoute(t *testing.T, ctx context.Context, original netlink.Link) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		routes, err := netlink.RouteList(original, netlink.FAMILY_V4)
		if err != nil {
			t.Fatal(err)
		}
		for i := range routes {
			route := &routes[i]
			if route.Dst == nil && route.Gw.String() == "198.51.100.1" && route.Priority == 73 && int(route.Protocol) == int(rtproto.Iface) {
				return
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("renegotiated original route missing: %+v", routes)
		}
	}
}

func waitNativeCPEAddress(t *testing.T, ctx context.Context, index int, want netip.Prefix) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		link, err := netlink.LinkByIndex(index)
		if err == nil {
			addresses, listErr := netlink.AddrList(link, netlink.FAMILY_V4)
			if listErr != nil {
				t.Fatal(listErr)
			}
			for _, address := range addresses {
				if address.IPNet.String() == want.String() {
					return
				}
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("actual original index %d never applied %s: %v", index, want, ctx.Err())
		}
	}
}
