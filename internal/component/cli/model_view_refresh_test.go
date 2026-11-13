package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIncidentalViewUnchangedWithoutTop proves the incidental views, which
// answer refreshConfig rather than a view built on the command's copy, draw
// what the copy-built view drew, at the root and inside a context.
//
// Method: each line runs through handleEnter and handleCommandResult, the
// path a typed line takes. The applied viewport is compared with
// configViewAtPath at the position the prompt names, rendered through
// setViewportData on the same model, which is what the copy-built view drew
// when no `top` emptied the copy's position.
//
// VALIDATES: AC-54, A-14, for the incidental sites a typed line reaches here:
// set, delete, deactivate, activate, discard all, option errors hints and
// hide, load file (applyLoad). `commit verify` with issues and a refused
// commit are not incidental views: they show their issues in the viewport
// (Q-7 a), which TestCommitVerifyShowsIssuesInTheViewport and
// TestCommitFixUsesTopInsideAContext prove.
// PREVENTS: an operator seeing a different view after a change with no `top`
// once the sites stop building their own view.
func TestIncidentalViewUnchangedWithoutTop(t *testing.T) {
	loadPath := filepath.Join(t.TempDir(), "load.conf")
	require.NoError(t, os.WriteFile(loadPath, []byte("bgp { router-id 9.9.9.9; }"), 0o600))

	cases := []struct {
		name   string
		setup  []string
		atRoot string
		inPeer string
	}{
		{name: "set", atRoot: "set bgp peer peer1 timer receive-hold-time 60", inPeer: "set timer receive-hold-time 60"},
		{name: "delete", atRoot: "delete bgp peer peer1 timer receive-hold-time", inPeer: "delete timer receive-hold-time"},
		{name: "deactivate", atRoot: "deactivate bgp peer peer1 timer", inPeer: "deactivate timer"},
		{
			name: "activate", setup: []string{"deactivate bgp peer peer1 timer"},
			atRoot: "activate bgp peer peer1 timer", inPeer: "activate timer",
		},
		{
			name: "discard all", setup: []string{"set bgp router-id 9.9.9.9"},
			atRoot: "discard all", inPeer: "discard all",
		},
		{name: "option errors hints", atRoot: "option errors hints", inPeer: "option errors hints"},
		{name: "option errors hide", atRoot: "option errors hide", inPeer: "option errors hide"},
		{
			name:   "load file",
			atRoot: "load file absolute merge " + loadPath, inPeer: "load file absolute merge " + loadPath,
		},
	}
	for _, tc := range cases {
		for _, inPeer := range []bool{false, true} {
			name := tc.name + " at the root"
			line := tc.atRoot
			if inPeer {
				name = tc.name + " in a context"
				line = tc.inPeer
			}
			t.Run(name, func(t *testing.T) {
				m, _ := topTestModel(t)
				for _, setup := range tc.setup {
					m = enterLine(t, m, setup)
					require.NoError(t, m.err, "setup line %q", setup)
				}
				if inPeer {
					m = enterLine(t, m, "edit bgp peer peer1")
					require.Equal(t, topPeerContext, m.contextPath)
				}
				position := m.contextPath

				out, result := enterLineResult(t, m, line)
				require.NoError(t, out.err)
				assert.True(t, result.refreshConfig, "an incidental view answers a refresh")
				assert.Nil(t, result.configView, "an incidental view is not built on the copy")
				assert.Equal(t, position, out.contextPath)

				ref := out
				ref.setViewportData(*ref.configViewAtPath(position))
				assert.Equal(t, ref.viewportContent, out.viewportContent, "the view at the prompt's position")
				assert.Equal(t, ref.viewport.YOffset(), out.viewport.YOffset(), "the scroll position")
				assert.Equal(t, ref.showingConfig, out.showingConfig)
				assertIncidentalView(t, m, out, result, line, result.statusMessage)
			})
		}
	}
}
