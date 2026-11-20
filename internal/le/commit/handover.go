// Design: docs/contributing/committing.md -- the gates ./le commit create runs
// Related: prepare.go -- Create, which calls this gate after the dependency gate
// Related: review.go -- workingText, the side this gate reads a carried spec from
// Related: ../spec/status/setstate.go -- DeriveSets, the one answer of each set's state

package commit

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/hunks"
	"github.com/ze-software/ze/internal/le/repo/objects"
	lesession "github.com/ze-software/ze/internal/le/session"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// handoverReaders are the gate's two inputs that the commit does not carry:
// the record of closed specs and the claims with the reaper's verdict on each
// holder.
type handoverReaders struct {
	closed func(context.Context, string) (specstatus.ClosedSpecs, error)
	claims func(string) ([]lesession.Claim, error)
}

// liveHandoverReaders are the readers Create's gate uses. Tests replace them,
// so no test reads the host's process table or the shared record.
var liveHandoverReaders = handoverReaders{
	closed: specstatus.LoadClosedSpecs,
	claims: func(root string) ([]lesession.Claim, error) { return lesession.ListClaims(root, "") },
}

// handoverRefusal refuses a commit that removes a handover of work still open:
// one tied to a spec set that DeriveSets answers live or stalled (started,
// with an open child left) or cannot tell, or named for the whole stem of an
// open spec, a set member included. The ties are specstatus's:
// SetView.Handovers for a set, and HandoverNamesSpec for a spec. The population is the tree the commit
// produces, so a spec the same commit removes is closed, and a commit that
// closes a set's last open child may remove its handover with it. An idea set,
// a set whose children all closed and a spec no file holds any more name
// nothing open, so their handovers go freely. A set whose state cannot be told
// is refused with the reason and what settles it: a removal judged on a guess
// is the loss this gate exists to stop.
//
// The record of closed specs is extended to HEAD here, and the claims are read
// with the reaper's verdict, only when the commit removes a handover. A record
// that cannot be extended is not computed, never the record as the file holds
// it: that one may predate a closure, and a child closed after it would be in
// neither the open nor the closed children.
func handoverRefusal(root string, paths, removed []string, selection hunks.Selection, head string) error {
	var handovers []string
	for _, candidate := range removed {
		if specstatus.IsHandover(candidate) {
			handovers = append(handovers, candidate)
		}
	}
	if len(handovers) == 0 {
		return nil
	}
	specs, err := committedSpecs(root, paths, removed, selection, head)
	if err != nil {
		return couldNotRun(errors.New("judge the removal of " + strings.Join(handovers, ", ") + ": " + err.Error()))
	}
	closed, err := liveHandoverReaders.closed(context.Background(), root)
	if err != nil {
		closed = specstatus.ClosedSpecsNotComputed(err.Error())
	}
	var claims specstatus.SetClaims
	if listed, err := liveHandoverReaders.claims(root); err != nil {
		claims = specstatus.ClaimsUnread("the claims on spec files could not be read: "+err.Error(),
			"repair what the error names, then run the same ./le commit create line again")
	} else {
		claims = specstatus.ClaimsRead(listed)
	}
	report := specstatus.DeriveSets(specs, closed, claims, handovers)
	var refusals []string
	for _, handover := range handovers {
		if text := handoverProblem(handover, specs, report.Sets); text != "" {
			refusals = append(refusals, text)
		}
	}
	if len(refusals) == 0 {
		return nil
	}
	return errors.New(strings.Join(refusals, "\n\n"))
}

// committedSpecs answers every spec of the tree the commit produces: a spec
// the commit carries is read as it carries it, any other as the parent holds
// it, and a spec the commit removes is left out, since the script deletes a
// removed spec's working copy only after the commit lands. The population is
// the working tree's spec files, so an untracked spec another session is
// writing is not one, and neither is a parent's spec whose working copy is gone.
// The parent's tree is resolved once, never once for each spec.
func committedSpecs(root string, paths, removed []string, selection hunks.Selection, head string) ([]specstatus.Spec, error) {
	population, err := specpath.All(root)
	if err != nil {
		return nil, err
	}
	tree := ""
	if head != "" {
		tree, err = objects.ResolveTree(context.Background(), root, head)
		if err != nil {
			return nil, err
		}
	}
	specs := make([]specstatus.Spec, 0, len(population))
	for _, rel := range population {
		if slices.Contains(removed, rel) {
			continue
		}
		text, present, err := committedSpecText(root, tree, rel, slices.Contains(paths, rel), selection)
		if err != nil {
			return nil, err
		}
		if !present {
			continue
		}
		spec, err := specstatus.Parse([]byte(text), rel, func(string) {})
		if err != nil {
			return nil, errors.New("read " + rel + ": " + err.Error())
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// committedSpecText answers rel as the commit produces it: the working copy
// when the commit carries it, otherwise the parent's tree ("" when there is
// no parent, which holds nothing).
func committedSpecText(root, tree, rel string, carried bool, selection hunks.Selection) (string, bool, error) {
	if carried {
		text, present, problem := workingText(root, rel, selection)
		if problem != "" {
			return "", false, errors.New(problem)
		}
		return text, present, nil
	}
	if tree == "" {
		return "", false, nil
	}
	content, present, err := objects.ReadPath(context.Background(), root, tree, rel)
	if err != nil {
		return "", false, err
	}
	return string(content), present, nil
}

// handoverProblem answers the refusal text for removing handover, or "" when
// it is tied to nothing still open. A handover is tied to a set by the set's
// prefix and judged by the set's state, and tied to any open spec, set member
// or not, by that spec's whole stem (AC-18 protects "the stem of an open
// spec"): a handover named for an open child of an idea set is still refused.
// The one spec the stem rule skips is a number-less umbrella, whose stem is
// its set's prefix: there the two ties are the same name, and the set's state
// decides, so an idea set's own prefix-only handover passes.
func handoverProblem(handover string, specs []specstatus.Spec, sets []specstatus.SetView) string {
	var open textbuf.Buffer
	var prefixUmbrellas, listed []string
	cannotTell := false
	for index := range sets {
		set := &sets[index]
		if set.Umbrella != "" && specpath.Stem(set.Umbrella) == set.Prefix {
			prefixUmbrellas = append(prefixUmbrellas, set.Umbrella)
		}
		if !handoverKept(set.State) {
			continue
		}
		if !slices.Contains(set.Handovers, handover) {
			continue
		}
		for _, child := range set.Open {
			listed = append(listed, child.Path)
		}
		open.Str("  -- set ").Str(set.Prefix).Str(" (").Str(set.State.String()).Str(")")
		if set.State == specstatus.SetCannotTell {
			cannotTell = true
			open.Str(": ").Str(set.Why)
		}
		if len(set.Open) == 0 {
			open.Byte('\n')
		} else {
			open.Str(", open children:\n")
		}
		for _, child := range set.Open {
			open.Str("       ").Int(int64(child.Number)).Byte(' ').Str(child.Path).Str(" (").Str(child.Status).Str(")\n")
		}
		if set.ClosedRead {
			if len(set.Closed) != 0 {
				open.Str("     closed children: ").Str(strings.Join(set.Closed, ", ")).Byte('\n')
			}
		}
		if set.Fix != "" {
			open.Str("     to tell its state: ").Str(set.Fix).Byte('\n')
		}
	}
	for index := range specs {
		spec := &specs[index]
		if slices.Contains(prefixUmbrellas, spec.Path) {
			continue
		}
		// An open child the refused set already lists is not named twice.
		if slices.Contains(listed, spec.Path) {
			continue
		}
		if !specstatus.HandoverNamesSpec(handover, spec.Name) {
			continue
		}
		open.Str("  -- spec ").Str(spec.Path).Str(" (").Str(spec.Status).Str(")\n")
	}
	if open.Len() == 0 {
		return ""
	}
	var text textbuf.Buffer
	text.Str("This commit removes ").Str(handover).Str(", the handover of work that is still open:\n")
	text.Str(open.String())
	text.Str("A handover is the committed record of where open work stands. Removed now, the next session cannot find where to resume it.\n")
	if cannotTell {
		text.Str("A set whose state cannot be told keeps its handover until it can: settle it as its line says, then run the same ./le commit create line again.\n")
	}
	text.Str("Keep it: drop this keyword from the ./le commit create line, and remove the file in the commit that closes the last open spec it names:\n")
	text.Str("  remove ").Str(handover)
	return text.String()
}

// handoverKept reports whether a set in state keeps its handover: started with
// an open child left (live, stalled), or a state nobody could judge. An idea
// and a set whose children all closed let it go.
func handoverKept(state specstatus.SetState) bool {
	switch state {
	case specstatus.SetLive, specstatus.SetStalled, specstatus.SetCannotTell:
		return true
	case specstatus.SetIdea, specstatus.SetChildrenClosed:
		return false
	case specstatus.SetStateUnspecified:
		panic("BUG: DeriveSets answered a set with no state")
	}
	panic("BUG: DeriveSets answered an unknown set state " + state.String())
}
