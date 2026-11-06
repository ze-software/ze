// Design: docs/architecture/plugin/feature-gates.md -- exact committed registration expressions.
// Related: source.go -- immutable build-context-selected packages and syntax.
package repoinventory

import (
	"fmt"
	"go/ast"
	"go/token"
	"path"
	"strconv"
	"strings"
)

// expression resolves only immutable declaration forms. The active declaration
// set and a depth bound prevent cycles or hostile source nesting from recursing
// without a limit; unsupported executable expressions are errors, not defaults.
type expression struct {
	snapshot        *Snapshot
	active          map[ast.Expr]bool
	depth           int
	mutations       *declarationMutationIndex
	mutationQueries map[declarationMutationQuery]declarationMutationResult
}

func (s *Snapshot) evaluator() *expression {
	return &expression{snapshot: s, active: make(map[ast.Expr]bool)}
}

func (e *expression) problem(source sourceExpr, reason string) error {
	position := e.snapshot.fset.Position(source.file.Pos())
	if source.value != nil {
		position = e.snapshot.fset.Position(source.value.Pos())
	}
	return fmt.Errorf("committed inventory %s:%d: %s; rewrite the registration as an immutable declaration before publishing inventory", position.Filename, position.Line, reason)
}

func (e *expression) target(source sourceExpr) (sourceExpr, error) {
	switch node := source.value.(type) {
	case *ast.ParenExpr:
		return sourceExpr{value: node.X, file: source.file, pkg: source.pkg}, nil
	case *ast.Ident:
		if node.Obj != nil {
			switch declaration := node.Obj.Decl.(type) {
			case *ast.AssignStmt:
				for index, left := range declaration.Lhs {
					name, ok := left.(*ast.Ident)
					if ok && name.Name == node.Name && index < len(declaration.Rhs) {
						return sourceExpr{value: declaration.Rhs[index], file: source.file, pkg: source.pkg, name: node.Name}, nil
					}
				}
			case *ast.ValueSpec:
				for index, name := range declaration.Names {
					if name.Name == node.Name && index < len(declaration.Values) {
						return sourceExpr{value: declaration.Values[index], file: source.file, pkg: source.pkg, name: node.Name}, nil
					}
				}
			}
			if binding := callablePackageBinding(source.pkg, node.Name); binding == node.Obj {
				if value, known := source.pkg.values[node.Name]; known {
					return value, nil
				}
			}
			return sourceExpr{}, e.problem(source, "local declaration ownership is unresolved")
		}
		if value, known := source.pkg.values[node.Name]; known {
			return value, nil
		}
	case *ast.SelectorExpr:
		dir, imported := e.callableImport(source, node)
		if !imported {
			return e.fieldTarget(source, node)
		}
		pkg, known := e.snapshot.packages[dir]
		if !known {
			break
		}
		if value, known := pkg.values[node.Sel.Name]; known {
			return value, nil
		}
	}
	return sourceExpr{}, e.problem(source, "cannot resolve declaration expression")
}

func (e *expression) function(source sourceExpr) (sourceFunction, bool) {
	if function, found := e.declaredFunction(source); found {
		return function, true
	}
	return e.method(source)
}

// declaredFunction excludes methods and lexical shadows. Constructor signatures
// cannot recursively infer an owner through a returned or dynamically bound callee.
func (e *expression) declaredFunction(source sourceExpr) (sourceFunction, bool) {
	switch node := source.value.(type) {
	case *ast.Ident:
		if node.Obj != nil {
			if node.Obj.Kind != ast.Fun {
				return sourceFunction{}, false
			}
		}
		function, found := source.pkg.functions[node.Name]
		return function, found
	case *ast.SelectorExpr:
		dir, imported := e.callableImport(source, node)
		if !imported {
			return sourceFunction{}, false
		}
		pkg, found := e.snapshot.packages[dir]
		if !found {
			return sourceFunction{}, false
		}
		function, found := pkg.functions[node.Sel.Name]
		return function, found
	}
	return sourceFunction{}, false
}

func (e *expression) enter(source sourceExpr) (func(), error) {
	if e.depth == 64 {
		return nil, e.problem(source, "registration expression exceeds 64 declaration steps")
	}
	if e.active[source.value] {
		return nil, e.problem(source, "registration expression contains a declaration cycle")
	}
	e.active[source.value] = true
	e.depth++
	return func() { delete(e.active, source.value); e.depth-- }, nil
}

// textOriginKind says where a resolved string's bytes are declared. The zero
// value is never produced by a resolution.
type textOriginKind uint8

const (
	textOriginUnspecified textOriginKind = iota
	// textOriginGo is a string declared in Go source: literals, their
	// concatenation, or a family name. It names no committed file.
	textOriginGo
	// textOriginEmbed is exactly the content of one //go:embed file.
	textOriginEmbed
	// textOriginMixed is a concatenation that includes an embedded file, so no
	// single committed file holds the string.
	textOriginMixed
)

// textOrigin is where a resolved string is declared. file is the snapshot
// path of the embedded file, set only for textOriginEmbed.
type textOrigin struct {
	kind textOriginKind
	file string
}

func (e *expression) text(source sourceExpr) (string, error) {
	value, _, err := e.textFrom(source)
	return value, err
}

// textFrom resolves a string and where it is declared, in one walk, so a
// caller that needs the embedded file never resolves the string twice.
func (e *expression) textFrom(source sourceExpr) (string, textOrigin, error) {
	if source.value == nil {
		return e.embeddedText(source)
	}
	leave, err := e.enter(source)
	if err != nil {
		return "", textOrigin{}, err
	}
	defer leave()
	switch node := source.value.(type) {
	case *ast.BasicLit:
		if node.Kind == token.STRING {
			value, err := strconv.Unquote(node.Value)
			if err != nil {
				return "", textOrigin{}, e.problem(source, "invalid string literal")
			}
			return value, textOrigin{kind: textOriginGo}, nil
		}
	case *ast.BinaryExpr:
		if node.Op == token.ADD {
			left, leftOrigin, err := e.textFrom(sourceExpr{value: node.X, file: source.file, pkg: source.pkg})
			if err != nil {
				return "", textOrigin{}, err
			}
			right, rightOrigin, err := e.textFrom(sourceExpr{value: node.Y, file: source.file, pkg: source.pkg})
			if err != nil {
				return "", textOrigin{}, err
			}
			origin := textOrigin{kind: textOriginMixed}
			if leftOrigin.kind == textOriginGo && rightOrigin.kind == textOriginGo {
				origin = textOrigin{kind: textOriginGo}
			}
			return left + right, origin, nil
		}
	case *ast.CallExpr:
		if method, ok := node.Fun.(*ast.SelectorExpr); ok {
			if method.Sel.Name == "String" && len(node.Args) == 0 {
				value, err := e.familyName(sourceExpr{value: method.X, file: source.file, pkg: source.pkg})
				return value, textOrigin{kind: textOriginGo}, err
			}
		}
		function, found := e.function(sourceExpr{value: node.Fun, file: source.file, pkg: source.pkg})
		if found && len(node.Args) == 0 && function.decl.Body != nil && len(function.decl.Body.List) == 1 {
			returned, ok := function.decl.Body.List[0].(*ast.ReturnStmt)
			if ok && len(returned.Results) == 1 {
				return e.textFrom(sourceExpr{value: returned.Results[0], file: function.file, pkg: function.pkg})
			}
		}
		return "", textOrigin{}, e.problem(source, "string value requires unsupported executable logic")
	case *ast.Ident, *ast.SelectorExpr, *ast.ParenExpr:
		if err := e.immutable(source); err != nil {
			return "", textOrigin{}, err
		}
		resolved, err := e.target(source)
		if err != nil {
			return "", textOrigin{}, err
		}
		return e.textFrom(resolved)
	}
	return "", textOrigin{}, e.problem(source, "unsupported string expression")
}

func (e *expression) texts(source sourceExpr) ([]string, error) {
	if source.value == nil {
		return nil, e.problem(source, "slice has no immutable initializer")
	}
	leave, err := e.enter(source)
	if err != nil {
		return nil, err
	}
	defer leave()
	switch node := source.value.(type) {
	case *ast.Ident:
		if predeclaredIdentifier(source, "nil") {
			return nil, nil
		}
	case *ast.CompositeLit:
		result := make([]string, 0, len(node.Elts))
		for _, element := range node.Elts {
			value, err := e.text(sourceExpr{value: element, file: source.file, pkg: source.pkg})
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		return result, nil
	case *ast.CallExpr:
		if len(node.Args) == 0 {
			function, found := e.function(sourceExpr{value: node.Fun, file: source.file, pkg: source.pkg})
			if found {
				return e.stringSliceHelper(function, source)
			}
		}
		callee := sourceExpr{value: node.Fun, file: source.file, pkg: source.pkg}
		if predeclaredIdentifier(callee, "append") && len(node.Args) > 0 {
			result, err := e.texts(sourceExpr{value: node.Args[0], file: source.file, pkg: source.pkg})
			if err != nil {
				return nil, err
			}
			for index, argument := range node.Args[1:] {
				if node.Ellipsis.IsValid() && index == len(node.Args)-2 {
					tail, err := e.texts(sourceExpr{value: argument, file: source.file, pkg: source.pkg})
					if err != nil {
						return nil, err
					}
					result = append(result, tail...)
					continue
				}
				value, err := e.text(sourceExpr{value: argument, file: source.file, pkg: source.pkg})
				if err != nil {
					return nil, err
				}
				result = append(result, value)
			}
			return result, nil
		}
		return nil, e.problem(source, "slice requires unsupported executable logic")
	}
	if err := e.immutable(source); err != nil {
		return nil, err
	}
	resolved, err := e.target(source)
	if err != nil {
		return nil, err
	}
	return e.texts(resolved)
}

func (e *expression) octets(source sourceExpr) ([]uint8, error) {
	if source.value == nil {
		return nil, e.problem(source, "capability slice has no initializer")
	}
	leave, err := e.enter(source)
	if err != nil {
		return nil, err
	}
	defer leave()
	literal, ok := source.value.(*ast.CompositeLit)
	if !ok {
		if err := e.immutable(source); err != nil {
			return nil, err
		}
		resolved, err := e.target(source)
		if err != nil {
			return nil, err
		}
		return e.octets(resolved)
	}
	result := make([]uint8, 0, len(literal.Elts))
	for _, element := range literal.Elts {
		value, err := e.integer(sourceExpr{value: element, file: source.file, pkg: source.pkg}, 0)
		if err != nil {
			return nil, err
		}
		if value > 255 {
			return nil, e.problem(source, "capability code exceeds one octet")
		}
		result = append(result, uint8(value))
	}
	return result, nil
}

func (e *expression) integer(source sourceExpr, depth int) (uint64, error) {
	if depth == 64 {
		return 0, e.problem(source, "numeric declaration contains a cycle or exceeds 64 steps")
	}
	literal, ok := source.value.(*ast.BasicLit)
	if ok && literal.Kind == token.INT {
		value, err := strconv.ParseUint(literal.Value, 0, 64)
		if err != nil {
			return 0, e.problem(source, "invalid unsigned numeric declaration")
		}
		return value, nil
	}
	if err := e.immutable(source); err != nil {
		return 0, err
	}
	resolved, err := e.target(source)
	if err != nil {
		return 0, err
	}
	return e.integer(resolved, depth+1)
}

func (e *expression) embeddedText(source sourceExpr) (string, textOrigin, error) {
	for _, declaration := range source.file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range group.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Values) != 0 {
				continue
			}
			matches := false
			for _, name := range value.Names {
				if name.Name == source.name {
					matches = true
				}
			}
			if !matches {
				continue
			}
			comments := value.Doc
			if comments == nil {
				comments = group.Doc
			}
			if comments == nil {
				continue
			}
			for _, comment := range comments.List {
				pattern, found := strings.CutPrefix(comment.Text, "//go:embed ")
				if !found {
					continue
				}
				fields := strings.Fields(pattern)
				if len(fields) != 1 || strings.ContainsAny(fields[0], "*?[\\\"") {
					return "", textOrigin{}, e.problem(source, "string embed requires one exact source filename")
				}
				name := path.Join(source.pkg.dir, fields[0])
				if !strings.HasPrefix(name, source.pkg.dir+"/") {
					return "", textOrigin{}, e.problem(source, "embedded schema escapes its package")
				}
				raw, found := e.snapshot.blobs[name]
				if !found {
					return "", textOrigin{}, e.problem(source, "embedded schema blob "+name+" is missing")
				}
				return string(raw), textOrigin{kind: textOriginEmbed, file: name}, nil
			}
		}
	}
	return "", textOrigin{}, e.problem(source, "string has no immutable initializer or exact embedded source")
}
