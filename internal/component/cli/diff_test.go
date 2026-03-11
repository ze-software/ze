package cli

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeAnnotatedDiffIdentical verifies identical content produces all unchanged markers.
//
// VALIDATES: No diff markers when content is identical.
// PREVENTS: False positives in diff gutter.
func TestComputeAnnotatedDiffIdentical(t *testing.T) {
	original := "line 1\nline 2\nline 3"
	result := computeAnnotatedDiff(original, original)

	require.Len(t, result, 3)
	for _, dl := range result {
		assert.Equal(t, diffUnchanged, dl.Marker)
	}
}

// TestComputeAnnotatedDiffAllAdded verifies empty original marks all lines as added.
//
// VALIDATES: New config from scratch shows all '+' markers.
// PREVENTS: Missing markers for new content.
func TestComputeAnnotatedDiffAllAdded(t *testing.T) {
	result := computeAnnotatedDiff("", "line 1\nline 2")

	require.Len(t, result, 2)
	assert.Equal(t, diffAdded, result[0].Marker)
	assert.Equal(t, "line 1", result[0].Text)
	assert.Equal(t, diffAdded, result[1].Marker)
	assert.Equal(t, "line 2", result[1].Text)
}

// TestComputeAnnotatedDiffAllRemoved verifies empty modified marks all lines as removed.
//
// VALIDATES: Clearing all config shows all '-' markers.
// PREVENTS: Missing markers for removed content.
func TestComputeAnnotatedDiffAllRemoved(t *testing.T) {
	result := computeAnnotatedDiff("line 1\nline 2", "")

	require.Len(t, result, 2)
	assert.Equal(t, diffRemoved, result[0].Marker)
	assert.Equal(t, diffRemoved, result[1].Marker)
}

// TestComputeAnnotatedDiffBothEmpty verifies no output for empty inputs.
//
// VALIDATES: No diff lines when both sides are empty.
// PREVENTS: Nil panic or spurious output.
func TestComputeAnnotatedDiffBothEmpty(t *testing.T) {
	result := computeAnnotatedDiff("", "")
	assert.Nil(t, result)
}

// TestComputeAnnotatedDiffModified verifies adjacent changed lines with same indent become '|'.
//
// VALIDATES: Value change at same indent level produces modified marker.
// PREVENTS: Showing separate -/+ for simple value edits.
func TestComputeAnnotatedDiffModified(t *testing.T) {
	original := "  hold-time 30;"
	modified := "  hold-time 90;"

	result := computeAnnotatedDiff(original, modified)

	require.Len(t, result, 1)
	assert.Equal(t, diffModified, result[0].Marker)
	assert.Equal(t, "  hold-time 90;", result[0].Text)
}

// TestComputeAnnotatedDiffMixed verifies a realistic config change.
//
// VALIDATES: Mixed unchanged/modified/added lines produce correct markers.
// PREVENTS: LCS alignment errors in real config diffs.
func TestComputeAnnotatedDiffMixed(t *testing.T) {
	original := "bgp {\n  router-id 1.2.3.4;\n  hold-time 30;\n}"
	modified := "bgp {\n  router-id 1.2.3.4;\n  hold-time 90;\n  local-as 65000;\n}"

	result := computeAnnotatedDiff(original, modified)

	require.Len(t, result, 5)
	assert.Equal(t, diffUnchanged, result[0].Marker, "bgp {")
	assert.Equal(t, diffUnchanged, result[1].Marker, "router-id")
	assert.Equal(t, diffModified, result[2].Marker, "hold-time changed")
	assert.Equal(t, "  hold-time 90;", result[2].Text)
	assert.Equal(t, diffAdded, result[3].Marker, "local-as added")
	assert.Equal(t, diffUnchanged, result[4].Marker, "}")
}

// TestComputeAnnotatedDiffRemovedLine verifies a removed line gets '-' marker.
//
// VALIDATES: Deleting a line from config shows it as removed.
// PREVENTS: Missing removed line in diff output.
func TestComputeAnnotatedDiffRemovedLine(t *testing.T) {
	original := "a\nb\nc"
	modified := "a\nc"

	result := computeAnnotatedDiff(original, modified)

	require.Len(t, result, 3)
	assert.Equal(t, diffUnchanged, result[0].Marker)
	assert.Equal(t, "a", result[0].Text)
	assert.Equal(t, diffRemoved, result[1].Marker)
	assert.Equal(t, "b", result[1].Text)
	assert.Equal(t, diffUnchanged, result[2].Marker)
	assert.Equal(t, "c", result[2].Text)
}

// TestDetectModificationsDifferentIndent verifies different indent stays as -/+.
//
// VALIDATES: Lines with different indent are not collapsed into modified.
// PREVENTS: False modification detection across indent levels.
func TestDetectModificationsDifferentIndent(t *testing.T) {
	lines := []diffLine{
		{diffRemoved, "  value 1;"},
		{diffAdded, "    value 2;"},
	}

	result := detectModifications(lines)

	require.Len(t, result, 2)
	assert.Equal(t, diffRemoved, result[0].Marker)
	assert.Equal(t, diffAdded, result[1].Marker)
}

// TestDetectModificationsSameIndent verifies same indent collapses to modified.
//
// VALIDATES: Adjacent -/+ with matching indent become '|'.
// PREVENTS: Noisy -/+ pairs for simple value changes.
func TestDetectModificationsSameIndent(t *testing.T) {
	lines := []diffLine{
		{diffRemoved, "  value 1;"},
		{diffAdded, "  value 2;"},
	}

	result := detectModifications(lines)

	require.Len(t, result, 1)
	assert.Equal(t, diffModified, result[0].Marker)
	assert.Equal(t, "  value 2;", result[0].Text)
}

// TestAnnotateContentWithGutterNoChanges verifies no gutter for identical content.
//
// VALIDATES: Identical content returns unchanged string and nil mapping.
// PREVENTS: Unnecessary gutter prefix on unchanged config.
func TestAnnotateContentWithGutterNoChanges(t *testing.T) {
	content := "line 1\nline 2"
	result, mapping := annotateContentWithGutter(content, content)

	assert.Equal(t, content, result)
	assert.Nil(t, mapping)
}

// TestAnnotateContentWithGutterMarkers verifies gutter markers appear in output.
//
// VALIDATES: Modified lines get '|' prefix, unchanged get ' ' prefix.
// PREVENTS: Missing or incorrect gutter characters.
func TestAnnotateContentWithGutterMarkers(t *testing.T) {
	original := "line 1\nline 2"
	modified := "line 1\nline 3"

	result, mapping := annotateContentWithGutter(original, modified)

	assert.Contains(t, result, "  line 1", "unchanged line should have space prefix")
	assert.Contains(t, result, "| line 3", "modified line should have | prefix")
	assert.NotNil(t, mapping)
}

// TestAnnotateContentWithGutterLineMapping verifies line mapping for validation.
//
// VALIDATES: Removed lines produce correct mapping gaps.
// PREVENTS: Validation highlighting on wrong lines when removed lines shift display.
func TestAnnotateContentWithGutterLineMapping(t *testing.T) {
	original := "a\nb\nc"
	modified := "a\nc" // b removed

	_, mapping := annotateContentWithGutter(original, modified)

	require.NotNil(t, mapping)

	// Display: line 1 = "a" (unchanged), line 2 = "b" (removed), line 3 = "c" (unchanged)
	assert.Equal(t, 1, mapping[1], "display line 1 → working line 1 (a)")
	assert.Equal(t, 2, mapping[3], "display line 3 → working line 2 (c)")

	// Removed line should not be in mapping (absent key → zero value)
	_, exists := mapping[2]
	assert.False(t, exists, "removed line should not have a mapping entry")
}

// TestAnnotateContentWithGutterAddedLines verifies added lines get '+' and mapping.
//
// VALIDATES: Added lines appear with '+' prefix and map to correct working line.
// PREVENTS: Added lines missing from mapping or showing wrong marker.
func TestAnnotateContentWithGutterAddedLines(t *testing.T) {
	original := "a\nc"
	modified := "a\nb\nc" // b added

	result, mapping := annotateContentWithGutter(original, modified)

	assert.Contains(t, result, "+ b", "added line should have + prefix")
	require.NotNil(t, mapping)

	// Display: line 1 = "a" (unchanged), line 2 = "b" (added), line 3 = "c" (unchanged)
	assert.Equal(t, 1, mapping[1], "a → working line 1")
	assert.Equal(t, 2, mapping[2], "b → working line 2")
	assert.Equal(t, 3, mapping[3], "c → working line 3")
}

// TestAnnotateContentWithGutterTrailingNewline verifies trailing newline handling.
//
// VALIDATES: Content with trailing newline produces same result as without.
// PREVENTS: Spurious empty line in diff output.
func TestAnnotateContentWithGutterTrailingNewline(t *testing.T) {
	original := "a\nb\n"
	modified := "a\nc\n"

	result, mapping := annotateContentWithGutter(original, modified)

	assert.Contains(t, result, "  a", "unchanged line")
	assert.Contains(t, result, "| c", "modified line")
	assert.NotNil(t, mapping)
}

// TestDetectModificationsConsecutiveRuns verifies multi-line -/+ runs collapse to '|'.
//
// VALIDATES: Consecutive removed+added runs with matching indent produce modified markers.
// PREVENTS: Only first pair collapsing when multiple lines change together.
func TestDetectModificationsConsecutiveRuns(t *testing.T) {
	lines := []diffLine{
		{diffRemoved, "  hold-time 30;"},
		{diffRemoved, "  keepalive 10;"},
		{diffAdded, "  hold-time 90;"},
		{diffAdded, "  keepalive 5;"},
	}

	result := detectModifications(lines)

	require.Len(t, result, 2)
	assert.Equal(t, diffModified, result[0].Marker)
	assert.Equal(t, "  hold-time 90;", result[0].Text)
	assert.Equal(t, diffModified, result[1].Marker)
	assert.Equal(t, "  keepalive 5;", result[1].Text)
}

// TestDetectModificationsUnevenRuns verifies unpaired lines survive in runs.
//
// VALIDATES: When removed count != added count, extras keep their original marker.
// PREVENTS: Lost lines when runs have different lengths.
func TestDetectModificationsUnevenRuns(t *testing.T) {
	lines := []diffLine{
		{diffRemoved, "  a;"},
		{diffRemoved, "  b;"},
		{diffRemoved, "  c;"},
		{diffAdded, "  x;"},
		{diffAdded, "  y;"},
	}

	result := detectModifications(lines)

	require.Len(t, result, 3)
	assert.Equal(t, diffModified, result[0].Marker, "a→x")
	assert.Equal(t, diffModified, result[1].Marker, "b→y")
	assert.Equal(t, diffRemoved, result[2].Marker, "c unpaired")
	assert.Equal(t, "  c;", result[2].Text)
}

// TestAnnotateContentWithGutterNewBlock verifies all-new content shows '+' markers.
//
// VALIDATES: Empty original with non-empty modified produces all '+' gutter markers.
// PREVENTS: New blocks missing gutter annotation when originalContent is "".
func TestAnnotateContentWithGutterNewBlock(t *testing.T) {
	modified := "bgp {\n  router-id 1.2.3.4;\n}"

	result, mapping := annotateContentWithGutter("", modified)

	assert.Contains(t, result, "+ bgp {", "new block should show + marker")
	assert.Contains(t, result, "+   router-id 1.2.3.4;", "new leaf should show + marker")
	assert.Contains(t, result, "+ }", "closing brace should show + marker")
	require.NotNil(t, mapping)
	assert.Equal(t, 1, mapping[1])
	assert.Equal(t, 2, mapping[2])
	assert.Equal(t, 3, mapping[3])
}

// TestSetViewportDataAppliesGutter verifies gutter markers flow through setViewportData.
//
// VALIDATES: The rendering pipeline applies diff gutter when originalContent differs.
// PREVENTS: Gutter algorithm working in isolation but not wired into viewport display.
func TestSetViewportDataAppliesGutter(t *testing.T) {
	vp := viewport.New(80, 24)
	m := Model{viewport: vp}

	m.setViewportData(viewportData{
		content:         "line 1\nline 3",
		originalContent: "line 1\nline 2",
		hasOriginal:     true,
	})

	assert.True(t, m.showViewport)
	assert.Contains(t, m.viewportContent, "  line 1", "unchanged line gets space prefix")
	assert.Contains(t, m.viewportContent, "| line 3", "modified line gets | prefix")
}

// TestSetViewportDataNewBlockGutter verifies new blocks get '+' through the pipeline.
//
// VALIDATES: Empty originalContent with content triggers all '+' gutter markers.
// PREVENTS: Guard skipping gutter for newly-added config blocks.
func TestSetViewportDataNewBlockGutter(t *testing.T) {
	vp := viewport.New(80, 24)
	m := Model{viewport: vp}

	m.setViewportData(viewportData{
		content:         "new line",
		originalContent: "",
		hasOriginal:     true,
	})

	assert.True(t, m.showViewport)
	assert.Contains(t, m.viewportContent, "+ new line", "new content gets + prefix")
}

// TestLeadingWhitespace verifies whitespace extraction.
//
// VALIDATES: Leading spaces/tabs are correctly extracted.
// PREVENTS: Incorrect modification detection from bad indent comparison.
func TestLeadingWhitespace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  value;", "  "},
		{"\tvalue;", "\t"},
		{"value;", ""},
		{"    ", "    "},
		{"", ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, leadingWhitespace(tt.input), "input: %q", tt.input)
	}
}
