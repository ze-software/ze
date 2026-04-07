//go:build integration && linux

package iface

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

const monitorEventTimeout = 5 * time.Second

// unmarshalEvent decodes a collected event's JSON payload into the
// generic address+name+unit record the interface monitor publishes.
// Used by tests to assert on payload fields (name, address, family).
type monitorTestPayload struct {
	Name         string `json:"name"`
	Unit         int    `json:"unit"`
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix-length"`
	Family       string `json:"family"`
}

func decodeMonitorPayload(t *testing.T, ev collectedEvent) monitorTestPayload {
	t.Helper()
	var p monitorTestPayload
	if err := json.Unmarshal([]byte(ev.Payload), &p); err != nil {
		t.Fatalf("decode payload: %v (payload=%q)", err, ev.Payload)
	}
	return p
}

func TestIntegrationMonitorLinkCreated(t *testing.T) {
	// VALIDATES: Monitor emits (interface, created) when a link appears.
	// PREVENTS: Monitor not detecting new interfaces via netlink.
	withNetNS(t, func() {
		bus := &collectingBus{}
		mon, err := NewMonitor(bus)
		if err != nil {
			t.Fatalf("NewMonitor: %v", err)
		}
		if err := mon.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { mon.Stop() })

		// Give the monitor time to subscribe to netlink.
		time.Sleep(200 * time.Millisecond)

		createDummyForTest(t, "test0")

		ev := waitForEvent(t, bus, plugin.NamespaceInterface, plugin.EventInterfaceCreated, monitorEventTimeout)
		p := decodeMonitorPayload(t, ev)
		if p.Name != "test0" {
			t.Errorf("event name = %q, want %q", p.Name, "test0")
		}
	})
}

func TestIntegrationMonitorAddrAdded(t *testing.T) {
	// VALIDATES: Monitor emits (interface, addr-added) when an IP is assigned.
	// PREVENTS: Address events lost or wrong family reported.
	withNetNS(t, func() {
		bus := &collectingBus{}
		mon, err := NewMonitor(bus)
		if err != nil {
			t.Fatalf("NewMonitor: %v", err)
		}
		if err := mon.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { mon.Stop() })

		time.Sleep(200 * time.Millisecond)

		createDummyForTest(t, "test0")
		// Wait for the created event first.
		waitForEvent(t, bus, plugin.NamespaceInterface, plugin.EventInterfaceCreated, monitorEventTimeout)

		if err := AddAddress("test0", "10.77.0.1/24"); err != nil {
			t.Fatalf("AddAddress: %v", err)
		}

		ev := waitForEvent(t, bus, plugin.NamespaceInterface, plugin.EventInterfaceAddrAdded, monitorEventTimeout)
		p := decodeMonitorPayload(t, ev)
		if p.Address != "10.77.0.1" {
			t.Errorf("event address = %q, want %q", p.Address, "10.77.0.1")
		}
		if p.Family != "ipv4" {
			t.Errorf("event family = %q, want %q", p.Family, "ipv4")
		}
	})
}

func TestIntegrationMonitorAddrRemoved(t *testing.T) {
	// VALIDATES: Monitor emits (interface, addr-removed) when an IP is removed.
	// PREVENTS: Removal events lost, leaving stale state.
	withNetNS(t, func() {
		bus := &collectingBus{}
		mon, err := NewMonitor(bus)
		if err != nil {
			t.Fatalf("NewMonitor: %v", err)
		}
		if err := mon.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { mon.Stop() })

		time.Sleep(200 * time.Millisecond)

		createDummyForTest(t, "test0")
		waitForEvent(t, bus, plugin.NamespaceInterface, plugin.EventInterfaceCreated, monitorEventTimeout)

		if err := AddAddress("test0", "10.77.0.1/24"); err != nil {
			t.Fatalf("AddAddress: %v", err)
		}
		waitForEvent(t, bus, plugin.NamespaceInterface, plugin.EventInterfaceAddrAdded, monitorEventTimeout)

		if err := RemoveAddress("test0", "10.77.0.1/24"); err != nil {
			t.Fatalf("RemoveAddress: %v", err)
		}

		ev := waitForEvent(t, bus, plugin.NamespaceInterface, plugin.EventInterfaceAddrRemoved, monitorEventTimeout)
		p := decodeMonitorPayload(t, ev)
		if p.Address != "10.77.0.1" {
			t.Errorf("event address = %q, want %q", p.Address, "10.77.0.1")
		}
	})
}

func TestIntegrationMonitorLinkDeleted(t *testing.T) {
	// VALIDATES: Monitor emits (interface, down) when a link is removed.
	// PREVENTS: Deletion events lost, leaving stale tracking state.
	withNetNS(t, func() {
		bus := &collectingBus{}
		mon, err := NewMonitor(bus)
		if err != nil {
			t.Fatalf("NewMonitor: %v", err)
		}
		if err := mon.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { mon.Stop() })

		time.Sleep(200 * time.Millisecond)

		if err := CreateDummy("test0"); err != nil {
			t.Fatalf("CreateDummy: %v", err)
		}
		waitForEvent(t, bus, plugin.NamespaceInterface, plugin.EventInterfaceCreated, monitorEventTimeout)

		if err := DeleteInterface("test0"); err != nil {
			t.Fatalf("DeleteInterface: %v", err)
		}

		// After migration the deletion event is (interface, down) because
		// the stream registry has no separate "deleted" event type; down
		// is the closest semantic match.
		ev := waitForEvent(t, bus, plugin.NamespaceInterface, plugin.EventInterfaceDown, monitorEventTimeout)
		p := decodeMonitorPayload(t, ev)
		if p.Name != "test0" {
			t.Errorf("event name = %q, want %q", p.Name, "test0")
		}
	})
}

func TestIntegrationMonitorLinkUpDown(t *testing.T) {
	// VALIDATES: Monitor emits (interface, down) and (interface, up) on state changes.
	// PREVENTS: State change events not emitted for admin up/down transitions.
	withNetNS(t, func() {
		bus := &collectingBus{}
		mon, err := NewMonitor(bus)
		if err != nil {
			t.Fatalf("NewMonitor: %v", err)
		}
		if err := mon.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}
		t.Cleanup(func() { mon.Stop() })

		time.Sleep(200 * time.Millisecond)

		createDummyForTest(t, "test0")
		// CreateDummy brings the link UP, so we get a created event.
		waitForEvent(t, bus, plugin.NamespaceInterface, plugin.EventInterfaceCreated, monitorEventTimeout)

		// Bring the link down.
		link, err := netlink.LinkByName("test0")
		if err != nil {
			t.Fatalf("LinkByName: %v", err)
		}
		if err := netlink.LinkSetDown(link); err != nil {
			t.Fatalf("LinkSetDown: %v", err)
		}

		waitForEvent(t, bus, plugin.NamespaceInterface, plugin.EventInterfaceDown, monitorEventTimeout)

		// Bring the link back up.
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatalf("LinkSetUp: %v", err)
		}

		waitForEvent(t, bus, plugin.NamespaceInterface, plugin.EventInterfaceUp, monitorEventTimeout)
	})
}
