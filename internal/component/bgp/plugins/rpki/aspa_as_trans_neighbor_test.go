// Design: docs/architecture/edge-cases/as4.md -- the ASPA neighbor-AS step for an AS_TRANS peer
// Related: rpki.go -- handleStructuredUpdate, the producer under test
// Related: aspa_verify.go -- aspaNeighbor and verifyASPANeighbor, the step this file proves

package rpki

import (
	"bytes"
	"encoding/binary"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// asFourOctetCompliant is the real AS of a compliant four-octet peer that a
// two-octet session can only name as AS_TRANS.
const asFourOctetCompliant = 4200000000

// asTransReconstructedBody is the UPDATE the plugin receives from a two-octet
// session whose peer is AS_TRANS, after the receive session's RFC 6793 Section
// 4.2.3 reconstruction: the peer sent AS_PATH [23456 65010] and AS4_PATH
// [64496 65010], so the path is [64496 65010] in four-octet form, with one
// IPv4 unicast prefix, 10.0.0.0/24.
var asTransReconstructedBody = []byte{
	0x00, 0x00, // Withdrawn Routes length 0
	0x00, 0x18, // Total Path Attribute Length 24
	0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
	0x40, 0x02, 0x0a, 0x02, 0x02, // AS_PATH: one AS_SEQUENCE of two 4-octet ASNs
	0x00, 0x00, 0xfb, 0xf0, // 64496
	0x00, 0x00, 0xfd, 0xf2, // 65010
	0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
	0x18, 0x0a, 0x00, 0x00, // NLRI 10.0.0.0/24
}

// reconstructedBody builds the UPDATE the plugin receives once the receive
// session has reconstructed the path: ORIGIN IGP, one AS_SEQUENCE of path in
// four-octet form, NEXT_HOP 10.0.0.1 and the prefix 10.0.0.0/24. path holds at
// most 255 ASNs, the most recently added first.
func reconstructedBody(t *testing.T, path ...uint32) []byte {
	t.Helper()
	require.LessOrEqual(t, len(path), 255, "one AS_SEQUENCE holds at most 255 ASNs")
	asPath := []byte{0x02, byte(len(path))}
	for _, asn := range path {
		asPath = binary.BigEndian.AppendUint32(asPath, asn)
	}
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	attrs = append(attrs, 0x40, 0x02, byte(len(asPath)))
	attrs = append(attrs, asPath...)
	attrs = append(attrs, 0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01)
	body := []byte{0x00, 0x00}
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs)))
	body = append(body, attrs...)
	return append(body, 0x18, 0x0a, 0x00, 0x00)
}

// feedASPAUpdateFrom drives body through handleStructuredUpdate from a session
// whose peer AS is peerAS and returns the validation requests it produced.
func feedASPAUpdateFrom(t *testing.T, rp *rPKIPlugin, peerAS uint32, body []byte) []validationRequest {
	t.Helper()
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err, "the test UPDATE body must parse")

	rp.handleStructuredUpdate(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: "192.0.2.50",
		PeerName:    "ix-192.0.2.50",
		PeerGroup:   "ix",
		PeerAS:      rpc.RemoteASOf(peerAS),
		LocalAS:     65000,
		MessageID:   72,
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   body,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	})
	return drainRequests(rp.validateCh)
}

// captureASPAWarnings installs a text logger over a buffer as the package
// logger until the test ends. The package logger is global, so a test that
// calls this MUST NOT run in parallel.
func captureASPAWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	previous := logger()
	t.Cleanup(func() { setLogger(previous) })
	var out bytes.Buffer
	setLogger(slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelWarn})))
	return &out
}

// TestASPAASTransNeighborUnconfirmed proves a route from a peer configured as
// AS_TRANS on a two-octet session is ASPA Invalid when the reconstructed
// path's most recently added AS is not the neighbor AS, and stays Invalid when
// a later ASPA change would authorize the path.
//
// VALIDATES: handleStructuredUpdate and the re-validation tracker apply the
// neighbor-AS step of Sections 5.5 and 5.6 to the reconstructed path when the
// receive session compared AS_TRANS with the AS_PATH as received.
// PREVENTS: ASPA verifying [64496 65010] with 64496 as the neighbor, a hop the
// peer chose in its own AS4_PATH and the session never confirmed.
func TestASPAASTransNeighborUnconfirmed(t *testing.T) {
	// The cache authorizes the path: 65010 names 64496 as its provider.
	rp, _ := aspaRolePlugin(t, "provider")
	rp.aspaCache.Set(65010, []uint32{64496})

	// Control: from a session whose peer is 64496 the same path is Valid, so an
	// Invalid below comes from the neighbor step and from nothing else.
	reqs := feedASPAUpdateFrom(t, rp, 64496, asTransReconstructedBody)
	require.Len(t, reqs, 1)
	require.Equal(t, ASPAValid, reqs[0].aspaState, "the cache must authorize the path")

	rp, _ = aspaRolePlugin(t, "provider")
	rp.aspaCache.Set(65010, []uint32{64496})
	reqs = feedASPAUpdateFrom(t, rp, attribute.ASTrans, asTransReconstructedBody)
	require.Len(t, reqs, 1, "the IPv4 unicast prefix must reach the decision path")
	assert.Equal(t, ASPAInvalid, reqs[0].aspaState,
		"ASPA took the reconstructed path's first AS as the AS_TRANS peer's neighbor AS")
	assert.False(t, rp.buildDecisions(reqs)[0].Accept, "an ASPA Invalid route is rejected")

	// Re-validation runs the same step: withdrawing the authorization and
	// granting it again must not turn the route Valid.
	rp.aspaCache.Set(65010, []uint32{999})
	rp.handleASPAChange([]uint32{65010})
	drainRequests(rp.validateCh)
	rp.aspaCache.Set(65010, []uint32{64496})
	rp.handleASPAChange([]uint32{65010})
	for _, req := range drainRequests(rp.validateCh) {
		assert.Equal(t, ASPAInvalid, req.aspaState,
			"re-validation verified the path without the neighbor-AS step")
	}
}

// TestASPAASTransNeighborCompliantFourOctetPeer pins the agreed behavior for a
// compliant four-octet peer behind a two-octet session. Per RFC 6793 Section
// 4.2.2 it sends AS_PATH [23456 65010] and AS4_PATH [4200000000 65010], which
// Section 4.2.3 reconstructs to [4200000000 65010]. Ze cannot confirm that
// 4200000000 is the neighbor, so the route is Invalid even though the cache
// authorizes the path, and the plugin logs one WARN for the peer, naming the
// fix, however many routes it sends.
//
// VALIDATES: the neighbor-AS step marks an honest AS_TRANS peer's routes
// Invalid, and verifyASPANeighbor warns once per peer with the fix.
// PREVENTS: a silent Invalid the operator cannot trace to the session config,
// and a warning repeated for every route.
func TestASPAASTransNeighborCompliantFourOctetPeer(t *testing.T) {
	body := reconstructedBody(t, asFourOctetCompliant, 65010)

	// Control: from a session that names the real AS the path is Valid.
	rp, _ := aspaRolePlugin(t, "provider")
	rp.aspaCache.Set(65010, []uint32{asFourOctetCompliant})
	reqs := feedASPAUpdateFrom(t, rp, asFourOctetCompliant, body)
	require.Len(t, reqs, 1)
	require.Equal(t, ASPAValid, reqs[0].aspaState, "the cache must authorize the path")

	warnings := captureASPAWarnings(t)
	rp, _ = aspaRolePlugin(t, "provider")
	rp.aspaCache.Set(65010, []uint32{asFourOctetCompliant})
	for range 3 {
		reqs = feedASPAUpdateFrom(t, rp, attribute.ASTrans, body)
		require.Len(t, reqs, 1)
		assert.Equal(t, ASPAInvalid, reqs[0].aspaState,
			"a compliant four-octet peer named only as AS_TRANS was not marked Invalid")
	}

	log := warnings.String()
	assert.Equal(t, 1, strings.Count(log, "ASPA marks every route from this peer Invalid"),
		"the neighbor-step warning must be logged once per peer, not once per route: %s", log)
	assert.Contains(t, log, "peer=192.0.2.50", "the warning must name the peer")
	assert.Contains(t, log, "capability { asn4 enable; }", "the warning must name the fix")
}

// TestASPAASTransNeighborFirstASIsASTrans proves the neighbor-AS step passes
// when the reconstructed path starts with AS_TRANS, so an AS_TRANS peer whose
// path is otherwise authorized stays Valid and logs nothing. Under RFC 6793
// Section 4.2.3 that happens when the AS4_PATH is absent, the path is then the
// AS_PATH [23456 65010] as received, and when the AS4_PATH [65020 65010] is
// shorter than the AS_PATH [23456 65020 65010], whose leading AS_TRANS the
// reconstruction keeps.
//
// VALIDATES: verifyASPANeighbor confirms a path whose first AS is the AS_TRANS
// neighbor, so the step forces Invalid only on a mismatch.
// PREVENTS: the neighbor-AS step rejecting every route of an AS_TRANS peer.
func TestASPAASTransNeighborFirstASIsASTrans(t *testing.T) {
	tests := []struct {
		name      string
		path      []uint32
		providers map[uint32][]uint32
	}{
		{
			name:      "AS4_PATH absent",
			path:      []uint32{attribute.ASTrans, 65010},
			providers: map[uint32][]uint32{65010: {attribute.ASTrans}},
		},
		{
			name: "AS4_PATH shorter than AS_PATH",
			path: []uint32{attribute.ASTrans, 65020, 65010},
			providers: map[uint32][]uint32{
				65010: {65020},
				65020: {attribute.ASTrans},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := captureASPAWarnings(t)
			rp, _ := aspaRolePlugin(t, "provider")
			for customer, providers := range tt.providers {
				rp.aspaCache.Set(customer, providers)
			}
			reqs := feedASPAUpdateFrom(t, rp, attribute.ASTrans, reconstructedBody(t, tt.path...))
			require.Len(t, reqs, 1)
			assert.Equal(t, ASPAValid, reqs[0].aspaState,
				"the neighbor-AS step refused a path whose first AS is the AS_TRANS neighbor")
			assert.NotContains(t, warnings.String(), "ASPA marks every route",
				"a confirmed neighbor must not log the neighbor-step warning")
		})
	}
}

// TestASPAASTransNeighborRSClientExempt proves the RS-client exception of
// Section 5.5 step 2 holds for an AS_TRANS route server: the receiving
// RS-client verifies the path without comparing its first AS.
func TestASPAASTransNeighborRSClientExempt(t *testing.T) {
	rp, _ := aspaRolePlugin(t, "rs-client")
	rp.aspaCache.Set(65010, []uint32{64496})
	reqs := feedASPAUpdateFrom(t, rp, attribute.ASTrans, asTransReconstructedBody)
	require.Len(t, reqs, 1)
	assert.Equal(t, ASPAValid, reqs[0].aspaState,
		"an RS-client applied the neighbor-AS step Section 5.5 exempts it from")
}
