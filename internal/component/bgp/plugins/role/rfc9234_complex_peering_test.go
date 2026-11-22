// Design: docs/guide/bgp-role.md -- Complex peering relationship prohibition
// RFC: rfc/short/rfc9234.md -- Section 6, roles must not be configured on Complex peering
// Related: role.go -- GetYANG
// Related: config.go -- parseRoleContainer, extractRoleCapabilities

package role

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC9234ComplexPeeringGuidanceAndRolelessSession verifies that operator
// documentation (both the embedded YANG schema description served by GetYANG
// and docs/guide/bgp-role.md) carries the RFC 9234 Section 6 prohibition
// against configuring BGP Roles on a Complex peering relationship, and that
// leaving a Complex eBGP session without a role container emits no Role
// capability while rejecting an invalid "complex" role token.
//
// RFC 9234 Section 6: "Roles MUST NOT be configured on an eBGP session with a
// Complex peering relationship."
//
// VALIDATES: GetYANG() and docs/guide/bgp-role.md state the Complex peering
// prohibition; parseRoleContainer accepts an omitted role block (nil) and
// refuses "complex"; extractRoleCapabilities emits no Role capability for a
// roleless peer.
// PREVENTS: shipping role configuration without documenting the Section 6
// Complex peering prohibition, or accepting a bogus role value.
//
// RFC requirement: RFC9234-6-1 positive -- both the YANG schema description
// returned by GetYANG and docs/guide/bgp-role.md state that roles MUST NOT be
// configured on an eBGP session with a Complex peering relationship, and a
// roleless eBGP peer emits no Role capability.
// RFC requirement: RFC9234-6-1 negative -- parseRoleContainer rejects a
// "complex" role token (returning nil rather than a valid peerRoleConfig), and
// omitting the role container produces zero Role capabilities while a valid
// normal role produces one.
func TestRFC9234ComplexPeeringGuidanceAndRolelessSession(t *testing.T) {
	t.Parallel()

	yangText := GetYANG()
	require.Contains(t, yangText, "Complex peering relationship",
		"YANG role container description must document RFC 9234 Section 6 Complex peering prohibition")
	require.Contains(t, yangText, "MUST NOT",
		"YANG role container description must state the MUST NOT obligation")

	guidePath := filepath.Join("..", "..", "..", "..", "..", "docs", "guide", "bgp-role.md")
	guideBytes, err := os.ReadFile(guidePath)
	require.NoError(t, err, "operator guide docs/guide/bgp-role.md must be readable")
	guideText := string(guideBytes)
	require.Contains(t, guideText, "Roles MUST NOT be configured on an eBGP session with a Complex peering relationship",
		"docs/guide/bgp-role.md must quote the RFC 9234 Section 6 prohibition")

	// A Complex peering session omits the role container: parseRoleContainer returns nil
	// and extractRoleCapabilities emits no Role capability for it.
	require.Nil(t, parseRoleContainer(map[string]any{}))
	require.Nil(t, parseRoleContainer(map[string]any{"import": "complex"}))

	complexCfg := `{"peer":{"complex-peer":{"connection":{"remote":{"ip":"192.0.2.10"}}}}}`
	require.Empty(t, extractRoleCapabilities(complexCfg),
		"a Complex eBGP peer without a role block must advertise no Role capability")

	normalCfg := `{"peer":{"normal-peer":{"connection":{"remote":{"ip":"192.0.2.11"}},"role":{"import":"provider"}}}}`
	caps := extractRoleCapabilities(normalCfg)
	require.Len(t, caps, 1, "a normal eBGP peer with a configured role advertises its Role capability")
	require.True(t, strings.Contains(yangText, "segregate"),
		"YANG description notes segregating a Complex relationship into separate sessions")
}
