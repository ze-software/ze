// Design: docs/architecture/plugin/feature-gates.md -- compiler-equivalent generated header membership.
// Related: assemblygeneratedkinds.go, assemblygeneratedtypes.go -- bounded kind and field-name evidence.
package repoinventory

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
)

type generatedConstant struct {
	name     string
	value    ast.Expr
	datatype ast.Expr
	file     *ast.File
	pkg      *sourcePackage
}

type generatedType struct {
	name string
	spec *ast.TypeSpec
	file *ast.File
	pkg  *sourcePackage
}

type generatedDeclaration struct {
	constant *generatedConstant
	datatype *generatedType
}

type generatedDeclarations struct {
	constants map[string]*generatedConstant
	types     map[string]*generatedType
	ordered   []generatedDeclaration
	bindings  []string
	complete  bool
}

type generatedAssembly struct {
	snapshot     *Snapshot
	declarations map[*sourcePackage]*generatedDeclarations
	kinds        map[*generatedConstant]generatedKind
	kindActive   map[*generatedConstant]bool
	shapes       map[*generatedType]generatedShape
	shapeActive  map[*generatedType]bool
}

// generatedAssemblyBindings preserves the compiler emission sequence, including
// duplicates. Evidence is derived lazily from this package's retained ASTs only.
func generatedAssemblyBindings(reader *generatedAssembly, pkg *sourcePackage) ([]string, error) {
	if reader.declarations == nil {
		reader.declarations = make(map[*sourcePackage]*generatedDeclarations)
		reader.kinds = make(map[*generatedConstant]generatedKind)
		reader.kindActive = make(map[*generatedConstant]bool)
		reader.shapes = make(map[*generatedType]generatedShape)
		reader.shapeActive = make(map[*generatedType]bool)
	}
	declarations, err := reader.packageDeclarations(pkg)
	if err != nil {
		return nil, err
	}
	if declarations.complete {
		return declarations.bindings, nil
	}
	var bindings []string
	for _, declaration := range declarations.ordered {
		if item := declaration.constant; item != nil {
			kind, err := reader.constantKind(item)
			if err != nil {
				return nil, fmt.Errorf("unsupported generated assembly constant kind for %s: %w", item.name, err)
			}
			switch kind.kind {
			case constant.Int, constant.Bool, constant.String:
				bindings = append(bindings, "const_"+item.name)
			case constant.Float, constant.Complex:
			default:
				return nil, fmt.Errorf("unsupported generated assembly constant kind for %s", item.name)
			}
			continue
		}
		item := declaration.datatype
		// writer.pkgDecl excludes generic declarations BEFORE AsmHdrDecls.
		if item.spec.TypeParams.NumFields() != 0 {
			continue
		}
		shape, err := reader.declaredShape(item)
		if err != nil {
			return nil, fmt.Errorf("unsupported generated assembly type %s: %w", item.name, err)
		}
		if shape.structure == nil {
			continue
		}
		bindings = append(bindings, item.name+"__size")
		for _, field := range shape.structure.Fields.List {
			if len(field.Names) == 0 {
				name, err := generatedEmbeddedName(field.Type)
				if err != nil {
					return nil, fmt.Errorf("unsupported generated assembly type %s: %w", item.name, err)
				}
				bindings = append(bindings, item.name+"_"+name)
				continue
			}
			for _, name := range field.Names {
				if name.Name != "_" {
					bindings = append(bindings, item.name+"_"+name.Name)
				}
			}
		}
	}
	declarations.bindings = bindings
	declarations.complete = true
	return bindings, nil
}

// packageDeclarations records declarations, not their evaluated membership.
// Unused unsupported declarations in imported defining packages stay lazy too.
func (reader *generatedAssembly) packageDeclarations(pkg *sourcePackage) (*generatedDeclarations, error) {
	if declarations, found := reader.declarations[pkg]; found {
		return declarations, nil
	}
	declarations := &generatedDeclarations{constants: make(map[string]*generatedConstant), types: make(map[string]*generatedType)}
	for _, source := range pkg.files {
		for _, declaration := range source.File.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			switch group.Tok {
			case token.CONST:
				var inherited []ast.Expr
				var datatype ast.Expr
				for _, raw := range group.Specs {
					spec := raw.(*ast.ValueSpec)
					if len(spec.Values) != 0 {
						inherited, datatype = spec.Values, spec.Type
					}
					for index, name := range spec.Names {
						if name.Name == "_" {
							continue
						}
						if _, found := declarations.constants[name.Name]; found {
							return nil, fmt.Errorf("unsupported generated assembly constant kind for duplicated %s", name.Name)
						}
						item := &generatedConstant{name: name.Name, datatype: datatype, file: source.File, pkg: pkg}
						if index < len(inherited) {
							item.value = inherited[index]
						}
						declarations.constants[name.Name] = item
						declarations.ordered = append(declarations.ordered, generatedDeclaration{constant: item})
					}
				}
			case token.TYPE:
				for _, raw := range group.Specs {
					spec := raw.(*ast.TypeSpec)
					if spec.Name.Name == "_" {
						continue
					}
					if _, found := declarations.types[spec.Name.Name]; found {
						return nil, fmt.Errorf("unsupported generated assembly type duplicated %s", spec.Name.Name)
					}
					item := &generatedType{name: spec.Name.Name, spec: spec, file: source.File, pkg: pkg}
					declarations.types[item.name] = item
					declarations.ordered = append(declarations.ordered, generatedDeclaration{datatype: item})
				}
			}
		}
	}
	reader.declarations[pkg] = declarations
	return declarations, nil
}

func (reader *generatedAssembly) importedPackage(pkg *sourcePackage, file *ast.File, qualifier ast.Expr) (*sourcePackage, error) {
	name, ok := qualifier.(*ast.Ident)
	if !ok {
		return nil, fmt.Errorf("unsupported generated qualifier")
	}
	target, found := pkg.imports[file][name.Name]
	if !found {
		return nil, fmt.Errorf("unknown generated import %s", name.Name)
	}
	defining, found := reader.snapshot.packages[target]
	if !found {
		return nil, fmt.Errorf("unretained generated import %s", name.Name)
	}
	return defining, nil
}
