// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 service carrier lifetime

package ppp

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"
)

type serviceTestCarrier struct {
	mu          sync.Mutex
	steps       []string
	rsStarted   chan struct{}
	dhcpStarted chan struct{}
	advertised  chan struct{}
	firstRA     sync.Once
}

func (c *serviceTestCarrier) record(step string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.steps = append(c.steps, step)
}
func (c *serviceTestCarrier) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.steps)
}
func (c *serviceTestCarrier) ReadRS(ctx context.Context) error {
	close(c.rsStarted)
	<-ctx.Done()
	c.record("rs-stopped")
	return ctx.Err()
}
func (c *serviceTestCarrier) ReadDHCPv6(ctx context.Context, _ []byte) (int, netip.AddrPort, error) {
	close(c.dhcpStarted)
	<-ctx.Done()
	c.record("dhcp-stopped")
	return 0, netip.AddrPort{}, ctx.Err()
}
func (c *serviceTestCarrier) WriteRA(payload []byte, hopLimit int) error {
	if hopLimit != 255 {
		return errors.New("Router Advertisement omitted hop limit 255")
	}
	if binary.BigEndian.Uint16(payload[6:8]) == 0 {
		c.record("final-ra")
		return nil
	}
	c.record("initial-ra")
	c.firstRA.Do(func() { close(c.advertised) })
	return nil
}
func (c *serviceTestCarrier) WriteDHCPv6(payload []byte, _ netip.AddrPort) (int, error) {
	return len(payload), nil
}
func (c *serviceTestCarrier) Close() error { c.record("carrier-close"); return nil }

func newServiceCarrier() *serviceTestCarrier {
	return &serviceTestCarrier{rsStarted: make(chan struct{}), dhcpStarted: make(chan struct{}), advertised: make(chan struct{})}
}
func awaitServiceCarrier(t *testing.T, ready <-chan struct{}) {
	t.Helper()
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("service carrier worker did not reach its ready boundary")
	}
}
func serviceHandleConfig(handle SessionHandle) iPv6ServiceConfig {
	return iPv6ServiceConfig{Dataplane: handle, LocalInterfaceID: [8]byte{2, 0, 0, 0xff, 0xfe, 0, 0, 1}, PeerInterfaceID: [8]byte{2, 0, 0, 0xff, 0xfe, 0, 0, 2}}
}

// Goal: carrier acquisition follows link-local installation and its refusal
// withdraws that address before returning the original failure.
func TestIPv6ServiceCarrierAcquisitionFailureCleansAddress(t *testing.T) {
	backend := &fakeBackend{}
	handle := newTestSessionHandle(backend, "ppp0")
	handle.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) {
		if len(backend.P2PCalls()) != 1 {
			t.Fatal("carrier acquired before link-local install")
		}
		return nil, io.ErrClosedPipe
	}
	service, err := startIPv6Service(serviceHandleConfig(handle), testServerID(), nil, slog.New(slog.DiscardHandler))
	if service != nil {
		t.Fatal("failed acquisition returned a running service")
	}
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("acquisition failure = %v", err)
	}
	if got := backend.AddrRemoveCalls(); len(got) != 1 {
		t.Fatalf("address withdrawals = %+v", got)
	}
}

// Goal: the actual service stop joins both receive workers before its final
// RA and carrier close, and repeats neither on a second stop. The existing RA
// schedule decides the final advertisement time; time is not the assertion.
func TestIPv6ServiceStopOrdersCarrierLifetime(t *testing.T) {
	backend := &fakeBackend{}
	handle := newTestSessionHandle(backend, "ppp0")
	carrier := newServiceCarrier()
	handle.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) { return carrier, nil }
	service, err := startIPv6Service(serviceHandleConfig(handle), testServerID(), nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Stop(); err != nil {
			t.Error(err)
		}
	})
	awaitServiceCarrier(t, carrier.rsStarted)
	awaitServiceCarrier(t, carrier.dhcpStarted)
	awaitServiceCarrier(t, carrier.advertised)
	stopped := make(chan error, 2)
	for range 2 {
		go func() { stopped <- service.Stop() }()
	}
	for range 2 {
		select {
		case err := <-stopped:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent service stop did not join workers")
		}
	}
	steps := carrier.snapshot()
	final := slices.Index(steps, "final-ra")
	closed := slices.Index(steps, "carrier-close")
	for _, lane := range []string{"rs-stopped", "dhcp-stopped"} {
		stopped := slices.Index(steps, lane)
		if stopped < 0 {
			t.Fatalf("missing %s in %v", lane, steps)
		}
		if stopped >= final {
			t.Fatalf("%s not joined before final RA: %v", lane, steps)
		}
	}
	if final < 0 {
		t.Fatalf("missing final RA: %v", steps)
	}
	if closed <= final {
		t.Fatalf("carrier closed before final RA: %v", steps)
	}
	if err := service.Stop(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(carrier.snapshot(), steps) {
		t.Fatalf("second stop repeated worker/RA/close: %v", carrier.snapshot())
	}
	if len(backend.AddrRemoveCalls()) != 1 {
		t.Fatal("link-local not withdrawn once")
	}
}

// Goal: failed resource withdrawal is returned and retains the route, address
// and allocated prefix for retry. A successful retry releases the prefix once.
func TestIPv6ServiceStopRetainsFailedWithdrawals(t *testing.T) {
	backend := &fakeBackend{}
	handle := newTestSessionHandle(backend, "ppp0")
	released := 0
	cfg := serviceHandleConfig(handle)
	cfg.ReleasePrefix = func(netip.Prefix) { released++ }
	service := newIPv6Service(cfg)
	if err := service.installLinkLocal(); err != nil {
		t.Fatal(err)
	}
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")
	if err := service.installRoute(prefix); err != nil {
		t.Fatal(err)
	}
	stops := 0
	service.stop = func() error { stops++; return nil }
	backend.removeRouteErr = io.ErrClosedPipe
	backend.removeAddrErr = io.ErrUnexpectedEOF
	err := service.Stop()
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("lost route withdrawal failure: %v", err)
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("lost address withdrawal failure: %v", err)
	}
	if !service.routeInstalled {
		t.Fatal("failed withdrawal cleared route marker")
	}
	if !service.linkLocalInstalled {
		t.Fatal("failed withdrawal cleared address marker")
	}
	if service.delegatedPrefix != prefix {
		t.Fatal("failed withdrawal lost allocated prefix")
	}
	if released != 0 {
		t.Fatal("failed withdrawal released allocated prefix")
	}
	backend.removeRouteErr = nil
	backend.removeAddrErr = nil
	if err := service.Stop(); err != nil {
		t.Fatal(err)
	}
	if service.routeInstalled {
		t.Fatal("successful retry retained route marker")
	}
	if service.linkLocalInstalled {
		t.Fatal("successful retry retained address marker")
	}
	if service.delegatedPrefix.IsValid() {
		t.Fatal("successful retry retained allocated prefix")
	}
	if released != 1 {
		t.Fatalf("prefix releases = %d", released)
	}
	if stops != 1 {
		t.Fatalf("worker stops = %d", stops)
	}
}

// Goal: a DHCPv6 Release cannot acknowledge or return the lease to the pool
// when its installed route failed to withdraw.
func TestIPv6ServiceReleaseRetainsPrefixOnWithdrawalFailure(t *testing.T) {
	backend := &fakeBackend{}
	handle := newTestSessionHandle(backend, "ppp0")
	cfg := serviceHandleConfig(handle)
	released := 0
	cfg.ReleasePrefix = func(netip.Prefix) { released++ }
	service := newIPv6Service(cfg)
	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")
	if err := service.installRoute(prefix); err != nil {
		t.Fatal(err)
	}
	backend.removeRouteErr = io.ErrClosedPipe
	server := testServerID()
	client := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{2, 0, 0, 0, 0, 1}}
	message, err := parseDHCPv6(buildRelease([3]byte{1, 2, 3}, client, server))
	if err != nil {
		t.Fatal(err)
	}
	reply, err := service.handleRelease(message, server)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("route withdrawal = %v", err)
	}
	if reply != nil {
		t.Fatal("failed route withdrawal acknowledged as released")
	}
	if !service.routeInstalled {
		t.Fatal("route marker cleared after failed release")
	}
	if service.delegatedPrefix != prefix {
		t.Fatal("lease lost after failed release")
	}
	if released != 0 {
		t.Fatal("lease returned to pool while route remained installed")
	}
}

// Goal: a carrier close failure is not read as successful lifetime cleanup.
// Stop retains that failure even when resource withdrawals themselves succeed.
func TestIPv6ServiceStopPreservesCarrierFailure(t *testing.T) {
	handle := newTestSessionHandle(&fakeBackend{}, "ppp0")
	service := newIPv6Service(serviceHandleConfig(handle))
	stops := 0
	service.stop = func() error { stops++; return io.ErrClosedPipe }
	for range 2 {
		if err := service.Stop(); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("carrier stop failure = %v", err)
		}
	}
	if stops != 1 {
		t.Fatalf("carrier stop count = %d", stops)
	}
}
