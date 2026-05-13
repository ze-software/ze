package redistribute

import (
	"context"
	"sync"
	"testing"

	configredist "codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedDispatch struct {
	selector string
	command  string
}

type fakeDispatcher struct {
	mu    sync.Mutex
	calls []recordedDispatch
}

func (f *fakeDispatcher) UpdateRoute(_ context.Context, selector, command string) (uint32, uint32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, recordedDispatch{selector: selector, command: command})
	return 1, 1, nil
}

func (f *fakeDispatcher) snapshot() []recordedDispatch {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]recordedDispatch, len(f.calls))
	copy(out, f.calls)
	return out
}

// TestBGPConsumerName verifies the consumer reports "bgp".
//
// VALIDATES: Consumer name is "bgp" for registry lookup.
// PREVENTS: Wrong name causing LookupConsumer("bgp") miss.
func TestBGPConsumerName(t *testing.T) {
	c := NewBGPConsumer(&fakeDispatcher{})
	assert.Equal(t, "bgp", c.Name())
}

// TestBGPConsumerInjectRoute verifies InjectRoute produces the canonical announce command.
//
// VALIDATES: AC-10 -- InjectRoute dispatches to UpdateRoute with correct text.
// PREVENTS: Command text drift from the format used by the egress plugin.
func TestBGPConsumerInjectRoute(t *testing.T) {
	disp := &fakeDispatcher{}
	c := NewBGPConsumer(disp)

	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix:  "10.0.0.1/32",
		NextHop: "",
	})

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "*", calls[0].selector)
	assert.Equal(t, "update text origin incomplete nhop self nlri ipv4/unicast add 10.0.0.1/32", calls[0].command)
}

// TestBGPConsumerInjectRouteExplicitNextHop verifies explicit next-hop is preserved.
//
// VALIDATES: AC-10 -- explicit nhop in RouteEntry appears in command text.
// PREVENTS: NextHop being silently replaced with "self".
func TestBGPConsumerInjectRouteExplicitNextHop(t *testing.T) {
	disp := &fakeDispatcher{}
	c := NewBGPConsumer(disp)

	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix:  "10.0.0.0/24",
		NextHop: "192.0.2.1",
	})

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "update text origin incomplete nhop 192.0.2.1 nlri ipv4/unicast add 10.0.0.0/24", calls[0].command)
}

// TestBGPConsumerInjectRouteIPv6 verifies IPv6 announce command.
//
// VALIDATES: AC-10 -- IPv6 family renders correctly.
// PREVENTS: IPv6 family string mismatch.
func TestBGPConsumerInjectRouteIPv6(t *testing.T) {
	disp := &fakeDispatcher{}
	c := NewBGPConsumer(disp)

	c.InjectRoute(context.Background(), family.IPv6Unicast, configredist.RouteEntry{
		Prefix:  "2001:db8::1/128",
		NextHop: "",
	})

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "update text origin incomplete nhop self nlri ipv6/unicast add 2001:db8::1/128", calls[0].command)
}

// TestBGPConsumerWithdrawRoute verifies WithdrawRoute produces the canonical withdraw command.
//
// VALIDATES: AC-11 -- WithdrawRoute dispatches to UpdateRoute with correct text.
// PREVENTS: Withdraw command including announce-only fields (origin, nhop).
func TestBGPConsumerWithdrawRoute(t *testing.T) {
	disp := &fakeDispatcher{}
	c := NewBGPConsumer(disp)

	c.WithdrawRoute(context.Background(), family.IPv4Unicast, "10.0.0.1/32")

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "*", calls[0].selector)
	assert.Equal(t, "update text nlri ipv4/unicast del 10.0.0.1/32", calls[0].command)
}

// TestBGPConsumerWithdrawRouteIPv6 verifies IPv6 withdraw command.
//
// VALIDATES: AC-11 -- IPv6 withdraw renders correctly.
// PREVENTS: IPv6 family string mismatch in withdraw path.
func TestBGPConsumerWithdrawRouteIPv6(t *testing.T) {
	disp := &fakeDispatcher{}
	c := NewBGPConsumer(disp)

	c.WithdrawRoute(context.Background(), family.IPv6Unicast, "2001:db8::/64")

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "update text nlri ipv6/unicast del 2001:db8::/64", calls[0].command)
}
