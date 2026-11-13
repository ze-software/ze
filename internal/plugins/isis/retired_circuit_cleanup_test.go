// Design: docs/architecture/isis/isis-4-component-config.md -- complete circuit retirement before replacement publication.
// Related: retired_circuit_callback_test.go -- raw identity and shared SDK fixture.
package isis

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestISISRetiringCircuitCleanupKeepsReplacement holds the old callback after
// map removal, at its worker join. A real SDK rebuild must not expose a new
// circuit to the old callback's name-scoped flooding and DIS cleanup.
func TestISISRetiringCircuitCleanupKeepsReplacement(t *testing.T) {
	const (
		name   = "isis-on-a"
		before = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l2","interfaces":{"interface":{"isis-on-a":{"circuit-type":"broadcast","hello-interval":"3600"}}}}}`
		after  = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"isis-on-a":{"circuit-type":"broadcast","hello-interval":"3600"}}}}}`
	)
	for _, bridged := range []bool{false, true} {
		t.Run(fmt.Sprintf("bridge=%t", bridged), func(t *testing.T) {
			useOnLinkBackend(t)
			backend := &retirementCleanupBackend{replacementOpened: make(chan struct{})}
			eng, call := startISISRuntimeRollbackPluginWithBackend(t, bridged, before, backend)
			assertISISRollbackRuntime(t, eng, before, "startup")
			oldRaw := backend.last(t)
			waitRetiredCircuitSignal(t, oldRaw.hello, "old broadcast IIH")
			eng.circuitsMu.Lock()
			oldCircuit := eng.circuitByName[name]
			oldDone := eng.circuitDone[name]
			close(eng.circuitStop[name])
			heldStop := make(chan struct{})
			heldDone := make(chan struct{})
			eng.circuitStop[name] = heldStop
			eng.circuitDone[name] = heldDone
			eng.circuitsMu.Unlock()
			linkResult := make(chan error, 1)
			applyResult := make(chan error, 1)
			var release sync.Once
			var linkStarted, linkJoined, applyStarted, applyJoined bool
			t.Cleanup(func() {
				release.Do(func() { close(heldDone) })
				if linkStarted {
					if !linkJoined {
						retirementCleanupResult(t, linkResult, "old link-down cleanup")
					}
				}
				if applyStarted {
					if !applyJoined {
						retirementCleanupResult(t, applyResult, "replacement apply cleanup")
					}
				}
			})
			// The real worker exits before the down callback. Only its completion
			// receipt is delayed; no test-created worker closes the held channel.
			waitRetiredCircuitSignal(t, oldDone, "old Hello worker exit")
			if err := call("ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: configRoot, Data: after}}}); err != nil {
				t.Fatal(err)
			}
			linkStarted = true
			go func() { linkResult <- eng.transport.HandleLinkDown(name) }()
			waitRetiredCircuitSignal(t, heldStop, "old callback stopped its circuit")
			// Taking the publication mutex after the stop receipt witnesses the
			// callback's map mutation before the replacement apply is admitted.
			eng.circuitsMu.RLock()
			stillOld := eng.circuitByName[name] == oldCircuit
			eng.circuitsMu.RUnlock()
			if stillOld {
				t.Fatal("old callback did not reach its held completion receipt")
			}
			applyStarted = true
			go func() {
				applyResult <- call("ze-plugin-callback:config-apply", rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: configRoot, Changed: after}}})
			}()
			// A correct close also joins a retirement that already removed its
			// binding, so the replacement raw open may wait for heldDone. Keep
			// the early-open schedule available to broken code, then require the
			// same positive open receipt after both operations finish below.
			select {
			case <-backend.replacementOpened:
			case <-time.After(250 * time.Millisecond):
			}
			var electionRelease sync.Once
			var electionLocked bool
			t.Cleanup(func() {
				if electionLocked {
					electionRelease.Do(eng.electMu.Unlock)
				}
			})

			// This window only gives an incorrectly unguarded publisher a chance
			// to complete. The final oracle is the retained SRM, SSN, DIS record
			// and live Hello worker after both operations have been joined.
			publishedEarly := false
			select {
			case err := <-applyResult:
				applyJoined = true
				if err != nil {
					t.Fatalf("replacement apply: %v", err)
				}
				publishedEarly = true
			case <-time.After(250 * time.Millisecond):
			}
			var state retirementCleanupState
			if publishedEarly {
				// Apply has finished setConfig, which itself acquires electMu.
				// Exclude periodic elections only while seeding/checking the
				// explicit replacement role, never before that apply receipt.
				eng.electMu.Lock()
				electionLocked = true
				state = seedRetirementReplacement(t, eng, backend, after, oldCircuit)
			}
			release.Do(func() { close(heldDone) })
			retirementCleanupResult(t, linkResult, "old link-down")
			linkJoined = true
			if !applyJoined {
				retirementCleanupResult(t, applyResult, "replacement apply")
				applyJoined = true
			}
			waitRetiredCircuitSignal(t, backend.replacementOpened, "replacement raw open")
			if !publishedEarly {
				eng.electMu.Lock()
				electionLocked = true
				state = seedRetirementReplacement(t, eng, backend, after, oldCircuit)
			}
			if !eng.lsdb.SRM(lsdb.Level1, state.lsp, state.circuit) {
				t.Error("old retirement cleared replacement SRM state")
			}
			if !eng.lsdb.SSN(lsdb.Level1, state.lsp, state.circuit) {
				t.Error("old retirement cleared replacement SSN state")
			}
			if got, present := eng.lookupElectedPseudonode(name, lsdb.Level1); !present || got != state.dis {
				t.Errorf("old retirement cleared replacement DIS state: got %v, present %t, want %v", got, present, state.dis)
			}
			eng.circuitsMu.RLock()
			stop, done := eng.circuitStop[name], eng.circuitDone[name]
			eng.circuitsMu.RUnlock()
			if stop == nil || done == nil {
				t.Fatal("replacement Hello worker lost its lifecycle channels")
			}
			select {
			case <-stop:
				t.Error("old retirement stopped replacement Hello worker")
			default:
			}
			electionRelease.Do(eng.electMu.Unlock)
			if err := eng.transport.HandleLinkDown(name); err != nil {
				t.Fatal(err)
			}
			waitRetiredCircuitSignal(t, done, "replacement worker exit on its own down")
			for _, raw := range []*retirementCleanupHandle{oldRaw, backend.last(t)} {
				if got := raw.closeCount(); got != 1 {
					t.Errorf("raw circuit closed %d times, want once", got)
				}
			}
		})
	}
}

// retirementCleanupState is an assertion snapshot, not a live engine owner.
type retirementCleanupState struct {
	lsp     types.LSPID
	circuit lsdb.CircuitID
	dis     types.SourceID
}

func seedRetirementReplacement(t *testing.T, eng *engine, backend *retirementCleanupBackend, config string, oldCircuit *circuit.Circuit) retirementCleanupState {
	t.Helper()
	const name = "isis-on-a"
	assertISISRollbackRuntime(t, eng, config, "replacement applied")
	waitRetiredCircuitSignal(t, backend.last(t).hello, "replacement initial broadcast IIH")
	eng.circuitsMu.RLock()
	replacement := eng.circuitByName[name]
	eng.circuitsMu.RUnlock()
	if replacement == nil {
		t.Fatal("replacement engine circuit absent")
	}
	if replacement == oldCircuit {
		t.Fatal("replacement reused the retired engine circuit")
	}
	if !eng.transport.CircuitOpen(name) {
		t.Fatal("replacement raw circuit is not open")
	}
	sys := types.SystemID{0, 0, 0, 0, 0, 9}
	id := types.NewLSPID(types.NewSourceID(sys, 0), 0)
	lsp := packet.LSP{PDUType: packet.PDUTypeL1LSP, LSPID: id, SequenceNumber: 1, RemainingLifetime: 1200, TypeBlock: packet.LSPFlagISTypeL1, MaxAreaAddresses: 3}
	raw := make([]byte, lsp.EncodedLen())
	lsp.WriteTo(raw, 0)
	cid := eng.circuitIDFor(name)
	if received := eng.flooder.ReceiveLSP(cid, false, &lsp, raw); !received.Stored {
		t.Fatal("replacement LSP was not stored")
	}
	eng.lsdb.SetSRM(lsdb.Level1, id, cid)
	eng.recordElectedPseudonode(name, lsdb.Level1, sys, 7)
	dis := types.NewSourceID(sys, 7)
	if !eng.lsdb.SRM(lsdb.Level1, id, cid) {
		t.Fatal("positive control: replacement SRM was not armed")
	}
	if !eng.lsdb.SSN(lsdb.Level1, id, cid) {
		t.Fatal("positive control: replacement SSN was not armed")
	}
	if got, present := eng.lookupElectedPseudonode(name, lsdb.Level1); !present || got != dis {
		t.Fatal("positive control: replacement DIS was not recorded")
	}
	return retirementCleanupState{lsp: id, circuit: cid, dis: dis}
}

func retirementCleanupResult(t *testing.T, result <-chan error, what string) {
	t.Helper()
	select {
	case err := <-result:
		if err != nil {
			t.Errorf("%s: %v", what, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("waiting for %s", what)
	}
}

type retirementCleanupBackend struct {
	mu                sync.Mutex
	opened            []*retirementCleanupHandle
	replacementOpened chan struct{}
}

func (b *retirementCleanupBackend) OpenCircuit(name string) (transport.CircuitHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	raw := &retirementCleanupHandle{retiredCircuitHandle: &retiredCircuitHandle{fakeCircuit: &fakeCircuit{name: name, recv: make(chan transport.RawFrame)}, ifindex: 1}, hello: make(chan struct{})}
	b.opened = append(b.opened, raw)
	if len(b.opened) == 2 {
		close(b.replacementOpened)
	}
	return raw, nil
}

func (b *retirementCleanupBackend) last(t *testing.T) *retirementCleanupHandle {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.opened) == 0 {
		t.Fatal("backend has not opened a raw circuit")
	}
	return b.opened[len(b.opened)-1]
}

type retirementCleanupHandle struct {
	*retiredCircuitHandle
	hello     chan struct{}
	helloOnce sync.Once
}

func (c *retirementCleanupHandle) Send(_, _ [transport.MACLen]byte, pdu []byte) error {
	if len(pdu) > offPDUType {
		kind := packet.PDUType(pdu[offPDUType] & pduTypeMask)
		if kind == packet.PDUTypeL1LANHello || kind == packet.PDUTypeL2LANHello {
			c.helloOnce.Do(func() { close(c.hello) })
		}
	}
	return nil
}
