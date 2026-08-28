// difflib.go preserves Python difflib's unified-diff output for golden drift.
package netlab

import (
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	diffContext   = 3
	autojunkFloor = 200
	tagEqual      = "equal"
	tagReplace    = "replace"
	tagDelete     = "delete"
	tagInsert     = "insert"
)

type opcode struct {
	tag            string
	i1, i2, j1, j2 int
}

type matcher struct {
	a, b []string
	b2j  map[string][]int
}

func newMatcher(a, b []string) *matcher {
	m := &matcher{a: a, b: b, b2j: make(map[string][]int, len(b))}
	for i, line := range b {
		m.b2j[line] = append(m.b2j[line], i)
	}
	if len(b) >= autojunkFloor {
		popular := len(b)/100 + 1
		for line, indexes := range m.b2j {
			if len(indexes) > popular {
				delete(m.b2j, line)
			}
		}
	}
	return m
}

func (m *matcher) findLongest(alo, ahi, blo, bhi int) (int, int, int) {
	bestI, bestJ, bestSize := alo, blo, 0
	previous := map[int]int{}
	for i := alo; i < ahi; i++ {
		current := map[int]int{}
		for _, j := range m.b2j[m.a[i]] {
			if j < blo {
				continue
			}
			if j >= bhi {
				break
			}
			size := previous[j-1] + 1
			current[j] = size
			if size > bestSize {
				bestI, bestJ, bestSize = i-size+1, j-size+1, size
			}
		}
		previous = current
	}
	for bestI > alo && bestJ > blo && m.a[bestI-1] == m.b[bestJ-1] {
		bestI--
		bestJ--
		bestSize++
	}
	for bestI+bestSize < ahi && bestJ+bestSize < bhi && m.a[bestI+bestSize] == m.b[bestJ+bestSize] {
		bestSize++
	}
	return bestI, bestJ, bestSize
}

func (m *matcher) matchingBlocks() [][3]int {
	type window struct{ alo, ahi, blo, bhi int }
	queue := []window{{0, len(m.a), 0, len(m.b)}}
	var found [][3]int
	for len(queue) > 0 {
		current := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		i, j, size := m.findLongest(current.alo, current.ahi, current.blo, current.bhi)
		if size == 0 {
			continue
		}
		found = append(found, [3]int{i, j, size})
		if current.alo < i && current.blo < j {
			queue = append(queue, window{current.alo, i, current.blo, j})
		}
		if i+size < current.ahi && j+size < current.bhi {
			queue = append(queue, window{i + size, current.ahi, j + size, current.bhi})
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
	firstI, firstJ, size := 0, 0, 0
	for _, block := range found {
		if firstI+size == block[0] && firstJ+size == block[1] {
			size += block[2]
			continue
		}
		if size > 0 {
			merged = append(merged, [3]int{firstI, firstJ, size})
		}
		firstI, firstJ, size = block[0], block[1], block[2]
	}
	if size > 0 {
		merged = append(merged, [3]int{firstI, firstJ, size})
	}
	return append(merged, [3]int{len(m.a), len(m.b), 0})
}

func (m *matcher) opcodes() []opcode {
	var out []opcode
	i, j := 0, 0
	for _, block := range m.matchingBlocks() {
		aI, bJ, size := block[0], block[1], block[2]
		tag := ""
		switch {
		case i < aI && j < bJ:
			tag = tagReplace
		case i < aI:
			tag = tagDelete
		case j < bJ:
			tag = tagInsert
		}
		if tag != "" {
			out = append(out, opcode{tag, i, aI, j, bJ})
		}
		i, j = aI+size, bJ+size
		if size > 0 {
			out = append(out, opcode{tagEqual, aI, i, bJ, j})
		}
	}
	return out
}

func (m *matcher) groupedOpcodes() [][]opcode {
	codes := m.opcodes()
	if len(codes) == 0 {
		codes = []opcode{{tagEqual, 0, 1, 0, 1}}
	}
	if first := codes[0]; first.tag == tagEqual {
		codes[0] = opcode{first.tag, max(first.i1, first.i2-diffContext), first.i2,
			max(first.j1, first.j2-diffContext), first.j2}
	}
	if last := codes[len(codes)-1]; last.tag == tagEqual {
		codes[len(codes)-1] = opcode{last.tag, last.i1, min(last.i2, last.i1+diffContext),
			last.j1, min(last.j2, last.j1+diffContext)}
	}

	window := diffContext * 2
	var groups [][]opcode
	var group []opcode
	for _, code := range codes {
		i1, j1 := code.i1, code.j1
		if code.tag == tagEqual && code.i2-code.i1 > window {
			group = append(group, opcode{code.tag,
				i1, min(code.i2, i1+diffContext), j1, min(code.j2, j1+diffContext)})
			groups = append(groups, group)
			group = nil
			i1 = max(i1, code.i2-diffContext)
			j1 = max(j1, code.j2-diffContext)
		}
		group = append(group, opcode{code.tag, i1, code.i2, j1, code.j2})
	}
	if len(group) > 0 && (len(group) != 1 || group[0].tag != tagEqual) {
		groups = append(groups, group)
	}
	return groups
}

func unifiedDiff(a, b []string, fromFile, toFile string) []string {
	var out []string
	var text textbuf.Buffer
	for _, group := range newMatcher(a, b).groupedOpcodes() {
		if len(out) == 0 {
			out = append(out, text.Str("--- ").Str(fromFile).Str("\n").String())
			text.Reset()
			out = append(out, text.Str("+++ ").Str(toFile).Str("\n").String())
			text.Reset()
		}
		first, last := group[0], group[len(group)-1]
		out = append(out, text.Str("@@ -").Str(unifiedRange(first.i1, last.i2)).
			Str(" +").Str(unifiedRange(first.j1, last.j2)).Str(" @@\n").String())
		text.Reset()

		for _, code := range group {
			if code.tag == tagEqual {
				for _, line := range a[code.i1:code.i2] {
					out = append(out, text.Byte(' ').Str(line).String())
					text.Reset()
				}
				continue
			}
			if code.tag == tagReplace || code.tag == tagDelete {
				for _, line := range a[code.i1:code.i2] {
					out = append(out, text.Byte('-').Str(line).String())
					text.Reset()
				}
			}
			if code.tag == tagReplace || code.tag == tagInsert {
				for _, line := range b[code.j1:code.j2] {
					out = append(out, text.Byte('+').Str(line).String())
					text.Reset()
				}
			}
		}
	}
	return out
}

func unifiedRange(start, stop int) string {
	beginning := start + 1
	length := stop - start
	var text textbuf.Buffer
	if length == 1 {
		return text.Int(int64(beginning)).String()
	}
	if length == 0 {
		beginning--
	}
	return text.Int(int64(beginning)).Byte(',').Int(int64(length)).String()
}

func linesKeepingEnds(text string) []string {
	var out []string
	for text != "" {
		at := strings.IndexByte(text, '\n')
		if at < 0 {
			return append(out, text)
		}
		out = append(out, text[:at+1])
		text = text[at+1:]
	}
	return out
}

func unifiedDiffText(a, b []string, fromFile, toFile string) string {
	return strings.Join(unifiedDiff(a, b, fromFile, toFile), "")
}
