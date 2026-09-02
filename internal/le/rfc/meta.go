// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
// Detail: render_ledger.go -- the three files this declaration generates
//
// meta.go reads the `## Meta` table of rfc/short/<stem>.md. That table is where
// a summary declares every fact about ITSELF: its title, its lineage, whether
// its obligations are gated, and what the public page claims for it.
//
// One reader for the whole table, rather than one regex per field. The per-field
// readers this replaces were asymmetric in the direction that hides defects: an
// absent field returned an empty value with no error, so nothing could tell a
// summary that declares a fact from one that omits it, and a stem could leave a
// gate's population while the gate read clean
// (plan/journal/gate-excludes-part-of-its-population.md).
package rfc

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// enrolmentEnrolled gates that RFC's MUST-level requirements. Every other
// member of enrolmentKinds is the recorded reason it is not gated.
//
// Two of those make a claim about the DOCUMENT and therefore excuse a public
// support claim: `non-normative` about what it obliges, `source-restricted`
// about whether its text can be held here at all. One is a SCOPE decision,
// `out-of-scope`. The remaining two are debt.
const enrolmentEnrolled = "enrolled"

// enrolmentKinds is the closed set, and the ONE declaration of it. The four
// un-enrolled kinds were declared a second time in ledger.go until 2026-09-01,
// beside the reader that parsed rfc/not-enrolled.txt; that reader is gone and a
// second copy of a closed set is what this whole change exists to remove.
var enrolmentKinds = map[string]bool{
	enrolmentEnrolled:           true,
	dispositionBacklog:          true,
	dispositionBlocked:          true,
	dispositionNonNormative:     true,
	dispositionSourceRestricted: true,
	dispositionOutOfScope:       true,
}

// enrolmentKindNames answers the closed set sorted, so every refusal naming it
// names it in one order.
func enrolmentKindNames() []string { return sortedSet(enrolmentKinds) }

// The Meta labels this package reads. Each names one fact, and each is the ONLY
// spelling that names it: a near-miss is refused rather than skipped, by the
// same rule the obsolescence labels have carried since three summaries naming a
// real successor were read as current.
const (
	metaTitle           = "Title"
	metaEnrolment       = "Enrolment"
	metaEnrolmentReason = "Enrolment reason"
	metaSupport         = "Support"
	metaSupportName     = "Support name"
	metaSupportArea     = "Support area"
	metaSupportStatus   = "Support status"
	metaSupportCoverage = "Support coverage"
	metaSupportRemain   = "Support remaining"
)

// supportNone is the `Support` value of a summary that makes no public claim.
// Written rather than omitted: 39 summaries have no row on the public page, and
// "this RFC deliberately has no row" must not look like "somebody forgot".
const supportNone = "-"

// metaRow is one `| Label | Value |` row of the Meta table.
type metaRow struct {
	Label string
	Value string
}

// splitMetaCells cuts one `| label | value |` line into its cells, splitting on
// an UNESCAPED pipe only.
//
// A regex over `[^|]*` cannot answer this. One public support cell writes
// `MD5(id\|\|secret\|\|challenge)`, which is a pipe a markdown table escapes
// rather than a cell boundary; reading it as a boundary is what silently
// truncated that row's coverage cell and discarded its remainder for as long as
// the public page was authored.
func splitMetaCells(line string) []string {
	if !strings.HasPrefix(strings.TrimSpace(line), "|") {
		return nil
	}
	line = strings.TrimSpace(line)
	var cells []string
	var cell textbuf.Buffer
	escaped := false
	for index := 1; index < len(line); index++ {
		char := line[index]
		switch {
		case escaped:
			cell.Byte('\\').Byte(char)
			escaped = false
		case char == '\\':
			escaped = true
		case char == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
		default:
			cell.Byte(char)
		}
	}
	if rest := strings.TrimSpace(cell.String()); rest != "" {
		cells = append(cells, rest)
	}
	return cells
}

// metaHeadingRE finds the `## Meta` heading.
var metaHeadingRE = regexp.MustCompile(`(?mi)^##\s+Meta\s*$`)

// metaSection answers the FIRST contiguous run of table rows under `## Meta`,
// and stops at the first line that is not one.
//
// The bound is the table itself rather than the next heading, because a summary
// writes more than one table under that heading: rfc/short/rfc8277.md states its
// AFI/SAFI scope as a second table whose first column repeats, and a reader
// bounded by the next heading takes `| 1 |` for a Meta field name and refuses
// the file for a duplicate that is not one. A `###` subheading between the two
// would not have saved it either, since it is not the `## ` the bound looked
// for. One table is what a Meta table is, so one table is what this reads.
func metaSection(text, where string) (string, error) {
	head := metaHeadingRE.FindStringIndex(text)
	if head == nil {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(where).
			Str(": has no `## Meta` heading. The Meta table is where a summary declares ").
			Str("its own title, its lineage and its ledger rows, so a summary without one ").
			Str("declares nothing and every fact about it would read as absent"))
	}
	var block textbuf.Buffer
	started := false
	for line := range strings.SplitSeq(text[head[1]:], "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			if started {
				break
			}
			// Only blank lines separate the heading from its own table. Any
			// other prose means the Meta table is absent and the next table
			// belongs to a later section, so this stops rather than
			// adopting it: reading a wire-format or AFI/SAFI table as the
			// Meta table is how a summary silently declares nothing.
			if trimmed == "" {
				continue
			}
			break
		}
		started = true
		block.Str(line).Byte('\n')
	}
	if !started {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(where).
			Str(": the `## Meta` heading carries no table. Every fact this reader needs is ").
			Str("a row of that table, so a heading with nothing under it declares nothing"))
	}
	return block.String(), nil
}

// metaRows answers every row of the Meta table, in file order, refusing a
// duplicated label.
func metaRows(text, where string) ([]metaRow, error) {
	section, err := metaSection(text, where)
	if err != nil {
		return nil, err
	}
	var rows []metaRow
	seen := map[string]bool{}
	for line := range strings.SplitSeq(section, "\n") {
		cells := splitMetaCells(line)
		if len(cells) < 2 {
			continue
		}
		label := cells[0]
		if label == "" || label == "Field" || strings.HasPrefix(label, "---") {
			continue
		}
		if seen[label] {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(where).Str(": Meta field `").Str(label).
				Str("` appears twice. One row per fact: two rows can carry two values ").
				Str("and nothing decides between them"))
		}
		seen[label] = true
		if len(cells) > 2 {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(where).Str(": Meta field `").Str(label).
				Str("` holds an unescaped `|`, so its value would be truncated at ").
				Str(pyRepr(truncateRunes(cells[1], 40))).
				Str(" and the rest dropped in silence. Write it as `\\|`. This is the defect ").
				Str("the authored public page carried: one coverage cell wrote `MD5(id||secret)` ").
				Str("and the page's own parser discarded that row's whole remainder"))
		}
		rows = append(rows, metaRow{Label: label, Value: cells[1]})
	}
	return rows, nil
}

// Meta is what one summary's Meta table declares.
//
// Every field this package reads is here, and a caller that wants one of them
// takes it from this one parse. Two readers of the same table can disagree
// about what it says; one cannot.
type Meta struct {
	Title string
	// Successor is the stem of the document that obsoletes this one, and is
	// empty when nothing does.
	Successor string
	// Enrolment is a member of enrolmentKinds, and EnrolmentReason is why.
	Enrolment       string
	EnrolmentReason string
	// Support is the public page section this RFC's row renders under, and is
	// empty when the summary claims no row. Rank orders it inside that section.
	Support string
	Rank    int
	// Name overrides the first cell the section would derive from the stem.
	// Empty everywhere the derivation is right, which is every stem but one.
	Name string
	// The four authored cells of the public row, moved verbatim. Nothing here
	// is derived and nothing here is rewritten: a public support claim is
	// editorial prose, and a generator that synthesized it would be inventing
	// the claim it exists to publish.
	Area      string
	Status    string
	Coverage  string
	Remaining string
}

// Enrolled answers whether this summary's MUST-level requirements are gated.
func (m Meta) Enrolled() bool { return m.Enrolment == enrolmentEnrolled }

// OutOfScope answers whether the owner decided not to offer this document's
// feature for now. Such a summary declares its obligations in full and gates
// none of them, so a new one is not asked to enroll.
func (m Meta) OutOfScope() bool { return m.Enrolment == dispositionOutOfScope }

// HasRow answers whether this summary renders a row on the public page.
func (m Meta) HasRow() bool { return m.Support != "" }

// Disposition answers the recorded reason this summary is not gated, for a
// summary that is not enrolled.
func (m Meta) Disposition() Disposition {
	return Disposition{Kind: m.Enrolment, Reason: m.EnrolmentReason}
}

// nearMissRE is the label a reader must not silently skip: anything naming
// enrolment, support or obsolescence that is not one of the exact labels above.
var nearMissRE = regexp.MustCompile(`(?i)enrol|support|obsolet`) //nolint:misspell // `enrol` matches Enrolment and Enrolled; `enroll` misses Enrolment.

// knownMetaLabels are every label nearMissRE is allowed to match.
var knownMetaLabels = map[string]bool{
	metaEnrolment: true, metaEnrolmentReason: true, metaSupport: true,
	metaSupportName: true, metaSupportArea: true, metaSupportStatus: true,
	metaSupportCoverage: true, metaSupportRemain: true,
}

// knownObsolescenceLabel is the pair of lineage labels that carry a qualifier,
// which is why they are matched by prefix where the eight above are exact.
var knownObsolescenceLabel = regexp.MustCompile(`(?i)^\s*Obsoletes\b|^\s*Obsoleted[ -]by\b`)

// ParseMeta reads one summary's Meta table into the facts this package needs.
//
// It REFUSES rather than defaulting, at every field. An absent enrolment field
// that defaulted to "not enrolled" would take an RFC out of the gated
// population with no author intending it and no gate saying so, which is the
// largest recorded class of defect in this repository.
func ParseMeta(text, stem, where string) (Meta, error) {
	if where == "" {
		where = stem
	}
	rows, err := metaRows(text, where)
	if err != nil {
		return Meta{}, err
	}
	values := map[string]string{}
	for _, row := range rows {
		if err := refuseNearMiss(row.Label, where); err != nil {
			return Meta{}, err
		}
		values[row.Label] = row.Value
	}
	out := Meta{Title: values[metaTitle], Name: values[metaSupportName]}
	if out.Successor, err = successorFrom(values, stem, where); err != nil {
		return Meta{}, err
	}
	if err := readEnrolment(values, &out, where); err != nil {
		return Meta{}, err
	}
	if err := readSupport(values, &out, stem, where); err != nil {
		return Meta{}, err
	}
	return out, nil
}

// refuseNearMiss reds a label that names one of this reader's facts in a
// spelling nothing reads.
//
// The failure it closes is silence: a field skipped because its label was
// unrecognized leaves the fact absent, and an absent fact reads exactly like a
// deliberate omission.
func refuseNearMiss(label, where string) error {
	if !nearMissRE.MatchString(label) {
		return nil
	}
	if knownMetaLabels[label] || knownObsolescenceLabel.MatchString(label) {
		return nil
	}
	known := sortedSet(knownMetaLabels)
	var tb textbuf.Buffer
	return parseErr(tb.Str(where).Str(": Meta field `").Str(label).
		Str("` names enrolment, support or obsolescence in a spelling nothing reads, ").
		Str("so the row would be skipped in silence. The labels this reader knows are ").
		Str(pyRepr(known)).Str(", plus `Obsoletes` and `Obsoleted by` with an optional qualifier"))
}

// readEnrolment fills the two enrolment fields, refusing an absent value, a
// value outside the closed set, and a kind with no reason.
func readEnrolment(values map[string]string, out *Meta, where string) error {
	kind, held := values[metaEnrolment]
	if !held {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": the Meta table has no `").Str(metaEnrolment).
			Str("` row. Every summary declares whether its MUST-level requirements are ").
			Str("gated: write one of ").Str(pyRepr(enrolmentKindNames())).
			Str(". There is no default -- an un-enrolled summary must be a decision, ").
			Str("never an absence"))
	}
	if !enrolmentKinds[kind] {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaEnrolment).Str("` is ").Str(pyRepr(kind)).
			Str(", which is not one of ").Str(pyRepr(enrolmentKindNames())).
			Str(". Use 'enrolled' to gate the RFC's MUSTs, 'non-normative' when the ").
			Str("DOCUMENT imposes none, 'backlog' when the extraction is owed, 'blocked' ").
			Str("when something outside the summary prevents enrolment, and ").
			Str("'source-restricted' when the standard's own text may not be redistributed ").
			Str("so no enrolment is ever reachable, and 'out-of-scope' when the extraction ").
			Str("is done and the owner decided not to offer the feature for now"))
	}
	reason := values[metaEnrolmentReason]
	if reason == "" {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaEnrolment).Str("` is ").Str(kind).
			Str(" with no `").Str(metaEnrolmentReason).
			Str("` row. A bare kind is an absence with a label on it: say what makes it true"))
	}
	out.Enrolment = kind
	out.EnrolmentReason = reason
	return nil
}

// readSupport fills the public row, refusing an absent declaration, an unknown
// section, a malformed rank, and any mismatch between the section's table shape
// and the cells the summary declares.
func readSupport(values map[string]string, out *Meta, stem, where string) error {
	declared, held := values[metaSupport]
	if !held {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": the Meta table has no `").Str(metaSupport).
			Str("` row. Every summary declares whether it renders a row on ").Str(statusRel).
			Str(": write `").Str(supportNone).Str("` for no row, or `<section> <rank>` with ").
			Str("a section from ").Str(pyRepr(statusSectionKeys())))
	}
	if declared == supportNone {
		return refuseOrphanCells(values, out, where)
	}
	fields := strings.Fields(declared)
	if len(fields) != 2 {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaSupport).Str("` is ").Str(pyRepr(declared)).
			Str(", which is neither `").Str(supportNone).Str("` nor `<section> <rank>`. The rank ").
			Str("orders this RFC inside its section, because the page's reading order is ").
			Str("authored rather than alphabetical"))
	}
	section := statusSection(fields[0])
	if section == nil {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaSupport).Str("` names section ").
			Str(pyRepr(fields[0])).Str(", which is not one of ").Str(pyRepr(statusSectionKeys())))
	}
	rank, err := strconv.Atoi(fields[1])
	if err != nil || rank <= 0 {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaSupport).Str("` rank ").Str(pyRepr(fields[1])).
			Str(" is not a positive whole number"))
	}
	out.Support = section.Key
	out.Rank = rank
	out.Area = values[metaSupportArea]
	out.Status = values[metaSupportStatus]
	out.Coverage = values[metaSupportCoverage]
	out.Remaining = values[metaSupportRemain]
	return checkRowCells(out, section, stem, where)
}

// refuseOrphanCells refuses a support cell on a summary that declares no row.
//
// A cell nothing renders is a claim nobody reads, and an author who wrote one
// believes their RFC is on the public page when it is not.
func refuseOrphanCells(values map[string]string, out *Meta, where string) error {
	for _, label := range []string{metaSupportName, metaSupportArea, metaSupportStatus,
		metaSupportCoverage, metaSupportRemain} {
		if _, held := values[label]; !held {
			continue
		}
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaSupport).Str("` is `").Str(supportNone).
			Str("`, so this summary renders no row on ").Str(statusRel).Str(", but it carries a `").
			Str(label).Str("` row. Nothing renders that cell: name a section, or delete the cell"))
	}
	out.Name = ""
	return nil
}

// checkRowCells refuses a row whose cells do not match its section's shape.
func checkRowCells(out *Meta, section *statusSectionSpec, stem, where string) error {
	for label, value := range map[string]string{
		metaSupportArea: out.Area, metaSupportStatus: out.Status, metaSupportCoverage: out.Coverage,
	} {
		if value != "" {
			continue
		}
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaSupport).
			Str("` names a section, so this summary renders a public row, but its `").
			Str(label).Str("` cell is absent or empty. A blank cell on the public page ").
			Str("states nothing where a reader expects a claim"))
	}
	if section.Brief && out.Remaining != supportNone {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": section ").Str(pyRepr(section.Key)).
			Str(" renders four columns and has no `Remaining` column, so `").
			Str(metaSupportRemain).Str("` must be `").Str(supportNone).Str("`, not ").
			Str(pyRepr(truncateRunes(out.Remaining, 40))))
	}
	if !section.Brief && out.Remaining == "" {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaSupportRemain).
			Str("` is absent. Write what is not complete, or `").Str(supportNone).
			Str("` when the row claims nothing is"))
	}
	if out.Name != "" && statusRowRFC.MatchString(stem) {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaSupportName).Str("` is ").Str(pyRepr(out.Name)).
			Str(", but an `rfc<number>` stem derives its own first cell. Delete the row"))
	}
	return nil
}

// statusRowRFC is the stem shape whose public first cell is derived.
var statusRowRFC = regexp.MustCompile(`\Arfc(\d+)\z`)

// RowName answers the first cell of this summary's public row.
//
// Derived from the stem, because the stem already says which document this is
// and a second spelling of it is a future disagreement. The override exists for
// the one row whose name a stem cannot carry: ISO/IEC 10589 is not an RFC, not
// a draft, and its published name holds a slash, a space and upper case.
func RowName(stem string, meta Meta) string {
	if meta.Name != "" {
		return meta.Name
	}
	if found := statusRowRFC.FindStringSubmatch(stem); found != nil {
		var tb textbuf.Buffer
		return tb.Str("RFC ").Str(found[1]).String()
	}
	return stem
}

// successorFrom reads the forward lineage row out of the parsed Meta table.
//
// Same answer parseSuccessorStem gave over the raw text, from the one parse:
// empty when no row exists or the row says nothing obsoletes this document, and
// the LAST reference of a chain written oldest first, because that is the
// document stating these obligations today.
func successorFrom(values map[string]string, stem, where string) (string, error) {
	// Sorted, and REFUSING a second forward label rather than taking whichever
	// the map walk reached first. `Obsoleted by` and `Obsoleted by (in part)`
	// are both spellings knownObsolescenceLabel accepts, so a summary carrying
	// both would name a different successor on different runs.
	value := ""
	held := false
	for _, label := range sortedKeysOf(values) {
		if !strings.HasPrefix(strings.ToLower(label), "obsoleted") {
			continue
		}
		if held {
			var tb textbuf.Buffer
			return "", parseErr(tb.Str(where).
				Str(": two Meta fields name the forward lineage, and nothing decides between ").
				Str("them. Write one `Obsoleted by` row, with the chain oldest first"))
		}
		value, held = cell(values, label), true
	}
	if !held || noSuccessorValue(value) {
		return "", nil
	}
	refs := rfcRefRE.FindAllStringSubmatch(value, -1)
	if len(refs) == 0 {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(where).Str(": the forward Meta row says ").Str(pyRepr(value)).
			Str(", which names no RFC. Write the chain of obsoleting documents oldest ").
			Str("first, or write `None`"))
	}
	var name textbuf.Buffer
	successor := name.Str("rfc").Str(refs[len(refs)-1][1]).String()
	if successor == stem {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(where).Str(": the forward Meta row ends at ").Str(successor).
			Str(", which is this document. A summary cannot obsolete itself"))
	}
	return successor, nil
}

// sortMetaStems orders a stem set for a message, so two runs name it alike.
func sortMetaStems(in map[string]Meta) []string {
	out := make([]string, 0, len(in))
	for stem := range in {
		out = append(out, stem)
	}
	sort.Strings(out)
	return out
}

// summaryMetas reads every summary's Meta table once.
//
// The ONE reader. Enrolment, the disposition, the public row, the title and the
// forward lineage all come out of this map, so no two consumers can read the
// same table and disagree about what it says.
func summaryMetas(tree string, stems map[string]bool) (map[string]Meta, map[string]string, error) {
	if stems == nil {
		var err error
		if stems, err = summaryStems(tree); err != nil {
			return nil, nil, err
		}
	}
	out := make(map[string]Meta, len(stems))
	problems := map[string]string{}
	for _, stem := range sortedSet(stems) {
		var name textbuf.Buffer
		rel := name.Str(summaryRel).Byte('/').Str(stem).Str(".md").String()
		text, err := readFile(treePath(tree, rel), rel)
		if err != nil {
			return nil, nil, err
		}
		meta, err := ParseMeta(text, stem, rel)
		if err != nil {
			// COLLECTED, not returned. Several sessions share this checkout,
			// so one summary somebody is midway through editing would
			// otherwise stop `./le rfc check` for everybody, and a gate
			// nobody can run enforces nothing. The stem is absent from the
			// map, which takes it out of the gated population -- but LOUDLY,
			// because its error travels with the other parse errors and
			// every driver prints them.
			//
			// The WRITE is the exception and refuses: NewRenderInput would
			// emit a ledger with that RFC missing, and `checkLedgerFresh`
			// answers a finding rather than an error so the refusal reaches
			// the writer without taking `check` down with it.
			problems[stem] = err.Error()
			continue
		}
		out[stem] = meta
	}
	return out, problems, nil
}

// enrolledFrom answers the gated set.
func enrolledFrom(metas map[string]Meta) map[string]bool {
	out := map[string]bool{}
	for stem := range metas {
		if metas[stem].Enrolled() {
			out[stem] = true
		}
	}
	return out
}

// enrolmentReasonsFrom answers why each gated RFC is gated.
func enrolmentReasonsFrom(metas map[string]Meta) map[string]string {
	out := map[string]string{}
	for stem := range metas {
		if metas[stem].Enrolled() {
			out[stem] = metas[stem].EnrolmentReason
		}
	}
	return out
}

// dispositionsFrom answers the recorded reason each un-enrolled summary is not
// gated.
//
// An enrolled stem is ABSENT rather than present with an empty kind, which is
// what makes "in both files" unrepresentable: one field holds one value, so a
// summary cannot be enrolled and declared at the same time.
func dispositionsFrom(metas map[string]Meta) map[string]Disposition {
	out := map[string]Disposition{}
	for stem := range metas {
		if !metas[stem].Enrolled() {
			out[stem] = metas[stem].Disposition()
		}
	}
	return out
}

// rowsFrom answers the public page row each summary declares, by the three
// cells the checks read.
func rowsFrom(metas map[string]Meta) map[string]LedgerRow {
	out := map[string]LedgerRow{}
	for stem := range metas {
		if !metas[stem].HasRow() {
			continue
		}
		remaining := metas[stem].Remaining
		if remaining == supportNone {
			remaining = ""
		}
		out[stem] = LedgerRow{
			Status: metas[stem].Status, Coverage: metas[stem].Coverage, Remaining: remaining,
		}
	}
	return out
}

// titlesFrom answers each summary's own Meta title.
//
// A stem whose summary declares none is ABSENT rather than present with an
// empty value, so a caller can tell "this summary states no title" from "this
// stem has no summary" (ai/rules/principles.md).
func titlesFrom(metas map[string]Meta) map[string]string {
	out := map[string]string{}
	for stem := range metas {
		if metas[stem].Title != "" {
			out[stem] = metas[stem].Title
		}
	}
	return out
}

// successorsFrom answers {stem: the stem that obsoletes it} over every summary
// that declares one.
func successorsFrom(metas map[string]Meta) map[string]string {
	out := map[string]string{}
	for stem := range metas {
		if metas[stem].Successor != "" {
			out[stem] = metas[stem].Successor
		}
	}
	return out
}

// cell answers one Meta value, so successorFrom reads the map in sorted order
// without indexing it inline twice.
func cell(values map[string]string, label string) string { return values[label] }
