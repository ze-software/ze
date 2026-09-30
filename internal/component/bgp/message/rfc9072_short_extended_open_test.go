package message

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// rfc9072OpenFixed is Version 4, My AS AS_TRANS, Hold Time 180, BGP Identifier
// 192.0.2.1: the fixed OPEN fields ahead of the Optional Parameters.
var rfc9072OpenFixed = []byte{0x04, 0x5B, 0xA0, 0x00, 0xB4, 0xC0, 0x00, 0x02, 0x01}

// rfc9072FourOctetAS is the Capability 65 value for AS 4200000001, 6 octets
// with its code and one-octet capability length.
var rfc9072FourOctetAS = []byte{0x41, 0x04, 0xFA, 0x56, 0xEA, 0x01}

// TestRFC9072ShortExtendedOpenIsAccepted proves that an OPEN using the RFC 9072
// encoding for 255 or fewer parameter octets is accepted as that encoding, all
// the way to the capability it carries.
//
// VALIDATES: an extended-form OPEN (Non-Ext OP Len 255, Non-Ext OP Type 255,
// Extended Opt. Parm. Length 9) with one Capabilities parameter of two-octet
// Parameter Length decodes with ExtendedParams set, the 9 parameter octets,
// and a 4-octet AS capability of 4200000001; the classic encoding of the same
// capability decodes with ExtendedParams clear to the same capability.
// PREVENTS: a decoder that accepts the short extended OPEN but reports it as
// classic, so the capability parser reads the two-octet Parameter Length as
// one octet and loses or misreads every capability.
//
// RFC requirement: RFC9072-2-2 positive -- the extended-form OPEN with 9 parameter octets is
// accepted: no error, ExtendedParams true, the 9 octets exact, and ASN4 4200000001 parsed.
// RFC requirement: RFC9072-2-2 negative -- reading those octets as classic framing (the
// non-compliant decode) does not yield that capability, and the classic OPEN of the same
// capability decodes with ExtendedParams false, so the extended branch is what accepts it.
func TestRFC9072ShortExtendedOpenIsAccepted(t *testing.T) {
	extendedParams := append([]byte{0x02, 0x00, byte(len(rfc9072FourOctetAS))}, rfc9072FourOctetAS...)
	extended := append(append([]byte{}, rfc9072OpenFixed...), 0xFF, 0xFF, 0x00, byte(len(extendedParams)))
	extended = append(extended, extendedParams...)

	open, err := UnpackOpen(extended)
	require.NoError(t, err)
	require.True(t, open.ExtendedParams, "the short extended OPEN must decode as extended")
	require.Equal(t, extendedParams, open.OptionalParams)
	caps, err := capability.ParseFromOptionalParams(open.OptionalParams, open.ExtendedParams)
	require.NoError(t, err)
	require.Equal(t, []capability.Capability{&capability.ASN4{ASN: 4200000001}}, caps)

	misread, err := capability.ParseFromOptionalParams(open.OptionalParams, false)
	if err == nil {
		require.NotEqual(t, caps, misread, "classic framing must not recover the capability")
	}

	classicParams := append([]byte{0x02, byte(len(rfc9072FourOctetAS))}, rfc9072FourOctetAS...)
	classic := append(append([]byte{}, rfc9072OpenFixed...), byte(len(classicParams)))
	classic = append(classic, classicParams...)
	plain, err := UnpackOpen(classic)
	require.NoError(t, err)
	require.False(t, plain.ExtendedParams)
	plainCaps, err := capability.ParseFromOptionalParams(plain.OptionalParams, plain.ExtendedParams)
	require.NoError(t, err)
	require.Equal(t, caps, plainCaps)
}
