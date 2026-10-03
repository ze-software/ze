package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func refreshFeedbackBatch() bgptypes.NLRIBatch {
	return bgptypes.NLRIBatch{
		Family: family.IPv4Unicast,
		NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
		Replay: true,
	}
}

// TestRefreshSentFeedbackRetainsReplayOrigin covers both immediate writes and
// refreshes queued behind initial sync. The RIB plugin needs this metadata to
// retain config-static ownership without mistaking normal replacements for replay.
// RFC requirement: RFC2918-4-3 positive -- immediate and queued refresh sends reach the wire and report replay ownership; a later ordinary announcement does not inherit that marker.
func TestRefreshSentFeedbackRetainsReplayOrigin(t *testing.T) {
	for _, queued := range []bool{false, true} {
		peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
		if !queued {
			peer.sendInitialRoutes()
		}
		var origins []bool
		peer.session.onMessageReceived = func(_ netip.Addr, typ msgtype.MessageType, body []byte, _ *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection, _ BufHandle, meta map[string]any, _ string, _ uint64) bool {
			if typ == msgtype.TypeUPDATE && direction == rpc.DirectionSent && len(body) > 4 {
				replay, _ := meta["replay"].(bool)
				origins = append(origins, replay)
			}
			return false
		}
		api := newSendPermissionReactor(peer)
		batch := refreshFeedbackBatch()
		if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
			t.Fatal(err)
		}
		if queued {
			peer.sendInitialRoutes()
		}
		batch.Replay = false
		batch.NLRIs = []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), 0)}
		if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
			t.Fatal(err)
		}
		if len(origins) != 2 || !origins[0] || origins[1] {
			t.Fatalf("queued=%v: sent replay flags = %v, want [true false]", queued, origins)
		}
	}
}

// RFC 2918 Section 4 requires re-advertisement "subject to the outbound
// routing policy of that peer". The policy is changed after the first send.
// RFC requirement: RFC2918-4-3 positive -- the replay announce rail re-advertises a stored route when the current outbound policy permits it.
// RFC requirement: RFC2918-4-3 negative -- the same refresh replay is not transmitted when the current outbound policy now rejects it.
func TestRFC2918RefreshRunsCurrentExportPolicy(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	peer.sendInitialRoutes()
	peer.settings.ExportFilters = []filterapi.FilterRef{{Name: "refresh-policy"}}
	peer.refreshForwardFacts()
	api := newSendPermissionReactor(peer)
	api.r.api = &pluginserver.Server{}
	accept, calls := true, 0
	api.r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		calls++
		if accept {
			return PolicyResponse{Action: PolicyAccept}
		}
		return PolicyResponse{Action: PolicyReject}
	}
	peer.session.egressRouteFilter = func(body []byte) (bool, []byte) { return api.r.exportFilterForBody(peer, body) }
	batch := refreshFeedbackBatch()
	before := len(conn.written())
	if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	accepted := bytes.Clone(conn.written())
	if len(accepted) <= before || calls != 1 {
		t.Fatalf("accepted refresh wrote %d additional bytes and evaluated policy %d times", len(accepted)-before, calls)
	}
	accept = false
	if err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !bytes.Equal(accepted, conn.written()) {
		t.Fatal("refresh did not re-evaluate the current policy before transmission")
	}
}
