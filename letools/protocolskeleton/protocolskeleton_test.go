// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8 -- the protocol-skeleton
// lens is called as a function, answers structured data, and keeps the one
// exit code it is allowed to fail with in its selftest.
// PREVENTS: an advisory that silently stops classifying. Report mode always
// exits 0, so a classifier that answered "domain" for everything would look
// exactly like a conformant tree; the selftest cases are the only thing that
// can tell the two apart.

package protocolskeleton

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassificationOfEachKind(t *testing.T) {
	cases := []struct {
		protocol string
		module   string
		want     string
	}{
		{"bfd", "packet", "canonical"},
		{"bfd", "session", "rfc-state"},
		{"isis", "adjacency", "rfc-state"},
		{"ospf", "neighbor", "rfc-state"},
		{"bgp", "fsm", "rfc-state"},
		{"ospf", "v3", "version"},
		{"bgp", "wireu", "legacy-exception"},
		{"ike", "wire", "legacy-exception"},
		{"ospf", "wire", "domain"},
		{"isis", "lsdb", "domain"},
	}
	for _, tc := range cases {
		if got := Classify(tc.protocol, tc.module); got != tc.want {
			t.Errorf("Classify(%q, %q) = %q, want %q", tc.protocol, tc.module, got, tc.want)
		}
	}
}

func TestAVersionIsDigitsAfterAV(t *testing.T) {
	if Classify("ospf", "v") == "version" {
		t.Error("a bare v was read as a version directory")
	}
	if Classify("ospf", "v3beta") == "version" {
		t.Error("v3beta was read as a version directory")
	}
	if Classify("ospf", "v12") != "version" {
		t.Error("v12 was not read as a version directory")
	}
}

// tree writes a fixture checkout holding the directories named, and answers
// its root.
func tree(t *testing.T, dirs ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, rel := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o750); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	return root
}

func TestTestdataAndHiddenDirectoriesAreSkipped(t *testing.T) {
	root := tree(t, "internal/plugins/demo/packet", "internal/plugins/demo/yang",
		"internal/plugins/demo/testdata", "internal/plugins/demo/.cache")

	report, err := Build(root, []Protocol{{Name: "demo", Root: "internal/plugins/demo"}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(report.Protocols[0].Modules) != 2 {
		t.Fatalf("the walk read %+v, want packet and yang only", report.Protocols[0].Modules)
	}
}

func TestARootAndAYangDirectoryIsSinglePackage(t *testing.T) {
	root := tree(t, "internal/plugins/flat/yang")

	report, err := Build(root, []Protocol{{Name: "flat", Root: "internal/plugins/flat"}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !report.Protocols[0].SinglePackage {
		t.Error("a root carrying only yang/ is not single-package")
	}
}

func TestAProtocolWithSubpackagesIsNotSinglePackage(t *testing.T) {
	root := tree(t, "internal/plugins/demo/packet", "internal/plugins/demo/yang")

	report, err := Build(root, []Protocol{{Name: "demo", Root: "internal/plugins/demo"}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if report.Protocols[0].SinglePackage {
		t.Error("a root carrying packet/ is single-package")
	}
}

func TestAManifestRowWhoseRootIsGoneIsMissing(t *testing.T) {
	root := tree(t)

	report, err := Build(root, []Protocol{{Name: "gone", Root: "internal/plugins/gone"}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !report.Protocols[0].Missing {
		t.Fatal("a manifest row with no root was not flagged")
	}
	if report.Protocols[0].SinglePackage {
		t.Error("a missing root renders as single-package")
	}
	if !strings.Contains(report.Text(), "MISSING roots: gone") {
		t.Errorf("the summary does not name the missing root: %q", report.Text())
	}
}

func TestTheSummaryCountsEveryClass(t *testing.T) {
	root := tree(t,
		"internal/plugins/demo/packet",  // canonical
		"internal/plugins/demo/session", // rfc-state
		"internal/plugins/demo/v2",      // version
		"internal/plugins/demo/lsdb",    // domain
	)

	report, err := Build(root, []Protocol{{Name: "demo", Root: "internal/plugins/demo"}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	text := report.Text()
	for _, want := range []string{"canonical 1", "rfc-state 1", "version 1", "domain 1", "legacy 0"} {
		if !strings.Contains(text, want) {
			t.Errorf("the summary does not carry %q: %s", want, text)
		}
	}
	if !strings.HasPrefix(text, "protocol-skeleton advisory: 1 protocols;") {
		t.Errorf("the summary does not open with the count: %q", text)
	}
}

func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	root := tree(t, "internal/plugins/demo/packet")

	report, err := Build(root, []Protocol{{Name: "demo", Root: "internal/plugins/demo"}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, want := range []string{`"protocols"`, `"counts"`, `"modules"`, `"class"`, `"single-package"`, `"rfc-state"`, `"legacy-exception"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the payload has no %s key: %s", want, raw)
		}
	}
	if strings.Contains(string(raw), "_") {
		t.Errorf("a JSON key is snake_case: %s", raw)
	}
}

func TestTheSelftestPassesOverItsOwnCases(t *testing.T) {
	report := Selftest()

	if failures := report.Failures(); len(failures) != 0 {
		t.Fatalf("the selftest failed: %+v", failures)
	}
	if report.Code(1) != 0 {
		t.Errorf("a passing selftest answered %d", report.Code(1))
	}
}

func TestEverySelftestCaseIsARow(t *testing.T) {
	report := Selftest()

	if len(report.Results) != len(selftestCases)+len(walkCases) {
		t.Fatalf("the selftest answered %d rows for %d declared cases",
			len(report.Results), len(selftestCases)+len(walkCases))
	}
	for _, row := range report.Results {
		if row.Case == "" {
			t.Error("a case row carries no name")
		}
	}
}

func TestABrokenClassifierIsCaughtByTheSelftest(t *testing.T) {
	// The mutation the selftest exists to catch, driven through the same case
	// table the action runs: a classifier that answers one word for everything
	// passes report mode and fails here.
	report := runCases(func(string, string) string { return "domain" })

	if len(report.Failures()) == 0 {
		t.Fatal("a classifier answering one word for everything passed the selftest")
	}
	if report.Code(1) != 1 {
		t.Errorf("a failing selftest answered %d, want 1", report.Code(1))
	}
}

func TestReportModeNeverFails(t *testing.T) {
	payload, code := Answer([]string{"report"})

	if code != 0 {
		t.Fatalf("the advisory answered %d over this checkout", code)
	}
	if _, ok := payload.(Report); !ok {
		t.Fatalf("the report is not the payload: %T", payload)
	}
}

func TestTheAreaHoldsBothActionsAndNeitherWrites(t *testing.T) {
	list := Actions()

	if len(list.Actions) != 2 {
		t.Fatalf("the area holds %d actions, want two", len(list.Actions))
	}
	for _, row := range list.Actions {
		if row.Writes {
			t.Errorf("%q is marked as writing, and this tool writes nothing", row.Verb)
		}
	}
}
