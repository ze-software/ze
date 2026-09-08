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
	"github.com/ze-software/ze/internal/le/testweakened"
)

var rfcTagPattern = regexp.MustCompile(`RFC requirement:\s*[A-Za-z0-9][A-Za-z0-9._/-]*(?:\s+(?:positive|negative))?`)

// RFCChange is one changed RFC-tagged test unit requiring owner approval.
type RFCChange struct {
	Path    string   `json:"path"`
	Package string   `json:"package"`
	Name    string   `json:"name"`
	Tags    []string `json:"tags"`
}

// rfcChangeProblems judges the tagged tests a prospective commit changes
// against this session's own shard of the RFC-changed ledger. It answers the
// rows the commit uses, so the caller can drop the ones an earlier commit of
// this session already landed.
func rfcChangeProblems(
	root, shard string, prospective testweakened.Prospective, carriesLedger bool,
) ([]RFCChange, []testweakened.Row, []string) {
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
			return nil, nil, []string{"RFC-tagged change gate could not run: " + problem}
		}
		if oldText == "" || !rfcTagPattern.MatchString(oldText) {
			continue
		}
		newText := ""
		if newPath != "" {
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(newPath))) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, nil, []string{"RFC-tagged change gate could not read " + newPath + ": " + err.Error()}
			}
			newText = string(content)
		}
		changes = append(changes, changedRFCUnits(newPathOrOld(newPath, oldPath), oldText, newText)...)
	}
	if len(changes) == 0 {
		return nil, nil, nil
	}
	if !carriesLedger {
		return changes, nil, []string{fmt.Sprintf(
			"this commit changes %d RFC-tagged test(s) and does not carry %s.\n"+
				"  The row records what the OWNER approved, and it is the only place a "+
				"later reader finds that approval beside the change it authorizes.\n"+
				"  Name the file too:\n    file %s", len(changes), shard, shard)}
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(shard))) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	if err != nil {
		return changes, nil, []string{"cannot read " + shard + ": " + err.Error()}
	}
	rows, problems := testweakened.ParseLedger(string(content), shard)
	if len(problems) != 0 {
		return changes, nil, problems
	}
	landed := testweakened.LandedRows(root, shard)
	claimed := make([]bool, len(changes))
	keep := make([]testweakened.Row, 0, len(rows))
	for _, row := range rows {
		hits := 0
		for index, change := range changes {
			if testweakened.RowMatches(row.Name, change.Package, change.Name) {
				claimed[index] = true
				hits++
			}
		}
		if hits != 0 {
			keep = append(keep, row)
			continue
		}
		// An approval this session already committed is not a leftover to
		// clear: git holds it beside the change it authorized, it approves
		// nothing further, and the prune drops it from the shard.
		if landed[row.Key()] {
			continue
		}
		problems = append(problems, fmt.Sprintf("%s:%d names %s, which this commit does not change", shard, row.Line, row.Name))
	}
	for index, change := range changes {
		if !claimed[index] {
			problems = append(problems, fmt.Sprintf("%s changes RFC-tagged test %s and %s has no owner-approval row", change.Path, change.Name, shard))
		}
	}
	return changes, keep, problems
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
