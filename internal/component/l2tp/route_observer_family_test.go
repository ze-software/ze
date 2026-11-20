// Design: docs/architecture/l2tp/subscriber-session-model.md -- route observer family release
//
// Goal: prove the redistribution feed follows each family of a session on its
// own: a family's install announces its own routes, its release withdraws only
// them, its reinstall re-adds only them, and session-down withdraws what each
// family still holds, once. Method: drive the observer's assign, release and
// withdraw handlers, the functions the subscriber-namespace subscriptions call,
// and fold the recorded route-change batches into the set of live prefixes.

package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// familyRouteFixture is one dual-stack L2TP subscriber holding one
// Framed-Route and one Framed-IPv6-Route.
type familyRouteFixture struct {
	bus      *recordingBus
	observer *subscriberRouteObserver
	session  subscriber.Session
	host     netip.Addr
	framed   []FramedRoute
}

func newFamilyRouteFixture(t *testing.T, sessionID uint16) *familyRouteFixture {
	t.Helper()
	bus := &recordingBus{}
	birth := fixedRouteFixtureBirth(t, uint64(sessionID))
	return &familyRouteFixture{
		bus:      bus,
		observer: newSubscriberRouteObserver(slog.Default(), bus),
		session: subscriber.Session{ID: birth.String(), L2TPBirth: birth, AccessType: subscriber.AccessL2TP,
			TunnelID: 1, SessionID: sessionID, Username: "dual"},
		host: netip.MustParseAddr("192.0.2.80"),
		framed: []FramedRoute{
			{Prefix: netip.MustParsePrefix("198.51.100.0/24"), Metric: 3},
			{Prefix: netip.MustParsePrefix("2001:db8:80::/48"), Metric: 7},
		},
	}
}

// installIPv4 delivers the IPCP assignment notice.
func (f *familyRouteFixture) installIPv4() {
	f.session.IPv4Addr = f.host
	f.observer.assign(&f.session, l2tpevents.NetworkLifetime{}, f.host, f.framed)
}

// installIPv6 delivers the IPv6CP notice: an interface identifier, no address.
func (f *familyRouteFixture) installIPv6() {
	f.session.IPv6InterfaceID = [8]byte{0x02, 0, 0, 0, 0, 0, 0, 0x80}
	f.observer.assign(&f.session, l2tpevents.NetworkLifetime{}, netip.Addr{}, f.framed)
}

func (f *familyRouteFixture) release(family l2tpevents.AddressFamily) {
	f.observer.release(&f.session, l2tpevents.NetworkLifetime{}, family)
}

// entries returns every route-change entry emitted from index from on.
func (f *familyRouteFixture) entries(t *testing.T, from int) []redistevents.RouteChangeEntry {
	t.Helper()
	var out []redistevents.RouteChangeEntry
	for _, event := range f.bus.events()[from:] {
		batch, ok := event.payload.(*redistevents.RouteChangeBatch)
		require.True(t, ok, "observer emitted %T", event.payload)
		out = append(out, batch.Entries...)
	}
	return out
}

func (f *familyRouteFixture) mark() int { return len(f.bus.events()) }

// livePrefixes folds every emitted batch into the prefixes now announced.
func (f *familyRouteFixture) livePrefixes(t *testing.T) []netip.Prefix {
	t.Helper()
	live := map[netip.Prefix]bool{}
	for _, entry := range f.entries(t, 0) {
		if entry.Action == redistevents.ActionAdd {
			live[entry.Prefix] = true
			continue
		}
		delete(live, entry.Prefix)
	}
	var out []netip.Prefix
	for prefix := range live {
		out = append(out, prefix)
	}
	return out
}

func prefixesOf(entries []redistevents.RouteChangeEntry, action redistevents.RouteAction) []netip.Prefix {
	var out []netip.Prefix
	for _, entry := range entries {
		if entry.Action == action {
			out = append(out, entry.Prefix)
		}
	}
	return out
}

var (
	familyHost    = netip.MustParsePrefix("192.0.2.80/32")
	familyFramed4 = netip.MustParsePrefix("198.51.100.0/24")
	familyFramed6 = netip.MustParsePrefix("2001:db8:80::/48")
)

// TestRouteObserverFamilyDownRemovesPrefix: an IPv4 release removes the /32
// and only the IPv4 framed routes; an IPv6 release removes only the IPv6
// framed routes. Each remove carries the record's instance, and the record
// stays. Fails before the per-family release: no release handler existed.
func TestRouteObserverFamilyDownRemovesPrefix(t *testing.T) {
	f := newFamilyRouteFixture(t, 80)
	f.installIPv4()
	f.installIPv6()
	require.ElementsMatch(t, []netip.Prefix{familyHost, familyFramed4, familyFramed6}, f.livePrefixes(t))
	instance := f.entries(t, 0)[0].SourceInstance

	mark := f.mark()
	f.release(l2tpevents.AddressFamilyIPv4)
	removed := f.entries(t, mark)
	require.ElementsMatch(t, []netip.Prefix{familyHost, familyFramed4}, prefixesOf(removed, redistevents.ActionRemove))
	require.Empty(t, prefixesOf(removed, redistevents.ActionAdd))
	for _, entry := range removed {
		require.Equal(t, instance, entry.SourceInstance, "release must carry the record's instance")
	}
	require.ElementsMatch(t, []netip.Prefix{familyFramed6}, f.livePrefixes(t))

	mark = f.mark()
	f.release(l2tpevents.AddressFamilyIPv6)
	removed = f.entries(t, mark)
	require.ElementsMatch(t, []netip.Prefix{familyFramed6}, prefixesOf(removed, redistevents.ActionRemove))
	require.Equal(t, instance, removed[0].SourceInstance)
	require.Empty(t, f.livePrefixes(t))
	_, _, active := f.observer.Stats()
	require.Equal(t, 1, active, "a family release keeps the record for the reopen")
}

// TestRouteObserverFamilyReopenReaddsFramedRoutes: after a release of either
// family, that family's next install re-adds exactly its own routes under the
// same instance, and nothing of the other family is emitted twice.
func TestRouteObserverFamilyReopenReaddsFramedRoutes(t *testing.T) {
	f := newFamilyRouteFixture(t, 81)
	f.installIPv4()
	f.installIPv6()
	instance := f.entries(t, 0)[0].SourceInstance

	f.release(l2tpevents.AddressFamilyIPv6)
	mark := f.mark()
	f.installIPv6()
	readded := f.entries(t, mark)
	require.ElementsMatch(t, []netip.Prefix{familyFramed6}, prefixesOf(readded, redistevents.ActionAdd))
	require.Empty(t, prefixesOf(readded, redistevents.ActionRemove))
	require.Equal(t, instance, readded[0].SourceInstance)

	f.release(l2tpevents.AddressFamilyIPv4)
	mark = f.mark()
	f.installIPv4()
	readded = f.entries(t, mark)
	require.ElementsMatch(t, []netip.Prefix{familyHost, familyFramed4}, prefixesOf(readded, redistevents.ActionAdd))
	require.Empty(t, prefixesOf(readded, redistevents.ActionRemove))
	for _, entry := range readded {
		require.Equal(t, instance, entry.SourceInstance)
	}
	require.ElementsMatch(t, []netip.Prefix{familyHost, familyFramed4, familyFramed6}, f.livePrefixes(t))
}

// TestRouteObserverIPv6OnlySessionEmitsIPv6FramedRoutes: a session with no
// IPv4 address announces its Framed-IPv6-Route prefixes on the IPv6 install,
// and nothing of the IPv4 family. Fails before the split: assign returned on
// the invalid address, so the session announced nothing.
func TestRouteObserverIPv6OnlySessionEmitsIPv6FramedRoutes(t *testing.T) {
	f := newFamilyRouteFixture(t, 82)
	f.installIPv6()
	added := f.entries(t, 0)
	require.ElementsMatch(t, []netip.Prefix{familyFramed6}, prefixesOf(added, redistevents.ActionAdd))
	require.Equal(t, uint32(7), added[0].Metric)
	require.NotZero(t, added[0].SourceInstance, "the IPv6 install draws the route lifetime's instance")
}

// TestRouteObserverSessionDownAfterIPv4ReleaseWithdrawsIPv6FramedRoutes:
// after an IPv4 release, session-down withdraws the IPv6 framed routes still
// announced, once, and no second remove of the IPv4 routes: no leak, no
// duplicate.
func TestRouteObserverSessionDownAfterIPv4ReleaseWithdrawsIPv6FramedRoutes(t *testing.T) {
	f := newFamilyRouteFixture(t, 83)
	f.installIPv4()
	f.installIPv6()
	f.release(l2tpevents.AddressFamilyIPv4)

	mark := f.mark()
	f.observer.withdraw(&f.session, l2tpevents.NetworkLifetime{})
	removed := f.entries(t, mark)
	require.Equal(t, []netip.Prefix{familyFramed6}, prefixesOf(removed, redistevents.ActionRemove))
	require.Empty(t, prefixesOf(removed, redistevents.ActionAdd))
	require.Empty(t, f.livePrefixes(t))
}
