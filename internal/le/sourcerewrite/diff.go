package sourcerewrite

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	replaceDiffContext   = 3
	replaceAutojunkFloor = 200
	replaceEqual         = "equal"
	replaceChanged       = "replace"
	replaceDeleted       = "delete"
	replaceInserted      = "insert"
)

type replaceOpcode struct {
	tag            string
	i1, i2, j1, j2 int
}

type replaceMatcher struct {
	a, b []string
	b2j  map[string][]int
}

func newReplaceMatcher(a, b []string) *replaceMatcher {
	matcher := &replaceMatcher{a: a, b: b, b2j: make(map[string][]int, len(b))}
	for index, line := range b {
		matcher.b2j[line] = append(matcher.b2j[line], index)
	}
	if len(b) >= replaceAutojunkFloor {
		popularAbove := len(b)/100 + 1
		for line, indices := range matcher.b2j {
			if len(indices) > popularAbove {
				delete(matcher.b2j, line)
			}
		}
	}
	return matcher
}

func (m *replaceMatcher) findLongest(aLow, aHigh, bLow, bHigh int) (int, int, int) {
	bestA, bestB, bestLength := aLow, bLow, 0
	previous := map[int]int{}
	for aIndex := aLow; aIndex < aHigh; aIndex++ {
		current := map[int]int{}
		for _, bIndex := range m.b2j[m.a[aIndex]] {
			if bIndex < bLow {
				continue
			}
			if bIndex >= bHigh {
				break
			}
			length := previous[bIndex-1] + 1
			current[bIndex] = length
			if length > bestLength {
				bestA, bestB, bestLength = aIndex-length+1, bIndex-length+1, length
			}
		}
		previous = current
	}
	for bestA > aLow && bestB > bLow && m.a[bestA-1] == m.b[bestB-1] {
		bestA--
		bestB--
		bestLength++
	}
	for bestA+bestLength < aHigh && bestB+bestLength < bHigh && m.a[bestA+bestLength] == m.b[bestB+bestLength] {
		bestLength++
	}
	return bestA, bestB, bestLength
}

func (m *replaceMatcher) matchingBlocks() [][3]int {
	type window struct{ aLow, aHigh, bLow, bHigh int }
	queue := []window{{0, len(m.a), 0, len(m.b)}}
	var found [][3]int
	for len(queue) > 0 {
		current := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		aIndex, bIndex, length := m.findLongest(current.aLow, current.aHigh, current.bLow, current.bHigh)
		if length == 0 {
			continue
		}
		found = append(found, [3]int{aIndex, bIndex, length})
		if current.aLow < aIndex && current.bLow < bIndex {
			queue = append(queue, window{current.aLow, aIndex, current.bLow, bIndex})
		}
		if aIndex+length < current.aHigh && bIndex+length < current.bHigh {
			queue = append(queue, window{aIndex + length, current.aHigh, bIndex + length, current.bHigh})
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i][0] != found[j][0] {
			return found[i][0] < found[j][0]
		}
		if found[i][1] != found[j][1] {
			return found[i][1] < found[j][1]
		}
		return found[i][2] < found[j][2]
	})
	var merged [][3]int
	previousA, previousB, previousLength := 0, 0, 0
	for _, block := range found {
		if previousA+previousLength == block[0] && previousB+previousLength == block[1] {
			previousLength += block[2]
			continue
		}
		if previousLength > 0 {
			merged = append(merged, [3]int{previousA, previousB, previousLength})
		}
		previousA, previousB, previousLength = block[0], block[1], block[2]
	}
	if previousLength > 0 {
		merged = append(merged, [3]int{previousA, previousB, previousLength})
	}
	return append(merged, [3]int{len(m.a), len(m.b), 0})
}

func (m *replaceMatcher) opcodes() []replaceOpcode {
	var output []replaceOpcode
	aIndex, bIndex := 0, 0
	for _, block := range m.matchingBlocks() {
		aMatch, bMatch, length := block[0], block[1], block[2]
		tag := ""
		switch {
		case aIndex < aMatch && bIndex < bMatch:
			tag = replaceChanged
		case aIndex < aMatch:
			tag = replaceDeleted
		case bIndex < bMatch:
			tag = replaceInserted
		}
		if tag != "" {
			output = append(output, replaceOpcode{tag, aIndex, aMatch, bIndex, bMatch})
		}
		aIndex, bIndex = aMatch+length, bMatch+length
		if length > 0 {
			output = append(output, replaceOpcode{replaceEqual, aMatch, aIndex, bMatch, bIndex})
		}
	}
	return output
}

func (m *replaceMatcher) groupedOpcodes() [][]replaceOpcode {
	codes := m.opcodes()
	if len(codes) == 0 {
		codes = []replaceOpcode{{replaceEqual, 0, 1, 0, 1}}
	}
	if first := codes[0]; first.tag == replaceEqual {
		codes[0] = replaceOpcode{first.tag, max(first.i1, first.i2-replaceDiffContext), first.i2, max(first.j1, first.j2-replaceDiffContext), first.j2}
	}
	if last := codes[len(codes)-1]; last.tag == replaceEqual {
		codes[len(codes)-1] = replaceOpcode{last.tag, last.i1, min(last.i2, last.i1+replaceDiffContext), last.j1, min(last.j2, last.j1+replaceDiffContext)}
	}
	window := replaceDiffContext * 2
	var groups [][]replaceOpcode
	var group []replaceOpcode
	for _, code := range codes {
		i1, j1 := code.i1, code.j1
		if code.tag == replaceEqual && code.i2-code.i1 > window {
			group = append(group, replaceOpcode{code.tag, i1, min(code.i2, i1+replaceDiffContext), j1, min(code.j2, j1+replaceDiffContext)})
			groups = append(groups, group)
			group = nil
			i1 = max(i1, code.i2-replaceDiffContext)
			j1 = max(j1, code.j2-replaceDiffContext)
		}
		group = append(group, replaceOpcode{code.tag, i1, code.i2, j1, code.j2})
	}
	if len(group) > 0 && (len(group) != 1 || group[0].tag != replaceEqual) {
		groups = append(groups, group)
	}
	return groups
}

func unifiedTextDiff(before, after, fromFile, toFile string) string {
	a, b := splitDiffLines(before), splitDiffLines(after)
	matcher := newReplaceMatcher(a, b)
	var output textbuf.Buffer
	output.Reset()
	for _, group := range matcher.groupedOpcodes() {
		if output.Len() == 0 {
			output.Str("--- ").Str(fromFile).Str("\n+++ ").Str(toFile).Byte('\n')
		}
		first, last := group[0], group[len(group)-1]
		output.Str("@@ -").Str(replaceUnifiedRange(first.i1, last.i2)).
			Str(" +").Str(replaceUnifiedRange(first.j1, last.j2)).Str(" @@\n")
		for _, code := range group {
			if code.tag == replaceEqual {
				for _, line := range a[code.i1:code.i2] {
					output.Byte(' ').Str(line)
				}
				continue
			}
			if code.tag == replaceChanged || code.tag == replaceDeleted {
				for _, line := range a[code.i1:code.i2] {
					output.Byte('-').Str(line)
				}
			}
			if code.tag == replaceChanged || code.tag == replaceInserted {
				for _, line := range b[code.j1:code.j2] {
					output.Byte('+').Str(line)
				}
			}
		}
	}
	return output.String()
}

func splitDiffLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := make([]string, 0, strings.Count(text, "\n")+1)
	start, consumedThrough := 0, 0
	for index, character := range text {
		if index < consumedThrough {
			continue
		}
		end := index
		switch character {
		case '\r':
			end++
			if end < len(text) && text[end] == '\n' {
				end++
			}
		case '\n', '\v', '\f', '\u001c', '\u001d', '\u001e':
			end++
		case '\u0085':
			end += len("\u0085")
		case '\u2028', '\u2029':
			end += len("\u2028")
		default:
			continue
		}
		lines = append(lines, text[start:end])
		start, consumedThrough = end, end
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

func replaceUnifiedRange(start, stop int) string {
	beginning := start + 1
	length := stop - start
	if length == 1 {
		return strconv.Itoa(beginning)
	}
	if length == 0 {
		beginning--
	}
	return fmt.Sprintf("%d,%d", beginning, length)
}
