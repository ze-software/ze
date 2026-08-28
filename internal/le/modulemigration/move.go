// Design: plan/spec-le-is-a-ze-binary.md -- native development-tool actions
package modulemigration

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	pluginGenerator = "internal/le/pluginimports/pluginimports.go"
	generatedAll    = "internal/component/plugin/all/all.go"
)

var sourceAreas = []string{"internal/component", "internal/plugins", "internal/core"}

var residualExtensions = map[string]bool{
	".go": true, ".mk": true, ".sh": true, ".ci": true, ".yang": true,
	".md": true, ".txt": true, ".py": true, ".yaml": true, ".yml": true,
	".json": true, ".toml": true,
}

// MoveOptions controls one package-tree relocation. Apply defaults to false.
type MoveOptions struct {
	Source       string
	Destination  string
	Apply        bool
	AllowRPCDrop bool
}

type textEdit struct {
	Relative string
	Contents []byte
	Mode     os.FileMode
	Count    int
}

type movePlan struct {
	source       string
	destination  string
	module       string
	merge        bool
	conflicts    []string
	imports      []textEdit
	pluginBefore []string
	pluginAfter  []string
	pluginEdit   *textEdit
	rpcPackages  []string
	rpcHazard    bool
	residual     []CountedPath
}

// Move previews or applies a boundary-safe internal package-tree relocation.
func Move(root string, options MoveOptions) (MoveReport, error) {
	plan, err := planMove(root, options.Source, options.Destination)
	if err != nil {
		return MoveReport{Apply: options.Apply, Code: 2}, err
	}
	report := moveReport(plan, options.Apply)
	if !options.Apply {
		return report, nil
	}
	if len(plan.conflicts) != 0 {
		report.Code = 2
		return report, fmt.Errorf("cannot merge, paths exist in both trees: %s", strings.Join(plan.conflicts, ", "))
	}
	if plan.rpcHazard && !options.AllowRPCDrop {
		report.Code = 3
		return report, fmt.Errorf("move would drop RPC registrations outside internal/component; widen generator discovery or pass allow-rpc-drop")
	}
	return applyMove(root, plan, report)
}

func planMove(root, sourceValue, destinationValue string) (movePlan, error) {
	module, err := readModulePath(root)
	if err != nil {
		return movePlan{}, err
	}
	source, err := findMoveSource(root, sourceValue, destinationValue)
	if err != nil {
		return movePlan{}, err
	}
	destination, err := moveDestination(source, destinationValue)
	if err != nil {
		return movePlan{}, err
	}
	if destination == source {
		return movePlan{}, fmt.Errorf("source and destination are both %s", source)
	}
	if strings.HasPrefix(destination, source+"/") {
		return movePlan{}, fmt.Errorf("destination is inside the source tree: %s", destination)
	}

	oldPrefix := module + "/" + source
	newPrefix := module + "/" + destination
	imports, err := planImportEdits(root, oldPrefix, newPrefix)
	if err != nil {
		return movePlan{}, err
	}
	before, after, pluginEdit, err := planPluginEdit(root, source, destination)
	if err != nil {
		return movePlan{}, err
	}
	rpcPackages, err := findRPCPackages(root, source)
	if err != nil {
		return movePlan{}, err
	}
	residual, err := moveResiduals(root, module, source)
	if err != nil {
		return movePlan{}, err
	}
	destinationInfo, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(destination)))
	merge := statErr == nil && destinationInfo.IsDir()
	if statErr != nil && !os.IsNotExist(statErr) {
		return movePlan{}, fmt.Errorf("inspect destination %s: %w", destination, statErr)
	}
	if statErr == nil && !destinationInfo.IsDir() {
		return movePlan{}, fmt.Errorf("destination already exists and is not a directory: %s", destination)
	}
	var conflicts []string
	if merge {
		conflicts, err = mergeConflicts(root, source, destination)
		if err != nil {
			return movePlan{}, err
		}
	}
	return movePlan{
		source: source, destination: destination, module: module, merge: merge,
		conflicts: conflicts, imports: imports, pluginBefore: before, pluginAfter: after,
		pluginEdit: pluginEdit, rpcPackages: rpcPackages,
		rpcHazard: len(rpcPackages) != 0 && strings.HasPrefix(source, "internal/component/") && !strings.HasPrefix(destination, "internal/component/"),
		residual:  residual,
	}, nil
}

func moveReport(plan movePlan, apply bool) MoveReport {
	imports := make([]CountedPath, 0, len(plan.imports))
	for _, edit := range plan.imports {
		imports = append(imports, CountedPath{Path: edit.Relative, Count: edit.Count})
	}
	beforeSet, afterSet := map[string]bool{}, map[string]bool{}
	for _, entry := range plan.pluginBefore {
		beforeSet[entry] = true
	}
	for _, entry := range plan.pluginAfter {
		afterSet[entry] = true
	}
	added, removed := []string{}, []string{}
	for _, entry := range plan.pluginAfter {
		if !beforeSet[entry] {
			added = append(added, entry)
		}
	}
	for _, entry := range plan.pluginBefore {
		if !afterSet[entry] {
			removed = append(removed, entry)
		}
	}
	return MoveReport{
		Apply: apply, Source: plan.source, Destination: plan.destination, Merge: plan.merge,
		Conflicts: append([]string{}, plan.conflicts...), ImportEdits: imports,
		PluginDirs:  PluginDirs{Before: append([]string{}, plan.pluginBefore...), After: append([]string{}, plan.pluginAfter...), Added: added, Removed: removed},
		RPCPackages: append([]string{}, plan.rpcPackages...), RPCHazard: plan.rpcHazard,
		Residual: append([]CountedPath{}, plan.residual...), Goimports: "not run",
		Registrations: RegistrationDelta{Dropped: []string{}, Added: []string{}}, Code: 0,
	}
}

var executePluginGenerator = runPluginGenerator
var executeMoveGoimports = runMoveGoimports

func applyMove(root string, plan movePlan, report MoveReport) (MoveReport, error) {
	before, err := blankImports(filepath.Join(root, filepath.FromSlash(generatedAll)))
	if err != nil && !os.IsNotExist(err) {
		report.Code = 2
		return report, err
	}
	if plan.merge {
		if err := mergeTrees(root, plan.source, plan.destination); err != nil {
			report.Code = 2
			return report, err
		}
	} else {
		destination := filepath.Join(root, filepath.FromSlash(plan.destination))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			report.Code = 2
			return report, fmt.Errorf("create destination parent: %w", err)
		}
		if err := os.Rename(filepath.Join(root, filepath.FromSlash(plan.source)), destination); err != nil {
			report.Code = 2
			return report, fmt.Errorf("move %s to %s: %w", plan.source, plan.destination, err)
		}
	}
	for _, edit := range plan.imports {
		relative := remapRelative(edit.Relative, plan.source, plan.destination)
		if err := writeAtomic(filepath.Join(root, filepath.FromSlash(relative)), edit.Contents, edit.Mode); err != nil {
			report.Code = 2
			return report, err
		}
	}
	if plan.pluginEdit != nil {
		if err := writeAtomic(filepath.Join(root, filepath.FromSlash(plan.pluginEdit.Relative)), plan.pluginEdit.Contents, plan.pluginEdit.Mode); err != nil {
			report.Code = 2
			return report, err
		}
	}

	report.GeneratorCode = executePluginGenerator(root)
	report.Goimports = executeMoveGoimports(root, plan)
	after, readErr := blankImports(filepath.Join(root, filepath.FromSlash(generatedAll)))
	if readErr != nil && !os.IsNotExist(readErr) {
		report.Code = 2
		return report, readErr
	}
	normalized := map[string]bool{}
	for path := range before {
		normalized[remapPackage(path, plan.source, plan.destination)] = true
	}
	for path := range normalized {
		if !after[path] {
			report.Registrations.Dropped = append(report.Registrations.Dropped, path)
		}
	}
	for path := range after {
		if !normalized[path] {
			report.Registrations.Added = append(report.Registrations.Added, path)
		}
	}
	sort.Strings(report.Registrations.Dropped)
	sort.Strings(report.Registrations.Added)
	report.Registrations.Preserved = len(report.Registrations.Dropped) == 0
	if report.GeneratorCode != 0 {
		report.Code = report.GeneratorCode
		return report, fmt.Errorf("plugin generator exited with code %d", report.GeneratorCode)
	}
	if len(report.Registrations.Dropped) != 0 {
		report.Code = 4
		return report, fmt.Errorf("generated registration set dropped %d package(s)", len(report.Registrations.Dropped))
	}
	return report, nil
}

func readModulePath(root string) (string, error) {
	file, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	return "", fmt.Errorf("go.mod: no module line found")
}

func findMoveSource(root, value, destination string) (string, error) {
	if strings.Contains(value, "/") || strings.HasPrefix(value, "internal/") {
		relative, err := cleanRelative(value, "source")
		if err != nil {
			return "", err
		}
		if !strings.HasPrefix(relative, "internal/") {
			return "", fmt.Errorf("source paths must start with internal/: %s", relative)
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("source not found or is not a real directory: %s", relative)
		}
		return relative, nil
	}
	var found []string
	for _, area := range sourceAreas {
		candidate := area + "/" + value
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate))); err == nil && info.IsDir() {
			found = append(found, candidate)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if len(found) == 0 {
		return "", fmt.Errorf("%q not found under internal/component, internal/plugins, or internal/core", value)
	}
	alias := destinationAlias(destination)
	if alias != "" {
		candidate := alias + "/" + value
		var alternatives []string
		for _, path := range found {
			if path != candidate {
				alternatives = append(alternatives, path)
			}
		}
		if len(alternatives) == 1 {
			return alternatives[0], nil
		}
	}
	return "", fmt.Errorf("%q exists in multiple top-level areas (%s); use an explicit source path", value, strings.Join(found, ", "))
}

func destinationAlias(value string) string {
	switch strings.TrimSuffix(strings.TrimSpace(value), "/") {
	case "component", "internal/component":
		return "internal/component"
	case "plugins", "internal/plugins":
		return "internal/plugins"
	case "core", "internal/core":
		return "internal/core"
	default:
		return ""
	}
}

func moveDestination(source, value string) (string, error) {
	leaf := filepath.Base(source)
	if strings.TrimSpace(value) == "" {
		switch source {
		case "internal/component/" + leaf:
			return "internal/plugins/" + leaf, nil
		case "internal/plugins/" + leaf:
			return "internal/component/" + leaf, nil
		default:
			return "", fmt.Errorf("destination is required for core or nested source paths")
		}
	}
	if alias := destinationAlias(value); alias != "" {
		return alias + "/" + leaf, nil
	}
	raw := strings.TrimSuffix(strings.TrimSpace(value), "/")
	for short, full := range map[string]string{"component": "internal/component", "plugins": "internal/plugins", "core": "internal/core"} {
		if strings.HasPrefix(raw, short+"/") {
			raw = full + strings.TrimPrefix(raw, short)
			break
		}
	}
	relative, err := cleanRelative(raw, "destination")
	if err != nil {
		return "", err
	}
	valid := false
	for _, area := range sourceAreas {
		if relative == area || strings.HasPrefix(relative, area+"/") {
			valid = true
			break
		}
	}
	if !valid {
		return "", fmt.Errorf("destination must be component, plugins, core, or an internal/{component,plugins,core}/... path")
	}
	for _, area := range sourceAreas {
		if relative == area {
			return area + "/" + leaf, nil
		}
	}
	return relative, nil
}

func planImportEdits(root, oldPrefix, newPrefix string) ([]textEdit, error) {
	pattern := regexp.MustCompile(`"` + regexp.QuoteMeta(oldPrefix) + `(/[^"\n]*)?"`)
	var edits []textEdit
	err := walkFiles(root, func(relative, absolute string) error {
		if filepath.Ext(relative) != ".go" {
			return nil
		}
		text, mode, textual, err := readText(absolute)
		if err != nil {
			return err
		}
		if !textual {
			return nil
		}
		count := 0
		updated := pattern.ReplaceAllStringFunc(text, func(match string) string {
			count++
			return `"` + newPrefix + strings.TrimPrefix(match, `"`+oldPrefix)
		})
		if count != 0 {
			edits = append(edits, textEdit{Relative: relative, Contents: []byte(updated), Mode: mode, Count: count})
		}
		return nil
	})
	sort.Slice(edits, func(i, j int) bool { return edits[i].Relative < edits[j].Relative })
	return edits, err
}

var pluginDirsPattern = regexp.MustCompile(`(?s)(var pluginDirs = \[\]string\{\n)(.*?)(\n\})`)
var nestedDomainsPattern = regexp.MustCompile(`(?s)(var nestedPluginDomains = \[\]string\{\n)(.*?)(\n\})`)
var quotedValuePattern = regexp.MustCompile(`"([^"]+)"`)

func planPluginEdit(root, source, destination string) ([]string, []string, *textEdit, error) {
	path := filepath.Join(root, filepath.FromSlash(pluginGenerator))
	text, mode, textual, err := readText(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read %s: %w", pluginGenerator, err)
	}
	if !textual {
		return nil, nil, nil, fmt.Errorf("%s is not UTF-8 text", pluginGenerator)
	}
	match := pluginDirsPattern.FindStringSubmatchIndex(text)
	if match == nil {
		return nil, nil, nil, fmt.Errorf("%s: could not locate pluginDirs literal", pluginGenerator)
	}
	before := quotedValues(text[match[4]:match[5]])
	var nested []string
	if nestedMatch := nestedDomainsPattern.FindStringSubmatchIndex(text); nestedMatch != nil {
		nested = quotedValues(text[nestedMatch[4]:nestedMatch[5]])
	}
	after := plannedPluginDirs(before, nested, source, destination)
	if equalStrings(before, after) {
		return before, after, nil, nil
	}
	inner := ""
	for _, entry := range after {
		inner += "\t\"" + entry + "\",\n"
	}
	inner = strings.TrimSuffix(inner, "\n")
	updated := text[:match[4]] + inner + text[match[5]:]
	return before, after, &textEdit{Relative: pluginGenerator, Contents: []byte(updated), Mode: mode}, nil
}

func quotedValues(text string) []string {
	matches := quotedValuePattern.FindAllStringSubmatch(text, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	return out
}

func plannedPluginDirs(entries, nested []string, source, destination string) []string {
	literal := map[string]bool{}
	for _, entry := range entries {
		literal[entry] = true
	}
	var effective []string
	effective = append(effective, entries...)
	for _, domain := range nested {
		effective = append(effective, "internal/component/"+domain+"/plugins")
	}
	discovered := false
	for _, entry := range effective {
		if covers(entry, source) {
			discovered = true
			break
		}
	}
	delete(literal, source)
	if discovered && (strings.HasPrefix(destination, "internal/component/") || strings.HasPrefix(destination, "internal/plugins/")) {
		covered := false
		for entry := range literal {
			if covers(entry, destination) {
				covered = true
			}
		}
		for _, domain := range nested {
			if covers("internal/component/"+domain+"/plugins", destination) {
				covered = true
			}
		}
		if !covered {
			literal[destination] = true
		}
	}
	return sortedKeys(literal)
}

func covers(root, path string) bool { return path == root || strings.HasPrefix(path, root+"/") }

func findRPCPackages(root, source string) ([]string, error) {
	set := map[string]bool{}
	base := filepath.Join(root, filepath.FromSlash(source))
	err := walkFiles(base, func(_ string, absolute string) error {
		if filepath.Ext(absolute) != ".go" || strings.HasSuffix(absolute, "_test.go") {
			return nil
		}
		text, _, textual, err := readText(absolute)
		if err != nil {
			return err
		}
		if textual && strings.Contains(text, "RegisterRPCs(") {
			relative, err := filepath.Rel(root, filepath.Dir(absolute))
			if err != nil {
				return err
			}
			set[filepath.ToSlash(relative)] = true
		}
		return nil
	})
	return sortedKeys(set), err
}

func moveResiduals(root, module, source string) ([]CountedPath, error) {
	quotedImport := `"` + module + "/" + source
	var rows []CountedPath
	err := walkFiles(root, func(relative, absolute string) error {
		if relative == pluginGenerator {
			return nil
		}
		if relative == "plan" || strings.HasPrefix(relative, "plan/") {
			return nil
		}
		if filepath.Base(relative) != "Makefile" && !residualExtensions[filepath.Ext(relative)] {
			return nil
		}
		text, _, textual, err := readText(absolute)
		if err != nil {
			return err
		}
		if !textual {
			return nil
		}
		count := strings.Count(text, source)
		if filepath.Ext(relative) == ".go" {
			count -= strings.Count(text, quotedImport)
		}
		if count > 0 {
			rows = append(rows, CountedPath{Path: relative, Count: count})
		}
		return nil
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	return rows, err
}

func mergeConflicts(root, source, destination string) ([]string, error) {
	base := filepath.Join(root, filepath.FromSlash(source))
	var conflicts []string
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == base {
			return nil
		}
		relative, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		targetInfo, err := os.Lstat(filepath.Join(root, filepath.FromSlash(destination), relative))
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !entry.IsDir() || !targetInfo.IsDir() {
			conflicts = append(conflicts, filepath.ToSlash(relative))
		}
		return nil
	})
	sort.Strings(conflicts)
	return conflicts, err
}

func mergeTrees(root, source, destination string) error {
	base := filepath.Join(root, filepath.FromSlash(source))
	var files []string
	if err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == base || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		files = append(files, relative)
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(files)
	for _, relative := range files {
		from := filepath.Join(base, relative)
		to := filepath.Join(root, filepath.FromSlash(destination), relative)
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("merge %s: %w", filepath.ToSlash(relative), err)
		}
	}
	return os.RemoveAll(base)
}

func blankImports(path string) (map[string]bool, error) {
	text, _, textual, err := readText(path)
	if err != nil {
		return map[string]bool{}, err
	}
	if !textual {
		return map[string]bool{}, nil
	}
	pattern := regexp.MustCompile(`_\s+"([^"]+)"`)
	out := map[string]bool{}
	for _, match := range pattern.FindAllStringSubmatch(text, -1) {
		out[match[1]] = true
	}
	return out, nil
}

func remapRelative(path, source, destination string) string {
	if path == source || strings.HasPrefix(path, source+"/") {
		return destination + strings.TrimPrefix(path, source)
	}
	return path
}

func remapPackage(path, source, destination string) string {
	marker := "/" + source
	index := strings.Index(path, marker)
	if index < 0 {
		return path
	}
	end := index + len(marker)
	if end != len(path) && path[end] != '/' {
		return path
	}
	return path[:index+1] + destination + path[end:]
}

func runPluginGenerator(root string) int {
	command := exec.Command("go", "run", pluginGenerator)
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return exit.ExitCode()
		}
		return 2
	}
	return 0
}

func runMoveGoimports(root string, plan movePlan) string {
	binary, err := exec.LookPath("goimports")
	if err != nil {
		return "not found"
	}
	set := map[string]bool{plan.destination: true}
	for _, edit := range plan.imports {
		if strings.HasPrefix(edit.Relative, plan.source+"/") {
			continue
		}
		if edit.Relative != generatedAll {
			set[edit.Relative] = true
		}
	}
	targets := sortedKeys(set)
	args := []string{"-w", "-local", plan.module}
	args = append(args, targets...)
	command := exec.Command(binary, args...)
	command.Dir = root
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return "failed: " + err.Error()
	}
	return fmt.Sprintf("ran on %d target(s)", len(targets))
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
