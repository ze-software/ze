package flowspecfirewall

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"

	// The extended-community JSON formatter registers from this plugin's
	// init(). The daemon links it, so a test that does not is asking the
	// writer a question the daemon never asks.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community"
)

// This file drives the daemon's own event writer. Every other test in the
// package starts from a JSON string, and a string is only ever as right as its
// author: the bridge shipped reading a flat envelope with the event type at the
// top level, while format.AppendMessage has always written the ze-bgp envelope
// with the type in message.type and the body under "update". Nothing in the
// suite compared the two, so the plugin never worked end to end.

// discardTrafficRate is the RFC 8955 Section 7.1 traffic-rate extended
// community with a rate of zero, which the same section defines as discard.
// Type 0x80, subtype 0x06, a 2-octet AS and a 4-octet IEEE 754 rate.
var discardTrafficRate = []byte{0x80, 0x06, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x00}

// daemonPeer is the peer the writer renders into the event's peer object.
func daemonPeer() *plugin.PeerInfo {
	return &plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
		Name:         "peer1",
		LocalAS:      65001,
		PeerAS:       65001,
	}
}

// daemonEncodingContext registers the encoding context a received UPDATE
// carries, as the reactor does before it hands the message to the writer.
func daemonEncodingContext() bgpctx.ContextID {
	ctx := bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65001, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true},
		bgpctx.DirectionRecv,
	)
	id, _ := bgpctx.Registry.Register(ctx)
	return id
}

// appendPathAttribute appends one path attribute (flags, code, length, value).
func appendPathAttribute(dst []byte, flags byte, code attribute.AttributeCode, value []byte) []byte {
	dst = append(dst, flags, byte(code), byte(len(value)))
	return append(dst, value...)
}

// appendMPReachFlowSpec appends an MP_REACH_NLRI (RFC 4760 Section 3) carrying
// one FlowSpec NLRI: AFI, SAFI, next-hop length, next-hop, the reserved octet,
// then the NLRI itself.
//
// The next-hop is present and RFC 8955 Section 4 says a FlowSpec receiver
// ignores it, which is what TestRFC8955NextHopIgnoredForFlowSpec asserts.
func appendMPReachFlowSpec(dst []byte, fam family.Family, nextHop netip.Addr, nlri []byte) []byte {
	value := make([]byte, 3)
	binary.BigEndian.PutUint16(value[0:2], uint16(fam.AFI))
	value[2] = byte(fam.SAFI)
	nh := nextHop.AsSlice()
	value = append(value, byte(len(nh)))
	value = append(value, nh...)
	value = append(value, 0x00) // Reserved.
	value = append(value, nlri...)
	return appendPathAttribute(dst, 0x80, attribute.AttrMPReachNLRI, value)
}

// appendMPUnreachFlowSpec appends an MP_UNREACH_NLRI (RFC 4760 Section 4)
// withdrawing one FlowSpec NLRI.
func appendMPUnreachFlowSpec(dst []byte, fam family.Family, nlri []byte) []byte {
	value := make([]byte, 3)
	binary.BigEndian.PutUint16(value[0:2], uint16(fam.AFI))
	value[2] = byte(fam.SAFI)
	value = append(value, nlri...)
	return appendPathAttribute(dst, 0x80, attribute.AttrMPUnreachNLRI, value)
}

// daemonFlowSpecEvent builds the UPDATE a peer puts on the wire and returns the
// JSON line the daemon writes for it, byte for byte, through
// format.AppendMessage. withdraw selects MP_UNREACH over MP_REACH.
func daemonFlowSpecEvent(t *testing.T, extComm []byte, withdraw bool, comps ...flowspec.FlowComponent) string {
	t.Helper()

	fam := flowFamily()
	fs := flowspec.NewFlowSpec(fam)
	for _, c := range comps {
		require.NoError(t, fs.AddComponent(c))
	}

	var attrs []byte
	attrs = appendPathAttribute(attrs, 0x40, attribute.AttrOrigin, []byte{0x00})
	attrs = appendPathAttribute(attrs, 0x40, attribute.AttrASPath, nil)
	if len(extComm) > 0 {
		attrs = appendPathAttribute(attrs, 0xc0, attribute.AttrExtCommunity, extComm)
	}
	if withdraw {
		attrs = appendMPUnreachFlowSpec(attrs, fam, fs.Bytes())
	} else {
		attrs = appendMPReachFlowSpec(attrs, fam, netip.MustParseAddr("192.0.2.1"), fs.Bytes())
	}

	body := make([]byte, 4, 4+len(attrs))
	binary.BigEndian.PutUint16(body[0:2], 0) // No withdrawn IPv4 routes.
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	body = append(body, attrs...)

	wireUpdate := wireu.NewWireUpdate(body, daemonEncodingContext())
	attrsWire, err := wireUpdate.Attrs()
	require.NoError(t, err, "the fixture attribute block must parse")
	require.NotNil(t, attrsWire, "the fixture carries attributes, so AttrsWire must not be nil")

	msg := bgptypes.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   body,
		Timestamp:  time.Now(),
		MessageID:  1,
		AttrsWire:  attrsWire,
		WireUpdate: wireUpdate,
	}
	content := bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatParsed}
	return string(format.AppendMessage(nil, daemonPeer(), msg, content))
}

// TestDaemonFlowSpecUpdateReachesTheRuleTable is the end-to-end assertion this
// bridge never had.
//
// VALIDATES: a FlowSpec route announced by a peer, rendered by the daemon's own
// event writer, reaches the bridge's rule table as a drop term.
// PREVENTS: the bridge reading an envelope no writer produces. handleEvent
// switched on a top-level "type" of "state" or "update"; every event the daemon
// writes carries "bgp" there and the kind in message.type, so every FlowSpec
// route was dropped by a switch with no default branch, in silence, and the
// plugin had never worked end to end.
func TestDaemonFlowSpecUpdateReachesTheRuleTable(t *testing.T) {
	event := daemonFlowSpecEvent(t, discardTrafficRate, false,
		flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.1.0.0/24")),
		flowspec.NewFlowIPProtocolComponent(6),
		flowspec.NewFlowDestPortComponent(80),
	)

	b := testBridge()
	require.NoError(t, b.handleEvent(event))

	tables := b.rules.buildTable()
	require.Len(t, tables, 1, "the announced FlowSpec route must reach the rule table: %s", event)
	require.Len(t, tables[0].Chains, 1)
	require.Len(t, tables[0].Chains[0].Terms, 1)

	term := tables[0].Chains[0].Terms[0]
	assert.ElementsMatch(t, []firewall.Match{
		firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("10.1.0.0/24")},
		firewall.MatchProtocol{Protocol: "tcp"},
		firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 80}}},
	}, term.Matches, "every component the peer announced must reach the term")
	require.Len(t, term.Actions, 1)
	assert.Equal(t, firewall.Drop{}, term.Actions[0])
}

// TestDaemonFlowSpecWithdrawRemovesTheRule covers the other half of the same
// envelope: a withdrawal arrives as an MP_UNREACH_NLRI and the writer renders
// it as action "del" under the family key.
//
// VALIDATES: a withdrawal from the daemon's writer removes the rule the
// matching announcement installed.
// PREVENTS: a peer's rule outliving the route that asked for it, which leaves
// traffic dropped after the peer said to stop dropping it.
func TestDaemonFlowSpecWithdrawRemovesTheRule(t *testing.T) {
	comps := []flowspec.FlowComponent{
		flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.1.0.0/24")),
	}
	// The announcement carries the traffic action, which only the extended
	// community renderer can put in the event, so it comes from the envelope
	// helper. Its NLRI is the writer's own output, so the rule key the
	// withdrawal computes is the key the announcement stored.
	announce := daemonUpdateJSON("10.0.0.1", []string{"rate-limit:0"}, daemonOp{
		action: "add",
		nlri:   []string{string(realNLRIJSON(t, flowFamily(), comps...))},
	})

	b := testBridge()
	require.NoError(t, b.handleEvent(announce))
	require.NotNil(t, b.rules.buildTable(), "the announcement must install the rule first")

	require.NoError(t, b.handleEvent(daemonFlowSpecEvent(t, nil, true, comps...)))
	assert.Nil(t, b.rules.buildTable(), "the withdrawal must remove the rule the announcement installed")
}

// TestDaemonPeerDownDropsTheRules drives the state-change writer.
//
// VALIDATES: the down event appendStateChangeJSON writes removes every rule
// learned from that peer.
// PREVENTS: the state envelope going the same way as the update envelope. It
// carries the kind in message.type and the state beside the peer, and the flat
// reader matched neither.
func TestDaemonPeerDownDropsTheRules(t *testing.T) {
	b := testBridge()
	// As in the withdrawal test, the announcement comes from the envelope
	// helper because only it can carry a traffic action today; the state event
	// under test is the writer's own.
	require.NoError(t, b.handleEvent(daemonUpdateJSON("10.0.0.1", []string{"rate-limit:0"}, daemonOp{
		action: "add",
		nlri: []string{string(realNLRIJSON(t, flowFamily(),
			flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.1.0.0/24"))))},
	})))
	require.NotNil(t, b.rules.buildTable(), "the announcement must install the rule first")

	down := string(format.AppendStateChange(nil, daemonPeer(), rpc.SessionStateDown,
		"hold timer expired", nil, plugin.EncodingJSON))
	require.NoError(t, b.handleEvent(down))
	assert.Nil(t, b.rules.buildTable(), "a peer that went down keeps no rules: %s", down)
}
