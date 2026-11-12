package gok

import "github.com/gokrazy/tools/internal/packer"

// HelperCompletion identifies a successful privileged partitioning helper.
// Context.Execute preserves it so embedded callers can skip image or VM
// postwork, run deferred cleanup, and return success at their command boundary.
type HelperCompletion = packer.HelperCompletion

// IsHelperCompletion recognizes a pure helper-completion result. A joined or
// wrapped cleanup failure must remain an error, even if it contains a helper
// result. Main and the command implementations preserve completion unchanged.
func IsHelperCompletion(err error) bool {
	_, complete := err.(interface{ HelperCompletion() })
	return complete
}
