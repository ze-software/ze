package filter_path_asn

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestRejectListReadsEveryNotation proves a reject-asn list takes an AS number
// in any of the three RFC 5396 spellings, and stores one number whichever the
// operator wrote. The method parses a list holding the same AS twice, once
// dotted and once decimal, and reads the positions back.
//
// VALIDATES: the leaf-lists are typed zt:asn, and addASNs and addNth read
// asn.Parse.
// PREVENTS: an operator who set `as-notation asdot`, read 1.10 on every show
// output, and then had the filter refuse the AS number they just read. That is
// AC-1 for a leaf the first pass missed.
func TestRejectListReadsEveryNotation(t *testing.T) {
	lists, err := parseLists(t, `        reject-asn DOTTED {
            direct [ 1.10 ]
            origin [ 0.100 ]
            anywhere [ 65546 ]
            nth 2 { asn [ 1.10 ] }
        }`)
	require.NoError(t, err)

	list, ok := lists["DOTTED"]
	require.True(t, ok, "the dotted list must parse")

	// 1.10 and 65546 are one AS number, so `direct` and `anywhere` union onto
	// one key rather than making two.
	require.Contains(t, list.positions, uint32(65546))
	require.Contains(t, list.positions, uint32(100))
	require.Len(t, list.positions, 2)
	require.Contains(t, list.nth, nthKey{index: 2, asn: 65546})
}

// TestRejectListRefusesAMalformedASNumber proves the dotted spellings did not
// widen the leaf into accepting anything with a period in it.
//
// The refusal is asserted at the SCHEMA rather than at the plugin's own parse,
// because that is where it happens now. `zt:asn` reaches asn.Parse through
// ValidateValue. An operator is told which field and which value at load, and
// the plugin never sees the token.
//
// VALIDATES: the leaf-lists are typed zt:asn, so the config parser refuses a
// token that names no AS number.
// PREVENTS: a filter that looks configured and rejects nothing. It accepts
// every route, and it reads like a safety filter.
func TestRejectListRefusesAMalformedASNumber(t *testing.T) {
	for _, value := range []string{"1.99999", "65536.0", "1.2.3", "peer"} {
		_, err := config.ParseTreeWithYANG(policyOnly(`        reject-asn BAD {
            direct [ `+value+` ]
        }`), nil)
		require.Error(t, err, "%q names no AS number", value)
		require.Contains(t, err.Error(), "direct", "the refusal must name the leaf the operator wrote")
	}
}
