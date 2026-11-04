// Design: docs/guide/bfd.md -- a client-named profile is checked at commit
// Related: profile_reference.go -- profileReference, the source registered here

package api

import "github.com/ze-software/ze/internal/component/config/yang"

func init() {
	// A BFD client's profile leaf declares `ze:reference "bfd-profile"`: the
	// editor completes it, and commit judges it, through this source.
	yang.RegisterReferenceSource(ProfileReferenceSource, profileReference)
}
