package sourcerewrite

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

// replaceReport contains the preview shared by preview and apply mode.
type replaceReport struct {
	File    string `json:"file"`
	Count   int    `json:"count"`
	Applied bool   `json:"applied"`
	Diff    string `json:"diff"`
}

func (r replaceReport) Text() string { return r.Diff }

type replaceInputError struct {
	err error
}

func (e replaceInputError) Error() string {
	return e.err.Error()
}

func (e replaceInputError) Unwrap() error {
	return e.err
}

func isReplaceInputError(err error) bool {
	_, ok := errors.AsType[replaceInputError](err)
	return ok
}

// replaceFile performs the replace.py operation. It always computes the preview;
// apply controls only whether the resulting bytes are written.
func replaceFile(path, old, replacement string, regularExpression, all, apply bool) (replaceReport, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // a rewrite tool reads the path the operator named
	if err != nil {
		if os.IsNotExist(err) {
			return replaceReport{}, replaceInputError{fmt.Errorf("file not found: %s", path)}
		}
		return replaceReport{}, err
	}
	if !utf8.Valid(raw) {
		return replaceReport{}, fmt.Errorf("%s is not valid UTF-8", path)
	}

	original := strings.ReplaceAll(string(raw), "\r\n", "\n")
	original = strings.ReplaceAll(original, "\r", "\n")
	var modified string
	var count int
	if regularExpression {
		pattern, compileErr := regexp.Compile(old)
		if compileErr != nil {
			return replaceReport{}, replaceInputError{fmt.Errorf("invalid regex: %w", compileErr)}
		}
		locations := pattern.FindAllStringSubmatchIndex(original, -1)
		if !all && len(locations) > 1 {
			locations = locations[:1]
		}
		count = len(locations)
		modified, err = expandRegexReplacement(pattern, original, replacement, locations)
		if err != nil {
			return replaceReport{}, err
		}
	} else {
		count = strings.Count(original, old)
		limit := 1
		if all {
			limit = -1
		} else if count > 1 {
			count = 1
		}
		modified = strings.Replace(original, old, replacement, limit)
	}

	report := replaceReport{File: path, Count: count, Applied: apply && modified != original}
	if modified == original {
		report.Count = 0
		return report, nil
	}
	report.Diff = unifiedTextDiff(original, modified, "a/"+path, "b/"+path)
	if apply {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return replaceReport{}, statErr
		}
		if writeErr := os.WriteFile(path, []byte(modified), info.Mode().Perm()); writeErr != nil {
			return replaceReport{}, writeErr
		}
	}
	return report, nil
}

func expandRegexReplacement(pattern *regexp.Regexp, source, replacement string, matches [][]int) (string, error) {
	if len(matches) == 0 {
		return source, nil
	}
	template, err := pythonReplacementTemplate(pattern, replacement)
	if err != nil {
		return "", err
	}
	var out []byte
	last := 0
	for _, match := range matches {
		out = append(out, source[last:match[0]]...)
		out = pattern.ExpandString(out, template, source, match)
		last = match[1]
	}
	out = append(out, source[last:]...)
	return string(out), nil
}

// pythonReplacementTemplate converts the backreferences accepted by re.sub to
// regexp.Expand's $-form. Ordinary escaped characters retain the Python spelling.
func pythonReplacementTemplate(pattern *regexp.Regexp, replacement string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(replacement); i++ {
		ch := replacement[i]
		if ch == '$' {
			out.WriteString("$$")
			continue
		}
		if ch != '\\' {
			out.WriteByte(ch)
			continue
		}
		if i+1 >= len(replacement) {
			return "", replaceInputError{fmt.Errorf("invalid replacement: bad escape at end of pattern")}
		}
		i++
		next := replacement[i]
		switch next {
		case '\\':
			out.WriteByte('\\')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'g':
			if i+1 >= len(replacement) || replacement[i+1] != '<' {
				return "", replaceInputError{fmt.Errorf("invalid replacement: missing < after \\g")}
			}
			end := strings.IndexByte(replacement[i+2:], '>')
			if end < 0 {
				return "", replaceInputError{fmt.Errorf("invalid replacement: missing >")}
			}
			name := replacement[i+2 : i+2+end]
			if !validReplacementGroup(pattern, name) {
				return "", replaceInputError{fmt.Errorf("invalid replacement: unknown group %q", name)}
			}
			out.WriteString("${")
			out.WriteString(name)
			out.WriteByte('}')
			i += end + 2
		default:
			if next >= '0' && next <= '9' {
				start := i
				if i+1 < len(replacement) && replacement[i+1] >= '0' && replacement[i+1] <= '9' {
					i++
				}
				name := replacement[start : i+1]
				if !validReplacementGroup(pattern, name) {
					return "", replaceInputError{fmt.Errorf("invalid replacement: unknown group %q", name)}
				}
				out.WriteByte('$')
				out.WriteString(name)
			} else {
				// Python currently retains unknown ASCII-letter escapes only as an
				// error. Punctuation escapes mean the punctuation itself.
				if (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') {
					return "", replaceInputError{fmt.Errorf("invalid replacement: bad escape \\%c", next)}
				}
				out.WriteByte(next)
			}
		}
	}
	return out.String(), nil
}

func validReplacementGroup(pattern *regexp.Regexp, name string) bool {
	if name == "0" {
		return true
	}
	for index, candidate := range pattern.SubexpNames() {
		if candidate == name || (candidate == "" && fmt.Sprint(index) == name) {
			return true
		}
	}
	return false
}
