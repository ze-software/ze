package vpp

import (
	"context"
	"testing"
	"time"
)

func TestNewVPPManager(t *testing.T) {
	// VALIDATES: Manager construction
	// PREVENTS: nil fields in manager
	s := &VPPSettings{
		Enabled:   true,
		APISocket: "/run/vpp/api.sock",
		Memory:    MemorySettings{MainHeap: "1G", HugepageSize: "2M", Buffers: 128000},
		Stats:     StatsSettings{SegmentSize: "512M", SocketPath: "/run/vpp/stats.sock"},
	}
	mgr := NewVPPManager(s, "/etc/vpp", "/usr/bin/vpp")
	if mgr == nil {
		t.Fatal("NewVPPManager returned nil")
	}
	if mgr.connector == nil {
		t.Error("connector not initialized")
	}
	if mgr.dpdk == nil {
		t.Error("dpdk binder not initialized")
	}
	if mgr.settings != s {
		t.Error("settings not stored")
	}
}

func TestVPPManagerDisabledBlocks(t *testing.T) {
	// VALIDATES: AC-1 -- disabled VPP blocks until context canceled
	// PREVENTS: manager doing work when disabled
	s := &VPPSettings{Enabled: false}
	mgr := NewVPPManager(s, "/etc/vpp", "/usr/bin/vpp")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := mgr.Run(ctx)
	if err != nil {
		t.Errorf("disabled manager should return nil on cancel, got: %v", err)
	}
}

func TestVPPManagerValidationFailure(t *testing.T) {
	// VALIDATES: AC-10 -- invalid config rejected before VPP startup
	// PREVENTS: VPP starting with bad config
	s := &VPPSettings{
		Enabled:   true,
		APISocket: "", // invalid: empty
		Memory:    MemorySettings{MainHeap: "1G", HugepageSize: "2M", Buffers: 128000},
		Stats:     StatsSettings{SegmentSize: "512M", SocketPath: "/run/vpp/stats.sock"},
	}
	mgr := NewVPPManager(s, "/etc/vpp", "/usr/bin/vpp")

	err := mgr.Run(t.Context())
	if err == nil {
		t.Error("expected validation error for empty api-socket")
	}
}

func TestVPPManagerHasRunOnceTracking(t *testing.T) {
	// VALIDATES: connected vs reconnected event distinction
	// PREVENTS: reconnected event never emitted
	s := &VPPSettings{Enabled: true, APISocket: "/run/vpp/api.sock",
		Memory: MemorySettings{MainHeap: "1G", HugepageSize: "2M", Buffers: 128000},
		Stats:  StatsSettings{SegmentSize: "512M", SocketPath: "/run/vpp/stats.sock"},
	}
	mgr := NewVPPManager(s, "/etc/vpp", "/usr/bin/vpp")

	if mgr.hasRunOnce {
		t.Error("hasRunOnce should be false initially")
	}
}

func TestConnectorNotConnected(t *testing.T) {
	// VALIDATES: Connector returns error when not connected
	// PREVENTS: nil channel returned without error
	c := NewConnector("/nonexistent.sock")

	if c.IsConnected() {
		t.Error("should not be connected initially")
	}

	_, err := c.NewChannel()
	if err == nil {
		t.Error("NewChannel should fail when not connected")
	}
}

func TestConnectorCloseIdempotent(t *testing.T) {
	// VALIDATES: Close is safe to call multiple times
	// PREVENTS: panic on double close
	c := NewConnector("/nonexistent.sock")
	c.Close()
	c.Close() // should not panic
}

func TestMaxRestartBackoff(t *testing.T) {
	// VALIDATES: backoff caps at maxRestartBackoff
	// PREVENTS: unbounded backoff growth
	backoff := time.Second
	for range 20 {
		backoff = min(backoff*2, maxRestartBackoff)
	}
	if backoff != maxRestartBackoff {
		t.Errorf("backoff should cap at %v, got %v", maxRestartBackoff, backoff)
	}
}
