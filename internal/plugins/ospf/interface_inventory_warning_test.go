// Design: docs/architecture/ospf/ospf-4-component-config.md -- inventory warning delivery.
package ospf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// newInventoryWarningFixture opens a protocol-only engine, then attaches the
// real SDK bridge. Bootstrap persistence is outside these delivery regressions.
func newInventoryWarningFixture(t *testing.T, onReport func(context.Context, rpc.ReportIssueInput) error) (*engine, *prefixOriginationBackend, func()) {
	t.Helper()
	installPrefixOriginationBackend(t)
	backend, ok := iface.GetBackend().(*prefixOriginationBackend)
	if !ok {
		t.Fatal("prefix origination backend was not loaded")
	}
	client := newInventoryReportClient(t, onReport)
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","instance-id":"7","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"lan0":{"area":"0","passive":"true"}}}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	stop := sync.OnceFunc(eng.shutdown)
	t.Cleanup(stop)
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatal(err)
	}
	synctest.Wait()
	eng.state = client
	return eng, backend, stop
}

func newInventoryReportClient(t *testing.T, onReport func(context.Context, rpc.ReportIssueInput) error) *sdk.Plugin {
	t.Helper()
	pluginEnd, engineEnd := net.Pipe()
	bridge := rpc.NewDirectBridge()
	bridge.SetDispatchRPC(func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		if method != rpc.MethodReportIssue {
			return nil, fmt.Errorf("unexpected SDK method %s", method)
		}
		var in rpc.ReportIssueInput
		if err := json.Unmarshal(params, &in); err != nil {
			return nil, err
		}
		if err := onReport(ctx, in); err != nil {
			return nil, err
		}
		return json.RawMessage(`{}`), nil
	})
	bridge.SetReady()
	client := sdk.NewWithConn(Namespace, rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
		if err := engineEnd.Close(); err != nil {
			t.Error(err)
		}
	})
	return client
}

func inventoryMaintenanceTick() {
	synctest.Wait()
	// sleep(timer): advance the existing one-second OSPF maintenance ticker;
	// the delivery contract retries there and never on a topology lookup.
	time.Sleep(time.Second)
	synctest.Wait()
}

func requireInventoryReport(t *testing.T, requests <-chan rpc.ReportIssueInput, action rpc.ReportAction) rpc.ReportIssueInput {
	t.Helper()
	select {
	case got := <-requests:
		if got.Action != action {
			t.Fatalf("report action = %s, want %s: %+v", got.Action, action, got)
		}
		if got.Code != "doctor-ospf-interface-addresses-unavailable" {
			t.Errorf("report code = %q", got.Code)
		}
		if got.Subject != "lan0 instance 7" {
			t.Errorf("report subject = %q, want lan0 instance 7", got.Subject)
		}
		return got
	default:
		t.Fatalf("maintenance sent no %s through the SDK bridge", action)
		return rpc.ReportIssueInput{}
	}
}

func assertNoInventoryReport(t *testing.T, requests <-chan rpc.ReportIssueInput) {
	t.Helper()
	select {
	case got := <-requests:
		t.Fatalf("topology lookup performed an SDK report RPC: %+v", got)
	default:
	}
}

func newInventoryWarningManager(t *testing.T, onReport func(context.Context, rpc.ReportIssueInput) error) (*instanceManager, *prefixOriginationBackend) {
	t.Helper()
	installPrefixOriginationBackend(t)
	backend, ok := iface.GetBackend().(*prefixOriginationBackend)
	if !ok {
		t.Fatal("prefix origination backend was not loaded")
	}
	client := newInventoryReportClient(t, onReport)
	base := newEngine(nil)
	base.state = client
	manager := newInstanceManager(base, func(uint8) *engine { return newEngine(nil) }, nil)
	t.Cleanup(manager.shutdownAll)
	return manager, backend
}

func openInventoryWarningInstance(t *testing.T, manager *instanceManager) *engine {
	t.Helper()
	manager.ensureSet([]uint8{0, 7})
	eng, ok := manager.engineFor(7)
	if !ok {
		t.Fatal("instance manager did not create instance 7")
	}
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","instance-id":"7","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"lan0":{"area":"0","passive":"true"},"lo":{"area":"0","passive":"true"}}}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatal(err)
	}
	synctest.Wait()
	return eng
}

func requireInventoryReports(t *testing.T, requests <-chan rpc.ReportIssueInput, action rpc.ReportAction) map[string]rpc.ReportIssueInput {
	t.Helper()
	want := map[string]bool{"lan0 instance 7": true, "lo instance 7": true}
	got := make(map[string]rpc.ReportIssueInput)
	for range 2 {
		select {
		case in := <-requests:
			if in.Action != action {
				t.Fatalf("report = %+v, want %s", in, action)
			}
			if in.Code != "doctor-ospf-interface-addresses-unavailable" {
				t.Errorf("report code = %q", in.Code)
			}
			if !want[in.Subject] {
				t.Errorf("unexpected or duplicate report identity %q", in.Subject)
			}
			delete(want, in.Subject)
			got[in.Subject] = in
		default:
			t.Fatalf("maintenance did not send both %s identities; got %+v", action, got)
		}
	}
	return got
}

// VALIDATES: removing instance 7 leaves failed clears with the base publisher,
// even with no base interfaces, and a failed identity does not block another.
func TestInterfaceInventoryWarningRemovedInstanceRetriesClear(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		requests := make(chan rpc.ReportIssueInput, 32)
		failedSubject := ""
		succeeded := make(map[string]bool)
		var clearMu sync.Mutex
		manager, backend := newInventoryWarningManager(t, func(_ context.Context, in rpc.ReportIssueInput) error {
			clearMu.Lock()
			defer clearMu.Unlock()
			requests <- in
			if in.Action == rpc.ReportClear {
				if failedSubject == "" {
					failedSubject = in.Subject
					return errors.New("test first instance clear unavailable")
				}
				succeeded[in.Subject] = true
			}
			return nil
		})
		eng := openInventoryWarningInstance(t, manager)
		backend.mu.Lock()
		backend.failAll = true
		backend.mu.Unlock()
		eng.lsdbTopology()
		assertNoInventoryReport(t, requests)
		inventoryMaintenanceTick()
		requireInventoryReports(t, requests, rpc.ReportRaise)
		manager.ensureSet([]uint8{0})
		assertNoInventoryReport(t, requests)
		inventoryMaintenanceTick()
		requireInventoryReports(t, requests, rpc.ReportClear)
		clearMu.Lock()
		if len(succeeded) != 1 {
			t.Errorf("one failed clear prevented the other identity's acknowledgement: %+v", succeeded)
		}
		clearMu.Unlock()
		inventoryMaintenanceTick()
		select {
		case retry := <-requests:
			if retry.Action != rpc.ReportClear {
				t.Errorf("retry = %+v, want clear", retry)
			}
			clearMu.Lock()
			expectedSubject := failedSubject
			clearMu.Unlock()
			if retry.Subject != expectedSubject {
				t.Errorf("retried subject = %q, want %q", retry.Subject, expectedSubject)
			}
		default:
			t.Fatal("removed instance's failed clear was not retried by base maintenance")
		}
		clearMu.Lock()
		if len(succeeded) != 2 {
			t.Errorf("retry did not acknowledge both identities: %+v", succeeded)
		}
		clearMu.Unlock()
		inventoryMaintenanceTick()
		assertNoInventoryReport(t, requests)
	})
}

// VALIDATES: a late successful clear from a removed instance cannot discard
// a newer failure of the same instance/name after re-enrollment.
func TestInterfaceInventoryWarningReaddedInstanceDuringClear(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		requests := make(chan rpc.ReportIssueInput, 32)
		releaseClear := make(chan struct{})
		release := sync.OnceFunc(func() { close(releaseClear) })
		blockClear := true
		manager, backend := newInventoryWarningManager(t, func(_ context.Context, in rpc.ReportIssueInput) error {
			requests <- in
			if in.Action == rpc.ReportClear {
				if blockClear {
					blockClear = false
					<-releaseClear
				}
			}
			return nil
		})
		t.Cleanup(release)
		eng := openInventoryWarningInstance(t, manager)
		backend.mu.Lock()
		backend.failAll = true
		backend.mu.Unlock()
		eng.lsdbTopology()
		inventoryMaintenanceTick()
		requireInventoryReports(t, requests, rpc.ReportRaise)
		manager.ensureSet([]uint8{0})
		inventoryMaintenanceTick()
		readded := openInventoryWarningInstance(t, manager)
		readded.lsdbTopology()
		release()
		synctest.Wait()
		requireInventoryReports(t, requests, rpc.ReportClear)
		inventoryMaintenanceTick()
		requireInventoryReports(t, requests, rpc.ReportRaise)
		inventoryMaintenanceTick()
		assertNoInventoryReport(t, requests)
	})
}

// VALIDATES: failed raises and clears are retried by maintenance with the
// original identity, while repeated topology reads perform no report RPC.
func TestInterfaceInventoryWarningRetriesOnMaintenance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		requests := make(chan rpc.ReportIssueInput, 32)
		failRaise, failClear := true, true
		eng, backend, _ := newInventoryWarningFixture(t, func(_ context.Context, in rpc.ReportIssueInput) error {
			requests <- in
			if in.Action == rpc.ReportRaise {
				if failRaise {
					failRaise = false
					return errors.New("test warning raise unavailable")
				}
			}
			if in.Action == rpc.ReportClear {
				if failClear {
					failClear = false
					return errors.New("test warning clear unavailable")
				}
			}
			return nil
		})
		backend.mu.Lock()
		backend.failName = "lan0"
		backend.mu.Unlock()
		eng.lsdbTopology()
		assertNoInventoryReport(t, requests)
		inventoryMaintenanceTick()
		first := requireInventoryReport(t, requests, rpc.ReportRaise)
		for range 4 {
			eng.lsdbTopology()
		}
		assertNoInventoryReport(t, requests)
		inventoryMaintenanceTick()
		if retry := requireInventoryReport(t, requests, rpc.ReportRaise); retry != first {
			t.Errorf("raise retry changed payload: got %+v, want %+v", retry, first)
		}
		inventoryMaintenanceTick()
		assertNoInventoryReport(t, requests)
		backend.mu.Lock()
		backend.failName = ""
		backend.mu.Unlock()
		eng.lsdbTopology()
		assertNoInventoryReport(t, requests)
		inventoryMaintenanceTick()
		clear := requireInventoryReport(t, requests, rpc.ReportClear)
		for range 4 {
			eng.lsdbTopology()
		}
		assertNoInventoryReport(t, requests)
		inventoryMaintenanceTick()
		if retry := requireInventoryReport(t, requests, rpc.ReportClear); retry != clear {
			t.Errorf("clear retry changed payload: got %+v, want %+v", retry, clear)
		}
		inventoryMaintenanceTick()
		assertNoInventoryReport(t, requests)
	})
}

// VALIDATES: recovery during an unacknowledged raise cannot lose the matching
// clear, and the SDK call holds neither the engine nor warning-state lock.
func TestInterfaceInventoryWarningRecoveryDuringRaise(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		requests := make(chan rpc.ReportIssueInput, 32)
		releaseRaise := make(chan struct{})
		release := sync.OnceFunc(func() { close(releaseRaise) })
		eng, backend, _ := newInventoryWarningFixture(t, func(_ context.Context, in rpc.ReportIssueInput) error {
			requests <- in
			if in.Action == rpc.ReportRaise {
				<-releaseRaise
			}
			return nil
		})
		t.Cleanup(release)
		backend.mu.Lock()
		backend.failName = "lan0"
		backend.mu.Unlock()
		eng.lsdbTopology()
		inventoryMaintenanceTick()
		requireInventoryReport(t, requests, rpc.ReportRaise)
		backend.mu.Lock()
		backend.failName = ""
		backend.mu.Unlock()
		eng.lsdbTopology()
		assertNoInventoryReport(t, requests)
		release()
		synctest.Wait()
		inventoryMaintenanceTick()
		requireInventoryReport(t, requests, rpc.ReportClear)
		inventoryMaintenanceTick()
		assertNoInventoryReport(t, requests)
	})
}

// VALIDATES: an inventory lookup that began before removal cannot re-raise a
// warning after closeInterface has requested its matching clear.
func TestInterfaceInventoryWarningRemovalDuringRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		requests := make(chan rpc.ReportIssueInput, 32)
		eng, backend, _ := newInventoryWarningFixture(t, func(_ context.Context, in rpc.ReportIssueInput) error {
			requests <- in
			return nil
		})
		backend.mu.Lock()
		backend.failName = "lan0"
		backend.mu.Unlock()
		eng.lsdbTopology()
		inventoryMaintenanceTick()
		requireInventoryReport(t, requests, rpc.ReportRaise)
		started, releaseRead := make(chan struct{}), make(chan struct{})
		release := sync.OnceFunc(func() { close(releaseRead) })
		t.Cleanup(release)
		backend.mu.Lock()
		backend.blockName = "lan0"
		backend.readStarted = started
		backend.releaseRead = releaseRead
		backend.mu.Unlock()
		readDone := make(chan struct{})
		go func() {
			eng.lsdbTopology()
			close(readDone)
		}()
		<-started
		eng.closeInterface("lan0")
		release()
		<-readDone
		inventoryMaintenanceTick()
		requireInventoryReport(t, requests, rpc.ReportClear)
		inventoryMaintenanceTick()
		assertNoInventoryReport(t, requests)
	})
}

// VALIDATES: success and failure from an old runtime cannot overwrite a
// newer enrollment's warning transition, even when the name and instance match.
func TestInterfaceInventoryWarningRejectsReplacedRuntime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		requests := make(chan rpc.ReportIssueInput, 32)
		eng, backend, _ := newInventoryWarningFixture(t, func(_ context.Context, in rpc.ReportIssueInput) error {
			requests <- in
			return nil
		})
		eng.mu.Lock()
		oldRuntime := eng.interfaces["lan0"]
		ic := eng.running["lan0"]
		eng.mu.Unlock()
		eng.closeInterface("lan0")
		if err := eng.openConfiguredInterface(ic); err != nil {
			t.Fatal(err)
		}
		backend.mu.Lock()
		backend.failName = "lan0"
		backend.mu.Unlock()
		eng.lsdbTopology()
		// These are the two possible completions of a lookup that retained
		// oldRuntime before close/re-enrollment, through the real guarded writer.
		eng.recordInterfaceAddressRead("lan0", 7, oldRuntime, nil)
		inventoryMaintenanceTick()
		requireInventoryReport(t, requests, rpc.ReportRaise)
		backend.mu.Lock()
		backend.failName = ""
		backend.mu.Unlock()
		eng.lsdbTopology()
		eng.recordInterfaceAddressRead("lan0", 7, oldRuntime, errors.New("stale old runtime inventory error"))
		inventoryMaintenanceTick()
		requireInventoryReport(t, requests, rpc.ReportClear)
		inventoryMaintenanceTick()
		assertNoInventoryReport(t, requests)
	})
}

// VALIDATES: a topology-only entry cannot queue an inventory warning without a
// live interface runtime, while real passive enrollment admits the same failure.
// Each engine is stopped even on failure so the rejected-path regression cannot
// leave its maintenance worker alive for a later test.
func TestInterfaceInventoryWarningRejectsAbsentRuntime(t *testing.T) {
	for _, mode := range []struct {
		name     string
		enrolled bool
	}{
		{name: "topology-only"},
		{name: "enrolled-passive", enrolled: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				installPrefixOriginationBackend(t)
				cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","instance-id":"7","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"lan0":{"area":"0","passive":"true"}}}}}`), nil)
				if err != nil {
					t.Fatal(err)
				}
				eng := newEngine(nil)
				defer eng.shutdown()
				eng.setConfig(cfg)
				if mode.enrolled {
					if err := eng.openConfiguredInterface(cfg.Interfaces[0]); err != nil {
						t.Fatal(err)
					}
				} else {
					eng.mu.Lock()
					eng.running["lan0"] = cfg.Interfaces[0]
					eng.mu.Unlock()
				}
				eng.mu.Lock()
				runtime := eng.interfaces["lan0"]
				eng.mu.Unlock()
				if mode.enrolled {
					if runtime == nil {
						t.Fatal("passive enrollment created no interface runtime")
					}
				}
				cause := errors.New("test interface inventory unavailable")
				eng.recordInterfaceAddressRead("lan0", 7, runtime, cause)
				eng.addressIssueMu.Lock()
				warning, queued := eng.addressIssues[interfaceAddressIdentity{name: "lan0", instance: 7}]
				eng.addressIssueMu.Unlock()
				if !mode.enrolled {
					if queued {
						t.Fatal("topology-only interface queued a warning without a live runtime")
					}
					return
				}
				if !queued {
					t.Fatal("live passive interface did not queue its inventory failure")
				}
				if warning.issue == nil {
					t.Fatal("live passive interface queued no inventory warning issue")
				}
				if !warning.pending {
					t.Fatal("live passive interface warning is not pending publication")
				}
			})
		})
	}
}

// VALIDATES: interface removal and engine shutdown clear the exact nonzero
// instance identity, and shutdown does not reuse its canceled engine context.
func TestInterfaceInventoryWarningClearsOnRemovalOrShutdown(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(fmt.Sprintf("shutdown=%t", shutdown), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				requests := make(chan rpc.ReportIssueInput, 32)
				eng, backend, stop := newInventoryWarningFixture(t, func(ctx context.Context, in rpc.ReportIssueInput) error {
					if err := ctx.Err(); err != nil {
						return err
					}
					requests <- in
					return nil
				})
				backend.mu.Lock()
				backend.failName = "lan0"
				backend.mu.Unlock()
				eng.lsdbTopology()
				inventoryMaintenanceTick()
				requireInventoryReport(t, requests, rpc.ReportRaise)
				if shutdown {
					stop()
				} else {
					eng.closeInterface("lan0")
					inventoryMaintenanceTick()
				}
				requireInventoryReport(t, requests, rpc.ReportClear)
				inventoryMaintenanceTick()
				assertNoInventoryReport(t, requests)
			})
		})
	}
}
