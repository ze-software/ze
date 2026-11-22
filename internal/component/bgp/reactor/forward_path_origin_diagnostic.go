// Design: docs/architecture/doctor-and-health-checks.md -- bounded historical forwarding refusals.
// Related: forward_path_origin.go -- original source mapping validation.
package reactor

import (
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

const codeForwardPathOrigin = "doctor-bgp-forward-path-origin"
const forwardPathOriginFix = "ze request restart"

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{Code: codeForwardPathOrigin, Title: "BGP forwarding lost original path identifier provenance",
		Description: "Ze refused path identifier work because the original source receipt, logical peer owner, physical connection or source identity could not be resolved. Inspect this event's cause and restart Ze to reestablish source session facts.",
		Examples:    []string{forwardPathOriginFix, "ze show diagnostic " + codeForwardPathOrigin}}); err != nil {
		panic("BUG: BGP path origin diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeForwardPathOrigin}); err != nil {
		panic("BUG: BGP path origin event registration failed: " + err.Error())
	}
}

// reportForwardPathOriginFailure records the actual refusal cause once per
// peer per second in the existing bounded event ring. Safe for concurrent use.
// The caller MUST refuse this operation after reporting; no source facts are guessed.
func reportForwardPathOriginFailure(peer string, cause error) {
	message := report.FitMessage("BGP path identifier work refused because its original source provenance is unavailable: " + cause.Error())
	if !report.RaiseErrorFixOnce(reportSourceBGP, codeForwardPathOrigin, peer, message, forwardPathOriginFix, time.Second) {
		return
	}
	fwdLogger().Warn(message, "peer", peer, "code", codeForwardPathOrigin, "fix", forwardPathOriginFix)
}
