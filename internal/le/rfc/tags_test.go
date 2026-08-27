// VALIDATES: spec-le-is-a-ze-binary AC-5 -- every tag reader is called as a
// function, and the carrier table is answered as data.
// PREVENTS: a reader that invents a tag from a place that is not a comment, and
// one that reports "no tags" for a file it could not read. Both are the same
// failure wearing two faces: a claim about evidence that nobody measured.

package rfc

import (
	"strings"
	"testing"
)

func TestAGoTagIsReadAnywhereInTheFile(t *testing.T) {
	src := "package x\n\nfunc a() {\n\t// RFC requirement: RFC1-2-3 positive -- inline at the case\n}\n"
	tags, err := ScanGoTags(src, "internal/x/x_test.go")
	if err != nil {
		t.Fatalf("ScanGoTags: %v", err)
	}
	if len(tags) != 1 || tags[0].RID != "RFC1-2-3" || tags[0].Polarity != "positive" {
		t.Fatalf("tags: %+v", tags)
	}
	if tags[0].Line != 4 {
		t.Errorf("line = %d, want 4", tags[0].Line)
	}
}

func TestATrailingPeriodOnATagIsPunctuationRatherThanPartOfTheWord(t *testing.T) {
	// godot requires a doc comment's last line to end in a period, so a tag
	// placed last becomes "positive." -- refusing that would make the lint rule
	// and the tag convention contradict each other.
	tags, err := ScanGoTags("// RFC requirement: RFC1-2-3 negative.\n", "x_test.go")
	if err != nil {
		t.Fatalf("ScanGoTags: %v", err)
	}
	if len(tags) != 1 || tags[0].Polarity != "negative" {
		t.Fatalf("tags: %+v", tags)
	}
}

func TestATagWithoutAPolarityIsRefused(t *testing.T) {
	// A negative-only test passes if the code rejects everything and a
	// positive-only one passes if it accepts everything. Only the pair pins
	// behavior to the requirement, so polarity is never inferred.
	_, err := ScanGoTags("// RFC requirement: RFC1-2-3\n", "x_test.go")
	if err == nil || !strings.Contains(err.Error(), "has no polarity") {
		t.Errorf("a tag with no polarity was accepted: %v", err)
	}
}

func TestATagWithAnUnknownPolarityIsRefused(t *testing.T) {
	_, err := ScanGoTags("// RFC requirement: RFC1-2-3 maybe\n", "x_test.go")
	if err == nil || !strings.Contains(err.Error(), "invalid polarity 'maybe'") {
		t.Errorf("an invented polarity was accepted: %v", err)
	}
}

func TestATerminatorBlockBodyIsNotCISyntax(t *testing.T) {
	// A terminator block's body is RAW file content: one .ci in this repository
	// embeds a Python shebang. Scanning those blocks invents phantom tags.
	src := "name=x\ntmpfs=run.sh terminator=EOF\n#!/bin/sh\n# RFC requirement: RFC1-2-3 positive -- a shell comment\nEOF\n" +
		"# RFC requirement: RFC4-5-6 negative -- a real tag\n"
	tags, err := ScanCITags(src, "test/plugin/x.ci")
	if err != nil {
		t.Fatalf("ScanCITags: %v", err)
	}
	if len(tags) != 1 || tags[0].RID != "RFC4-5-6" {
		t.Fatalf("tags: %+v", tags)
	}
}

func TestAPythonTagIsReadOnlyWhereThePythonReaderSaysItIsAComment(t *testing.T) {
	src := `"""A docstring.

# RFC requirement: RFC9-9-9 positive -- inside a docstring
"""

PROMPT = "# RFC requirement: RFC9-9-9 negative -- inside a string"

# RFC requirement: RFC1-2-3 negative -- a real tag
def check():
    pass  # RFC requirement: RFC9-9-9 positive -- trailing, not a line-start tag
`
	tags, err := ScanPythonTags(src, "test/interop/scenarios/x/check.py")
	if err != nil {
		t.Fatalf("ScanPythonTags: %v", err)
	}
	if len(tags) != 1 || tags[0].RID != "RFC1-2-3" {
		t.Fatalf("tags: %+v", tags)
	}
}

func TestAPythonFileNoReaderCanTrustIsRefusedRatherThanReportedEmpty(t *testing.T) {
	cases := []struct{ name, src string }{
		{"an unterminated single-quoted literal", "X = 'open\n"},
		{"an unterminated triple-quoted literal", "X = \"\"\"open\nmore\n"},
		{"a bracket still open at end of file", "X = [1, 2,\n"},
		{"a dedent that matches no enclosing block", "def f():\n    if x:\n        a = 1\n      b = 2\n"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			_, err := ScanPythonTags(one.src, "check.py")
			if err == nil {
				t.Fatalf("%s was reported as carrying no tags", one.name)
			}
			if !strings.Contains(err.Error(), "cannot tokenize as Python") {
				t.Errorf("the refusal does not say why: %v", err)
			}
		})
	}
}

func TestAValidPythonFileIsNotRefused(t *testing.T) {
	// The refusals above must fire on a file Python would refuse and on nothing
	// else. A reader that refused ordinary code would take every scenario
	// check's evidence away.
	src := "def f():\n    s = 'a \\' b'\n    t = \"\"\"multi\nline\"\"\"\n    u = [1,\n         2]\n    return s, t, u\n"
	tags, err := ScanPythonTags(src, "check.py")
	if err != nil {
		t.Fatalf("ordinary Python was refused: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("tags found in a file carrying none: %+v", tags)
	}
}

func TestTheCarrierTableAnswersOneRowPerRunSuite(t *testing.T) {
	tree := checkoutRoot(t)
	carriers, err := Carriers(tree)
	if err != nil {
		t.Fatalf("Carriers: %v", err)
	}

	suites := FunctionalSuites()
	found := map[string]Carrier{}
	for _, one := range carriers {
		found[one.Name] = one
	}
	for _, suite := range suites {
		var name strings.Builder
		name.WriteString("functional-")
		name.WriteString(suite)
		one, held := found[name.String()]
		if !held {
			t.Fatalf("no carrier row for the %s suite", suite)
		}
		if one.Tier != tierVerify {
			t.Errorf("%s: tier = %q, want %q", name.String(), one.Tier, tierVerify)
		}
	}
	if found["functional-unrun"].Tier != tierUnrun {
		t.Errorf("the catch-all row is not unrun: %+v", found["functional-unrun"])
	}
	if found["unit"].Kind != "unit" || found["unit"].Tier != tierVerify {
		t.Errorf("the unit row is %+v", found["unit"])
	}
}

func TestTheFirstMatchingCarrierWinsAndTheIncubatorIsSkipped(t *testing.T) {
	carriers, err := Carriers(checkoutRoot(t))
	if err != nil {
		t.Fatalf("Carriers: %v", err)
	}
	cases := []struct {
		path, want string
		held       bool
	}{
		{path: "internal/x/x_test.go", want: "unit", held: true},
		{path: "internal/rfc/audit_test.go", want: "unit", held: true},
		{path: "internal/le/rfc/audit_test.go", held: false},
		{path: "test/plugin/x.ci", want: "functional-plugin", held: true},
		{path: "test/nosuite/x.ci", want: "functional-unrun", held: true},
		{path: "test/editor/x.et", want: "editor-editor", held: true},
		{path: "test/nosuite/x.et", want: "editor-unrun", held: true},
		{path: "test/exabgp-compat/x.ci", want: "functional-exabgp", held: true},
		{path: "test/interop/scenarios/one/check.py", want: "interop-bgp", held: true},
		{path: "test/stress/scenarios/one/check.py", want: "scenario-check", held: true},
		{path: "test/draft/x.ci", held: false},
		{path: "internal/x/x.go", held: false},
	}
	for _, one := range cases {
		t.Run(one.path, func(t *testing.T) {
			carrier, held := CarrierFor(one.path, carriers)
			if held != one.held {
				t.Fatalf("CarrierFor(%q) held=%v, want %v", one.path, held, one.held)
			}
			if held && carrier.Name != one.want {
				t.Errorf("CarrierFor(%q) = %q, want %q", one.path, carrier.Name, one.want)
			}
		})
	}
}

func TestOnlyAScheduledWorkflowGrantsANightlyTier(t *testing.T) {
	sources := map[string]string{
		"nightly.yml": "on:\n  schedule:\n    - cron: '0 3 * * *'\njobs:\n  a:\n    steps:\n      - run: make ze-interop-test\n",
		"push.yml":    "on:\n  push:\njobs:\n  a:\n    steps:\n      - run: make ze-interop-ipsec-test\n",
	}
	got := ScheduledTargetsFrom(sources)
	if got["ze-interop-test"] != "nightly.yml" {
		t.Errorf("a scheduled target is not credited: %v", got)
	}
	if _, held := got["ze-interop-ipsec-test"]; held {
		t.Errorf("a push-only target was credited as nightly: %v", got)
	}
}

func TestTheFirstScheduledWorkflowNamingATargetIsTheOneRecorded(t *testing.T) {
	// The workflow a target is credited to is named in the refusal an unrun
	// carrier prints and in the ledger legend, so which of two schedules is
	// recorded is an answer rather than an accident. First wins, in the sorted
	// order the map is walked in, because a walk that recorded the last one
	// would answer differently on a filesystem that listed the files
	// differently.
	sources := map[string]string{
		"b-nightly.yml": "on:\n  schedule:\n    - cron: '0 3 * * *'\njobs:\n  a:\n    steps:\n      - run: make ze-interop-test\n",
		"a-nightly.yml": "on:\n  schedule:\n    - cron: '0 4 * * *'\njobs:\n  a:\n    steps:\n      - run: make ze-interop-test\n",
	}
	if got := ScheduledTargetsFrom(sources)["ze-interop-test"]; got != "a-nightly.yml" {
		t.Errorf("the target is credited to %q, want the first workflow in order", got)
	}
}

func TestACommentedOutCommandGrantsNothing(t *testing.T) {
	sources := map[string]string{
		"nightly.yml": "on:\n  schedule:\n    - cron: '0 3 * * *'\njobs:\n  a:\n    steps:\n      # - run: make ze-interop-test\n",
	}
	if got := ScheduledTargetsFrom(sources); len(got) != 0 {
		t.Errorf("a commented-out command granted a tier: %v", got)
	}
}

func TestEveryBareWordAfterMakeIsATarget(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{"two targets on one line", "run: make a b\n", []string{"a", "b"}},
		{"a variable assignment is not a target", "run: make V=1 a\n", []string{"a"}},
		{"a flag with a separate argument", "run: make -C dir a\n", []string{"a"}},
		{"a wrapper before make", "run: sudo make a\n", []string{"a"}},
		{"a chain", "run: make a && make b\n", []string{"a", "b"}},
		{"a quoted scalar", "- \"make a\"\n", []string{"a"}},
		{"no make at all", "run: echo a\n", nil},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			got := MakeTargetsIn(one.src)
			if len(got) != len(one.want) {
				t.Fatalf("MakeTargetsIn(%q) = %v, want %v", one.src, got, one.want)
			}
			for i := range got {
				if got[i] != one.want[i] {
					t.Fatalf("MakeTargetsIn(%q) = %v, want %v", one.src, got, one.want)
				}
			}
		})
	}
}

func TestAWorkflowDirectoryTheGateCannotReadIsRefused(t *testing.T) {
	// Not knowing what CI runs is a different fact from CI running nothing, and
	// answering "everything runs" there would credit evidence to a pipeline
	// nobody confirmed.
	_, err := ScheduledWorkflowTargets(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "cannot read the workflow directory") {
		t.Errorf("an absent workflow directory was read as empty: %v", err)
	}
}
