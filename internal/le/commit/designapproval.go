// Design: docs/contributing/committing.md -- the owner keyword that changes a design decision, and its trailer
// Related: authorization.go -- the owner terminal that admits decision-change
//
// designapproval.go writes the trailers that record Thomas's approval of a
// design: today the Decision-changed: trailer of `./le commit create ...
// decision-change "<his words>"`, which changes or removes a row of a page's
// Design decisions section outside a spec
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-11).
package commit

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
	docconsistency "github.com/ze-software/ze/internal/le/doc/consistency"
)

// decisionChangeWordsProblem refuses owner words that span lines: a trailer
// is one line, and a second line would start a line of its own in the
// message.
func decisionChangeWordsProblem(words string) error {
	if !strings.ContainsAny(words, "\r\n") {
		return nil
	}
	return errors.New("decision-change refused: Thomas's words span lines, and a trailer is one line; " +
		`give them as one line: decision-change "<what he said>"`)
}

// decisionChangedTrailers answers one `Decision-changed: <page>: <words>`
// trailer per page whose decision rows change changes or removes, when
// change.OwnerWords carries Thomas's words, and none when it carries none.
// Owner words on a commit that changes no decision row are refused: they
// would record an admission of nothing.
func decisionChangedTrailers(change changecheck.Change) ([]string, error) {
	words := strings.TrimSpace(change.OwnerWords)
	if words == "" {
		return nil, nil
	}
	lost, err := docconsistency.LostDecisions(change)
	if err != nil {
		return nil, couldNotRun(errors.Join(errors.New("read the decision rows this commit changes"), err))
	}
	var pages []string
	for _, decision := range lost {
		if !slices.Contains(pages, decision.Page) {
			pages = append(pages, decision.Page)
		}
	}
	if len(pages) == 0 {
		return nil, refusedKeywords(errors.New("decision-change refused: this commit changes or removes no row of a " +
			"## Design decisions section, so Thomas's words would admit nothing; drop decision-change, " +
			"or carry the page whose decision he changed"))
	}
	slices.Sort(pages)
	trailers := make([]string, 0, len(pages))
	for _, page := range pages {
		trailers = append(trailers, docconsistency.DecisionChangedTrailer+page+": "+words)
	}
	return trailers, nil
}
