// Design: docs/architecture/testing/interop.md -- exact typed-NLRI retention.
// Related: check_typed_discard_wire.go -- the wire check itself, behind the same gate
//
// The typed-NLRI discard check decodes the retained EVPN route with the BGP
// engine's own EVPN codec, which feature-gates.txt keeps behind ze_bgp. A
// build without that tag has no decoder, so the check fails and says why
// rather than letting the scenario pass with the retained route unread.

//go:build !ze_bgp

package bgp

import "errors"

// errTypedEVPNWireNeedsBGP is what the wire check answers without the BGP gate.
var errTypedEVPNWireNeedsBGP = errors.New("interop-bgp bgp-rfc7606-typed-nlri-discard: this binary is built without ze_bgp, so it carries no EVPN decoder and cannot read the retained route; rebuild it with the ze_bgp tag, which the launcher's own build carries, and run ./le test integration interop")

func requireTypedEVPNWire(string) error {
	return errTypedEVPNWireNeedsBGP
}
