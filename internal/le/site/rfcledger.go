// Design: website/AI.md -- the per-RFC requirement ledger is internal/le/rfc's own answer
// Detail: rfcdetail.go renders one stem's page; rfccompliance.go writes the index over them.
// Related: build.go publishes data/rfc-requirements.json before any producer runs.
package site

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcLedger is one whole reading of the requirement ledger, one entry per
// summary stem.
//
// It is a value rather than a set of calls, for the reason rfcCompliance is
// one: the build derives it once through refreshNativeSurfaces, publishes it as
// data/rfc-requirements.json, and the producer renders 191 pages from that file
// alone. A test can then state a ledger instead of walking 190 summaries and
// every test tag in the tree.
//
// Every field is DERIVED by internal/le/rfc. Nothing here is counted a second
// time: the requirement cells are rfc.RequirementRows, the per-RFC buckets are
// rfc.CoverageRows, and the vocabulary words are that package's own constants
// (ai/rules/principles.md).
type rfcLedger struct {
	Stems []rfcLedgerStem `json:"stems"`
}

// rfcLedgerStem is one summary and everything the repository knows about it.
type rfcLedgerStem struct {
	Stem    string `json:"stem"`
	Display string `json:"display"`
	// Title is the RFC's own title, from the summary's Meta row. It is empty
	// for a summary that declares none, and the page then shows the display
	// name alone rather than a guess.
	Title string `json:"title,omitempty"`
	// Enrolled says the gate holds this RFC to every MUST it declares, and
	// EnrolmentReason is the sentence the summary's `| Enrolment reason |` Meta
	// row states beside it.
	Enrolled        bool   `json:"enrolled"`
	EnrolmentReason string `json:"enrolment-reason,omitempty"`
	// Disposition is why an un-enrolled summary is not enrolled: the kind and
	// the reason of the same Meta rows. It is absent for an enrolled stem.
	Disposition *rfcLedgerDisposition `json:"disposition,omitempty"`
	// The three cells this RFC's row on docs/features/rfc-status.md carries.
	// They are the summary's own `| Support status |`, `| Support coverage |`
	// and `| Support remaining |` Meta rows, which that page is generated from.
	// Each is empty when the summary declares `| Support | - |` and so renders
	// no row, which the page says in words.
	PublicStatus    string `json:"public-status,omitempty"`
	PublicCoverage  string `json:"public-coverage,omitempty"`
	PublicRemaining string `json:"public-remaining,omitempty"`
	// The three repository paths a reader follows: the authored summary, the
	// generated requirement shard, and the RFC's own text. ShardPath is empty
	// for a summary declaring no requirement, and SourcePath for an RFC whose
	// text this checkout does not carry.
	SummaryPath string `json:"summary-path"`
	ShardPath   string `json:"shard-path,omitempty"`
	SourcePath  string `json:"source-path,omitempty"`
	// Successor is the stem of the document that obsoletes this one, when the
	// summary's forward Meta row names one.
	Successor    string                 `json:"successor,omitempty"`
	Coverage     rfcLedgerCoverage      `json:"coverage"`
	Requirements []rfcLedgerRequirement `json:"requirements"`
	Extraction   *rfcLedgerExtraction   `json:"extraction,omitempty"`
}

// rfcLedgerDisposition is the decision that keeps one summary out of the gate,
// as its own Meta table declares it. It is what one row of the generated
// rfc/not-enrolled.txt is rendered from, and never read back from that file.
type rfcLedgerDisposition struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

// rfcLedgerCoverage is one RFC's counters.
//
// Gated, Both, One, Annotated, Missing and NightlyOnly come from
// rfc.CoverageRows unchanged. The four beside them count what the page states
// elsewhere, so the at-a-glance panel and the sections under it cannot
// disagree about the same population.
type rfcLedgerCoverage struct {
	Requirements int `json:"requirements"`
	Gated        int `json:"gated"`
	Both         int `json:"both"`
	One          int `json:"one"`
	Annotated    int `json:"annotated"`
	Missing      int `json:"missing"`
	NightlyOnly  int `json:"nightly-only"`
	// Gaps counts EVERY declared gap, at any level, because that is the number
	// the page discloses. GatedGaps counts the subset the gate holds, which is
	// the one the binding arithmetic below can use: four SHOULD-level gaps in
	// this corpus are gaps a reader is owed and are outside the gated
	// population every ratio is taken over.
	Gaps      int `json:"gaps"`
	GatedGaps int `json:"gated-gaps"`
	// NotApplicable counts the gated requirements whose `{not-applicable}`
	// annotation says the obligation does not bind Ze at all. It is SCOPE, and
	// it is subtracted from the gated count to answer the population that does
	// bind: an obligation that never bound Ze is not a coverage achievement,
	// and leaving it in the denominator flatters every ratio above it (owner
	// ruling, 2026-09-01).
	NotApplicable int `json:"out-of-scope"`
	// SinglePolarity counts the gated requirements a `{single-polarity}`
	// annotation excuses from one direction. Those DO bind.
	SinglePolarity int `json:"excused-one-polarity"`
	// UnmappedAnnotations counts the gated requirements whose annotation kind
	// this page has no bucket for.
	//
	// It is the arithmetic hole made visible. The three counters above split
	// the gate's own Annotated total, and a kind none of them claims would
	// otherwise vanish from the shares while the gate went on counting it: the
	// cards would sum to less than their whole and the page would say they add
	// to 100% (independent review, 2026-09-02). A hole that is counted is a
	// hole the page can disclose and a test can find; a hole that is skipped is
	// neither.
	UnmappedAnnotations int `json:"unmapped-annotations,omitempty"`
	Tags                int `json:"tags"`
	// Units counts the tagged units, which is what a discrimination record is
	// keyed on. Tags counts tag occurrences, and two tags in one function share
	// a unit, so the two are different numbers and only Units is the
	// denominator of a proof ratio.
	Units   int `json:"units"`
	Audited int `json:"audited"`
	Records int `json:"discrimination-records"`
	// Escapes counts the records that claim NO break exists. An escape is not a
	// proof, so it is carried apart wherever a proven count is published.
	Escapes int `json:"escapes"`
	// Stale counts the records that DID prove a break and no longer verify:
	// the tagged unit, the claim or the producer has changed since the red was
	// observed, or the tag or the citation is gone. A break is observed once
	// and never re-run, so what stands behind a proof is that nothing it rested
	// on has moved; when something has, the record is a lapsed proof and not a
	// proof (verifyOneDiscrimination, internal/le/rfc/discriminate.go). Counting
	// it as one would publish a red nobody has seen against bytes nobody has
	// checked.
	Stale int `json:"stale-records"`
}

// Binding answers the obligations that bind Ze: the gated population less the
// requirements whose annotation says the obligation does not apply to Ze.
func (c rfcLedgerCoverage) Binding() int { return c.Gated - c.NotApplicable }

// NoTest answers the binding obligations no test carries: the declared gaps and
// the unexcused misses. A gap states a reason and supplies no evidence, so it
// counts here (owner ruling, 2026-09-01).
func (c rfcLedgerCoverage) NoTest() int { return c.GatedGaps + c.Missing }

// Proven answers the tagged units a LIVE recorded break stands behind: a red
// observed once under a recorded procedure, whose unit, claim and producer are
// still the bytes it was observed against.
//
// An escape claims no break exists and a stale record rests on bytes that have
// moved, so neither is a proof.
func (c rfcLedgerCoverage) Proven() int { return c.Records - c.Escapes - c.Stale }

// rfcLedgerRequirement is one Compliance Checklist line and its proof state.
//
// Positive, Negative and Note are the cells rfc/requirements/<stem>.md carries,
// answered by rfc.RequirementRows. Note is the RAW note: this page escapes it
// for HTML and the mirror escapes it for markdown, and neither assembles it
// again.
type rfcLedgerRequirement struct {
	RID         string               `json:"rid"`
	Level       string               `json:"level"`
	Section     string               `json:"section"`
	Text        string               `json:"text"`
	Gated       bool                 `json:"gated"`
	Positive    string               `json:"positive-test"`
	Negative    string               `json:"negative-test"`
	Note        string               `json:"note"`
	NightlyOnly bool                 `json:"nightly-only"`
	Annotation  *rfcLedgerAnnotation `json:"annotation,omitempty"`
	Superseded  *rfcLedgerSuccessor  `json:"superseded,omitempty"`
	Covers      []rfcLedgerCover     `json:"covers,omitempty"`
	Audit       *rfcLedgerVerdict    `json:"audit,omitempty"`
}

// rfcLedgerAnnotation is a `{kind: reason}` marker: why this requirement owes
// less than a positive and a negative test.
type rfcLedgerAnnotation struct {
	Kind     string `json:"kind"`
	Polarity string `json:"polarity,omitempty"`
	Reason   string `json:"reason"`
}

// rfcLedgerSuccessor is where one requirement of a superseded document lives now.
type rfcLedgerSuccessor struct {
	Disposition string `json:"disposition"`
	Target      string `json:"target,omitempty"`
	Reason      string `json:"reason"`
}

// rfcLedgerCover is one tagged unit that proves one requirement in one
// direction, and whether a recorded break stands behind it.
//
// The unit rather than the tag, because that is the identity a discrimination
// record is keyed on: two tags in one function share a unit and owe one record
// between them.
type rfcLedgerCover struct {
	Polarity string `json:"polarity"`
	Unit     string `json:"unit"`
	// File and Line are where the first tag of this unit is written, so a
	// published row links to the assertion rather than to a long file. Line is
	// zero for a tag whose scan recorded none, and the link then addresses the
	// file.
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
	// Carrier is the `kind/tier` cell of the file the unit sits in, and is
	// empty for a path no carrier claims.
	Carrier string `json:"carrier,omitempty"`
	Claim   string `json:"claim,omitempty"`
	Tags    int    `json:"tags"`
	// Proof is the recorded break this unit was seen to fail under. It is
	// absent when no record exists, which is what makes the tag unproven.
	Proof *rfcLedgerProof `json:"proof,omitempty"`
}

// rfcLedgerProof is one stored discrimination record, re-verified against this
// tree.
//
// Proves is false for the `no-break` route, which is the ESCAPE. It is carried
// as its own field so no reader has to compare the route against a literal to
// tell a proof from a claim that nothing could be broken.
type rfcLedgerProof struct {
	Route    string `json:"route"`
	State    string `json:"state"`
	Detail   string `json:"detail,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Producer string `json:"producer,omitempty"`
	Break    string `json:"break,omitempty"`
	Verified bool   `json:"verified"`
	Proves   bool   `json:"proves"`
}

// rfcLedgerVerdict is one recorded audit judgement and how current it is.
type rfcLedgerVerdict struct {
	Verdict string `json:"verdict"`
	// Meaning is what the verdict word says, from the vocabulary that declares
	// it. A page publishing the word alone tells a reader nothing.
	Meaning       string   `json:"meaning"`
	Note          string   `json:"note,omitempty"`
	Freshness     string   `json:"freshness"`
	Moved         []string `json:"moved,omitempty"`
	UpgradeReason string   `json:"upgrade-reason,omitempty"`
	NoCodePath    string   `json:"no-code-path,omitempty"`
}

// rfcLedgerExtraction is the sign-off that decided which sentences of the RFC
// became requirements at all.
type rfcLedgerExtraction struct {
	Path       string `json:"path"`
	Reviewer   string `json:"reviewer,omitempty"`
	SignedOff  string `json:"signed-off,omitempty"`
	Register   string `json:"register,omitempty"`
	SourcePath string `json:"source-path,omitempty"`
	SourceSHA  string `json:"source-sha,omitempty"`
	Mapped     int    `json:"mapped-sites"`
	Excluded   int    `json:"excluded-sites"`
	Relocated  int    `json:"relocated-sites"`
	// Unclassified counts the sites this walk left with no disposition, which
	// is a walk in progress rather than a decision.
	Unclassified int                     `json:"unclassified"`
	Sections     []rfcLedgerSection      `json:"sections,omitempty"`
	Exclusions   []rfcLedgerExcludedSite `json:"exclusions,omitempty"`
}

// rfcLedgerSection is one section of the RFC and what the reviewer did with it.
//
// Name is the section's own title, which internal/le/rfc derives from the
// reason. It is carried here rather than derived at render time so the fact is
// stated once, in the published ledger every reader of this family reads.
type rfcLedgerSection struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Sites       int    `json:"sites"`
	Disposition string `json:"disposition,omitempty"`
	SkipKind    string `json:"skip-kind,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// rfcLedgerExcludedSite is one sentence the walk declined to map, and why.
type rfcLedgerExcludedSite struct {
	ID          string `json:"id"`
	Quote       string `json:"quote,omitempty"`
	Kind        string `json:"excluded-kind,omitempty"`
	Reason      string `json:"reason,omitempty"`
	RelocatedTo string `json:"relocated-to,omitempty"`
	ReservedID  string `json:"reserved-id,omitempty"`
}

// liveRequirementLedger answers the requirement ledger for one checkout. It is a
// variable so a test can state a ledger rather than walk every summary and
// every test tag in the tree.
var liveRequirementLedger = collectRequirementLedger

// publishRFCLedger derives the requirement ledger once and publishes it as a
// named artifact, before any producer runs.
//
// It matches publishPluginRegistry and publishCommandCatalog: an input a
// producer reads cannot be an input another producer writes, because producers
// run in registration order. The published JSON is also the machine-readable
// ledger itself, which is what the disclosure ruling of 2026-09-01 wants.
func publishRFCLedger(paths Paths) error {
	ledger, err := liveRequirementLedger(paths.Repository)
	if err != nil {
		return err
	}
	content, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", rfcLedgerFile, err)
	}
	return writeNamedArtifact(paths.Output, rfcLedgerFile, string(content)+"\n")
}

// collectRequirementLedger reads one checkout's requirement ledger.
//
// ONE reading of the tree: rfc.Collect parses every summary and scans the test
// tree once, and rfc.NewRenderInput derives the carriers, the public ledger
// rows, the dispositions, the audits, their freshness, the tagged units and
// every stored proof re-verified against this tree.
//
// rfc.Check is NOT called and must not be. It is a second full pass that also
// type-checks every tagged package under a fifteen-minute vet timeout, and the
// aggregate page already pays for it once and publishes the verdict it answers.
func collectRequirementLedger(tree string) (rfcLedger, error) {
	collected, err := rfc.Collect(tree)
	if err != nil {
		return rfcLedger{}, err
	}
	input, err := rfc.NewRenderInput(tree, collected, nil, nil)
	if err != nil {
		return rfcLedger{}, err
	}
	extractions, err := rfc.LoadExtractions(tree)
	if err != nil {
		return rfcLedger{}, err
	}

	rows := rfc.RequirementRows(input)
	buckets := make(map[string]rfc.CoverageRow, len(input.Stems))
	for _, row := range rfc.CoverageRows(input.Requirements, input.Tags, input.Carriers) {
		buckets[row.RFC] = row
	}
	byStem := rfcRequirementsByStem(input.Requirements)
	coversByRID := rfcCoversByRID(input.Covers, input.Carriers)
	proofs := rfcProofsByCover(input.Discrimination)

	ledger := rfcLedger{Stems: make([]rfcLedgerStem, 0, len(input.Stems))}
	for _, stem := range sortedSetKeys(input.Stems) {
		one := rfcLedgerInput{
			Tree: tree, Stem: stem, Render: input,
			Rows: rows[stem], Bucket: buckets[stem], Requirements: byStem[stem],
			Covers: coversByRID, Proofs: proofs, Extraction: extractions[stem],
		}
		ledger.Stems = append(ledger.Stems, rfcLedgerStemOf(&one))
	}
	return ledger, nil
}

// rfcLedgerInput is everything one stem's entry is derived from, gathered by
// the walk above so the assembly below reads no tree of its own.
type rfcLedgerInput struct {
	Tree         string
	Stem         string
	Render       rfc.RenderInput
	Rows         []rfc.RequirementRow
	Bucket       rfc.CoverageRow
	Requirements []rfc.Requirement
	Covers       map[string][]rfcLedgerCover
	Proofs       map[rfc.Cover]rfcLedgerProof
	Extraction   rfc.Extraction
}

// rfcLedgerStemOf assembles one summary's whole entry from the summary's OWN
// `## Meta` table.
//
// Every fact here is authored in one place. `rfc/enrolled.txt`,
// `rfc/not-enrolled.txt` and `docs/features/rfc-status.md` were three authored
// files until spec-rfc-ledger-single-declaration on 2026-09-01, and are now
// GENERATED from these tables by `./le rfc index-update`. Reading a generated
// copy to publish a fact the summary declares would be reading the echo
// (ai/rules/principles.md).
func rfcLedgerStemOf(in *rfcLedgerInput) rfcLedgerStem {
	meta := in.Render.Metas[in.Stem]
	entry := rfcLedgerStem{
		Stem:        in.Stem,
		Display:     rfcDisplayName(in.Stem),
		Title:       meta.Title,
		Enrolled:    meta.Enrolled(),
		SummaryPath: rfcSummaryPath(in.Stem),
		Successor:   meta.Successor,
		// The authored sentence in full, for an enrolled summary and for one
		// that is not: ParseMeta refuses a kind with no reason, so both sides
		// carry it.
		EnrolmentReason: meta.EnrolmentReason,
	}
	if !entry.Enrolled {
		disposition := meta.Disposition()
		entry.Disposition = &rfcLedgerDisposition{Kind: disposition.Kind, Reason: disposition.Reason}
	}
	if meta.HasRow() {
		entry.PublicStatus, entry.PublicCoverage, entry.PublicRemaining =
			meta.Status, meta.Coverage, meta.Remaining
	}
	if source, held := rfc.SourcePath(in.Tree, in.Stem); held {
		entry.SourcePath = source
	}
	if len(in.Rows) != 0 {
		entry.ShardPath = rfcShardPath(in.Stem)
	}

	cells := make(map[string]rfc.RequirementRow, len(in.Rows))
	for _, row := range in.Rows {
		cells[row.RID] = row
	}
	entry.Requirements = make([]rfcLedgerRequirement, 0, len(in.Requirements))
	for index := range in.Requirements {
		entry.Requirements = append(entry.Requirements,
			rfcLedgerRequirementOf(in, &in.Requirements[index], cells))
	}
	entry.Coverage = rfcLedgerCoverageOf(in.Bucket, entry.Requirements)
	if in.Extraction.Path != "" {
		entry.Extraction = rfcLedgerExtractionOf(in.Extraction)
	}
	return entry
}

// rfcLedgerRequirementOf assembles one requirement's entry from the cells the
// shard carries, the tagged units that prove it, and the recorded verdict.
func rfcLedgerRequirementOf(in *rfcLedgerInput, requirement *rfc.Requirement,
	cells map[string]rfc.RequirementRow,
) rfcLedgerRequirement {
	row := cells[requirement.RID]
	entry := rfcLedgerRequirement{
		RID:      requirement.RID,
		Level:    requirement.Level,
		Section:  requirement.Section,
		Text:     requirement.Text,
		Gated:    requirement.Gated(),
		Positive: row.Positive,
		Negative: row.Negative,
		Note:     row.Note,
	}
	if requirement.Annotation != nil {
		entry.Annotation = &rfcLedgerAnnotation{Kind: requirement.Annotation.Kind,
			Polarity: requirement.Annotation.Polarity, Reason: requirement.Annotation.Reason}
	}
	if requirement.Superseded != nil {
		entry.Superseded = &rfcLedgerSuccessor{Disposition: requirement.Superseded.Disposition,
			Target: requirement.Superseded.Target, Reason: requirement.Superseded.Reason}
	}
	entry.Covers = append(entry.Covers, in.Covers[requirement.RID]...)
	for index := range entry.Covers {
		key := rfc.Cover{RID: requirement.RID, Polarity: entry.Covers[index].Polarity,
			Unit: entry.Covers[index].Unit}
		if proof, held := in.Proofs[key]; held {
			entry.Covers[index].Proof = &proof
		}
	}
	entry.NightlyOnly = rfcRequirementIsNightlyOnly(in, requirement.RID)
	entry.Audit = rfcLedgerVerdictOf(in, requirement)
	return entry
}

// rfcRequirementIsNightlyOnly answers whether this requirement HAS evidence and
// none of it runs on the merge path, by the rule internal/le/rfc already holds.
func rfcRequirementIsNightlyOnly(in *rfcLedgerInput, rid string) bool {
	var found []rfc.Tag
	for _, tag := range in.Render.Tags {
		if tag.RID == rid {
			found = append(found, tag)
		}
	}
	return rfc.NightlyOnly(found, in.Render.Carriers)
}

// rfcLedgerVerdictOf answers one requirement's recorded verdict, its published
// meaning and its freshness, or nil when nobody has judged it.
func rfcLedgerVerdictOf(in *rfcLedgerInput, requirement *rfc.Requirement) *rfcLedgerVerdict {
	record, held := in.Render.Audits[requirement.RFC].Record(requirement.RID)
	if !held {
		return nil
	}
	meaning, _ := rfc.AuditVerdictMeaning(record.Verdict)
	verdict := &rfcLedgerVerdict{
		Verdict: record.Verdict, Meaning: meaning, Note: record.Note,
		Freshness: rfc.FreshState, UpgradeReason: record.UpgradeReason,
		NoCodePath: record.NoCodePath,
	}
	if state, known := in.Render.States[requirement.RID]; known {
		verdict.Freshness, verdict.Moved = state.State, state.Moved
	}
	return verdict
}

// rfcLedgerCoverageOf answers one RFC's counters: the six rfc.CoverageRows
// derived, and the four the sections under them count.
func rfcLedgerCoverageOf(bucket rfc.CoverageRow, requirements []rfcLedgerRequirement) rfcLedgerCoverage {
	coverage := rfcLedgerCoverage{
		Requirements: len(requirements),
		Gated:        bucket.Gated,
		Both:         bucket.Both,
		One:          bucket.One,
		Annotated:    bucket.Annotated,
		Missing:      bucket.Missing,
		NightlyOnly:  bucket.NightlyOnly,
	}
	for index := range requirements {
		requirement := &requirements[index]
		if requirement.Annotation != nil && requirement.Annotation.Kind == rfc.AnnotationGap {
			coverage.Gaps++
		}
		if requirement.Gated && requirement.Annotation != nil {
			bucket, known := rfcAnnotationBucket(requirement.Annotation.Kind)
			switch {
			case !known:
				coverage.UnmappedAnnotations++
			case bucket == rfcGapBucket:
				coverage.GatedGaps++
			case bucket == rfcNotApplyBucket:
				coverage.NotApplicable++
			case bucket == rfcSingleBucket:
				coverage.SinglePolarity++
			}
		}
		if requirement.Audit != nil {
			coverage.Audited++
		}
		for _, cover := range requirement.Covers {
			coverage.Tags += cover.Tags
			coverage.Units++
			if cover.Proof == nil {
				continue
			}
			coverage.Records++
			switch {
			case !cover.Proof.Proves:
				coverage.Escapes++
			case !cover.Proof.Verified:
				coverage.Stale++
			}
		}
	}
	return coverage
}

// rfcLedgerExtractionOf answers one sign-off: the census, every section, and
// every sentence the walk declined to map.
//
// The excluded sites are carried whole and never as a count. An exclusion is a
// DECISION, so the reason that took it is the fact a reader needs
// (ai/rules/rfc-compliance.md).
func rfcLedgerExtractionOf(extraction rfc.Extraction) *rfcLedgerExtraction {
	sites, sections := extraction.Unclassified()
	entry := &rfcLedgerExtraction{
		Path: extraction.Path, Reviewer: extraction.Reviewer, SignedOff: extraction.SignedOff,
		Register: extraction.Register, SourcePath: extraction.SourcePath,
		SourceSHA: extraction.SourceSHA, Mapped: extraction.Mapped(),
		Excluded: extraction.Excluded(), Relocated: extraction.Relocated(),
		Unclassified: sites + sections,
	}
	for _, section := range extraction.Sections {
		entry.Sections = append(entry.Sections, rfcLedgerSection{ID: section.ID,
			Name: section.Title(), Sites: section.Sites, Disposition: section.Disposition,
			SkipKind: section.SkipKind, Reason: section.Reason})
	}
	for _, site := range extraction.Sites {
		if site.Disposition != rfc.DispositionExcluded {
			continue
		}
		entry.Exclusions = append(entry.Exclusions, rfcLedgerExcludedSite{ID: site.ID,
			Quote: site.Quote, Kind: site.ExcludedKind, Reason: site.Reason,
			RelocatedTo: site.RelocatedTo, ReservedID: site.ReservedID})
	}
	return entry
}

// rfcRequirementsByStem groups the parsed requirements by summary stem, keeping
// the order the summary declares them in.
//
// Summary order rather than sorted order: a reader following the page against
// the summary reads the obligations as the document states them.
func rfcRequirementsByStem(requirements []rfc.Requirement) map[string][]rfc.Requirement {
	out := map[string][]rfc.Requirement{}
	for index := range requirements {
		out[requirements[index].RFC] = append(out[requirements[index].RFC], requirements[index])
	}
	return out
}

// rfcCoversByRID groups the tagged units by requirement id, sorted by polarity
// and then by unit so the published page is byte-stable.
func rfcCoversByRID(covers map[rfc.Cover][]rfc.Tag, carriers []rfc.Carrier) map[string][]rfcLedgerCover {
	out := map[string][]rfcLedgerCover{}
	for key, tags := range covers {
		entry := rfcLedgerCover{Polarity: key.Polarity, Unit: key.Unit, Tags: len(tags)}
		if len(tags) != 0 {
			entry.Claim = tags[0].Claim
			entry.File, entry.Line = tags[0].File, tags[0].Line
			if carrier, held := rfc.CarrierFor(tags[0].File, carriers); held {
				entry.Carrier = carrier.Label()
			}
		}
		out[key.RID] = append(out[key.RID], entry)
	}
	for rid := range out {
		sortRFCCovers(out[rid])
	}
	return out
}

// rfcProofsByCover keys every re-verified proof on the tagged unit it proves.
func rfcProofsByCover(verdicts []rfc.DiscriminationVerdict) map[rfc.Cover]rfcLedgerProof {
	out := make(map[rfc.Cover]rfcLedgerProof, len(verdicts))
	for index := range verdicts {
		verdict := &verdicts[index]
		out[verdict.Record.Cover()] = rfcLedgerProof{
			Route: verdict.Record.Route, State: verdict.State, Detail: verdict.Detail,
			Reason: verdict.Record.Reason, Producer: verdict.Record.Producer,
			Break: verdict.Record.Break, Verified: verdict.Verified(),
			Proves: verdict.Record.Proves(),
		}
	}
	return out
}

// sortRFCCovers orders one requirement's tagged units by polarity and then by
// unit, so a published page is byte-stable across machines.
func sortRFCCovers(covers []rfcLedgerCover) {
	sort.Slice(covers, func(left, right int) bool {
		if covers[left].Polarity != covers[right].Polarity {
			return covers[left].Polarity < covers[right].Polarity
		}
		return covers[left].Unit < covers[right].Unit
	})
}

// rfcSummaryPath and rfcShardPath answer the two repository files one stem has.
func rfcSummaryPath(stem string) string { return "rfc/short/" + stem + ".md" }
func rfcShardPath(stem string) string   { return "rfc/requirements/" + stem + ".md" }
