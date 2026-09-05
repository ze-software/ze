// Design: docs/contributing/rfc-conformance-gates.md -- the extraction sign-off walk
// Overview: extraction_create.go -- the skeleton whose dispositions this applies
// Related: artifact.go -- the closed vocabularies every decision is held to
//
// extraction_classify.go is `le rfc extraction-classify`. A walk over a long
// RFC decides hundreds of sites, and applying those decisions by hand was the
// one step le did not provide, so two throwaway scripts did it. This is that
// step, with the refusals the scripts had and three the rule requires.
//
// It transcribes, and it does not author. It writes no disposition the
// decisions file does not name, holds no default, matches no locator pattern,
// and refuses a decision for a site the source does not derive. A site the file
// leaves out stays UNCLASSIFIED and is named in the report: an obligation
// nobody could classify is a question for the owner, never a null annotated
// away (ai/rules/rfc-compliance.md).
package rfc

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// keyDecisions is the keyword `extraction-classify` takes its one value under:
// the path of the file that carries the walk's decisions. Keyword before value
// (ai/rules/cli.md), and the file names the stem it decides, so no second
// keyword can disagree with it about which RFC is being classified.
const keyDecisions = "decisions"

// quoteMin is the shortest span inside quotation marks this writer will accept
// as the quoted SENTENCE a feature-out-of-scope reason owes. Below it a match
// says nothing: "MAY" appears in every RFC, and a reason quoting one word has
// not shown the reader the construction that made the feature optional.
const quoteMin = 20

// quotedSpanRE finds what an author put inside quotation marks. Both spellings
// are live in the landed corpus, and a reason often nests one inside the other,
// which is why every span is tried rather than the longest.
var quotedSpanRE = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)

// citedStemRE finds the documents a reason names, so a quote from a document
// this one only cites is checked against the document it came from.
var citedStemRE = regexp.MustCompile(`(?i)\bRFC[ -]?(\d{3,5})\b`)

// classifyDecisions is the authored input: one decision per site and per
// section, plus the sign-off fields the walk earned.
//
// It is not the artifact. Every derived field -- the quote, the site list, the
// register, the source fingerprint -- is absent on purpose, because a decisions
// file that could restate one could disagree with the source.
type classifyDecisions struct {
	Stem           string                    `json:"stem"`
	SignedOff      string                    `json:"signed-off,omitempty"`
	Reviewer       string                    `json:"reviewer,omitempty"`
	RegisterReason string                    `json:"register-reason,omitempty"`
	ResignReason   string                    `json:"resign-reason,omitempty"`
	Sections       []classifySectionDecision `json:"sections,omitempty"`
	Sites          []classifySiteDecision    `json:"sites,omitempty"`
}

// classifySiteDecision is one sentence's decision.
//
// Producer and Residual are authored here and reach no artifact. Producer is
// what binds-another-role owes and the reason must carry it, so the check is
// on a field the author had to write rather than on prose a reader must judge.
// Residual is why a site was left UNCLASSIFIED, and it is printed rather than
// stored, because the artifact records decisions and this is the absence of
// one.
type classifySiteDecision struct {
	ID           string `json:"id"`
	Disposition  string `json:"disposition,omitempty"`
	MappedTo     string `json:"mapped-to,omitempty"`
	ExcludedKind string `json:"excluded-kind,omitempty"`
	Reason       string `json:"reason,omitempty"`
	RelocatedTo  string `json:"relocated-to,omitempty"`
	ReservedID   string `json:"reserved-id,omitempty"`
	Producer     string `json:"producer,omitempty"`
	Residual     string `json:"residual,omitempty"`
}

// classifySectionDecision is one section's decision.
type classifySectionDecision struct {
	ID           string   `json:"id"`
	Disposition  string   `json:"disposition,omitempty"`
	SkipKind     string   `json:"skip-kind,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	UnsourcedIDs []string `json:"unsourced-ids,omitempty"`
}

// classifyResidual is one site left unclassified, and what the walk said about
// it. Note is empty when the decisions file said nothing, which the report
// prints as its own sentence rather than as a blank cell.
type classifyResidual struct {
	ID   string `json:"id"`
	Note string `json:"note"`
}

// extractionClassifyReport is the local-data answer for one applied walk.
type extractionClassifyReport struct {
	Path                 string             `json:"path"`
	Destination          string             `json:"destination"`
	Placed               bool               `json:"placed"`
	Stem                 string             `json:"stem"`
	Sites                int                `json:"sites"`
	Sections             int                `json:"sections"`
	SitesDecided         int                `json:"sites-decided"`
	SectionsDecided      int                `json:"sections-decided"`
	Mapped               int                `json:"mapped"`
	Excluded             int                `json:"excluded"`
	ExclusionRatio       float64            `json:"exclusion-ratio"`
	BindsAnotherRole     int                `json:"binds-another-role"`
	Unclassified         []classifyResidual `json:"unclassified"`
	UnclassifiedSections []string           `json:"unclassified-sections"`
}

// Text answers what the walk decided, what it left, and where the bytes went.
//
// The exclusion ratio leads the counts because it is the control a reviewer
// reads on a FIRST sign-off, which no ratchet can compare against a baseline
// (rfc/extraction/README.md). The unclassified sites follow, one line each: a
// count alone lets a hard obligation leave as a number.
func (r extractionClassifyReport) Text() string {
	var out textbuf.Buffer
	out.Str("wrote ").Str(r.Path).Str(": ").Int(int64(r.SitesDecided)).Str(" of ").
		Int(int64(r.Sites)).Str(" site(s) and ").Int(int64(r.SectionsDecided)).Str(" of ").
		Int(int64(r.Sections)).Str(" section(s) decided by this file.\n").
		Str("mapped ").Int(int64(r.Mapped)).Str(", excluded ").Int(int64(r.Excluded)).
		Str(" (").Float2(r.ExclusionRatio).Str(" of every site), of which ").
		Int(int64(r.BindsAnotherRole)).Str(" bind another role.\n")

	for _, left := range r.Unclassified {
		out.Str("  UNCLASSIFIED ").Str(left.ID).Str(": ")
		if left.Note == "" {
			out.Str("no reason recorded, which is itself the defect\n")
			continue
		}
		out.Str(left.Note).Byte('\n')
	}
	for _, id := range r.UnclassifiedSections {
		out.Str("  UNCLASSIFIED section ").Str(id).Byte('\n')
	}

	if r.Placed {
		return out.Str("Every site and section carries a disposition, so the walk was ").
			Str("written to the corpus.\n").String()
	}
	return out.Int(int64(len(r.Unclassified))).Str(" site(s) and ").
		Int(int64(len(r.UnclassifiedSections))).Str(" section(s) are UNCLASSIFIED, so the ").
		Str("walk was NOT written to ").Str(r.Destination).Str(".\n").
		Str("Report each one to the owner: an obligation no honest disposition fits is a ").
		Str("question, never a null to annotate away.\n").String()
}

// classifyExtraction applies one decisions file to its stem's skeleton.
func classifyExtraction(tree, decisionsPath string) (extractionClassifyReport, error) {
	decisions, err := readClassifyDecisions(tree, decisionsPath)
	if err != nil {
		return extractionClassifyReport{}, err
	}
	rel := relTo(tree, decisionsPath)
	if err := validateExtractionStem(decisions.Stem); err != nil {
		var message textbuf.Buffer
		return extractionClassifyReport{}, errors.New(message.Str(rel).Str(": ").Err(err).String())
	}

	document, err := deriveExtractionDocument(tree, decisions.Stem)
	if err != nil {
		return extractionClassifyReport{}, err
	}
	report, err := applyClassifyDecisions(tree, rel, decisions, &document)
	if err != nil {
		return extractionClassifyReport{}, err
	}

	report.Destination = relTo(tree, treePath(tree, extractionRel+"/"+decisions.Stem+".json"))
	report.Path, report.Placed, err = placeExtractionDocument(tree, decisions.Stem, document)
	if err != nil {
		return extractionClassifyReport{}, err
	}
	return report, nil
}

// readClassifyDecisions reads and decodes one decisions file.
//
// An unknown key is refused rather than ignored, because a misspelled
// `excluded-kind` would otherwise apply an exclusion with no kind and the
// author would read the refusal as a schema defect somewhere else.
func readClassifyDecisions(tree, path string) (classifyDecisions, error) {
	rel := relTo(tree, path)
	raw, err := os.ReadFile(path) // #nosec G304 -- the decisions file the author names, read once
	if err != nil {
		var message textbuf.Buffer
		return classifyDecisions{}, errors.New(message.Str(rel).Str(": cannot read: ").Err(err).String())
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var decisions classifyDecisions
	if err := decoder.Decode(&decisions); err != nil {
		var message textbuf.Buffer
		return classifyDecisions{}, errors.New(message.Str(rel).
			Str(": cannot read the decisions: ").Err(err).String())
	}
	if len(decisions.Sites) == 0 && len(decisions.Sections) == 0 {
		var message textbuf.Buffer
		return classifyDecisions{}, errors.New(message.Str(rel).
			Str(": names no site and no section decision. A decisions file that decides ").
			Str("nothing cannot be told from one whose decisions went to the wrong keys").String())
	}
	return decisions, nil
}

// applyClassifyDecisions validates every decision and writes it into document.
//
// Nothing is written into document until every decision has passed, so a
// refusal leaves the caller with a document it will not place: a half-applied
// walk on disk is the one outcome worse than a refused one.
func applyClassifyDecisions(tree, rel string, decisions classifyDecisions,
	document *extractionDocument) (extractionClassifyReport, error) {
	source, held := SourceText(tree, decisions.Stem)
	if !held {
		var message textbuf.Buffer
		return extractionClassifyReport{}, errors.New(message.Str(rel).Str(": ").
			Str(decisions.Stem).Str(" has no source text under rfc/full/ or rfc/drafts/, so a ").
			Str("quoted sentence cannot be checked against the document it claims to quote").String())
	}

	siteAt := map[string]int{}
	for index, site := range document.Sites {
		siteAt[site.ID] = index
	}
	sectionAt := map[string]int{}
	for index, section := range document.Sections {
		sectionAt[section.ID] = index
	}

	if err := validateSiteDecisions(tree, rel, decisions, source, siteAt, document); err != nil {
		return extractionClassifyReport{}, err
	}
	if err := validateSectionDecisions(rel, decisions, sectionAt); err != nil {
		return extractionClassifyReport{}, err
	}

	report := extractionClassifyReport{
		Stem:            decisions.Stem,
		Sites:           len(document.Sites),
		Sections:        len(document.Sections),
		SitesDecided:    len(decisions.Sites),
		SectionsDecided: len(decisions.Sections),
	}
	residual := map[string]string{}
	for _, decision := range decisions.Sites {
		if decision.Disposition == "" {
			residual[decision.ID] = decision.Residual
			continue
		}
		document.Sites[siteAt[decision.ID]] = siteFromDecision(document.Sites[siteAt[decision.ID]], decision)
	}
	for _, decision := range decisions.Sections {
		document.Sections[sectionAt[decision.ID]] = sectionFromDecision(
			document.Sections[sectionAt[decision.ID]], decision)
	}
	if decisions.SignedOff != "" {
		document.SignedOff = decisions.SignedOff
	}
	if decisions.Reviewer != "" {
		document.Reviewer = decisions.Reviewer
	}
	if decisions.RegisterReason != "" {
		document.RegisterReason = decisions.RegisterReason
	}
	if decisions.ResignReason != "" {
		document.ResignReason = decisions.ResignReason
	}

	censusClassifyReport(&report, residual, document)
	return report, nil
}

// censusClassifyReport counts what the applied document now holds.
func censusClassifyReport(report *extractionClassifyReport, residual map[string]string,
	document *extractionDocument) {
	for _, site := range document.Sites {
		if site.Disposition == nil {
			report.Unclassified = append(report.Unclassified,
				classifyResidual{ID: site.ID, Note: residual[site.ID]})
			continue
		}
		switch *site.Disposition {
		case DispositionMapped:
			report.Mapped++
		case DispositionExcluded:
			report.Excluded++
			if site.ExcludedKind == bindsAnotherRole {
				report.BindsAnotherRole++
			}
		}
	}
	for _, section := range document.Sections {
		if section.Disposition == nil {
			report.UnclassifiedSections = append(report.UnclassifiedSections, section.ID)
		}
	}
	if len(document.Sites) > 0 {
		report.ExclusionRatio = float64(report.Excluded) / float64(len(document.Sites))
	}
}

// siteFromDecision rebuilds one site entry from its decision.
//
// It starts from the DERIVED fields alone, so a decision that replaces a
// carried-forward exclusion with a mapping cannot leave the exclusion's kind
// and reason behind it.
func siteFromDecision(derived extractionDocumentSite, decision classifySiteDecision) extractionDocumentSite {
	entry := extractionDocumentSite{ID: derived.ID, Quote: derived.Quote}
	entry.Disposition = stringPointer(decision.Disposition)
	entry.MappedTo = decision.MappedTo
	if decision.Disposition == DispositionExcluded {
		entry.ExcludedKind = decision.ExcludedKind
		entry.Reason = decision.Reason
		entry.RelocatedTo = decision.RelocatedTo
		entry.ReservedID = decision.ReservedID
	}
	return entry
}

// sectionFromDecision rebuilds one section entry from its decision.
func sectionFromDecision(derived extractionDocumentSection,
	decision classifySectionDecision) extractionDocumentSection {
	entry := extractionDocumentSection{ID: derived.ID, Sites: derived.Sites}
	entry.Disposition = stringPointer(decision.Disposition)
	entry.UnsourcedIDs = decision.UnsourcedIDs
	if decision.Disposition == dispositionSkipped {
		entry.SkipKind = decision.SkipKind
		entry.Reason = decision.Reason
	}
	return entry
}

// validateSiteDecisions holds every site decision to the closed vocabularies
// and to the three things a kind owes beyond them.
func validateSiteDecisions(tree, rel string, decisions classifyDecisions, source string,
	siteAt map[string]int, document *extractionDocument) error {
	seen := map[string]bool{}
	for _, decision := range decisions.Sites {
		index, derived := siteAt[decision.ID]
		if !derived {
			var message textbuf.Buffer
			return errors.New(message.Str(rel).Str(": site ").Str(pyRepr(decision.ID)).
				Str(" is not a site this source derives. A decision for a sentence that is ").
				Str("not there classifies nothing, and it is the shape a decisions file ").
				Str("applied to the wrong RFC takes").String())
		}
		if seen[decision.ID] {
			var message textbuf.Buffer
			return errors.New(message.Str(rel).Str(": site ").Str(pyRepr(decision.ID)).
				Str(" is decided twice, so one of the two decisions is unreachable").String())
		}
		seen[decision.ID] = true
		if err := validateSiteDecision(tree, rel, decisions.Stem, source, decision,
			document.Sites[index]); err != nil {
			return err
		}
	}
	return nil
}

// validateSiteDecision holds ONE decision to what its disposition owes.
func validateSiteDecision(tree, rel, stem, source string, decision classifySiteDecision,
	derived extractionDocumentSite) error {
	var where textbuf.Buffer
	place := where.Str(rel).Str(": site ").Str(decision.ID).String()

	if decision.Disposition == "" {
		return validateResidual(place, decision, derived)
	}
	if decision.Residual != "" {
		var message textbuf.Buffer
		return errors.New(message.Str(place).
			Str(": carries both a disposition and a 'residual'. A residual says the walk ").
			Str("could not classify the sentence, so the two cannot both be true").String())
	}
	if !siteDispositions[decision.Disposition] {
		var message textbuf.Buffer
		return errors.New(message.Str(place).Str(": disposition ").Str(pyRepr(decision.Disposition)).
			Str(" is not one of ").Str(pyRepr(SiteDispositions())).
			Str(". Leave the disposition out to record the site as unclassified").String())
	}
	if decision.Disposition == DispositionMapped {
		return validateMappedDecision(place, decision)
	}
	return validateExcludedDecision(tree, place, stem, source, decision)
}

// validateResidual holds a site the walk left unclassified.
func validateResidual(place string, decision classifySiteDecision, derived extractionDocumentSite) error {
	if decision.MappedTo != "" || decision.ExcludedKind != "" || decision.Reason != "" ||
		decision.RelocatedTo != "" || decision.ReservedID != "" || decision.Producer != "" {
		var message textbuf.Buffer
		return errors.New(message.Str(place).
			Str(": carries decision fields with no 'disposition', so none of them would be ").
			Str("written. Name the disposition, or leave only 'residual'").String())
	}
	if derived.Disposition == nil {
		return nil
	}
	var message textbuf.Buffer
	return errors.New(message.Str(place).Str(": already carries the disposition ").
		Str(pyRepr(*derived.Disposition)).Str(" from the landed sign-off, which a 'residual' ").
		Str("cannot withdraw. Re-classify the site, or leave it out of this file").String())
}

// validateMappedDecision holds a mapping to the id it must name.
func validateMappedDecision(place string, decision classifySiteDecision) error {
	if decision.ExcludedKind != "" || decision.RelocatedTo != "" ||
		decision.ReservedID != "" || decision.Producer != "" {
		var message textbuf.Buffer
		return errors.New(message.Str(place).
			Str(": a mapping carries an exclusion's fields, which nothing would write").String())
	}
	if decision.MappedTo != "" {
		return nil
	}
	var message textbuf.Buffer
	return errors.New(message.Str(place).
		Str(": mapped needs a 'mapped-to' naming the requirement id in rfc/short/ that this ").
		Str("sentence is the source of").String())
}

// validateExcludedDecision holds an exclusion to the closed vocabulary, to a
// reason, and to what its own kind owes on top.
func validateExcludedDecision(tree, place, stem, source string, decision classifySiteDecision) error {
	if _, held := exclusionKinds[decision.ExcludedKind]; !held {
		var message textbuf.Buffer
		return errors.New(message.Str(place).Str(": excluded needs an 'excluded-kind' from ").
			Str(pyRepr(ExclusionKinds())).Str(", got ").Str(pyRepr(decision.ExcludedKind)).
			Str(". The vocabulary is closed because each kind names the decision that put ").
			Str("the obligation out of reach, and a kind nobody defined names none").String())
	}
	if strings.TrimSpace(decision.Reason) == "" {
		var message textbuf.Buffer
		return errors.New(message.Str(place).
			Str(": excluded needs a non-empty 'reason'. A bare exclusion is an escape hatch; ").
			Str("say why this sentence binds nothing").String())
	}
	if decision.ExcludedKind != relocatedToSpec &&
		(decision.RelocatedTo != "" || decision.ReservedID != "") {
		var message textbuf.Buffer
		return errors.New(message.Str(place).
			Str(": 'relocated-to' and 'reserved-id' mean something only on a ").
			Str(relocatedToSpec).Str(" exclusion").String())
	}
	if decision.ExcludedKind != bindsAnotherRole && decision.Producer != "" {
		var message textbuf.Buffer
		return errors.New(message.Str(place).Str(": 'producer' is what ").Str(bindsAnotherRole).
			Str(" owes, and on any other kind it reaches no reader").String())
	}

	switch decision.ExcludedKind {
	case exclusionDuplicate:
		if decision.MappedTo == "" {
			var message textbuf.Buffer
			return errors.New(message.Str(place).Str(": ").Str(exclusionDuplicate).
				Str(" needs a 'mapped-to' naming the id that already captures this ").
				Str("obligation. A duplicate that names nothing cannot be compared with ").
				Str("anything").String())
		}
	case relocatedToSpec:
		if _, _, err := validateRelocation(decision.RelocatedTo, decision.ReservedID, place, stem); err != nil {
			return err
		}
	case bindsAnotherRole:
		return validateBindsAnotherRole(tree, place, decision)
	case featureOutOfScope:
		return validateFeatureOutOfScope(tree, place, stem, source, decision)
	}
	return nil
}

// validateBindsAnotherRole holds the one kind the repository presumes wrong.
//
// `binds-another-role` reads on the public ledger as "not our problem" where
// the truth is usually "our problem, unbuilt", so the rule requires the reason
// to name the role, show Ze never acts as it, and cite the producer that would
// act as it if Ze did (ai/rules/rfc-compliance.md, owner directive
// 2026-08-31). No gate can read a sentence, so what is checked here is the
// citation: the producer is a field the author had to write, it has to appear
// in the reason a reader will meet, and where it names a path in this tree that
// path has to exist. A producer nobody can open is the failure this catches.
func validateBindsAnotherRole(tree, place string, decision classifySiteDecision) error {
	if decision.Producer == "" {
		var message textbuf.Buffer
		return errors.New(message.Str(place).Str(": ").Str(bindsAnotherRole).
			Str(" needs a 'producer': the file that would act as the role if Ze did, or the ").
			Str("role itself where Ze holds no code for it. This kind is PRESUMED WRONG ").
			Str("(ai/rules/rfc-compliance.md): Ze speaks both sides of nearly every ").
			Str("protocol, so an obligation addressed to a sender or a receiver almost ").
			Str("always binds it, and an unbuilt role is a gap rather than another role").String())
	}
	if !strings.Contains(decision.Reason, decision.Producer) {
		var message textbuf.Buffer
		return errors.New(message.Str(place).Str(": the 'reason' does not carry the producer ").
			Str(pyRepr(decision.Producer)).Str(". The reason is what lands in the artifact and ").
			Str("what a reader of the ledger meets, so a producer named only here is named ").
			Str("nowhere").String())
	}
	if !strings.Contains(decision.Producer, "/") {
		return nil
	}
	if _, err := os.Stat(treePath(tree, decision.Producer)); err == nil {
		return nil
	}
	var message textbuf.Buffer
	return errors.New(message.Str(place).Str(": the producer ").Str(pyRepr(decision.Producer)).
		Str(" is not in this tree. A citation a reader cannot open is the shape a producer ").
		Str("nobody read takes; name the path as it is written from the checkout root").String())
}

// validateFeatureOutOfScope holds the kind that excludes an obligation
// conditional on an OPTIONAL feature.
//
// The rule requires the reason to QUOTE the sentence that makes the feature
// optional (ai/rules/rfc-compliance.md, owner directive 2026-08-31), and that
// is a claim about a document this repository holds, so it is checked against
// the document. A reason quoting a document the source only CITES is checked
// against that document, which is how the RFC 5176 site quoting RFC 2865
// Section 5.29 is a true citation rather than a mismatch.
func validateFeatureOutOfScope(tree, place, stem, source string, decision classifySiteDecision) error {
	quoted := quotedSpans(decision.Reason)
	if len(quoted) == 0 {
		var message textbuf.Buffer
		return errors.New(message.Str(place).Str(": ").Str(featureOutOfScope).
			Str(" needs the reason to QUOTE the sentence that makes the feature optional, ").
			Str("inside quotation marks and at least ").Int(quoteMin).
			Str(" characters long. Without it the reason asserts the option rather than ").
			Str("showing it, and a MUST conditional on a feature Ze DOES offer would read ").
			Str("the same").String())
	}
	for _, document := range citedSources(tree, stem, source, decision.Reason) {
		for _, span := range quoted {
			if strings.Contains(document, span) {
				return nil
			}
		}
	}
	var message textbuf.Buffer
	return errors.New(message.Str(place).Str(": no quoted sentence in the reason appears in ").
		Str(stem).Str("'s own text or in any RFC the reason names. Quote the sentence ").
		Str("verbatim from rfc/full/, and fetch the text first when the sentence belongs to a ").
		Str("document this repository does not hold").String())
}

// quotedSpans answers what an author put inside quotation marks, normalized
// for the line wrapping an RFC's own text carries.
func quotedSpans(reason string) []string {
	var spans []string
	for _, match := range quotedSpanRE.FindAllStringSubmatch(reason, -1) {
		span := match[1]
		if span == "" {
			span = match[2]
		}
		flat := strings.Join(strings.Fields(span), " ")
		if len(flat) >= quoteMin {
			spans = append(spans, flat)
		}
	}
	return spans
}

// citedSources answers the texts a quote may have come from: this stem's own,
// and every RFC the reason names by number.
func citedSources(tree, stem, source, reason string) []string {
	texts := []string{flattenSource(source)}
	for _, match := range citedStemRE.FindAllStringSubmatch(reason, -1) {
		var name textbuf.Buffer
		cited := name.Str("rfc").Str(match[1]).String()
		if cited == stem {
			continue
		}
		text, held := SourceText(tree, cited)
		if !held {
			continue
		}
		texts = append(texts, flattenSource(text))
	}
	return texts
}

// flattenSource answers one source text as a single line, so a quote that
// crosses a line wrap or a page break still matches the sentence it came from.
func flattenSource(text string) string {
	return strings.Join(strings.Fields(stripPageFurniture(text)), " ")
}

// validateSectionDecisions holds every section decision to its closed set.
func validateSectionDecisions(rel string, decisions classifyDecisions, sectionAt map[string]int) error {
	seen := map[string]bool{}
	for _, decision := range decisions.Sections {
		if _, derived := sectionAt[decision.ID]; !derived {
			var message textbuf.Buffer
			return errors.New(message.Str(rel).Str(": section ").Str(pyRepr(decision.ID)).
				Str(" is not a section this source derives").String())
		}
		if seen[decision.ID] {
			var message textbuf.Buffer
			return errors.New(message.Str(rel).Str(": section ").Str(pyRepr(decision.ID)).
				Str(" is decided twice, so one of the two decisions is unreachable").String())
		}
		seen[decision.ID] = true
		if err := validateSectionDecision(rel, decision); err != nil {
			return err
		}
	}
	return nil
}

// validateSectionDecision holds ONE section decision.
func validateSectionDecision(rel string, decision classifySectionDecision) error {
	var where textbuf.Buffer
	place := where.Str(rel).Str(": section ").Str(decision.ID).String()

	if !sectionDispositions[decision.Disposition] {
		var message textbuf.Buffer
		return errors.New(message.Str(place).Str(": disposition ").Str(pyRepr(decision.Disposition)).
			Str(" is not one of ").Str(pyRepr(SectionDispositions())).
			Str(". Leave the section out of this file to record it as unclassified").String())
	}
	if decision.Disposition != dispositionSkipped {
		if decision.SkipKind == "" {
			return nil
		}
		var message textbuf.Buffer
		return errors.New(message.Str(place).
			Str(": a walked section carries a 'skip-kind', which nothing would write").String())
	}
	if !sectionSkipKinds[decision.SkipKind] {
		var message textbuf.Buffer
		return errors.New(message.Str(place).Str(": skipped needs a 'skip-kind' from ").
			Str(pyRepr(SectionSkipKinds())).Str(", got ").Str(pyRepr(decision.SkipKind)).String())
	}
	if strings.TrimSpace(decision.Reason) != "" {
		return nil
	}
	var message textbuf.Buffer
	return errors.New(message.Str(place).
		Str(": skipped needs a non-empty 'reason' saying what the section holds instead of ").
		Str("obligations").String())
}
