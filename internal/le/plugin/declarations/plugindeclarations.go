// Design: docs/architecture/api/commands.md -- a plugin's command declaration
//
// Package plugindeclarations holds a plugin's command declarations to ONE
// function.
//
// A plugin states its commands to two readers. registry.Registration.Commands
// is what anything linking the composition root reads, with no engine started,
// which is how the published catalog learns what a plugin serves. The
// sdk.Registration a runner passes to p.Run is what a RUNNING daemon reads,
// over the Stage 1 registration message. Both are meant to be one expression,
// a single commandDecls() call, so that nothing is copied and nothing can
// disagree.
//
// The drift this gate exists to catch is a plugin that declares a command to
// Stage 1 and not to its registration. The daemon then serves the command, the
// catalog never names it, and no test goes red: the catalog is simply short,
// which is exactly the failure that has no signal of its own.
//
// The scan needs no list of where a plugin may live. Its subject is a package
// holding BOTH literals, which is what a registered plugin is. A test fixture
// that builds an sdk.Registration and registers no plugin has no registration
// to agree with, so that rule passes it over rather than a path exclusion
// doing it.

package plugindeclarations

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// sdkImportPath and registryImportPath are the two packages whose Registration
// type carries a Commands field. The local identifier for each is resolved per
// file from that file's own import declarations, so a renamed import is still
// matched.
const (
	sdkImportPath      = "github.com/ze-software/ze/pkg/plugin/sdk"
	registryImportPath = "github.com/ze-software/ze/internal/component/plugin/registry"
)

// registrationToken is the byte sequence a file must carry before the gate
// parses it. Both literals open with it, and a file naming neither cannot hold
// either one.
var registrationToken = []byte("Registration{")

// scanRoot is the one subtree a registered plugin can live in. pkg/ is the
// plugin CONTRACT and registers nothing, and cmd/ holds main packages.
const scanRoot = "internal"

// packageFloor is the least plugin packages the walk must find before the gate
// believes it read the tree. This checkout carried 33 on 2026-09-07, so the
// floor fires on a walk that read nothing rather than on a plugin somebody
// deleted.
const packageFloor = 20

// ErrWalkTooSmall says the walk found fewer plugin packages than the floor, so
// its empty answer is a failed read rather than a clean tree.
var ErrWalkTooSmall = errors.New("the plugin-declaration walk found too few plugin packages")

// Check walks tree for every package that builds both a registry.Registration
// and an sdk.Registration, and answers every command the runner declares that
// the registration does not carry.
//
// floor is a parameter rather than a constant because a fixture tree holds one
// package: le passes packageFloor and a test passes 0.
func Check(tree string, floor int) (Findings, error) {
	packages, err := collect(tree)
	if err != nil {
		return nil, err
	}
	if len(packages) < floor {
		return nil, fmt.Errorf("%w: %d found, at least %d expected",
			ErrWalkTooSmall, len(packages), floor)
	}

	var findings Findings
	for _, pkg := range packages {
		findings = append(findings, pkg.disagreements()...)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Package != findings[j].Package {
			return findings[i].Package < findings[j].Package
		}
		return findings[i].Command < findings[j].Command
	})
	return findings, nil
}

// declaration is one Commands or Pipes field found on a Registration literal:
// where it was written, and the identities it resolves to.
type declaration struct {
	file  string
	line  int
	names []string
	// unresolved holds the source position of every entry whose command name
	// the reader could not resolve. It is never dropped: a name the gate
	// cannot read is a comparison it cannot make, and answering "they agree"
	// for one would be a zero standing in for an answer
	// (ai/rules/principles.md).
	unresolved []string
	// fromFunc is the parameterless function in this package the field CALLS,
	// and "" for a field written as a literal or as anything else.
	//
	// It is what lets the gate answer for a declaration function whose body it
	// cannot read. Two literals that both call one parameterless function
	// return one slice, so they cannot disagree, whatever the body does.
	// bgp-rib builds its list in a loop over its own dispatch table
	// (internal/component/bgp/plugins/rib/rib.go, commandDecls), which is the
	// shape this repository asks for and which no syntactic reader can
	// evaluate.
	fromFunc string
}

// pluginPackage is one package directory that registers a plugin: what its
// runner declares to Stage 1, and what its registration carries.
type pluginPackage struct {
	// dir is the package directory relative to the tree, with forward slashes.
	// It NAMES the plugin in a finding: the registration's own Name field is
	// usually a constant, so the directory is the identity every finding can
	// carry.
	dir string
	// buildsRunner and buildsRegistration say which Registration LITERALS the
	// package writes, which is a different fact from whether either sets a
	// Commands field. The scope rule turns on the literals: a plugin whose
	// registration names no command is precisely the drift this gate looks
	// for, so reading its absence as "not a plugin" would make the gate
	// vacuous for its own subject.
	buildsRunner       bool
	buildsRegistration bool
	runner             []declaration
	register           []declaration
	// runnerPipes and registerPipes are the same two readings of the PIPE
	// channel: the aliases a plugin puts on its own commands. They are kept
	// apart from the command channel because the two are identified
	// differently and a finding must name which channel drifted.
	runnerPipes   []declaration
	registerPipes []declaration
}

// channel is one declaration channel a plugin writes on a Registration literal.
//
// The gate compares both the same way, so the difference between them is data:
// the field to read, how one entry is identified, and what a finding says when
// the registration is missing it.
type channel struct {
	field    string
	identity func(entry *ast.CompositeLit) (string, bool)
	reason   string
}

// channels answers the two declaration channels, in the order a finding sorts.
//
// A command is identified by its Name alone. A pipe alias is identified by the
// PAIR its registry keys it on, the command path it sits on and the name an
// operator types, because one plugin puts the same alias name on more than one
// command and those are two declarations rather than one
// (internal/component/command/alias.go, RegisterPluginAliases).
func channels() []channel {
	return []channel{
		{
			field:    "Commands",
			identity: commandIdentity,
			reason:   "declared to Stage 1 and absent from registry.Registration.Commands",
		},
		{
			field:    "Pipes",
			identity: pipeIdentity,
			reason:   "declared to Stage 1 and absent from registry.Registration.Pipes",
		},
	}
}

// commandIdentity answers the command a CommandDecl entry names.
func commandIdentity(entry *ast.CompositeLit) (string, bool) {
	return declaredField(entry, "Name")
}

// pipeIdentity answers the command path and name a PipeDecl entry names, as one
// string a reader can compare and read.
func pipeIdentity(entry *ast.CompositeLit) (string, bool) {
	command, named := declaredField(entry, "Command")
	if !named {
		return "", false
	}
	name, spelled := declaredField(entry, "Name")
	if !spelled {
		return "", false
	}
	var tb textbuf.Buffer
	return tb.Str(command).Str(" | ").Str(name).String(), true
}

// declaredField answers what one field of a declaration entry states, and
// whether the gate could read it.
func declaredField(entry *ast.CompositeLit, field string) (string, bool) {
	value := fieldValue(entry, field)
	if value == nil {
		return "", false
	}
	return commandName(value)
}

// disagreements answers one finding per declaration the runner makes that the
// registration does not, over BOTH channels, and one per declaration entry the
// gate could not read.
func (p *pluginPackage) disagreements() Findings {
	var findings Findings
	for _, channel := range channels() {
		runner, register := p.runner, p.register
		if channel.field == "Pipes" {
			runner, register = p.runnerPipes, p.registerPipes
		}
		findings = append(findings, compareChannel(p.dir, channel.reason, runner, register)...)
	}
	return findings
}

// compareChannel answers what one channel's two readings disagree about.
//
// A runner field that CALLS the same parameterless function the registration
// calls is skipped whole. One function answers one slice, so the two readings
// are the same declaration and there is nothing to compare; reporting its body
// as unreadable would refuse the very pattern this gate's own remedy asks for.
func compareChannel(dir, reason string, runner, register []declaration) Findings {
	registered := make(map[string]bool)
	shared := make(map[string]bool)
	for _, decl := range register {
		for _, name := range decl.names {
			registered[name] = true
		}
		if decl.fromFunc != "" {
			shared[decl.fromFunc] = true
		}
	}

	var findings Findings
	for _, decl := range runner {
		if decl.fromFunc != "" && shared[decl.fromFunc] {
			continue
		}
		for _, where := range decl.unresolved {
			findings = append(findings, Finding{
				Package: dir,
				Command: where,
				File:    decl.file,
				Line:    decl.line,
				Reason:  "the runner's declaration cannot be read, so it cannot be compared",
			})
		}
		for _, name := range decl.names {
			if registered[name] {
				continue
			}
			findings = append(findings, Finding{
				Package: dir,
				Command: name,
				File:    decl.file,
				Line:    decl.line,
				Reason:  reason,
			})
		}
	}
	return findings
}

// collect walks tree's scan root and answers one entry per package that builds
// both Registration types, keyed by the package directory.
func collect(tree string) (map[string]pluginPackage, error) {
	root := filepath.Join(tree, scanRoot)
	byDir := map[string][]string{}

	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" || entry.Name() == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		byDir[filepath.Dir(path)] = append(byDir[filepath.Dir(path)], path)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk %s: %w", root, walkErr)
	}

	packages := map[string]pluginPackage{}
	for dir, files := range byDir {
		pkg, err := readPackage(tree, dir, files)
		if err != nil {
			return nil, err
		}
		if !pkg.isPlugin() {
			continue
		}
		packages[pkg.dir] = pkg
	}
	return packages, nil
}

// readPackage parses one directory and answers what it declares. A directory
// that is not a plugin answers the zero value, which isPlugin reports on, so
// there is no nil for a caller to tell apart from a failure
// (ai/rules/principles.md).
func readPackage(tree, dir string, files []string) (pluginPackage, error) {
	fset := token.NewFileSet()
	parsed := make([]*ast.File, 0, len(files))

	for _, path := range files {
		source, err := os.ReadFile(path) //nolint:gosec // a repository gate reads the checkout it walks
		if err != nil {
			return pluginPackage{}, fmt.Errorf("read %s: %w", path, err)
		}
		// A file naming neither type cannot hold either literal, and this byte
		// test is what keeps the gate off a full parse of the whole tree. It
		// reads the bytes rather than a string conversion of them, which would
		// copy every Go file under internal/ to answer one substring question.
		if !bytes.Contains(source, registrationToken) {
			continue
		}
		file, err := parser.ParseFile(fset, path, source, parser.SkipObjectResolution)
		if err != nil {
			return pluginPackage{}, fmt.Errorf("parse %s: %w", path, err)
		}
		parsed = append(parsed, file)
	}
	if len(parsed) == 0 {
		return pluginPackage{}, nil
	}

	relative, err := filepath.Rel(tree, dir)
	if err != nil {
		return pluginPackage{}, fmt.Errorf("locate %s under %s: %w", dir, tree, err)
	}
	pkg := pluginPackage{dir: filepath.ToSlash(relative)}

	// The whole directory's declaration functions are indexed first, because a
	// runner in one file calls a commandDecls() written in another.
	decls := commandFuncs(parsed)

	for _, file := range parsed {
		sdkName := importAlias(file, sdkImportPath)
		registryName := importAlias(file, registryImportPath)
		if sdkName == "" && registryName == "" {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			owner := registrationOwner(literal)
			if owner == "" {
				return true
			}
			if owner != sdkName && owner != registryName {
				return true
			}
			if owner == sdkName {
				pkg.buildsRunner = true
			} else {
				pkg.buildsRegistration = true
			}

			for _, channel := range channels() {
				value := fieldValue(literal, channel.field)
				if value == nil {
					continue
				}

				position := fset.Position(value.Pos())
				found := resolve(value, decls, fset, channel.identity)
				found.file = filepath.ToSlash(relativeTo(tree, position.Filename))
				found.line = position.Line

				switch {
				case owner == sdkName && channel.field == "Pipes":
					pkg.runnerPipes = append(pkg.runnerPipes, found)
				case owner == sdkName:
					pkg.runner = append(pkg.runner, found)
				case channel.field == "Pipes":
					pkg.registerPipes = append(pkg.registerPipes, found)
				default:
					pkg.register = append(pkg.register, found)
				}
			}
			return true
		})
	}

	return pkg, nil
}

// isPlugin reports whether the package builds BOTH Registration literals, which
// is what a registered plugin does and what this gate judges. A package that
// builds one of them, a test fixture driving an sdk.Registration for instance,
// has no second reading to agree with.
func (p *pluginPackage) isPlugin() bool { return p.buildsRunner && p.buildsRegistration }

// importAlias answers the identifier this file names an import path by, and ""
// when the file does not import it. A renamed import answers its rename, so a
// literal written against it is still matched.
func importAlias(file *ast.File, path string) string {
	for _, spec := range file.Imports {
		quoted, err := strconv.Unquote(spec.Path.Value)
		if err != nil || quoted != path {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return path[strings.LastIndex(path, "/")+1:]
	}
	return ""
}

// registrationOwner answers the package identifier of a `<pkg>.Registration`
// composite literal, and "" for any other literal.
func registrationOwner(literal *ast.CompositeLit) string {
	selector, ok := literal.Type.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if selector.Sel.Name != "Registration" {
		return ""
	}
	owner, ok := selector.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return owner.Name
}

// fieldValue answers the value expression of one named field of a composite
// literal, and nil when the literal does not set that field.
func fieldValue(literal *ast.CompositeLit, field string) ast.Expr {
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if key.Name == field {
			return pair.Value
		}
	}
	return nil
}

// commandFuncs indexes every parameterless function in the package whose body
// is one return statement, keyed by name. That is the shape every plugin's
// commandDecls() and pipeDecls() has, and it is what lets both sides of either
// channel resolve to one list.
func commandFuncs(files []*ast.File) map[string]ast.Expr {
	found := map[string]ast.Expr{}
	for _, file := range files {
		for _, node := range file.Decls {
			function, ok := node.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if function.Recv != nil || function.Body == nil {
				continue
			}
			if function.Type.Params != nil && len(function.Type.Params.List) != 0 {
				continue
			}
			if len(function.Body.List) != 1 {
				continue
			}
			returned, ok := function.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(returned.Results) != 1 {
				continue
			}
			found[function.Name.Name] = returned.Results[0]
		}
	}
	return found
}

// resolve reads one Commands or Pipes expression into the identities it holds,
// with identity saying how one entry of that channel is named.
//
// It follows a call to a parameterless function in the same package ONE hop,
// which is the commandDecls() indirection both sides are meant to use. It does
// not follow a chain: `func f() []sdk.CommandDecl { return f() }` compiles, and
// a resolver that kept following would spin on it forever. A second hop buys
// nothing either, because a declaration written that way is one this gate
// reports as unreadable rather than one it resolves.
//
// An entry whose name it cannot read is recorded as unresolved rather than
// dropped.
func resolve(
	value ast.Expr,
	decls map[string]ast.Expr,
	fset *token.FileSet,
	identity func(entry *ast.CompositeLit) (string, bool),
) declaration {
	var found declaration
	if call, ok := value.(*ast.CallExpr); ok {
		callee, named := call.Fun.(*ast.Ident)
		if named && len(call.Args) == 0 {
			// Recorded whether or not the body is readable: the identity of
			// the function is what makes two readings one declaration.
			found.fromFunc = callee.Name
			if body, known := decls[callee.Name]; known {
				value = body
			}
		}
	}

	literal, ok := value.(*ast.CompositeLit)
	if !ok {
		found.unresolved = append(found.unresolved, where(value, fset))
		return found
	}
	for _, element := range literal.Elts {
		entry, ok := element.(*ast.CompositeLit)
		if !ok {
			found.unresolved = append(found.unresolved, where(element, fset))
			continue
		}
		text, readable := identity(entry)
		if !readable {
			found.unresolved = append(found.unresolved, where(entry, fset))
			continue
		}
		found.names = append(found.names, text)
	}
	return found
}

// commandName answers the command a Name field states. A quoted string answers
// its own value, and an identifier answers its spelling: both literals in one
// package name a constant the same way, so the spellings compare.
func commandName(value ast.Expr) (string, bool) {
	switch named := value.(type) {
	case *ast.BasicLit:
		if named.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(named.Value)
		if err != nil {
			return "", false
		}
		return text, true
	case *ast.Ident:
		return named.Name, true
	}
	return "", false
}

// where answers the file base name and line an expression was written at,
// which is what a reader needs to go and look at a declaration the gate could
// not read.
func where(node ast.Node, fset *token.FileSet) string {
	position := fset.Position(node.Pos())
	var tb textbuf.Buffer
	return tb.Str(filepath.Base(position.Filename)).Byte(':').Int(int64(position.Line)).String()
}

// relativeTo answers path relative to tree, and path itself when it sits
// outside tree. A gate reporting an absolute machine path is unreadable in a
// log, and stopping the whole run over a path format is worse than reporting
// the path whole.
func relativeTo(tree, path string) string {
	relative, err := filepath.Rel(tree, path)
	if err != nil {
		return path
	}
	return relative
}
