// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- signaling engine tests (fake transport)
package rsvpte

import (
	"errors"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

type sentMsg struct {
	dst     netip.Addr
	payload []byte
	route   PathRoute
}

type fakeTransport struct {
	mu             sync.Mutex
	sent           []sentMsg
	recvCh         chan Packet
	localAddresses []netip.Addr
	routes         map[netip.Prefix]RouteInfo
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{recvCh: make(chan Packet, 16)}
}

func (f *fakeTransport) Send(dst netip.Addr, msg []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]byte, len(msg))
	copy(cp, msg)
	f.sent = append(f.sent, sentMsg{dst: dst, payload: cp})
	return nil
}

func (f *fakeTransport) SendPath(route PathRoute, msg []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentMsg{dst: route.NextHop, payload: slices.Clone(msg), route: route})
	return nil
}

func (f *fakeTransport) LocalAddresses() ([]InterfaceAddress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	addresses := make([]InterfaceAddress, len(f.localAddresses))
	for i, address := range f.localAddresses {
		addresses[i] = InterfaceAddress{Address: address, Up: true, MTU: 1500}
	}
	return addresses, nil
}

func (f *fakeTransport) ResolveRoute(target netip.Prefix, destination netip.Addr, _ uint32) (RouteInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.routes != nil {
		if route, ok := f.routes[target]; ok {
			return route, nil
		}
		return RouteInfo{}, errors.New("no fixture route")
	}
	next := target.Addr()
	if target.Contains(destination) {
		next = destination
	} else if target.Bits() != 32 {
		return RouteInfo{}, errors.New("abstract node needs an explicit fixture route")
	}
	return RouteInfo{NextHop: next, Lookup: next, MTU: 1500}, nil
}

func (f *fakeTransport) Recv() <-chan Packet { return f.recvCh }
func (f *fakeTransport) Close() error        { close(f.recvCh); return nil }

func (f *fakeTransport) lastByType(msgType uint8) (*ParsedMessage, netip.Addr, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for index := range slices.Backward(f.sent) {
		sent := &f.sent[index]
		msg, err := DecodeMessage(sent.payload)
		if err == nil && msg.Header.MsgType == msgType {
			return msg, sent.dst, true
		}
	}
	return nil, netip.Addr{}, false
}

type swapRec struct {
	in, out uint32
}

type backupRec struct {
	in      uint32
	out     []uint32
	nextHop netip.Addr
}

type fakeFIB struct {
	pushed       []netip.Prefix
	pushLabels   [][]uint32
	pushTables   []uint32
	removed      []netip.Prefix
	removeTables []uint32
	swapped      []swapRec
	backups      []backupRec
	popped       []uint32
	removedSwap  []uint32
	pathMTUs     []uint32
}

func (f *fakeFIB) programPush(fec netip.Prefix, labels []uint32, _ netip.Addr, tableID, pathMTU uint32) error {
	f.pathMTUs = append(f.pathMTUs, pathMTU)
	f.pushed = append(f.pushed, fec)
	f.pushLabels = append(f.pushLabels, slices.Clone(labels))
	f.pushTables = append(f.pushTables, tableID)
	return nil
}

func (f *fakeFIB) programSwap(inLabel, outLabel uint32, _ netip.Addr, pathMTU uint32) error {
	f.pathMTUs = append(f.pathMTUs, pathMTU)
	f.swapped = append(f.swapped, swapRec{in: inLabel, out: outLabel})
	return nil
}

func (f *fakeFIB) programBackup(inLabel uint32, outLabels []uint32, nextHop netip.Addr, pathMTU uint32) error {
	f.pathMTUs = append(f.pathMTUs, pathMTU)
	f.backups = append(f.backups, backupRec{in: inLabel, out: append([]uint32(nil), outLabels...), nextHop: nextHop})
	return nil
}

func (f *fakeFIB) programPop(inLabel uint32, _ netip.Addr, pathMTU uint32) error {
	f.pathMTUs = append(f.pathMTUs, pathMTU)
	f.popped = append(f.popped, inLabel)
	return nil
}

func (f *fakeFIB) removePush(fec netip.Prefix, tableID uint32) error {
	f.removed = append(f.removed, fec)
	f.removeTables = append(f.removeTables, tableID)
	return nil
}

func (f *fakeFIB) removeSwap(inLabel uint32) error {
	f.removedSwap = append(f.removedSwap, inLabel)
	return nil
}

func testEngine(t *testing.T, routerID string, cfg func(*rsvpteConfig)) (*engine, *fakeTransport, *fakeFIB) {
	t.Helper()
	c := rsvpteConfig{RouterID: netip.MustParseAddr(routerID), RefreshPeriod: DefaultRefreshPeriod}
	if cfg != nil {
		cfg(&c)
	}
	ft := newFakeTransport()
	ft.localAddresses = []netip.Addr{c.RouterID}
	fib := &fakeFIB{}
	e := newEngine(ft, newLSPTable(), newAdmissionController(), fib, c, slogutil.DiscardLogger())
	return e, ft, fib
}

// VALIDATES: AC-2 -- PATH at egress allocates a label and returns a RESV; LSP up.
func TestEngineEgressPathToResv(t *testing.T) {
	e, ft, fib := testEngine(t, "10.0.0.9", nil)

	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
		RefreshPeriod:  DefaultRefreshPeriod,
	}
	path := buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})

	resv, dst, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok, "egress sends a RESV")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), dst, "RESV goes to PATH source")
	require.Len(t, resv.FlowDescriptors, 1)
	require.Len(t, resv.FlowDescriptors[0].Filters, 1)
	require.True(t, resv.FlowDescriptors[0].Filters[0].HasLabel)
	assert.GreaterOrEqual(t, resv.FlowDescriptors[0].Filters[0].Label.Label, uint32(1000), "label allocated from pool")

	lsp, ok := e.table.Get(keyFromFilter(resv.Session, resv.FlowDescriptors[0].Filters[0].Filter))
	require.True(t, ok)
	assert.Equal(t, LSPStateUp, lsp.State)
	assert.Equal(t, RoleEgress, lsp.Role)

	require.Len(t, fib.popped, 1, "egress programs a pop for its in-label")
	assert.Equal(t, resv.FlowDescriptors[0].Filters[0].Label.Label, fib.popped[0])
}

func TestAssignedInterfaceAddressIsLocalRSVPHop(t *testing.T) {
	local := netip.MustParseAddr("192.0.2.5")
	ingress := netip.MustParseAddr("10.0.0.1")
	peer := netip.MustParseAddr("192.0.2.1")
	for _, role := range []string{"transit", "egress", "prefix-only"} {
		t.Run(role, func(t *testing.T) {
			e, ft, fib := testEngine(t, "10.0.0.5", func(cfg *rsvpteConfig) {
				cfg.Interfaces = []ifaceConfig{{Name: "eth0", Prefix: netip.PrefixFrom(local, 24).Masked()}}
			})
			ft.localAddresses = append(ft.localAddresses, local)
			e.admission.setInterface("eth0", 1e9, 1e9)
			endpoint := netip.MustParseAddr("10.0.0.9")
			switch role {
			case "egress":
				endpoint = local
			case "prefix-only":
				endpoint = e.cfg().Interfaces[0].Prefix.Addr()
			}
			psb := &pathStateBlock{
				Session:        sessionIPv4{TunnelEndpoint: endpoint, TunnelID: 1, ExtTunnelID: 1},
				SenderTemplate: senderTemplateIPv4{SenderAddr: ingress, LSPID: 1},
				ERO:            []eroHop{{Address: netip.PrefixFrom(local, 32)}, {Address: netip.PrefixFrom(endpoint, 32)}},
				LabelRequest:   labelRequest{L3PID: 0x0800},
			}
			e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, peer, defaultIPTTL)})
			if role != "egress" {
				path, dst, ok := ft.lastByType(MsgTypePath)
				require.True(t, ok)
				assert.Equal(t, endpoint, dst, "do not send the PATH back to a local address")
				require.Len(t, path.ERO, 1)
				assert.Equal(t, endpoint, path.ERO[0].Address.Addr())
				return
			}
			resv, dst, ok := ft.lastByType(MsgTypeResv)
			require.True(t, ok, "an assigned interface endpoint terminates the PATH")
			assert.Equal(t, peer, dst, "reply follows RSVP_HOP, not the preserved sender IP")
			assert.Equal(t, endpoint, resv.Session.TunnelEndpoint)
			require.Len(t, fib.popped, 1)
			assert.Equal(t, fib.popped[0], resv.FlowDescriptors[0].Filters[0].Label.Label)
		})
	}
}

// VALIDATES: AC-4 -- RESV at ingress records the label, programs push, LSP up.
func TestEngineIngressResvToUp(t *testing.T) {
	e, ft, fib := testEngine(t, "10.0.0.1", nil)

	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		ExtTunnelID: 0x0a000001, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	setupTunnel(e.log, e.table, tunnelConfig{Destination: key.TunnelEndpoint, TunnelID: key.TunnelID}, e.cfg(), e)
	require.Equal(t, 1, ft.countByType(MsgTypePath), "ingress signaled the PATH")

	rsb := &resvStateBlock{
		Session: sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: key.TunnelID, ExtTunnelID: key.ExtTunnelID},
		Label:   labelObject{Label: 16050},
		Style:   StyleSharedExplicit,
	}
	filter := senderTemplateIPv4{SenderAddr: key.SenderAddr, LSPID: key.LSPID}
	resv := buildResv(rsb, filter, DefaultRefreshPeriod, netip.MustParseAddr("10.0.0.9"))
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.9"), Payload: resv})

	got, _ := e.table.Get(key)
	assert.Equal(t, LSPStateUp, got.State)
	assert.Equal(t, uint32(16050), got.OutLabel)
	require.Len(t, fib.pushed, 1, "ingress programs a push entry")
	assert.Equal(t, netip.MustParsePrefix("10.0.0.9/32"), fib.pushed[0])
}

// VALIDATES: AC-3 -- a transit node relays PATH downstream along the ERO, and
// on the returning RESV allocates a local label, programs a swap, and relays the
// RESV upstream carrying that label.
func TestEngineTransitForwarding(t *testing.T) {
	e, ft, fib := testEngine(t, "10.0.0.5", nil)

	ingress := netip.MustParseAddr("10.0.0.1")
	egress := netip.MustParseAddr("10.0.0.9")
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: egress, TunnelID: 1, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: ingress, LSPID: 1},
		ERO:            []eroHop{{Address: netip.MustParsePrefix("10.0.0.5/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}},
		SenderTSpec:    FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
	}
	// Transit node receives PATH from the ingress.
	e.handlePacket(Packet{Src: ingress, Payload: buildPath(psb, ingress, 64)})

	// It relays a PATH toward the next ERO hop (the egress).
	fwd, dst, ok := ft.lastByType(MsgTypePath)
	require.True(t, ok, "transit relays PATH downstream")
	assert.Equal(t, egress, dst, "PATH relayed to next ERO hop")
	// RFC requirement: RFC3209-4.3.4-1 positive -- the transit node removes its own leading ERO subobject (nextHopFromERO, engine.go:407-416) before relaying, so the forwarded PATH's ERO begins at the next hop (the egress).
	require.Len(t, fwd.ERO, 1, "transit consumed its own ERO subobject; only the egress hop remains")
	assert.Equal(t, egress, fwd.ERO[0].Address.Addr())

	key := keyFromMessage(fwd)
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	assert.Equal(t, RoleTransit, lsp.Role)
	assert.Equal(t, LSPStatePathReceived, lsp.State)

	// The egress's RESV comes back with its label; transit swaps and relays.
	rsb := &resvStateBlock{
		Session: psb.Session,
		Label:   labelObject{Label: 18000},
		Style:   StyleSharedExplicit,
	}
	filter := psb.SenderTemplate
	e.handlePacket(Packet{Src: egress, Payload: buildResv(rsb, filter, DefaultRefreshPeriod, egress)})

	up, _ := e.table.Get(key)
	assert.Equal(t, LSPStateUp, up.State)
	assert.Equal(t, uint32(18000), up.OutLabel)
	assert.NotZero(t, up.InLabel, "transit allocated a local in-label")

	require.Len(t, fib.swapped, 1, "transit programs a swap")
	assert.Equal(t, up.InLabel, fib.swapped[0].in)
	assert.Equal(t, uint32(18000), fib.swapped[0].out)

	resv, resvDst, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok, "transit relays RESV upstream")
	assert.Equal(t, ingress, resvDst, "RESV relayed to the upstream PHOP")
	assert.Equal(t, up.InLabel, resv.FlowDescriptors[0].Filters[0].Label.Label, "upstream RESV carries our local label")
}

// VALIDATES: label reuse (regression for N1) -- a released label is handed out
// again before the monotonic counter advances, so wraparound cannot collide
// with a live label.
func TestLSPTableLabelReuse(t *testing.T) {
	tbl := newLSPTable()
	l1 := tbl.AllocateLabel()
	l2 := tbl.AllocateLabel()
	assert.NotEqual(t, l1, l2)

	tbl.releaseLabel(l1)
	l3 := tbl.AllocateLabel()
	assert.Equal(t, l1, l3, "released label is reused")

	tbl.releaseLabel(0) // no-op, must not be handed out
	l4 := tbl.AllocateLabel()
	assert.NotZero(t, l4)
	assert.NotEqual(t, l2, l4)
}

// VALIDATES: refresh idempotency (regression for B2/B3) -- a repeated PATH (RFC
// 2205 soft-state refresh) must NOT reserve bandwidth again or hand out a new
// label; the reservation stays at one LSP's worth and the egress label is stable.
func TestEngineEgressRefreshIdempotent(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{{Name: "eth0", MaxBW: 10e9, MaxReservableBW: 10e9}}
	})
	e.admission.setInterface("eth0", 10e9, 10e9)

	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 1e9, TokenBucket: 1e9, PeakRate: 1e9},
		LabelRequest:   labelRequest{L3PID: 0x0800},
	}
	path := buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64)

	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})
	resv1, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	label1 := resv1.FlowDescriptors[0].Filters[0].Label.Label
	ib, _ := e.admission.GetInterface("eth0")
	require.InDelta(t, 8e9, ib.ReservedBandwidth, 1, "first PATH reserves once")

	// Refresh: same PATH again.
	beforeRefresh := ft.countByType(MsgTypeResv)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})
	require.Equal(t, beforeRefresh+1, ft.countByType(MsgTypeResv), "refresh emits a new RESV")
	assert.Zero(t, ft.countByType(MsgTypePathErr), "refresh is accepted, not rejected by admission")
	ib, _ = e.admission.GetInterface("eth0")
	assert.InDelta(t, 8e9, ib.ReservedBandwidth, 1, "refresh must not double-reserve")

	resv2, _, ok := ft.lastByType(MsgTypeResv)
	require.True(t, ok)
	assert.Equal(t, label1, resv2.FlowDescriptors[0].Filters[0].Label.Label, "egress label is stable across refresh")
	assert.Len(t, e.table.All(), 1, "still one LSP")
}

// VALIDATES: transit relay loop bound (regression for I4) -- a PATH whose TTL is
// exhausted is dropped, neither relayed nor installed.
func TestEngineTransitTTLExhausted(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.5", nil)
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		ERO:            []eroHop{{Address: netip.MustParsePrefix("10.0.0.5/32")}, {Address: netip.MustParsePrefix("10.0.0.9/32")}},
		SenderTSpec:    FlowSpec{TokenRate: 1e8},
	}
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: buildPath(psb, netip.MustParseAddr("10.0.0.1"), 1)})

	if _, _, ok := ft.lastByType(MsgTypePath); ok {
		t.Fatal("TTL-exhausted PATH must not be relayed")
	}
	assert.Empty(t, e.table.All(), "no state installed for a dropped PATH")
}

// VALIDATES: AC-8 -- a PATH that oversubscribes the egress interface is rejected
// with a PathErr (admission control failure) and no RESV is sent.
func TestEngineAdmissionDeniedPathErr(t *testing.T) {
	e, ft, _ := testEngine(t, "10.0.0.9", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{{Name: "eth0", MaxBW: 1e9, MaxReservableBW: 1e9}}
	})
	e.admission.setInterface("eth0", 1e9, 1e9)
	require.NoError(t, e.admission.Reserve("eth0", 1e9)) // fill the link

	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 2},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 5e8, TokenBucket: 5e8, PeakRate: 5e8},
		LabelRequest:   labelRequest{L3PID: 0x0800},
	}
	path := buildPath(psb, netip.MustParseAddr("10.0.0.1"), 64)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})

	perr, dst, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok, "admission failure sends a PathErr")
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), dst)
	assert.Equal(t, ErrCodeAdmissionControlFailure, perr.ErrorSpec.ErrorCode)

	if _, _, ok := ft.lastByType(MsgTypeResv); ok {
		t.Fatal("no RESV should be sent on admission failure")
	}
}

// VALIDATES: AC-11 -- PathTear removes LSP state and releases bandwidth.
func TestEnginePathTearReleases(t *testing.T) {
	e, _, fib := testEngine(t, "10.0.0.9", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{{Name: "eth0", MaxBW: 10e9, MaxReservableBW: 10e9}}
	})
	e.admission.setInterface("eth0", 10e9, 10e9)

	// Establish an egress LSP via PATH.
	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 3},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		SenderTSpec:    FlowSpec{TokenRate: 4e8, TokenBucket: 4e8, PeakRate: 4e8},
	}
	src := netip.MustParseAddr("10.0.0.1")
	e.handlePacket(Packet{Src: src, Payload: buildPath(psb, src, 64)})
	ib, _ := e.admission.GetInterface("eth0")
	require.InDelta(t, 3.2e9, ib.ReservedBandwidth, 1, "reservation held after PATH")

	require.Len(t, fib.popped, 1, "egress programmed a pop entry")
	popLabel := fib.popped[0]

	// Tear it down. The egress withdraws its pop entry (in-label keyed, so via
	// removeSwap) and releases bandwidth; no push entry exists to remove.
	e.handlePacket(Packet{Src: src, Payload: buildPathTear(psb, src)})
	ib, _ = e.admission.GetInterface("eth0")
	assert.InDelta(t, 0, ib.ReservedBandwidth, 1, "bandwidth released after PathTear")
	assert.Empty(t, e.table.All(), "LSP removed")
	assert.Empty(t, fib.removed, "egress programs no push entry to withdraw")
	require.Len(t, fib.removedSwap, 1, "egress pop entry withdrawn")
	assert.Equal(t, popLabel, fib.removedSwap[0])
}

// VALIDATES: a RESV that carries no LABEL object is rejected -- the LABEL is
// mandatory in a RESV, so decoding drops it and the ingress LSP is not brought up.
func TestEngineResvWithoutLabelRejected(t *testing.T) {
	// RFC requirement: RFC3209-4.1-1 negative -- a RESV with no LABEL object is rejected by DecodeMessage; the ingress LSP stays path-sent and no push is programmed.
	e, ft, fib := testEngine(t, "10.0.0.1", nil)

	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		ExtTunnelID: 0x0a000001, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	setupTunnel(e.log, e.table, tunnelConfig{Destination: key.TunnelEndpoint, TunnelID: key.TunnelID, Bandwidth: 8e6}, e.cfg(), e)
	require.Equal(t, 1, ft.countByType(MsgTypePath), "ingress signaled the PATH")

	// A RESV carrying every object RFC 2205 Section 3.1.4 makes mandatory, so the
	// only thing wrong with it is the deliberately absent LABEL.
	session := sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: key.TunnelID, ExtTunnelID: key.ExtTunnelID}
	filter := senderTemplateIPv4{SenderAddr: key.SenderAddr, LSPID: key.LSPID}
	raw := encodeMessage(MsgTypeResv, defaultIPTTL, []objEncoder{
		func(b []byte) int { return encodeSessionIPv4(b, session) },
		func(b []byte) int { return encodeRSVPHop(b, rsvpHop{NextHop: netip.MustParseAddr("10.0.0.9")}) },
		func(b []byte) int { return encodeTimeValues(b, timeValues{RefreshPeriod: 30000}) },
		func(b []byte) int { return encodeStyle(b, StyleSharedExplicit) },
		func(b []byte) int {
			return encodeFlowSpec(b, ClassFlowSpec, FlowSpec{TokenRate: 1e6, TokenBucket: 1e6, PeakRate: 1e6})
		},
		func(b []byte) int { return encodeFilterSpec(b, filter) },
	})

	// The descriptor is otherwise legal; only its mandatory LABEL is absent.
	msg, err := DecodeMessage(raw)
	require.Error(t, err, "a RESV descriptor without LABEL must not decode")
	require.True(t, msg.HasSession)
	require.Len(t, msg.FlowDescriptors, 1)
	require.Len(t, msg.FlowDescriptors[0].Filters, 1)
	require.False(t, msg.FlowDescriptors[0].Filters[0].HasLabel, "crafted RESV must carry no LABEL")

	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.9"), Payload: raw})

	got, ok := e.table.Get(key)
	require.True(t, ok)
	assert.Equal(t, LSPStatePathSent, got.State, "labelless RESV is rejected; LSP stays path-sent")
	assert.Zero(t, got.OutLabel, "no out-label recorded from a labelless RESV")
	assert.Empty(t, fib.pushed, "no push programmed for a rejected RESV")
}

// VALIDATES: when a transit node exhausts the ERO and implicit routing has no
// route to the destination, it returns Routing Problem / No route available
// toward the sender without forwarding or installing state.
func TestEngineTransitNoUsableERONextHop(t *testing.T) {
	// RFC requirement: RFC3209-4.3.4-1 negative -- after removing the exhausted ERO, a failed implicit lookup forwards no PATH and sends Routing Problem / No route available.
	e, ft, _ := testEngine(t, "10.0.0.5", nil)
	ft.routes = map[netip.Prefix]RouteInfo{}

	psb := &pathStateBlock{
		Session:        sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001},
		SenderTemplate: senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1},
		// The ERO names only this transit node; after it removes itself nothing remains.
		ERO:          []eroHop{{Address: netip.MustParsePrefix("10.0.0.5/32")}},
		SenderTSpec:  FlowSpec{TokenRate: 1e8},
		LabelRequest: labelRequest{L3PID: 0x0800},
	}
	src := netip.MustParseAddr("10.0.0.1")
	e.handlePacket(Packet{Src: src, Payload: buildPath(psb, src, 64)})

	perr, dst, ok := ft.lastByType(MsgTypePathErr)
	require.True(t, ok, "a PATH with no usable ERO next hop sends a PathErr")
	assert.Equal(t, src, dst, "PathErr goes back to the PATH source")
	assert.Equal(t, ErrCodeRoutingProblem, perr.ErrorSpec.ErrorCode)
	assert.Equal(t, ErrValueNoRouteAvailable, perr.ErrorSpec.ErrorValue)

	if _, _, relayed := ft.lastByType(MsgTypePath); relayed {
		t.Fatal("no PATH must be relayed when the ERO has no usable next hop")
	}
	assert.Empty(t, e.table.All(), "no state installed for an unusable-ERO PATH")
}
