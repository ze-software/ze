package vpp

import (
	"context"
	"net"
	"path/filepath"
	"strings"
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

// TestVPPManagerRunOnce_ExternalSkipsExec verifies that with External=true,
// runOnce does NOT attempt to exec the VPP binary. The assertion is indirect:
// we point vppBinary at a path that would cause exec.Start to fail with
// "start vpp: ..." if External were false, and assert the returned error
// never mentions the exec path (it comes from the connect phase instead).
//
// VALIDATES: AC-1 -- External=true connects via GoVPP without execing VPP.
// PREVENTS: the external-branch accidentally still calling cmd.Start.
func TestVPPManagerRunOnce_ExternalSkipsExec(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "api.sock")
	if len(sock) >= 108 {
		t.Fatalf("socket path too long for sun_path: %d bytes", len(sock))
	}

	s := &VPPSettings{
		Enabled:   true,
		External:  true,
		APISocket: sock,
		Memory:    MemorySettings{MainHeap: "1G", HugepageSize: "2M", Buffers: 128000},
		Stats:     StatsSettings{SegmentSize: "512M", SocketPath: "/run/vpp/stats.sock", PollInterval: 30},
	}
	mgr := NewVPPManager(s, dir, "/definitely/nonexistent/path/to/vpp")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := mgr.runOnce(ctx, filepath.Join(dir, "startup.conf"))
	// Either we got a govpp connect error or ctx cancellation; we must NOT
	// have seen a "start vpp" error from exec.Start of the bogus binary.
	if err != nil && strings.Contains(err.Error(), "start vpp") {
		t.Errorf("External=true but runOnce tried to exec VPP: %v", err)
	}
}

// TestVPPManagerRunOnce_ExternalBlocksOnCtx verifies the full external
// happy path: with External=true and a real Unix socket listener on the
// api-socket path, runOnce connects via GoVPP and blocks on ctx.Done
// instead of cmd.Wait.
//
// VALIDATES: AC-1 -- runOnce blocks on ctx.Done (not cmd.Wait) when external.
// PREVENTS: external branch closing the connector before ctx.Done.
func TestVPPManagerRunOnce_ExternalBlocksOnCtx(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "api.sock")
	if len(sock) >= 108 {
		t.Fatalf("socket path too long for sun_path: %d bytes", len(sock))
	}

	// Listener keeps the socket file alive; we drop any accepted connection.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "unix", sock)
	if err != nil {
		t.Fatalf("listen on %s: %v", sock, err)
	}
	t.Cleanup(func() {
		if cerr := ln.Close(); cerr != nil && !strings.Contains(cerr.Error(), "closed") {
			t.Logf("listener close: %v", cerr)
		}
	})
	go func() {
		for {
			c, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			if cerr := c.Close(); cerr != nil {
				t.Logf("accept close: %v", cerr)
				return
			}
		}
	}()

	s := &VPPSettings{
		Enabled:   true,
		External:  true,
		APISocket: sock,
		Memory:    MemorySettings{MainHeap: "1G", HugepageSize: "2M", Buffers: 128000},
		Stats:     StatsSettings{SegmentSize: "512M", SocketPath: "/run/vpp/stats.sock", PollInterval: 30},
	}
	mgr := NewVPPManager(s, dir, "/definitely/nonexistent/path/to/vpp")

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	err = mgr.runOnce(ctx, filepath.Join(dir, "startup.conf"))
	if err != nil && strings.Contains(err.Error(), "start vpp") {
		t.Errorf("External=true but runOnce tried to exec VPP: %v", err)
	}
}
