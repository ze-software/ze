// Design: docs/architecture/isis/isis-4-component-config.md -- raw-circuit callback ownership.
// Related: runtime_config_rollback_test.go -- the real SDK lifecycle fixture.
package isis

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A retired raw circuit's delayed down callback must leave the replacement
// circuit alive, both when its physical ifindex is reused and when it changes.
// Holding the first Close spans a real SDK level-change apply and its first IIH.
func TestISISRetiredCircuitDownKeepsReplacement(t *testing.T) {
	const (
		name   = "isis-on-a"
		before = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"isis-on-a":{"circuit-type":"point-to-point","hello-interval":"3600"}}}}}`
		after  = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"isis-on-a":{"circuit-type":"point-to-point","hello-interval":"3600"}}}}}`
	)
	for _, bridged := range []bool{false, true} {
		for _, replacementIndex := range []int{1, 2} {
			t.Run(fmt.Sprintf("bridge=%t/ifindex=%d", bridged, replacementIndex), func(t *testing.T) {
				useOnLinkBackend(t)
				backend := &retiredCircuitBackend{closeEntered: make(chan struct{}), closeRelease: make(chan struct{}), replacementIndex: replacementIndex}
				eng, call := startISISRuntimeRollbackPluginWithBackend(t, bridged, before, backend)
				linkDone := make(chan error, 1)
				var release sync.Once
				var linkStarted bool
				var linkJoined bool
				t.Cleanup(func() {
					release.Do(func() { close(backend.closeRelease) })
					if linkStarted {
						if !linkJoined {
							select {
							case err := <-linkDone:
								if err != nil {
									t.Errorf("join old link-down: %v", err)
								}
							case <-time.After(5 * time.Second):
								t.Error("old link-down did not finish after releasing Close")
							}
						}
					}
				})
				assertISISRollbackRuntime(t, eng, before, "startup")
				oldRaw := backend.last(t)
				waitRetiredCircuitSignal(t, oldRaw.hello, "initial circuit IIH")
				eng.circuitsMu.RLock()
				oldCircuit := eng.circuitByName[name]
				oldDone := eng.circuitDone[name]
				eng.circuitsMu.RUnlock()

				linkStarted = true
				go func() { linkDone <- eng.transport.HandleLinkDown(name) }()
				waitRetiredCircuitSignal(t, backend.closeEntered, "old raw Close entered")
				if err := call("ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: configRoot, Data: after}}}); err != nil {
					t.Fatal(err)
				}
				if err := call("ze-plugin-callback:config-apply", rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: configRoot, Changed: after}}}); err != nil {
					t.Fatal(err)
				}
				assertISISRollbackRuntime(t, eng, after, "replacement applied before old Close returns")
				replacementRaw := backend.last(t)
				if replacementRaw == oldRaw {
					t.Fatal("level change did not open a replacement raw circuit")
				}
				if got := oldRaw.IfIndex(); got != 1 {
					t.Fatalf("original physical ifindex = %d, want 1", got)
				}
				if got := replacementRaw.IfIndex(); got != replacementIndex {
					t.Fatalf("replacement physical ifindex = %d, want %d", got, replacementIndex)
				}
				waitRetiredCircuitSignal(t, replacementRaw.hello, "replacement initial IIH")
				waitRetiredCircuitSignal(t, oldDone, "retired Hello worker exit")
				eng.circuitsMu.RLock()
				replacement := eng.circuitByName[name]
				stop := eng.circuitStop[name]
				done := eng.circuitDone[name]
				eng.circuitsMu.RUnlock()
				if replacement == nil {
					t.Fatal("replacement has no live engine circuit")
				}
				if replacement == oldCircuit {
					t.Fatal("level change reused the retired engine circuit")
				}
				if stop == nil {
					t.Fatal("replacement has no Hello worker stop channel")
				}
				if done == nil {
					t.Fatal("replacement has no Hello worker completion channel")
				}

				release.Do(func() { close(backend.closeRelease) })
				select {
				case err := <-linkDone:
					linkJoined = true
					if err != nil {
						t.Fatalf("old link-down: %v", err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("old link-down callback did not finish")
				}
				assertISISRollbackRuntime(t, eng, after, "after retired raw callback")
				eng.circuitsMu.RLock()
				current := eng.circuits[replacement.IfIndex()].circuit
				currentStop := eng.circuitStop[name]
				currentDone := eng.circuitDone[name]
				eng.circuitsMu.RUnlock()
				if current != replacement {
					t.Errorf("retired raw callback removed replacement engine circuit: got %p, want %p", current, replacement)
				}
				if currentStop != stop {
					t.Error("retired raw callback removed replacement Hello stop channel")
				}
				if currentDone != done {
					t.Error("retired raw callback removed replacement Hello completion channel")
				}
				select {
				case <-stop:
					t.Error("retired raw callback stopped replacement Hello worker")
				default:
				}
				if !eng.transport.CircuitOpen(name) {
					t.Error("replacement raw circuit is no longer open")
				}
				if got := replacementRaw.closeCount(); got != 0 {
					t.Errorf("replacement raw circuit closed %d times before its own down", got)
				}
				// The current circuit's own callback must still stop and join it.
				if err := eng.transport.HandleLinkDown(name); err != nil {
					t.Fatal(err)
				}
				waitRetiredCircuitSignal(t, done, "replacement Hello worker exit after its own down")
				for _, raw := range []*retiredCircuitHandle{oldRaw, replacementRaw} {
					if got := raw.closeCount(); got != 1 {
						t.Errorf("raw circuit %p closed %d times, want exactly once", raw, got)
					}
				}
			})
		}
	}
}

// A circuit retired after the metadata snapshot must not be published by a
// builder that was waiting for the active config. The engine lock is the
// barrier between CircuitInfo and publication; the raw MTU read witnesses it.
func TestISISRetiredCircuitSnapshotCannotPublish(t *testing.T) {
	const name = "isis-on-a"
	useOnLinkBackend(t)
	backend := &retiredCircuitBackend{snapshot: make(chan struct{})}
	tr := transport.New(backend)
	eng := newEngine(tr)
	t.Cleanup(eng.shutdown)
	cfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"isis-on-a":{"circuit-type":"point-to-point","hello-interval":"3600"}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	eng.setConfig(cfg)
	ic := cfg.EnabledCircuits()[0]
	tr.EnableInterface(name, ic.Level.TransportLevel())
	if err := tr.HandleLinkUp(name); err != nil {
		t.Fatal(err)
	}
	if !tr.CircuitOpen(name) {
		t.Fatal("positive control: raw circuit was not opened")
	}
	eng.mu.Lock()
	var unlock sync.Once
	launched := make(chan struct{})
	t.Cleanup(func() {
		unlock.Do(eng.mu.Unlock)
		waitRetiredCircuitSignal(t, launched, "circuit launch cleanup")
	})
	go func() {
		defer close(launched)
		eng.launchCircuitGoroutine(ic)
	}()
	waitRetiredCircuitSignal(t, backend.snapshot, "raw circuit metadata snapshot")
	if err := tr.HandleLinkDown(name); err != nil {
		t.Fatal(err)
	}
	unlock.Do(eng.mu.Unlock)
	waitRetiredCircuitSignal(t, launched, "circuit launch returned after retirement")
	eng.circuitsMu.RLock()
	defer eng.circuitsMu.RUnlock()
	if got := len(eng.circuits); got != 0 {
		t.Errorf("retired snapshot published %d engine circuits", got)
	}
	if got := eng.circuitByName[name]; got != nil {
		t.Errorf("retired snapshot published engine circuit %p by name", got)
	}
	if got := len(eng.circuitStop); got != 0 {
		t.Errorf("retired snapshot published %d Hello worker stop channels", got)
	}
	if got := len(eng.circuitDone); got != 0 {
		t.Errorf("retired snapshot published %d Hello worker completion channels", got)
	}
}

// retiredCircuitBackend retains the fake raw lifecycle with a chosen replacement
// ifindex and holds only the first Close until the test releases its barrier.
type retiredCircuitBackend struct {
	mu               sync.Mutex
	opened           []*retiredCircuitHandle
	closeEntered     chan struct{}
	closeRelease     chan struct{}
	snapshot         chan struct{}
	replacementIndex int
}

func (b *retiredCircuitBackend) OpenCircuit(name string) (transport.CircuitHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := &retiredCircuitHandle{fakeCircuit: &fakeCircuit{name: name, recv: make(chan transport.RawFrame)}, hello: make(chan struct{})}
	c.snapshot = b.snapshot
	c.ifindex = 1
	if len(b.opened) > 0 {
		c.ifindex = b.replacementIndex
	}
	if len(b.opened) == 0 {
		c.closeEntered, c.closeRelease = b.closeEntered, b.closeRelease
	}
	b.opened = append(b.opened, c)
	return c, nil
}

func (b *retiredCircuitBackend) last(t *testing.T) *retiredCircuitHandle {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.opened) == 0 {
		t.Fatal("raw backend did not open a circuit")
	}
	return b.opened[len(b.opened)-1]
}

type retiredCircuitHandle struct {
	*fakeCircuit
	hello        chan struct{}
	helloOnce    sync.Once
	closeEntered chan struct{}
	closeRelease <-chan struct{}
	mu           sync.Mutex
	closes       int
	snapshot     chan struct{}
	snapshotOnce sync.Once
	ifindex      int
}

func (c *retiredCircuitHandle) IfIndex() int { return c.ifindex }

func (c *retiredCircuitHandle) MTU() int {
	if c.snapshot != nil {
		c.snapshotOnce.Do(func() { close(c.snapshot) })
	}
	return c.fakeCircuit.MTU()
}

func (c *retiredCircuitHandle) Send(_, _ [transport.MACLen]byte, pdu []byte) error {
	if len(pdu) > offPDUType {
		if packet.PDUType(pdu[offPDUType]&pduTypeMask) == packet.PDUTypeP2PHello {
			c.helloOnce.Do(func() { close(c.hello) })
		}
	}
	return nil
}

func (c *retiredCircuitHandle) Close() error {
	c.mu.Lock()
	c.closes++
	c.mu.Unlock()
	if c.closeRelease != nil {
		close(c.closeEntered)
		<-c.closeRelease
	}
	return c.fakeCircuit.Close()
}

func (c *retiredCircuitHandle) closeCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closes
}

func waitRetiredCircuitSignal(t *testing.T, signal <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("waiting for %s", what)
	}
}
