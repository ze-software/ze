// Design: docs/architecture/plugin/rib-storage-design.md -- original source receipts
package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginapi "github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSourceReceiptReceivedRails sends genuine formatted JSON and typed wire
// events through the two received handlers, then refreshes identical attributes.
func TestSourceReceiptReceivedRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		for _, labeled := range []bool{false, true} {
			t.Run(testReceiptRailName(jsonRail, labeled), func(t *testing.T) {
				r := newTestRIBManager(t)
				peer := netip.MustParseAddr("192.0.2.92")
				const owner uint64 = 1
				electionFixtureLocRIB(t, r, netip.MustParseAddr("10.0.0.1"))
				t.Cleanup(func() {
					for _, routes := range r.bgpPeers {
						routes.Release()
					}
				})
				fam := family.IPv4Unicast
				raw := []byte{8, 10}
				attrs := unicastAttrs([4]byte{10, 0, 0, 1}, 30, 100)
				body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
				body = append(body, raw...)
				if labeled {
					fam = labeledFamily
					body = labeledUpdateBody([4]byte{10, 0, 0, 1}, 30, labeledNLRI(0, false, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100}))
				}
				ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
				require.NoError(t, err)
				for _, receipt := range []uint64{97, 98, 0} {
					wu := wireu.NewWireUpdate(body, ctxID)
					wu.SetSourceOwner(owner)
					aw, err := wu.Attrs()
					require.NoError(t, err)
					msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: 7, SourcePolicyEpoch: 17, SourceReceipt: receipt, SourceOwner: owner, RawBytes: body, WireUpdate: wu, AttrsWire: aw}
					if jsonRail {
						info := pluginapi.PeerInfo{ConnectionID: 41, PeerOwner: owner, Address: peer, AddressStr: peer.String(), PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000}
						line := format.AppendMessage(nil, &info, msg, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull})
						event, err := parseEvent(line)
						require.NoError(t, err)
						r.dispatch(event)
					} else {
						r.handleReceivedStructured(&rpc.StructuredEvent{ConnectionID: 41, EventType: rpc.EventKindUpdate, PeerOwner: owner, PeerAddress: peer.String(), PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000, SourceReceipt: receipt, RawMessage: &msg})
					}
					stored, found := r.bgpPeers[owner].Lookup(fam, raw)
					require.True(t, found)
					require.Equal(t, receipt, stored.SourceReceipt)
					require.Equal(t, uint64(17), stored.SourcePolicyEpoch)
				}
			})
		}
	}
}

func testReceiptRailName(jsonRail, labeled bool) string {
	if jsonRail {
		if labeled {
			return "json-labeled"
		}
		return "json-cidr"
	}
	if labeled {
		return "typed-labeled"
	}
	return "typed-cidr"
}

// TestSourceReceiptLabeledFallback stores an unparsed path-zero labeled route
// and verifies that its original source receipt is not lost in label stripping.
func TestSourceReceiptLabeledFallback(t *testing.T) {
	r := newTestRIBManager(t)
	peer := storage.NewPeerRIB("192.0.2.93")
	t.Cleanup(peer.Release)
	peer.SetAddPath(labeledFamily, true)
	wire := labeledNLRI(0, true, netip.MustParsePrefix("10.0.0.0/8"), []uint32{100})
	var affected []affectedPrefix
	r.insertLabeled(peer, labeledFamily, staleInsertAttrs, wire, true, false, 19, 29, 39, &affected, nil)
	entry, found := peer.Lookup(labeledFamily, []byte{0, 0, 0, 0, 8, 10})
	require.True(t, found)
	require.Equal(t, uint64(39), entry.SourceReceipt)
}

// TestSourceReceiptCaptureRevision changes only the issuing session and holds
// the previous immutable capture across the resulting selected revision.
func TestSourceReceiptCaptureRevision(t *testing.T) {
	r, peer := advertiseCaptureRIB(t)
	raw := ipv4Prefix(24, 198, 18, 93)
	advertiseCapturePath(t, r, advertiseCaptureOwner, family.IPv4Unicast, raw, unicastAttrs(peer.As4(), 30, 100), 79, 89)
	advertiseCaptureReceipt(t, r, advertiseCaptureOwner, family.IPv4Unicast, raw, 99, 42)
	r.checkBestPathChange(family.IPv4Unicast, raw, false, nil)
	first, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, uint64(99), first[0].route.SourceReceipt)
	advertiseCaptureReceipt(t, r, advertiseCaptureOwner, family.IPv4Unicast, raw, 100, 43)
	r.checkBestPathChange(family.IPv4Unicast, raw, false, nil)
	current, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Greater(t, current[0].revision, first[0].revision)
	require.Equal(t, uint64(100), current[0].route.SourceReceipt)
	require.Equal(t, uint64(99), first[0].route.SourceReceipt)
	require.Equal(t, first[0].route.Attributes, current[0].route.Attributes)
}

// TestSourceReceiptFlowSpecAndRecoveryViews checks the actual path query and
// recovery constructor, including their external JSON value representations.
func TestSourceReceiptFlowSpecAndRecoveryViews(t *testing.T) {
	r, _ := sourceLeaseTestRIB(t, true)
	peer := netip.MustParseAddr("192.0.2.94")
	routes := storage.NewPeerRIB(peer.String())
	r.peerMeta[1] = &peerMetadata{Address: peer}
	r.bgpPeers[1] = routes
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	raw := []byte{2, 1, 0}
	routes.SetAddPath(fam, true)
	framed := append([]byte{0, 0, 0, 0}, raw...)
	routes.Insert(fam, staleInsertAttrs, framed)
	ref, err := r.sourceLeases.acquire(t.Context(), peer, 1, 39, 41)
	require.NoError(t, err)
	defer ref.Release()
	routes.ModifyFamilyEntry(fam, framed, func(entry *storage.RouteEntry) {
		entry.SetSourceReference(ref)
		entry.MsgID, entry.SourcePolicyEpoch, entry.SourceReceipt = 19, 29, 39
	})
	path, found := r.flowSpecPath(ribevents.ValidationRoute{ConnectionID: 41, Peer: peer, PeerOwner: 1, Family: fam, NLRI: string(raw), PathID: 0})
	require.True(t, found)
	require.Equal(t, uint64(39), path.SourceReceipt)
	encoded, err := json.Marshal(path)
	require.NoError(t, err)
	var decoded ribevents.FlowSpecPath
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, path, decoded)
	entry, found := routes.LookupRetained(fam, framed)
	require.True(t, found)
	t.Cleanup(entry.Release)
	best := Candidate{PeerIP: peer, PeerOwner: 1, Route: string(raw), PathID: 0, AddPath: true, entry: entry}
	recovery, err := recoveryCandidate(fam, netip.Prefix{}, &best)
	require.NoError(t, err)
	require.Equal(t, uint64(39), recovery.SourceReceipt)
	require.True(t, recovery.AddPath)
	require.Zero(t, recovery.PathID)
	encoded, err = json.Marshal(recovery)
	require.NoError(t, err)
	var recovered ribevents.RecoveryRoute
	require.NoError(t, json.Unmarshal(encoded, &recovered))
	require.Equal(t, recovery, recovered)
}
