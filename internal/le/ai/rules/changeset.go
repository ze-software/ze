// Design: docs/contributing/rule-authoring.md -- binding a check to a point
// Overview: coverage.go -- the gate map these bindings join
// Related: hooktable.go -- the published tables, which judge hook bindings only
//
// changeset.go reads the second roster the gate map joins: the change-set
// checks `./le commit create` runs (internal/le/commit/changecheck). A check
// registers from its own package's init(), so the roster is discovered from
// the changecheck.Register calls in the source and is listed nowhere here.
//
// A change-set check is its package, not one function: its Judge and the text
// it refuses with sit in different functions. So every binding comment in a
// non-test Go file of that package binds the check's registered name, which
// is the name a refusal at commit time prints. A binding in a package under
// internal/le that registers no check sits above no check and dangles, so no
// binding comment is text that no gate reads.

package airules

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// changeSetRel is the tree the discovery walks. A tool le runs is a
	// package under internal/le (ai/doors/le-tooling.md, "Areas register").
	changeSetRel = "internal/le"
	// changeSetImport is the seam a change-set check registers into.
	changeSetImport = "github.com/ze-software/ze/internal/le/commit/changecheck"
	// changeSetPackage and changeSetRegister name the registering call when
	// the import is not renamed.
	changeSetPackage  = "changecheck"
	changeSetRegister = "Register"
	// bindingMarker is the text every binding comment carries. A file without
	// it holds no binding, so only files holding it are scanned line by line.
	bindingMarker = "ze point:"
)

// changeSetSources answers the non-test Go sources of every package under
// internal/le that calls the changecheck seam's Register or carries a binding
// comment, keyed by root-relative slash path.
//
// hookruntime is left out: its bindings belong to the native hook join. A
// directory Go itself ignores (testdata, a dot or underscore prefix) is left
// out too, because no check is built from it.
func changeSetSources(root string) (map[string]string, []string) {
	base := filepath.Join(root, filepath.FromSlash(changeSetRel))
	files := map[string][]string{}
	walked := filepath.WalkDir(base, func(at string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if at == base {
				return nil
			}
			if goIgnoresDir(name) {
				return filepath.SkipDir
			}
			if relTo(root, at) == hookRuntimeRel {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		if strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel := relTo(root, at)
		files[path.Dir(rel)] = append(files[path.Dir(rel)], rel)
		return nil
	})
	var tb textbuf.Buffer
	if walked != nil {
		return nil, []string{tb.Str(changeSetRel).Str(": cannot be walked (").Err(walked).
			Str("); the change-set check roster is unknown").String()}
	}

	sources := map[string]string{}
	var problems []string
	for _, dir := range sortedKeys(files) {
		texts := map[string]string{}
		marked := false
		for _, rel := range files[dir] {
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- a path the walk of the checkout found
			if err != nil {
				tb.Reset()
				problems = append(problems, tb.Str(rel).Str(": cannot be read: ").Err(err).String())
				continue
			}
			texts[rel] = string(body)
			if registersCheck(texts[rel]) || holdsBinding(texts[rel]) {
				marked = true
			}
		}
		if marked {
			maps.Copy(sources, texts)
		}
	}
	return sources, problems
}

// goIgnoresDir reports whether the go tool builds nothing from a directory of
// this name, so no check le runs can be registered there.
func goIgnoresDir(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// registersCheck reports whether a Go source can hold a Register call: it
// imports the seam and calls a Register under some name. The commit code
// imports the seam only to run the checks, so its package, which registers
// none, is not read. readChangeSetPackage decides from the syntax.
func registersCheck(text string) bool {
	if !strings.Contains(text, changeSetImport) {
		return false
	}
	return strings.Contains(text, "."+changeSetRegister+"(")
}

// holdsBinding reports whether a Go source has a line the binding pattern
// matches. A source that only mentions the marker, as this package's own
// comments do, is not one.
func holdsBinding(text string) bool {
	if !strings.Contains(text, bindingMarker) {
		return false
	}
	for line := range strings.SplitSeq(text, "\n") {
		if bindingLine.MatchString(line) {
			return true
		}
	}
	return false
}

// changeSetRoster is the change-set checks a set of sources registers and the
// bindings their packages carry.
type changeSetRoster struct {
	bindings []Binding
	// checks maps a registered check name to the file and line of its
	// Register call.
	checks map[string]string
}

// unbound answers a refusal for each registered check whose package carries
// no binding comment, the same refusal a native hook check gets.
func (r changeSetRoster) unbound() []string {
	bound := map[string]bool{}
	for _, binding := range r.bindings {
		bound[binding.Check] = true
	}
	var problems []string
	var tb textbuf.Buffer
	for _, check := range sortedKeys(r.checks) {
		if bound[check] {
			continue
		}
		tb.Reset()
		problems = append(problems, tb.Str(r.checks[check]).Str(": registered change-set check ").
			Str(check).Str(" has no `// ze point:` binding in its package").String())
	}
	return problems
}

// readChangeSetRoster parses the sources package by package and binds each
// package's binding comments to the check it registers.
//
// A name that is not a constant string, two checks in one package, and one
// name in two packages are each refused: the attribution would be a guess.
func readChangeSetRoster(sources map[string]string) (changeSetRoster, []string) {
	packages := map[string][]string{}
	for rel := range sources {
		packages[path.Dir(rel)] = append(packages[path.Dir(rel)], rel)
	}
	roster := changeSetRoster{checks: map[string]string{}}
	var problems []string
	var tb textbuf.Buffer
	for _, dir := range sortedKeys(packages) {
		names := packages[dir]
		slices.Sort(names)
		check, bindings, found := readChangeSetPackage(dir, names, sources)
		problems = append(problems, found...)
		if len(found) > 0 {
			continue
		}
		if check.name != noCheck {
			if previous, twice := roster.checks[check.name]; twice {
				tb.Reset()
				problems = append(problems, tb.Str(check.at).Str(": change-set check ").Str(check.name).
					Str(" is also registered in ").Str(previous).String())
				continue
			}
			roster.checks[check.name] = check.at
		}
		roster.bindings = append(roster.bindings, bindings...)
	}
	return roster, problems
}

// registration is one changecheck.Register call: the check name it registers
// and where the call sits.
type registration struct {
	name string
	at   string
}

// readChangeSetPackage answers the check one package registers, noCheck when
// it registers none, and every binding comment in the package attached to it.
func readChangeSetPackage(dir string, names []string, sources map[string]string) (registration, []Binding, []string) {
	files := token.NewFileSet()
	trees := make([]*ast.File, 0, len(names))
	var problems []string
	var tb textbuf.Buffer
	for _, rel := range names {
		tree, err := parser.ParseFile(files, rel, sources[rel], parser.ParseComments)
		if err != nil {
			tb.Reset()
			problems = append(problems, tb.Str(rel).Str(": cannot be parsed: ").Err(err).String())
			continue
		}
		trees = append(trees, tree)
	}
	if len(problems) > 0 {
		return registration{}, nil, problems
	}

	consts := packageConsts(trees)
	var registered []registration
	for index, tree := range trees {
		found, refused := registrations(tree, names[index], files, consts)
		registered = append(registered, found...)
		problems = append(problems, refused...)
	}
	if len(registered) > 1 {
		tb.Reset()
		problems = append(problems, tb.Str(dir).Str(": registers ").Int(int64(len(registered))).
			Str(" change-set checks; a binding there cannot say which one it gates").String())
	}
	if len(problems) > 0 {
		return registration{}, nil, problems
	}

	check := registration{name: noCheck}
	if len(registered) == 1 {
		check = registered[0]
	}
	var bindings []Binding
	for index, tree := range trees {
		for _, group := range tree.Comments {
			for _, comment := range group.List {
				if binding, ok := bindingComment(comment.Text, names[index],
					files.Position(comment.Slash).Line, check.name); ok {
					bindings = append(bindings, binding)
				}
			}
		}
	}
	return check, bindings, nil
}

// packageConsts answers each package-level constant that has its own value
// expression. A constant repeating the previous line's expression (an iota
// run) has none, and no check name is spelled that way.
func packageConsts(trees []*ast.File) map[string]ast.Expr {
	consts := map[string]ast.Expr{}
	for _, tree := range trees {
		for _, declaration := range tree.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			if generic.Tok != token.CONST {
				continue
			}
			for _, raw := range generic.Specs {
				spec, ok := raw.(*ast.ValueSpec)
				if ok && len(spec.Values) == len(spec.Names) {
					for index, name := range spec.Names {
						consts[name.Name] = spec.Values[index]
					}
				}
			}
		}
	}
	return consts
}

// registrations answers the changecheck.Register calls in one file, under the
// name the file imports the seam as.
func registrations(tree *ast.File, rel string, files *token.FileSet, consts map[string]ast.Expr) ([]registration, []string) {
	local := ""
	for _, spec := range tree.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if imported != changeSetImport {
			continue
		}
		local = changeSetPackage
		if spec.Name != nil {
			local = spec.Name.Name
		}
	}
	var tb textbuf.Buffer
	if local == "." {
		tb.Str(rel).Str(": a dot import of ").Str(changeSetImport).
			Str(" hides its Register call from the gate map; import it by name")
		return nil, []string{tb.String()}
	}
	// A file that does not import the seam, or imports it blank, can call no
	// Register: it registers nothing, which is an answer rather than a miss.
	if local == "" {
		return nil, nil
	}
	if local == "_" {
		return nil, nil
	}

	var found []registration
	var problems []string
	ast.Inspect(tree, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, named := selector.X.(*ast.Ident)
		if !named {
			return true
		}
		if receiver.Name != local {
			return true
		}
		if selector.Sel.Name != changeSetRegister {
			return true
		}
		tb.Reset()
		at := tb.Str(rel).Byte(':').Int(int64(files.Position(call.Pos()).Line)).String()
		name, ok := registeredName(call, consts)
		if !ok {
			tb.Reset()
			problems = append(problems, tb.Str(at).Str(": the change-set check name is not a constant string; ").
				Str("spell Name as a literal or a package constant so the gate map can name the check").String())
			return true
		}
		found = append(found, registration{name: name, at: at})
		return true
	})
	return found, problems
}

// registeredName answers the Name field of the Check literal a Register call
// passes, when it is a constant string.
func registeredName(call *ast.CallExpr, consts map[string]ast.Expr) (string, bool) {
	if len(call.Args) != 1 {
		return "", false
	}
	literal, ok := call.Args[0].(*ast.CompositeLit)
	if !ok {
		return "", false
	}
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, named := field.Key.(*ast.Ident); named && key.Name == "Name" {
			return constString(field.Value, consts, len(consts))
		}
	}
	return "", false
}

// constString answers the value of a constant string expression: a literal,
// a package constant, parentheses, or a `+` of those.
//
// The recursion is bounded by depth, which starts at the package's constant
// count: a chain longer than that names a constant twice, which only a cycle
// Go would refuse to compile can do.
func constString(expression ast.Expr, consts map[string]ast.Expr, depth int) (string, bool) {
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(value.Value)
		return text, err == nil
	case *ast.Ident:
		named, ok := consts[value.Name]
		if !ok {
			return "", false
		}
		if depth <= 0 {
			return "", false
		}
		return constString(named, consts, depth-1)
	case *ast.ParenExpr:
		return constString(value.X, consts, depth)
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", false
		}
		left, ok := constString(value.X, consts, depth)
		if !ok {
			return "", false
		}
		right, ok := constString(value.Y, consts, depth)
		if !ok {
			return "", false
		}
		var tb textbuf.Buffer
		return tb.Str(left).Str(right).String(), true
	}
	return "", false
}
