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
	"encoding/hex"
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
// Both rails emit one rpki event per UPDATE, so the plugin needs a working
// EmitEvent. It gets the DirectBridge the engine itself gives an internal plugin
// (rpc.NewBridgedConn). Tests that inspect emitted events can replace the
// default handler on the returned bridge before feeding an UPDATE.
func groupMemberPlugin(t *testing.T) (*rPKIPlugin, *rpc.DirectBridge) {
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
	return rp, bridge
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
	rp, _ := groupMemberPlugin(t)

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
	rp, _ := groupMemberPlugin(t)

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
	rp, _ := groupMemberPlugin(t)

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

// The tracker keeping the group is only half of it: handleROAChange is what turns
// a re-validated route back into a validationRequest, and that is the value
// buildDecisions resolves the actions from. Asserting the tracker alone leaves
// the dispatch site free to drop the field.
func TestOriginRevalidationDispatchesWithTheGroup(t *testing.T) {
	rp, _ := groupMemberPlugin(t)
	key := routeKey{peerAddr: "192.0.2.50", family: "ipv4/unicast", prefix: "10.0.0.0/24"}
	rp.originTracker.Track(key, originRoute{peerGroup: "ix", originAS: 65001,
		state: ValidationNotFound, aspaState: aspaStateNone, unavailable: true})

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
	rp, _ := groupMemberPlugin(t)
	rp.aspaEnabled.Store(true)
	rp.aspaInvalidAction.Store(uint32(ASPAPolicyReject))

	rp.aspaCache.Set(64501, []uint32{64500})
	rp.aspaCache.Set(64502, []uint32{64501})

	key := routeKey{peerAddr: "192.0.2.50", family: "ipv4/unicast", prefix: "10.0.0.0/24"}
	rp.originTracker.Track(key, originRoute{peerGroup: "ix", originAS: 64502,
		state: ValidationNotFound, aspaState: ASPAValid, unavailable: true})
	rp.aspaTracker.Track(trackedRoute{
		key:       key,
		peerName:  "ix-192.0.2.50",
		peerGroup: "ix",
		peerASN:   64500,
		path:      []uint32{64500, 64501, 64502},
		aspaState: ASPAValid,
		mode:      aspaUpstream,
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

// TestASPAJSONRailKeepsSegmentTypes rejects an AS_SET even when the JSON
// projection also contains a flattened path whose ordered hops are authorized.
func TestASPAJSONRailKeepsSegmentTypes(t *testing.T) {
	rp, _ := aspaRolePlugin(t, "provider")
	rp.aspaCache.Set(200, []uint32{100})
	rp.aspaCache.Set(300, []uint32{200})
	rp.cache.Add(makeVRP("10.0.0.0/24", 24, 300))
	event := &bgp.Event{
		Message: &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 71},
		Peer:    memberPeerJSON(t, "192.0.2.50", "ix"),
		ASPath:  []uint32{100, 200, 300},
		RawAttributeBytes: []byte{
			0x40, 0x02, 16,
			2, 2, 0, 0, 0, 100, 0, 0, 0, 200,
			1, 1, 0, 0, 1, 44,
		},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {{Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24"}}},
		},
	}
	rp.handleEvent(event)
	reqs := drainRequests(rp.validateCh)
	require.Len(t, reqs, 1)
	assert.Equal(t, ASPAInvalid, reqs[0].aspaState)
	assert.Equal(t, ValidationInvalid, reqs[0].state)
	assert.False(t, rp.buildDecisions(reqs)[0].Accept)

	// Without segment-bearing attributes the ordered JSON fallback remains usable.
	event.RawAttributeBytes = nil
	event.Message.ID++
	rp.handleEvent(event)
	reqs = drainRequests(rp.validateCh)
	require.Len(t, reqs, 1)
	assert.Equal(t, ASPAValid, reqs[0].aspaState)
	assert.True(t, rp.buildDecisions(reqs)[0].Accept)

	// Invalid explicitly supplied raw bytes cannot fall back to a flattened
	// sequence that discards their segment boundaries.
	event.RawAttributes = "not-hex"
	event.Message.ID++
	rp.handleEvent(event)
	reqs = drainRequests(rp.validateCh)
	require.Len(t, reqs, 1)
	assert.Equal(t, ASPAInvalid, reqs[0].aspaState)
	assert.Equal(t, ValidationInvalid, reqs[0].state)
	assert.False(t, rp.buildDecisions(reqs)[0].Accept)
}

// TestASPASessionDownStopsRevalidation prevents cache changes from producing
// decisions for received routes whose session and Adj-RIB-In are gone.
func TestASPASessionDownStopsRevalidation(t *testing.T) {
	rp := aspaInvalidPlugin(t)
	feedASPAUpdate(t, rp,
		aspaMPReachBody(1, 1, []byte{192, 0, 2, 50}, []byte{24, 10, 0, 0}))
	rp.handleEvent(&bgp.Event{
		Message: &bgp.MessageInfo{Type: rpc.EventKindState},
		Peer:    memberPeerJSON(t, "192.0.2.50", "ix"),
		State:   "down",
	})
	rp.aspaCache.Set(300, []uint32{200})
	rp.handleASPAChange([]uint32{300})
	rp.cache.Add(makeVRP("10.0.0.0/24", 24, 300))
	rp.handleROAChange()
	assert.Empty(t, drainRequests(rp.validateCh))
}

// Both event delivery forms must use the local speaker AS for an empty or
// confederation-ending path, while an AS_SET still has no origin.
func TestJSONOriginUsesLocalASAndRawSegmentTypes(t *testing.T) {
	for _, path := range [][]byte{
		{},
		{3, 1, 0, 0, 0xfd, 0xe9},
		{4, 1, 0, 0, 0xfd, 0xe9},
		{1, 1, 0, 0, 0xfd, 0xe8},
	} {
		for _, encoded := range []bool{false, true} {
			rp, _ := groupMemberPlugin(t)
			rp.cache.Add(makeVRP("10.0.0.0/24", 24, 65000))
			raw := append([]byte{0x40, 2, byte(len(path))}, path...)
			event := &bgp.Event{
				Message: &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 71},
				Peer:    json.RawMessage(`{"local":{"as":65000},"remote":{"address":"192.0.2.50","as":65000}}`),
				FamilyOps: map[family.Family][]bgp.FamilyOperation{
					family.IPv4Unicast: {{Action: routeaction.Add, NLRIs: []any{"10.0.0.0/24"}}},
				},
			}
			if encoded {
				event.RawAttributes = hex.EncodeToString(raw)
			} else {
				event.RawAttributeBytes = raw
			}
			rp.handleEvent(event)
			decisions := rp.buildDecisions(drainRequests(rp.validateCh))
			require.Len(t, decisions, 1)
			want := ValidationValid
			if len(path) > 0 {
				if path[0] == 1 {
					want = ValidationInvalid
				}
			}
			assert.Equal(t, want, decisions[0].ValState, "path=%x encoded=%t", path, encoded)
		}
	}
}

// An UPDATE can arrive before the first RTR load. Cache availability is an
// observable transition even when both validation verdicts stay unchanged.
func TestCacheAvailabilityRepublishesUnchangedRPKIVerdicts(t *testing.T) {
	for _, name := range []string{"disabled", "unknown", "valid"} {
		t.Run(name, func(t *testing.T) {
			rp, bridge := aspaRolePlugin(t, "provider")
			if name == "disabled" {
				rp.aspaEnabled.Store(false)
			}
			if name == "valid" {
				rp.aspaCache.Set(200, []uint32{100})
				rp.aspaCache.Set(300, []uint32{200})
			}
			type observedEvent struct {
				BGP struct {
					Peer struct {
						Name   string `json:"name"`
						Remote struct {
							Address string `json:"address"`
							AS      uint32 `json:"as"`
						} `json:"remote"`
					} `json:"peer"`
					Message struct {
						ID uint64 `json:"id"`
					} `json:"message"`
					RPKI struct {
						Status    string            `json:"status"`
						ASPAState *string           `json:"aspa-state"`
						Origins   map[string]string `json:"ipv4/unicast"`
					} `json:"rpki"`
				} `json:"bgp"`
			}
			var events []observedEvent
			bridge.SetEmitEvent(func(_, _, _, _, raw string) (int, error) {
				var event observedEvent
				err := json.Unmarshal([]byte(raw), &event)
				if err == nil {
					events = append(events, event)
				}
				return 1, err
			})
			reqs := feedASPAUpdate(t, rp,
				aspaMPReachBody(1, 1, []byte{192, 0, 2, 50}, []byte{24, 10, 0, 1}))
			require.Len(t, reqs, 1)
			require.Equal(t, ValidationNotFound, reqs[0].state)
			require.Len(t, events, 1)
			require.Equal(t, "unavailable", events[0].BGP.RPKI.Status)

			// This unrelated VRP makes the cache available without changing
			// this route's NotFound origin or its existing ASPA verdict.
			rp.cache.Add(makeVRP("203.0.113.0/24", 24, 65001))
			rp.handleROAChange()
			assert.Empty(t, drainRequests(rp.validateCh),
				"availability alone must not rerun an unchanged eligibility decision")
			require.Len(t, events, 2)
			got := events[1].BGP
			assert.Empty(t, got.RPKI.Status)
			assert.Equal(t, "not-found", got.RPKI.Origins["10.0.1.0/24"])
			if name == "disabled" {
				assert.Nil(t, got.RPKI.ASPAState)
			} else {
				require.NotNil(t, got.RPKI.ASPAState)
				assert.Equal(t, name, *got.RPKI.ASPAState)
			}
			assert.Equal(t, "ix-192.0.2.50", got.Peer.Name)
			assert.Equal(t, "192.0.2.50", got.Peer.Remote.Address)
			assert.Equal(t, uint32(100), got.Peer.Remote.AS)
			assert.Equal(t, uint64(71), got.Message.ID)

			rp.cache.Clear()
			rp.handleROAChange()
			require.Len(t, events, 3)
			assert.Equal(t, "unavailable", events[2].BGP.RPKI.Status)
		})
	}
}
