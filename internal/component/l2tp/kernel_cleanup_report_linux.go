// Design: docs/guide/monitoring.md -- original native cleanup warning recovery
// Related: kernel_linux.go -- actual original-owner cleanup producers

//go:build linux

package l2tp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// These labels exist only at the log/report boundary, never as cleanup authority.
const (
	kernelCleanupChannelRelease = "session channel release"
	kernelCleanupSessionClose   = "session descriptor close"
	kernelCleanupSessionDelete  = "session removal"
	kernelCleanupTunnelClose    = "tunnel descriptor close"
	kernelCleanupTunnelDelete   = "tunnel removal"
	kernelCleanupFix            = "ze show warnings"
	kernelCleanupWhy            = "The original L2TP resources may still be present. Repair the reported Linux error and confirm their removal before restarting service."
)

// reportKernelCleanupFailure preserves the actual original cleanup failure.
// Callers MUST hold worker.mu and pass their actual nonnil operation error.
// Diagnostic labels and numeric IDs locate a report; they grant no I/O authority.
func (w *kernelWorker) reportKernelCleanupFailure(operation string, tid, sid uint16, birth subscriberid.L2TPBirth, fd int, cause error) {
	failure := opererr.Wrap(cause, "L2TP "+operation+" is unconfirmed", kernelCleanupWhy, kernelCleanupFix)
	w.logger.LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", diagnostic.CodeDoctorPPPCleanup),
			slog.Uint64(reportFieldTunnelID, uint64(tid)), slog.Uint64("session-id", uint64(sid)),
			slog.Int("fd", fd), slog.String("birth", birth.String()))...)
	detail := map[string]any{reportFieldTunnelID: tid, "operation": operation, "error": cause.Error()}
	if sid != 0 {
		detail["session-id"] = sid
	}
	if fd >= 0 {
		detail["fd"] = fd
	}
	if birth.IsValid() {
		detail["birth"] = birth.String()
	}
	report.RaiseWarningFix(ppp.CleanupReportSource, diagnostic.CodeDoctorPPPCleanup,
		kernelCleanupWarningSubject(operation, tid, sid, birth, fd),
		report.FitMessage(failure.Problem()+". "+failure.Why()+": "+cause.Error()), failure.Fix(), detail)
}

// clearKernelSessionDeleteWarning MUST be called under worker.mu only after
// the same original session's Delete confirms removal. Close warnings remain.
func clearKernelSessionDeleteWarning(key sessionKey, owner *pppSessionFDs) {
	report.ClearWarning(ppp.CleanupReportSource, diagnostic.CodeDoctorPPPCleanup,
		kernelCleanupWarningSubject(kernelCleanupSessionDelete, key.tunnelID, key.sessionID, kernelCleanupBirth(owner), -1))
}

// clearKernelTunnelDeleteWarning MUST be called under worker.mu only after
// this original tunnel's Delete confirms removal. Close warnings remain.
func clearKernelTunnelDeleteWarning(owner *kernelTunnelState) {
	report.ClearWarning(ppp.CleanupReportSource, diagnostic.CodeDoctorPPPCleanup,
		kernelCleanupWarningSubject(kernelCleanupTunnelDelete, owner.localTID, 0, owner.birth, -1))
}

// Operation labels and numeric/packed identity have fixed bounds below the
// report subject limit. A missing Birth is optional diagnostic metadata only.
func kernelCleanupWarningSubject(operation string, tid, sid uint16, birth subscriberid.L2TPBirth, fd int) string {
	return fmt.Sprintf("l2tp-kernel/%s/%d/%d/%s/%d", birth.String(), tid, sid, operation, fd)
}

// kernelCleanupBirth reads only the retained original row. A legacy/fixture
// owner without a row has no Birth to report; no replacement is looked up.
func kernelCleanupBirth(owner *pppSessionFDs) subscriberid.L2TPBirth {
	if owner.session == nil {
		return subscriberid.L2TPBirth{}
	}
	return owner.session.Birth()
}
