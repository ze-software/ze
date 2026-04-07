//go:build linux

package ifacenetlink

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// emittedEvent records a single (namespace, eventType, payload) tuple as
// seen by collectingEventBus.
type emittedEvent struct {
	Namespace string
	EventType string
	Payload   string
}

// collectingEventBus is a minimal ze.EventBus that records every emission.
// Subscribe is a no-op because the tests drive the monitor's handlers
// directly and inspect the recorded emissions instead of waiting for
// cross-component delivery.
type collectingEventBus struct {
	mu     sync.Mutex
	events []emittedEvent
}

func (b *collectingEventBus) Emit(namespace, eventType, payload string) (int, error) {
	b.mu.Lock()
	b.events = append(b.events, emittedEvent{
		Namespace: namespace,
		EventType: eventType,
		Payload:   payload,
	})
	b.mu.Unlock()
	return 0, nil
}

func (b *collectingEventBus) Subscribe(_, _ string, _ func(string)) func() {
	return func() {}
}

func (b *collectingEventBus) snapshot() []emittedEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]emittedEvent, len(b.events))
	copy(cp, b.events)
	return cp
}

func newTestMonitor() (*monitor, *collectingEventBus) {
	bus := &collectingEventBus{}
	m := newMonitor(bus)
	return m, bus
}

func TestHandleLinkUpdate_Create(t *testing.T) {
	// VALIDATES: First RTM_NEWLINK for an index emits (interface, created).
	// PREVENTS: New interfaces misclassified as state changes.
	m, bus := newTestMonitor()

	lu := netlink.LinkUpdate{
		Link: &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{
			Name: "eth0", Index: 5, MTU: 1500,
		}},
	}
	lu.Header = unix.NlMsghdr{Type: unix.RTM_NEWLINK}

	m.handleLinkUpdate(lu)

	events := bus.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Namespace != plugin.NamespaceInterface {
		t.Errorf("namespace = %q, want %q", events[0].Namespace, plugin.NamespaceInterface)
	}
	if events[0].EventType != plugin.EventInterfaceCreated {
		t.Errorf("event type = %q, want %q", events[0].EventType, plugin.EventInterfaceCreated)
	}
}

func TestHandleLinkUpdate_StateChange(t *testing.T) {
	// VALIDATES: Second RTM_NEWLINK for same index emits (interface, up/down).
	// PREVENTS: State changes on existing interfaces misclassified as created.
	m, bus := newTestMonitor()

	attrs := netlink.LinkAttrs{Name: "eth0", Index: 5, MTU: 1500, OperState: netlink.OperUp}
	lu := netlink.LinkUpdate{Link: &netlink.Dummy{LinkAttrs: attrs}}
	lu.Header = unix.NlMsghdr{Type: unix.RTM_NEWLINK}

	m.handleLinkUpdate(lu)
	events := bus.snapshot()
	if events[0].EventType != plugin.EventInterfaceCreated {
		t.Fatalf("first event type = %q, want %q", events[0].EventType, plugin.EventInterfaceCreated)
	}

	m.handleLinkUpdate(lu)
	events = bus.snapshot()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].EventType != plugin.EventInterfaceUp {
		t.Errorf("second event type = %q, want %q", events[1].EventType, plugin.EventInterfaceUp)
	}

	attrs.OperState = netlink.OperDown
	lu.Link = &netlink.Dummy{LinkAttrs: attrs}
	m.handleLinkUpdate(lu)
	events = bus.snapshot()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[2].EventType != plugin.EventInterfaceDown {
		t.Errorf("third event type = %q, want %q", events[2].EventType, plugin.EventInterfaceDown)
	}
}

func TestHandleLinkUpdate_Delete(t *testing.T) {
	// VALIDATES: RTM_DELLINK emits (interface, down) and clears tracking.
	// PREVENTS: Stale index tracking after interface removal.
	m, bus := newTestMonitor()

	attrs := netlink.LinkAttrs{Name: "eth0", Index: 5, MTU: 1500}
	lu := netlink.LinkUpdate{Link: &netlink.Dummy{LinkAttrs: attrs}}

	lu.Header = unix.NlMsghdr{Type: unix.RTM_NEWLINK}
	m.handleLinkUpdate(lu)

	lu.Header = unix.NlMsghdr{Type: unix.RTM_DELLINK}
	m.handleLinkUpdate(lu)
	events := bus.snapshot()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].EventType != plugin.EventInterfaceDown {
		t.Errorf("delete event type = %q, want %q (down)", events[1].EventType, plugin.EventInterfaceDown)
	}

	lu.Header = unix.NlMsghdr{Type: unix.RTM_NEWLINK}
	m.handleLinkUpdate(lu)
	events = bus.snapshot()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[2].EventType != plugin.EventInterfaceCreated {
		t.Errorf("re-create event type = %q, want %q", events[2].EventType, plugin.EventInterfaceCreated)
	}
}

func TestAddrUpdateToEventType(t *testing.T) {
	// VALIDATES: Address update direction maps to correct event type.
	// PREVENTS: Addr added/removed events swapped.
	tests := []struct {
		name  string
		isNew bool
		want  string
	}{
		{"addr added", true, plugin.EventInterfaceAddrAdded},
		{"addr removed", false, plugin.EventInterfaceAddrRemoved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addrUpdateToEventType(tt.isNew)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveVLANUnit(t *testing.T) {
	// VALIDATES: VLAN subinterface events resolve to parent name + unit.
	// PREVENTS: VLAN events emitted without unit context.
	tests := []struct {
		name       string
		ifaceName  string
		wantParent string
		wantUnit   int
		wantVLAN   bool
	}{
		{"plain interface", "eth0", "eth0", 0, false},
		{"vlan subinterface", "eth0.100", "eth0", 100, true},
		{"vlan with dots in parent", "br0.50", "br0", 50, true},
		{"no dot suffix", "lo", "lo", 0, false},
		{"dot but not numeric", "eth0.abc", "eth0.abc", 0, false},
		{"leading dot", ".100", ".100", 0, false},
		{"empty string", "", "", 0, false},
		{"negative vlan", "eth0.-1", "eth0.-1", 0, false},
		{"vlan zero", "eth0.0", "eth0", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent, unit, isVLAN := resolveVLANUnit(tt.ifaceName)
			if parent != tt.wantParent {
				t.Errorf("parent = %q, want %q", parent, tt.wantParent)
			}
			if unit != tt.wantUnit {
				t.Errorf("unit = %d, want %d", unit, tt.wantUnit)
			}
			if isVLAN != tt.wantVLAN {
				t.Errorf("isVLAN = %v, want %v", isVLAN, tt.wantVLAN)
			}
		})
	}
}

func TestIsLinkUp(t *testing.T) {
	// VALIDATES: Link up detection handles OperUp, OperUnknown+IFF_UP, and down.
	// PREVENTS: Virtual interfaces (loopback, dummy) incorrectly treated as down.
	tests := []struct {
		name     string
		oper     netlink.LinkOperState
		rawFlags uint32
		wantUp   bool
	}{
		{"oper up", netlink.OperUp, 0, true},
		{"oper down", netlink.OperDown, 0, false},
		{"oper unknown with IFF_UP", netlink.OperUnknown, unix.IFF_UP, true},
		{"oper unknown without IFF_UP", netlink.OperUnknown, 0, false},
		{"oper dormant", netlink.OperDormant, unix.IFF_UP, false},
		{"oper testing", netlink.OperTesting, unix.IFF_UP, false},
		{"oper lower layer down", netlink.OperLowerLayerDown, unix.IFF_UP, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := &netlink.LinkAttrs{OperState: tt.oper, RawFlags: tt.rawFlags}
			got := isLinkUp(attrs)
			if got != tt.wantUp {
				t.Errorf("isLinkUp(%v, flags=%x) = %v, want %v",
					tt.oper, tt.rawFlags, got, tt.wantUp)
			}
		})
	}
}

func TestAddrFamily(t *testing.T) {
	// VALIDATES: Address family detection returns correct string.
	// PREVENTS: Wrong family in event payload.
	tests := []struct {
		name   string
		addr   string
		want   string
		wantOK bool
	}{
		{"ipv4", "10.0.0.1/24", "ipv4", true},
		{"ipv6", "fd00::1/64", "ipv6", true},
		{"ipv4 host", "192.168.1.1/32", "ipv4", true},
		{"ipv6 loopback", "::1/128", "ipv6", true},
		{"invalid", "not-an-ip", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := addrFamily(tt.addr)
			if ok != tt.wantOK {
				t.Errorf("addrFamily(%q) ok = %v, want %v", tt.addr, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("addrFamily(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestHandleLinkUpdate_NilAttrs(t *testing.T) {
	// VALIDATES: handleLinkUpdate handles nil attrs without panic.
	// PREVENTS: Nil dereference on malformed netlink events.
	m, bus := newTestMonitor()

	lu := netlink.LinkUpdate{Link: &nilAttrsLink{}}
	lu.Header = unix.NlMsghdr{Type: unix.RTM_NEWLINK}

	m.handleLinkUpdate(lu)
	if len(bus.snapshot()) != 0 {
		t.Errorf("expected 0 events for nil attrs, got %d", len(bus.snapshot()))
	}
}

type nilAttrsLink struct{}

func (n *nilAttrsLink) Attrs() *netlink.LinkAttrs { return nil }
func (n *nilAttrsLink) Type() string              { return "nil" }

func TestEmitCreatedPayload(t *testing.T) {
	// VALIDATES: Created event payload has correct JSON structure.
	// PREVENTS: Malformed JSON emitted on the EventBus.
	m, bus := newTestMonitor()

	attrs := netlink.LinkAttrs{Name: "eth0", Index: 5, MTU: 9000}
	lu := netlink.LinkUpdate{Link: &netlink.Dummy{LinkAttrs: attrs}}
	lu.Header = unix.NlMsghdr{Type: unix.RTM_NEWLINK}

	m.handleLinkUpdate(lu)

	events := bus.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	var payload linkEventPayload
	if err := json.Unmarshal([]byte(events[0].Payload), &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if payload.Name != "eth0" {
		t.Errorf("name = %q, want %q", payload.Name, "eth0")
	}
	if payload.Index != 5 {
		t.Errorf("index = %d, want %d", payload.Index, 5)
	}
	if payload.MTU != 9000 {
		t.Errorf("mtu = %d, want %d", payload.MTU, 9000)
	}
}

var _ = nl.FAMILY_ALL
