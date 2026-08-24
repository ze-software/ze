// Design: ai/rules/plugins.md -- ze_l2tp-off capture stub
// Related: capture_l2tp.go -- the real l2tp branch (ze_l2tp builds)

//go:build !ze_l2tp

package cmd

// l2tpNotInBuild is the one answer every l2tp-off capture path returns. It is
// a constant because three of them say it, here and in
// capture_raw_l2tp_off.go, and an operator comparing two answers must not read
// two spellings of the same fact.
const l2tpNotInBuild = "l2tp is not included in this build (ze_l2tp off)"

// captureL2TPInto is the ze_l2tp-off counterpart: `show capture` still
// answers, but an explicit l2tp request states the feature is not in this
// build instead of pretending the subsystem is merely not running.
func captureL2TPInto(result map[string]any, protocol string, _ int, _ uint16, _ string) {
	if protocol == capL2TP {
		result["l2tp"] = l2tpNotInBuild
	}
}
