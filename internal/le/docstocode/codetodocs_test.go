// A reader searches the reverse index when the reader edits code. Its two
// checks find claims that nobody can verify. These cases exercise the anchor
// grammar, declaration scan, severity rules, and fail-closed unreadable-file
// path.

package docstocode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestAnchorSegmentsReadsBothFormats(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []Segment
	}{
		{
			"one path and its description",
			"internal/a/a.go -- Run",
			[]Segment{{Paths: []string{"internal/a/a.go"}, Description: "Run"}},
		},
		{
			"a comma-separated name is relative to the last directory",
			"internal/a/a.go, b.go -- Run",
			[]Segment{{Paths: []string{"internal/a/a.go", "internal/a/b.go"}, Description: "Run"}},
		},
		{
			"semicolons separate segments, each with its own description",
			"internal/a/a.go -- Run; cmd/x/main.go -- Main",
			[]Segment{
				{Paths: []string{"internal/a/a.go"}, Description: "Run"},
				{Paths: []string{"cmd/x/main.go"}, Description: "Main"},
			},
		},
		{
			"an em dash separates as readily as two hyphens",
			"internal/a/a.go — Run",
			[]Segment{{Paths: []string{"internal/a/a.go"}, Description: "Run"}},
		},
		{
			"a token under no known root and no directory before it is dropped",
			"nowhere.go -- Ghost",
			nil,
		},
		{
			"a path with no description carries none",
			"internal/a/a.go",
			[]Segment{{Paths: []string{"internal/a/a.go"}, Description: ""}},
		},
	}
	for _, tc := range cases {
		got := anchorSegments(tc.content)
		if len(got) != len(tc.want) {
			t.Errorf("%s: %d segment(s), want %d: %+v", tc.name, len(got), len(tc.want), got)
			continue
		}
		for i := range got {
			if !slices.Equal(got[i].Paths, tc.want[i].Paths) || got[i].Description != tc.want[i].Description {
				t.Errorf("%s: segment %d is %+v, want %+v", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}

func TestAnchorSymbolTokensKeepsOnlyDeclarationClaims(t *testing.T) {
	got := anchorSymbolTokens("Run(), Peer.Name, the wire format, ze-gok, routing, sa_count, StateIdle..StateEstablished")
	want := []string{"Run", "Peer.Name", "sa_count"}
	if !slices.Equal(got, want) {
		t.Errorf("anchorSymbolTokens answered %v, want %v", got, want)
	}
}

const declarationFixture = `package a

func Run() {}

func (p *Peer) Send() {}

type Peer struct {
	Name string

	Age int
}

type Reader interface {
	Read() error
}

const (
	First = 1
	Second, Third = 2, 3
)

var Live = true

func hidden() { Untouched := 1; _ = Untouched }
`

func TestGoDeclarationsReadsDeclarationsAndNeverABody(t *testing.T) {
	decls := goDeclarations(declarationFixture)

	for _, name := range []string{"Run", "Send", "Peer", "Name", "Age", "Reader", "Read", "First", "Second", "Third", "Live", "hidden"} {
		if !decls.Names[name] {
			t.Errorf("%q is declared and was not found", name)
		}
	}
	for _, dotted := range []string{"Peer.Send", "Peer.Name", "Peer.Age", "Reader.Read"} {
		if !decls.Dotted[dotted] {
			t.Errorf("%q is declared and was not found", dotted)
		}
	}
	// A blank line inside a struct body does not open or close that body.
	// Treating it as a top-level line would drop every later member.
	if !decls.Dotted["Peer.Age"] {
		t.Error("a member after a blank line was dropped")
	}
	if decls.Names["Untouched"] {
		t.Error("a local inside a function body was read as a declaration")
	}
}

func TestClaimIsDeclaredNeverResolvesOnThePrefix(t *testing.T) {
	names := map[string]bool{"Register": false, "Run": true}
	dotted := map[string]bool{"Peer.Send": true}

	cases := []struct {
		claim string
		want  bool
	}{
		{"Run", true},
		{"Peer.Send", true},
		{"events.Register", false},
		{"Missing", false},
	}
	for _, tc := range cases {
		if got := claimIsDeclared(tc.claim, names, dotted); got != tc.want {
			t.Errorf("claimIsDeclared(%q) = %v, want %v", tc.claim, got, tc.want)
		}
	}
}

func TestClaimAppearsAsWordDemotesAMentionAndNotAPrefix(t *testing.T) {
	texts := []string{"func Run() { p.Send(); events.Register(x) }\n"}

	for _, claim := range []string{"Send", "events.Register"} {
		if !claimAppearsAsWord(claim, texts) {
			t.Errorf("%q is in the text and was not found", claim)
		}
	}
	for _, claim := range []string{"Sen", "Register.Extra"} {
		if claimAppearsAsWord(claim, texts) {
			t.Errorf("%q is not a whole word of the text and was found", claim)
		}
	}
}

// TestCheckAnchorsFailsClosedOnAFileItCannotRead exercises the fail-closed path.
// An unreadable anchored file is a finding. Its unresolved claims are UNKNOWN,
// not absent. Otherwise, "not declared" would itself be a false claim about
// unread text.
func TestCheckAnchorsFailsClosedOnAFileItCannotRead(t *testing.T) {
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("an unreadable file needs a mode this user cannot be exempt from")
	}

	root := t.TempDir()
	path := filepath.Join(root, "internal", "a", "a.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("package a\n"), 0o200); err != nil {
		t.Fatalf("fixture file: %v", err)
	}

	problems := checkAnchors(root, []Anchor{
		{Doc: "docs/one.md", Line: 4, Paths: []string{"internal/a/a.go"}, Descrip: "Nowhere"},
	})

	if len(problems) != 1 {
		t.Fatalf("an unreadable anchored file produced %d finding(s): %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "cannot read the anchored file") {
		t.Errorf("the finding does not say the file was unreadable: %q", problems[0])
	}
	if strings.Contains(problems[0], "not declared there") {
		t.Errorf("an unreadable file produced an absence claim: %q", problems[0])
	}
}

func TestPackageDirGroupsByDirectory(t *testing.T) {
	cases := map[string]string{
		"internal/a/a.go": "internal/a",
		"internal/a/":     "internal/a",
	}
	for path, want := range cases {
		if got := packageDir(path); got != want {
			t.Errorf("packageDir(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestTheStaleReportNamesThreeDocumentsAndCountsTheRest is the bound of the
// finding's rendering.
func TestTheStaleReportNamesThreeDocumentsAndCountsTheRest(t *testing.T) {
	report := CodeReport{File: CodeOutputRel, Paths: 1, Packages: 1}
	for i := range namedRefs + 2 {
		report.Stale = append(report.Stale, StaleRef{Path: "internal/gone/gone.go", Doc: "docs/one.md", Line: i + 1})
	}

	text := report.Text()
	if !strings.Contains(text, "  MISSING: internal/gone/gone.go") {
		t.Errorf("the stale path is not named:\n%s", text)
	}
	if got := strings.Count(text, "           <- "); got != namedRefs {
		t.Errorf("the report names %d documents, want %d", got, namedRefs)
	}
	if !strings.Contains(text, "... and 2 more") {
		t.Errorf("the documents past the bound were not counted:\n%s", text)
	}
}

// TestTheRenderingSwitchesToATableAtFourFiles is the other bound: a table of
// two rows costs more to read than it saves.
func TestTheRenderingSwitchesToATableAtFourFiles(t *testing.T) {
	index := codeIndex{Refs: map[string][]Ref{}}
	for i := range namedInline {
		index.Refs[filePath(i)] = []Ref{{Doc: "docs/one.md", Line: 1}}
	}
	if body := renderCodeIndex(index); strings.Contains(body, "| File | Docs |") {
		t.Errorf("%d files rendered as a table:\n%s", namedInline, body)
	}

	index.Refs[filePath(namedInline)] = []Ref{{Doc: "docs/one.md", Line: 1}}
	body := renderCodeIndex(index)
	if !strings.Contains(body, "| File | Docs |") {
		t.Errorf("%d files did not render as a table:\n%s", namedInline+1, body)
	}
	if !strings.Contains(body, "Files: 4 | Docs: `docs/one.md`") { // <!-- doc-links: ignore (a synthetic path in a test fixture; these tests are about dead-path detection, so the paths must not resolve) -->
		t.Errorf("the table's header does not carry the counts:\n%s", body)
	}
}

// filePath answers the nth distinct file of one package.
func filePath(n int) string {
	return "internal/a/" + string(rune('a'+n)) + ".go"
}

func TestTheReverseIndexIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	raw, err := json.Marshal(CodeReport{
		File: CodeOutputRel, Paths: 1, Packages: 1,
		Stale:  []StaleRef{{Path: "internal/gone/gone.go", Doc: "docs/one.md", Line: 3}},
		Claims: []string{"docs/one.md:3: source anchor internal/a/a.go names 'X', which is not declared there"},
	})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, key := range []string{`"file"`, `"paths"`, `"packages"`, `"stale"`, `"claims"`, `"written"`, `"doc"`, `"line"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
}

// TestCheckAnchorsReadsAPathThatCarriesALineNumber pins the suffix removal,
// which the command cannot reach.
//
// The stale-reference check does NOT remove the suffix. Thus, it reports
// `file.go:12` as a missing path and exits before it prints a claim. This port
// preserves the SCRIPT behavior. The strip remains the function's contract, so
// this test drives it directly.
func TestCheckAnchorsReadsAPathThatCarriesALineNumber(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "a", "a.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("package a\n\nfunc Run() {}\n"), 0o600); err != nil {
		t.Fatalf("fixture file: %v", err)
	}

	problems := checkAnchors(root, []Anchor{
		{Doc: "docs/one.md", Line: 3, Paths: []string{"internal/a/a.go:12"}, Descrip: "Nowhere"},
	})
	if len(problems) != 1 {
		t.Fatalf("an anchor naming a line produced %d finding(s): %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "'Nowhere'") {
		t.Errorf("the finding does not name the claim: %q", problems[0])
	}

	declared := checkAnchors(root, []Anchor{
		{Doc: "docs/one.md", Line: 3, Paths: []string{"internal/a/a.go:12"}, Descrip: "Run"},
	})
	if len(declared) != 0 {
		t.Errorf("a declared symbol behind a line number was reported: %v", declared)
	}
}
