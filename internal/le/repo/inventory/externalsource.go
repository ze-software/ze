// Design: docs/architecture/plugin/feature-gates.md -- committed external-effect authority.
package repoinventory

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"

	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// externalSource owns a private selected syntax universe. Its constructor reads
// retained blobs only; it never adds potential imports to public Sources or to
// the certain initialization population. A zero value is not initialized.
type externalSource struct {
	snapshot *Snapshot
	imports  map[string][]string
	roots    map[string]bool
	entries  map[string]externalEntrySet
}

// newExternalSource bounds package selection by the captured directory set.
// It creates a private FileSet and ASTs: concurrent snapshot readers never share
// mutable selection or analysis state.
func newExternalSource(committed *Snapshot) (*externalSource, error) {
	manifest, err := modfile.Parse("go.mod", committed.blobs["go.mod"], nil)
	if err != nil {
		return nil, fmt.Errorf("external source identity: %w", err)
	}
	if len(manifest.Replace) != 0 {
		return nil, fmt.Errorf("external source identity has unresolved module replacements; retain an exact replacement selection before publishing inventory")
	}
	if _, present := committed.blobs["go.work"]; present {
		return nil, fmt.Errorf("external source identity has an unresolved workspace; select one committed module before publishing inventory")
	}
	for _, requirement := range manifest.Require {
		if externalParentModuleOverlap(requirement.Mod.Path) {
			return nil, fmt.Errorf("external parent namespace overlaps required module %s; retain one committed module owner before publishing inventory", requirement.Mod.Path)
		}
	}
	if err := committed.checkExternalModuleSelection(); err != nil {
		return nil, err
	}
	if _, err := externalVendorPackages(committed, manifest); err != nil {
		return nil, err
	}
	gates, err := repofeaturetags.ParseGates(committed.revision+":feature-gates.txt", committed.blobs["feature-gates.txt"])
	if err != nil {
		return nil, err
	}
	context := linuxContext(gates, committed.blobs)
	potentialDirectories, err := externalPotentialDirectories(committed, &context, manifest)
	if err != nil {
		return nil, err
	}
	private := *committed
	private.directories = potentialDirectories
	private.fset = token.NewFileSet()
	private.files = nil
	private.packages = make(map[string]*sourcePackage)
	result := &externalSource{snapshot: &private, imports: make(map[string][]string), roots: make(map[string]bool), entries: make(map[string]externalEntrySet)}
	for directory := range committed.packages {
		result.roots[directory] = true
	}
	directories := make([]string, 0, len(private.directories))
	for directory := range private.directories {
		directories = append(directories, directory)
	}
	slices.Sort(directories)
	for _, directory := range directories {
		if committed.externalNestedModule(directory) != "" {
			continue
		}
		selected := false
		for _, name := range private.directories[directory] {
			if !strings.HasSuffix(name, ".go") {
				continue
			}
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			match, err := context.MatchFile(directory, path.Base(name))
			if err != nil {
				return nil, fmt.Errorf("external source selection %s: %w", name, err)
			}
			if !match {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), name, committed.blobs[name], parser.ImportsOnly)
			if err != nil {
				return nil, fmt.Errorf("external source identity %s: %w", name, err)
			}
			if importsC(file) {
				continue
			}
			selected = true
			break
		}
		if !selected {
			continue
		}
		if err := private.selectPackage(directory, &context); err != nil {
			if boundaryErr := private.checkExternalModuleSelection(); boundaryErr != nil {
				return nil, boundaryErr
			}
			return nil, fmt.Errorf("external retained universe: %w", err)
		}
	}
	if err := private.checkExternalModuleSelection(); err != nil {
		return nil, err
	}
	for directory, pkg := range private.packages {
		for _, file := range pkg.files {
			for _, imported := range file.File.Imports {
				identity, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return nil, fmt.Errorf("external import identity %s: %w", file.Path, err)
				}
				result.imports[directory] = append(result.imports[directory], identity)
			}
		}
		slices.Sort(result.imports[directory])
		result.imports[directory] = slices.Compact(result.imports[directory])
	}
	if err := result.acyclic(); err != nil {
		return nil, err
	}
	return result, nil
}

// reaches uses exact qualified import identities. An empty directory used by
// the ordinary evaluator for a standard import is never a graph node here.
// Each retained package enters the work list once, bounding traversal by input.
func (s *externalSource) reaches(directory, external string) bool {
	pending := []string{directory}
	seen := map[string]bool{directory: true}
	for len(pending) != 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, imported := range s.imports[current] {
			if imported == external {
				return true
			}
			next := "vendor/" + imported
			if strings.HasPrefix(imported, modulePrefix) {
				next = strings.TrimPrefix(imported, modulePrefix)
			}
			if _, retained := s.snapshot.packages[next]; !retained {
				continue
			}
			if seen[next] {
				continue
			}
			seen[next] = true
			pending = append(pending, next)
		}
	}
	return false
}

func (s *externalSource) witness(identity string) error {
	_, err := s.entrySet(identity)
	return err
}

// externalImport proves lexical binding, then recovers the exact import path.
// It cannot borrow another empty-directory import's identity or a local shadow.
func (s *externalSource) externalImport(source sourceExpr, selector *ast.SelectorExpr) (string, bool) {
	directory, imported := s.snapshot.evaluator().callableImport(source, selector)
	if !imported {
		return "", false
	}
	if directory != "" {
		return "", false
	}
	owner, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	for _, declaration := range source.file.Imports {
		identity, err := strconv.Unquote(declaration.Path.Value)
		if err != nil {
			return "", false
		}
		alias := path.Base(identity)
		if declaration.Name != nil {
			alias = declaration.Name.Name
		}
		if alias == owner.Name {
			return identity, true
		}
	}
	return "", false
}
