// Design: docs/contributing/gh-pages.md -- Quality pages
// Related: rfccompliance.go -- the compliance index these words introduce
//
// rfcreader.go holds the words the compliance index gives a reader who is not a
// Ze contributor: a network engineer judging Ze. Each section opens with what
// it shows and why it matters, a glossary near the top explains the project's
// terms once, and the check results lead with one plain line for each group of
// checks. The raw diagnostics stay on the page, unchanged, in a collapsed block
// for contributors.
package site

import (
	"html"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcHeroLead is the first sentence under the page title.
const rfcHeroLead = "How far Ze meets each RFC it implements, and the tests that prove it."

// rfcReaderHeading is the heading of the reader's introduction.
const rfcReaderHeading = "How to read this page"

// rfcReaderParagraphs introduce the page to a reader who has never seen Ze's
// repository.
var rfcReaderParagraphs = []string{
	"This page shows how far Ze meets the standards (RFCs) it implements, and how that " +
		"is proven. For each RFC, Ze keeps a summary that lists every requirement the RFC " +
		"states and links each requirement to the tests that prove Ze meets it. An automatic " +
		"check, the gate, reads those summaries and the tests, and the figures on this page " +
		"are computed from the same summaries and tests.",
	"If you are evaluating Ze for one role, start with Coverage by profile. To check one " +
		"RFC, find it in the Enrolled RFCs table and open its own page, which lists each " +
		"requirement and the tests behind it. The terms below are used throughout this page; " +
		"the per-RFC pages and Ze's source use the same terms.",
}

// rfcGlossaryTerm is one term the page uses, as a reader meets it.
type rfcGlossaryTerm struct {
	Term    string
	Meaning string
}

// rfcGlossary explains each project term once. Every meaning restates what a
// card, a legend or a section on this page already says about the term, so the
// glossary adds no claim of its own.
func rfcGlossary() []rfcGlossaryTerm {
	return append([]rfcGlossaryTerm{
		{"Summary", "Ze's file for one RFC: it lists the RFC's requirements, the tests " +
			"behind each one, and the support status Ze publishes for the RFC."},
		{"Requirement", "one obligation an RFC states, kept in the summary with an " +
			"identifier such as RFC4271-8.2.2-12 that names the RFC and its section."},
		{"MUST-level (mandatory)", "a requirement stated with " +
			strings.Join(rfc.MandatoryKeywords(), ", ") + ". The optional levels are " +
			strings.Join(rfc.OptionalKeywords(), ", ") + "."},
		{"Enrolled", "an RFC whose summary the gate holds Ze to. Summaries that are not " +
			"enrolled are listed at the end of the page, each with its reason."},
		{"Gate, gated", "the gate is the automatic check that reads every summary and the " +
			"tests. A gated requirement is a MUST-level requirement of an enrolled RFC, one " +
			"the gate holds Ze to."},
		{"Positive and negative test (polarity)", "a positive test proves Ze does what the " +
			"requirement demands; a negative test proves Ze refuses what the requirement " +
			"forbids. Tested both ways means both exist."},
		{"Tagged unit", "one test that names, in a tag, the requirement it proves. One " +
			"requirement can have several."},
		{"Recorded break", "the proof that a test can fail: the code under the test was " +
			"broken once on purpose, the test was seen to fail, and a record stores what the " +
			"test, its claim and the code looked like. The break is not run again. Ze's " +
			"source calls this a discrimination record."},
		{"Escaped", "a tagged unit whose record claims that no break of the code exists. " +
			"It is not counted as a proof."},
		{"Lapsed", "a recorded break whose test, claim or code has changed since it was " +
			"recorded. It is not counted as a proof."},
		{"Gap", "a requirement the summary openly declares Ze does not meet yet. Every gap " +
			"is work Ze owes."},
		{"Annotation", "a note on a requirement, written in braces, that says why it " +
			"needs no ordinary pair of tests. {single-polarity}: the requirement admits no " +
			"counter-case, so one direction plus a recorded reason is the whole proof. " +
			"{not-applicable}: the obligation does not bind Ze. {feature-declined}: the " +
			"obligation applies only to a feature the RFC makes optional and Ze does not " +
			"offer. {lower-layer}: a layer under Ze performs the behavior, on state Ze " +
			"installs into that layer. {gap}: a declared gap. A partial requirement has " +
			"tests for part of it; the rest is unmet or unproven and earns no credit."},
		{"Linux cannot do this", "a verified kernel limitation prevents the required primitive. " +
			"The annotation names the Linux Limitations Register entry. The row is OS blocked: " +
			"a limit of the kernel, never counted against Ze, so it sits with the rows not owed " +
			"and in no section of the MUST bar."},
		{"Walked, sign-off", "a reviewer read the RFC's own text sentence by sentence and " +
			"checked the requirement list against it; the sign-off records that review. " +
			"Not walked means the review has not been done, so no figure is counted."},
		{"Excluded sentence", "a sentence of the RFC that the review decided not to turn " +
			"into a requirement, with a kind and a reason."},
		{"Semantic verdict", "a reviewer's recorded judgement that a requirement's tests " +
			"enforce what the requirement says. Fresh: still current. Shifted: the test is " +
			"unchanged and only the file around it moved. Stale: what it judged has changed."},
	}, rfcBarGlossary()...)
}

// rfcSegmentMeanings defines each bar segment, keyed by the segment name
// rfcBars declares. rfcBarGlossary reads rfcBars for the segments and their
// words, so a segment added there without a meaning here is refused by
// TestGlossaryDefinesBarTerms rather than published undefined.
var rfcSegmentMeanings = map[string]string{
	rfcSegZeProven: "a MUST row Ze's own code performs, with a tagged test whose " +
		"recorded break was seen to fail.",
	rfcSegOSVerified: "a row a layer under Ze performs (the Linux kernel Ze ships, or a Go " +
		"library Ze links), with a tagged test that checks that layer's behavior on the " +
		"state Ze installs, and a recorded break.",
	rfcSegOSGap: "a row a layer under Ze performs, with no verified test of that behavior " +
		"yet. It is a gap in testing, not in correctness.",
	rfcSegZeGap: "a MUST row Ze's own code owes that no recorded break proves yet: a " +
		"declared gap, a partial row, a row with no proof, or an exclusion whose " +
		"evidence does not hold. Every one is work Ze owes.",
	rfcSegShouldProven: "a SHOULD or MAY row Ze's code performs, with a tagged test whose " +
		"recorded break was seen to fail.",
	rfcSegNotOwed: "a row that does not bind Ze, shown by checked evidence: it binds a role " +
		"Ze does not fill, it applies only to an optional feature Ze does not offer, or " +
		"Linux cannot provide its primitive (OS blocked). It is left out of the MUST bar.",
	rfcSegNotEver: "a SHOULD or MAY row Ze will never support. Only the project owner sets it.",
	rfcSegNotYet:  "a SHOULD or MAY row Ze intends to support and does not support yet.",
	rfcSegNotDecided: "a SHOULD or MAY row nobody has decided on yet: every unproven " +
		"optional row starts here.",
}

// rfcBarGlossary answers one glossary term per bar segment, in the order the
// bars draw them. A segment the two bars share is one term, named by every
// label it carries, so each word a bar shows is defined exactly once. A
// segment with no meaning panics: rfcBars and rfcSegmentMeanings are both
// package constants, so only a Ze defect reaches it, and the glossary test
// runs it.
func rfcBarGlossary() []rfcGlossaryTerm {
	order := []string{}
	labels := map[string][]string{}
	for _, segment := range append(slices.Clone(rfcBars.Must), rfcBars.Overview...) {
		if _, seen := labels[segment.Segment]; !seen {
			order = append(order, segment.Segment)
		}
		if !slices.Contains(labels[segment.Segment], segment.Label) {
			labels[segment.Segment] = append(labels[segment.Segment], segment.Label)
		}
	}
	terms := make([]rfcGlossaryTerm, 0, len(order))
	for _, name := range order {
		meaning, ok := rfcSegmentMeanings[name]
		if !ok {
			panic("BUG: bar segment " + name + " has no glossary meaning")
		}
		terms = append(terms, rfcGlossaryTerm{strings.Join(labels[name], ", ") + " (bar segment)", meaning})
	}
	return terms
}

// rfcReaderIntroHTML renders the introduction and the glossary.
func rfcReaderIntroHTML() string {
	var out textbuf.Buffer
	out.Str("<section><h2>").Str(rfcReaderHeading).Str("</h2>\n")
	for _, paragraph := range rfcReaderParagraphs {
		out.Str("<p>").Str(html.EscapeString(paragraph)).Str("</p>\n")
	}
	// The glossary is folded: the figures come first, and a reader opens it
	// when a term stops them.
	out.Str("<details class=\"rfc-glossary-fold\"><summary>Glossary of the terms this page uses</summary>\n")
	out.Str("<dl class=\"rfc-glossary\">\n")
	for _, term := range rfcGlossary() {
		out.Str("<dt>").Str(html.EscapeString(term.Term)).Str("</dt><dd>").
			Str(html.EscapeString(term.Meaning)).Str("</dd>\n")
	}
	out.Str("</dl>\n</details>\n</section>\n")
	return out.String()
}

// rfcReaderIntroMirror states the same introduction.
func rfcReaderIntroMirror() string {
	var out textbuf.Buffer
	out.Str("## ").Str(rfcReaderHeading).Str("\n\n")
	for _, paragraph := range rfcReaderParagraphs {
		out.Str(paragraph).Str("\n\n")
	}
	for _, term := range rfcGlossary() {
		out.Str("- **").Str(term.Term).Str("**: ").Str(term.Meaning).Byte('\n')
	}
	return out.String()
}

// The section leads: what each section shows, and why it matters to someone
// choosing a router. Each one is the first paragraph under its heading, on the
// page and in the mirror.
const (
	rfcLeadVerdict = "The gate holds Ze to every MUST-level requirement of the RFCs it " +
		"implements. The bars and the Total row below put each MUST row of a walked " +
		"enrolled RFC in one section: proven by Ze's tests, verified in the operating " +
		"system, not owed, or still owed by Ze or by the operating system's tests. The cards " +
		"after them say how many tagged tests carry a recorded break and whether the gate passes."
	rfcLeadProfiles = "A profile groups the RFCs one kind of deployment needs. If you are " +
		"evaluating Ze for one role, its profile page gives the figures for that role alone."
	rfcLeadImplements = "Some RFCs are carried out by Ze's own code, some by a layer under or " +
		"beside Ze, and some oblige no implementation at all. This section says which is " +
		"which, because the figures on this page count only what Ze's own code is held to."
	rfcLeadGaps = "A gap is a requirement Ze does not meet yet and says so in public. This " +
		"section counts the requirements declared as gaps and the RFCs that carry them."
	rfcLeadExclusions = "Not every sentence of an RFC becomes a requirement. This section " +
		"counts the sentences a review left out, by the reason given, so you can check what " +
		"was left out and why."
	rfcLeadClusters = "The RFCs that declare the most gaps. If one of them matters to your " +
		"deployment, its own page names each gap."
	rfcLeadMechanism = "Where the figures come from: the files that hold the requirements " +
		"and the tests, and the command that reproduces the verdict. You do not need this " +
		"section to read the results."
	rfcLeadChecks = "The gate runs several groups of checks. Each line below says what one " +
		"group verifies and whether it passed, and for a group that failed, what that means " +
		"for the figures. The raw findings, written for Ze's contributors, are in the " +
		"collapsed block after the list."
	rfcLeadEnrolled = "Every RFC the gate holds Ze to, with its own figures. Open an RFC to " +
		"see each of its requirements and the tests behind it."
	rfcLeadDeclined = "RFCs Ze keeps a summary for that the gate does not hold Ze to, each " +
		"with the kind of reason and the reason itself."
)

// rfcLeadHTML renders one section lead.
func rfcLeadHTML(lead string) string {
	return "<p class=\"rfc-lead\">" + html.EscapeString(lead) + "</p>\n"
}

// rfcContributorSummary labels the collapsed block that keeps the raw findings.
const rfcContributorSummary = "For contributors: the raw findings and the commands that fix them"

// rfcCheckLine is one group of checks as the reader meets it.
type rfcCheckLine struct {
	Name     string
	Verifies string
	Failure  string
	Problems int
}

// rfcCheckLines answers one line for every group the gate declares, in the
// gate's order, with the number of findings each raised. Findings that name no
// group get a line of their own, so none of them is counted under a guess.
func rfcCheckLines(gate rfcGate) []rfcCheckLine {
	counts := map[rfc.CheckGroupKey]int{}
	for index := range gate.Findings {
		counts[gate.Findings[index].Check]++
	}
	lines := make([]rfcCheckLine, 0, len(rfc.CheckGroups)+1)
	for _, group := range rfc.CheckGroups {
		lines = append(lines, rfcCheckLine{Name: group.Name,
			Verifies: group.Verifies, Failure: group.Failure, Problems: counts[group.Key]})
	}
	if unnamed := counts[""]; unnamed > 0 {
		lines = append(lines, rfcCheckLine{Name: "Other checks",
			Verifies: "These findings do not name the group of checks that raised them.",
			Failure:  "The contributor detail below states each one.", Problems: unnamed})
	}
	return lines
}

// rfcCheckVerdict states whether one group passed, in words.
func rfcCheckVerdict(line rfcCheckLine) string {
	if line.Problems == 0 {
		return "Passed."
	}
	return "Failed, " + plural(line.Problems, "problem") + "."
}

// rfcCheckLinesHTML renders the plain list of check groups.
func rfcCheckLinesHTML(gate rfcGate) string {
	var out textbuf.Buffer
	out.Str("<ul class=\"rfc-check-groups\">\n")
	for _, line := range rfcCheckLines(gate) {
		tone := rfcToneOK
		if line.Problems > 0 {
			tone = rfcToneBad
		}
		out.Str("<li class=\"rfc-").Str(tone).Str("\"><strong>").Str(html.EscapeString(line.Name)).
			Str(":</strong> ").Str(rfcCheckVerdict(line)).Str(" ").Str(html.EscapeString(line.Verifies))
		if line.Problems > 0 {
			out.Str(" What this means: ").Str(html.EscapeString(line.Failure))
		}
		out.Str("</li>\n")
	}
	out.Str("</ul>\n")
	return out.String()
}

// rfcCheckLinesMirror states the same list.
func rfcCheckLinesMirror(gate rfcGate) string {
	var out textbuf.Buffer
	for _, line := range rfcCheckLines(gate) {
		out.Str("- **").Str(line.Name).Str(":** ").Str(rfcCheckVerdict(line)).Str(" ").Str(line.Verifies)
		if line.Problems > 0 {
			out.Str(" What this means: ").Str(line.Failure)
		}
		out.Byte('\n')
	}
	return out.String()
}
