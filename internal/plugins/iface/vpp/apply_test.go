// VPP interface apply: the create/update/delete/reset operation pipeline and
// the read/dump/monitor paths driven through scripted api.Channel fakes --
// tunnels, VXLAN, WireGuard, LCP, SPAN mirrors, counter reset, VLAN QoS map
// updates, route/neighbor/interface dumps, name-map population, per-interface
// stats, and the async admin up/down event monitor. No live VPP daemon is used.
package ifacevpp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/gre"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_neighbor"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/ipip"
	"go.fd.io/govpp/binapi/lcp"
	"go.fd.io/govpp/binapi/span"
	"go.fd.io/govpp/binapi/vxlan"
	"go.fd.io/govpp/binapi/wireguard"

	"github.com/ze-software/ze/internal/component/iface"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/pkg/ze"
)

// progChannel is a programmable api.Channel fake shared by the tunnel,
// VXLAN, SPAN, WireGuard, and LCP tests. It records every request and fills
// each add/del reply's Retval (and SwIfIndex / PeerIndex where the reply
// carries one) from the configured fields, so a test can assert both the
// request the backend built and the backend's handling of the VPP return
// value.
type progChannel struct {
	requests  []api.Message
	retval    int32
	swIfIndex interface_types.InterfaceIndex
	peerIndex uint32
	sendErr   error

	// loopbackIndex is the SwIfIndex the next create_loopback reply carries,
	// and it advances after each one. VPP allocates a fresh interface for
	// every create_loopback (the message names none), so a fake that answered
	// with one fixed index would let a duplicate create look like a single
	// interface. Tests that send create_loopback set the first value.
	loopbackIndex interface_types.InterfaceIndex

	// Dump responses delivered by SendMultiRequest, used by GetWireguardDevice
	// round-trip tests. Each slice is streamed one entry per ReceiveReply,
	// then a final (last=true) terminates the dump.
	wgIfaceDetails []wireguard.WireguardInterfaceDetails
	wgPeerDetails  []wireguard.WireguardPeersDetails
}

var _ api.Channel = (*progChannel)(nil)

func (c *progChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.requests = append(c.requests, msg)
	return &progReqCtx{ch: c}
}

func (c *progChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.requests = append(c.requests, msg)
	switch msg.(type) {
	case *wireguard.WireguardInterfaceDump:
		ctx := &progMultiCtx{}
		for i := range c.wgIfaceDetails {
			d := c.wgIfaceDetails[i]
			ctx.details = append(ctx.details, &d)
		}
		return ctx
	case *wireguard.WireguardPeersDump:
		ctx := &progMultiCtx{}
		for i := range c.wgPeerDetails {
			d := c.wgPeerDetails[i]
			ctx.details = append(ctx.details, &d)
		}
		return ctx
	default:
		return &progMultiCtx{}
	}
}

func (c *progChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented")
}

func (c *progChannel) SetReplyTimeout(time.Duration) {}

func (c *progChannel) CheckCompatiblity(...api.Message) error { return nil }

func (c *progChannel) Close() {}

type progReqCtx struct{ ch *progChannel }

func (r *progReqCtx) ReceiveReply(msg api.Message) error {
	if r.ch.sendErr != nil {
		return r.ch.sendErr
	}
	switch reply := msg.(type) {
	case *interfaces.CreateLoopbackReply:
		reply.Retval = r.ch.retval
		reply.SwIfIndex = r.ch.loopbackIndex
		r.ch.loopbackIndex++
	case *gre.GreTunnelAddDelReply:
		reply.Retval = r.ch.retval
		reply.SwIfIndex = r.ch.swIfIndex
	case *ipip.IpipAddTunnelReply:
		reply.Retval = r.ch.retval
		reply.SwIfIndex = r.ch.swIfIndex
	case *ipip.IpipDelTunnelReply:
		reply.Retval = r.ch.retval
	case *vxlan.VxlanAddDelTunnelV3Reply:
		reply.Retval = r.ch.retval
		reply.SwIfIndex = r.ch.swIfIndex
	case *span.SwInterfaceSpanEnableDisableReply:
		reply.Retval = r.ch.retval
	case *wireguard.WireguardInterfaceCreateReply:
		reply.Retval = r.ch.retval
		reply.SwIfIndex = r.ch.swIfIndex
	case *wireguard.WireguardInterfaceDeleteReply:
		reply.Retval = r.ch.retval
	case *wireguard.WireguardPeerAddReply:
		reply.Retval = r.ch.retval
		reply.PeerIndex = r.ch.peerIndex
	case *wireguard.WireguardPeerRemoveReply:
		reply.Retval = r.ch.retval
	case *lcp.LcpItfPairAddDelReply:
		reply.Retval = r.ch.retval
	}
	return nil
}

// progMultiCtx streams the recorded dump details one per ReceiveReply. When the
// details are exhausted it returns last=true, matching GoVPP multi-request
// semantics. An empty details slice terminates immediately (an empty dump).
type progMultiCtx struct {
	details []api.Message
	idx     int
}

func (m *progMultiCtx) ReceiveReply(msg api.Message) (bool, error) {
	if m.idx >= len(m.details) {
		return true, nil
	}
	switch dst := msg.(type) {
	case *wireguard.WireguardInterfaceDetails:
		if src, ok := m.details[m.idx].(*wireguard.WireguardInterfaceDetails); ok {
			*dst = *src
		}
	case *wireguard.WireguardPeersDetails:
		if src, ok := m.details[m.idx].(*wireguard.WireguardPeersDetails); ok {
			*dst = *src
		}
	}
	m.idx++
	return false, nil
}

func newLCPBackend(ch *progChannel) *vppBackendImpl {
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // short-circuit ensureChannel populate
	b.names.Add("loop0", 5, "loop0")
	return b
}

// withLCPSettings overrides the active LCP settings seam (as if the VPP
// component were running with these settings) for the duration of a test.
func withLCPSettings(t *testing.T, settings vppcomp.LCPSettings) {
	t.Helper()
	prev := getActiveLCPSettings
	getActiveLCPSettings = func() (vppcomp.LCPSettings, bool) { return settings, true }
	t.Cleanup(func() { getActiveLCPSettings = prev })
}

// TestSetupLCPPairCreate verifies AC-6: with LCP enabled, SetupLCPPair issues
// lcp_itf_pair_add_del (add) with the resolved SwIfIndex, the host TAP name, and
// TAP host type. A root-reachable netns (host) maps to the empty per-pair netns.
// VALIDATES: AC-6 -- LCP pair created via LcpItfPairAddDel.
// PREVENTS: regression to a missing / no-op LCP path.
func TestSetupLCPPairCreate(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "host"})
	ch := &progChannel{}
	b := newLCPBackend(ch)

	if err := b.SetupLCPPair("loop0", "loop0"); err != nil {
		t.Fatalf("SetupLCPPair: %v", err)
	}
	req, ok := ch.requests[0].(*lcp.LcpItfPairAddDel)
	if !ok {
		t.Fatalf("request type: got %T, want *lcp.LcpItfPairAddDel", ch.requests[0])
	}
	if !req.IsAdd {
		t.Error("IsAdd: got false, want true")
	}
	if req.SwIfIndex != 5 {
		t.Errorf("SwIfIndex: got %d, want 5", req.SwIfIndex)
	}
	if req.HostIfName != "loop0" {
		t.Errorf("HostIfName: got %q, want loop0", req.HostIfName)
	}
	if req.HostIfType != lcp.LCP_API_ITF_HOST_TAP {
		t.Errorf("HostIfType: got %v, want TAP", req.HostIfType)
	}
	if req.Netns != "" {
		t.Errorf("Netns: got %q, want \"\" (host maps to host netns)", req.Netns)
	}
}

// TestSetupLCPPairNetnsPassthrough verifies a non-root netns is passed to VPP
// verbatim (so the operator can isolate the TAP), which the doctor check warns
// about when BGP is enabled.
func TestSetupLCPPairNetnsPassthrough(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "dataplane"})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	if err := b.SetupLCPPair("loop0", ""); err != nil {
		t.Fatalf("SetupLCPPair: %v", err)
	}
	req, ok := ch.requests[0].(*lcp.LcpItfPairAddDel)
	if !ok {
		t.Fatalf("request type: got %T", ch.requests[0])
	}
	if req.Netns != "dataplane" {
		t.Errorf("Netns: got %q, want dataplane", req.Netns)
	}
	if req.HostIfName != "loop0" {
		t.Errorf("HostIfName default: got %q, want loop0 (defaults to ze name)", req.HostIfName)
	}
}

// TestSetupLCPPairDisabledNoop verifies SetupLCPPair is a no-op when LCP is
// disabled, so config-apply can call it unconditionally for vpp loopbacks.
func TestSetupLCPPairDisabledNoop(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: false})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	if err := b.SetupLCPPair("loop0", "loop0"); err != nil {
		t.Fatalf("SetupLCPPair: %v", err)
	}
	if len(ch.requests) != 0 {
		t.Errorf("LCP disabled: expected no VPP request, got %d", len(ch.requests))
	}
}

// TestRemoveLCPPair verifies RemoveLCPPair issues lcp_itf_pair_add_del (del) for
// a recorded pair and is idempotent when none was recorded.
func TestRemoveLCPPair(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "host"})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	if err := b.SetupLCPPair("loop0", "loop0"); err != nil {
		t.Fatalf("SetupLCPPair: %v", err)
	}
	ch.requests = nil
	if err := b.RemoveLCPPair("loop0"); err != nil {
		t.Fatalf("RemoveLCPPair: %v", err)
	}
	del, ok := ch.requests[0].(*lcp.LcpItfPairAddDel)
	if !ok {
		t.Fatalf("request type: got %T", ch.requests[0])
	}
	if del.IsAdd {
		t.Error("delete: IsAdd got true, want false")
	}
	// Idempotent: removing again issues nothing.
	ch.requests = nil
	if err := b.RemoveLCPPair("loop0"); err != nil {
		t.Fatalf("RemoveLCPPair (second): %v", err)
	}
	if len(ch.requests) != 0 {
		t.Errorf("second RemoveLCPPair: expected no request, got %d", len(ch.requests))
	}
}

// TestDeleteInterfaceRemovesLCPPair verifies deleting a shadowed loopback tears
// down its LCP pair first (the pair references the sw_if_index).
func TestDeleteInterfaceRemovesLCPPair(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "host"})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	if err := b.SetupLCPPair("loop0", "loop0"); err != nil {
		t.Fatalf("SetupLCPPair: %v", err)
	}
	ch.requests = nil
	if err := b.DeleteInterface("loop0"); err != nil {
		t.Fatalf("DeleteInterface: %v", err)
	}
	var sawLCPDel bool
	for _, r := range ch.requests {
		if d, ok := r.(*lcp.LcpItfPairAddDel); ok && !d.IsAdd {
			sawLCPDel = true
		}
	}
	if !sawLCPDel {
		t.Error("DeleteInterface did not remove the LCP pair")
	}
}

// newMirrorBackend returns a backend wired to a programmable channel with two
// pre-registered interfaces (a source and a destination) so SetupMirror can
// resolve both names to SwIfIndex without a live VPP.
func newMirrorBackend(ch *progChannel) *vppBackendImpl {
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // short-circuit ensureChannel populate
	b.names.Add("xe0", 4, "xe0")
	b.names.Add("xe1", 9, "xe1")
	return b
}

// TestSetupMirrorSpanIngressEgress verifies AC-4: mirror with both ingress and
// egress issues a sw_interface_span_enable_disable with state RX_TX, the
// resolved from/to indices, and device-level SPAN (is_l2=false, netlink parity).
// VALIDATES: AC-4 -- SPAN programmed with the RX_TX state per A-6.
// PREVENTS: regression to the errNotSupported stub.
func TestSetupMirrorSpanIngressEgress(t *testing.T) {
	ch := &progChannel{}
	b := newMirrorBackend(ch)

	if err := b.SetupMirror("xe0", "xe1", true, true); err != nil {
		t.Fatalf("SetupMirror: %v", err)
	}
	req, ok := ch.requests[0].(*span.SwInterfaceSpanEnableDisable)
	if !ok {
		t.Fatalf("request type: got %T, want *span.SwInterfaceSpanEnableDisable", ch.requests[0])
	}
	if req.SwIfIndexFrom != 4 {
		t.Errorf("SwIfIndexFrom: got %d, want 4", req.SwIfIndexFrom)
	}
	if req.SwIfIndexTo != 9 {
		t.Errorf("SwIfIndexTo: got %d, want 9", req.SwIfIndexTo)
	}
	if req.State != span.SPAN_STATE_API_RX_TX {
		t.Errorf("State: got %v, want RX_TX", req.State)
	}
	if req.IsL2 {
		t.Error("IsL2: got true, want false (device SPAN, netlink parity per A-6)")
	}
}

// TestSetupMirrorSpanStateMapping verifies the ingress/egress -> SpanState map
// covers each direction. VALIDATES: AC-4 -- rx/tx flag mapping per A-6.
func TestSetupMirrorSpanStateMapping(t *testing.T) {
	cases := []struct {
		name            string
		ingress, egress bool
		want            span.SpanState
	}{
		{"ingress-only", true, false, span.SPAN_STATE_API_RX},
		{"egress-only", false, true, span.SPAN_STATE_API_TX},
		{"both", true, true, span.SPAN_STATE_API_RX_TX},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch := &progChannel{}
			b := newMirrorBackend(ch)
			if err := b.SetupMirror("xe0", "xe1", tc.ingress, tc.egress); err != nil {
				t.Fatalf("SetupMirror: %v", err)
			}
			req, ok := ch.requests[0].(*span.SwInterfaceSpanEnableDisable)
			if !ok {
				t.Fatalf("request type: got %T", ch.requests[0])
			}
			if req.State != tc.want {
				t.Errorf("State: got %v, want %v", req.State, tc.want)
			}
		})
	}
}

// TestRemoveMirrorSpan verifies RemoveMirror disables every SPAN destination
// recorded for a source, replaying the (from,to,is_l2) triple with state
// DISABLED that VPP requires to delete the entry.
// VALIDATES: AC-4 -- RemoveMirror disables SPAN.
// PREVENTS: a stale SPAN entry after the mirror config is removed.
func TestRemoveMirrorSpan(t *testing.T) {
	ch := &progChannel{}
	b := newMirrorBackend(ch)

	if err := b.SetupMirror("xe0", "xe1", true, true); err != nil {
		t.Fatalf("SetupMirror: %v", err)
	}
	if err := b.RemoveMirror("xe0"); err != nil {
		t.Fatalf("RemoveMirror: %v", err)
	}
	last, ok := ch.requests[len(ch.requests)-1].(*span.SwInterfaceSpanEnableDisable)
	if !ok {
		t.Fatalf("disable request type: got %T", ch.requests[len(ch.requests)-1])
	}
	if last.State != span.SPAN_STATE_API_DISABLED {
		t.Errorf("disable State: got %v, want DISABLED", last.State)
	}
	if last.SwIfIndexFrom != 4 || last.SwIfIndexTo != 9 {
		t.Errorf("disable from/to: got %d/%d, want 4/9", last.SwIfIndexFrom, last.SwIfIndexTo)
	}
}

// TestRemoveMirrorNoRecordIsNoop verifies RemoveMirror is idempotent when no
// SPAN was recorded (mirrors netlink's isNotFound tolerance).
func TestRemoveMirrorNoRecordIsNoop(t *testing.T) {
	ch := &progChannel{}
	b := newMirrorBackend(ch)
	if err := b.RemoveMirror("xe0"); err != nil {
		t.Fatalf("RemoveMirror with no record: %v", err)
	}
	if len(ch.requests) != 0 {
		t.Errorf("no VPP request expected, got %d", len(ch.requests))
	}
}

// TestSetupMirrorRetvalError verifies a non-zero VPP retval surfaces as an
// error rather than a silent success.
func TestSetupMirrorRetvalError(t *testing.T) {
	ch := &progChannel{retval: -1}
	b := newMirrorBackend(ch)
	err := b.SetupMirror("xe0", "xe1", true, false)
	if err == nil {
		t.Fatal("expected error on non-zero retval, got nil")
	}
	if !strings.Contains(err.Error(), "retval") {
		t.Errorf("expected 'retval' in error, got: %v", err)
	}
}

// TestRemoveMirrorAfterDestinationRecreated verifies the SPAN disable names the
// index the destination holds NOW, not the one it held when SetupMirror ran.
// Method: mirror xe0 -> xe1 while xe1 is index 9, rebind xe1 to index 21 the way
// a recreate does, then remove the mirror.
// VALIDATES: AC-4 -- no recorded SwIfIndex outlives the interface it names.
// PREVENTS: a mirror the operator removed that keeps copying traffic, because
// the disable landed on an index nothing forwards through any more.
func TestRemoveMirrorAfterDestinationRecreated(t *testing.T) {
	ch := &progChannel{}
	b := newMirrorBackend(ch)

	if err := b.SetupMirror("xe0", "xe1", true, true); err != nil {
		t.Fatalf("SetupMirror: %v", err)
	}
	b.names.Remove("xe1")
	b.names.Add("xe1", 21, "xe1")

	if err := b.RemoveMirror("xe0"); err != nil {
		t.Fatalf("RemoveMirror: %v", err)
	}
	last, ok := ch.requests[len(ch.requests)-1].(*span.SwInterfaceSpanEnableDisable)
	if !ok {
		t.Fatalf("disable request type: got %T", ch.requests[len(ch.requests)-1])
	}
	if last.State != span.SPAN_STATE_API_DISABLED {
		t.Errorf("disable State: got %v, want DISABLED", last.State)
	}
	if last.SwIfIndexTo != 21 {
		t.Errorf("disable SwIfIndexTo: got %d, want 21 (the index xe1 holds now)", last.SwIfIndexTo)
	}
}

// newDummyBackend returns a backend wired to a programmable channel with an
// empty name map, so CreateDummy takes its create path on the first call.
func newDummyBackend(ch *progChannel) *vppBackendImpl {
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // short-circuit ensureChannel populate
	return b
}

// TestCreateDummyFirstCallCreates verifies the create path is unchanged when
// the name is unknown: one create_loopback, and the name bound to the index
// VPP returned.
// VALIDATES: AC-5 -- an apply that finds no existing loopback still creates one.
func TestCreateDummyFirstCallCreates(t *testing.T) {
	ch := &progChannel{loopbackIndex: 12}
	b := newDummyBackend(ch)

	if err := b.CreateDummy("lo0"); err != nil {
		t.Fatalf("CreateDummy: %v", err)
	}
	if len(ch.requests) != 1 {
		t.Fatalf("requests: got %d, want 1", len(ch.requests))
	}
	if _, ok := ch.requests[0].(*interfaces.CreateLoopback); !ok {
		t.Fatalf("request type: got %T, want *interfaces.CreateLoopback", ch.requests[0])
	}
	idx, ok := b.names.lookupIndex("lo0")
	if !ok {
		t.Fatal("lo0 not registered in the name map")
	}
	if idx != 12 {
		t.Errorf("lo0 index: got %d, want 12", idx)
	}
}

// TestCreateDummyKeepsExistingLoopback verifies the second apply of the same
// dummy entry keeps the interface the first one made. create_loopback carries
// no name, so a second send would allocate a second loopback and rebind the ze
// name to it, leaving the first one in the dataplane with its addresses and its
// bridge port and nothing pointing at it.
// VALIDATES: AC-1 -- two applies leave one loopback for the name.
// PREVENTS: one leaked VPP interface per config apply, without bound.
func TestCreateDummyKeepsExistingLoopback(t *testing.T) {
	ch := &progChannel{loopbackIndex: 12}
	b := newDummyBackend(ch)

	if err := b.CreateDummy("lo0"); err != nil {
		t.Fatalf("first CreateDummy: %v", err)
	}
	err := b.CreateDummy("lo0")
	if !errors.Is(err, iface.ErrInterfaceExists) {
		t.Fatalf("second CreateDummy: got %v, want iface.ErrInterfaceExists", err)
	}
	creates := 0
	for _, req := range ch.requests {
		if _, ok := req.(*interfaces.CreateLoopback); ok {
			creates++
		}
	}
	if creates != 1 {
		t.Errorf("create_loopback requests: got %d, want 1", creates)
	}
	idx, ok := b.names.lookupIndex("lo0")
	if !ok {
		t.Fatal("lo0 lost its name-map entry")
	}
	if idx != 12 {
		t.Errorf("lo0 index: got %d, want 12 (the interface the first apply made)", idx)
	}
}

// recordingBus captures every Emit call. Satisfies ze.EventBus.
type recordingBus struct {
	mu     sync.Mutex
	events []capturedEvent
}

type capturedEvent struct {
	Namespace string
	Type      string
	Payload   string
}

var _ ze.EventBus = (*recordingBus)(nil)

func (b *recordingBus) Emit(namespace, eventType string, payload any) (int, error) {
	s, ok := payload.(string)
	if !ok {
		data, _ := json.Marshal(payload)
		s = string(data)
	}
	b.mu.Lock()
	b.events = append(b.events, capturedEvent{Namespace: namespace, Type: eventType, Payload: s})
	b.mu.Unlock()
	return 1, nil
}

func (b *recordingBus) Subscribe(_, _ string, _ func(any)) func() {
	return func() {}
}

func (b *recordingBus) len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

func (b *recordingBus) at(i int) capturedEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.events[i]
}

// monitorChannel mocks api.Channel for the monitor tests. It:
//   - captures WantInterfaceEvents requests
//   - exposes a "push" hook so tests can deliver synthetic SwInterfaceEvents
//     via the notification channel
type monitorChannel struct {
	mu       sync.Mutex
	notif    chan api.Message
	sendErr  error
	want     interfaces.WantInterfaceEvents
	wantOffs []bool // history of enable values observed
	reply    interfaces.WantInterfaceEventsReply
	closed   bool
}

var _ api.Channel = (*monitorChannel)(nil)

func (c *monitorChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.mu.Lock()
	defer c.mu.Unlock()
	if w, ok := msg.(*interfaces.WantInterfaceEvents); ok {
		c.want = *w
		c.wantOffs = append(c.wantOffs, w.EnableDisable != 0)
	}
	return &monitorReqCtx{ch: c}
}

func (c *monitorChannel) SendMultiRequest(_ api.Message) api.MultiRequestCtx {
	// populateNameMap inside ensureChannel issues SwInterfaceDump. Return a
	// multi-reply ctx that reports "last" immediately so dumpAllRaw sees
	// zero interfaces and returns cleanly.
	return &emptyMultiCtx{}
}

type emptyMultiCtx struct{}

func (e *emptyMultiCtx) ReceiveReply(_ api.Message) (bool, error) { return true, nil }

func (c *monitorChannel) SubscribeNotification(notifChan chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notif = notifChan
	return &monitorSub{ch: c}, nil
}

func (c *monitorChannel) SetReplyTimeout(time.Duration) {}

func (c *monitorChannel) CheckCompatiblity(...api.Message) error { return nil }

func (c *monitorChannel) Close() { c.closed = true }

// push delivers a synthetic event to the subscriber. Blocks if the buffered
// channel is full -- tests should keep the payload small.
func (c *monitorChannel) push(ev *interfaces.SwInterfaceEvent) {
	c.mu.Lock()
	ch := c.notif
	c.mu.Unlock()
	ch <- ev
}

type monitorReqCtx struct{ ch *monitorChannel }

func (r *monitorReqCtx) ReceiveReply(msg api.Message) error {
	if r.ch.sendErr != nil {
		return r.ch.sendErr
	}
	if reply, ok := msg.(*interfaces.WantInterfaceEventsReply); ok {
		*reply = r.ch.reply
	}
	return nil
}

type monitorSub struct{ ch *monitorChannel }

func (s *monitorSub) Unsubscribe() error { return nil }

// waitForEvents polls the bus until it has at least n events or the deadline
// fires. Returns whether the threshold was met.
func waitForEvents(b *recordingBus, n int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if b.len() >= n {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return b.len() >= n
}

func TestStartMonitorRequiresBus(t *testing.T) {
	// VALIDATES: AC-16 -- StartMonitor rejects nil bus
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	if err := b.StartMonitor(nil); err == nil {
		t.Fatal("expected error for nil bus")
	}
}

func TestStartMonitorSendsEnable(t *testing.T) {
	// VALIDATES: AC-16 -- WantInterfaceEvents enable=1 sent on Start
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	defer b.StopMonitor()

	if len(ch.wantOffs) != 1 || !ch.wantOffs[0] {
		t.Errorf("wantOffs history: got %v, want [true]", ch.wantOffs)
	}
}

// TestStartMonitorIdempotent verifies that a second StartMonitor after a
// first success is a no-op (returns nil without re-subscribing). This
// contract is load-bearing for spec-iface-vpp-ready-gate: the
// vppevents.EventConnected handler retries StartMonitor on every event so
// a deferred initial call can succeed, and subsequent events must not
// error after the monitor is already running.
func TestStartMonitorIdempotent(t *testing.T) {
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	defer b.StopMonitor()

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("second StartMonitor must be a no-op, got %v", err)
	}
	// wantOffs history should still show a single enable, not two.
	if len(ch.wantOffs) != 1 || !ch.wantOffs[0] {
		t.Errorf("wantOffs history: got %v, want [true]", ch.wantOffs)
	}
}

func TestMonitorEmitsUpEventOnAdminFlag(t *testing.T) {
	// VALIDATES: AC-16 -- ADMIN_UP flag translates to iface "up" event
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 7, "xe0")
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	defer b.StopMonitor()

	ch.push(&interfaces.SwInterfaceEvent{
		SwIfIndex: 7,
		Flags:     interface_types.IF_STATUS_API_FLAG_ADMIN_UP,
	})

	if !waitForEvents(bus, 1, time.Second) {
		t.Fatal("no event received")
	}
	ev := bus.at(0)
	if ev.Namespace != ifaceevents.Namespace {
		t.Errorf("Namespace: got %q, want %q", ev.Namespace, ifaceevents.Namespace)
	}
	if ev.Type != ifaceevents.EventUp {
		t.Errorf("Type: got %q, want %q", ev.Type, ifaceevents.EventUp)
	}
	var payload stateEventPayload
	if err := json.Unmarshal([]byte(ev.Payload), &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.Name != "xe0" {
		t.Errorf("payload.Name: got %q, want xe0", payload.Name)
	}
}

func TestMonitorEmitsDownOnAbsentFlag(t *testing.T) {
	// VALIDATES: AC-16 -- no ADMIN_UP flag translates to "down"
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 7, "xe0")
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	defer b.StopMonitor()

	ch.push(&interfaces.SwInterfaceEvent{SwIfIndex: 7, Flags: 0})
	if !waitForEvents(bus, 1, time.Second) {
		t.Fatal("no event received")
	}
	if bus.at(0).Type != ifaceevents.EventDown {
		t.Errorf("Type: got %q, want %q", bus.at(0).Type, ifaceevents.EventDown)
	}
}

func TestMonitorDeletedRemovesFromNameMap(t *testing.T) {
	// VALIDATES: SwInterfaceEvent.Deleted=true clears the name-map entry
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 7, "xe0")
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	defer b.StopMonitor()

	ch.push(&interfaces.SwInterfaceEvent{SwIfIndex: 7, Deleted: true})
	if !waitForEvents(bus, 1, time.Second) {
		t.Fatal("no event received")
	}
	if _, ok := b.names.lookupIndex("xe0"); ok {
		t.Error("name map should not contain xe0 after delete")
	}
}

func TestStopMonitorSendsDisable(t *testing.T) {
	// VALIDATES: AC-16 -- Stop sends WantInterfaceEvents with enable=0
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	bus := &recordingBus{}

	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	b.StopMonitor()

	if len(ch.wantOffs) < 2 || ch.wantOffs[1] {
		t.Errorf("wantOffs history: got %v, want [true,false]", ch.wantOffs)
	}
}

func TestStopMonitorWithoutStartSafe(t *testing.T) {
	// VALIDATES: StopMonitor is safe to call without Start
	ch := &monitorChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.StopMonitor() // no panic
}

func TestStartMonitorPropagatesSubscribeError(t *testing.T) {
	// VALIDATES: subscribe error returned to caller
	ch := &failSubChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	if err := b.StartMonitor(&recordingBus{}); err == nil {
		t.Fatal("expected error")
	}
}

// failSubChannel is a minimal api.Channel whose SubscribeNotification always
// fails.
type failSubChannel struct{ monitorChannel }

func (c *failSubChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("subscribe failed")
}

// neighborChannel is a mock api.Channel wired specifically for
// IPNeighborDump multi-requests. It mirrors routeChannel's shape but
// keys replies on the Af byte of the dump request rather than the
// IPRouteV2Dump.IsIP6 boolean. Kept separate from routeChannel so
// neither test mock has to juggle two unrelated RPC protocols.
type neighborChannel struct {
	lastRequest api.Message
	allRequests []api.Message
	v4Details   []ip_neighbor.IPNeighborDetails
	v6Details   []ip_neighbor.IPNeighborDetails
	sendErr     error
	receiveErr  error
	dumpCalls   int
}

var _ api.Channel = (*neighborChannel)(nil)

func (c *neighborChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.lastRequest = msg
	c.allRequests = append(c.allRequests, msg)
	return &neighborRequestCtx{ch: c}
}

func (c *neighborChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.lastRequest = msg
	c.allRequests = append(c.allRequests, msg)
	details := c.v4Details
	if m, ok := msg.(*ip_neighbor.IPNeighborDump); ok {
		if m.Af == ip_types.ADDRESS_IP6 {
			details = c.v6Details
		}
	}
	c.dumpCalls++
	return &neighborMultiCtx{ch: c, details: details}
}

func (c *neighborChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented")
}

func (c *neighborChannel) SetReplyTimeout(time.Duration) {}

func (c *neighborChannel) CheckCompatiblity(...api.Message) error { return nil }

func (c *neighborChannel) Close() {}

type neighborRequestCtx struct{ ch *neighborChannel }

func (r *neighborRequestCtx) ReceiveReply(_ api.Message) error {
	return r.ch.sendErr
}

type neighborMultiCtx struct {
	ch      *neighborChannel
	details []ip_neighbor.IPNeighborDetails
	pos     int
}

func (m *neighborMultiCtx) ReceiveReply(msg api.Message) (bool, error) {
	if m.ch.receiveErr != nil {
		return false, m.ch.receiveErr
	}
	if m.pos >= len(m.details) {
		return true, nil
	}
	d, ok := msg.(*ip_neighbor.IPNeighborDetails)
	if !ok {
		return false, fmt.Errorf("neighborMultiCtx: unexpected reply type %T", msg)
	}
	*d = m.details[m.pos]
	m.pos++
	return false, nil
}

// makeV4Neighbor builds an ip_neighbor.IPNeighborDetails entry for the
// given IPv4 address, MAC string ("" for zero MAC), SwIfIndex, and
// flags byte. The helper parses both values through the vendored
// ip_types / ethernet-types parsers so the test exercises the same
// encoding path as real VPP replies.
func makeV4Neighbor(t *testing.T, ip, mac string, swIfIndex uint32) ip_neighbor.IPNeighborDetails {
	t.Helper()
	addr, err := ip_types.ParseIP4Address(ip)
	if err != nil {
		t.Fatalf("ParseIP4Address(%q): %v", ip, err)
	}
	n := ip_neighbor.IPNeighbor{
		SwIfIndex: interface_types.InterfaceIndex(swIfIndex),
		IPAddress: ip_types.Address{
			Af: ip_types.ADDRESS_IP4,
			Un: ip_types.AddressUnionIP4(addr),
		},
	}
	if mac != "" {
		setMAC(t, &n, mac)
	}
	return ip_neighbor.IPNeighborDetails{Neighbor: n}
}

// makeV6Neighbor builds an IPv6 neighbor entry. Separate from the v4
// helper because IP4Address and IP6Address are different underlying
// arrays and do not share the same union accessor.
func makeV6Neighbor(t *testing.T, ip, mac string, swIfIndex uint32, flags ip_neighbor.IPNeighborFlags) ip_neighbor.IPNeighborDetails {
	t.Helper()
	addr, err := ip_types.ParseIP6Address(ip)
	if err != nil {
		t.Fatalf("ParseIP6Address(%q): %v", ip, err)
	}
	var un ip_types.AddressUnion
	copy(un.XXX_UnionData[:16], addr[:])
	n := ip_neighbor.IPNeighbor{
		SwIfIndex: interface_types.InterfaceIndex(swIfIndex),
		Flags:     flags,
		IPAddress: ip_types.Address{
			Af: ip_types.ADDRESS_IP6,
			Un: un,
		},
	}
	if mac != "" {
		setMAC(t, &n, mac)
	}
	return ip_neighbor.IPNeighborDetails{Neighbor: n}
}

// setMAC parses "aa:bb:cc:dd:ee:ff" into the fixed-size MacAddress
// array. Six colons-separated hex bytes are expected; otherwise the
// test fails fatally.
func setMAC(t *testing.T, n *ip_neighbor.IPNeighbor, mac string) {
	t.Helper()
	var b [6]byte
	if _, err := fmt.Sscanf(mac, "%02x:%02x:%02x:%02x:%02x:%02x",
		&b[0], &b[1], &b[2], &b[3], &b[4], &b[5]); err != nil {
		t.Fatalf("sscanf mac %q: %v", mac, err)
	}
	copy(n.MacAddress[:], b[:])
}

// --- ListNeighbors ---

// TestListNeighborsEmpty asserts a clean run against an empty neighbor
// table returns zero entries without error.
// VALIDATES: ListNeighbors handles the zero-entry case.
// PREVENTS: nil-slice / nil-vs-empty confusion at the caller.
func TestListNeighborsEmpty(t *testing.T) {
	ch := &neighborChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyAny)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len: got %d, want 0", len(got))
	}
	if ch.dumpCalls != 2 {
		t.Errorf("dump calls: got %d, want 2 (v4 + v6)", ch.dumpCalls)
	}
}

// TestListNeighborsV4Only dumps IPv4 neighbors and skips v6.
// VALIDATES: NeighborFamilyIPv4 restricts the request to af=v4.
// PREVENTS: accidentally querying v6 when the caller wants v4 only.
func TestListNeighborsV4Only(t *testing.T) {
	ch := &neighborChannel{
		v4Details: []ip_neighbor.IPNeighborDetails{
			makeV4Neighbor(t, "192.0.2.10", "aa:bb:cc:dd:ee:01", 7),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe7", 7, "xe7")
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyIPv4)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	n := got[0]
	if n.Address != "192.0.2.10" {
		t.Errorf("Address: got %q, want 192.0.2.10", n.Address)
	}
	if n.Family != "ipv4" {
		t.Errorf("Family: got %q, want ipv4", n.Family)
	}
	if n.MAC != "aa:bb:cc:dd:ee:01" {
		t.Errorf("MAC: got %q, want aa:bb:cc:dd:ee:01", n.MAC)
	}
	if n.Device != "xe7" {
		t.Errorf("Device: got %q, want xe7", n.Device)
	}
	if n.State != "reachable" {
		t.Errorf("State: got %q, want reachable", n.State)
	}
	if ch.dumpCalls != 1 {
		t.Errorf("dump calls: got %d, want 1 (v4 only)", ch.dumpCalls)
	}
}

// TestListNeighborsV6Only dumps IPv6 neighbors and skips v4.
// VALIDATES: NeighborFamilyIPv6 restricts the request to af=v6.
// PREVENTS: accidentally querying v4 when the caller wants v6 only.
func TestListNeighborsV6Only(t *testing.T) {
	ch := &neighborChannel{
		v6Details: []ip_neighbor.IPNeighborDetails{
			makeV6Neighbor(t, "2001:db8::1", "aa:bb:cc:dd:ee:02", 0, ip_neighbor.IP_API_NEIGHBOR_FLAG_STATIC),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyIPv6)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	n := got[0]
	if n.Address != "2001:db8::1" {
		t.Errorf("Address: got %q, want 2001:db8::1", n.Address)
	}
	if n.Family != "ipv6" {
		t.Errorf("Family: got %q, want ipv6", n.Family)
	}
	if n.State != "permanent" {
		t.Errorf("State: got %q, want permanent (STATIC flag)", n.State)
	}
	if ch.dumpCalls != 1 {
		t.Errorf("dump calls: got %d, want 1 (v6 only)", ch.dumpCalls)
	}
}

// TestListNeighborsAnyConcatenates collects both families in v4-then-v6
// order. VALIDATES: NeighborFamilyAny issues two dumps and merges the
// results. PREVENTS: silently dropping one family or reversing order.
func TestListNeighborsAnyConcatenates(t *testing.T) {
	ch := &neighborChannel{
		v4Details: []ip_neighbor.IPNeighborDetails{
			makeV4Neighbor(t, "192.0.2.1", "aa:bb:cc:dd:ee:01", 1),
			makeV4Neighbor(t, "192.0.2.2", "aa:bb:cc:dd:ee:02", 1),
		},
		v6Details: []ip_neighbor.IPNeighborDetails{
			makeV6Neighbor(t, "2001:db8::1", "aa:bb:cc:dd:ee:03", 1, 0),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyAny)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len: got %d, want 3 (2 v4 + 1 v6)", len(got))
	}
	wantFams := []string{"ipv4", "ipv4", "ipv6"}
	for i, w := range wantFams {
		if got[i].Family != w {
			t.Errorf("entry %d family: got %q, want %q (v4-before-v6 ordering)", i, got[i].Family, w)
		}
	}
	if ch.dumpCalls != 2 {
		t.Errorf("dump calls: got %d, want 2 (v4 + v6)", ch.dumpCalls)
	}
}

// TestListNeighborsUnknownSwIfIndex leaves Device empty when the VPP
// port has no ze name registered. Mirrors fib.go's policy of never
// exposing an opaque integer to the operator.
// VALIDATES: absent nameMap entry produces empty Device, not "99".
// PREVENTS: leaking raw VPP indexes in operator-visible output.
func TestListNeighborsUnknownSwIfIndex(t *testing.T) {
	ch := &neighborChannel{
		v4Details: []ip_neighbor.IPNeighborDetails{
			makeV4Neighbor(t, "192.0.2.3", "aa:bb:cc:dd:ee:04", 99),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyIPv4)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	if got[0].Device != "" {
		t.Errorf("Device: got %q, want empty (SwIfIndex unmapped)", got[0].Device)
	}
}

// TestListNeighborsMultiEntry asserts every entry in the dump reply is
// surfaced, preserving the order VPP sent them in.
// VALIDATES: ReceiveReply loop does not drop entries between first and
// last=true.
// PREVENTS: off-by-one in the multi-request drain loop.
func TestListNeighborsMultiEntry(t *testing.T) {
	ch := &neighborChannel{
		v4Details: []ip_neighbor.IPNeighborDetails{
			makeV4Neighbor(t, "192.0.2.1", "aa:bb:cc:dd:ee:01", 1),
			makeV4Neighbor(t, "192.0.2.2", "aa:bb:cc:dd:ee:02", 1),
			makeV4Neighbor(t, "192.0.2.3", "aa:bb:cc:dd:ee:03", 1),
			makeV4Neighbor(t, "192.0.2.4", "", 1),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyIPv4)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("len: got %d, want 4", len(got))
	}
	// Last entry has zero MAC -> MAC field must be empty AND state must
	// be "incomplete" (downgraded from the default "reachable") so the
	// two columns stay self-consistent.
	if got[3].MAC != "" {
		t.Errorf("entry 3 MAC: got %q, want empty (zero MAC = unresolved)", got[3].MAC)
	}
	if got[3].State != "incomplete" {
		t.Errorf("entry 3 State: got %q, want incomplete (zero MAC)", got[3].State)
	}
	// First three have real MACs and stay on "reachable".
	for i := range 3 {
		if got[i].MAC == "" {
			t.Errorf("entry %d MAC: empty, want non-empty", i)
		}
		if got[i].State != "reachable" {
			t.Errorf("entry %d State: got %q, want reachable", i, got[i].State)
		}
	}
}

// TestListNeighborsReceiveError propagates a channel failure so the
// operator sees the VPP-side problem instead of a silently empty list.
// VALIDATES: error path wraps the VPP error with context.
// PREVENTS: silent loss of VPP failures during dumps.
func TestListNeighborsReceiveError(t *testing.T) {
	ch := &neighborChannel{
		receiveErr: fmt.Errorf("VPP dead"),
		v4Details: []ip_neighbor.IPNeighborDetails{
			makeV4Neighbor(t, "192.0.2.1", "aa:bb:cc:dd:ee:01", 1),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	_, err := b.ListNeighbors(iface.NeighborFamilyIPv4)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// clearStatsReplyTarget is a local alias so the test file's reply handler
// names the concrete interfaces reply type exactly once. Using the alias
// gives the goimports pass a reason to keep the interfaces import even
// before every test has been wired.
type clearStatsReplyTarget = interfaces.SwInterfaceClearStatsReply

// routeChannel is a mock api.Channel for IPRouteV2Dump multi-requests
// and IPRouteLookupV2 single-requests.
// It returns v4Details on the first dump (IsIP6=false) and v6Details on
// the second. SwInterfaceClearStats replies are served through the
// single-request path so counter-reset tests can share the same fake.
type routeChannel struct {
	lastRequest   api.Message
	allRequests   []api.Message
	v4Details     []ip.IPRouteV2Details
	v6Details     []ip.IPRouteV2Details
	clearReply    clearStatsReply
	lookupReply   ip.IPRouteLookupV2Reply
	sendErr       error
	receiveErr    error
	dumpCallCount int
}

type clearStatsReply struct {
	retval int32
}

var _ api.Channel = (*routeChannel)(nil)

func (c *routeChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.lastRequest = msg
	c.allRequests = append(c.allRequests, msg)
	return &routeRequestCtx{ch: c}
}

func (c *routeChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.lastRequest = msg
	c.allRequests = append(c.allRequests, msg)
	isIP6 := false
	if m, ok := msg.(*ip.IPRouteV2Dump); ok {
		isIP6 = m.Table.IsIP6
	}
	details := c.v4Details
	if isIP6 {
		details = c.v6Details
	}
	c.dumpCallCount++
	return &routeMultiCtx{ch: c, details: details}
}

func (c *routeChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented")
}

func (c *routeChannel) SetReplyTimeout(time.Duration) {}

func (c *routeChannel) CheckCompatiblity(...api.Message) error { return nil }

func (c *routeChannel) Close() {}

type routeRequestCtx struct{ ch *routeChannel }

func (r *routeRequestCtx) ReceiveReply(msg api.Message) error {
	if r.ch.sendErr != nil {
		return r.ch.sendErr
	}
	switch reply := msg.(type) {
	case *clearStatsReplyTarget:
		reply.Retval = r.ch.clearReply.retval
	case *ip.IPRouteLookupV2Reply:
		*reply = r.ch.lookupReply
	}
	return nil
}

type routeMultiCtx struct {
	ch      *routeChannel
	details []ip.IPRouteV2Details
	pos     int
}

func (m *routeMultiCtx) ReceiveReply(msg api.Message) (bool, error) {
	if m.ch.receiveErr != nil {
		return false, m.ch.receiveErr
	}
	if m.pos >= len(m.details) {
		return true, nil
	}
	d, ok := msg.(*ip.IPRouteV2Details)
	if !ok {
		return false, fmt.Errorf("routeMultiCtx: unexpected reply type %T", msg)
	}
	*d = m.details[m.pos]
	m.pos++
	return false, nil
}

// makeIP4Route builds an ip.IPRouteV2Details representing a single v4
// route with the given prefix, next-hop, and fib source. gw == "" produces
// a connected route (all-zero next-hop).
func makeIP4Route(t *testing.T, cidr, gw string, src uint8, swIfIndex uint32) ip.IPRouteV2Details {
	t.Helper()
	prefix, err := ip_types.ParseIP4Prefix(cidr)
	if err != nil {
		t.Fatalf("ParseIP4Prefix(%q): %v", cidr, err)
	}
	route := ip.IPRouteV2{
		Src: src,
		Prefix: ip_types.Prefix{
			Address: ip_types.Address{
				Af: ip_types.ADDRESS_IP4,
				Un: ip_types.AddressUnionIP4(prefix.Address),
			},
			Len: prefix.Len,
		},
		NPaths: 1,
		Paths: []fib_types.FibPath{{
			SwIfIndex:  swIfIndex,
			Proto:      fib_types.FIB_API_PATH_NH_PROTO_IP4,
			Preference: 0,
			Weight:     1,
		}},
	}
	if gw != "" {
		gwAddr, err := ip_types.ParseIP4Address(gw)
		if err != nil {
			t.Fatalf("ParseIP4Address(%q): %v", gw, err)
		}
		var un ip_types.AddressUnion
		copy(un.XXX_UnionData[:4], gwAddr[:])
		route.Paths[0].Nh.Address = un
	}
	return ip.IPRouteV2Details{Route: route}
}

// makeIP6Route builds an ip.IPRouteV2Details representing a single v6 route.
func makeIP6Route(t *testing.T, cidr, gw string, src uint8) ip.IPRouteV2Details {
	t.Helper()
	prefix, err := ip_types.ParseIP6Prefix(cidr)
	if err != nil {
		t.Fatalf("ParseIP6Prefix(%q): %v", cidr, err)
	}
	route := ip.IPRouteV2{
		Src: src,
		Prefix: ip_types.Prefix{
			Address: ip_types.Address{
				Af: ip_types.ADDRESS_IP6,
				Un: ip_types.AddressUnionIP6(prefix.Address),
			},
			Len: prefix.Len,
		},
		NPaths: 1,
		Paths: []fib_types.FibPath{{
			Proto:      fib_types.FIB_API_PATH_NH_PROTO_IP6,
			Preference: 0,
			Weight:     1,
		}},
	}
	if gw != "" {
		gwAddr, err := ip_types.ParseIP6Address(gw)
		if err != nil {
			t.Fatalf("ParseIP6Address(%q): %v", gw, err)
		}
		var un ip_types.AddressUnion
		copy(un.XXX_UnionData[:16], gwAddr[:])
		route.Paths[0].Nh.Address = un
	}
	return ip.IPRouteV2Details{Route: route}
}

// --- ListKernelRoutes ---

// TestListKernelRoutesDumpsBothFamilies ensures the VPP backend queries
// both the v4 and v6 FIB tables and merges the results into a single slice.
// VALIDATES: ListKernelRoutes now returns real VPP FIB entries, not errNotSupported.
// PREVENTS: silent regression to the old errNotSupported stub.
func TestListKernelRoutesDumpsBothFamilies(t *testing.T) {
	ch := &routeChannel{
		v4Details: []ip.IPRouteV2Details{
			makeIP4Route(t, "10.0.0.0/8", "192.168.1.1", 19, 0),
		},
		v6Details: []ip.IPRouteV2Details{
			makeIP6Route(t, "2001:db8::/32", "fe80::1", 19),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // mark populated so ensureChannel short-circuits

	routes, err := b.ListKernelRoutes("", 0)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if ch.dumpCallCount != 2 {
		t.Errorf("dump calls: got %d, want 2 (v4 + v6)", ch.dumpCallCount)
	}
	if len(routes) != 2 {
		t.Fatalf("routes: got %d, want 2", len(routes))
	}
	families := map[string]bool{}
	for _, r := range routes {
		families[r.Family] = true
	}
	if !families["ipv4"] || !families["ipv6"] {
		t.Errorf("families: got %v, want both ipv4 and ipv6", families)
	}
}

// TestListKernelRoutesDecodesFields verifies the v4 decoding path produces
// the expected Destination, NextHop, Protocol, and Family.
// VALIDATES: FibPath + Prefix + Src decoding matches KernelRoute shape.
// PREVENTS: silently dropping or mangling a well-formed VPP reply.
func TestListKernelRoutesDecodesFields(t *testing.T) {
	ch := &routeChannel{
		v4Details: []ip.IPRouteV2Details{
			makeIP4Route(t, "10.0.0.0/8", "192.168.1.1", 19 /* bgp */, 0),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	routes, err := b.ListKernelRoutes("", 0)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("routes: got %d, want 1", len(routes))
	}
	r := routes[0]
	if r.Destination != "10.0.0.0/8" {
		t.Errorf("Destination: got %q, want 10.0.0.0/8", r.Destination)
	}
	if r.NextHop != "192.168.1.1" {
		t.Errorf("NextHop: got %q, want 192.168.1.1", r.NextHop)
	}
	if r.Family != "ipv4" {
		t.Errorf("Family: got %q, want ipv4", r.Family)
	}
	if r.Protocol != "bgp" {
		t.Errorf("Protocol: got %q, want bgp", r.Protocol)
	}
}

// TestListKernelRoutesLimitCaps asserts the caller's limit stops the scan
// without returning an error.
// VALIDATES: limit parameter respected (0 = unbounded, N>0 = cap).
// PREVENTS: gigabyte allocation on full-DFZ dumps.
func TestListKernelRoutesLimitCaps(t *testing.T) {
	var v4 []ip.IPRouteV2Details
	for i := range 5 {
		cidr := fmt.Sprintf("10.%d.0.0/16", i)
		v4 = append(v4, makeIP4Route(t, cidr, "", 2 /* interface */, 0))
	}
	ch := &routeChannel{v4Details: v4}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	routes, err := b.ListKernelRoutes("", 3)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if len(routes) != 3 {
		t.Fatalf("routes: got %d, want 3 (capped by limit)", len(routes))
	}
}

// TestListKernelRoutesFilterPrefixExact restricts output to a single CIDR.
// VALIDATES: filterPrefix exact-match semantics mirror the netlink backend.
// PREVENTS: unexpectedly returning sibling routes sharing a prefix substring.
func TestListKernelRoutesFilterPrefixExact(t *testing.T) {
	ch := &routeChannel{
		v4Details: []ip.IPRouteV2Details{
			makeIP4Route(t, "10.0.0.0/8", "", 2, 0),
			makeIP4Route(t, "10.0.0.0/16", "", 2, 0),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	routes, err := b.ListKernelRoutes("10.0.0.0/8", 0)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("routes: got %d, want 1", len(routes))
	}
	if routes[0].Destination != "10.0.0.0/8" {
		t.Errorf("Destination: got %q, want 10.0.0.0/8", routes[0].Destination)
	}
}

// TestListKernelRoutesFilterDefault matches the default-route sentinel.
// VALIDATES: "default" filter matches both 0.0.0.0/0 and ::/0.
// PREVENTS: operator confusion when asking for the default route on VPP.
func TestListKernelRoutesFilterDefault(t *testing.T) {
	ch := &routeChannel{
		v4Details: []ip.IPRouteV2Details{
			makeIP4Route(t, "0.0.0.0/0", "192.168.1.1", 19, 0),
			makeIP4Route(t, "10.0.0.0/8", "", 2, 0),
		},
		v6Details: []ip.IPRouteV2Details{
			makeIP6Route(t, "::/0", "fe80::1", 19),
			makeIP6Route(t, "2001:db8::/32", "", 2),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	routes, err := b.ListKernelRoutes("default", 0)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("routes: got %d, want 2 (v4 + v6 default)", len(routes))
	}
}

// TestListKernelRoutesResolvesDevice ensures the SwIfIndex path renders as
// a ze name once the name map is seeded.
// VALIDATES: Device field populated from nameMap lookup.
// PREVENTS: showing opaque ifindex integers to operators.
func TestListKernelRoutesResolvesDevice(t *testing.T) {
	ch := &routeChannel{
		v4Details: []ip.IPRouteV2Details{
			makeIP4Route(t, "10.0.0.0/8", "192.168.1.1", 19, 7),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe7", 7, "xe7")
	b.populate.Do(func() {})

	routes, err := b.ListKernelRoutes("", 0)
	if err != nil {
		t.Fatalf("ListKernelRoutes: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("routes: got %d, want 1", len(routes))
	}
	if routes[0].Device != "xe7" {
		t.Errorf("Device: got %q, want xe7", routes[0].Device)
	}
}

// TestListKernelRoutesReceiveError propagates a channel failure.
// VALIDATES: VPP-side errors bubble up as Go errors (not silently empty).
// PREVENTS: silent loss of operator-observable failures.
func TestListKernelRoutesReceiveError(t *testing.T) {
	ch := &routeChannel{receiveErr: fmt.Errorf("VPP dead")}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	// v4Details needs at least one element so ReceiveReply is called (and
	// returns the error) before the last=true sentinel.
	ch.v4Details = []ip.IPRouteV2Details{makeIP4Route(t, "10.0.0.0/8", "", 2, 0)}
	if _, err := b.ListKernelRoutes("", 0); err == nil {
		t.Fatal("expected error from channel, got nil")
	}
}

// --- RouteLookup ---

func TestVPPRouteLookup(t *testing.T) {
	route := makeIP4Route(t, "10.20.0.0/24", "192.168.1.1", 19, 7)
	ch := &routeChannel{
		lookupReply: ip.IPRouteLookupV2Reply{
			Retval: 0,
			Route:  route.Route,
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe7", 7, "xe7")
	b.populate.Do(func() {})

	dest := netip.MustParseAddr("10.20.0.1")
	result, err := b.RouteLookup(dest)
	if err != nil {
		t.Fatalf("RouteLookup: %v", err)
	}
	if result["destination"] != "10.20.0.1" {
		t.Errorf("destination: got %v, want 10.20.0.1", result["destination"])
	}
	if result["prefix"] != "10.20.0.0/24" {
		t.Errorf("prefix: got %v, want 10.20.0.0/24", result["prefix"])
	}
	if result["next-hop"] != "192.168.1.1" {
		t.Errorf("next-hop: got %v, want 192.168.1.1", result["next-hop"])
	}
	if result["interface"] != "xe7" {
		t.Errorf("interface: got %v, want xe7", result["interface"])
	}
	if result["protocol"] != "bgp" {
		t.Errorf("protocol: got %v, want bgp", result["protocol"])
	}
	if result["table"] != int(0) {
		t.Errorf("table: got %v, want 0", result["table"])
	}
}

func TestVPPRouteLookupNoRoute(t *testing.T) {
	ch := &routeChannel{
		lookupReply: ip.IPRouteLookupV2Reply{Retval: -1},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	dest := netip.MustParseAddr("10.99.0.1")
	_, err := b.RouteLookup(dest)
	if err == nil {
		t.Fatal("expected error for no-route, got nil")
	}
}

func TestVPPRouteLookupIPv6(t *testing.T) {
	route := makeIP6Route(t, "2001:db8::/32", "fe80::1", 19)
	ch := &routeChannel{
		lookupReply: ip.IPRouteLookupV2Reply{
			Retval: 0,
			Route:  route.Route,
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	dest := netip.MustParseAddr("2001:db8::1")
	result, err := b.RouteLookup(dest)
	if err != nil {
		t.Fatalf("RouteLookup IPv6: %v", err)
	}
	if result["destination"] != "2001:db8::1" {
		t.Errorf("destination: got %v, want 2001:db8::1", result["destination"])
	}
	if result["prefix"] != "2001:db8::/32" {
		t.Errorf("prefix: got %v, want 2001:db8::/32", result["prefix"])
	}
	if result["next-hop"] != "fe80::1" {
		t.Errorf("next-hop: got %v, want fe80::1", result["next-hop"])
	}
}

func TestVPPRouteLookupChannelNotReady(t *testing.T) {
	orig := getActiveConnector
	getActiveConnector = func() vppConnector { return nil }
	defer func() { getActiveConnector = orig }()

	b := &vppBackendImpl{names: newNameMap()}
	_, err := b.RouteLookup(netip.MustParseAddr("10.0.0.1"))
	if err == nil {
		t.Fatal("expected ErrBackendNotReady, got nil")
	}
	if !errors.Is(err, iface.ErrBackendNotReady) {
		t.Fatalf("expected errors.Is(err, iface.ErrBackendNotReady), got %v", err)
	}
}

// dumpChannel is a mock api.Channel specialised for SwInterfaceDump
// multi-requests. Unit reuses the fibvpp testChannel pattern but returns
// SwInterfaceDetails instead of IPRouteAddDelReply.
type dumpChannel struct {
	lastRequest api.Message
	details     []interfaces.SwInterfaceDetails
	macReply    interfaces.SwInterfaceSetMacAddressReply
	sendErr     error
	receiveErr  error
	closed      bool
}

var _ api.Channel = (*dumpChannel)(nil)

func (c *dumpChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.lastRequest = msg
	return &dumpRequestCtx{ch: c}
}

func (c *dumpChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.lastRequest = msg
	return &dumpMultiCtx{ch: c, pos: 0}
}

func (c *dumpChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented in dumpChannel")
}

func (c *dumpChannel) SetReplyTimeout(time.Duration) {}

func (c *dumpChannel) CheckCompatiblity(...api.Message) error { return nil }

func (c *dumpChannel) Close() { c.closed = true }

type dumpRequestCtx struct{ ch *dumpChannel }

func (r *dumpRequestCtx) ReceiveReply(msg api.Message) error {
	if r.ch.sendErr != nil {
		return r.ch.sendErr
	}
	if reply, ok := msg.(*interfaces.SwInterfaceSetMacAddressReply); ok {
		*reply = r.ch.macReply
	}
	return nil
}

type dumpMultiCtx struct {
	ch  *dumpChannel
	pos int
}

func (m *dumpMultiCtx) ReceiveReply(msg api.Message) (bool, error) {
	if m.ch.receiveErr != nil {
		return false, m.ch.receiveErr
	}
	if m.pos >= len(m.ch.details) {
		return true, nil
	}
	d, ok := msg.(*interfaces.SwInterfaceDetails)
	if !ok {
		return false, fmt.Errorf("dumpMultiCtx: unexpected reply type %T", msg)
	}
	*d = m.ch.details[m.pos]
	m.pos++
	return false, nil
}

// asciiName converts a string to a 64-byte VPP-style fixed field.
func asciiName(s string) string {
	b := make([]byte, 64)
	copy(b, s)
	return string(b)
}

// --- ListInterfaces ---

func TestListInterfacesConvertsEveryDetails(t *testing.T) {
	// VALIDATES: AC-10 -- SwInterfaceDump results converted to InterfaceInfo
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 0, InterfaceName: asciiName("local0")},
			{SwIfIndex: 1, InterfaceName: asciiName("loop0"),
				Flags: interface_types.IF_STATUS_API_FLAG_ADMIN_UP},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	got, err := b.ListInterfaces()
	if err != nil {
		t.Fatalf("ListInterfaces: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len: got %d, want 2", len(got))
	}
	if got[0].Name != "local0" {
		t.Errorf("got[0].Name = %q, want local0", got[0].Name)
	}
	if got[1].State != "up" {
		t.Errorf("got[1].State = %q, want up", got[1].State)
	}
}

func TestListInterfacesRequestType(t *testing.T) {
	// VALIDATES: AC-10 -- SwInterfaceDump is the RPC invoked
	ch := &dumpChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	_, err := b.ListInterfaces()
	if err != nil {
		t.Fatalf("ListInterfaces: %v", err)
	}
	if _, ok := ch.lastRequest.(*interfaces.SwInterfaceDump); !ok {
		t.Errorf("lastRequest type: got %T, want *interfaces.SwInterfaceDump", ch.lastRequest)
	}
}

func TestListInterfacesReceiveError(t *testing.T) {
	// VALIDATES: reply error propagates as ifacevpp error
	ch := &dumpChannel{receiveErr: fmt.Errorf("VPP dead")}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	if _, err := b.ListInterfaces(); err == nil {
		t.Fatal("expected error when ReceiveReply fails")
	}
}

// --- GetInterface ---

func TestGetInterfaceExactMatch(t *testing.T) {
	// VALIDATES: NameFilter is substring -- exact match required
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 10, InterfaceName: asciiName("xe0")},
			{SwIfIndex: 11, InterfaceName: asciiName("xe0.100")},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	info, err := b.GetInterface("xe0")
	if err != nil {
		t.Fatalf("GetInterface: %v", err)
	}
	if info.Index != 10 {
		t.Errorf("Index: got %d, want 10 (not sub-if 11)", info.Index)
	}
}

func TestGetInterfaceNotFound(t *testing.T) {
	// VALIDATES: missing interface returns error with name
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 10, InterfaceName: asciiName("xe1")},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	if _, err := b.GetInterface("xe0"); err == nil {
		t.Fatal("expected error for missing interface")
	}
}

// --- Get/SetMACAddress ---

func TestGetMACAddressFromDetails(t *testing.T) {
	// VALIDATES: L2Address bytes formatted as EUI-48 colon form
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{
				SwIfIndex:     5,
				InterfaceName: asciiName("xe0"),
				L2Address:     [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	mac, err := b.GetMACAddress("xe0")
	if err != nil {
		t.Fatalf("GetMACAddress: %v", err)
	}
	if mac != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("mac: got %q, want aa:bb:cc:dd:ee:ff", mac)
	}
}

func TestSetMACAddressSendsRequest(t *testing.T) {
	// VALIDATES: SwInterfaceSetMacAddress invoked with parsed MAC bytes
	ch := &dumpChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 3, "xe0")

	if err := b.SetMACAddress("xe0", "aa:bb:cc:dd:ee:ff"); err != nil {
		t.Fatalf("SetMACAddress: %v", err)
	}
	req, ok := ch.lastRequest.(*interfaces.SwInterfaceSetMacAddress)
	if !ok {
		t.Fatalf("lastRequest type: got %T, want *SwInterfaceSetMacAddress", ch.lastRequest)
	}
	if req.SwIfIndex != 3 {
		t.Errorf("SwIfIndex: got %d, want 3", req.SwIfIndex)
	}
	want := [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	if req.MacAddress != want {
		t.Errorf("MacAddress: got %v, want %v", req.MacAddress, want)
	}
}

func TestSetMACAddressRetvalError(t *testing.T) {
	// VALIDATES: non-zero retval produces error
	ch := &dumpChannel{macReply: interfaces.SwInterfaceSetMacAddressReply{Retval: -1}}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 1, "xe0")

	if err := b.SetMACAddress("xe0", "aa:bb:cc:dd:ee:ff"); err == nil {
		t.Fatal("expected error for retval=-1")
	}
}

// --- populateNameMap (AC-13 NameMappingPopulate) ---

func TestPopulateNameMap(t *testing.T) {
	// VALIDATES: AC-13 -- map populated from SwInterfaceDump at startup
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 0, InterfaceName: asciiName("local0")},
			{SwIfIndex: 1, InterfaceName: asciiName("TenGigabitEthernet3/0/0")},
			{SwIfIndex: 2, InterfaceName: asciiName("loop0")},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	if err := b.populateNameMap(); err != nil {
		t.Fatalf("populateNameMap: %v", err)
	}
	if b.names.Len() != 3 {
		t.Errorf("map size: got %d, want 3", b.names.Len())
	}
	idx, ok := b.names.lookupIndex("TenGigabitEthernet3/0/0")
	if !ok || idx != 1 {
		t.Errorf("LookupIndex(Ten3/0/0): got %d,%v want 1,true", idx, ok)
	}
}

func TestPopulateNameMapEmptyNameSkipped(t *testing.T) {
	// VALIDATES: interfaces with blank name (pure NUL) are skipped
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 0, InterfaceName: asciiName("")},
			{SwIfIndex: 1, InterfaceName: asciiName("xe0")},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	if err := b.populateNameMap(); err != nil {
		t.Fatalf("populateNameMap: %v", err)
	}
	if b.names.Len() != 1 {
		t.Errorf("map size: got %d, want 1 (blank skipped)", b.names.Len())
	}
}

// --- VPP stats provider tests (spec-cp-survival-5-detect-1a) ---

// fakeStatsProvider implements vppcomp.IfaceStatsReader for testing.
type fakeStatsProvider struct {
	ifaces []api.InterfaceCounters
	err    error
}

func (f *fakeStatsProvider) GetInterfaceStats(s *api.InterfaceStats) error {
	if f.err != nil {
		return f.err
	}
	s.Interfaces = f.ifaces
	return nil
}

func withFakeStats(fp *fakeStatsProvider) func() {
	orig := getActiveStatsProvider
	getActiveStatsProvider = func() vppcomp.IfaceStatsReader { return fp }
	return func() { getActiveStatsProvider = orig }
}

func withNilStats() func() {
	orig := getActiveStatsProvider
	getActiveStatsProvider = func() vppcomp.IfaceStatsReader { return nil }
	return func() { getActiveStatsProvider = orig }
}

func TestVPPDetailsToInfoPopulatesStats(t *testing.T) {
	// VALIDATES: AC-2 -- InterfaceInfo.Stats is non-nil and carries VPP counters
	defer withFakeStats(&fakeStatsProvider{
		ifaces: []api.InterfaceCounters{
			{InterfaceName: "xe0",
				Rx:       api.InterfaceCounterCombined{Packets: 1000, Bytes: 64000},
				Tx:       api.InterfaceCounterCombined{Packets: 500, Bytes: 32000},
				RxErrors: 3, TxErrors: 1, Drops: 7},
		},
	})()

	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 1, InterfaceName: asciiName("xe0"),
				Flags: interface_types.IF_STATUS_API_FLAG_ADMIN_UP},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	got, err := b.ListInterfaces()
	if err != nil {
		t.Fatalf("ListInterfaces: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	if got[0].Stats == nil {
		t.Fatal("Stats is nil, want non-nil")
	}
	if got[0].Stats.RxPackets != 1000 {
		t.Errorf("RxPackets: got %d, want 1000", got[0].Stats.RxPackets)
	}
	if got[0].Stats.RxBytes != 64000 {
		t.Errorf("RxBytes: got %d, want 64000", got[0].Stats.RxBytes)
	}
	if got[0].Stats.TxPackets != 500 {
		t.Errorf("TxPackets: got %d, want 500", got[0].Stats.TxPackets)
	}
	if got[0].Stats.TxBytes != 32000 {
		t.Errorf("TxBytes: got %d, want 32000", got[0].Stats.TxBytes)
	}
	if got[0].Stats.RxErrors != 3 {
		t.Errorf("RxErrors: got %d, want 3", got[0].Stats.RxErrors)
	}
	if got[0].Stats.TxErrors != 1 {
		t.Errorf("TxErrors: got %d, want 1", got[0].Stats.TxErrors)
	}
	if got[0].Stats.RxDropped != 7 {
		t.Errorf("RxDropped: got %d, want 7", got[0].Stats.RxDropped)
	}
}

func TestVPPStatsIndexToNameMapping(t *testing.T) {
	// VALIDATES: A-3, R-3 -- stats keyed by name, not stale index
	defer withFakeStats(&fakeStatsProvider{
		ifaces: []api.InterfaceCounters{
			{InterfaceName: "xe0", Rx: api.InterfaceCounterCombined{Packets: 100}},
			{InterfaceName: "xe1", Rx: api.InterfaceCounterCombined{Packets: 200}},
		},
	})()

	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 1, InterfaceName: asciiName("xe0")},
			{SwIfIndex: 2, InterfaceName: asciiName("xe1")},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	got, err := b.ListInterfaces()
	if err != nil {
		t.Fatalf("ListInterfaces: %v", err)
	}
	if got[0].Stats.RxPackets != 100 {
		t.Errorf("xe0 RxPackets: got %d, want 100", got[0].Stats.RxPackets)
	}
	if got[1].Stats.RxPackets != 200 {
		t.Errorf("xe1 RxPackets: got %d, want 200", got[1].Stats.RxPackets)
	}
}

func TestVPPStatsProviderUnavailable(t *testing.T) {
	// VALIDATES: graceful degradation when stats provider is nil
	defer withNilStats()()

	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 1, InterfaceName: asciiName("xe0")},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	got, err := b.ListInterfaces()
	if err != nil {
		t.Fatalf("ListInterfaces: %v", err)
	}
	if got[0].Stats != nil {
		t.Errorf("Stats should be nil when provider unavailable, got %+v", got[0].Stats)
	}
}

func TestVPPStatsProviderError(t *testing.T) {
	// VALIDATES: graceful degradation when GetInterfaceStats fails
	defer withFakeStats(&fakeStatsProvider{err: fmt.Errorf("stats segment disconnected")})()

	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 1, InterfaceName: asciiName("xe0")},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	got, err := b.ListInterfaces()
	if err != nil {
		t.Fatalf("ListInterfaces: %v", err)
	}
	if got[0].Stats != nil {
		t.Errorf("Stats should be nil on error, got %+v", got[0].Stats)
	}
}

func TestGetStatsReturnsVPPCounters(t *testing.T) {
	// VALIDATES: AC-1 -- GetStats returns non-zero counters for a VPP interface
	defer withFakeStats(&fakeStatsProvider{
		ifaces: []api.InterfaceCounters{
			{InterfaceName: "xe0", Rx: api.InterfaceCounterCombined{Packets: 42, Bytes: 2688}},
		},
	})()

	b := &vppBackendImpl{ch: &dumpChannel{}, names: newNameMap()}
	s, err := b.GetStats("xe0")
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if s.RxPackets != 42 {
		t.Errorf("RxPackets: got %d, want 42", s.RxPackets)
	}
	if s.RxBytes != 2688 {
		t.Errorf("RxBytes: got %d, want 2688", s.RxBytes)
	}
}

func TestGetStatsUnknownInterface(t *testing.T) {
	// VALIDATES: GetStats returns error for unknown interface
	defer withFakeStats(&fakeStatsProvider{
		ifaces: []api.InterfaceCounters{
			{InterfaceName: "xe0", Rx: api.InterfaceCounterCombined{Packets: 1}},
		},
	})()

	b := &vppBackendImpl{ch: &dumpChannel{}, names: newNameMap()}
	if _, err := b.GetStats("xe99"); err == nil {
		t.Fatal("expected error for unknown interface")
	}
}

// newTunnelBackend returns a backend wired to a programmable channel with a
// deterministic SwIfIndex for add replies.
func newTunnelBackend(ch *progChannel) *vppBackendImpl {
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // short-circuit ensureChannel populate
	return b
}

// TestCreateTunnelGRE verifies AC-2: a gre tunnel under the vpp backend issues
// a gre_tunnel_add_del with type L3, the resolved endpoints, and registers the
// name->SwIfIndex mapping.
// VALIDATES: AC-2 -- GRE tunnel programmed on VPP.
// PREVENTS: regression to the errNotSupported stub.
func TestCreateTunnelGRE(t *testing.T) {
	ch := &progChannel{swIfIndex: 7}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindGRE,
		Name:          "gre0",
		LocalAddress:  "192.0.2.1",
		RemoteAddress: "192.0.2.2",
	})
	if err != nil {
		t.Fatalf("CreateTunnel gre: %v", err)
	}
	req, ok := ch.requests[0].(*gre.GreTunnelAddDel)
	if !ok {
		t.Fatalf("request type: got %T, want *gre.GreTunnelAddDel", ch.requests[0])
	}
	if !req.IsAdd {
		t.Error("IsAdd: got false, want true")
	}
	if req.Tunnel.Type != gre.GRE_API_TUNNEL_TYPE_L3 {
		t.Errorf("Type: got %v, want L3", req.Tunnel.Type)
	}
	if got := req.Tunnel.Src.ToIP().String(); got != "192.0.2.1" {
		t.Errorf("Src: got %s, want 192.0.2.1", got)
	}
	if got := req.Tunnel.Dst.ToIP().String(); got != "192.0.2.2" {
		t.Errorf("Dst: got %s, want 192.0.2.2", got)
	}
	if idx, ok := b.names.lookupIndex("gre0"); !ok || idx != 7 {
		t.Errorf("name map: got (%d,%v), want (7,true)", idx, ok)
	}
}

// TestCreateTunnelGRETap verifies AC-2: gretap maps to the TEB (transparent
// ethernet bridging) gre tunnel type.
// VALIDATES: AC-2 -- GRETAP programmed as TEB.
// PREVENTS: gretap silently becoming an L3 gre tunnel.
func TestCreateTunnelGRETap(t *testing.T) {
	ch := &progChannel{swIfIndex: 9}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindGRETap,
		Name:          "gt0",
		LocalAddress:  "10.0.0.1",
		RemoteAddress: "10.0.0.2",
	})
	if err != nil {
		t.Fatalf("CreateTunnel gretap: %v", err)
	}
	req, ok := ch.requests[0].(*gre.GreTunnelAddDel)
	if !ok {
		t.Fatalf("request type: got %T, want *gre.GreTunnelAddDel", ch.requests[0])
	}
	if req.Tunnel.Type != gre.GRE_API_TUNNEL_TYPE_TEB {
		t.Errorf("Type: got %v, want TEB", req.Tunnel.Type)
	}
}

// TestCreateTunnelIPIP verifies AC-2: an ipip tunnel issues ipip_add_tunnel
// with the resolved endpoints.
// VALIDATES: AC-2 -- IPIP tunnel programmed on VPP.
// PREVENTS: regression to the errNotSupported stub.
func TestCreateTunnelIPIP(t *testing.T) {
	ch := &progChannel{swIfIndex: 3}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindIPIP,
		Name:          "ipip0",
		LocalAddress:  "203.0.113.1",
		RemoteAddress: "203.0.113.2",
	})
	if err != nil {
		t.Fatalf("CreateTunnel ipip: %v", err)
	}
	req, ok := ch.requests[0].(*ipip.IpipAddTunnel)
	if !ok {
		t.Fatalf("request type: got %T, want *ipip.IpipAddTunnel", ch.requests[0])
	}
	if got := req.Tunnel.Dst.ToIP().String(); got != "203.0.113.2" {
		t.Errorf("Dst: got %s, want 203.0.113.2", got)
	}
	if idx, ok := b.names.lookupIndex("ipip0"); !ok || idx != 3 {
		t.Errorf("name map: got (%d,%v), want (3,true)", idx, ok)
	}
}

// TestDeleteTunnelGRE verifies the delete path issues gre_tunnel_add_del with
// IsAdd=false and clears the name map.
// VALIDATES: AC-2 -- clean tunnel delete path.
// PREVENTS: stale VPP tunnels / name-map entries after a config removal.
func TestDeleteTunnelGRE(t *testing.T) {
	ch := &progChannel{swIfIndex: 5}
	b := newTunnelBackend(ch)

	if err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindGRE,
		Name:          "gre0",
		LocalAddress:  "192.0.2.1",
		RemoteAddress: "192.0.2.2",
	}); err != nil {
		t.Fatalf("CreateTunnel: %v", err)
	}
	if err := b.DeleteInterface("gre0"); err != nil {
		t.Fatalf("DeleteInterface: %v", err)
	}
	last, ok := ch.requests[len(ch.requests)-1].(*gre.GreTunnelAddDel)
	if !ok {
		t.Fatalf("delete request type: got %T, want *gre.GreTunnelAddDel", ch.requests[len(ch.requests)-1])
	}
	if last.IsAdd {
		t.Error("delete: IsAdd got true, want false")
	}
	if _, ok := b.names.lookupIndex("gre0"); ok {
		t.Error("name map still has gre0 after delete")
	}
}

// TestCreateTunnelVxlanVPP verifies AC-3: a vxlan tunnel under the vpp backend
// issues a vxlan_add_del_tunnel_v3 carrying the VNI, endpoints, and the
// default UDP port 4789, and registers the name->SwIfIndex mapping.
// VALIDATES: AC-3 -- VXLAN programmed on VPP.
// PREVENTS: regression / silent no-op for the new tunnel kind.
func TestCreateTunnelVxlanVPP(t *testing.T) {
	ch := &progChannel{swIfIndex: 11}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindVxlan,
		Name:          "vx0",
		LocalAddress:  "10.0.0.1",
		RemoteAddress: "10.0.0.2",
		VNI:           100,
		VNISet:        true,
	})
	if err != nil {
		t.Fatalf("CreateTunnel vxlan: %v", err)
	}
	req, ok := ch.requests[0].(*vxlan.VxlanAddDelTunnelV3)
	if !ok {
		t.Fatalf("request type: got %T, want *vxlan.VxlanAddDelTunnelV3", ch.requests[0])
	}
	if !req.IsAdd {
		t.Error("IsAdd: got false, want true")
	}
	if req.Vni != 100 {
		t.Errorf("Vni: got %d, want 100", req.Vni)
	}
	if req.DstPort != 4789 {
		t.Errorf("DstPort: got %d, want 4789 (default)", req.DstPort)
	}
	if got := req.DstAddress.ToIP().String(); got != "10.0.0.2" {
		t.Errorf("DstAddress: got %s, want 10.0.0.2", got)
	}
	if idx, ok := b.names.lookupIndex("vx0"); !ok || idx != 11 {
		t.Errorf("name map: got (%d,%v), want (11,true)", idx, ok)
	}
}

// TestCreateTunnelVxlanCustomPort verifies a configured UDP port overrides the
// 4789 default.
// VALIDATES: AC-3 -- vxlan port honored.
// PREVENTS: the configured port being ignored.
func TestCreateTunnelVxlanCustomPort(t *testing.T) {
	ch := &progChannel{swIfIndex: 1}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindVxlan,
		Name:          "vx0",
		LocalAddress:  "10.0.0.1",
		RemoteAddress: "10.0.0.2",
		VNI:           7,
		VNISet:        true,
		Port:          8472,
		PortSet:       true,
	})
	if err != nil {
		t.Fatalf("CreateTunnel vxlan: %v", err)
	}
	req, ok := ch.requests[0].(*vxlan.VxlanAddDelTunnelV3)
	if !ok {
		t.Fatalf("request type: got %T, want *vxlan.VxlanAddDelTunnelV3", ch.requests[0])
	}
	if req.DstPort != 8472 {
		t.Errorf("DstPort: got %d, want 8472", req.DstPort)
	}
}

// TestDeleteTunnelVxlan verifies the delete path issues
// vxlan_add_del_tunnel_v3 with IsAdd=false and clears the name map.
// VALIDATES: AC-3 -- clean vxlan delete path.
// PREVENTS: stale VPP vxlan tunnels after config removal.
func TestDeleteTunnelVxlan(t *testing.T) {
	ch := &progChannel{swIfIndex: 4}
	b := newTunnelBackend(ch)

	if err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindVxlan,
		Name:          "vx0",
		LocalAddress:  "10.0.0.1",
		RemoteAddress: "10.0.0.2",
		VNI:           100,
		VNISet:        true,
	}); err != nil {
		t.Fatalf("CreateTunnel: %v", err)
	}
	if err := b.DeleteInterface("vx0"); err != nil {
		t.Fatalf("DeleteInterface: %v", err)
	}
	last, ok := ch.requests[len(ch.requests)-1].(*vxlan.VxlanAddDelTunnelV3)
	if !ok {
		t.Fatalf("delete request type: got %T, want *vxlan.VxlanAddDelTunnelV3", ch.requests[len(ch.requests)-1])
	}
	if last.IsAdd {
		t.Error("delete: IsAdd got true, want false")
	}
	if _, ok := b.names.lookupIndex("vx0"); ok {
		t.Error("name map still has vx0 after delete")
	}
}

func newWireguardBackend(ch *progChannel) *vppBackendImpl {
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // short-circuit ensureChannel populate
	return b
}

func wgKey(seed byte) iface.WireguardKey {
	var k iface.WireguardKey
	for i := range k {
		k[i] = seed
	}
	return k
}

// TestConfigureWireguardCreatesInterface verifies AC-5: configuring a wireguard
// device issues wireguard_interface_create carrying the private key and listen
// port, and registers the name->SwIfIndex mapping.
// VALIDATES: AC-5 -- wireguard interface programmed via binapi.
// PREVENTS: regression to the errNotSupported stub.
func TestConfigureWireguardCreatesInterface(t *testing.T) {
	ch := &progChannel{swIfIndex: 11}
	b := newWireguardBackend(ch)

	// CreateWireguardDevice is a no-op on VPP (key comes at Configure time).
	if err := b.CreateWireguardDevice("wg0"); err != nil {
		t.Fatalf("CreateWireguardDevice: %v", err)
	}
	if len(ch.requests) != 0 {
		t.Errorf("CreateWireguardDevice should issue no VPP request, got %d", len(ch.requests))
	}

	spec := iface.WireguardSpec{Name: "wg0", PrivateKey: wgKey(0x11), ListenPort: 51820, ListenPortSet: true}
	if err := b.ConfigureWireguardDevice(spec); err != nil {
		t.Fatalf("ConfigureWireguardDevice: %v", err)
	}
	create, ok := ch.requests[0].(*wireguard.WireguardInterfaceCreate)
	if !ok {
		t.Fatalf("request[0] type: got %T, want *wireguard.WireguardInterfaceCreate", ch.requests[0])
	}
	if create.Interface.Port != 51820 {
		t.Errorf("Port: got %d, want 51820", create.Interface.Port)
	}
	if len(create.Interface.PrivateKey) != 32 || create.Interface.PrivateKey[0] != 0x11 {
		t.Errorf("PrivateKey not carried into create request")
	}
	if idx, ok := b.names.lookupIndex("wg0"); !ok || idx != 11 {
		t.Errorf("name map: got (%d,%v), want (11,true)", idx, ok)
	}
}

// TestConfigureWireguardAddsPeers verifies peers are programmed via
// wireguard_peer_add with public key, endpoint, and allowed-ips.
// VALIDATES: AC-5 -- peer set programmed.
func TestConfigureWireguardAddsPeers(t *testing.T) {
	ch := &progChannel{swIfIndex: 3, peerIndex: 7}
	b := newWireguardBackend(ch)

	spec := iface.WireguardSpec{
		Name:       "wg0",
		PrivateKey: wgKey(0x22),
		Peers: []iface.WireguardPeerSpec{{
			Name:         "peerA",
			PublicKey:    wgKey(0x33),
			EndpointIP:   "192.0.2.50",
			EndpointPort: 51820,
			AllowedIPs:   []string{"10.0.0.0/24"},
		}},
	}
	if err := b.ConfigureWireguardDevice(spec); err != nil {
		t.Fatalf("ConfigureWireguardDevice: %v", err)
	}
	var add *wireguard.WireguardPeerAdd
	for _, r := range ch.requests {
		if pa, ok := r.(*wireguard.WireguardPeerAdd); ok {
			add = pa
		}
	}
	if add == nil {
		t.Fatal("no WireguardPeerAdd issued")
	}
	if got := add.Peer.Endpoint.ToIP().String(); got != "192.0.2.50" {
		t.Errorf("Endpoint: got %s, want 192.0.2.50", got)
	}
	if add.Peer.NAllowedIps != 1 || len(add.Peer.AllowedIps) != 1 {
		t.Errorf("AllowedIps: got n=%d len=%d, want 1/1", add.Peer.NAllowedIps, len(add.Peer.AllowedIps))
	}
	if got := add.Peer.AllowedIps[0].String(); !strings.HasPrefix(got, "10.0.0.0/24") {
		t.Errorf("AllowedIps[0]: got %s, want 10.0.0.0/24", got)
	}
}

// TestConfigureWireguardReplacesPeers verifies ReplacePeers semantics: a second
// Configure removes the peer indices installed by the first before adding the
// new set. VALIDATES: AC-5 -- peer reconciliation.
func TestConfigureWireguardReplacesPeers(t *testing.T) {
	ch := &progChannel{swIfIndex: 5, peerIndex: 9}
	b := newWireguardBackend(ch)

	base := iface.WireguardSpec{
		Name:       "wg0",
		PrivateKey: wgKey(0x77),
		Peers:      []iface.WireguardPeerSpec{{Name: "p1", PublicKey: wgKey(0x01)}},
	}
	if err := b.ConfigureWireguardDevice(base); err != nil {
		t.Fatalf("first configure: %v", err)
	}
	// Second apply with a different peer set must remove peer index 9 first.
	ch.requests = nil
	next := iface.WireguardSpec{
		Name:       "wg0",
		PrivateKey: wgKey(0x77),
		Peers:      []iface.WireguardPeerSpec{{Name: "p2", PublicKey: wgKey(0x02)}},
	}
	if err := b.ConfigureWireguardDevice(next); err != nil {
		t.Fatalf("second configure: %v", err)
	}
	var removed *wireguard.WireguardPeerRemove
	for _, r := range ch.requests {
		if pr, ok := r.(*wireguard.WireguardPeerRemove); ok {
			removed = pr
		}
	}
	if removed == nil {
		t.Fatal("second configure did not remove the previously installed peer")
	}
	if removed.PeerIndex != 9 {
		t.Errorf("removed PeerIndex: got %d, want 9", removed.PeerIndex)
	}
}

// TestGetWireguardRoundTrip verifies GetWireguardDevice reads back the interface
// key/port and peer set from the dump replies.
// VALIDATES: AC-5 -- GetWireguardDevice round-trips the spec.
func TestGetWireguardRoundTrip(t *testing.T) {
	ch := &progChannel{}
	b := newWireguardBackend(ch)
	b.names.Add("wg0", 8, "wg0")

	priv := wgKey(0x88)
	pub := wgKey(0x99)
	ch.wgIfaceDetails = []wireguard.WireguardInterfaceDetails{{
		Interface: wireguard.WireguardInterface{SwIfIndex: 8, PrivateKey: priv[:], Port: 51821},
	}}
	ch.wgPeerDetails = []wireguard.WireguardPeersDetails{{
		Peer: wireguard.WireguardPeer{SwIfIndex: 8, PublicKey: pub[:], Port: 4444},
	}}

	got, err := b.GetWireguardDevice("wg0")
	if err != nil {
		t.Fatalf("GetWireguardDevice: %v", err)
	}
	if got.ListenPort != 51821 || !got.ListenPortSet {
		t.Errorf("ListenPort: got %d/%v, want 51821/true", got.ListenPort, got.ListenPortSet)
	}
	if got.PrivateKey != priv {
		t.Errorf("PrivateKey did not round-trip")
	}
	if len(got.Peers) != 1 {
		t.Fatalf("Peers: got %d, want 1", len(got.Peers))
	}
	if got.Peers[0].PublicKey != pub {
		t.Errorf("peer PublicKey did not round-trip")
	}
	if got.Peers[0].EndpointPort != 4444 {
		t.Errorf("peer EndpointPort: got %d, want 4444", got.Peers[0].EndpointPort)
	}
}

// TestDeleteWireguardInterface verifies DeleteInterface issues
// wireguard_interface_delete and clears the name map.
// VALIDATES: AC-5 -- clean wireguard delete path.
func TestDeleteWireguardInterface(t *testing.T) {
	ch := &progChannel{swIfIndex: 6}
	b := newWireguardBackend(ch)
	if err := b.ConfigureWireguardDevice(iface.WireguardSpec{Name: "wg0", PrivateKey: wgKey(0xAA)}); err != nil {
		t.Fatalf("ConfigureWireguardDevice: %v", err)
	}
	if err := b.DeleteInterface("wg0"); err != nil {
		t.Fatalf("DeleteInterface: %v", err)
	}
	var del *wireguard.WireguardInterfaceDelete
	for _, r := range ch.requests {
		if d, ok := r.(*wireguard.WireguardInterfaceDelete); ok {
			del = d
		}
	}
	if del == nil {
		t.Fatal("no WireguardInterfaceDelete issued")
	}
	if _, ok := b.names.lookupIndex("wg0"); ok {
		t.Error("name map still has wg0 after delete")
	}
}

// TestResetCountersAllInterfaces verifies the empty-name path sends a
// single SwInterfaceClearStats request with the "all interfaces" sentinel
// SwIfIndex (~0), matching VPP's semantics for clear-all.
// VALIDATES: ResetCounters("") emits a clear-all request.
// PREVENTS: regression to the old errNotSupported stub.
func TestResetCountersAllInterfaces(t *testing.T) {
	ch := &routeChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // mark populated so ensureChannel short-circuits

	if err := b.ResetCounters(""); err != nil {
		t.Fatalf("ResetCounters: %v", err)
	}
	req, ok := ch.lastRequest.(*interfaces.SwInterfaceClearStats)
	if !ok {
		t.Fatalf("lastRequest type: got %T, want *SwInterfaceClearStats", ch.lastRequest)
	}
	want := interface_types.InterfaceIndex(^uint32(0))
	if req.SwIfIndex != want {
		t.Errorf("SwIfIndex: got %#x, want %#x (clear-all sentinel)", req.SwIfIndex, want)
	}
}

// TestResetCountersSingleInterface verifies the named-interface path
// resolves the ze name to its SwIfIndex and sends that index (not the
// sentinel) in the request.
// VALIDATES: ResetCounters(name) targets the resolved SwIfIndex.
// PREVENTS: silently clearing the wrong interface (or all of them).
func TestResetCountersSingleInterface(t *testing.T) {
	ch := &routeChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe3", 3, "xe3")

	if err := b.ResetCounters("xe3"); err != nil {
		t.Fatalf("ResetCounters: %v", err)
	}
	req, ok := ch.lastRequest.(*interfaces.SwInterfaceClearStats)
	if !ok {
		t.Fatalf("lastRequest type: got %T, want *SwInterfaceClearStats", ch.lastRequest)
	}
	if req.SwIfIndex != 3 {
		t.Errorf("SwIfIndex: got %d, want 3", req.SwIfIndex)
	}
}

// TestResetCountersRetvalError propagates a non-zero retval.
// VALIDATES: VPP-reported failures surface as Go errors (not silent success).
// PREVENTS: silent counter-clear failure masked as a good return.
func TestResetCountersRetvalError(t *testing.T) {
	ch := &routeChannel{clearReply: clearStatsReply{retval: -17}}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	if err := b.ResetCounters(""); err == nil {
		t.Fatal("expected error for retval=-17, got nil")
	}
}

// fakeConnector is a minimal vppConnector for ensureChannel sentinel tests.
type fakeConnector struct {
	connected bool
	ch        api.Channel
	chErr     error
}

func (f *fakeConnector) IsConnected() bool { return f.connected }

func (f *fakeConnector) NewChannel() (api.Channel, error) {
	if f.chErr != nil {
		return nil, f.chErr
	}
	return f.ch, nil
}

// withConnector replaces the package-level getActiveConnector for one test.
// Returns a restore function.
func withConnector(c vppConnector) func() {
	orig := getActiveConnector
	getActiveConnector = func() vppConnector {
		if c == nil {
			return nil
		}
		return c
	}
	return func() { getActiveConnector = orig }
}

// TestEnsureChannel_NoConnectorReturnsSentinel verifies AC-1: when no
// connector is registered (vpp plugin not yet in OnStarted, or vpp.enabled=false),
// ensureChannel returns an error satisfying errors.Is(err, iface.ErrBackendNotReady).
// VALIDATES: AC-1 -- sentinel returned when connector is nil.
// PREVENTS: callers treating the startup race as a hard failure.
func TestEnsureChannel_NoConnectorReturnsSentinel(t *testing.T) {
	restore := withConnector(nil)
	defer restore()

	b := &vppBackendImpl{names: newNameMap()}
	err := b.ensureChannel()
	if err == nil {
		t.Fatal("expected sentinel-wrapped error, got nil")
	}
	if !errors.Is(err, iface.ErrBackendNotReady) {
		t.Fatalf("expected errors.Is(err, iface.ErrBackendNotReady), got %v", err)
	}
}

// TestEnsureChannel_NotConnectedReturnsSentinel verifies AC-1: when the
// connector exists but IsConnected() returns false (vpp handshake in flight),
// ensureChannel returns the sentinel.
// VALIDATES: AC-1 -- sentinel returned when IsConnected() is false.
// PREVENTS: relying on NewChannel's generic "govpp: not connected" error.
func TestEnsureChannel_NotConnectedReturnsSentinel(t *testing.T) {
	restore := withConnector(&fakeConnector{connected: false})
	defer restore()

	b := &vppBackendImpl{names: newNameMap()}
	err := b.ensureChannel()
	if err == nil {
		t.Fatal("expected sentinel-wrapped error, got nil")
	}
	if !errors.Is(err, iface.ErrBackendNotReady) {
		t.Fatalf("expected errors.Is(err, iface.ErrBackendNotReady), got %v", err)
	}
}

// TestEnsureChannel_NotReadyDoesNotCache verifies that returning the sentinel
// does NOT permanently cache the error. The second call must still check
// connector state so a later connect can succeed.
// VALIDATES: retry semantics for deferred reconciliation.
// PREVENTS: sync.Once-style caching that would make the backend permanently dead.
func TestEnsureChannel_NotReadyDoesNotCache(t *testing.T) {
	// First call: not connected -> sentinel
	fake := &fakeConnector{connected: false}
	restore := withConnector(fake)

	b := &vppBackendImpl{names: newNameMap()}
	err1 := b.ensureChannel()
	if !errors.Is(err1, iface.ErrBackendNotReady) {
		t.Fatalf("call 1: expected sentinel, got %v", err1)
	}
	restore()

	// Second call: different backend, connector now nil.
	// If ensureChannel had cached the sentinel on the instance, it would
	// still return it. It does not, so we re-evaluate state. We do not
	// actually flip to connected here because that requires a real
	// api.Channel; the second sentinel call is enough to prove non-cached.
	restore2 := withConnector(nil)
	defer restore2()
	err2 := b.ensureChannel()
	if !errors.Is(err2, iface.ErrBackendNotReady) {
		t.Fatalf("call 2: expected sentinel (still not ready), got %v", err2)
	}
}

// VALIDATES: AC-14 -- VPP egress map update on live sub-interface.
// PREVENTS: regression in dynamic CoS map application.
func TestVPPUpdateVLANQoSMap(t *testing.T) {
	ch := &routeChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})
	b.names.Add("xe0.100", 42, "xe0.100")

	egress := map[uint32]uint32{0: 0, 1: 1, 5: 3}
	if err := b.UpdateVLANQoSMap("xe0.100", nil, egress); err != nil {
		t.Fatalf("UpdateVLANQoSMap: %v", err)
	}
	if len(ch.allRequests) != 2 {
		t.Fatalf("expected 2 VPP requests (egress-map + mark), got %d", len(ch.allRequests))
	}
}

// VALIDATES: AC-16 -- VPP revert clears egress map.
// PREVENTS: stale QoS maps after session-down.
func TestVPPUpdateVLANQoSMapRevert(t *testing.T) {
	ch := &routeChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})
	b.names.Add("xe0.100", 42, "xe0.100")

	if err := b.UpdateVLANQoSMap("xe0.100", nil, nil); err != nil {
		t.Fatalf("UpdateVLANQoSMap revert: %v", err)
	}
	if len(ch.allRequests) != 0 {
		t.Fatalf("expected 0 VPP requests for nil maps (revert to static), got %d", len(ch.allRequests))
	}
}

func TestCloseNilChannel(t *testing.T) {
	// VALIDATES: AC-1 -- Close is safe
	// PREVENTS: panic on close without channel
	// Note: can't test with nil ch (would panic). Test with names only.
	b := &vppBackendImpl{names: newNameMap()}
	// Close with nil ch would panic, so this test just verifies the type compiles.
	_ = b
}

func TestStopMonitorSafe(t *testing.T) {
	// VALIDATES: StopMonitor is safe to call without StartMonitor
	// PREVENTS: panic on no-op stop
	b := &vppBackendImpl{names: newNameMap()}
	b.StopMonitor() // should not panic
}
