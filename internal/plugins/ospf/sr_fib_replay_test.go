// OSPF-SR on the mpls-fib bus: a refused entry is logged and stays wanted, and
// a writer that opens late (fib-vpp after VPP connects or restarts) gets every
// wanted entry back through a targeted replay.
package ospf

import (
	"bytes"
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/server"
	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/ze"
)

// srLabelWriter stands in for a data plane's label writer (fib-kernel,
// fib-vpp) on the production bus: it consumes the batches that address the
// replay token it holds, records them and acknowledges with refusal.
type srLabelWriter struct {
	mu      sync.Mutex
	held    uint64
	refusal error
	batches []mplsfibevents.EntryBatch
}

func (w *srLabelWriter) handle(batch *mplsfibevents.EntryBatch) {
	if !batch.Addresses(w.held) {
		return
	}
	w.mu.Lock()
	w.batches = append(w.batches, mplsfibevents.EntryBatch{Entries: slices.Clone(batch.Entries), ReplayID: batch.ReplayID})
	w.mu.Unlock()
	batch.Acknowledge(w.refusal)
}

func newSRLabelBus(t *testing.T, writers ...*srLabelWriter) ze.EventBus {
	t.Helper()
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	for _, w := range writers {
		t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, w.handle))
	}
	return bus
}

// TestSRFIBEmitLogsRefusal proves an SR label entry the writer refuses is
// logged with the entry and the cause, where the bare Emit it replaced never
// waited for an acknowledgement and so could not see a refusal at all.
//
// VALIDATES: AC-25 -- OSPF-SR uses mplsfib.Apply, logs the refused entry and its
// cause at WARN, and keeps the entry wanted for the next replay.
// METHOD: a refusing writer on the production bus; the log record must carry
// the refusal text and the in-label of the refused Adj-SID pop.
func TestSRFIBEmitLogsRefusal(t *testing.T) {
	refusal := errors.New("vpp: mpls_route_add_del retval -6")
	bus := newSRLabelBus(t, &srLabelWriter{refusal: refusal})
	f := newSRFIB(bus, mplsSourceOSPFSR)
	var logged bytes.Buffer
	f.log = slog.New(slog.NewTextHandler(&logged, nil))

	f.installAdjSID(40001, netip.MustParseAddr("10.0.0.2"))

	out := logged.String()
	for _, want := range []string{"level=WARN", refusal.Error(), "in-label=40001"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log %q lacks %q", out, want)
		}
	}
	if _, wanted := f.pops[40001]; !wanted {
		t.Fatal("a refused add must stay wanted for the next replay")
	}
}

// TestSRFIBEmitLogsMissingWriter proves an entry applied while no writer runs
// (fib-vpp has not opened yet) is reported, not dropped in silence.
//
// VALIDATES: AC-25 -- no acknowledgement is a refusal the producer logs.
// METHOD: the production bus with no label writer subscribed.
func TestSRFIBEmitLogsMissingWriter(t *testing.T) {
	bus := newSRLabelBus(t)
	f := newSRFIB(bus, mplsSourceOSPFSR)
	var logged bytes.Buffer
	f.log = slog.New(slog.NewTextHandler(&logged, nil))

	f.programSwap(16509, 16009, netip.MustParseAddr("10.0.0.2"))

	out := logged.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "in-label=16509") {
		t.Fatalf("log %q does not report the unacknowledged swap", out)
	}
	if _, wanted := f.swaps[16509]; !wanted {
		t.Fatal("an unacknowledged add must stay wanted for the next replay")
	}
}

// TestSRFIBReplayReappliesWantedEntries proves OSPF-SR answers a writer's
// replay request with every entry it wants installed, tagged with the request's
// token, so only the asking writer consumes it.
//
// VALIDATES: AC-25 -- OSPF-SR re-applies every wanted push, swap and pop with
// the token; a withdrawn entry is not re-applied; an in-label that moved from
// swap to pop is re-applied once, as the pop; a writer holding no token
// (fib-kernel) does not receive the replay batch; a request with the
// incremental token asks for nothing.
// METHOD: two writers on the production bus, one holding a fresh token; the
// producer subscribes through subscribeReplay, the path initSPF uses.
func TestSRFIBReplayReappliesWantedEntries(t *testing.T) {
	kernel := &srLabelWriter{}
	vpp := &srLabelWriter{held: mplsfibevents.NewReplayToken()}
	bus := newSRLabelBus(t, kernel, vpp)
	f := newSRFIB(bus, mplsSourceOSPFSR)
	t.Cleanup(f.subscribeReplay())

	nh := netip.MustParseAddr("10.0.0.2")
	keptFEC, goneFEC := netip.MustParsePrefix("10.0.0.9/32"), netip.MustParsePrefix("10.0.0.8/32")
	f.programPush(keptFEC, 16009, nh)
	f.programPush(goneFEC, 16008, nh)
	f.removePush(goneFEC)
	f.programSwap(16510, 16010, nh)
	f.programSwap(16509, 16009, nh)
	f.programPop(16509, nh) // the same SID label, now penultimate hop
	f.installAdjSID(40001, nh)
	f.installAdjSID(40002, nh)
	f.withdrawAdjSID(40002)
	kernelBefore, vppBefore := len(kernel.batches), len(vpp.batches)

	if _, err := mplsfibevents.ReplayRequest.Emit(bus, &replay.Request{ReplayID: vpp.held}); err != nil {
		t.Fatalf("emit replay request: %v", err)
	}

	if len(vpp.batches) != vppBefore+1 {
		t.Fatalf("asking writer got %d batches, want one replay batch", len(vpp.batches)-vppBefore)
	}
	if len(kernel.batches) != kernelBefore {
		t.Fatal("a writer that did not ask received the replay batch")
	}
	answer := vpp.batches[len(vpp.batches)-1]
	if answer.ReplayID != vpp.held {
		t.Fatalf("replay batch token = %d, want %d", answer.ReplayID, vpp.held)
	}
	type key struct {
		op      mplsfibevents.Op
		inLabel uint32
		fec     netip.Prefix
	}
	got := map[key]mplsfibevents.Entry{}
	for _, e := range answer.Entries {
		if e.Action != mplsfibevents.ActionAdd || e.Source != mplsSourceOSPFSR {
			t.Fatalf("replayed entry %+v: want an add with source %d", e, mplsSourceOSPFSR)
		}
		got[key{e.Op, e.InLabel, e.FEC}] = e
	}
	want := []key{
		{mplsfibevents.OpPush, 0, keptFEC},
		{mplsfibevents.OpSwap, 16510, netip.Prefix{}},
		{mplsfibevents.OpPop, 16509, netip.Prefix{}},
		{mplsfibevents.OpPop, 40001, netip.Prefix{}},
	}
	if len(answer.Entries) != len(want) {
		t.Fatalf("replayed %d entries (%+v), want %d", len(answer.Entries), answer.Entries, len(want))
	}
	for _, k := range want {
		e, ok := got[k]
		if !ok {
			t.Fatalf("replay lacks %+v (got %+v)", k, answer.Entries)
		}
		if e.NextHop != nh {
			t.Fatalf("replayed %+v next hop = %v, want %v", k, e.NextHop, nh)
		}
	}
	if e := got[key{mplsfibevents.OpPush, 0, keptFEC}]; !slices.Equal(e.OutLabels, []uint32{16009}) {
		t.Fatalf("replayed push out-labels = %v, want [16009]", e.OutLabels)
	}

	if _, err := mplsfibevents.ReplayRequest.Emit(bus, &replay.Request{}); err != nil {
		t.Fatalf("emit incremental replay request: %v", err)
	}
	if len(vpp.batches) != vppBefore+1 {
		t.Fatal("a request with the incremental token must ask for nothing")
	}
}
