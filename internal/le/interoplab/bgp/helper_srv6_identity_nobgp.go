// Design: docs/architecture/testing/interop.md -- compiled BGP interop personalities.
// Related: helper_srv6_identity.go -- the helper itself, behind the same gate
//
// The SRv6 identity export filter decodes each UPDATE with the BGP engine's
// own message types, which feature-gates.txt keeps behind ze_bgp. A build
// without that tag has no decoder to run, so the registered helper says so
// rather than leaving the scenario without its process.

//go:build !ze_bgp

package bgp

import "errors"

// errSRv6IdentityNeedsBGP is what the helper answers without the BGP gate.
var errSRv6IdentityNeedsBGP = errors.New("interop-bgp process " + srv6IdentityScenario + ": this binary is built without ze_bgp, so it carries no BGP message decoder; rebuild it with the ze_bgp tag")

func runSRv6IdentityPolicy(string) error {
	return errSRv6IdentityNeedsBGP
}
