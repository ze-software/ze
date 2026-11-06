// Design: docs/architecture/plugin/feature-gates.md -- complete selected mutation queries.
package repoinventory

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// declarationMutationQuery includes both declaration identity and the policy
// that decides whether taking its address or changing a field is a refusal.
type declarationMutationQuery struct {
	reference declarationReference
	check     mutationCheck
}

// A completed result may contain a nil failure. Only writes publishes one after
// its selected-source query has returned; absence is never an immutable fact.
type declarationMutationResult struct {
	failure error
}

// declarationMutationIndex is private to an expression reader and its immutable
// snapshot. All indexed nodes retain their selected file and package identities.
type declarationMutationIndex struct {
	files     map[*ast.File][]*declarationMutationFile
	packages  map[*sourcePackage][]*declarationMutationFile
	importers map[string][]*declarationMutationFile
}

type declarationMutationFile struct {
	pkg        *sourcePackage
	file       *ast.File
	operations *declarationMutationOperations
}

// A nonnil operations pointer certifies a complete preorder file scan, including
// a real zero-operation file. It is published only after that scan finishes.
type declarationMutationOperations struct {
	nodes []ast.Node
}

func newDeclarationMutationIndex(snapshot *Snapshot) *declarationMutationIndex {
	index := &declarationMutationIndex{
		files:     make(map[*ast.File][]*declarationMutationFile),
		packages:  make(map[*sourcePackage][]*declarationMutationFile),
		importers: make(map[string][]*declarationMutationFile),
	}
	directories := make([]string, 0, len(snapshot.packages))
	for directory := range snapshot.packages {
		directories = append(directories, directory)
	}
	slices.Sort(directories)
	for _, directory := range directories {
		pkg := snapshot.packages[directory]
		files := slices.Clone(pkg.files)
		slices.SortFunc(files, func(left, right SourceFile) int { return strings.Compare(left.Path, right.Path) })
		for _, source := range files {
			file := &declarationMutationFile{pkg: pkg, file: source.File}
			index.files[source.File] = append(index.files[source.File], file)
			index.packages[pkg] = append(index.packages[pkg], file)
			seen := make(map[string]bool)
			for _, imported := range pkg.imports[source.File] {
				if !seen[imported] {
					index.importers[imported] = append(index.importers[imported], file)
					seen[imported] = true
				}
			}
		}
	}
	return index
}

func (file *declarationMutationFile) indexOperations() *declarationMutationOperations {
	if file.operations != nil {
		return file.operations
	}
	operations := &declarationMutationOperations{}
	ast.Inspect(file.file, func(node ast.Node) bool {
		switch operation := node.(type) {
		case *ast.AssignStmt, *ast.IncDecStmt:
			operations.nodes = append(operations.nodes, node)
		case *ast.RangeStmt:
			if operation.Tok == token.ASSIGN {
				operations.nodes = append(operations.nodes, node)
			}
		case *ast.UnaryExpr:
			if operation.Op == token.AND {
				operations.nodes = append(operations.nodes, node)
			}
		}
		return true
	})
	file.operations = operations
	return operations
}

func (index *declarationMutationIndex) writes(reader *expression, reference declarationReference, check mutationCheck) error {
	if reference.local != nil {
		for _, file := range index.files[reference.file] {
			if file.pkg != reference.pkg {
				if !importsPackage(file.pkg.imports[file.file], reference.pkg.dir) {
					continue
				}
			}
			if err := file.writes(reader, reference, check); err != nil {
				return err
			}
		}
		return nil
	}
	for _, file := range index.packages[reference.pkg] {
		if err := file.writes(reader, reference, check); err != nil {
			return err
		}
	}
	for _, file := range index.importers[reference.pkg.dir] {
		if file.pkg == reference.pkg {
			continue
		}
		if err := file.writes(reader, reference, check); err != nil {
			return err
		}
	}
	return nil
}

func (file *declarationMutationFile) writes(reader *expression, reference declarationReference, check mutationCheck) error {
	for _, node := range file.indexOperations().nodes {
		var targets []ast.Expr
		switch statement := node.(type) {
		case *ast.AssignStmt:
			targets = statement.Lhs
		case *ast.IncDecStmt:
			targets = []ast.Expr{statement.X}
		case *ast.RangeStmt:
			targets = []ast.Expr{statement.Key, statement.Value}
		case *ast.UnaryExpr:
			if check != callableBindingWrites {
				continue
			}
			if refersTo(statement.X, reference, file.pkg, file.file, 0) {
				return reader.problem(sourceExpr{value: statement, file: file.file, pkg: file.pkg}, "callable declaration input "+reference.name+" has an escaping address")
			}
			continue
		default:
			return fmt.Errorf("unsupported indexed declaration mutation %T; retain its exact mutation policy before publishing inventory", node)
		}
		for _, target := range targets {
			if assignment, assigned := node.(*ast.AssignStmt); assigned {
				if assignment.Tok == token.DEFINE {
					if declared, named := target.(*ast.Ident); named {
						if declared.Obj != nil {
							if declared.Obj.Decl == assignment {
								continue
							}
						}
					}
				}
			}
			if check == registrationFieldWrites {
				if field, selected := target.(*ast.SelectorExpr); selected {
					if !metadataField(field.Sel.Name) {
						continue
					}
				}
			}
			if refersTo(target, reference, file.pkg, file.file, 0) {
				return reader.problem(sourceExpr{value: target, file: file.file, pkg: file.pkg}, "declaration input "+reference.name+" is mutated after initialization")
			}
		}
	}
	return nil
}
