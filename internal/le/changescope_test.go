// Design: docs/architecture/testing/verify-freshness-scope.md -- a partial run's verdict on a shared stage
// Related: verify/engine/stages.go -- Stage.Whole, the claim these tests refuse

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

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// changeScopePath is the import path of the package that defines the scope a
// verify run publishes and the calls that read it.
const changeScopePath = "github.com/ze-software/ze/internal/le/repo/changed"

// verifyDepsReaders is what every verify deps stage reaches: the one reader in
// its package, which only the unit-changed verb calls (planCommands' switch on
// the verb sends VerbUnitChanged, and nothing else, to planUnitChanged).
var verifyDepsReaders = []string{"verify/deps:planUnitChanged"}

// reviewedReach names, for a stage declared Whole whose package reads the
// change scope, every reader in that package, each reviewed as one the stage
// never calls. The key is the stage's name with a cut piece's
// `/part/<i>/of/<n>` dropped.
//
// It is the one place a Whole claim beside a reader is accepted, and it is
// pinned to the readers themselves: a reader added to, removed from or renamed
// in the package changes the list the scan answers, the entry no longer
// matches, and the claim is refused until somebody reviews it again. A second
// reader in the same directory is therefore never accepted unseen: the
// package-granular scan can refuse a true claim, and never accepts one it was
// not shown.
var reviewedReach = map[string][]string{
	// changedScope returns its arguments untouched unless the invocation names
	// the changed keyword, and `go lint run` does not.
	"go lint/run":                   {"go/lint:changedScope"},
	"verify deps/evidence-vet":      verifyDepsReaders,
	"verify deps/vulnerability":     verifyDepsReaders,
	"verify deps/unit-cached":       verifyDepsReaders,
	"verify deps/unit-race-changed": verifyDepsReaders,
	"verify deps/alloc":             verifyDepsReaders,
	"test functional/exabgp-test":   {"test/functional:selectSuites"},
}

// TestAWholeStageReachesNoChangeScopeReader refuses a Whole claim the scan
// cannot accept. A stage declared Whole is a full judgement in a partial
// population, so its partial green replaces a full red; a stage whose action
// reads the scope that claim lets a partial green erase a red over code the
// run never judged. The default needs no check: a stage that declares nothing
// is judged in part, which is the safe answer.
//
// Method: scan every shipped Go file under internal/le for a read of the
// scope, close each package over its internal/le imports, and refuse every
// Whole stage of every population whose package reaches a reader, unless
// reviewedReach holds exactly those readers for it. Every reviewed entry must
// still be needed.
func TestAWholeStageReachesNoChangeScopeReader(t *testing.T) {
	scan := scanChangeScope(t)
	if len(scan.readers) == 0 {
		t.Fatal("found no reader of the change scope under internal/le; the scan is broken")
	}
	used := map[string]bool{}
	for _, mode := range []string{verifyengine.Mode, verifyengine.FastMode, verifyengine.ChangedMode} {
		stages := verifyengine.StagesForMode(mode)
		for _, refusal := range wholeClaimRefusals(stages, scan.reach, reviewedReach) {
			t.Errorf("%s population: %s", mode, refusal)
		}
		for _, stage := range stages {
			if stage.Whole {
				used[reviewKey(stage.Identity.Name)] = len(scan.reach(stage.Identity.Command)) > 0
			}
		}
	}
	for key := range reviewedReach {
		if !used[key] {
			t.Errorf("reviewedReach holds %q, which is no Whole stage that reaches a reader; remove the entry", key)
		}
	}
}

// TestAWholeClaimOnAScopeReaderIsRefused proves the check refuses the claim it
// exists for: the gating suite and `go lint run changed` read the scope, so
// declaring either Whole is refused, while `go lint run`, whose reader is
// reviewed, is accepted.
//
// Method: run wholeClaimRefusals over three stages built Whole, with the real
// scan and the real reviewed entries.
func TestAWholeClaimOnAScopeReaderIsRefused(t *testing.T) {
	scan := scanChangeScope(t)
	claim := func(command string, args ...string) verifyengine.Stage {
		name := command + "/" + strings.Join(args, "/")
		return verifyengine.Stage{Identity: verifyengine.Identity{Name: name, Command: command, Args: args}, Whole: true}
	}
	for _, check := range []struct {
		stage   verifyengine.Stage
		refused bool
	}{
		{claim(verifyengine.FunctionalCommand, "gating"), true},
		{claim("go lint", "run", "changed"), true},
		{claim("go staticcheck", "check", "part", "1", "of", "2"), true},
		{claim("go lint", "run"), false},
	} {
		refusals := wholeClaimRefusals([]verifyengine.Stage{check.stage}, scan.reach, reviewedReach)
		if got := len(refusals) > 0; got != check.refused {
			t.Errorf("Whole %s: refused %v (%q), want %v", check.stage.Identity.Name, got, refusals, check.refused)
		}
	}
}

// wholeClaimRefusals answers one line for each stage declared Whole whose
// command's package reaches a reader of the change scope that reviewed does
// not hold, exactly, for it.
func wholeClaimRefusals(stages []verifyengine.Stage, reach func(command string) []string, reviewed map[string][]string) []string {
	var refusals []string
	for _, stage := range stages {
		if !stage.Whole {
			continue
		}
		reached := reach(stage.Identity.Command)
		if len(reached) == 0 {
			continue
		}
		key := reviewKey(stage.Identity.Name)
		want, found := reviewed[key]
		if !found {
			refusals = append(refusals, "stage "+strconv.Quote(stage.Identity.Name)+
				" is declared Whole, and its package can reach a reader of the change scope ("+strings.Join(reached, ", ")+
				"); drop its whole() in internal/le/verify/engine/stages.go, or review that it never calls them and add "+
				strconv.Quote(key)+" to reviewedReach")
			continue
		}
		if !slices.Equal(want, reached) {
			refusals = append(refusals, "stage "+strconv.Quote(stage.Identity.Name)+
				" is declared Whole and reviewed against the readers "+strings.Join(want, ", ")+
				", but it now reaches "+strings.Join(reached, ", ")+"; review the change and update reviewedReach")
		}
	}
	return refusals
}

// reviewKey is a stage's name without a cut piece's `/part/<i>/of/<n>`.
func reviewKey(name string) string {
	key, _, _ := strings.Cut(name, "/part/")
	return key
}

// changeScopeScan is what the scan found: the readers in each package
// directory under internal/le, relative to it.
type changeScopeScan struct {
	readers map[string][]string
}

// reach answers every reader in the package of command, sorted. It is
// package-granular and does not follow imports: a reader in a package the
// action calls into is not seen here, which is the bound of this check.
func (s changeScopeScan) reach(command string) []string {
	reached := slices.Clone(s.readers[directoryFor(command)])
	slices.Sort(reached)
	return slices.Compact(reached)
}

// scanChangeScope parses every shipped Go file under internal/le, the
// package's own directory, that imports the scope package, and names every
// read of the scope there by the function that holds it.
func scanChangeScope(t *testing.T) changeScopeScan {
	t.Helper()
	scan := changeScopeScan{readers: map[string][]string{}}
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
		file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		dir := filepath.Dir(path)
		for _, function := range readsInFile(file) {
			scan.readers[dir] = append(scan.readers[dir], filepath.ToSlash(dir)+":"+function)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/le: %v", err)
	}
	return scan
}

// readsInFile answers the function holding each read of the published scope
// in one file: a call of Packages, or a Get whose argument is ScopeTagsKey or
// ScopeFileKey, each through the file's own name for the scope package. A read
// outside every function is named for the file's package-level declarations.
func readsInFile(file *ast.File) []string {
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
	for _, declaration := range file.Decls {
		holder := "package level"
		if function, isFunction := declaration.(*ast.FuncDecl); isFunction {
			holder = function.Name.Name
		}
		ast.Inspect(declaration, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			if isScopeRead(call, local) {
				reads = append(reads, holder)
			}
			return true
		})
	}
	return reads
}

// isScopeRead reports whether call reads the published scope through the
// file's local name for the scope package.
func isScopeRead(call *ast.CallExpr, local string) bool {
	function, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}
	if isScopeName(function, local, "Packages") {
		return true
	}
	if function.Sel.Name != "Get" {
		return false
	}
	return slices.ContainsFunc(call.Args, func(argument ast.Expr) bool {
		key, isKey := argument.(*ast.SelectorExpr)
		if !isKey {
			return false
		}
		return isScopeName(key, local, "ScopeTagsKey") || isScopeName(key, local, "ScopeFileKey")
	})
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
