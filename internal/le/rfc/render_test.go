// VALIDATES: spec-le-is-a-ze-binary AC-5 and AC-11 -- the render half and the
// generator are called as functions, and every branch this checkout never
// reaches is driven from the entry point that owns it.
// PREVENTS: a render proven only by the bytes it produces over HEAD. HEAD is a
// green corpus: its worklist is never empty, every enrolled RFC has a public
// row, every summary is declared, no requirement is nightly-only, no verdict is
// stale, no summary fails to parse and no shard is an orphan. Every one of those
// is a branch of this code, and a comparison over HEAD alone exercises the
// opposite of each.

package rfc

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// fixtureMeta builds one fixture summary's `## Meta` table, which is where a
// summary declares its own enrolment and its own public row.
//
// Three arguments, because three facts are all a case here varies: how the
// summary is gated, why it is gated that way, and whether it renders a row on
// the public page. `support` is `<section> <rank>` for a summary that renders a
// row, and `supportNone` for one that renders none. The four authored cells of
// a row are the same wherever a row exists, because no case here asserts their
// text, so they are written once.
//
// A Meta table is one contiguous run of rows, so a case needing one more field
// -- a forward lineage row, for instance -- appends it to what this answers.
func fixtureMeta(enrolment, reason, support string) string {
	var tb textbuf.Buffer
	tb.Str("## Meta\n\n| Field | Value |\n|-------|-------|\n").
		Str("| Title | Widgets |\n").
		Str("| Enrolment | ").Str(enrolment).Str(" |\n").
		Str("| Enrolment reason | ").Str(reason).Str(" |\n").
		Str("| Support | ").Str(support).Str(" |\n")
	if support == supportNone {
		return tb.String()
	}
	return tb.Str("| Support area | Widgets |\n| Support status | Supported |\n").
		Str("| Support coverage | full |\n| Support remaining | - |\n").String()
}

// renderFixture is the smallest tree the two pages can be rendered over: one
// summary declaring one MUST-level row, whose own Meta table declares it
// enrolled and gives it a public row.
func renderFixture(t *testing.T, extra map[string]string) RenderInput {
	t.Helper()

	files := map[string]string{
		"rfc/short/rfc9999.md": "# RFC 9999\n\n" +
			fixtureMeta(enrolmentEnrolled,
				"the fixture RFC, gated so the render has a population", "bgp-base 10") +
			"\n- [ ] [RFC9999-2-1] [MUST] A speaker MUST answer (§2)\n",
		// The carrier table derives each interop tree's tier from whether a
		// SCHEDULED workflow names its runner, and an unreadable workflow
		// directory is refused rather than read as "nothing runs". Every
		// fixture therefore carries one, and a case that needs a schedule
		// replaces it.
		".github/workflows/ci.yml": "on: push\njobs:\n  a:\n    steps:\n      - run: ./le verify deps unit-cached\n",
	}
	maps.Copy(files, extra)
	tree := fixtureTree(t, files)
	collected, err := Collect(tree)
	if err != nil {
		t.Fatalf("collecting the fixture: %v", err)
	}
	if len(collected.ParseErrors) > 0 {
		t.Fatalf("the fixture summary did not parse: %v", collected.ParseErrors)
	}
	in, err := NewRenderInput(tree, collected, nil, nil)
	if err != nil {
		t.Fatalf("assembling the render input: %v", err)
	}
	return in
}

// renderedIndex answers the generated ledger body for one input.
func renderedIndex(t *testing.T, in RenderInput) string {
	t.Helper()

	body, err := RenderIndex(in)
	if err != nil {
		t.Fatalf("rendering the index: %v", err)
	}
	return body
}

func TestAnEmptyAuditWorklistSaysSoRatherThanRenderingAnEmptyTable(t *testing.T) {
	// HEAD's worklist is never empty, so this branch has no live example. It is
	// the one that reads a clean result as a WARNING, which is the judgement the
	// audit skill asks for and the opposite of what a bare "0 rows" would say.
	in := renderFixture(t, nil)
	body := renderedIndex(t, in)
	if !strings.Contains(body, "None: every recorded verdict is a fresh `enforced`") {
		t.Error("an empty worklist rendered no verdict at all")
	}
	if strings.Contains(body, "| Requirement | Verdict | Meaning |") {
		t.Error("an empty worklist still rendered its table header")
	}
}

func TestAnEnrolledRFCWithNoPublicRowIsNamedInTheBacklog(t *testing.T) {
	// The completeness ratchet grandfathers the rowless enrolments that predate
	// it, so they are invisible unless this table names them.
	in := renderFixture(t, map[string]string{
		"rfc/short/rfc9999.md": "# RFC 9999\n\n" +
			fixtureMeta(enrolmentEnrolled, "gated, and claiming no row on the public page",
				supportNone) +
			"\n- [ ] [RFC9999-2-1] [MUST] A speaker MUST answer (§2)\n",
	})
	body := renderedIndex(t, in)
	if !strings.Contains(body, "| `rfc9999` | yes |") {
		t.Error("the rowless enrolment was not named")
	}
	if !strings.Contains(body, "1 enrolled RFC(s) have no row") {
		t.Error("the count above the table is wrong")
	}
}

func TestEveryEnrolledRFCHavingARowRendersTheAbsenceOfDebtOutLoud(t *testing.T) {
	in := renderFixture(t, nil)
	body := renderedIndex(t, in)
	if !strings.Contains(body, "None: every enrolled RFC has a row.") {
		t.Error("a complete public page rendered no statement at all")
	}
	if !strings.Contains(body, "None: every summary is enrolled.") {
		t.Error("an empty disposition file rendered no statement at all")
	}
}

func TestADeclaredSummaryIsRenderedAsDebtUnlessItIsNonNormative(t *testing.T) {
	in := renderFixture(t, map[string]string{
		"rfc/short/rfc1000.md": "# RFC 1000\n\n" + fixtureMeta(dispositionNonNormative,
			"an Informational document with no RFC 2119 key-words machinery", supportNone),
		"rfc/short/rfc1001.md": "# RFC 1001\n\n" + fixtureMeta(dispositionBacklog,
			"nobody has walked it", supportNone),
	})
	body := renderedIndex(t, in)
	if !strings.Contains(body, "| `rfc1000` | non-normative | no |") {
		t.Error("a non-normative disposition was rendered as debt")
	}
	if !strings.Contains(body, "| `rfc1001` | backlog | **DEBT** |") {
		t.Error("a backlog disposition was not rendered as debt")
	}
}

func TestADispositionReasonCarryingAPipeCannotSplitItsRow(t *testing.T) {
	// An enrolment reason writes a grep alternation in this register, and the
	// reason now lives in a markdown cell at BOTH ends: the Meta table it is
	// declared in, and the rendered row it is published in. A pipe read as a
	// cell boundary at either end gives the row a column its header does not
	// have and drops the tail of the reason.
	//
	// So the reason escapes its pipe as `\|` in the Meta cell, and
	// splitMetaCells is what keeps the row whole: a reader cutting there would
	// take `B'` for the next field and shift every cell after it.
	const reason = `waits on grep 'A\|B'`
	cells := splitMetaCells(`| Enrolment reason | ` + reason + ` |`)
	if len(cells) != 2 || cells[1] != reason {
		t.Fatalf("the escaped pipe split the Meta row into %q", cells)
	}

	in := renderFixture(t, map[string]string{
		"rfc/short/rfc1000.md": "# RFC 1000\n\n" +
			fixtureMeta(dispositionBlocked, reason, supportNone),
	})
	body := renderedIndex(t, in)
	rendered := ""
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, "| `rfc1000` | "+dispositionBlocked+" |") {
			rendered = line
			break
		}
	}
	if rendered == "" {
		t.Fatalf("the disposition rendered no row at all:\n%s", body)
	}
	// Four cells, and the last of them still ends where the reason ends.
	if got := splitMetaCells(rendered); len(got) != 4 || !strings.HasSuffix(got[3], `B'`) {
		t.Errorf("the pipe split the rendered row into %q", got)
	}
}

func TestAnEnrollableRFCIsNamedSoTheNextOneToFinishIsAtTheTop(t *testing.T) {
	// Nothing in HEAD is enrollable: every summary with full coverage is already
	// enrolled. The branch exists so the operator is told which RFC costs
	// nothing to gate.
	in := renderFixture(t, map[string]string{
		"rfc/short/rfc9999.md": "# RFC 9999\n\n" +
			fixtureMeta(dispositionBacklog, "not gated yet", supportNone) +
			"\n- [ ] [RFC9999-2-1] [MUST] A speaker MUST answer (§2)\n",
	})
	body := renderedIndex(t, in)
	// The block renders only when something IS enrollable, so its absence is
	// the assertion here: a requirement with neither test nor annotation must
	// not be advertised as free to gate.
	if strings.Contains(body, "**Enrollable now**") {
		t.Errorf("a requirement with neither test nor annotation was called enrollable:\n%s", body)
	}

	covered := renderFixture(t, map[string]string{
		"rfc/short/rfc9999.md": "# RFC 9999\n\n" +
			fixtureMeta(dispositionBacklog, "not gated yet", supportNone) +
			"\n- [ ] [RFC9999-2-1] [MUST] A speaker MUST answer (§2) {gap: nothing sends it}\n",
	})
	body = renderedIndex(t, covered)
	if !strings.Contains(body, "**Enrollable now** (1)") {
		t.Errorf("an annotated requirement did not make its RFC enrollable:\n%s", body)
	}
	if !strings.Contains(body, "| `rfc9999` | 1 | 0 | 0 | 1 | 0 | 0 | 0 | enrollable |") {
		t.Errorf("the rollup row is wrong:\n%s", body)
	}
}

func TestAVerdictInAnUnpublishedStateSaysSoRatherThanRenderingAnEmptyCell(t *testing.T) {
	// The vocabulary can grow, and an unexplained verdict in a worklist is a row
	// a reader silently skips. Both halves of the fail-closed answer are here:
	// an unknown STATE and an unknown VERDICT WORD.
	cases := []struct{ name, reason, want string }{
		{"a known verdict", "weak", "cannot fail on non-compliance"},
		{"a known state", "weak (shifted)", "re-stamp it with `./le rfc reseal`"},
		{"an unknown state", "weak (invented)",
			"unpublished freshness state `invented` -- add it to _STATE_MEANING"},
		{"an unknown verdict", "invented", "outside the recorded vocabulary"},
		{"a fresh enforced is not a worklist row", "enforced", "outside the recorded vocabulary"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			if got := verdictMeaning(one.reason); !strings.Contains(got, one.want) {
				t.Errorf("%q means %q, want it to say %q", one.reason, got, one.want)
			}
		})
	}
}

func TestARequirementProvenOnlyByNightlyEvidenceIsMarkedOnItsOwnRow(t *testing.T) {
	// Nothing in HEAD is nightly-only, so the marker never renders there. It is
	// the distinction between "a tag exists" and "a merge blocks on it".
	in := renderFixture(t, map[string]string{
		"internal/le/interoplab/bgp/scenario_test.go": "// " + rfcTagMarker + " RFC9999-2-1 positive\n" +
			"// " + rfcTagMarker + " RFC9999-2-1 negative\n",
		".github/workflows/nightly.yml": "on:\n  schedule:\n    - cron: '0 3 * * *'\n" +
			"jobs:\n  a:\n    steps:\n      - run: ./le integration interop\n",
	})
	shard := RenderShards(in)["rfc9999"]
	if !strings.Contains(shard, "**nightly-only**") {
		t.Errorf("interop-only evidence was not marked:\n%s", shard)
	}
	body := renderedIndex(t, in)
	if !strings.Contains(body, "**Nightly-only** (1 requirement(s))") {
		t.Errorf("the rollup did not count the nightly-only requirement:\n%s", body)
	}
	// The polarity view still counts it: Both and Nightly-only answer different
	// questions, and the overlap is the point.
	if !strings.Contains(body, "| `rfc9999` | 1 | 1 | 0 | 0 | 0 | 0 | 1 |") {
		t.Errorf("the polarity columns dropped the nightly-only row:\n%s", body)
	}
}

func TestASupersededSummaryCarriesItsSuccessorInEveryPlaceItIsNamed(t *testing.T) {
	in := renderFixture(t, map[string]string{
		"rfc/short/rfc9999.md": "# RFC 9999\n\n" +
			fixtureMeta(enrolmentEnrolled,
				"the fixture RFC, gated so the render has a population", "bgp-base 10") +
			"| Obsoleted-by | RFC 9998 |\n" +
			"\n- [ ] [RFC9999-2-1] [MUST] A speaker MUST answer (§2) " +
			"{superseded: unresolved; the successor is not here} {gap: nothing sends it}\n",
	})
	body := renderedIndex(t, in)
	if !strings.Contains(body, "**Superseded** (1 summaries)") {
		t.Errorf("the rollup did not name the superseded summary:\n%s", body)
	}
	if !strings.Contains(body, "`rfc9999` -> RFC9998") {
		t.Errorf("the forward pointer is missing:\n%s", body)
	}
	if !strings.Contains(body, "1 point at a document this repository does not hold") {
		t.Errorf("the unresolved debt was not counted:\n%s", body)
	}
	if !strings.Contains(body, ", superseded by RFC9998 |") {
		t.Errorf("the rollup row's State cell does not name the successor:\n%s", body)
	}
	shard := RenderShards(in)["rfc9999"]
	if !strings.HasPrefix(shard, "# RFC9999 -- enrolled (gated), superseded by RFC9998\n") {
		t.Errorf("the shard banner does not name the successor:\n%s", shard)
	}
}

func TestASummaryWithNoObligationIsJudgedAgainstItsOwnSourceText(t *testing.T) {
	// Three verdicts, and the pre-RFC-2119 one is the case a bare uppercase
	// count gets wrong: RFC 1035 shows 0 uppercase and 23 lowercase `must`, and
	// "consistent: source declares none" reads a wire specification as
	// non-normative.
	in := renderFixture(t, map[string]string{
		"rfc/short/rfc1000.md": "# RFC 1000\n\n" + fixtureMeta(dispositionBacklog, "owed", supportNone),
		"rfc/full/rfc1000.txt": "A resolver must answer the query.\n",
		"rfc/short/rfc1001.md": "# RFC 1001\n\n" + fixtureMeta(dispositionBacklog, "owed", supportNone),
		"rfc/full/rfc1001.txt": "This document describes an idea.\n",
		"rfc/short/rfc1002.md": "# RFC 1002\n\n" + fixtureMeta(dispositionBacklog, "owed", supportNone),
		"rfc/full/rfc1002.txt": "A speaker MUST answer the query.\n",
		"rfc/short/rfc1003.md": "# RFC 1003\n\n" + fixtureMeta(dispositionBacklog, "owed", supportNone),
	})
	body := renderedIndex(t, in)
	for _, want := range []string{
		"| `rfc1000` | 0 | 1 | **UNDECIDED**: 0 uppercase keywords but 1 lowercase",
		"| `rfc1001` | 0 | 0 | consistent: source declares none in either register",
		"| `rfc1002` | 1 | 0 | **RE-AUTHOR**: source is normative, summary captured nothing",
		"| `rfc1003` | ? | ? | no source text under `rfc/full/` -- cannot judge",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the verdict row is missing:\n  want %q\ngot:\n%s", want, body)
		}
	}
}

func TestASummaryCapturingOnlyAdvisoryRowsStillCountsAsCapturingNothing(t *testing.T) {
	// One SHOULD bought a summary immunity from this table while the caller
	// passed every parsed requirement at any level. The table's whole purpose is
	// to name summaries that captured no OBLIGATION.
	in := renderFixture(t, map[string]string{
		"rfc/short/rfc1000.md": "# RFC 1000\n\n" +
			fixtureMeta(dispositionBacklog, "owed", supportNone) +
			"\n- [ ] [RFC1000-2-1] [SHOULD] A speaker SHOULD answer (§2)\n",
		"rfc/full/rfc1000.txt": "A speaker MUST answer the query.\n",
	})
	body := renderedIndex(t, in)
	if !strings.Contains(body, "| `rfc1000` | 1 | 0 | **RE-AUTHOR**") {
		t.Errorf("an advisory-only summary hid from the table:\n%s", body)
	}
}

func TestASummaryDeclaringNoMUSTLevelRowRendersNoSectionAtAll(t *testing.T) {
	// The zero boundary the prune relies on: a stem the render never produced is
	// exactly a stem the write deletes.
	in := renderFixture(t, map[string]string{
		"rfc/short/rfc1000.md": "# RFC 1000\n\n" +
			fixtureMeta(dispositionBacklog, "owed", supportNone),
	})
	if _, held := RenderShards(in)["rfc1000"]; held {
		t.Error("a summary with no requirement rendered a shard")
	}
	if _, held := RenderShards(in)["rfc9999"]; !held {
		t.Error("a summary with a requirement rendered no shard")
	}
}

func TestTheAuditTableCannotBeMistakenForTheRollup(t *testing.T) {
	// internal/le/testhealth/actions.go pins the polarity rollup with a nine-cell
	// regex and matches it against every line of the ledger, so an audit row
	// with the same shape would be folded into that tool's proof-density figure.
	in := renderFixture(t, nil)
	for line := range strings.SplitSeq(renderedIndex(t, in), "\n") {
		if !strings.HasPrefix(line, "| `rfc") {
			continue
		}
		if cells := strings.Count(line, "|"); cells != 10 && cells != 7 && cells != 5 && cells != 9 {
			t.Errorf("a rendered row carries %d pipes, a shape no consumer expects: %q",
				cells, line)
		}
	}
}

func TestASignOffWithNoSiteRendersADashRatherThanDividingByZero(t *testing.T) {
	// An artifact with an empty site list has no exclusion ratio to publish,
	// and a zero there would read as "nothing was excluded" rather than as
	// "there was nothing to exclude".
	art := Extraction{Stem: "rfc9999", Register: registerManualWalk,
		SignedOff: "2026-01-01", Reviewer: "someone"}
	in := renderFixture(t, nil)
	rows, err := renderExtractionTable(in)
	if err != nil {
		t.Fatalf("rendering the extraction table: %v", err)
	}
	if strings.Contains(strings.Join(rows, "\n"), "| rfc9999 |") {
		t.Fatal("the fixture already carries a sign-off, so the case below proves nothing")
	}
	if art.Excluded() != 0 || art.Mapped() != 0 || art.Relocated() != 0 {
		t.Errorf("an empty artifact counted %d excluded, %d mapped, %d relocated",
			art.Excluded(), art.Mapped(), art.Relocated())
	}
}

func TestTheEvidenceLegendNamesEveryCarrierIncludingTheOnesNothingRuns(t *testing.T) {
	in := renderFixture(t, nil)
	body := renderedIndex(t, in)
	for _, c := range in.Carriers {
		if c.Tier == tierUnrun {
			continue
		}
		if !strings.Contains(body, "| `"+c.Label()+"` |") {
			t.Errorf("the legend omits the executable carrier %q", c.Name)
		}
	}
	if !strings.Contains(body, "A tag in a carrier nothing executes is REFUSED") {
		t.Error("the legend does not say what happens to an unrun carrier")
	}
	// The derived suite rows collapse to one line each, and the suites they
	// stand for are listed rather than dropped.
	if !strings.Contains(body, " -- suites: `") {
		t.Errorf("the derived carriers did not collapse:\n%s", body)
	}
}

func TestTheAuditRowsTwoPartitionsHoldOverEveryRFC(t *testing.T) {
	// Two partitions over two populations, and each has to hold on its own:
	// Auditable = Audited + Unaudited over REQUIREMENTS, and
	// Verdicts = Proven + Findings over RECORDS. They are not the same
	// population, which is why a verdict on a requirement that is not auditable
	// is counted in the second and in neither column of the first.
	//
	// Audited <= Auditable is what makes the table's row filter's second clause
	// unreachable: a row with nothing auditable can carry no verdict in the
	// requirement view either. Asserted rather than argued, so the day the two
	// counters stop nesting the filter has to be decided again.
	in := renderFixture(t, nil)
	rows, worklist := auditCoverageRows(auditCoverageInput{
		Requirements: in.Requirements, Tags: in.Tags, Enrolled: in.Enrolled,
		Carriers: in.Carriers, Audits: in.Audits, States: in.States,
	})
	if len(rows) == 0 {
		t.Fatal("the fixture produced no audit row, so the properties below are vacuous")
	}
	findings := 0
	for _, r := range rows {
		if r.Audited > r.Auditable {
			t.Errorf("%s counts %d audited of %d auditable", r.RFC, r.Audited, r.Auditable)
		}
		if r.Proven+r.Findings != r.Verdicts {
			t.Errorf("%s records %d verdict(s), split %d proven and %d not",
				r.RFC, r.Verdicts, r.Proven, r.Findings)
		}
		findings += r.Findings
	}
	if findings != len(worklist) {
		t.Errorf("the rows count %d finding(s) and the worklist holds %d row(s)",
			findings, len(worklist))
	}
}

func TestEveryTagTheScanProducesResolvesToACarrier(t *testing.T) {
	// EvidenceLabel falls back to `unknown/unrun` for a path no carrier claims,
	// and that fallback is unreachable through the gate: the scan hands a file
	// to a reader only after the carrier table matched it, and a tag in a
	// carrier nothing executes is REFUSED rather than labeled. So the fallback
	// can only be reached by a synthetic tag.
	//
	// This is the property that makes the fallback's exact wording untestable
	// from the entry point, and it is asserted rather than argued: the day the
	// scan starts producing a tag with no carrier, the label it gets is a
	// decision somebody has to take again.
	in := renderFixture(t, map[string]string{
		"internal/widget/widget_test.go": "package widget\n\n" +
			"// RFC requirement" + ": RFC9999-2-1 positive\nfunc TestSend(t *testing.T) {}\n",
		"test/plugin/widget.ci": "# RFC requirement" + ": RFC9999-2-1 negative\nname=widget\n",
	})
	if len(in.Tags) == 0 {
		t.Fatal("the fixture produced no tag, so the property below is vacuous")
	}
	for _, tag := range in.Tags {
		if _, held := CarrierFor(tag.File, in.Carriers); !held {
			t.Errorf("the scan produced a tag in %s, which no carrier claims", tag.File)
		}
		if evidenceLabel(tag.File, in.Carriers) == "unknown/unrun" {
			t.Errorf("a scanned tag in %s labeled itself unknown", tag.File)
		}
	}
	// And the fallback still answers visibly wrong rather than plausibly right
	// for the one caller that can reach it.
	if got := evidenceLabel("nothing/at/all.txt", in.Carriers); got != "unknown/unrun" {
		t.Errorf("an unrecognized carrier labeled itself %q", got)
	}
	if got := evidenceTier("nothing/at/all.txt", in.Carriers); got != tierUnrun {
		t.Errorf("an unrecognized carrier claimed the %q tier", got)
	}
}

func TestAPipelineWithNoSuiteQualifierKeepsItsWholeText(t *testing.T) {
	// stageOf cuts at the LAST comma, which is where a derived row's suite name
	// sits. A pipeline with no comma at all must not lose its text.
	if got := stageOf("./le verify current mode full (unit stage)"); got != "./le verify current mode full (unit stage))" {
		t.Errorf("a comma-free pipeline rendered as %q", got)
	}
	if got := suiteOfPrefix(""); got != "" {
		t.Errorf("an empty prefix named the suite %q", got)
	}
	if got := suiteOfPrefix("test/plugin/"); got != "plugin" {
		t.Errorf("test/plugin/ named the suite %q", got)
	}

	// stageOf cuts at the LAST comma and every derived pipeline holds exactly
	// one, so first and last are the same cut today and no output can tell them
	// apart. That is a property of the carrier table rather than of this
	// function, so it is asserted: the day a stage name gains a comma, the
	// choice between the two has to be taken again rather than discovered in a
	// rendered page.
	in := renderFixture(t, nil)
	derived := 0
	for _, c := range in.Carriers {
		if !c.Derived {
			continue
		}
		derived++
		if n := strings.Count(c.Pipeline, ","); n != 1 {
			t.Errorf("the derived carrier %q holds %d comma(s) in %q; stageOf cuts at the last",
				c.Name, n, c.Pipeline)
		}
	}
	if derived == 0 {
		t.Fatal("no carrier is derived, so the property above is vacuous")
	}
}

func TestAnAnnotatedRequirementWithOneTestIsAuditableOnlyWhenTheAnnotationSaysWhy(t *testing.T) {
	// The coverage rule the schema reads. A {single-polarity} line IS the
	// missing polarity's justification; a {gap} line is not, and judging one as
	// complete cover would let a verdict claim proof over a requirement with a
	// declared hole.
	one := []Tag{{RID: "RFC9999-2-1", Polarity: "positive", File: "a_test.go"}}
	both := append(append([]Tag(nil), one...),
		Tag{RID: "RFC9999-2-1", Polarity: "negative", File: "a_test.go"})
	cases := []struct {
		name string
		req  Requirement
		tags []Tag
		want bool
	}{
		{"both polarities need no annotation", Requirement{}, both, true},
		{"one polarity alone is incomplete", Requirement{}, one, false},
		{"single-polarity justifies the missing one",
			Requirement{Annotation: &Annotation{Kind: AnnotationSinglePolarity}}, one, true},
		{"a gap does not justify it",
			Requirement{Annotation: &Annotation{Kind: AnnotationGap}}, one, false},
		{"an annotation with no test at all is not cover",
			Requirement{Annotation: &Annotation{Kind: AnnotationSinglePolarity}}, nil, false},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			if got := polarityCovered(one.req, one.tags); got != one.want {
				t.Errorf("PolarityCovered answered %v, want %v", got, one.want)
			}
		})
	}
}

// ─── The generator ──────────────────────────────────────────────────────────

// generatorTree writes a checkout the generator can be run over, and answers
// its root.
func generatorTree(t *testing.T, extra map[string]string) string {
	t.Helper()

	files := map[string]string{
		"rfc/short/rfc9999.md": "# RFC 9999\n\n" +
			fixtureMeta(enrolmentEnrolled,
				"the fixture RFC, gated so the generator has a population", "bgp-base 10") +
			"\n- [ ] [RFC9999-2-1] [MUST] A speaker MUST answer (§2) {gap: nothing sends it}\n",
		// The carrier table derives each interop tree's tier from whether a
		// SCHEDULED workflow names its runner, and an unreadable workflow
		// directory is refused rather than read as "nothing runs". Every
		// fixture therefore carries one, and a case that needs a schedule
		// replaces it.
		".github/workflows/ci.yml": "on: push\njobs:\n  a:\n    steps:\n      - run: ./le verify deps unit-cached\n",
	}
	maps.Copy(files, extra)
	return fixtureTree(t, files)
}

func TestTheGeneratorWritesTheLedgerAndOneTablePerRFC(t *testing.T) {
	tree := generatorTree(t, nil)
	report, err := IndexUpdate(tree)
	if err != nil {
		t.Fatalf("the generator refused a clean tree: %v", err)
	}
	if report.Shards != 1 || report.Ledger != ledgerRel {
		t.Errorf("the report says %d shard(s) under %q", report.Shards, report.Ledger)
	}
	if len(report.Deleted) != 0 {
		t.Errorf("a first run deleted %v", report.Deleted)
	}
	body, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(shardRel("rfc9999"))))
	if err != nil {
		t.Fatalf("the shard was not written: %v", err)
	}
	if !strings.HasSuffix(string(body), "\n\n") {
		t.Errorf("the shard does not end in the blank line the comparison expects: %q",
			string(body[max(0, len(body)-8):]))
	}
}

func TestTheGeneratorIsIdempotent(t *testing.T) {
	tree := generatorTree(t, nil)
	if _, err := IndexUpdate(tree); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first := readGenerated(t, tree)
	if _, err := IndexUpdate(tree); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second := readGenerated(t, tree)
	for name, body := range first {
		if second[name] != body {
			t.Errorf("a second run over an unchanged tree rewrote %s", name)
		}
	}
	if len(first) != len(second) {
		t.Errorf("a second run produced %d file(s), the first produced %d",
			len(second), len(first))
	}
}

// readGenerated answers every generated file of a tree, keyed by its
// repo-relative path.
func readGenerated(t *testing.T, tree string) map[string]string {
	t.Helper()

	out := map[string]string{}
	entries, err := os.ReadDir(filepath.Join(tree, filepath.FromSlash(shardRelDir)))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("reading the shard directory: %v", err)
	}
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(shardRelDir), entry.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		out[entry.Name()] = string(body)
	}
	if ledger, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(ledgerRel))); err == nil {
		out[ledgerRel] = string(ledger)
	}
	return out
}

func TestTheGeneratorDeletesAShardItNoLongerProduces(t *testing.T) {
	tree := generatorTree(t, map[string]string{
		"rfc/requirements/rfc1000.md": "# RFC 1000 -- stale\n",
		"rfc/requirements/README.md":  "authored, not generated\n",
		"rfc/requirements/.gitkeep":   "",
	})
	report, err := IndexUpdate(tree)
	if err != nil {
		t.Fatalf("the generator refused: %v", err)
	}
	if len(report.Deleted) != 1 || report.Deleted[0] != "rfc1000" {
		t.Errorf("the prune removed %v, want [rfc1000]", report.Deleted)
	}
	for _, kept := range []string{"README.md", ".gitkeep"} {
		if _, err := os.Stat(filepath.Join(tree, "rfc", "requirements", kept)); err != nil {
			t.Errorf("the prune deleted %s, which the generator does not own", kept)
		}
	}
}

func TestThePruneNeverDescendsIntoASubdirectory(t *testing.T) {
	tree := generatorTree(t, map[string]string{
		"rfc/requirements/notes/rfc1000.md": "kept\n",
	})
	if _, err := IndexUpdate(tree); err != nil {
		t.Fatalf("the generator refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tree, "rfc", "requirements", "notes", "rfc1000.md")); err != nil {
		t.Errorf("the prune reached into a subdirectory: %v", err)
	}
}

func TestTheGeneratorRefusesToWriteWhenASummaryDidNotParse(t *testing.T) {
	// The refusal exists because the write DELETES: the stem that failed to
	// parse renders nothing, drops out of the rendered set, and its tracked file
	// is removed as an orphan while the run exits 0.
	tree := generatorTree(t, map[string]string{
		"rfc/short/rfc1000.md": "# RFC 1000\n\n" +
			fixtureMeta(dispositionBacklog, "owed", supportNone) +
			"\n- [ ] [MUST] no id here at all\n",
		"rfc/requirements/rfc1000.md": "# RFC 1000 -- would be deleted\n",
	})
	_, err := IndexUpdate(tree)
	if err == nil {
		t.Fatal("the generator wrote over a tree it could not fully read")
	}
	if !strings.Contains(err.Error(), "refusing to write") {
		t.Errorf("the refusal does not say what it refused: %v", err)
	}
	if !strings.Contains(err.Error(), "rfc/short/rfc1000.md") {
		t.Errorf("the refusal does not name the summary to fix: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tree, "rfc", "requirements", "rfc1000.md")); statErr != nil {
		t.Error("the refusal deleted a shard before refusing")
	}
	if _, statErr := os.Stat(filepath.Join(tree, "ai", "RFC-REQUIREMENTS.md")); statErr == nil {
		t.Error("the refusal wrote the ledger before refusing")
	}
}

func TestTheGeneratorRefusesToWriteWhenTheRenderProducesNoRowAtAll(t *testing.T) {
	// The same failure at full scale: an absent rfc/short/ leaves every stem
	// unrendered and every tracked file an orphan.
	tree := fixtureTree(t, map[string]string{
		"rfc/requirements/rfc9999.md": "# RFC 9999 -- would be deleted\n",
		".github/workflows/ci.yml":    "on: push\njobs:\n  a:\n    steps:\n      - run: ./le verify deps unit-cached\n",
	})
	_, err := IndexUpdate(tree)
	if err == nil {
		t.Fatal("the generator pruned every tracked file and reported success")
	}
	if !strings.Contains(err.Error(), "would be deleted as an orphan") {
		t.Errorf("the refusal does not say what was at stake: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tree, "rfc", "requirements", "rfc9999.md")); statErr != nil {
		t.Error("the refusal deleted a shard before refusing")
	}
}

func TestTheGeneratorPageNamesWhatItWroteAndWhatItRemoved(t *testing.T) {
	quiet := IndexReport{Ledger: ledgerRel, Shards: 3}
	if quiet.Text() != "wrote ai/RFC-REQUIREMENTS.md and 3 shard(s) under rfc/requirements\n" {
		t.Errorf("a run that deleted nothing prints %q", quiet.Text())
	}
	busy := IndexReport{Ledger: ledgerRel, Shards: 3, Deleted: []string{"rfc1000", "rfc1001"}}
	if !strings.HasSuffix(busy.Text(), "deleted orphan shard(s): rfc1000, rfc1001\n") {
		t.Errorf("a run that deleted does not name what it removed: %q", busy.Text())
	}
}
