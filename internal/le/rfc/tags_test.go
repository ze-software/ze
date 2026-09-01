// VALIDATES: spec-le-is-a-ze-binary AC-5 -- every tag reader is called as a
// function, and the carrier table is answered as data.
// PREVENTS: a reader that invents a tag from a place that is not a comment, and
// one that reports "no tags" for a file it could not read. Both are the same
// failure wearing two faces: a claim about evidence that nobody measured.

package rfc

import (
	"slices"
	"strings"
	"testing"
)

func TestAGoTagIsReadAnywhereInTheFile(t *testing.T) {
	src := "package x\n\nfunc a() {\n\t// " + rfcTagMarker + " RFC1-2-3 positive -- inline at the case\n}\n"
	tags, err := scanGoTags(src, "internal/x/x_test.go")
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
	tags, err := scanGoTags("// "+rfcTagMarker+" RFC1-2-3 negative.\n", "x_test.go")
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
	_, err := scanGoTags("// "+rfcTagMarker+" RFC1-2-3\n", "x_test.go")
	if err == nil || !strings.Contains(err.Error(), "has no polarity") {
		t.Errorf("a tag with no polarity was accepted: %v", err)
	}
}

func TestATagWithAnUnknownPolarityIsRefused(t *testing.T) {
	_, err := scanGoTags("// "+rfcTagMarker+" RFC1-2-3 maybe\n", "x_test.go")
	if err == nil || !strings.Contains(err.Error(), "invalid polarity 'maybe'") {
		t.Errorf("an invented polarity was accepted: %v", err)
	}
}

func TestATerminatorBlockBodyIsNotCISyntax(t *testing.T) {
	// A terminator block's body is raw fixture content. Scanning those blocks
	// invents phantom tags.
	src := "name=x\ntmpfs=payload.txt terminator=EOF\n# " + rfcTagMarker + " RFC1-2-3 positive -- payload text\nEOF\n" +
		"# " + rfcTagMarker + " RFC4-5-6 negative -- a real tag\n"
	tags, err := scanCITags(src, "test/plugin/x.ci")
	if err != nil {
		t.Fatalf("ScanCITags: %v", err)
	}
	if len(tags) != 1 || tags[0].RID != "RFC4-5-6" {
		t.Fatalf("tags: %+v", tags)
	}
}

func TestChangedTagsDistinguishesBehaviorFromLayoutAndComments(t *testing.T) {
	const tag = "" + rfcTagMarker + " RFC9999-2-1"
	cases := []struct {
		name, path, oldText, newText string
		changed                      bool
	}{
		{
			name: "behavior", path: "x_test.go",
			oldText: "// " + tag + " positive\nfunc TestOne() { got := 1; _ = got }\n",
			newText: "// " + tag + " positive\nfunc TestOne() { got := 2; _ = got }\n",
			changed: true,
		},
		{
			name: "comment", path: "x_test.go",
			oldText: "// " + tag + " positive\nfunc TestOne() { got := 1; _ = got } // old\n",
			newText: "// " + tag + " positive\nfunc TestOne() { got := 1; _ = got } // clearer\n",
		},
		{
			name: "tag removal", path: "x_test.go",
			oldText: "// " + tag + " positive\nfunc TestOne() {}\n",
			newText: "func TestOne() {}\n", changed: true,
		},
		{
			name: "Go import only", path: "x_test.go",
			oldText: "// " + tag + " positive\nimport \"example/a\"\n",
			newText: "// " + tag + " positive\nimport (\n\t\"example/a\"\n\talias \"example/b\"\n)\n",
		},
		{
			name: "CI comment", path: "x.ci",
			oldText: "# " + tag + " positive\nsend packet\n",
			newText: "# " + tag + " positive\n# clearer reason\nsend packet\n",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			got := ChangedTags(one.path, one.oldText, one.newText)
			if one.changed && (len(got) != 1 || got[0] != tag) {
				t.Errorf("ChangedTags = %v, want [%s]", got, tag)
			}
			if !one.changed && len(got) != 0 {
				t.Errorf("ChangedTags = %v, want no changed tag", got)
			}
		})
	}
}

func TestTheCarrierTableAnswersOneRowPerRunSuite(t *testing.T) {
	tree := checkoutRoot(t)
	carriers, err := carriers(tree)
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
	carriers, err := carriers(checkoutRoot(t))
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
		{path: "internal/le/interoplab/bgp/scenario_test.go", want: "interop-bgp", held: true},
		{path: "internal/le/interoplab/new/scenario_test.go", want: "interop-unrun", held: true},
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
		"nightly.yml": "on:\n  schedule:\n    - cron: '0 3 * * *'\njobs:\n  a:\n    steps:\n      - run: ./le integration interop\n",
		"push.yml":    "on:\n  push:\njobs:\n  a:\n    steps:\n      - run: ./le integration interop-ipsec\n",
	}
	got := scheduledActionsFrom(sources)
	if got["integration/interop"] != "nightly.yml" {
		t.Errorf("a scheduled action is not credited: %v", got)
	}
	if _, held := got["integration/interop-ipsec"]; held {
		t.Errorf("a push-only action was credited as nightly: %v", got)
	}
}

func TestTheFirstScheduledWorkflowNamingAnActionIsTheOneRecorded(t *testing.T) {
	// First wins in sorted workflow order, so filesystem listing order cannot
	// change which scheduled pipeline the ledger names.
	sources := map[string]string{
		"b-nightly.yml": "on:\n  schedule:\n    - cron: '0 3 * * *'\njobs:\n  a:\n    steps:\n      - run: ./le integration interop\n",
		"a-nightly.yml": "on:\n  schedule:\n    - cron: '0 4 * * *'\njobs:\n  a:\n    steps:\n      - run: ./le integration interop\n",
	}
	if got := scheduledActionsFrom(sources)["integration/interop"]; got != "a-nightly.yml" {
		t.Errorf("the action is credited to %q, want the first workflow in order", got)
	}
}

func TestACommentedOutCommandGrantsNothing(t *testing.T) {
	sources := map[string]string{
		"nightly.yml": "on:\n  schedule:\n    - cron: '0 3 * * *'\njobs:\n  a:\n    steps:\n      # - run: ./le integration interop\n",
	}
	if got := scheduledActionsFrom(sources); len(got) != 0 {
		t.Errorf("a commented-out command granted a tier: %v", got)
	}
}

func TestNativeActionsInWorkflowCommands(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{"one action", "run: ./le integration interop\n", []string{"integration/interop"}},
		{"a wrapper", "run: sudo ./le rfc check\n", []string{"rfc/check"}},
		{"a chain", "run: ./le rfc check && ./le doc check verify\n", []string{"rfc/check", "doc check/verify"}},
		{"a quoted scalar", "- \"./le tier check\"\n", []string{"tier/check"}},
		{"arguments do not change identity", "run: ./le verify current mode full\n", []string{"verify/current"}},
		{"no native action", "run: echo a\n", nil},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			got := nativeActionsIn(one.src, func(name string) bool { return name == "doc check" })
			if !slices.Equal(got, one.want) {
				t.Fatalf("NativeActionsIn(%q) = %v, want %v", one.src, got, one.want)
			}
		})
	}
}

func TestAWorkflowDirectoryTheGateCannotReadIsRefused(t *testing.T) {
	// Not knowing what CI runs is a different fact from CI running nothing, and
	// answering "everything runs" there would credit evidence to a pipeline
	// nobody confirmed.
	_, err := scheduledWorkflowActions(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "cannot read the workflow directory") {
		t.Errorf("an absent workflow directory was read as empty: %v", err)
	}
}

// VALIDATES: a carrier's `terminator=` block holds a fixture rather than an
// assertion, so moving that fixture out of the file changes no RFC-tagged
// behavior, while any edit to the carrier's own directives still does.
// PREVENTS: the owner-approval gate demanding a ruling on a fixture move, and
// the opposite failure of a weakened assertion slipping through beside one.
func TestACarrierFixtureBlockIsNotTestedBehavior(t *testing.T) {
	const head = "# " + rfcTagMarker + " RFC4271-5.1.4-1 positive\n" +
		"cmd=start\n" +
		"expect=bgp:conn=1:seq=1:hex=FFFF\n"
	embedded := head +
		"tmpfs=probe.run:mode=755:terminator=EOF_PROBE\n" +
		"#!/usr/bin/env python3\n" +
		"runtime_fail('the fixture asserts here')\n" +
		"EOF_PROBE\n"
	compiled := head +
		"tmpfs=probe.run:mode=755:terminator=EOF_PROBE\n" +
		"run \"ze-test fixture plugin/probe\"\n" +
		"EOF_PROBE\n"

	if tags := ChangedTags("test/plugin/probe.ci", embedded, compiled); len(tags) != 0 {
		t.Errorf("moving a fixture out of the block reported %v, want no change", tags)
	}

	weakened := strings.Replace(compiled,
		"expect=bgp:conn=1:seq=1:hex=FFFF\n", "", 1)
	if tags := ChangedTags("test/plugin/probe.ci", embedded, weakened); len(tags) == 0 {
		t.Error("dropping an expect= directive reported no change, so a weakened assertion needs no approval")
	}

	retagged := strings.Replace(compiled,
		"# "+rfcTagMarker+" RFC4271-5.1.4-1 positive\n", "", 1)
	if tags := ChangedTags("test/plugin/probe.ci", embedded, retagged); len(tags) == 0 {
		t.Error("dropping the RFC tag itself reported no change")
	}
}

// rfcTagMarker is the tag word spelled in two pieces so that this package's own
// FIXTURES are not read as tags when a tree walk scans these test files.
//
// A fixture here is Go or .ci source held in a string, and it must contain a
// real tag for the scanner under test to find. Written whole, that literal also
// makes the test file itself a tag carrier: `IsTagCarrier` and `rfcTagPattern`
// (`internal/le/commit/rfcchange.go`) read a file's TEXT, so every edit to this
// file demanded owner approval for requirement ids that name no RFC. The same
// idiom guards `staleLeReferences` against its own corpus in
// `internal/le/contract_test.go`.
const rfcTagMarker = "RFC requi" + "rement:"
