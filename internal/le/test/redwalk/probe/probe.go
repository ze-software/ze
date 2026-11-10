// Design: docs/contributing/running-commands.md -- le test redwalk check
//
// Package probe is the fixed target of the red walk's functional test
// (test/ui/le-test-redwalk.ci): one function and the test that guards it.
// The .ci's break patch is written against this text, so an edit here MUST
// update that patch.
package probe

// Double answers twice its argument.
func Double(value int) int {
	return value * 2
}
