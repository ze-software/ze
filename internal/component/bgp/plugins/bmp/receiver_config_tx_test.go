// Design: docs/architecture/config/transaction-protocol.md -- BMP bind failures
// must propagate across the real SDK configure, apply and rollback callbacks.
// Related: locrib_runtime_config_test.go -- shared pipe/SDK startup harness.

package bmp

import (
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestBMPReceiverConfigureReturnsBindFailure requires Stage 2 to fail rather
// than silently accepting the subset of endpoints the host let it bind.
func TestBMPReceiverConfigureReturnsBindFailure(t *testing.T) {
	blocked := receiverBindListen(t)
	cfg := receiverBindConfig(t, "4", map[string]string{
		"blocked": blocked.Addr().String(), "available": receiverBindAvailableAddress(t),
	})
	bp, _, err := receiverBindRuntimeConfig(t, []sdk.ConfigSection{receiverBindSection(t, cfg)})
	receiverBindRequireRefusal(t, err, blocked.Addr().String())
	if len(bp.listeners) != 0 {
		t.Errorf("refused configure left %d listeners bound", len(bp.listeners))
	}
}

// TestBMPReceiverApplyFailurePreservesAllRoots changes every root in the
// candidate. The SDK's failed-own-apply gate cannot repair partial publication.
func TestBMPReceiverApplyFailurePreservesAllRoots(t *testing.T) {
	original := receiverBindConfig(t, "4", map[string]string{"old": "127.0.0.1:0"})
	bp, engine, err := receiverBindRuntimeConfig(t, []sdk.ConfigSection{
		receiverBindSection(t, original),
		{Root: configRootSystem, Data: `{"system":{"host":"before"}}`},
		{Root: configRootBGP, Data: senderJSON(t, map[string]any{"route-mirroring": "false"})},
	})
	if err != nil {
		t.Fatal(err)
	}
	old := bp.listeners["127.0.0.1:0"]
	if old == nil {
		t.Fatal("initial receiver was not bound")
	}
	blocked := receiverBindListen(t)
	candidate := receiverBindConfig(t, "8", map[string]string{"blocked": blocked.Addr().String()})
	err = engine.reloadSections([]rpc.ConfigSection{
		{Root: configRootSystem, Data: `{"system":{"host":"after"}}`},
		receiverBindSection(t, candidate),
		{Root: configRootBGP, Data: senderJSON(t, map[string]any{"route-mirroring": "true"})},
	})
	receiverBindRequireRefusal(t, err, blocked.Addr().String())
	bp.mu.RLock()
	kept := bp.listeners["127.0.0.1:0"] == old
	name := bp.systemName
	mirroring := bp.routeMirroring
	bp.mu.RUnlock()
	if !kept {
		t.Error("refused apply removed the previous listener")
	}
	if name == nil || name.Host == nil || *name.Host != "before" {
		t.Errorf("refused apply changed system identity to %v", name)
	}
	if mirroring {
		t.Error("refused apply changed sender route mirroring")
	}
	if cap := bp.maxSessions.Load(); cap != 4 {
		t.Errorf("refused apply changed receiver cap to %d, want 4", cap)
	}
	// A failed transaction must not poison the next accepted candidate.
	if err := engine.rollback(); err != nil {
		t.Fatalf("rollback after refusal: %v", err)
	}
	if err := engine.reloadSections([]rpc.ConfigSection{receiverBindSection(t, original)}); err != nil {
		t.Fatalf("valid reload following refusal: %v", err)
	}
	if bp.listeners["127.0.0.1:0"] != old {
		t.Error("valid reload after refusal replaced the original socket")
	}
	bp.mu.RLock()
	name = bp.systemName
	mirroring = bp.routeMirroring
	bp.mu.RUnlock()
	if name == nil || name.Host == nil || *name.Host != "before" {
		t.Errorf("receiver-only repair leaked the failed candidate identity: %v", name)
	}
	if mirroring {
		t.Error("receiver-only repair leaked failed candidate sender mirroring")
	}
	if cap := bp.maxSessions.Load(); cap != 4 {
		t.Errorf("receiver-only repair cap %d, want original cap 4", cap)
	}
}

// TestBMPReceiverRollbackRestoresPriorConfig proves a successful apply can be
// undone through the SDK, including the previous cap and exact listen address.
func TestBMPReceiverRollbackRestoresPriorConfig(t *testing.T) {
	oldAddr := receiverBindAvailableAddress(t)
	newAddr := receiverBindAvailableAddress(t)
	before := receiverBindConfig(t, "4", map[string]string{"old": oldAddr})
	bp, engine, err := receiverBindRuntimeConfig(t, []sdk.ConfigSection{receiverBindSection(t, before)})
	if err != nil {
		t.Fatal(err)
	}
	after := receiverBindConfig(t, "8", map[string]string{"new": newAddr})
	if err := engine.reloadSections([]rpc.ConfigSection{receiverBindSection(t, after)}); err != nil {
		t.Fatal(err)
	}
	receiverBindDial(t, newAddr)
	if err := engine.rollback(); err != nil {
		t.Fatal(err)
	}
	receiverBindDial(t, oldAddr)
	if bp.maxSessions.Load() != 4 {
		t.Fatalf("rollback cap %d, want 4", bp.maxSessions.Load())
	}
	var lc net.ListenConfig
	reclaimed, err := lc.Listen(t.Context(), "tcp", newAddr)
	if err != nil {
		t.Fatalf("rollback retained candidate listener %s: %v", newAddr, err)
	}
	closeLog(reclaimed, "rollback-reclaimed")
}

// TestBMPReceiverRollbackFailureIsReturned takes the old endpoint while the
// candidate runs. Rollback must report its inability to restore, preserving the
// candidate set until the transaction's existing recovery handles it.
func TestBMPReceiverRollbackFailureIsReturned(t *testing.T) {
	oldAddr := receiverBindAvailableAddress(t)
	newAddr := receiverBindAvailableAddress(t)
	before := receiverBindConfig(t, "4", map[string]string{"old": oldAddr})
	bp, engine, err := receiverBindRuntimeConfig(t, []sdk.ConfigSection{receiverBindSection(t, before)})
	if err != nil {
		t.Fatal(err)
	}
	after := receiverBindConfig(t, "8", map[string]string{"new": newAddr})
	if err := engine.reloadSections([]rpc.ConfigSection{receiverBindSection(t, after)}); err != nil {
		t.Fatal(err)
	}
	var lc net.ListenConfig
	holder, err := lc.Listen(t.Context(), "tcp", oldAddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeLog(holder, "rollback-port-holder") })
	err = engine.rollback()
	if err == nil {
		t.Error("SDK rollback reported success although the previous address was occupied")
	} else if !strings.Contains(err.Error(), oldAddr) {
		t.Errorf("rollback error %q does not name endpoint %s", err, oldAddr)
	}
	if bp.listeners[newAddr] == nil {
		t.Error("failed rollback destroyed the candidate listener before restoring its predecessor")
	}
	if cap := bp.maxSessions.Load(); cap != 8 {
		t.Errorf("failed rollback changed cap to %d, want candidate cap 8", cap)
	}
	receiverBindDial(t, newAddr)
	if err := holder.Close(); err != nil {
		t.Fatal(err)
	}
	if err := engine.reloadSections([]rpc.ConfigSection{receiverBindSection(t, before)}); err != nil {
		t.Fatalf("repair reload after failed rollback: %v", err)
	}
	receiverBindDial(t, oldAddr)
}

// receiverBindRuntimeConfig closes the SDK transport before the shared
// harness joins receiver sessions. Their teardown can issue a withdraw request;
// no engine command handler runs in this callback-only fixture.
func receiverBindRuntimeConfig(t *testing.T, sections []sdk.ConfigSection) (*BMPPlugin, *reloadEngine, error) {
	t.Helper()
	bp, engine, err := startBMPRuntimeConfig(t, sections)
	t.Cleanup(func() { closeLog(bp.plugin, "receiver-test-plugin") })
	return bp, engine, err
}

func receiverBindSection(t *testing.T, cfg *receiverConfig) sdk.ConfigSection {
	t.Helper()
	data, err := json.Marshal(map[string]any{"environment": map[string]any{"bmp": cfg}})
	if err != nil {
		t.Fatal(err)
	}
	return sdk.ConfigSection{Root: configRootEnvironment, Data: string(data)}
}

func receiverBindRequireRefusal(t *testing.T, err error, address string) {
	t.Helper()
	if err == nil {
		t.Error("SDK accepted a receiver set whose configured port was already held")
		return
	}
	for _, part := range []string{address, "blocked", "delete environment bmp server blocked"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("receiver refusal %q does not name %q", err, part)
		}
	}
}
