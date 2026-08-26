// Design: docs/architecture/core-design.md -- the drift report, byte for byte
// Overview: points.go -- the split whose round trip is reported as one of these
// Overview: render.go -- the render check that prints one of these
//
// difflib.go is Python's difflib.unified_diff over lines, ported.
//
// Two gates PRINT a diff that no other code reads. Each stale or non-round-trip
// rule appears as the first 24 lines of a unified diff. Authors use that page to
// fix the rule. Go has no standard unified diff. Another algorithm groups hunks
// differently and produces different bytes for the same files. The parity proof
// would then require a weaker verdict comparison.
//
// This implementation therefore uses difflib's algorithm, not Myers. It
// reproduces SequenceMatcher's longest-block search, the 200-element autojunk
// heuristic, and three-line grouped-opcode context. Each changes the output
// bytes.

package rules

import (
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// diffContext is difflib's default `n`: how many equal lines frame each hunk.
const diffContext = 3

// autojunkFloor is the length at or above which SequenceMatcher purges popular
// elements. Below it, every element of b takes part in the match search.
const autojunkFloor = 200

// The four edit-script tags difflib names. A hunk line's prefix is derived from
// the tag, so the spelling is data rather than a label.
const (
	tagEqual   = "equal"
	tagReplace = "replace"
	tagDelete  = "delete"
	tagInsert  = "insert"
)

// opcode is one edit-script instruction. It contains a tag and the half-open
// range for each sequence.
type opcode struct {
	tag            string
	i1, i2, j1, j2 int
}

// matcher implements difflib.SequenceMatcher for two line sequences, with
// isjunk set to None. Thus, find_longest_match cannot reach its junk-extension
// passes. The separate autojunk purge remains because it changes the answer.
type matcher struct {
	a, b []string
	b2j  map[string][]int
}

// newMatcher indexes b by element. For b with at least 200 elements, it then
// removes POPULAR elements that occur in more than one percent of the lines.
//
// This is difflib's autojunk. Its removal would silently change the hunks. A
// 400-line rule has many blank lines and table separators, which this heuristic
// removes.
func newMatcher(a, b []string) *matcher {
	m := &matcher{a: a, b: b, b2j: make(map[string][]int, len(b))}
	for i, line := range b {
		m.b2j[line] = append(m.b2j[line], i)
	}
	if len(b) >= autojunkFloor {
		ntest := len(b)/100 + 1
		for line, idx := range m.b2j {
			if len(idx) > ntest {
				delete(m.b2j, line)
			}
		}
	}
	return m
}

// findLongest answers the longest matching block in a[alo:ahi] against
// b[blo:bhi], as (i, j, size).
//
// For equally long blocks, it answers the earliest block in a. For those
// blocks, it answers the earliest one in b. This makes the diff deterministic.
func (m *matcher) findLongest(alo, ahi, blo, bhi int) (int, int, int) {
	besti, bestj, bestsize := alo, blo, 0
	j2len := map[int]int{}
	for i := alo; i < ahi; i++ {
		newj2len := map[int]int{}
		for _, j := range m.b2j[m.a[i]] {
			if j < blo {
				continue
			}
			if j >= bhi {
				break
			}
			k := j2len[j-1] + 1
			newj2len[j] = k
			if k > bestsize {
				besti, bestj, bestsize = i-k+1, j-k+1, k
			}
		}
		j2len = newj2len
	}
	// Extend the block over elements the index no longer carries. A purged
	// popular line matches here even though it started no block of its own.
	for besti > alo && bestj > blo && m.a[besti-1] == m.b[bestj-1] {
		besti--
		bestj--
		bestsize++
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi && m.a[besti+bestsize] == m.b[bestj+bestsize] {
		bestsize++
	}
	return besti, bestj, bestsize
}

// matchingBlocks answers every maximal matching block, in order, adjacent ones
// collapsed, closed by the empty sentinel difflib appends.
func (m *matcher) matchingBlocks() [][3]int {
	type window struct{ alo, ahi, blo, bhi int }
	queue := []window{{0, len(m.a), 0, len(m.b)}}
	var found [][3]int
	for len(queue) > 0 {
		w := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		i, j, k := m.findLongest(w.alo, w.ahi, w.blo, w.bhi)
		if k == 0 {
			continue
		}
		found = append(found, [3]int{i, j, k})
		if w.alo < i && w.blo < j {
			queue = append(queue, window{w.alo, i, w.blo, j})
		}
		if i+k < w.ahi && j+k < w.bhi {
			queue = append(queue, window{i + k, w.ahi, j + k, w.bhi})
		}
	}
	sort.Slice(found, func(x, y int) bool {
		if found[x][0] != found[y][0] {
			return found[x][0] < found[y][0]
		}
		if found[x][1] != found[y][1] {
			return found[x][1] < found[y][1]
		}
		return found[x][2] < found[y][2]
	})

	var merged [][3]int
	i1, j1, k1 := 0, 0, 0
	for _, block := range found {
		if i1+k1 == block[0] && j1+k1 == block[1] {
			k1 += block[2]
			continue
		}
		if k1 > 0 {
			merged = append(merged, [3]int{i1, j1, k1})
		}
		i1, j1, k1 = block[0], block[1], block[2]
	}
	if k1 > 0 {
		merged = append(merged, [3]int{i1, j1, k1})
	}
	return append(merged, [3]int{len(m.a), len(m.b), 0})
}

// opcodes answers the whole edit script turning a into b.
func (m *matcher) opcodes() []opcode {
	var out []opcode
	i, j := 0, 0
	for _, block := range m.matchingBlocks() {
		ai, bj, size := block[0], block[1], block[2]
		tag := ""
		switch {
		case i < ai && j < bj:
			tag = tagReplace
		case i < ai:
			tag = tagDelete
		case j < bj:
			tag = tagInsert
		}
		if tag != "" {
			out = append(out, opcode{tag, i, ai, j, bj})
		}
		i, j = ai+size, bj+size
		if size > 0 {
			out = append(out, opcode{tagEqual, ai, i, bj, j})
		}
	}
	return out
}

// groupedOpcodes answers the edit script cut into hunks, each framed by at most
// diffContext equal lines. A run of equal lines longer than twice the context
// ends one hunk and starts the next.
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
			// The next hunk starts with the LAST diffContext lines of this run,
			// not its last window. The same context cuts both ends of the run.
			// A window here would give the second hunk twice the leading context.
			i1 = max(i1, code.i2-diffContext)
			j1 = max(j1, code.j2-diffContext)
		}
		group = append(group, opcode{code.tag, i1, code.i2, j1, code.j2})
	}
	// A trailing group holding one equal opcode is context with nothing to
	// frame, so it is dropped rather than printed as an empty hunk.
	if len(group) > 0 && (len(group) != 1 || group[0].tag != tagEqual) {
		groups = append(groups, group)
	}
	return groups
}

// unifiedDiff answers difflib.unified_diff(a, b, fromfile, tofile, lineterm="")
// as a slice of lines, with no terminator on any of them.
//
// The headers are emitted only when there is at least one hunk, which is what
// makes an identical pair answer nothing at all.
func unifiedDiff(a, b []string, fromFile, toFile string) []string {
	var out []string
	var tb textbuf.Buffer
	m := newMatcher(a, b)
	for _, group := range m.groupedOpcodes() {
		if len(out) == 0 {
			out = append(out, tb.Str("--- ").Str(fromFile).String())
			tb.Reset()
			out = append(out, tb.Str("+++ ").Str(toFile).String())
			tb.Reset()
		}
		first, last := group[0], group[len(group)-1]
		out = append(out, tb.Str("@@ -").Str(unifiedRange(first.i1, last.i2)).
			Byte(' ').Byte('+').Str(unifiedRange(first.j1, last.j2)).Str(" @@").String())
		tb.Reset()

		for _, code := range group {
			if code.tag == tagEqual {
				for _, line := range a[code.i1:code.i2] {
					out = append(out, tb.Byte(' ').Str(line).String())
					tb.Reset()
				}
				continue
			}
			if code.tag == tagReplace || code.tag == tagDelete {
				for _, line := range a[code.i1:code.i2] {
					out = append(out, tb.Byte('-').Str(line).String())
					tb.Reset()
				}
			}
			if code.tag == tagReplace || code.tag == tagInsert {
				for _, line := range b[code.j1:code.j2] {
					out = append(out, tb.Byte('+').Str(line).String())
					tb.Reset()
				}
			}
		}
	}
	return out
}

// unifiedRange renders one hunk-header side. A one-line range prints only its
// start. An EMPTY range prints the preceding line to locate its insertion or
// deletion.
func unifiedRange(start, stop int) string {
	beginning := start + 1
	length := stop - start
	var tb textbuf.Buffer
	if length == 1 {
		return tb.Int(int64(beginning)).String()
	}
	if length == 0 {
		beginning--
	}
	return tb.Int(int64(beginning)).Byte(',').Int(int64(length)).String()
}

// diffHead answers the first diffBudget lines of a unified diff, joined, which
// is the budget both gates print. A page longer than that is a rule somebody
// rewrote rather than a drift somebody can read.
func diffHead(lines []string) string {
	if len(lines) > diffBudget {
		lines = lines[:diffBudget]
	}
	var tb textbuf.Buffer
	return tb.Join(lines, "\n").String()
}
