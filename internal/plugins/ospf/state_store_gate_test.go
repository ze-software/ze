// Design: docs/architecture/api/process-protocol.md -- OSPF state starts after the handshake.
package ospf

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// Construction and configuration cannot issue runtime RPCs. Initialization must
// happen once before packets, and a new engine must obtain a new durable counter.
func TestOSPFStateInitializationAfterConfigure(t *testing.T) {
	installDaemonState(t)
	store := daemonStateClient{}
	e := newEngine(nil)
	defer e.shutdown()
	e.state = store
	e.setConfig(ospfConfig{})
	if _, found, err := store.StateGet(context.Background(), zefs.KeyOSPFAuthBootCount.Key()); err != nil || found {
		t.Fatalf("construction/configure reached runtime state: found=%v error=%v", found, err)
	}
	if err := e.initializeState(); err != nil {
		t.Fatal(err)
	}
	assertEngineBootSequence(t, e, 1)
	if err := e.initializeState(); err != nil {
		t.Fatal(err)
	}
	assertEngineBootSequence(t, e, 1)
	next := newEngine(nil)
	defer next.shutdown()
	next.state = store
	if err := next.initializeState(); err != nil {
		t.Fatal(err)
	}
	assertEngineBootSequence(t, next, 2)
}

type failedStateClient struct {
	*fakeGRStore
	err error
}

func (s failedStateClient) StateIncrement(context.Context, string) (uint32, error) {
	return 0, s.err
}

func (s failedStateClient) StatePut(context.Context, string, []byte) error {
	return s.err
}

// No unacknowledged persistence outcome can enable packet processing or claim a
// planned graceful restart. Retrying initialization must not bypass its failure.
func TestOSPFStateFailureRefusesStartupAndPrepare(t *testing.T) {
	for _, status := range []rpc.StateStatus{rpc.StateUnavailable, rpc.StateCorrupt, rpc.StatePersistFailed} {
		t.Run(string(status), func(t *testing.T) {
			e := grEnableEngine(t, false, time.Now())
			defer e.shutdown()
			e.state = failedStateClient{fakeGRStore: newFakeGRStore(), err: &rpc.StateError{Status: status, Message: "injected failure"}}
			for range 2 {
				if err := e.openInterfaces(); err == nil {
					t.Fatal("failed durable state allowed packet startup")
				}
			}
			if got := e.grPrepare(); got.Prepared {
				t.Fatal("unacknowledged restart fact reported prepared")
			}
			if e.gr.suppressInstall() {
				t.Fatal("failed prepare retained the FIB")
			}
		})
	}
}

// A malformed but CRC-valid restart record must not look like an absent record.
func TestRestartFactCorruptionIsExplicit(t *testing.T) {
	store := newFakeGRStore()
	key := zefs.KeyOSPFGRFact.Key("v4")
	if err := store.StatePut(context.Background(), key, []byte("{")); err != nil {
		t.Fatal(err)
	}
	_, found, err := readRestartFact(context.Background(), store, key)
	var stateErr *rpc.StateError
	if !errors.As(err, &stateErr) || stateErr.Status != rpc.StateCorrupt || found {
		t.Fatalf("corrupt fact: found=%v error=%v", found, err)
	}
}

// A newly configured instance must initialize state during apply, after the
// handshake, with its own instance key and before its interfaces can run.
func TestOSPFReloadCreatedInstanceInitializesState(t *testing.T) {
	installDaemonState(t)
	store := daemonStateClient{}
	mgr := newTestInstanceManager()
	defer mgr.shutdownAll()
	mgr.engines[0].state = store
	build := mgr.build
	mgr.build = func(id uint8) *engine {
		e := build(id)
		e.state = store
		return e
	}
	base, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	mgr.setConfig(base)
	if err := mgr.start(base); err != nil {
		t.Fatal(err)
	}
	next, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth1":{"area":"0","instance-id":"5","enabled":false}}}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	fact := restartFact{Restarting: true, GraceEndUnix: time.Now().Add(time.Hour).Unix(), Expected: []string{"10.0.0.2"}}
	if err := writeRestartFact(context.Background(), store, zefs.KeyOSPFGRFact.Key("v4-5"), fact); err != nil {
		t.Fatal(err)
	}
	if err := mgr.reconcile(next); err != nil {
		t.Fatal(err)
	}
	e, found := mgr.engineFor(5)
	if !found {
		t.Fatal("reload did not create instance 5")
	}
	assertEngineBootSequence(t, e, 2)
	if !e.gr.suppressInstall() {
		t.Fatal("reload-created instance did not resume its own durable restart fact")
	}
}

func TestOSPFReloadCreatedAFInitializesState(t *testing.T) {
	installDaemonState(t)
	store := daemonStateClient{}
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	m := newV6EngineSet()
	m.state = store
	defer m.shutdownAll()
	first := []v6AFConfig{{af: afIPv6Unicast, cfg: cfg}}
	m.configure(first, false)
	if _, found, err := store.StateGet(context.Background(), zefs.KeyOSPFAuthBootCount.Key()); err != nil || found {
		t.Fatalf("configure accessed runtime state: found=%v error=%v", found, err)
	}
	if err := m.start(first, false); err != nil {
		t.Fatal(err)
	}
	both := []v6AFConfig{first[0], {af: afIPv4Unicast, cfg: cfg}}
	if err := m.apply(both, true); err != nil {
		t.Fatal(err)
	}
	e := m.engines[afIPv4Unicast]
	if e == nil {
		t.Fatal("reload did not create IPv4-over-OSPFv3 engine")
	}
	assertEngineBootSequence(t, e, 2)
}

func assertEngineBootSequence(t *testing.T, e *engine, want uint32) {
	t.Helper()
	e.auth.configure(ospfConfig{
		KeyChains:  []keyChainConfig{{Name: "state-test", ExtendedSequence: true, Keys: []keyConfig{{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "state-test-secret"}}}},
		Areas:      []areaConfig{{AuthKeyChain: "state-test"}},
		Interfaces: []interfaceConfig{{Name: "state-test-iface", Authentication: authConfig{Mode: "inherit"}}},
	})
	_, _, sequence, _, ok := e.auth.signKey("state-test-iface")
	if !ok {
		t.Fatal("engine cannot authenticate its first packet")
	}
	if got := uint32(sequence >> 32); got != want {
		t.Fatalf("packet boot sequence = %d, want %d", got, want)
	}
}
