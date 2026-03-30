//go:build integration && linux

package iface

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestIntegrationMigrateFullCycle(t *testing.T) {
	// VALIDATES: MigrateInterface creates new interface, adds address, waits for
	// BGP readiness, and removes the old address.
	// PREVENTS: Migration fails in real kernel or rollback logic is wrong.
	withNetNS(t, func() {
		// Create "old0" dummy with an address.
		createDummyForTest(t, "old0")
		if err := AddAddress("old0", "10.88.0.1/24"); err != nil {
			t.Fatalf("AddAddress old0: %v", err)
		}
		requireAddress(t, "old0", "10.88.0.1/24")

		bus := &subscribableBus{}

		cfg := MigrateConfig{
			OldIface:     "old0",
			NewIface:     "new0",
			NewIfaceType: "dummy",
			Address:      "10.88.0.1/24",
		}

		// Run MigrateInterface in a goroutine since it blocks waiting for BGP readiness.
		errCh := make(chan error, 1)
		go func() {
			errCh <- MigrateInterface(cfg, bus, 10*time.Second)
		}()

		// Give MigrateInterface time to reach phase 3 (subscribe and wait).
		time.Sleep(1 * time.Second)

		// Simulate BGP readiness by publishing a matching event.
		payload, _ := json.Marshal(map[string]string{"address": "10.88.0.1"})
		bus.Publish(topicBGPListenerReady, payload, map[string]string{"address": "10.88.0.1"})

		// Wait for MigrateInterface to complete.
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("MigrateInterface: %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("MigrateInterface timed out")
		}

		// Verify: "new0" exists with the address.
		if !linkExists("new0") {
			t.Error("new0 should exist after migration")
		}
		requireAddress(t, "new0", "10.88.0.1/24")

		// Verify: "old0" no longer has the address.
		requireNoAddress(t, "old0", "10.88.0.1/24")

		// Cleanup.
		t.Cleanup(func() { _ = DeleteInterface("new0") })
	})
}

func TestIntegrationMigrateTimeout(t *testing.T) {
	// VALIDATES: MigrateInterface rolls back when BGP readiness times out.
	// PREVENTS: Leaked interfaces/addresses on timeout without rollback.
	withNetNS(t, func() {
		createDummyForTest(t, "old0")
		if err := AddAddress("old0", "10.88.0.1/24"); err != nil {
			t.Fatalf("AddAddress old0: %v", err)
		}

		bus := &subscribableBus{}

		cfg := MigrateConfig{
			OldIface:     "old0",
			NewIface:     "new0",
			NewIfaceType: "dummy",
			Address:      "10.88.0.1/24",
		}

		// Use a short timeout and do NOT publish BGP readiness.
		err := MigrateInterface(cfg, bus, 1*time.Second)
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Errorf("error = %q, want containing 'timed out'", err.Error())
		}

		// Verify rollback: new0 should not have the address (or should not exist).
		// The rollback removes the address from new0 and deletes the interface.
		if linkExists("new0") {
			// If new0 still exists, it should not have the migrated address.
			requireNoAddress(t, "new0", "10.88.0.1/24")
		}

		// old0 should still have its address (it was never removed because
		// phase 4 was not reached).
		requireAddress(t, "old0", "10.88.0.1/24")
	})
}
