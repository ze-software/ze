// VALIDATES: spec-le-is-a-ze-binary AC-5 and AC-11 -- the split, the manifest
// and the render are called as functions, and the round trip over a rule is
// byte-identical.
// PREVENTS: a split that loses an instruction. Every refusal here names a way
// for a point to stop being rendered with nothing going red, and the rendered
// rule is what every agent reads: a lossy split deletes an instruction from the
// corpus with every gate green.

package rules

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// oneRule is a rendered rule holding one section and two points.
const oneRule = `# A Title

**When:** when something happens
**Severity:** blocking

## First Section

- **MUST do the thing.**

| a | b |
|---|---|
| 1 | 2 |
`

func TestASplitPartitionsEveryLine(t *testing.T) {
	split, err := SplitRule(oneRule, "sample")
	if err != nil {
		t.Fatalf("SplitRule: %v", err)
	}
	if len(split.Sections) != 1 {
		t.Fatalf("SplitRule found %d sections, want 1", len(split.Sections))
	}
	section := split.Sections[0]
	if section.Slug != "first-section" || section.Heading != "## First Section" {
		t.Errorf("the section is %q / %q", section.Slug, section.Heading)
	}
	if len(section.Points) != 2 {
		t.Fatalf("the section holds %d points, want 2", len(section.Points))
	}
	if section.Points[0].Kind != kindDirective || section.Points[0].Level != levelMust {
		t.Errorf("the first point is %s / %s", section.Points[0].Kind, section.Points[0].Level)
	}
	if section.Points[1].Kind != kindTable {
		t.Errorf("the second point is %s, want a table", section.Points[1].Kind)
	}
	want := []string{"sample/first-section/must-do-the-thing", "sample/first-section/a-b"}
	got := split.IDs()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the ids are %v, want %v", got, want)
	}
}

func TestASplitRefusesEveryShapeItCannotPutBack(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"no trailing newline", strings.TrimSuffix(oneRule, "\n"), "must end with a newline"},
		{"blank final line", oneRule + "\n", "must not end with a blank line"},
		{"no title", "not a title\n\n## S\n\n- **MUST x.**\n", "first line must be '# Title'"},
		{"no blank after title", "# T\n**When:** when x\n", "one blank line must follow the title"},
		{"metadata out of order", "# T\n\n**Severity:** blocking\n**When:** when x\n\n## S\n\n- **MUST x.**\n",
			"metadata must be When, Severity, then optional Related"},
		{"two blank lines", "# T\n\n**When:** when x\n**Severity:** blocking\n\n\n## S\n\n- **MUST x.**\n",
			"blank lines, not 1"},
		{"point before a section", "# T\n\n**When:** when x\n**Severity:** blocking\n\n- **MUST x.**\n",
			"comes before the first `##` section"},
		{"no section at all", "# T\n\n**When:** when x\n**Severity:** blocking\n", "one blank line must follow the metadata"},
		{"section carries a body", "# T\n\n**When:** when x\n**Severity:** blocking\n\n## S\n- **MUST x.**\n",
			"must stand alone"},
	}
	for _, tc := range cases {
		_, err := SplitRule(tc.text, "sample")
		if err == nil {
			t.Errorf("%s: SplitRule accepted it", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: SplitRule said %q, want it to mention %q", tc.name, err, tc.want)
		}
	}
}

func TestThePartitionCheckRefusesALineOwnedTwiceOrByNothing(t *testing.T) {
	// A round trip alone cannot detect either form. A dropped line and an
	// equivalent rendered replacement still compare equal. This direct test
	// checks the partition property instead of its visible result.
	lines := []string{"# T", "", "**When:** when x", "**Severity:** blocking", "",
		"## S", "", "- **MUST a.**", "", "- **MUST b.**"}
	good := Split{Stem: "sample", HeaderStart: 0, HeaderEnd: 4, Sections: []Section{{
		Slug: "s", Heading: "## S", Start: 5,
		Points: []Point{
			{Slug: "a", Start: 7, End: 8},
			{Slug: "b", Start: 9, End: 10},
		},
	}}}
	if err := verifyPartition(lines, good); err != nil {
		t.Fatalf("verifyPartition refused a total partition: %v", err)
	}

	overlapping := good
	overlapping.Sections = []Section{{
		Slug: "s", Heading: "## S", Start: 5,
		Points: []Point{{Slug: "a", Start: 7, End: 10}, {Slug: "b", Start: 9, End: 10}},
	}}
	err := verifyPartition(lines, overlapping)
	if err == nil || !strings.Contains(err.Error(), "claimed by a and b") {
		t.Errorf("an overlapping claim was accepted: %v", err)
	}

	dropped := good
	dropped.Sections = []Section{{
		Slug: "s", Heading: "## S", Start: 5,
		Points: []Point{{Slug: "a", Start: 7, End: 8}},
	}}
	err = verifyPartition(lines, dropped)
	if err == nil || !strings.Contains(err.Error(), "is non-blank and belongs to no point") {
		t.Errorf("a dropped line was accepted: %v", err)
	}
}

func TestATightHeadingKeepsItsMissingBlankLine(t *testing.T) {
	// One rule in the corpus carries a `##` heading with no blank line above
	// it. The renderer would otherwise insert one, and the rendered bytes would
	// change for a rule nobody edited.
	tight := "# T\n\n**When:** when x\n**Severity:** blocking\n\n## One\n\n- **MUST a.**\n## Two\n\n- **MUST b.**\n"
	split, err := SplitRule(tight, "sample")
	if err != nil {
		t.Fatalf("SplitRule: %v", err)
	}
	if len(split.Sections) != 2 || !split.Sections[1].Tight {
		t.Fatalf("the second section is not marked tight: %+v", split.Sections)
	}
	if got := RenderText(split.Header, split.Sections); got != tight {
		t.Errorf("the render is\n%q\nwant\n%q", got, tight)
	}
	if !strings.Contains(FormatManifest(split), "\n^two ## Two\n") {
		t.Errorf("the manifest does not carry the tight mark: %s", FormatManifest(split))
	}
}

func TestAFencedBlockKeepsItsBlankLines(t *testing.T) {
	// The corpus carries blank lines inside fences. A walker without fence
	// state would cut the fence in half and make each half its own point.
	fenced := "# T\n\n**When:** when x\n**Severity:** blocking\n\n## S\n\n```\nfirst\n\nsecond\n```\n"
	split, err := SplitRule(fenced, "sample")
	if err != nil {
		t.Fatalf("SplitRule: %v", err)
	}
	points := split.Sections[0].Points
	if len(points) != 1 || points[0].Kind != kindFence {
		t.Fatalf("the fence was cut into %d points: %+v", len(points), points)
	}
	if got := RenderText(split.Header, split.Sections); got != fenced {
		t.Errorf("the render is\n%q\nwant\n%q", got, fenced)
	}
}

func TestASectionHeadingInsideAFenceIsNotASection(t *testing.T) {
	// A `##` inside a fenced report template is quoted output, not a section.
	lines := []string{"## Real", "```", "## Quoted", "```", "## Second"}
	got := sectionHeadings(lines)
	if strings.Join(got, ",") != "## Real,## Second" {
		t.Errorf("sectionHeadings = %v", got)
	}
}

func TestAPointFileRoundTripsThroughItsFrontmatter(t *testing.T) {
	point := Point{
		Slug: "p", Kind: kindDirective, Level: levelMust, Stage: "",
		Body:      []string{"- **MUST do it.**", "  and keep doing it."},
		Rationale: "ai/rationale/x.md", ExceptedBy: "other/section/slug",
	}
	text := FormatPoint(point)
	back, err := ParsePoint(text, "p")
	if err != nil {
		t.Fatalf("ParsePoint: %v", err)
	}
	if strings.Join(back.Body, "\n") != strings.Join(point.Body, "\n") {
		t.Errorf("the body changed: %v", back.Body)
	}
	if back.Rationale != point.Rationale || back.ExceptedBy != point.ExceptedBy {
		t.Errorf("the links changed: %q / %q", back.Rationale, back.ExceptedBy)
	}
	// An empty rationale is written as no line at all: the split cannot derive
	// one, so an empty line would say "examined and has none".
	bare := FormatPoint(Point{Slug: "p", Kind: kindNote, Body: []string{"x"}})
	if strings.Contains(bare, "rationale:") || strings.Contains(bare, exceptedBy) {
		t.Errorf("an unlinked point wrote a link line: %s", bare)
	}
	if !strings.Contains(bare, "\nlevel:\n") || !strings.Contains(bare, "\nstage:\n") {
		t.Errorf("a derived field was not written empty: %s", bare)
	}
}

func TestAPointBodyMayOpenWithTheDelimiter(t *testing.T) {
	// The header ends at the first delimiter AFTER line 1, so a body whose own
	// first line is the delimiter round-trips.
	point := Point{Slug: "p", Kind: kindNote, Body: []string{"---", "after"}}
	back, err := ParsePoint(FormatPoint(point), "p")
	if err != nil {
		t.Fatalf("ParsePoint: %v", err)
	}
	if strings.Join(back.Body, "\n") != "---\nafter" {
		t.Errorf("the body is %v", back.Body)
	}
}

func TestAPointFileRefusesWhatItCannotMean(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"no delimiter", "kind: note\n", "first line must be '---'"},
		{"unterminated", "---\nkind: note\n", "header is not terminated by '---'"},
		{"not key value", "---\nnonsense\n---\nx\n", "header line is not 'key: value'"},
		{"unknown field", "---\nkind: note\nwhat: x\n---\nbody\n", "unknown header field(s) ['what']"},
		{"bad kind", "---\nkind: nonsense\n---\nbody\n", "kind must be one of"},
		{"bad level", "---\nkind: note\nlevel: SHALL\n---\nbody\n", "level must be empty or one of"},
		{"empty body", "---\nkind: note\n---\n", "has an empty body"},
	}
	for _, tc := range cases {
		_, err := ParsePoint(tc.text, "p")
		if err == nil {
			t.Errorf("%s: ParsePoint accepted it", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: ParsePoint said %q, want it to mention %q", tc.name, err, tc.want)
		}
	}
}

func TestAManifestLineThatIsNeitherShapeIsRefused(t *testing.T) {
	// A skipped line is an instruction that stops being rendered with nothing
	// going red, which is the one failure this design exists to prevent.
	_, _, err := ParseManifest("---\ntitle: T\nwhen: when x\nseverity: blocking\n---\nnot a shape\n", "s")
	if err == nil || !strings.Contains(err.Error(), "is neither a section line") {
		t.Fatalf("ParseManifest said %v", err)
	}
	_, _, err = ParseManifest("---\ntitle: T\nwhen: when x\nseverity: blocking\n---\n  orphan\n", "s")
	if err == nil || !strings.Contains(err.Error(), "comes before any section line") {
		t.Fatalf("ParseManifest said %v", err)
	}
	_, _, err = ParseManifest("---\ntitle: T\nwhen: when x\n---\ns ## S\n  p\n", "s")
	if err == nil || !strings.Contains(err.Error(), "missing 'severity'") {
		t.Fatalf("ParseManifest said %v", err)
	}
}

func TestASlugIsCutAtAWordBoundary(t *testing.T) {
	long := []string{"- **" + strings.Repeat("alpha beta ", 12) + "**"}
	slug := slugify(long, kindDirective)
	if len(slug) > slugMax {
		t.Errorf("the slug is %d characters: %s", len(slug), slug)
	}
	if strings.HasSuffix(slug, "-") || strings.Contains(slug, "--") {
		t.Errorf("the slug was cut mid-separator: %s", slug)
	}
	// A block whose first line carries nothing sluggable falls back to its kind.
	if got := slugify([]string{"###"}, kindHeading); got != kindHeading {
		t.Errorf("an unsluggable heading became %q", got)
	}
}

func TestARenderRefusesAPointNothingWouldRead(t *testing.T) {
	build := func(t *testing.T, files map[string]string) string {
		t.Helper()
		root := t.TempDir()
		for rel, body := range files {
			path := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatalf("fixture: %v", err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("fixture: %v", err)
			}
		}
		return filepath.Join(root, "sample")
	}

	good := map[string]string{
		"sample/manifest.md": "---\ntitle: T\nwhen: when x\nseverity: blocking\n---\ns ## S\n  p\n",
		"sample/s/p.md":      "---\nkind: directive\nlevel: MUST\n---\n- **MUST x.**\n",
	}
	if _, err := RenderDir(build(t, good)); err != nil {
		t.Fatalf("RenderDir refused a clean tree: %v", err)
	}

	cases := []struct {
		name  string
		extra map[string]string
		want  string
	}{
		{"a loose point", map[string]string{"sample/loose.md": "x\n"}, "sit(s) directly in the rule directory"},
		{"a point one level deeper", map[string]string{"sample/s/deeper/q.md": "x\n"}, "sit(s) below its `##` section directory"},
		{"an unlisted section", map[string]string{"sample/other/q.md": "x\n"}, "the manifest does not list them"},
		{"an unlisted point", map[string]string{"sample/s/q.md": "---\nkind: note\n---\nx\n"}, "point file(s) ['q'] exist"},
	}
	for _, tc := range cases {
		files := map[string]string{}
		maps.Copy(files, good)
		maps.Copy(files, tc.extra)
		_, err := RenderDir(build(t, files))
		if err == nil {
			t.Errorf("%s: RenderDir accepted it", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: RenderDir said %q, want it to mention %q", tc.name, err, tc.want)
		}
	}
}

func TestARenderRefusesASlugThatIsAPath(t *testing.T) {
	// A manifest names directories and files the renderer opens, so a separator
	// or a parent reference would let it read outside its own rule directory.
	root := t.TempDir()
	ruleDir := filepath.Join(root, "sample")
	if err := os.MkdirAll(ruleDir, 0o750); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	manifest := "---\ntitle: T\nwhen: when x\nseverity: blocking\n---\n../escape ## S\n  p\n"
	if err := os.WriteFile(filepath.Join(ruleDir, manifestName), []byte(manifest), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	_, err := RenderDir(ruleDir)
	if err == nil || !strings.Contains(err.Error(), "must be a bare lowercase path component") {
		t.Fatalf("RenderDir said %v", err)
	}
}

func TestARenderRefusesAHeadingHidingInAPointBody(t *testing.T) {
	// A body that contains `##` renders identically but names no directory. Later
	// point ids would name a section that readers never see.
	root := t.TempDir()
	ruleDir := filepath.Join(root, "sample")
	if err := os.MkdirAll(filepath.Join(ruleDir, "s"), 0o750); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	files := map[string]string{
		manifestName: "---\ntitle: T\nwhen: when x\nseverity: blocking\n---\ns ## S\n  p\n",
		"s/p.md":     "---\nkind: note\n---\ntext\n## Hidden\nmore\n",
	}
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(ruleDir, filepath.FromSlash(rel)), []byte(body), 0o600); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	_, err := RenderDir(ruleDir)
	if err == nil || !strings.Contains(err.Error(), "sit(s) inside a point BODY") {
		t.Fatalf("RenderDir said %v", err)
	}
}

func TestWriteSplitRefusesToDeleteAnAuthorsFile(t *testing.T) {
	// Deleting would destroy an author's file on a slug or a section rename, so
	// reporting is the only safe answer.
	split, err := SplitRule(oneRule, "sample")
	if err != nil {
		t.Fatalf("SplitRule: %v", err)
	}
	out := t.TempDir()
	if err := WriteSplit(split, out); err != nil {
		t.Fatalf("WriteSplit: %v", err)
	}
	stray := filepath.Join(out, "sample", "first-section", "stray.md")
	if err := os.WriteFile(stray, []byte("mine\n"), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	err = WriteSplit(split, out)
	if err == nil || !strings.Contains(err.Error(), "which this split does not produce") {
		t.Fatalf("WriteSplit said %v", err)
	}
	if _, statErr := os.Stat(stray); statErr != nil {
		t.Errorf("WriteSplit deleted the author's file: %v", statErr)
	}
}

func TestOneRuleRoundTripsThroughTheFilesystem(t *testing.T) {
	dir := t.TempDir()
	rulesIn := filepath.Join(dir, "rules")
	if err := os.MkdirAll(rulesIn, 0o750); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesIn, "sample.md"), []byte(oneRule), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	report, err := RoundTrip(rulesIn, t.TempDir())
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if report.Failed() || report.Rules != 1 {
		t.Fatalf("RoundTrip reported %+v", report)
	}
	if got := report.Text(); got != "rules-points: all 1 rules round-trip byte-identical\n" {
		t.Errorf("the page is %q", got)
	}
}
