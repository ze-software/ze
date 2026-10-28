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
	"slices"
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

// The four kinds a summary declares about WHO implements the document (owner
// directive, 2026-09-21). The ledger answers one question, whether ZE's Go code
// does what the RFC says, so a document Ze writes no Go for leaves the count
// and names what implements it instead.
//
// This is the DOCUMENT-level parent of the requirement-level `{lower-layer}`
// annotation, and the two never decide one row. That annotation governs an
// obligation whose role Ze fills and which a layer under Ze meets on state Ze
// installs: it stays counted and met, per the owner directive of 2026-08-31. A
// document Ze merely configures is a document Ze implements part of, which is
// implementationMixed.
const (
	// implementationZe is the ordinary case: Ze's Go answers the document.
	implementationZe = "ze"
	// implementationMixed splits one document. Ze's part is counted and the
	// rest names its implementer, as RFC 4303 does: Ze builds and installs the
	// security association in Go and the kernel moves every packet.
	implementationMixed = "mixed"
	// implementationThirdParty says a layer under or beside Ze performs the
	// behavior and Ze holds no Go code for it.
	implementationThirdParty = "third-party"
	// implementationFoundation says the document defines, registers or
	// describes, and obliges no implementer. A document nobody implements while
	// Ze OWES the behavior is not this: that is a gap, and it stays on the
	// ledger until the behavior exists (ai/rules/rfc-compliance.md).
	implementationFoundation = "foundation"
)

// implementationKinds is the closed set, and the ONE declaration of it.
var implementationKinds = map[string]bool{
	implementationZe:         true,
	implementationMixed:      true,
	implementationThirdParty: true,
	implementationFoundation: true,
}

// implementationKindNames answers the closed set sorted, so every refusal
// naming it names it in one order.
func implementationKindNames() []string { return sortedSet(implementationKinds) }

// ImplementationKinds answers the closed set to a renderer, sorted.
func ImplementationKinds() []string { return implementationKindNames() }

// implementationNamesAnImplementer answers whether the kind owes the name of
// the component that performs what Ze's Go does not.
func implementationNamesAnImplementer(kind string) bool {
	return kind == implementationMixed || kind == implementationThirdParty
}

// The Meta labels this package reads. Each names one fact, and each is the ONLY
// spelling that names it: a near-miss is refused rather than skipped, by the
// same rule the obsolescence labels have carried since three summaries naming a
// real successor were read as current.
const (
	metaTitle                = "Title"
	metaUpdates              = "Updates"
	metaUpdatedBy            = "Updated by"
	metaObsoletes            = "Obsoletes"
	metaEnrolment            = "Enrolment"
	metaEnrolmentReason      = "Enrolment reason"
	metaImplementation       = "Implementation"
	metaImplementationReason = "Implementation reason"
	metaSupport              = "Support"
	metaSupportName          = "Support name"
	metaSupportArea          = "Support area"
	metaSupportStatus        = "Support status"
	metaSupportCoverage      = "Support coverage"
	metaSupportRemain        = "Support remaining"
	// metaAggregate names the aggregate of a summary whose `Support` is `-`:
	// it renders no row and still joins its aggregate's rollup.
	metaAggregate = "Aggregate"
	// metaUseCases names the use-case profiles that require this RFC: it is
	// counted in each one's figures. metaOptionalUseCases names those that
	// list it as optional: shown with its own figures, never counted.
	metaUseCases         = "Use cases"
	metaOptionalUseCases = "Optional use cases"
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
	// Updates is the stems of the RFCs this document declares it updates, from
	// the Meta `Updates` cell, in the order the cell writes them. Empty when the
	// cell is absent, `-` or `None`.
	Updates []string
	// Obsoletes and UpdatedBy are the RFC stems the `Obsoletes` and
	// `Updated by` cells name, parentheticals cut, in the order the cell
	// writes them. Read leniently, because these cells carry prose, and judged
	// against the RFC headers by checkHeaderLinks rather than here.
	Obsoletes []string
	UpdatedBy []string
	// Enrolment is a member of enrolmentKinds, and EnrolmentReason is why.
	Enrolment       string
	EnrolmentReason string
	// Implementation is a member of implementationKinds and says WHOSE code
	// answers the document. ImplementationReason is why, and for the two kinds
	// that name another layer it carries the implementer.
	Implementation       string
	ImplementationReason string
	// Aggregate is the technology aggregate this RFC belongs to, declared in
	// rfc/profiles/<Aggregate>.md. Every summary names one: a summary with a
	// public row names it in its `Support` cell, and one without names it in
	// its `Aggregate` cell. The public row renders under the aggregate's
	// heading, in the aggregate's table shape.
	Aggregate string
	// UseCases is the use-case profiles that require this RFC, counted in
	// each one's figures, and OptionalUseCases those that list it as
	// optional, never counted. Both sorted, both empty for `-`, and no name
	// is in both. These two rows are the only declaration of use-case
	// membership: no profile file lists a member, and no citation adds one.
	UseCases         []string
	OptionalUseCases []string
	// Row is whether this summary renders a row on the public page, which is
	// exactly a `Support` cell naming an aggregate rather than `-`.
	Row bool
	// Name overrides the first cell the row would derive from the stem.
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
//
// A document Ze writes no Go for is never gated, whatever its enrolment row
// says (owner directive, 2026-09-21). The ledger answers one question, whether
// ZE's Go code does what the RFC says, so a published share taken over
// obligations another layer owns measures the wrong thing.
func (m Meta) Enrolled() bool {
	return m.Enrolment == enrolmentEnrolled && implementationCounts(m.Implementation)
}

// implementationCounts answers whether a document declaring this kind has its
// obligations counted against Ze's own Go. It is the ONE declaration of that
// division: Enrolled reads it for a summary, and Disposition reads it for the
// kind that took one off the gate.
//
// `mixed` counts. A split document keeps the requirements Ze's Go answers, and
// the rest carry the requirement-level annotations that name the layer meeting
// them: `{lower-layer}` for an obligation a layer under Ze meets on state Ze
// installs, which the 2026-08-31 directive keeps MET and counted.
func implementationCounts(kind string) bool {
	return kind == implementationZe || kind == implementationMixed
}

// OutOfScope answers whether the owner decided not to offer this document's
// feature for now. Such a summary declares its obligations in full and gates
// none of them, so a new one is not asked to enroll.
func (m Meta) OutOfScope() bool { return m.Enrolment == dispositionOutOfScope }

// HasRow answers whether this summary renders a row on the public page.
func (m Meta) HasRow() bool { return m.Row }

// Disposition answers the recorded reason this summary is not gated, for a
// summary that is not enrolled.
//
// Two facts take a document off the gate, and the kind names whichever one did
// it. The enrolment row is the first. The second is the document Ze writes no
// Go for: such a summary declares `enrolled` and is still not gated (owner
// directive, 2026-09-21), so reading the enrolment cell alone publishes the
// word `enrolled` as the reason a summary is not enrolled.
func (m Meta) Disposition() Disposition {
	// The enrolment row is the first fact, and where it says anything but
	// `enrolled` it names the reason itself.
	if m.Enrolment != enrolmentEnrolled {
		return Disposition{Kind: m.Enrolment, Reason: m.EnrolmentReason}
	}
	// The row says `enrolled` and the summary reached this answer anyway, so
	// the document Ze writes no Go for is what took it off the gate.
	return Disposition{Kind: m.Implementation, Reason: m.ImplementationReason}
}

// nearMissRE is the label a reader must not silently skip: anything naming
// enrolment, support, obsolescence, an aggregate, a use case or a profile that
// is not one of the exact labels above.
var nearMissRE = regexp.MustCompile(`(?i)enrol|support|obsolet|aggregat|use.?case|profile`) //nolint:misspell // `enrol` matches Enrolment and Enrolled; `enroll` misses Enrolment.

// knownMetaLabels are every label nearMissRE is allowed to match.
var knownMetaLabels = map[string]bool{
	metaEnrolment: true, metaEnrolmentReason: true, metaSupport: true,
	metaSupportName: true, metaSupportArea: true, metaSupportStatus: true,
	metaSupportCoverage: true, metaSupportRemain: true,
	metaImplementation: true, metaImplementationReason: true, metaAggregate: true,
	metaUseCases: true, metaOptionalUseCases: true,
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
	if out.Updates, err = updatesFrom(values[metaUpdates], where); err != nil {
		return Meta{}, err
	}
	out.Obsoletes = linkStemsFrom(values[metaObsoletes])
	out.UpdatedBy = linkStemsFrom(values[metaUpdatedBy])
	if err := readEnrolment(values, &out, where); err != nil {
		return Meta{}, err
	}
	if err := readImplementation(values, &out, where); err != nil {
		return Meta{}, err
	}
	if err := readSupport(values, &out, where); err != nil {
		return Meta{}, err
	}
	if err := readUseCases(values, &out, where); err != nil {
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
		Str("` names enrolment, support, obsolescence, an aggregate or a use case in a spelling nothing reads, ").
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

// readImplementation fills the two implementation fields, refusing an absent
// value, a value outside the closed set, a kind with no reason, and a kind that
// owes an implementer while naming none.
//
// "the kernel" alone is refused on purpose. An unnamed implementer is the claim
// without the showing, and on the public page it reads as work nobody owns
// (ai/rules/rfc-compliance.md).
func readImplementation(values map[string]string, out *Meta, where string) error {
	kind, held := values[metaImplementation]
	if !held {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": the Meta table has no `").Str(metaImplementation).
			Str("` row. Every summary declares whose code answers the document: write one of ").
			Str(pyRepr(implementationKindNames())).
			Str(". There is no default -- the ledger counts what ZE implements in Go, so a ").
			Str("document leaving that count must be a decision, never an absence"))
	}
	if !implementationKinds[kind] {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaImplementation).Str("` is ").Str(pyRepr(kind)).
			Str(", which is not one of ").Str(pyRepr(implementationKindNames())).
			Str(". Use 'ze' when Ze's Go answers the document, 'mixed' when Ze implements ").
			Str("part in Go and another layer performs the rest, 'third-party' when a layer ").
			Str("under or beside Ze performs it and Ze holds no Go code for it, and ").
			Str("'foundation' when the document defines or registers and obliges no ").
			Str("implementer. A document nobody implements while Ze OWES the behavior is a ").
			Str("gap rather than any of these"))
	}
	reason := values[metaImplementationReason]
	if reason == "" {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaImplementation).Str("` is ").Str(kind).
			Str(" with no `").Str(metaImplementationReason).
			Str("` row. A bare kind is an absence with a label on it: say what makes it true"))
	}
	if implementationNamesAnImplementer(kind) && !namesAnImplementer(reason) {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaImplementation).Str("` is ").Str(kind).
			Str(" and its reason names no implementer. Name the component and the mechanism ").
			Str("a reader can go and check, as in 'Linux XFRM builds every ESP packet' or ").
			Str("'the Linux TCP stack carries the session'. A bare 'the kernel' is refused: ").
			Str("an unnamed implementer reads on the public page as work nobody owns"))
	}
	out.Implementation = kind
	out.ImplementationReason = reason
	return nil
}

// namedComponentRE matches something a reader can go and check: a repository
// path, a `pkg::Symbol` citation, or a proper noun such as Linux, XFRM, the
// Go standard library or an RPKI cache.
//
// Derived from the shape of a name rather than from a list of names. The thing
// being named lives OUTSIDE this repository, so no registry here could hold the
// closed set, and a hand-kept list would refuse the next correct answer
// somebody writes (ai/rules/principles.md).
var namedComponentRE = regexp.MustCompile(`(?:\binternal/[\w./-]+|::|\b[A-Z][A-Za-z_0-9]{2,}\b)`)

// vagueImplementerRE matches the gesture this refusal exists to stop: "the
// kernel", with nothing named beside it.
var vagueImplementerRE = regexp.MustCompile(`(?i)\bthe\s+kernel\b`)

// namesAnImplementer answers whether the reason names a component a reader can
// go and check, rather than gesturing at "the kernel".
//
// A reason naming the kernel AND a mechanism passes: "Linux XFRM builds every
// AH header" is exactly what this asks for. What fails is the bare gesture,
// which on the public page reads as work nobody owns.
func namesAnImplementer(reason string) bool {
	stripped := vagueImplementerRE.ReplaceAllString(reason, " ")
	return namedComponentRE.MatchString(stripped)
}

// readSupport fills the public row and the aggregate, refusing an absent
// declaration and a value that is not one aggregate name.
//
// Whether the aggregate EXISTS, and whether the cells fit its table shape, is
// not decided here: the aggregates are files beside the summaries, and this
// reader sees one summary's text. checkAggregate decides both once the
// profiles are loaded (summaryMetas).
func readSupport(values map[string]string, out *Meta, where string) error {
	declared, held := values[metaSupport]
	if !held {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": the Meta table has no `").Str(metaSupport).
			Str("` row. Every summary declares whether it renders a row on ").Str(statusRel).
			Str(": write `<aggregate>`, a file name under ").Str(profileRel).
			Str(", or `").Str(supportNone).Str("` for no row together with an `").
			Str(metaAggregate).Str("` row naming the aggregate"))
	}
	if declared == supportNone {
		if err := refuseOrphanCells(values, out, where); err != nil {
			return err
		}
		aggregate, err := aggregateName(values[metaAggregate], metaAggregate, where)
		if err != nil {
			return err
		}
		out.Aggregate = aggregate
		return nil
	}
	if _, doubled := values[metaAggregate]; doubled {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaSupport).Str("` already names aggregate ").
			Str(pyRepr(declared)).Str(", so the `").Str(metaAggregate).
			Str("` row is a second declaration of it. Delete the `").Str(metaAggregate).Str("` row"))
	}
	aggregate, err := aggregateName(declared, metaSupport, where)
	if err != nil {
		return err
	}
	out.Aggregate = aggregate
	out.Row = true
	out.Area = values[metaSupportArea]
	out.Status = values[metaSupportStatus]
	out.Coverage = values[metaSupportCoverage]
	out.Remaining = values[metaSupportRemain]
	return nil
}

// aggregateName refuses a cell that is not one lower-case hyphenated name,
// which is what the retired `<section> <rank>` form fails.
func aggregateName(value, label, where string) (string, error) {
	if profileNameRE.MatchString(value) {
		return value, nil
	}
	var tb textbuf.Buffer
	return "", parseErr(tb.Str(where).Str(": `").Str(label).Str("` is ").Str(pyRepr(value)).
		Str(", which is not one aggregate name. Write the name of a file under ").
		Str(profileRel).Str(" (`").Str(label).Str(" | bgp-core |` for rfc/profiles/bgp-core.md); ").
		Str("a rank is no longer written, because a row's place is its aggregate and its RFC number"))
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
			Str(label).Str("` row. Nothing renders that cell: name an aggregate in `").Str(metaSupport).
			Str("`, or delete the cell"))
	}
	out.Name = ""
	return nil
}

// readUseCases fills the two use-case lists, refusing a missing row, an empty
// cell, `-` mixed with names, a token that is not one profile name, a name
// written twice in a row, and a name in both rows.
//
// Whether each name is a use-case profile is not decided here: the profiles
// are files beside the summaries. checkUseCases decides it once they are
// loaded (summaryMetas), as checkAggregate does for the aggregate.
func readUseCases(values map[string]string, out *Meta, where string) error {
	var err error
	if out.UseCases, err = useCaseNames(values, metaUseCases, where); err != nil {
		return err
	}
	if out.OptionalUseCases, err = useCaseNames(values, metaOptionalUseCases, where); err != nil {
		return err
	}
	for _, name := range out.UseCases {
		if !slices.Contains(out.OptionalUseCases, name) {
			continue
		}
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": the Meta table names ").Str(pyRepr(name)).Str(" in both `").
			Str(metaUseCases).Str("` and `").Str(metaOptionalUseCases).
			Str("`, two declarations of one fact. Keep it in one row: `").Str(metaUseCases).
			Str("` counts the RFC in the profile's figures, `").Str(metaOptionalUseCases).Str("` does not"))
	}
	return nil
}

// useCaseNames reads one use-case row into sorted names, nil for `-`.
func useCaseNames(values map[string]string, label, where string) ([]string, error) {
	value, held := values[label]
	if !held {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(where).Str(": the Meta table has no `").Str(label).
			Str("` row. Every summary declares the use-case profiles that require it (`").
			Str(metaUseCases).Str("`, counted) and those that list it as optional (`").
			Str(metaOptionalUseCases).Str("`, not counted): write names of files under ").
			Str(profileRel).Str(", comma separated, or `").Str(supportNone).
			Str("` for none. There is no default -- a forgotten row must not read as \"not owed\""))
	}
	if value == "" {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(where).Str(": `").Str(label).Str("` is empty. Write use-case ").
			Str("profile names, comma separated, or `").Str(supportNone).Str("` for none"))
	}
	if value == supportNone {
		return nil, nil
	}
	var names []string
	for item := range strings.SplitSeq(value, ",") {
		name := strings.TrimSpace(item)
		if name == supportNone {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(where).Str(": `").Str(label).Str("` is ").Str(pyRepr(value)).
				Str(", which mixes `").Str(supportNone).Str("` with profile names. Write either `").
				Str(supportNone).Str("` alone or the names alone"))
		}
		if !profileNameRE.MatchString(name) {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(where).Str(": `").Str(label).Str("` item ").Str(pyRepr(name)).
				Str(" is not one lower-case hyphenated profile name. Write the name of a use-case file under ").
				Str(profileRel).Str(" (`route-server` for rfc/profiles/route-server.md), comma separated"))
		}
		if slices.Contains(names, name) {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(where).Str(": `").Str(label).Str("` names ").Str(pyRepr(name)).
				Str(" twice. Delete one"))
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// checkUseCases refuses a summary naming, in either use-case row, a name that
// is no use-case profile: a technology aggregate, or a name with no profile
// file at all (AC-3).
func checkUseCases(meta *Meta, profiles Profiles, where string) error {
	for _, row := range []struct {
		label string
		names []string
	}{{metaUseCases, meta.UseCases}, {metaOptionalUseCases, meta.OptionalUseCases}} {
		for _, name := range row.names {
			if _, held := profiles.UseCases[name]; held {
				continue
			}
			var tb textbuf.Buffer
			tb.Str(where).Str(": `").Str(row.label).Str("` names ").Str(pyRepr(name))
			if _, aggregate := profiles.Aggregates[name]; aggregate {
				tb.Str(", which is a technology aggregate, not a use-case profile: an aggregate is named in ").
					Str("`").Str(metaSupport).Str("` or `").Str(metaAggregate).Str("`")
			} else {
				tb.Str(", but there is no ").Str(profileRel).Byte('/').Str(name).Str(".md")
			}
			return parseErr(tb.Str(". The use cases are ").Str(pyRepr(sortedKeysOf(profiles.UseCases))))
		}
	}
	return nil
}

// checkUseCaseMembers refuses every use-case profile no summary requires
// (AC-5), an optional-only one included: its figure would be 0 of 0, which
// reads complete and measures nothing. Sorted by profile name.
func checkUseCaseMembers(profiles Profiles, metas map[string]Meta) []string {
	required := map[string]bool{}
	for stem := range metas {
		for _, name := range metas[stem].UseCases {
			required[name] = true
		}
	}
	var out []string
	for _, name := range sortedKeysOf(profiles.UseCases) {
		if required[name] {
			continue
		}
		var tb textbuf.Buffer
		out = append(out, tb.Str(profileRel).Byte('/').Str(name).Str(".md: no summary lists ").
			Str(pyRepr(name)).Str(" in its `").Str(metaUseCases).Str("` row, so the profile owes ").
			Str("nothing and its figure would read 0 of 0. A summary joins it by writing `| ").
			Str(metaUseCases).Str(" | ").Str(name).Str(" |` in its Meta table; a profile no role ").
			Str("needs is deleted").String())
	}
	return out
}

// checkAggregate refuses a summary naming an aggregate no profile declares,
// and a public row whose cells do not fit its aggregate's table shape (AC-4).
func checkAggregate(meta *Meta, aggregates map[string]Aggregate, stem, where string) error {
	aggregate, held := aggregates[meta.Aggregate]
	if !held {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": aggregate ").Str(pyRepr(meta.Aggregate)).
			Str(" has no ").Str(profileRel).Byte('/').Str(meta.Aggregate).
			Str(".md, so this RFC belongs to no declared aggregate. The aggregates are ").
			Str(pyRepr(sortedKeysOf(aggregates))))
	}
	if !meta.Row {
		return nil
	}
	return checkRowCells(meta, &aggregate, stem, where)
}

// checkRowCells refuses a row whose cells do not match its aggregate's shape.
func checkRowCells(out *Meta, aggregate *Aggregate, stem, where string) error {
	for label, value := range map[string]string{
		metaSupportArea: out.Area, metaSupportStatus: out.Status, metaSupportCoverage: out.Coverage,
	} {
		if value != "" {
			continue
		}
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": `").Str(metaSupport).
			Str("` names an aggregate, so this summary renders a public row, but its `").
			Str(label).Str("` cell is absent or empty. A blank cell on the public page ").
			Str("states nothing where a reader expects a claim"))
	}
	if aggregate.Shape == ShapeBrief && out.Remaining != supportNone {
		var tb textbuf.Buffer
		return parseErr(tb.Str(where).Str(": aggregate ").Str(pyRepr(aggregate.Name)).
			Str(" renders four columns and has no `Remaining` column, so `").
			Str(metaSupportRemain).Str("` must be `").Str(supportNone).Str("`, not ").
			Str(pyRepr(truncateRunes(out.Remaining, 40))))
	}
	if aggregate.Shape != ShapeBrief && out.Remaining == "" {
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

// publicRowName answers the first cell of this summary's public row.
//
// Derived from the stem, because the stem already says which document this is
// and a second spelling of it is a future disagreement. The override exists for
// a row whose name a stem cannot carry: a standard that is not an RFC and not a
// draft, whose published name holds a slash, a space or upper case. ISO/IEC
// 10589 was that row, and is no longer one: IS-IS is counted in no conformance
// figure (ai/META-DECISIONS.md).
func publicRowName(stem string, meta Meta) string {
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

// updatesItemRE is one item of the `Updates` cell: an RFC number, with or
// without its `RFC` word. updatesNoteRE is a parenthetical, cut from the cell
// first, so the errata number in `RFC 4271 (verified errata 7915)` is never
// read as an RFC.
var (
	updatesItemRE = regexp.MustCompile(`(?i)^(?:RFC\s*)?(\d{3,5})$`)
	updatesNoteRE = regexp.MustCompile(`\([^()]*\)`)
)

// updatesFrom reads the Meta `Updates` cell into the stems it names.
//
// The cell is this summary's declaration of the RFC header's `Updates:` line,
// and checkHeaderLinks holds it to that header. An item that names no RFC is
// refused rather than skipped: a skipped item is an update the header check
// would then report as missing.
func updatesFrom(value, where string) ([]string, error) {
	// Parentheticals go first, before the comma split: they carry prose, an
	// erratum number or a comma of their own, and never the updated RFC.
	value = strings.TrimSpace(updatesNoteRE.ReplaceAllString(value, ""))
	if value == "" {
		return nil, nil
	}
	if value == "-" {
		return nil, nil
	}
	if strings.EqualFold(value, "none") {
		return nil, nil
	}
	var out []string
	for item := range strings.SplitSeq(value, ",") {
		found := updatesItemRE.FindStringSubmatch(strings.TrimSpace(item))
		if found == nil {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(where).Str(": the Meta `Updates` cell item ").
				Str(pyRepr(strings.TrimSpace(item))).Str(" names no RFC. Write the updated RFCs as ").
				Str("a comma list of numbers, e.g. `RFC 1997, 4271`, or write `None`"))
		}
		var stem textbuf.Buffer
		out = append(out, stem.Str("rfc").Str(found[1]).String())
	}
	return out, nil
}

// linkNumberRE is one RFC number in a lineage cell, with or without `RFC`.
var linkNumberRE = regexp.MustCompile(`(?i)(?:\bRFCs?\s*-?\s*)?\b(\d{3,5})\b`)

// linkStemsFrom reads the RFC stems a lineage cell names, parentheticals cut
// first so an erratum number or a date in a note is never read as an RFC.
// `-`, `None` and prose naming no number all answer nothing.
func linkStemsFrom(value string) []string {
	var out []string
	for _, found := range linkNumberRE.FindAllStringSubmatch(updatesNoteRE.ReplaceAllString(value, ""), -1) {
		var stem textbuf.Buffer
		name := stem.Str("rfc").Str(strings.TrimLeft(found[1], "0")).String()
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

// sortMetaStems orders a stem set for a message, so two runs name it alike.
func sortMetaStems(in map[string]Meta) []string {
	out := make([]string, 0, len(in))
	for stem := range in {
		out = append(out, stem)
	}
	slices.Sort(out)
	return out
}

// summaryMetas reads every summary's Meta table once.
//
// The ONE reader. Enrolment, the disposition, the public row, the title and the
// forward lineage all come out of this map, so no two consumers can read the
// same table and disagree about what it says.
//
// Each summary's aggregate is judged against profiles, the parsed
// rfc/profiles/: a summary naming an undeclared aggregate, or carrying cells
// its aggregate's shape does not render, is a problem like a parse error.
func summaryMetas(tree string, stems map[string]bool, profiles Profiles) (map[string]Meta, map[string]string, error) {
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
		if err := checkAggregate(&meta, profiles.Aggregates, stem, rel); err != nil {
			problems[stem] = err.Error()
			continue
		}
		if err := checkUseCases(&meta, profiles, rel); err != nil {
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
//
// A four-column aggregate (ShapeBrief) renders no Remaining cell,
// and checkRowCells holds its `Support remaining` at `-`: the row's Note, its
// Coverage, is the only cell that can say what is not complete. Remaining is
// therefore the Note for such a row, so a {gap} or {partial} disclosed there
// is read where the public page prints it.
//
// aggregates is the parsed rfc/profiles/. summaryMetas admits no summary whose
// aggregate is undeclared, so every row's aggregate is present in it.
func rowsFrom(metas map[string]Meta, aggregates map[string]Aggregate) map[string]LedgerRow {
	out := map[string]LedgerRow{}
	for stem := range metas {
		if !metas[stem].HasRow() {
			continue
		}
		remaining := metas[stem].Remaining
		if remaining == supportNone {
			remaining = ""
		}
		if aggregates[metas[stem].Aggregate].Shape == ShapeBrief {
			remaining = metas[stem].Coverage
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
