// Design: docs/architecture/l2tp/followup-l2tp-call.md -- pre-bridge native release.
package pppoe

import (
	"crypto/sha256"
	"fmt"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/report"
)

// The report retains the same original admission subject as the prior
// post-bridge release. It is emitted before refusing fresh channel capture.
func (h *realmHandoff) reportBridgeReleaseFailure(err error) {
	report.RaiseWarningFix(ppp.CleanupReportSource, ppp.CodeDoctorPPPCleanup,
		fmt.Sprintf("realm-admission/%x", sha256.Sum256([]byte(h.request.SubscriberID))),
		report.FitMessage(fmt.Sprintf("PPPoE realm relay admission release failed for subscriber %s on access interface %d in realm %s; cleanup is unconfirmed",
			h.request.SubscriberID, h.ifindex, h.realm)), ppp.CleanupFailureFix,
		map[string]any{"subscriber-id": h.request.SubscriberID, "ifindex": h.ifindex,
			"realm": h.realm, "error": err.Error()})
	h.relay.logger.Warn("pppoe: realm relay admission release failed",
		"code", ppp.CodeDoctorPPPCleanup, "fix", ppp.CleanupFailureFix,
		"subscriber", h.request.SubscriberID, "error", err)
}
