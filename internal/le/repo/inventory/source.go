// Design: docs/architecture/plugin/feature-gates.md -- one committed Linux source population.
// Related: expressions.go -- registration metadata over these retained syntax trees.
package repoinventory

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/modfile"

	pluginimports "github.com/ze-software/ze/internal/le/plugin/imports"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	repoobjects "github.com/ze-software/ze/internal/le/repo/objects"
)

// SourceFile is syntax selected by the declared Linux le build context. Callers
// MUST treat File as immutable; the snapshot retains it for metadata resolution.
type SourceFile struct {
	Path string
	File *ast.File
}

// Snapshot holds one resolved HEAD and its retained source bytes. Its zero value
// is not initialized; callers MUST obtain a pointer through ReadCommitted. Safe
// for concurrent read-only use after construction; metadata resolution is local
// to each caller and never changes its parsed source trees.
type Snapshot struct {
	revision           string
	blobs              map[string][]byte
	paths              []string
	directories        map[string][]string
	files              []SourceFile
	fset               *token.FileSet
	packages           map[string]*sourcePackage
	gates              pluginimports.GateIndex
	headerPresence     map[string]bool
	assemblyInvalid    map[string]string
	assemblyPredefines []string
}

type sourcePackage struct {
	dir               string
	name              string
	files             []SourceFile
	imports           map[*ast.File]map[string]string
	values            map[string]sourceExpr
	functions         map[string]sourceFunction
	initializers      []sourceFunction
	initializerValues []sourceExpr
	assembly          []string
}

type sourceExpr struct {
	value ast.Expr
	file  *ast.File
	pkg   *sourcePackage
	name  string
}

type sourceFunction struct {
	decl *ast.FuncDecl
	file *ast.File
	pkg  *sourcePackage
}

// ReadCommitted resolves HEAD exactly once and captures the source blobs before
// selecting any membership, metadata, feature gates or coupling source. The
// declared entry is cmd/ze with ze_le, every manifest gate, Linux/amd64 and no
// cgo, independent of the operating system compiling or running this reader.
func ReadCommitted(ctx context.Context, root string) (*Snapshot, error) {
	revision, err := repoobjects.Resolve(ctx, root, "HEAD")
	if err != nil {
		return nil, err
	}
	population, err := sourceObjects(ctx, root, revision)
	if err != nil {
		return nil, err
	}
	payloads, err := repoobjects.Read(ctx, root, population.ids)
	if err != nil {
		return nil, err
	}
	snapshot := &Snapshot{
		revision: revision, paths: population.reportPaths, blobs: make(map[string][]byte, len(population.payloadPaths)),
		fset: token.NewFileSet(), packages: make(map[string]*sourcePackage), directories: make(map[string][]string),
		headerPresence: population.headers, assemblyInvalid: population.invalidAssembly,
	}
	for index, name := range population.payloadPaths {
		snapshot.blobs[name] = payloads[index]
	}
	for _, names := range [][]string{population.reportPaths, population.assemblyPaths} {
		for _, name := range names {
			directory := path.Dir(name)
			snapshot.directories[directory] = append(snapshot.directories[directory], name)
		}
	}
	manifest, found := snapshot.blobs["feature-gates.txt"]
	if !found {
		return nil, fmt.Errorf("committed %s has no feature-gates.txt; commit the declared feature population before reading inventory", revision)
	}
	gates, err := repofeaturetags.ParseGates(revision+":feature-gates.txt", manifest)
	if err != nil {
		return nil, err
	}
	snapshot.gates = pluginimports.GateIndexFrom(gates)
	buildContext := linuxContext(gates, snapshot.blobs)
	snapshot.assemblyPredefines, err = assemblyContextBindings(&buildContext)
	if err != nil {
		return nil, err
	}
	module := modfile.ModulePath(snapshot.blobs["go.mod"])
	if module != strings.TrimSuffix(modulePrefix, "/") {
		return nil, fmt.Errorf("committed %s declares module %q, not %s; select the Ze repository", revision, module, strings.TrimSuffix(modulePrefix, "/"))
	}
	if err := snapshot.selectPackage("cmd/ze", &buildContext); err != nil {
		return nil, err
	}
	slices.SortFunc(snapshot.files, func(a, b SourceFile) int { return strings.Compare(a.Path, b.Path) })
	return snapshot, nil
}

// Revision identifies the commit supplying every input of this snapshot.
func (s *Snapshot) Revision() string { return s.revision }

// Sources returns selected source syntax and its positions. Callers MUST NOT
// mutate the returned ASTs or FileSet; only the returned slice itself is owned.
func (s *Snapshot) Sources() ([]SourceFile, *token.FileSet) {
	return slices.Clone(s.files), s.fset
}

// Gates returns the manifest index built from this snapshot's retained blob.
func (s *Snapshot) Gates() pluginimports.GateIndex { return s.gates }

// sourcePopulation keeps assembly evidence separate from report inputs. Header
// presence is sufficient to refuse a shadow; no header payload is needed.
type sourcePopulation struct {
	reportPaths     []string
	payloadPaths    []string
	assemblyPaths   []string
	ids             []string
	headers         map[string]bool
	invalidAssembly map[string]string
}

func sourceObjects(ctx context.Context, root, revision string) (sourcePopulation, error) {
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, "git", "--no-replace-objects", "-C", root, "ls-tree", "-r", "-t", "-z", "--full-tree", revision) //nolint:gosec // revision is a resolved commit ID; no shell is used.
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return sourcePopulation{}, fmt.Errorf("cannot enumerate committed inventory %s; repair repository history: %w: %s", revision, err, stderr.String())
	}
	population := sourcePopulation{headers: make(map[string]bool), invalidAssembly: make(map[string]string)}
	for record := range bytes.SplitSeq(output, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		header, name, found := bytes.Cut(record, []byte{'\t'})
		if !found {
			return sourcePopulation{}, fmt.Errorf("malformed committed inventory tree at %s; repair repository objects", revision)
		}
		rel := string(name)
		fields := strings.Fields(string(header))
		if len(fields) != 3 {
			return sourcePopulation{}, fmt.Errorf("invalid committed inventory entry %q; repair repository objects", rel)
		}
		if assemblyHeaderNamed(path.Base(rel)) != nil {
			population.headers[rel] = true
		}
		if fields[1] == "tree" {
			continue
		}
		report := inventoryInput(rel)
		identity := rel == "go.work" || strings.HasSuffix(rel, "/go.mod") || rel == "vendor/modules.txt"
		assembly := strings.HasSuffix(rel, ".s")
		if !report {
			if !assembly {
				if !identity {
					continue
				}
			}
		}
		regular := fields[1] == "blob" && (fields[0] == "100644" || fields[0] == "100755")
		if assembly {
			population.assemblyPaths = append(population.assemblyPaths, rel)
			if !regular {
				population.invalidAssembly[rel] = fields[0]
				continue
			}
		}
		if !regular {
			return sourcePopulation{}, fmt.Errorf("committed inventory source %q has type %s mode %s; commit a regular source file", rel, fields[1], fields[0])
		}
		if report {
			population.reportPaths = append(population.reportPaths, rel)
		}
		population.payloadPaths = append(population.payloadPaths, rel)
		population.ids = append(population.ids, fields[2])
	}
	return population, nil
}

func inventoryInput(name string) bool {
	if name == "go.mod" || name == "feature-gates.txt" {
		return true
	}
	if strings.HasSuffix(name, ".go") {
		return true
	}
	for _, area := range codeAreas {
		if strings.HasPrefix(name, area+"/") && strings.HasSuffix(name, yangSuffix) {
			return true
		}
	}
	return strings.HasPrefix(name, "test/") && strings.HasSuffix(name, ".ci")
}

func linuxContext(gates []repofeaturetags.Gate, blobs map[string][]byte) build.Context {
	return normalizeLinuxContext(&build.Default, gates, blobs)
}

// normalizeLinuxContext replaces host selectors with the declared target,
// retaining only toolchain release and experiment information from the seed.
func normalizeLinuxContext(seed *build.Context, gates []repofeaturetags.Gate, blobs map[string][]byte) build.Context {
	tags := []string{repofeaturetags.LEBase}
	for _, gate := range gates {
		tags = append(tags, gate.Tag)
	}
	slices.Sort(tags)
	tags = slices.Compact(tags)
	toolTags := []string{"amd64.v1"}
	for _, tag := range seed.ToolTags {
		if strings.HasPrefix(tag, "goexperiment.") {
			toolTags = append(toolTags, tag)
		}
	}
	return build.Context{
		GOOS: "linux", GOARCH: "amd64", Compiler: "gc", CgoEnabled: false,
		GOROOT: seed.GOROOT, BuildTags: tags,
		ToolTags: toolTags, ReleaseTags: slices.Clone(seed.ReleaseTags),
		JoinPath: path.Join,
		OpenFile: func(name string) (io.ReadCloser, error) {
			raw, found := blobs[name]
			if !found {
				return nil, fmt.Errorf("committed source blob %q is missing; restore repository objects", name)
			}
			return io.NopCloser(bytes.NewReader(raw)), nil
		},
	}
}

// selectPackage follows a finite committed import graph; the package map bounds
// recursion and breaks already-visited edges before descending into imports.
func (s *Snapshot) selectPackage(dir string, context *build.Context) error {
	if _, seen := s.packages[dir]; seen {
		return nil
	}
	pkg := &sourcePackage{dir: dir, imports: make(map[*ast.File]map[string]string), values: make(map[string]sourceExpr), functions: make(map[string]sourceFunction)}
	s.packages[dir] = pkg
	for _, name := range s.directories[dir] {
		if strings.HasSuffix(name, ".s") {
			match, err := context.MatchFile(dir, path.Base(name))
			if err != nil {
				if mode, invalid := s.assemblyInvalid[name]; invalid {
					return fmt.Errorf("committed assembly %s has mode %s; commit a regular assembly source", name, mode)
				}
				return fmt.Errorf("select committed assembly %s: %w", name, err)
			}
			if match {
				pkg.assembly = append(pkg.assembly, name)
			}
			continue
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		match, err := context.MatchFile(dir, path.Base(name))
		if err != nil {
			return fmt.Errorf("select committed source %s: %w", name, err)
		}
		if !match {
			continue
		}
		file, err := parser.ParseFile(s.fset, name, s.blobs[name], parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse committed source %s: %w", name, err)
		}
		if importsC(file) {
			continue
		}
		if pkg.name != "" && pkg.name != file.Name.Name {
			return fmt.Errorf("committed Linux package %s has ambiguous package names %s and %s; repair build constraints", dir, pkg.name, file.Name.Name)
		}
		pkg.name = file.Name.Name
		source := SourceFile{Path: name, File: file}
		pkg.files = append(pkg.files, source)
		s.files = append(s.files, source)
	}
	if len(pkg.files) == 0 {
		return fmt.Errorf("committed Linux le imports %s but selects no Go source; commit its sources and correct the feature constraints", dir)
	}
	for _, source := range pkg.files {
		aliases := make(map[string]string)
		for _, imported := range source.File.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return fmt.Errorf("read import in %s: %w", source.Path, err)
			}
			target := ""
			if strings.HasPrefix(name, modulePrefix) {
				target = strings.TrimPrefix(name, modulePrefix)
			} else if strings.Contains(strings.SplitN(name, "/", 2)[0], ".") {
				target = "vendor/" + name
			}
			alias := path.Base(name)
			if target != "" {
				if err := s.selectPackage(target, context); err != nil {
					return err
				}
				alias = s.packages[target].name
			}
			if imported.Name != nil {
				alias = imported.Name.Name
			}
			aliases[alias] = target
		}
		pkg.imports[source.File] = aliases
	}
	return pkg.indexDeclarations()
}

func importsC(file *ast.File) bool {
	for _, imported := range file.Imports {
		if imported.Path.Value == `"C"` {
			return true
		}
	}
	return false
}

func (p *sourcePackage) indexDeclarations() error {
	for _, source := range p.files {
		for _, declaration := range source.File.Decls {
			switch node := declaration.(type) {
			case *ast.FuncDecl:
				if node.Recv != nil || node.Name.Name == "_" {
					continue
				}
				function := sourceFunction{decl: node, file: source.File, pkg: p}
				if node.Name.Name == "init" {
					p.initializers = append(p.initializers, function)
					continue
				}
				if _, duplicate := p.functions[node.Name.Name]; duplicate {
					return fmt.Errorf("ambiguous committed function %s.%s; repair build constraints", p.dir, node.Name.Name)
				}
				p.functions[node.Name.Name] = function
			case *ast.GenDecl:
				var previous []ast.Expr
				for _, spec := range node.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					if node.Tok == token.VAR {
						for _, initializer := range value.Values {
							p.initializerValues = append(p.initializerValues, sourceExpr{value: initializer, file: source.File, pkg: p})
						}
					}
					values := value.Values
					if len(values) == 0 && node.Tok == token.CONST {
						values = previous
					}
					for index, name := range value.Names {
						if name.Name == "_" {
							continue
						}
						if _, duplicate := p.values[name.Name]; duplicate {
							return fmt.Errorf("ambiguous committed value %s.%s; repair build constraints", p.dir, name.Name)
						}
						if index < len(values) {
							p.values[name.Name] = sourceExpr{value: values[index], file: source.File, pkg: p, name: name.Name}
						} else {
							p.values[name.Name] = sourceExpr{file: source.File, pkg: p, name: name.Name}
						}
					}
					previous = values
				}
			}
		}
	}
	return nil
}
