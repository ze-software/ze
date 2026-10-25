// Design: docs/architecture/testing/verify-freshness-scope.md -- a scoped stage's verdict
// Related: verify/engine/stages.go -- Stage.Scoped, the copy these tests compare

package le

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// changeScopePath is the import path of the package that defines the scope a
// verify run publishes and the calls that read it.
const changeScopePath = "github.com/ze-software/ze/internal/le/repo/changed"

// TestStageScopedIsTheActionsDeclaration proves Stage.Scoped, the stage-side
// copy, says what the action the stage runs declares where its verb registers
// (leaction.Action.ReadsChangeScope, or the same field on a keyword the stage
// names). A stage whose action reads the published scope and is not marked
// would let a partial green replace a full red it never re-judged.
//
// Method: resolve every stage of every population through the registered
// action table and compare. A stage whose area registers no action table, or
// whose verb its table lacks, cannot declare anything, so its area's directory
// must hold no reader of the scope (changeScopeReaders) and the stage must not
// be marked.
func TestStageScopedIsTheActionsDeclaration(t *testing.T) {
	readers := changeScopeReaders(t)
	for _, mode := range []string{verifyengine.Mode, verifyengine.FastMode, verifyengine.ChangedMode} {
		for _, stage := range verifyengine.StagesForMode(mode) {
			reads, resolved := stageReadsChangeScope(stage.Identity)
			if !resolved {
				if where, found := readers[directoryFor(stage.Identity.Command)]; found {
					t.Errorf("%s stage %q runs an action no table declares, and its area reads the change scope at %s",
						mode, stage.Identity.Name, where[0])
				}
				if stage.Scoped {
					t.Errorf("%s stage %q is marked Scoped, but no registered action declares that it reads the change scope",
						mode, stage.Identity.Name)
				}
				continue
			}
			if reads != stage.Scoped {
				t.Errorf("%s stage %q: Scoped is %v, but its action declares ReadsChangeScope %v; make the "+
					"scoped() mark in internal/le/verify/engine/stages.go and the action's declaration agree",
					mode, stage.Identity.Name, stage.Scoped, reads)
			}
		}
	}
}

// TestEveryChangeScopeReaderDeclaresIt ties the declaration to the code that
// reads the scope. Every directory whose shipped Go calls repochanged.Packages,
// or reads repochanged.ScopeTagsKey or ScopeFileKey through a Get, is an
// area's package whose table declares a scope-reading action, and every area
// declaring one holds such a reader.
//
// Method: parse every non-test Go file under internal/le that imports the
// scope package, collect the directories holding a read, and compare them with
// the directories of the areas whose action rows declare ReadsChangeScope.
func TestEveryChangeScopeReaderDeclaresIt(t *testing.T) {
	readers := changeScopeReaders(t)
	if len(readers) == 0 {
		t.Fatal("found no reader of the change scope under internal/le; the scan is broken")
	}
	declaring := map[string]string{}
	for _, tool := range commandsAtStart {
		table, declared := leroot.ActionsOf(tool.Name)
		if !declared {
			continue
		}
		if slices.ContainsFunc(table.Actions, rowReadsChangeScope) {
			declaring[directoryFor(tool.Name)] = tool.Name
		}
	}
	for dir, where := range readers {
		if _, found := declaring[dir]; !found {
			t.Errorf("%s reads the change scope, and no area at %s declares an action with ReadsChangeScope", where[0], dir)
		}
	}
	for dir, area := range declaring {
		if _, found := readers[dir]; !found {
			t.Errorf("area %q declares ReadsChangeScope, and no Go file in %s reads the change scope", area, dir)
		}
	}
}

// stageReadsChangeScope answers what the action a stage runs declares, and
// whether that action resolved at all. The first argument is the verb; a later
// argument that is a keyword declared ReadsChangeScope counts too.
func stageReadsChangeScope(identity verifyengine.Identity) (reads, resolved bool) {
	table, declared := leroot.ActionsOf(identity.Command)
	if !declared {
		return false, false
	}
	verb := ""
	if len(identity.Args) > 0 {
		verb = identity.Args[0]
	}
	index := slices.IndexFunc(table.Actions, func(row leaction.Row) bool { return row.Verb == verb })
	if index < 0 {
		return false, false
	}
	row := table.Actions[index]
	if row.ReadsChangeScope {
		return true, true
	}
	for _, parameter := range row.Parameters {
		if parameter.ReadsChangeScope && slices.Contains(identity.Args[1:], parameter.Keyword) {
			return true, true
		}
	}
	return false, true
}

// rowReadsChangeScope reports whether an action row, or any of its keywords,
// declares that it reads the change scope.
func rowReadsChangeScope(row leaction.Row) bool {
	if row.ReadsChangeScope {
		return true
	}
	return slices.ContainsFunc(row.Parameters, func(parameter leaction.Parameter) bool {
		return parameter.ReadsChangeScope
	})
}

// changeScopeReaders answers every directory under internal/le, relative to
// it, whose shipped Go reads the published change scope, with the position of
// each read.
func changeScopeReaders(t *testing.T) map[string][]string {
	t.Helper()
	readers := map[string][]string{}
	needle := []byte(strconv.Quote(changeScopePath))
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path) //nolint:gosec // a source file under the package's own tree
		if err != nil {
			return err
		}
		if !bytes.Contains(source, needle) {
			return nil
		}
		for _, where := range readsInFile(t, path, source) {
			readers[filepath.Dir(path)] = append(readers[filepath.Dir(path)], where)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/le: %v", err)
	}
	return readers
}

// readsInFile answers the position of every read of the published scope in
// one file: a call of Packages, or a Get whose argument is ScopeTagsKey or
// ScopeFileKey, each through the file's own name for the scope package.
func readsInFile(t *testing.T, path string, source []byte) []string {
	t.Helper()
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, path, source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	local := ""
	for _, spec := range file.Imports {
		if spec.Path.Value != strconv.Quote(changeScopePath) {
			continue
		}
		local = "changed"
		if spec.Name != nil {
			local = spec.Name.Name
		}
	}
	var reads []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		function, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		if isScopeName(function, local, "Packages") {
			reads = append(reads, files.Position(call.Pos()).String())
			return true
		}
		if function.Sel.Name != "Get" {
			return true
		}
		for _, argument := range call.Args {
			key, isKey := argument.(*ast.SelectorExpr)
			if !isKey {
				continue
			}
			if isScopeName(key, local, "ScopeTagsKey") || isScopeName(key, local, "ScopeFileKey") {
				reads = append(reads, files.Position(call.Pos()).String())
			}
		}
		return true
	})
	return reads
}

// isScopeName reports whether selector names the given member of the scope
// package through the file's local name for it.
func isScopeName(selector *ast.SelectorExpr, local, member string) bool {
	qualifier, isIdent := selector.X.(*ast.Ident)
	if !isIdent {
		return false
	}
	return qualifier.Name == local && selector.Sel.Name == member
}
