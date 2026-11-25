// Design: docs/contributing/navigating-the-code.md -- the symbol map of one Go file
// Related: actions.go -- the `le go symbols list` action that reads the file and renders this map
//
// The map is the one `gopls symbols` prints, built the way gopls builds its
// documentSymbol answer (golang.org/x/tools/gopls/internal/golang/symbols.go,
// DocumentSymbols) and printed the way its command line prints it
// (gopls/internal/cmd/symbols.go, printDocumentSymbol). gopls answers from the
// file's syntax alone, but its command line first loads the whole workspace,
// which costs seconds; this reads the one file.

package gosymbols

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Kind is the kind of one symbol, a member of the closed set of kinds gopls
// gives a Go declaration. The zero value names no kind.
type Kind uint8

const (
	// KindUnspecified is a Kind no symbol is built with.
	KindUnspecified Kind = iota
	// KindClass is a named type that is no struct, interface or function
	// type: a defined type over a basic, map, slice or pointer type, and an
	// alias of a named type.
	KindClass
	// KindConstant is a name a const declaration declares.
	KindConstant
	// KindField is a struct field, or a type an interface embeds.
	KindField
	// KindFunction is a function, or a named function type.
	KindFunction
	// KindInterface is a named interface type.
	KindInterface
	// KindMethod is a method declaration, or a method an interface declares.
	KindMethod
	// KindStruct is a named struct type.
	KindStruct
	// KindVariable is a name a var declaration declares.
	KindVariable
)

// String answers the word gopls prints for the kind: the name of the
// language server protocol's SymbolKind.
func (k Kind) String() string {
	switch k {
	case KindClass:
		return "Class"
	case KindConstant:
		return "Constant"
	case KindField:
		return "Field"
	case KindFunction:
		return "Function"
	case KindInterface:
		return "Interface"
	case KindMethod:
		return "Method"
	case KindStruct:
		return "Struct"
	case KindVariable:
		return "Variable"
	case KindUnspecified:
		panic("BUG: gosymbols: a symbol was built with no kind")
	default:
		panic("BUG: gosymbols: unknown symbol kind")
	}
}

// MarshalText renders the kind as the word gopls prints, so `| json` carries
// the same word the text line does.
func (k Kind) MarshalText() ([]byte, error) {
	return []byte(k.String()), nil
}

// Symbol is one declared name and the position of that name: 1-based lines,
// and 1-based columns counted in UTF-16 code units, the unit the language
// server protocol counts in and gopls prints.
type Symbol struct {
	Name        string `json:"name"`
	Kind        Kind   `json:"kind"`
	StartLine   int    `json:"start-line"`
	StartColumn int    `json:"start-column"`
	EndLine     int    `json:"end-line"`
	EndColumn   int    `json:"end-column"`
	// Members are the fields of a struct and the methods and embedded types
	// of an interface, sorted by name as gopls prints them. gopls prints one
	// level of them, so a member holds none.
	Members []Symbol `json:"members,omitempty"`
}

// List parses the Go source of one file and answers its symbols in
// declaration order. filename names the file in a parse error. A source that
// does not parse answers the parser's error: a symbol map of an incomplete
// syntax tree would omit or misplace declarations with nothing to say so.
func List(filename string, source []byte) ([]Symbol, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filename, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	at := positions{file: fileSet.File(file.Pos()), source: source}
	var symbols []Symbol
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Name.Name == "_" {
				continue
			}
			symbols = append(symbols, at.function(declaration))
		case *ast.GenDecl:
			symbols = append(symbols, at.general(declaration)...)
		}
	}
	return symbols, nil
}

// positions turns the positions of one parsed file into the lines and
// columns gopls prints.
type positions struct {
	file   *token.File
	source []byte
}

// function answers the symbol of a function or method declaration. A method
// is named by its receiver type as written, then its name: `(*List[T]).Push`.
func (at positions) function(declaration *ast.FuncDecl) Symbol {
	symbol := at.symbol(declaration.Name.Name, KindFunction, declaration.Name)
	if declaration.Recv != nil && len(declaration.Recv.List) > 0 {
		symbol.Kind = KindMethod
		symbol.Name = "(" + types.ExprString(declaration.Recv.List[0].Type) + ")." + declaration.Name.Name
	}
	return symbol
}

// general answers the symbols of an import, const, var or type declaration,
// skipping every name spelled `_`. An import declares no symbol.
func (at positions) general(declaration *ast.GenDecl) []Symbol {
	var symbols []Symbol
	for _, spec := range declaration.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			if spec.Name.Name == "_" {
				continue
			}
			kind, members := at.typeOf(spec.Type)
			symbol := at.symbol(spec.Name.Name, kind, spec.Name)
			symbol.Members = members
			symbols = append(symbols, symbol)
		case *ast.ValueSpec:
			kind := KindVariable
			if declaration.Tok == token.CONST {
				kind = KindConstant
			}
			for _, name := range spec.Names {
				if name.Name == "_" {
					continue
				}
				symbol := at.symbol(name.Name, kind, name)
				// A value's members are those of its written type, as for
				// `var x struct{ ... }`; an untyped value has none.
				if spec.Type != nil {
					_, symbol.Members = at.typeOf(spec.Type)
				}
				symbols = append(symbols, symbol)
			}
		}
	}
	return symbols
}

// typeOf answers the kind a type expression gives the name that declares it,
// and the members it holds.
func (at positions) typeOf(expression ast.Expr) (Kind, []Symbol) {
	switch expression := expression.(type) {
	case *ast.StructType:
		return KindStruct, at.members(expression.Fields, KindField)
	case *ast.InterfaceType:
		return KindInterface, at.members(expression.Methods, KindMethod)
	case *ast.FuncType:
		return KindFunction, nil
	default:
		// gopls's catch-all, for a type it cannot name from syntax alone.
		return KindClass, nil
	}
}

// members answers the members of a struct's fields or an interface's method
// list. A named member takes kind; an embedded one is a Field named by the
// type it embeds. Members named `_` are kept, as gopls keeps them.
func (at positions) members(fields *ast.FieldList, kind Kind) []Symbol {
	var symbols []Symbol
	for _, field := range fields.List {
		if len(field.Names) > 0 {
			for _, name := range field.Names {
				symbols = append(symbols, at.symbol(name.Name, kind, name))
			}
			continue
		}
		if name := embeddedName(field.Type); name != nil {
			symbols = append(symbols, at.symbol(name.Name, KindField, name))
			continue
		}
		// An embedded type with no name of its own, such as the union
		// `~int | ~string`, is named by its detail and spans the whole type.
		symbols = append(symbols, at.symbol(detail(field.Type), KindField, field.Type))
	}
	// gopls sorts members with sort.Slice; slices.SortFunc runs the same
	// pattern-defeating quicksort over the same comparisons, so members of the
	// same name, such as two `_` fields, keep the order gopls gives them.
	slices.SortFunc(symbols, func(left, right Symbol) int {
		return strings.Compare(left.Name, right.Name)
	})
	return symbols
}

// embeddedName answers the identifier that names an embedded type, through a
// pointer, a type instantiation and a package qualifier, or nil when the type
// has no such name.
func embeddedName(expression ast.Expr) *ast.Ident {
	if star, ok := expression.(*ast.StarExpr); ok {
		expression = star.X
	}
	switch instance := expression.(type) {
	case *ast.IndexExpr:
		expression = instance.X
	case *ast.IndexListExpr:
		expression = instance.X
	}
	switch named := expression.(type) {
	case *ast.Ident:
		return named
	case *ast.SelectorExpr:
		if _, qualified := named.X.(*ast.Ident); qualified {
			return named.Sel
		}
	}
	return nil
}

// detail answers the text gopls names an unnamed embedded type by: a struct
// or interface literal is abbreviated, any other type is written out.
func detail(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.StructType:
		if len(expression.Fields.List) > 0 {
			return "struct{...}"
		}
		return "struct{}"
	case *ast.InterfaceType:
		if len(expression.Methods.List) > 0 {
			return "interface{...}"
		}
		return "interface{}"
	default:
		return types.ExprString(expression)
	}
}

// symbol answers the symbol of name, positioned at the node that names it.
func (at positions) symbol(name string, kind Kind, node ast.Node) Symbol {
	startLine, startColumn := at.position(node.Pos())
	endLine, endColumn := at.position(node.End())
	return Symbol{
		Name: name, Kind: kind,
		StartLine: startLine, StartColumn: startColumn,
		EndLine: endLine, EndColumn: endColumn,
	}
}

// position answers the line and column of pos as gopls prints them. The line
// is the line in this file, never one a `//line` directive names, and the
// column counts the UTF-16 code units before pos on its line, plus one.
func (at positions) position(pos token.Pos) (line, column int) {
	line = at.file.PositionFor(pos, false).Line
	start := at.file.Offset(at.file.LineStart(line))
	units := 0
	for text := at.source[start:at.file.Offset(pos)]; len(text) > 0; {
		character, size := utf8.DecodeRune(text)
		// An invalid byte decodes to utf8.RuneError, one code unit, as
		// gopls counts it.
		units += utf16.RuneLen(character)
		text = text[size:]
	}
	return line, units + 1
}
