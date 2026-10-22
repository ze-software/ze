// Design: docs/architecture/exabgp-bridge.md -- the route attributes the bridge translates
// Overview: bridge_attribute.go -- parseRouteAttributes, the parser under test

package bridge

import (
	"strings"
	"testing"
)

// TestAggregatorInBracketsTranslated verifies the bracketed AGGREGATOR forms.
//
// VALIDATES: ExaBGP reads `<asn>:<router-id>` "in brackets or not: `( 65000:10.0.0.1 )`,
// `(65000:10.0.0.1)`" (AggregatorType, configuration/grammar/types/bgp.py at
// f9bfb853b), and 4.2 and 5.0 accepted `aggregator ( 18144:219.118.225.189 )`
// and its siblings in 53 recorded commands in 5.0 and 19 in 4.2
// (testdata/exabgp-old/commands-5.0.json.gz, commands-4.2.json.gz). Each form
// becomes ze's `aggregator <asn>:<address>`.
// PREVENTS: the bridge reading `(` as the value and refusing the address that
// follows as an unknown attribute.
func TestAggregatorInBracketsTranslated(t *testing.T) {
	for _, line := range []string{
		"announce route 10.0.0.0/24 next-hop 1.2.3.4 aggregator ( 65001:192.0.2.1 )",
		"announce route 10.0.0.0/24 next-hop 1.2.3.4 aggregator (65001:192.0.2.1)",
		"announce route 10.0.0.0/24 next-hop 1.2.3.4 aggregator (65001:192.0.2.1 )",
		"announce route 10.0.0.0/24 next-hop 1.2.3.4 aggregator ( 65001:192.0.2.1)",
		"announce route 10.0.0.0/24 next-hop 1.2.3.4 aggregator 65001:192.0.2.1",
	} {
		translation, err := TranslateLine(line)
		if err != nil {
			t.Errorf("%q: %v", line, err)
			continue
		}
		if len(translation.Commands) != 1 {
			t.Errorf("%q: %d commands, want 1", line, len(translation.Commands))
			continue
		}
		text := translation.Commands[0].Text
		if !strings.Contains(text, " aggregator 65001:192.0.2.1 ") {
			t.Errorf("%q became %q, want aggregator 65001:192.0.2.1", line, text)
		}
	}
}

// aggregatorRefusal is the refusal every malformed AGGREGATOR value carries.
const aggregatorRefusal = "aggregator takes one <asn>:<router-id>, in brackets or not: " +
	"write aggregator ( 65000:10.0.0.1 )"

// TestAggregatorUnclosedBracketRefused verifies a bracket left open is refused.
//
// VALIDATES: "invalid aggregator - missing closing ')'" (AggregatorType, as above),
// and that the refusal names the value aggregator takes rather than `<value>`.
// PREVENTS: a line whose group never closes reaching a peer, and a refusal
// that does not tell the operator what to write.
func TestAggregatorUnclosedBracketRefused(t *testing.T) {
	_, err := TranslateLine("announce route 10.0.0.0/24 next-hop 1.2.3.4 aggregator ( 65001:192.0.2.1")
	if err == nil {
		t.Fatal("an unclosed aggregator group was translated")
	}
	want := "write aggregator <asn>:<router-id>, in brackets or not"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("refusal %q does not name %q", err, want)
	}
}

// TestAggregatorMalformedGroupRefused verifies that only one bracketed word is read.
//
// VALIDATES: AggregatorType at f9bfb853b takes exactly one `<asn>:<router-id>`
// word, bare or as `(w)`, `( w )`, `(w )` or `( w)`, and refuses anything else.
// PREVENTS: a group holding more than one word becoming extra ze tokens (the
// words after the first would skip the repeated-attribute check), a stray `)`
// being stripped from a bare value, and a doubled bracket being accepted.
func TestAggregatorMalformedGroupRefused(t *testing.T) {
	for _, value := range []string{
		"( 65001:192.0.2.1 community 1:1 )",
		"65001:192.0.2.1)",
		"((65001:192.0.2.1))",
	} {
		line := "announce route 10.0.0.0/24 next-hop 1.2.3.4 aggregator " + value
		translation, err := TranslateLine(line)
		if err == nil {
			t.Errorf("%q was translated to %v", line, translation.Commands)
			continue
		}
		if !strings.Contains(err.Error(), aggregatorRefusal) {
			t.Errorf("%q: refusal %q does not carry %q", line, err, aggregatorRefusal)
		}
	}
}
