package archerrorinventory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// findingTypesFloor is fewer finding types than internal/le declared on
// 2026-11-11 (24), so a walk that matched no file or no type is refused
// rather than read as a tree with nothing to classify.
const findingTypesFloor = 20

// TestEveryFindingTextFieldIsClassified proves the finding lists are
// complete over the real tree. Every text field of every struct type
// internal/le declares with a name ending in findingSuffix is named by
// exactly one of findingMessageFields and findingIdentityFields; every such
// type declares a message field or is in rendererWordedFindings, never both;
// and every rendererWordedFindings entry names a type that exists and a
// renderer its package declares. A new finding type, or a new text field,
// that no list accounts for is red here until someone classifies it, so it
// cannot leave the inventory in silence. A text field is one written string
// or []string, or a type the same package declares as one of those; a type
// imported from another package is not resolved, and no finding type uses
// one for text today. Method: parse every non-test Go file under
// internal/le (no type check, so it stays fast), outside testdata, then
// judge each directory once all of its files are read.
func TestEveryFindingTextFieldIsClassified(t *testing.T) {
	for _, field := range findingMessageFields {
		if slices.Contains(findingIdentityFields, field) {
			t.Errorf("field %s is in both findingMessageFields and findingIdentityFields", field)
		}
	}
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	packages, err := readFindingPackages(root)
	if err != nil {
		t.Fatalf("read %s: %v", toolingDir, err)
	}
	dirs := make([]string, 0, len(packages))
	for dir := range packages {
		dirs = append(dirs, dir)
	}
	slices.Sort(dirs)
	findingTypes := 0
	for _, dir := range dirs {
		pkg := packages[dir]
		names := make([]string, 0, len(pkg.findings))
		for name := range pkg.findings {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			findingTypes++
			judgeFindingType(t, dir, name, pkg.findings[name], pkg.textTypes)
		}
	}
	if findingTypes < findingTypesFloor {
		t.Fatalf("found %d finding types under %s, want at least %d: the walk read too little to judge", findingTypes, toolingDir, findingTypesFloor)
	}
	for key, renderer := range rendererWordedFindings {
		dir, name, cut := strings.Cut(key, ".")
		if !cut {
			t.Errorf("rendererWordedFindings key %q is not <directory>.<type>", key)
			continue
		}
		pkg, known := packages[dir]
		if !known {
			t.Errorf("rendererWordedFindings names %s, but %s declares no finding type", key, dir)
			continue
		}
		if _, declared := pkg.findings[name]; !declared {
			t.Errorf("rendererWordedFindings names %s, but %s declares no such finding type", key, dir)
		}
		if !pkg.renderers[renderer] {
			t.Errorf("rendererWordedFindings says %s words %s, but %s declares no %s", renderer, key, dir, renderer)
		}
	}
}

// findingPackage is what TestEveryFindingTextFieldIsClassified reads of one
// internal/le directory.
type findingPackage struct {
	// findings maps each finding struct type to its declaration.
	findings map[string]*ast.StructType
	// textTypes holds the types the package declares as string or []string.
	textTypes map[string]bool
	// renderers holds each function as Name and each method as Recv.Name.
	renderers map[string]bool
}

// readFindingPackages parses every non-test Go file under internal/le,
// outside testdata, and answers what each directory declares, keyed by its
// slash path relative to root.
func readFindingPackages(root string) (map[string]*findingPackage, error) {
	fset := token.NewFileSet()
	packages := make(map[string]*findingPackage)
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(toolingDir)), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		dir := filepath.ToSlash(rel)
		pkg := packages[dir]
		if pkg == nil {
			pkg = &findingPackage{findings: map[string]*ast.StructType{}, textTypes: map[string]bool{}, renderers: map[string]bool{}}
			packages[dir] = pkg
		}
		pkg.add(file)
		return nil
	})
	return packages, err
}

// add records the finding types, text types and renderers one file declares.
func (pkg *findingPackage) add(file *ast.File) {
	for _, decl := range file.Decls {
		switch node := decl.(type) {
		case *ast.FuncDecl:
			pkg.renderers[rendererName(node)] = true
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				typeSpec, isType := spec.(*ast.TypeSpec)
				if !isType {
					continue
				}
				if isTextType(typeSpec.Type, nil) {
					pkg.textTypes[typeSpec.Name.Name] = true
					continue
				}
				if !strings.HasSuffix(typeSpec.Name.Name, findingSuffix) {
					continue
				}
				if structType, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
					pkg.findings[typeSpec.Name.Name] = structType
				}
			}
		}
	}
}

// rendererName answers a function's name, or Recv.Name for a method, the
// form rendererWordedFindings writes.
func rendererName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	receiver := fn.Recv.List[0].Type
	if star, isStar := receiver.(*ast.StarExpr); isStar {
		receiver = star.X
	}
	return types.ExprString(receiver) + "." + fn.Name.Name
}

// judgeFindingType reports each text field of the finding type dir.name
// that no list classifies, and the type itself when it declares no message
// field and no renderer words it, or declares one and is listed anyway.
func judgeFindingType(t *testing.T, dir, name string, structType *ast.StructType, textTypes map[string]bool) {
	t.Helper()
	key := dir + "." + name
	messageFields := 0
	for _, field := range structType.Fields.List {
		if !isTextType(field.Type, textTypes) {
			continue
		}
		for _, fieldName := range field.Names {
			if slices.Contains(findingMessageFields, fieldName.Name) {
				messageFields++
				continue
			}
			if slices.Contains(findingIdentityFields, fieldName.Name) {
				continue
			}
			t.Errorf("%s.%s is a text field neither findingMessageFields nor findingIdentityFields names: add it to findingMessageFields if the report prints it as the finding, else to findingIdentityFields", key, fieldName.Name)
		}
	}
	_, worded := rendererWordedFindings[key]
	if messageFields == 0 && !worded {
		t.Errorf("%s declares no field of findingMessageFields and is not in rendererWordedFindings, so no inventory row judges its wording: add the field that carries its text, or list it with the renderer that words it", key)
	}
	if messageFields > 0 && worded {
		t.Errorf("%s declares %d field(s) of findingMessageFields, so its literals are message sites: remove it from rendererWordedFindings", key, messageFields)
	}
}

// isTextType reports whether typ is string or []string as written, or a
// name textTypes holds (a type its package declares as one of those).
func isTextType(typ ast.Expr, textTypes map[string]bool) bool {
	if slice, isSlice := typ.(*ast.ArrayType); isSlice && slice.Len == nil {
		typ = slice.Elt
	}
	ident, isIdent := typ.(*ast.Ident)
	if !isIdent {
		return false
	}
	return ident.Name == "string" || textTypes[ident.Name]
}

const identityFindingFile = "internal/le/test/sample/finding.go"

// identityFindingSource holds the literals of one finding type that sets
// identity fields only, and the ones that set a message field beside them.
// Each literal carries a marker comment naming its case.
const identityFindingSource = `package sample

type Finding struct {
	Path    string
	Package string
	Name    string
	Reason  string
	Details []string
}

func identity(path, packageName, name string) Finding {
	return Finding{Path: path, Package: packageName, Name: name} // not-a-writer:identity
}

func lookup() (Finding, bool) {
	return Finding{}, false // not-a-writer:empty
}

func keys() []Finding {
	return []Finding{
		{Path: "a.go", Name: "TestA"}, // not-a-writer:identity-elided
	}
}

func judged(path string) Finding {
	return Finding{Path: path, Reason: "run ./le test weakened check"} // writer:message-field
}

func judgedElided() []Finding {
	return []Finding{
		{Path: "b.go", Details: []string{"the assertion was removed"}}, // writer:message-field-elided
	}
}

func positional() Finding {
	return Finding{"c.go", "sample", "TestC", "", nil} // writer:positional
}

func label(Finding) string { return "" }

func nested() Finding {
	return Finding{Path: "d.go", Name: label(Finding{Path: "e.go", Reason: "a nested finding"})} // writer:nested-in-identity
}

func nestedElided() []Finding {
	return []Finding{
		{
			Path: "f.go",
			Name: label(Finding{Path: "g.go", Reason: "a nested finding"}), // writer:nested-in-identity-elided
		},
	}
}
`

// TestIdentityOnlyFindingIsNotAMessage proves that a finding literal which
// sets no message field is no message site (AC-37): the identity-only
// Finding{Path, Package, Name} the weakening gate builds as a key, the empty
// Finding{} answered beside a verdict, and an elided identity-only element
// give no row, so the inventory reports no plain message with text "".
// The survivors are the literals that do hand the user text: a keyed literal
// that sets a message field, as a whole literal and as an elided element,
// and a positional literal, which sets every field the type declares. An
// identity literal is still walked, so a message-carrying finding nested in
// it is a row, whether the identity literal is written whole or elided.
// Method: one marked literal per case, scanned once, every row accounted for.
func TestIdentityOnlyFindingIsNotAMessage(t *testing.T) {
	rows := scanSources(t, map[string]string{identityFindingFile: identityFindingSource})

	for _, marker := range []string{"not-a-writer:identity", "not-a-writer:empty", "not-a-writer:identity-elided"} {
		line := lineOf(t, identityFindingSource, marker)
		for _, row := range rows {
			if row.File == identityFindingFile && row.Line == line {
				t.Errorf("%s: line %d is a %s row with message %q, want no row", marker, line, row.Writer, row.Message)
			}
		}
	}

	want := []struct {
		marker string
		status Status
	}{
		{"writer:message-field", StatusPlainNamesCommand},
		{"writer:message-field-elided", StatusPlain},
		{"writer:nested-in-identity", StatusPlain},
		{"writer:nested-in-identity-elided", StatusPlain},
		{"writer:positional", StatusPlain},
	}
	for _, tc := range want {
		t.Run(tc.marker, func(t *testing.T) {
			row := rowAt(t, rows, identityFindingFile, lineOf(t, identityFindingSource, tc.marker))
			if row.Writer != "Finding" {
				t.Errorf("writer %q, want %q", row.Writer, "Finding")
			}
			if row.Surface != SurfaceTooling {
				t.Errorf("surface %s, want %s", row.Surface, SurfaceTooling)
			}
			if row.Status != tc.status {
				t.Errorf("status %s, want %s (message %q)", row.Status, tc.status, row.Message)
			}
		})
	}
	if len(rows) != len(want) {
		var found []string
		for _, row := range rows {
			found = append(found, row.File+":"+strconv.Itoa(row.Line)+" "+row.Writer+" "+strconv.Quote(row.Message))
		}
		slices.Sort(found)
		t.Errorf("the scan found %d rows, want %d:\n%s", len(rows), len(want), strings.Join(found, "\n"))
	}
}

// TestEveryFindingMessageFieldIsJudged proves that no declared message
// field lets a finding escape the inventory: a literal that sets only an
// identity field and that one message field is a row, for every name
// findingMessageFields declares. A name dropped from the list, or misspelled
// in the check, turns its literal into no row and this test red.
// Method: one finding type declaring every message field, one marked
// literal per field, scanned once.
func TestEveryFindingMessageFieldIsJudged(t *testing.T) {
	if len(findingMessageFields) == 0 {
		t.Fatal("findingMessageFields declares no field, so no finding literal can be a message")
	}
	var source strings.Builder
	source.WriteString("package sample\n\ntype Finding struct {\n\tFile string\n")
	for _, field := range findingMessageFields {
		source.WriteString("\t" + field + " string\n")
	}
	source.WriteString("}\n")
	for _, field := range findingMessageFields {
		source.WriteString("\nfunc set" + field + "() Finding {\n")
		source.WriteString("\treturn Finding{File: \"a.go\", " + field + ": \"no fix\"} // writer:" + field + "\n}\n")
	}
	text := source.String()
	rows := scanSources(t, map[string]string{identityFindingFile: text})

	for _, field := range findingMessageFields {
		t.Run(field, func(t *testing.T) {
			row := rowAt(t, rows, identityFindingFile, lineOf(t, text, "writer:"+field))
			if row.Writer != "Finding" {
				t.Errorf("writer %q, want %q", row.Writer, "Finding")
			}
			if row.Status != StatusPlain {
				t.Errorf("status %s, want %s (message %q)", row.Status, StatusPlain, row.Message)
			}
		})
	}
	if len(rows) != len(findingMessageFields) {
		t.Errorf("the scan found %d rows, want %d: %+v", len(rows), len(findingMessageFields), rows)
	}
}
