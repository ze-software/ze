// Design: plan/spec-le-is-a-ze-binary.md -- native development-tool actions
package modulemigration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc"
)

var modulePathPattern = regexp.MustCompile(`^[a-z0-9.-]+\.[a-z]{2,}(/[A-Za-z0-9_.~-]+)+$`)

var renameSkipPrefixes = []string{"vendor/", ".claude/worktrees/", "tmp/", "bin/", "rfc/audit/"}

var renameExcludedFiles = map[string]bool{".claude/settings.local.json": true}

// RenameOptions controls a repository module-path rename. Apply defaults to false.
type RenameOptions struct {
	Old         string
	New         string
	Apply       bool
	Limit       int
	NoGoimports bool
	NoReseal    bool
}

type renamePlan struct {
	old        string
	new        string
	files      []string
	edits      []textEdit
	moves      []PathMove
	regenerate []CountedPath
	skipped    []SkippedPath
}

// Rename previews or applies an exact module-path rename over tracked text and
// tracked path mirrors. Ignored, untracked, binary, generated protobuf, and
// historical audit data retain their producer-defined handling.
func Rename(root string, options RenameOptions) (RenameReport, error) {
	if options.Limit < 0 {
		return RenameReport{Old: options.Old, New: options.New, Apply: options.Apply, Limit: options.Limit, Code: 2}, fmt.Errorf("limit must be non-negative")
	}
	old := options.Old
	if old == "" {
		var err error
		old, err = readModulePath(root)
		if err != nil {
			return RenameReport{Apply: options.Apply, Code: 2}, err
		}
	}
	if !modulePathPattern.MatchString(old) {
		return RenameReport{Old: old, New: options.New, Apply: options.Apply, Code: 2}, fmt.Errorf("from %q is not a module path (host/owner/name)", old)
	}
	if !modulePathPattern.MatchString(options.New) {
		return RenameReport{Old: old, New: options.New, Apply: options.Apply, Code: 2}, fmt.Errorf("to %q is not a module path (host/owner/name)", options.New)
	}
	if old == options.New {
		return RenameReport{Old: old, New: options.New, Apply: options.Apply, Code: 2}, fmt.Errorf("from and to are both %s", old)
	}
	files, err := gitTrackedFiles(root)
	if err != nil {
		return RenameReport{Old: old, New: options.New, Apply: options.Apply, Code: 2}, err
	}
	plan, err := buildRenamePlan(root, files, old, options.New)
	if err != nil {
		return RenameReport{Old: old, New: options.New, Apply: options.Apply, Code: 2}, err
	}
	report := renameReport(plan, options)
	if !options.Apply {
		return report, nil
	}
	if err := preflightRenameMoves(root, plan.moves); err != nil {
		report.Code = 2
		return report, err
	}
	return applyRename(root, plan, options, report)
}

func buildRenamePlan(root string, files []string, old, new string) (renamePlan, error) {
	plan := renamePlan{old: old, new: new, files: append([]string(nil), files...)}
	for _, relative := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return renamePlan{}, fmt.Errorf("inspect tracked file %s: %w", relative, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		text, mode, textual, err := readText(path)
		if err != nil {
			return renamePlan{}, fmt.Errorf("read tracked file %s: %w", relative, err)
		}
		if !textual {
			continue
		}
		updated, count := rewriteModuleText(text, old, new)
		if count == 0 {
			continue
		}
		if reason := renameSkipReason(relative); reason != "" {
			plan.skipped = append(plan.skipped, SkippedPath{Path: relative, Count: count, Reason: reason})
		} else if strings.HasSuffix(relative, ".pb.go") {
			plan.regenerate = append(plan.regenerate, CountedPath{Path: relative, Count: count})
		} else {
			plan.edits = append(plan.edits, textEdit{Relative: relative, Contents: []byte(updated), Mode: mode, Count: count})
		}
	}
	plan.moves = planRenameMoves(files, old, new)
	plan.moves = existingMoveSources(root, plan.moves)
	sort.Slice(plan.edits, func(i, j int) bool { return plan.edits[i].Relative < plan.edits[j].Relative })
	sort.Slice(plan.regenerate, func(i, j int) bool { return plan.regenerate[i].Path < plan.regenerate[j].Path })
	sort.Slice(plan.skipped, func(i, j int) bool { return plan.skipped[i].Path < plan.skipped[j].Path })
	return plan, nil
}

func renameReport(plan renamePlan, options RenameOptions) RenameReport {
	edits := make([]CountedPath, 0, len(plan.edits))
	occurrences := 0
	for _, edit := range plan.edits {
		edits = append(edits, CountedPath{Path: edit.Relative, Count: edit.Count})
		occurrences += edit.Count
	}
	return RenameReport{
		Old: plan.old, New: plan.new, Apply: options.Apply, Limit: options.Limit, Occurrences: occurrences,
		Edits: edits, Moves: append([]PathMove{}, plan.moves...),
		Regenerate: append([]CountedPath{}, plan.regenerate...), Skipped: append([]SkippedPath{}, plan.skipped...),
		Goimports: "not run", Left: []CountedPath{}, Resealed: []string{}, ResealRefused: []string{},
		ResidualHost: []CountedPath{}, Code: 0,
	}
}

func applyRename(root string, plan renamePlan, options RenameOptions, report RenameReport) (RenameReport, error) {
	for _, edit := range plan.edits {
		if err := writeAtomic(filepath.Join(root, filepath.FromSlash(edit.Relative)), edit.Contents, edit.Mode); err != nil {
			report.Code = 2
			return report, err
		}
		report.ChangedFiles++
	}
	for _, move := range plan.moves {
		source := filepath.Join(root, filepath.FromSlash(move.From))
		destination := filepath.Join(root, filepath.FromSlash(move.To))
		if _, err := os.Stat(source); os.IsNotExist(err) {
			continue
		} else if err != nil {
			report.Code = 2
			return report, fmt.Errorf("inspect move source %s: %w", move.From, err)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			report.Code = 2
			return report, fmt.Errorf("create parent for %s: %w", move.To, err)
		}
		if err := os.Rename(source, destination); err != nil {
			report.Code = 2
			return report, fmt.Errorf("move %s to %s: %w", move.From, move.To, err)
		}
		report.MovedDirs++
		removeEmptyParents(filepath.Dir(source), root)
	}
	if options.NoGoimports {
		report.Goimports = "skipped"
	} else {
		report.Goimports = runRenameGoimports(root, plan.new, renameGoTargets(plan.edits, plan.moves))
	}

	tracked := postMoveTracked(plan.files, plan.moves)
	leftPlan, err := buildRenamePlan(root, tracked, plan.old, plan.new)
	if err != nil {
		report.Code = 2
		return report, err
	}
	for _, edit := range leftPlan.edits {
		report.Left = append(report.Left, CountedPath{Path: edit.Relative, Count: edit.Count})
	}
	if len(report.Left) != 0 {
		report.Code = 1
		return report, fmt.Errorf("%d tracked file(s) still contain the old module path", len(report.Left))
	}

	if !options.NoReseal {
		note := fmt.Sprintf("Mechanical re-stamp by `le module rename`: the module path moved from %s to %s. Each named file was independently proved to differ from HEAD by this rename only; verdicts failing that proof remain stale.", plan.old, plan.new)
		reseal, resealErr := rfc.ResealWithProof(root, func(relative string) bool {
			return renameOnlySinceHead(root, relative, plan.old, plan.new)
		}, note)
		if resealErr != nil {
			report.ResealRefused = append(report.ResealRefused, resealErr.Error())
		} else {
			report.Resealed = append(report.Resealed, reseal.Resealed...)
			report.ResealRefused = append(report.ResealRefused, reseal.Refused...)
		}
	}
	residual, err := renameResidualHost(root, plan.old, tracked)
	if err != nil {
		report.Code = 2
		return report, err
	}
	report.ResidualHost = residual
	return report, nil
}

func gitTrackedFiles(root string) ([]string, error) {
	command := exec.Command("git", "ls-files", "-z")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}
	parts := bytes.Split(output, []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) != 0 {
			files = append(files, filepath.ToSlash(string(part)))
		}
	}
	sort.Strings(files)
	return files, nil
}

func rewriteModuleText(text, old, new string) (string, int) {
	count := strings.Count(text, old)
	text = strings.ReplaceAll(text, old, new)
	oldParts, newParts := strings.Split(old, "/"), strings.Split(new, "/")
	if len(oldParts) != len(newParts) {
		return text, count
	}
	pattern := `"` + regexp.QuoteMeta(oldParts[0]) + `"`
	for _, part := range oldParts[1:] {
		pattern += `(\s*,\s*)"` + regexp.QuoteMeta(part) + `"`
	}
	rx := regexp.MustCompile(pattern)
	matches := rx.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, count
	}
	var out strings.Builder
	at := 0
	for _, match := range matches {
		out.WriteString(text[at:match[0]])
		out.WriteByte('"')
		out.WriteString(newParts[0])
		out.WriteByte('"')
		for index, part := range newParts[1:] {
			capture := 2 + index*2
			out.WriteString(text[match[capture]:match[capture+1]])
			out.WriteByte('"')
			out.WriteString(part)
			out.WriteByte('"')
		}
		at = match[1]
	}
	out.WriteString(text[at:])
	return out.String(), count + len(matches)
}

func renameSkipReason(relative string) string {
	for _, prefix := range renameSkipPrefixes {
		if strings.HasPrefix(relative, prefix) {
			return strings.TrimSuffix(prefix, "/")
		}
	}
	if renameExcludedFiles[relative] {
		return "not-a-module-path"
	}
	return ""
}

func planRenameMoves(files []string, old, new string) []PathMove {
	needle := "/" + old + "/"
	set := map[string]string{}
	for _, relative := range files {
		if renameSkipReason(relative) != "" {
			continue
		}
		marked := "/" + filepath.ToSlash(relative)
		index := strings.Index(marked, needle)
		if index < 0 {
			continue
		}
		source := marked[1 : index+len(needle)-1]
		destination := strings.TrimSuffix(source, old) + new
		set[source] = destination
	}
	sources := make([]string, 0, len(set))
	for source := range set {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	moves := make([]PathMove, 0, len(sources))
	for _, source := range sources {
		moves = append(moves, PathMove{From: source, To: set[source]})
	}
	return moves
}

func postMoveTracked(files []string, moves []PathMove) []string {
	out := make([]string, 0, len(files))
	for _, relative := range files {
		for _, move := range moves {
			if strings.HasPrefix(relative, move.From+"/") {
				relative = move.To + strings.TrimPrefix(relative, move.From)
				break
			}
		}
		out = append(out, relative)
	}
	sort.Strings(out)
	return out
}

func existingMoveSources(root string, moves []PathMove) []PathMove {
	out := make([]PathMove, 0, len(moves))
	for _, move := range moves {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(move.From))); err == nil && info.IsDir() {
			out = append(out, move)
		}
	}
	return out
}

func preflightRenameMoves(root string, moves []PathMove) error {
	root = filepath.Clean(root)
	for _, move := range moves {
		destination := filepath.Join(root, filepath.FromSlash(move.To))
		if _, err := os.Lstat(destination); err == nil {
			return fmt.Errorf("destination already exists: %s", move.To)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect destination %s: %w", move.To, err)
		}
		for parent := filepath.Dir(destination); parent != root; parent = filepath.Dir(parent) {
			info, err := os.Stat(parent)
			if err == nil {
				if !info.IsDir() {
					relative, _ := filepath.Rel(root, parent)
					return fmt.Errorf("destination parent is not a directory: %s", filepath.ToSlash(relative))
				}
				break
			}
			if !os.IsNotExist(err) {
				return fmt.Errorf("inspect destination parent %s: %w", parent, err)
			}
		}
	}
	return nil
}

func removeEmptyParents(directory, root string) {
	root = filepath.Clean(root)
	for directory != root && strings.HasPrefix(directory, root+string(filepath.Separator)) {
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 0 {
			return
		}
		if err := os.Remove(directory); err != nil {
			return
		}
		directory = filepath.Dir(directory)
	}
}

func renameGoTargets(edits []textEdit, moves []PathMove) []string {
	set := map[string]bool{}
	for _, edit := range edits {
		if filepath.Ext(edit.Relative) != ".go" {
			continue
		}
		relative := edit.Relative
		for _, move := range moves {
			if strings.HasPrefix(relative, move.From+"/") {
				relative = move.To + strings.TrimPrefix(relative, move.From)
				break
			}
		}
		name := filepath.Base(relative)
		if name == "all.go" || strings.HasPrefix(name, "all_") && strings.HasSuffix(name, ".go") {
			continue
		}
		set[relative] = true
	}
	return sortedKeys(set)
}

func runRenameGoimports(root, module string, targets []string) string {
	if len(targets) == 0 {
		return "no targets"
	}
	binary, err := exec.LookPath("goimports")
	if err != nil {
		return "not found"
	}
	for start := 0; start < len(targets); start += 400 {
		end := start + 400
		if end > len(targets) {
			end = len(targets)
		}
		args := []string{"-format-only", "-w", "-local", module}
		args = append(args, targets[start:end]...)
		command := exec.Command(binary, args...)
		command.Dir = root
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			return "failed: " + err.Error()
		}
	}
	return fmt.Sprintf("ran on %d file(s)", len(targets))
}

func renameOnlySinceHead(root, relative, old, new string) bool {
	command := exec.Command("git", "show", "HEAD:"+relative)
	command.Dir = root
	head, err := command.Output()
	if err != nil {
		return false
	}
	current, _, textual, err := readText(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil || !textual {
		return false
	}
	return normalizeRFCText(strings.ReplaceAll(string(head), old, new)) == normalizeRFCText(current)
}

func normalizeRFCText(text string) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func renameResidualHost(root, old string, files []string) ([]CountedPath, error) {
	host := strings.SplitN(old, "/", 2)[0]
	var rows []CountedPath
	for _, relative := range files {
		skipped := false
		for _, prefix := range renameSkipPrefixes {
			if strings.HasPrefix(relative, prefix) {
				skipped = true
				break
			}
		}
		if skipped {
			continue
		}
		text, _, textual, readErr := readText(filepath.Join(root, filepath.FromSlash(relative)))
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			return nil, readErr
		}
		if textual {
			if count := strings.Count(text, host); count != 0 {
				rows = append(rows, CountedPath{Path: relative, Count: count})
			}
		}
	}
	return rows, nil
}
