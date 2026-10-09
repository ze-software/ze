// Design: docs/guide/monitoring.md — ze_bgp_open_rejected_bad_peer_as_total
// Related: session_open_as.go — rejectOpenPeerAS, the producer under test
// Related: reactor_metrics.go — openBadPeerAS, the counter it increments

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenBadPeerASCounted drives handleOpen with OPENs the session must refuse with
// Bad Peer AS, and one it must accept, and reads the counter after each.
//
// VALIDATES: every Bad Peer AS refusal increments ze_bgp_open_rejected_bad_peer_as_total
// under the peer's address label, for both refusal reasons (an AS the peer is not
// configured for, and AS zero), and an accepted OPEN leaves the counter untouched.
// PREVENTS: the refusal being visible only in the log, which is how the RFC 4271
// Section 6.2 tightening would otherwise reach an operator whose remote-as is mistyped.
func TestOpenBadPeerASCounted(t *testing.T) {
	const localID uint32 = 0x0A000002

	tests := []struct {
		name       string
		body       []byte
		wantCount  float64
		wantReject bool
	}{
		{"an AS the peer is not configured for", openBodyWithIdentifier(65099, 0x0A000001), 1, true},
		{"my-as zero", openBodyWithIdentifier(0, 0x0A000001), 1, true},
		{"the configured AS", openBodyWithIdentifier(65002, 0x0A000001), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := newSpyRegistry()
			session, _ := openSentSessionAS(t, 65002, localID)
			session.prefixMetrics = initReactorMetrics(registry, "test", "10.0.0.2", "65001")

			err := session.handleOpen(tt.body)
			if tt.wantReject {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			counter := registry.counterVecs["ze_bgp_open_rejected_bad_peer_as_total"]
			require.NotNil(t, counter, "the counter must be registered")
			series := counter.get(session.addrLabel)
			if tt.wantCount == 0 {
				assert.Nil(t, series, "an accepted OPEN must not create a rejection series")
				return
			}
			require.NotNil(t, series, "the refusal must be counted under the peer label")
			assert.InDelta(t, tt.wantCount, series.Value(), 0)
		})
	}
}
