package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// tree builds a fixture checkout from a path -> content map and answers its
// root.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("writing %s: %v", rel, err)
		}
	}
	return root
}

// messages answers the finding messages of a whole run over the fixture.
func messages(t *testing.T, root string, changed []string) []string {
	t.Helper()
	report, err := Run(t.Context(), root, changed)
	if err != nil {
		t.Fatalf("running the checks: %v", err)
	}
	out := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		out = append(out, finding.Message)
	}
	return out
}

// VALIDATES: an anchor carrying a line number is reported, and the message
// names the anchor as written.
// PREVENTS: the two anchor patterns being collapsed, which would either stop
// reporting the line number or name the anchor "unknown".
func TestASourceAnchorWithALineNumberIsReported(t *testing.T) {
	root := tree(t, map[string]string{
		"docs/a.md":       "<!-- source: internal/a/x.go:42 -- why -->\n",
		"internal/a/x.go": "package a\n",
	})
	findings, err := checkSourceAnchorLineNumbers(root)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	want := "source anchor internal/a/x.go:42 contains line number; use path only (line numbers rot)"
	if len(findings) != 1 || findings[0].Message != want {
		t.Fatalf("findings %+v, want exactly %q", findings, want)
	}
	if findings[0].File != "docs/a.md" || findings[0].Line != 1 {
		t.Fatalf("located at %s:%d, want docs/a.md:1", findings[0].File, findings[0].Line)
	}
}

// VALIDATES: an anchor naming a file that is gone is reported, and each of the
// four spellings of an anchor pointing OUTSIDE the repository is not.
// PREVENTS: a sibling checkout's path deciding a gate's verdict, which would
// make the answer depend on where the reader keeps their checkouts.
func TestOnlyAnInRepositoryAnchorIsResolved(t *testing.T) {
	root := tree(t, map[string]string{
		"docs/a.md": strings.Join([]string{
			"<!-- source: internal/a/gone.go -- missing -->",
			"<!-- source: internal/a/x.go -- here -->",
			"<!-- source: https://example.com/x.go -- a url -->",
			"<!-- source: ~/checkouts/other/x.go -- a home path -->",
			"<!-- source: /etc/x.go -- an absolute path -->",
			"<!-- source: ../gh-pages/tools/build.py -- a sibling checkout -->",
			"<!-- source: register.go -- a bare name -->",
			"",
		}, "\n"),
		"internal/a/x.go": "package a\n",
	})
	findings, err := checkSourceAnchorStalePaths(root)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	want := "source anchor points to non-existent file: internal/a/gone.go"
	if len(findings) != 1 || findings[0].Message != want {
		t.Fatalf("findings %+v, want exactly %q", findings, want)
	}
}

// VALIDATES: the anchor path's trailing line number is stripped before the file
// is looked for.
// PREVENTS: an anchor with a line number being reported twice, once for the
// line number and once as a file that does not exist.
func TestTheLineSuffixIsStrippedBeforeTheFileIsLookedFor(t *testing.T) {
	root := tree(t, map[string]string{
		"docs/a.md":       "<!-- source: internal/a/x.go:42 -- why -->\n",
		"internal/a/x.go": "package a\n",
	})
	findings, err := checkSourceAnchorStalePaths(root)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings %+v, want none: the file exists once the suffix is stripped", findings)
	}
}

// VALIDATES: documents are read in Python's path-COMPONENT order, so two
// findings come back in the order the script reports them.
// PREVENTS: a plain string sort, which puts docs/a-b before docs/a and reverses
// the page a developer compares against the script's.
func TestDocumentsAreOrderedByPathComponent(t *testing.T) {
	root := tree(t, map[string]string{
		"docs/a-b/x.md": "<!-- source: internal/gone2.go -- -->\n",
		"docs/a/x.md":   "<!-- source: internal/gone1.go -- -->\n",
	})
	findings, err := checkSourceAnchorStalePaths(root)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("findings %+v, want two", findings)
	}
	if findings[0].File != "docs/a/x.md" || findings[1].File != "docs/a-b/x.md" {
		t.Fatalf("order %s then %s, want docs/a/x.md first", findings[0].File, findings[1].File)
	}
}

// VALIDATES: an unreadable document is an ERROR rather than a document that
// contributed no finding.
// PREVENTS: the script's fail-open, where a file the walk cannot read makes the
// gate greener and the page says nothing about it.
func TestAnUnreadableDocumentIsAnError(t *testing.T) {
	root := tree(t, map[string]string{"docs/a.md": "<!-- source: internal/gone.go -- -->\n"})
	if err := os.Chmod(filepath.Join(root, "docs", "a.md"), 0o000); err != nil {
		t.Skipf("cannot make a file unreadable here: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "docs", "a.md"), 0o600) })

	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so this case cannot be staged")
	}
	if _, err := checkSourceAnchorStalePaths(root); err == nil {
		t.Fatal("an unreadable document was passed over, so a lower finding count is what passing looks like")
	}
}

// VALIDATES: only an in-progress spec is read, only rows inside the Acceptance
// Criteria section count, and only an empty Demonstrated By is a
// finding.
// PREVENTS: the section boundary being dropped, which would report every
// AC-shaped row anywhere in the spec.
func TestOnlyAnInProgressSpecsAcceptanceCriteriaAreRead(t *testing.T) {
	inProgress := strings.Join([]string{
		"| Status | in-progress |",
		"### Acceptance Criteria",
		"| AC-1 | does a thing | TestOne | a note |",
		"| AC-2 | does another |  | a note |",
		"### Another Section",
		"| AC-9 | outside the section |  | a note |",
		"",
	}, "\n")
	done := strings.Join([]string{
		"| Status | complete |",
		"### Acceptance Criteria",
		"| AC-1 | does a thing |  | a note |",
		"",
	}, "\n")
	root := tree(t, map[string]string{
		"plan/spec-open.md":   inProgress,
		"plan/spec-closed.md": done,
		"plan/notes.md":       inProgress,
	})

	findings, err := checkSpecACCompleteness(root)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings %+v, want exactly one", findings)
	}
	if findings[0].Message != "AC-2 has empty 'Demonstrated By' column" {
		t.Fatalf("message %q", findings[0].Message)
	}
	if findings[0].File != "plan/spec-open.md" || findings[0].Line != 4 {
		t.Fatalf("located at %s:%d, want plan/spec-open.md:4", findings[0].File, findings[0].Line)
	}
}

// VALIDATES: a changed CLI file's registered command must be named by some .ci
// test, and the whole corpus is one document.
// PREVENTS: the corpus being read per file, where a command tested in one .ci
// and registered beside another would be reported.
func TestARegisteredCommandNeedsACITestNamingIt(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/plugins/x/cmd.go": "package x\nfunc init(){ MustRegisterRootHandler(\"show thing\", nil) \n MustRegisterRootHandler(\"hide thing\", nil) }\n",
		"test/ui/a.ci":              "command=show thing\n",
	})
	findings, err := checkCLIHandlerCoverage(root, []string{"internal/plugins/x/cmd.go"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	want := "CLI command 'hide thing' has no .ci test mentioning it"
	if len(findings) != 1 || findings[0].Message != want {
		t.Fatalf("findings %+v, want exactly %q", findings, want)
	}
	if findings[0].Line != 0 {
		t.Fatalf("line %d, want 0: the finding is about the file rather than a line", findings[0].Line)
	}
}

// VALIDATES: an exported symbol with no caller outside its own package is
// reported, and one with such a caller is not.
// PREVENTS: the package-directory comparison being dropped, which would let a
// symbol's own package count as its caller and flag nothing.
func TestAnExportedSymbolNeedsACallerOutsideItsPackage(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/a/x.go":    "package a\n\nfunc Called() {}\n\nfunc Orphan() {}\n",
		"internal/a/self.go": "package a\n\nfunc use() { Orphan() }\n",
		"internal/b/call.go": "package b\n\nfunc use() { a.Called() }\n",
		"internal/b/near.go": "package b\n\nvar OrphanRelated = 1\n",
	})
	got := messages(t, root, []string{"internal/a/x.go"})
	want := "exported symbol Orphan has no cross-package non-test caller"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("findings %q, want exactly %q -- OrphanRelated must not count as a caller of Orphan", got, want)
	}
}

func TestDeclarationScanIgnoresSourceTextInsideStrings(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/a/x.go":               "package a\n\nconst fixture = `func Fake() {}\ntype FakeType struct{}`\n\nfunc Orphan() {}\n",
		"internal/a/testdata/broken.go": "package a\nfunc Broken( {\n",
	})
	got := messages(t, root, []string{"internal/a/x.go", "internal/a/testdata/broken.go"})
	if len(got) != 1 || !strings.Contains(got[0], "Orphan") {
		t.Fatalf("findings %q, want only the real exported declaration", got)
	}
}

// VALIDATES: a test file is not a caller, unless the symbol is declared under
// internal/test/ AND the test file imports that helper package.
// PREVENTS: both halves of the exemption being lost: a bare word in any test
// file counting as wiring, or a test helper always reading as dead.
func TestATestFileIsACallerOnlyForATestHelperItImports(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/a/x.go":           "package a\n\nfunc Orphan() {}\n",
		"internal/b/x_test.go":      "package b\n\nfunc TestX() { a.Orphan() }\n",
		"internal/test/golden/g.go": "package golden\n\nfunc Compare() {}\n",
		"internal/test/golden/h.go": "package golden\n\nfunc Bare() {}\n",
		"internal/c/c_test.go":      "package c\n\nimport \"example.com/m/internal/test/golden\"\n\nfunc TestC() { golden.Compare() }\n",
		"internal/d/d_test.go":      "package d\n\nfunc TestD() { Bare() }\n",
		// A test file imports an ordinary package and calls its symbol. The
		// import makes this case discriminate. Without it, the wrong refusal
		// CAN widen the helper exemption to every package without detection.
	})

	got := messages(t, root, []string{"internal/a/x.go"})
	if len(got) != 1 || !strings.Contains(got[0], "Orphan") {
		t.Fatalf("findings %q, want Orphan flagged: a test caller is not wiring for an ordinary package, "+
			"even from a test file that imports it", got)
	}

	got = messages(t, root, []string{"internal/test/golden/g.go", "internal/test/golden/h.go"})
	if len(got) != 1 || !strings.Contains(got[0], "Bare") {
		t.Fatalf("findings %q, want Bare alone: Compare is called from a test that IMPORTS the helper and Bare from one that does not", got)
	}
}

// VALIDATES: a *ForTest helper is never reported.
// PREVENTS: the naming convention being dropped, which would flag every
// test-only helper the caller search excludes by design.
func TestAForTestHelperIsNeverReported(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/a/x.go": "package a\n\nfunc ResetForTest() {}\n",
	})
	if got := messages(t, root, []string{"internal/a/x.go"}); len(got) != 0 {
		t.Fatalf("findings %q, want none", got)
	}
}

// VALIDATES: a type reached only through its exported constants is exempt, and
// the iota inheritance rule is what recovers those constants.
// PREVENTS: the block-const walk regressing, where a typed enum whose callers
// only ever spell its values reads as dead.
func TestATypeReachedThroughItsConstantsIsExempt(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/a/x.go": strings.Join([]string{
			"package a",
			"",
			"type RouteVerb int",
			"",
			"const (",
			"\tRouteVerbInstall RouteVerb = iota",
			"\tRouteVerbWithdraw",
			")",
			"",
			"type DeadVerb int",
			"",
			"const (",
			"\tDeadVerbOne DeadVerb = iota",
			")",
			"",
		}, "\n"),
		"internal/b/use.go": "package b\n\nfunc use() { _ = a.RouteVerbWithdraw }\n",
	})
	got := messages(t, root, []string{"internal/a/x.go"})
	if len(got) != 1 || !strings.Contains(got[0], "DeadVerb ") {
		t.Fatalf("findings %q, want DeadVerb alone: RouteVerb is reached through RouteVerbWithdraw, which inherits its type from the iota spec above it", got)
	}
}

// VALIDATES: an explicit value expression with no type RESETS the iota
// inheritance, so a bare spec after it is not counted.
// PREVENTS: the reset being dropped, which would give every later constant of
// the block the earlier type and exempt a dead type by accident.
func TestAValueExpressionResetsTheTypeInheritance(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/a/x.go": strings.Join([]string{
			"package a",
			"",
			"type Kind int",
			"",
			"const (",
			"\tFirst Kind = iota",
			"\tReset = 7",
			"\tAfter",
			")",
			"",
		}, "\n"),
		"internal/b/use.go": "package b\n\nfunc use() { _ = a.After }\n",
	})
	got := messages(t, root, []string{"internal/a/x.go"})
	if len(got) != 1 || !strings.Contains(got[0], "Kind ") {
		t.Fatalf("findings %q, want Kind flagged: After follows a value expression, so it is not a Kind", got)
	}
}

// VALIDATES: the four remaining type seams each exempt on their own, and each
// is bounded by a cross-package caller on the function or interface
// that carries the type.
// PREVENTS: an unbounded seam, where any type mentioned in any signature is
// exempt and the check fails open over nearly every dead type.
func TestEachTypeSeamIsExemptAndBounded(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		changed string
		flagged []string
	}{
		{
			name: "struct field",
			files: map[string]string{
				"internal/a/x.go": "package a\n\ntype CPU struct{}\n\ntype Dead struct{}\n\ntype Inventory struct {\n\tCPU CPU\n}\n",
				"internal/b/u.go": "package b\n\nfunc use() { _ = a.Inventory{} }\n",
			},
			changed: "internal/a/x.go",
			flagged: []string{"Dead"},
		},
		{
			name: "returned by a wired constructor",
			files: map[string]string{
				"internal/a/x.go": "package a\n\ntype Evaluator struct{}\n\ntype Dead struct{}\n\nfunc Global() *Evaluator { return nil }\n\nfunc Unused() *Dead { return nil }\n",
				"internal/b/u.go": "package b\n\nfunc use() { _ = a.Global() }\n",
			},
			changed: "internal/a/x.go",
			flagged: []string{"Dead", "Unused"},
		},
		{
			name: "parameter of a wired setter",
			files: map[string]string{
				"internal/a/x.go": "package a\n\ntype PingFactory func()\n\ntype Dead func()\n\nfunc SetPingFactory(f PingFactory) {}\n\nfunc SetDead(d Dead) {}\n",
				"internal/b/u.go": "package b\n\nfunc use() { a.SetPingFactory(nil) }\n",
			},
			changed: "internal/a/x.go",
			flagged: []string{"Dead", "SetDead"},
		},
		{
			name: "embedded in a live interface",
			files: map[string]string{
				"internal/a/x.go": "package a\n\ntype Reader interface {\n\tRead()\n}\n\ntype Dead interface {\n\tGone()\n}\n\ntype Store interface {\n\tReader\n}\n\ntype Shelf interface {\n\tDead\n}\n",
				"internal/b/u.go": "package b\n\nfunc use(s a.Store) {}\n",
			},
			changed: "internal/a/x.go",
			flagged: []string{"Dead", "Shelf"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := tree(t, testCase.files)
			got := messages(t, root, []string{testCase.changed})
			if len(got) != len(testCase.flagged) {
				t.Fatalf("findings %q, want %d of them naming %q", got, len(testCase.flagged), testCase.flagged)
			}
			for i, name := range testCase.flagged {
				var found bool
				for _, message := range got {
					if strings.Contains(message, name+" has no") {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("finding %d: %q does not name %s", i, got, name)
				}
			}
		})
	}
}

// VALIDATES: a method on an UNEXPORTED receiver is exempt when its name is
// declared by an exported interface of the same package, and is not
// when it is merely a neighbor of one.
// PREVENTS: the exemption widening to every method of a package that declares
// any interface, which would stop the check reading methods at all.
func TestAnUnexportedReceiversMethodIsExemptOnlyWhenAnInterfaceDeclaresIt(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/a/x.go": strings.Join([]string{
			"package a",
			"",
			"type Backend interface {",
			"\tStart()",
			"}",
			"",
			// An UNEXPORTED interface, which must exempt nothing: no other
			// package can name it, so a method satisfying it is reached from
			// inside this package alone.
			"type hidden interface {",
			"\tHidden()",
			"}",
			"",
			"type backend struct{}",
			"",
			"func (b *backend) Start() {}",
			"",
			"func (b *backend) Hidden() {}",
			"",
			"func (b *backend) Neighbor() {}",
			"",
		}, "\n"),
	})
	got := messages(t, root, []string{"internal/a/x.go"})
	if len(got) != 3 {
		t.Fatalf("findings %q, want three: Backend has no caller, and neither Hidden nor Neighbor is declared "+
			"by an EXPORTED interface", got)
	}
	var sawHidden bool
	for _, message := range got {
		if strings.Contains(message, "symbol Hidden ") {
			sawHidden = true
		}
	}
	if !sawHidden {
		t.Fatalf("Hidden was not flagged, so an unexported interface exempted it: %q", got)
	}
	for _, message := range got {
		if strings.Contains(message, "Start ") {
			t.Fatalf("Start was flagged, but the exported Backend interface declares it: %q", got)
		}
	}
}

// VALIDATES: a gRPC service registration makes its interface's methods
// reachable on the receiver it binds, and only on that receiver.
// PREVENTS: the receiver being ignored, which would exempt every method name
// the interface declares wherever it is defined.
func TestAGRPCRegistrationExemptsItsOwnReceiversMethods(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/a/x.go": strings.Join([]string{
			"package a",
			"",
			"type thingServer struct{}",
			"",
			"func (s *thingServer) List() {}",
			"",
			"type otherServer struct{}",
			"",
			"func (s *otherServer) List() {}",
			"",
			"func wire(g *grpc.Server) {",
			"\tpb.RegisterThingServer(g, &thingServer{})",
			"}",
			"",
		}, "\n"),
		"api/pb.go": "package pb\n\ntype ThingServer interface {\n\tList()\n}\n",
	})
	got := messages(t, root, []string{"internal/a/x.go"})
	if len(got) != 1 || !strings.Contains(got[0], "List ") {
		t.Fatalf("findings %q, want exactly one List: only thingServer is registered", got)
	}
}

// VALIDATES: the named interface-dispatch allowlist exempts the exact methods
// it names and nothing else.
// PREVENTS: the list being read as a package-wide exemption, which is what its
// own comment forbids.
func TestTheInterfaceDispatchAllowlistIsExact(t *testing.T) {
	site := dispatchSite{Package: "internal/component/api/grpc", Receiver: "transportCompletionStatsHandler"}
	methods, ok := InterfaceDispatchMethods[site]
	if !ok {
		t.Fatal("the one dispatch site is gone")
	}
	for _, name := range []string{"TagRPC", "HandleRPC", "TagConn", "HandleConn"} {
		if !methods[name] {
			t.Errorf("%s is no longer exempt", name)
		}
	}
	if methods["Anything"] {
		t.Error("the allowlist exempts a method it does not name")
	}
	if len(methods) != 4 {
		t.Errorf("the allowlist holds %d methods, want the four grpc-go dispatches", len(methods))
	}
}

// VALIDATES: a failing git command is an error rather than an empty changed set.
// PREVENTS: the script's fail-open, where a git that could not run leaves both
// changed-file checks judging a population they never obtained.
func TestAFailingGitCommandIsAnError(t *testing.T) {
	root := tree(t, map[string]string{"go.mod": "module example.com/m\n"})
	if _, err := ChangedFiles(t.Context(), root); err == nil {
		t.Fatal("a directory that is not a git checkout answered a changed set instead of an error")
	}
}

// VALIDATES: the answer is structured data with the counts and the changed set
// beside the findings.
// PREVENTS: a payload a caller cannot act on, which would leave `| json` with
// nothing but a rendered page.
func TestTheReportIsStructuredData(t *testing.T) {
	root := tree(t, map[string]string{
		"docs/a.md": "<!-- source: internal/gone.go -- -->\n",
	})
	report, err := Run(t.Context(), root, nil)
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	if report.Issues != 1 || report.Warnings != 0 || report.Code() != 1 {
		t.Fatalf("issues %d warnings %d code %d, want 1, 0 and 1", report.Issues, report.Warnings, report.Code())
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	for _, key := range []string{"changed", "findings", "issues", "warnings"} {
		if _, ok := document[key]; !ok {
			t.Errorf("the payload has no %q key: %s", key, raw)
		}
	}
	rows, _ := document["findings"].([]any)
	row, _ := rows[0].(map[string]any)
	for _, key := range []string{"severity", "file", "line", "message"} {
		if _, ok := row[key]; !ok {
			t.Errorf("a finding row has no %q key: %s", key, raw)
		}
	}
}

// VALIDATES: the page carries the script's colors and its two headings.
// PREVENTS: a page that colors itself only on a terminal, which would compare
// equal to the script's for the wrong reason.
func TestThePageIsColoredUnconditionally(t *testing.T) {
	clean := Report{Findings: []Finding{}}
	want := colorGreen + "./le repository: all checks passed" + colorReset + "\n"
	if got := clean.Text(); got != want {
		t.Errorf("clean page %q", got)
	}

	red := Report{
		Findings: []Finding{
			{Severity: severityIssue, File: "internal/a/x.go", Line: 3, Message: "one"},
			{Severity: severityWarn, File: "docs/a.md", Message: "two"},
		},
		Issues: 1, Warnings: 1,
	}
	page := red.Text()
	for _, want := range []string{
		colorRed + "./le repository: 1 issue(s) found" + colorReset + "\n",
		"  " + colorRed + "[ISSUE] internal/a/x.go:3: one" + colorReset + "\n",
		colorYellow + "./le repository: 1 warning(s)" + colorReset + "\n",
		"  " + colorYellow + "[WARN] docs/a.md: two" + colorReset + "\n",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page is missing %q:\n%q", want, page)
		}
	}
}

// VALIDATES: an issue and warning use DIFFERENT semantic palette roles.
// PREVENTS: identical rendering for the two severities.
//
// The script uses plain three-bit codes, which compiled Ze packages cannot use
// (docs/architecture/cli/color-system.md). This test asserts the SHADE, then
// removes color before it compares the two migration pages.
func TestTheTwoSeveritiesAreColoredApart(t *testing.T) {
	red := Report{
		Findings: []Finding{
			{Severity: severityIssue, File: "internal/a/x.go", Line: 3, Message: "one"},
			{Severity: severityWarn, File: "docs/a.md", Message: "two"},
		},
		Issues: 1, Warnings: 1,
	}
	page := red.Text()
	issue := spanBefore(page, "[ISSUE]")
	warning := spanBefore(page, "[WARN]")
	if issue == warning {
		t.Fatalf("both severities are colored %q, so the report reads as one severity", issue)
	}
	if issue != textbuf.ColorBoldRed || warning != textbuf.ColorBrightYellow {
		t.Errorf("the severities are colored %q and %q, want the palette's roles", issue, warning)
	}
	if got := spanBefore(Report{Findings: []Finding{}}.Text(), "all checks passed"); got != textbuf.ColorBrightGreen {
		t.Errorf("the pass line is colored %q, want the palette's role", got)
	}
	if plainText(page) == page {
		t.Error("the page carries no color at all, so it depends on a terminal the gate never sees")
	}
}

// ansiRE matches an SGR escape sequence, so an assertion can be about the text
// rather than about the shade.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plainText(text string) string { return ansiRE.ReplaceAllString(text, "") }

// spanBefore answers the escape sequence immediately before the first
// occurrence of needle.
func spanBefore(text, needle string) string {
	before, _, found := strings.Cut(text, needle)
	if !found {
		return ""
	}
	spans := ansiRE.FindAllString(before, -1)
	if len(spans) == 0 {
		return ""
	}
	return spans[len(spans)-1]
}

// VALIDATES: the tree-wide gate runs the three tree-wide checks and neither
// changed-file check, whatever the working tree holds.
// PREVENTS: an empty changed set being read as "no flag given", which would
// send the pre-commit gate back to git diff and put both
// changed-file checks over other sessions' half-written files.
func TestAnEmptyChangedSetRunsNeitherChangedFileCheck(t *testing.T) {
	root := tree(t, map[string]string{
		"docs/a.md":                 "<!-- source: internal/gone.go -- -->\n",
		"internal/a/x.go":           "package a\n\nfunc Orphan() {}\n",
		"internal/plugins/x/cmd.go": "package x\nfunc init(){ MustRegisterRootHandler(\"show thing\", nil) }\n",
		"test/ui/a.ci":              "nothing\n",
	})

	got := messages(t, root, nil)
	if len(got) != 1 || !strings.Contains(got[0], "source anchor") {
		t.Fatalf("findings %q, want the one tree-wide finding", got)
	}

	got = messages(t, root, []string{"internal/a/x.go", "internal/plugins/x/cmd.go"})
	if len(got) != 3 {
		t.Fatalf("findings %q, want three once the changed set names those files", got)
	}
}

// VALIDATES: repository checks and generation use native action names.
// PREVENTS: losing a repository action during routing changes.
func TestTheAreaPublishesNativeActions(t *testing.T) {
	list := Actions()
	want := []string{"check", "tree-check", "generate", "generated-check"}
	if len(list.Actions) != len(want) {
		t.Fatalf("actions %+v", list.Actions)
	}
	for index, row := range list.Actions {
		if row.Verb != want[index] {
			t.Fatalf("action %d = %+v, want %q", index, row, want[index])
		}
	}
	for _, row := range list.Actions {
		if row.Writes != (row.Verb == "generate") {
			t.Errorf("%s writes=%v", row.Verb, row.Writes)
		}
	}
}

// VALIDATES: an action this area does not hold answers 2, and an action given a
// value answers 1.
// PREVENTS: the two refusals collapsing into one code, which a caller reading
// them apart depends on.
func TestTheAreaRefusesAnUnknownActionApartFromAValue(t *testing.T) {
	if _, code := Answer([]string{"nope"}); code != 2 {
		t.Errorf("an unknown action answered %d, want 2", code)
	}
	if _, code := Answer([]string{"check", "internal/a"}); code != 2 {
		t.Errorf("a value after an action answered %d, want 2", code)
	}
}

// VALIDATES: grep's word bound is reproduced, which is what decides whether a
// symbol has a caller at all.
// PREVENTS: a substring search, where every symbol whose name is a prefix of
// another reads as wired.
func TestTheWordSearchBoundsItsMatch(t *testing.T) {
	cases := []struct {
		text, word string
		want       bool
	}{
		{"a.Report()", "Report", true},
		{"a.ReportError()", "Report", false},
		{"myReport", "Report", false},
		{"_Report", "Report", false},
		{"Report", "Report", true},
		{"(Report)", "Report", true},
		{"Report9", "Report", false},
	}
	for _, testCase := range cases {
		if got := hasWord(testCase.text, testCase.word); got != testCase.want {
			t.Errorf("hasWord(%q, %q) = %v, want %v", testCase.text, testCase.word, got, testCase.want)
		}
	}
}

// VALIDATES: a type mention is not counted when it is a selector's tail or part
// of a longer identifier, which is the lookbehind RE2 cannot write.
// PREVENTS: pkg.Report exempting a local Report, which would make the
// constructor seam fire for a type another package declares.
func TestATypeMentionIsNotASelectorsTail(t *testing.T) {
	cases := []struct {
		part, name string
		want       bool
	}{
		{"() *Report", "Report", true},
		{"() *pkg.Report", "Report", false},
		{"() *MyReport", "Report", false},
		{"(r Report, n int)", "Report", true},
		{"(r Reporter)", "Report", false},
	}
	for _, testCase := range cases {
		if got := mentionsType(testCase.part, testCase.name); got != testCase.want {
			t.Errorf("mentionsType(%q, %q) = %v, want %v", testCase.part, testCase.name, got, testCase.want)
		}
	}
}

// VALIDATES: the signature split balances parentheses from the PARAMETER list,
// so a func-typed parameter and a multi-value return are both read.
// PREVENTS: a split at the first ')', which would put half the parameter list
// into the return signature and exempt a type through the wrong seam.
func TestTheSignatureSplitBalancesItsParentheses(t *testing.T) {
	name, params, returns, ok := funcSignature("func (s *store) Wrap(f func(int) error, n int) (*Report, error) {")
	if !ok {
		t.Fatal("an exported method declaration was not recognized")
	}
	if name != "Wrap" {
		t.Errorf("name %q, want Wrap", name)
	}
	if params != "f func(int) error, n int" {
		t.Errorf("parameters %q", params)
	}
	if strings.TrimSpace(returns) != "(*Report, error)" {
		t.Errorf("returns %q", returns)
	}
}

// VALIDATES: the checks run in the script's order, so the page a developer
// compares lists its findings in the same sequence.
// PREVENTS: a reordering that leaves the two halves reporting the same set in
// a different order, which no verdict comparison would catch.
func TestTheChecksRunInTheScriptsOrder(t *testing.T) {
	root := tree(t, map[string]string{
		"docs/a.md":                 "<!-- source: internal/a/x.go:1 -- line number -->\n<!-- source: internal/gone.go -- stale -->\n",
		"internal/a/x.go":           "package a\n\nfunc Orphan() {}\n",
		"plan/spec-open.md":         "| Status | in-progress |\n### Acceptance Criteria\n| AC-1 | thing |  | a note |\n",
		"internal/plugins/x/cmd.go": "package x\nfunc init(){ MustRegisterRootHandler(\"show thing\", nil) }\n",
		"test/ui/a.ci":              "nothing\n",
	})
	got := messages(t, root, []string{"internal/a/x.go", "internal/plugins/x/cmd.go"})
	want := []string{"contains line number", "non-existent file", "no cross-package", "Demonstrated By", "no .ci test"}
	if len(got) != len(want) {
		t.Fatalf("findings %q, want %d", got, len(want))
	}
	for i, fragment := range want {
		if !strings.Contains(got[i], fragment) {
			t.Errorf("finding %d is %q, want it to carry %q", i, got[i], fragment)
		}
	}
}

// VALIDATES: a changed file that was DELETED contributes no symbol and no
// command, rather than stopping the run.
// PREVENTS: a deleted file being read as an error, which would red the gate for
// every session that removed a file.
func TestADeletedChangedFileContributesNothing(t *testing.T) {
	root := tree(t, map[string]string{"go.mod": "module example.com/m\n"})
	if got := messages(t, root, []string{"internal/a/gone.go", "internal/plugins/x/gone.go"}); len(got) != 0 {
		t.Fatalf("findings %q, want none", got)
	}
}

// VALIDATES: repository generation runs every declared native action and
// reports each failure instead of stopping at the first stale artifact.
// PREVENTS: one broken generator hiding the remaining stale artifacts.
func TestGenerationRunsEveryAction(t *testing.T) {
	calls := make([]string, 0, 3)
	actions := []generationAction{
		{area: "a", verb: "one", answer: func(args []string) (any, int) {
			calls = append(calls, args[0])
			return nil, 0
		}},
		{area: "b", verb: "two", answer: func(args []string) (any, int) {
			calls = append(calls, args[0])
			return nil, 2
		}},
		{area: "c", verb: "three", answer: func(args []string) (any, int) {
			calls = append(calls, args[0])
			return nil, 0
		}},
	}

	payload, code := runGeneration("check", actions)
	if code != 1 {
		t.Fatalf("runGeneration exited %d, want 1", code)
	}
	if !slices.Equal(calls, []string{"one", "two", "three"}) {
		t.Fatalf("calls = %v", calls)
	}
	report, ok := payload.(generationReport)
	if !ok {
		t.Fatalf("runGeneration payload = %T, want generationReport", payload)
	}
	if len(report.Steps) != 3 || report.Steps[1].Code != 2 {
		t.Fatalf("report = %#v", report)
	}
}
