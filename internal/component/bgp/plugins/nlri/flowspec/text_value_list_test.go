// Design: docs/architecture/api/architecture.md — FlowSpec NLRI update text grammar
// RFC: rfc/short/rfc8955.md — numeric operator (Section 4.2.1.1)

package flowspec

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
)

// TestTextNumericValuesAreORUnlessJoinedByAnd proves the update text grammar
// reads successive numeric values under one keyword as alternatives, and joins
// terms with "&" only where the operator writes one.
//
// VALIDATES: `destination-port =80 =443` encodes two terms with the AND bit
// clear (80 OR 443); `>8080&<8088 =3128` and `>8080 &<8088 =3128` both encode
// (>8080 AND <8088) OR =3128, with the AND bit set on the second term only.
// This is the spelling of ExaBGP 6.0 (configuration/grammar/types/flow.py,
// _bracketed and _expression), of Ze's config, and of Ze's own bitmask terms,
// so a list the ExaBGP bridge passes through keeps its meaning.
// PREVENTS: an operator's or an ExaBGP script's list of alternatives being sent
// as an AND that no packet can match, or refused as unmatchable.
func TestTextNumericValuesAreORUnlessJoinedByAnd(t *testing.T) {
	t.Parallel()
	// The component octets after the NLRI length. Each operator octet is
	// e(0x80) a(0x40) len(0x30) 0 lt(0x04) gt(0x02) eq(0x01), RFC 8955 4.2.1.1.
	for _, tc := range []struct {
		text string
		want string
	}{
		// 0x01 =80 (len 1, AND clear), 0x91 =443 (end, len 2, AND clear).
		{"destination-port =80 =443", "05" + "01" + "50" + "91" + "01bb"},
		// 0x12 >8080, 0x54 AND <8088, 0x91 end =3128 with the AND bit clear.
		{"destination-port >8080&<8088 =3128", "05" + "12" + "1f90" + "54" + "1f98" + "91" + "0c38"},
		{"destination-port >8080 &<8088 =3128", "05" + "12" + "1f90" + "54" + "1f98" + "91" + "0c38"},
		// 0x13 >=1024, 0xd5 end AND <=65535.
		{"destination-port >=1024&<=65535", "05" + "13" + "0400" + "d5" + "ffff"},
	} {
		got, err := EncodeFlowSpecComponents(IPv4FlowSpec, strings.Fields(tc.text))
		if err != nil {
			t.Errorf("%q refused: %v", tc.text, err)
			continue
		}
		if len(got) < 1 {
			t.Errorf("%q encoded nothing", tc.text)
			continue
		}
		if hexed := hex.EncodeToString(got[1:]); hexed != tc.want {
			t.Errorf("%q encoded %s, want %s", tc.text, hexed, tc.want)
		}
	}
}

// TestTextNumericDanglingAndRefused proves an "&" with no term on one side is
// refused by name rather than read as an OR or dropped.
//
// VALIDATES: a first value carrying a leading "&", and a value ending on "&",
// are refused.
// PREVENTS: a term with nothing to join being encoded with a meaning the
// operator did not write.
func TestTextNumericDanglingAndRefused(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"destination-port &=80",
		"destination-port >80&",
		"destination-port >80&&<90",
	} {
		if _, err := EncodeFlowSpecComponents(IPv4FlowSpec, strings.Fields(text)); err == nil {
			t.Errorf("%q accepted, want a refusal naming the dangling &", text)
		}
	}
}

// TestTextNumericStringRoundTrip proves the text a numeric component renders
// is the text that encodes it again.
//
// VALIDATES: String() of an OR list and of an AND range parses back to the
// same wire octets.
// PREVENTS: `destination-port =80 =443` rendered for 80 OR 443 and read back
// as 80 AND 443.
func TestTextNumericStringRoundTrip(t *testing.T) {
	t.Parallel()
	for _, comp := range []flowspec.FlowComponent{
		flowspec.NewFlowDestPortComponent(80, 443),
		flowspec.NewFlowNumericComponent(flowspec.FlowDestPort, []flowspec.FlowMatch{
			{Op: flowspec.FlowOpGreater | flowspec.FlowOpEqual, Value: 1024},
			{Op: flowspec.FlowOpLess | flowspec.FlowOpEqual, Value: 65535, And: true},
			{Op: flowspec.FlowOpEqual, Value: 80},
		}),
	} {
		fs := flowspec.NewFlowSpec(IPv4FlowSpec)
		if err := fs.AddComponent(comp); err != nil {
			t.Fatal(err)
		}
		text := comp.String()
		got, err := EncodeFlowSpecComponents(IPv4FlowSpec, strings.Fields(text))
		if err != nil {
			t.Errorf("%q refused: %v", text, err)
			continue
		}
		if hex.EncodeToString(got) != hex.EncodeToString(fs.Bytes()) {
			t.Errorf("%q encoded %x, want %x", text, got, fs.Bytes())
		}
	}
}
