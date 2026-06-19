// Design: plan/spec-mpls-3-rsvp-te.md -- ze-to-ze RSVP-TE signaling interop
// Related: engine.go -- the engine under test; transport.go -- the Transport seam
// Related: engine_test.go -- reuses fakeFIB and the single-engine harness
//
// The engine_test.go cases drive ONE engine with hand-built PATH/RESV packets.
// These cases instead wire TWO (or more) real engines through an in-memory
// fabric, so each engine's OWN encoded bytes (buildPath via sendPath, buildResv
// via handlePathEgress) are delivered to the peer's DecodeMessage. Nothing in the
// exchange is hand-built: the test only originates an LSP at the head-end and
// inspects the resulting state and FIB programming at both ends. This is the
// closest fully-open check that ze's RSVP-TE encoder and decoder agree on the
// wire, in the absence of any open-source RSVP-TE peer to interop against.
package rsvpte

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// fabric is an in-memory RSVP transport mesh. A node's Send is delivered to the
// destination node's receive queue as a Packet whose Src is the sender's router
// address -- exactly the (src, payload) shape the real raw-IP transport yields on
// Recv. A packet addressed to an unattached node is dropped (black hole), as it
// would be on a real network with no listener.
type fabric struct {
	mu    sync.Mutex
	ports map[netip.Addr]*fabricPort
}

func newFabric() *fabric { return &fabric{ports: make(map[netip.Addr]*fabricPort)} }

// fabricPort is one node's Transport into the fabric. It satisfies the Transport
// interface the engine sends/receives over.
type fabricPort struct {
	fab    *fabric
	self   netip.Addr
	recvCh chan Packet
}

// attach registers a node at self and returns its Transport. recvCh is buffered
// well above the per-exchange message count so Send never blocks under the
// synchronous pump (a single PATH/RESV setup enqueues a handful of packets).
func (fab *fabric) attach(self netip.Addr) *fabricPort {
	p := &fabricPort{fab: fab, self: self, recvCh: make(chan Packet, 256)}
	fab.mu.Lock()
	fab.ports[self] = p
	fab.mu.Unlock()
	return p
}

func (p *fabricPort) Send(dst netip.Addr, msg []byte) error {
	cp := append([]byte(nil), msg...) // the engine reuses its send buffer; copy out
	p.fab.mu.Lock()
	peer := p.fab.ports[dst]
	p.fab.mu.Unlock()
	if peer == nil {
		return nil // no node at dst: dropped, like a packet to nowhere
	}
	peer.recvCh <- Packet{Src: p.self, Payload: cp}
	return nil
}

func (p *fabricPort) Recv() <-chan Packet { return p.recvCh }
func (p *fabricPort) Close() error        { return nil }

// pump delivers every queued packet to its node's handlePacket until the fabric
// is quiescent. It is fully synchronous -- no engine goroutines, no sleeps -- so
// the exchange is deterministic: a converged LSP setup settles in a few rounds,
// and a signaling loop trips the iteration bound (a clear failure) instead of
// hanging. nodes maps each attached address to the engine that owns it.
func (fab *fabric) pump(t *testing.T, nodes map[netip.Addr]*engine) {
	t.Helper()
	for round := 0; ; round++ {
		require.Less(t, round, 1000, "rsvp-te fabric did not converge (signaling loop?)")
		moved := false
		for addr, port := range fab.ports {
			select {
			case pkt := <-port.recvCh:
				nodes[addr].handlePacket(pkt)
				moved = true
			default:
			}
		}
		if !moved {
			return
		}
	}
}

// fabricEngine constructs an engine attached to fab at self, with a fresh LSP
// table, admission controller (no interfaces -> admission skipped, as in
// TestEngineEgressPathToResv) and a recording fakeFIB (defined in engine_test.go).
func fabricEngine(t *testing.T, fab *fabric, self netip.Addr) (*engine, *fakeFIB) {
	t.Helper()
	fib := &fakeFIB{}
	cfg := rsvpteConfig{RouterID: self, RefreshPeriod: DefaultRefreshPeriod}
	e := newEngine(fab.attach(self), newLSPTable(), newAdmissionController(), fib, cfg, slogutil.DiscardLogger())
	return e, fib
}

// originateLSP sets up an ingress (head-end) LSP on e toward endpoint and emits
// its first PATH through e's real sendPath -- e GENERATES the PATH, the fabric
// carries those exact bytes. This mirrors what the configured tunnel head-end
// does. The endpoint is directly reachable (no ERO), so the egress is the next
// hop and treats the PATH as its own (SESSION.TunnelEndpoint == its RouterID).
func originateLSP(t *testing.T, e *engine, sender, endpoint netip.Addr, tunnelID uint16) lspKey {
	t.Helper()
	key := lspKey{
		TunnelEndpoint: endpoint,
		TunnelID:       tunnelID,
		ExtTunnelID:    0x0a000001,
		SenderAddr:     sender,
		LSPID:          1,
	}
	lsp, _ := e.table.GetOrCreate(key)
	lsp.mu.Lock()
	lsp.Role = RoleIngress
	lsp.PSB = &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: endpoint, TunnelID: tunnelID, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: sender, LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
	}
	lsp.setState(LSPStatePathSent)
	lsp.mu.Unlock()
	require.NoError(t, e.sendPath(lsp))
	return key
}

// VALIDATES: mpls-3 -- a full ze-to-ze LSP setup. One ze engine originates and
// ENCODES a PATH; a second ze engine DECODES it, allocates a label, programs a
// pop and ENCODES a RESV; the first ze engine DECODES that RESV and programs a
// push. No packet is hand-built, so a green run proves ze's RSVP-TE encoder and
// decoder agree across two independent engine instances.
// PREVENTS: an encoder/decoder divergence (object order, length, checksum, label
// placement) that single-engine round-trip tests, which decode what the same
// process encoded with the same helpers, could mask.
func TestEngineZeToZeLSPInterop(t *testing.T) {
	fab := newFabric()
	ingressAddr := netip.MustParseAddr("10.0.0.1")
	egressAddr := netip.MustParseAddr("10.0.0.9")

	ingress, ingFib := fabricEngine(t, fab, ingressAddr)
	egress, egrFib := fabricEngine(t, fab, egressAddr)
	nodes := map[netip.Addr]*engine{ingressAddr: ingress, egressAddr: egress}

	key := originateLSP(t, ingress, ingressAddr, egressAddr, 1)
	fab.pump(t, nodes)

	// The egress decoded the ze-generated PATH and acted on it.
	egrLSP, ok := egress.table.Get(key)
	require.True(t, ok, "egress installed LSP state from the ze-generated PATH")
	assert.Equal(t, LSPStateUp, egrLSP.State, "egress LSP up")
	assert.Equal(t, RoleEgress, egrLSP.Role)
	require.Len(t, egrFib.popped, 1, "egress programmed a pop for its in-label")
	egressLabel := egrFib.popped[0]
	assert.GreaterOrEqual(t, egressLabel, uint32(firstDynamicLabel), "label from the dynamic pool")

	// The ingress decoded the ze-generated RESV and acted on it. The crux: the
	// label the ingress learned must equal the label the egress allocated and
	// encoded -- proof the value survived encode -> wire -> decode across engines.
	ingLSP, ok := ingress.table.Get(key)
	require.True(t, ok)
	assert.Equal(t, LSPStateUp, ingLSP.State, "ingress LSP up")
	assert.Equal(t, egressLabel, ingLSP.OutLabel,
		"ingress out-label must equal the label the egress encoded in its RESV")
	require.Len(t, ingFib.pushed, 1, "ingress programmed a push entry")
	assert.Equal(t, netip.PrefixFrom(egressAddr, 32), ingFib.pushed[0],
		"push FEC is the tunnel endpoint")
}

// VALIDATES: mpls-3 -- the "vice versa": each ze engine acts as BOTH head-end
// (encodes PATH, decodes RESV) and tail-end (decodes PATH, encodes RESV) at once.
// Two LSPs are signaled in opposite directions across the same engine pair; both
// reach Up at both ends with the label each tail-end allocated reflected in the
// matching head-end's out-label.
// PREVENTS: a direction-dependent codec bug (e.g. a field only set on send, not
// read on receive) that a single-direction test would miss.
func TestEngineZeToZeInteropBothDirections(t *testing.T) {
	fab := newFabric()
	a := netip.MustParseAddr("10.0.0.1")
	b := netip.MustParseAddr("10.0.0.9")

	eA, fibA := fabricEngine(t, fab, a)
	eB, fibB := fabricEngine(t, fab, b)
	nodes := map[netip.Addr]*engine{a: eA, b: eB}

	keyAB := originateLSP(t, eA, a, b, 1) // A -> B: A ingress, B egress
	keyBA := originateLSP(t, eB, b, a, 2) // B -> A: B ingress, A egress
	fab.pump(t, nodes)

	// LSP A->B: B is the tail-end (allocates + pops), A is the head-end (push).
	bEgress, ok := eB.table.Get(keyAB)
	require.True(t, ok, "B installed egress state for A->B")
	assert.Equal(t, LSPStateUp, bEgress.State)
	require.Len(t, fibB.popped, 1, "B popped for the A->B LSP")
	aHead, ok := eA.table.Get(keyAB)
	require.True(t, ok)
	assert.Equal(t, LSPStateUp, aHead.State)
	assert.Equal(t, fibB.popped[0], aHead.OutLabel, "A learned B's encoded label")

	// LSP B->A: A is the tail-end (allocates + pops), B is the head-end (push).
	aEgress, ok := eA.table.Get(keyBA)
	require.True(t, ok, "A installed egress state for B->A")
	assert.Equal(t, LSPStateUp, aEgress.State)
	require.Len(t, fibA.popped, 1, "A popped for the B->A LSP")
	bHead, ok := eB.table.Get(keyBA)
	require.True(t, ok)
	assert.Equal(t, LSPStateUp, bHead.State)
	assert.Equal(t, fibA.popped[0], bHead.OutLabel, "B learned A's encoded label")

	// Each engine pushed exactly once (as head-end of its own LSP).
	assert.Len(t, fibA.pushed, 1, "A pushed for A->B")
	assert.Len(t, fibB.pushed, 1, "B pushed for B->A")
}
