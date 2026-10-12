// Design: docs/architecture/testing/interop.md -- native OSPF export at a BGP collector.
// Related: check_ospf_bgpls.go -- the check itself, behind the same gate
//
// The OSPF BGP-LS collector checks decode BGP-LS with the BGP engine's own
// types, which feature-gates.txt keeps behind ze_bgp. A build without that
// tag has no decoder, so each check fails and says why rather than letting
// the scenario pass with nothing verified.

//go:build !ze_bgp

package bgp

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// errOSPFBGPLSNeedsBGP is what the checks answer without the BGP gate.
var errOSPFBGPLSNeedsBGP = errors.New("interop-bgp OSPF BGP-LS check: this binary is built without ze_bgp, so it carries no BGP-LS decoder; rebuild it with the ze_bgp tag")

func checkOSPFTEBGPLS(context.Context, *interoplab.CheckContext) error {
	return errOSPFBGPLSNeedsBGP
}

func checkOSPFInterASBGPLS(context.Context, *interoplab.CheckContext) error {
	return errOSPFBGPLSNeedsBGP
}
