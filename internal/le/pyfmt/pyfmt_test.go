package pyfmt

import (
	"os/exec"
	"strings"
	"testing"
)

// pythonRepr asks CPython to render a value.
// The cases therefore compare against the implementation they reproduce, not a second interpretation of its rules.
func pythonRepr(t *testing.T, expression string) string {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not on PATH, so the reference cannot be consulted")
	}
	//nolint:gosec,noctx // a fixed expression printed by the interpreter; no timeout is needed for a one-line program
	out, err := exec.CommandContext(t.Context(), "python3", "-c", "import sys; sys.stdout.write(repr("+expression+"))").Output()
	if err != nil {
		t.Fatalf("asking python for repr(%s): %v", expression, err)
	}
	return string(out)
}

// VALIDATES: Repr picks the quote the way Python picks it and escapes what
// Python escapes, checked against CPython itself.
// PREVENTS: a rendering that always uses one quote, which changes the message
// for any value holding an apostrophe.
func TestReprAgreesWithPython(t *testing.T) {
	cases := []struct{ value, expression string }{
		{"plain", `"plain"`},
		{"internal/component/bgp", `"internal/component/bgp"`},
		{"has'quote", `"has'quote"`},
		{`has"double`, `'has"double'`},
		{`has'both"quotes`, `"has'both\"quotes"`},
		{`back\slash`, `"back\\slash"`},
		{"", `""`},
	}
	for _, testCase := range cases {
		want := pythonRepr(t, testCase.expression)
		if got := Repr(testCase.value); got != want {
			t.Errorf("Repr(%q) = %s, python says %s", testCase.value, got, want)
		}
	}
}

// VALIDATES: List renders the brackets, the separator and each item the way
// Python renders a list of str.
// PREVENTS: a JSON-shaped rendering, which would put double quotes into a
// message the script writes with single ones.
func TestListAgreesWithPython(t *testing.T) {
	cases := []struct {
		items      []string
		expression string
	}{
		{nil, `[]`},
		{[]string{"repo root"}, `["repo root"]`},
		{[]string{"a", "b", "c"}, `["a", "b", "c"]`},
		{[]string{"hand-fixable", "generator-fixable", "needs-design"}, `["hand-fixable", "generator-fixable", "needs-design"]`},
	}
	for _, testCase := range cases {
		want := pythonRepr(t, testCase.expression)
		if got := List(testCase.items); got != want {
			t.Errorf("List(%q) = %s, python says %s", testCase.items, got, want)
		}
	}
}

// VALIDATES: Escapes names the values whose rendering Repr does not reproduce.
// PREVENTS: a caller believing Repr covers every string, which would leave a
// control character rendered as itself where Python writes \x01.
func TestEscapesNamesWhatReprDoesNotReproduce(t *testing.T) {
	for _, value := range []string{"plain", "with space", "dash-and_underscore"} {
		if Escapes(value) {
			t.Errorf("Escapes(%q) is true, and Repr does render it", value)
		}
	}
	for _, value := range []string{"tab\there", "newline\n", "\x01", "café"} {
		if !Escapes(value) {
			t.Errorf("Escapes(%q) is false, but python would escape it", value)
		}
	}

	// The claim above is checked against python for one of them, so this is
	// not two readings of the same rule.
	if want := pythonRepr(t, `"tab\there"`); !strings.Contains(want, `\t`) {
		t.Fatalf("python renders a tab as %s, so the premise of Escapes is wrong", want)
	}
}
