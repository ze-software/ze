// Design: docs/architecture/doctor-and-health-checks.md -- captured control-send failures remain visible
// Related: teardown.go -- reports only a failed send after the exact local teardown
package l2tp

import (
	"context"
	"log/slog"
	"net/netip"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	codeL2TPControlSend = "doctor-l2tp-control-send"
	controlSendFix      = "ze show errors"
	controlSendWhy      = "The peer may not know that the local session ended. Inspect the recorded destination and socket error, then repair the local listener or network before retrying the call."
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code: codeL2TPControlSend, Title: "L2TP control message could not be sent",
		Description: controlSendWhy + " This event records a failed transmission after local captured-session teardown; peer receipt and native PPP cleanup remain separate outcomes.",
		Examples:    []string{controlSendFix, "ze show diagnostic " + codeL2TPControlSend},
	}); err != nil {
		panic("BUG: L2TP control-send diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeL2TPControlSend}); err != nil {
		panic("BUG: L2TP control-send event registration failed: " + err.Error())
	}
}

// reportCapturedControlSendFailure records only the failed control transmission.
// The caller MUST pass the destination and nonnil error of its actual Send.
// Safe for concurrent use; the bounded error ring retains the event after local
// teardown, without changing teardown ownership or claiming peer receipt.
func reportCapturedControlSendFailure(logger *slog.Logger, peer netip.AddrPort, cause error) {
	destination := peer.String()
	failure := opererr.Wrap(cause, "L2TP session teardown message could not be sent to "+destination, controlSendWhy, controlSendFix)
	logger.LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", codeL2TPControlSend), slog.String("peer", destination))...)
	// A numeric AddrPort has a fixed bounded textual representation, below the
	// bus subject limit. Only the unbounded socket error needs message fitting.
	report.RaiseErrorFix(SubsystemName, codeL2TPControlSend, destination,
		report.FitMessage(failure.Problem()+". "+failure.Why()+": "+cause.Error()), failure.Fix(),
		map[string]any{"peer": destination, "error": cause.Error()})
}
