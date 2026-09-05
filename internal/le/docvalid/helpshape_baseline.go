// Design: docs/contributing/documentation-testing.md -- the help shape gate
// Overview: helpshape.go -- the gate this baseline scopes one rule of
// Related: usage.go -- gitOutput and gitHeadBlobs, the same two readers
//
// helpshape_baseline.go answers one question: was this summary already declared
// at HEAD?
//
// Only the `missing-long-help` rule asks it. The corpus does not yet carry a
// long explanation everywhere -- 275 command nodes, 184 rpcs and 1,700 config
// nodes declare a summary alone -- so a rule armed over the whole tree would be
// red on the day it lands, and a red nobody can close is a red every session
// learns to ignore. The rule therefore judges what the commit under test ADDED
// or CHANGED, and the caps beside it stay absolute
// (plan/spec-command-help-and-description.md, AC-1).
//
// The baseline is the SUMMARY TEXT, not the path that declares it. No file is
// written and none is read: HEAD is the baseline, so there is nothing a session
// can append a path to in order to silence the gate, and a declaration that
// MOVES between modules is not read as new debt
// (plan/spec-shrink-only-baseline-cannot-see-a-relocation.md).
//
// A baseline that cannot be read accuses nobody. A checkout with no git, a root
// commit, and a `git grep` that answers nothing each leave the rule unjudged
// and say so in the report, rather than billing every declaration in the tree
// (ai/rules/principles.md).

package docvalid

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// helpBaseline holds the summaries HEAD already declared.
//
// read is the field that matters. An empty set and an unreadable HEAD are the
// same map, and they have opposite meanings to the rule that consults it: the
// first says every declaration in the tree is new, the second says nothing is
// known. A caller that could not tell them apart would refuse the whole corpus
// the first time git was unavailable.
type helpBaseline struct {
	summaries map[string]struct{}
	read      bool
}

// declaredAtHEAD answers whether HEAD already carried this summary.
//
// A baseline nobody could read answers true for everything, so the rule that
// asks judges nothing at all.
func (b helpBaseline) declaredAtHEAD(summary string) bool {
	if !b.read {
		return true
	}
	_, held := b.summaries[flattenSummary(summary)]
	return held
}

// scope names what the report prints about the rule this baseline scopes.
func (b helpBaseline) scope() string {
	if !b.read {
		return "not judged: HEAD could not be read"
	}
	return "declarations this working tree added or changed against HEAD"
}

// flattenSummary is the one form a summary is compared in: trimmed, with every
// run of whitespace collapsed to one space.
//
// It is also the form an operator reads. `entryDescription` and
// `enumKeyVocabulary` (internal/component/cli/completer.go) each join
// strings.Fields with a space before the text reaches the row, so a description
// rewrapped over different lines is the same summary to a reader and must be
// the same summary here.
func flattenSummary(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// The two corpora a summary is declared in, and the registrar whose presence
// marks a Go file as one of them.
//
// A summary reaches an operator from a YANG description statement or from a
// registry.Meta beside an offline handler, and nowhere else. The Go side is
// narrowed by the registrar name rather than read whole: the checkout holds
// some 6,000 tracked Go files and 28 of them register a local command.
const (
	yangCorpusGlob  = "*.yang"
	goCorpusGlob    = "*.go"
	localRegistrar  = "RegisterLocal"
	baselineHeadRev = "HEAD"
)

// headHelpBaseline answers every summary the checkout declared at HEAD.
//
// Any reader that fails leaves the baseline UNREAD rather than short. A short
// baseline is worse than none: it names a declaration as new because the file
// that holds it could not be opened, and the author is then told to write a
// long explanation for text they never touched.
func headHelpBaseline(root string) helpBaseline {
	if _, err := gitOutput(root, "rev-parse", "--verify", "--quiet", baselineHeadRev+"^{commit}"); err != nil {
		return helpBaseline{}
	}

	summaries := map[string]struct{}{}
	if !readYangBaseline(root, summaries) {
		return helpBaseline{}
	}
	if !readLocalBaseline(root, summaries) {
		return helpBaseline{}
	}
	return helpBaseline{summaries: summaries, read: true}
}

// readYangBaseline collects every description statement the tracked YANG
// modules declared at HEAD.
//
// The statements are read with goyang's own parser, so the owner of a
// description is resolved by the braces the author wrote rather than by the
// nearest preceding keyword, and a description written over several lines
// arrives in the same form the working-tree walk produces.
func readYangBaseline(root string, summaries map[string]struct{}) bool {
	paths, ok := trackedPaths(root, yangCorpusGlob)
	if !ok {
		return false
	}
	blobs, err := gitHeadBlobs(root, paths)
	if err != nil {
		return false
	}
	for path, text := range blobs {
		statements, err := gyang.Parse(text, path)
		if err != nil {
			// A module that does not parse at HEAD declared no summary this
			// reader can name, and it is not this gate's defect to report.
			continue
		}
		for _, statement := range statements {
			collectDescriptions(statement, summaries)
		}
	}
	return true
}

// collectDescriptions records every description argument at or under one
// statement.
//
// The recursion is over a YANG module in this repository, whose nesting depth
// is what an author wrote in the file. No external input reaches it.
func collectDescriptions(statement *gyang.Statement, summaries map[string]struct{}) {
	if statement.Keyword == "description" && statement.Argument != "" {
		summaries[flattenSummary(statement.Argument)] = struct{}{}
	}
	for _, sub := range statement.SubStatements() {
		collectDescriptions(sub, summaries)
	}
}

// readLocalBaseline collects every registry.Meta summary the Go files that
// register an offline local command declared at HEAD.
func readLocalBaseline(root string, summaries map[string]struct{}) bool {
	listed, err := gitOutput(root, "grep", "-l", "-z", "-e", localRegistrar, "--", goCorpusGlob)
	if err != nil {
		return false
	}
	paths := map[string]string{}
	for rel := range strings.SplitSeq(string(listed), "\x00") {
		if rel == "" {
			continue
		}
		paths[rel] = rel
	}
	blobs, err := gitHeadBlobs(root, paths)
	if err != nil {
		return false
	}
	for path, text := range blobs {
		file, err := parser.ParseFile(token.NewFileSet(), path, text, 0)
		if err != nil {
			// A file that does not parse at HEAD declared no summary this
			// reader can name.
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			composite, ok := node.(*ast.CompositeLit)
			if !ok || !isMetaType(composite.Type) {
				return true
			}
			if summary := metaDescriptionLiteral(composite); summary != "" {
				summaries[flattenSummary(summary)] = struct{}{}
			}
			return true
		})
	}
	return true
}

// metaDescriptionLiteral answers the Description a registry.Meta literal
// declares, or the empty string when it declares none this reader can see.
func metaDescriptionLiteral(composite *ast.CompositeLit) string {
	for _, element := range composite.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok || key.Name != "Description" {
			continue
		}
		value, ok := stringLiteral(field.Value)
		if !ok {
			continue
		}
		return value
	}
	return ""
}

// trackedPaths answers the tracked files matching one glob, keyed by path, in
// the shape gitHeadBlobs takes.
//
// A path under a testdata directory is a fixture rather than a module of this
// product. It is kept here all the same: the baseline answers what text HEAD
// held, and a wider answer can only make the gate more forgiving, never make it
// accuse a declaration nobody wrote.
func trackedPaths(root, glob string) (map[string]string, bool) {
	listed, err := gitOutput(root, "ls-files", "-z", "--", glob)
	if err != nil {
		return nil, false
	}
	paths := map[string]string{}
	for rel := range strings.SplitSeq(string(listed), "\x00") {
		if rel == "" {
			continue
		}
		paths[rel] = rel
	}
	if len(paths) == 0 {
		return nil, false
	}
	return paths, true
}
