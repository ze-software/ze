// Design: plan/spec-mpls-3-rsvp-te.md -- ze-to-ze RSVP-TE signaling interop
// Related: engine.go -- the engine under test; transport.go -- the Transport seam
// Related: engine_test.go -- reuses fakeFIB and the single-engine harness
//
// engine_test.go drives ONE engine with hand-built PATH/RESV packets. These cases
// wire TWO or THREE real engines through an in-memory fabric, so each engine's OWN
// encoded bytes (buildPath via sendPath, buildResv/buildPathErr inside the
// handlers) are delivered to the peer's DecodeMessage. Nothing in the exchange is
// hand-built: a test only originates an LSP at the head-end and inspects the
// resulting state, labels, RRO and FIB programming at every node. A green run is
// the fully-open evidence that ze's RSVP-TE encoder and decoder agree on the wire
// across nodes, in the absence of any open-source RSVP-TE peer to interop against.
package rsvpte

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// fabric is an in-memory RSVP transport mesh. A node's Send is delivered to the
// next node along the path as a Packet whose Src is the sender's router address
// (the (src, payload) shape the real raw-IP transport yields on Recv). RESV,
// PathErr and PathTear go to their addressed destination; a PATH instead follows
// its explicit route -- it is delivered to the first remaining ERO hop, modeling
// the Router Alert hop-by-hop interception RSVP-TE relies on. Each node strips
// itself from the ERO before relaying (engine.nextHopFromERO), so ERO[0] is always
// the immediate downstream neighbor. A packet to an unattached address is dropped,
// as on a real network with no listener.
type fabric struct {
	mu    sync.Mutex
	ports map[netip.Addr]*fabricPort
	tap   []deliveredMsg
}

// deliveredMsg records one routed packet so tests can assert the message flow.
type deliveredMsg struct {
	dst     netip.Addr
	msgType uint8
}

func newFabric() *fabric { return &fabric{ports: make(map[netip.Addr]*fabricPort)} }

// fabricPort is one node's Transport into the fabric.
type fabricPort struct {
	fab    *fabric
	self   netip.Addr
	recvCh chan Packet
}

// attach registers a node at self and returns its Transport. recvCh is buffered
// above the per-exchange message count so Send never blocks under the synchronous
// pump.
func (fab *fabric) attach(self netip.Addr) *fabricPort {
	p := &fabricPort{fab: fab, self: self, recvCh: make(chan Packet, 256)}
	fab.mu.Lock()
	fab.ports[self] = p
	fab.mu.Unlock()
	return p
}

func (p *fabricPort) Send(dst netip.Addr, msg []byte) error {
	cp := append([]byte(nil), msg...) // the engine reuses its send buffer; copy out

	// A PATH follows its explicit route: deliver to the first remaining ERO hop
	// (the immediate downstream neighbor), not straight to the tunnel endpoint.
	deliverTo := dst
	var msgType uint8
	if parsed, err := DecodeMessage(cp); err == nil {
		msgType = parsed.Header.MsgType
		if msgType == MsgTypePath && parsed.HasERO && len(parsed.ERO) > 0 {
			deliverTo = parsed.ERO[0].Address.Addr()
		}
	}

	p.fab.mu.Lock()
	p.fab.tap = append(p.fab.tap, deliveredMsg{dst: deliverTo, msgType: msgType})
	peer := p.fab.ports[deliverTo]
	p.fab.mu.Unlock()
	if peer == nil {
		return nil // no node at the destination: dropped, like a packet to nowhere
	}
	peer.recvCh <- Packet{Src: p.self, Payload: cp}
	return nil
}

func (p *fabricPort) Recv() <-chan Packet { return p.recvCh }
func (p *fabricPort) Close() error        { return nil }

// delivered counts packets of msgType the fabric routed to dst.
func (fab *fabric) delivered(dst netip.Addr, msgType uint8) int {
	fab.mu.Lock()
	defer fab.mu.Unlock()
	n := 0
	for _, d := range fab.tap {
		if d.dst == dst && d.msgType == msgType {
			n++
		}
	}
	return n
}

// pump delivers every queued packet to its node's handlePacket until the fabric is
// quiescent. Fully synchronous -- no engine goroutines, no sleeps -- so the
// exchange is deterministic: a converged setup settles in a few rounds, and a
// signaling loop trips the iteration bound (a clear failure) instead of hanging.
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

// originateLSP sets up an ingress (head-end) LSP on e toward endpoint and emits its
// first PATH through e's real sendPath -- e GENERATES the PATH, the fabric carries
// those exact bytes. An optional explicit route (ero) pins the hops the PATH must
// traverse; with none, endpoint is the direct next hop.
func originateLSP(t *testing.T, e *engine, sender, endpoint netip.Addr, tunnelID uint16, ero ...netip.Addr) lspKey {
	t.Helper()
	return originateLSPProtected(t, e, sender, endpoint, tunnelID, nil, ero...)
}

// originateLSPProtected is originateLSP with an optional RFC 4090 protection
// request set on the head-end PSB (so the PATH carries FAST_REROUTE/SESSION_ATTRIBUTE).
func originateLSPProtected(t *testing.T, e *engine, sender, endpoint netip.Addr, tunnelID uint16, pr *protectionRequest, ero ...netip.Addr) lspKey {
	t.Helper()
	key := lspKey{
		TunnelEndpoint: endpoint,
		TunnelID:       tunnelID,
		ExtTunnelID:    0x0a000001,
		SenderAddr:     sender,
		LSPID:          1,
	}
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: endpoint, TunnelID: tunnelID, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: sender, LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
		Protection:     pr,
	}
	for _, hop := range ero {
		psb.ERO = append(psb.ERO, eroHop{Address: netip.PrefixFrom(hop, hop.BitLen())})
	}
	lsp, _ := e.table.GetOrCreate(key)
	lsp.mu.Lock()
	lsp.Role = RoleIngress
	lsp.PSB = psb
	lsp.setState(LSPStatePathSent)
	lsp.mu.Unlock()
	require.NoError(t, e.sendPath(lsp))
	return key
}

// fabricEngineCfg is fabricEngine with a config modifier, so a PLR node can be
// given interfaces (for link-down matching) and configured bypass LSPs.
func fabricEngineCfg(t *testing.T, fab *fabric, self netip.Addr, cfgFn func(*rsvpteConfig)) (*engine, *fakeFIB) {
	t.Helper()
	fib := &fakeFIB{}
	cfg := rsvpteConfig{RouterID: self, RefreshPeriod: DefaultRefreshPeriod}
	if cfgFn != nil {
		cfgFn(&cfg)
	}
	e := newEngine(fab.attach(self), newLSPTable(), newAdmissionController(), fib, cfg, slogutil.DiscardLogger())
	for _, ic := range cfg.Interfaces {
		e.admission.setInterface(ic.Name, float64(ic.MaxBW), float64(ic.MaxReservableBW))
	}
	return e, fib
}

// setEngineInterfaces swaps the engine's interface config (the engine config is
// behind an atomic pointer, so it cannot be mutated in place).
func setEngineInterfaces(e *engine, ifaces []ifaceConfig) {
	cfg := e.cfg()
	cfg.Interfaces = ifaces
	e.setConfig(cfg)
}

// VALIDATES: mpls-3 -- a full ze-to-ze LSP setup. One ze engine originates and
// ENCODES a PATH; a second ze engine DECODES it, allocates a label, programs a pop
// and ENCODES a RESV; the first ze engine DECODES that RESV and programs a push. No
// packet is hand-built, so a green run proves ze's RSVP-TE encoder and decoder
// agree across two independent engine instances.
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
	assert.Equal(t, netip.PrefixFrom(egressAddr, 32), ingFib.pushed[0], "push FEC is the tunnel endpoint")

	// Exactly one PATH downstream and one RESV upstream crossed the wire.
	assert.Equal(t, 1, fab.delivered(egressAddr, MsgTypePath), "one PATH to the egress")
	assert.Equal(t, 1, fab.delivered(ingressAddr, MsgTypeResv), "one RESV back to the ingress")
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

// VALIDATES: mpls-3 -- a full three-node LSP (ingress -> transit -> egress)
// signaled across three real engines. The PATH is relayed hop-by-hop along the ERO
// and the RESV back upstream; each node decodes the peer's encoded message and
// acts. The label stack threads correctly through all three: the ingress pushes the
// transit's in-label, the transit swaps it to the egress's in-label, the egress
// pops. The head-end records the full route (RRO) the RESV carried.
// PREVENTS: a multi-hop encode/decode bug -- ERO consumption, label swap, or RRO
// accumulation -- that a two-node test cannot reach.
func TestEngineZeToZeTransitInterop(t *testing.T) {
	fab := newFabric()
	ingressAddr := netip.MustParseAddr("10.0.0.1")
	transitAddr := netip.MustParseAddr("10.0.0.5")
	egressAddr := netip.MustParseAddr("10.0.0.9")

	ingress, ingFib := fabricEngine(t, fab, ingressAddr)
	transit, trFib := fabricEngine(t, fab, transitAddr)
	egress, egrFib := fabricEngine(t, fab, egressAddr)
	nodes := map[netip.Addr]*engine{ingressAddr: ingress, transitAddr: transit, egressAddr: egress}

	// Explicit route ingress -> transit -> egress.
	key := originateLSP(t, ingress, ingressAddr, egressAddr, 1, transitAddr, egressAddr)
	fab.pump(t, nodes)

	// Egress: tail-end -- allocated its in-label and popped it.
	egrLSP, ok := egress.table.Get(key)
	require.True(t, ok, "egress installed state")
	assert.Equal(t, LSPStateUp, egrLSP.State)
	assert.Equal(t, RoleEgress, egrLSP.Role)
	require.Len(t, egrFib.popped, 1, "egress popped its in-label")
	labelE := egrFib.popped[0]

	// Transit: allocated its own in-label and programmed a swap to the egress's.
	trLSP, ok := transit.table.Get(key)
	require.True(t, ok, "transit installed state")
	assert.Equal(t, LSPStateUp, trLSP.State)
	assert.Equal(t, RoleTransit, trLSP.Role)
	require.Len(t, trFib.swapped, 1, "transit programmed a swap")
	swap := trFib.swapped[0]
	assert.Equal(t, labelE, swap.out, "transit swaps to the egress's in-label")
	assert.Equal(t, swap.in, trLSP.InLabel)
	assert.Equal(t, labelE, trLSP.OutLabel)

	// Ingress: head-end -- learned the transit's in-label and pushed it.
	ingLSP, ok := ingress.table.Get(key)
	require.True(t, ok, "ingress installed state")
	assert.Equal(t, LSPStateUp, ingLSP.State)
	assert.Equal(t, swap.in, ingLSP.OutLabel,
		"ingress pushes the transit's in-label (the stack threads through all three engines)")
	require.Len(t, ingFib.pushed, 1, "ingress programmed a push")
	assert.Equal(t, netip.PrefixFrom(egressAddr, 32), ingFib.pushed[0])

	// RRO accumulated as the RESV traveled upstream (RFC 3209 Section 4.4): the
	// head-end sees the full recorded path, every entry round-tripped on the wire.
	require.NotNil(t, ingLSP.RSB)
	require.Len(t, ingLSP.RSB.RRO, 3, "head-end records all three hops")
	assert.Equal(t, ingressAddr, ingLSP.RSB.RRO[0].Address)
	assert.Equal(t, transitAddr, ingLSP.RSB.RRO[1].Address)
	assert.Equal(t, egressAddr, ingLSP.RSB.RRO[2].Address)

	// Message flow: PATH relayed downstream twice, RESV relayed upstream twice.
	assert.Equal(t, 1, fab.delivered(transitAddr, MsgTypePath), "PATH ingress->transit")
	assert.Equal(t, 1, fab.delivered(egressAddr, MsgTypePath), "PATH transit->egress")
	assert.Equal(t, 1, fab.delivered(transitAddr, MsgTypeResv), "RESV egress->transit")
	assert.Equal(t, 1, fab.delivered(ingressAddr, MsgTypeResv), "RESV transit->ingress")
}

// VALIDATES: mpls-3 -- the error path round-trips. An ingress originates a PATH
// whose explicit route ends at the transit (no hop on toward the endpoint), so the
// transit cannot resolve a next hop and ENCODES a PathErr back upstream. The ingress
// DECODES it: the LSP never comes up and no forwarding entry is programmed. Proves
// the PathErr encoder and decoder agree across engines.
// PREVENTS: a RESV-vs-PathErr divergence that would let a failed LSP look
// established, or a malformed PathErr the head-end silently drops.
func TestEngineZeToZePathErrInterop(t *testing.T) {
	fab := newFabric()
	ingressAddr := netip.MustParseAddr("10.0.0.1")
	transitAddr := netip.MustParseAddr("10.0.0.5")
	endpoint := netip.MustParseAddr("10.0.0.9") // no node here: the ERO dead-ends at the transit

	ingress, ingFib := fabricEngine(t, fab, ingressAddr)
	transit, trFib := fabricEngine(t, fab, transitAddr)
	nodes := map[netip.Addr]*engine{ingressAddr: ingress, transitAddr: transit}

	// The explicit route names only the transit: it strips itself and finds no next
	// hop, so it rejects with a PathErr (RFC 3209 routing-problem / bad ERO).
	key := originateLSP(t, ingress, ingressAddr, endpoint, 1, transitAddr)
	fab.pump(t, nodes)

	// The transit encoded a PathErr that crossed back to the ingress; no RESV.
	assert.Equal(t, 1, fab.delivered(ingressAddr, MsgTypePathErr), "transit sent a PathErr upstream")
	assert.Zero(t, fab.delivered(ingressAddr, MsgTypeResv), "no RESV on a routing failure")

	// The transit installed no state and programmed nothing (it rejected before
	// creating the LSP).
	assert.Empty(t, transit.table.All(), "transit holds no state for a rejected PATH")
	assert.Empty(t, trFib.swapped, "transit programmed no swap")

	// The ingress decoded the PathErr: the LSP is not up and nothing was pushed.
	ingLSP, ok := ingress.table.Get(key)
	require.True(t, ok)
	assert.NotEqual(t, LSPStateUp, ingLSP.State, "ingress LSP must not come up on a PathErr")
	assert.Empty(t, ingFib.pushed, "no push entry for a failed LSP")
}

// mustLSP fetches an LSP that must exist, failing the test otherwise.
func mustLSP(t *testing.T, e *engine, key lspKey) *LSP {
	t.Helper()
	lsp, ok := e.table.Get(key)
	require.True(t, ok, "LSP %s present", key.String())
	return lsp
}

// VALIDATES: mpls-3 -- teardown round-trips and propagates. After a three-node LSP
// is up, the head-end's teardownLSP sends a PathTear that each node decodes and acts
// on: every node removes its LSP state and forwarding entry, and the transit relays
// the PathTear on toward the egress. This is the head-end-originated teardown wired
// into config removal (reconcileTunnels) and make-before-break.
// PREVENTS: a PathTear that clears the head-end but strands transit swaps or the
// egress pop in the FIB.
func TestEngineZeToZeTeardownInterop(t *testing.T) {
	fab := newFabric()
	ingressAddr := netip.MustParseAddr("10.0.0.1")
	transitAddr := netip.MustParseAddr("10.0.0.5")
	egressAddr := netip.MustParseAddr("10.0.0.9")

	ingress, ingFib := fabricEngine(t, fab, ingressAddr)
	transit, trFib := fabricEngine(t, fab, transitAddr)
	egress, egrFib := fabricEngine(t, fab, egressAddr)
	nodes := map[netip.Addr]*engine{ingressAddr: ingress, transitAddr: transit, egressAddr: egress}

	key := originateLSP(t, ingress, ingressAddr, egressAddr, 1, transitAddr, egressAddr)
	fab.pump(t, nodes)
	require.Equal(t, LSPStateUp, mustLSP(t, ingress, key).State, "LSP up before teardown")
	require.Len(t, trFib.swapped, 1)
	require.Len(t, egrFib.popped, 1)
	transitInLabel := trFib.swapped[0].in
	egressInLabel := egrFib.popped[0]

	// Head-end tears the LSP down; the PathTear propagates downstream.
	ingress.teardownLSP(key)
	fab.pump(t, nodes)

	// Every node dropped its LSP state.
	assert.Empty(t, ingress.table.All(), "ingress state cleared")
	assert.Empty(t, transit.table.All(), "transit state cleared")
	assert.Empty(t, egress.table.All(), "egress state cleared")

	// Every forwarding entry was withdrawn.
	assert.Equal(t, []netip.Prefix{netip.PrefixFrom(egressAddr, 32)}, ingFib.removed, "ingress push withdrawn")
	assert.Equal(t, []uint32{transitInLabel}, trFib.removedSwap, "transit swap withdrawn")
	assert.Equal(t, []uint32{egressInLabel}, egrFib.removedSwap, "egress pop withdrawn")

	// The PathTear was relayed hop-by-hop.
	assert.Equal(t, 1, fab.delivered(transitAddr, MsgTypePathTear), "PathTear ingress->transit")
	assert.Equal(t, 1, fab.delivered(egressAddr, MsgTypePathTear), "PathTear transit->egress")
}

// VALIDATES: mpls-3 -- soft-state refresh round-trips idempotently. After an LSP is
// up, the head-end re-sends its PATH (RFC 2205 soft-state); the egress decodes the
// refresh, keeps the same label and reservation, and does NOT pop a second time.
// PREVENTS: a refresh PATH the egress mishandles as a new LSP -- re-allocating a
// label, double-popping, or creating a duplicate session.
func TestEngineZeToZeRefreshInterop(t *testing.T) {
	fab := newFabric()
	ingressAddr := netip.MustParseAddr("10.0.0.1")
	egressAddr := netip.MustParseAddr("10.0.0.9")

	ingress, _ := fabricEngine(t, fab, ingressAddr)
	egress, egrFib := fabricEngine(t, fab, egressAddr)
	nodes := map[netip.Addr]*engine{ingressAddr: ingress, egressAddr: egress}

	key := originateLSP(t, ingress, ingressAddr, egressAddr, 1)
	fab.pump(t, nodes)
	require.Len(t, egrFib.popped, 1)
	label := egrFib.popped[0]
	require.Equal(t, label, mustLSP(t, ingress, key).OutLabel)

	// Refresh: re-send the same PATH (what runRefreshLoop does for an ingress LSP).
	require.NoError(t, ingress.sendPath(mustLSP(t, ingress, key)))
	fab.pump(t, nodes)

	assert.Len(t, egress.table.All(), 1, "refresh does not create a second egress LSP")
	assert.Equal(t, LSPStateUp, mustLSP(t, egress, key).State, "egress still up after refresh")
	assert.Len(t, egrFib.popped, 1, "egress does not pop again on refresh")
	assert.Equal(t, label, egrFib.popped[0], "egress label stable across refresh")
	assert.Equal(t, label, mustLSP(t, ingress, key).OutLabel, "ingress out-label stable")
}

// VALIDATES: mpls-3 -- admission rejection round-trips. The egress link is full, so
// the egress rejects the reservation and encodes a PathErr (admission control
// failure) instead of a RESV. The head-end decodes it: the LSP stays down with no
// push, and no RESV is ever sent.
// PREVENTS: an admission-failure PathErr the head-end misreads as success.
func TestEngineZeToZeAdmissionDeniedInterop(t *testing.T) {
	fab := newFabric()
	ingressAddr := netip.MustParseAddr("10.0.0.1")
	egressAddr := netip.MustParseAddr("10.0.0.9")

	ingress, ingFib := fabricEngine(t, fab, ingressAddr)
	egress, _ := fabricEngine(t, fab, egressAddr)
	// One interface on the egress, filled: any further reservation is rejected.
	setEngineInterfaces(egress, []ifaceConfig{{Name: "eth0", MaxBW: 1e9, MaxReservableBW: 1e9}})
	egress.admission.setInterface("eth0", 1e9, 1e9)
	require.NoError(t, egress.admission.Reserve("eth0", 1e9))
	nodes := map[netip.Addr]*engine{ingressAddr: ingress, egressAddr: egress}

	key := originateLSP(t, ingress, ingressAddr, egressAddr, 1)
	fab.pump(t, nodes)

	// The egress encoded a PathErr to the ingress; no RESV.
	require.Equal(t, 1, fab.delivered(ingressAddr, MsgTypePathErr), "egress sent a PathErr")
	assert.Zero(t, fab.delivered(ingressAddr, MsgTypeResv), "no RESV on admission failure")

	// The ingress decoded it: LSP not up, nothing pushed.
	assert.NotEqual(t, LSPStateUp, mustLSP(t, ingress, key).State, "ingress LSP must not come up")
	assert.Empty(t, ingFib.pushed, "no push on a denied LSP")
}

// VALIDATES: mpls-3 -- make-before-break reroute round-trips across engines. An LSP
// up via one transit is rerouted onto a path via a different transit; the new PATH
// signals end-to-end and, once its RESV returns, the head-end tears the old LSP down
// (PathTear via the old transit). The new LSP is up via the new transit and the old
// is gone everywhere.
// PREVENTS: a reroute that brings up the new path but strands the old LSP, or tears
// the old before the new is up.
func TestEngineZeToZeRerouteInterop(t *testing.T) {
	fab := newFabric()
	ingressAddr := netip.MustParseAddr("10.0.0.1")
	transitA := netip.MustParseAddr("10.0.0.5")
	transitB := netip.MustParseAddr("10.0.0.6")
	egressAddr := netip.MustParseAddr("10.0.0.9")

	ingress, _ := fabricEngine(t, fab, ingressAddr)
	tA, tAFib := fabricEngine(t, fab, transitA)
	tB, tBFib := fabricEngine(t, fab, transitB)
	egress, _ := fabricEngine(t, fab, egressAddr)
	nodes := map[netip.Addr]*engine{ingressAddr: ingress, transitA: tA, transitB: tB, egressAddr: egress}

	// Up via transitA.
	oldKey := originateLSP(t, ingress, ingressAddr, egressAddr, 1, transitA, egressAddr)
	fab.pump(t, nodes)
	require.Equal(t, LSPStateUp, mustLSP(t, ingress, oldKey).State)
	require.Len(t, tAFib.swapped, 1, "transitA swapped for the original LSP")

	// Reroute onto a path via transitB.
	newERO := []eroHop{
		{Address: netip.PrefixFrom(transitB, 32)},
		{Address: netip.PrefixFrom(egressAddr, 32)},
	}
	newKey, ok := ingress.reroute(oldKey, newERO)
	require.True(t, ok, "reroute started")
	fab.pump(t, nodes)

	// New LSP up via transitB.
	newLSP, ok := ingress.table.Get(newKey)
	require.True(t, ok)
	assert.Equal(t, LSPStateUp, newLSP.State, "new LSP up")
	assert.Equal(t, transitB, newLSP.NextHop, "new LSP routed via transitB")
	require.Len(t, tBFib.swapped, 1, "transitB swapped for the new LSP")

	// Old LSP torn down everywhere.
	_, ok = ingress.table.Get(oldKey)
	assert.False(t, ok, "old LSP removed at the head-end")
	_, ok = tA.table.Get(oldKey)
	assert.False(t, ok, "old LSP removed at transitA")
	assert.Equal(t, 1, fab.delivered(transitA, MsgTypePathTear), "old PathTear relayed via transitA")
	assert.Len(t, egress.table.All(), 1, "egress holds only the new LSP")
}

// VALIDATES: mpls-3 reload -- a configured tunnel whose ERO changes on reload
// reroutes through the production path: reconcileTunnels -> setupTunnel sees the up
// LSP with a changed ERO and triggers make-before-break. Drives the whole
// composition (not eng.reroute directly): the LSP comes up via one transit, a
// second reconcile with a new ERO brings the replacement up via the other transit
// and tears the original down.
// PREVENTS: the reload reroute trigger silently not firing (the gap the OnConfigApply
// wiring closed) even though reroute itself works.
func TestEngineZeToZeReloadRerouteInterop(t *testing.T) {
	fab := newFabric()
	ingressAddr := netip.MustParseAddr("10.0.0.1")
	transitA := netip.MustParseAddr("10.0.0.5")
	transitB := netip.MustParseAddr("10.0.0.6")
	egressAddr := netip.MustParseAddr("10.0.0.9")
	log := slogutil.DiscardLogger()

	ingress, _ := fabricEngine(t, fab, ingressAddr)
	tA, tAFib := fabricEngine(t, fab, transitA)
	tB, tBFib := fabricEngine(t, fab, transitB)
	egress, _ := fabricEngine(t, fab, egressAddr)
	nodes := map[netip.Addr]*engine{ingressAddr: ingress, transitA: tA, transitB: tB, egressAddr: egress}

	cfg := rsvpteConfig{RouterID: ingressAddr, RefreshPeriod: DefaultRefreshPeriod}
	cfg.Tunnels = []tunnelConfig{{
		Name: "t1", Destination: egressAddr, TunnelID: 1, Bandwidth: 1e8,
		ERO: []eroHop{{Address: netip.PrefixFrom(transitA, 32)}, {Address: netip.PrefixFrom(egressAddr, 32)}},
	}}

	// Initial reconcile: the tunnel is set up and signals up via transitA.
	prev := reconcileTunnels(log, ingress.table, cfg, ingress, nil)
	fab.pump(t, nodes)
	oldKey := tunnelKey(cfg.Tunnels[0], ingressAddr)
	require.Equal(t, LSPStateUp, mustLSP(t, ingress, oldKey).State, "LSP up via transitA")
	require.Len(t, tAFib.swapped, 1)

	// Reload the same tunnel with the ERO changed to via transitB: reconcileTunnels
	// -> setupTunnel sees the up LSP with a changed ERO -> make-before-break reroute.
	cfg.Tunnels[0].ERO = []eroHop{{Address: netip.PrefixFrom(transitB, 32)}, {Address: netip.PrefixFrom(egressAddr, 32)}}
	reconcileTunnels(log, ingress.table, cfg, ingress, prev)
	fab.pump(t, nodes)

	newKey := oldKey
	newKey.LSPID = oldKey.LSPID + 1
	newLSP, ok := ingress.table.Get(newKey)
	require.True(t, ok, "reload reroute created the replacement LSP")
	assert.Equal(t, LSPStateUp, newLSP.State, "replacement up after reload reroute")
	assert.Equal(t, transitB, newLSP.NextHop, "replacement routed via transitB")
	require.Len(t, tBFib.swapped, 1, "transitB swapped for the replacement")
	_, ok = ingress.table.Get(oldKey)
	assert.False(t, ok, "original LSP torn down once the replacement is up")
}

// VALIDATES: mpls-3 -- soft-state refresh across a three-node LSP. The head-end
// re-sends its PATH; the transit re-relays it and the egress re-RESVs, all decoded
// and acted on by the peer. The egress does not pop a second time and every node's
// label is stable: a refresh maintains state, it does not re-establish it.
// PREVENTS: a refresh that a transit or egress mis-handles as a new LSP, churning
// labels or reservations across the path.
func TestEngineZeToZeTransitRefreshInterop(t *testing.T) {
	fab := newFabric()
	ingressAddr := netip.MustParseAddr("10.0.0.1")
	transitAddr := netip.MustParseAddr("10.0.0.5")
	egressAddr := netip.MustParseAddr("10.0.0.9")

	ingress, _ := fabricEngine(t, fab, ingressAddr)
	transit, trFib := fabricEngine(t, fab, transitAddr)
	egress, egrFib := fabricEngine(t, fab, egressAddr)
	nodes := map[netip.Addr]*engine{ingressAddr: ingress, transitAddr: transit, egressAddr: egress}

	key := originateLSP(t, ingress, ingressAddr, egressAddr, 1, transitAddr, egressAddr)
	fab.pump(t, nodes)
	require.Len(t, egrFib.popped, 1)
	require.Len(t, trFib.swapped, 1)
	egressLabel := egrFib.popped[0]
	transitInLabel := trFib.swapped[0].in

	// Refresh: re-send the PATH; it propagates to the egress and the RESV back.
	require.NoError(t, ingress.sendPath(mustLSP(t, ingress, key)))
	fab.pump(t, nodes)

	// The egress pop is idempotent (guarded by the reservation); the transit
	// re-programs its swap on the returning RESV (idempotent at the FIB), but with
	// the SAME labels -- no path-wide re-allocation, and every node stays up.
	assert.Len(t, egrFib.popped, 1, "egress does not pop again on refresh")
	assert.Equal(t, egressLabel, egrFib.popped[0], "egress label stable")
	assert.Equal(t, transitInLabel, mustLSP(t, transit, key).InLabel, "transit in-label stable")
	for _, sw := range trFib.swapped {
		assert.Equal(t, transitInLabel, sw.in, "transit swap in-label stable across refresh")
		assert.Equal(t, egressLabel, sw.out, "transit swap out-label stable across refresh")
	}
	assert.Equal(t, LSPStateUp, mustLSP(t, transit, key).State, "transit still up")
	assert.Equal(t, LSPStateUp, mustLSP(t, egress, key).State, "egress still up")
	assert.Len(t, transit.table.All(), 1, "no duplicate transit LSP")
	assert.Len(t, egress.table.All(), 1, "no duplicate egress LSP")
}

// VALIDATES: mpls-3 -- admission resolves the correct interface by prefix in a
// multi-interface egress. The link facing the ingress is full while another link is
// free; the egress must reject (PathErr) because the reservation maps to the full
// link, not be admitted onto the unrelated free one.
// PREVENTS: admission charging the wrong interface (or skipping it) when several
// interfaces are configured, which would let an oversubscribed link accept an LSP.
func TestEngineZeToZeMultiIfaceAdmissionInterop(t *testing.T) {
	fab := newFabric()
	ingressAddr := netip.MustParseAddr("10.0.0.1")
	egressAddr := netip.MustParseAddr("10.0.0.9")

	ingress, ingFib := fabricEngine(t, fab, ingressAddr)
	egress, _ := fabricEngine(t, fab, egressAddr)
	// Two interfaces; the ingress neighbor (10.0.0.1) resolves to eth0 by prefix.
	setEngineInterfaces(egress, []ifaceConfig{
		{Name: "eth0", MaxBW: 1e9, MaxReservableBW: 1e9, Prefix: netip.MustParsePrefix("10.0.0.0/24")},
		{Name: "eth1", MaxBW: 1e9, MaxReservableBW: 1e9, Prefix: netip.MustParsePrefix("10.1.0.0/24")},
	})
	egress.admission.setInterface("eth0", 1e9, 1e9)
	egress.admission.setInterface("eth1", 1e9, 1e9)
	require.NoError(t, egress.admission.Reserve("eth0", 1e9)) // fill only the ingress-facing link
	nodes := map[netip.Addr]*engine{ingressAddr: ingress, egressAddr: egress}

	key := originateLSP(t, ingress, ingressAddr, egressAddr, 1)
	fab.pump(t, nodes)

	// eth0 (resolved from the ingress neighbor) is full, so the egress rejects even
	// though eth1 is free: the reservation must map to the path's link, not any link.
	require.Equal(t, 1, fab.delivered(ingressAddr, MsgTypePathErr), "egress rejected on the full eth0")
	assert.Zero(t, fab.delivered(ingressAddr, MsgTypeResv), "no RESV onto the wrong free link")
	assert.NotEqual(t, LSPStateUp, mustLSP(t, ingress, key).State, "ingress LSP must not come up")
	assert.Empty(t, ingFib.pushed, "no push on a denied LSP")
}

// VALIDATES: mpls-4 (RFC 4090 facility backup) end to end over the fabric --
// four real ze engines: a head-end (A), a Point of Local Repair (B), a merge
// point / egress (C), and a bypass transit (E). A protected LSP A->B->C is
// signaled with fast-reroute; B arms a configured bypass B->E->C. When B's link
// to C fails, B redirects the protected LSP onto the bypass with a 2-label stack
// via E (no re-signaling round trip), sends a Notify (code 25/3) toward A, keeps
// the LSP up, and A re-optimizes make-before-break. Every PATH/RESV/PathErr is
// each engine's own encoded bytes decoded by its peer.
// PREVENTS: a wire or control-flow divergence in the multi-node FRR exchange that
// single-engine tests cannot surface (no open-source RSVP-TE peer exists).
func TestEngineZeToZeFRRLocalRepair(t *testing.T) {
	fab := newFabric()
	aAddr := netip.MustParseAddr("10.0.0.1") // head-end
	bAddr := netip.MustParseAddr("10.0.0.2") // Point of Local Repair
	cAddr := netip.MustParseAddr("10.0.0.3") // merge point + egress
	eAddr := netip.MustParseAddr("10.0.2.5") // bypass transit

	a, _ := fabricEngine(t, fab, aAddr)
	b, bFib := fabricEngineCfg(t, fab, bAddr, func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{
			{Name: "eth0", Prefix: netip.MustParsePrefix("10.0.0.0/24"), MaxBW: 10e9, MaxReservableBW: 10e9},
			{Name: "eth1", Prefix: netip.MustParsePrefix("10.0.2.0/24"), MaxBW: 10e9, MaxReservableBW: 10e9},
		}
		c.Bypasses = []bypassConfig{{
			Name: "bp", MergePoint: cAddr,
			ERO: []eroHop{{Address: netip.PrefixFrom(eAddr, 32)}, {Address: netip.PrefixFrom(cAddr, 32)}},
		}}
	})
	c, _ := fabricEngine(t, fab, cAddr)
	e, _ := fabricEngine(t, fab, eAddr)
	nodes := map[netip.Addr]*engine{aAddr: a, bAddr: b, cAddr: c, eAddr: e}

	// B signals its configured bypass (B->E->C) and it comes up over the fabric.
	reconcileTunnels(slogutil.DiscardLogger(), b.table, b.cfg(), b, nil)
	fab.pump(t, nodes)
	bypass, ok := b.table.Get(bypassKey(b.cfg().Bypasses[0], bAddr))
	require.True(t, ok)
	require.Equal(t, LSPStateUp, bypass.State, "bypass LSP up via E")
	require.Equal(t, eAddr, bypass.NextHop, "bypass next hop is the transit E")

	// Head-end A originates the protected LSP A->B->C with facility protection.
	pr := &protectionRequest{Facility: true, HopLimit: 16, Bandwidth: 1e8}
	key := originateLSPProtected(t, a, aAddr, cAddr, 1, pr, bAddr, cAddr)
	fab.pump(t, nodes)

	bLSP, ok := b.table.Get(key)
	require.True(t, ok, "PLR installed the protected transit LSP")
	require.NotNil(t, bLSP.Bypass, "PLR armed the bypass from the PATH protection request")
	require.Equal(t, LSPStateUp, mustLSP(t, a, key).State, "protected LSP up end to end")

	// B's link to C fails. B locally repairs onto the bypass instead of tearing
	// down. The Notify is queued but not yet delivered, so the repaired LSP is
	// still in place here -- check the local-repair state before re-optimization
	// (the head-end will tear the repaired LSP down once it re-optimizes, AC-5).
	b.handleLinkDown("eth0")

	require.Len(t, bFib.backups, 1, "PLR programmed a backup swap on local repair")
	assert.Len(t, bFib.backups[0].out, 2, "2-label facility-backup stack (bypass over protected)")
	assert.Equal(t, eAddr, bFib.backups[0].nextHop, "traffic redirected via the bypass next hop E")

	repaired, ok := b.table.Get(key)
	require.True(t, ok, "protected LSP retained (not torn down) on local repair")
	repaired.mu.Lock()
	inUse := repaired.ProtectionInUse
	repaired.mu.Unlock()
	assert.True(t, inUse, "PLR marked local protection in use")

	// Deliver the Notify and let the head-end re-optimize make-before-break.
	fab.pump(t, nodes)
	assert.GreaterOrEqual(t, fab.delivered(aAddr, MsgTypePathErr), 1, "Notify delivered to the head-end")
	newKey := key
	newKey.LSPID = 2
	_, reopt := a.table.Get(newKey)
	assert.True(t, reopt, "head-end re-optimized onto a fresh LSP after the Notify")
}
