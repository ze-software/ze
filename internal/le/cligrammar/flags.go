// Design: docs/architecture/cli/root-namespace-grammar.md -- the flag-register feeder
//
// flags.go is feeder 7 of the grammar gate: which register a token belongs to.
//
// Feeders 1 to 6 read the YANG command model and the root names, where a flag
// is banned outright and the check is a string test. The offline surface is the
// one where a flag is LEGAL, so nothing checked it: 57 flag.NewFlagSet call
// sites, 121 flag declarations and 78 distinct flag names, and no gate over any
// of them. This feeder answers the four questions ai/rules/cli.md asks of a
// flag, from source:
//
//	F1 is this flag a command name (a registered root spelled as a flag)
//	F2 does a client build this flag into a command string it sends to the daemon
//	F3 does this flag render, which is the pipe layer's job on every command
//	F4 does the flag registry agree with the parser about which flags exist
//
// The checks themselves are grammar.CheckRootFlagForm, grammar.DaemonCommandFlag,
// grammar.CheckPipeFlags and grammar.CheckFlagDeclarations. This file collects
// the populations they judge, by parsing every Go source under cmd/ze and
// internal exactly once. The debt this tree already carries, and the
// partitioning of a finding into open or tracked, is flagdebt.go.

package cligrammar

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/grammar"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// flagCallArity is the least arguments a flag-declaring call carries: a name, a
// default and a usage line. It keeps an unrelated method that happens to share
// a name (a Stringer's String, a buffer's Bool) out of the count.
const flagCallArity = 3

// flagDeclaringMethods maps a *flag.FlagSet method to the argument index that
// carries the flag name. The Var forms take the destination first, so their
// name is the second argument.
var flagDeclaringMethods = map[string]int{
	"Bool": 0, "Duration": 0, "Float64": 0, "Func": 0, "BoolFunc": 0,
	"Int": 0, "Int64": 0, "String": 0, "Uint": 0, "Uint64": 0,
	"BoolVar": 1, "DurationVar": 1, "Float64Var": 1, "Func1": 1,
	"IntVar": 1, "Int64Var": 1, "StringVar": 1, "TextVar": 1,
	"UintVar": 1, "Uint64Var": 1, "Var": 1,
}

// parsedFlagSet is one flag.NewFlagSet call site and the flags declared on it.
type parsedFlagSet struct {
	// Name is the literal name the flag set was built with, which is the
	// command path an operator types (with or without a leading "ze").
	Name  string
	File  string
	Line  int
	Flags []string
}

// goSurface is everything the source-reading feeders take from one parse of the
// checkout's Go sources.
type goSurface struct {
	// Roots are the string-literal names of every registered root handler.
	Roots []string
	// FlagSets are the resolved flag.NewFlagSet call sites.
	FlagSets []parsedFlagSet
	// Declared maps a command path to the flag tokens
	// registry.RegisterCommandFlags declares for it.
	Declared map[string][]string
	// LocalData holds every path registered through RegisterLocalData, whose
	// answer therefore reaches the pipe layer.
	LocalData []string
	// Literals holds the string literals that might be a daemon command; the
	// F2 check reads them once the daemon path set is known.
	Literals []rawLiteral
	// FilesRead is how many Go sources the walk parsed, which is what the floor
	// is measured against.
	FilesRead int
	// UnresolvedSetNames counts flag sets built with a name this static scan
	// cannot read (a constant, a buffer). Their flags are checked by nothing,
	// so the count is published rather than dropped.
	UnresolvedSetNames int
	// UnresolvedFlagNames counts flag declarations whose name is not a literal.
	UnresolvedFlagNames int
	// UnattributedFlags counts flag declarations on a set the enclosing
	// function did not build, whose command this scan cannot decide.
	UnattributedFlags int
}

// rawLiteral is a string literal and where it was written.
type rawLiteral struct {
	File string
	Line int
	Text string
}

// scanGoSurface parses every Go source under the tree's cmd/ze and internal
// directories once and answers the populations the source-reading feeders need.
//
// A file the parser cannot read is an ERROR rather than an empty answer: a root
// or a flag set this scan never resolved is one no feeder checked, and a gate
// that walks past it reports OK having read less than it claims.
//
// LIMITATION (shared with internal/le/commandownership): this is a static AST
// scan, so a root registered with a NON-LITERAL name -- a constant, or a
// variable passed through a helper like internal/test/cli's registerRoot -- is
// not resolved and is not checked. Every `ze` CLI root but `l2tp` is registered
// with a literal; the invisible cases are that one and the `ze-test <suite>`
// roots, which are a different binary and surface. A flag set named the same
// way is counted in UnresolvedSetNames rather than dropped silently.
//
// Local metas (RegisterLocalMeta) are deliberately excluded from Roots:
// `update serve` is a two-token local path, not a compound root.
func scanGoSurface(tree string) (goSurface, error) {
	surface := goSurface{Declared: map[string][]string{}}
	for _, dir := range []string{filepath.Join("cmd", "ze"), "internal"} {
		err := filepath.WalkDir(filepath.Join(tree, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(tree, path)
			if relErr != nil {
				return relErr
			}
			return scanGoFile(path, filepath.ToSlash(rel), &surface)
		})
		if err != nil {
			return goSurface{}, err
		}
	}
	return surface, nil
}

// scanGoFile adds one Go source's registrations, flag sets and literals to the
// surface.
func scanGoFile(path, rel string, surface *goSurface) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		var tb textbuf.Buffer
		return &scanError{message: tb.Str("parse ").Str(rel).Str(": ").Err(err).String()}
	}
	surface.FilesRead++

	specSets := flagSpecSets(file)
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			registryCall(typed, specSets, surface)
		case *ast.FuncDecl:
			functionFlagSets(typed, fset, rel, surface)
		case *ast.BasicLit:
			if typed.Kind != token.STRING {
				return true
			}
			text, unquoteErr := strconv.Unquote(typed.Value)
			if unquoteErr != nil || !strings.Contains(text, " ") || !strings.Contains(text, "-") {
				return true
			}
			surface.Literals = append(surface.Literals, rawLiteral{
				File: rel, Line: fset.Position(typed.Pos()).Line, Text: text,
			})
		}
		return true
	})
	return nil
}

// scanError is a source the scan could not read, kept as a type so the walk's
// error reads the same however deep it was raised.
type scanError struct{ message string }

func (e *scanError) Error() string { return e.message }

// registryCall records a root registration, a flag declaration, or a local
// data registration, each of which is a population a feeder reads.
func registryCall(call *ast.CallExpr, specSets map[string][]string, surface *goSurface) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || (pkg.Name != "registry" && pkg.Name != "cmdregistry") {
		return
	}
	path, ok := stringArg(call, 0)
	if !ok {
		return
	}
	switch selector.Sel.Name {
	case "RegisterRootHandler", "MustRegisterRootHandler", "RegisterRoot":
		surface.Roots = append(surface.Roots, path)
	case "RegisterLocalData", "MustRegisterLocalData":
		surface.LocalData = append(surface.LocalData, path)
	case "RegisterCommandFlags":
		if len(call.Args) < 2 {
			return
		}
		surface.Declared[path] = append(surface.Declared[path], flagSpecNames(call.Args[1], specSets)...)
	}
}

// flagSpecSets answers every `<name> := []registry.FlagSpec{...}` in one file,
// so a RegisterCommandFlags call given a variable is still resolved. The l2tp
// CLI declares one set and registers it under three paths.
func flagSpecSets(file *ast.File) map[string][]string {
	sets := map[string][]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		name, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		if names := compositeFlagSpecNames(assign.Rhs[0]); names != nil {
			sets[name.Name] = names
		}
		return true
	})
	return sets
}

// flagSpecNames answers the flag tokens a RegisterCommandFlags argument holds,
// whether it is written inline or named by a variable this file assigns.
func flagSpecNames(arg ast.Expr, specSets map[string][]string) []string {
	if names := compositeFlagSpecNames(arg); names != nil {
		return names
	}
	if ident, ok := arg.(*ast.Ident); ok {
		return specSets[ident.Name]
	}
	return nil
}

// compositeFlagSpecNames reads the Name field of every FlagSpec in a composite
// literal.
func compositeFlagSpecNames(expr ast.Expr) []string {
	composite, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(composite.Elts))
	for _, element := range composite.Elts {
		spec, ok := element.(*ast.CompositeLit)
		if !ok {
			continue
		}
		for _, field := range spec.Elts {
			pair, ok := field.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := pair.Key.(*ast.Ident)
			if !ok || key.Name != "Name" {
				continue
			}
			if name, ok := literalString(pair.Value); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

// functionFlagSets records every flag set one function builds, with the flags
// it declares on each.
//
// Resolution is by identifier within the function, which is what a flag set is:
// built on one line and given its flags on the next few.
//
// A declaration on an identifier this function did not build is UNATTRIBUTED
// and counted, never attributed to whichever set happens to be in scope. Eight
// NLRI plugins declare their decode flags that way (`cfg.ExtraFlags = func(fs
// *flag.FlagSet)`, internal/component/bgp/plugins/nlri/*/register.go), and the
// command those flags belong to is decided by the caller, which a static scan
// of one function cannot follow.
func functionFlagSets(function *ast.FuncDecl, fset *token.FileSet, rel string, surface *goSurface) {
	if function.Body == nil {
		return
	}
	index := map[string]int{}
	sets := make([]parsedFlagSet, 0, 2)

	ast.Inspect(function.Body, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok || !isSelector(call.Fun, "flag", "NewFlagSet") {
			return true
		}
		name, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		literal, ok := stringArg(call, 0)
		if !ok {
			surface.UnresolvedSetNames++
			return true
		}
		index[name.Name] = len(sets)
		sets = append(sets, parsedFlagSet{
			Name: literal, File: rel, Line: fset.Position(call.Pos()).Line,
		})
		return true
	})
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		at, declares := flagDeclaringMethods[selector.Sel.Name]
		if !declares || len(call.Args) < flagCallArity {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		position, known := index[receiver.Name]
		name, isLiteral := literalString(call.Args[at])
		switch {
		case !known && isLiteral:
			surface.UnattributedFlags++
		case !known:
			return true
		case !isLiteral:
			surface.UnresolvedFlagNames++
		default:
			sets[position].Flags = append(sets[position].Flags, name)
		}
		return true
	})
	surface.FlagSets = append(surface.FlagSets, sets...)
}

// isSelector reports whether an expression is `<pkg>.<name>`.
func isSelector(expr ast.Expr, pkg, name string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == pkg
}

// stringArg answers the string literal at one argument position.
func stringArg(call *ast.CallExpr, at int) (string, bool) {
	if len(call.Args) <= at {
		return "", false
	}
	return literalString(call.Args[at])
}

// literalString answers the value of a string-literal expression.
func literalString(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

// commandPaths answers every path the command tree dispatches, in the spelling
// that carries the verb and in the spelling a client that supplies the verb
// itself uses.
//
// Both are needed because a client writes either one: `ze interface migrate`
// sent "interface migrate ..." while the daemon dispatches on "request
// interface migrate". Only paths of two tokens or more are added, so a bare
// verb never matches.
func commandPaths(node *command.Node, prefix string, into map[string]bool) {
	if node == nil {
		return
	}
	var tb textbuf.Buffer
	for name, child := range node.Children {
		path := name
		if prefix != "" {
			path = tb.Reset().Str(prefix).Byte(' ').Str(name).String()
		}
		if strings.Contains(path, " ") {
			into[path] = true
			if _, tail, found := strings.Cut(path, " "); found && strings.Contains(tail, " ") {
				into[tail] = true
			}
		}
		commandPaths(child, path, into)
	}
}

// pathAliases answers the two spellings one registered path is written as: the
// path itself, and the path with its leading verb dropped. `show config dump`
// is the registration; `config dump` is what the flag set and the operator
// call it. flagSetPath drops a leading `ze` before the comparison.
func pathAliases(path string) []string {
	aliases := []string{path}
	if _, tail, found := strings.Cut(path, " "); found && tail != "" {
		aliases = append(aliases, tail)
	}
	return aliases
}

// flagSetPath answers the command path a flag set's name names, with a leading
// `ze` dropped. `ze interface migrate` and `config dump` are both written, and
// they are the same kind of name.
func flagSetPath(name string) string {
	trimmed := strings.TrimSpace(name)
	if rest, found := strings.CutPrefix(trimmed, "ze "); found {
		return strings.TrimSpace(rest)
	}
	return trimmed
}

// checkFlagRegister runs feeder 7 over one checkout's Go surface and adds what
// it finds to the result.
//
// daemonPaths is every path the daemon dispatches; roots are the registered
// root names, which say which flag sets belong to the `ze` command surface
// rather than to a separate binary.
func checkFlagRegister(surface goSurface, daemonPaths map[string]bool, roots []string, result *Result) {
	result.GoFilesRead = surface.FilesRead
	result.FlagSetsRead = len(surface.FlagSets)
	result.FlagNamesUnresolved = surface.UnresolvedFlagNames
	result.FlagSetNamesUnresolved = surface.UnresolvedSetNames
	result.FlagsUnattributed = surface.UnattributedFlags

	served := map[string]bool{}
	for _, path := range surface.LocalData {
		for _, alias := range pathAliases(path) {
			served[alias] = true
		}
	}

	var hits []FlagRegisterHit
	for _, finding := range grammar.CheckRootFlagForm(roots) {
		hits = append(hits, flagHit(finding, "", 0))
	}

	for _, literal := range surface.Literals {
		path, flag, found := grammar.DaemonCommandFlag(literal.Text, daemonPaths)
		if !found {
			continue
		}
		// A path command.ServeLocal answers is served in THIS process, so
		// (*Dispatcher).Dispatch never sees the string and the refusal F2 names
		// never happens. Its flags are judged where they are declared, by F3
		// and F4, so the exclusion loses no coverage of the defect: it drops a
		// second symptom whose message would be false. The count is published
		// so the population set aside is visible rather than silently dropped.
		if served[path] {
			result.ClientLiteralsServedLocally++
			continue
		}
		hits = append(hits, flagHit(grammar.ClientFlagFinding(path, flag), literal.File, literal.Line))
	}
	for _, set := range surface.FlagSets {
		path := flagSetPath(set.Name)
		inSurface := offlineZeCommand(path, roots, surface.Declared)
		// F3 judges every command of the `ze` surface, registered for local
		// data or not: rendering is the pipe layer's job unconditionally, and a
		// command that reaches no pipe layer is the defect rather than the
		// exemption. served decides only how the finding words the fix.
		if inSurface || served[path] {
			hits = append(hits, sited(grammar.CheckPipeFlags(path, set.Flags, served[path]), set)...)
		}
		if !inSurface {
			result.FlagSetsOutOfScope++
			continue
		}
		result.FlagSetsInScope++
		hits = append(hits, sited(grammar.CheckFlagDeclarations(path, set.Flags, surface.Declared[path]), set)...)
	}

	result.FlagFindings, result.FlagDebt = partitionDebt(hits)
}

// sited pairs each finding with the flag set's own call site, which is where a
// reader goes to fix it.
func sited(findings []grammar.FlagFinding, set parsedFlagSet) []FlagRegisterHit {
	hits := make([]FlagRegisterHit, 0, len(findings))
	for _, finding := range findings {
		hits = append(hits, flagHit(finding, set.File, set.Line))
	}
	return hits
}

// offlineZeCommand reports whether a flag set's path is a `ze` offline command,
// which is the population that owes a RegisterCommandFlags declaration.
//
// A path qualifies when its first token is a registered root, or when the flag
// registry already holds a path with that first token. The second arm is what
// covers `l2tp`, whose root is registered with a constant this scan cannot
// read. Everything else -- `ze-test <suite>`, `ze-perf run`, `ze-chaos`, the
// mock servers, the appliance's internal tools -- is a separate binary with no
// completion surface to be invisible to, and is counted out of scope rather
// than judged.
func offlineZeCommand(path string, roots []string, declared map[string][]string) bool {
	head, _, _ := strings.Cut(path, " ")
	if head == "" {
		return false
	}
	if slices.Contains(roots, head) {
		return true
	}
	for known := range declared {
		if first, _, _ := strings.Cut(known, " "); first == head {
			return true
		}
	}
	return false
}

// sortedKeys answers a string-keyed map's keys in order.
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
