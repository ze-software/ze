// Design: docs/guide/bfd.md -- a client-named profile is checked at commit
// Related: registry.go -- the same in-process seam shape, for the live Service
//
// Commit-time profile check. A BGP peer (`connection bfd { profile ... }`) and
// a static next-hop (`bfd-profile`) name a profile the bfd plugin defines. The
// name is a plain string in their schemas, so without this seam an undefined
// or mode-incompatible profile passes commit and is refused only when the
// client asks for its session, which leaves the route or peer running with no
// BFD and a log line as the only trace.
//
// The client reads the candidate bfd section (its registration lists the bfd
// root under ConfigReads) and asks CheckProfile. The bfd plugin registers the
// answer from its own parser in init(), so the check a client runs at commit
// is the check EnsureSession runs at session start, never a second copy.
package api

import (
	"errors"
	"sync/atomic"
)

// ConfigRoot is the top-level YANG container the bfd plugin owns. A client
// lists it under ConfigReads to receive the candidate bfd section.
const ConfigRoot = "bfd"

// ProfileChecker answers whether the candidate bfd section defines profile
// and whether a session in mode may use it. bfdData is the Data of the config
// section whose Root is ConfigRoot, and empty when the candidate has none.
type ProfileChecker func(bfdData, profile string, mode HopMode) error

// errNoProfileChecker is the answer when the bfd plugin is not in this build:
// a profile name then names nothing, and no session could ever use it.
var errNoProfileChecker = errors.New("bfd: this build has no BFD plugin, so no bfd profile can be named")

var profileChecker atomic.Pointer[ProfileChecker]

// SetProfileChecker registers the bfd plugin's profile check. Called once from
// the bfd plugin's init().
func SetProfileChecker(fn ProfileChecker) {
	profileChecker.Store(&fn)
}

// CheckProfile answers whether a client in mode may name profile, given the
// candidate bfd section. Safe for concurrent use.
func CheckProfile(bfdData, profile string, mode HopMode) error {
	fn := profileChecker.Load()
	if fn == nil {
		return errNoProfileChecker
	}
	return (*fn)(bfdData, profile, mode)
}
