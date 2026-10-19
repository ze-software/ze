// VALIDATES: gap #55 owed item 3, OSPF side. An IPv4 address leaving an open
// interface closes its OSPF socket, and the interface stays out of OSPF while
// it holds no IPv4 address; an added address opens it again.
// METHOD: drive the transport's iface address subscriptions through a bus that
// keeps the handlers, over a backend that refuses an open while the device has
// no IPv4 address (the real backend's interfaceIPv4 refusal).
package transport

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
)

type handlerBus struct {
	mu       sync.Mutex
	handlers map[string]func(any)
}

func (b *handlerBus) Emit(_, _ string, _ any) (int, error) { return 0, nil }

func (b *handlerBus) Subscribe(_, eventType string, fn func(any)) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = fn
	return func() {}
}

func (b *handlerBus) fire(eventType, payload string) {
	b.mu.Lock()
	fn := b.handlers[eventType]
	b.mu.Unlock()
	fn(payload)
}

// addrBackend refuses an open while noIPv4 is set, as interfaceIPv4 does.
type addrBackend struct {
	mu     sync.Mutex
	noIPv4 bool
	inner  *fakeBackend
}

func (b *addrBackend) OpenInterface(name string, r dropRecorder) (InterfaceHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.noIPv4 {
		return nil, errors.New("ospf/transport: interface eth0 has no IPv4 address")
	}
	return b.inner.OpenInterface(name, r)
}

func (b *addrBackend) setNoIPv4(v bool) {
	b.mu.Lock()
	b.noIPv4 = v
	b.mu.Unlock()
}

func TestAddressLossTakesInterfaceOut(t *testing.T) {
	backend := &addrBackend{inner: newFakeBackend()}
	tr := New(backend)
	tr.subscribe = func(string) (<-chan iface.LinkEvent, func()) { return make(chan iface.LinkEvent), func() {} }
	var mu sync.Mutex
	var downs, ups int
	tr.OnInterfaceDown(func(int, string) { mu.Lock(); downs++; mu.Unlock() })
	tr.OnInterfaceUp(func(int, string) { mu.Lock(); ups++; mu.Unlock() })
	bus := &handlerBus{handlers: make(map[string]func(any))}
	cleanup := tr.subscribeIfaceEventsWithRescan(bus, 0)
	defer cleanup()
	tr.EnableInterface("eth0")
	if err := tr.HandleLinkUp("eth0"); err != nil {
		t.Fatalf("HandleLinkUp: %v", err)
	}

	isOpen := func() bool {
		tr.mu.Lock()
		defer tr.mu.Unlock()
		_, open := tr.interfaces["eth0"]
		return open
	}
	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	// The donor went down: iface withdrew the borrowed /32, the last IPv4.
	backend.setNoIPv4(true)
	bus.fire(ifaceevents.EventAddrRemoved, `{"name":"eth0","address":"10.255.0.1","prefix-length":32,"family":"ipv4"}`)
	waitFor("the interface to leave OSPF", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return downs == 1 && !isOpen()
	})

	// An IPv6 event never touches the OSPFv2 socket.
	bus.fire(ifaceevents.EventAddrAdded, `{"name":"eth0","address":"2001:db8::1","prefix-length":128,"family":"ipv6"}`)

	// The donor recovered: iface restored the /32.
	backend.setNoIPv4(false)
	bus.fire(ifaceevents.EventAddrAdded, `{"name":"eth0","address":"10.255.0.1","prefix-length":32,"family":"ipv4"}`)
	waitFor("the interface to return to OSPF", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return ups == 2 && isOpen()
	})
}
