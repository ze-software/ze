// Design: docs/architecture/config/yang-config-design.md — config diff annotation
// Related: model_render.go — viewport rendering with gutter markers

package editor

import "strings"

// diffMarker represents the change status of a line in a diff gutter.
type diffMarker byte

const (
	// diffUnchanged marks a line present in both original and modified.
	diffUnchanged diffMarker = ' '
	// diffAdded marks a line present only in modified.
	diffAdded diffMarker = '+'
	// diffRemoved marks a line present only in original.
	diffRemoved diffMarker = '-'
	// diffModified marks a line where the value changed (same indent, different content).
	diffModified diffMarker = '|'
)

// diffLine is a single line in the annotated diff output.
type diffLine struct {
	Marker diffMarker
	Text   string
}

// computeAnnotatedDiff produces a full annotated diff between original and modified content.
// Every line appears with a change marker: unchanged, added, removed, or modified.
// Uses LCS (longest common subsequence) for line alignment.
func computeAnnotatedDiff(original, modified string) []diffLine {
	origLines := splitDiffLines(original)
	modLines := splitDiffLines(modified)

	if len(origLines) == 0 && len(modLines) == 0 {
		return nil
	}

	if len(origLines) == 0 {
		result := make([]diffLine, len(modLines))
		for i, line := range modLines {
			result[i] = diffLine{diffAdded, line}
		}
		return result
	}

	if len(modLines) == 0 {
		result := make([]diffLine, len(origLines))
		for i, line := range origLines {
			result[i] = diffLine{diffRemoved, line}
		}
		return result
	}

	dp := lcsTable(origLines, modLines)
	raw := backtrackDiff(dp, origLines, modLines)
	return detectModifications(raw)
}

// annotateContentWithGutter takes original and modified content and returns
// the annotated content with gutter markers prepended to each line.
// Returns the annotated string and a line mapping from displayed line (1-based)
// to working content line (1-based). Removed lines have no mapping entry
// (absent key → zero value, which highlightValidationIssues treats as "skip").
func annotateContentWithGutter(original, modified string) (string, map[int]int) {
	if original == modified {
		return modified, nil
	}

	diffs := computeAnnotatedDiff(original, modified)
	if len(diffs) == 0 {
		return modified, nil
	}

	var b strings.Builder
	lineMapping := make(map[int]int)
	displayLine := 0
	workingLine := 0

	for _, dl := range diffs {
		if displayLine > 0 {
			b.WriteByte('\n')
		}
		displayLine++

		b.WriteByte(byte(dl.Marker))
		b.WriteByte(' ')
		b.WriteString(dl.Text)

		switch dl.Marker {
		case diffUnchanged, diffAdded, diffModified:
			workingLine++
			lineMapping[displayLine] = workingLine
		case diffRemoved:
			// No corresponding working line — absent from mapping
		}
	}

	return b.String(), lineMapping
}

// splitDiffLines splits content into lines, removing a trailing empty line
// that strings.Split produces from a trailing newline.
func splitDiffLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// lcsTable computes the longest common subsequence DP table for two slices of strings.
func lcsTable(a, b []string) [][]int {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			switch {
			case a[i-1] == b[j-1]:
				dp[i][j] = dp[i-1][j-1] + 1
			case dp[i-1][j] >= dp[i][j-1]:
				dp[i][j] = dp[i-1][j]
			case dp[i-1][j] < dp[i][j-1]:
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	return dp
}

// backtrackDiff walks the LCS table backwards to produce a raw diff sequence.
func backtrackDiff(dp [][]int, a, b []string) []diffLine {
	i, j := len(a), len(b)
	var rev []diffLine

	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			rev = append(rev, diffLine{diffUnchanged, b[j-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			rev = append(rev, diffLine{diffAdded, b[j-1]})
			j--
		case i > 0:
			rev = append(rev, diffLine{diffRemoved, a[i-1]})
			i--
		}
	}

	result := make([]diffLine, len(rev))
	for k, line := range rev {
		result[len(rev)-1-k] = line
	}
	return result
}

// detectModifications converts adjacent removed/added runs into modified markers.
// A run of N removed lines followed by N added lines where each pair shares
// leading whitespace is treated as N modifications: show the new versions with '|'.
// Unmatched trailing lines in either run are emitted with their original marker.
func detectModifications(lines []diffLine) []diffLine {
	result := make([]diffLine, 0, len(lines))

	i := 0
	for i < len(lines) {
		// Scan a run of consecutive removed lines
		if lines[i].Marker != diffRemoved {
			result = append(result, lines[i])
			i++
			continue
		}

		remStart := i
		for i < len(lines) && lines[i].Marker == diffRemoved {
			i++
		}
		remEnd := i

		// Scan a run of consecutive added lines
		addStart := i
		for i < len(lines) && lines[i].Marker == diffAdded {
			i++
		}
		addEnd := i

		// Pair up removed/added lines with matching indent → modified
		remCount := remEnd - remStart
		addCount := addEnd - addStart
		paired := min(remCount, addCount)

		for k := range paired {
			rem := lines[remStart+k]
			add := lines[addStart+k]
			if leadingWhitespace(rem.Text) == leadingWhitespace(add.Text) {
				result = append(result, diffLine{diffModified, add.Text})
			} else {
				result = append(result, rem, add)
			}
		}

		// Emit unpaired remainder
		for k := range remCount - paired {
			result = append(result, lines[remStart+paired+k])
		}
		for k := range addCount - paired {
			result = append(result, lines[addStart+paired+k])
		}
	}

	return result
}

// leadingWhitespace returns the leading whitespace prefix of a string.
func leadingWhitespace(s string) string {
	for i, c := range s {
		if c != ' ' && c != '\t' {
			return s[:i]
		}
	}
	return s
}
