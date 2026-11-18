package markupcheck

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// servedAssets is what the file server hands out, which is the sub-FS the
// package embeds. A test that resolved against the embed root instead would
// pass on a path the server answers 404 for.
func servedAssets(names ...string) fstest.MapFS {
	served := fstest.MapFS{}
	for _, n := range names {
		served[n] = &fstest.MapFile{Data: []byte("x")}
	}

	return served
}

func writeAssetTempl(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "page.templ"), []byte(body), 0o600))

	return dir
}

// TestAssetFindingsNamesAPathNothingServes proves a reference the embedded FS
// cannot answer is a finding.
//
// VALIDATES: a renamed or deleted asset reds a test.
// PREVENTS: the failure this scan exists for. A page whose script 404s renders
// and reports success, so the live view stops updating.
func TestAssetFindingsNamesAPathNothingServes(t *testing.T) {
	dir := writeAssetTempl(t, "package p\n"+
		"templ a() {\n"+
		"\t<script src=\"/assets/snapshot-live.js\" defer></script>\n"+
		"\t<link rel=\"stylesheet\" href=\"/assets/style.css\">\n"+
		"}\n")

	findings, refs, err := assetFindings(dir, "/assets/", servedAssets("style.css"))
	require.NoError(t, err)
	assert.Equal(t, 2, refs)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0], "snapshot-live.js")
}

// TestAssetFindingsPassesEveryServedPath proves a reference the FS answers is
// not a finding, and that a templ expression attribute is not read as a path.
//
// VALIDATES: the scan discriminates.
// PREVENTS: a false finding on `href={ v.URL }`, which no static check can
// resolve.
func TestAssetFindingsPassesEveryServedPath(t *testing.T) {
	dir := writeAssetTempl(t, "package p\n"+
		"templ a() {\n"+
		"\t<script src=\"/assets/htmx.min.js\"></script>\n"+
		"\t<a href={ v.URL }>go</a>\n"+
		"\t<a href=\"/show/\">show</a>\n"+
		"}\n")

	findings, refs, err := assetFindings(dir, "/assets/", servedAssets("htmx.min.js"))
	require.NoError(t, err)
	assert.Equal(t, 1, refs, "only the one path under the prefix is an asset reference")
	assert.Empty(t, findings)
}

// TestAssetFindingsRefusesAnAssetPathOffThisPrefix proves a reference to an
// asset tree this package does not serve is a finding rather than a silent
// skip.
//
// The looking glass serves /lg/assets/ and the web UI serves /assets/. A page
// naming the other one resolves against nothing, and a scan that only matched
// its own prefix would pass over it.
//
// VALIDATES: the scan is fail-closed on a path it cannot resolve.
// PREVENTS: a copied component pointing at the sibling package's asset tree.
func TestAssetFindingsRefusesAnAssetPathOffThisPrefix(t *testing.T) {
	dir := writeAssetTempl(t, "package p\n"+
		"templ a() {\n"+
		"\t<script src=\"/assets/theme.js\"></script>\n"+
		"}\n")

	findings, refs, err := assetFindings(dir, "/lg/assets/", servedAssets("theme.js"))
	require.NoError(t, err)
	assert.Zero(t, refs)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0], "/lg/assets/")
}

// TestAssetFindingsLeavesANestedPackageToItsOwnScan puts a package that
// serves its own asset tree in a directory under the scanned one.
//
// The fleet interface nests under the web package and serves /fleet/assets/.
// Its templates are that package's to scan, against that package's served
// filesystem. Read from the web package's root, each of its stylesheets is a
// path off the web prefix, which the scan reports.
//
// VALIDATES: the scan reads the directory it is given and no package under
// it, and still reads that directory's own templates.
// PREVENTS: a nested package's asset paths reported against the asset tree of
// the package above it.
func TestAssetFindingsLeavesANestedPackageToItsOwnScan(t *testing.T) {
	dir := writeAssetTempl(t, "package p\n"+
		"templ a() {\n"+
		"\t<script src=\"/assets/htmx.min.js\"></script>\n"+
		"}\n")

	nested := filepath.Join(dir, "nested")
	require.NoError(t, os.Mkdir(nested, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "page.templ"), []byte("package nested\n"+
		"templ b() {\n"+
		"\t<link rel=\"stylesheet\" href=\"/nested/assets/shell.css\">\n"+
		"}\n"), 0o600))

	findings, refs, err := assetFindings(dir, "/assets/", servedAssets("htmx.min.js"))
	require.NoError(t, err)
	assert.Equal(t, 1, refs, "the scanned directory's own reference is still resolved")
	assert.Empty(t, findings)

	findings, refs, err = assetFindings(nested, "/nested/assets/", servedAssets("shell.css"))
	require.NoError(t, err)
	assert.Equal(t, 1, refs, "the nested package's own scan resolves its reference")
	assert.Empty(t, findings)
}
