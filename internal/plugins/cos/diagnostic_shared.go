// Design: docs/architecture/traffic/cos-dynamic.md -- original-owner diagnostics
// Related: diagnostic_publisher.go -- explicit daemon publisher and local codes

//go:build ze_l2tp

package cos

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// newSharedCoSReportKey is called once when an original owner is captured.
// Its bounded nonce identifies only a diagnostic lifetime, never permission
// to write native state, an interface identity, or a configured generation.
func newSharedCoSReportKey() string { return "access-" + rand.Text() }

// policyFailure records a refusal as history. Caller MUST hold h.mu and pass
// the actual refusal cause. The returned error preserves both that cause and
// a failed publication; event callbacks rely on the same single WARN record.
func (h *cosHandler) policyFailure(labels sessionDiagnostic, name, profile string, cause error) error {
	failure := opererr.Wrap(cause, "Class-of-service profile "+strconv.Quote(profile)+" was refused on access "+strconv.Quote(name),
		policyRefusalWhy, policyRefusalFix)
	subject := "session-" + strconv.FormatUint(uint64(labels.tunnelID), 10) + "/" + strconv.FormatUint(uint64(labels.sessionID), 10)
	publishErr := h.reportIssue(rpc.ReportIssueInput{
		Action: rpc.ReportRaise, Severity: rpc.ReportError,
		Code: codeCoSPolicyRefused, Subject: subject,
		Message: report.FitMessage(failure.Problem() + ". " + failure.Why() + " Cause: " + cause.Error()), Fix: failure.Fix(),
	})
	logger().LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", codeCoSPolicyRefused), slog.String("interface", name),
			slog.String("profile", profile), slog.Uint64("tunnel", uint64(labels.tunnelID)),
			slog.Uint64("session", uint64(labels.sessionID)), slog.Any("publication-error", publishErr))...)
	return errors.Join(failure, publishErr)
}

// cleanupFailure publishes through the actual resource episode before asking
// the existing report callback to acknowledge its observation. The callback
// alone is no resource authority. Its ordinary daemon warning write is a
// protected-subject no-op; policy history keeps its ordinary publisher.
// Caller MUST hold h.mu and pass the retained original access and raw cause.
func (h *cosHandler) cleanupFailure(access *sharedCoSAccess, labels sessionDiagnostic, cause error) error {
	fix := cleanupFix
	var unavailable *originalCoSTargetUnavailable
	if errors.As(cause, &unavailable) {
		fix = originalCoSTargetFix
	}
	if _, unowned := access.target.(*unresolvedCoSTarget); unowned {
		fix = originalCoSTargetFix
	}
	failure := opererr.Wrap(cause, "Class-of-service cleanup is not confirmed on access "+strconv.Quote(access.name),
		cleanupWhy, fix)
	message := failure.Problem() + ". " + failure.Why() + " Cause: " + cause.Error()
	publishErr := h.publishCleanupWarning(access, message, failure.Fix())
	logger().LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", codeCoSCleanupUnconfirmed), slog.String("interface", access.name),
			slog.Uint64("tunnel", uint64(labels.tunnelID)), slog.Uint64("session", uint64(labels.sessionID)),
			slog.Any("publication-error", publishErr))...)
	return errors.Join(failure, publishErr)
}

// cleanupClear is called only after original target recovery/readback and any
// required native release. A callback ACK alone cannot clear a reservation:
// the exact retained episode must clear successfully too. A failed ACK keeps
// that episode and a report-only retry; it never repeats confirmed native work.
// Caller MUST hold h.mu. Live/surviving users retain their reservation.
func (h *cosHandler) cleanupClear(access *sharedCoSAccess) error {
	if _, unraised := access.warning.(sharedCoSWarningUnraised); unraised {
		return h.closeUnusedCleanupReservation(access)
	}
	if access.episode == nil {
		return h.retainCleanupClearFailure(access, errors.New("the original resource warning episode is not retained"))
	}
	if err := h.reportIssue(rpc.ReportIssueInput{
		Action: rpc.ReportClear, Code: codeCoSCleanupUnconfirmed, Subject: access.reportKey,
	}); err != nil {
		return h.retainCleanupClearFailure(access, err)
	}
	if err := access.episode.Clear(); err != nil {
		return h.retainCleanupClearFailure(access, err)
	}
	access.episode = nil
	access.warning = sharedCoSWarningUnraised{}
	return h.closeUnusedCleanupReservation(access)
}

// publishCleanupWarning retains the same typed episode across a failed or
// unknown acknowledgement. Bounds and original/latest causes are the report
// package's, never a CoS copy of its store or ownership mechanism.
// Caller MUST hold h.mu. A failed real Publish never reaches the ACK callback.
func (h *cosHandler) publishCleanupWarning(access *sharedCoSAccess, message, fix string) error {
	bounded := report.FitResourceWarningCause(message)
	if access.episode == nil {
		episode, err := access.reportScope.NewEpisode(codeCoSCleanupUnconfirmed, bounded, fix)
		if err != nil {
			return err
		}
		access.episode = episode
	} else {
		if err := access.episode.Update(bounded); err != nil {
			return err
		}
	}
	access.warning = sharedCoSWarningAttempted{}
	if err := access.episode.Publish(); err != nil {
		return err
	}
	return h.reportIssue(rpc.ReportIssueInput{
		Action: rpc.ReportRaise, Severity: rpc.ReportWarning,
		Code: codeCoSCleanupUnconfirmed, Subject: access.reportKey,
		Message: report.FitMessage(message), Fix: fix,
	})
}

func (h *cosHandler) retainCleanupClearFailure(access *sharedCoSAccess, cause error) error {
	failure := opererr.Wrap(cause, "Class-of-service cleanup warning could not be cleared for access "+strconv.Quote(access.name),
		"The required original cleanup was confirmed, but warning clearance was not acknowledged. Retry the cleanup command to confirm the original warning is cleared.", cleanupFix)
	access.warning = &sharedCoSWarningClearPending{cause: failure}
	logger().LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", codeCoSCleanupUnconfirmed), slog.String("interface", access.name))...)
	return failure
}

// closeUnusedCleanupReservation releases diagnostics only after native release
// and an unused/cleared episode. A member, compensatable transaction or native
// pending variant keeps the original reservation. No native action runs here.
// Caller MUST hold h.mu; native target.finish MUST already have succeeded for
// an absent or report-only original owner.
func (h *cosHandler) closeUnusedCleanupReservation(access *sharedCoSAccess) error {
	if len(access.users) != 0 {
		return nil
	}
	if h.pending != nil {
		if h.pending.prepared().access == access {
			return nil
		}
	}
	if state := h.accesses[access.name]; state != nil {
		if state.sharedOwner() != access {
			return errors.New("the cleanup reservation belongs to another original access owner")
		}
		if _, reportOnly := state.(*sharedCoSReportPending); !reportOnly {
			return nil
		}
	}
	return access.reportScope.Close()
}
