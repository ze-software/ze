// Design: docs/guide/graceful-restart.md -- received validation cleanup.
// Related: gr.go -- original source commands and final-writer completion.
package gr

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

var errGRSourceCommandRefused = errors.New("GR source command was refused")

func sourceCommandSubject(command string, args []string) string {
	return command + " " + strings.Join(args, " ")
}

// Failure enters the existing bounded recent-event report. It creates no
// retry queue or completed-source registry.
func (gp *grPlugin) reportValidationCleanupFailure(subject string, cause error) {
	failure := opererr.Wrap(cause, "GR received-validation cleanup was not completed",
		"Original metadata cleanup, replay settlement or final source output is incomplete", "Resolve the reported subscriber failure, then request reload")
	logger().LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", diagnostic.CodeDoctorRecentEvents), slog.String("command", subject))...)
	report.RaiseError("bgp-gr", diagnostic.CodeDoctorRecentEvents, subject, failure.Error())
}

// prepareValidationCleanup closes metadata/replay admission and joins the
// captured consumers BEFORE final source purge and its output barrier. A source
// receipt is session-wide: a retained sibling still holding it cannot authorize
// a removed path captured by an earlier replay. No final success precedes this
// join and the following existing source command.
func (gp *grPlugin) prepareValidationCleanup(source grSource, command string, args []string) bool {
	var families []family.Family
	noLLGR := false
	switch command {
	case "request bgp rib source release-routes":
	case "request bgp rib source purge-stale":
		if len(args) != 0 {
			if len(args) != 2 {
				return false
			}
			if args[0] != "family" {
				return false
			}
			fam, known := family.LookupFamily(args[1])
			if !known {
				return false
			}
			families = []family.Family{fam}
		}
	case "request bgp rib source delete-with-community":
		if len(args) != 4 {
			return false
		}
		if args[0] != "family" {
			return false
		}
		if args[2] != "community" {
			return false
		}
		if args[3] != "ffff0007" {
			return false
		}
		fam, known := family.LookupFamily(args[1])
		if !known {
			return false
		}
		families = []family.Family{fam}
		noLLGR = true
	default:
		// Retain, stale marking and decoration do not remove validation inputs.
		return true
	}
	payload, err := retention.EncodeCleanup(source.owner, source.connectionID, source.peer(), families, noLLGR)
	if err != nil {
		gp.reportValidationCleanupFailure(sourceCommandSubject(command, args), err)
		return false
	}
	if gp.cleanupHook != nil {
		gp.cleanupHook(payload)
		return true
	}
	if gp.sdk == nil {
		return true
	} // A source-only unit without a lifecycle observer.
	if _, err := gp.sdk.EmitEventApplied(context.Background(), retention.Namespace, retention.CleanupEventType, "", "", payload); err != nil {
		gp.reportValidationCleanupFailure(sourceCommandSubject(command, args), err)
		return false
	}
	return true
}
