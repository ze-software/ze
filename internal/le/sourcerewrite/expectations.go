package sourcerewrite

import (
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	messageTypeUpdate  = 0x02
	attributeMPUnreach = 15
	flagExtendedLength = 0x10
	markerLength       = 16
	bgpHeaderLength    = 19
)

var rawExpectationLine = regexp.MustCompile(`^(\d+:raw:)([0-9A-Fa-f:]+?)(\s*)$`)

type unparseableError struct{ message string }

func (e unparseableError) Error() string { return e.message }

type rawAttribute struct {
	code byte
	raw  []byte
}

// ExpectationFileResult reports the outcome for one expectation file.
type ExpectationFileResult struct {
	File       string `json:"file"`
	Changed    int    `json:"changed"`
	LeftAlone  int    `json:"left_alone"`
	WouldWrite bool   `json:"would_write"`
}

// expectationRewriteReport aggregates deterministic expectation rewrites.
type expectationRewriteReport struct {
	Write     bool                    `json:"write"`
	Changed   int                     `json:"changed"`
	LeftAlone int                     `json:"left_alone"`
	Files     []ExpectationFileResult `json:"files"`
	Warnings  []string                `json:"warnings"`
}

func (r expectationRewriteReport) Text() string {
	var b strings.Builder
	verb := "would reorder"
	if r.Write {
		verb = "reordered"
	}
	for _, file := range r.Files {
		if file.Changed > 0 {
			fmt.Fprintf(&b, "%s: %s %d expectation(s)\n", file.File, verb, file.Changed)
		}
	}
	fmt.Fprintf(&b, "total: %d expectation(s), %d left alone\n", r.Changed, r.LeftAlone)
	return b.String()
}

func splitAttributes(block []byte) ([]rawAttribute, error) {
	out := make([]rawAttribute, 0)
	for off := 0; off < len(block); {
		if off+3 > len(block) {
			return nil, unparseableError{fmt.Sprintf("attribute header truncated at offset %d", off)}
		}
		flags, code := block[off], block[off+1]
		header, length := 3, int(block[off+2])
		if flags&flagExtendedLength != 0 {
			if off+4 > len(block) {
				return nil, unparseableError{fmt.Sprintf("extended length truncated at offset %d", off)}
			}
			header = 4
			length = int(block[off+2])<<8 | int(block[off+3])
		}
		end := off + header + length
		if end > len(block) {
			return nil, unparseableError{fmt.Sprintf("attribute %d value truncated at offset %d", code, off)}
		}
		raw := append([]byte(nil), block[off:end]...)
		out = append(out, rawAttribute{code: code, raw: raw})
		off = end
	}
	return out, nil
}

func canonicalAttributes(attrs []rawAttribute) []rawAttribute {
	ordered := append([]rawAttribute(nil), attrs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].code == attributeMPUnreach {
			return ordered[j].code != attributeMPUnreach
		}
		if ordered[j].code == attributeMPUnreach {
			return false
		}
		return ordered[i].code < ordered[j].code
	})
	return ordered
}

func reorderUpdatePayload(payload []byte) ([]byte, error) {
	if len(payload) < 2 {
		return nil, unparseableError{"payload shorter than the withdrawn-routes length field"}
	}
	withdrawnLength := int(payload[0])<<8 | int(payload[1])
	attributesLengthOffset := 2 + withdrawnLength
	if attributesLengthOffset+2 > len(payload) {
		return nil, unparseableError{"withdrawn-routes length runs past the payload"}
	}
	attributesLength := int(payload[attributesLengthOffset])<<8 | int(payload[attributesLengthOffset+1])
	attributesOffset := attributesLengthOffset + 2
	attributesEnd := attributesOffset + attributesLength
	if attributesEnd > len(payload) {
		return nil, unparseableError{"path-attribute length runs past the payload"}
	}
	attrs, err := splitAttributes(payload[attributesOffset:attributesEnd])
	if err != nil {
		return nil, err
	}
	ordered := canonicalAttributes(attrs)
	beforeCodes, afterCodes := make([]int, len(attrs)), make([]int, len(ordered))
	beforeBytes, afterBytes := make([]string, len(attrs)), make([]string, len(ordered))
	for i := range attrs {
		beforeCodes[i] = int(attrs[i].code)
		beforeBytes[i] = string(attrs[i].raw)
	}
	for i := range ordered {
		afterCodes[i] = int(ordered[i].code)
		afterBytes[i] = string(ordered[i].raw)
	}
	sort.Ints(beforeCodes)
	sort.Ints(afterCodes)
	sort.Strings(beforeBytes)
	sort.Strings(afterBytes)
	if fmt.Sprint(beforeCodes) != fmt.Sprint(afterCodes) {
		return nil, unparseableError{"attribute set changed"}
	}
	if fmt.Sprint(beforeBytes) != fmt.Sprint(afterBytes) {
		return nil, unparseableError{"attribute bytes changed"}
	}
	rebuilt := make([]byte, 0, attributesLength)
	for _, attr := range ordered {
		rebuilt = append(rebuilt, attr.raw...)
	}
	if len(rebuilt) != attributesLength {
		return nil, unparseableError{"attribute block length changed"}
	}
	out := make([]byte, 0, len(payload))
	out = append(out, payload[:attributesOffset]...)
	out = append(out, rebuilt...)
	out = append(out, payload[attributesEnd:]...)
	return out, nil
}

// reorderExpectationLine permutes one live :raw: BGP UPDATE expectation.
func reorderExpectationLine(line string) (string, bool, error) {
	match := rawExpectationLine.FindStringSubmatch(line)
	if match == nil {
		if strings.Contains(line, ":raw:") && !strings.HasPrefix(strings.TrimLeft(line, " \t\r\n\v\f"), "#") {
			return line, false, unparseableError{"raw expectation is not hex"}
		}
		return line, false, nil
	}
	body := match[2]
	colonPositions := make([]int, 0, strings.Count(body, ":"))
	for i := range len(body) {
		if body[i] == ':' {
			colonPositions = append(colonPositions, i)
		}
	}
	hexText := strings.ReplaceAll(body, ":", "")
	if len(hexText)%2 != 0 {
		return line, false, unparseableError{fmt.Sprintf("odd hex length %d", len(hexText))}
	}
	message, err := hex.DecodeString(hexText)
	if err != nil {
		return line, false, err
	}
	if len(message) < bgpHeaderLength {
		return line, false, unparseableError{fmt.Sprintf("message shorter than a BGP header (%d bytes)", len(message))}
	}
	for i := 0; i < markerLength; i++ {
		if message[i] != 0xff {
			return line, false, unparseableError{"no all-ones BGP marker"}
		}
	}
	declared := int(message[markerLength])<<8 | int(message[markerLength+1])
	if declared != len(message) {
		return line, false, unparseableError{fmt.Sprintf("header length %d != %d bytes present", declared, len(message))}
	}
	if message[markerLength+2] != messageTypeUpdate {
		return line, false, nil
	}
	payload := message[bgpHeaderLength:]
	newPayload, err := reorderUpdatePayload(payload)
	if err != nil {
		return line, false, err
	}
	if string(newPayload) == string(payload) {
		return line, false, nil
	}
	if len(newPayload) != len(payload) {
		return line, false, unparseableError{"payload length changed"}
	}
	encoded := strings.ToUpper(hex.EncodeToString(append(append([]byte(nil), message[:bgpHeaderLength]...), newPayload...)))
	chars := []byte(encoded)
	for _, position := range colonPositions {
		chars = append(chars, 0)
		copy(chars[position+1:], chars[position:])
		chars[position] = ':'
	}
	return match[1] + string(chars) + match[3], true, nil
}

// reorderExpectationFiles checks or rewrites each named expectation file.
func reorderExpectationFiles(paths []string, write bool) (expectationRewriteReport, error) {
	report := expectationRewriteReport{Write: write, Files: []ExpectationFileResult{}, Warnings: []string{}}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return report, err
		}
		if !utf8.Valid(raw) {
			return report, fmt.Errorf("%s is not valid UTF-8", path)
		}
		text := strings.ReplaceAll(string(raw), "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
		lines := splitLinesKeepEnds(text)
		out := make([]string, 0, len(lines))
		result := ExpectationFileResult{File: path, WouldWrite: !write}
		for i, line := range lines {
			changedLine, changed, err := reorderExpectationLine(line)
			if err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s:%d: left alone: %v", path, i+1, err))
				result.LeftAlone++
				out = append(out, line)
				continue
			}
			out = append(out, changedLine)
			if changed {
				result.Changed++
			}
		}
		result.WouldWrite = result.Changed > 0 && !write
		if result.Changed > 0 && write {
			info, err := os.Stat(path)
			if err != nil {
				return report, err
			}
			if err := os.WriteFile(path, []byte(strings.Join(out, "")), info.Mode().Perm()); err != nil {
				return report, err
			}
		}
		report.Changed += result.Changed
		report.LeftAlone += result.LeftAlone
		report.Files = append(report.Files, result)
	}
	return report, nil
}

func splitLinesKeepEnds(text string) []string {
	if text == "" {
		return []string{}
	}
	parts := strings.SplitAfter(text, "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
