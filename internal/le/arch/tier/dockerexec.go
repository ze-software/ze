// Design: docs/architecture/module-tiers.md -- the one route to the docker binary
// Overview: tier.go -- the source walk that feeds this scan

package archtier

import (
	"cmp"
	"go/ast"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// DockerRoute is the one tree whose code may exec the docker binary: the
// interoplab wrapper, which labels what it creates and bounds every command.
const DockerRoute = "internal/le/interoplab"

// dockerProgram is the program name the scan refuses outside DockerRoute.
const dockerProgram = "docker"

// execPackage is the import path whose Command and CommandContext start a
// host process.
const execPackage = "os/exec"

// symbolDepthMax bounds how many name-to-name hops a program name is followed
// through, so a cycle in broken source cannot loop the walk.
const symbolDepthMax = 16

// dockerExecSite is one exec.Command or exec.CommandContext call whose
// program is docker, or one package-level var holding it. Action is what the
// diagnosis says the line does: "runs", or "holds the docker program in".
type dockerExecSite struct {
	File    string
	Line    int
	Action  string
	Program string
}

// fileImport is one module import of a file: the imported package's
// tree-relative directory, and the alias the file gives it, empty for none.
type fileImport struct {
	Dir   string
	Alias string
}

// sourceScope is where an expression was written: its package directory and
// its file's module imports, which together resolve an identifier or a
// selector in it.
type sourceScope struct {
	Dir     string
	Imports []fileImport
}

// symbol is one package-level constant or var and the expression that
// initializes it.
type symbol struct {
	Scope sourceScope
	Value ast.Expr
}

// localValues maps each name a top-level declaration's function bodies assign
// to every expression assigned to it.
type localValues map[string][]ast.Expr

// programUse is an expression naming a program that the walk judges once it
// has read every file: the program of an exec call, or the initializer of a
// package-level var. Locals are the names of the declaration holding the
// call, nil for a package-level var.
type programUse struct {
	File   string
	Line   int
	Action string
	Name   string
	Scope  sourceScope
	Locals localValues
	Value  ast.Expr
}

// dockerExecScan collects the docker exec sites of every parsed file. A
// program can name a constant declared in a file the walk reads later, or
// in a package whose name the walk has not read yet, so every use is resolved
// once the walk ends.
type dockerExecScan struct {
	// packages maps a package directory to the name its package clause
	// declares, which is what an unaliased import of it is called.
	packages map[string]string
	// symbols maps a package directory to its package-level constants and vars.
	symbols map[string]map[string]symbol
	uses    []programUse
}

// scanFile records the package name, the package-level constants and vars,
// and the exec calls of one file with its function-scoped names. module is the module path, rel the file's
// tree-relative path.
func (s *dockerExecScan) scanFile(files *token.FileSet, module, rel string, file *ast.File) {
	scope := sourceScope{Dir: path.Dir(rel)}
	execName, imports := fileImports(module, file)
	scope.Imports = imports
	s.recordPackage(scope.Dir, file.Name.Name)
	s.recordSymbols(files, rel, scope, file)
	if execName == "" {
		return
	}
	for _, decl := range file.Decls {
		s.recordExecCalls(files, rel, scope, execName, decl)
	}
}

// recordExecCalls keeps each exec call of one top-level declaration as a use,
// with the names the declaration's function bodies assign, so a program held
// in a function-scoped const or var is followed as far as a package-level one.
func (s *dockerExecScan) recordExecCalls(files *token.FileSet, rel string, scope sourceScope, execName string, decl ast.Decl) {
	locals := collectLocals(decl)
	ast.Inspect(decl, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		program, ok := execProgram(call, execName)
		if !ok {
			return true
		}
		s.uses = append(s.uses, programUse{
			File: rel, Line: files.Position(call.Pos()).Line, Action: "runs",
			Name: programName(program), Scope: scope, Locals: locals, Value: program,
		})
		return true
	})
}

// collectLocals answers every name decl's function bodies assign, with each
// expression assigned to it: a const or var spec, and a `:=` or `=` to a bare
// name. The map is flow-insensitive and spans nested closures, so a name
// assigned docker anywhere in decl is docker at every exec of it: a false
// refusal costs a rename, a missed one an unlabeled container.
func collectLocals(decl ast.Decl) localValues {
	locals := localValues{}
	ast.Inspect(decl, func(node ast.Node) bool {
		if statement, ok := node.(*ast.DeclStmt); ok {
			if gen, ok := statement.Decl.(*ast.GenDecl); ok {
				eachValue(gen, func(name *ast.Ident, value ast.Expr) {
					locals[name.Name] = append(locals[name.Name], value)
				})
			}
			return true
		}
		if statement, ok := node.(*ast.AssignStmt); ok {
			locals.addAssign(statement)
		}
		return true
	})
	return locals
}

// addAssign keeps the right-hand sides of a `:=` or `=` that pairs each bare
// name with one expression. A compound assignment does not set a program
// name, and a multi-value call has no expression per name to follow.
func (l localValues) addAssign(statement *ast.AssignStmt) {
	if statement.Tok != token.DEFINE && statement.Tok != token.ASSIGN {
		return
	}
	if len(statement.Lhs) != len(statement.Rhs) {
		return
	}
	for i, target := range statement.Lhs {
		name, ok := target.(*ast.Ident)
		if !ok {
			continue
		}
		if name.Name == "_" {
			continue
		}
		l[name.Name] = append(l[name.Name], statement.Rhs[i])
	}
}

// eachValue calls record for every name gen declares with an initializer. A
// const spec with no values repeats the previous spec's, as the language
// defines, so `const ( a = "docker"; b )` gives b the docker value too.
func eachValue(gen *ast.GenDecl, record func(*ast.Ident, ast.Expr)) {
	if gen.Tok != token.CONST && gen.Tok != token.VAR {
		return
	}
	var previous []ast.Expr
	for _, spec := range gen.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		values := value.Values
		if len(values) == 0 && gen.Tok == token.CONST {
			values = previous
		}
		previous = values
		for i, name := range value.Names {
			if i >= len(values) {
				break
			}
			record(name, values[i])
		}
	}
}

// recordPackage keeps the package name of dir. An external test package
// (`name_test`) is not what an import of dir is called, so it is skipped.
func (s *dockerExecScan) recordPackage(dir, name string) {
	if strings.HasSuffix(name, "_test") {
		return
	}
	if s.packages == nil {
		s.packages = make(map[string]string)
	}
	s.packages[dir] = name
}

// recordSymbols keeps every package-level constant and var file declares with
// its initializer, and keeps each var as a use: a var holding docker is
// refused wherever it is read, because the scan cannot follow the var.
func (s *dockerExecScan) recordSymbols(files *token.FileSet, rel string, scope sourceScope, file *ast.File) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		eachValue(gen, func(name *ast.Ident, value ast.Expr) {
			s.recordSymbol(scope, name.Name, value)
			if gen.Tok == token.VAR {
				s.uses = append(s.uses, programUse{
					File: rel, Line: files.Position(name.Pos()).Line, Action: "holds the docker program in",
					Name: "var " + name.Name, Scope: scope, Value: value,
				})
			}
		})
	}
}

// recordSymbol keeps one package-level name and its initializer.
func (s *dockerExecScan) recordSymbol(scope sourceScope, name string, value ast.Expr) {
	if s.symbols == nil {
		s.symbols = make(map[string]map[string]symbol)
	}
	if s.symbols[scope.Dir] == nil {
		s.symbols[scope.Dir] = make(map[string]symbol)
	}
	s.symbols[scope.Dir][name] = symbol{Scope: scope, Value: value}
}

// fileImports answers the local name file gives os/exec (empty when it does
// not import it by name) and the file's module imports. The local name of an
// unaliased module import is the package clause of its directory, which the
// scan resolves once the walk ends, so a package named apart from its
// directory is still followed.
func fileImports(module string, file *ast.File) (string, []fileImport) {
	execName := ""
	var imports []fileImport
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		alias := ""
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		if imported == execPackage {
			execName = cmp.Or(alias, path.Base(imported))
			continue
		}
		if underDir(imported, module) && len(imported) > len(module) {
			imports = append(imports, fileImport{Dir: imported[len(module)+1:], Alias: alias})
		}
	}
	if execName == "_" {
		execName = ""
	}
	return execName, imports
}

// execProgram answers the program argument of a call to exec.Command or
// exec.CommandContext through execName.
func execProgram(call *ast.CallExpr, execName string) (ast.Expr, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	receiver, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil, false
	}
	if receiver.Name != execName {
		return nil, false
	}
	index := -1
	if selector.Sel.Name == "Command" {
		index = 0
	}
	if selector.Sel.Name == "CommandContext" {
		index = 1
	}
	if index < 0 {
		return nil, false
	}
	if index >= len(call.Args) {
		return nil, false
	}
	return call.Args[index], true
}

// programName is how the diagnosis names a program expression: a literal
// quoted, an identifier or a selector by the name it reads.
func programName(program ast.Expr) string {
	if text, ok := stringLiteral(program); ok {
		return strconv.Quote(text)
	}
	if ident, ok := program.(*ast.Ident); ok {
		return ident.Name
	}
	if selector, ok := program.(*ast.SelectorExpr); ok {
		return selector.Sel.Name
	}
	return "an expression"
}

// namesDocker reports whether expr names the docker binary: a literal, or a
// chain of names ending in one, through locals (the function-scoped names of
// the declaration expr was written in, nil at package level) and then the
// package-level names. A local shadows a package-level name, and a local
// assigned several expressions names docker when any of them does. depth
// counts the hops taken, bounded by symbolDepthMax.
func (s *dockerExecScan) namesDocker(scope sourceScope, locals localValues, expr ast.Expr, depth int) bool {
	if depth > symbolDepthMax {
		return false
	}
	if text, ok := stringLiteral(expr); ok {
		return dockerBinary(text)
	}
	if ident, ok := expr.(*ast.Ident); ok {
		if values, local := locals[ident.Name]; local {
			return slices.ContainsFunc(values, func(value ast.Expr) bool {
				return s.namesDocker(scope, locals, value, depth+1)
			})
		}
		target, found := s.symbols[scope.Dir][ident.Name]
		if !found {
			return false
		}
		return s.namesDocker(target.Scope, nil, target.Value, depth+1)
	}
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	dir, found := s.importedDir(scope, receiver.Name)
	if !found {
		return false
	}
	target, found := s.symbols[dir][selector.Sel.Name]
	if !found {
		return false
	}
	return s.namesDocker(target.Scope, nil, target.Value, depth+1)
}

// importedDir answers the directory of the module import scope calls local:
// by its alias, or by the package clause of its directory, or, for a
// directory the walk never read, by its last path element.
func (s *dockerExecScan) importedDir(scope sourceScope, local string) (string, bool) {
	for _, imported := range scope.Imports {
		name := imported.Alias
		if name == "" {
			name = cmp.Or(s.packages[imported.Dir], path.Base(imported.Dir))
		}
		if name == local {
			return imported.Dir, true
		}
	}
	return "", false
}

// stringLiteral answers the value of a string literal expression.
func stringLiteral(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok {
		return "", false
	}
	if literal.Kind != token.STRING {
		return "", false
	}
	text, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}
	return text, true
}

// dockerBinary reports whether program names the docker binary, bare or by path.
func dockerBinary(program string) bool {
	return path.Base(program) == dockerProgram
}

// violations answers every docker exec site and docker-holding var outside
// DockerRoute, the names resolved, sorted by file and line.
func (s *dockerExecScan) violations() []dockerExecSite {
	var found []dockerExecSite
	for _, use := range s.uses {
		if underDir(use.File, DockerRoute) {
			continue
		}
		if s.namesDocker(use.Scope, use.Locals, use.Value, 0) {
			found = append(found, dockerExecSite{File: use.File, Line: use.Line, Action: use.Action, Program: use.Name})
		}
	}
	slices.SortFunc(found, func(a, b dockerExecSite) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
	})
	return found
}

// dockerExecGate refuses an exec of the docker binary outside DockerRoute.
// It has no exemption list: every docker command goes through
// interoplab.Docker, whose labels let a later run remove what a killed run
// left, and a test starts a server through labtest.Start.
func dockerExecGate(scan *dockerExecScan) CheckResult {
	result := CheckResult{Name: "docker-exec-route"}
	var tb textbuf.Buffer
	found := scan.violations()
	if len(found) == 0 {
		result.Page = tb.Str("OK: no code outside ").Str(DockerRoute).Str(" execs docker.\n").String()
		return result
	}
	tb.Str("FAIL: exec of the docker binary outside ").Str(DockerRoute).Str(":\n")
	for _, site := range found {
		tb.Str("  ").Str(site.File).Byte(':').Int(int64(site.Line)).Byte(' ').Str(site.Action).Byte(' ').Str(site.Program).Byte('\n')
	}
	tb.Str("  Rule: every docker command goes through interoplab.Docker, which labels what it creates so")
	tb.Str(" `./le test integration clean` removes what a killed run left, and bounds each command")
	tb.Str(" (docs/architecture/module-tiers.md, \"The docker route\").\n")
	tb.Str("  Fix: call the interoplab.Docker method for the command; a package test starts its server")
	tb.Str(" with labtest.Start (docs/architecture/testing/interop.md).\n")
	result.Diagnosis = tb.String()
	result.Code = 2
	return result
}
