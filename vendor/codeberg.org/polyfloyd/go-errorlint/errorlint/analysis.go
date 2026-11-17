package errorlint

import (
	"go/ast"
	"go/types"
	"sort"

	"golang.org/x/tools/go/analysis"
)

func NewAnalyzer(opts ...Option) *analysis.Analyzer {
	cfg := &config{allowed: newAllowList()}

	a := &analysis.Analyzer{
		Name: "errorlint",
		Doc:  "Linter for error wrapping issues.",
		Run:  cfg.run,
	}

	// Registering a flag assigns its default value to the variable, so this must happen before the
	// options are applied, or the options would be overwritten.
	a.Flags.BoolVar(&cfg.checkComparison, "comparison", true, "Check for plain error comparisons")
	a.Flags.BoolVar(&cfg.checkAsserts, "asserts", true, "Check for plain type assertions and type switches")
	a.Flags.BoolVar(&cfg.checkErrorf, "errorf", false, "Check whether fmt.Errorf uses the %w verb for formatting errors. See the readme for caveats")
	a.Flags.BoolVar(&cfg.checkErrorfMulti, "errorf-multi", true, "Permit more than 1 %w verb, valid per Go 1.20 (Requires -errorf=true)")

	for _, o := range opts {
		o(cfg)
	}

	return a
}

func (cfg *config) run(pass *analysis.Pass) (interface{}, error) {
	var lints []analysis.Diagnostic
	extInfo := newTypesInfoExt(pass)
	extInfo.allowed = cfg.allowed
	if cfg.checkComparison {
		l := LintErrorComparisons(extInfo)
		lints = append(lints, l...)
	}
	if cfg.checkAsserts {
		l := LintErrorTypeAssertions(pass.Fset, extInfo)
		lints = append(lints, l...)
	}
	if cfg.checkErrorf {
		l := LintFmtErrorfCalls(pass.Fset, *pass.TypesInfo, cfg.checkErrorfMulti)
		lints = append(lints, l...)
	}
	sort.Sort(ByPosition(lints))

	for _, l := range lints {
		pass.Report(l)
	}
	return nil, nil
}

type TypesInfoExt struct {
	*analysis.Pass

	// Maps AST nodes back to the node they are contained within.
	NodeParent map[ast.Node]ast.Node

	// Maps an object back to all identifiers to refer to it.
	IdentifiersForObject map[types.Object][]*ast.Ident

	// The errors that may be compared directly. The defaults are used when nil.
	allowed *allowList
}

func (info *TypesInfoExt) allowList() *allowList {
	if info.allowed == nil {
		return defaultAllowList()
	}
	return info.allowed
}

func newTypesInfoExt(pass *analysis.Pass) *TypesInfoExt {
	nodeParent := map[ast.Node]ast.Node{}
	for node := range pass.TypesInfo.Scopes {
		file, ok := node.(*ast.File)
		if !ok {
			continue
		}
		stack := []ast.Node{file}
		ast.Inspect(file, func(n ast.Node) bool {
			nodeParent[n] = stack[len(stack)-1]
			if n == nil {
				stack = stack[:len(stack)-1]
			} else {
				stack = append(stack, n)
			}
			return true
		})
	}

	identifiersForObject := map[types.Object][]*ast.Ident{}
	for node, obj := range pass.TypesInfo.Defs {
		identifiersForObject[obj] = append(identifiersForObject[obj], node)
	}
	for node, obj := range pass.TypesInfo.Uses {
		identifiersForObject[obj] = append(identifiersForObject[obj], node)
	}

	return &TypesInfoExt{
		Pass:                 pass,
		NodeParent:           nodeParent,
		IdentifiersForObject: identifiersForObject,
	}
}

func (info *TypesInfoExt) ContainingFuncDecl(node ast.Node) *ast.FuncDecl {
	for parent := info.NodeParent[node]; ; parent = info.NodeParent[parent] {
		if _, ok := parent.(*ast.File); ok {
			break
		}
		if fun, ok := parent.(*ast.FuncDecl); ok {
			return fun
		}
	}
	return nil
}
