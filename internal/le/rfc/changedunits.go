// Design: docs/contributing/rfc-implementation-guide.md -- owner approval for RFC-tagged test changes
// Related: goscope.go -- canonical tagged-unit boundaries.
// Related: approve.go -- the approval that names a unit.
package rfc

import (
	"path/filepath"
	"regexp"
	"strings"
)

var requirementTagPattern = regexp.MustCompile(`RFC requirement:\s*[A-Za-z0-9][A-Za-z0-9._/-]*(?:\s+(?:positive|negative))?`)

// UnitChange is one changed RFC-tagged test unit: the file, its package
// directory name, the unit name and the tags whose unit changed.
type UnitChange struct {
	Path    string   `json:"path"`
	Package string   `json:"package"`
	Name    string   `json:"name"`
	Tags    []string `json:"tags"`
}

// HasRequirementTag reports whether text carries an `RFC requirement:` tag.
func HasRequirementTag(text string) bool {
	return requirementTagPattern.MatchString(text)
}

// ChangedUnits splits the change of one RFC tag carrier into the tagged units
// it changed, the unit an owner approval names. A Go file's tagged test
// functions are each a unit; a file whose tag sits outside a function, or a
// carrier that is not Go, is one unit named after the file. The commit gate
// and the audit both call it, so the two cannot split one change two ways.
func ChangedUnits(path, oldText, newText string) []UnitChange {
	packageName := filepath.Base(filepath.Dir(path))
	if filepath.Dir(path) == "." {
		packageName = ""
	}
	if ScopeReader(path) != ScopeGo || tagFallsOutsideFunction(path, oldText) {
		if tags := ChangedTags(path, oldText, newText); len(tags) != 0 {
			return []UnitChange{{Path: path, Package: packageName, Name: fileStem(path), Tags: tags}}
		}
		return nil
	}
	newByName := make(map[string][]string)
	for _, unit := range FunctionUnits(newText) {
		newByName[unit.Name] = append(newByName[unit.Name], unit.Text)
	}
	changes := make([]UnitChange, 0)
	for _, unit := range FunctionUnits(oldText) {
		if !requirementTagPattern.MatchString(unit.Text) {
			continue
		}
		newUnit := ""
		if len(newByName[unit.Name]) == 1 {
			newUnit = newByName[unit.Name][0]
		}
		tags := ChangedTags(path, unit.Text, newUnit)
		if len(tags) == 0 {
			continue
		}
		name := unit.Name
		if name == "" {
			name = fileStem(path)
		}
		changes = append(changes, UnitChange{Path: path, Package: packageName, Name: name, Tags: tags})
	}
	return changes
}

func tagFallsOutsideFunction(path, content string) bool {
	for _, location := range requirementTagPattern.FindAllStringIndex(content, -1) {
		line := 1 + strings.Count(content[:location[0]], "\n")
		if UnitAt(path, content, line).Scope == ScopeFile {
			return true
		}
	}
	return false
}

func fileStem(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}
