// Design: docs/architecture/behavior/peer-lifecycle.md -- a failed connection rejection remains visible after closure.
// Related: reactor_connection.go -- rejectPendingConnection owns and closes the pending socket.
// Related: register_diagnostic.go -- metadata and recent-event registration for this failure.

package reactor

import (
	"net"

	"github.com/ze-software/ze/internal/core/report"
)

const codePendingRefusalUndelivered = "doctor-bgp-pending-refusal-undelivered"
const pendingRefusalFailureFix = "ze show bgp peer list"
const pendingRefusalFailureMessage = "BGP rejection delivery failed; Ze closes the pending connection, and the peer may not receive the reason"

// reportPendingRefusalFailure records a failure on an accepted connection that
// has no Session writer. The caller MUST retain ownership and close conn after
// reporting. This function takes no peer lock and closes no connection; its
// only retained state is an event in the existing bounded report bus.
func reportPendingRefusalFailure(conn net.Conn, operation string, cause error) {
	peer := conn.RemoteAddr().String()
	reactorLogger().Warn(pendingRefusalFailureMessage,
		"code", codePendingRefusalUndelivered, "peer", peer,
		"operation", operation, "error", cause, "fix", pendingRefusalFailureFix)
	report.RaiseErrorFix(reportSourceBGP, codePendingRefusalUndelivered, peer,
		report.FitMessage(pendingRefusalFailureMessage+": "+operation+": "+cause.Error()),
		pendingRefusalFailureFix, map[string]any{"operation": operation})
}
