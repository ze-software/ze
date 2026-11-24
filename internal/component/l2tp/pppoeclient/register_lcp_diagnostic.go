// Design: docs/architecture/doctor-and-health-checks.md -- PPPoE client LCP write failures carry a registered doctor code.
// Related: ipcp_independent.go -- no-NCP idle LCP Terminate-Request write failure.
// Related: network_phase.go -- network-phase LCP Protocol-Reject write failure.

package pppoeclient

import (
	"log/slog"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	codePPPoEClientLCPWrite = "doctor-pppoe-client-lcp-write"
	lcpWriteFix             = "ze show errors"
	lcpWriteWhy             = "The PPPoE session transport refused an outgoing LCP control frame; inspect the interface and reconnect state."
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codePPPoEClientLCPWrite,
		Title:       "PPPoE client LCP control frame write failed",
		Description: lcpWriteWhy + " Recorded when the PPPoE client cannot write an LCP Protocol-Reject or idle-timeout LCP Terminate-Request on the session channel.",
		Examples:    []string{lcpWriteFix, "ze show diagnostic " + codePPPoEClientLCPWrite},
	}); err != nil {
		panic("BUG: pppoe-client: LCP write diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codePPPoEClientLCPWrite}); err != nil {
		panic("BUG: pppoe-client: LCP write event registration failed: " + err.Error())
	}
}

func reportLCPWriteFailure(logger *slog.Logger, subject, problem string, err error) {
	logger.Warn(problem, "code", codePPPoEClientLCPWrite, "error", err)
	report.RaiseWarningFix("pppoe-client", codePPPoEClientLCPWrite, subject,
		report.FitMessage(problem+": "+err.Error()), lcpWriteFix)
}
