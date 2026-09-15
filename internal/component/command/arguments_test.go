package command

import (
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestWriteInvocationPlacesEachValueWhereTheDispatcherBindsIt: the one
// declaration of argument placement the web form and the MCP builder share.
//
// VALIDATES: an anchored value goes bare after its anchor keyword, a declared
// value follows the command as keyword and value in declaration order, an
// undeclared value trails in name order, an empty value is left out, and a
// value holding a space is quoted.
// PREVENTS: an anchored selector written in keyword form, which the dispatcher
// validates and never binds.
func TestWriteInvocationPlacesEachValueWhereTheDispatcherBindsIt(t *testing.T) {
	defs := []ArgDef{
		{Name: "selector", Kind: ArgString, Mandatory: true, Anchor: "peer"},
		{Name: "prefix", Kind: ArgString, Mandatory: true},
		{Name: "label", Kind: ArgString},
	}
	path := []string{"peer", "announce", "unicast"}
	cases := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{name: "no value", values: nil, want: "peer announce unicast"},
		{name: "anchored value bare after its keyword", values: map[string]string{"selector": "192.0.2.9"}, want: "peer 192.0.2.9 announce unicast"},
		{name: "declaration order after the command", values: map[string]string{"label": "edge", "prefix": "198.51.100.0/24"}, want: "peer announce unicast prefix 198.51.100.0/24 label edge"},
		{name: "empty value left out", values: map[string]string{"selector": "", "prefix": "198.51.100.0/24"}, want: "peer announce unicast prefix 198.51.100.0/24"},
		{name: "a value with a space is quoted", values: map[string]string{"label": "edge router"}, want: `peer announce unicast label "edge router"`},
		{name: "undeclared values trail in name order", values: map[string]string{"zeta": "1", "alpha": "2", "prefix": "p"}, want: "peer announce unicast prefix p alpha 2 zeta 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var tb textbuf.Buffer
			if err := WriteInvocation(&tb, path, defs, tc.values); err != nil {
				t.Fatal(err)
			}
			if got := tb.String(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestWriteInvocationRefusesADoubleQuote: the dispatcher's grammar carries no
// escape for a double quote, so a value holding one is refused by name rather
// than sent as tokens nobody typed.
func TestWriteInvocationRefusesADoubleQuote(t *testing.T) {
	var tb textbuf.Buffer
	err := WriteInvocation(&tb, []string{"socket", "open"}, []ArgDef{{Name: "label", Kind: ArgString}}, map[string]string{"label": `a"b`})
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if got := err.Error(); got != "argument label: a value cannot hold a double quote" {
		t.Errorf("error = %q", got)
	}
}
