// Design: docs/architecture/core-design.md -- live hook and checker names in instructional docs
// Overview: links.go -- the complete links gate.

package doccheck

import (
	"fmt"
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
	".claude/hooks/pretool-writeedit.py",
	".claude/hooks/pretool-bash.py",
	".claude/hooks/pretool-agent-skill.py",
	".claude/hooks/posttool-writeedit.py",
	"scripts/dev/verify_wiring_docs.py",
}

var (
	shellTokenRe     = regexp.MustCompile(`([A-Za-z0-9][\w.-]*\.sh)\b`)
	checkTokenRe     = regexp.MustCompile(`\b((?:c_|check_)[a-z0-9_]+)\b`)
	headingRe        = regexp.MustCompile(`^(#+)\s`)
	retiredHeadingRe = regexp.MustCompile(`(?i)^#+\s*Retired\b`)
	retiredCellRe    = regexp.MustCompile(`(?i)^(?:retired\b|fixed\b.*\bat the source\b)`)
	statusHeadingRe  = regexp.MustCompile(`(?i)^status\b`)
	tableSeparatorRe = regexp.MustCompile(`^:?-{2,}:?$`)
	moduleDefRe      = regexp.MustCompile(`(?m)^(?:async )?def[ \t]+([A-Za-z_]\w*)[ \t]*\(`)
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
						Str(" -- names no tracked script, no check source file, and no check ").
						Str("function under scripts/ or .claude/hooks/; correct it to the live ").
						Str("name, or move the entry under '## Retired'").String())
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
	var sources []string
	for _, rel := range files {
		if strings.HasSuffix(rel, ".sh") {
			scripts[filepath.Base(rel)] = true
		}
		if strings.HasSuffix(rel, ".py") {
			scripts[filepath.Base(rel)] = true
		}
		if !strings.HasSuffix(rel, ".py") {
			continue
		}
		if !regularFile(root, rel) {
			continue
		}
		if strings.HasPrefix(rel, "scripts/") {
			sources = append(sources, rel)
			continue
		}
		if strings.HasPrefix(rel, ".claude/hooks/") {
			sources = append(sources, rel)
		}
	}
	sort.Strings(sources)
	definitions := make(map[string]bool)
	for _, rel := range sources {
		definitions[strings.TrimSuffix(filepath.Base(rel), ".py")] = true
	}
	var tb textbuf.Buffer
	var findings []string
	for _, required := range nameLintSources {
		if !containsString(sources, required) {
			findings = append(findings, tb.Reset().Str(required).
				Str(": dead-name lint source is missing from the tree -- restore it or update ").
				Str("nameLintSources in internal/le/doccheck/names.go").String())
		}
	}
	for _, rel := range sources {
		body, readErr := readRepositoryFile(root, rel)
		if readErr != nil {
			findings = append(findings, tb.Reset().Str(rel).
				Str(": cannot parse for check names: ").Err(readErr).
				Str(" -- every name it defines would read as dead until this parses").String())
			continue
		}
		scrubbed, parseErr := scrubPython(string(body))
		if parseErr != nil {
			findings = append(findings, tb.Reset().Str(rel).
				Str(": cannot parse for check names: ").Err(parseErr).
				Str(" -- every name it defines would read as dead until this parses").String())
			continue
		}
		for _, found := range moduleDefRe.FindAllStringSubmatch(scrubbed, -1) {
			definitions[found[1]] = true
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

func scrubPython(text string) (string, error) {
	out := []byte(text)
	for index := 0; index < len(text); {
		switch text[index] {
		case '#':
			for index < len(text) && text[index] != '\n' {
				out[index] = ' '
				index++
			}
		case '\'', '"':
			quote := text[index : index+1]
			if strings.HasPrefix(text[index:], strings.Repeat(quote, 3)) {
				quote = strings.Repeat(quote, 3)
			}
			end := closingQuote(text, index, quote)
			if end < 0 {
				return "", fmt.Errorf("unterminated string literal at offset %d", index)
			}
			for cursor := index; cursor < end; cursor++ {
				if out[cursor] != '\n' {
					out[cursor] = ' '
				}
			}
			index = end
		default:
			index++
		}
	}
	scrubbed := string(out)
	if err := validatePythonStructure(scrubbed); err != nil {
		return "", err
	}
	return scrubbed, nil
}
func validatePythonStructure(text string) error {
	stack := make([]byte, 0, 16)
	for offset := range len(text) {
		switch text[offset] {
		case '(', '[', '{':
			stack = append(stack, text[offset])
		case ')', ']', '}':
			if len(stack) == 0 {
				return fmt.Errorf("unexpected %q at offset %d", text[offset], offset)
			}
			open := stack[len(stack)-1]
			if !matchingDelimiters(open, text[offset]) {
				return fmt.Errorf("delimiter %q at offset %d closes %q", text[offset], offset, open)
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) > 0 {
		return fmt.Errorf("unclosed delimiter %q", stack[len(stack)-1])
	}
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "def ") {
			if !moduleDefRe.MatchString(line) {
				return fmt.Errorf("malformed top-level function declaration: %s", strings.TrimSpace(line))
			}
		}
		if strings.HasPrefix(line, "async def ") {
			if !moduleDefRe.MatchString(line) {
				return fmt.Errorf("malformed top-level function declaration: %s", strings.TrimSpace(line))
			}
		}
	}
	return nil
}

func matchingDelimiters(open, close byte) bool {
	switch open {
	case '(':
		return close == ')'
	case '[':
		return close == ']'
	case '{':
		return close == '}'
	default:
		return false
	}
}

func closingQuote(text string, start int, quote string) int {
	for index := start + len(quote); index < len(text); {
		if text[index] == '\\' {
			index += 2
			continue
		}
		if strings.HasPrefix(text[index:], quote) {
			return index + len(quote)
		}
		if len(quote) == 1 && text[index] == '\n' {
			return -1
		}
		index++
	}
	return -1
}
