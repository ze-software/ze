// Design: ai/rules/feature-gate-registration.md -- ze_l2tp partition of the capture display
// Related: capture.go -- the always-on capture dispatcher this fills in

//go:build ze_l2tp

package cmd

import (
	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
)

// captureL2TPInto fills the l2tp section of `show capture`: the moved l2tp
// branch of HandleShowCapture. The BGP branch reaches its captures through the
// plugin.BGPCaptureProvider seam; l2tp predates that inversion and calls the
// nil-safe service locator directly, so the direct import lives in this gated
// file instead.
func captureL2TPInto(result map[string]any, protocol string, limit int, tunnelIDFilter uint16, peerFilter string) {
	if protocol != "" && protocol != capL2TP {
		return
	}
	svc := l2tp.LookupService()
	if svc != nil {
		entries := svc.CaptureSnapshot(limit, tunnelIDFilter, peerFilter)
		if entries != nil {
			result["l2tp"] = entries
			result["l2tp-count"] = len(entries)
		} else {
			result["l2tp"] = "capture not enabled"
		}
	} else if protocol == capL2TP {
		result["l2tp"] = msgSubsystemNotRunning
	}
}
