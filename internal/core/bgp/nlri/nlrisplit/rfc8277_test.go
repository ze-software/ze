package nlrisplit

import (
	"testing"

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

	framed, err := SplitLabeled(conformant, false)
	require.NoError(t, err)
	require.Len(t, framed, 1)
	assert.Equal(t, conformant, framed[0])

	// Negative: the same NLRI with every Rsrv bit set is NOT rejected and does
	// not shift the decoded label or the NLRI boundary.
	labelsR, cidrR, err := ExtractLabels(rsrvNonZero, false)
	require.NoError(t, err, "non-zero Rsrv must be ignored, never rejected")
	assert.Equal(t, labels, labelsR, "Rsrv bits must not alter the decoded label value")
	assert.Equal(t, cidr, cidrR, "Rsrv bits must not alter the decoded prefix")

	framedR, err := SplitLabeled(rsrvNonZero, false)
	require.NoError(t, err)
	require.Len(t, framedR, 1)
	assert.Equal(t, rsrvNonZero, framedR[0], "Rsrv bits must not alter NLRI framing")
}

// TestLabeledCompatibilityFieldWithdrawalGap documents the RFC 8277 Section 2.4
// withdrawal encoding as ze reads it today. Section 2.4 frames a withdrawal as
// [Length][Compatibility(3)][Prefix], with Compatibility RECOMMENDED to be
// 0x800000 and MUST-ignored on reception. ze's readers instead walk those three
// octets as a label stack entry and stop on the S bit, which 0x800000 leaves
// clear, so the reader runs past the end of the NLRI and reports a truncated
// label stack.
//
// VALIDATES: the exact observable behavior behind the RFC8277-2.4-1 gap.
// PREVENTS: the gap being closed silently, or being mis-recorded as closed.
func TestLabeledCompatibilityFieldWithdrawalGap(t *testing.T) {
	t.Parallel()

	// Withdrawal of 10.0.0.0/8: Length = 24 + 8 = 32, Compatibility = 0x800000.
	withdrawal := []byte{32, 0x80, 0x00, 0x00, 10}

	_, _, err := ExtractLabels(withdrawal, false)
	assert.ErrorIs(t, err, errNlrisplitTruncatedLabelStack,
		"gap RFC8277-2.4-1: the Compatibility field is parsed as a label stack entry instead of being ignored")

	_, splitErr := SplitLabeled(withdrawal, false)
	assert.Error(t, splitErr,
		"gap RFC8277-2.4-1: NLRI framing also depends on the Compatibility field's S bit")
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
