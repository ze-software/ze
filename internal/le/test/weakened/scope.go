// Design: docs/contributing/rfc-implementation-guide.md -- one definition of a Go test unit
// Related: detector.go -- the lexical verdicts applied to these units.
package testweakened

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var goFunctionDeclarationPattern = regexp.MustCompile(
	`(?m)^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)`,
)

type namedUnit struct {
	name string
	text string
}

type textSpan struct {
	start int
	end   int
}

func weakenedUnits(path, oldText, newText string) []namedVerdict {
	oldText = executableTestText(path, oldText)
	newText = executableTestText(path, newText)
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	fileVerdict := detect(oldText, newText, path)
	fileDetails := appendVerdict(nil, fileVerdict)
	if !strings.HasSuffix(path, ".go") {
		if len(fileDetails) == 0 {
			return nil
		}
		return []namedVerdict{{name: stem, details: fileDetails}}
	}

	oldUnits := unitsByName(oldText)
	newUnits := unitsByName(newText)
	found := make([]namedVerdict, 0, len(oldUnits)+1)
	namedKinds := make(map[string]bool)
	for _, oldUnit := range oldUnits {
		if strings.TrimSpace(oldUnit.text) == "" {
			continue
		}
		verdict := detect(oldUnit.text, unitText(newUnits, oldUnit.name), path)
		details := appendVerdict(nil, verdict)
		if len(details) == 0 {
			continue
		}
		name := oldUnit.name
		if name == "" {
			name = stem
		}
		found = append(found, namedVerdict{name: name, details: details})
		for _, detail := range details {
			namedKinds[verdictKind(detail)] = true
		}
	}
	residual := make([]string, 0, len(fileDetails))
	for _, detail := range fileDetails {
		if !namedKinds[verdictKind(detail)] {
			residual = append(residual, detail)
		}
	}
	if len(residual) != 0 {
		found = append(found, namedVerdict{name: stem, details: residual})
	}
	return found
}

type namedVerdict struct {
	name    string
	details []string
}

func appendVerdict(target []string, verdict detectorVerdict) []string {
	target = append(target, verdict.blocking...)
	return append(target, verdict.advisory...)
}

func verdictKind(detail string) string {
	at := len(detail)
	if index := strings.IndexByte(detail, '('); index >= 0 && index < at {
		at = index
	}
	if index := strings.IndexByte(detail, ';'); index >= 0 && index < at {
		at = index
	}
	return strings.TrimSpace(detail[:at])
}

func unitsByName(content string) []namedUnit {
	units := goFunctionUnits(content)
	grouped := make([]namedUnit, 0, len(units))
	indexes := make(map[string]int, len(units))
	var text textbuf.Buffer
	for _, unit := range units {
		index, found := indexes[unit.name]
		if !found {
			indexes[unit.name] = len(grouped)
			grouped = append(grouped, unit)
			continue
		}
		grouped[index].text = text.Reset().Str(grouped[index].text).Byte('\n').Str(unit.text).String()
	}
	return grouped
}

func unitText(units []namedUnit, name string) string {
	for _, unit := range units {
		if unit.name == name {
			return unit.text
		}
	}
	return ""
}

func goFunctionUnits(content string) []namedUnit {
	spans := goFunctionSpans(content)
	units := make([]namedUnit, 0, len(spans))
	for _, span := range spans {
		text := content[span.start:span.end]
		match := goFunctionDeclarationPattern.FindStringSubmatch(text)
		name := ""
		if len(match) == 2 {
			name = match[1]
		}
		units = append(units, namedUnit{name: name, text: text})
	}
	return units
}

func goFunctionSpans(content string) []textSpan {
	starts := lineStartsWith(content, "func")
	ends := lineStartsWith(content, "}")
	spans := make([]textSpan, 0, len(starts))
	for index, start := range starts {
		begin := docCommentStart(content, start)
		capAt := len(content)
		if index+1 < len(starts) {
			capAt = docCommentStart(content, starts[index+1])
		}
		brace := -1
		for _, end := range ends {
			if end > start {
				brace = end
				break
			}
		}
		end := capAt
		if brace >= 0 && brace+2 < end {
			end = brace + 2
		}
		if end > len(content) {
			end = len(content)
		}
		minimum := start + 1
		if end < minimum {
			end = minimum
		}
		spans = append(spans, textSpan{start: begin, end: end})
	}
	return spans
}

func lineStartsWith(content, token string) []int {
	starts := make([]int, 0)
	for offset := 0; offset < len(content); {
		end := strings.IndexByte(content[offset:], '\n')
		if end < 0 {
			end = len(content) - offset
		}
		line := content[offset : offset+end]
		if strings.HasPrefix(line, token) &&
			(len(line) == len(token) || !isIdentifierByte(line[len(token)])) {
			starts = append(starts, offset)
		}
		offset += end
		if offset < len(content) {
			offset++
		}
	}
	return starts
}

func isIdentifierByte(character byte) bool {
	return character == '_' || character >= '0' && character <= '9' ||
		character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
}

func docCommentStart(content string, at int) int {
	lineStart := at
	for lineStart > 0 {
		previous := strings.LastIndex(content[:lineStart-1], "\n") + 1
		if !strings.HasPrefix(strings.TrimLeft(content[previous:lineStart], " \t"), "//") {
			break
		}
		lineStart = previous
	}
	return lineStart
}
