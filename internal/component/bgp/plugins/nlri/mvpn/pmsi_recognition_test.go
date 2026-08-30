// Related: register.go — the PMSI_TUNNEL name registration
//
// VALIDATES: PMSI_TUNNEL is NAMED for display and not claimed as recognized,
// because no parser reads attribute code 22.
// PREVENTS: ze forwarding a peer's PMSI attribute with the Partial bit clear,
// which RFC 4271 Section 9 requires to be SET on an unrecognized optional
// transitive attribute.
//
// It lives in this package rather than beside Recognized itself, because the
// registration runs from this package's init and internal/core/bgp/attribute
// does not import it. A test asserting the live registration has to be where
// the registration happens.
package mvpn

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

func TestPMSITunnelIsNamedButNotRecognized(t *testing.T) {
	const pmsi = attribute.AttributeCode(22)

	if pmsi.Recognized() {
		t.Error("PMSI_TUNNEL is registered as recognized while no parser reads it: " +
			"code 22 is absent from knownAttrParsers and no code reads a PMSI byte. " +
			"ze would forward a peer's PMSI attribute with the Partial bit clear")
	}
	if got := pmsi.String(); got != "PMSI_TUNNEL" {
		t.Errorf("PMSI_TUNNEL renders as %q; the name is still owed to an operator "+
			"reading a dump, only the recognition claim is not", got)
	}
}
