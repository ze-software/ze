package sitefleetdemo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// manifestWith answers a scratch checkout whose feature-gates.txt holds rows.
func manifestWith(t *testing.T, rows ...string) string {
	t.Helper()

	repository := t.TempDir()
	content := "# a scratch manifest\n" + strings.Join(rows, "\n") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(repository, "feature-gates.txt"), []byte(content), 0o644)) //nolint:gosec // a scratch checkout
	return repository
}

// TestFleetDemoAbsentRefusalNamesTheFix builds the refusal of a le without
// the fleet interface over scratch manifests, one declaring both fleet rows
// and one lacking each.
//
// VALIDATES: the refusal's fix is never the action that just failed. When the
// manifest declares ze_web and ze_fleet_ui, the le was built by hand and the
// fix runs the action through the checkout's launcher; when the manifest
// lacks a row, the fix appends that row.
// PREVENTS: a refusal that tells the operator to run the failing command
// again.
func TestFleetDemoAbsentRefusalNamesTheFix(t *testing.T) {
	const otherRow = "ze_bgp  internal/component/bgp"
	webRow := "ze_web  internal/component/web"
	fleetRow := "ze_fleet_ui  internal/component/web/fleetui"

	both := manifestWith(t, otherRow, webRow, fleetRow)
	refusal := absentRefusal(both)
	assert.Equal(t, absentProblem, refusal.Problem())
	assert.Equal(t, "cd "+shellQuote(both)+" && ./le site fleet-demo render", refusal.Fix(),
		"a manifest declaring both rows means a hand build, and the launcher builds le with both")
	assert.NotEqual(t, RenderCommand, refusal.Fix(), "the fix is not the command that failed")
	assert.Contains(t, refusal.Why(), "built by hand")

	for _, gate := range fleetGates {
		t.Run(gate.tag, func(t *testing.T) {
			var rows []string
			for _, row := range []string{otherRow, webRow, fleetRow} {
				if strings.Fields(row)[0] != gate.tag {
					rows = append(rows, row)
				}
			}
			repository := manifestWith(t, rows...)
			refusal := absentRefusal(repository)
			manifest := filepath.Join(repository, "feature-gates.txt")
			assert.Equal(t, "printf '%s\\n' '"+gate.tag+"  "+gate.packagePath+"' >> "+shellQuote(manifest), refusal.Fix(),
				"the fix restores the missing row")
			assert.Contains(t, refusal.Why(), "declares no "+gate.tag+" row")
		})
	}

	unreadable := t.TempDir()
	refusal = absentRefusal(unreadable)
	assert.Equal(t, "git -C "+shellQuote(unreadable)+" status -- feature-gates.txt", refusal.Fix(),
		"a manifest that cannot be read is named, never read as declaring nothing")
	assert.Error(t, refusal.Unwrap())
}

// TestFleetDemoGatesAreRowsOfTheManifest compares the refusal's copy of the
// two fleet rows with this checkout's feature-gates.txt.
//
// VALIDATES: each row fleetGates names, tag and package, is a row of the
// manifest, so the row a refusal restores is the row the manifest declares.
// PREVENTS: a copy that drifts from its source and restores a wrong row.
func TestFleetDemoGatesAreRowsOfTheManifest(t *testing.T) {
	root, err := lepath.Root()
	require.NoError(t, err)
	gates, err := repofeaturetags.Gates(root)
	require.NoError(t, err)

	for _, gate := range fleetGates {
		assert.Contains(t, gates, repofeaturetags.Gate{Tag: gate.tag, Package: gate.packagePath},
			"feature-gates.txt declares the row %s %s", gate.tag, gate.packagePath)
	}
}

// TestFleetDemoRefusalQuotesTheCheckoutPath writes the fix of a checkout
// whose path holds a space and a single quote.
//
// VALIDATES: the path is one shell word in the fix command, its quote
// escaped, so the command pasted as written acts on that checkout.
// PREVENTS: a fix that splits the path at the space and runs against
// another directory, or opens a quote it never closes.
func TestFleetDemoRefusalQuotesTheCheckoutPath(t *testing.T) {
	assert.Equal(t, `'/home/op/it'\''s here'`, shellQuote("/home/op/it's here"))
	assert.Equal(t, `'/srv/ze'`, shellQuote("/srv/ze"))
}
