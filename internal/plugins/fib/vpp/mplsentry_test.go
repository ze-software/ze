// Design: docs/architecture/mpls/mpls-kernel.md -- label-switching entries on the VPP data plane, tests
package fibvpp

import (
	"errors"
	"net/netip"
	"slices"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/mpls"

	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
)

// syncBus is an in-process EventBus that delivers each Emit synchronously to
// every subscriber, as the daemon's bus does, so mplsfib.Apply sees the
// acknowledgement before it returns.
type syncBus struct {
	mu   sync.Mutex
	subs map[string][]func(any)
}

func (b *syncBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	handlers := slices.Clone(b.subs[namespace+"/"+eventType])
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (b *syncBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs == nil {
		b.subs = make(map[string][]func(any))
	}
	key := namespace + "/" + eventType
	b.subs[key] = append(b.subs[key], handler)
	index := len(b.subs[key]) - 1
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.subs[key][index] = func(any) {}
	}
}

// labelWriter is a fib-vpp writer over the recording fake channel, published
// through the real (mpls-fib, entry) subscription.
type labelWriter struct {
	ch  *testChannel
	bus *syncBus
	fib *fibVPP
	mu  sync.Mutex
}

func newLabelWriter(t *testing.T, ipTable uint32) *labelWriter {
	t.Helper()
	ch := &testChannel{}
	f := newFibVPP(newGovppBackend(ch, ipTable))
	f.mplsBackend = newGovppMPLSBackend(ch, ipTable)
	f.ipTableID = ipTable
	f.labelReplayToken = mplsfib.NewReplayToken()
	w := &labelWriter{ch: ch, bus: &syncBus{}, fib: f}
	unsub := subscribeFibVPPLabels(w.bus, &w.mu, &w.fib, func() error { return nil })
	t.Cleanup(unsub)
	return w
}

func (w *labelWriter) apply(entries ...mplsfib.Entry) error {
	return mplsfib.Apply(w.bus, entries)
}

func (w *labelWriter) routes() []*mpls.MplsRouteAddDel {
	var out []*mpls.MplsRouteAddDel
	for _, m := range w.ch.requests {
		if req, ok := m.(*mpls.MplsRouteAddDel); ok {
			out = append(out, req)
		}
	}
	return out
}

func swapEntry(in uint32, out []uint32) mplsfib.Entry {
	return mplsfib.Entry{Action: mplsfib.ActionAdd, Op: mplsfib.OpSwap, InLabel: in, OutLabels: out,
		NextHop: netip.MustParseAddr("192.0.2.1"), Source: 1}
}

func pathLabels(p fib_types.FibPath) []uint32 {
	out := make([]uint32, 0, p.NLabels)
	for i := range int(p.NLabels) {
		out = append(out, p.LabelStack[i].Label)
	}
	return out
}

// VALIDATES: AC-17 -- a one-label swap and a two-label facility-backup stack
// each reach VPP as two routes in MPLS table 0 for the in-label, S=1 with the
// next hop's family as payload and S=0 with MPLS, each imposing the whole stack
// in order. Method: mplsfib.Apply through the subscription onto the fake channel.
// RFC requirement: RFC3032-2.1-1 positive -- the S=1 and S=0 MplsRouteAddDel requests fib-vpp sends for one swap entry carry the entry's whole out stack in order.
func TestFibVPPLabelSwapProgramsBothStackPositions(t *testing.T) {
	for _, stack := range [][]uint32{{200}, {300, 301}} {
		w := newLabelWriter(t, 0)
		require.NoError(t, w.apply(swapEntry(100, stack)))
		reqs := w.routes()
		require.Len(t, reqs, 2)
		for i, eos := range []uint8{1, 0} {
			r := reqs[i].MrRoute
			assert.True(t, reqs[i].MrIsAdd)
			assert.Equal(t, uint32(100), r.MrLabel)
			assert.Equal(t, uint32(0), r.MrTableID)
			assert.Equal(t, eos, r.MrEos)
			require.Len(t, r.MrPaths, 1)
			assert.Equal(t, stack, pathLabels(r.MrPaths[0]))
			assert.Equal(t, fib_types.FIB_API_PATH_NH_PROTO_IP4, r.MrPaths[0].Proto)
			assert.Equal(t, noInterface, r.MrPaths[0].SwIfIndex)
		}
		assert.Equal(t, uint8(fib_types.FIB_API_PATH_NH_PROTO_IP4), reqs[0].MrRoute.MrEosProto)
	}
}

// VALIDATES: AC-18 -- a penultimate-hop pop programs both S routes whose path
// via the next hop carries the single label Implicit NULL in uniform mode.
// Method: as above.
func TestFibVPPLabelPenultimatePop(t *testing.T) {
	w := newLabelWriter(t, 0)
	require.NoError(t, w.apply(mplsfib.Entry{Action: mplsfib.ActionAdd, Op: mplsfib.OpPop, InLabel: 100,
		NextHop: netip.MustParseAddr("2001:db8::1"), Source: 1}))
	reqs := w.routes()
	require.Len(t, reqs, 2)
	for _, req := range reqs {
		p := req.MrRoute.MrPaths[0]
		assert.Equal(t, []uint32{implicitNullLabel}, pathLabels(p))
		assert.Equal(t, uint8(1), p.LabelStack[0].IsUniform)
		assert.Equal(t, fib_types.FIB_API_PATH_NH_PROTO_IP6, p.Proto)
	}
}

// VALIDATES: AC-19 -- an egress pop looks the payload up in IP table
// fib.vpp.table-id of the FEC's family at S=1, and in MPLS table 0 at S=0; one
// with neither next hop nor FEC is refused naming RFC 3032 Section 2.2.
// Method: as above, for an IPv4 and an IPv6 FEC.
// RFC requirement: RFC3032-2.2-1 positive -- the S=1 route fib-vpp sends for an egress pop names the FEC's family as its payload protocol, and an egress pop naming no family is refused.
func TestFibVPPLabelEgressPopLooksUp(t *testing.T) {
	for _, tc := range []struct {
		fec   string
		proto fib_types.FibPathNhProto
	}{{"192.0.2.9/32", fib_types.FIB_API_PATH_NH_PROTO_IP4}, {"2001:db8::9/128", fib_types.FIB_API_PATH_NH_PROTO_IP6}} {
		w := newLabelWriter(t, 5)
		require.NoError(t, w.apply(mplsfib.Entry{Action: mplsfib.ActionAdd, Op: mplsfib.OpPop, InLabel: 16,
			FEC: netip.MustParsePrefix(tc.fec), Source: 1}))
		reqs := w.routes()
		require.Len(t, reqs, 2)
		eos := reqs[0].MrRoute
		assert.Equal(t, uint8(tc.proto), eos.MrEosProto)
		assert.Equal(t, tc.proto, eos.MrPaths[0].Proto)
		assert.Equal(t, uint32(5), eos.MrPaths[0].TableID)
		assert.Equal(t, noInterface, eos.MrPaths[0].SwIfIndex)
		neos := reqs[1].MrRoute
		assert.Equal(t, fib_types.FIB_API_PATH_NH_PROTO_MPLS, neos.MrPaths[0].Proto)
		assert.Equal(t, uint32(0), neos.MrPaths[0].TableID)
	}

	w := newLabelWriter(t, 5)
	err := w.apply(mplsfib.Entry{Action: mplsfib.ActionAdd, Op: mplsfib.OpPop, InLabel: 17, Source: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RFC 3032 Section 2.2")
	assert.Empty(t, w.routes())
	assert.NotContains(t, w.fib.labelOwners, uint32(17))
}

// VALIDATES: AC-20 -- table-id 5 still programs MPLS table 0, and
// MplsTableAddDel table 0 precedes the first label route, once per writer.
// Method: two swaps through the subscription; the request order is read back.
func TestFibVPPLabelUsesMPLSTableZero(t *testing.T) {
	w := newLabelWriter(t, 5)
	require.NoError(t, w.apply(swapEntry(100, []uint32{200})))
	require.NoError(t, w.apply(swapEntry(101, []uint32{201})))
	tables := 0
	for i, m := range w.ch.requests {
		if req, ok := m.(*mpls.MplsTableAddDel); ok {
			tables++
			assert.Equal(t, 0, i, "table 0 is created before any route")
			assert.True(t, req.MtIsAdd)
			assert.Equal(t, uint32(0), req.MtTable.MtTableID)
		}
	}
	assert.Equal(t, 1, tables)
	for _, req := range w.routes() {
		assert.Equal(t, uint32(0), req.MrRoute.MrTableID)
	}
}

// VALIDATES: AC-21 -- swap, pop and labeled push labels carry IsUniform 1 and
// no fixed TTL. Method: one entry of each through the subscription, and a
// labeled best-change push through addMPLSRoute.
// RFC requirement: RFC3032-2.4.3-1 positive -- the label fib-vpp imposes on a labeled push is sent in uniform mode with no fixed TTL, so VPP copies the IP TTL.
func TestFibVPPLabelTTLUniform(t *testing.T) {
	w := newLabelWriter(t, 0)
	require.NoError(t, w.apply(swapEntry(100, []uint32{200, 201})))
	require.NoError(t, w.apply(mplsfib.Entry{Action: mplsfib.ActionAdd, Op: mplsfib.OpPop, InLabel: 101,
		NextHop: netip.MustParseAddr("192.0.2.1"), Source: 1}))
	require.NoError(t, w.apply(mplsfib.Entry{Action: mplsfib.ActionAdd, Op: mplsfib.OpPush,
		FEC: netip.MustParsePrefix("198.51.100.0/24"), OutLabels: []uint32{300},
		NextHop: netip.MustParseAddr("192.0.2.1"), Source: 1}))
	var paths []fib_types.FibPath
	for _, m := range w.ch.requests {
		switch req := m.(type) {
		case *mpls.MplsRouteAddDel:
			paths = append(paths, req.MrRoute.MrPaths...)
		case *ip.IPRouteAddDel:
			paths = append(paths, req.Route.Paths...)
		}
	}
	require.Len(t, paths, 5)
	for _, p := range paths {
		require.NotZero(t, p.NLabels)
		for i := range int(p.NLabels) {
			assert.Equal(t, uint8(1), p.LabelStack[i].IsUniform)
			assert.Equal(t, uint8(0), p.LabelStack[i].TTL)
		}
	}
}

// VALIDATES: AC-22 -- a nonzero Retval and a transport error reach the producer
// through Apply, and no in-label is recorded as owned. Method: the fake channel
// answers -6, then fails the transport.
func TestFibVPPLabelAckVPPRefusal(t *testing.T) {
	w := newLabelWriter(t, 0)
	require.NoError(t, w.fib.ensureMPLSTableLocked())
	w.ch.retval = -6
	err := w.apply(swapEntry(100, []uint32{200}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retval=-6")
	assert.NotContains(t, w.fib.labelOwners, uint32(100))

	w.ch.retval = 0
	w.ch.sendErr = errors.New("socket closed")
	err = w.apply(swapEntry(100, []uint32{200}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "socket closed")
	assert.NotContains(t, w.fib.labelOwners, uint32(100))
}

// VALIDATES: AC-23 -- the shared validation refuses an oversize in-label, an
// empty swap stack, a stack deeper than MaxLabelStack and a swap at nonzero
// TableID; another source's in-label is refused with EEXIST; remove deletes
// both S routes; RemoveLabelSource removes only that source's labels and keeps
// a failed deletion owned. Method: the subscription over the fake channel.
func TestFibVPPLabelValidationAndOwnership(t *testing.T) {
	w := newLabelWriter(t, 0)
	deep := make([]uint32, mplsfib.MaxLabelStack+1)
	for _, e := range []mplsfib.Entry{
		swapEntry(mplsfib.MaxLabel+1, []uint32{200}),
		swapEntry(100, nil),
		swapEntry(100, deep),
		{Action: mplsfib.ActionAdd, Op: mplsfib.OpSwap, InLabel: 100, OutLabels: []uint32{200},
			NextHop: netip.MustParseAddr("192.0.2.1"), TableID: 7, Source: 1},
	} {
		require.Error(t, w.apply(e))
	}
	assert.Empty(t, w.routes())

	require.NoError(t, w.apply(swapEntry(100, []uint32{200})))
	foreign := swapEntry(100, []uint32{201})
	foreign.Source = 2
	err := w.apply(foreign)
	require.ErrorIs(t, err, syscall.EEXIST)

	require.NoError(t, w.apply(swapEntry(101, []uint32{202})))
	other := swapEntry(102, []uint32{203})
	other.Source = 2
	require.NoError(t, w.apply(other))

	before := len(w.routes())
	remove := swapEntry(100, nil)
	remove.Action = mplsfib.ActionRemove
	require.NoError(t, w.apply(remove))
	dels := w.routes()[before:]
	require.Len(t, dels, 2)
	assert.False(t, dels[0].MrIsAdd)
	assert.Equal(t, uint8(1), dels[0].MrRoute.MrEos)
	assert.Equal(t, uint8(0), dels[1].MrRoute.MrEos)

	w.ch.retval = -1
	require.Error(t, mplsfib.RemoveLabelSource(w.bus, 1))
	assert.Contains(t, w.fib.labelOwners, uint32(101), "a failed deletion stays owned")
	w.ch.retval = 0
	require.NoError(t, mplsfib.RemoveLabelSource(w.bus, 1))
	assert.NotContains(t, w.fib.labelOwners, uint32(101))
	assert.Equal(t, uint16(2), w.fib.labelOwners[102], "another source's label is kept")
}

// VALIDATES: AC-24 -- a push with a nonzero TableID (facility-backup control
// context) is refused naming the private table and the missing packet-mark
// context; nothing is programmed. Method: the subscription.
func TestFibVPPLabelPrivateTableRefused(t *testing.T) {
	w := newLabelWriter(t, 0)
	err := w.apply(mplsfib.Entry{Action: mplsfib.ActionAdd, Op: mplsfib.OpPush, TableID: 0x7a000001,
		FEC: netip.MustParsePrefix("198.51.100.0/24"), OutLabels: []uint32{300},
		NextHop: netip.MustParseAddr("192.0.2.1"), Source: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0x7a000001")
	assert.Contains(t, err.Error(), "packet-mark")
	assert.Empty(t, w.ch.requests)
}

// VALIDATES: AC-26 -- an entry with a PathMTU is installed without one and the
// warning is raised once per writer. Method: two entries carrying PathMTU.
func TestFibVPPLabelPathMTUWarnsOnce(t *testing.T) {
	w := newLabelWriter(t, 0)
	first := swapEntry(100, []uint32{200})
	first.PathMTU = 1400
	require.NoError(t, w.apply(first))
	assert.True(t, w.fib.pathMTUWarned)
	second := swapEntry(101, []uint32{201})
	second.PathMTU = 1300
	require.NoError(t, w.apply(second))
	assert.Len(t, w.routes(), 4, "both entries are installed without an MTU")
}

// VALIDATES: AC-25 -- a batch answering another writer's replay request is
// neither programmed nor acknowledged; one carrying this writer's token is.
// Method: ApplyReplay with a foreign token, then with the writer's own.
func TestFibVPPLabelReplayTargetsThisWriter(t *testing.T) {
	w := newLabelWriter(t, 0)
	foreign := mplsfib.NewReplayToken()
	err := mplsfib.ApplyReplay(w.bus, foreign, []mplsfib.Entry{swapEntry(100, []uint32{200})})
	require.Error(t, err, "no writer acknowledged the foreign batch")
	assert.Empty(t, w.routes())
	require.NoError(t, mplsfib.ApplyReplay(w.bus, w.fib.labelReplayToken,
		[]mplsfib.Entry{swapEntry(100, []uint32{200})}))
	assert.Len(t, w.routes(), 2)
}

// VALIDATES: AC-25 -- opening a writer creates table 0 and then emits one
// targeted (mpls-fib, replay-request) carrying the writer's token. Method:
// requestLabelReplay against the synchronous bus.
func TestFibVPPLabelReplayOnWriterOpen(t *testing.T) {
	w := newLabelWriter(t, 0)
	var tokens []uint64
	unsub := mplsfib.ReplayRequest.Subscribe(w.bus, func(r *replay.Request) {
		tokens = append(tokens, r.ReplayID)
		assert.True(t, w.fib.labelTableReady, "table 0 exists before the request")
	})
	t.Cleanup(unsub)
	w.fib.requestLabelReplay(w.bus)
	assert.Equal(t, []uint64{w.fib.labelReplayToken}, tokens)
}

// VALIDATES: AC-22 -- with no writer the subscription acknowledges with the
// cause, so Apply returns it; a targeted batch is left unacknowledged.
// Method: the subscription with a nil current writer.
func TestFibVPPLabelAckWithoutWriter(t *testing.T) {
	bus := &syncBus{}
	var mu sync.Mutex
	var current *fibVPP
	cause := errors.New("VPP is not answering")
	unsub := subscribeFibVPPLabels(bus, &mu, &current, func() error { return cause })
	t.Cleanup(unsub)
	err := mplsfib.Apply(bus, []mplsfib.Entry{swapEntry(100, []uint32{200})})
	require.ErrorIs(t, err, cause)
	err = mplsfib.ApplyReplay(bus, mplsfib.NewReplayToken(), []mplsfib.Entry{swapEntry(100, []uint32{200})})
	require.Error(t, err)
	assert.NotErrorIs(t, err, cause)
}

// VALIDATES: AC-29 -- flush removes every label route and push entry the
// writer installed, with the IP routes. Method: install, then flushRoutes.
func TestFibVPPFlushRemovesLabelEntries(t *testing.T) {
	w := newLabelWriter(t, 0)
	require.NoError(t, w.apply(swapEntry(100, []uint32{200})))
	require.NoError(t, w.apply(mplsfib.Entry{Action: mplsfib.ActionAdd, Op: mplsfib.OpPush,
		FEC: netip.MustParsePrefix("198.51.100.0/24"), OutLabels: []uint32{300},
		NextHop: netip.MustParseAddr("192.0.2.1"), Source: 1}))
	before := len(w.ch.requests)
	w.fib.flushRoutes()
	var labelDels, ipDels int
	for _, m := range w.ch.requests[before:] {
		switch req := m.(type) {
		case *mpls.MplsRouteAddDel:
			if !req.MrIsAdd {
				labelDels++
			}
		case *ip.IPRouteAddDel:
			if !req.IsAdd {
				ipDels++
			}
		}
	}
	assert.Equal(t, 2, labelDels)
	assert.Equal(t, 1, ipDels)
	assert.Empty(t, w.fib.labelOwners)
	assert.Empty(t, w.fib.labelPushes)
}
