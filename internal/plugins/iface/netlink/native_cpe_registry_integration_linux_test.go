//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- actual registry lifecycle
// Related: native_cpe_fault_fixture_integration_linux_test.go -- actual native faults

package ifacenetlink

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/pppoeclient"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginmgr "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	_ "github.com/ze-software/ze/internal/component/sysctl"
	"github.com/ze-software/ze/internal/test/userns"
)

func cpeRegistryConfig(name string) string {
	// Only the two fixed test keys reach this operator-config boundary.
	return `interface {
  backend netlink;
  ethernet zcf0 { }
  pppoe-client ` + name + ` {
    source-interface zcf0;
    authentication { username subscriber; password secret; }
    mtu 1400;
    route-priority 73;
    keepalive { interval 30; failures 3; }
    reconnect { delay-min 1; delay-max 1; }
  }
}`
}

type cpeRegistryEngine struct {
	server   *pluginserver.Server
	manager  *pluginmgr.Manager
	stopOnce sync.Once
}

// startCPERegistryEngine uses the same registered configuration delivery as the
// operator path. The caller MUST use a fresh userns child and keep its peer alive;
// stop MUST be called before the fixture closes peer/backend resources.
func startCPERegistryEngine(t *testing.T, input string) *cpeRegistryEngine {
	t.Helper()
	loaded, err := config.LoadConfig(input, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	coordinator := plugin.NewCoordinator(loaded.Tree.ToPluginMap())
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: loaded.Plugins,
		ConfiguredPaths: config.CollectContainerPaths(loaded.Tree), DataPlane: iface.BackendNameFromTree(loaded.Tree.ToMap()),
		ConfigNeedHolds: config.ConfigNeedHolds(loaded.Tree.ToMap())}, coordinator)
	if err != nil {
		t.Fatal(err)
	}
	registry.SetPluginServer(srv)
	registry.SetEventBus(srv)
	mgr := pluginmgr.NewManager()
	if err := mgr.StartAll(t.Context(), srv, nil); err != nil {
		t.Fatal(err)
	}
	srv.SetProcessSpawner(mgr)
	engine := &cpeRegistryEngine{server: srv, manager: mgr}
	t.Cleanup(func() { engine.stop(t) })
	if err := srv.StartWithContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := srv.WaitForStartupComplete(ctx); err != nil {
		t.Fatal(err)
	}
	return engine
}

// stop MUST follow startCPERegistryEngine and precede fixture teardown. It joins
// the registered engine and process manager; no test-owned server survives it.
func (engine *cpeRegistryEngine) stop(t *testing.T) {
	t.Helper()
	engine.stopOnce.Do(func() {
		engine.server.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := engine.server.Wait(ctx); err != nil {
			t.Error(err)
		}
		if raw := engine.manager.GetProcessManager(); raw != nil {
			manager, ok := raw.(*process.ProcessManager)
			if !ok {
				t.Errorf("native engine returned process manager %T", raw)
			}
			if ok {
				for _, proc := range manager.AllProcesses() {
					if err := proc.WaitEngine(ctx); err != nil {
						t.Error(err)
					}
				}
			}
		}
		if err := engine.manager.StopAll(ctx); err != nil {
			t.Error(err)
		}
	})
}

func waitCPERegistryObservation(t *testing.T, ctx context.Context, name string, accept func(iface.NativePPPoEObservation) bool) iface.NativePPPoEObservation {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		observation, found, err := iface.ObserveNativePPPoERegistryForTest(name)
		if err != nil {
			if !errors.Is(err, iface.ErrNativePPPoEObservationBusy) {
				t.Fatal(err)
			}
		} else if found {
			if accept(observation) {
				return observation
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("actual registry observation for %s missing", name)
		}
	}
}

// TestNativeCPERegistryReadmitsOnlyAfterOriginalPhysicalCleanup drives the real
// config/reconcile/worker path while a submitted native request has uncertain
// receive custody. Reload stops that original outside the map lock. A genuine
// queued ACK then settles its SAME writer, physical cleanup finishes, and the
// new configured client performs a second real PPP negotiation and native writes
// with a distinct original backend and creator. Nothing fabricates readmission.
func TestNativeCPERegistryReadmitsOnlyAfterOriginalPhysicalCleanup(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := newCPENativeFault(t, cpeUncertainACK, false, false, true)
	iface.SetPPPoEDialer(h.dialer)
	t.Cleanup(func() { iface.SetPPPoEDialer(&pppoeclient.Dialer{}) })
	engine := startCPERegistryEngine(t, cpeRegistryConfig("faultwan"))
	cpeFaultWait(t, h.ctx, h.backend.ackEntered, "actual registry client did not reach submitted receive")
	original := waitCPERegistryObservation(t, h.ctx, "faultwan", func(o iface.NativePPPoEObservation) bool { return o.NativeState == "writing" })
	loaded, err := config.LoadConfig(cpeRegistryConfig("nextwan"), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	reloadDone := make(chan error, 1)
	go func() { reloadDone <- engine.server.ReloadConfig(h.ctx, loaded.Tree.ToPluginMap()) }()
	joined := false
	t.Cleanup(func() {
		h.releaseACK()
		h.releaseCleanup()
		if !joined {
			select {
			case <-reloadDone:
			case <-h.ctx.Done():
				t.Error("native reload worker did not join")
			}
		}
	})
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		stopped, err := original.Client.Stopped()
		if err != nil {
			t.Fatal(err)
		}
		if stopped {
			break
		}
		select {
		case <-ticker.C:
		case <-h.ctx.Done():
			t.Fatal("real reload did not stop its original")
		}
	}
	// The original is blocked at actual ACK reception and Stop has entered.
	// No other registry writer is active here; holding this mutex across Stop
	// is an immediate explicit busy observation, not a process-timeout verdict.
	observed, found, err := iface.ObserveNativePPPoERegistryForTest("faultwan")
	if err != nil {
		t.Fatalf("registry mutex spans actual native Stop: %v", err)
	}
	if !found {
		t.Fatal("reload forgot pending original before cleanup")
	}
	if observed.Client != original.Client {
		t.Fatal("reload replaced original client identity early")
	}
	if h.factoryCalls.Load() != 1 {
		t.Fatal("reload admitted native replacement before original outcome")
	}
	h.releaseACK()
	writeErr := cpeFaultWaitResult(t, h, h.backend.writeResult)
	if !errors.Is(writeErr, iface.ErrNativePPPPending) {
		t.Fatalf("actual write uncertainty missing: %v", writeErr)
	}
	cpeFaultWait(t, h.ctx, h.dialer.cleanupEntered, "actual retained callback missing")
	settle, cancel := context.WithTimeout(h.ctx, 5*time.Second)
	err = h.backend.facet.Settle(settle)
	cancel()
	if errors.Is(err, iface.ErrNativePPPPending) {
		t.Fatalf("genuine late ACK did not settle: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("settlement erased real receive history: %v", err)
	}
	h.releaseCleanup()
	select {
	case err = <-reloadDone:
		joined = true
		if err != nil {
			t.Fatal(err)
		}
	case <-h.ctx.Done():
		t.Fatal("real reload did not finish")
	}
	cpeFaultWait(t, h.ctx, h.backend.backendDone, "original backend never physically completed")
	var second *cpeNativeFaultBackend
	select {
	case second = <-h.secondBackend:
	case <-h.ctx.Done():
		t.Fatal("new actual original backend never admitted")
	}
	if second == h.backend {
		t.Fatal("readmission reused original backend owner")
	}
	cpeFaultWait(t, h.ctx, second.bound, "second actual bind missing")
	if second.unit == h.backend.unit {
		t.Fatal("readmission reused original created unit owner")
	}
	for range 2 {
		select {
		case peer := <-h.peerReady:
			if peer == nil {
				t.Fatal("actual wire epoch supplied no session")
			}
		case <-h.ctx.Done():
			t.Fatal("two actual wire epochs never finished")
		}
	}
	current := waitCPERegistryObservation(t, h.ctx, "nextwan", func(o iface.NativePPPoEObservation) bool {
		if o.NativeState != "idle" {
			return false
		}
		return second.mutations.Load() == 4
	})
	if current.Registry != original.Registry {
		t.Fatal("reload abandoned actual registry")
	}
	if current.Client == original.Client {
		t.Fatal("new config resurrected stopped original client")
	}
	link, err := netlink.LinkByIndex(int(second.unit.Identity().NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	if link.Type() != "ppp" {
		t.Fatalf("replacement is not real PPP: %s", link.Type())
	}
	if link.Attrs().Name != "nextwan" {
		t.Fatalf("actual replacement name: %s", link.Attrs().Name)
	}
	if second.mutations.Load() != 4 {
		t.Fatalf("second actual consumer has not applied its useful state: %d", second.mutations.Load())
	}
	if h.backend.sends.Load() != 1 {
		t.Fatal("original request was resent during reload")
	}
	engine.stop(t)
}

// TestNativeCPERegistryRestartRetainsFailedCloseOriginal uses an actual kernel
// socket-close denial, then stops and starts the registered engine. The SAME
// old client/map and real partial socket remain retained; a desired new worker
// demonstrably reaches its retry state but cannot enter another native Dial.
func TestNativeCPERegistryRestartRetainsFailedCloseOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := newCPENativeFault(t, cpeSocketClose, true, false, false)
	iface.SetPPPoEDialer(h.dialer)
	t.Cleanup(func() { iface.SetPPPoEDialer(&pppoeclient.Dialer{}) })
	first := startCPERegistryEngine(t, cpeRegistryConfig("faultwan"))
	peer := cpeFaultWaitInstalled(t, h)
	original := waitCPERegistryObservation(t, h.ctx, "faultwan", func(o iface.NativePPPoEObservation) bool { return o.NativeState == "idle" })
	if err := peer.SendPADT(); err != nil {
		t.Fatal(err)
	}
	h.releaseCleanup()
	err := cpeFaultWaitResult(t, h, h.dialer.firstCleanup)
	if !errors.Is(err, unix.EPERM) {
		t.Fatalf("actual socket-close cause missing: %v", err)
	}
	if !errors.Is(err, iface.ErrNativePPPPending) {
		t.Fatalf("actual close failure became history: %v", err)
	}
	first.stop(t)
	afterStop := waitCPERegistryObservation(t, h.ctx, "faultwan", func(o iface.NativePPPoEObservation) bool { return o.CleanupPending })
	if afterStop.Registry != original.Registry {
		t.Fatal("shutdown overwrote original registry")
	}
	if afterStop.Client != original.Client {
		t.Fatal("shutdown discarded actual original")
	}
	second := startCPERegistryEngine(t, cpeRegistryConfig("nextwan"))
	afterRestart := waitCPERegistryObservation(t, h.ctx, "faultwan", func(o iface.NativePPPoEObservation) bool { return o.CleanupPending })
	if afterRestart.Registry != original.Registry {
		t.Fatal("restart created a replacement registry")
	}
	if afterRestart.Client != original.Client {
		t.Fatal("restart dropped exact pending client")
	}
	waitCPERegistryObservation(t, h.ctx, "nextwan", func(o iface.NativePPPoEObservation) bool { return o.Attempts > 0 })
	if h.dialer.calls.Load() != 1 {
		t.Fatal("restarted registry admitted another actual Dial")
	}
	if h.factoryCalls.Load() != 1 {
		t.Fatal("restarted registry captured another native backend")
	}
	if h.backend.backendCloses.Load() != 0 {
		t.Fatal("restart released pending original backend")
	}
	if !h.backend.facet.ClosePending() {
		t.Fatal("actual still-live socket lost custody")
	}
	if err := h.backend.unit.WaitNativePPPUnitRetired(h.ctx); err != nil {
		t.Fatalf("unit did not advance despite socket failure: %v", err)
	}
	second.stop(t)
}
