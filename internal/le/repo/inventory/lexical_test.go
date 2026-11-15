// Design: docs/architecture/plugin/feature-gates.md -- retained lexical identities follow Go scopes.
package repoinventory

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"
	"strings"
	"testing"
)

const lexicalDirectory = "internal/lexical"
const lexicalFile = lexicalDirectory + "/bindings.go"

// lexicalSnapshot selects real retained syntax using the test constructor that
// mirrors ReadCommitted's final lexical-index construction.
func lexicalSnapshot(t *testing.T, sources map[string]string) *Snapshot {
	t.Helper()
	return selectedSnapshot(t, lexicalDirectory, sources)
}

// lexicalIdent identifies authored syntax by a unique comment and its spelling.
// ordinal distinguishes a declaration and its RHS use on the same source line.
func lexicalIdent(t *testing.T, snapshot *Snapshot, file, marker, name string, ordinal int) (*ast.File, *ast.Ident) {
	t.Helper()
	line := 0
	for index, text := range strings.Split(string(snapshot.blobs[file]), "\n") {
		_, comment, present := strings.Cut(text, "//")
		if !present {
			continue
		}
		if strings.TrimSpace(comment) == marker {
			if line != 0 {
				t.Fatalf("marker %q appears twice in %s", marker, file)
			}
			line = index + 1
		}
	}
	if line == 0 {
		t.Fatalf("marker %q is missing in %s", marker, file)
	}
	for _, source := range snapshot.files {
		if source.Path != file {
			continue
		}
		var identifiers []*ast.Ident
		ast.Inspect(source.File, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			if identifier.Name != name {
				return true
			}
			if snapshot.fset.Position(identifier.Pos()).Line == line {
				identifiers = append(identifiers, identifier)
			}
			return true
		})
		slices.SortFunc(identifiers, func(a, b *ast.Ident) int { return int(a.Pos() - b.Pos()) })
		if ordinal < len(identifiers) {
			return source.File, identifiers[ordinal]
		}
		t.Fatalf("marker %q has %d identifiers named %s, need position %d", marker, len(identifiers), name, ordinal)
	}
	t.Fatalf("retained file %s is missing", file)
	return nil, nil
}

func lexicalBindingAt(t *testing.T, snapshot *Snapshot, file, marker, name string, ordinal int) *lexicalDeclaration {
	t.Helper()
	owner, identifier := lexicalIdent(t, snapshot, file, marker, name, ordinal)
	declaration, known := snapshot.binding(owner, identifier)
	if !known {
		t.Fatalf("%s at %s has unresolved lexical ownership", name, marker)
	}
	return declaration
}

func lexicalSame(t *testing.T, snapshot *Snapshot, use, declaration, name string) {
	t.Helper()
	if lexicalBindingAt(t, snapshot, lexicalFile, use, name, 0) != lexicalBindingAt(t, snapshot, lexicalFile, declaration, name, 0) {
		t.Fatalf("%s at %s does not resolve to %s", name, use, declaration)
	}
}

func lexicalUnknown(t *testing.T, snapshot *Snapshot, marker, name string) {
	t.Helper()
	file, identifier := lexicalIdent(t, snapshot, lexicalFile, marker, name, 0)
	if _, known := snapshot.binding(file, identifier); known {
		t.Fatalf("%s at %s acquired an unsupported lexical identity", name, marker)
	}
}

// TestLexicalBindings proves declaration identity through package, file, block,
// closure and control scopes by comparing the declarations each use reaches.
func TestLexicalBindings(t *testing.T) {
	snapshot := lexicalSnapshot(t, map[string]string{
		lexicalFile: `package lexical
import alias "os" // import-one
var global, sibling = 1, 2 // package-names
type Duplicate int // duplicate-one
type Duplicate string // duplicate-two
func outer() {
 _ = global // cross-file-back
 _ = alias.Environ // import-use
 local := 1 // local-declaration
 local, fresh := local, 2 // same-block
 _, _ = local, fresh // same-block-use
 { local := 3; _ = local } // inner-shadow
 _ = func() int { return local } // closure-use
 if branch := local; branch > 0 { _ = branch // if-then
 } else { _ = branch } // if-else
 _ = branch // if-ended
 for loop := 0; loop < 1; loop++ { _ = loop } // for-scope
 _ = loop // for-ended
 switch arm := local; arm { case 1: _ = arm // switch-one
 case 2: arm := 3; _ = arm } // switch-two
 _ = arm // switch-ended
 select { case received := <-make(chan int): _ = received // select-one
 default: _ = received } // select-other
 _ = received // select-ended
 _ = Duplicate // ambiguous-use
}
`,
		lexicalDirectory + "/other.go": `package lexical
import alias "strings" // import-two
var other = global // cross-file-use
func imported() { _ = alias.Clone } // other-import-use
`,
	})
	for _, row := range []struct{ use, declaration, name string }{
		{"cross-file-back", "package-names", "global"},
		{"import-use", "import-one", "alias"},
		{"same-block", "local-declaration", "local"},
		{"same-block-use", "same-block", "fresh"},
		{"closure-use", "local-declaration", "local"},
		{"if-else", "if-then", "branch"},
	} {
		lexicalSame(t, snapshot, row.use, row.declaration, row.name)
	}
	for _, row := range []struct{ marker, name string }{
		{"if-ended", "branch"}, {"for-ended", "loop"}, {"switch-ended", "arm"},
		{"select-other", "received"}, {"select-ended", "received"}, {"ambiguous-use", "Duplicate"},
	} {
		lexicalUnknown(t, snapshot, row.marker, row.name)
	}
	global := lexicalBindingAt(t, snapshot, lexicalFile, "package-names", "global", 0)
	sibling := lexicalBindingAt(t, snapshot, lexicalFile, "package-names", "sibling", 0)
	if global == sibling {
		t.Fatal("separate names in one ValueSpec share an identity")
	}
	otherFile := lexicalDirectory + "/other.go"
	if global != lexicalBindingAt(t, snapshot, otherFile, "cross-file-use", "global", 0) {
		t.Fatal("cross-file package use lost its declaration")
	}
	if lexicalBindingAt(t, snapshot, lexicalFile, "import-one", "alias", 0) == lexicalBindingAt(t, snapshot, otherFile, "import-two", "alias", 0) {
		t.Fatal("per-file imports share an identity")
	}
	if lexicalBindingAt(t, snapshot, lexicalFile, "inner-shadow", "local", 0) == lexicalBindingAt(t, snapshot, lexicalFile, "local-declaration", "local", 0) {
		t.Fatal("inner short declaration reused an outer identity")
	}
	lexicalBindingAt(t, snapshot, lexicalFile, "for-scope", "loop", 1)
	lexicalBindingAt(t, snapshot, lexicalFile, "select-one", "received", 1)
}

// TestLexicalDeclarationOrder proves source-position binding rather than name
// lookup by placing shadows in initializers, signatures and generic constraints.
func TestLexicalDeclarationOrder(t *testing.T) {
	snapshot := lexicalSnapshot(t, map[string]string{lexicalFile: `package lexical
type Thing struct{} // package-type
type Pair[A interface{ ~[]B }, B any] struct{} // type-parameters
func (receiver Pair[T, U]) Method(value T) U { var result U; return result } // generic-receiver
var value = 1 // package-value
func signature(Thing Thing) (result Thing) { _ = Thing; return } // signature-types
func order() {
 var value = value // local-var
 { const value = value // local-constant
 _ = value } // const-use
 type Recursive struct { Next *Recursive } // recursive-type
 _ = value // local-use
}
`})
	packageValue := lexicalBindingAt(t, snapshot, lexicalFile, "package-value", "value", 0)
	localValue := lexicalBindingAt(t, snapshot, lexicalFile, "local-var", "value", 0)
	if lexicalBindingAt(t, snapshot, lexicalFile, "local-var", "value", 1) != packageValue {
		t.Fatal("local var initializer saw its own new binding")
	}
	if lexicalBindingAt(t, snapshot, lexicalFile, "local-constant", "value", 1) != localValue {
		t.Fatal("local constant initializer saw its own new binding")
	}
	lexicalSame(t, snapshot, "local-use", "local-var", "value")
	lexicalSame(t, snapshot, "const-use", "local-constant", "value")
	if lexicalBindingAt(t, snapshot, lexicalFile, "recursive-type", "Recursive", 0) != lexicalBindingAt(t, snapshot, lexicalFile, "recursive-type", "Recursive", 1) {
		t.Fatal("local type was unavailable inside its underlying type")
	}
	packageType := lexicalBindingAt(t, snapshot, lexicalFile, "package-type", "Thing", 0)
	for _, ordinal := range []int{1, 2} {
		if lexicalBindingAt(t, snapshot, lexicalFile, "signature-types", "Thing", ordinal) != packageType {
			t.Fatal("parameter name shadowed a signature type")
		}
	}
	if lexicalBindingAt(t, snapshot, lexicalFile, "signature-types", "Thing", 0) != lexicalBindingAt(t, snapshot, lexicalFile, "signature-types", "Thing", 3) {
		t.Fatal("parameter did not enter the function body scope")
	}
	if lexicalBindingAt(t, snapshot, lexicalFile, "type-parameters", "B", 0) != lexicalBindingAt(t, snapshot, lexicalFile, "type-parameters", "B", 1) {
		t.Fatal("constraint could not see a later type parameter")
	}
	if lexicalBindingAt(t, snapshot, lexicalFile, "generic-receiver", "T", 0) != lexicalBindingAt(t, snapshot, lexicalFile, "generic-receiver", "T", 1) {
		t.Fatal("generic receiver parameter did not reach method signature")
	}
	if lexicalBindingAt(t, snapshot, lexicalFile, "generic-receiver", "U", 0) != lexicalBindingAt(t, snapshot, lexicalFile, "generic-receiver", "U", 2) {
		t.Fatal("generic receiver parameter did not reach method body")
	}
}

// TestLexicalRangeAndTypeSwitch proves the new index retains real range nodes
// and gives each type-switch case its own guard declaration.
func TestLexicalRangeAndTypeSwitch(t *testing.T) {
	snapshot := lexicalSnapshot(t, map[string]string{lexicalFile: `package lexical
var item = []int{1} // collection
type Choice int // case-type
func controls(input any) {
 for item := range item { // range-defining
  item := item // range-body
  _ = item // range-body-use
 }
 existing := 0 // existing
 for existing = range item { _ = existing } // range-assigning
 switch Choice := input.(type) { // type-guard
 case Choice: _ = Choice // type-case-one
 case string: _ = Choice // type-case-two
 default: _ = Choice // type-case-default
 }
}
`})
	if lexicalBindingAt(t, snapshot, lexicalFile, "range-defining", "item", 1) != lexicalBindingAt(t, snapshot, lexicalFile, "collection", "item", 0) {
		t.Fatal("range collection resolved against the iteration binding")
	}
	ranged := lexicalBindingAt(t, snapshot, lexicalFile, "range-defining", "item", 0)
	if _, ok := ranged.node.(*ast.RangeStmt); !ok {
		t.Fatalf("defining range retained %T instead of its real RangeStmt", ranged.node)
	}
	if lexicalBindingAt(t, snapshot, lexicalFile, "range-body", "item", 1) != ranged {
		t.Fatal("range body RHS lost its iteration binding")
	}
	if lexicalBindingAt(t, snapshot, lexicalFile, "range-body", "item", 0) == ranged {
		t.Fatal("range body and iteration clause share one block")
	}
	lexicalSame(t, snapshot, "range-body-use", "range-body", "item")
	lexicalSame(t, snapshot, "range-assigning", "existing", "existing")
	if lexicalBindingAt(t, snapshot, lexicalFile, "type-case-one", "Choice", 0) != lexicalBindingAt(t, snapshot, lexicalFile, "case-type", "Choice", 0) {
		t.Fatal("type-switch guard shadowed its case type")
	}
	first := lexicalBindingAt(t, snapshot, lexicalFile, "type-case-one", "Choice", 1)
	second := lexicalBindingAt(t, snapshot, lexicalFile, "type-case-two", "Choice", 0)
	last := lexicalBindingAt(t, snapshot, lexicalFile, "type-case-default", "Choice", 0)
	if first == second {
		t.Fatal("different type-switch cases share a guard identity")
	}
	if second == last {
		t.Fatal("default case reused another case guard identity")
	}
}

// TestLexicalNonValueNames proves map keys remain expressions while selector,
// field and label spellings cannot manufacture lexical value ownership.
func TestLexicalNonValueNames(t *testing.T) {
	snapshot := lexicalSnapshot(t, map[string]string{
		lexicalFile: `package lexical
var Field = 1 // field-value
var Key = "a" // key-value
func forms() {
 _ = Holder{Field: Field} // struct-key
 _ = map[string]int{Key: 1} // map-key
 _ = []Holder{{Field: Field}} // elided-struct
 _ = []map[string]int{{Key: 1}} // elided-map
 _ = Unknown{Field: 1} // unresolved-key
 var holder Holder
 _ = holder.Field // selector-member
Field: // label-name
 goto Field // label-use
}
`,
		lexicalDirectory + "/types.go": `package lexical
type Holder = Actual
type Actual struct { Field int }
`,
	})
	for _, marker := range []string{"struct-key", "elided-struct", "unresolved-key", "selector-member", "label-name", "label-use"} {
		lexicalUnknown(t, snapshot, marker, "Field")
	}
	lexicalSame(t, snapshot, "map-key", "key-value", "Key")
	lexicalSame(t, snapshot, "elided-map", "key-value", "Key")
	for _, marker := range []string{"struct-key", "elided-struct"} {
		if lexicalBindingAt(t, snapshot, lexicalFile, marker, "Field", 1) != lexicalBindingAt(t, snapshot, lexicalFile, "field-value", "Field", 0) {
			t.Fatalf("%s lost its value expression", marker)
		}
	}
}

// TestLexicalOwnership refuses fabricated nodes and files, even when a name or
// a token position matches retained syntax from this or another snapshot.
func TestLexicalOwnership(t *testing.T) {
	sources := map[string]string{lexicalFile: "package lexical\nimport remote \"os\"\nvar Known = 1 // known\nfunc use() { _ = missing } // unknown\nfunc imported() { _ = remote.Environ } // imported\n"}
	first := lexicalSnapshot(t, sources)
	second := lexicalSnapshot(t, sources)
	file, identifier := lexicalIdent(t, first, lexicalFile, "known", "Known", 0)
	foreignFile, foreignIdentifier := lexicalIdent(t, second, lexicalFile, "known", "Known", 0)
	for _, row := range []struct {
		name       string
		file       *ast.File
		identifier *ast.Ident
	}{
		{"foreign identifier", file, foreignIdentifier},
		{"foreign file", foreignFile, identifier},
		{"spoofed file", &ast.File{Name: file.Name}, identifier},
		{"fabricated known", file, ast.NewIdent("Known")},
		{"fabricated universe", file, ast.NewIdent("int")},
		{"copied position", file, &ast.Ident{Name: "Known", NamePos: identifier.NamePos}},
	} {
		if _, known := first.binding(row.file, row.identifier); known {
			t.Fatalf("%s acquired retained ownership", row.name)
		}
	}
	lexicalUnknown(t, first, "unknown", "missing")
	var selector *ast.SelectorExpr
	ast.Inspect(file, func(node ast.Node) bool {
		if selected, ok := node.(*ast.SelectorExpr); ok {
			selector = selected
		}
		return true
	})
	if selector == nil {
		t.Fatal("authored import selector is missing")
	}
	if _, known := first.ImportedDirectory(SourceFile{Path: lexicalFile, File: file}, selector); !known {
		t.Fatal("retained import lost its file ownership")
	}
	if _, known := first.ImportedDirectory(SourceFile{Path: lexicalDirectory + "/spoof.go", File: file}, selector); known {
		t.Fatal("unselected path borrowed another retained file's import ownership")
	}
}

// TestLexicalUniverseTypes proves source shadows do not capture synthesized
// default types, including string-range and iterator-produced defaults.
func TestLexicalUniverseTypes(t *testing.T) {
	snapshot := lexicalSnapshot(t, map[string]string{lexicalFile: `package lexical
type int struct{} // shadow-int
type rune struct{} // shadow-rune
var len = 1 // shadow-len
func each(yield func(string) bool) {}
func defaults() {
 literal := 1 // literal
 for index, character := range "ab" { _, _ = index, character } // string-range
 for index := range 3 { _ = index } // integer-range
 for element := range each { _ = element } // iterator-range
 _ = len // builtin-shadow
 var explicit int // explicit-shadow
 _ = append // genuine-builtin
}
`})
	lexicalSame(t, snapshot, "builtin-shadow", "shadow-len", "len")
	lexicalSame(t, snapshot, "explicit-shadow", "shadow-int", "int")
	file, builtin := lexicalIdent(t, snapshot, lexicalFile, "genuine-builtin", "append", 0)
	if _, known := snapshot.binding(file, builtin); !known {
		t.Fatal("retained predeclared builtin became unresolved")
	}
	reader := snapshot.evaluator()
	for _, row := range []struct{ marker, name, expected string }{
		{"literal", "literal", "int"}, {"string-range", "index", "int"},
		{"string-range", "character", "rune"}, {"integer-range", "index", "int"},
		{"iterator-range", "element", "string"},
	} {
		owner, identifier := lexicalIdent(t, snapshot, lexicalFile, row.marker, row.name, 0)
		actual, known := reader.valueType(sourceExpr{value: identifier, file: owner, pkg: snapshot.packages[lexicalDirectory]}, 0)
		if !known {
			t.Fatalf("%s has no retained default type", row.name)
		}
		if types.ExprString(actual.value) != row.expected {
			t.Fatalf("%s has type %s, want %s", row.name, types.ExprString(actual.value), row.expected)
		}
		if _, _, concrete := reader.concreteType(actual); concrete {
			t.Fatalf("inferred %s borrowed a same-named source type", row.name)
		}
	}
}

// TestLexicalTypeSwitchMutationOwnership distinguishes a guard's per-case
// declarations from writes to a package input. Assigning initializers and
// case-body writes must still identify their actual source operation.
func TestLexicalTypeSwitchMutationOwnership(t *testing.T) {
	for _, row := range []struct {
		name     string
		body     string
		mutation string
	}{
		{
			name: "unrelated defining guard",
			body: `func inspect(input any) {
 switch kind := input.(type) {
 case string: _ = kind
 }
}
`,
		},
		{
			name: "same-named guard owns local writes",
			body: `func inspect(input any) {
 switch dependencies := input.(type) {
 case []string: dependencies = nil
 }
}
`,
		},
		{
			name: "assigning initializer writes package input",
			body: `func inspect(input any) {
 switch dependencies = []string{"changed"}; input.(type) { // mutation
 case string:
 }
}
`,
			mutation: "dependencies",
		},
		{
			name: "case body writes package input",
			body: `func inspect(input any) {
 switch kind := input.(type) {
 case string:
  dependencies[0] = kind // mutation
 }
}
`,
			mutation: "dependencies",
		},
		{
			name: "case body alias writes package input",
			body: `func inspect(input any) {
 switch kind := input.(type) {
 case string:
  alias := dependencies
  alias[0] = kind // mutation
 }
}
`,
			mutation: "alias",
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			snapshot := lexicalSnapshot(t, map[string]string{
				lexicalFile: "package lexical\nvar dependencies = []string{\"base\"} // queried\n" + row.body,
			})
			file, name := lexicalIdent(t, snapshot, lexicalFile, "queried", "dependencies", 0)
			failure := snapshot.evaluator().immutable(sourceExpr{value: name, file: file, pkg: snapshot.packages[lexicalDirectory]})
			if row.mutation == "" {
				if failure != nil {
					t.Fatalf("a defining guard wrote the package input: %v", failure)
				}
				return
			}
			if failure == nil {
				t.Fatal("a real write inside a type switch was certified immutable")
			}
			if !strings.Contains(failure.Error(), "declaration input dependencies is mutated after initialization") {
				t.Fatalf("type-switch write lost its mutation cause: %v", failure)
			}
			_, mutated := lexicalIdent(t, snapshot, lexicalFile, "mutation", row.mutation, 0)
			position := snapshot.fset.Position(mutated.Pos())
			expected := fmt.Sprintf("committed inventory %s:%d:", position.Filename, position.Line)
			if !strings.HasPrefix(failure.Error(), expected) {
				t.Fatalf("a guard declaration hid the real write at %s: %v", expected, failure)
			}
		})
	}
}

// TestLexicalTypeSwitchMutationRHSAddress proves guard declarations do not hide
// an address evaluated by the guard. The two mutation policies remain distinct.
func TestLexicalTypeSwitchMutationRHSAddress(t *testing.T) {
	snapshot := lexicalSnapshot(t, map[string]string{lexicalFile: `package lexical
var callback = func() {} // queried
func expose(value *func()) any { return value }
func inspect() {
 switch kind := expose(&callback).(type) { // escaped
 case *func(): _ = kind
 }
}
`})
	file, name := lexicalIdent(t, snapshot, lexicalFile, "queried", "callback", 0)
	source := sourceExpr{value: name, file: file, pkg: snapshot.packages[lexicalDirectory]}
	reader := snapshot.evaluator()
	if err := reader.immutable(source); err != nil {
		t.Fatalf("guard declaration or address-only use became a write: %v", err)
	}
	failure := reader.callableImmutable(source)
	if failure == nil {
		t.Fatal("the callable address escaping in a guard RHS was certified immutable")
	}
	if !strings.Contains(failure.Error(), "callable declaration input callback has an escaping address") {
		t.Fatalf("guard RHS lost its address-escape cause: %v", failure)
	}
	_, escaped := lexicalIdent(t, snapshot, lexicalFile, "escaped", "callback", 0)
	position := snapshot.fset.Position(escaped.Pos())
	expected := fmt.Sprintf("committed inventory %s:%d:", position.Filename, position.Line)
	if !strings.HasPrefix(failure.Error(), expected) {
		t.Fatalf("guard declaration hid the actual address escape at %s: %v", expected, failure)
	}
}
