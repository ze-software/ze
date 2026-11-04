// Design: docs/architecture/config/yang-config-design.md -- config editor validation
package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
)

// withPeerValidator installs a peer validator on the infra seam for one test and
// restores it to nil. Restoring nil is correct here: this test binary does not
// link internal/component/bgp/config, so the seam starts nil and a test that
// wants the engine's answer supplies it. The rule under test is the EDITOR
// reaching that seam at all, which is what the editor never did.
func withPeerValidator(t *testing.T, fn func(*config.Tree) error) {
	t.Helper()
	infra.SetBGPPeerValidator(fn)
	t.Cleanup(func() { infra.SetBGPPeerValidator(nil) })
}

// editorOnConfig writes a config file and returns a model over it.
func editorOnConfig(t *testing.T) Model {
	content := testValidBGPConfigWithPeer
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	ed, err := NewLooseFileEditor(nil, path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() }) //nolint:errcheck // test cleanup
	ed.MarkDirty()

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	return model
}

// TestEditorCommitBlocksOnPeerPipelineRefusal drives `commit` over a config the
// BGP peer pipeline refuses, through a stubbed peer validator that answers
// one refusal naming the peer.
//
// VALIDATES: AC-39. The refusal reads as a blocked commit naming the peer,
// while the operator still has the config on the screen, rather than as a
// `commit failed:` line about a file already written.
// PREVENTS: the editor accepting every peer-pipeline refusal and discovering it
// one step later at reload. That gap hid EVERY such refusal.
func TestEditorCommitBlocksOnPeerPipelineRefusal(t *testing.T) {
	refusal := "peer peer1: the BGP peer pipeline refuses this config"
	withPeerValidator(t, func(*config.Tree) error { return errors.New(refusal) })

	model := editorOnConfig(t)
	reloaded := false
	model.editor.SetReloadNotifier(func() error {
		reloaded = true
		return nil
	})

	result, err := model.cmdCommit()
	require.NoError(t, err, "a blocked commit is reported as command status")

	assert.Contains(t, result.statusMessage, "commit blocked")
	assert.False(t, reloaded, "the commit must not reach the daemon")

	validation := model.validator.ValidateTransition(
		model.editor.OriginalContent(), model.editor.WorkingContent())
	require.NotEmpty(t, validation.Errors, "the refusal is a validation error, not a warning")
	assert.Contains(t, validation.Errors[0].Message, "peer1", "the operator is told which peer")
}

// TestEditorCommitPassesWhenThePeerPipelineAccepts is the other half: the same
// path with an accepting engine commits.
//
// VALIDATES: the new call adds no refusal of its own.
// PREVENTS: every commit blocking because the editor mistook a nil error for a
// failure, which would make the editor unusable rather than merely late.
func TestEditorCommitPassesWhenThePeerPipelineAccepts(t *testing.T) {
	asked := false
	withPeerValidator(t, func(*config.Tree) error {
		asked = true
		return nil
	})

	model := editorOnConfig(t)
	result := model.validator.ValidateTransition(
		model.editor.OriginalContent(), model.editor.WorkingContent())

	assert.True(t, asked, "the editor consults the peer pipeline for a config carrying a bgp block")
	assert.Empty(t, result.Errors)
}

// TestEditorValidationSkipsTheEngineWithoutABGPBlock checks the guard.
//
// VALIDATES: a tree with no bgp block, and a tree that failed to parse at all,
// never reach the BGP seam.
// PREVENTS: paying for a peer resolution on a config that declares no BGP, and
// handing a nil tree to the engine after a parse error.
func TestEditorValidationSkipsTheEngineWithoutABGPBlock(t *testing.T) {
	asked := false
	withPeerValidator(t, func(*config.Tree) error {
		asked = true
		return errors.New("must not be consulted")
	})

	validator, err := NewConfigValidator()
	require.NoError(t, err)

	assert.Empty(t, validator.bgpPeerErrors(nil), "a config that did not parse has no peer to validate")
	assert.Empty(t, validator.bgpPeerErrors(config.NewTree()), "no bgp block, no peer to validate")
	assert.False(t, asked, "the seam is not consulted in either case")
}

// TestEditorCommitConfirmedBlocksOnPeerPipelineRefusal drives the OTHER commit
// command over the same stubbed refusal.
//
// VALIDATES: AC-39 on the safe-commit path. `commit confirmed <N>` writes the
// trial config to .conf and starts a rollback timer, so a config the daemon
// refuses would be written and then fail at reload, which is the failure AC-39
// exists to prevent.
// PREVENTS: the guard covering one commit command and not its sibling.
// cmdCommitConfirmed validated with Validate, which never reaches the BGP peer
// pipeline; cmdCommit validated with ValidateTransition, which does. Found by
// the closure review, fixed by making both commit commands take one path.
func TestEditorCommitConfirmedBlocksOnPeerPipelineRefusal(t *testing.T) {
	refusal := "peer peer1: the BGP peer pipeline refuses this config"
	withPeerValidator(t, func(*config.Tree) error { return errors.New(refusal) })

	model := editorOnConfig(t)
	reloaded := false
	model.editor.SetReloadNotifier(func() error {
		reloaded = true
		return nil
	})

	_, err := model.cmdCommitConfirmed(60, false)
	require.Error(t, err, "the refused config must not reach .conf")
	assert.Contains(t, err.Error(), "peer1", "the operator is told which peer")
	assert.False(t, reloaded, "the commit must not reach the daemon")
}

// TestEditorCommitConfirmedForcedStillBlocksOnAnError is the force half.
//
// VALIDATES: `commit confirmed <N> force` skips WARNINGS, never errors. A peer
// pipeline refusal is an error, so force does not carry it past.
// PREVENTS: force reading as "commit whatever I typed", which would put the
// safe-commit path back where it was for any operator who types force.
func TestEditorCommitConfirmedForcedStillBlocksOnAnError(t *testing.T) {
	withPeerValidator(t, func(*config.Tree) error { return errors.New("peer peer1: refused") })

	model := editorOnConfig(t)
	_, err := model.cmdCommitConfirmed(60, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer1")
}

// TestEditorErrorsRefreshesPeerPipelineRefusal drives a forced refusal, the
// errors pipe and a repair through command dispatch in file and session modes.
// The errors view must read the current candidate, preserve warning line
// mapping and leave the pending edit and committed configuration untouched.
func TestEditorErrorsRefreshesPeerPipelineRefusal(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		name := "file"
		if user != "" {
			name = "session"
		}
		t.Run(name, func(t *testing.T) {
			const refusal = "peer peer1: refused by the peer pipeline"
			withPeerValidator(t, func(tree *config.Tree) error {
				peer := tree.GetContainer("bgp").GetList("peer")["peer1"]
				require.NotNil(t, peer)
				description, _ := peer.Get("description")
				if description == "invalid" {
					return errors.New(refusal)
				}
				return nil
			})
			model, store, path := newCommitGrammarModel(t, user)
			run := func(input string) commandResult {
				t.Helper()
				result, err := model.dispatchCommand(input)
				require.NoError(t, err)
				model.applyResult(result)
				return result
			}
			run(peerWithoutRemoteASN)
			require.Len(t, model.validationWarnings, 1)
			warning := model.validationWarnings[0]
			require.Positive(t, warning.Line)
			lines := strings.Split(model.editor.ContentAtPath(nil), "\n")
			require.Contains(t, lines[warning.Line-1], "peer peer1 {",
				"a missing session field is anchored to the hierarchical peer declaration")
			run("set bgp peer peer1 description invalid")
			candidate := model.editor.WorkingContent()
			committed, err := store.ReadFile(path)
			require.NoError(t, err)
			reloaded := false
			model.editor.SetReloadNotifier(func() error {
				reloaded = true
				return nil
			})

			refused := run("commit now force")
			assert.Contains(t, refused.statusMessage, "commit blocked: 1 error(s)")
			assert.Contains(t, refused.statusMessage, "never errors")
			assert.Contains(t, refused.statusMessage, refusal)
			assert.False(t, reloaded)
			assert.True(t, model.editor.Dirty())
			run("show | errors")
			assert.Contains(t, model.ViewportContent(), refusal)
			assert.Contains(t, model.ViewportContent(), warning.Message)
			assert.Contains(t, model.ViewportContent(), "line "+strconv.Itoa(warning.Line)+": "+warning.Message)
			assert.Equal(t, candidate, model.editor.WorkingContent())

			// Change the candidate without applying its result or refreshing
			// debounce hints, as a queued errors command can observe it.
			_, err = model.dispatchCommand("set bgp peer peer1 description repaired")
			require.NoError(t, err)
			repaired := model.editor.WorkingContent()
			run("show | errors")
			assert.NotContains(t, model.ViewportContent(), refusal)
			assert.Contains(t, model.ViewportContent(), warning.Message)
			assert.Contains(t, model.ViewportContent(), "line "+strconv.Itoa(warning.Line)+": "+warning.Message)
			assert.Equal(t, repaired, model.editor.WorkingContent())
			assert.True(t, model.editor.Dirty())
			assert.False(t, reloaded)
			stillCommitted, err := store.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, committed, stillCommitted, "refusal and errors inspection never apply the candidate")
		})
	}
}
