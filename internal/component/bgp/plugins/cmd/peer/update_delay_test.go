// VALIDATES: `show bgp update-delay` answers the one question a holding daemon
//            cannot otherwise be asked: is this speaker withholding its first
//            advertisement, and what is it waiting for.
// PREVENTS:  an operator having to read an INFO log line the default WARN level
//            suppresses in order to tell a converging speaker from a wedged one.
//            That is why the interop scenario could not assert on the log at
//            all, and it is the gap this command closes.

package peer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestBgpUpdateDelayReportsAHoldInProgress(t *testing.T) {
	reactor := &mockReactor{updateDelay: plugin.UpdateDelayStatus{
		Configured:           true,
		MaxDelaySeconds:      30,
		EstablishWaitSeconds: 10,
		Holding:              true,
		Reason:               "not-released",
		ExpectedPeers:        3,
		PeersHeld:            2,
		PeersConverged:       1,
	}}

	response, err := handleBgpUpdateDelay(newTestContext(reactor), nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)

	data, ok := response.Data.(plugin.Map)
	require.True(t, ok, "the payload must be structured data, so | json and | table each render it")
	require.Equal(t, true, data[fieldUpdateDelayConfigured])
	require.Equal(t, true, data[fieldUpdateDelayHolding])
	require.Equal(t, false, data[fieldUpdateDelayReleased])
	require.Equal(t, "not-released", data[fieldUpdateDelayReason])
	require.Equal(t, 3, data[fieldUpdateDelayExpectedPeers])
	require.Equal(t, 2, data[fieldUpdateDelayPeersHeld])
	require.Equal(t, 1, data[fieldUpdateDelayPeersConverged],
		"the converged count is what names the neighbor that has not finished")
	require.Equal(t, 30, data[fieldUpdateDelayMaxDelay])
	require.Equal(t, 10, data[fieldUpdateDelayEstablishWait])
}

// TestBgpUpdateDelayReportsAnUnconfiguredDaemon is the other half. An operator
// must be able to tell "the feature is off" from "the feature is holding", and
// both answer the same command.
func TestBgpUpdateDelayReportsAnUnconfiguredDaemon(t *testing.T) {
	response, err := handleBgpUpdateDelay(newTestContext(&mockReactor{}), nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)

	data, ok := response.Data.(plugin.Map)
	require.True(t, ok)
	require.Equal(t, false, data[fieldUpdateDelayConfigured])
	require.Equal(t, false, data[fieldUpdateDelayHolding])
	require.Equal(t, false, data[fieldUpdateDelayReleased])
	require.Equal(t, 0, data[fieldUpdateDelayMaxDelay])
	// Reason is the one field an unconfigured daemon does NOT leave at the Go
	// zero value, and both producers spell it this way. Asserting the empty
	// string here would pin a record no daemon emits.
	require.Equal(t, plugin.UpdateDelayReasonNotReleased, data[fieldUpdateDelayReason])
}

// TestBgpUpdateDelayFailsClosedWithoutAReactor pins the guard. A handler that
// returned an empty record with no reactor would publish "not configured" for a
// daemon whose engine is absent, which is a different fact wearing the same
// shape (ai/rules/principles.md).
func TestBgpUpdateDelayFailsClosedWithoutAReactor(t *testing.T) {
	response, err := handleBgpUpdateDelay(newTestContext(nil), nil)
	require.Error(t, err)
	require.Equal(t, plugin.StatusError, response.Status)
	require.Contains(t, response.Error, "reactor")
}
