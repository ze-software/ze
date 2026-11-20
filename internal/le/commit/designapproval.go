// Design: docs/contributing/committing.md -- the owner keyword that changes a design decision, and its trailer
// Related: authorization.go -- the owner terminal that admits decision-change
//
// designapproval.go writes the trailers that record Thomas's approval of a
// design, and refuses a closure that leaves a departure unapproved or
// uncorrected (plan/release/validation/spec-design-needs-owner-approval.md):
// the Decision-changed: trailer of `./le commit create ... decision-change
// "<his words>"`, which changes or removes a row of a page's Design decisions
// section outside a spec (AC-11); the Design-approved: trailer that carries an
// approval, or an away admission, to every checkout git reaches (AC-28,
// AC-31); and the two closure checks over the spec's departs rows (AC-32,
// AC-33). What an approval is, and which text it binds, is the spec design
// package's one lookup; this file asks it and names no record.
package commit

import (
	"errors"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
	"github.com/ze-software/ze/internal/le/commit/hunks"
	docconsistency "github.com/ze-software/ze/internal/le/doc/consistency"
	specdesign "github.com/ze-software/ze/internal/le/spec/design"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// designApprovedTrailers answers one Design-approved: line per spec the
// commit carries whose design text, as the commit carries it, an approval
// record or an away admission covers and no reachable commit records yet
// (specdesign.OwedApprovalTrailer), in the order paths lists them. A removed
// spec is carried by no text, so a closure writes none.
func designApprovedTrailers(root string, paths []string, content prospectiveContent, now time.Time) ([]string, error) {
	var trailers []string
	for _, carried := range paths {
		if !specpath.IsSpec(carried) {
			continue
		}
		text, present, problem := committedText(root, content.tree, carried)
		if problem != "" {
			return nil, couldNotRun(errors.New("read the carried spec " + carried + ": " + problem))
		}
		if !present {
			continue
		}
		line, err := specdesign.OwedApprovalTrailer(root, carried, text, now)
		if err != nil {
			return nil, couldNotRun(err)
		}
		if line != "" {
			trailers = append(trailers, line)
		}
	}
	return trailers, nil
}

// departureRefusal judges the closure of a spec whose Design documents table
// carries departs rows, as the spec stood at the parent: each row must be one
// Thomas approved for this design text (AC-33), and the page it departs from
// must be in the spec's change set, which the closure review covers too
// (AC-32). A commit that closes no spec, or a spec with no departs row, owes
// nothing.
func departureRefusal(root string, options *Options, paths, removed []string, selection hunks.Selection,
	content prospectiveContent,
) error {
	stem, err := closedSpecStem(paths, removed, workingTreeSides(root, content.before, selection))
	if err != nil {
		return err
	}
	if stem == "" {
		return nil
	}
	name := "spec-" + stem
	closed, found := closedSpecPath(removed, stem)
	if !found {
		return couldNotRun(errors.New("the closed spec spec-" + stem + ".md is not among the removed paths"))
	}
	text, present, problem := committedText(root, content.before, closed)
	if problem != "" {
		return couldNotRun(errors.New("read the closed spec " + closed + ": " + problem))
	}
	// A guard, not a silent pass: a spec the parent does not hold was never
	// presented as a design there, so it carries no departs row to approve.
	if !present {
		return nil
	}
	departures := specdesign.Departures(text)
	if len(departures) == 0 {
		return nil
	}
	approval, approved, err := specdesign.Approved(root, name, text)
	if err != nil {
		return couldNotRun(err)
	}
	changed, err := closureChangeSet(root, stem, options, paths, removed, content)
	if err != nil {
		return err
	}
	var problems []string
	for _, departure := range departures {
		if !approved || !slices.Contains(approval.Departures, departure) {
			problems = append(problems, name+" departs from "+departure.Page+" without Thomas's approval\n"+
				"  why: a spec never departs from the design documentation without a discussion with Thomas, and his approval lists each departure\n"+
				"  fix: present the departure to Thomas; he runs, from his own terminal: ./le spec design approve spec "+closed+
				` words "<what he said>"`)
			continue
		}
		if !slices.Contains(changed, departure.Page) {
			problems = append(problems, "closure of "+name+": it departs from "+departure.Page+", and the change set does not correct it\n"+
				"  why: once Thomas agrees to a departure, the documentation it departs from is corrected in the same work\n"+
				"  fix: edit "+departure.Page+" so its "+departure.Section+" states the new decision, commit it under the spec (Spec-work: "+
				stem+"), then prepare the closure again")
		}
	}
	if len(problems) != 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

// closedSpecPath is the removed path of the spec stem names: the one
// closedSpecStem found it closes. found false means no removed path names it,
// which closedSpecStem never answers, and the caller refuses rather than read
// an empty path as a spec with no departure.
func closedSpecPath(removed []string, stem string) (closed string, found bool) {
	for _, candidate := range removed {
		if specpath.Stem(candidate) == stem && path.Base(candidate) == "spec-"+stem+".md" {
			return candidate, true
		}
	}
	return "", false
}

// closureChangeSet answers every path the closing spec's work touched: the
// commits attributed to it (Spec-work:), the earlier blocks of the script this
// closure appends to, and the closure itself, as checkVerificationGates
// gathers them for the review.
func closureChangeSet(root, stem string, options *Options, paths, removed []string, content prospectiveContent) ([]string, error) {
	population, err := specWorkPopulation(root, stem, content.head)
	if err != nil {
		return nil, couldNotRun(err)
	}
	changed := slices.Concat(paths, removed, population.Paths)
	if !options.Append {
		return changed, nil
	}
	earlier, err := scriptPaths(content.scriptPath, content.script)
	if err != nil {
		return nil, couldNotRun(err)
	}
	return append(changed, earlier...), nil
}

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
