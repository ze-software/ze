// Design: docs/architecture/wire/nlri-bgpls.md -- native origination contract
// RFC: rfc/short/rfc9514.md
// Related: export_rfc9514_test.go -- nativeSRv6Commands
//
// VALIDATES: the originate clause of the RFC 9514 sentences that state both
// halves, "MUST be set to 0 when originated and ignored on receipt", for the
// SRv6 Capabilities TLV 1038 and the SRv6 Locator TLV 1162 on the UPDATE the
// exporter emits. The receipt half is proven on the consumer (nlri/ls
// reserved_rfc9514_test.go).
// PREVENTS: a Reserved word reaching a collector non-zero.
package ls_export

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// requireOriginatedSRv6ReservedZero originates the native SRv6 snapshot whose
// source reserved words are all set to reserved, and checks the Reserved word
// of TLV 1038 (octets 2-3) and of TLV 1162 (octets 2-3) is 0 on the wire while
// the fields around them are unchanged.
func requireOriginatedSRv6ReservedZero(t *testing.T, reserved byte) {
	t.Helper()
	commands := nativeSRv6Commands(t, reserved)
	require.Equal(t, [][]byte{{0x40, 0, 0, 0}}, exportTLVValues(t, exportCommandBytes(t, commands[0], "attr")[11:], 1038))
	require.Equal(t, [][]byte{{0x80, 128, 0, 0, 0, 0, 0, 7, 0xc3, 0x50, 0, 2, 0xaa, 0xbb}},
		exportTLVValues(t, exportCommandBytes(t, commands[1], "attr")[11:], 1162))
}

// TestRFC9514OriginatedReservedWordsZero is the ordinary case: a source whose
// reserved words are already 0.
//
// RFC requirement: RFC9514-3.1-3 positive -- the originated SRv6 Capabilities TLV 1038 carries its flags and a Reserved word of 0 (§3.1).
// RFC requirement: RFC9514-5.1-2 positive -- the originated SRv6 Locator TLV 1162 carries its flags, algorithm and metric with a Reserved word of 0 (§5.1).
func TestRFC9514OriginatedReservedWordsZero(t *testing.T) {
	requireOriginatedSRv6ReservedZero(t, 0)
}

// TestRFC9514PoisonedSourceReservedNeverOriginated sets every source reserved
// octet to 0xff, the input that reaches the wire if the producer copies the
// native value.
//
// RFC requirement: RFC9514-3.1-3 negative -- a source capability whose reserved octets are 0xff still originates TLV 1038 with a Reserved word of 0 (§3.1).
// RFC requirement: RFC9514-5.1-2 negative -- a source locator whose reserved octets are 0xff still originates TLV 1162 with a Reserved word of 0 and its other fields intact (§5.1).
func TestRFC9514PoisonedSourceReservedNeverOriginated(t *testing.T) {
	requireOriginatedSRv6ReservedZero(t, 0xff)
}
