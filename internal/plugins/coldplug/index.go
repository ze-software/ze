// Design: docs/architecture/appliance/kernel-profiles.md -- Loading driver modules on the device: the module index
// Overview: coldplug.go -- the cold-plug pass that reads this index

package coldplug

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The four index files `make modules_install` and depmod write under
// /lib/modules/<release>.
const (
	aliasFile   = "modules.alias"
	depFile     = "modules.dep"
	builtinFile = "modules.builtin"
	softdepFile = "modules.softdep"
)

// indexFileBytesMax bounds one index file. A distribution kernel's
// modules.alias is about 1.5 MiB; a file past this is not an index.
const indexFileBytesMax = 64 << 20

// errNoIndex says the kernel has no module index to read: no
// /lib/modules/<release> tree, or one without modules.alias or modules.dep.
// It is the answer of a kernel built with every driver in it, a distribution
// kernel whose modules are not installed, or a container, and it is not a
// fault, so the pass does nothing and says nothing alarming.
var errNoIndex = errors.New("no kernel module index")

// moduleIndex is the parsed module index of one kernel release. Every module
// it names is keyed by its module name, the file name without ".ko" and with
// '-' read as '_', which is the spelling /proc/modules and modules.alias use.
type moduleIndex struct {
	aliases  []aliasRule
	modules  map[string]moduleFile
	builtin  map[string]bool
	softdeps map[string]softdep
}

// aliasRule is one modules.alias line: a device that matches pattern is
// driven by module.
type aliasRule struct {
	pattern string
	// literal is the pattern's prefix up to its first special byte. A
	// modalias that does not start with it cannot match, which rules out
	// most patterns before the glob runs.
	literal string
	module  string
}

// moduleFile is one modules.dep line: the module's file, relative to the
// modules root, and the modules it needs loaded before it, by name.
type moduleFile struct {
	path string
	deps []string
}

// softdep is one modules.softdep line: modules to load before (pre) and after
// (post) a module, which the module works without but works worse.
type softdep struct {
	pre  []string
	post []string
}

// readIndex parses the module index under modulesRoot. It answers errNoIndex
// when the tree or one of its two required files is absent, and any other
// error when an index file is present but cannot be read or does not parse:
// that is a broken install, and the caller reports it.
func readIndex(modulesRoot string) (moduleIndex, error) {
	modules, err := readIndexFile(modulesRoot, depFile, true, parseDepLine)
	if err != nil {
		return moduleIndex{}, err
	}
	aliases, err := readIndexFile(modulesRoot, aliasFile, true, parseAliasLine)
	if err != nil {
		return moduleIndex{}, err
	}
	builtin, err := readIndexFile(modulesRoot, builtinFile, false, parseBuiltinLine)
	if err != nil {
		return moduleIndex{}, err
	}
	softdeps, err := readIndexFile(modulesRoot, softdepFile, false, parseSoftdepLine)
	if err != nil {
		return moduleIndex{}, err
	}
	idx := moduleIndex{
		aliases:  aliases,
		modules:  make(map[string]moduleFile, len(modules)),
		builtin:  make(map[string]bool, len(builtin)),
		softdeps: make(map[string]softdep, len(softdeps)),
	}
	for _, entry := range modules {
		idx.modules[entry.name] = entry.file
	}
	for _, name := range builtin {
		idx.builtin[name] = true
	}
	for _, entry := range softdeps {
		idx.softdeps[entry.name] = entry.dep
	}
	return idx, nil
}

// readIndexFile reads one index file and parses each line that is neither
// blank nor a '#' comment. A required file that is absent answers errNoIndex;
// an optional one answers no entries, which is what depmod writes for a
// kernel with no built-in module or no soft dependency.
func readIndexFile[T any](modulesRoot, name string, required bool, parse func(string) (T, error)) ([]T, error) {
	path := filepath.Join(modulesRoot, name)
	data, err := readBounded(path)
	if errors.Is(err, fs.ErrNotExist) {
		if required {
			return nil, fmt.Errorf("%w: %s is absent", errNoIndex, path)
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []T
	for number, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line[0] == '#' {
			continue
		}
		entry, parseErr := parse(line)
		if parseErr != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, number+1, parseErr)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// readBounded reads a whole index file, refusing one past indexFileBytesMax.
func readBounded(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > indexFileBytesMax {
		return nil, fmt.Errorf("%s is %d bytes, more than the %d an index file can hold", path, info.Size(), indexFileBytesMax)
	}
	return os.ReadFile(path) //nolint:gosec // a file of the running kernel's own module index
}

// parseAliasLine reads "alias <pattern> <module>".
func parseAliasLine(line string) (aliasRule, error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return aliasRule{}, fmt.Errorf("want \"alias <pattern> <module>\", got %q", line)
	}
	if fields[0] != "alias" {
		return aliasRule{}, fmt.Errorf("want \"alias <pattern> <module>\", got %q", line)
	}
	pattern := fields[1]
	literal := pattern
	if special := strings.IndexAny(pattern, `*?[\`); special >= 0 {
		literal = pattern[:special]
	}
	return aliasRule{pattern: pattern, literal: literal, module: moduleName(fields[2])}, nil
}

// depEntry is one parsed modules.dep line, before it is keyed by name.
type depEntry struct {
	name string
	file moduleFile
}

// parseDepLine reads "<path>: <dependency path> ...".
func parseDepLine(line string) (depEntry, error) {
	path, rest, found := strings.Cut(line, ":")
	if !found {
		return depEntry{}, fmt.Errorf("want \"<module path>: <dependency paths>\", got %q", line)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return depEntry{}, fmt.Errorf("want \"<module path>: <dependency paths>\", got %q", line)
	}
	file := moduleFile{path: path}
	for dep := range strings.FieldsSeq(rest) {
		file.deps = append(file.deps, moduleName(dep))
	}
	return depEntry{name: moduleName(path), file: file}, nil
}

// parseBuiltinLine reads one built-in module's path.
func parseBuiltinLine(line string) (string, error) {
	return moduleName(line), nil
}

// softdepEntry is one parsed modules.softdep line, before it is keyed by
// name.
type softdepEntry struct {
	name string
	dep  softdep
}

// parseSoftdepLine reads "softdep <module> pre: <modules> post: <modules>",
// where either list may be absent.
func parseSoftdepLine(line string) (softdepEntry, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return softdepEntry{}, fmt.Errorf("want \"softdep <module> pre: ... post: ...\", got %q", line)
	}
	if fields[0] != "softdep" {
		return softdepEntry{}, fmt.Errorf("want \"softdep <module> pre: ... post: ...\", got %q", line)
	}
	entry := softdepEntry{name: moduleName(fields[1])}
	var list *[]string
	for _, field := range fields[2:] {
		switch field {
		case "pre:":
			list = &entry.dep.pre
		case "post:":
			list = &entry.dep.post
		default:
			if list == nil {
				return softdepEntry{}, fmt.Errorf("module %q comes before any \"pre:\" or \"post:\" in %q", field, line)
			}
			*list = append(*list, moduleName(field))
		}
	}
	return entry, nil
}

// moduleName answers the module name a path or a name spells: the base name
// without its ".ko" suffix and any compression suffix after it, with '-'
// read as '_', as the kernel itself names modules.
func moduleName(spelling string) string {
	name := filepath.Base(spelling)
	if dot := strings.Index(name, ".ko"); dot >= 0 {
		name = name[:dot]
	}
	return strings.ReplaceAll(name, "-", "_")
}
