// Design: docs/architecture/plugin/feature-gates.md -- synthetic judgment fixtures only.
package coupling

import (
 "errors"
 "go/parser"
 "go/token"
 "io/fs"
 "os"
 "path/filepath"
 "strings"

 pluginimports "github.com/ze-software/ze/internal/le/plugin/imports"
 repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

var scanRoots = []string{"cmd", "internal", "pkg"}

func buildModel(root string, plugins []repoinventory.Plugin) (model, error) {
 gates, err := pluginimports.LoadGateIndex(root)
 if err != nil { return model{}, err }
 files, fset, err := parseTree(root)
 if err != nil { return model{}, err }
 built := model{plugins: plugins, gates: gates}
 built.scan = scanTree(files, fset, func(dir string) bool { return built.owner(dir) != "" })
 return built, nil
}

// parseTree parses every shipped, non-test Go file under the scan roots.
func parseTree(root string) ([]sourceFile, *token.FileSet, error) {
	fset := token.NewFileSet()
	var files []sourceFile
	for _, top := range scanRoots {
		// A scan root the tree does not hold has no source to read; rootExists
		// already refused a tree with no internal/ at all.
		if _, statErr := os.Stat(filepath.Join(root, top)); errors.Is(statErr, fs.ErrNotExist) {
			continue
		}
		walkErr := filepath.WalkDir(filepath.Join(root, top), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if entry.IsDir() {
				if skipped(rel) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") {
				return nil
			}
			if strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if parseErr != nil {
				return parseErr
			}
			files = append(files, sourceFile{dir: filepath.ToSlash(filepath.Dir(rel)), path: rel, file: file})
			return nil
		})
		if walkErr != nil {
			return nil, nil, walkErr
		}
	}
	return files, fset, nil
}

// rootExists reports whether root holds the scan roots, so a wrong root is an
// error rather than a tree with no coupling in it.
func rootExists(root string) error {
	_, err := os.Stat(filepath.Join(root, "internal"))
	return err
}
