// Design: docs/architecture/wire/capabilities.md -- OPEN Optional Parameter parsing
// RFC: rfc/short/rfc9072.md -- Section 3, type code 255 in non-indicator position
// Related: optional_params.go -- ParseFromOptionalParams

package capability

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC9072Type255NonIndicatorRejected verifies that Optional Parameter type
// code 255 encountered anywhere other than the Non-Ext OP Type indicator
// position is treated as an unrecognized Optional Parameter
// (ErrUnsupportedParameter, mapped to NOTIFICATION 2/4 Unsupported Optional
// Parameters by Session.rejectOpenCapabilityError).
//
// RFC 9072 Section 3: "Although the Optional Parameter type code 255 is used
// in this specification as the indication that the extended encoding is in use,
// it is not a bona fide Optional Parameter type code in the usual sense and
// MUST NOT be used other than as described above.  If encountered other than as
// the Non-Ext OP Type, it MUST be treated as an unrecognized Optional Parameter
// and handled according to [RFC4271], Section 6.2."
//
// VALIDATES: type code 255 inside an extended Optional Parameters list (first
// or subsequent parameter) and as a subsequent parameter in a classic Optional
// Parameters list returns ErrUnsupportedParameter; valid type-2 Capabilities
// parameters succeed in both framings.
// PREVENTS: silently skipping unrecognized Optional Parameter type 255 instead
// of rejecting the OPEN with RFC 4271 Section 6.2 Unsupported Optional Parameters.
//
// RFC requirement: RFC9072-3-3 positive -- Optional Parameter type code 255
// encountered inside an extended OPEN (first or second parameter) or as a
// subsequent parameter in a classic OPEN returns ErrUnsupportedParameter.
// RFC requirement: RFC9072-3-3 negative -- well-formed type-2 Capabilities
// Optional Parameters in both extended and classic framing succeed without
// ErrUnsupportedParameter.
func TestRFC9072Type255NonIndicatorRejected(t *testing.T) {
	t.Parallel()

	// Extended OPEN whose first Optional Parameter after the indicator is type 255.
	extFirst255 := []byte{0xFF, 0x00, 0x02, 0x01, 0x02}
	_, err := ParseFromOptionalParams(extFirst255, true)
	require.ErrorIs(t, err, ErrUnsupportedParameter)

	// Extended OPEN with a valid type-2 parameter followed by type 255.
	extSecond255 := []byte{
		0x02, 0x00, 0x02, 0x02, 0x00, // valid Type 2 (Route Refresh)
		0xFF, 0x00, 0x00, // non-indicator Type 255
	}
	_, err = ParseFromOptionalParams(extSecond255, true)
	require.ErrorIs(t, err, ErrUnsupportedParameter)

	// Classic OPEN with a valid type-2 parameter followed by type 255.
	classicSecond255 := []byte{
		0x02, 0x02, 0x02, 0x00, // valid Type 2 (Route Refresh)
		0xFF, 0x00, // non-indicator Type 255
	}
	_, err = ParseFromOptionalParams(classicSecond255, false)
	require.ErrorIs(t, err, ErrUnsupportedParameter)

	// Negative controls: valid type-2 Capabilities parameters succeed in both framings.
	extValid := []byte{0x02, 0x00, 0x02, 0x02, 0x00}
	caps, err := ParseFromOptionalParams(extValid, true)
	require.NoError(t, err)
	require.Len(t, caps, 1)

	classicValid := []byte{0x02, 0x02, 0x02, 0x00}
	caps, err = ParseFromOptionalParams(classicValid, false)
	require.NoError(t, err)
	require.Len(t, caps, 1)
}
