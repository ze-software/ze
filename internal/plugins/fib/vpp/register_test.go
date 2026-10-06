// VPP FIB register: init() side-effects. The package init() (register.go) must
// register the "fib-vpp" plugin into the shared plugin registry with its config
// root and dependencies, so the composition root discovers it.
package fibvpp

import (
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/sr"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
)

func TestFibVPPRegistered(t *testing.T) {
	// VALIDATES: init() (register.go) wires "fib-vpp" into the plugin registry.
	// PREVENTS: a silently-unregistered backend that the composition root cannot find.
	reg := registry.Lookup("fib-vpp")
	if reg == nil {
		t.Fatal("fib-vpp not registered; init() did not run registry.Register")
	}
	if reg.Name != "fib-vpp" {
		t.Errorf("Name = %q, want fib-vpp", reg.Name)
	}
	if !slices.Contains(reg.ConfigRoots, "fib/vpp") {
		t.Errorf("ConfigRoots = %v, want to contain %q", reg.ConfigRoots, "fib/vpp")
	}
	for _, dep := range []string{"rib", "vpp"} {
		if !slices.Contains(reg.Dependencies, dep) {
			t.Errorf("Dependencies = %v, want to contain %q", reg.Dependencies, dep)
		}
	}
}

// TestSRv6SubscriptionResolvesCurrentWriter delivers a callback copied before
// replacement and requires it to use the newly published backend.
// MUTATION: capture *current at subscription time instead of resolving it under
// mu; the callback reaches the retired writer and leaves live steering behind.
func TestSRv6SubscriptionResolvesCurrentWriter(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	store := newSRv6TestStore(t)
	old := newFibVPP(&mockBackend{})
	old.srv6Backend = newGovppSRv6Backend(channel, 0, store)
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	old.processEvent(&incomingBatch{Changes: []incomingChange{{
		Action: routeaction.Add, Prefix: prefix, SRv6SID: netip.MustParseAddr("2001:db8:1::a"),
	}}})
	if len(channel.steers) != 1 {
		t.Fatal("fixture did not install the service route")
	}

	var mu sync.Mutex
	current := old
	bus := &fibSubscriptionBus{}
	mu.Lock()
	unsub := subscribeFibVPP(bus, &mu, &current)
	defer unsub()
	// The bus may have copied this callback before restart. It MUST resolve
	// the replacement after the lifecycle lock, not retain the old writer.
	deliver := bus.handler
	old.retired = true
	current = newFibVPP(&mockBackend{})
	current.srv6Backend = newGovppSRv6Backend(channel, 0, store)
	if err := current.restoreSRv6(); err != nil {
		mu.Unlock()
		t.Fatal(err)
	}
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		deliver(&incomingBatch{Changes: []incomingChange{{
			Action: routeaction.Withdraw, Prefix: prefix,
		}}})
		close(done)
	}()
	<-started
	mu.Unlock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("withdrawal did not finish after ownership restore")
	}
	if len(channel.steers) != 0 {
		t.Fatal("withdrawal during replacement left stale steering")
	}
	if len(channel.policies) != 0 {
		t.Fatal("last withdrawal during replacement left an owned policy")
	}
}

// TestSRv6SubscriptionWaitsForPublication holds replacement unpublished while
// delivering an actual withdrawal, then restores ownership before publishing.
// MUTATION: remove the callback's lifecycle lock; delivery finishes against the
// retired writer before publication and leaves the restored steering behind.
func TestSRv6SubscriptionWaitsForPublication(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	store := newSRv6TestStore(t)
	old := newFibVPP(&mockBackend{})
	old.srv6Backend = newGovppSRv6Backend(channel, 0, store)
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	old.processEvent(&incomingBatch{Changes: []incomingChange{{
		Action: routeaction.Add, Prefix: prefix, SRv6SID: netip.MustParseAddr("2001:db8:1::a"),
	}}})
	if len(channel.steers) != 1 {
		t.Fatal("fixture did not install the service route")
	}
	var mu sync.Mutex
	current := old
	bus := &fibSubscriptionBus{}
	mu.Lock()
	unsub := subscribeFibVPP(bus, &mu, &current)
	defer unsub()
	old.retired = true
	deliver := bus.handler
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		deliver(&incomingBatch{Changes: []incomingChange{{
			Action: routeaction.Withdraw, Prefix: prefix,
		}}})
		close(done)
	}()
	<-started
	// This is a negative concurrency assertion, not a readiness sleep: a
	// callback MUST NOT finish while the replacement is unpublished.
	select {
	case <-done:
		mu.Unlock()
		t.Fatal("withdrawal bypassed the lifecycle lock before publication")
	case <-time.After(100 * time.Millisecond):
	}
	next := newFibVPP(&mockBackend{})
	next.srv6Backend = newGovppSRv6Backend(channel, 0, store)
	if err := next.restoreSRv6(); err != nil {
		mu.Unlock()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("withdrawal did not finish after failed restore")
		}
		t.Fatal(err)
	}
	current = next
	mu.Unlock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("withdrawal did not finish after publication")
	}
	if len(channel.steers) != 0 {
		t.Fatal("withdrawal during restore left stale steering")
	}
	if len(channel.policies) != 0 {
		t.Fatal("withdrawal during restore left the last-reference policy")
	}
}

// fibSubscriptionBus retains the actual typed subscription callback so the test
// can deliver a previously copied handler across the lifecycle lock.
type fibSubscriptionBus struct {
	handler func(any)
}

func (b *fibSubscriptionBus) Subscribe(_, _ string, handler func(any)) func() {
	b.handler = handler
	return func() { b.handler = nil }
}

func (*fibSubscriptionBus) Emit(_, _ string, _ any) (int, error) {
	return 0, nil
}
