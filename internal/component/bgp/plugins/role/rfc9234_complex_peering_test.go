// Design: docs/guide/bgp-role.md -- Complex peering relationship prohibition
// RFC: rfc/short/rfc9234.md -- Section 6, roles must not be configured on Complex peering
// RFC naming: untagged -- checks that the operator documentation states the RFC 9234 Section 6 prohibition; Ze cannot tell a Complex peering from another, so no test here proves the clause (RFC9234-6-1 is a gap).
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

// TestRFC9234ComplexPeeringGuidanceAndRolelessSession checks the operator
// documentation of RFC 9234 Section 6 and the role parser's handling of a peer
// with no role.
//
// RFC 9234 Section 6: "Roles MUST NOT be configured on an eBGP session with a
// Complex peering relationship."
//
// Goal: the embedded YANG description served by GetYANG and
// docs/guide/bgp-role.md tell the operator to omit the role on a Complex
// peering, and a peer configured without a role advertises no Role capability.
// Method: read both texts; parse an empty role container and one holding the
// unknown token "complex"; extract the capabilities of a roleless peer and of a
// peer with a provider role.
//
// This proves the documentation and the roleless case only. Ze does not know
// whether a peering relationship is Complex, so nothing here shows a role on a
// Complex peering refused, and the test carries no RFC requirement tag.
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
