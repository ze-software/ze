// Design: docs/architecture/wire/nlri-bgpls.md -- native origination contract
// RFC: rfc/short/rfc9552.md
// RFC: rfc/short/rfc9086.md

package ls

import (
	"context"
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: the Protocol-ID of every native NLRI is the one of the protocol
// the source derived it from, and a source with no Protocol-ID is refused.
// PREVENTS: an IS-IS or OSPF object advertised under another protocol's ID.
func TestRFC9552NativeProtocolIDFollowsSource(t *testing.T) {
	// RFC requirement: RFC9552-5.2-3 positive -- an IS-IS Level 1 snapshot emits Protocol-ID 1 and an OSPFv2 snapshot emits Protocol-ID 3.
	for _, tc := range []struct {
		protocol linkstateevents.Protocol
		want     byte
	}{{linkstateevents.ISISLevel1, 1}, {linkstateevents.OSPFv2, 3}} {
		e, capture := exportFixture(t)
		snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: tc.protocol}, Generation: 1,
			Nodes: []linkstateevents.Node{{ID: nativeTestNode()}}}
		require.NoError(t, e.replace("isis", snapshot))
		require.NoError(t, e.reconcile(context.Background()))
		require.Len(t, capture.commands, 1)
		require.Equal(t, tc.want, exportCommandBytes(t, capture.commands[0], "nlri")[4])
	}
	// RFC requirement: RFC9552-5.2-3 negative -- a snapshot whose protocol has no BGP-LS Protocol-ID is refused and emits no UPDATE.
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.BGP + 1}, Generation: 1,
		Nodes: []linkstateevents.Node{{ID: nativeTestNode()}}}
	require.Error(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Empty(t, capture.commands)
}

// VALIDATES: when a link's descriptor TLVs change, the old NLRI is withdrawn
// before the new one is announced, while an attribute-only change announces
// the same NLRI again without a withdrawal.
// PREVENTS: two NLRIs for one link, or a withdrawal the change did not need.
func TestRFC9552NativeDescriptorChangeWithdrawsFirst(t *testing.T) {
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1}, Generation: 1,
		Links: []linkstateevents.Link{{Local: nativeTestNode(), Remote: nativeTestNode(),
			LocalAddresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}, Attributes: []linkstateevents.TLV{{Type: 1089, Value: []byte{0, 0, 0, 1}}}}}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
	old := exportCommandBytes(t, capture.commands[0], "nlri")
	// RFC requirement: RFC9552-5.2-6 positive -- changing a link's address descriptor withdraws the old NLRI before announcing the new one.
	snapshot.Generation = 2
	snapshot.Links[0].LocalAddresses = []netip.Addr{netip.MustParseAddr("192.0.2.9")}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 3)
	require.Contains(t, capture.commands[1], " del ")
	require.Equal(t, old, exportCommandBytes(t, capture.commands[1], "nlri"))
	require.NotContains(t, capture.commands[2], " del ")
	current := exportCommandBytes(t, capture.commands[2], "nlri")
	require.NotEqual(t, old, current)
	// RFC requirement: RFC9552-5.2-6 negative -- changing only a link attribute announces the same NLRI again with no withdrawal.
	snapshot.Generation = 3
	snapshot.Links[0].Attributes = []linkstateevents.TLV{{Type: 1089, Value: []byte{0, 0, 0, 2}}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 4)
	require.NotContains(t, capture.commands[3], " del ")
	require.Equal(t, current, exportCommandBytes(t, capture.commands[3], "nlri"))
}

// VALIDATES: the complementary polarities of the Link Descriptor rules: an
// address that cannot be carried is refused rather than dropped, and a link
// with global addresses beside link-local ones and link identifiers is still
// advertised by its global address TLVs.
// PREVENTS: silently omitting a present address, or suppressing the link.
func TestRFC9552NativeLinkDescriptorComplements(t *testing.T) {
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1}, Generation: 1,
		Links: []linkstateevents.Link{{Local: nativeTestNode(), Remote: nativeTestNode(), HasLinkIDs: true, LocalID: 4, RemoteID: 8,
			LocalAddresses:  []netip.Addr{netip.MustParseAddr("fe80::1"), netip.MustParseAddr("2001:db8::1")},
			RemoteAddresses: []netip.Addr{netip.MustParseAddr("169.254.0.2"), netip.MustParseAddr("192.0.2.2")}}}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
	wire := exportCommandBytes(t, capture.commands[0], "nlri")
	local := netip.MustParseAddr("2001:db8::1").As16()
	// RFC requirement: RFC9552-5.2.2-2 positive -- a link with global addresses and link identifiers is advertised, identified by its address TLVs 261 and 260.
	// RFC requirement: RFC9552-5.2.2-3 positive -- the global addresses beside link-local ones are carried in TLVs 261 and 260, exactly once each.
	require.Equal(t, [][]byte{local[:]}, exportTLVValues(t, wire[13:], 261))
	require.Equal(t, [][]byte{{192, 0, 2, 2}}, exportTLVValues(t, wire[13:], 260))
	// RFC requirement: RFC9552-5.2.2-1 negative -- a link whose local address is invalid is refused, not advertised with that address dropped.
	snapshot.Generation = 2
	snapshot.Links[0].LocalAddresses = []netip.Addr{{}, netip.MustParseAddr("2001:db8::1")}
	require.Error(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
}

// VALIDATES: a link in a non-default topology carries that topology's
// Multi-Topology Identifier TLV, and is never advertised without it.
// PREVENTS: a non-default-topology link advertised as a default one.
func TestRFC9552NativeLinkNonDefaultTopology(t *testing.T) {
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1}, Generation: 1,
		Links: []linkstateevents.Link{{Local: nativeTestNode(), Remote: nativeTestNode(), HasLinkIDs: true, LocalID: 1, RemoteID: 2, Topologies: []uint16{2}}}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
	wire := exportCommandBytes(t, capture.commands[0], "nlri")
	// RFC requirement: RFC9552-5.2.2-5 positive -- a link in topology 2 carries TLV 263 with MT-ID 2.
	require.Equal(t, [][]byte{{0, 2}}, exportTLVValues(t, wire[13:], 263))
	// RFC requirement: RFC9552-5.2.2-5 negative -- the topology 2 link emits no second NLRI without TLV 263 or with MT-ID 0.
	for _, command := range capture.commands {
		values := exportTLVValues(t, exportCommandBytes(t, command, "nlri")[13:], 263)
		require.Len(t, values, 1)
		require.NotEqual(t, []byte{0, 0}, values[0])
	}
}

// VALIDATES: an OSPF prefix whose route type the source knows carries the
// OSPF Route Type TLV in its Prefix Descriptor.
// PREVENTS: dropping the route type the LSA signaled.
func TestRFC9552NativeOSPFRouteType(t *testing.T) {
	// RFC requirement: RFC9552-5.2.3.1-1 positive -- an OSPFv2 prefix with route type 1 (Intra-Area) carries TLV 264 with value 1.
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.OSPFv2}, Generation: 1,
		Prefixes: []linkstateevents.Prefix{{Node: nativeTestNode(), Prefix: netip.MustParsePrefix("198.51.100.0/24"), RouteType: 1}}}
	require.NoError(t, e.replace("ospf", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
	require.Equal(t, [][]byte{{1}}, exportTLVValues(t, exportCommandBytes(t, capture.commands[0], "nlri")[13:], 264))
}

// VALIDATES: a prefix whose bits are already masked reaches the collector with
// exactly its prefix octets.
// PREVENTS: altering a conforming prefix while clearing trailing bits.
func TestRFC9552NativeMaskedPrefixUnchanged(t *testing.T) {
	// RFC requirement: RFC9552-5.2.3-1 positive -- a masked 198.51.100.128/25 prefix is emitted as IP Reachability Information 25, 198.51.100.128.
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1}, Generation: 1,
		Prefixes: []linkstateevents.Prefix{{Node: nativeTestNode(), Prefix: netip.MustParsePrefix("198.51.100.128/25")}}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
	require.Equal(t, [][]byte{{25, 198, 51, 100, 128}}, exportTLVValues(t, exportCommandBytes(t, capture.commands[0], "nlri")[13:], 265))
}

// VALIDATES: an IS-IS MT-ID with any of its four reserved R bits set is refused.
// PREVENTS: originating an IS-IS Multi-Topology Identifier with R bits set.
func TestRFC9552NativeISISTopologyReservedBits(t *testing.T) {
	// RFC requirement: RFC9552-5.2.2.1-1 negative -- an IS-IS link in topology 4096, which sets an R bit, is refused and emits no UPDATE.
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1}, Generation: 1,
		Links: []linkstateevents.Link{{Local: nativeTestNode(), Remote: nativeTestNode(), HasLinkIDs: true, LocalID: 1, RemoteID: 2, Topologies: []uint16{4096}}}}
	require.Error(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Empty(t, capture.commands)
}

// VALIDATES: an Instance-ID that does not fit in 8 octets is refused by the
// configuration parser.
// PREVENTS: truncating or wrapping an operator Instance-ID.
func TestRFC9552NativeInstanceIDOverflowRefused(t *testing.T) {
	// RFC requirement: RFC9552-8.2.3-5 negative -- an Instance-ID of 2^64 is refused by the configuration parser.
	_, err := parseExportConfig([]rpc.ConfigSection{{Root: exporterName, Data: `{"bgp-ls-export":{"domain":{"core":{"source":"isis","protocol-id":"1","native-instance":"1","native-area":"1","instance-id":"18446744073709551616"}}}}`}})
	require.Error(t, err)
}

// VALIDATES: the native producer refuses a private-use attribute TLV whose
// value is too short to hold the 4-octet Enterprise Code.
// PREVENTS: originating a private-use TLV without its Enterprise Code.
func TestRFC9552NativePrivateUseWithoutEnterpriseRefused(t *testing.T) {
	// RFC requirement: RFC9552-5.4-1 negative -- a node attribute of private-use type 65000 with a 2-octet value is refused and emits no UPDATE.
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1}, Generation: 1,
		Nodes: []linkstateevents.Node{{ID: nativeTestNode(), Attributes: []linkstateevents.TLV{{Type: 65000, Value: []byte{0xab, 0xcd}}}}}}
	require.Error(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Empty(t, capture.commands)
}

// VALIDATES: a native BGP (EPE) link that carries no PeerNode SID is refused.
// PREVENTS: describing a BGP session without its PeerNode SID.
func TestRFC9086NativeLinkWithoutPeerNodeRefused(t *testing.T) {
	// RFC requirement: RFC9086-3-1 negative -- a BGP session link with no PeerNode SID attribute is refused and emits no UPDATE.
	// RFC requirement: RFC9086-5-1 negative -- an EPE Link whose BGP-LS Attribute lacks the PeerNode SID TLV is refused and emits no UPDATE.
	exporter, capture := exportFixture(t)
	local := linkstateevents.NodeID{ASN: 65000, BGPRouterID: netip.MustParseAddr("192.0.2.1")}
	remote := linkstateevents.NodeID{ASN: 65001, BGPRouterID: netip.MustParseAddr("192.0.2.2")}
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.BGP},
		Nodes: []linkstateevents.Node{{ID: local}}, Links: []linkstateevents.Link{{Local: local, Remote: remote}}}
	require.Error(t, exporter.replace(epeName, snapshot))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Empty(t, capture.commands)
}

// VALIDATES: repeating the session's up event instantiates no second PeerNode
// SID: one label and one PeerNode SID TLV describe the session.
// PREVENTS: a second PeerNode SID for one BGP session.
func TestRFC9086NativeRepeatedUpKeepsOnePeerNodeSID(t *testing.T) {
	// RFC requirement: RFC9086-3-2 negative -- a second up event and a replay for the same session leave one installed label and one PeerNode SID TLV on the one advertised Link.
	exporter, capture := exportFixture(t)
	bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr)}
	source := newEPESource(bus)
	peer := netip.MustParseAddr("198.51.100.2")
	require.NoError(t, source.configure(epeConfig{base: 16000, size: 100, peers: map[netip.Addr]epePeerConfig{peer: {index: 7, weight: 5}}}))
	require.NoError(t, source.state(epeProofEvent(t, "up")))
	require.NoError(t, source.state(epeProofEvent(t, "up")))
	require.NoError(t, source.replay())
	require.Len(t, bus.labels, 1)
	require.NoError(t, exporter.reconcile(context.Background()))
	links := 0
	for _, command := range capture.commands {
		if binary.BigEndian.Uint16(exportCommandBytes(t, command, "nlri")) != 2 {
			continue
		}
		links++
		require.Len(t, exportTLVValues(t, exportCommandBytes(t, command, "attr")[11:], 1101), 1)
	}
	require.Equal(t, 1, links)
}

// VALIDATES: an originated PeerNode SID keeps its defined V, L, B and P flags
// and carries zero reserved flag bits and a zero Reserved field.
// PREVENTS: a source's reserved bits reaching the collector.
func TestRFC9086NativePeerSIDReservedFlagsZero(t *testing.T) {
	exporter, capture := exportFixture(t)
	local := linkstateevents.NodeID{ASN: 65000, BGPRouterID: netip.MustParseAddr("192.0.2.1")}
	remote := linkstateevents.NodeID{ASN: 65001, BGPRouterID: netip.MustParseAddr("192.0.2.2")}
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.BGP},
		Nodes: []linkstateevents.Node{{ID: local}}, Links: []linkstateevents.Link{{Local: local, Remote: remote,
			Attributes: []linkstateevents.TLV{{Type: 1101, Value: []byte{0xcf, 5, 0xff, 0xff, 0x00, 0x3e, 0x80}}}}}}
	require.NoError(t, exporter.replace(epeName, snapshot))
	require.NoError(t, exporter.reconcile(context.Background()))
	var sid []byte
	for _, command := range capture.commands {
		if strings.Contains(command, " attr ") && binary.BigEndian.Uint16(exportCommandBytes(t, command, "nlri")) == 2 {
			values := exportTLVValues(t, exportCommandBytes(t, command, "attr")[11:], 1101)
			require.Len(t, values, 1)
			sid = values[0]
		}
	}
	require.NotNil(t, sid)
	// RFC requirement: RFC9086-5-3 positive -- the defined V and L flags and the weight and label of the source PeerNode SID reach the collector unchanged.
	require.Equal(t, byte(0xc0), sid[0]&0xf0)
	require.Equal(t, []byte{5, 0x00, 0x3e, 0x80}, []byte{sid[1], sid[4], sid[5], sid[6]})
	// RFC requirement: RFC9086-5-3 negative -- the four reserved flag bits and the 2-octet Reserved field the source set are zero on the wire.
	require.Equal(t, byte(0), sid[0]&0x0f)
	require.Equal(t, []byte{0, 0}, sid[2:4])
}
