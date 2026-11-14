// VALIDATES: spec-le-is-a-ze-binary AC-5 and AC-11 -- the split, the manifest
// and the render are called as functions, and the round trip over a rule is
// byte-identical.
// PREVENTS: a split that loses an instruction. Every refusal here names a way
// for a point to stop being rendered with nothing going red, and the rendered
// rule is what every agent reads: a lossy split deletes an instruction from the
// corpus with every gate green.

package airules

import (
	"maps"
	"os"
	"path"
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
	split, err := splitRule(oneRule, "sample")
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
		_, err := splitRule(tc.text, "sample")
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
	split, err := splitRule(tight, "sample")
	if err != nil {
		t.Fatalf("SplitRule: %v", err)
	}
	if len(split.Sections) != 2 || !split.Sections[1].Tight {
		t.Fatalf("the second section is not marked tight: %+v", split.Sections)
	}
	if got := RenderText(split.Header, split.Sections); got != tight {
		t.Errorf("the render is\n%q\nwant\n%q", got, tight)
	}
	if !strings.Contains(formatManifest(split), "\n^two ## Two\n") {
		t.Errorf("the manifest does not carry the tight mark: %s", formatManifest(split))
	}
}

func TestAFencedBlockKeepsItsBlankLines(t *testing.T) {
	// The corpus carries blank lines inside fences. A walker without fence
	// state would cut the fence in half and make each half its own point.
	fenced := "# T\n\n**When:** when x\n**Severity:** blocking\n\n## S\n\n```\nfirst\n\nsecond\n```\n"
	split, err := splitRule(fenced, "sample")
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
	text := formatPoint(point)
	back, err := parsePoint(text, "p")
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
	bare := formatPoint(Point{Slug: "p", Kind: kindNote, Body: []string{"x"}})
	if strings.Contains(bare, "rationale:") || strings.Contains(bare, exceptedBy) {
		t.Errorf("an unlinked point wrote a link line: %s", bare)
	}
	if !strings.Contains(bare, "\nlevel:\n") || !strings.Contains(bare, "\nstage:\n") {
		t.Errorf("a derived field was not written empty: %s", bare)
	}
}

func TestAppliesRoundTripsThroughThePointFile(t *testing.T) {
	// Goal: `applies` is a header field like rationale: written only when it
	// carries a value, read back unchanged, never part of the body.
	point := Point{Slug: "p", Kind: kindDirective, Level: levelMust,
		Body: []string{"- **MUST do it.**"}, Applies: "preparing a commit"}
	text := formatPoint(point)
	back, err := parsePoint(text, "p")
	if err != nil {
		t.Fatalf("ParsePoint: %v", err)
	}
	if back.Applies != point.Applies {
		t.Errorf("applies changed: %q", back.Applies)
	}
	if strings.Join(back.Body, "\n") != "- **MUST do it.**" {
		t.Errorf("the body changed: %v", back.Body)
	}
	if bare := formatPoint(Point{Slug: "p", Kind: kindNote, Body: []string{"x"}}); strings.Contains(bare, keyApplies) {
		t.Errorf("a point with no situation wrote an applies line: %s", bare)
	}
}

func TestAnEmptyAppliesLineIsRefused(t *testing.T) {
	// Goal: an `applies:` line with no situation is refused, because it claims
	// the point was scoped and scopes nothing.
	_, err := parsePoint("---\nkind: directive\nlevel: MUST\napplies:  \n---\n- **MUST x.**\n", "p")
	if err == nil {
		t.Fatalf("an empty applies line was accepted")
	}
	if !strings.Contains(err.Error(), "names no situation") || !strings.Contains(err.Error(), "Fix:") {
		t.Errorf("the refusal does not say what is wrong and how to fix it: %v", err)
	}
}

func TestAppliesIsRefusedOnAProhibitionAndOutsideTheCore(t *testing.T) {
	// Goal: AC-7. `applies` on a point that states a prohibition, by its label
	// or by its body, or on a point of a rule that is not always-on, is
	// refused, naming the point and the reason. Method: the same field is
	// accepted on a MUST point of a core member, so each refusal is tied to its
	// condition and not to the field. A body holding a MUST and a MUST NOT may
	// carry either label, so the MUST-labeled one proves the body decides.
	for _, test := range []struct {
		name, level, body, keyword string
	}{
		{"labeled", "MUST NOT", "- **MUST NOT push.**", "MUST NOT"},
		{"must-label-must-not-body", "MUST", "- **MUST push by SHA, and MUST NOT push a branch.**", "MUST NOT"},
		{"shall-not-body", "MUST", "- **MUST push by SHA.** It SHALL NOT force.", "SHALL NOT"},
	} {
		_, err := parsePoint("---\nkind: directive\nlevel: "+test.level+"\napplies: pushing\n---\n"+test.body+"\n", "never-push")
		if err == nil {
			t.Fatalf("%s: applies on a prohibition was accepted", test.name)
		}
		for _, want := range []string{"never-push", "states the prohibition " + test.keyword, "split the point", "Fix: "} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the refusal does not say %q: %v", test.name, want, err)
			}
		}
	}
	// A prohibition the body quotes in an RFC citation is read as stated: no
	// check holds the quote to an RFC's text, so the citation form could carry
	// any prohibition out of the always-on core (review round 3, N4).
	for _, body := range []string{
		"- **MUST push by SHA.** RFC 4271 Section 6.3: \"The speaker MUST NOT force it.\"",
		"- **MUST push by SHA.** RFC 1 Section 1: \"Agents MUST NOT push.\"",
	} {
		if _, err := parsePoint("---\nkind: directive\nlevel: MUST\napplies: pushing\n---\n"+body+"\n", "quoted"); err == nil ||
			!strings.Contains(err.Error(), "states the prohibition MUST NOT") {
			t.Errorf("a MUST NOT quoted in an RFC citation kept its applies line: %v\n%s", err, body)
		}
	}
	accepted, err := parsePoint("---\nkind: directive\nlevel: MUST\napplies: pushing\n---\n- **MUST push by SHA.**\n", "push-by-sha")
	if err != nil || accepted.Applies != "pushing" {
		t.Fatalf("applies on a MUST point was refused or lost: %v %q", err, accepted.Applies)
	}

	routed := digestCorpus()
	routed["ai/rules/points/completion/manifest.md"] = "---\ntitle: Completion\n" +
		"when: before claiming any gokrazy qdisc work done\nseverity: blocking\n---\n" +
		"directives ## Directives\n  finish\n"
	routed["ai/rules/points/completion/directives/finish.md"] =
		"---\nkind: directive\nlevel: MUST\napplies: closing a spec\n---\n- MUST finish it.\n"
	routedTree := digestTree(t, routed)
	_, err = Digest(routedTree, false)
	if err == nil {
		t.Fatalf("applies on a routed rule's point was accepted")
	}
	for _, want := range []string{"completion/directives/finish", "not an always-on rule", "Fix: remove the `applies` line"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
	// `check metadata` refuses the same point file, through the same walk, so
	// no route accepts what another refuses.
	lint, err := Lint(routedTree)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if !lint.Failed() || !strings.Contains(lint.Text(), "not an always-on rule") {
		t.Errorf("check metadata accepted applies on a routed rule's point: %+v", lint)
	}
	if err := refuseRoutedApplies(routedTree, rulesDir(routedTree), pointsDir(routedTree)); err == nil {
		t.Errorf("the refusal render and check metadata share accepted applies on a routed rule's point")
	}
	if _, err := Digest(digestTree(t, situatedCorpus("walking a requirement list")), false); err != nil {
		t.Errorf("applies on a core member's MUST point was refused: %v", err)
	}
}

func TestAppliesIsRefusedOnEveryProhibitionForm(t *testing.T) {
	// Goal: every prohibition form the rule format allows keeps its point in the
	// always-on core, in every point kind and inside a code span, a blockquote
	// or a fence. Method: each refused row is paired with an accepted control
	// that differs only in the condition the refusal turns on (the positive
	// keyword, the word quoted or inflected, "none" in another sentence), so a
	// refusal that fired on every point turns a control red. A prohibition
	// quoted in an RFC citation is refused like any other.
	point := func(kind, level, body string) string {
		return "---\nkind: " + kind + "\nlevel: " + level + "\napplies: pushing\n---\n" + body + "\n"
	}
	for _, test := range []struct {
		name, refused, accepted, says string
	}{
		{"should-not-body", point(kindDirective, levelMust, "- **MUST push.** It SHOULD NOT force."),
			point(kindDirective, levelMust, "- **MUST push.** It SHOULD tag."), "SHOULD NOT"},
		{"not-recommended", point(kindDirective, levelShould, "- **Forcing is NOT RECOMMENDED.**"),
			point(kindDirective, levelShould, "- **Tagging is RECOMMENDED.**"), "NOT RECOMMENDED"},
		{"never", point(kindDirective, levelMust, "- **MUST push by SHA,** never a branch."),
			point(kindDirective, levelMust, "- **MUST push by SHA.** Nevertheless, push now."), "never"},
		{"never-capitalised", point(kindDirective, levelMust, "- **MUST push.** Never force."),
			point(kindDirective, levelMust, "- **MUST push.** Push now."), "Never"},
		{"is-forbidden", point(kindDirective, levelMust, "- **MUST push.** Forcing is forbidden."),
			point(kindDirective, levelMust, "- **MUST push.** Forbiddance names nothing here."), "forbidden"},
		{"none-must", point(kindDirective, levelMust, "- **MUST push,** and none of them MUST wait."),
			point(kindDirective, levelMust, "- **MUST push.** None remain. Each MUST be tagged."), "none ... MUST"},
		{"code-span", point(kindDirective, levelMust, "- **MUST push.** Read it as `MUST NOT force`."),
			point(kindDirective, levelMust, "- **MUST push.** Read it as `MUST tag`."), "MUST NOT"},
		{"blockquote", point(kindNote, "", "> Agents MUST NOT force."),
			point(kindNote, "", "> Agents MUST tag."), "MUST NOT"},
		{"fence", point(kindFence, "", "```\nagents MUST NOT force\n```"),
			point(kindFence, "", "```\nagents MUST tag\n```"), "MUST NOT"},
		{"note", point(kindNote, "", "Agents MUST NOT force a push."),
			point(kindNote, "", "Agents MUST tag a push."), "MUST NOT"},
		{"table", point(kindTable, "", "| Case | Rule |\n|---|---|\n| push | MUST NOT force |"),
			point(kindTable, "", "| Case | Rule |\n|---|---|\n| push | MUST tag |"), "MUST NOT"},
		{"rfc-quote", point(kindDirective, levelMust, "- **MUST push.** RFC 4271 Section 6.3: \"The peer MUST NOT force it.\""),
			point(kindDirective, levelMust, "- **MUST push.** RFC 4271 Section 6.3: \"The peer MUST tag it.\""), "MUST NOT"},
		{"never-in-quotes", point(kindDirective, levelMust, "- **MUST push by SHA;** the rule is \"never push a branch\"."),
			point(kindDirective, levelMust, "- **MUST push by SHA;** the rule is \"push by SHA\"."), "never"},
		{"quotes-paired-across-unquoted-text", point(kindDirective, levelMust, "- **MUST push.** The \"rule\" holds, then\" never \"again."),
			point(kindDirective, levelMust, "- **MUST push.** The \"rule\" holds, then\" now \"again."), "never"},
		{"do-not", point(kindDirective, levelMust, "- **MUST push.** Do not force."),
			point(kindDirective, levelMust, "- **MUST push.** Do nothing else."), "Do not"},
		{"dont", point(kindDirective, levelMust, "- **MUST push.** Don't force."),
			point(kindDirective, levelMust, "- **MUST push.** The donor tags it."), "Don't"},
		{"lowercase-must-not", point(kindDirective, levelMust, "- **MUST push.** Agents must not force."),
			point(kindDirective, levelMust, "- **MUST push.** Agents must tag."), "must not"},
		{"lowercase-should-not", point(kindNote, "", "Agents should not force a push."),
			point(kindNote, "", "Agents should tag a push."), "should not"},
	} {
		_, err := parsePoint(test.refused, "p")
		if err == nil {
			t.Errorf("%s: applies on a prohibition was accepted:\n%s", test.name, test.refused)
		} else if !strings.Contains(err.Error(), "states the prohibition "+test.says) {
			t.Errorf("%s: the refusal does not name %q: %v", test.name, test.says, err)
		}
		if _, err := parsePoint(test.accepted, "p"); err != nil {
			t.Errorf("%s: the control was refused, so the refusal is not tied to its condition: %v", test.name, err)
		}
	}
}

func TestRenderRefusesAppliesOnARoutedRule(t *testing.T) {
	// Goal: `./le ai rules update render` (RenderAll) refuses `applies` on a
	// point of a rule that is not always-on, through refuseRoutedApplies, as
	// Digest and check metadata do. Method: RenderAll refuses a rule with no
	// point directory before it reaches the refusal, so every fixture rule is
	// carried as points: completion's by hand, the other three split from
	// their own text. The routed corpus then reports the refusal and nothing
	// else, in the refusal's own words, and the same corpus without the field
	// reports no failure. Deleting the refuseRoutedApplies call from RenderAll
	// leaves the routed corpus with no failure, so the first check goes red.
	routed := digestCorpus()
	routed["ai/rules/points/completion/manifest.md"] = "---\ntitle: Completion\n" +
		"when: before claiming any gokrazy qdisc work done\nseverity: blocking\n---\n" +
		"directives ## Directives\n  finish\n"
	finish := "ai/rules/points/completion/directives/finish.md"
	routed[finish] = "---\nkind: directive\nlevel: MUST\napplies: closing a spec\n---\n- MUST finish it.\n"
	tree := pointedTree(t, routed)
	report, err := RenderAll(tree, rulesDir(tree), pointsDir(tree), false)
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	refusal := "completion/directives/finish: declares `applies`, and completion is not an always-on rule;"
	fix := "Fix: remove the `applies` line from the point file"
	if len(report.Failures) != 1 || !strings.HasPrefix(report.Failures[0], refusal) || !strings.HasSuffix(report.Failures[0], fix) {
		t.Errorf("render did not refuse applies on a routed rule's point, and only that: %+v", report.Failures)
	}
	routed[finish] = "---\nkind: directive\nlevel: MUST\n---\n- MUST finish it.\n"
	clean := pointedTree(t, routed)
	report, err = RenderAll(clean, rulesDir(clean), pointsDir(clean), false)
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	if len(report.Failures) != 0 {
		t.Errorf("render failed a corpus with no applies line: %+v", report.Failures)
	}
}

// pointedTree writes files as a fixture checkout, then splits each rule under
// ai/rules/ that files carries no points for into its point directory, so
// RenderAll finds every rule generated from points. The split of a rule
// renders back byte-identical (RoundTrip), so the render rewrites no rule.
func pointedTree(t *testing.T, files map[string]string) string {
	t.Helper()

	tree := digestTree(t, files)
	for rel, text := range files {
		if path.Dir(rel) != "ai/rules" || path.Ext(rel) != ".md" {
			continue
		}
		stem := strings.TrimSuffix(path.Base(rel), ".md")
		if _, carried := files["ai/rules/points/"+stem+"/manifest.md"]; carried {
			continue
		}
		split, err := splitRule(text, stem)
		if err != nil {
			t.Fatalf("splitting fixture rule %s: %v", rel, err)
		}
		if err := writeSplit(split, pointsDir(tree)); err != nil {
			t.Fatalf("writing fixture points for %s: %v", rel, err)
		}
	}
	return tree
}

func TestAPointBodyMayOpenWithTheDelimiter(t *testing.T) {
	// The header ends at the first delimiter AFTER line 1, so a body whose own
	// first line is the delimiter round-trips.
	point := Point{Slug: "p", Kind: kindNote, Body: []string{"---", "after"}}
	back, err := parsePoint(formatPoint(point), "p")
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
		_, err := parsePoint(tc.text, "p")
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
	_, _, err := parseManifest("---\ntitle: T\nwhen: when x\nseverity: blocking\n---\nnot a shape\n", "s")
	if err == nil || !strings.Contains(err.Error(), "is neither a section line") {
		t.Fatalf("ParseManifest said %v", err)
	}
	_, _, err = parseManifest("---\ntitle: T\nwhen: when x\nseverity: blocking\n---\n  orphan\n", "s")
	if err == nil || !strings.Contains(err.Error(), "comes before any section line") {
		t.Fatalf("ParseManifest said %v", err)
	}
	_, _, err = parseManifest("---\ntitle: T\nwhen: when x\n---\ns ## S\n  p\n", "s")
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
	if _, err := renderDir(build(t, good)); err != nil {
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
		_, err := renderDir(build(t, files))
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
	_, err := renderDir(ruleDir)
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
	_, err := renderDir(ruleDir)
	if err == nil || !strings.Contains(err.Error(), "sit(s) inside a point BODY") {
		t.Fatalf("RenderDir said %v", err)
	}
}

func TestWriteSplitRefusesToDeleteAnAuthorsFile(t *testing.T) {
	// Deleting would destroy an author's file on a slug or a section rename, so
	// reporting is the only safe answer.
	split, err := splitRule(oneRule, "sample")
	if err != nil {
		t.Fatalf("SplitRule: %v", err)
	}
	out := t.TempDir()
	if err := writeSplit(split, out); err != nil {
		t.Fatalf("WriteSplit: %v", err)
	}
	stray := filepath.Join(out, "sample", "first-section", "stray.md")
	if err := os.WriteFile(stray, []byte("mine\n"), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	err = writeSplit(split, out)
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
