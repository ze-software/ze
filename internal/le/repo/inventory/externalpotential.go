// Design: docs/architecture/plugin/feature-gates.md -- proved source nonimportability.
package repoinventory

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/version"
	"path"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
)

// externalPotentialSelection is local parser evidence. A zero value records a
// directory with no target-selected non-cgo Go source.
type externalPotentialSelection struct {
	imports     []string
	selected    bool
	invalid     bool
	languageErr error
}

// externalPotentialDirectories removes only complete selected packages proved
// syntactically impossible or importing an absent package in the closed parent
// namespace, and their importers. Its caller MUST first exclude overlapping
// module/vendor identities, replacements and workspace selection.
// A path exemption or dropping only the malformed file would retain capabilities
// belonging to a package no valid Go import graph can contain.
func externalPotentialDirectories(snapshot *Snapshot, context *build.Context, manifest *modfile.File) (map[string][]string, error) {
	namespaces, err := snapshot.externalModuleNamespaces()
	if err != nil {
		return nil, err
	}
	directories := make([]string, 0, len(snapshot.directories))
	for directory := range snapshot.directories {
		directories = append(directories, directory)
	}
	slices.Sort(directories)
	imports := make(map[string][]string)
	selected := make(map[string]bool)
	excluded := make(map[string]bool)
	languageErrors := make(map[string]error)
	languages := externalPotentialLanguages(snapshot, manifest)
	for _, directory := range directories {
		if snapshot.externalNestedModule(directory) != "" {
			continue
		}
		selection, err := externalPotentialPackage(snapshot, context, directory, languages)
		if err != nil {
			return nil, err
		}
		for _, identity := range selection.imports {
			for _, namespace := range namespaces {
				if err := namespace.check(identity, directory); err != nil {
					return nil, err
				}
			}
		}
		imports[directory] = selection.imports
		selected[directory] = selection.selected
		excluded[directory] = selection.invalid
		languageErrors[directory] = selection.languageErr
	}
	for _, directory := range directories {
		for _, identity := range imports[directory] {
			target := externalPotentialImportDirectory(identity)
			if target == "" {
				continue
			}
			if selected[target] {
				continue
			}
			if !externalParentPackageIdentity(identity) {
				return nil, fmt.Errorf("external source identity import %s from %s selects no retained Go source; retain its exact package selection before publishing inventory", identity, directory)
			}
			if err := languageErrors[directory]; err != nil {
				return nil, err
			}
			excluded[directory] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, directory := range directories {
			if excluded[directory] {
				continue
			}
			for _, identity := range imports[directory] {
				target := externalPotentialImportDirectory(identity)
				if !excluded[target] {
					continue
				}
				if err := languageErrors[directory]; err != nil {
					return nil, err
				}
				excluded[directory] = true
				changed = true
				break
			}
		}
	}
	result := make(map[string][]string, len(snapshot.directories))
	for _, directory := range directories {
		if excluded[directory] {
			if err := languageErrors[directory]; err != nil {
				return nil, err
			}
			if _, certain := snapshot.packages[directory]; certain {
				return nil, fmt.Errorf("external certain import selects nonimportable package %s; repair its selected source before publishing inventory", directory)
			}
			continue
		}
		result[directory] = snapshot.directories[directory]
	}
	return result, nil
}

func externalPotentialPackage(snapshot *Snapshot, context *build.Context, directory string, languages map[string]string) (externalPotentialSelection, error) {
	var selection externalPotentialSelection
	packageName := ""
	for _, name := range snapshot.directories[directory] {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if !strings.HasSuffix(name, ".go") {
			if strings.HasSuffix(name, ".s") {
				if _, err := context.MatchFile(directory, path.Base(name)); err != nil {
					return externalPotentialSelection{}, fmt.Errorf("external source selection %s: %w", name, err)
				}
			}
			continue
		}
		match, err := context.MatchFile(directory, path.Base(name))
		if err != nil {
			return externalPotentialSelection{}, fmt.Errorf("external source selection %s: %w", name, err)
		}
		if !match {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, snapshot.blobs[name], parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			return externalPotentialSelection{}, fmt.Errorf("external source selection %s cannot establish non-cgo source: %w", name, err)
		}
		if importsC(file) {
			continue
		}
		selection.selected = true
		if selection.languageErr == nil {
			selection.languageErr = externalPotentialLanguage(name, directory, file, languages)
		}
		if packageName != "" {
			if packageName != file.Name.Name {
				return externalPotentialSelection{}, fmt.Errorf("external source identity %s has ambiguous package names %s and %s; repair build constraints", directory, packageName, file.Name.Name)
			}
		}
		packageName = file.Name.Name
		_, parseErr := parser.ParseFile(token.NewFileSet(), name, snapshot.blobs[name], parser.ParseComments)
		if parseErr != nil {
			if _, syntax := parseErr.(scanner.ErrorList); !syntax {
				return externalPotentialSelection{}, fmt.Errorf("external retained source %s cannot certify parser failure: %w", name, parseErr)
			}
			if selection.languageErr != nil {
				return externalPotentialSelection{}, selection.languageErr
			}
			selection.invalid = true
		}
		for _, imported := range file.Imports {
			identity, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return externalPotentialSelection{}, fmt.Errorf("external import identity %s: %w", name, err)
			}
			target := externalPotentialImportDirectory(identity)
			if target != "" {
				if module := snapshot.externalNestedModule(target); module != "" {
					return externalPotentialSelection{}, fmt.Errorf("external source identity selects %s across nested module %s/go.mod; retain its exact package selection before publishing inventory", target, module)
				}
			}
			selection.imports = append(selection.imports, identity)
		}
	}
	return selection, nil
}

// externalPotentialImportDirectory maps exact root and vendor providers for
// the nonimportability certificate. The module itself is provided by ".".
// An empty target keeps an external identity outside retained selection.
func externalPotentialImportDirectory(identity string) string {
	if identity == strings.TrimSuffix(modulePrefix, "/") {
		return "."
	}
	if strings.HasPrefix(identity, modulePrefix) {
		return strings.TrimPrefix(identity, modulePrefix)
	}
	first, _, _ := strings.Cut(identity, "/")
	if strings.Contains(first, ".") {
		return "vendor/" + identity
	}
	return ""
}

// externalPotentialLanguages retains each vendored package's own module
// language. The validated vendor manifest, rather than a directory prefix or
// root language, supplies that authority. Missing metadata stays unknown.
func externalPotentialLanguages(snapshot *Snapshot, manifest *modfile.File) map[string]string {
	languages := make(map[string]string)
	if manifest.Go != nil {
		languages[""] = "go" + manifest.Go.Version
	}
	language := ""
	for _, line := range strings.Split(string(snapshot.blobs["vendor/modules.txt"]), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			language = ""
			continue
		}
		if strings.HasPrefix(line, "## ") {
			for _, item := range strings.Split(strings.TrimPrefix(line, "## "), ";") {
				if value, declared := strings.CutPrefix(strings.TrimSpace(item), "go "); declared {
					language = "go" + value
				}
			}
			continue
		}
		if line != "" {
			languages["vendor/"+line] = language
		}
	}
	return languages
}

func externalPotentialLanguage(name, directory string, file *ast.File, languages map[string]string) error {
	declared := languages[""]
	if strings.HasPrefix(directory, "vendor/") {
		declared = languages[directory]
	}
	parserLanguage := version.Lang(runtime.Version())
	if parserLanguage == "" {
		return fmt.Errorf("external source language %s has unknown analyzer parser authority; use a versioned Go analyzer before publishing inventory", name)
	}
	if !version.IsValid(declared) {
		return fmt.Errorf("external source language %s has no exact committed module language; retain its language before publishing inventory", name)
	}
	if file != nil {
		if file.GoVersion != "" {
			if !version.IsValid(file.GoVersion) {
				return fmt.Errorf("external source language %s has unsupported file language %q", name, file.GoVersion)
			}
			if version.Compare(file.GoVersion, declared) > 0 {
				declared = file.GoVersion
			}
		}
	}
	if version.Compare(version.Lang(declared), parserLanguage) > 0 {
		return fmt.Errorf("external source language %s requires %s beyond analyzer parser %s; use a matching analyzer before certifying source nonimportability", name, declared, parserLanguage)
	}
	return nil
}
