package markupcheck_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/markupcheck"
)

// writeFixtures builds a throwaway capture directory, so no committed fixture
// can drift from the rules below.
func writeFixtures(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()

	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}

	return dir
}

// page wraps one body in the smallest whole page: a head loading the named
// assets, then the markup.
func page(head, body string) string {
	return "<!doctype html><html><head>" + head + "</head><body>" + body + "</body></html>"
}

// TestHeadCoverageNamesAnAttributeTheHeadDoesNotLoad proves a page rendering an
// attribute whose asset it never loads is a finding.
//
// VALIDATES: the under-approximating half of the pair reports the page and the
// attribute.
// PREVENTS: the failure the whole check exists for. Every byte is the byte the
// handler meant to write, so no other gate can see it: only the browser does,
// by doing nothing.
func TestHeadCoverageNamesAnAttributeTheHeadDoesNotLoad(t *testing.T) {
	dir := writeFixtures(t, map[string]string{
		"peers.txt": page(
			`<script src="/assets/htmx.min.js"></script>`,
			`<div hx-ext="sse" sse-connect="/events"></div>`),
	})

	findings, pages, err := markupcheck.HeadCoverageFindings(dir, "/assets/")
	require.NoError(t, err)
	assert.Equal(t, 1, pages)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0], "peers.txt")
	assert.Contains(t, findings[0], "sse-connect")
	assert.Contains(t, findings[0], "sse.js")
}

// TestHeadCoveragePassesAPageThatLoadsWhatItRenders proves a head carrying the
// asset of every attribute below it is not a finding, and that a page loading
// more than it renders is not one either.
//
// VALIDATES: the check discriminates, and it allows the over-approximation the
// generator is entitled to. An asset shipped and unused costs bytes; the other
// direction costs the page.
func TestHeadCoveragePassesAPageThatLoadsWhatItRenders(t *testing.T) {
	dir := writeFixtures(t, map[string]string{
		"peers.txt": page(
			`<script src="/assets/htmx.min.js"></script><script src="/assets/sse.js"></script>`,
			`<div hx-get="/peers" sse-swap="peer-update"></div>`),
		"search.txt": page(
			`<script src="/assets/htmx.min.js"></script><script src="/assets/sse.js"></script>`,
			`<form hx-post="/search"></form>`),
	})

	findings, pages, err := markupcheck.HeadCoverageFindings(dir, "/assets/")
	require.NoError(t, err)
	assert.Equal(t, 2, pages)
	assert.Empty(t, findings)
}

// TestHeadCoverageReadsNoFragment proves a captured fragment is not a page.
//
// A fragment is swapped into a page that has already loaded its assets, so it
// states nothing about what it needs. Reading one as a page would report every
// fragment carrying an attribute, which is every fragment.
//
// VALIDATES: only a capture holding a head is judged.
// PREVENTS: a check nobody can keep green, which is a check that gets deleted.
func TestHeadCoverageReadsNoFragment(t *testing.T) {
	dir := writeFixtures(t, map[string]string{
		"row.html": `<tr hx-get="/peer/1" hx-target="#detail"></tr>`,
	})

	findings, pages, err := markupcheck.HeadCoverageFindings(dir, "/assets/")
	require.NoError(t, err)
	assert.Zero(t, pages)
	assert.Empty(t, findings)
}
