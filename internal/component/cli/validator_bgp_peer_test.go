// Design: docs/architecture/config/yang-config-design.md -- config editor validation
package cli

import (
	"errors"
	"os"
	"path/filepath"
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
func editorOnConfig(t *testing.T, content string) Model {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	ed, err := NewEditor(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() }) //nolint:errcheck // test cleanup
	ed.MarkDirty()

	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	return model
}

// TestEditorCommitBlocksOnMissingLeakFilter drives `commit` over a config the
// BGP peer pipeline refuses.
//
// VALIDATES: AC-39. The refusal reads as a blocked commit naming the peer,
// while the operator still has the config on the screen, rather than as a
// `commit failed:` line about a file already written.
// PREVENTS: the editor accepting every peer-pipeline refusal and discovering it
// one step later at reload. That gap hid EVERY such refusal, not only this
// spec's rule.
func TestEditorCommitBlocksOnMissingLeakFilter(t *testing.T) {
	refusal := "peer peer1: role peer requires a filter against a transit leak in the import and export filter chain"
	withPeerValidator(t, func(*config.Tree) error { return errors.New(refusal) })

	model := editorOnConfig(t, testValidBGPConfigWithPeer)
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

	model := editorOnConfig(t, testValidBGPConfigWithPeer)
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

	validator, err := newConfigValidator()
	require.NoError(t, err)

	assert.Empty(t, validator.bgpPeerErrors(nil), "a config that did not parse has no peer to validate")
	assert.Empty(t, validator.bgpPeerErrors(config.NewTree()), "no bgp block, no peer to validate")
	assert.False(t, asked, "the seam is not consulted in either case")
}

// TestEditorCommitConfirmedBlocksOnMissingLeakFilter drives the OTHER commit
// command over the same refused config.
//
// VALIDATES: AC-39 on the safe-commit path. `commit confirmed <N>` writes the
// trial config to .conf and starts a rollback timer, so a config the daemon
// refuses would be written and then fail at reload, which is the failure AC-39
// exists to prevent.
// PREVENTS: the guard covering one commit command and not its sibling.
// cmdCommitConfirmed validated with Validate, which never reaches the BGP peer
// pipeline; cmdCommit validated with ValidateTransition, which does. Found by
// the closure review, fixed by making both commit commands take one path.
func TestEditorCommitConfirmedBlocksOnMissingLeakFilter(t *testing.T) {
	refusal := "peer peer1: role peer requires a filter against a transit leak in the import and export filter chains"
	withPeerValidator(t, func(*config.Tree) error { return errors.New(refusal) })

	model := editorOnConfig(t, testValidBGPConfigWithPeer)
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

	model := editorOnConfig(t, testValidBGPConfigWithPeer)
	_, err := model.cmdCommitConfirmed(60, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer1")
}
