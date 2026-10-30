// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet
// Related: ../../../cmd/ze/ze_le_personality_test.go -- TestNormalZeLinksNoInternalLe, which links each flavor
// Related: ../../le/rfc/check_evidence_linked.go -- goListLinked, which lists what each flavor links

// Package buildflavor reads the shipped build flavors of ze from the
// //go:build line of each cmd/ze/build_tag_<name>_test.go file, so a new
// flavor file joins every reader without an edit to any of them. It is the
// one parser both readers call: le's exclusion evidence ratchet and the
// cmd/ze personality test. It lives outside internal/le because the door
// keeps a helper both sides need out of it.
package buildflavor

import (
	"errors"
	"go/build/constraint"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Glob matches, relative to the repository root, the files whose //go:build
// line declares one shipped flavor each.
const Glob = "cmd/ze/build_tag_*_test.go"

// Flavor is one shipped flavor: its name, from the file name between
// "build_tag_" and "_test.go", and the tags its //go:build line requires true.
type Flavor struct {
	Name string
	Tags []string
}

// Shipped answers every flavor declared under root, in file name order. No
// flavor file, or one whose tags cannot be read (Tags), is an error naming
// the file: the set of flavors cannot be told from it.
func Shipped(root string) ([]Flavor, error) {
	files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(Glob)))
	if err != nil {
		return nil, errors.New("cannot list " + Glob + ": " + err.Error())
	}
	if len(files) == 0 {
		return nil, errors.New("no " + Glob + " file declares a shipped flavor")
	}
	slices.Sort(files)
	flavors := make([]Flavor, 0, len(files))
	for _, file := range files {
		rel := "cmd/ze/" + filepath.Base(file)
		body, err := os.ReadFile(file) //nolint:gosec // a flavor file of this checkout, found by glob
		if err != nil {
			return nil, errors.New("cannot read " + rel + ": " + err.Error())
		}
		tags, err := Tags(string(body))
		if err != nil {
			return nil, errors.New(rel + ": " + err.Error())
		}
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file), "build_tag_"), "_test.go")
		flavors = append(flavors, Flavor{Name: name, Tags: tags})
	}
	return flavors, nil
}

// Tags answers the tags the first //go:build line of body requires true. A
// body with no such line, a line that does not parse, a line that is not a
// conjunction of tags and negations, and tags that do not satisfy their own
// line (`a && !a`) are each an error: a flavor read from any of them would
// name tags the build does not set.
func Tags(body string) ([]string, error) {
	for line := range strings.SplitSeq(body, "\n") {
		if !constraint.IsGoBuild(line) {
			continue
		}
		expr, err := constraint.Parse(line)
		if err != nil {
			return nil, errors.New("its //go:build line does not parse (" + err.Error() + "); correct it")
		}
		var tags []string
		if !positiveTags(expr, &tags) {
			return nil, errors.New("its //go:build line `" + expr.String() +
				"` is not a conjunction of tags and negations, so its flavor's tags cannot be read; write it as one")
		}
		if !expr.Eval(func(tag string) bool { return slices.Contains(tags, tag) }) {
			return nil, errors.New("the tags " + strings.Join(tags, ",") + " do not satisfy its own //go:build line `" +
				expr.String() + "`; write a line its own tags satisfy")
		}
		return tags, nil
	}
	return nil, errors.New("carries no //go:build line, so its flavor is unknown; add the line naming the flavor's tags")
}

// positiveTags appends the tags expr requires true, false when expr is not a
// conjunction of tags and negations. Recursion is bounded by the one
// //go:build line it reads.
func positiveTags(expr constraint.Expr, tags *[]string) bool {
	switch e := expr.(type) {
	case *constraint.AndExpr:
		return positiveTags(e.X, tags) && positiveTags(e.Y, tags)
	case *constraint.TagExpr:
		*tags = append(*tags, e.Tag)
		return true
	case *constraint.NotExpr:
		return true
	default:
		return false
	}
}
