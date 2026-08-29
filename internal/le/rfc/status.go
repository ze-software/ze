// Design: docs/architecture/core-design.md -- the extraction envelope
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// status.go answers the counts the umbrella's drain quota consumes.
//
// Every figure is DERIVED from rfc/extraction/ plus the live summaries. There
// is no second hand-kept list of who has been signed off: that is the rotting
// registry ai/rules/evidence.md forbids.
package rfc

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Status is the extraction envelope, as data.
//
// The keys are kebab-case because every le command answers structured data and
// `| json` renders it. They are also what the umbrella's counter reads, so the
// spelling is a contract rather than a style.
type Status struct {
	SchemaVersion int `json:"schema-version"`
	Enrolled      int `json:"enrolled"`
	// Signed is counted INDEPENDENTLY of the split, never as the sum of it.
	// Defining the total as the sum makes "the keys sum to the published
	// total" a tautology no test could ever fail; two derivations that must
	// agree is a real cross-check, and a register dropped from the split now
	// disagrees with the total out loud.
	Signed           int            `json:"signed"`
	SignedByRegister map[string]int `json:"signed-by-register"`
	// Relocated is published beside the totals rather than folded into them. A
	// relocation counts as an exclusion everywhere a decision is taken, because
	// it is one: this walk declined to map the sentence. It is counted apart
	// HERE because what it declined to do and what a dismissal declined to do
	// are different facts, and a drain policy that could not tell them apart
	// could not act on either.
	Relocated int      `json:"relocated"`
	Backlog   int      `json:"backlog"`
	Unsigned  []string `json:"unsigned"`
}

// Text renders the envelope the way the script printed it: json.dumps with
// two-space indentation and sorted keys.
//
// It is the DEFAULT rendering and nothing more. The envelope's only consumer
// reads this shape, so a command that answered Go's compact encoding by default
// would have moved the contract while passing every verdict comparison.
func (s Status) Text() string {
	var tb textbuf.Buffer
	return tb.Str(pyDump(s.document())).Byte('\n').String()
}

// document is the envelope as the map the page renderer walks.
//
// It spells the keys a second time, because the encoder answers a map with
// float values and the page prints integers. The two spellings are held
// together by a test rather than by this comment:
// TestTheTwoRenderingsOfTheEnvelopeCarryTheSameKeys compares the key set here
// against the one json.Marshal produces, so a tag renamed on one side reddens
// rather than leaving `| json` and the page disagreeing about what the answer
// holds.
func (s Status) document() map[string]any {
	return map[string]any{
		keySchemaVersion:     s.SchemaVersion,
		"enrolled":           s.Enrolled,
		"signed":             s.Signed,
		"signed-by-register": s.SignedByRegister,
		"relocated":          s.Relocated,
		"backlog":            s.Backlog,
		"unsigned":           s.Unsigned,
	}
}

// extractionStatus answers the envelope for one checkout.
func extractionStatus(deriver *Deriver, requirements []Requirement,
	enrolled map[string]bool) (Status, error) {
	valid, _, err := evaluateExtractions(deriver, requirements)
	if err != nil {
		return Status{}, err
	}
	signed := credited(valid, enrolled)
	counts := registerCounts(signed)

	unsigned := make([]string, 0)
	for _, stem := range sortedSet(enrolled) {
		if _, held := signed[stem]; !held {
			unsigned = append(unsigned, stem)
		}
	}
	relocated := 0
	for stem := range signed {
		relocated += signed[stem].Relocated()
	}
	return Status{
		SchemaVersion:    extractionSchemaVersion,
		Enrolled:         len(enrolled),
		Signed:           len(signed),
		SignedByRegister: counts,
		Relocated:        relocated,
		Backlog:          len(unsigned),
		Unsigned:         unsigned,
	}, nil
}

// Collected is what every driver of this gate reads: the enrolled set, every
// requirement, the parse failures, and the tag scan.
//
// The tag scan is here rather than in the drivers that consume its RESULT,
// because two of them discard it and still depend on it: scanning refuses a
// malformed tag and a tag in a carrier nothing executes, and a gate that
// skipped the scan would answer a clean envelope over a tree whose evidence
// cannot be read.
type Collected struct {
	Enrolled     map[string]bool
	Requirements []Requirement
	ParseErrors  []string
	ParseByStem  map[string]string
	Tags         []Tag
}

// Collect parses every summary tolerantly and scans the tree once.
//
// It returns EVERY parse error. Enrolment does not filter what is reported: a
// summary the gate cannot read is a summary whose obligations nobody can see,
// enrolled or not.
func Collect(tree string) (Collected, error) {
	enrolled, err := loadEnrolled(tree)
	if err != nil {
		return Collected{}, err
	}
	stems, err := summaryStems(tree)
	if err != nil {
		return Collected{}, err
	}
	out := Collected{Enrolled: enrolled, ParseByStem: map[string]string{}}
	for _, stem := range sortedSet(stems) {
		var name textbuf.Buffer
		path := treePath(tree, summaryRel, name.Str(stem).Str(".md").String())
		reqs, parseErr := parseSummaryFile(tree, path)
		if parseErr != nil {
			if !isParseError(parseErr) {
				return Collected{}, parseErr
			}
			out.ParseErrors = append(out.ParseErrors, parseErr.Error())
			out.ParseByStem[stem] = parseErr.Error()
			continue
		}
		out.Requirements = append(out.Requirements, reqs...)
	}
	if out.Tags, err = ScanTree(tree); err != nil {
		return Collected{}, err
	}
	return out, nil
}
