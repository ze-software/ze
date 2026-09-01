// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
// Detail: sections.go -- the four body sections of the generated index
// Detail: write.go -- the gate that emits both pages and prunes what it no longer owns
//
// render.go is the ONE producer of ai/RFC-REQUIREMENTS.md and of every file
// under rfc/requirements/. The write emits what it returns and the freshness
// check compares against what it returns, so a caller that assembled the same
// rows itself could drift from the file on disk and nothing would see it.
//
// Every figure here is DERIVED. Nothing on either page is authored except the
// requirement text, the disposition reasons and the public status cells, and
// each of those is quoted from the file that owns it.
package rfc

import (
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// shardRelDir is where one stem's requirement table is written.
const shardRelDir = "rfc/requirements"

// ledgerRel is the generated index over those files.
const ledgerRel = "ai/RFC-REQUIREMENTS.md"

// RenderInput is everything both pages are derived from.
//
// It is assembled ONCE per run and shared, because the alternative is six
// readers of docs/features/rfc-status.md and three walks of the audit
// directory in one gate. NewRenderInput builds it; a caller that already holds
// a parse (the gate reads the public page for its own checks) hands it in
// rather than paying for a second one, and the bytes are the same either way,
// which is what keeps the freshness comparison meaningful.
type RenderInput struct {
	Tree         string
	Deriver      *Deriver
	Requirements []Requirement
	Tags         []Tag
	Enrolled     map[string]bool
	Stems        map[string]bool
	Carriers     []Carrier
	Rows         map[string]LedgerRow
	Dispositions map[string]Disposition
	Successors   map[string]string
	Audits       map[string]Audit
	States       map[string]Freshness
	// Covers is the tagged unit each tag sits in, Discrimination is every
	// stored proof re-verified against this tree, and Unscanned is the
	// `RFC requirement:` comments in production Go that no carrier claims.
	//
	// Derived here rather than handed in, unlike Rows and Dispositions. Two
	// callers render this page -- `./le rfc index-update` writes it and the
	// gate compares against it -- so a figure only one of them could compute
	// would publish differently depending on who rendered it.
	Covers         map[Cover][]Tag
	Discrimination []DiscriminationVerdict
	Unscanned      []UnscannedTag
}

// NewRenderInput derives everything the two pages need from one checkout.
//
// Rows and Dispositions are filled here only when the caller left them nil, so
// a gate that has already parsed the public page for its own checks shares that
// parse rather than reparsing it.
func NewRenderInput(tree string, collected Collected, rows map[string]LedgerRow,
	dispositions map[string]Disposition) (RenderInput, error) {
	in := RenderInput{
		Tree:         tree,
		Deriver:      NewDeriver(tree),
		Requirements: collected.Requirements,
		Tags:         collected.Tags,
		Enrolled:     collected.Enrolled,
		Rows:         rows,
		Dispositions: dispositions,
	}
	var err error
	if in.Stems, err = summaryStems(tree); err != nil {
		return RenderInput{}, err
	}
	if in.Carriers, err = carriers(tree); err != nil {
		return RenderInput{}, err
	}
	if in.Successors, err = summarySuccessors(tree, in.Stems); err != nil {
		return RenderInput{}, err
	}
	if in.Rows == nil {
		if in.Rows, err = loadStatusLedger(tree); err != nil {
			return RenderInput{}, err
		}
	}
	if in.Dispositions == nil {
		if in.Dispositions, err = loadDispositions(tree); err != nil {
			return RenderInput{}, err
		}
	}
	if in.Audits, err = loadAudits(tree, in.Enrolled); err != nil {
		return RenderInput{}, err
	}
	in.States = auditFreshness(auditFreshnessInput{
		Tree: tree, Requirements: in.Requirements, Tags: in.Tags,
		Enrolled: in.Enrolled, Audits: in.Audits,
	})
	if in.Covers, err = tagCovers(newSourceReader(tree), newScopeIndex(), in.Tags); err != nil {
		return RenderInput{}, err
	}
	records, err := loadDiscrimination(tree)
	if err != nil {
		return RenderInput{}, err
	}
	if in.Discrimination, err = verifyDiscrimination(tree, records, in.Covers); err != nil {
		return RenderInput{}, err
	}
	if in.Unscanned, err = unscannedTags(tree, in.Carriers); err != nil {
		return RenderInput{}, err
	}
	return in, nil
}

// ShardStems answers the stems that render a shard: one per RFC that declares
// at least one requirement.
//
// The ONE producer of that set. The shard render iterates it, the index takes
// its first entry as the example path it cites, and the write prunes against
// it -- so the index can never name a shard the write did not produce, and the
// prune can never delete one it did.
func ShardStems(requirements []Requirement) []string {
	seen := map[string]bool{}
	for _, req := range requirements {
		seen[req.RFC] = true
	}
	return sortedSet(seen)
}

// shardRel is the repo-relative shard path, for citing inside a generated page.
func shardRel(stem string) string {
	var tb textbuf.Buffer
	return tb.Str(shardRelDir).Byte('/').Str(stem).Str(".md").String()
}

// summaryRelOf is the repo-relative path of the authored summary a shard
// derives from.
//
// Composed here rather than spelled into the banner's format string, because
// internal/le/doc/check/links.go reads a backticked path out of ANY tracked
// file, this one included. A summary path written with a placeholder in it is a
// dead citation to that sweep, even though every path it renders resolves.
func summaryRelOf(stem string) string {
	var tb textbuf.Buffer
	return tb.Str(summaryRel).Byte('/').Str(stem).Str(".md").String()
}

// tagSite answers where a tag sits, NAMED so the citation survives an edit
// above it.
//
// The file plus the enclosing top-level function, never file:line. The line is
// derived at scan time and recoverable on demand, so storing it in a generated
// page bought a reader nothing and cost that page a rewrite on every edit above
// any tag in it. 3591 such citations stood across 177 shards.
//
// The symbol is the SAME anchor the audit fingerprints are keyed on, resolved
// through the same scope reader, so the two layers cannot disagree about which
// unit an obligation sits in. The bare path comes back for a non-Go carrier,
// for a line no single span encloses, and for a file that could not be read --
// which is the honest answer in each case.
func tagSite(tag Tag, reader *sourceReader, index *scopeIndex) string {
	content := reader.text(tag.File)
	var tb textbuf.Buffer
	if content == "" {
		return tb.Byte('`').Str(tag.File).Byte('`').String()
	}
	name := index.funcNameAt(tag.File, content, tag.Line)
	if name == "" {
		return tb.Byte('`').Str(tag.File).Byte('`').String()
	}
	return tb.Byte('`').Str(tag.File).Str("` `").Str(name).Byte('`').String()
}

// tagSites answers the cited sites for one polarity, deduplicated and
// order-stable.
//
// Several tags in one function collapse to one citation, which is the point:
// the reader is told which unit enforces the requirement, and the count of
// inline cases inside it was never a fact the page could keep current anyway.
func tagSites(found []Tag, polarity string, in RenderInput,
	reader *sourceReader, index *scopeIndex) string {
	seen := map[string]bool{}
	var order []string
	for _, tag := range found {
		if tag.Polarity != polarity {
			continue
		}
		var tb textbuf.Buffer
		cell := tb.Str(tagSite(tag, reader, index)).Str(" (").
			Str(evidenceLabel(tag.File, in.Carriers)).Byte(')').String()
		if seen[cell] {
			continue
		}
		seen[cell] = true
		order = append(order, cell)
	}
	return strings.Join(order, ", ")
}

// TableCell makes one AUTHORED string safe to put in a markdown table cell.
//
// Exported because the published per-RFC page's Markdown mirror puts the same
// authored prose in the same shape of cell, and a second escape rule beside
// this one is a second answer to one question (ai/rules/principles.md).
//
// A bare pipe closes the cell, so a reason quoting a grep alternation splits its
// row into extra columns and the published page renders a broken table. Seven
// rows of rfc/requirements/rfc7752.md were in that state, all from one
// {not-applicable} reason quoting a grep alternation.
//
// Only cells built from authored prose need this. A requirement id, a level, a
// section and a test link are all derived and can hold no pipe.
func TableCell(text string) string {
	// The BACKSLASH first, then the pipe. A reason that already carries a
	// literal `\|` -- a grep BRE alternation, which two annotation reasons do
	// -- would otherwise render as `\\|`, and GFM reads that as an escaped
	// backslash followed by a LIVE pipe: the row gains a cell, the header does
	// not, and the tail of the reason is dropped from the published page.
	return strings.ReplaceAll(strings.ReplaceAll(text, `\`, `\\`), "|", `\|`)
}

// RequirementRow is one requirement's row of rfc/requirements/<stem>.md, cell
// by cell, before any of them is formatted into markdown.
//
// It exists so the shard and every other publication of a requirement are ONE
// derivation. The site publishes the same six cells on /quality/rfc-compliance/
// <stem>/, and a second assembly of them would let the repository and the
// public page state different things about one obligation with nothing to
// arbitrate it (ai/rules/principles.md).
//
// Note is the RAW note: the nightly-only mark, the audit marker, the annotation
// and the superseded pointer, joined by a space and escaped by nothing. Each
// consumer escapes at its own render boundary, which is markdown here and HTML
// on the site.
type RequirementRow struct {
	RID      string `json:"rid"`
	Level    string `json:"level"`
	Section  string `json:"section"`
	Positive string `json:"positive"`
	Negative string `json:"negative"`
	Note     string `json:"note"`
}

// NoteCell answers the note as a markdown table cell: the raw note with the
// characters that would break a row escaped.
func (r RequirementRow) NoteCell() string { return TableCell(r.Note) }

// RequirementRows answers every RFC's requirement rows, keyed by summary stem
// and sorted by requirement id inside each one.
//
// The ONE producer of those rows. RenderShards formats what this answers, and
// any other publisher of a requirement reads it rather than assembling the
// cells again.
func RequirementRows(in RenderInput) map[string][]RequirementRow {
	byRID := tagsByRID(in.Tags)
	_, byRFC := requirementsByRFC(in.Requirements)
	verdictByRID := auditMarks(in)

	// One read per tagged file for the whole walk, shared across every stem.
	reader := newSourceReader(in.Tree)
	index := newScopeIndex()

	out := make(map[string][]RequirementRow, len(byRFC))
	for _, rfc := range ShardStems(in.Requirements) {
		reqs := append([]Requirement(nil), byRFC[rfc]...)
		sort.Slice(reqs, func(i, j int) bool { return reqs[i].RID < reqs[j].RID })
		rows := make([]RequirementRow, 0, len(reqs))
		for _, req := range reqs {
			rows = append(rows, requirementRow(req, byRID[req.RID], verdictByRID[req.RID],
				in, reader, index))
		}
		out[rfc] = rows
	}
	return out
}

// auditMarks answers the per-row audit marker: the verdict word, and the
// freshness state beside it whenever the verdict is not current.
//
// A reader scanning ONE requirement sees the verdict without reconstructing it
// from the coverage section.
func auditMarks(in RenderInput) map[string]string {
	out := map[string]string{}
	for _, req := range in.Requirements {
		verdict, held := in.Audits[req.RFC].Verdict(req.RID)
		if !held || len(verdict) == 0 {
			continue
		}
		value := verdictValue(verdict)
		if state, known := in.States[req.RID]; known && state.State != FreshState {
			var tb textbuf.Buffer
			value = tb.Str(value).Str(", ").Str(state.State).String()
		}
		out[req.RID] = value
	}
	return out
}

// RenderShards answers one RFC's requirement table per entry, keyed by summary
// stem.
//
// A stem absent from the requirements renders nothing at all. That is the zero
// boundary the ledger already had -- a summary declaring no requirement rendered
// no section -- and it is why the prune deletes only what this did not produce.
func RenderShards(in RenderInput) map[string]string {
	rows := RequirementRows(in)
	shards := map[string]string{}
	for _, rfc := range ShardStems(in.Requirements) {
		state := "not enrolled"
		if in.Enrolled[rfc] {
			state = "enrolled (gated)"
		}
		if successor, held := in.Successors[rfc]; held {
			var tb textbuf.Buffer
			state = tb.Str(state).Str(", superseded by ").Str(Prefix(successor)).String()
		}

		var head, banner textbuf.Buffer
		// No line numbers in the banner, deliberately. A citation names the
		// file and the enclosing function, which is what survives an edit above
		// the tag; the exact lines are recoverable on demand.
		out := make([]string, 0, len(rows[rfc])+6)
		out = append(out,
			head.Str("# ").Str(Prefix(rfc)).Str(" -- ").Str(state).String(),
			"",
			banner.
				Str("GENERATED by `./le rfc index-update` -- do not edit. Requirement ").
				Str("text is authored in `").Str(summaryRelOf(rfc)).
				Str("`; the test links are derived from `RFC requirement:` tags in the ").
				Str("tests themselves. The exact rows live in `rfc/requirements/").
				Str(rfc).Str(".md`, and the index over every RFC is ").
				Str("`ai/RFC-REQUIREMENTS.md`.").String(),
			"",
			"| Requirement | Level | § | Positive test | Negative test | Note |",
			"|---|---|---|---|---|---|")
		for _, row := range rows[rfc] {
			out = append(out, shardRow(row))
		}
		out = append(out, "")
		shards[rfc] = strings.Join(out, "\n")
	}
	return shards
}

// requirementRow answers one requirement's six cells.
func requirementRow(req Requirement, found []Tag, audited string, in RenderInput,
	reader *sourceReader, index *scopeIndex) RequirementRow {
	// Sorted by (file, line) so the page is byte-stable regardless of the order
	// the tree happened to be walked in: directory order is filesystem
	// dependent, so an unsorted render churns across machines and defeats the
	// freshness gate.
	found = append([]Tag(nil), found...)
	sort.Slice(found, func(i, j int) bool {
		if found[i].File != found[j].File {
			return found[i].File < found[j].File
		}
		return found[i].Line < found[j].Line
	})

	var marks []string
	// The marker is on the ROW, so a reader scanning one requirement sees the
	// weakness without reconstructing it from the per-link tiers.
	if NightlyOnly(found, in.Carriers) {
		marks = append(marks, "**nightly-only**")
	}
	if audited != "" && audited != VerdictEnforced {
		// An `enforced` verdict is unmarked on purpose: it says the row's tests
		// do what the row already claims. Every OTHER value contradicts the
		// row, and a contradiction has to be visible where the claim is made.
		var tb textbuf.Buffer
		marks = append(marks, tb.Str("**audit: ").Str(audited).Str("**").String())
	}
	if req.Annotation != nil {
		var tb textbuf.Buffer
		marks = append(marks, tb.Byte('{').Str(req.Annotation.Kind).Str("} ").
			Str(req.Annotation.Reason).String())
	}
	if req.Superseded != nil {
		// Both marks render when both are present. They answer different
		// questions -- what Ze still owes, and which document states it today
		// -- and dropping either would leave a reader with half the row's facts.
		var tb textbuf.Buffer
		tb.Byte('{').Str(SupersededKind).Str(": ").Str(req.Superseded.Disposition)
		if req.Superseded.Target != "" {
			tb.Byte(' ').Str(req.Superseded.Target)
		}
		marks = append(marks, tb.Str("} ").Str(req.Superseded.Reason).String())
	}

	return RequirementRow{
		RID:      req.RID,
		Level:    req.Level,
		Section:  req.Section,
		Positive: orDashes(tagSites(found, PolarityPositive, in, reader, index)),
		Negative: orDashes(tagSites(found, PolarityNegative, in, reader, index)),
		Note:     strings.Join(marks, " "),
	}
}

// shardRow formats one requirement's row as the shard writes it.
func shardRow(row RequirementRow) string {
	var out textbuf.Buffer
	return out.Str("| `").Str(row.RID).Str("` | ").Str(row.Level).Str(" | ").
		Str(row.Section).Str(" | ").Str(row.Positive).Str(" | ").Str(row.Negative).
		Str(" | ").Str(row.NoteCell()).Str(" |").String()
}

// orDashes answers the empty-cell marker for a polarity with no citation.
func orDashes(cell string) string {
	if cell == "" {
		return "--"
	}
	return cell
}

// RenderIndex answers the generated ai/RFC-REQUIREMENTS.md body: every head
// section, no requirement row.
//
// The per-RFC tables are RenderShards. This is the index over them: the counts,
// the evidence legend, the coverage rollup internal/le/testhealth/actions.go parses,
// the audit coverage, the extraction sign-off, the status backlog and the
// no-MUST-summary table.
func RenderIndex(in RenderInput) (string, error) {
	_, byRFC := requirementsByRFC(in.Requirements)
	total, gatedTotal := 0, 0
	for _, req := range in.Requirements {
		if !req.Gated() {
			continue
		}
		total++
		if in.Enrolled[req.RFC] {
			gatedTotal++
		}
	}

	out := []string{
		"# RFC Requirement Ledger",
		"",
		"GENERATED by `./le rfc index-update` -- do not edit. Requirement text is " +
			"authored in `rfc/short/*.md`; the test links are derived from " +
			"`RFC requirement:` tags in the tests themselves (`ai/rules/evidence.md`).",
		"",
	}
	var counts textbuf.Buffer
	out = append(out, counts.Int(int64(len(in.Requirements))).Str(" requirements across ").
		Int(int64(len(byRFC))).Str(" summaries. ").Int(int64(total)).
		Str(" are MUST-level; ").Int(int64(gatedTotal)).
		Str(" of those are enrolled and gated by `./le rfc check`.").String(), "")

	stems := ShardStems(in.Requirements)
	if len(stems) > 0 {
		// A REAL shard, derived from the rendered set, never a placeholder with
		// an angle-bracket stem: the doc-link sweep requires each cited path to
		// exist, so a placeholder here is a dead citation in a generated page.
		var tb textbuf.Buffer
		out = append(out, tb.
			Str("One RFC's requirement rows are one file, named for its summary's stem: `").
			Str(shardRel(stems[0])).Str("` holds ").Str(Prefix(stems[0])).
			Str(". This page is the index over those ").Int(int64(len(stems))).
			Str(" files.").String(), "")
	}
	out = append(out,
		"An RFC is **enrolled** (`rfc/enrolled.txt`) when every MUST-level requirement "+
			"it declares is covered by a positive AND a negative test, or annotated. "+
			"Un-enrolled RFCs are listed here but not gated: that remainder is tracked, "+
			"not hidden (`ai/rules/testing.md`, Back-Fill New Test Types).",
		"")

	out = append(out, renderEvidenceLegend(in.Carriers)...)
	out = append(out, renderRollup(in)...)
	out = append(out, renderAuditCoverage(in)...)
	out = append(out, renderDiscrimination(in)...)
	extraction, err := renderExtractionTable(in)
	if err != nil {
		return "", err
	}
	out = append(out, extraction...)
	out = append(out, renderStatusBacklog(in)...)
	out = append(out, renderUnconverted(in)...)
	return strings.Join(out, "\n"), nil
}

// renderEvidenceLegend says what each `kind/tier` cell means, derived from the
// carrier table rather than restated.
//
// Without it the ledger prints a vocabulary it never defines, and a reader has
// to open the scanner to learn whether `interop/nightly` is stronger or weaker
// than `functional/verify`.
func renderEvidenceLegend(carriers []Carrier) []string {
	out := make([]string, 0, len(carriers)+9)
	out = append(out,
		"## Evidence kinds",
		"",
		"Every test link in the per-RFC requirement files carries a `kind/tier` cell. "+
			"**kind** is the layer the test exercises; **tier** is whether anything "+
			"executes it. A unit test proves the algorithm, a `.ci` proves the daemon "+
			"exposes the behavior to a user, an interop scenario proves a foreign peer "+
			"accepts it -- and a tier of `nightly` means the proof does not run on the "+
			"merge path.",
		"",
		"| Cell | Carrier | Executed by | Pipeline |",
		"|---|---|---|---|")
	// Collapsed by (label, suffix, runner). The .ci and .et carriers are one row
	// PER SUITE -- that is what ties a tier to something that runs -- but
	// printing 20-odd rows differing only in a suite name is an inventory, not a
	// legend. The suites are listed in the collapsed row's Pipeline cell
	// instead, still derived, still complete.
	type legendKey struct{ label, suffix, runner string }
	groups := map[legendKey][]Carrier{}
	var order []legendKey
	for _, c := range carriers {
		if c.Tier == tierUnrun {
			continue
		}
		key := legendKey{c.Label(), c.Suffix, c.Runner}
		if _, held := groups[key]; !held {
			order = append(order, key)
		}
		groups[key] = append(groups[key], c)
	}
	for _, key := range order {
		rows := groups[key]
		pipeline := rows[0].Pipeline
		if len(rows) > 1 || rows[0].Derived {
			var suites textbuf.Buffer
			for i, c := range rows {
				if i > 0 {
					suites.Str(", ")
				}
				suites.Byte('`').Str(suiteOfPrefix(c.Prefix)).Byte('`')
			}
			var tb textbuf.Buffer
			pipeline = tb.Str(stageOf(rows[0].Pipeline)).Str(" -- suites: ").
				Str(suites.String()).String()
		}
		var row textbuf.Buffer
		out = append(out, row.Str("| `").Str(key.label).Str("` | `*").Str(key.suffix).
			Str("` | `").Str(key.runner).Str("` | ").Str(pipeline).Str(" |").String())
	}
	out = append(out, "")

	// A catch-all row has no prefix to name, so describe it by shape instead. No
	// inner backticks: each entry is wrapped in backticks below, and nesting
	// them renders as literal characters.
	seen := map[string]bool{}
	var unrun []string
	for _, c := range carriers {
		if c.Tier != tierUnrun {
			continue
		}
		name := c.Prefix
		if name == "" {
			var tb textbuf.Buffer
			name = tb.Str("any *").Str(c.Suffix).Str(" the table above does not cover").String()
		}
		if !seen[name] {
			seen[name] = true
			unrun = append(unrun, name)
		}
	}
	sort.Strings(unrun)
	var tail textbuf.Buffer
	out = append(out, tail.
		Str("A tag in a carrier nothing executes is REFUSED by `./le rfc check`, not ").
		Str("listed here with a caveat. These have no automated caller: ").
		Str(joinBackticked(unrun)).
		Str(". A tag in one of them would be an absence of evidence wearing evidence's ").
		Str("clothes, so the scanner denies it and names the fix ").
		Str("(`ai/rules/evidence.md`).").String(), "")
	return out
}

// suiteOfPrefix answers the suite name a derived carrier's prefix carries:
// `test/<suite>/` names <suite>.
func suiteOfPrefix(prefix string) string {
	parts := strings.Split(prefix, "/")
	if len(parts) < 2 {
		return prefix
	}
	return parts[1]
}

// stageOf answers a derived carrier's pipeline with its suite qualifier
// replaced by a closing bracket, so the collapsed row names the STAGE and lists
// the suites once.
func stageOf(pipeline string) string {
	cut := strings.LastIndex(pipeline, ",")
	if cut < 0 {
		var tb textbuf.Buffer
		return tb.Str(pipeline).Byte(')').String()
	}
	var tb textbuf.Buffer
	return tb.Str(pipeline[:cut]).Byte(')').String()
}
