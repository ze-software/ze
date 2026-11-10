package peer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/decode"
)

// keepaliveHex is a whole KEEPALIVE: the marker, length 19, type 4.
const keepaliveHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304"

// TestMismatchDiffReportsAMissingPartNotInvalidHex holds the report a decode
// test prints when a `contains=` or `prefix=` expectation is not met.
//
// GOAL: the author reads that the expected message did not arrive, with the
// bytes the line asked for and the message that came instead, never a decode
// error over a test line that is correct.
// METHOD: parse each .ci line through the checker's own parser, render the
// mismatch against a KEEPALIVE, and require the whole report.
//
// VALIDATES: AC-14 of spec-operator-errors-name-the-fix (journal
// earlier-guard-hides-the-better-error, 2026-10-18 row).
// PREVENTS: an unmet needle decoded as a whole message, which printed
// `invalid hex: ... U+004F 'O'` and sent the author to a line with no fault.
func TestMismatchDiffReportsAMissingPartNotInvalidHex(t *testing.T) {
	cases := []struct {
		rule string
		want string
	}{
		{
			"expect=bgp:conn=1:seq=1:contains=00:11:aa:bb",
			"\n--- Expected vs Received ---\n" +
				"Expected: a message matching contains=0011AABB\n" +
				"\n" +
				"Received: KEEPALIVE (len=19)\n" +
				"\n" +
				"Missing: the received message does not match contains=0011AABB, so the expected message has not arrived\n",
		},
		{
			"expect=bgp:conn=1:seq=1:prefix=FFFF0000",
			"\n--- Expected vs Received ---\n" +
				"Expected: a message matching prefix=FFFF0000\n" +
				"\n" +
				"Received: KEEPALIVE (len=19)\n" +
				"\n" +
				"Missing: the received message does not match prefix=FFFF0000, so the expected message has not arrived\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			_, _, expected, err := parseExpectRule(tc.rule)
			require.NoError(t, err)
			require.Equal(t, tc.want, mismatchDiff(expected, keepaliveHex))
		})
	}
}

// TestMismatchDiffDecodesAWholeMessage holds the other half of the routing: a
// `hex=` expectation is a whole message, so it is still decoded beside the one
// received and its attributes compared.
//
// VALIDATES: a hex expectation keeps decode.Diff's report.
// PREVENTS: the needle route swallowing whole-message expectations.
func TestMismatchDiffDecodesAWholeMessage(t *testing.T) {
	const update = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0017020000000000"
	_, _, expected, err := parseExpectRule("expect=bgp:conn=1:seq=1:hex=" + update)
	require.NoError(t, err)
	require.Equal(t, decode.Diff(update, keepaliveHex), mismatchDiff(expected, keepaliveHex))
}
