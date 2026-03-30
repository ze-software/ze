//go:build integration && linux

package iface

import (
	"testing"
	"time"

	"github.com/vishvananda/netlink"
)

const monitorEventTimeout = 5 * time.Second

func TestIntegrationMonitorLinkCreated(t *testing.T) {
	// VALIDATES: Monitor publishes interface/created when a link appears.
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

		ev := waitForEvent(t, bus, TopicCreated, monitorEventTimeout)
		if name, ok := ev.Metadata["name"]; !ok || name != "test0" {
			t.Errorf("event name = %q, want %q", name, "test0")
		}
	})
}

func TestIntegrationMonitorAddrAdded(t *testing.T) {
	// VALIDATES: Monitor publishes interface/addr/added when an IP is assigned.
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
		waitForEvent(t, bus, TopicCreated, monitorEventTimeout)

		if err := AddAddress("test0", "10.77.0.1/24"); err != nil {
			t.Fatalf("AddAddress: %v", err)
		}

		ev := waitForEvent(t, bus, TopicAddrAdded, monitorEventTimeout)
		if addr, ok := ev.Metadata["address"]; !ok || addr != "10.77.0.1" {
			t.Errorf("event address = %q, want %q", addr, "10.77.0.1")
		}
		if fam, ok := ev.Metadata["family"]; !ok || fam != "ipv4" {
			t.Errorf("event family = %q, want %q", fam, "ipv4")
		}
	})
}

func TestIntegrationMonitorAddrRemoved(t *testing.T) {
	// VALIDATES: Monitor publishes interface/addr/removed when an IP is removed.
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
		waitForEvent(t, bus, TopicCreated, monitorEventTimeout)

		if err := AddAddress("test0", "10.77.0.1/24"); err != nil {
			t.Fatalf("AddAddress: %v", err)
		}
		waitForEvent(t, bus, TopicAddrAdded, monitorEventTimeout)

		if err := RemoveAddress("test0", "10.77.0.1/24"); err != nil {
			t.Fatalf("RemoveAddress: %v", err)
		}

		ev := waitForEvent(t, bus, TopicAddrRemoved, monitorEventTimeout)
		if addr, ok := ev.Metadata["address"]; !ok || addr != "10.77.0.1" {
			t.Errorf("event address = %q, want %q", addr, "10.77.0.1")
		}
	})
}

func TestIntegrationMonitorLinkDeleted(t *testing.T) {
	// VALIDATES: Monitor publishes interface/deleted when a link is removed.
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
		waitForEvent(t, bus, TopicCreated, monitorEventTimeout)

		if err := DeleteInterface("test0"); err != nil {
			t.Fatalf("DeleteInterface: %v", err)
		}

		ev := waitForEvent(t, bus, TopicDeleted, monitorEventTimeout)
		if name, ok := ev.Metadata["name"]; !ok || name != "test0" {
			t.Errorf("event name = %q, want %q", name, "test0")
		}
	})
}

func TestIntegrationMonitorLinkUpDown(t *testing.T) {
	// VALIDATES: Monitor publishes interface/down and interface/up on state changes.
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
		waitForEvent(t, bus, TopicCreated, monitorEventTimeout)

		// Bring the link down.
		link, err := netlink.LinkByName("test0")
		if err != nil {
			t.Fatalf("LinkByName: %v", err)
		}
		if err := netlink.LinkSetDown(link); err != nil {
			t.Fatalf("LinkSetDown: %v", err)
		}

		waitForEvent(t, bus, TopicDown, monitorEventTimeout)

		// Bring the link back up.
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatalf("LinkSetUp: %v", err)
		}

		waitForEvent(t, bus, TopicUp, monitorEventTimeout)
	})
}
