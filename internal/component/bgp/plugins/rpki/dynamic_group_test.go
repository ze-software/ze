// VALIDATES: the group identity reaches the validation request on both event
// rails and survives the state rpki keeps between an UPDATE and a later
// re-validation, so buildDecisions resolves a listen-range group's stated actions
// for every session that group accepts.
// PREVENTS: the decision path being fed the group by the test's own hand. The
// config side can key a template correctly and still change nothing if the rail
// carrying an UPDATE drops the identity on the way in, or if RFC 6811 Section 4
// re-validation judges the same route by the global actions instead.

package rpki

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// groupMemberPlugin builds a plugin that enqueues to a buffered channel instead
// of a running worker, so one UPDATE can be read back as the requests it made.
//
// Both rails emit an rpki event per family, so the plugin needs a working
// EmitEvent. It gets the DirectBridge the engine itself gives an internal plugin
// (rpc.NewBridgedConn), with a handler that counts nothing: the event is not what
// is under test here, and the alternative is an RPC round trip over a pipe with
// nobody at the other end.
func groupMemberPlugin(t *testing.T) *rPKIPlugin {
	t.Helper()

	bridge := rpc.NewDirectBridge()
	bridge.SetEmitEvent(func(_, _, _, _, _ string) (int, error) { return 0, nil })
	bridge.SetReady()
	client, server := net.Pipe()
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Logf("closing the client pipe: %v", err)
		}
		if err := server.Close(); err != nil {
			t.Logf("closing the server pipe: %v", err)
		}
	})

	rp := &rPKIPlugin{
		plugin:        sdk.NewWithConn("bgp-rpki", rpc.NewBridgedConn(client, bridge)),
		cache:         newROACache(),
		aspaCache:     newASPACache(),
		aspaTracker:   newASPATracker(),
		originTracker: newOriginTracker(),
		validateCh:    make(chan validationRequest, 8),
		stopCh:        make(chan struct{}),
	}
	rp.active.Store(true)
	t.Cleanup(func() { close(rp.stopCh) })
	return rp
}

// drainRequests reads every request the channel holds without blocking.
func drainRequests(ch chan validationRequest) []validationRequest {
	var out []validationRequest
	for {
		select {
		case req := <-ch:
			out = append(out, req)
		default:
			return out
		}
	}
}

// memberPeerJSON is the peer object the engine emits for a session a listen-range
// group accepted: a real address, and the group's name.
func memberPeerJSON(t *testing.T, addr, group string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(bgp.PeerInfoJSON{
		Name:   "ix-" + addr,
		Group:  group,
		Remote: bgp.PeerRemoteInfo{Address: addr},
	})
	require.NoError(t, err)
	return raw
}

// The JSON rail. handleEvent must read the group off the event, or every route
// from a dynamic member reaches buildDecisions with no identity to resolve.
func TestJSONRailCarriesTheGroupIntoTheValidationRequest(t *testing.T) {
	rp := groupMemberPlugin(t)

	rp.handleEvent(&bgp.Event{
		Message: &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 7},
		Peer:    memberPeerJSON(t, "192.0.2.50", "ix"),
		ASPath:  []uint32{64511},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {
				{NextHop: "192.0.2.50", Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24"}},
			},
		},
	})

	reqs := drainRequests(rp.validateCh)
	require.Len(t, reqs, 1)
	assert.Equal(t, "192.0.2.50", reqs[0].peerAddr)
	assert.Equal(t, "ix", reqs[0].peerGroup, "the group was dropped on the JSON rail")
}

// The structured rail's NLRI walker, which is where its requests are built. The
// address, the name and the group all come off one StructuredEvent three lines
// above the call in handleStructuredUpdate.
func TestStructuredRailCarriesTheGroupIntoTheValidationRequest(t *testing.T) {
	rp := groupMemberPlugin(t)

	// 10.0.0.0/24 as wire NLRI: one length octet, then the significant bytes.
	nlri := []byte{24, 10, 0, 0}
	rp.validateNLRIs("192.0.2.50", "ix-192.0.2.50", "ix", 64511, 7, "ipv4/unicast",
		nlri, false, false, 64511, false, aspaStateNone, false)

	reqs := drainRequests(rp.validateCh)
	require.Len(t, reqs, 1)
	assert.Equal(t, "192.0.2.50", reqs[0].peerAddr)
	assert.Equal(t, "ix", reqs[0].peerGroup, "the group was dropped on the structured rail")
}

// The test above hands validateNLRIs the group by name, so it pins the walker and
// not the READ. handleStructuredUpdate is what takes the group off the event, and
// it is the entry point the default in-process deployment uses: without this,
// deleting that read leaves every rpki test green while an IXP member loses its
// group's RFC 6811 actions and its RFC 7999 exemption.
func TestStructuredRailReadsTheGroupOffTheEvent(t *testing.T) {
	rp := groupMemberPlugin(t)

	// One UPDATE announcing 10.0.0.0/24 from AS 64511: withdrawn length 0, then
	// ORIGIN, a one-ASN AS_PATH and NEXT_HOP, then the NLRI.
	body := []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x14, // Total Path Attribute Length 20
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFB, 0xFF, // AS_PATH = [64511]
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x18, 0x0a, 0x00, 0x00, // NLRI 10.0.0.0/24
	}
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, _ := wu.Attrs()

	rp.handleStructuredUpdate(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: "192.0.2.50",
		PeerName:    "ix-192.0.2.50",
		PeerGroup:   "ix",
		PeerAS:      64511,
		LocalAS:     65000,
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   body,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	})

	reqs := drainRequests(rp.validateCh)
	require.Len(t, reqs, 1, "the announced prefix must reach the decision path")
	assert.Equal(t, "10.0.0.0/24", reqs[0].prefix)
	assert.Equal(t, "ix", reqs[0].peerGroup,
		"handleStructuredUpdate did not read the group off the event, so the member "+
			"resolves the global actions instead of its group's")
}

// Re-validation has no UPDATE in hand. RFC 6811 Section 4 re-runs the decision on
// a VRP change, and a member whose actions came from its group must keep them: a
// route accepted on arrival and rejected on re-validation is one route judged by
// two policies. The group is not part of routeKey, so nothing about the route's
// identity carries it.
func TestOriginRevalidationKeepsTheGroupIdentity(t *testing.T) {
	tr := newOriginTracker()
	key := routeKey{peerAddr: "192.0.2.50", family: "ipv4/unicast", prefix: "10.0.0.0/24"}
	tr.Track(key, "ix", 65001, ValidationNotFound, aspaStateNone, false)

	cache := newROACache()
	cache.Add(makeVRP("10.0.0.0/24", 24, 65001))

	changed := tr.revalidate(cache)
	require.Len(t, changed, 1)
	assert.Equal(t, "ix", changed[0].peerGroup, "re-validation lost the session's group")
}

// The tracker keeping the group is only half of it: handleROAChange is what turns
// a re-validated route back into a validationRequest, and that is the value
// buildDecisions resolves the actions from. Asserting the tracker alone leaves
// the dispatch site free to drop the field.
func TestOriginRevalidationDispatchesWithTheGroup(t *testing.T) {
	rp := groupMemberPlugin(t)
	key := routeKey{peerAddr: "192.0.2.50", family: "ipv4/unicast", prefix: "10.0.0.0/24"}
	rp.originTracker.Track(key, "ix", 65001, ValidationNotFound, aspaStateNone, false)

	rp.cache.Add(makeVRP("10.0.0.0/24", 24, 65001))
	rp.handleROAChange()

	reqs := drainRequests(rp.validateCh)
	require.Len(t, reqs, 1, "a VRP change that flips the state must re-dispatch")
	assert.Equal(t, "192.0.2.50", reqs[0].peerAddr)
	assert.Equal(t, "ix", reqs[0].peerGroup,
		"handleROAChange dropped the group, so the member's route would be re-judged by the global actions")
}

// The same for the ASPA rail: handleASPAChange re-dispatches from a trackedRoute
// when the new ASPA state overrides accept.
func TestASPARevalidationDispatchesWithTheGroup(t *testing.T) {
	rp := groupMemberPlugin(t)
	rp.aspaEnabled.Store(true)
	rp.aspaInvalidAction.Store(uint32(ASPAPolicyReject))

	rp.aspaCache.Set(64501, []uint32{64500})
	rp.aspaCache.Set(64502, []uint32{64501})

	key := routeKey{peerAddr: "192.0.2.50", family: "ipv4/unicast", prefix: "10.0.0.0/24"}
	rp.aspaTracker.Track(trackedRoute{
		key:       key,
		peerName:  "ix-192.0.2.50",
		peerGroup: "ix",
		peerASN:   64500,
		path:      []uint32{64500, 64501, 64502},
		aspaState: ASPAValid,
	})

	// 64502 stops authorizing 64501, which flips the route to Invalid. With the
	// invalid action set to reject, that overrides accept and re-dispatches.
	rp.aspaCache.Set(64502, []uint32{99999})
	rp.handleASPAChange([]uint32{64502})

	reqs := drainRequests(rp.validateCh)
	require.Len(t, reqs, 1, "an ASPA state that overrides accept must re-dispatch")
	assert.Equal(t, "ix", reqs[0].peerGroup,
		"handleASPAChange dropped the group, so the member's route would be re-judged by the global actions")
}

// The ASPA tracker keeps the same identity for the same reason: handleASPAChange
// re-dispatches from a trackedRoute, also with no UPDATE in hand.
func TestASPATrackerKeepsTheGroupIdentity(t *testing.T) {
	tr := newASPATracker()
	key := routeKey{peerAddr: "192.0.2.50", family: "ipv4/unicast", prefix: "10.0.0.0/24"}
	tr.Track(trackedRoute{
		key:       key,
		peerName:  "ix-192.0.2.50",
		peerGroup: "ix",
		peerASN:   64511,
		path:      []uint32{64511},
		aspaState: ASPAUnknown,
	})

	got, ok := tr.routes[key]
	require.True(t, ok)
	assert.Equal(t, "ix", got.peerGroup, "the ASPA tracker lost the session's group")
}
