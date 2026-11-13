//go:build linux

// Design: docs/features/interfaces.md -- retained address publication

package ifacenetlink

import (
	"encoding/json"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
)

// replayBus delivers synchronously like the engine, including reentrant emits.
type replayBus struct {
	mu       sync.Mutex
	next     int
	handlers map[int]replayHandler
	observed chan emittedEvent
}

type replayHandler struct {
	namespace, event string
	fn               func(any)
}

func newReplayBus() *replayBus {
	return &replayBus{handlers: make(map[int]replayHandler), observed: make(chan emittedEvent, 128)}
}

func (b *replayBus) Emit(namespace, event string, payload any) (int, error) {
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
	b.observed <- emittedEvent{Namespace: namespace, EventType: event, Payload: payload}
	return len(handlers), nil
}

func (b *replayBus) Subscribe(namespace, event string, fn func(any)) func() {
	b.mu.Lock()
	b.next++
	id := b.next
	b.handlers[id] = replayHandler{namespace: namespace, event: event, fn: fn}
	b.mu.Unlock()
	return func() { b.mu.Lock(); delete(b.handlers, id); b.mu.Unlock() }
}

func awaitReplayEvent(t *testing.T, events <-chan emittedEvent, event string) emittedEvent {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		select {
		case got := <-events:
			if got.EventType == event {
				return got
			}
		case <-deadline.C:
			t.Fatalf("no %s publication from monitor owner", event)
		}
	}
}

func replayAddr(cidr string, added bool) netlink.AddrUpdate {
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	network.IP = ip
	return netlink.AddrUpdate{LinkIndex: 7, LinkAddress: *network, NewAddr: added}
}

func startReplayOwner(t *testing.T, bus *replayBus) (*monitor, chan netlink.LinkUpdate, chan netlink.AddrUpdate) {
	t.Helper()
	m := newMonitor(bus)
	m.linkNames.Store(7, "xe0.100")
	links := make(chan netlink.LinkUpdate)
	addresses := make(chan netlink.AddrUpdate)
	neighbors := make(chan netlink.NeighUpdate)
	m.started.Store(true)
	go m.run(links, addresses, neighbors)
	t.Cleanup(func() {
		select {
		case <-m.done:
		default:
			close(addresses)
		}
		m.stop()
	})
	return m, links, addresses
}

func subscribeReplayAdds(bus *replayBus) (<-chan emittedEvent, func()) {
	added := make(chan emittedEvent, 32)
	stop := bus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrAdded, func(payload any) {
		added <- emittedEvent{EventType: ifaceevents.EventAddrAdded, Payload: payload}
	})
	return added, stop
}

func requestReplay(t *testing.T, bus *replayBus) {
	t.Helper()
	if _, err := bus.Emit(ifaceevents.Namespace, "address-replay-request", nil); err != nil {
		t.Fatal(err)
	}
}

func requireReplayAddresses(t *testing.T, added <-chan emittedEvent, want []string) {
	t.Helper()
	got := make([]string, 0, len(want))
	for range want {
		event := awaitReplayEvent(t, added, ifaceevents.EventAddrAdded)
		raw, ok := event.Payload.(string)
		if !ok {
			t.Fatalf("address payload %T", event.Payload)
		}
		var addr addrEventPayload
		if err := json.Unmarshal([]byte(raw), &addr); err != nil {
			t.Fatal(err)
		}
		if addr.Name != "xe0" || addr.Unit != 100 || addr.Index != 7 {
			t.Fatalf("address identity %+v", addr)
		}
		got = append(got, addr.Address)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("replayed addresses %v, want %v", got, want)
	}
	select {
	case extra := <-added:
		t.Fatalf("unexpected extra address %+v", extra)
	default:
	}
}

// The owner is warm before the subscriber exists. No further kernel event
// follows the request, so a live-only implementation cannot satisfy this test.
func TestMonitorAddressReplayLateSubscriber(t *testing.T) {
	bus := newReplayBus()
	_, _, addresses := startReplayOwner(t, bus)
	for _, cidr := range []string{"192.0.2.7/24", "2001:db8::7/64"} {
		addresses <- replayAddr(cidr, true)
		awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrAdded)
	}
	added, stop := subscribeReplayAdds(bus)
	defer stop()
	for range 2 {
		requestReplay(t, bus)
		requireReplayAddresses(t, added, []string{"192.0.2.7", "2001:db8::7"})
	}
}

func TestMonitorAddressReplayExcludesRemovedTentativeAndDeletedLink(t *testing.T) {
	bus := newReplayBus()
	_, links, addresses := startReplayOwner(t, bus)
	addresses <- replayAddr("192.0.2.8/24", true)
	awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrAdded)
	addresses <- replayAddr("192.0.2.8/24", false)
	awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrRemoved)
	tentative := replayAddr("2001:db8::8/64", true)
	tentative.Flags = unix.IFA_F_TENTATIVE
	addresses <- tentative
	addresses <- replayAddr("192.0.2.9/24", true)
	awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrAdded)
	links <- netlink.LinkUpdate{Header: unix.NlMsghdr{Type: unix.RTM_DELLINK}, Link: &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "xe0.100", Index: 7}}}
	awaitReplayEvent(t, bus.observed, ifaceevents.EventDown)
	links <- netlink.LinkUpdate{Header: unix.NlMsghdr{Type: unix.RTM_NEWLINK}, Link: &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "xe0.100", Index: 7}}}
	awaitReplayEvent(t, bus.observed, ifaceevents.EventDown)
	addresses <- replayAddr("192.0.2.10/24", true)
	awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrAdded)
	added, stop := subscribeReplayAdds(bus)
	defer stop()
	requestReplay(t, bus)
	requireReplayAddresses(t, added, []string{"192.0.2.10"})
}

func TestMonitorAddressReplayStopAndRestart(t *testing.T) {
	bus := newReplayBus()
	m, _, addresses := startReplayOwner(t, bus)
	addresses <- replayAddr("192.0.2.7/24", true)
	awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrAdded)
	added, stop := subscribeReplayAdds(bus)
	defer stop()
	requestReplay(t, bus)
	requireReplayAddresses(t, added, []string{"192.0.2.7"})
	stopped := make(chan struct{})
	go func() { m.stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		close(addresses)
		<-stopped
		t.Fatal("monitor did not stop with producer channels open")
	}
	requestReplay(t, bus)
	select {
	case extra := <-added:
		t.Fatalf("publication after stop: %+v", extra)
	default:
	}
	bus.mu.Lock()
	for _, h := range bus.handlers {
		if h.event == "address-replay-request" {
			t.Error("stopped monitor retained replay subscription")
		}
	}
	bus.mu.Unlock()
	_, _, fresh := startReplayOwner(t, bus)
	fresh <- replayAddr("192.0.2.8/24", true)
	requireReplayAddresses(t, added, []string{"192.0.2.8"})
	requestReplay(t, bus)
	requireReplayAddresses(t, added, []string{"192.0.2.8"})
}

func TestMonitorAddressReplayWithdrawsAddressThatBecomesUnusable(t *testing.T) {
	for _, flag := range []int{unix.IFA_F_TENTATIVE, unix.IFA_F_DADFAILED} {
		t.Run(netlinkFlagName(flag), func(t *testing.T) {
			bus := newReplayBus()
			_, _, addresses := startReplayOwner(t, bus)
			addresses <- replayAddr("2001:db8::7/64", true)
			awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrAdded)
			unusable := replayAddr("2001:db8::7/64", true)
			unusable.Flags = flag
			addresses <- unusable
			awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrRemoved)
			addresses <- replayAddr("192.0.2.7/24", true)
			awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrAdded)
			added, stop := subscribeReplayAdds(bus)
			defer stop()
			requestReplay(t, bus)
			requireReplayAddresses(t, added, []string{"192.0.2.7"})
		})
	}
}

func netlinkFlagName(flag int) string {
	if flag == unix.IFA_F_TENTATIVE {
		return "tentative"
	}
	return "dad-failed"
}

func TestMonitorAddressReplayTracksRenamedInterface(t *testing.T) {
	bus := newReplayBus()
	_, links, addresses := startReplayOwner(t, bus)
	addresses <- replayAddr("192.0.2.7/24", true)
	awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrAdded)
	added, stop := subscribeReplayAdds(bus)
	defer stop()
	links <- netlink.LinkUpdate{Header: unix.NlMsghdr{Type: unix.RTM_NEWLINK}, Link: &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "renamed.200", Index: 7}}}
	removed := awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrRemoved)
	if raw, ok := removed.Payload.(string); !ok || !json.Valid([]byte(raw)) {
		t.Fatalf("invalid withdrawal %+v", removed)
	}
	awaitReplayEvent(t, added, ifaceevents.EventAddrAdded)
	requestReplay(t, bus)
	got := awaitReplayEvent(t, added, ifaceevents.EventAddrAdded)
	raw, ok := got.Payload.(string)
	if !ok {
		t.Fatalf("payload %T", got.Payload)
	}
	var payload addrEventPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	want := addrEventPayload{Name: "renamed", Unit: 200, Index: 7, Address: "192.0.2.7", PrefixLength: 24, Family: "ipv4", Origin: "dynamic"}
	if payload != want {
		t.Fatalf("renamed address %+v, want %+v", payload, want)
	}
}

func TestMonitorAddressReplayCoalescesRequestsDuringPublication(t *testing.T) {
	bus := newReplayBus()
	_, _, addresses := startReplayOwner(t, bus)
	addresses <- replayAddr("192.0.2.7/24", true)
	awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrAdded)
	entered := make(chan struct{})
	release := make(chan struct{})
	var first sync.Once
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	added := make(chan emittedEvent, 8)
	stop := bus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrAdded, func(payload any) {
		first.Do(func() {
			close(entered)
			<-release
		})
		added <- emittedEvent{EventType: ifaceevents.EventAddrAdded, Payload: payload}
	})
	defer stop()
	requestReplay(t, bus)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("replay did not enter publication")
	}
	for range 100 {
		requestReplay(t, bus)
	}
	releaseOnce.Do(func() { close(release) })
	// The active replay and one coalesced pending replay each publish once.
	requireReplayAddresses(t, added, []string{"192.0.2.7", "192.0.2.7"})
	addresses <- replayAddr("192.0.2.7/24", false)
	awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrRemoved)
	addresses <- replayAddr("192.0.2.8/24", true)
	requireReplayAddresses(t, added, []string{"192.0.2.8"})
	requestReplay(t, bus)
	requireReplayAddresses(t, added, []string{"192.0.2.8"})
}

func TestMonitorAddressReplayRemovalDuringPublication(t *testing.T) {
	bus := newReplayBus()
	_, _, addresses := startReplayOwner(t, bus)
	addresses <- replayAddr("192.0.2.7/24", true)
	awaitReplayEvent(t, bus.observed, ifaceevents.EventAddrAdded)
	entered := make(chan struct{})
	release := make(chan struct{})
	var first sync.Once
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	ordered := make(chan emittedEvent, 16)
	stopAdds := bus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrAdded, func(payload any) {
		ordered <- emittedEvent{EventType: ifaceevents.EventAddrAdded, Payload: payload}
		first.Do(func() { close(entered); <-release })
	})
	defer stopAdds()
	stopRemoves := bus.Subscribe(ifaceevents.Namespace, ifaceevents.EventAddrRemoved, func(payload any) {
		ordered <- emittedEvent{EventType: ifaceevents.EventAddrRemoved, Payload: payload}
	})
	defer stopRemoves()
	requestReplay(t, bus)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("replay did not enter publication")
	}
	go func() { addresses <- replayAddr("192.0.2.7/24", false) }()
	requestReplay(t, bus)
	releaseOnce.Do(func() { close(release) })
	awaitReplayEvent(t, ordered, ifaceevents.EventAddrRemoved)
	// The replacement is a barrier in the same owner. No old add is allowed
	// after the withdrawal, including a pending replay's publication.
	addresses <- replayAddr("192.0.2.8/24", true)
	requireReplayAddresses(t, ordered, []string{"192.0.2.8"})
	requestReplay(t, bus)
	requireReplayAddresses(t, ordered, []string{"192.0.2.8"})
}
