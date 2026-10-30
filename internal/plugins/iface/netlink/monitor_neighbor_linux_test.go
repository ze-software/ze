//go:build linux

package ifacenetlink

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
)

// neighborChanges returns the neighbor-changed payloads the bus recorded.
func neighborChanges(t *testing.T, bus *collectingEventBus) []iface.NeighborEventPayload {
	t.Helper()
	var out []iface.NeighborEventPayload
	for _, event := range bus.snapshot() {
		if event.EventType != ifaceevents.EventNeighborChanged {
			continue
		}
		raw, ok := event.Payload.(string)
		if !ok {
			t.Fatalf("payload type %T, want the JSON string emit writes", event.Payload)
		}
		var payload iface.NeighborEventPayload
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			t.Fatalf("payload %q: %v", raw, err)
		}
		out = append(out, payload)
	}
	return out
}

// neighUpdate builds one netlink neighbor notification on link 7.
func neighUpdate(msgType uint16, ip, mac string, state int) netlink.NeighUpdate {
	nu := netlink.NeighUpdate{Type: msgType}
	nu.LinkIndex = 7
	nu.IP = net.ParseIP(ip)
	nu.State = state
	if mac != "" {
		hw, err := net.ParseMAC(mac)
		if err != nil {
			panic("BUG: test MAC " + mac + ": " + err.Error())
		}
		nu.HardwareAddr = hw
	}
	return nu
}

// TestNeighbourChangedEventEmitted proves the monitor emits
// (interface, neighbor-changed) for a GLOBAL IPv6 neighbor as well as a
// Link-Local one, on add, MAC change, a resolved/unresolved crossing and
// delete, and stays silent on a flip inside one class (REACHABLE to STALE),
// on a repeat of the same state, and for IPv4. BGP re-reads the neighbor table
// on this event for RFC 2545 Section 3's third-party Link-Local.
func TestNeighbourChangedEventEmitted(t *testing.T) {
	m, bus := newTestMonitor()
	m.linkNames.Store(7, "eth0")

	const global, mac, other = "2001:db8:1::9", "02:00:00:00:00:09", "02:00:00:00:00:0a"
	steps := []struct {
		name   string
		update netlink.NeighUpdate
		emits  bool
	}{
		{"global add", neighUpdate(unix.RTM_NEWNEIGH, global, mac, netlink.NUD_REACHABLE), true},
		{"reachable to stale", neighUpdate(unix.RTM_NEWNEIGH, global, mac, netlink.NUD_STALE), false},
		{"same state again", neighUpdate(unix.RTM_NEWNEIGH, global, mac, netlink.NUD_STALE), false},
		{"mac change", neighUpdate(unix.RTM_NEWNEIGH, global, other, netlink.NUD_STALE), true},
		{"to failed", neighUpdate(unix.RTM_NEWNEIGH, global, "", netlink.NUD_FAILED), true},
		{"back to reachable", neighUpdate(unix.RTM_NEWNEIGH, global, other, netlink.NUD_REACHABLE), true},
		{"link-local add", neighUpdate(unix.RTM_NEWNEIGH, "fe80::9", other, netlink.NUD_PERMANENT), true},
		{"global delete", neighUpdate(unix.RTM_DELNEIGH, global, other, netlink.NUD_REACHABLE), true},
		{"unknown delete", neighUpdate(unix.RTM_DELNEIGH, "2001:db8:1::8", other, netlink.NUD_REACHABLE), false},
		{"ipv4 ignored", neighUpdate(unix.RTM_NEWNEIGH, "192.0.2.9", mac, netlink.NUD_REACHABLE), false},
	}
	want := 0
	for _, step := range steps {
		m.handleNeighUpdate(step.update)
		if step.emits {
			want++
		}
		if got := len(neighborChanges(t, bus)); got != want {
			t.Fatalf("after %q: %d neighbor-changed events, want %d", step.name, got, want)
		}
	}

	changes := neighborChanges(t, bus)
	if first := changes[0]; first.Name != "eth0" || first.Address != global || first.LinkAddress != mac || !first.Resolved {
		t.Errorf("add payload %+v, want eth0 %s %s resolved", first, global, mac)
	}
	if failed := changes[2]; failed.Resolved || failed.LinkAddress != "" {
		t.Errorf("failed payload %+v, want unresolved with no link address", failed)
	}
	if deleted := changes[len(changes)-1]; deleted.Address != global || deleted.Resolved {
		t.Errorf("delete payload %+v, want %s unresolved", deleted, global)
	}

	// The router events keep their own contract: none of the updates above
	// carried NTF_ROUTER, so no router event was emitted.
	for _, event := range bus.snapshot() {
		if event.EventType == ifaceevents.EventRouterDiscovered || event.EventType == ifaceevents.EventRouterLost {
			t.Errorf("unexpected router event %s", event.EventType)
		}
	}
}
