// Design: docs/contributing/rfc-implementation-guide.md -- owner approval for RFC-tagged test changes
// Related: internal/le/rfc/goscope.go -- canonical tagged-unit boundaries.
package commit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/repo/objects"
	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"

	"github.com/ze-software/ze/internal/le/commit/hunks"
)

// RFCChange is one changed RFC-tagged test unit requiring owner approval, as
// rfc.ChangedUnits splits a file into units.
type RFCChange rfc.UnitChange

// RFCApprovals is what the gate learned from this session's approval file:
// the trailer line for each row the commit uses, and the exact file lines the
// generated script drops once the commit lands. Dropped holds the used rows
// and every row whose trailer git already carries (R-1: a row a landed
// commit used approves nothing further, even when the prune that should have
// removed it failed).
type RFCApprovals struct {
	Path     string   `json:"path"`
	Trailers []string `json:"trailers,omitempty"`
	Dropped  []string `json:"dropped,omitempty"`
	// Unapproved is each changed unit no live row names, as `<package>.<Test>`:
	// what `rfc-change-ok` records as one RFC-approval-owed trailer each.
	Unapproved []string `json:"unapproved,omitempty"`
	// Targets are immutable admitted operands for the generated per-attempt
	// publication guard, not another approval source.
	targets []resolvedRFCApprovalSelection
}

// RFCApprovalSelection names one judge row to publish. Revision is normalized
// once by ResolveRFCApprovalSelections before namespace creation.
type RFCApprovalSelection struct {
	Revision string `json:"revision"`
	Unit     string `json:"unit"`
}

// resolvedRFCApprovalSelection carries only the exact identity admitted by
// ResolveRFCApprovalSelections; the raw Options DTO cannot select a row.
type resolvedRFCApprovalSelection struct {
	revision string
	unit     string
}

// ResolveRFCApprovalSelections resolves each revision once and checks the exact
// historical unit before any namespace or generated artifact can be written.
// Callers MUST pass the returned immutable selections unchanged to SelectRFCApprovals.
func ResolveRFCApprovalSelections(root string, selections []RFCApprovalSelection) ([]resolvedRFCApprovalSelection, error) {
	resolved := make([]resolvedRFCApprovalSelection, 0, len(selections))
	seen := make(map[string]bool)
	for _, selection := range selections {
		if seen[selection.Unit] {
			return nil, errors.New("rfc-approval: select a unit only once per publication; multiple verdicts for the same unit cannot pair uniquely")
		}
		seen[selection.Unit] = true
		sha, err := commitOf(root, selection.Revision)
		if err != nil {
			return nil, fmt.Errorf("rfc-approval: resolve %q: %w", selection.Revision, err)
		}
		if _, err := rfc.ResolveApprovalUnit(root, sha, selection.Unit); err != nil {
			return nil, err
		}
		ancestor, err := isAncestor(root, sha, "HEAD")
		if err != nil {
			return nil, err
		}
		if !ancestor {
			return nil, errors.New("rfc-approval: the target must precede its publication on this branch: " + sha)
		}
		resolved = append(resolved, resolvedRFCApprovalSelection{revision: sha, unit: selection.Unit})
	}
	return resolved, nil
}

// SelectRFCApprovals selects exact live target/unit rows without changing the
// ledger. Only successful generated publication may prune Dropped.
// Callers MUST supply the unchanged result of ResolveRFCApprovalSelections.
func SelectRFCApprovals(root, session string, selections []resolvedRFCApprovalSelection) (RFCApprovals, error) {
	selected := RFCApprovals{Path: rfc.ApprovalPath(session)}
	if len(selections) == 0 {
		return selected, nil
	}
	rows, problems := testweakened.LoadApprovals(root, testweakened.RFCApprovals(session))
	if len(problems) != 0 {
		return selected, errors.New(strings.Join(problems, "\n"))
	}
	for _, selection := range selections {
		found := false
		for _, row := range rows {
			if row.Target != selection.revision || row.Name != selection.unit {
				continue
			}
			if row.Carrier != "" {
				return selected, errors.New("rfc-approval: exact approval already published by " + row.Carrier)
			}
			found = true
			selected.Trailers = append(selected.Trailers, row.Trailer,
				rfc.ApprovalForTrailerLine(row.Target, row.Name))
			selected.Dropped = append(selected.Dropped, row.Text)
			selected.targets = append(selected.targets, selection)
		}
		if !found {
			return selected, errors.New("rfc-approval: no pending judge row names exact target " +
				selection.revision + " and unit " + selection.unit)
		}
	}
	return selected, nil
}

// mergeSelectedRFCApprovals refuses a verdict that would have two meanings in
// one carrier, then adds the exact selected rows to normal current approvals.
func mergeSelectedRFCApprovals(current, selected RFCApprovals) (RFCApprovals, error) {
	for _, binding := range selected.Trailers {
		value, bound := strings.CutPrefix(binding, rfc.ApprovalForTrailer)
		if !bound {
			continue
		}
		_, unit, _ := strings.Cut(value, " ")
		for _, trailer := range current.Trailers {
			if strings.HasPrefix(trailer, rfc.ApprovalTrailer+unit+": ") {
				return current, errors.New("rfc-approval: a current-change verdict and a historical verdict for " +
					unit + " cannot pair uniquely in the same carrier; publish them separately")
			}
		}
	}
	for _, trailer := range selected.Trailers {
		if !slices.Contains(current.Trailers, trailer) {
			current.Trailers = append(current.Trailers, trailer)
		}
	}
	current.Dropped = append(current.Dropped, selected.Dropped...)
	current.targets = append(current.targets, selected.targets...)
	if current.Path == "" {
		current.Path = selected.Path
	}
	return current, nil
}

// selectedOrdinaryApprovalTrailers retains only verdicts this block spends as
// ordinary approvals. A bound RFC verdict is historical authority, while an
// ordinary weakening verdict stays independent of an RFC companion.
func selectedOrdinaryApprovalTrailers(approved RFCApprovals, weakeningTrailers []string) []string {
	if len(approved.Trailers) == 2*len(approved.targets) {
		return slices.Clone(weakeningTrailers)
	}
	var boundUnits map[string]bool
	if len(approved.targets) != 0 {
		boundUnits = make(map[string]bool, len(approved.targets))
		for _, target := range approved.targets {
			boundUnits[target.unit] = true
		}
	}
	ordinary := make([]string, 0, len(approved.Trailers)+len(weakeningTrailers))
	for _, trailer := range approved.Trailers {
		value, verdict := strings.CutPrefix(trailer, rfc.ApprovalTrailer)
		if !verdict {
			continue
		}
		unit, _, _ := strings.Cut(value, ": ")
		if !boundUnits[unit] {
			ordinary = append(ordinary, trailer)
		}
	}
	return append(ordinary, weakeningTrailers...)
}

// renderApprovalPublicationCheck checks the actual parent of every publication
// attempt. The caller MUST place it inside each CAS loop after reading
// _ze_parent; preparation alone cannot spend a row only once when two distinct
// messages were both prepared while it was live.
func renderApprovalPublicationCheck(targets []resolvedRFCApprovalSelection, ordinary []string, notPublished string) string {
	if len(targets) == 0 && len(ordinary) == 0 {
		return ""
	}
	lines := make([]string, 0, len(targets)*16+4)
	refusalLine := "echo " + shellQuote(notPublished) + " >&2"
	for _, target := range targets {
		binding := rfc.ApprovalForTrailerLine(target.revision, target.unit)
		// The native message producer also uses stripspace, so trailing message
		// whitespace cannot renew the same exact binding.
		lines = append(lines,
			`_ze_approval_ancestry=0`,
			`git merge-base --is-ancestor `+shellQuote(target.revision)+` "$_ze_parent" || _ze_approval_ancestry=$?`,
			renderRefusal(`"$_ze_approval_ancestry" -eq 1`, notPublished,
				`echo `+shellQuote("RFC approval target does not precede publication parent: "+target.revision+" "+target.unit+"; actual parent")+` "$_ze_parent" >&2`,
				`echo 'The selected full SHA is immutable. Record a fresh judge approval for the rewritten change, then prepare a new publication.' >&2`),
			renderRefusal(`"$_ze_approval_ancestry" -ne 0`, notPublished,
				`echo 'Cannot read the selected RFC approval target ancestry; no approval is spent.' >&2`),
			`_ze_approval_history=$(git log --no-show-signature --no-color --fixed-strings --grep=`+shellQuote(binding)+` --format=%B "$_ze_parent" -- | git stripspace) || {`,
			"  "+refusalLine,
			`  echo 'Cannot read previously published RFC approval bindings; no approval is spent.' >&2`,
			`  exit 1`,
			`}`,
			`case $'\n'"$_ze_approval_history"$'\n' in`,
			`  *$'\n'`+shellQuote(binding)+`$'\n'*)`,
			"    "+refusalLine,
			`    echo `+shellQuote("RFC approval binding already published: "+target.revision+" "+target.unit)+` >&2`,
			`    echo 'Prepare a fresh ./le commit create subject "<subject>" rfc-approval <revision> <unit> selecting an unspent row.' >&2`,
			`    exit 1 ;;`,
			`esac`,
		)
	}
	if len(ordinary) != 0 {
		lines = append(lines,
			`if ! `+ordinaryApprovalCheckCommand(ordinary)+`; then`,
			"  "+refusalLine,
			`  exit 1`,
			`fi`,
		)
	}
	return strings.Join(lines, "\n")
}

// unit names the change the way an owner approval and its trailers do,
// `<package>.<Test>`. It is the one place a change becomes a unit name, so the
// gate, the owed trailer and the push derivation cannot spell one unit two
// ways.
func (c *RFCChange) unit() string {
	return c.Package + "." + c.Name
}

// rfcChangeProblems judges the tagged tests a prospective commit changes
// against this session's approval file, the one `./le rfc approve` writes.
// A changed unit no live row names is a problem naming the command; a row
// the commit does not use stays in the file for a later commit.
func rfcChangeProblems(
	root, session string, prospective testweakened.Prospective, selection hunks.Selection, head string,
) ([]RFCChange, RFCApprovals, []string) {
	approvals := RFCApprovals{Path: rfc.ApprovalPath(session)}
	pairsByOld := make(map[string]testweakened.RenamePair)
	pairedNew := make(map[string]bool)
	for _, pair := range prospective.RenamePairs {
		pairsByOld[pair.OldPath] = pair
		pairedNew[pair.NewPath] = true
	}
	removed := make(map[string]bool)
	for _, path := range prospective.Removed {
		removed[path] = true
	}
	changes := make([]RFCChange, 0)
	seen := make(map[string]bool)
	for _, oldPath := range append(append([]string{}, prospective.Removed...), prospective.Paths...) {
		if seen[oldPath] || pairedNew[oldPath] || !rfc.IsTagCarrier(oldPath) {
			continue
		}
		seen[oldPath] = true
		newPath := oldPath
		if pair, paired := pairsByOld[oldPath]; paired {
			newPath = pair.NewPath
		} else if removed[oldPath] {
			newPath = ""
		}
		oldText, _, problem := committedText(root, head, oldPath)
		if problem != "" {
			return nil, approvals, []string{"RFC-tagged change gate could not run: " + problem}
		}
		if oldText == "" || !rfc.HasRequirementTag(oldText) {
			continue
		}
		newText := ""
		if newPath != "" {
			content, err := selection.Read(root, newPath)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, approvals, []string{"RFC-tagged change gate could not read " + newPath + ": " + err.Error()}
			}
			newText = string(content)
		}
		for _, unit := range rfc.ChangedUnits(newPathOrOld(newPath, oldPath), oldText, newText) {
			changes = append(changes, RFCChange(unit))
		}
	}
	// The one loader the edit hook and the weakening gate read this file
	// with: it refuses a malformed row and a bare unit, and marks each row a
	// landed commit already carries.
	rows, problems := testweakened.LoadApprovals(root, testweakened.RFCApprovals(session))
	if len(problems) != 0 {
		return changes, approvals, problems
	}
	claimed := make([]bool, len(changes))
	landedBy := make([]string, len(changes))
	for _, row := range rows {
		if row.Target != "" {
			continue
		}
		if row.Carrier != "" {
			// A commit already carries this trailer, so the row did its work
			// and the prune that should have removed it did not run. It
			// approves nothing further and leaves with this commit; the
			// refusal for a unit it names says which commit used it.
			approvals.Dropped = append(approvals.Dropped, row.Text)
			for index, change := range changes {
				if testweakened.RowMatches(row.Name, change.Package, change.Name) {
					landedBy[index] = row.Carrier
				}
			}
			continue
		}
		used := false
		for index, change := range changes {
			if testweakened.RowMatches(row.Name, change.Package, change.Name) {
				claimed[index] = true
				used = true
			}
		}
		if used {
			approvals.Trailers = append(approvals.Trailers, row.Trailer)
			approvals.Dropped = append(approvals.Dropped, row.Text)
		}
	}
	problems = make([]string, 0)
	for index, change := range changes {
		if claimed[index] {
			continue
		}
		why := "no approval names it"
		if landedBy[index] != "" {
			why = "the approval row naming it was already used by commit " + landedBy[index] + ", so it approves nothing further"
		}
		approvals.Unapproved = append(approvals.Unapproved, change.unit())
		problems = append(problems, fmt.Sprintf(
			"%s changes RFC-tagged test %s and %s.\n"+
				"  Only an independent ze-test-judge agent approves a change to a tagged test. On approval it records its verdict; then retry:\n"+
				"    %s", change.Path, change.Name, why, rfc.ApproveCommand(change.unit())))
	}
	return changes, approvals, problems
}

func newPathOrOld(newPath, oldPath string) string {
	if newPath != "" {
		return newPath
	}
	return oldPath
}

// committedText reads only immutable Git membership and blobs. Empty revision is
// the explicit root-parent baseline; an unavailable named revision is unknown,
// never evidence that the requested path did not exist.
func committedText(root, revision, path string) (string, bool, string) {
	if revision == "" {
		return "", false, ""
	}
	tree, err := objects.ResolveTree(context.Background(), root, revision)
	if err != nil {
		return "", false, err.Error()
	}
	content, present, err := objects.ReadPath(context.Background(), root, tree, path)
	if err != nil {
		return "", false, err.Error()
	}
	return string(content), present, ""
}
