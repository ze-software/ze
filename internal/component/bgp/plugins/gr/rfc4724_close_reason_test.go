// Design: docs/guide/graceful-restart.md -- Receiving Speaker procedures
// RFC: rfc/short/rfc4724.md -- Section 4, NOTIFICATION follows the normal BGP procedures
// Overview: gr.go -- handleStructuredState, which reads the reactor's close reason
// Related: rfc4724_retention_test.go -- newRecordedGRPlugin, the production callbacks with the
// dispatch captured

package gr

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC4724NotificationCloseRetainsNothing delivers the down the reactor sends for a
// session that ended by NOTIFICATION, on the structured path in-process plugins take.
//
// VALIDATES: the reason the reactor names a NOTIFICATION close (rpc.ReasonNotification,
// reactor/peer_close.go) makes Graceful Restart dispatch no retention at all.
// PREVENTS: GR comparing against a string the reactor does not send, which kept every
// route of a peer whose session ended by NOTIFICATION.
//
// RFC requirement: RFC4724-4-2 positive -- a down with reason rpc.ReasonNotification for a
// GR-capable peer dispatches no purge-stale, retain-routes or mark-stale command, so the
// peer's routes are deleted by the normal BGP procedures.
func TestRFC4724NotificationCloseRetainsNothing(t *testing.T) {
	gp, rec := newRecordedGRPlugin()
	source := grFixtureSource(t, testPeer, 701, 41)
	gp.recordSentLLGR(source, nil)
	t.Cleanup(func() { gp.state.removePeer(source.grPeer) })
	gp.peerCaps[701] = testCap(120, famIPv4)

	gp.handleStructuredState(source, rpc.SessionStateDown, rpc.ReasonNotification)

	assert.Empty(t, rec.all(), "a NOTIFICATION close takes the normal BGP procedures")
}

// TestRFC4724TransportCloseRetains delivers the two downs the reactor sends for a session
// that ended with no NOTIFICATION: a lost transport and a close ze made itself.
//
// VALIDATES: both retain the captured original source and mark its routes stale.
// A previous retained lifetime would additionally owe its own purge.
// PREVENTS: a fix for the NOTIFICATION case that stops retaining for every close.
//
// RFC requirement: RFC4724-4-2 negative -- a down with reason rpc.ReasonTCPFailure or
// rpc.ReasonLocalClose for a GR-capable peer is not a NOTIFICATION close, and dispatches
// the exact source-scoped retain and stale-marking sequence.
func TestRFC4724TransportCloseRetains(t *testing.T) {
	for _, reason := range []string{rpc.ReasonTCPFailure, rpc.ReasonLocalClose} {
		t.Run(reason, func(t *testing.T) {
			gp, rec := newRecordedGRPlugin()
			source := grFixtureSource(t, testPeer, 701, 41)
			gp.recordSentLLGR(source, nil)
			t.Cleanup(func() { gp.state.removePeer(source.grPeer) })
			gp.peerCaps[701] = testCap(120, famIPv4)

			gp.handleStructuredState(source, rpc.SessionStateDown, reason)

			assert.Equal(t, []string{
				"request bgp rib source retain-routes owner 701 connection 41 on-down families ipv4/unicast",
				"request bgp rib source mark-stale owner 701 connection 41 restart-time 120",
			}, rec.all(), "the first DOWN retains and marks its exact original source; it has no prior stale lifetime to purge")
		})
	}
}
