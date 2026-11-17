// Design: docs/architecture/plugin/feature-gates.md -- Go scopes over selected syntax only.
// Related: lexical.go -- immutable declaration and occurrence queries.
package repoinventory

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
)

type lexicalWalker struct {
	index                *lexicalIndex
	pkg                  *sourcePackage
	file                 *ast.File
	scope                *lexicalScope
	importName           func(string) (string, bool)
	unresolvedImports    bool
	unresolvedDotImports bool
}

func newLexicalScope(parent *lexicalScope) *lexicalScope {
	return &lexicalScope{parent: parent, names: make(map[string]*lexicalDeclaration)}
}

func (w lexicalWalker) nested() lexicalWalker {
	w.scope = newLexicalScope(w.scope)
	return w
}

func (w lexicalWalker) declare(name *ast.Ident, node ast.Node, kind lexicalKind) {
	if name.Name == "_" {
		return
	}
	declaration := &lexicalDeclaration{node: node, ident: name, file: w.file, pkg: w.pkg, kind: kind, scope: w.scope}
	if _, present := w.scope.names[name.Name]; present {
		w.scope.names[name.Name] = nil
	} else {
		w.scope.names[name.Name] = declaration
	}
	w.index.identifiers[name] = lexicalOccurrence{file: w.file, declaration: declaration, lexical: true}
}

func (w lexicalWalker) use(name *ast.Ident) {
	if name.Name == "_" {
		return
	}
	for scope := w.scope; scope != nil; scope = scope.parent {
		if declaration, present := scope.names[name.Name]; present {
			w.index.identifiers[name] = lexicalOccurrence{file: w.file, declaration: declaration, lexical: true}
			return
		}
	}
	if w.unresolvedImports {
		w.index.identifiers[name] = lexicalOccurrence{file: w.file, lexical: true}
		return
	}
	declaration := w.index.universe[name.Name]
	if declaration == nil {
		if object := types.Universe.Lookup(name.Name); object != nil {
			declaration = &lexicalDeclaration{kind: lexicalUniverse, universe: object}
			w.index.universe[name.Name] = declaration
		}
	}
	w.index.identifiers[name] = lexicalOccurrence{file: w.file, declaration: declaration, lexical: true}
}

func (w lexicalWalker) predeclare() {
	for _, declaration := range w.file.Decls {
		switch node := declaration.(type) {
		case *ast.FuncDecl:
			if node.Recv == nil {
				if node.Name.Name != "init" {
					w.declare(node.Name, node, lexicalFunction)
				}
			}
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				switch value := spec.(type) {
				case *ast.ValueSpec:
					kind := lexicalVariable
					if node.Tok == token.CONST {
						kind = lexicalConstant
					}
					for _, name := range value.Names {
						w.declare(name, value, kind)
					}
				case *ast.TypeSpec:
					w.declare(value.Name, value, lexicalType)
				}
			}
		}
	}
}

func (w lexicalWalker) fileDeclarations() {
	for _, imported := range w.file.Imports {
		if imported.Name != nil {
			if imported.Name.Name == "." {
				if w.unresolvedDotImports {
					w.unresolvedImports = true
				}
				continue
			}
			w.declare(imported.Name, imported, lexicalImport)
			continue
		}
		name, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			// A malformed import cannot establish ownership; its uses remain
			// unresolved. No alias is invented from an unreadable path.
			continue
		}
		if w.importName == nil {
			w.unresolvedImports = true
			continue
		}
		alias, known := w.importName(name)
		if !known {
			w.unresolvedImports = true
			continue
		}
		if alias == "" {
			w.unresolvedImports = true
			continue
		}
		// An implicit import has no defining Ident in the AST. Its ImportSpec
		// remains the identity; no fabricated identifier enters the index.
		declaration := &lexicalDeclaration{node: imported, file: w.file, pkg: w.pkg, kind: lexicalImport, scope: w.scope}
		if _, present := w.scope.names[alias]; present {
			w.scope.names[alias] = nil
		} else {
			w.scope.names[alias] = declaration
		}
	}
	for _, declaration := range w.file.Decls {
		switch node := declaration.(type) {
		case *ast.FuncDecl:
			w.function(node.Type, node.Recv, node.Body)
		case *ast.GenDecl:
			w.declarations(node, true)
		}
	}
}

func (w lexicalWalker) typeParameters(parameters *ast.FieldList) {
	if parameters == nil {
		return
	}
	for _, field := range parameters.List {
		for _, name := range field.Names {
			w.declare(name, field, lexicalType)
		}
	}
	for _, field := range parameters.List {
		w.walk(field.Type)
	}
}

func (w lexicalWalker) declarations(group *ast.GenDecl, packageLevel bool) {
	for _, spec := range group.Specs {
		switch node := spec.(type) {
		case *ast.ValueSpec:
			w.walk(node.Type)
			for _, value := range node.Values {
				w.walk(value)
			}
			if !packageLevel {
				kind := lexicalVariable
				if group.Tok == token.CONST {
					kind = lexicalConstant
				}
				for _, name := range node.Names {
					w.declare(name, node, kind)
				}
			}
		case *ast.TypeSpec:
			if !packageLevel {
				w.declare(node.Name, node, lexicalType)
			}
			typed := w.nested()
			typed.typeParameters(node.TypeParams)
			typed.walk(node.Type)
		}
	}
}

func (w lexicalWalker) fields(fields *ast.FieldList, declare bool) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		if declare {
			for _, name := range field.Names {
				w.declare(name, field, lexicalVariable)
			}
		} else {
			w.walk(field.Type)
		}
	}
}

func (w lexicalWalker) receiverParameters(receiver *ast.FieldList) {
	if receiver == nil {
		return
	}
	for _, field := range receiver.List {
		base := field.Type
		if pointer, indirect := base.(*ast.StarExpr); indirect {
			base = pointer.X
		}
		var arguments []ast.Expr
		switch indexed := base.(type) {
		case *ast.IndexExpr:
			arguments = []ast.Expr{indexed.Index}
		case *ast.IndexListExpr:
			arguments = indexed.Indices
		}
		for _, argument := range arguments {
			if name, named := argument.(*ast.Ident); named {
				w.declare(name, field, lexicalType)
			}
		}
	}
}

func (w lexicalWalker) function(signature *ast.FuncType, receiver *ast.FieldList, body *ast.BlockStmt) {
	function := w.nested()
	function.receiverParameters(receiver)
	function.typeParameters(signature.TypeParams)
	function.fields(receiver, false)
	function.fields(signature.Params, false)
	function.fields(signature.Results, false)
	function.fields(receiver, true)
	function.fields(signature.Params, true)
	function.fields(signature.Results, true)
	if body != nil {
		function.statements(body.List)
	}
}

func (w lexicalWalker) statements(statements []ast.Stmt) {
	for _, statement := range statements {
		w.walk(statement)
	}
}

func (w lexicalWalker) assignment(assignment *ast.AssignStmt) {
	for _, right := range assignment.Rhs {
		w.walk(right)
	}
	for _, left := range assignment.Lhs {
		if assignment.Tok == token.DEFINE {
			if name, named := left.(*ast.Ident); named {
				if _, present := w.scope.names[name.Name]; !present {
					w.declare(name, assignment, lexicalVariable)
					continue
				}
			}
		}
		w.walk(left)
	}
}

func (w lexicalWalker) typeSwitch(statement *ast.TypeSwitchStmt) {
	switchScope := w.nested()
	switchScope.walk(statement.Init)
	var guard *ast.Ident
	if assignment, assigned := statement.Assign.(*ast.AssignStmt); assigned {
		for _, right := range assignment.Rhs {
			switchScope.walk(right)
		}
		if assignment.Tok == token.DEFINE {
			if len(assignment.Lhs) == 1 {
				guard, _ = assignment.Lhs[0].(*ast.Ident)
			}
		} else {
			for _, left := range assignment.Lhs {
				switchScope.walk(left)
			}
		}
	} else {
		switchScope.walk(statement.Assign)
	}
	for _, clause := range statement.Body.List {
		if arm, matched := clause.(*ast.CaseClause); matched {
			for _, datatype := range arm.List {
				switchScope.walk(datatype)
			}
			branch := switchScope.nested()
			if guard != nil {
				branch.declare(guard, statement.Assign, lexicalVariable)
			}
			branch.statements(arm.Body)
		}
	}
	if guard != nil {
		// The shared guard syntax does not select one case's declaration.
		// Each case use already holds that case's distinct identity.
		w.index.identifiers[guard] = lexicalOccurrence{file: w.file, lexical: true}
	}
}

// walk follows the retained AST recursively; ParseFile's source nesting bound
// bounds this traversal. Only lexical names reach use, including map keys until
// the completed index classifies composite keys.
func (w lexicalWalker) walk(node ast.Node) {
	if node == nil {
		return
	}
	switch value := node.(type) {
	case *ast.Ident:
		w.use(value)
	case *ast.BasicLit, *ast.BadExpr, *ast.BadStmt, *ast.EmptyStmt, *ast.BranchStmt:
		// Literal text, malformed syntax and labels own no lexical values.
	case *ast.SelectorExpr:
		w.walk(value.X)
	case *ast.ParenExpr:
		w.walk(value.X)
	case *ast.StarExpr:
		w.walk(value.X)
	case *ast.UnaryExpr:
		w.walk(value.X)
	case *ast.BinaryExpr:
		w.walk(value.X)
		w.walk(value.Y)
	case *ast.CallExpr:
		w.walk(value.Fun)
		for _, argument := range value.Args {
			w.walk(argument)
		}
	case *ast.IndexExpr:
		w.walk(value.X)
		w.walk(value.Index)
	case *ast.IndexListExpr:
		w.walk(value.X)
		for _, index := range value.Indices {
			w.walk(index)
		}
	case *ast.SliceExpr:
		w.walk(value.X)
		w.walk(value.Low)
		w.walk(value.High)
		w.walk(value.Max)
	case *ast.TypeAssertExpr:
		w.walk(value.X)
		w.walk(value.Type)
	case *ast.CompositeLit:
		w.walk(value.Type)
		for _, element := range value.Elts {
			w.walk(element)
		}
	case *ast.KeyValueExpr:
		w.walk(value.Key)
		w.walk(value.Value)
	case *ast.ArrayType:
		w.walk(value.Len)
		w.walk(value.Elt)
	case *ast.MapType:
		w.walk(value.Key)
		w.walk(value.Value)
	case *ast.ChanType:
		w.walk(value.Value)
	case *ast.Ellipsis:
		w.walk(value.Elt)
	case *ast.StructType:
		w.fields(value.Fields, false)
	case *ast.InterfaceType:
		w.fields(value.Methods, false)
	case *ast.FuncType:
		w.function(value, nil, nil)
	case *ast.FuncLit:
		w.function(value.Type, nil, value.Body)
	case *ast.BlockStmt:
		w.nested().statements(value.List)
	case *ast.DeclStmt:
		if group, declared := value.Decl.(*ast.GenDecl); declared {
			w.declarations(group, false)
		}
	case *ast.AssignStmt:
		w.assignment(value)
	case *ast.ExprStmt:
		w.walk(value.X)
	case *ast.IncDecStmt:
		w.walk(value.X)
	case *ast.ReturnStmt:
		for _, result := range value.Results {
			w.walk(result)
		}
	case *ast.SendStmt:
		w.walk(value.Chan)
		w.walk(value.Value)
	case *ast.GoStmt:
		w.walk(value.Call)
	case *ast.DeferStmt:
		w.walk(value.Call)
	case *ast.LabeledStmt:
		w.walk(value.Stmt)
	case *ast.IfStmt:
		branch := w.nested()
		branch.walk(value.Init)
		branch.walk(value.Cond)
		branch.walk(value.Body)
		branch.walk(value.Else)
	case *ast.ForStmt:
		loop := w.nested()
		loop.walk(value.Init)
		loop.walk(value.Cond)
		loop.walk(value.Post)
		loop.walk(value.Body)
	case *ast.RangeStmt:
		w.walk(value.X)
		loop := w.nested()
		for _, operand := range []ast.Expr{value.Key, value.Value} {
			if operand == nil {
				continue
			}
			if value.Tok == token.DEFINE {
				if name, named := operand.(*ast.Ident); named {
					loop.declare(name, value, lexicalVariable)
					continue
				}
			}
			loop.walk(operand)
		}
		loop.walk(value.Body)
	case *ast.SwitchStmt:
		branch := w.nested()
		branch.walk(value.Init)
		branch.walk(value.Tag)
		for _, arm := range value.Body.List {
			branch.walk(arm)
		}
	case *ast.TypeSwitchStmt:
		w.typeSwitch(value)
	case *ast.CaseClause:
		for _, expression := range value.List {
			w.walk(expression)
		}
		w.nested().statements(value.Body)
	case *ast.SelectStmt:
		for _, clause := range value.Body.List {
			w.walk(clause)
		}
	case *ast.CommClause:
		branch := w.nested()
		branch.walk(value.Comm)
		branch.statements(value.Body)
	}
}
