// Design: docs/architecture/plugin/feature-gates.md -- selected symbol ownership, not Go alias spelling.
// Related: assemblysyntax.go -- retained assembler definitions and byte ranges.
package repoinventory

import (
	"fmt"
	"go/ast"
	"go/token"
	"path"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type assemblyLinkage struct {
	local  string
	target string
	site   string
}

type assemblyIndex struct {
	definitions  map[string][]assemblyDefinition
	declarations map[string][]*ast.FuncDecl
	objects      map[string]bool
	linkages     []assemblyLinkage
	prefixes     map[*sourcePackage]string
}

// assemblyEvidence scans the finite selected population once per registration
// reader. It retains no mutable state on Snapshot and is safe across readers.
func (s *Snapshot) assemblyEvidence() (*assemblyIndex, error) {
	index := &assemblyIndex{
		definitions: make(map[string][]assemblyDefinition), declarations: make(map[string][]*ast.FuncDecl),
		objects: make(map[string]bool), prefixes: make(map[*sourcePackage]string),
	}
	directories := make([]string, 0, len(s.packages))
	for directory := range s.packages {
		directories = append(directories, directory)
	}
	slices.Sort(directories)
	headers := assemblyHeaderReader{snapshot: s, generated: generatedAssembly{snapshot: s}}
	for _, directory := range directories {
		pkg := s.packages[directory]
		prefix := linkerPackagePrefix(pkg)
		index.prefixes[pkg] = prefix
		for _, source := range pkg.files {
			if err := index.goLinkage(source, prefix, s.fset); err != nil {
				return nil, err
			}
		}
		for _, name := range pkg.assembly {
			if path.Dir(name) != pkg.dir {
				return nil, fmt.Errorf("assembly %s does not share its declared package directory", name)
			}
			unit, err := headers.unit(pkg)
			if err != nil {
				return nil, err
			}
			definitions, err := assemblyDefinitions(s.blobs[name], name, prefix, unit)
			if err != nil {
				return nil, err
			}
			for _, definition := range definitions {
				index.definitions[definition.symbol] = append(index.definitions[definition.symbol], definition)
			}
		}
	}
	return index, nil
}

func (index *assemblyIndex) goLinkage(source SourceFile, prefix string, positions *token.FileSet) error {
	for _, group := range source.File.Comments {
		for _, comment := range group.List {
			if !strings.HasPrefix(comment.Text, "//go:") {
				continue
			}
			fields := strings.Fields(strings.TrimPrefix(comment.Text, "//"))
			if len(fields) == 0 {
				continue
			}
			linkage := fields[0] == "go:linkname" || fields[0] == "go:linknamestd"
			if !linkage {
				if strings.HasPrefix(fields[0], "go:link") {
					return fmt.Errorf("%s: unsupported linkage directive %s", positions.Position(comment.Slash), fields[0])
				}
				if strings.HasPrefix(fields[0], "go:cgo_") {
					return fmt.Errorf("%s: unsupported linkage directive %s", positions.Position(comment.Slash), fields[0])
				}
				continue
			}
			if len(fields) < 2 {
				return fmt.Errorf("%s: malformed linkage directive", positions.Position(comment.Slash))
			}
			if len(fields) > 3 {
				return fmt.Errorf("%s: malformed linkage directive", positions.Position(comment.Slash))
			}
			if !token.IsIdentifier(fields[1]) {
				return fmt.Errorf("%s: unsupported linkage local name", positions.Position(comment.Slash))
			}
			local := prefix + "." + fields[1]
			target := local
			if len(fields) == 3 {
				target = fields[2]
			}
			if strings.ContainsAny(target, "<>") {
				return fmt.Errorf("%s: unsupported linkage ABI form", positions.Position(comment.Slash))
			}
			index.linkages = append(index.linkages, assemblyLinkage{local: local, target: target, site: positions.Position(comment.Slash).String()})
		}
	}
	for _, declaration := range source.File.Decls {
		switch node := declaration.(type) {
		case *ast.FuncDecl:
			if node.Recv != nil {
				continue
			}
			if node.Name.Name == "_" {
				continue
			}
			symbol := prefix + "." + node.Name.Name
			index.declarations[symbol] = append(index.declarations[symbol], node)
		case *ast.GenDecl:
			if node.Tok != token.VAR {
				continue
			}
			for _, item := range node.Specs {
				value, ok := item.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range value.Names {
					if name.Name != "_" {
						index.objects[prefix+"."+name.Name] = true
					}
				}
			}
		}
	}
	return nil
}

// linkerPackagePrefix follows cmd/go's executable-main rule and objabi's
// PathToPrefix encoding. Go-local import aliases never enter this identity.
func linkerPackagePrefix(pkg *sourcePackage) string {
	name := modulePrefix + pkg.dir
	if strings.HasPrefix(pkg.dir, "vendor/") {
		name = strings.TrimPrefix(pkg.dir, "vendor/")
	}
	if pkg.name == "main" {
		name = "main"
	}
	return linkerPathPrefix(name)
}

func linkerPathPrefix(name string) string {
	slash := strings.LastIndexByte(name, '/')
	var prefix textbuf.Buffer
	const hex = "0123456789abcdef"
	for offset := range len(name) {
		ch := name[offset]
		escape := ch <= ' ' || ch == '%' || ch == '"' || ch >= 0x7f || (ch == '.' && offset > slash)
		if escape {
			prefix.Byte('%').Byte(hex[ch>>4]).Byte(hex[ch&15])
		} else {
			prefix.Byte(ch)
		}
	}
	return prefix.String()
}
