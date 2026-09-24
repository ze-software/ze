package rpki

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// draft-ietf-sidrops-aspa-verification Section 5.2: "Let the sequence COMPRESSED_AS_PATH
// {AS(N), AS(N-1),..., AS(2), AS(1)} represent the AS_PATH after removing consecutive
// duplicate ASNs". normalizeASPath builds that sequence from the AS_PATH segments.

// TestASPACompressedASPath pins the consecutive-duplicate removal.
//
// VALIDATES: Prepends collapse to one hop while a non-consecutive repeat is kept.
// PREVENTS: counting a prepend as an extra hop, or removing a legitimate repeat that is
// not adjacent to its twin.
func TestASPACompressedASPath(t *testing.T) {
	t.Run("consecutive duplicates are removed", func(t *testing.T) {
		// Consecutive prepends collapse without changing the hop sequence.
		segments := []attribute.ASPathSegment{{
			Type: attribute.ASSequence,
			ASNs: []uint32{65001, 65001, 65002, 65002, 65002, 65003},
		}}

		hops, hasSet := normalizeASPath(segments)
		require.False(t, hasSet)
		assert.Equal(t, []uint32{65001, 65002, 65003}, hops)
	})

	t.Run("a non-consecutive repeat is kept", func(t *testing.T) {
		// Non-consecutive repeats survive; the path is not a set of ASNs.
		segments := []attribute.ASPathSegment{{
			Type: attribute.ASSequence,
			ASNs: []uint32{65001, 65002, 65001},
		}}

		hops, hasSet := normalizeASPath(segments)
		require.False(t, hasSet)
		assert.Equal(t, []uint32{65001, 65002, 65001}, hops)
		assert.NotEqual(t, []uint32{65001, 65002}, hops, "a repeat across another AS is not a duplicate")
	})
}

// observeCompressedASPAUpdate feeds a wire UPDATE through the receiving plugin
// and reads the event delivered to the engine plus the resulting route decision.
func observeCompressedASPAUpdate(t *testing.T, rp *rPKIPlugin, bridge *rpc.DirectBridge, segments []attribute.ASPathSegment) (string, bool) {
	t.Helper()
	var events []string
	bridge.SetEmitEvent(func(_, _, _, _, event string) (int, error) {
		events = append(events, event)
		return 1, nil
	})

	path := &attribute.ASPath{Segments: segments}
	pathBytes := make([]byte, path.Len())
	path.WriteTo(pathBytes, 0)
	require.Less(t, len(pathBytes), 256, "fixture uses a one-octet attribute length")
	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, byte(len(pathBytes))} //nolint:gosec // bounded above
	attrs = append(attrs, pathBytes...)
	attrs = append(attrs, 0x40, 0x03, 0x04, 192, 0, 2, 50)        // NEXT_HOP
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))} //nolint:gosec // fixture attribute block is bounded
	body = append(body, attrs...)
	body = append(body, 24, 10, 0, 0) // 10.0.0.0/24
	reqs := feedASPAUpdate(t, rp, body)

	require.Len(t, events, 1, "the received route must emit its RPKI verdict")
	var event struct {
		BGP struct {
			RPKI struct {
				ASPAState string            `json:"aspa-state"`
				Origins   map[string]string `json:"ipv4/unicast"`
			} `json:"rpki"`
		} `json:"bgp"`
	}
	require.NoError(t, json.Unmarshal([]byte(events[0]), &event))
	require.Equal(t, "valid", event.BGP.RPKI.Origins["10.0.0.0/24"],
		"origin validation must not cause the ASPA policy outcome")
	decisions := rp.buildDecisions(reqs)
	require.Len(t, decisions, 1)
	require.Equal(t, "10.0.0.0/24", decisions[0].Prefix)
	return event.BGP.RPKI.ASPAState, decisions[0].Accept
}

// TestASPACompressedUpdatePrependsValid observes the verdict, not the helper's
// compressed slice. The cache deliberately does not authorize any AS as its
// own provider, so an unremoved prepend changes Valid to Invalid.
//
// MUTATION: append every ASN in normalizeASPath, including consecutive repeats.
// The prepended paths then emit Invalid and the default policy rejects them.
//
// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1 positive -- normalizeASPath removes consecutive duplicates within and across AS_SEQUENCE segments before handleStructuredUpdate emits a Valid ASPA verdict and accepts the authorized route (revision 28 Section 5.2).
func TestASPACompressedUpdatePrependsValid(t *testing.T) {
	cases := []struct {
		name     string
		segments []attribute.ASPathSegment
		origin   uint32
	}{
		{
			name:     "no prepends",
			segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{100, 200, 300}}},
			origin:   300,
		},
		{
			name:     "prepends at every hop",
			segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{100, 100, 200, 200, 200, 300, 300}}},
			origin:   300,
		},
		{
			name: "prepend across segment boundary",
			segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{100, 200}},
				{Type: attribute.ASSequence, ASNs: []uint32{200, 300}},
			},
			origin: 300,
		},
		{
			name:     "single AS with prepends",
			segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{100, 100, 100}}},
			origin:   100,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rp, bridge := aspaRolePlugin(t, "provider")
			rp.cache.Add(makeVRP("10.0.0.0/24", 24, tc.origin))
			rp.aspaCache.Set(100, []uint32{999})
			rp.aspaCache.Set(200, []uint32{100})
			rp.aspaCache.Set(300, []uint32{200})

			state, accepted := observeCompressedASPAUpdate(t, rp, bridge, tc.segments)
			assert.Equal(t, "valid", state)
			assert.True(t, accepted, "prepends must not create an unauthorized self-hop")
		})
	}
}

// TestASPACompressedUpdatePreservesHops keeps the limits of compression visible
// in the emitted verdict: it cannot remove a separated repeat or an unknown hop.
//
// MUTATION: normalizeASPath drops any ASN already present, not only the previous
// ASN. The separated repeat then becomes [100 200 300], emits Valid and is accepted.
//
// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1 negative -- normalizeASPath must not remove non-consecutive ASNs: a separated repeat retains its unauthorized hop, emits Invalid, and is rejected; an unattested distinct hop remains Unknown (revision 28 Section 5.2).
func TestASPACompressedUpdatePreservesHops(t *testing.T) {
	cases := []struct {
		name     string
		path     []uint32
		state    string
		accepted bool
	}{
		{name: "separated repeat", path: []uint32{100, 200, 300, 200}, state: "invalid"},
		{name: "unauthorized distinct hop", path: []uint32{100, 300}, state: "invalid"},
		{name: "unattested distinct hop", path: []uint32{100, 400}, state: "unknown", accepted: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rp, bridge := aspaRolePlugin(t, "provider")
			rp.cache.Add(makeVRP("10.0.0.0/24", 24, tc.path[len(tc.path)-1]))
			rp.aspaCache.Set(200, []uint32{100})
			rp.aspaCache.Set(300, []uint32{200})

			segments := []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: tc.path}}
			state, accepted := observeCompressedASPAUpdate(t, rp, bridge, segments)
			assert.Equal(t, tc.state, state)
			assert.Equal(t, tc.accepted, accepted)
		})
	}
}
