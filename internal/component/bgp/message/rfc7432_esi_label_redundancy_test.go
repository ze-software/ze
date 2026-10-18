// Design: docs/architecture/wire/nlri-evpn.md -- locally originated EVPN validation
// RFC: rfc/short/rfc7432.md

package message

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// rfc7432RedundancyCommunities is a Route Target followed by the ESI Label
// community given as 16 hex digits: 06 01 <flags> 00 00 <ESI label:3>.
func rfc7432RedundancyCommunities(t *testing.T, esiLabel string) []byte {
	t.Helper()
	label, err := hex.DecodeString(esiLabel)
	require.NoError(t, err)
	require.Len(t, label, 8)
	return append(append([]byte{}, rfc7432RT...), label...)
}

// VALIDATES: a per-ES Ethernet A-D route whose ESI Label signals the requested
// redundancy mode is accepted: All-Active with the Single-Active flag at 0 and
// a label from 16 to 1048575, Single-Active with the flag at 1, and a route
// with no requested mode whose flag states All-Active over a valid label.
// PREVENTS: the redundancy check refusing a conforming ESI Label.
func TestRFC7432ESILabelRedundancyAccepted(t *testing.T) {
	cases := []struct {
		name     string
		mode     EVPNRedundancyMode
		esiLabel string
	}{
		// RFC requirement: RFC7432-8.2.1-4 positive -- All-Active with flag 0 and labels 16 and 1048575 (06 01 00 0000 000100 / fffff0) is accepted.
		{"all-active lowest label", EVPNRedundancyAllActive, "0601000000000100"},
		{"all-active highest label", EVPNRedundancyAllActive, "0601000000fffff0"},
		{"flag states all-active", EVPNRedundancyUnspecified, "0601000000001000"},
		// RFC requirement: RFC7432-8.2.1-7 positive -- Single-Active with flag 1 (06 01 01 ...) is accepted, whatever its label.
		{"single-active", EVPNRedundancySingleActive, "0601010000001000"},
		{"single-active label zero", EVPNRedundancySingleActive, "0601010000000000"},
		{"flag states single-active", EVPNRedundancyUnspecified, "0601010000000000"},
	}
	for _, tc := range cases {
		err := ValidateEVPNOriginationMode(rfc7432PerESNLRI(), rfc7432RedundancyCommunities(t, tc.esiLabel), false, tc.mode)
		require.NoError(t, err, tc.name)
	}
}

// VALIDATES: a per-ES Ethernet A-D route whose ESI Label contradicts the
// requested mode, or that signals All-Active over a reserved label (0 to 15,
// RFC 3032 Section 6), is refused with the community and its correction named.
// PREVENTS: originating an ESI Label that signals the wrong redundancy mode,
// or an All-Active ESI label that is not a valid MPLS label.
func TestRFC7432ESILabelRedundancyRefused(t *testing.T) {
	cases := []struct {
		name     string
		mode     EVPNRedundancyMode
		esiLabel string
		message  string
	}{
		// RFC requirement: RFC7432-8.2.1-4 negative -- All-Active refuses a set Single-Active flag, and a reserved ESI label (0, 15, or 3 with no mode stated) under flag 0.
		{"all-active flag set", EVPNRedundancyAllActive, "0601010000001000",
			"redundancy-mode all-active needs the ESI Label Single-Active flag at 0, but extended community 0x0601010000001000 sets it to 1 (RFC 7432 Section 8.2.1); use extended community 0x0601000000001000, or set redundancy-mode single-active"},
		{"all-active label zero", EVPNRedundancyAllActive, "0601000000000000",
			"All-Active mode needs a valid MPLS label in the ESI Label, but extended community 0x0601000000000000 carries label 0, a reserved value from 0 to 15 (RFC 7432 Section 8.2.1); put a label from 16 to 1048575 in the high-order 20 bits of its last three octets, for example extended community 0x0601000000000100 for label 16"},
		{"all-active label fifteen", EVPNRedundancyAllActive, "06010000000000f0", "carries label 15, a reserved value"},
		{"flag states all-active over label three", EVPNRedundancyUnspecified, "0601000000000030", "carries label 3, a reserved value"},
		// RFC requirement: RFC7432-8.2.1-7 negative -- Single-Active refuses an ESI Label whose Single-Active flag is 0.
		{"single-active flag clear", EVPNRedundancySingleActive, "0601000000001000",
			"redundancy-mode single-active needs the ESI Label Single-Active flag at 1, but extended community 0x0601000000001000 sets it to 0 (RFC 7432 Section 8.2.1); use extended community 0x0601010000001000, or set redundancy-mode all-active"},
	}
	for _, tc := range cases {
		err := ValidateEVPNOriginationMode(rfc7432PerESNLRI(), rfc7432RedundancyCommunities(t, tc.esiLabel), false, tc.mode)
		require.ErrorIs(t, err, ErrEVPNOrigination, tc.name)
		require.ErrorContains(t, err, tc.message, tc.name)
	}
}
