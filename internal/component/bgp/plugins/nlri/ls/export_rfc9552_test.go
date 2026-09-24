// Design: docs/architecture/wire/nlri-bgpls.md -- native origination contract
// RFC: rfc/short/rfc9552.md

package ls

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type exportCapture struct{ commands []string }

func (c *exportCapture) UpdateRouteWithMeta(_ context.Context, _, command string, _ map[string]any) (uint32, uint32, error) {
	c.commands = append(c.commands, command)
	if strings.Contains(command, " del ") {
		return 0, 1, nil
	}
	return 1, 0, nil
}

func exportFixture(t *testing.T) (*topologyExporter, *exportCapture) {
	t.Helper()
	capture := &exportCapture{}
	exporter := newTopologyExporter(capture)
	exporter.configure(exportConfig{enabled: true})
	exporter.peerState("192.0.2.9", true)
	return exporter, capture
}

func exportCommandBytes(t *testing.T, command, field string) []byte {
	t.Helper()
	parts := strings.Fields(command)
	for i, part := range parts {
		if part == field {
			offset := 2
			if field == "nlri" {
				offset = 3
			}
			require.Greater(t, len(parts), i+offset)
			wire, err := hex.DecodeString(parts[i+offset])
			require.NoError(t, err)
			return wire
		}
	}
	t.Fatalf("command lacks %s: %s", field, command)
	return nil
}

func exportTLVValues(t *testing.T, wire []byte, kind uint16) [][]byte {
	t.Helper()
	var values [][]byte
	for len(wire) > 0 {
		require.GreaterOrEqual(t, len(wire), 4)
		n := int(binary.BigEndian.Uint16(wire[2:4]))
		require.GreaterOrEqual(t, len(wire), 4+n)
		if binary.BigEndian.Uint16(wire[:2]) == kind {
			values = append(values, wire[4:4+n])
		}
		wire = wire[4+n:]
	}
	return values
}

func nativeTestNode() linkstateevents.NodeID {
	return linkstateevents.NodeID{ASN: 65000, RouterID: []byte{1, 2, 3, 4, 5, 6}}
}

// VALIDATES: one native multi-topology link produces independently keyed NLRIs;
// non-default prefixes carry their own topology rather than inheriting a link's.
// PREVENTS: emitting one NLRI with a list of MT-IDs or dropping a prefix's MT-ID.
func TestRFC9552NativeTopologySeparation(t *testing.T) {
	// RFC requirement: RFC9552-5.2.2.1-2 positive -- native link membership emits one independently keyed NLRI per topology.
	// RFC requirement: RFC9552-5.2.3-2 positive -- a native non-default prefix carries its topology descriptor.
	e, capture := exportFixture(t)
	// RFC requirement: RFC9552-5.2.3-1 negative -- an unmasked native prefix cannot leak trailing host bits into its wire identity.
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1}, Generation: 1,
		Links:    []linkstateevents.Link{{Local: nativeTestNode(), Remote: nativeTestNode(), LocalID: 1, RemoteID: 2, HasLinkIDs: true, Topologies: []uint16{0, 7, 7}}},
		Prefixes: []linkstateevents.Prefix{{Node: nativeTestNode(), Prefix: netip.MustParsePrefix("198.51.100.129/25"), Topology: 7}}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 3)
	var topologies []uint16
	for _, command := range capture.commands {
		nlri := exportCommandBytes(t, command, "nlri")
		values := exportTLVValues(t, nlri[13:], 263)
		require.Len(t, values, 1)
		require.Len(t, values[0], 2)
		topologies = append(topologies, binary.BigEndian.Uint16(values[0]))
		if binary.BigEndian.Uint16(nlri) == 3 {
			require.Equal(t, [][]byte{{25, 198, 51, 100, 128}}, exportTLVValues(t, nlri[13:], 265))
		}
	}
	require.ElementsMatch(t, []uint16{0, 7, 7}, topologies)
}

// VALIDATES: OSPF's highest legal topology reaches the collector with zero R
// bits, while an out-of-range link or prefix cannot replace accepted state.
// PREVENTS: applying the wider IS-IS MT-ID range to native OSPF origination.
func TestRFC9552NativeOSPFTopologyBoundary(t *testing.T) {
	// RFC requirement: RFC9552-5.2.2-6 positive -- native OSPF link and prefix MT-ID 127 is emitted with zero upper R bits (§5.2.2.1).
	// RFC requirement: RFC9552-5.2.2-6 negative -- native OSPF link or prefix MT-ID 128 is refused without replacing the accepted database.
	e, capture := exportFixture(t)
	local := linkstateevents.NodeID{RouterID: []byte{192, 0, 2, 1}, HasArea: true}
	remote := linkstateevents.NodeID{RouterID: []byte{192, 0, 2, 2}, HasArea: true}
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.OSPFv2}, Generation: 1,
		Links:    []linkstateevents.Link{{Local: local, Remote: remote, LocalID: 1, RemoteID: 2, HasLinkIDs: true, Topologies: []uint16{127}}},
		Prefixes: []linkstateevents.Prefix{{Node: local, Prefix: netip.MustParsePrefix("198.51.100.0/24"), Topology: 127, RouteType: 1}}}
	require.NoError(t, e.replace("ospf", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 2)
	for _, command := range capture.commands {
		wire := exportCommandBytes(t, command, "nlri")
		require.Equal(t, [][]byte{{0, 127}}, exportTLVValues(t, wire[13:], 263))
	}
	accepted := append([]string(nil), capture.commands...)
	snapshot.Generation++
	snapshot.Links[0].Topologies[0] = 128
	require.Error(t, e.replace("ospf", snapshot))
	e.peerRefresh("192.0.2.9")
	require.NoError(t, e.reconcile(context.Background()))
	require.Equal(t, accepted, capture.commands[2:])
	snapshot.Generation++
	snapshot.Links[0].Topologies[0] = 127
	snapshot.Prefixes[0].Topology = 128
	require.Error(t, e.replace("ospf", snapshot))
	e.peerRefresh("192.0.2.9")
	require.NoError(t, e.reconcile(context.Background()))
	require.Equal(t, accepted, capture.commands[4:])
}

// VALIDATES: replacing a topology removes its old identity before announcing its
// successor, and a delayed source snapshot cannot resurrect the retired identity.
// PREVENTS: stale MT-ID NLRIs surviving database replacement or replay.
func TestRFC9552NativeTopologyReplacement(t *testing.T) {
	// RFC requirement: RFC9552-5.2.2.1-2 negative -- a topology replacement withdraws the old unique NLRI instead of retaining a second stale topology.
	// RFC requirement: RFC9552-5.2.3-2 negative -- a prefix changing non-default topology never retains its former topology identity.
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel2}, Generation: 1,
		Prefixes: []linkstateevents.Prefix{{Node: nativeTestNode(), Prefix: netip.MustParsePrefix("2001:db8::/64"), Topology: 2}}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	old := exportCommandBytes(t, capture.commands[0], "nlri")
	snapshot.Generation = 2
	snapshot.Prefixes[0].Topology = 9
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 3)
	require.Contains(t, capture.commands[1], " del ")
	require.Equal(t, old, exportCommandBytes(t, capture.commands[1], "nlri"))
	newNLRI := exportCommandBytes(t, capture.commands[2], "nlri")
	require.Equal(t, [][]byte{{0, 9}}, exportTLVValues(t, newNLRI[13:], 263))
	snapshot.Generation = 1
	snapshot.Prefixes[0].Topology = 2
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 3)
}

func nativeFlagOutput(t *testing.T, node, prefix, mpls byte) ([]byte, []byte, []byte) {
	t.Helper()
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.Direct},
		Nodes:    []linkstateevents.Node{{ID: nativeTestNode(), Attributes: []linkstateevents.TLV{{Type: 1024, Value: []byte{node}}}}},
		Links:    []linkstateevents.Link{{Local: nativeTestNode(), Remote: nativeTestNode(), LocalID: 1, HasLinkIDs: true, Attributes: []linkstateevents.TLV{{Type: 1094, Value: []byte{mpls}}}}},
		Prefixes: []linkstateevents.Prefix{{Node: nativeTestNode(), Prefix: netip.MustParsePrefix("192.0.2.0/24"), Attributes: []linkstateevents.TLV{{Type: 1152, Value: []byte{prefix}}}}}}
	require.NoError(t, e.replace("local", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 3)
	values := make(map[uint16][]byte)
	for _, command := range capture.commands {
		attrs := exportCommandBytes(t, command, "attr")
		for _, kind := range []uint16{1024, 1094, 1152} {
			found := exportTLVValues(t, attrs[11:], kind)
			if len(found) != 0 {
				require.Len(t, found, 1)
				values[kind] = found[0]
			}
		}
	}
	return values[1024], values[1152], values[1094]
}

// VALIDATES: originating Node, IGP and MPLS flags preserves every defined bit.
// PREVENTS: zeroing the whole capability octet while clearing reserved bits.
func TestRFC9552NativeDefinedFlags(t *testing.T) {
	// RFC requirement: RFC9552-5.3.1.1-1 positive -- native node origination retains defined flags.
	// RFC requirement: RFC9552-5.3.3.1-1 positive -- native prefix origination retains defined IGP flags.
	// RFC requirement: RFC9552-5.3.2.2-3 positive -- native Direct link origination retains LDP/RSVP flags.
	// RFC requirement: RFC9552-5.3.2.2-1 positive -- the MPLS Protocol Mask is available for a Direct link, not indiscriminately suppressed.
	node, prefix, mpls := nativeFlagOutput(t, 0xfc, 0xf0, 0xc0)
	require.Equal(t, []byte{0xfc}, node)
	require.Equal(t, []byte{0xf0}, prefix)
	require.Equal(t, []byte{0xc0}, mpls)
}

// VALIDATES: source reserved bits never reach an originated BGP-LS advertisement.
// PREVENTS: copying a native source flag word without its protocol-defined mask.
func TestRFC9552NativeReservedFlagsCleared(t *testing.T) {
	// RFC requirement: RFC9552-5.3.1.1-1 negative -- native node source reserved bits are absent on emitted wire.
	// RFC requirement: RFC9552-5.3.3.1-1 negative -- native prefix source reserved bits are absent on emitted wire.
	// RFC requirement: RFC9552-5.3.2.2-3 negative -- native MPLS source reserved bits are absent on emitted wire.
	node, prefix, mpls := nativeFlagOutput(t, 0x03, 0x0f, 0x3f)
	require.Equal(t, []byte{0}, node)
	require.Equal(t, []byte{0}, prefix)
	require.Equal(t, []byte{0}, mpls)
}

// VALIDATES: an IGP source cannot replace accepted state with a forbidden MPLS
// Protocol Mask; refusing it leaves the previously advertised database intact.
// PREVENTS: applying Direct-link MPLS attributes to IS-IS advertisements.
func TestRFC9552NativeIGPMPLSMaskRefused(t *testing.T) {
	// RFC requirement: RFC9552-5.3.2.2-1 negative -- an IS-IS replacement containing TLV 1094 emits no replacement or forbidden attribute.
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1},
		Generation: 1, Links: []linkstateevents.Link{{Local: nativeTestNode(), Remote: nativeTestNode(), HasLinkIDs: true, LocalID: 1}}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
	require.Empty(t, exportTLVValues(t, exportCommandBytes(t, capture.commands[0], "attr")[11:], 1094))
	snapshot.Generation = 2
	snapshot.Links[0].Attributes = []linkstateevents.TLV{{Type: 1094, Value: []byte{0xc0}}}
	require.Error(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
}

// VALIDATES: address descriptors take precedence over link identifiers, while
// a link with only link-local addresses falls back to its real identifiers.
// PREVENTS: ambiguous Link NLRIs and forbidden link-local address descriptors.
func TestRFC9552NativeLinkIdentitySelection(t *testing.T) {
	// RFC requirement: RFC9552-5.2.2-1 positive -- available global addresses appear in collector-visible Link descriptors.
	// RFC requirement: RFC9552-5.2.2-2 negative -- a link carrying global addresses does not also carry link identifiers.
	// RFC requirement: RFC9552-5.2.2-3 negative -- link-local source addresses never appear in the corresponding address descriptors.
	// RFC requirement: RFC9552-5.2.2-4 positive -- a link with no usable global address retains its actual local/remote identifiers.
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1}, Generation: 1,
		Links: []linkstateevents.Link{{Local: nativeTestNode(), Remote: nativeTestNode(), HasLinkIDs: true, LocalID: 0, RemoteID: 8,
			LocalAddresses:  []netip.Addr{netip.MustParseAddr("169.254.1.1"), netip.MustParseAddr("192.0.2.1")},
			RemoteAddresses: []netip.Addr{netip.MustParseAddr("fe80::2"), netip.MustParseAddr("2001:db8::2")}}}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
	wire := exportCommandBytes(t, capture.commands[0], "nlri")
	require.Equal(t, [][]byte{{192, 0, 2, 1}}, exportTLVValues(t, wire[13:], 259))
	remote := netip.MustParseAddr("2001:db8::2").As16()
	require.Equal(t, [][]byte{remote[:]}, exportTLVValues(t, wire[13:], 262))
	require.Empty(t, exportTLVValues(t, wire[13:], 258))
	snapshot.Generation = 2
	snapshot.Links[0].LocalAddresses = snapshot.Links[0].LocalAddresses[:1]
	snapshot.Links[0].RemoteAddresses = snapshot.Links[0].RemoteAddresses[:1]
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 3)
	require.Contains(t, capture.commands[1], " del ")
	wire = exportCommandBytes(t, capture.commands[2], "nlri")
	require.Equal(t, [][]byte{{0, 0, 0, 0, 0, 0, 0, 8}}, exportTLVValues(t, wire[13:], 258))
	for _, kind := range []uint16{259, 260, 261, 262} {
		require.Empty(t, exportTLVValues(t, wire[13:], kind))
	}
}

// VALIDATES: the configuration parser preserves all 64 Identifier bits and
// domain selection changes only the configured source, instance and area.
// PREVENTS: float64 rounding and accidental cross-domain Instance-ID overrides.
func TestRFC9552NativeInstanceIDFullWidth(t *testing.T) {
	// RFC requirement: RFC9552-8.2.3-5 positive -- the maximum 8-octet operator Instance-ID reaches the emitted NLRI unchanged.
	cfg, err := parseExportConfig([]rpc.ConfigSection{{Root: exporterName, Data: `{"bgp-ls-export":{"domain":{"core":{"source":"isis","protocol-id":"1","native-instance":"1","native-area":"1","instance-id":"18446744073709551615"}}}}`}})
	require.NoError(t, err)
	e, capture := exportFixture(t)
	e.configure(cfg)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1, Instance: 1, Area: 1, Identifier: 17},
		Generation: 1, Nodes: []linkstateevents.Node{{ID: nativeTestNode()}}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
	require.Equal(t, ^uint64(0), binary.BigEndian.Uint64(exportCommandBytes(t, capture.commands[0], "nlri")[5:13]))
	snapshot.Domain.Area = 2
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 2)
	require.Equal(t, uint64(17), binary.BigEndian.Uint64(exportCommandBytes(t, capture.commands[1], "nlri")[5:13]))
}

// VALIDATES: source-owned memory may be reused after publication, and a new
// collector session receives the accepted wire rather than the mutated slices.
// PREVENTS: delayed publication reading freed source buffers or losing replay.
func TestNativeTopologySnapshotOwnershipAndReplay(t *testing.T) {
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1},
		Generation: 1, Nodes: []linkstateevents.Node{{ID: nativeTestNode(), Attributes: []linkstateevents.TLV{{Type: 1024, Value: []byte{0x80}}}}}}
	require.NoError(t, e.replace("isis", snapshot))
	snapshot.Nodes[0].ID.RouterID[0] = 99
	snapshot.Nodes[0].Attributes[0].Value[0] = 0
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
	wire := exportCommandBytes(t, capture.commands[0], "nlri")
	local := exportTLVValues(t, wire[13:], 256)
	require.Equal(t, [][]byte{{1, 2, 3, 4, 5, 6}}, exportTLVValues(t, local[0], 515))
	require.Equal(t, [][]byte{{0x80}}, exportTLVValues(t, exportCommandBytes(t, capture.commands[0], "attr")[11:], 1024))
	e.peerState("192.0.2.9", false)
	e.peerState("192.0.2.9", true)
	require.NoError(t, e.reconcile(context.Background()))
	require.Equal(t, []string{capture.commands[0], capture.commands[0]}, capture.commands)
}

// VALIDATES: native SPF reachability removes an origin's still-live LSDB
// objects, retains a reachable origin's half-link, and restores every object
// when reachability returns without requiring those LSDB records to change.
// PREVENTS: stale disconnected topology or permanent loss after SPF recovery.
func TestRFC9552NativeReachabilityWithdrawalAndRestoration(t *testing.T) {
	// RFC requirement: RFC9552-5.9-1 positive -- unchanged link-state objects are re-advertised after their origin becomes reachable again.
	// RFC requirement: RFC9552-5.9-1 negative -- live LSDB records cannot resurrect an origin while native SPF still marks it unreachable.
	e, capture := exportFixture(t)
	snapshot := nativeSRv6Snapshot(0)
	remote := nativeTestNode()
	remote.RouterID[5] = 7
	snapshot.Nodes[0].ID = remote
	snapshot.Prefixes[0].Node = remote
	snapshot.SIDs[0].Node = remote
	root := nativeTestNode()
	snapshot.Nodes = append(snapshot.Nodes, linkstateevents.Node{ID: root})
	snapshot.Links = []linkstateevents.Link{
		{Local: root, Remote: remote, HasLinkIDs: true, LocalID: 1, RemoteID: 2},
		{Local: remote, Remote: root, HasLinkIDs: true, LocalID: 2, RemoteID: 1},
	}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 6)
	var remoteAnnouncements []string
	var remoteNLRIs [][]byte
	for _, command := range capture.commands {
		wire := exportCommandBytes(t, command, "nlri")
		local := exportTLVValues(t, wire[13:], 256)
		router := exportTLVValues(t, local[0], 515)
		if bytes.Equal(router[0], remote.RouterID) {
			remoteAnnouncements = append(remoteAnnouncements, command)
			remoteNLRIs = append(remoteNLRIs, wire)
		}
	}
	require.Len(t, remoteNLRIs, 4)
	snapshot.Generation++
	unreachable := remote
	unreachable.Area = 99 // HasArea remains false: this field is absent from the NLRI identity.
	snapshot.Unreachable = []linkstateevents.NodeID{unreachable}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 10)
	var withdrawn [][]byte
	for _, command := range capture.commands[6:] {
		require.Contains(t, command, " del ")
		withdrawn = append(withdrawn, exportCommandBytes(t, command, "nlri"))
	}
	require.ElementsMatch(t, remoteNLRIs, withdrawn, "the reachable root's half-link must remain advertised")
	snapshot.Generation++
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 10)
	snapshot.Generation++
	snapshot.Unreachable = nil
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.ElementsMatch(t, remoteAnnouncements, capture.commands[10:])
}

// VALIDATES: the native standard-only producer cannot originate a vendor-private
// attribute, even when it has a well-formed Enterprise Code.
// PREVENTS: an accidental local private producer bypassing the configured role.
func TestNativeStandardOnlyRejectsPrivateAttributes(t *testing.T) {
	e, capture := exportFixture(t)
	snapshot := &linkstateevents.Snapshot{
		Domain:     linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1},
		Generation: 1,
		Nodes: []linkstateevents.Node{{
			ID:         nativeTestNode(),
			Attributes: []linkstateevents.TLV{{Type: 1026, Value: []byte("router")}},
		}},
	}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
	advertisement := capture.commands[0]
	snapshot.Generation++
	snapshot.Nodes[0].Attributes = append(snapshot.Nodes[0].Attributes,
		linkstateevents.TLV{Type: 65000, Value: []byte{0, 0, 0, 42, 0xab}})
	require.Error(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Equal(t, []string{advertisement}, capture.commands)
}
