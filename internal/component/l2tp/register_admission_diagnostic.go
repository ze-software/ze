// Design: docs/architecture/doctor-and-health-checks.md -- recent events: an L2TP call or tunnel refused at its configured limit
// Related: session.go -- sessionSlotsFull refuses a session at max-sessions
// Related: session_retirement.go -- tunnelSlotsFullLocked refuses a tunnel at max-tunnels
package l2tp

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	codeL2TPAdmissionLimit = "doctor-l2tp-admission-limit"
	admissionLimitWhy      = "The configured limit is reached, so the LAC's new call or tunnel is refused until a session or tunnel ends."
	sessionLimitFix        = "set l2tp max-sessions <count>"
	tunnelLimitFix         = "set l2tp max-tunnels <count>"
	// admissionEventWindow bounds the events one subject raises: a LAC at its
	// limit retries every call, and one event per refusal would overwrite
	// every other owner's events in the bounded error ring.
	admissionEventWindow = time.Minute
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code: codeL2TPAdmissionLimit, Title: "L2TP call or tunnel refused at its configured limit",
		Description: admissionLimitWhy + " A session refused at max-sessions is answered by a CDN with Result Code 4; an SCCRQ refused at max-tunnels is dropped, and a dial is refused. " +
			"Raise the limit when the load is expected. A refusal caused by sessions whose cleanup is unconfirmed is reported under doctor-ppp-cleanup instead.",
		Examples: []string{"ze show errors", "ze show diagnostic " + codeL2TPAdmissionLimit},
	}); err != nil {
		panic("BUG: L2TP admission-limit diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeL2TPAdmissionLimit}); err != nil {
		panic("BUG: L2TP admission-limit event registration failed: " + err.Error())
	}
}

// reportSessionLimit logs and raises a session admission refused at
// max-sessions, once per tunnel per admissionEventWindow. Caller holds
// tunnelsMu; the report store takes no l2tp lock.
func reportSessionLimit(logger *slog.Logger, refusal string, localTID uint16, limit, live int) {
	failure := opererr.New("l2tp: "+refusal+"; max sessions reached", admissionLimitWhy, sessionLimitFix)
	logger.LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", codeL2TPAdmissionLimit),
			slog.Uint64(reportFieldTunnelID, uint64(localTID)), slog.Int("max", limit), slog.Int("current", live))...)
	subject := "session-slots/" + strconv.Itoa(int(localTID))
	report.RaiseErrorFixOnce(SubsystemName, codeL2TPAdmissionLimit, subject,
		report.FitMessage("L2TP tunnel "+strconv.Itoa(int(localTID))+": "+refusal+" at max-sessions "+
			strconv.Itoa(limit)+". "+admissionLimitWhy), failure.Fix(), admissionEventWindow)
}

// reportTunnelLimit logs and raises a tunnel admission refused at
// max-tunnels, once per admissionEventWindow. Its log text keeps
// "max-tunnels limit reached". Caller holds tunnelsMu.
func reportTunnelLimit(logger *slog.Logger, refusal string, peer slog.Attr, limit int) {
	failure := opererr.New("l2tp: max-tunnels limit reached; "+refusal, admissionLimitWhy, tunnelLimitFix)
	logger.LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", codeL2TPAdmissionLimit), peer, slog.Int("limit", limit))...)
	report.RaiseErrorFixOnce(SubsystemName, codeL2TPAdmissionLimit, "tunnel-slots",
		report.FitMessage("L2TP "+refusal+" at max-tunnels "+strconv.Itoa(limit)+". "+admissionLimitWhy),
		failure.Fix(), admissionEventWindow)
}
