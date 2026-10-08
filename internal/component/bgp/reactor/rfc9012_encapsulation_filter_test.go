// Design: docs/architecture/meta/filter-community.md -- configured named strip sets.
// Related: reactor_notify.go, reactor_api_forward.go -- consuming filter paths.
package reactor

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Configure the actual plugin through its registered SDK runner, rather than
// substituting a callback that merely returns the operations this test expects.
func teConfigureCommunityFilter(t *testing.T) {
	t.Helper()
	engine, pluginConn := net.Pipe()
	conn := rpc.NewConn(engine, engine)
	done := make(chan int, 1)
	runner := plugin.GetInternalPluginRunner("bgp-filter-community")
	require.NotNil(t, runner)
	go func() { done <- runner(pluginConn) }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() {
		_ = conn.Close()
		select {
		case <-done:
		case <-ctx.Done():
			t.Error("community plugin did not stop after closing its RPC connection")
		}
	})
	request, err := conn.ReadRequest(ctx)
	require.NoError(t, err)
	require.Equal(t, rpc.MethodDeclareRegistration, request.Method)
	require.NoError(t, conn.SendOK(ctx, request.ID))
	peer := func(address, direction, strip string) map[string]any {
		return map[string]any{
			"connection": map[string]any{"remote": map[string]any{"ip": address}},
			"filter":     map[string]any{direction: map[string]any{"community": map[string]any{"strip": []any{strip}}}},
		}
	}
	cfg := map[string]any{
		"community": map[string]any{"extended": map[string]any{
			"encapsulation": map[string]any{"value": []any{"030c000000000002", "030c000000000008"}},
			"no-match":      map[string]any{"value": []any{"030c000000000009"}},
		}},
		"peer": map[string]any{
			"ingress-remove": peer("192.0.2.1", "ingress", "encapsulation"),
			"ingress-keep":   peer("192.0.2.1", "ingress", "no-match"),
			"egress-remove":  peer("192.0.2.2", "egress", "encapsulation"),
			"egress-keep":    peer("192.0.2.2", "egress", "no-match"),
		},
	}
	encoded, err := json.Marshal(cfg)
	require.NoError(t, err)
	_, err = conn.CallRPC(ctx, "ze-plugin-callback:configure", rpc.ConfigureInput{Sections: []rpc.ConfigSection{{Root: "bgp", Data: string(encoded)}}})
	require.NoError(t, err)
	request, err = conn.ReadRequest(ctx)
	require.NoError(t, err)
	require.Equal(t, "ze-plugin-engine:declare-capabilities", request.Method)
	require.NoError(t, conn.SendOK(ctx, request.ID))
	_, err = conn.CallRPC(ctx, "ze-plugin-callback:share-registry", map[string]any{"commands": []any{}})
	require.NoError(t, err)
	request, err = conn.ReadRequest(ctx)
	require.NoError(t, err)
	require.Equal(t, "ze-plugin-engine:ready", request.Method)
	require.NoError(t, conn.SendOK(ctx, request.ID))
	t.Cleanup(func() {
		_, err := conn.CallRPC(ctx, "ze-plugin-callback:configure", rpc.ConfigureInput{Sections: []rpc.ConfigSection{{Root: "bgp", Data: "{}"}}})
		require.NoError(t, err)
	})
}

// RFC requirement: RFC9012-11-9 positive -- configured exact-value ingress stripping removes both received Encapsulation ECs before dispatch and subsequent export.
// RFC requirement: RFC9012-11-9 negative -- a populated no-match ingress policy preserves the identical Encapsulation EC input; unrelated RT, route attributes and NLRI survive both policies.
func TestRFC9012ConfiguredEncapsulationECIngress(t *testing.T) {
	teConfigureCommunityFilter(t)
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "preserve", true: "remove"}[remove], func(t *testing.T) {
			teEncapsulationFilterRoundTrip(t, true, remove)
		})
	}
}

// RFC requirement: RFC9012-11-10 positive -- configured destination stripping reaches the registered Extended Communities handler and removes both Encapsulation ECs from actual outgoing UPDATE bytes.
// RFC requirement: RFC9012-11-10 negative -- a destination with populated no-match policy receives both original Encapsulation ECs; unrelated RT, route attributes and NLRI survive both policies.
func TestRFC9012ConfiguredEncapsulationECEgress(t *testing.T) {
	teConfigureCommunityFilter(t)
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "preserve", true: "remove"}[remove], func(t *testing.T) {
			teEncapsulationFilterRoundTrip(t, false, remove)
		})
	}
}

func teEncapsulationFilterRoundTrip(t *testing.T, ingress, remove bool) {
	t.Helper()
	encap := []byte{3, 12, 0, 0, 0, 0, 0, 2, 3, 12, 0, 0, 0, 0, 0, 8}
	rt := []byte{0, 2, 0xfd, 0xe9, 0, 0, 0, 7}
	ext := slices.Concat(encap, rt)
	attrs := []byte{
		0x40, 1, 1, 0,
		0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xea,
		0x40, 3, 4, 192, 0, 2, 1,
	}
	body := makeUpdateBody(nil, slices.Concat(attrs, makeAttr(0xc0, 16, ext)), []byte{24, 10, 30, 0})
	source, _ := tePropagationPeer(t, "192.0.2.1", 65002, family.IPv4Unicast)
	destination, socket := tePropagationPeer(t, "192.0.2.2", 65001, family.IPv4Unicast)
	suffix := "keep"
	if remove {
		suffix = "remove"
	}
	if ingress {
		source.Settings().Name = "ingress-" + suffix
	} else {
		destination.Settings().Name = "egress-" + suffix
	}
	source.refreshForwardFacts()
	destination.refreshForwardFacts()
	cache := newRecentUpdateCache(16)
	t.Cleanup(cache.Stop)
	cache.RegisterConsumer("tunnel-forwarder")
	r := &Reactor{
		clock: source.clock, config: &Config{LocalAS: 65001},
		peers:         map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source, destination.Settings().PeerKey(): destination},
		recentUpdates: cache, attrModHandlers: attrModHandlersWithDefaults(),
	}
	for _, f := range filterapi.IngressOrdered() {
		if f.Name == "bgp-filter-community" {
			r.orderedIngressSteps = append(r.orderedIngressSteps, orderedIngressStep{name: f.Name, stage: f.Stage, priority: f.Priority, inproc: f.Ingress})
		}
	}
	for _, f := range filterapi.EgressOrdered() {
		if f.Name == "bgp-filter-community" {
			r.orderedEgressSteps = append(r.orderedEgressSteps, orderedEgressStep{name: f.Name, stage: f.Stage, priority: f.Priority, inproc: f.Egress})
		}
	}
	require.Len(t, r.orderedIngressSteps, 1)
	require.Len(t, r.orderedEgressSteps, 1)
	r.fwdPool = newFwdPool(fwdBatchHandler, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: destination.Settings().PeerKey()}, 4096)
	t.Cleanup(r.fwdPool.Stop)
	assertBody := func(got, wantExt []byte) {
		sections, err := wire.ParseUpdateSections(got)
		require.NoError(t, err)
		require.Equal(t, []byte{24, 10, 30, 0}, sections.NLRI(got))
		require.Empty(t, sections.Withdrawn(got))
		count, value := countAttrCode(sections.Attrs(got), uint8(attribute.AttrExtCommunity))
		require.Equal(t, 1, count)
		require.Equal(t, wantExt, value)
		for _, code := range []uint8{1, 2, 3} {
			_, want := countAttrCode(attrs, code)
			count, value := countAttrCode(sections.Attrs(got), code)
			require.Equal(t, 1, count)
			require.Equal(t, want, value)
		}
	}
	var id uint64
	r.setMessageReceiver(&testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
		id = msg.MessageID
		want := ext
		if ingress && remove {
			want = rt
		}
		assertBody(msg.WireUpdate.Payload(), want)
	}})
	source.session.onMessageReceived = r.notifyMessageReceiver
	header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))}
	err, kept := source.session.processMessage(&header, body, BufHandle{ID: noPoolBufID, Buf: body})
	require.NoError(t, err)
	require.True(t, kept)
	require.NotZero(t, id)
	sel, err := selector.Parse(destination.Settings().Address.String())
	require.NoError(t, err)
	require.NoError(t, (&reactorAPIAdapter{r: r}).ForwardUpdate(sel, id, "tunnel-forwarder", plugin.ProcessSender("tunnel-forwarder")))
	forwardSocketBarrier(t, r)
	bodies := aigpSocketBodies(t, socket)
	require.Len(t, bodies, 1)
	want := ext
	if remove {
		want = rt
	}
	assertBody(bodies[0], want)
	downstream, _ := tePropagationPeer(t, "192.0.2.9", 65001, family.IPv4Unicast)
	received := 0
	downstream.session.onMessageReceived = func(_ netip.Addr, typ msgtype.MessageType, _ []byte, wu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection, _ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		received++
		require.Equal(t, msgtype.TypeUPDATE, typ)
		assertBody(wu.Payload(), want)
		return false
	}
	header.Length = uint16(message.HeaderLen + len(bodies[0]))
	err, kept = downstream.session.processMessage(&header, bodies[0], BufHandle{ID: noPoolBufID, Buf: bodies[0]})
	require.NoError(t, err)
	require.False(t, kept)
	require.Equal(t, 1, received)
}
