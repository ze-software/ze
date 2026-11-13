// Design: docs/features/interfaces.md -- retained address publication

package ifacevpp

import (
	"encoding/json"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_types"

	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
)

type replayEventBus struct {
	mu       sync.Mutex
	next     int
	handlers map[int]replayEventHandler
}

type replayEventHandler struct {
	namespace, event string
	fn               func(any)
}

func (b *replayEventBus) Emit(namespace, event string, payload any) (int, error) {
	b.mu.Lock()
	handlers := make([]func(any), 0, len(b.handlers))
	for _, h := range b.handlers {
		if h.namespace == namespace && h.event == event {
			handlers = append(handlers, h.fn)
		}
	}
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (b *replayEventBus) Subscribe(namespace, event string, fn func(any)) func() {
	b.mu.Lock()
	if b.handlers == nil {
		b.handlers = make(map[int]replayEventHandler)
	}
	b.next++
	id := b.next
	b.handlers[id] = replayEventHandler{namespace: namespace, event: event, fn: fn}
	b.mu.Unlock()
	return func() { b.mu.Lock(); delete(b.handlers, id); b.mu.Unlock() }
}

// replayChannel combines the existing API seams: dump readback plus monitor
// notification ownership. Address mutation replies update that readback.
type replayChannel struct {
	monitorChannel
	dump      *dumpChannel
	addressMu sync.Mutex
}

func newReplayChannel() *replayChannel {
	return &replayChannel{dump: &dumpChannel{
		details: []interfaces.SwInterfaceDetails{{SwIfIndex: 7, InterfaceName: "xe0.100"}},
		addresses: map[interface_types.InterfaceIndex][]ip_types.AddressWithPrefix{7: {
			toAddressWithPrefix(netip.MustParsePrefix("192.0.2.7/24")),
			toAddressWithPrefix(netip.MustParsePrefix("2001:db8::7/64")),
		}},
	}}
}

func (c *replayChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.addressMu.Lock()
	defer c.addressMu.Unlock()
	return c.dump.SendMultiRequest(msg)
}

func (c *replayChannel) SendRequest(msg api.Message) api.RequestCtx {
	if change, ok := msg.(*interfaces.SwInterfaceAddDelAddress); ok {
		c.addressMu.Lock()
		current := c.dump.addresses[change.SwIfIndex]
		if change.IsAdd {
			current = append(current, change.Prefix)
		} else {
			current = slices.DeleteFunc(current, func(p ip_types.AddressWithPrefix) bool { return p == change.Prefix })
		}
		c.dump.addresses[change.SwIfIndex] = current
		c.addressMu.Unlock()
	}
	return c.monitorChannel.SendRequest(msg)
}

func replaySubscription(bus *replayEventBus, event string) (<-chan any, func()) {
	values := make(chan any, 64)
	stop := bus.Subscribe(ifaceevents.Namespace, event, func(payload any) { values <- payload })
	return values, stop
}

func awaitReplayValue(t *testing.T, values <-chan any) any {
	t.Helper()
	select {
	case payload := <-values:
		return payload
	case <-time.After(time.Second):
		t.Fatal("no retained address publication from active VPP monitor")
	}
	return nil
}

func requestAddressReplay(t *testing.T, bus *replayEventBus) {
	t.Helper()
	if _, err := bus.Emit(ifaceevents.Namespace, "address-replay-request", nil); err != nil {
		t.Fatal(err)
	}
}

func requireVPPReplay(t *testing.T, values <-chan any, want []string) {
	t.Helper()
	got := make([]string, 0, len(want))
	for range want {
		raw, ok := awaitReplayValue(t, values).(string)
		if !ok {
			t.Fatal("address publication is not JSON string")
		}
		var payload addrEventPayload
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Name != "xe0" || payload.Unit != 100 || payload.Index != 7 {
			t.Fatalf("address identity %+v", payload)
		}
		got = append(got, payload.Address)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("replayed %v, want %v", got, want)
	}
	select {
	case extra := <-values:
		t.Fatalf("unexpected extra publication %v", extra)
	default:
	}
}

func TestVPPAddressReplaySeedsExistingAndRetainsLiveChanges(t *testing.T) {
	ch := newReplayChannel()
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &replayEventBus{}
	if err := b.StartMonitor(bus); err != nil {
		t.Fatal(err)
	}
	defer b.StopMonitor()
	// These mutations happen before the late subscriber starts.
	if err := b.RemoveAddress("xe0.100", "192.0.2.7/24"); err != nil {
		t.Fatal(err)
	}
	if err := b.AddAddress("xe0.100", "192.0.2.8/24"); err != nil {
		t.Fatal(err)
	}
	values, stop := replaySubscription(bus, ifaceevents.EventAddrAdded)
	defer stop()
	for range 2 {
		requestAddressReplay(t, bus)
		requireVPPReplay(t, values, []string{"192.0.2.8", "2001:db8::7"})
	}
}

func TestVPPAddressReplayStopAndRestart(t *testing.T) {
	ch := newReplayChannel()
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &replayEventBus{}
	if err := b.StartMonitor(bus); err != nil {
		t.Fatal(err)
	}
	values, stop := replaySubscription(bus, ifaceevents.EventAddrAdded)
	defer stop()
	requestAddressReplay(t, bus)
	requireVPPReplay(t, values, []string{"192.0.2.7", "2001:db8::7"})
	b.StopMonitor()
	requestAddressReplay(t, bus)
	select {
	case value := <-values:
		t.Fatalf("publication after StopMonitor: %v", value)
	default:
	}
	bus.mu.Lock()
	for _, h := range bus.handlers {
		if h.event == "address-replay-request" {
			t.Error("stopped monitor retained replay subscription")
		}
	}
	bus.mu.Unlock()
	if err := b.RemoveAddress("xe0.100", "192.0.2.7/24"); err != nil {
		t.Fatal(err)
	}
	if err := b.StartMonitor(bus); err != nil {
		t.Fatal(err)
	}
	defer b.StopMonitor()
	requestAddressReplay(t, bus)
	requireVPPReplay(t, values, []string{"2001:db8::7"})
}

func TestVPPAddressReplayExcludesDeletedInterface(t *testing.T) {
	ch := newReplayChannel()
	ch.dump.details = append(ch.dump.details, interfaces.SwInterfaceDetails{SwIfIndex: 8, InterfaceName: "retired0"})
	ch.dump.addresses[8] = []ip_types.AddressWithPrefix{toAddressWithPrefix(netip.MustParsePrefix("198.51.100.8/24"))}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &replayEventBus{}
	if err := b.StartMonitor(bus); err != nil {
		t.Fatal(err)
	}
	defer b.StopMonitor()
	down, stopDown := replaySubscription(bus, ifaceevents.EventDown)
	defer stopDown()
	ch.push(&interfaces.SwInterfaceEvent{SwIfIndex: 8, Deleted: true})
	awaitReplayValue(t, down)
	values, stop := replaySubscription(bus, ifaceevents.EventAddrAdded)
	defer stop()
	requestAddressReplay(t, bus)
	requireVPPReplay(t, values, []string{"192.0.2.7", "2001:db8::7"})
}

func TestVPPAddressReplayCoalescesRequestsDuringPublication(t *testing.T) {
	ch := newReplayChannel()
	delete(ch.dump.addresses, 7)
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &replayEventBus{}
	if err := b.StartMonitor(bus); err != nil {
		t.Fatal(err)
	}
	defer b.StopMonitor()
	if err := b.AddAddress("xe0.100", "192.0.2.7/24"); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var first sync.Once
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	values := make(chan any, 8)
	stop := bus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrAdded, func(payload any) {
		first.Do(func() { close(entered); <-release })
		values <- payload
	})
	defer stop()
	requestAddressReplay(t, bus)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("replay did not enter publication")
	}
	for range 100 {
		requestAddressReplay(t, bus)
	}
	releaseOnce.Do(func() { close(release) })
	requireVPPReplay(t, values, []string{"192.0.2.7", "192.0.2.7"})
	if err := b.RemoveAddress("xe0.100", "192.0.2.7/24"); err != nil {
		t.Fatal(err)
	}
	if err := b.AddAddress("xe0.100", "192.0.2.8/24"); err != nil {
		t.Fatal(err)
	}
	requireVPPReplay(t, values, []string{"192.0.2.8"})
	requestAddressReplay(t, bus)
	requireVPPReplay(t, values, []string{"192.0.2.8"})
}

func TestVPPAddressReplayRemovalDuringPublication(t *testing.T) {
	ch := newReplayChannel()
	ch.dump.addresses[7] = ch.dump.addresses[7][:1]
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &replayEventBus{}
	if err := b.StartMonitor(bus); err != nil {
		t.Fatal(err)
	}
	defer b.StopMonitor()
	entered := make(chan struct{})
	release := make(chan struct{})
	var first sync.Once
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	type publication struct {
		kind    string
		payload any
	}
	ordered := make(chan publication, 16)
	stopAdds := bus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrAdded, func(payload any) {
		ordered <- publication{kind: ifaceevents.EventAddrAdded, payload: payload}
		first.Do(func() { close(entered); <-release })
	})
	defer stopAdds()
	stopRemoves := bus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrRemoved, func(payload any) {
		ordered <- publication{kind: ifaceevents.EventAddrRemoved, payload: payload}
	})
	defer stopRemoves()
	requestAddressReplay(t, bus)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("replay did not enter publication")
	}
	removed := make(chan error, 1)
	go func() { removed <- b.RemoveAddress("xe0.100", "192.0.2.7/24") }()
	requestAddressReplay(t, bus)
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-removed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("address removal did not finish after replay release")
	}
	for {
		select {
		case event := <-ordered:
			if event.kind == ifaceevents.EventAddrRemoved {
				goto withdrawn
			}
		case <-time.After(time.Second):
			t.Fatal("address removal was not published")
		}
	}
withdrawn:
	if err := b.AddAddress("xe0.100", "192.0.2.8/24"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case event := <-ordered:
			raw, ok := event.payload.(string)
			if !ok {
				t.Fatalf("payload %T", event.payload)
			}
			var payload addrEventPayload
			if err := json.Unmarshal([]byte(raw), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Address != "192.0.2.8" {
				t.Fatalf("stale address after withdrawal: %+v", payload)
			}
		case <-time.After(time.Second):
			t.Fatal("replacement address not published")
		}
		requestAddressReplay(t, bus)
	}
}
