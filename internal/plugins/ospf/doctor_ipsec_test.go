// VALIDATES: spec-ospf-ext-16 AC-12 / R-7 -- the OSPFv3 IPsec doctor check warns with
// doctor-ospfv3-ipsec when an IPv6-family interface configures IPsec but the kernel XFRM
// dataplane is unavailable, and stays silent when IPsec is not configured or XFRM is up.
// PREVENTS: an operator believing an interface is IPsec-protected when no SA can install.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func ipsecDiagCodes(cfg ospfConfig, xfrmOK bool) []string {
	diags := ospfIPsecDiagnostics(cfg, xfrmOK)
	out := make([]string, 0, len(diags))
	for i := range diags {
		out = append(out, diags[i].Code)
	}
	return out
}

func TestOSPFIPsecDoctorWarnsWhenXFRMUnavailable(t *testing.T) {
	withIPsec := ospfConfig{present: true, V6: &ospfConfig{
		present: true,
		Interfaces: []interfaceConfig{{
			Name:  "eth1",
			IPsec: &ipsecInterfaceConfig{SPI: 256, Protocol: "esp", AuthAlgo: "sha256"},
		}},
	}}

	// IPsec configured + XFRM unavailable -> warn.
	assert.Contains(t, ipsecDiagCodes(withIPsec, false), codeOSPFv3IPsec)

	// IPsec configured + XFRM available -> silent.
	assert.Empty(t, ipsecDiagCodes(withIPsec, true))

	// v6 family with no IPsec block -> silent even when XFRM is down.
	noIPsec := ospfConfig{present: true, V6: &ospfConfig{
		present:    true,
		Interfaces: []interfaceConfig{{Name: "eth1"}},
	}}
	assert.Empty(t, ipsecDiagCodes(noIPsec, false))

	// No v6 family at all -> silent.
	assert.Empty(t, ipsecDiagCodes(ospfConfig{present: true}, false))
}
