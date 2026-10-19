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

// dockerExecSite is one exec.Command or exec.CommandContext call whose
// program is docker, by literal or by a string constant.
type dockerExecSite struct {
	File    string
	Line    int
	Program string
}

// pendingProgram is a call whose program is a constant the walk has not yet
// read: Package is the module-relative directory, or the import path, that
// declares Name.
type pendingProgram struct {
	File    string
	Line    int
	Package string
	Name    string
}

// dockerExecScan collects the docker exec sites of every parsed file. The
// constants a program names can sit in a file the walk reads later, so
// calls naming a constant are resolved once the walk ends.
type dockerExecScan struct {
	// constants maps a package directory to its string constants.
	constants map[string]map[string]string
	sites     []dockerExecSite
	pending   []pendingProgram
}

// scanFile records the string constants and the exec calls of one file.
// module is the module path, rel the file's tree-relative path.
func (s *dockerExecScan) scanFile(files *token.FileSet, module, rel string, file *ast.File) {
	dir := path.Dir(rel)
	s.recordConstants(dir, file)
	execName, imports := fileImports(module, file)
	if execName == "" {
		return
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		program, ok := execProgram(call, execName)
		if !ok {
			return true
		}
		line := files.Position(call.Pos()).Line
		s.recordProgram(rel, line, dir, imports, program)
		return true
	})
}

// recordConstants keeps every string constant file declares.
func (s *dockerExecScan) recordConstants(dir string, file *ast.File) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range value.Names {
				if i >= len(value.Values) {
					break
				}
				text, ok := stringLiteral(value.Values[i])
				if !ok {
					continue
				}
				if s.constants == nil {
					s.constants = make(map[string]map[string]string)
				}
				if s.constants[dir] == nil {
					s.constants[dir] = make(map[string]string)
				}
				s.constants[dir][name.Name] = text
			}
		}
	}
}

// fileImports answers the local name file gives os/exec (empty when it does
// not import it by name) and, for each module import, the local name the
// file calls it by mapped to its tree-relative directory. An import without
// an alias is called by its last path element, which is how every package
// in this module is named.
func fileImports(module string, file *ast.File) (string, map[string]string) {
	execName := ""
	imports := make(map[string]string)
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		local := path.Base(imported)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		if imported == execPackage {
			execName = local
			continue
		}
		if underDir(imported, module) && len(imported) > len(module) {
			imports[local] = imported[len(module)+1:]
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

// recordProgram keeps a literal docker program as a site, and a constant as
// pending until the walk has read every constant.
func (s *dockerExecScan) recordProgram(rel string, line int, dir string, imports map[string]string, program ast.Expr) {
	if text, ok := stringLiteral(program); ok {
		if dockerBinary(text) {
			s.sites = append(s.sites, dockerExecSite{File: rel, Line: line, Program: strconv.Quote(text)})
		}
		return
	}
	if ident, ok := program.(*ast.Ident); ok {
		s.pending = append(s.pending, pendingProgram{File: rel, Line: line, Package: dir, Name: ident.Name})
		return
	}
	selector, ok := program.(*ast.SelectorExpr)
	if !ok {
		return
	}
	receiver, ok := selector.X.(*ast.Ident)
	if !ok {
		return
	}
	if target, ok := imports[receiver.Name]; ok {
		s.pending = append(s.pending, pendingProgram{File: rel, Line: line, Package: target, Name: selector.Sel.Name})
	}
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

// violations answers every docker exec site outside DockerRoute, the
// constants resolved, sorted by file and line.
func (s *dockerExecScan) violations() []dockerExecSite {
	found := make([]dockerExecSite, 0, len(s.sites))
	found = append(found, s.sites...)
	for _, call := range s.pending {
		value, ok := s.constants[call.Package][call.Name]
		if !ok {
			continue
		}
		if dockerBinary(value) {
			found = append(found, dockerExecSite{File: call.File, Line: call.Line, Program: call.Name})
		}
	}
	found = slices.DeleteFunc(found, func(site dockerExecSite) bool { return underDir(site.File, DockerRoute) })
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
		tb.Str("  ").Str(site.File).Byte(':').Int(int64(site.Line)).Str(" runs ").Str(site.Program).Byte('\n')
	}
	tb.Str("  Rule: every docker command goes through interoplab.Docker, which labels what it creates so")
	tb.Str(" `./le test integration cleanup` removes what a killed run left, and bounds each command")
	tb.Str(" (docs/architecture/module-tiers.md, \"The docker route\").\n")
	tb.Str("  Fix: call the interoplab.Docker method for the command; a package test starts its server")
	tb.Str(" with labtest.Start (docs/architecture/testing/interop.md).\n")
	result.Diagnosis = tb.String()
	result.Code = 2
	return result
}
