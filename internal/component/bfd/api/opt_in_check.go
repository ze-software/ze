// Design: docs/guide/bfd.md -- a client opting into BFD needs BFD running
// Related: profile_check.go -- the same commit seam shape, for a named profile
//
// Commit-time opt-in check. A BGP peer (`connection bfd`), a static next-hop
// (`bfd-profile`) and an OSPF interface (`bfd { enabled true }`) opt into BFD.
// A candidate config that holds such an opt-in while the bfd section is absent
// or sets `enabled false` cannot give that client a session, so it is refused
// at commit, naming every client still opting in. BFD therefore cannot be
// removed from under a protected client: the operator removes the opt-ins
// first, in the same commit or an earlier one.
package api

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
)

// EnabledChecker answers whether the candidate bfd section runs BFD for its
// clients: nil when it does, and the reason it does not otherwise. bfdData is
// the Data of the config section whose Root is ConfigRoot, and empty when the
// candidate has none.
type EnabledChecker func(bfdData string) error

// errNoEnabledChecker is the answer when the bfd plugin is not in this build:
// no session can ever be opened, so no client may opt in.
var errNoEnabledChecker = errors.New("this build has no BFD plugin")

var enabledChecker atomic.Pointer[EnabledChecker]

// SetEnabledChecker registers the bfd plugin's enabled check. Called once from
// the bfd plugin's init().
func SetEnabledChecker(fn EnabledChecker) {
	enabledChecker.Store(&fn)
}

// CheckOptIns refuses a candidate config in which the clients named in optIns
// opt into BFD while the candidate bfd section does not run it. optIns names
// each client the way the operator wrote it ("peer 192.0.2.1"); an empty list
// is always accepted, because no client then depends on BFD. Safe for
// concurrent use.
func CheckOptIns(bfdData string, optIns []string) error {
	if len(optIns) == 0 {
		return nil
	}
	reason := errNoEnabledChecker
	if fn := enabledChecker.Load(); fn != nil {
		reason = (*fn)(bfdData)
	}
	if reason == nil {
		return nil
	}
	return fmt.Errorf("bfd: %w, so no client may opt into BFD; remove the bfd opt-in of %s first",
		reason, strings.Join(optIns, ", "))
}
