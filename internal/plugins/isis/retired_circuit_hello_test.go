// Design: docs/architecture/isis/isis-4-component-config.md -- join the old Hello worker before opening replacement transport.
// Related: retired_circuit_cleanup_test.go -- complete engine-state retirement.
package isis

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestISISRetiringHelloCannotReachReplacementRaw pauses the actual old worker
// immediately before its transport lookup. A level-change apply must join that
// worker before opening a raw handle that could send its obsolete L2 IIH.
func TestISISRetiringHelloCannotReachReplacementRaw(t *testing.T) {
	const (
		name   = "isis-on-a"
		before = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"isis-on-a":{"circuit-type":"broadcast","hello-interval":"3600"}}}}}`
		after  = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"isis-on-a":{"circuit-type":"broadcast","hello-interval":"3600"}}}}}`
	)
	for _, bridged := range []bool{false, true} {
		t.Run(fmt.Sprintf("bridge=%t", bridged), func(t *testing.T) {
			useOnLinkBackend(t)
			backend := &retirementHelloBackend{replacementSnapshot: make(chan struct{})}
			eng, call := startISISRuntimeRollbackPluginWithBackend(t, bridged, before, backend)
			assertISISRollbackRuntime(t, eng, before, "startup")
			oldRaw := backend.last(t)
			waitRetiredCircuitSignal(t, oldRaw.l2Hello, "initial L2 IIH")
			if err := call("ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: configRoot, Data: after}}}); err != nil {
				t.Fatal(err)
			}
			eng.circuitsMu.RLock()
			oldCircuit := eng.circuitByName[name]
			oldStop, oldDone := eng.circuitStop[name], eng.circuitDone[name]
			reload := eng.circuitReload[name]
			eng.circuitsMu.RUnlock()
			if oldCircuit == nil {
				t.Fatal("startup has no engine circuit")
			}
			if oldStop == nil || oldDone == nil || reload == nil {
				t.Fatal("startup has no Hello worker lifecycle")
			}
			signEntered, signRelease := make(chan struct{}), make(chan struct{})
			var entered, release sync.Once
			linkResult, applyResult := make(chan error, 1), make(chan error, 1)
			var linkStarted, linkJoined, applyStarted, applyJoined bool
			t.Cleanup(func() {
				release.Do(func() { close(signRelease) })
				if linkStarted {
					if !linkJoined {
						retirementCleanupResult(t, linkResult, "retired Hello link-down cleanup")
					}
				}
				if applyStarted {
					if !applyJoined {
						retirementCleanupResult(t, applyResult, "retired Hello apply cleanup")
					}
				}
			})
			oldCircuit.SetSigner(func(_ adjacency.Level, pdu []byte) []byte {
				entered.Do(func() { close(signEntered) })
				<-signRelease
				return pdu
			}, nil)
			// Wake the actual worker without waiting for a periodic Hello. An
			// already queued wake reads the same circuit signer when consumed.
			select {
			case reload <- struct{}{}:
			default:
			}
			waitRetiredCircuitSignal(t, signEntered, "old worker entered signer before transport send")
			linkStarted = true
			go func() { linkResult <- eng.transport.HandleLinkDown(name) }()
			waitRetiredCircuitSignal(t, oldStop, "link-down stopped old worker")
			eng.circuitsMu.RLock()
			stillOld := eng.circuitByName[name] == oldCircuit
			eng.circuitsMu.RUnlock()
			if stillOld {
				t.Fatal("link-down did not enter its held worker join")
			}
			applyStarted = true
			go func() {
				applyResult <- call("ze-plugin-callback:config-apply", rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: configRoot, Changed: after}}})
			}()
			// CircuitInfo reads MTU only after the raw handle is published. A
			// broken close can reach that witness while the old signer is held;
			// a correct close instead waits for oldDone. The timeout only gives
			// the former schedule an opportunity; captured PDU types decide it.
			select {
			case <-backend.replacementSnapshot:
			case <-time.After(250 * time.Millisecond):
			}
			release.Do(func() { close(signRelease) })
			waitRetiredCircuitSignal(t, oldDone, "retired actual Hello worker exit")
			retirementCleanupResult(t, linkResult, "retired Hello link-down")
			linkJoined = true
			retirementCleanupResult(t, applyResult, "replacement level apply")
			applyJoined = true
			assertISISRollbackRuntime(t, eng, after, "replacement active")
			replacement := backend.last(t)
			if replacement == oldRaw {
				t.Fatal("candidate did not open a replacement raw circuit")
			}
			waitRetiredCircuitSignal(t, replacement.l1Hello, "replacement initial L1 IIH")
			if got := replacement.l2Count(); got != 0 {
				t.Errorf("replacement raw circuit sent %d stale L2 IIHs from the retired worker; want zero", got)
			}
			if !eng.transport.CircuitOpen(name) {
				t.Error("replacement raw circuit is not open")
			}
			eng.circuitsMu.RLock()
			current, stop, done := eng.circuitByName[name], eng.circuitStop[name], eng.circuitDone[name]
			eng.circuitsMu.RUnlock()
			if current == nil || current == oldCircuit {
				t.Fatal("replacement has no distinct engine circuit")
			}
			if stop == nil || done == nil {
				t.Fatal("replacement has no Hello worker lifecycle")
			}
			select {
			case <-stop:
				t.Error("retirement stopped replacement Hello worker")
			default:
			}
			if got := replacement.closeCount(); got != 0 {
				t.Errorf("replacement closed %d times before its own down", got)
			}
			if err := eng.transport.HandleLinkDown(name); err != nil {
				t.Fatal(err)
			}
			waitRetiredCircuitSignal(t, done, "replacement worker exit after its own down")
			for _, raw := range []*retirementHelloHandle{oldRaw, replacement} {
				if got := raw.closeCount(); got != 1 {
					t.Errorf("raw circuit closed %d times, want once", got)
				}
			}
		})
	}
}

type retirementHelloBackend struct {
	mu                  sync.Mutex
	opened              []*retirementHelloHandle
	replacementSnapshot chan struct{}
}

func (b *retirementHelloBackend) OpenCircuit(name string) (transport.CircuitHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	raw := &retirementHelloHandle{retiredCircuitHandle: &retiredCircuitHandle{fakeCircuit: &fakeCircuit{name: name, recv: make(chan transport.RawFrame)}, ifindex: 1}, l1Hello: make(chan struct{}), l2Hello: make(chan struct{})}
	if len(b.opened) == 1 {
		raw.snapshot = b.replacementSnapshot
	}
	b.opened = append(b.opened, raw)
	return raw, nil
}

func (b *retirementHelloBackend) last(t *testing.T) *retirementHelloHandle {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.opened) == 0 {
		t.Fatal("raw backend has not opened a circuit")
	}
	return b.opened[len(b.opened)-1]
}

type retirementHelloHandle struct {
	*retiredCircuitHandle
	l1Hello chan struct{}
	l2Hello chan struct{}
	l1Once  sync.Once
	l2Once  sync.Once
	mu      sync.Mutex
	l2Sent  int
}

func (c *retirementHelloHandle) Send(_, _ [transport.MACLen]byte, pdu []byte) error {
	if len(pdu) <= offPDUType {
		return nil
	}
	kind := packet.PDUType(pdu[offPDUType] & pduTypeMask)
	if kind == packet.PDUTypeL1LANHello {
		c.l1Once.Do(func() { close(c.l1Hello) })
	}
	if kind == packet.PDUTypeL2LANHello {
		c.mu.Lock()
		c.l2Sent++
		c.mu.Unlock()
		c.l2Once.Do(func() { close(c.l2Hello) })
	}
	return nil
}

func (c *retirementHelloHandle) l2Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.l2Sent
}
