// Design: docs/architecture/core-design.md -- native le tool implementation
//
// Package journal reads plan/journal class files from git HEAD. The working
// tree is used only to name class files that the committed count did not read.
// A malformed row or date is a refusal, never an omitted occurrence.
package journal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	area          = "journal"
	journalDir    = "plan/journal"
	journalPrefix = "plan/journal/"
	journalReadme = "README.md"
	markdownExt   = ".md"
)

var journalHeader = [...]string{"date", "spec", "surface", "symptom", "fix"}

type journalRow struct {
	cells     [5]string
	malformed bool
}

type journalClass struct {
	name string
	rows []journalRow
}

type gitAnswer struct {
	stdout   []byte
	stderr   string
	code     int
	startErr error
}

// reportHere runs the report action over this checkout.
func reportHere() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		var tb textbuf.Buffer
		fmt.Fprintln(os.Stderr, tb.Str("journal: ").Err(err).String()) //nolint:errcheck // CLI output
		return nil, 2
	}

	report, code := Run(tree, os.Stderr)
	if code == 2 {
		return nil, code
	}
	return report, code
}

// Run reads one checkout, writes its warnings and refusals to errOut, and
// returns its structured report and exact gate code.
func Run(tree string, errOut io.Writer) (Report, int) {
	report, err := Check(tree)
	if err != nil {
		var tb textbuf.Buffer
		if _, writeErr := fmt.Fprintln(errOut, tb.Str("journal: ").Err(err).String()); writeErr != nil {
			return Report{}, 2
		}
		return Report{}, 2
	}

	for _, path := range report.Unread {
		var tb textbuf.Buffer
		if _, err := fmt.Fprintln(errOut, tb.Str("NOT AT HEAD: ").Str(path).
			Str(" is on disk and not committed, so its rows are not in the counts above").String()); err != nil {
			return report, 2
		}
	}
	for _, problem := range report.Problems {
		if _, err := fmt.Fprintln(errOut, problem.Text()); err != nil {
			return report, 2
		}
	}
	if len(report.Problems) > 0 {
		return report, 1
	}
	return report, 0
}

// Check reads every journal class at git HEAD and returns the trustworthy
// recurring classes, worktree-only files, and refused classes.
func Check(tree string) (Report, error) {
	paths, err := classFilesAtHead(tree)
	if err != nil {
		return Report{}, err
	}
	onDisk, err := classFilesOnDisk(tree)
	if err != nil {
		return Report{}, err
	}

	if len(paths) == 0 {
		if len(onDisk) > 0 {
			var tb textbuf.Buffer
			return Report{}, errors.New(tb.Str(journalDir).
				Str("/ exists in the working tree but HEAD carries no journal class file.\n").
				Str("  This detector reads HEAD, so it would count zero rows over an uncommitted journal\n").
				Str("  and report no recurrence at all.  Commit ").Str(journalDir).Str("/.").String())
		}
		return Report{}, nil
	}

	report := Report{Unread: unreadPaths(paths, onDisk)}
	classes, err := readClasses(tree, paths)
	if err != nil {
		return Report{}, err
	}
	for _, class := range classes {
		appendClass(&report, class)
	}
	return report, nil
}

func classFilesAtHead(tree string) ([]string, error) {
	answer := runGit(tree, "ls-tree", "--name-only", "-r", "HEAD", "--", journalDir)
	if answer.startErr != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("cannot run git in ").Str(tree).Str(": ").Err(answer.startErr).String())
	}
	if answer.code != 0 {
		return nil, gitReadError("git ls-tree HEAD -- plan/journal failed: ", answer)
	}

	var paths []string
	for line := range strings.SplitSeq(string(answer.stdout), "\n") {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		if !strings.HasSuffix(path, markdownExt) {
			continue
		}
		if strings.HasSuffix(path, "/README.md") {
			continue
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func classFilesOnDisk(tree string) ([]string, error) {
	dir := filepath.Join(tree, filepath.FromSlash(journalDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == journalReadme {
			continue
		}
		if !strings.HasSuffix(name, markdownExt) {
			continue
		}
		paths = append(paths, filepath.ToSlash(filepath.Join(journalDir, name)))
	}
	sort.Strings(paths)
	return paths, nil
}

func unreadPaths(head, onDisk []string) []string {
	committed := make(map[string]struct{}, len(head))
	for _, path := range head {
		committed[path] = struct{}{}
	}

	var unread []string
	for _, path := range onDisk {
		if _, ok := committed[path]; ok {
			continue
		}
		unread = append(unread, path)
	}
	return unread
}

func readClasses(tree string, paths []string) ([]journalClass, error) {
	classes := make([]journalClass, 0, len(paths))
	for _, path := range paths {
		answer := runGit(tree, "show", headObject(path))
		if answer.startErr != nil {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str("cannot run git in ").Str(tree).Str(": ").Err(answer.startErr).String())
		}
		if answer.code != 0 {
			var prefix textbuf.Buffer
			prefix.Str("git show HEAD:").Str(path).Str(" failed: ")
			return nil, gitReadError(prefix.String(), answer)
		}

		rows := journalRows(string(answer.stdout))
		if len(rows) == 0 {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(path, journalPrefix), markdownExt)
		classes = append(classes, journalClass{name: name, rows: rows})
	}
	sort.Slice(classes, func(left, right int) bool { return classes[left].name < classes[right].name })
	return classes, nil
}

func headObject(path string) string {
	var tb textbuf.Buffer
	return tb.Str("HEAD:").Str(path).String()
}

func runGit(tree string, args ...string) gitAnswer {
	// Git is the gate's data source. The command is bounded by local repository
	// I/O and has no network or lock wait. A timeout could turn a slow disk into
	// a false report that HEAD has no journal.
	cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // fixed git verbs over a checkout chosen by lepath or a test
	cmd.Dir = tree
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	answer := gitAnswer{stdout: stdout, stderr: strings.TrimSpace(stderr.String())}
	if err == nil {
		return answer
	}

	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		answer.code = exit.ExitCode()
		return answer
	}
	answer.startErr = err
	return answer
}

func gitReadError(prefix string, answer gitAnswer) error {
	var tb textbuf.Buffer
	tb.Str(prefix)
	if answer.stderr != "" {
		return errors.New(tb.Str(answer.stderr).String())
	}
	return errors.New(tb.Str("exit ").Int(int64(answer.code)).String())
}

func journalRows(text string) []journalRow {
	var rows []journalRow
	for line := range strings.SplitSeq(text, "\n") {
		row, ok := parseJournalRow(line)
		if ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func parseJournalRow(line string) (journalRow, bool) {
	if !strings.ContainsRune(line, '|') {
		return journalRow{}, false
	}
	if !strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), "|") {
		return journalRow{malformed: true}, true
	}

	fields := strings.Split(line, "|")
	if len(fields) != 7 {
		return journalRow{malformed: true}, true
	}
	if strings.TrimSpace(fields[0]) != "" {
		return journalRow{malformed: true}, true
	}
	if strings.TrimSpace(fields[6]) != "" {
		return journalRow{malformed: true}, true
	}

	var row journalRow
	for index := range row.cells {
		row.cells[index] = strings.TrimSpace(fields[index+1])
	}
	if isJournalHeader(row.cells) {
		return journalRow{}, false
	}
	if isJournalSeparator(row.cells) {
		return journalRow{}, false
	}
	return row, true
}

func isJournalHeader(cells [5]string) bool {
	for index, cell := range cells {
		if !strings.EqualFold(cell, journalHeader[index]) {
			return false
		}
	}
	return true
}

func isJournalSeparator(cells [5]string) bool {
	for _, cell := range cells {
		if !isSeparatorCell(cell) {
			return false
		}
	}
	return true
}

func isSeparatorCell(cell string) bool {
	cell = strings.TrimSpace(cell)
	cell = strings.TrimPrefix(cell, ":")
	cell = strings.TrimSuffix(cell, ":")
	if len(cell) < 2 {
		return false
	}
	for _, one := range cell {
		if one != '-' {
			return false
		}
	}
	return true
}

func appendClass(report *Report, class journalClass) {
	malformed := 0
	for _, row := range class.rows {
		if row.malformed {
			malformed++
		}
	}
	path := classPath(class.name)
	if malformed > 0 {
		report.Problems = append(report.Problems, Problem{
			Class: class.name,
			Path:  path,
			Kind:  ProblemMalformed,
			Rows:  malformed,
		})
		return
	}

	dates := make([]time.Time, 0, len(class.rows))
	var invalid []string
	for _, row := range class.rows {
		parsed, err := time.Parse(time.DateOnly, row.cells[0])
		if err != nil {
			invalid = append(invalid, row.cells[0])
			continue
		}
		dates = append(dates, parsed)
	}
	if len(invalid) > 0 {
		report.Problems = append(report.Problems, Problem{
			Class:        class.name,
			Path:         path,
			Kind:         ProblemUnparseableDate,
			InvalidDates: invalid[:min(3, len(invalid))],
		})
		return
	}
	if len(class.rows) < 2 {
		return
	}

	first := dates[0]
	last := dates[0]
	for _, parsed := range dates[1:] {
		if parsed.Before(first) {
			first = parsed
		}
		if parsed.After(last) {
			last = parsed
		}
	}
	report.Classes = append(report.Classes, Class{
		Name:     class.name,
		Rows:     len(class.rows),
		SpanDays: int(last.Sub(first).Hours() / 24),
		First:    first.Format(time.DateOnly),
		Last:     last.Format(time.DateOnly),
	})
}

func classPath(name string) string {
	var tb textbuf.Buffer
	return tb.Str(journalPrefix).Str(name).Str(markdownExt).String()
}

// specStems parses the canonical Spec-cell vocabulary. The boolean distinguishes
// an unreadable key from a row that explicitly names no spec.
func specStems(cell string) ([]string, bool) {
	text := strings.TrimSpace(cell)
	if strings.HasSuffix(text, ")") {
		if open := strings.LastIndexByte(text, '('); open >= 0 {
			note := text[open+1 : len(text)-1]
			if !strings.ContainsAny(note, "()") {
				text = strings.TrimSpace(text[:open])
			}
		}
	}
	stems := make([]string, 0)
	seen := make(map[string]bool)
	for token := range strings.SplitSeq(text, ",") {
		token = strings.TrimSpace(token)
		switch strings.ToLower(token) {
		case "", "-", "none", "n/a":
			continue
		}
		if !validSpecStem(token) {
			return nil, false
		}
		if !seen[token] {
			seen[token] = true
			stems = append(stems, token)
		}
	}
	return stems, true
}
func validSpecStem(stem string) bool {
	for index, character := range []byte(stem) {
		if character >= 'A' && character <= 'Z' ||
			character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			index != 0 && (character == '.' || character == '_' || character == '-') {
			continue
		}
		return false
	}
	return stem != ""
}

// AddedSpecEvidence returns spec stems named by rows added or rewritten in the
// selected worktree shards. Byte-level table reformatting does not count: rows
// are paired by their five canonical cells against HEAD before new stems are read.
func AddedSpecEvidence(tree string, paths []string) ([]string, []string, error) {
	headPaths, err := classFilesAtHead(tree)
	if err != nil {
		return nil, nil, err
	}
	atHead := make(map[string]bool, len(headPaths))
	for _, path := range headPaths {
		atHead[path] = true
	}
	stems := make([]string, 0)
	seen := make(map[string]bool)
	malformed := make([]string, 0)
	for _, path := range paths {
		current, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(path))) //nolint:gosec // the path is a journal shard under the checkout root
		if err != nil {
			return nil, nil, fmt.Errorf("read journal shard %s: %w", path, err)
		}
		currentRows, valid := evidenceRows(string(current))
		if !valid {
			malformed = append(malformed, path)
			continue
		}
		headRows := []journalRow(nil)
		if atHead[path] {
			answer := runGit(tree, "show", headObject(path))
			if answer.startErr != nil || answer.code != 0 {
				return nil, nil, gitReadError("read "+headObject(path), answer)
			}
			headRows, valid = evidenceRows(string(answer.stdout))
			if !valid {
				malformed = append(malformed, path)
				continue
			}
		}
		held := make(map[[5]string]int, len(headRows))
		for _, row := range headRows {
			held[row.cells]++
		}
		for _, row := range currentRows {
			if held[row.cells] != 0 {
				held[row.cells]--
				continue
			}
			rowStems, _ := specStems(row.cells[1])
			for _, stem := range rowStems {
				if !seen[stem] {
					seen[stem] = true
					stems = append(stems, stem)
				}
			}
		}
	}
	sort.Strings(malformed)
	return stems, malformed, nil
}

func evidenceRows(contents string) ([]journalRow, bool) {
	rows := journalRows(contents)
	for _, row := range rows {
		if row.malformed {
			return nil, false
		}
		if _, valid := specStems(row.cells[1]); !valid {
			return nil, false
		}
	}
	return rows, true
}

// HeadSpecEvidence returns the committed journal rows that name a spec stem.
// The map value is the journal shard carrying the evidence. A malformed shard
// contributes no evidence and is named separately so callers fail closed.
func HeadSpecEvidence(tree string) (map[string]string, []string, error) {
	paths, err := classFilesAtHead(tree)
	if err != nil {
		return nil, nil, err
	}
	classes, err := readClasses(tree, paths)
	if err != nil {
		return nil, nil, err
	}
	evidence := make(map[string]string)
	malformed := make([]string, 0)
	for _, class := range classes {
		path := classPath(class.name)
		valid := true
		for _, row := range class.rows {
			if row.malformed {
				valid = false
				break
			}
		}
		if !valid {
			malformed = append(malformed, path)
			continue
		}
		classEvidence := make([]string, 0)
		for _, row := range class.rows {
			rowStems, readable := specStems(row.cells[1])
			if !readable {
				valid = false
				break
			}
			classEvidence = append(classEvidence, rowStems...)
		}
		if !valid {
			malformed = append(malformed, path)
			continue
		}
		for _, stem := range classEvidence {
			if _, exists := evidence[stem]; !exists {
				evidence[stem] = path
			}
		}
	}
	sort.Strings(malformed)
	return evidence, malformed, nil
}
