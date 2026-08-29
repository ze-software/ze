// Design: docs/architecture/core-design.md -- live hook and checker names in instructional docs
// Overview: links.go -- the complete links gate.

package doccheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var nameLintFiles = [...]string{
	"plan/learned/HOOK-FRICTION.md",
	"plan/learned/RECURRING-PATTERNS.md",
	"ai/rules/repo-maintenance.md",
}

var nameLintSources = [...]string{
	"internal/le/hookruntime/agent.go",
	"internal/le/hookruntime/bash.go",
	"internal/le/hookruntime/lifecycle.go",
	"internal/le/hookruntime/postwrite.go",
	"internal/le/hookruntime/session.go",
	"internal/le/hookruntime/writeedit.go",
	"internal/le/doc/wiring/checks.go",
}

var (
	shellTokenRe     = regexp.MustCompile(`([A-Za-z0-9][\w.-]*\.sh)\b`)
	checkTokenRe     = regexp.MustCompile(`\b((?:c_|check_)[a-z0-9_]+)\b`)
	headingRe        = regexp.MustCompile(`^(#+)\s`)
	retiredHeadingRe = regexp.MustCompile(`(?i)^#+\s*Retired\b`)
	retiredCellRe    = regexp.MustCompile(`(?i)^(?:retired\b|fixed\b.*\bat the source\b)`)
	statusHeadingRe  = regexp.MustCompile(`(?i)^status\b`)
	tableSeparatorRe = regexp.MustCompile(`^:?-{2,}:?$`)
)

func checkHookNames(root string, _ bool) ([]string, error) {
	present := make([]string, 0, len(nameLintFiles))
	for _, rel := range nameLintFiles {
		if pathExists(root, rel) {
			present = append(present, rel)
		}
	}
	if len(present) == 0 {
		return nil, nil
	}
	scripts, definitions, findings, err := knownNames(root)
	if err != nil {
		return nil, err
	}
	var tb textbuf.Buffer
	for _, rel := range present {
		body, readErr := readRepositoryFile(root, rel)
		if readErr != nil {
			findings = append(findings, tb.Reset().Str(rel).
				Str(": cannot read for the dead-name lint: ").Err(readErr).String())
			continue
		}
		excused := retiredLines(string(body))
		for index, line := range strings.Split(string(body), "\n") {
			lineNo := index + 1
			if suppressed(line) {
				continue
			}
			for _, match := range backtickRe.FindAllStringSubmatch(line, -1) {
				span := match[1]
				var dead []string
				for _, found := range shellTokenRe.FindAllStringSubmatch(span, -1) {
					if !scripts[found[1]] {
						dead = append(dead, found[1])
					}
				}
				for _, found := range checkTokenRe.FindAllStringSubmatch(span, -1) {
					if !definitions[found[1]] {
						dead = append(dead, found[1])
					}
				}
				for _, token := range dead {
					if excused[lineNo] {
						continue
					}
					findings = append(findings, tb.Reset().
						Str(rel).Byte(':').Int(int64(lineNo)).
						Str(": dead name reference: ").Str(token).
						Str(" -- names no tracked native check source or hook action; ").
						Str("correct it to the live name, or move the entry under '## Retired'").String())
				}
			}
		}
	}
	return findings, nil
}

func knownNames(root string) (map[string]bool, map[string]bool, []string, error) {
	files, err := trackedNames(root)
	if err != nil {
		return nil, nil, nil, err
	}
	scripts := make(map[string]bool)
	for _, rel := range files {
		if strings.HasSuffix(rel, ".sh") {
			scripts[filepath.Base(rel)] = true
		}
	}
	definitions := make(map[string]bool)
	var findings []string
	var text textbuf.Buffer
	for _, rel := range nameLintSources {
		if !containsString(files, rel) || !regularFile(root, rel) {
			findings = append(findings, text.Reset().Str(rel).
				Str(": dead-name lint source is missing from the tree").String())
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(root, rel), nil, 0)
		if parseErr != nil {
			findings = append(findings, text.Reset().Str(rel).
				Str(": cannot parse for check names: ").Err(parseErr).String())
			continue
		}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				definitions[function.Name.Name] = true
			}
		}
	}
	return scripts, definitions, findings, nil
}

func regularFile(root, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func containsString(values []string, wanted string) bool {
	index := sort.SearchStrings(values, wanted)
	return index < len(values) && values[index] == wanted
}

func retiredLines(text string) map[int]bool {
	lines := strings.Split(text, "\n")
	excused := make(map[int]bool)
	retiredDepth := 0
	statusColumn := -1
	for index, line := range lines {
		lineNo := index + 1
		if heading := headingRe.FindStringSubmatch(line); heading != nil {
			depth := len(heading[1])
			if retiredHeadingRe.MatchString(line) {
				retiredDepth = depth
			} else if retiredDepth > 0 && depth <= retiredDepth {
				retiredDepth = 0
			}
		}
		if retiredDepth > 0 {
			excused[lineNo] = true
			continue
		}
		if !strings.HasPrefix(strings.TrimLeft(line, " \t"), "|") {
			statusColumn = -1
			continue
		}
		cells := rowCells(line)
		if index+1 < len(lines) && strings.HasPrefix(strings.TrimLeft(lines[index+1], " \t"), "|") {
			separator := rowCells(lines[index+1])
			allSeparators := true
			for _, cell := range separator {
				if cell != "" && !tableSeparatorRe.MatchString(cell) {
					allSeparators = false
				}
			}
			if allSeparators {
				statusColumn = -1
				for cellIndex, cell := range cells {
					if statusHeadingRe.MatchString(cell) {
						statusColumn = cellIndex
						break
					}
				}
			}
		}
		entitled := []int{1}
		if statusColumn >= 0 {
			entitled = append(entitled, statusColumn)
		}
		for _, cellIndex := range entitled {
			if cellIndex < len(cells) && retiredCellRe.MatchString(cells[cellIndex]) {
				excused[lineNo] = true
				break
			}
		}
	}
	return excused
}

func rowCells(line string) []string {
	parts := strings.Split(strings.TrimLeft(line, " \t"), "|")
	for index := range parts {
		parts[index] = strings.Trim(strings.TrimSpace(parts[index]), "*_ ")
	}
	return parts
}
