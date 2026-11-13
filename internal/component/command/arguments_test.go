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
		mustArgDef(NewStringArg("selector", nil, nil, ArgOptions{Mandatory: true, Anchor: "peer"})),
		mustArgDef(NewStringArg("prefix", nil, nil, ArgOptions{Mandatory: true})),
		mustArgDef(NewStringArg("label", nil, nil, ArgOptions{})),
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
			WriteInvocation(&tb, path, InvocationArgs(defs), tc.values)
			if got := tb.String(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestWriteInvocationQuotesLiteralSyntax proves admitted argument bytes are
// encoded as one operational token, including the formerly unrepresentable
// double quote. Actual dispatcher replay is covered in the server package.
func TestWriteInvocationQuotesLiteralSyntax(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{value: `a"b`, want: `socket open label "a\"b"`},
		{value: `a\b`, want: `socket open label "a\\b"`},
		{value: `a|b`, want: `socket open label "a|b"`},
		{value: "a b", want: `socket open label "a b"`},
		{value: "a\nb", want: `socket open label "a\nb"`},
		{value: "a\rb", want: `socket open label "a\rb"`},
		{value: "a\tb", want: `socket open label "a\tb"`},
		{value: "\u00a0a\u00a0", want: "socket open label \"\u00a0a\u00a0\""},
		{value: "it's", want: `socket open label "it's"`},
		{value: "'tis", want: `socket open label "'tis"`},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			var tb textbuf.Buffer
			WriteInvocation(&tb, []string{"socket", "open"}, []InvocationArg{{Name: "label"}}, map[string]string{"label": tc.value})
			if got := tb.String(); got != tc.want {
				t.Errorf("invocation = %q, want %q", got, tc.want)
			}
		})
	}
}
