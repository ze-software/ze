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
	"github.com/ze-software/ze/internal/le/weakened"
)

const rfcChangedPath = "test/rfc-changed.md"

var rfcTagPattern = regexp.MustCompile(`RFC requirement:\s*[A-Za-z0-9][A-Za-z0-9._/-]*(?:\s+(?:positive|negative))?`)

// RFCChange is one changed RFC-tagged test unit requiring owner approval.
type RFCChange struct {
	Path    string   `json:"path"`
	Package string   `json:"package"`
	Name    string   `json:"name"`
	Tags    []string `json:"tags"`
}

func rfcChangeProblems(root string, prospective weakened.Prospective, carriesLedger bool) ([]RFCChange, []string) {
	pairsByOld := make(map[string]weakened.RenamePair)
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
		oldText, problem := committedText(root, "HEAD", oldPath)
		if problem != "" {
			return nil, []string{"RFC-tagged change gate could not run: " + problem}
		}
		if oldText == "" || !rfcTagPattern.MatchString(oldText) {
			continue
		}
		newText := ""
		if newPath != "" {
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(newPath)))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, []string{"RFC-tagged change gate could not read " + newPath + ": " + err.Error()}
			}
			newText = string(content)
		}
		changes = append(changes, changedRFCUnits(newPathOrOld(newPath, oldPath), oldText, newText)...)
	}
	if len(changes) == 0 {
		return nil, nil
	}
	if !carriesLedger {
		return changes, []string{fmt.Sprintf("this commit changes %d RFC-tagged test(s) and does not carry %s", len(changes), rfcChangedPath)}
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rfcChangedPath)))
	if err != nil {
		return changes, []string{"cannot read " + rfcChangedPath + ": " + err.Error()}
	}
	rows, problems := weakened.ParseLedger(string(content), rfcChangedPath)
	if len(problems) != 0 {
		return changes, problems
	}
	claimed := make([]bool, len(changes))
	for _, row := range rows {
		hits := 0
		for index, change := range changes {
			if weakened.RowMatches(row.Name, change.Package, change.Name) {
				claimed[index] = true
				hits++
			}
		}
		if hits == 0 {
			problems = append(problems, fmt.Sprintf("%s:%d names %s, which this commit does not change", rfcChangedPath, row.Line, row.Name))
		}
	}
	for index, change := range changes {
		if !claimed[index] {
			problems = append(problems, fmt.Sprintf("%s changes RFC-tagged test %s and %s has no owner-approval row", change.Path, change.Name, rfcChangedPath))
		}
	}
	return changes, problems
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

func committedText(root, revision, path string) (string, string) {
	resolve := exec.CommandContext(context.Background(), "git", "rev-parse", "--verify", "-q", revision+"^{commit}") // #nosec G204 -- fixed Git query; revision is an argv operand.
	resolve.Dir = root
	if err := resolve.Run(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", ""
		}
		return "", err.Error()
	}
	list := exec.CommandContext(context.Background(), "git", "ls-tree", "--name-only", revision, "--", path) // #nosec G204 -- fixed Git query; revision and path are argv.
	list.Dir = root
	var names bytes.Buffer
	var complaint bytes.Buffer
	list.Stdout = &names
	list.Stderr = &complaint
	if err := list.Run(); err != nil {
		return "", "git ls-tree " + revision + " -- " + path + " failed: " +
			strings.TrimSpace(complaint.String())
	}
	if strings.TrimSpace(names.String()) == "" {
		return "", ""
	}
	object := revision + ":" + path
	command := exec.CommandContext(context.Background(), "git", "show", object) // #nosec G204 -- fixed Git query; object is data.
	command.Dir = root
	var stdout bytes.Buffer
	complaint.Reset()
	command.Stdout = &stdout
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		return "", "git show " + object + " failed: " + strings.TrimSpace(complaint.String())
	}
	return stdout.String(), ""
}
