// Design: docs/architecture/wire/attributes.md -- Tunnel Encapsulation propagation.
// Related: filter_ordered.go -- configured destination export chains and raw overrides.
// Related: rfc9012_tunnel_encap_carry_test.go -- receive/cache/socket fixtures.
// This proof covers session-scoped attribute-23 filtering, not EBGP defaults,
// ingress filtering, attribute-16 filtering, or tunnel dataplane consumption.

package reactor

import (
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
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

// TestRFC9012TunnelFilterSelectedSession forwards one received announcement to
// two independent sessions, whose configured export chains remove or retain
// attribute 23. Both socket bodies are consumed by real downstream sessions.
// RFC 9012 Section 11: "This filtering SHOULD be possible on a per-BGP-session basis."
// MUTATION: in runEgressPolicyChainASN4, discard a nonempty raw override and
// return accept without wireOverride; the selected session then leaks attr23.
// RFC requirement: RFC9012-11-7 positive -- the selected destination's configured raw export chain removes attribute 23 on socket output and downstream receipt while preserving the announcement, unrelated attributes and cached source.
func TestRFC9012TunnelFilterSelectedSession(t *testing.T) {
	// RFC 9012 Section 11.
	teSessionFilterRoundTrip(t, 0)
}

// TestRFC9012TunnelFilterDoesNotAffectOtherSession swaps the configured filter
// to the other destination. The formerly filtered address must now retain the
// complete original attribute, while the other address loses it.
// RFC 9012 Section 11: "This filtering SHOULD be possible on a per-BGP-session basis."
// MUTATION: in forwardUpdateSection, substitute the stripping destination's
// exportFilters for every destination's facts.exportFilters; the other session
// must fail its exact original-attribute assertion rather than inherit the strip.
// RFC requirement: RFC9012-11-7 negative -- with the filtering destination swapped, its sibling retains exact original attribute-23 bytes on socket output and downstream receipt; filtering must not leak into the sibling or mutate the received/cached announcement.
func TestRFC9012TunnelFilterDoesNotAffectOtherSession(t *testing.T) {
	// RFC 9012 Section 11.
	teSessionFilterRoundTrip(t, 1)
}

const (
	teSessionFilterPlugin = "tunnel-policy"
	teSessionFilterRemove = "remove-tunnel"
	teSessionFilterKeep   = "keep-tunnel"
)

// teSessionFilterRoundTrip keeps receive dispatch, cache ownership, destination
// facts, PolicyFilterChain, forward workers and socket writes on production paths.
// Only the external plugin transport is replaced: it transforms the raw request
// supplied by policyFilterFunc, never a precomputed output or a callback echo.
// RFC 9012 Section 11: "In addition, any BGP speaker that understands the attribute
// MUST be able to filter the attribute from outgoing BGP UPDATE messages."
func teSessionFilterRoundTrip(t *testing.T, filtered int) {
	t.Helper()
	// RFC 9012 Sections 2 and 13: a valid received tunnel with one endpoint.
	body := teCarryBody(teCarryValue(0, false))
	original := bytes.Clone(body)
	source, _ := tePropagationPeer(t, "192.0.2.1", 65002, family.IPv4Unicast)
	first, firstSocket := tePropagationPeer(t, "192.0.2.2", 65001, family.IPv4Unicast)
	second, secondSocket := tePropagationPeer(t, "192.0.2.3", 65001, family.IPv4Unicast)
	destinations := []*Peer{first, second}
	sockets := []*recordingConn{firstSocket, secondSocket}
	cache := newRecentUpdateCache(16)
	t.Cleanup(cache.Stop)
	cache.RegisterConsumer("tunnel-forwarder")
	r := &Reactor{
		clock: source.clock, config: &Config{LocalAS: 65001},
		peers: map[netip.AddrPort]*Peer{
			source.Settings().PeerKey(): source,
			first.Settings().PeerKey():  first,
			second.Settings().PeerKey(): second,
		},
		recentUpdates:       cache,
		attrModHandlers:     attrModHandlersWithDefaults(),
		filterTransportSeam: teSessionFilterTransport{},
		orderedEgressSteps: []orderedEgressStep{{
			name: policyChainStepName, stage: filterapi.FilterStagePeerChain, policyChain: true,
		}},
	}
	r.fwdPool = newFwdPool(fwdBatchHandler, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(r.fwdPool.Stop)
	for i, destination := range destinations {
		filter := teSessionFilterKeep
		if i == filtered {
			filter = teSessionFilterRemove
		}
		destination.settings.ExportFilters = []filterapi.FilterRef{{Name: teSessionFilterPlugin + ":" + filter}}
		destination.refreshForwardFacts()
		r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: destination.Settings().PeerKey()}, 4096)
	}

	var id uint64
	r.setMessageReceiver(&testDeliveryReceiver{
		consumerCount: 1,
		onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
			id = msg.MessageID
			if !bytes.Equal(msg.WireUpdate.Payload(), original) {
				t.Error("source receive dispatch changed the valid announcement")
			}
		},
	})
	source.session.onMessageReceived = r.notifyMessageReceiver
	header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))}
	// RFC 9012 Sections 11 and 13: receive validation precedes export policy.
	err, kept := source.session.processMessage(&header, body, BufHandle{ID: noPoolBufID, Buf: body})
	if err != nil {
		t.Fatal(err)
	}
	if !kept {
		t.Fatal("valid received announcement was not retained for dispatch")
	}
	if id == 0 {
		t.Fatal("received announcement never reached the forward cache")
	}
	// Retain MUST precede forwarding's consumer Ack so the cache can be inspected;
	// the paired cleanup MUST release this test's retain after the socket barrier.
	if !cache.Retain(id) {
		t.Fatal("cannot retain received announcement")
	}
	t.Cleanup(func() {
		if !cache.Release(id) {
			t.Error("received announcement vanished before retained-cache release")
		}
	})
	cached, ok := cache.Get(id)
	if !ok {
		t.Fatal("received announcement absent from cache")
	}
	if !bytes.Equal(cached.WireUpdate.Payload(), original) {
		t.Fatal("cache does not contain the original received announcement")
	}
	sel, err := selector.Parse("*")
	if err != nil {
		t.Fatal(err)
	}
	// RFC 9012 Section 11: one ForwardUpdate fans the same cache entry to both sessions.
	if err := (&reactorAPIAdapter{r: r}).ForwardUpdate(sel, id, "tunnel-forwarder", plugin.ProcessSender("tunnel-forwarder")); err != nil {
		t.Fatal(err)
	}
	forwardSocketBarrier(t, r)
	for i, socket := range sockets {
		bodies := aigpSocketBodies(t, socket)
		if len(bodies) != 1 {
			t.Fatalf("destination %d: socket announcements = %d, want 1", i, len(bodies))
		}
		// RFC 9012 Section 11: judge actual written bytes, not the policy answer.
		teSessionFilterAssertBody(t, bodies[0], original, i == filtered)
		// RFC 9012 Section 11: the downstream consumer must accept the same announcement.
		teSessionFilterDownstream(t, bodies[0], original, i == filtered)
	}
	cached, ok = cache.Get(id)
	if !ok {
		t.Fatal("forwarding evicted the explicitly retained source")
	}
	if !bytes.Equal(cached.WireUpdate.Payload(), original) {
		t.Error("per-session export mutated the cached source announcement")
	}
	if !bytes.Equal(body, original) {
		t.Error("per-session export mutated the original receive buffer")
	}
}

// teSessionFilterTransport is a stateless raw-policy plugin fixture. Its zero
// value is usable and safe for concurrent calls; no callback owns test state.
// It substitutes only the external IPC endpoint, not the reactor's policy chain.
type teSessionFilterTransport struct{}

func (teSessionFilterTransport) FilterInfo(_, _ string) ([]string, bool) {
	return nil, true
}

func (teSessionFilterTransport) FilterOnError(_, _ string) rpc.OnErrorPolicy {
	return rpc.OnErrorReject
}

// CallFilterUpdate removes only attribute 23 from the raw UPDATE supplied by the
// real policy request. Rebuilding owns new bytes, as an external plugin would.
// RFC 9012 Section 11: "In addition, any BGP speaker that understands the attribute
// MUST be able to filter the attribute from outgoing BGP UPDATE messages."
func (teSessionFilterTransport) CallFilterUpdate(_ context.Context, pluginName string, input *rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
	if pluginName != teSessionFilterPlugin {
		return nil, fmt.Errorf("unexpected filter plugin %q", pluginName)
	}
	if input.Direction != "export" {
		return nil, fmt.Errorf("unexpected policy direction %q", input.Direction)
	}
	if input.Filter == teSessionFilterKeep {
		return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
	}
	if input.Filter != teSessionFilterRemove {
		return nil, fmt.Errorf("unexpected filter %q", input.Filter)
	}
	sections, err := wire.ParseUpdateSections(input.Raw)
	if err != nil {
		return nil, fmt.Errorf("raw export request: %w", err)
	}
	attrs := sections.Attrs(input.Raw)
	retained := make([]byte, 0, len(attrs))
	for len(attrs) > 0 {
		_, code, length, headerLen, err := attribute.ParseHeader(attrs)
		if err != nil {
			return nil, fmt.Errorf("raw export attribute: %w", err)
		}
		size := headerLen + int(length)
		if size > len(attrs) {
			return nil, fmt.Errorf("raw export attribute %d exceeds remaining bytes", code)
		}
		if code != attribute.AttrTunnelEncap {
			retained = append(retained, attrs[:size]...)
		}
		attrs = attrs[size:]
	}
	return &rpc.FilterUpdateOutput{
		Action: rpc.FilterModify,
		Raw:    makeUpdateBody(sections.Withdrawn(input.Raw), retained, sections.NLRI(input.Raw)),
	}, nil
}

// teSessionFilterAssertBody compares complete attribute encodings, including the
// original attribute-23 header, not only decoded tunnel values. The only normal
// export addition is mandatory IBGP LOCAL_PREF; even attribute 16 stays unchanged.
// RFC 9012 Section 11: "This filtering SHOULD be possible on a per-BGP-session basis."
func teSessionFilterAssertBody(t *testing.T, got, original []byte, filtered bool) {
	t.Helper()
	before, err := wire.ParseUpdateSections(original)
	if err != nil {
		t.Fatal(err)
	}
	after, err := wire.ParseUpdateSections(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after.NLRI(got), before.NLRI(original)) {
		t.Fatalf("exported NLRI = %x, want %x", after.NLRI(got), before.NLRI(original))
	}
	if !bytes.Equal(after.Withdrawn(got), before.Withdrawn(original)) {
		t.Fatal("export synthesized a withdrawal")
	}
	want := teSessionFilterAttributes(t, before.Attrs(original))
	if len(want[attribute.AttrTunnelEncap]) == 0 {
		t.Fatal("source fixture lacks attribute 23")
	}
	if filtered {
		delete(want, attribute.AttrTunnelEncap)
	}
	want[attribute.AttrLocalPref] = []byte{0x40, 5, 4, 0, 0, 0, 100}
	actual := teSessionFilterAttributes(t, after.Attrs(got))
	if len(actual) != len(want) {
		t.Errorf("exported attribute count = %d, want %d (filtered=%t)", len(actual), len(want), filtered)
	}
	for code, encoded := range want {
		if !bytes.Equal(actual[code], encoded) {
			t.Errorf("attribute %d = %x, want original encoding %x", code, actual[code], encoded)
		}
	}
	if filtered {
		if _, found := actual[attribute.AttrTunnelEncap]; found {
			t.Error("selected filtering destination received attribute 23")
		}
	}
}

// teSessionFilterAttributes bounds iteration by the attribute section and rejects
// duplicates or truncated tails so exact-count assertions cannot hide extras.
func teSessionFilterAttributes(t *testing.T, attrs []byte) map[attribute.AttributeCode][]byte {
	t.Helper()
	encoded := make(map[attribute.AttributeCode][]byte)
	it := attribute.NewAttrIterator(attrs)
	for it.Remaining() > 0 {
		start := it.Offset()
		code, _, _, ok := it.Next()
		if !ok {
			t.Fatal("malformed attribute section")
		}
		if _, duplicate := encoded[code]; duplicate {
			t.Fatalf("duplicate attribute %d", code)
		}
		encoded[code] = attrs[start:it.Offset()]
	}
	return encoded
}

// teSessionFilterDownstream observes processMessage's received-route consumer,
// after the real socket writer, and requires an accepted UPDATE rather than TAW.
// RFC 9012 Section 11: "This filtering SHOULD be possible on a per-BGP-session basis."
func teSessionFilterDownstream(t *testing.T, sent, original []byte, filtered bool) {
	t.Helper()
	downstream, _ := tePropagationPeer(t, "192.0.2.9", 65001, family.IPv4Unicast)
	received := 0
	downstream.session.onMessageReceived = func(_ netip.Addr, typ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		received++
		if typ != msgtype.TypeUPDATE {
			t.Errorf("downstream received message type %d, want UPDATE", typ)
		}
		// RFC 9012 Section 11.
		teSessionFilterAssertBody(t, wu.Payload(), original, filtered)
		if !bytes.Equal(wu.Payload(), sent) {
			t.Error("downstream rewrote the socket announcement")
		}
		return false
	}
	header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(sent))}
	// RFC 9012 Sections 11 and 13.
	err, kept := downstream.session.processMessage(&header, sent, BufHandle{ID: noPoolBufID, Buf: sent})
	if err != nil {
		t.Fatal(err)
	}
	if kept {
		t.Error("downstream retained a buffer its consumer released")
	}
	if received != 1 {
		t.Errorf("downstream received announcements = %d, want 1", received)
	}
}
