// Design: docs/guide/graceful-restart.md -- Receiving Speaker procedures
// Overview: gr.go -- handleStructuredState and handleStateEvent, which read the close reason
// Related: rfc4724_close_reason_test.go -- the NOTIFICATION and transport closes
// Related: rfc4724_retention_test.go -- newRecordedGRPlugin

package gr

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAdjRIBInInvalidCloseRetainsNothing delivers the down the reactor sends for a
// session whose received UPDATEs it could not publish and whose Cease never reached the
// wire, on both paths a down reaches Graceful Restart: the structured one in-process
// plugins take and the JSON event an external process decodes.
//
// VALIDATES: rpc.ReasonAdjRIBInInvalid makes both handlers dispatch no retention for a
// GR-capable peer, through rpc.ReasonForbidsRetention (spec
// spec-bgp-rs-replacement-on-withdrawal, AC-17 GR-capable half).
// PREVENTS: GR keeping, for a whole restart time, routes the plugins never saw, or that
// the peer withdrew in an UPDATE that was never published.
func TestAdjRIBInInvalidCloseRetainsNothing(t *testing.T) {
	t.Run("structured", func(t *testing.T) {
		gp, rec := newRecordedGRPlugin()
		gp.peerCaps[testPeer] = testCap(120, famIPv4)

		gp.handleStructuredState(testPeer, rpc.SessionStateDown, rpc.ReasonAdjRIBInInvalid)

		assert.Empty(t, rec.all(), "an invalid Adj-RIB-In takes the normal BGP procedures")
	})
	t.Run("json event", func(t *testing.T) {
		gp, rec := newRecordedGRPlugin()
		gp.peerCaps[testPeer] = testCap(120, famIPv4)

		gp.handleStateEvent(testPeer, map[string]any{"state": "down", "reason": rpc.ReasonAdjRIBInInvalid})

		assert.Empty(t, rec.all(), "an invalid Adj-RIB-In takes the normal BGP procedures")
	})
}
