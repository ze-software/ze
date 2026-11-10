// Design: docs/architecture/doctor-and-health-checks.md -- the doctor-coverage gate
//
// Overview: doctorcoverage.go -- the site kinds and statuses, and the scan
//
// codes.go builds the covered set: every code a doctor check declaration in
// the scanned tree lists, with the checks that list it. It is read from the
// same parsed source the sites come from, so the covered set is the one of
// the tree side being judged: the checkout for check and report, each side
// of the commit for the commit seam. A check in a package no composition root
// imports covers its codes, and a commit that adds a check with its first use
// is judged against the check it adds. Nothing here transcribes a code
// (ai/rules/principles.md).

package archdoctorcoverage

import (
	"errors"
	"go/ast"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Covered maps each code a doctor check declaration lists to the names of the
// checks that list it, sorted. A check whose name the source does not fix is
// named by its file and line.
type Covered map[string][]string

// typeRef names a Go type by its tree directory and its name.
type typeRef struct {
	dir  string
	name string
}

// checkDeclaration is a type a doctor check is declared as, and the fields of
// it the covered set reads.
type checkDeclaration struct {
	typeRef
	// nameField holds the check's name, which namePrefix opens.
	nameField  string
	namePrefix string
	// listFields each hold a list of codes; codeFields each hold one code.
	listFields []string
	codeFields []string
	// addedCodes are codes the type's registration adds to every check it
	// builds (kernelcap.MustRegister adds the forced code); the source of the
	// declaration does not spell them, so the table does, and the oracle test
	// compares the result with what the product registers.
	addedCodes []string
}

// checkNameField is the field most declaration types name their check in.
const checkNameField = "Name"

// codesField is the list field every declaration type but kernelcap names its codes in.
var codesField = []string{"Codes"}

// checkDeclarations are the types a doctor check is declared as. This is a
// second copy of what the product registries hold, so
// TestDoctorCoverageSourceHoldsEveryRegisteredCode compares the two over the
// checkout: a new declaration shape that registers a check this table misses
// fails it.
var checkDeclarations = []checkDeclaration{
	// An in-tree check, given to diagnostic.RegisterDoctorCheck.
	{dir: "internal/core/diagnostic", name: "DoctorCheck", nameField: checkNameField, listFields: codesField},
	// A plugin's check, in registry.Registration.DoctorChecks.
	{dir: "internal/component/plugin/registry", name: "DoctorCheckDef", nameField: checkNameField, listFields: codesField},
	// An SDK plugin's declaration, sent at startup, and its SDK alias.
	{dir: "pkg/plugin/rpc", name: "DoctorCheckDecl", nameField: checkNameField, listFields: codesField},
	{dir: "pkg/plugin/sdk", name: "DoctorCheckDecl", nameField: checkNameField, listFields: codesField},
	// A kernel capability: kernelcap.MustRegister registers the doctor check
	// kernel-capability-<Subsystem>, which declares the capability's two codes.
	{dir: "internal/component/kernelcap", name: "Capability", nameField: "Subsystem", namePrefix: "kernel-capability-", codeFields: []string{"CodeAbsent", "CodeUnknown"}, addedCodes: []string{"doctor-kernel-capability-forced"}},
}

// coveredCodes reads the covered set from every doctor check declaration in
// the index: a composite literal of a declaration type, or an element of a
// slice, array or map literal of one whose type is elided. A declaration is
// read where it is written, whatever registers it (a variable, a function's
// result, a loop over a slice), so the syntax decides and no data flow is
// traced.
func (index *treeIndex) coveredCodes() Covered {
	covered := Covered{}
	for _, file := range index.files {
		ast.Inspect(file.ast, func(node ast.Node) bool {
			literal, isLiteral := node.(*ast.CompositeLit)
			if !isLiteral {
				return true
			}
			if declaration, isCheck := index.declarationOf(file, literal.Type); isCheck {
				index.addCheck(covered, file, literal, declaration)
				return true
			}
			element := elementType(literal.Type)
			if element == nil {
				return true
			}
			declaration, isCheck := index.declarationOf(file, element)
			if !isCheck {
				return true
			}
			for _, value := range literal.Elts {
				if pair, isPair := value.(*ast.KeyValueExpr); isPair {
					value = pair.Value
				}
				inner, isInner := value.(*ast.CompositeLit)
				if !isInner {
					continue
				}
				// A typed element is its own literal, which the walk visits.
				if inner.Type != nil {
					continue
				}
				index.addCheck(covered, file, inner, declaration)
			}
			return true
		})
	}
	for code := range covered {
		slices.Sort(covered[code])
	}
	return covered
}

// declarationOf answers the declaration a type expression names: by its name
// inside the declaring package, or by the package's import name elsewhere.
func (index *treeIndex) declarationOf(file *sourceFile, expr ast.Expr) (checkDeclaration, bool) {
	var ref typeRef
	switch node := expr.(type) {
	case *ast.Ident:
		ref = typeRef{dir: file.dir, name: node.Name}
	case *ast.SelectorExpr:
		receiver, isIdent := node.X.(*ast.Ident)
		if !isIdent {
			return checkDeclaration{}, false
		}
		path, imported := file.imports[receiver.Name]
		if !imported {
			return checkDeclaration{}, false
		}
		dir, inTree := cutModule(path)
		if !inTree {
			return checkDeclaration{}, false
		}
		ref = typeRef{dir: dir, name: node.Sel.Name}
	default:
		return checkDeclaration{}, false
	}
	for _, declaration := range checkDeclarations {
		if declaration.typeRef == ref {
			return declaration, true
		}
	}
	return checkDeclaration{}, false
}

// elementType answers the element type of a slice, array or map literal's
// type, through one pointer, or nil for any other type.
func elementType(expr ast.Expr) ast.Expr {
	var element ast.Expr
	switch node := expr.(type) {
	case *ast.ArrayType:
		element = node.Elt
	case *ast.MapType:
		element = node.Value
	default:
		return nil
	}
	if pointer, isPointer := element.(*ast.StarExpr); isPointer {
		return pointer.X
	}
	return element
}

// addCheck records the codes one doctor check declaration lists. A code the
// source does not fix (a function parameter's field, a call) is not recorded:
// no site can be matched to it, so a site naming it is refused rather than
// covered.
func (index *treeIndex) addCheck(covered Covered, file *sourceFile, literal *ast.CompositeLit, declaration checkDeclaration) {
	var name ast.Expr
	var codes []constRef
	for _, value := range literal.Elts {
		pair, isPair := value.(*ast.KeyValueExpr)
		if !isPair {
			continue
		}
		key, isKey := pair.Key.(*ast.Ident)
		if !isKey {
			continue
		}
		if key.Name == declaration.nameField {
			name = pair.Value
		}
		if slices.Contains(declaration.codeFields, key.Name) {
			codes = append(codes, constRef{expr: pair.Value, file: file})
		}
		if !slices.Contains(declaration.listFields, key.Name) {
			continue
		}
		// A list built at run time gives no code the scan can match a site
		// to, so a site naming one of its codes is refused, never covered.
		if list, isFixed := index.listOf(file, pair.Value, 0); isFixed {
			codes = append(codes, list...)
		}
	}
	check := index.checkName(file, literal, declaration.namePrefix, name)
	for _, code := range codes {
		value, isFixed := index.resolve(code.file, code.expr, 0)
		if !isFixed {
			continue
		}
		covered.add(check, value)
	}
	for _, code := range declaration.addedCodes {
		covered.add(check, code)
	}
}

// listOf answers the elements of a code list, each with the file it is
// written in, so it resolves against that file's imports: a slice literal, or
// a package-level variable of this package or another tree package holding
// one. Anything else (a call, a field) answers false: its list is known only
// at run time.
func (index *treeIndex) listOf(file *sourceFile, expr ast.Expr, depth int) ([]constRef, bool) {
	if depth > codeDepthMax {
		return nil, false
	}
	switch node := expr.(type) {
	case *ast.CompositeLit:
		elements := make([]constRef, 0, len(node.Elts))
		for _, element := range node.Elts {
			elements = append(elements, constRef{expr: element, file: file})
		}
		return elements, true
	case *ast.Ident:
		return index.listOfVariable(file.dir, node.Name, depth)
	case *ast.SelectorExpr:
		receiver, isIdent := node.X.(*ast.Ident)
		if !isIdent {
			return nil, false
		}
		path, imported := file.imports[receiver.Name]
		if !imported {
			return nil, false
		}
		dir, inTree := cutModule(path)
		if !inTree {
			return nil, false
		}
		return index.listOfVariable(dir, node.Sel.Name, depth)
	}
	return nil, false
}

// listOfVariable answers the elements of the code list the package-level
// variable name of directory dir holds, read in the variable's own file.
func (index *treeIndex) listOfVariable(dir, name string, depth int) ([]constRef, bool) {
	ref, declared := index.variables[dir][name]
	if !declared {
		return nil, false
	}
	return index.listOf(ref.file, ref.expr, depth+1)
}

// checkName answers a declaration's name as the source fixes it, after its
// prefix, or its file and line when the name is not a literal or a constant.
func (index *treeIndex) checkName(file *sourceFile, literal *ast.CompositeLit, prefix string, name ast.Expr) string {
	if name != nil {
		if value, isFixed := index.resolve(file, name, 0); isFixed {
			return prefix + value
		}
	}
	var tb textbuf.Buffer
	return tb.Str(file.path).Byte(':').Int(int64(index.fset.Position(literal.Pos()).Line)).String()
}

// add records that check declares code.
func (c Covered) add(check, code string) {
	if slices.Contains(c[code], check) {
		return
	}
	c[code] = append(c[code], check)
}

// ErrThinCoveredSet names a covered set smaller than the tree is known to
// declare.
var ErrThinCoveredSet = errors.New("the doctor-coverage gate read too few doctor codes from the tree's doctor check declarations to judge it")

// checkCodesFloor refuses a covered set below floor codes. A declaration
// shape the gate no longer reads (a renamed check type, a moved package, a
// renamed Codes field) would otherwise answer every site as uncovered, and a
// refusal of every new line is a broken gate, not a finding.
func checkCodesFloor(covered Covered, floor int) error {
	if len(covered) >= floor {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str(": read ").Int(int64(len(covered))).Str(", at least ").Int(int64(floor)).
		Str(" expected. Check that checkDeclarations in internal/le/arch/doctorcoverage/codes.go still names the doctor check declaration types and their name and code fields, then run TestDoctorCoverageSourceHoldsEveryRegisteredCode")
	return errors.New(ErrThinCoveredSet.Error() + tb.String())
}
