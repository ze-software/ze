package reactor

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/network"
)

// fakeAddressTable stands in for the kernel address table and its registered
// watcher: a table the test edits, and watches the test signals. Each watch
// closes its channel when its ctx ends, as network.AddressWatcher requires.
type fakeAddressTable struct {
	mu       sync.Mutex
	prefixes []netip.Prefix
	watches  []chan struct{}
	// startErr, when set, is every watch start's answer.
	startErr error
}

// install replaces the reactor's address read and watch with this fake until
// the test ends.
func (f *fakeAddressTable) install(t *testing.T) {
	t.Helper()
	savedRead, savedWatch := readAddressTable, watchAddressTable
	readAddressTable = f.read
	watchAddressTable = f.watch
	t.Cleanup(func() {
		readAddressTable = savedRead
		watchAddressTable = savedWatch
	})
}

func (f *fakeAddressTable) read() ([]netip.Prefix, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]netip.Prefix(nil), f.prefixes...), nil
}

func (f *fakeAddressTable) watch(ctx context.Context) (<-chan struct{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startErr != nil {
		return nil, f.startErr
	}
	changes := make(chan struct{}, 1)
	f.watches = append(f.watches, changes)
	go closeFakeAddressWatch(ctx, changes)
	return changes, nil
}

// closeFakeAddressWatch is the fake watch's goroutine: it closes the channel
// when ctx ends.
func closeFakeAddressWatch(ctx context.Context, changes chan struct{}) {
	<-ctx.Done()
	close(changes)
}

// set replaces the table.
func (f *fakeAddressTable) set(prefixes []netip.Prefix) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prefixes = append([]netip.Prefix(nil), prefixes...)
}

// signal leaves a signal on the last watch started.
func (f *fakeAddressTable) signal() {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case f.watches[len(f.watches)-1] <- struct{}{}:
	default:
	}
}

// listening reports whether r holds a listener on addr at r's port.
func listening(r *Reactor, addr netip.Addr) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.listeners[net.JoinHostPort(addr.String(), strconv.Itoa(r.config.Port))]
	return ok
}

// TestAddressWatchRunsWithoutEventBus is the defect's own test: a reactor
// started with NO event bus, the shape of a BGP-only config where no interface
// monitor emits (interface, addr-added), still starts a listener for a peer's
// local address that appears after start, and stops it when the address goes
// away. It drives the reactor's real start path (StartWithContext) with the
// kernel address table replaced by a fake.
//
// MUTATION: remove r.startAddressWatch() from StartWithContext, or start it
// only when an event bus is set: no watch runs, and the added address gets no
// listener.
func TestAddressWatchRunsWithoutEventBus(t *testing.T) {
	table := &fakeAddressTable{}
	table.install(t)

	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	require.Nil(t, r.eventBus, "the test needs a reactor with no event bus")
	require.NoError(t, r.StartWithContext(context.Background()))
	t.Cleanup(func() { stopAndWait(t, r) })

	local := netip.MustParseAddr("127.0.0.1")
	s := NewPeerSettings(netip.MustParseAddr("192.0.2.9"), 65000, 65001, 0x01010101)
	s.LocalAddress = local
	r.mu.Lock()
	r.peers[s.PeerKey()] = NewPeer(s)
	r.mu.Unlock()
	require.False(t, listening(r, local), "before the address appears there is no listener")

	table.set([]netip.Prefix{netip.MustParsePrefix("127.0.0.1/8")})
	table.signal()
	require.Eventually(t, func() bool { return listening(r, local) },
		5*time.Second, time.Millisecond, "the added address got its listener")

	table.set(nil)
	table.signal()
	require.Eventually(t, func() bool { return !listening(r, local) },
		5*time.Second, time.Millisecond, "the removed address lost its listener")
}

// TestAddressSyncFirstReadActsOnNothing proves the first read only records
// the table: startAddressWatch makes it before the listeners start, so an
// address already present is not an addition, and a peer that is not passive
// gains no listener for it.
//
// MUTATION: act on the first read as on any other (drop the early return in
// addressSync.apply): the present address starts a listener.
func TestAddressSyncFirstReadActsOnNothing(t *testing.T) {
	table := &fakeAddressTable{prefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/8")}}
	table.install(t)
	r := newIfaceTestReactor(t)
	local := netip.MustParseAddr("127.0.0.1")
	s := NewPeerSettings(netip.MustParseAddr("192.0.2.9"), 65000, 65001, 0x01010101)
	s.LocalAddress = local
	r.peers[s.PeerKey()] = NewPeer(s)

	sync := &addressSync{r: r}
	sync.apply()
	assert.False(t, listening(r, local), "the first read starts nothing")
	_, recorded := sync.applied[local]
	assert.True(t, recorded, "the first read records the address as applied")

	sync.apply()
	assert.False(t, listening(r, local), "an unchanged table starts nothing")
}

// TestAddressSyncRetriesAnAddressNotUsableYet proves an address the kernel
// refuses to bind (EADDRNOTAVAIL, the answer for an IPv6 address still in
// duplicate address detection) is left unrecorded, so the next read tries it
// again: the end of DAD is an address update of its own, and an address
// recorded on the failed try would never get its listener. 192.0.2.1 is not a
// local address, so the bind is refused the same way.
//
// MUTATION: record the address whatever handleAddressAdded answers: the
// address is recorded and the next read does not try it again.
func TestAddressSyncRetriesAnAddressNotUsableYet(t *testing.T) {
	table := &fakeAddressTable{}
	table.install(t)
	r := newIfaceTestReactor(t)
	unusable := netip.MustParseAddr("192.0.2.1")
	s := NewPeerSettings(netip.MustParseAddr("192.0.2.9"), 65000, 65001, 0x01010101)
	s.LocalAddress = unusable
	r.peers[s.PeerKey()] = NewPeer(s)

	sync := &addressSync{r: r}
	sync.apply()
	table.set([]netip.Prefix{netip.MustParsePrefix("192.0.2.1/24")})
	sync.apply()

	require.False(t, listening(r, unusable), "the kernel refused the bind")
	_, recorded := sync.applied[unusable]
	assert.False(t, recorded, "an address that could not be bound yet stays unrecorded, so the next read retries it")
}

// TestAddressWatchStartFailureRetries proves a watch that cannot start (no
// watcher registered, as on a platform without netlink) is started again after
// the retry delay, and that the worker returns once the wait reports the end.
//
// MUTATION: return from addressWatchWorker when the first start fails: the
// watch is never started again.
func TestAddressWatchStartFailureRetries(t *testing.T) {
	table := &fakeAddressTable{startErr: network.ErrAddressWatchNotRegistered}
	table.install(t)
	var mu sync.Mutex
	waits := 0
	saved := waitAddressWatch
	waitAddressWatch = func(_ context.Context, _ clock.Clock, _ time.Duration) bool {
		mu.Lock()
		defer mu.Unlock()
		waits++
		return waits < 3
	}
	t.Cleanup(func() { waitAddressWatch = saved })

	r := newIfaceTestReactor(t)
	changes, err := watchAddressTable(r.ctx)
	require.ErrorIs(t, err, network.ErrAddressWatchNotRegistered)
	r.addressWatchWorker(r.ctx, &addressSync{r: r}, changes, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 3, waits, "each failed start waits, then starts again, until the wait reports the end")
}

// TestAddressWatchOutageAddressGetsListener proves an address that appears
// while the watch could not open still gets its listener once a watch runs:
// startAddressWatch records the table even when the first watch fails, so the
// worker's first successful watch applies the change instead of recording it
// as the starting table.
//
// MUTATION: record the table in startAddressWatch only when the first watch
// opened: the address added during the outage is taken as the starting table
// and gets no listener.
func TestAddressWatchOutageAddressGetsListener(t *testing.T) {
	table := &fakeAddressTable{startErr: network.ErrAddressWatchNotRegistered}
	table.install(t)
	local := netip.MustParseAddr("127.0.0.1")
	saved := waitAddressWatch
	waitAddressWatch = func(ctx context.Context, _ clock.Clock, _ time.Duration) bool {
		// The outage ends, and the address appeared during it.
		table.mu.Lock()
		table.startErr = nil
		table.prefixes = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/8")}
		table.mu.Unlock()
		return ctx.Err() == nil
	}
	t.Cleanup(func() { waitAddressWatch = saved })

	r := newIfaceTestReactor(t)
	s := NewPeerSettings(netip.MustParseAddr("192.0.2.9"), 65000, 65001, 0x01010101)
	s.LocalAddress = local
	r.peers[s.PeerKey()] = NewPeer(s)

	r.mu.Lock()
	r.startAddressWatch()
	r.mu.Unlock()
	require.Eventually(t, func() bool { return listening(r, local) },
		5*time.Second, time.Millisecond, "the address added during the outage got its listener")
}
