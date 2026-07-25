package redistribute

import (
	"context"
	"sync"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"

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

// TestBGPConsumerInjectRouteOriginASN verifies a nonzero OriginASN turns the
// announce into a locally-originated virtual-router route: `origin igp` plus the
// `origin-as` directive (the reactor applies the normal iBGP/eBGP export rule,
// unlike a verbatim as-path). as112 is the first user; the consumer stays
// protocol-agnostic.
//
// VALIDATES: AC-3/AC-4 -- OriginASN renders `origin igp origin-as <asn>`.
// PREVENTS: origin staying `incomplete`, the origin AS being dropped, or
// regressing to a verbatim `as-path` (which would not prepend on eBGP).
func TestBGPConsumerInjectRouteOriginASN(t *testing.T) {
	disp := &fakeDispatcher{}
	c := NewBGPConsumer(disp)

	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix:    "192.175.48.0/24",
		OriginASN: 112,
	})

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "update text origin igp origin-as 112 nhop self nlri ipv4/unicast add 192.175.48.0/24", calls[0].command)
}

// TestBGPConsumerInjectRouteCommunity verifies a community list renders as
// `community [ <asn>:<val> ... ]`, including a well-known value: NO_EXPORT
// (0xFFFFFF01) round-trips to 65535:65281 (high16:low16).
//
// VALIDATES: AC-5 -- Community renders and every uint32 round-trips.
// PREVENTS: community dropped or misencoded on the wire.
func TestBGPConsumerInjectRouteCommunity(t *testing.T) {
	disp := &fakeDispatcher{}
	c := NewBGPConsumer(disp)

	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix:    "192.175.48.0/24",
		OriginASN: 112,
		Community: []uint32{0xFFFFFF01, 0x00010002}, // no-export, then 1:2
	})

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "update text origin igp origin-as 112 community [65535:65281 1:2] nhop self nlri ipv4/unicast add 192.175.48.0/24", calls[0].command)
}

// TestBGPConsumerInjectRouteCommunityNoOrigin verifies a community with no
// OriginASN still emits the community while keeping `origin incomplete` -- the
// two attributes are independent.
//
// VALIDATES: community is emitted independently of OriginASN.
// PREVENTS: community only being emitted when an origin-ASN is also set.
func TestBGPConsumerInjectRouteCommunityNoOrigin(t *testing.T) {
	disp := &fakeDispatcher{}
	c := NewBGPConsumer(disp)

	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix:    "10.0.0.0/24",
		Community: []uint32{0x00010002},
	})

	calls := disp.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "update text origin incomplete community [1:2] nhop self nlri ipv4/unicast add 10.0.0.0/24", calls[0].command)
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
