// BGP EPE on the mpls-fib bus: a writer that opens late (fib-vpp after VPP
// connects or restarts) gets every PeerNode SID label EPE holds back through
// a targeted replay.
package epe

import (
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/ze"
)

// epeLabelWriter stands in for a data plane's label writer (fib-kernel,
// fib-vpp) on the production bus: it consumes the batches that address the
// replay token it holds, records them and acknowledges with nil.
type epeLabelWriter struct {
	mu      sync.Mutex
	held    uint64
	batches []mplsfib.EntryBatch
}

func (w *epeLabelWriter) handle(batch *mplsfib.EntryBatch) {
	if !batch.Addresses(w.held) {
		return
	}
	w.mu.Lock()
	w.batches = append(w.batches, mplsfib.EntryBatch{Entries: slices.Clone(batch.Entries), ReplayID: batch.ReplayID})
	w.mu.Unlock()
	batch.Acknowledge(nil)
}

func newEPELabelBus(t *testing.T, writers ...*epeLabelWriter) ze.EventBus {
	t.Helper()
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	for _, w := range writers {
		t.Cleanup(mplsfib.EntryChange.Subscribe(bus, w.handle))
	}
	return bus
}

// TestEPEReplayReappliesInstalledLabels proves EPE answers a writer's replay
// request with every PeerNode SID label it holds for an established,
// configured peer, tagged with the request's token, so only the asking writer
// consumes it.
//
// VALIDATES: AC-25 -- EPE re-applies each wanted label as a pop toward its
// peer with Source 5 and the token; a label whose peer is no longer
// established is not re-applied; a writer holding no token (fib-kernel) does
// not receive the replay batch; a request with the incremental token asks for
// nothing; a stopped source answers nothing.
// METHOD: two writers on the production bus, one holding a fresh token; the
// source's sessions, config and installed labels are set as publishLocked
// leaves them, then the replay handler the plugin subscribes is driven
// through a request emitted on the bus.
func TestEPEReplayReappliesInstalledLabels(t *testing.T) {
	kernel := &epeLabelWriter{}
	vpp := &epeLabelWriter{held: mplsfib.NewReplayToken()}
	bus := newEPELabelBus(t, kernel, vpp)
	source := NewSource(bus)
	t.Cleanup(mplsfib.ReplayRequest.Subscribe(bus, source.ReplayLabels))

	up, down := netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")
	source.config = Config{Base: 16000, Peers: map[netip.Addr]PeerConfig{up: {Index: 1}, down: {Index: 2}}}
	source.sessions[up] = epeSession{local: linkstateevents.NodeID{ASN: 65000}}
	// A removal that failed leaves down's label installed although its
	// session is gone: replay must not reinstall what EPE is retiring.
	source.installed[up] = 16001
	source.installed[down] = 16002
	source.acquired = true

	if _, err := mplsfib.ReplayRequest.Emit(bus, &replay.Request{ReplayID: vpp.held}); err != nil {
		t.Fatalf("emit replay request: %v", err)
	}
	if len(kernel.batches) != 0 {
		t.Fatal("a writer that did not ask received the replay batch")
	}
	if len(vpp.batches) != 1 {
		t.Fatalf("asking writer got %d batches, want one replay batch", len(vpp.batches))
	}
	answer := vpp.batches[0]
	if answer.ReplayID != vpp.held {
		t.Fatalf("replay batch token = %d, want %d", answer.ReplayID, vpp.held)
	}
	want := mplsfib.Entry{Action: mplsfib.ActionAdd, Op: mplsfib.OpPop, InLabel: 16001, NextHop: up, Source: epeMPLSSource}
	if len(answer.Entries) != 1 {
		t.Fatalf("replayed %+v, want only %+v", answer.Entries, want)
	}
	got := answer.Entries[0]
	if got.Action != want.Action || got.Op != want.Op || got.InLabel != want.InLabel ||
		got.NextHop != want.NextHop || got.Source != want.Source {
		t.Fatalf("replayed %+v, want %+v", got, want)
	}

	if _, err := mplsfib.ReplayRequest.Emit(bus, &replay.Request{}); err != nil {
		t.Fatalf("emit incremental replay request: %v", err)
	}
	if len(vpp.batches) != 1 {
		t.Fatal("a request with the incremental token must ask for nothing")
	}

	source.stopped = true
	if _, err := mplsfib.ReplayRequest.Emit(bus, &replay.Request{ReplayID: vpp.held}); err != nil {
		t.Fatalf("emit replay request after stop: %v", err)
	}
	if len(vpp.batches) != 1 {
		t.Fatal("a stopped source must re-apply nothing")
	}
}
