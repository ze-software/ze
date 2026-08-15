package markupcheck_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/markupcheck"
)

// writeTempl builds a throwaway directory holding one page.templ, so no
// committed fixture can drift from the rules below.
func writeTempl(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "page.templ"), []byte(body), 0o600))

	return dir
}

// TestInlineFindingsNamesEachRefusedForm proves the scan reports the three
// things a strict CSP refuses, one message each.
//
// VALIDATES: an inline handler is a red test rather than a dead button.
// PREVENTS: the lg graph-mode outage returning. Its onclick ran nowhere under
// default-src 'self', and no test in the package can see that.
func TestInlineFindingsNamesEachRefusedForm(t *testing.T) {
	dir := writeTempl(t, "package p\n"+
		"templ a() {\n"+
		"\t<script>alert(1)</script>\n"+
		"\t<div style=\"color:red\"></div>\n"+
		"\t<button onclick=\"go()\">Go</button>\n"+
		"}\n")

	findings, files, err := markupcheck.InlineFindings(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, files)
	require.Len(t, findings, 3)
	assert.Contains(t, findings[0], "<script>")
	assert.Contains(t, findings[1], "style")
	assert.Contains(t, findings[2], "event handler")
}

// TestInlineFindingsPassesExternalAssets proves the scan accepts the form that
// works: a script loaded from the package's own origin, and behavior reached
// through a data attribute.
//
// VALIDATES: the check discriminates. A gate that reds on the fix would be
// turned off rather than obeyed.
// PREVENTS: a false finding teaching the next reader to ignore this check.
func TestInlineFindingsPassesExternalAssets(t *testing.T) {
	dir := writeTempl(t, "package p\n"+
		"templ a() {\n"+
		"\t<link rel=\"stylesheet\" href=\"/assets/style.css\">\n"+
		"\t<script src=\"/assets/graph-mode.js\" defer></script>\n"+
		"\t<button class=\"graph-mode-btn\" data-mode=\"aspath\">Go</button>\n"+
		"}\n")

	findings, files, err := markupcheck.InlineFindings(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, files)
	assert.Empty(t, findings)
}

// TestInlineFindingsReadsOnlyTemplSources proves the scan reads the source a
// person writes. The generated *_templ.go holds the same markup as a Go string,
// and counting it twice would report each finding twice.
//
// VALIDATES: the scan's scope.
// PREVENTS: a duplicate message for one defect.
func TestInlineFindingsReadsOnlyTemplSources(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "page_templ.go"),
		[]byte("package p\nfunc a() string { return `<button onclick=\"go()\">` }\n"), 0o600))

	findings, files, err := markupcheck.InlineFindings(dir)
	require.NoError(t, err)
	assert.Zero(t, files, "no .templ source is present, which AssertNoInlineScriptOrStyle refuses")
	assert.Empty(t, findings)
}

// TestShortfallReportsOnlyBelowTheFloor proves the vacuity guard fires one step
// below its floor and stays quiet at it and above it.
//
// VALIDATES: the boundary of every floor in this package.
// PREVENTS: an off-by-one turning a floor into a hole, or into noise.
func TestShortfallReportsOnlyBelowTheFloor(t *testing.T) {
	assert.NotEmpty(t, markupcheck.Shortfall("files", 39, 40), "one below the floor is a finding")
	assert.Empty(t, markupcheck.Shortfall("files", 40, 40), "the floor itself passes")
	assert.Empty(t, markupcheck.Shortfall("files", 41, 40), "above the floor passes")
	assert.Empty(t, markupcheck.Shortfall("files", 0, 0), "a zero floor asks for nothing")
}
