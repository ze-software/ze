package yang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStagesRunEveryStageOfResolve: every stage resolveDefault and Resolve
// run is run by the staged build in loader_stages.go, so the benchmark of each
// stage (BenchmarkSchemaBuildStages) cannot drift from the build it measures.
//
// Method: parse loader.go and collect the stages of resolveDefault and
// Resolve: every call whose selector chain starts at their Loader `l` (`l.X()`
// and `l.modules.X()`), and every package function called by its bare name.
// Resolve is the transition being staged, so resolveDefault's call of it is
// not a stage. Then require loader_stages.go to call each stage by name. The
// stages known today are required to be found, so a collector that finds
// nothing cannot pass.
//
// What it does not catch: a stage added inside a function those two call
// (process, checkStructure and the rest) is run by the staged call of that
// function, so it is measured, but on the line of the function that holds it,
// never on a line of its own. A statement that is not a call (Resolve's
// `l.modules.resolved = true`), a call through a function value, and a call
// that loader_stages.go makes under the same name on another receiver are not
// told apart from what Resolve does.
//
// VALIDATES: AC-19, the staged benchmark runs the stage functions Resolve runs.
// PREVENTS: a stage added to Resolve or resolveDefault that no staged
// benchmark line measures.
func TestStagesRunEveryStageOfResolve(t *testing.T) {
	fset := token.NewFileSet()
	loaderFile, err := parser.ParseFile(fset, "loader.go", nil, parser.SkipObjectResolution)
	require.NoError(t, err)
	stagesFile, err := parser.ParseFile(fset, "loader_stages.go", nil, parser.SkipObjectResolution)
	require.NoError(t, err)

	var stages []string
	for _, name := range []string{"resolveDefault", "Resolve"} {
		fn := loaderFunc(loaderFile, name)
		require.NotNil(t, fn, "loader.go declares no %s", name)
		stages = append(stages, buildStages(fn)...)
	}
	for _, known := range []string{"NewLoader", "LoadEmbedded", "LoadRegistered", "refuseLoad", "process",
		"checkExtensions", "checkPatterns", "checkStructure", "checkReferences"} {
		require.Contains(t, stages, known, "the stage collector missed a stage loader.go runs today")
	}

	staged := calledNames(stagesFile)
	for _, stage := range stages {
		if stage == "Resolve" {
			continue
		}
		if !staged[stage] {
			t.Errorf("loader.go runs stage %s in resolveDefault or Resolve, and loader_stages.go never calls it:"+
				" add a staged entry that runs it, and its line to BenchmarkSchemaBuildStages", stage)
		}
	}
}

// TestStageProcessRunsOnceAsResolve: StageProcess runs goyang's Process once,
// and leaves its Loader refusing every load, Resolve and StageProcess, as
// Resolve does.
//
// Method: stage the embedded modules, then ask again for each of the three.
//
// VALIDATES: AC-19, the process stage leaves the loader in the state Resolve
// leaves it in.
// PREVENTS: a staged Loader that a later load or Resolve reuses, so a set no
// check ever read reaches a reader.
func TestStageProcessRunsOnceAsResolve(t *testing.T) {
	l := NewLoader()
	require.NoError(t, l.LoadEmbedded())
	require.NoError(t, l.StageProcess())

	require.ErrorIs(t, l.StageProcess(), ErrLoaderResolved)
	require.ErrorIs(t, l.LoadEmbedded(), ErrLoaderResolved)
	resolved, err := l.Resolve()
	require.ErrorIs(t, err, ErrLoaderResolved)
	require.Nil(t, resolved)
}

// TestRecheckAnswersResolveVerdict: each Recheck method runs its check over a
// Resolved's module set and answers the check's verdict: nil for a set
// Resolve accepted, and the check's own refusal for a set holding a module
// that check refuses.
//
// Method: resolve the embedded modules and recheck them, each recheck nil.
// Then, for each of the four rechecks, stage without Resolve the embedded
// modules and one module that only that check refuses (goyang's Process
// accepts it: StageProcess answers nil), and recheck through a Resolved built
// over the staged set. Only a test of this package can build that Resolved,
// since Resolve refuses such a set; it is the one way to show a recheck
// reports what its check finds rather than nil.
//
// VALIDATES: AC-19, a recheck answers what Resolve's own check answers.
// PREVENTS: a Recheck that answers nil whatever its check finds, so the
// benchmark line times a stage that refuses nothing.
func TestRecheckAnswersResolveVerdict(t *testing.T) {
	l := NewLoader()
	require.NoError(t, l.LoadEmbedded())
	resolved, err := l.Resolve()
	require.NoError(t, err)
	require.NoError(t, resolved.RecheckExtensions())
	require.NoError(t, resolved.RecheckPatterns())
	require.NoError(t, resolved.RecheckStructure())
	require.NoError(t, resolved.RecheckReferences())

	cases := []struct {
		name     string
		body     string
		recheck  func(*Resolved) error
		sentinel error
	}{
		{"extensions", `container top { ze:no-such-extension; }`,
			(*Resolved).RecheckExtensions, ErrUndeclaredExtension},
		{"patterns", `leaf probe { type string { pattern '[a-z'; } }`,
			(*Resolved).RecheckPatterns, ErrUncompilablePattern},
		{"structure", `leaf probe { type string { length "10..20 | 1..5"; } }`,
			(*Resolved).RecheckStructure, ErrLengthOrder},
		{"references", `container top { leaf peer { type leafref { path "/rp:top/rp:no-such-node"; } } }`,
			(*Resolved).RecheckReferences, ErrLeafrefPath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			staged := stagedWithProbe(t, tc.body)
			require.ErrorIs(t, tc.recheck(staged), tc.sentinel)
		})
	}
}

// TestRecheckRefusesAJoinedSet: on a Resolved the join built, whose
// compiled-in modules the join marked judged, the extension, pattern and
// structure rechecks refuse with ErrRecheckJoinedSet instead of judging the
// unjudged modules alone, and the leafref recheck, which judges the whole set,
// still answers its verdict.
//
// Method: resolve the embedded modules, reload them as the join does
// (Resolved.reloaded, which marks every reloaded module judged), resolve that
// loader, and recheck the joined Resolved.
//
// VALIDATES: AC-19, a recheck never measures a smaller set under its name.
// PREVENTS: a benchmark over a joined set reporting the cost of judging the
// external modules as the cost of the whole check.
func TestRecheckRefusesAJoinedSet(t *testing.T) {
	l := NewLoader()
	require.NoError(t, l.LoadEmbedded())
	compiled, err := l.Resolve()
	require.NoError(t, err)
	reloaded, err := compiled.reloaded()
	require.NoError(t, err)
	joined, err := reloaded.Resolve()
	require.NoError(t, err)

	require.ErrorIs(t, joined.RecheckExtensions(), ErrRecheckJoinedSet)
	require.ErrorIs(t, joined.RecheckPatterns(), ErrRecheckJoinedSet)
	require.ErrorIs(t, joined.RecheckStructure(), ErrRecheckJoinedSet)
	require.NoError(t, joined.RecheckReferences())
}

// stagedWithProbe answers a Resolved over the embedded modules and one probe
// module whose top level is body, after goyang's Process alone: no Ze check
// ran, so the set may hold what Resolve refuses.
func stagedWithProbe(t *testing.T, body string) *Resolved {
	t.Helper()
	staged := NewLoader()
	require.NoError(t, staged.LoadEmbedded())
	require.NoError(t, staged.AddModuleFromText("recheck-probe.yang", `module recheck-probe {
  namespace "urn:test:recheck-probe";
  prefix rp;
  import ze-extensions { prefix ze; }
  `+body+`
}`))
	require.NoError(t, staged.StageProcess())
	return &Resolved{modules: staged.modules}
}

// TestForgetCompiledPatternsEmptiesTheCache: after ForgetCompiledPatterns the
// pattern cache holds no pattern, so the next check compiles each again.
//
// Method: compile one pattern, see it cached, forget, see it gone.
//
// VALIDATES: AC-19, the pattern stage can be measured as a process's first
// Resolve pays it.
// PREVENTS: a pattern stage line that times cache hits.
func TestForgetCompiledPatternsEmptiesTheCache(t *testing.T) {
	const pattern = "[a-z]+-forget-test"
	_, err := compilePattern(pattern)
	require.NoError(t, err)
	_, cached := patternCache.Load(pattern)
	require.True(t, cached)

	ForgetCompiledPatterns()

	_, cached = patternCache.Load(pattern)
	require.False(t, cached)
}

// loaderFunc answers the function or method named name that file declares,
// or nil.
func loaderFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// buildStages answers, in call order, every call fn makes through a selector
// chain that starts at its Loader `l` (`l.X()`, `l.modules.X()`) and every
// package function it calls by its bare name, builtins aside.
func buildStages(fn *ast.FuncDecl) []string {
	var stages []string
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch callee := call.Fun.(type) {
		case *ast.SelectorExpr:
			if selectorRoot(callee) == "l" {
				stages = append(stages, callee.Sel.Name)
			}
		case *ast.Ident:
			if types.Universe.Lookup(callee.Name) == nil {
				stages = append(stages, callee.Name)
			}
		}
		return true
	})
	return stages
}

// selectorRoot answers the identifier a selector chain starts at (`l` for
// `l.modules.X`), or "" when the chain starts at anything else, such as a
// call's result.
func selectorRoot(selector *ast.SelectorExpr) string {
	expr := selector.X
	// Bounded by the depth of the selector expression in the source.
	for {
		switch x := expr.(type) {
		case *ast.Ident:
			return x.Name
		case *ast.SelectorExpr:
			expr = x.X
		default:
			return ""
		}
	}
}

// calledNames answers the name of every function and method file calls, by
// the name at the call: the selector of a method or a qualified function, the
// identifier of a bare call.
func calledNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch callee := call.Fun.(type) {
		case *ast.SelectorExpr:
			names[callee.Sel.Name] = true
		case *ast.Ident:
			names[callee.Name] = true
		}
		return true
	})
	return names
}
