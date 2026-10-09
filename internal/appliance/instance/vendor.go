// Design: docs/architecture/appliance/gokrazy-build-pins.md
package instance

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

type vendorModule struct {
	path, version, goVersion string
}

// bindVendoredModules gives the prepared Ze build module the same dependency
// sources as the host vendor build. Dependency replace directives are ignored
// by Go, so these replacements MUST live in the build module itself.
func bindVendoredModules(goModPath string, data []byte) ([]byte, error) {
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil, err
	}
	root := ""
	for _, r := range f.Replace {
		if r.Old.Path == "github.com/ze-software/ze" {
			root = r.New.Path
			break
		}
	}
	if root == "" {
		return data, nil
	}
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("bind vendor: Ze replacement %q must be an absolute directory", root)
	}
	vendor := filepath.Join(root, "vendor")
	declarations, err := os.ReadFile(filepath.Join(vendor, "modules.txt")) //nolint:gosec // trusted checked-in vendor tree under the Ze replacement
	if err != nil {
		return nil, fmt.Errorf("bind vendor: read module declarations: %w", err)
	}
	modules, err := parseVendorModules(string(declarations))
	if err != nil {
		return nil, err
	}
	roots := make(map[string]bool, len(modules))
	for _, m := range modules {
		roots[filepath.Join(vendor, filepath.FromSlash(m.path))] = true
	}
	// Numbered siblings keep nested modules separate from their parent modules.
	// The enclosing prepared instance owns these files and MUST remove them in
	// its cleanup, after gok has finished reading the sources.
	for i, m := range modules {
		dst, err := filepath.Abs(filepath.Join(filepath.Dir(goModPath), "vendor-modules", fmt.Sprint(i)))
		if err != nil {
			return nil, err
		}
		src := filepath.Join(vendor, filepath.FromSlash(m.path))
		if err := linkVendorModule(src, dst, roots); err != nil {
			return nil, fmt.Errorf("bind vendor module %s: %w", m.path, err)
		}
		manifest := "module " + m.path + "\n"
		if m.goVersion != "" {
			manifest += "\ngo " + m.goVersion + "\n"
		}
		if err := os.WriteFile(filepath.Join(dst, GoModName), []byte(manifest), 0o600); err != nil {
			return nil, err
		}
		// modules.txt holds the selected graph. Minimal bridge manifests need no
		// upstream requirements because every vendored version is required here.
		if err := f.AddRequire(m.path, m.version); err != nil {
			return nil, err
		}
		if err := f.AddReplace(m.path, "", dst, ""); err != nil {
			return nil, err
		}
	}
	f.Cleanup()
	return f.Format()
}

// vendorRecord is one `# ` header of modules.txt and the lines under it.
// A record that provides no package only documents a replacement: Go writes
// those for unused and wildcard replace directives, and vendors no source.
type vendorRecord struct {
	module   vendorModule
	wildcard bool // `# path => ...`: a replace directive with no version
	packages bool
}

// parseVendorModules reads Go's module headers and language-version annotations,
// and returns the modules that provide a vendored package, the same build list
// cmd/go/internal/modload derives from modules.txt. A replaced module keeps its
// original path and version: its sources live under vendor/<original path>, and
// the build module requires and replaces that path. An unknown header fails
// preparation rather than dropping a source binding.
func parseVendorModules(data string) ([]vendorModule, error) {
	var records []vendorRecord
	for line := range strings.SplitSeq(data, "\n") {
		if header, ok := strings.CutPrefix(line, "# "); ok {
			record, err := parseVendorHeader(header)
			if err != nil {
				return nil, fmt.Errorf("bind vendor: unsupported module declaration %q: %w", line, err)
			}
			records = append(records, record)
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(records) == 0 {
			return nil, fmt.Errorf("bind vendor: line without a module: %q", line)
		}
		current := &records[len(records)-1]
		if annotations, ok := strings.CutPrefix(line, "## "); ok {
			for annotation := range strings.SplitSeq(annotations, ";") {
				if version, ok := strings.CutPrefix(strings.TrimSpace(annotation), "go "); ok {
					current.module.goVersion = version
				}
			}
			continue
		}
		if current.wildcard {
			return nil, fmt.Errorf("bind vendor: package %q under a replacement without a version", line)
		}
		current.packages = true
	}
	var modules []vendorModule
	for _, r := range records {
		if r.packages {
			modules = append(modules, r.module)
		}
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("bind vendor: modules.txt contains no module that provides a package")
	}
	return modules, nil
}

// parseVendorHeader reads the fields after `# ` in the forms
// cmd/go/internal/modcmd moduleLine writes: `path version`, then optionally
// `=> directory` or `=> path version`; a wildcard replace omits the first version.
func parseVendorHeader(header string) (vendorRecord, error) {
	fields := strings.Fields(header)
	if len(fields) < 2 {
		return vendorRecord{}, fmt.Errorf("want a path and a version or a replacement")
	}
	var record vendorRecord
	record.module.path = fields[0]
	replacement := fields[1:]
	if fields[1] == "=>" {
		record.wildcard = true
		if err := module.CheckPath(fields[0]); err != nil {
			return vendorRecord{}, err
		}
	} else {
		record.module.version = fields[1]
		if err := module.Check(fields[0], fields[1]); err != nil {
			return vendorRecord{}, err
		}
		replacement = fields[2:]
	}
	if len(replacement) == 0 {
		if record.wildcard {
			return vendorRecord{}, fmt.Errorf("replacement has no target")
		}
		return record, nil
	}
	if replacement[0] != "=>" {
		return vendorRecord{}, fmt.Errorf("want => before the replacement")
	}
	switch len(replacement) {
	case 2:
		if !modfile.IsDirectoryPath(replacement[1]) {
			return vendorRecord{}, fmt.Errorf("replacement %q has no version and is not a directory", replacement[1])
		}
		return record, nil
	case 3:
		if err := module.Check(replacement[1], replacement[2]); err != nil {
			return vendorRecord{}, err
		}
		return record, nil
	}
	return vendorRecord{}, fmt.Errorf("replacement wants a directory or a path and a version")
}

// linkVendorModule creates private directories with hardlinks to canonical
// files. Regular files are required by go:embed; source symlinks fail that check.
// The traversal is bounded by the checked-in vendor tree. Nested modules MUST
// be omitted here because each gets its own replacement and import ownership.
func linkVendorModule(src, dst string, roots map[string]bool) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != src && roots[path] {
			return fs.SkipDir
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: expected a regular vendored file", path)
		}
		if d.Name() == GoModName {
			return fmt.Errorf("%s: unexpected vendored module manifest", path)
		}
		return os.Link(path, target) //nolint:gosec // G122: walks the checked-in vendor tree into a build dir this preparer just created; the regular-file check above refuses a symlink
	})
}
