// Design: ai/rules/plugins.md -- ze_l2tp-off raw-capture stub
// Related: capture_raw_l2tp.go -- the real l2tp branches (ze_l2tp builds)

//go:build !ze_l2tp

package cmd

// The ze_l2tp-off counterparts: `capture-raw` still serves BGP and BFD, and an
// explicit l2tp request states the feature is not in this build instead of
// pretending the subsystem is merely not running.

func captureRawL2TPStart(started []string, _ string) []string { return started }

func captureRawL2TPStop(stopped []string, _ string) []string { return stopped }

// captureRawL2TPNote makes an explicit `capture-raw start|stop l2tp` answer
// honestly instead of returning an empty list with no explanation (the
// stub-honesty rule in ai/rules/plugins.md).
func captureRawL2TPNote(protocol string) string {
	if protocol == capL2TP {
		return l2tpNotInBuild
	}
	return ""
}

func captureRawL2TPDump(result map[string]any, protocol, _ string, _ int) {
	if protocol == capL2TP {
		result["l2tp"] = l2tpNotInBuild
	}
}
