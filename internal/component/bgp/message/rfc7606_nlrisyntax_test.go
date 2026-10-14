// RFC naming: untagged -- wiring test for validateMPNLRISyntax reading the per-family NLRI syntax registry; the RFC 7606, RFC 9552 and RFC 9830 verdicts it maps are proven by the tagged receive tests.

package message

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisyntax"
	"github.com/ze-software/ze/internal/core/family"
)

// VALIDATES: validateMPNLRISyntax reads the nlrisyntax registry by family. BGP-LS reaches
// its RFC 9552 Section 8.2.2 rule through the registry with the verdict it had before; a
// family with a registered Judge gets that verdict mapped to treat-as-withdraw or session
// reset with no Notification (so the session sends 3/1); a rule answering no verdict is
// session reset; a family with no rule still returns nil, and IPv4 unicast still takes the
// prefix walk.
// PREVENTS: a hard-coded family branch beside the registry (ai/rules/no-layering.md), and
// a rule's zero verdict read as well-formed.
func TestValidateMPNLRISyntaxConsultsRegistry(t *testing.T) {
	const code = uint8(attribute.AttrMPReachNLRI)

	// BGP-LS, through its real registration: a Total NLRI Length overrunning the section.
	overrun := []byte{0x00, 0x01, 0x00, 0xff, 0xaa}
	r := validateMPNLRISyntax(code, afiBGPLS, safiBGPLS, overrun, false)
	require.NotNil(t, r)
	require.Equal(t, RFC7606ActionSessionReset, r.Action)
	require.Nil(t, r.Notification)
	require.Equal(t, code, r.AttrCode)

	// The same bytes under IPv4 unicast are judged by the prefix walk, never by the BGP-LS
	// rule: 0x00 is a default route, then 0x01 with no octet left overruns.
	r = validateMPNLRISyntax(code, attribute.AFIIPv4, attribute.SAFIUnicast, overrun, false)
	require.NotNil(t, r, "IPv4 unicast must still take the prefix walk")

	t.Cleanup(nlrisyntax.SnapshotForTest())
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	var answer nlrisyntax.Judgement
	require.NoError(t, nlrisyntax.Register(fam, nlrisyntax.Rule{
		Citation: "test",
		Judge:    func([]byte, bool) nlrisyntax.Judgement { return answer },
	}))
	afi, safi := attribute.AFI(fam.AFI), attribute.SAFI(fam.SAFI)

	answer = nlrisyntax.WellFormed
	require.Nil(t, validateMPNLRISyntax(code, afi, safi, []byte{1}, false))

	answer = nlrisyntax.Judgement{Action: nlrisyntax.ActionTreatAsWithdraw, Reason: "bad length"}
	r = validateMPNLRISyntax(code, afi, safi, []byte{1}, false)
	require.NotNil(t, r)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, r.Action)
	require.Equal(t, "bad length", r.Description)
	require.Nil(t, r.Notification)

	answer = nlrisyntax.Judgement{Action: nlrisyntax.ActionSessionReset, Reason: "unwalkable"}
	r = validateMPNLRISyntax(code, afi, safi, []byte{1}, false)
	require.NotNil(t, r)
	require.Equal(t, RFC7606ActionSessionReset, r.Action)
	require.Nil(t, r.Notification)

	answer = nlrisyntax.Judgement{}
	r = validateMPNLRISyntax(code, afi, safi, []byte{1}, false)
	require.NotNil(t, r, "a rule's zero verdict must not pass as well-formed")
	require.Equal(t, RFC7606ActionSessionReset, r.Action)

	// The registry is now empty but for the test rule: BGP-LS has no rule and is permissive.
	require.Nil(t, validateMPNLRISyntax(code, afiBGPLS, safiBGPLS, overrun, false))
}
