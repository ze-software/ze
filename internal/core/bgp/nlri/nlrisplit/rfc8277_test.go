package nlrisplit

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLabeledRsrvIgnoredOnReceive pins RFC 8277 Section 2.2/2.3: the three
// Rsrv bits between the 20-bit label value and the S bit carry no meaning for
// the receiver. Both label-stack readers (SplitLabeled for framing,
// ExtractLabels for the value) consult only bit 0 (S) and the upper 20 bits,
// so a sender that leaves Rsrv non-zero must still have its label decoded to
// the same value and its NLRI framed identically.
//
// VALIDATES: ExtractLabels / SplitLabeled ignore the Rsrv bits on reception.
// PREVENTS: an RFC 3107-era sender that puts EXP bits in the Rsrv nibble
// having its labeled routes rejected or decoded with a corrupted label value.
//
// RFC requirement: RFC8277-2.2-3 positive -- Rsrv=0 decodes label 100 and frames the NLRI
// RFC requirement: RFC8277-2.2-3 negative -- Rsrv=0b111 (non-conformant on transmission) is ignored rather than rejected: the same label 100 and the same framing come back.
func TestLabeledRsrvIgnoredOnReceive(t *testing.T) {
	t.Parallel()

	// 10.0.0.0/8 with label 100: totalBits = 24 (label) + 8 (prefix) = 32.
	// label 100 -> 0x00 0x06 0x4_, low nibble = Rsrv(3 bits) + S(1 bit).
	conformant := []byte{32, 0x00, 0x06, 0x41, 10}  // Rsrv = 000, S = 1
	rsrvNonZero := []byte{32, 0x00, 0x06, 0x4F, 10} // Rsrv = 111, S = 1

	// Positive: the conformant encoding decodes to label 100 over 10.0.0.0/8.
	labels, cidr, err := ExtractLabels(conformant, false)
	require.NoError(t, err)
	assert.Equal(t, []uint32{100}, labels)
	assert.Equal(t, []byte{8, 10}, cidr, "CIDR bytes are [prefix-bits][prefix]")

	framed, err := splitAll(t, SplitLabeled, conformant, false)
	require.NoError(t, err)
	require.Len(t, framed, 1)
	assert.Equal(t, conformant, framed[0])

	// Negative: the same NLRI with every Rsrv bit set is NOT rejected and does
	// not shift the decoded label or the NLRI boundary.
	labelsR, cidrR, err := ExtractLabels(rsrvNonZero, false)
	require.NoError(t, err, "non-zero Rsrv must be ignored, never rejected")
	assert.Equal(t, labels, labelsR, "Rsrv bits must not alter the decoded label value")
	assert.Equal(t, cidr, cidrR, "Rsrv bits must not alter the decoded prefix")

	framedR, err := splitAll(t, SplitLabeled, rsrvNonZero, false)
	require.NoError(t, err)
	require.Len(t, framedR, 1)
	assert.Equal(t, rsrvNonZero, framedR[0], "Rsrv bits must not alter NLRI framing")
}

// TestLabeledWithdrawalFramingSkipsCompatibilityField reads one RFC 8277
// Section 2.4 withdrawal, [Length][Compatibility(3)][Prefix], with both readers.
// Compatibility is RECOMMENDED to be 0x800000, whose S bit is clear, and
// "Upon reception, the value of the Compatibility field MUST be ignored."
//
// VALIDATES: the announcement readers (ExtractLabels, SplitLabeled) walk the
// three octets as a label stack entry and report a truncated stack, which is
// right for an announcement and is why a withdrawal MUST NOT be read with them;
// the withdrawal readers (SplitWithdrawn, RouteCIDR with withdraw set) frame
// the one NLRI and name 10.0.0.0/8 whatever the field holds.
// PREVENTS: a withdrawal caller reaching for the announcement framing (the
// MPUnreachWire.NLRIs defect), or the withdrawal framing starting to depend on
// the field's value.
func TestLabeledWithdrawalFramingSkipsCompatibilityField(t *testing.T) {
	t.Parallel()

	labeled := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	// Withdrawal of 10.0.0.0/8: Length = 24 + 8 = 32, Compatibility = 0x800000.
	withdrawal := []byte{32, 0x80, 0x00, 0x00, 10}

	_, _, err := ExtractLabels(withdrawal, false)
	assert.ErrorIs(t, err, errNlrisplitTruncatedLabelStack,
		"the announcement reader walks the Compatibility field as a label stack entry")
	_, splitErr := splitAll(t, SplitLabeled, withdrawal, false)
	assert.Error(t, splitErr, "the announcement framing depends on the field's S bit")

	for _, compat := range [][3]byte{{0x80, 0x00, 0x00}, {0x00, 0x00, 0x00}, {0x00, 0x06, 0x41}} {
		nlri := []byte{32, compat[0], compat[1], compat[2], 10}
		parts, err := SplitWithdrawn(labeled, nlri, false)
		require.NoError(t, err, "Compatibility %x", compat)
		require.Len(t, parts, 1, "Compatibility %x", compat)
		assert.Equal(t, nlri, parts[0], "Compatibility %x", compat)

		var scratch [PrefixKeyScratchSize]byte
		cidr, err := RouteCIDR(labeled, nlri, scratch[:], true)
		require.NoError(t, err, "Compatibility %x", compat)
		assert.Equal(t, []byte{8, 10}, cidr, "Compatibility %x: the route is 10.0.0.0/8", compat)
	}
}

// TestLabeledSingleLabelSBitClearGap documents the RFC 8277 Section 2.2
// receive rule as ze implements it today: in the single-label encoding the S
// bit "MUST be ignored on reception", so an NLRI whose one label entry leaves
// S clear (an RFC 3107-era sender) still carries exactly one label. ze's
// readers are S-driven and keep consuming 3-octet entries, running past the
// prefix.
//
// VALIDATES: the exact observable behavior behind the RFC8277-2.2-2 gap.
// PREVENTS: the gap being closed silently, or being mis-recorded as closed.
func TestLabeledSingleLabelSBitClearGap(t *testing.T) {
	t.Parallel()

	// 10.0.0.0/8, label 100, S = 0. The Length octet still says 24 + 8 bits,
	// so a single label is unambiguous from the Length field alone.
	sClear := []byte{32, 0x00, 0x06, 0x40, 10}

	_, _, err := ExtractLabels(sClear, false)
	assert.ErrorIs(t, err, errNlrisplitTruncatedLabelStack,
		"gap RFC8277-2.2-2: the S bit drives the label count instead of being ignored")
}
