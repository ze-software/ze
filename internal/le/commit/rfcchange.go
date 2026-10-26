// Design: docs/contributing/rfc-implementation-guide.md -- owner approval for RFC-tagged test changes
// Related: internal/le/rfc/goscope.go -- canonical tagged-unit boundaries.
package commit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	root, session string, prospective testweakened.Prospective, selection hunks.Selection,
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
		oldText, _, problem := committedText(root, "HEAD", oldPath)
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

// committedText answers a path's bytes at a revision, whether the revision
// CARRIES the path, and the reason the read failed.
//
// present is what tells an absent path from an empty file, and a caller that
// treats the two alike answers a question about a file the commit never held.
// A revision that does not resolve is not present and is not a problem: the
// gate below asks about HEAD in a checkout that may hold no commit yet.
func committedText(root, revision, path string) (text string, present bool, problem string) {
	resolve := exec.CommandContext(context.Background(), "git", "rev-parse", "--verify", "-q", revision+"^{commit}") // #nosec G204 -- fixed Git query; revision is an argv operand.
	resolve.Dir = root
	if err := resolve.Run(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", false, ""
		}
		return "", false, err.Error()
	}
	list := exec.CommandContext(context.Background(), "git", "ls-tree", "--name-only", revision, "--", path) // #nosec G204 -- fixed Git query; revision and path are argv.
	list.Dir = root
	var names bytes.Buffer
	var complaint bytes.Buffer
	list.Stdout = &names
	list.Stderr = &complaint
	if err := list.Run(); err != nil {
		return "", false, "git ls-tree " + revision + " -- " + path + " failed: " +
			strings.TrimSpace(complaint.String())
	}
	if strings.TrimSpace(names.String()) == "" {
		return "", false, ""
	}
	object := revision + ":" + path
	command := exec.CommandContext(context.Background(), "git", "show", object) // #nosec G204 -- fixed Git query; object is data.
	command.Dir = root
	var stdout bytes.Buffer
	complaint.Reset()
	command.Stdout = &stdout
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		return "", false, "git show " + object + " failed: " + strings.TrimSpace(complaint.String())
	}
	return stdout.String(), true, ""
}
