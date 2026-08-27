// VALIDATES: spec-le-is-a-ze-binary AC-11 -- the four Python spellings this
// package keeps answer what Python answers, compared against the interpreter
// rather than against a hand-written expectation.
// PREVENTS: a message that reads right and compares wrong. repr(), rune
// slicing and json.dumps each reach the parity proof through a violation
// message, so a Go-shaped substitution turns a byte comparison into a verdict
// comparison without anything going red.

package rfc

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// reprCases are the strings the two implementations are compared over. Each one
// is a shape the RFC corpus or an artifact actually produces.
var reprCases = []string{
	"plain",
	"",
	"it's got an apostrophe",
	`it has "quotes"`,
	`both ' and "`,
	"a backslash \\ here",
	"a tab\there",
	"a newline\nhere",
	"a carriage\rreturn",
	"a bell\ahere",
	"a section § mark",
	"an em dash — here",
	"a non-breaking space",
	"RFC7296-2.19-2",
	"plan/spec-ipsec-ipcomp.md",
}

// pythonRepr asks the interpreter for repr() of each case and of a few
// non-string values, so the comparison is against Python rather than against a
// belief about Python.
func pythonRepr(t *testing.T, cases []string) []string {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), pythonTimeout)
	defer cancel()

	payload, err := json.Marshal(cases)
	if err != nil {
		t.Fatalf("encoding the cases: %v", err)
	}
	const program = `
import json, sys
print(json.dumps([repr(one) for one in json.loads(sys.argv[1])]))
`
	cmd := exec.CommandContext(ctx, "python3", "-c", program, string(payload)) // #nosec G204 -- a test's own cases
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("asking Python for repr(): %v: %s", err, errOut.String())
	}
	var found []string
	if err := json.Unmarshal(out.Bytes(), &found); err != nil {
		t.Fatalf("the driver answered no JSON: %v: %s", err, out.String())
	}
	return found
}

func TestReprAnswersWhatPythonAnswers(t *testing.T) {
	want := pythonRepr(t, reprCases)
	if len(want) != len(reprCases) {
		t.Fatalf("the driver answered %d reprs for %d cases", len(want), len(reprCases))
	}
	for i, one := range reprCases {
		if got := pyRepr(one); got != want[i] {
			t.Errorf("pyRepr(%q) = %s, Python answers %s", one, got, want[i])
		}
	}
}

func TestReprOfANonStringValueAnswersItsPythonSpelling(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"an absent field", nil, "None"},
		{"a true flag", true, "True"},
		{"a false flag", false, "False"},
		{"an integer", json.Number("42"), "42"},
		{"a float", json.Number("3.0"), "3.0"},
		{"a list of strings", []string{"gap", "not-applicable"}, "['gap', 'not-applicable']"},
		{"an empty list", []any{}, "[]"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			if got := pyRepr(one.value); got != one.want {
				t.Errorf("pyRepr(%v) = %s, want %s", one.value, got, one.want)
			}
		})
	}
}

func TestATypeNameIsThePythonTypeName(t *testing.T) {
	cases := []struct {
		value any
		want  string
	}{
		{nil, "NoneType"}, {true, "bool"}, {"a", "str"},
		{[]any{}, "list"}, {map[string]any{}, "dict"},
		{json.Number("1"), "int"}, {json.Number("1.5"), "float"},
	}
	for _, one := range cases {
		if got := pyTypeName(one.value); got != one.want {
			t.Errorf("pyTypeName(%v) = %q, want %q", one.value, got, one.want)
		}
	}
}

func TestSlicingCountsCharactersAndNotBytes(t *testing.T) {
	// Python's [:n] takes n CHARACTERS. A byte slice cuts a multi-byte rune in
	// half, and the corpus is full of them: an em dash is three bytes and a
	// section mark two.
	text := "§§§§§ five section marks"
	if got := firstRunes(text, 5); got != "§§§§§" {
		t.Errorf("firstRunes(%q, 5) = %q", text, got)
	}
	if got := firstRunes("short", 80); got != "short" {
		t.Errorf("a string shorter than the bound was cut: %q", got)
	}
	if got := firstRunes("—", 1); got != "—" {
		t.Errorf("an em dash was cut in half: %q", got)
	}
}

func TestTheDumpMatchesPythonsJSONShape(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), pythonTimeout)
	defer cancel()

	document := map[string]any{
		"schema-version": 1,
		"enrolled":       171,
		"signed-by-register": map[string]int{
			registerRFC2119: 2, registerProse: 3, registerManualWalk: 1,
		},
		"unsigned": []string{"rfc1071", "draft-ietf-bess-mup-safi"},
		"empty":    []string{},
	}
	const program = `
import json, sys
print(json.dumps(json.loads(sys.argv[1]), indent=2, sort_keys=True))
`
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encoding the document: %v", err)
	}
	cmd := exec.CommandContext(ctx, "python3", "-c", program, string(raw)) // #nosec G204 -- a test's own document
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("asking Python for json.dumps: %v: %s", err, errOut.String())
	}

	want := strings.TrimSuffix(out.String(), "\n")
	if got := pyDump(document); got != want {
		t.Errorf("pyDump answers\n%s\nPython answers\n%s", got, want)
	}
}
