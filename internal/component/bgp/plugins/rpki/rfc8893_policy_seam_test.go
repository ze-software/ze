// Design: docs/architecture/rib/unified-locrib.md -- bgp-rpki publishes its ROACache to the shared policy seam
// Related: rpki.go -- validateOriginForPolicy, startSessions

package rpki

import (
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// RFC requirement: RFC8893-1-1 positive -- bgp-rpki publishes its live ROACache
// through validateOriginForPolicy to the shared ribpolicy origin-validator seam,
// enforcing RFC 6811 Section 2 semantics (Valid, Invalid, NotFound, maxLength,
// AS 0 VRP, and OriginNone for AS_SET) when active and failing closed to
// ValidationNotValidated when inactive.
func TestRPKIPluginPublishesOriginValidatorSeam(t *testing.T) {
	t.Cleanup(func() { ribpolicy.SetOriginValidator(nil) })

	rp := &rPKIPlugin{
		cache: newROACache(),
	}
	_, ipnet, err := net.ParseCIDR("198.51.100.0/24")
	require.NoError(t, err)
	rp.cache.Add(VRP{Prefix: *ipnet, MaxLength: 24, ASN: 65001})

	_, as0Net, err := net.ParseCIDR("192.0.2.0/24")
	require.NoError(t, err)
	rp.cache.Add(VRP{Prefix: *as0Net, MaxLength: 24, ASN: 0})

	rp.active.Store(true)
	ribpolicy.SetOriginValidator(rp.validateOriginForPolicy)

	pfx := netip.MustParsePrefix("198.51.100.0/24")
	require.Equal(t, ribpolicy.ValidationValid, ribpolicy.ValidateOrigin(pfx, 65001))
	require.Equal(t, ribpolicy.ValidationInvalid, ribpolicy.ValidateOrigin(pfx, 65099))
	// More-specific prefix exceeding VRP maxLength (25 > 24) is Invalid.
	require.Equal(t, ribpolicy.ValidationInvalid, ribpolicy.ValidateOrigin(netip.MustParsePrefix("198.51.100.0/25"), 65001))
	// AS_SET (OriginNone) covered by a VRP is Invalid; uncovered is NotFound.
	require.Equal(t, ribpolicy.ValidationInvalid, ribpolicy.ValidateOrigin(pfx, ribpolicy.OriginNone))
	require.Equal(t, ribpolicy.ValidationNotFound, ribpolicy.ValidateOrigin(netip.MustParsePrefix("203.0.113.0/24"), ribpolicy.OriginNone))
	// AS 0 VRP rejects any origin AS including 0.
	require.Equal(t, ribpolicy.ValidationInvalid, ribpolicy.ValidateOrigin(netip.MustParsePrefix("192.0.2.0/24"), 65001))
	require.Equal(t, ribpolicy.ValidationInvalid, ribpolicy.ValidateOrigin(netip.MustParsePrefix("192.0.2.0/24"), 0))
	require.Equal(t, ribpolicy.ValidationNotFound, ribpolicy.ValidateOrigin(netip.MustParsePrefix("203.0.113.0/24"), 65001))

	// Inactive plugin stops answering validation states (fail closed to NotValidated).
	rp.active.Store(false)
	require.Equal(t, ribpolicy.ValidationNotValidated, ribpolicy.ValidateOrigin(pfx, 65001))
}
