// Design: docs/architecture/core-design.md -- le's native development gates
// Related: report.go -- structured findings and the legacy text rendering
//
// Package speccitation checks references from active specs to sibling specs.
// It also reports line-token drift in active specs and learned summaries.
package speccitation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	baselinePath = "plan/.citation-baseline"
	planPath     = "plan"
)

var (
	specReferencePattern = regexp.MustCompile(`plan/spec-[a-z0-9][a-z0-9-]*\.md`)
	backtickPattern      = regexp.MustCompile("`([^`]+)`")
	citationPattern      = regexp.MustCompile(`^([^\x60\s]+):(\d+)$`)
	lineSuffixPattern    = regexp.MustCompile(`:\d+$`)
)

type backtickToken struct {
	start int
	end   int
	value string
}

type sourceFile struct {
	lines  []string
	exists bool
}

// Scan reads the citation population below root and returns every finding in
// producer order. It returns an error when an existing input cannot be read.
func Scan(root string) (Report, error) {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return Report{}, fmt.Errorf("open repository root: %w", err)
	}
	report, scanErr := scanRepository(root, repository)
	closeErr := repository.Close()
	if scanErr != nil {
		return report, scanErr
	}
	if closeErr != nil {
		return report, fmt.Errorf("close repository root: %w", closeErr)
	}
	return report, nil
}

func scanRepository(root string, repository *os.Root) (Report, error) {
	report := Report{
		Baseline: []string{},
		Dangling: []DanglingFinding{},
		Warnings: []DriftFinding{},
	}

	specs, err := documentPaths(root, filepath.Join(planPath, "spec-*.md"), true)
	if err != nil {
		return report, err
	}
	learned, err := learnedPaths(root)
	if err != nil {
		return report, err
	}
	baseline, err := loadBaseline(repository)
	if err != nil {
		return report, err
	}

	report.Specs = len(specs)
	report.Baseline = baseline
	baselineSet := make(map[string]struct{}, len(baseline))
	for _, target := range baseline {
		baselineSet[target] = struct{}{}
	}

	for _, spec := range specs {
		findings, err := danglingInSpec(repository, spec, baselineSet)
		if err != nil {
			return report, err
		}
		report.Dangling = append(report.Dangling, findings...)
	}

	sources := make(map[string]sourceFile)
	documents := slices.Concat(specs, learned)
	for _, document := range documents {
		warnings, err := driftInDocument(repository, document, sources)
		if err != nil {
			return report, err
		}
		report.Warnings = append(report.Warnings, warnings...)
	}
	return report, nil
}

func documentPaths(root, pattern string, excludeTemplate bool) ([]string, error) {
	plan := filepath.Join(root, planPath)
	info, err := os.Stat(plan)
	if err != nil {
		return nil, fmt.Errorf("read citation population under %s: %w", planPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("read citation population under %s: not a directory", planPath)
	}

	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return nil, fmt.Errorf("match citation population %s: %w", pattern, err)
	}
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		if excludeTemplate {
			if filepath.Base(match) == "spec-template.md" {
				continue
			}
		}
		relative, err := filepath.Rel(root, match)
		if err != nil {
			return nil, fmt.Errorf("make %s relative to repository: %w", match, err)
		}
		paths = append(paths, filepath.ToSlash(relative))
	}
	sort.Strings(paths)
	return paths, nil
}

func learnedPaths(root string) ([]string, error) {
	learned := filepath.Join(root, planPath, "learned")
	info, err := os.Stat(learned)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read learned citation population: %w", err)
	}
	if !info.IsDir() {
		return nil, nil
	}
	return documentPaths(root, filepath.Join(planPath, "learned", "*.md"), false)
}

func loadBaseline(repository *os.Root) ([]string, error) {
	data, err := repository.ReadFile(filepath.FromSlash(baselinePath))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", baselinePath, err)
	}
	text, err := decodeText(data, baselinePath)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]struct{})
	for _, line := range splitLines(text) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		entries[line] = struct{}{}
	}
	baseline := make([]string, 0, len(entries))
	for entry := range entries {
		baseline = append(baseline, entry)
	}
	sort.Strings(baseline)
	return baseline, nil
}

func danglingInSpec(repository *os.Root, relative string, baseline map[string]struct{}) ([]DanglingFinding, error) {
	lines, err := readDocument(repository, relative)
	if err != nil {
		return nil, err
	}
	var findings []DanglingFinding
	for index, line := range lines {
		for _, target := range specReferencePattern.FindAllString(line, -1) {
			if target == relative {
				continue
			}
			exists, err := regularFile(repository, target)
			if err != nil {
				return nil, fmt.Errorf("inspect cited spec %s: %w", target, err)
			}
			if exists {
				continue
			}
			if _, ok := baseline[target]; ok {
				continue
			}
			findings = append(findings, DanglingFinding{
				Citer:  DocumentLocation{Path: relative, Line: index + 1},
				Target: target,
			})
		}
	}
	return findings, nil
}

func driftInDocument(repository *os.Root, relative string, sources map[string]sourceFile) ([]DriftFinding, error) {
	lines, err := readDocument(repository, relative)
	if err != nil {
		return nil, err
	}
	var findings []DriftFinding
	for index, line := range lines {
		tokens := backtickTokens(line)
		citations, plain := classifyTokens(tokens)
		for _, citation := range citations {
			sourceToken, ok := nearestToken(citation, plain)
			if !ok {
				continue
			}
			parts := citationPattern.FindStringSubmatch(citation.value)
			sourcePath := parts[1]
			sourceLine := canonicalLine(parts[2])
			file, err := readSource(repository, sourcePath, sources)
			if err != nil {
				return nil, err
			}
			if !file.exists {
				continue
			}
			lineText := sourceLineText(file.lines, sourceLine)
			if strings.Contains(lineText, sourceToken) {
				continue
			}
			findings = append(findings, DriftFinding{
				Citer:       DocumentLocation{Path: relative, Line: index + 1},
				Source:      SourceLocation{Path: sourcePath, Line: sourceLine},
				SourceToken: sourceToken,
			})
		}
	}
	return findings, nil
}

func readDocument(repository *os.Root, relative string) ([]string, error) {
	data, err := repository.ReadFile(filepath.FromSlash(relative))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", relative, err)
	}
	text, err := decodeText(data, relative)
	if err != nil {
		return nil, err
	}
	return splitLines(text), nil
}

func regularFile(repository *os.Root, path string) (bool, error) {
	info, err := repository.Stat(filepath.FromSlash(path))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.Mode().IsRegular(), nil
}

func backtickTokens(line string) []backtickToken {
	indices := backtickPattern.FindAllStringSubmatchIndex(line, -1)
	tokens := make([]backtickToken, 0, len(indices))
	for _, index := range indices {
		tokens = append(tokens, backtickToken{
			start: index[0],
			end:   index[1],
			value: line[index[2]:index[3]],
		})
	}
	return tokens
}

func classifyTokens(tokens []backtickToken) ([]backtickToken, []backtickToken) {
	var citations []backtickToken
	var plain []backtickToken
	for _, token := range tokens {
		if citationPattern.MatchString(token.value) {
			citations = append(citations, token)
			continue
		}
		if lineSuffixPattern.MatchString(token.value) {
			continue
		}
		if containsLetter(token.value) {
			plain = append(plain, token)
		}
	}
	return citations, plain
}

func containsLetter(value string) bool {
	for _, character := range value {
		if unicode.IsLetter(character) {
			return true
		}
	}
	return false
}

func nearestToken(citation backtickToken, plain []backtickToken) (string, bool) {
	before := -1
	after := -1
	for index, token := range plain {
		if token.end <= citation.start {
			before = index
			continue
		}
		if after == -1 {
			if token.start >= citation.end {
				after = index
			}
		}
	}
	if before >= 0 {
		return plain[before].value, true
	}
	if after >= 0 {
		return plain[after].value, true
	}
	return "", false
}

func readSource(repository *os.Root, sourcePath string, cache map[string]sourceFile) (sourceFile, error) {
	if cached, ok := cache[sourcePath]; ok {
		return cached, nil
	}
	exists, err := regularFile(repository, sourcePath)
	if err != nil {
		return sourceFile{}, fmt.Errorf("inspect citation source %s: %w", sourcePath, err)
	}
	if !exists {
		cache[sourcePath] = sourceFile{}
		return sourceFile{}, nil
	}
	data, err := repository.ReadFile(filepath.FromSlash(sourcePath))
	if err != nil {
		return sourceFile{}, fmt.Errorf("read citation source %s: %w", sourcePath, err)
	}
	text, err := decodeText(data, sourcePath)
	if err != nil {
		return sourceFile{}, err
	}
	file := sourceFile{lines: splitLines(text), exists: true}
	cache[sourcePath] = file
	return file, nil
}

func decodeText(data []byte, path string) (string, error) {
	if !utf8.Valid(data) {
		return "", fmt.Errorf("decode %s as UTF-8: invalid encoding", path)
	}
	return string(data), nil
}

func canonicalLine(line string) string {
	canonical := strings.TrimLeft(line, "0")
	if canonical == "" {
		return "0"
	}
	return canonical
}

func sourceLineText(lines []string, line string) string {
	lineNumber, err := strconv.ParseUint(line, 10, 64)
	if err != nil {
		return ""
	}
	if lineNumber == 0 {
		return ""
	}
	if lineNumber > uint64(len(lines)) {
		return ""
	}
	return lines[lineNumber-1]
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	var lines []string
	start := 0
	for offset := 0; offset < len(text); {
		character, size := utf8.DecodeRuneInString(text[offset:])
		end := offset + size
		if character == '\r' {
			if end < len(text) {
				if text[end] == '\n' {
					end++
				}
			}
		}
		if isLineBreak(character) {
			lines = append(lines, text[start:offset])
			start = end
		}
		offset = end
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

func isLineBreak(character rune) bool {
	switch character {
	case '\n', '\r', '\v', '\f', '\u001c', '\u001d', '\u001e', '\u0085', '\u2028', '\u2029':
		return true
	default:
		return false
	}
}
