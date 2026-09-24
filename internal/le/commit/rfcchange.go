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
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

var rfcTagPattern = regexp.MustCompile(`RFC requirement:\s*[A-Za-z0-9][A-Za-z0-9._/-]*(?:\s+(?:positive|negative))?`)

// RFCChange is one changed RFC-tagged test unit requiring owner approval.
type RFCChange struct {
	Path    string   `json:"path"`
	Package string   `json:"package"`
	Name    string   `json:"name"`
	Tags    []string `json:"tags"`
}

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
}

// rfcChangeProblems judges the tagged tests a prospective commit changes
// against this session's approval file, the one `./le rfc approve` writes.
// A changed unit no live row names is a problem naming the command; a row
// the commit does not use stays in the file for a later commit.
func rfcChangeProblems(
	root, session string, prospective testweakened.Prospective,
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
		if oldText == "" || !rfcTagPattern.MatchString(oldText) {
			continue
		}
		newText := ""
		if newPath != "" {
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(newPath))) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, approvals, []string{"RFC-tagged change gate could not read " + newPath + ": " + err.Error()}
			}
			newText = string(content)
		}
		changes = append(changes, changedRFCUnits(newPathOrOld(newPath, oldPath), oldText, newText)...)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(approvals.Path))) //nolint:gosec // the path is this session's own tmp/ file under the checkout root
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return changes, approvals, []string{"cannot read " + approvals.Path + ": " + err.Error()}
	}
	rows := []testweakened.Row(nil)
	lines := []string(nil)
	if err == nil {
		var problems []string
		rows, problems = testweakened.ParseLedger(string(content), approvals.Path)
		if len(problems) != 0 {
			return changes, approvals, problems
		}
		lines = strings.Split(string(content), "\n")
	}
	claimed := make([]bool, len(changes))
	landedBy := make([]string, len(changes))
	for _, row := range rows {
		line := lines[row.Line-1]
		trailer := rfc.ApprovalTrailerLine(row.Name, row.Reason)
		carrier, landed, problem := trailerLanded(root, trailer)
		if problem != "" {
			return changes, approvals, []string{"RFC-tagged change gate could not run: " + problem}
		}
		if landed {
			// A commit already carries this trailer, so the row did its work
			// and the prune that should have removed it did not run. It
			// approves nothing further and leaves with this commit; the
			// refusal for a unit it names says which commit used it.
			approvals.Dropped = append(approvals.Dropped, line)
			for index, change := range changes {
				if testweakened.RowMatches(row.Name, change.Package, change.Name) {
					landedBy[index] = carrier
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
			approvals.Trailers = append(approvals.Trailers, trailer)
			approvals.Dropped = append(approvals.Dropped, line)
		}
	}
	problems := make([]string, 0)
	for index, change := range changes {
		if claimed[index] {
			continue
		}
		why := "no approval names it"
		if landedBy[index] != "" {
			why = "the approval row naming it was already used by commit " + landedBy[index] + ", so it approves nothing further"
		}
		problems = append(problems, fmt.Sprintf(
			"%s changes RFC-tagged test %s and %s.\n"+
				"  Only the OWNER approves a change to a tagged test. Once Thomas has answered, record his words and retry:\n"+
				"    %s", change.Path, change.Name, why, rfc.ApproveCommand(change.Package+"."+change.Name)))
	}
	return changes, approvals, problems
}

// trailerLanded reports whether a commit in this repository already carries
// one trailer line, which is how the gate knows a row was used, and names
// that commit as `<short hash> <subject>` for the refusal.
//
// `git log --grep` matches a substring, so a landed trailer whose reason
// EXTENDS this one would answer for it. It only narrows the candidates: the
// verdict is a whole message line equal to the trailer.
func trailerLanded(root, trailer string) (carrier string, landed bool, problem string) {
	command := exec.CommandContext(context.Background(), "git", "log", "--fixed-strings", "--grep="+trailer, "--format=%x1e%h %s%x1f%B") // #nosec G204 -- fixed Git query; the trailer is one argv operand.
	command.Dir = root
	var stdout bytes.Buffer
	var complaint bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok && strings.Contains(complaint.String(), "does not have any commits") {
			return "", false, ""
		}
		return "", false, "git log --grep failed: " + strings.TrimSpace(complaint.String())
	}
	for record := range strings.SplitSeq(stdout.String(), "\x1e") {
		name, body, found := strings.Cut(record, "\x1f")
		if !found {
			continue
		}
		for line := range strings.SplitSeq(body, "\n") {
			if strings.TrimSpace(line) == trailer {
				return strings.TrimSpace(name), true, ""
			}
		}
	}
	return "", false, ""
}

func changedRFCUnits(path, oldText, newText string) []RFCChange {
	packageName := filepath.Base(filepath.Dir(path))
	if filepath.Dir(path) == "." {
		packageName = ""
	}
	if rfc.ScopeReader(path) != rfc.ScopeGo || tagFallsOutsideFunction(path, oldText) {
		if tags := rfc.ChangedTags(path, oldText, newText); len(tags) != 0 {
			return []RFCChange{{Path: path, Package: packageName, Name: fileStem(path), Tags: tags}}
		}
		return nil
	}
	newByName := make(map[string][]string)
	for _, unit := range rfc.FunctionUnits(newText) {
		newByName[unit.Name] = append(newByName[unit.Name], unit.Text)
	}
	changes := make([]RFCChange, 0)
	for _, unit := range rfc.FunctionUnits(oldText) {
		if !rfcTagPattern.MatchString(unit.Text) {
			continue
		}
		newUnit := ""
		if len(newByName[unit.Name]) == 1 {
			newUnit = newByName[unit.Name][0]
		}
		tags := rfc.ChangedTags(path, unit.Text, newUnit)
		if len(tags) == 0 {
			continue
		}
		name := unit.Name
		if name == "" {
			name = fileStem(path)
		}
		changes = append(changes, RFCChange{Path: path, Package: packageName, Name: name, Tags: tags})
	}
	return changes
}

func tagFallsOutsideFunction(path, content string) bool {
	for _, location := range rfcTagPattern.FindAllStringIndex(content, -1) {
		line := 1 + strings.Count(content[:location[0]], "\n")
		if rfc.UnitAt(path, content, line).Scope == rfc.ScopeFile {
			return true
		}
	}
	return false
}

func newPathOrOld(newPath, oldPath string) string {
	if newPath != "" {
		return newPath
	}
	return oldPath
}

func fileStem(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
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
