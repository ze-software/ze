// Design: docs/architecture/exabgp-bridge.md -- what the bridge refuses, and what it keeps
// Related: bridge_attribute.go -- parseRouteAttributes, keep-first and the value refusal
// Related: bridge_route_forms.go -- splitPrefix and the mask refusal
// Related: bridge_operational.go -- convertOperational
//
// ExaBGP 6 keeps 5.0's leniency on the API for a repeated attribute and a short
// split (keep the first, send the prefix whole, log it), and refuses a
// value-less attribute, a mask that is no number and `announce operational`.
// These tests drive one translated line each and read what reaches ze, what is
// refused, and what is logged.
package bridge

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureWarnings routes the default slog logger into a buffer for the test,
// and restores the previous logger after it. The bridge logs a route line's
// warnings through the default logger (warnRouteLine), so a test that reads
// them MUST NOT run in parallel with another that does.
func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &logged
}

// commandTexts is the ze command text of a translation, in order.
func commandTexts(translation Translation) []string {
	texts := make([]string, 0, len(translation.Commands))
	for _, command := range translation.Commands {
		texts = append(texts, command.Text)
	}
	return texts
}

// TestRepeatedAttributeKeepsFirstAndWarns is X-26 and X-27 (AC-32).
//
// VALIDATES: `next-hop 1.1.1.1 next-hop 2.2.2.2` sends next-hop 1.1.1.1 only,
// and one warning names the dropped 2.2.2.2; another single-value attribute
// repeated (med) is kept first the same way, in both the route and the
// attributes forms.
// PREVENTS: forwarding both values and letting ze's parser pick the last.
func TestRepeatedAttributeKeepsFirstAndWarns(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		want    string
		dropped string
	}{
		{
			name:    "next-hop twice keeps the first",
			line:    "announce route 10.0.0.0/24 next-hop 1.1.1.1 next-hop 2.2.2.2",
			want:    "send bgp * update text nhop 1.1.1.1 nlri add 10.0.0.0/24",
			dropped: "next-hop 2.2.2.2 is dropped",
		},
		{
			name:    "med twice keeps the first",
			line:    "announce route 10.0.0.0/24 next-hop 1.1.1.1 med 10 med 20",
			want:    "send bgp * update text nhop 1.1.1.1 med 10 nlri add 10.0.0.0/24",
			dropped: "med 20 is dropped",
		},
		{
			name:    "the attributes form keeps the first",
			line:    "announce attributes next-hop 1.1.1.1 next-hop 2.2.2.2 nlri 10.0.0.0/24",
			want:    "send bgp * update text nhop 1.1.1.1 nlri add 10.0.0.0/24",
			dropped: "next-hop 2.2.2.2 is dropped",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logged := captureWarnings(t)
			translation, err := TranslateLine(tc.line)
			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, commandTexts(translation))
			assert.Equal(t, 1, strings.Count(logged.String(), "exabgp-bridge route line"), logged.String())
			assert.Contains(t, logged.String(), tc.dropped)
		})
	}
}

// TestSplitShorterThanPrefixSendsPrefix is X-27 (AC-33).
//
// VALIDATES: `split /23` on a /24 sends the /24 whole and logs one warning
// naming the split; a split longer than an address is still refused.
// PREVENTS: refusing a line ExaBGP's API sends, and a silent whole send.
func TestSplitShorterThanPrefixSendsPrefix(t *testing.T) {
	logged := captureWarnings(t)
	translation, err := TranslateLine("announce route 10.0.0.0/24 next-hop 1.1.1.1 split /23")
	require.NoError(t, err)
	assert.Equal(t, []string{"send bgp * update text nhop 1.1.1.1 nlri add 10.0.0.0/24"}, commandTexts(translation))
	assert.Equal(t, 1, strings.Count(logged.String(), "exabgp-bridge route line"), logged.String())
	assert.Contains(t, logged.String(), "split /23 is shorter than the prefix 10.0.0.0/24")

	_, err = TranslateLine("announce route 10.0.0.0/24 next-hop 1.1.1.1 split /33")
	require.ErrorIs(t, err, errSplitLength)
}

// TestPathInformationWithoutValueRefused is X-28 (AC-34), and settles A-6.
//
// VALIDATES: `path-information` with no value is refused, and the reason names
// the missing path identifier and what to write.
// PREVENTS: 5.0's silent ignore, and a refusal that names no identifier.
func TestPathInformationWithoutValueRefused(t *testing.T) {
	translation, err := TranslateLine("announce route 10.0.0.0/24 next-hop 1.1.1.1 path-information")
	require.ErrorIs(t, err, errMissingAttrValue)
	require.ErrorIs(t, err, ErrLineNotTranslated)
	assert.Empty(t, translation.Commands)
	assert.Contains(t, err.Error(), "path-information is written with no value")
	assert.Contains(t, err.Error(), "write path-information <path identifier, a number or an IPv4 address>")
}

// TestPrefixMaskNotNumberRefused is X-29 (AC-35).
//
// VALIDATES: `10.0.0.0/x` is refused in both the route and the attributes
// forms, and the reason names the mask and the prefix length to write.
// PREVENTS: taking it as the host route 10.0.0.0/32, as 5.0 did.
func TestPrefixMaskNotNumberRefused(t *testing.T) {
	for _, line := range []string{
		"announce route 10.0.0.0/x next-hop 1.1.1.1",
		"announce attributes next-hop 1.1.1.1 nlri 10.0.0.0/x",
	} {
		t.Run(line, func(t *testing.T) {
			translation, err := TranslateLine(line)
			require.ErrorIs(t, err, errMaskNotNumber)
			assert.Empty(t, translation.Commands)
			assert.Contains(t, err.Error(), `has the mask "x"`)
			assert.Contains(t, err.Error(), "10.0.0.0/<length>")
		})
	}
}

// TestOperationalRefusedByKind is X-30 (AC-36) and OD-1.
//
// VALIDATES: an unknown kind and a bare `announce operational` are refused
// naming the command; a known kind is refused naming that Ze does not send
// OPERATIONAL; the API 6 spelling after a selector is refused the same way.
// PREVENTS: a known kind refused as unknown, and an operational line answered done.
func TestOperationalRefusedByKind(t *testing.T) {
	cases := []struct {
		line string
		want error
		name string
	}{
		{line: "announce operational foo afi ipv4 safi unicast", want: errOperationalUnknown, name: "foo"},
		{line: "announce operational", want: errOperationalUnknown, name: "announce operational"},
		{line: "announce operational asm afi ipv4 safi unicast advisory \"hello\"", want: errOperationalNotSent, name: "(asm)"},
		{line: "peer * operational rpcq afi ipv4 safi unicast sequence 1", want: errOperationalNotSent, name: "(rpcq)"},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			translation, err := TranslateLine(tc.line)
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.want), "err = %v, want %v", err, tc.want)
			assert.Empty(t, translation.Commands)
			assert.Contains(t, err.Error(), tc.name)
		})
	}
}
