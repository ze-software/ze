// VALIDATES: the DHCP client goroutine lifecycle drives the real kernel paths —
// Start spawns the v4 worker (iface.Resolve + raw-socket DORA attempt) and Stop
// tears it down and returns promptly. Auto-enrolled in the QEMU integration run
// via the derived `integration && linux` package list.
// PREVENTS: a Start/Stop regression that leaks a worker goroutine or blocks on a
// raw socket that never observes the stop signal.

//go:build integration && linux

package ifacedhcp

import (
	"testing"
	"time"
)

func TestDHCPClientStartStopLifecycle(t *testing.T) {
	bus := &recordingBus{}
	// Loopback always exists; there is no DHCP server on it, so the worker will
	// loop on retry and must exit cleanly the moment Stop closes the stop channel.
	c, err := newDHCPClient("lo", "0", bus, true, false, dHCPConfig{})
	if err != nil {
		t.Fatalf("newDHCPClient: %v", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("Stop did not return within 15s: worker goroutine leaked or blocked")
	}
}
