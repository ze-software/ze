// Design: plan/spec-le-is-a-ze-binary.md -- native port of rename_schema_to_yang.py
package yangmigration

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

const skippedSchemaDirectory = "internal/component/config/schema"

// schemaToYang previews or applies schema-to-yang directory moves and syntax updates.
func schemaToYang(root string, apply bool) (Report, error) {
	report := Report{Workflow: WorkflowSchemaToYang, Apply: apply}
	directories, err := discoverSchemaDirectories(root)
	if err != nil {
		return report, err
	}
	oldToNew := make(map[string]string, len(directories))
	movingDirectory := make(map[string]bool, len(directories))
	for _, directory := range directories {
		oldRel := relative(root, directory)
		newRel := relative(root, filepath.Join(filepath.Dir(directory), "yang"))
		oldToNew[oldRel] = newRel
		movingDirectory[filepath.Clean(directory)] = true
		planSchemaDirectory(root, directory, &report)
	}
	removedFiles := make(map[string]bool)
	for _, removal := range report.Removals {
		if filepath.Ext(removal) != "" {
			removedFiles[removal] = true
		}
	}
	goPaths, err := discoverGoFiles(root)
	if err != nil {
		return report, err
	}
	for _, path := range goPaths {
		if removedFiles[relative(root, path)] {
			continue
		}
		report.Scanned++
		before, _, err := readRegular(path)
		if err != nil {
			appendRefusal(&report, root, path, err)
			continue
		}
		after, err := transformSchemaGo(path, before, oldToNew, movingDirectory)
		if err != nil {
			appendRefusal(&report, root, path, err)
			continue
		}
		planEdit(root, path, before, after, &report)
	}
	docPaths, err := walkFiles(filepath.Join(root, "docs"), func(path string) bool { return filepath.Ext(path) == ".md" })
	if err != nil {
		return report, err
	}
	for _, path := range docPaths {
		report.Scanned++
		before, _, err := readRegular(path)
		if err != nil {
			appendRefusal(&report, root, path, err)
			continue
		}
		after := transformSchemaText(before, oldToNew)
		planEdit(root, path, before, after, &report)
	}
	preflightSchemaMoves(root, &report)
	pruneCoalescedSchemaEdits(&report)
	sort.Slice(report.Moves, func(i, j int) bool { return report.Moves[i].Source < report.Moves[j].Source })
	sort.Slice(report.Edits, func(i, j int) bool { return report.Edits[i].Path < report.Edits[j].Path })
	sort.Slice(report.Removals, func(i, j int) bool {
		leftDepth := strings.Count(report.Removals[i], "/")
		rightDepth := strings.Count(report.Removals[j], "/")
		if leftDepth == rightDepth {
			return report.Removals[i] < report.Removals[j]
		}
		return leftDepth > rightDepth
	})
	if err := applyReport(root, &report); err != nil {
		return report, err
	}
	return report, nil
}

func discoverSchemaDirectories(root string) ([]string, error) {
	internal := filepath.Join(root, "internal")
	var directories []string
	err := filepath.WalkDir(internal, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != internal && (entry.Name() == "vendor" || entry.Name() == "tmp" || entry.Name() == "testdata") {
			return filepath.SkipDir
		}
		if entry.Name() != "schema" || relative(root, path) == skippedSchemaDirectory {
			return nil
		}
		children, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, child := range children {
			if child.Type().IsRegular() && filepath.Ext(child.Name()) == ".yang" {
				directories = append(directories, path)
				break
			}
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	sort.Strings(directories)
	return directories, err
}

func planSchemaDirectory(root, sourceDirectory string, report *Report) {
	destinationDirectory := filepath.Join(filepath.Dir(sourceDirectory), "yang")
	entries, err := os.ReadDir(sourceDirectory)
	if err != nil {
		appendRefusal(report, root, sourceDirectory, err)
		return
	}
	merge := false
	if info, err := os.Lstat(destinationDirectory); err == nil {
		if !info.IsDir() {
			report.Refusals = append(report.Refusals, Refusal{Path: relative(root, destinationDirectory), Reason: "yang destination is not a directory"})
			return
		}
		merge = true
	} else if !os.IsNotExist(err) {
		appendRefusal(report, root, destinationDirectory, err)
		return
	}
	for _, entry := range entries {
		source := filepath.Join(sourceDirectory, entry.Name())
		if !entry.Type().IsRegular() {
			report.Refusals = append(report.Refusals, Refusal{Path: relative(root, source), Reason: "schema directory contains a non-regular entry"})
			continue
		}
		if filepath.Ext(entry.Name()) == ".yang" {
			content, _, err := readRegular(source)
			if err != nil {
				appendRefusal(report, root, source, err)
				continue
			}
			if _, err := gyang.Parse(string(content), source); err != nil {
				appendRefusal(report, root, source, fmt.Errorf("parse YANG: %w", err))
				continue
			}
		}
		if merge && (entry.Name() == "embed.go" || entry.Name() == "register.go") {
			report.Removals = append(report.Removals, relative(root, source))
			continue
		}
		report.Moves = append(report.Moves, Move{
			Source:      relative(root, source),
			Destination: relative(root, filepath.Join(destinationDirectory, entry.Name())),
		})
	}
	report.Removals = append(report.Removals, relative(root, sourceDirectory))
}

func preflightSchemaMoves(root string, report *Report) {
	afterByPath := make(map[string][]byte, len(report.Edits))
	for _, edit := range report.Edits {
		afterByPath[edit.Path] = []byte(edit.After)
	}
	for index := range report.Moves {
		move := &report.Moves[index]
		expected, edited := afterByPath[move.Source]
		if !edited {
			content, _, err := readRegular(filepath.Join(root, filepath.FromSlash(move.Source)))
			if err != nil {
				appendRefusal(report, root, filepath.Join(root, filepath.FromSlash(move.Source)), err)
				continue
			}
			expected = content
		}
		destination := filepath.Join(root, filepath.FromSlash(move.Destination))
		actual, _, err := readRegular(destination)
		switch {
		case err == nil && bytes.Equal(expected, actual):
			move.Identical = true
		case err == nil:
			report.Refusals = append(report.Refusals, Refusal{Path: move.Destination, Reason: "destination exists with different post-migration content"})
		case os.IsNotExist(err):
		default:
			report.Refusals = append(report.Refusals, Refusal{Path: move.Destination, Reason: "cannot inspect destination: " + err.Error()})
		}
	}
}

func pruneCoalescedSchemaEdits(report *Report) {
	coalesced := make(map[string]bool)
	for _, move := range report.Moves {
		if move.Identical {
			coalesced[move.Source] = true
		}
	}
	edits := report.Edits[:0]
	for _, edit := range report.Edits {
		if !coalesced[edit.Path] {
			edits = append(edits, edit)
		}
	}
	report.Edits = edits
}

func discoverGoFiles(root string) ([]string, error) {
	var paths []string
	for _, directory := range []string{"internal", "cmd", "pkg", "test"} {
		found, err := walkFiles(filepath.Join(root, directory), func(path string) bool { return filepath.Ext(path) == ".go" })
		if err != nil {
			return nil, err
		}
		paths = append(paths, found...)
	}
	sort.Strings(paths)
	return paths, nil
}

func transformSchemaGo(path string, content []byte, oldToNew map[string]string, movingDirectory map[string]bool) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse Go: %w", err)
	}
	var replacements []textReplacement
	aliasChanges := make(map[string]string)
	usedAliases := make(map[string]string)
	plannedAliases := make(map[string]string)
	for _, importSpec := range file.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			continue
		}
		local := filepath.Base(importPath)
		if importSpec.Name != nil {
			local = importSpec.Name.Name
		}
		usedAliases[local] = importPath
	}
	for _, importSpec := range file.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			continue
		}
		newPath, ok := migratedImportPath(importPath, oldToNew)
		if !ok {
			continue
		}
		replacements = append(replacements, nodeReplacement(fset, importSpec.Path, strconv.Quote(newPath)))
		oldAlias := "schema"
		newAlias := "yang"
		if importSpec.Name != nil {
			oldAlias = importSpec.Name.Name
			switch {
			case oldAlias == "_" || oldAlias == ".":
				continue
			case oldAlias == "schema":
				start := fset.Position(importSpec.Name.Pos()).Offset
				end := fset.Position(importSpec.Path.Pos()).Offset
				replacements = append(replacements, textReplacement{start: start, end: end, text: ""})
			case strings.Contains(oldAlias, "schema"):
				newAlias = strings.ReplaceAll(oldAlias, "schema", "yang")
				replacements = append(replacements, nodeReplacement(fset, importSpec.Name, newAlias))
			default:
				newAlias = oldAlias
			}
		}
		if owner, exists := usedAliases[newAlias]; exists && owner != importPath && newAlias != oldAlias {
			return nil, fmt.Errorf("renamed import alias %q collides with %q", newAlias, owner)
		}
		if _, exists := plannedAliases[newAlias]; exists {
			return nil, fmt.Errorf("migrated imports collide on alias %q", newAlias)
		}
		plannedAliases[newAlias] = newPath
		if oldAlias != newAlias {
			aliasChanges[oldAlias] = newAlias
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.SelectorExpr:
			identifier, ok := current.X.(*ast.Ident)
			if !ok {
				return true
			}
			if changed, ok := aliasChanges[identifier.Name]; ok {
				replacements = append(replacements, nodeReplacement(fset, identifier, changed))
			}
		case *ast.BasicLit:
			if current.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(current.Value)
			if err != nil {
				return true
			}
			changed := string(transformSchemaText([]byte(value), oldToNew))
			if changed != value {
				replacements = append(replacements, nodeReplacement(fset, current, quoteLike(current.Value, changed)))
			}
		}
		return true
	})
	if movingDirectory[filepath.Clean(filepath.Dir(path))] {
		if file.Name.Name == "schema" {
			replacements = append(replacements, nodeReplacement(fset, file.Name, "yang"))
		} else if file.Name.Name == "schema_test" {
			replacements = append(replacements, nodeReplacement(fset, file.Name, "yang_test"))
		}
	}
	return applyTextReplacements(content, replacements)
}

func migratedImportPath(importPath string, oldToNew map[string]string) (string, bool) {
	for oldPath, newPath := range oldToNew {
		oldSlash := filepath.ToSlash(oldPath)
		newSlash := filepath.ToSlash(newPath)
		if importPath == "github.com/ze-software/ze/"+oldSlash {
			return "github.com/ze-software/ze/" + newSlash, true
		}
	}
	return importPath, false
}

func transformSchemaText(content []byte, oldToNew map[string]string) []byte {
	text := string(content)
	oldPaths := make([]string, 0, len(oldToNew))
	for oldPath := range oldToNew {
		oldPaths = append(oldPaths, filepath.ToSlash(oldPath))
	}
	sort.Slice(oldPaths, func(i, j int) bool { return len(oldPaths[i]) > len(oldPaths[j]) })
	for _, oldPath := range oldPaths {
		text = strings.ReplaceAll(text, oldPath, filepath.ToSlash(oldToNew[oldPath]))
	}
	return []byte(text)
}
