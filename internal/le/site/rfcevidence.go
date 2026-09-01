// Design: website/AI.md -- what a reader has verified about one RFC's requirements
// Overview: rfcdetail.go -- the page these sections are assembled into
//
// The evidence half of a per-RFC page: the gaps, the proof state of every
// tagged unit, the extraction sign-off that decided which sentences became
// requirements, and where a superseded document's obligations live now.
//
// The disclosure here is FULL, by owner ruling of 2026-09-01. Every bad state
// is named under the requirement id it belongs to, and a count never stands in
// for the list (AC-17, AC-18).
package site

import (
	"html"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcGapRow is one requirement this RFC owes and does not prove: a declared
// gap, or a gated MUST with no test bound to it.
type rfcGapRow struct {
	RID    string
	Text   string
	Kind   string
	Reason string
}

// rfcGapRows answers every declared gap and every untested gated MUST, ONE ROW
// per requirement, with every state that holds for it named in that row.
//
// A `{gap}` is an ISSUE and is never rendered as coverage. A gated MUST with no
// tag is listed even where an annotation excuses it, because the annotation is
// the reason and not the test (ai/rules/rfc-compliance.md). A requirement that
// is both takes ONE row naming both: two rows quoting the same reason under one
// id read as two findings, and there is one (owner review, 2026-09-01).
func rfcGapRows(entry *rfcLedgerStem) []rfcGapRow {
	var rows []rfcGapRow
	for index := range entry.Requirements {
		requirement := &entry.Requirements[index]
		declared := requirement.Annotation != nil &&
			requirement.Annotation.Kind == rfc.AnnotationGap
		untested := requirement.Gated && len(requirement.Covers) == 0
		if !declared && !untested {
			continue
		}
		var states []string
		if declared {
			states = append(states, "{"+rfc.AnnotationGap+"}")
		}
		if untested {
			states = append(states, "no test")
		}
		rows = append(rows, rfcGapRow{RID: requirement.RID, Text: requirement.Text,
			Kind: strings.Join(states, ", "), Reason: rfcGapReason(requirement, declared)})
	}
	return rows
}

// rfcGapReason answers why one requirement is on that table: the gap's own
// prose where it declares one, the annotation that excuses the missing test
// where it carries one, and the plain fact otherwise.
func rfcGapReason(requirement *rfcLedgerRequirement, declared bool) string {
	if declared {
		return requirement.Annotation.Reason
	}
	if requirement.Annotation != nil {
		return "no test carries this requirement id; annotated {" +
			requirement.Annotation.Kind + "}: " + requirement.Annotation.Reason
	}
	return "no test carries this requirement id"
}

// rfcGapsHTML renders every gap and every untested MUST.
//
// The public ledger's own Remaining cell is NOT here: it is the ledger's prose
// and it is stated once, under What the public ledger says, beside the Coverage
// cell it belongs with (owner review, 2026-09-01). This section is the
// requirement-by-requirement answer.
func rfcGapsHTML(entry *rfcLedgerStem) string {
	var out strings.Builder
	rows := rfcGapRows(entry)
	if len(rows) == 0 {
		out.WriteString("<p>" + html.EscapeString(rfcNoGapsText(entry)) + "</p>")
		return out.String()
	}
	var body strings.Builder
	for _, row := range rows {
		body.WriteString(rfcRowCells(rfcRequirementRefHTML(row.RID, row.Text),
			html.EscapeString(row.Kind), html.EscapeString(row.Reason)))
	}
	out.WriteString(rfcTableHTML(rfcHeadCells("Requirement", "State", "Reason"), body.String()))
	return out.String()
}

// rfcGapsMirror states the same rows.
func rfcGapsMirror(entry *rfcLedgerStem) string {
	var out strings.Builder
	rows := rfcGapRows(entry)
	if len(rows) == 0 {
		out.WriteString(rfcNoGapsText(entry) + "\n")
		return out.String()
	}
	out.WriteString(rfcMirrorHead("Requirement", "State", "Reason"))
	for _, row := range rows {
		out.WriteString(rfcMirrorRow(rfc.TableCell(rfcRequirementRefMirror(row.RID, row.Text)),
			row.Kind, rfc.TableCell(row.Reason)))
	}
	return out.String()
}

// rfcNoGapsText says what an empty gap section means, in one sentence.
//
// One sentence, and not a sentence followed by a bare "None." The two read as
// two facts and only one of them is (owner review, 2026-09-01).
func rfcNoGapsText(entry *rfcLedgerStem) string {
	return entry.Display + " declares no gap, and every gated MUST it carries has a test " +
		"bound to it."
}

// rfcProofCounts is what one summary's tagged units and recorded verdicts add
// up to.
//
// Every count here is also published as a list of requirement ids under the
// Proof state heading, so no count on this page stands in for a list (AC-18).
type rfcProofCounts struct {
	Units      int
	Unproven   int
	Escapes    int
	Unsound    int
	NotCurrent int
}

// rfcProofCountsOf counts the tagged units that carry no proof and the recorded
// verdicts that are unsound or no longer current.
func rfcProofCountsOf(entry *rfcLedgerStem) rfcProofCounts {
	var counts rfcProofCounts
	for index := range entry.Requirements {
		requirement := &entry.Requirements[index]
		if requirement.Audit != nil {
			if !rfcVerdictIsSound(requirement.Audit.Verdict) {
				counts.Unsound++
			}
			if requirement.Audit.Freshness != rfc.FreshState {
				counts.NotCurrent++
			}
		}
		for coverIndex := range requirement.Covers {
			cover := &requirement.Covers[coverIndex]
			counts.Units++
			switch {
			case cover.Proof == nil:
				counts.Unproven++
			case !cover.Proof.Proves:
				counts.Escapes++
			}
		}
	}
	return counts
}

// rfcVerdictIsSound answers whether one recorded verdict says the tests hold.
//
// The two sound words are the vocabulary's own, so a sixth verdict lands on the
// unsound side rather than being counted as good news by an omission here.
func rfcVerdictIsSound(verdict string) bool {
	return verdict == rfc.VerdictEnforced || verdict == rfc.VerdictNotApplicable
}

// rfcProofRows answers the proof state of every requirement that has one, and
// of every gated MUST that has none.
func rfcProofRows(entry *rfcLedgerStem) []*rfcLedgerRequirement {
	rows := make([]*rfcLedgerRequirement, 0, len(entry.Requirements))
	for index := range entry.Requirements {
		requirement := &entry.Requirements[index]
		if !requirement.Gated && requirement.Audit == nil && len(requirement.Covers) == 0 {
			continue
		}
		rows = append(rows, requirement)
	}
	return rows
}

// rfcVerdictText says what a reader judged about one requirement, and says so
// plainly when nobody has.
func rfcVerdictText(verdict *rfcLedgerVerdict) string {
	if verdict == nil {
		return "not audited: no reader has judged these tests"
	}
	text := verdict.Verdict
	if verdict.Meaning != "" {
		text += " (" + verdict.Meaning + ")"
	}
	text += ", " + verdict.Freshness
	if len(verdict.Moved) != 0 {
		text += ": " + strings.Join(verdict.Moved, ", ") + " moved"
	}
	if verdict.Note != "" {
		text += ". " + verdict.Note
	}
	return text
}

// rfcProofStateText says what stands behind one tagged unit, in the one column
// that answers it.
//
// A unit with no record reads as the single word the legend above the table
// explains. A `no-break` record is named as the escape it is and is never
// counted as a proof (ai/rules/rfc-compliance.md).
func rfcProofStateText(cover *rfcLedgerCover) string {
	if cover.Proof == nil {
		return rfcUnproven
	}
	text := cover.Proof.Route
	if !cover.Proof.Proves {
		text += " escape (" + cover.Proof.Reason + "), which is not a proof"
	}
	text += ", " + cover.Proof.State
	if cover.Proof.Detail != "" {
		text += " (" + cover.Proof.Detail + ")"
	}
	return text
}

// rfcProofHTML renders one block per requirement: the id, its text, the
// recorded verdict, and one ROW PER TAGGED UNIT.
//
// One row per unit, because the retired shape joined eight of them into one
// cell with semicolons and repeated the same eight words on every entry (owner
// review, 2026-09-01).
func rfcProofHTML(entry *rfcLedgerStem) string {
	rows := rfcProofRows(entry)
	if len(rows) == 0 {
		return "<p>" + html.EscapeString(entry.Display+
			" carries no gated, tagged or audited requirement, so there is no proof state "+
			"to state.") + "</p>"
	}
	ambiguous := rfcAmbiguousNames(entry)
	var out strings.Builder
	out.WriteString("<p>" + html.EscapeString(rfcProofLegend) + "</p>\n")
	for _, requirement := range rows {
		out.WriteString("<h3>" + rfcRequirementRefHTML(requirement.RID, "") + "</h3>\n")
		out.WriteString("<p>" + html.EscapeString(requirement.Text) + "</p>\n")
		out.WriteString("<p><strong>Audit verdict:</strong> " +
			html.EscapeString(rfcVerdictText(requirement.Audit)) + "</p>\n")
		if len(requirement.Covers) == 0 {
			out.WriteString("<p>" + html.EscapeString("No test carries "+requirement.RID+
				", so no unit is bound to it.") + "</p>\n")
			continue
		}
		var body strings.Builder
		for index := range requirement.Covers {
			cover := &requirement.Covers[index]
			body.WriteString(rfcRowCells(html.EscapeString(cover.Polarity),
				rfcCitationHTML(cover, ambiguous),
				html.EscapeString(rfcOrUnstated(cover.Carrier)),
				html.EscapeString(rfcProofStateText(cover))))
		}
		out.WriteString(rfcTableHTML(rfcHeadCells("Polarity", "Test",
			"Kind and tier", "Proof state"), body.String()) + "\n")
	}
	return strings.TrimSuffix(out.String(), "\n")
}

// rfcProofMirror states the same blocks.
func rfcProofMirror(entry *rfcLedgerStem) string {
	rows := rfcProofRows(entry)
	if len(rows) == 0 {
		return entry.Display + " carries no gated, tagged or audited requirement, so there " +
			"is no proof state to state.\n"
	}
	ambiguous := rfcAmbiguousNames(entry)
	var out strings.Builder
	out.WriteString(rfcProofLegend + "\n")
	for _, requirement := range rows {
		out.WriteString("\n### " + rfcRequirementRefMirror(requirement.RID, "") + "\n\n")
		out.WriteString(requirement.Text + "\n\n")
		out.WriteString("Audit verdict: " + rfcVerdictText(requirement.Audit) + "\n")
		if len(requirement.Covers) == 0 {
			out.WriteString("\nNo test carries " + requirement.RID +
				", so no unit is bound to it.\n")
			continue
		}
		out.WriteString("\n" + rfcMirrorHead("Polarity", "Test",
			"Kind and tier", "Proof state"))
		for index := range requirement.Covers {
			cover := &requirement.Covers[index]
			out.WriteString(rfcMirrorRow(cover.Polarity,
				rfc.TableCell(rfcCitationMirror(cover, ambiguous)),
				rfcOrUnstated(cover.Carrier),
				rfc.TableCell(rfcProofStateText(cover))))
		}
	}
	return out.String()
}

// rfcExtractionHTML renders the sign-off that decided which sentences of the
// RFC became requirements at all.
func rfcExtractionHTML(entry *rfcLedgerStem) string {
	if entry.Extraction == nil {
		return "<p>" + html.EscapeString("No extraction sign-off exists for "+entry.Display+
			", so no reviewer has walked its text sentence by sentence.") + "</p>"
	}
	signoff := entry.Extraction
	var facts strings.Builder
	for _, fact := range rfcExtractionFacts(signoff) {
		facts.WriteString(rfcRowCells(html.EscapeString(fact[0]),
			html.EscapeString(rfcOrUnstated(fact[1]))))
	}
	var out strings.Builder
	out.WriteString(rfcTableHTML(rfcHeadCells("Field", "Value"), facts.String()) + "\n")

	out.WriteString("<h3>Sections</h3>\n")
	if len(signoff.Sections) == 0 {
		out.WriteString("<p>" + html.EscapeString(rfcNoSectionsText(entry)) + "</p>\n")
	} else {
		var body strings.Builder
		for _, section := range signoff.Sections {
			body.WriteString(rfcRowCells("<code>"+html.EscapeString(section.ID)+"</code>",
				html.EscapeString(rfcOrUnstated(section.Name)), strconv.Itoa(section.Sites),
				html.EscapeString(rfcOrUnstated(rfcSectionDisposition(&section))),
				html.EscapeString(rfcOrUnstated(section.Reason))))
		}
		out.WriteString(rfcTableHTML(rfcHeadCells("Section", "Name", "Sites",
			"Disposition", "Reason"), body.String()) + "\n")
	}

	out.WriteString("<h3>Excluded sentences</h3>\n")
	if len(signoff.Exclusions) == 0 {
		out.WriteString("<p>" + html.EscapeString(rfcNoExclusionsText(entry)) + "</p>")
		return out.String()
	}
	var body strings.Builder
	for _, site := range signoff.Exclusions {
		body.WriteString(rfcRowCells("<code>"+html.EscapeString(site.ID)+"</code>",
			rfcExclusionKindHTML(site.Kind),
			html.EscapeString(rfcOrUnstated(rfcExclusionReason(&site))),
			html.EscapeString(rfcOrUnstated(site.Quote))))
	}
	out.WriteString(rfcTableHTML(rfcHeadCells("Site", "Excluded kind", "Reason", "Quote"),
		body.String()))
	return out.String()
}

// rfcExclusionKindHTML names one exclusion kind and what it MEANS.
//
// The kind is the project's own word and means nothing to a reader outside it,
// so the sentence internal/le/rfc declares beside the word comes with it. A
// kind the repository's own rule presumes wrong says so here, where its
// justification is the cell beside it (ai/rules/rfc-compliance.md).
func rfcExclusionKindHTML(kind string) string {
	if strings.TrimSpace(kind) == "" {
		return html.EscapeString(rfcOrUnstated(kind))
	}
	out := "<code>" + html.EscapeString(kind) + "</code>"
	if group, held := rfc.ExclusionKindGroup(kind); held {
		out += "<br /><strong>" + html.EscapeString(rfcExclusionGroupWord(group)) + "</strong>"
	}
	if meaning, held := rfc.ExclusionKindMeaning(kind); held {
		out += "<br />" + html.EscapeString(meaning)
	}
	if rfc.ExclusionPresumedWrong(kind) {
		out += "<br /><strong>" + html.EscapeString(rfcPresumedWrongMark) + "</strong>"
	}
	return out
}

// rfcExclusionKindMirror states the same kind and the same meaning.
func rfcExclusionKindMirror(kind string) string {
	if strings.TrimSpace(kind) == "" {
		return rfcOrUnstated(kind)
	}
	out := "`" + kind + "`"
	if group, held := rfc.ExclusionKindGroup(kind); held {
		out += " (" + rfcExclusionGroupWord(group) + ")"
	}
	if meaning, held := rfc.ExclusionKindMeaning(kind); held {
		out += ": " + meaning
	}
	if rfc.ExclusionPresumedWrong(kind) {
		out += ". " + rfcPresumedWrongMark
	}
	return out
}

// rfcPresumedWrongMark is what a kind the repository presumes wrong carries
// beside it, so a reader meets the caution where the reason is.
const rfcPresumedWrongMark = "Presumed wrong until justified: Ze rarely implements one side " +
	"of a protocol, so the reason beside this row must name the role, show Ze never acts as " +
	"it, and cite the producer that would."

// rfcExtractionMirror states the sign-off the page states.
func rfcExtractionMirror(entry *rfcLedgerStem) string {
	if entry.Extraction == nil {
		return "No extraction sign-off exists for " + entry.Display +
			", so no reviewer has walked its text sentence by sentence.\n"
	}
	signoff := entry.Extraction
	var out strings.Builder
	out.WriteString(rfcMirrorHead("Field", "Value"))
	for _, fact := range rfcExtractionFacts(signoff) {
		out.WriteString(rfcMirrorRow(fact[0], rfc.TableCell(rfcOrUnstated(fact[1]))))
	}

	out.WriteString("\n### Sections\n\n")
	if len(signoff.Sections) == 0 {
		out.WriteString(rfcNoSectionsText(entry) + "\n")
	} else {
		out.WriteString(rfcMirrorHead("Section", "Name", "Sites", "Disposition", "Reason"))
		for _, section := range signoff.Sections {
			out.WriteString(rfcMirrorRow("`"+section.ID+"`",
				rfc.TableCell(rfcOrUnstated(section.Name)), strconv.Itoa(section.Sites),
				rfcOrUnstated(rfcSectionDisposition(&section)),
				rfc.TableCell(rfcOrUnstated(section.Reason))))
		}
	}

	out.WriteString("\n### Excluded sentences\n\n")
	if len(signoff.Exclusions) == 0 {
		out.WriteString(rfcNoExclusionsText(entry) + "\n")
		return out.String()
	}
	out.WriteString(rfcMirrorHead("Site", "Excluded kind", "Reason", "Quote"))
	for _, site := range signoff.Exclusions {
		out.WriteString(rfcMirrorRow("`"+site.ID+"`",
			rfc.TableCell(rfcExclusionKindMirror(site.Kind)),
			rfc.TableCell(rfcOrUnstated(rfcExclusionReason(&site))),
			rfc.TableCell(rfcOrUnstated(site.Quote))))
	}
	return out.String()
}

// rfcExtractionFacts answers the sign-off census, read by both renderings.
func rfcExtractionFacts(signoff *rfcLedgerExtraction) [][2]string {
	return [][2]string{
		{"Reviewer", signoff.Reviewer},
		{"Signed off", signoff.SignedOff},
		{"Register", signoff.Register},
		{"Source", signoff.SourcePath},
		{"Source fingerprint", signoff.SourceSHA},
		{"Record", signoff.Path},
		{"Mapped sentences", strconv.Itoa(signoff.Mapped)},
		{"Declined as scope", strconv.Itoa(signoff.Excluded - signoff.Relocated)},
		{"Relocated to a spec, which Ze OWES", strconv.Itoa(signoff.Relocated)},
		{"Unclassified", strconv.Itoa(signoff.Unclassified)},
	}
}

// rfcNoSectionsText and rfcNoExclusionsText each state an empty sub-section in
// one sentence.
func rfcNoSectionsText(entry *rfcLedgerStem) string {
	return "The sign-off for " + entry.Display + " classifies no section."
}

func rfcNoExclusionsText(entry *rfcLedgerStem) string {
	return "The walk over " + entry.Display +
		" declined no sentence: every site it found is mapped to a requirement."
}

// rfcSectionDisposition says what the reviewer did with one section, with the
// skip kind beside it where the section was skipped.
func rfcSectionDisposition(section *rfcLedgerSection) string {
	if section.SkipKind == "" {
		return section.Disposition
	}
	return section.Disposition + " (" + section.SkipKind + ")"
}

// rfcExclusionReason says why one sentence was not mapped, and names the spec
// that owes it where the exclusion relocated the obligation.
func rfcExclusionReason(site *rfcLedgerExcludedSite) string {
	if site.RelocatedTo == "" {
		return site.Reason
	}
	return site.Reason + " (relocated to " + site.RelocatedTo + " as " + site.ReservedID + ")"
}

// rfcSupersededHTML renders where this document's obligations live now.
func rfcSupersededHTML(entry *rfcLedgerStem) string {
	rows := rfcSupersededRows(entry)
	if len(rows) == 0 {
		return "<p>" + html.EscapeString(rfcNoSupersededText(entry)) + "</p>"
	}
	var out strings.Builder
	out.WriteString("<p>" + html.EscapeString(entry.Display+" is obsoleted by "+
		rfcDisplayName(entry.Successor)+".") + "</p>\n")
	var body strings.Builder
	for _, row := range rows {
		body.WriteString(rfcRowCells(rfcRequirementRefHTML(row.RID, row.Text),
			html.EscapeString(row.Superseded.Disposition),
			html.EscapeString(rfcOrUnstated(row.Superseded.Target)),
			html.EscapeString(row.Superseded.Reason)))
	}
	out.WriteString(rfcTableHTML(rfcHeadCells("Requirement", "Disposition", "Now stated at",
		"Reason"), body.String()))
	return out.String()
}

// rfcSupersededMirror states the same forward pointers.
func rfcSupersededMirror(entry *rfcLedgerStem) string {
	rows := rfcSupersededRows(entry)
	if len(rows) == 0 {
		return rfcNoSupersededText(entry) + "\n"
	}
	var out strings.Builder
	out.WriteString(entry.Display + " is obsoleted by " + rfcDisplayName(entry.Successor) + ".\n\n")
	out.WriteString(rfcMirrorHead("Requirement", "Disposition", "Now stated at", "Reason"))
	for _, row := range rows {
		out.WriteString(rfcMirrorRow(rfc.TableCell(rfcRequirementRefMirror(row.RID, row.Text)),
			row.Superseded.Disposition, rfcOrUnstated(row.Superseded.Target),
			rfc.TableCell(row.Superseded.Reason)))
	}
	return out.String()
}

// rfcNoSupersededText states an empty Superseded section in one sentence,
// which says whether a successor exists as well as that no requirement moved.
func rfcNoSupersededText(entry *rfcLedgerStem) string {
	if entry.Successor == "" {
		return "No document obsoletes " + entry.Display +
			", so its obligations are stated where they were written."
	}
	return entry.Display + " is obsoleted by " + rfcDisplayName(entry.Successor) +
		", and no requirement of this summary carries a forward pointer to it."
}

// rfcSupersededRows answers every requirement carrying a forward pointer.
func rfcSupersededRows(entry *rfcLedgerStem) []*rfcLedgerRequirement {
	var rows []*rfcLedgerRequirement
	for index := range entry.Requirements {
		if entry.Requirements[index].Superseded != nil {
			rows = append(rows, &entry.Requirements[index])
		}
	}
	return rows
}
