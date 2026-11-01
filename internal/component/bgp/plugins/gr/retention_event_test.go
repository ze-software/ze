// Design: docs/guide/graceful-restart.md -- the bgp-gr/retention event.
package gr

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRetentionEmittedOnEveryDown proves bgp-gr ends every DOWN it handles
// with one bgp-gr/retention verdict for that peer, on both event paths: the
// retained families after a TCP failure of a GR peer, and an empty verdict
// after a NOTIFICATION close and after a peer removal.
// PREVENTS: bgp-rr and bgp-rs deciding a DOWN without its verdict, or from an
// earlier DOWN's (R-10). RFC 4724 Section 4.2: the receiving speaker "MUST
// retain the routes received from the peer for all the address families that
// were previously received in the Graceful Restart Capability".
func TestRetentionEmittedOnEveryDown(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason string
		want   []string
	}{
		{name: "tcp failure retains the GR families", reason: rpc.ReasonTCPFailure, want: []string{"ipv4/unicast"}},
		{name: "notification retains nothing", reason: rpc.ReasonNotification, want: []string{}},
		{name: "peer removed retains nothing", reason: rpc.ReasonPeerRemoved, want: []string{}},
	} {
		for _, path := range []string{"structured", "text"} {
			t.Run(tc.name+" "+path, func(t *testing.T) {
				gp := newGRPluginWithGRCap()
				var emitted []string
				gp.emitHook = func(payload string) { emitted = append(emitted, payload) }
				if path == "structured" {
					gp.handleStructuredState(removalTestPeer, rpc.SessionStateDown, tc.reason)
				} else {
					gp.handleStateEvent(removalTestPeer, map[string]any{"state": "down", "reason": tc.reason})
				}
				require.Len(t, emitted, 1, "one verdict per DOWN")
				var verdict retention.Verdict
				require.NoError(t, json.Unmarshal([]byte(emitted[0]), &verdict))
				require.Equal(t, removalTestPeer, verdict.PeerAddress)
				require.Equal(t, tc.want, verdict.Families)
			})
		}
	}
}
