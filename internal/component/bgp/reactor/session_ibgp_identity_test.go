// Design: docs/architecture/encoding-context.md — the identity an EncodingContext carries
// Overview: session_negotiate.go — negotiateWith, the producer under test
// Related: session_as_migration.go — isIBGPWith, the one rule for the internal verdict
// Related: peer.go — openAdvertisedAS and sessionPeerAS, the AS this session's peer is judged by
// RFC: rfc/short/rfc4271.md — the Decision Process takes a different arm for an internal peer
// RFC: rfc/short/rfc6793.md — a four-octet speaker sends AS_TRANS in My Autonomous System

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestNegotiatedIdentityAfterOpenExchange drives a real OPEN exchange over the handleOpen
// rail and asks the encoding context the ANSWER the rest of ze reads: is this peer internal.
//
// VALIDATES: after a session negotiates from OPEN bytes, the EncodingContext built from the
// negotiation carries the peer's advertised AS and reports an internal peer as internal, for
// a two-octet AS and for a four-octet AS that arrives as AS_TRANS plus the Four-octet AS
// capability.
//
// PREVENTS: the eBGP arm being taken for every session. negotiateWith passed the raw
// message.Open.ASN4 field, which UnpackOpen never populates, so Negotiated.PeerASN was 0 on
// every established session and the context answered eBGP for a genuine iBGP peer. The
// LOCAL_PREF that RFC 4271 Section 5.1.5 requires on an internal UPDATE was then omitted by
// the commit rail (rib.CommitService.isIBGP, commit.go), which reads exactly this verdict.
//
// A unit test over capability.Negotiate cannot see this: it hands the peer AS in by
// argument, so it passes against the broken call site. The OPEN bytes are the entry point,
// so they are what this drives.
func TestNegotiatedIdentityAfterOpenExchange(t *testing.T) {
	// localID differs from the 10.0.0.1 identifier each OPEN body carries, so RFC 6286
	// Section 2.2 does not refuse the internal cases for presenting ze's own identifier.
	const localID uint32 = 0x0A000002

	const fourOctetAS uint32 = 4200000000

	tests := []struct {
		name       string
		localAS    uint32
		peerAS     uint32
		body       []byte
		wantPeerAS uint32
		wantIBGP   bool
	}{
		{
			name:       "two-octet internal peer",
			localAS:    65001,
			peerAS:     65001,
			body:       openBodyWithIdentifier(65001, 0x0A000001),
			wantPeerAS: 65001,
			wantIBGP:   true,
		},
		{
			name:       "four-octet internal peer sending AS_TRANS",
			localAS:    fourOctetAS,
			peerAS:     fourOctetAS,
			body:       openBodyWithASN4Capability(fourOctetAS),
			wantPeerAS: fourOctetAS,
			wantIBGP:   true,
		},
		{
			name:       "external peer",
			localAS:    65001,
			peerAS:     65002,
			body:       openBodyWithIdentifier(65002, 0x0A000001),
			wantPeerAS: 65002,
			wantIBGP:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, _ := openSentSessionASPair(t, tt.localAS, tt.peerAS, localID)

			require.NoError(t, session.handleOpen(tt.body))
			require.Equal(t, fsm.StateOpenConfirm, session.State())

			neg := session.negotiated
			require.NotNil(t, neg, "a session that reached OpenConfirm has negotiated")

			assert.Equal(t, tt.wantPeerAS, neg.Identity.PeerASN,
				"the identity carries the AS the peer advertised in its OPEN")

			ctx := bgpctx.FromNegotiatedSend(neg)
			require.NotNil(t, ctx)
			assert.Equal(t, tt.wantIBGP, ctx.IsIBGP(),
				"the send context reports the session type every encoder branches on")
		})
	}
}
