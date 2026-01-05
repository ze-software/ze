package functional

import "strings"

// ColoredCharDiff returns a colored character-level diff between expected and actual.
// Algorithm inspired by github.com/sergi/go-diff (MIT license).
func ColoredCharDiff(expected, actual string) string {
	diffs := diffRunes([]rune(expected), []rune(actual))
	return formatDiffs(diffs)
}

// diffOp represents a diff operation type.
type diffOp int

const (
	diffEqual diffOp = iota
	diffInsert
	diffDelete
)

// diff represents a single diff chunk.
type diff struct {
	Op   diffOp
	Text string
}

// diffRunes computes character-level diff using Myers algorithm.
// Based on Myers 1986 "An O(ND) Difference Algorithm".
func diffRunes(a, b []rune) []diff {
	// Handle trivial cases
	if string(a) == string(b) {
		if len(a) == 0 {
			return nil
		}
		return []diff{{diffEqual, string(a)}}
	}
	if len(a) == 0 {
		return []diff{{diffInsert, string(b)}}
	}
	if len(b) == 0 {
		return []diff{{diffDelete, string(a)}}
	}

	// Trim common prefix
	prefixLen := commonPrefixLen(a, b)
	prefix := a[:prefixLen]
	a = a[prefixLen:]
	b = b[prefixLen:]

	// Trim common suffix
	suffixLen := commonSuffixLen(a, b)
	suffix := a[len(a)-suffixLen:]
	a = a[:len(a)-suffixLen]
	b = b[:len(b)-suffixLen]

	// Compute diff on middle part
	diffs := myersDiff(a, b)

	// Restore prefix/suffix
	if len(prefix) > 0 {
		diffs = append([]diff{{diffEqual, string(prefix)}}, diffs...)
	}
	if len(suffix) > 0 {
		diffs = append(diffs, diff{diffEqual, string(suffix)})
	}

	return mergeDiffs(diffs)
}

func commonPrefixLen(a, b []rune) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func commonSuffixLen(a, b []rune) int {
	la, lb := len(a), len(b)
	n := la
	if lb < n {
		n = lb
	}
	for i := 0; i < n; i++ {
		if a[la-1-i] != b[lb-1-i] {
			return i
		}
	}
	return n
}

// myersDiff implements the core Myers diff algorithm.
func myersDiff(a, b []rune) []diff {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 {
		return []diff{{diffInsert, string(b)}}
	}
	if m == 0 {
		return []diff{{diffDelete, string(a)}}
	}

	// Myers algorithm: find shortest edit script
	max := n + m
	v := make([]int, 2*max+1)
	var trace [][]int

	for d := 0; d <= max; d++ {
		// Save state for backtracking
		vc := make([]int, len(v))
		copy(vc, v)
		trace = append(trace, vc)

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[max+k-1] < v[max+k+1]) {
				x = v[max+k+1] // move down
			} else {
				x = v[max+k-1] + 1 // move right
			}
			y := x - k

			// Follow diagonal (matches)
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[max+k] = x

			if x >= n && y >= m {
				// Found the path, backtrack to build diff
				return backtrack(trace, a, b, max)
			}
		}
	}
	return nil
}

// backtrack reconstructs the diff from Myers trace.
func backtrack(trace [][]int, a, b []rune, max int) []diff {
	var diffs []diff
	x, y := len(a), len(b)

	for d := len(trace) - 1; d >= 0; d-- {
		k := x - y

		var prevK int
		var prevX, prevY int

		if d == 0 {
			// At d=0, we started at (0,0)
			prevX, prevY = 0, 0
		} else {
			v := trace[d-1]

			// Bounds check for k-1 and k+1
			kMinus := max + k - 1
			kPlus := max + k + 1

			if k == -d || (k != d && kMinus >= 0 && kPlus < len(v) && v[kMinus] < v[kPlus]) {
				prevK = k + 1
			} else {
				prevK = k - 1
			}

			prevIdx := max + prevK
			if prevIdx >= 0 && prevIdx < len(v) {
				prevX = v[prevIdx]
			}
			prevY = prevX - prevK
		}

		// Diagonal moves (equals)
		for x > prevX && y > prevY && x > 0 && y > 0 {
			x--
			y--
			diffs = append([]diff{{diffEqual, string(a[x])}}, diffs...)
		}

		if d > 0 {
			if x == prevX && y > 0 {
				// Vertical move = insert
				y--
				diffs = append([]diff{{diffInsert, string(b[y])}}, diffs...)
			} else if x > 0 {
				// Horizontal move = delete
				x--
				diffs = append([]diff{{diffDelete, string(a[x])}}, diffs...)
			}
		}
	}

	return diffs
}

// mergeDiffs combines adjacent diffs of the same type.
func mergeDiffs(diffs []diff) []diff {
	if len(diffs) == 0 {
		return nil
	}
	result := []diff{diffs[0]}
	for i := 1; i < len(diffs); i++ {
		last := &result[len(result)-1]
		if diffs[i].Op == last.Op {
			last.Text += diffs[i].Text
		} else {
			result = append(result, diffs[i])
		}
	}
	return result
}

// formatDiffs renders diffs with ANSI colors.
func formatDiffs(diffs []diff) string {
	const (
		red   = "\033[31m"
		green = "\033[32m"
		reset = "\033[0m"
		dim   = "\033[2m"
	)

	var sb strings.Builder
	for _, d := range diffs {
		switch d.Op {
		case diffEqual:
			sb.WriteString(dim + d.Text + reset)
		case diffDelete:
			sb.WriteString(red + d.Text + reset)
		case diffInsert:
			sb.WriteString(green + d.Text + reset)
		}
	}
	return sb.String()
}
