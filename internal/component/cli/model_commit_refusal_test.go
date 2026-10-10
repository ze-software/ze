// Design: docs/guide/config-editor.md -- commit subcommands and the force modifier
// Related: model_commands_commit.go -- commitValidationRefusal
package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
)

// peerWithoutRemoteASN is a peer whose missing `session asn remote` validation
// reports as a warning, not an error.
const peerWithoutRemoteASN = "set bgp peer peer1 connection remote ip 1.1.1.1"

// TestCommitRefusalListsIssuesAndNamesForce drives `commit now`, `commit
// verify` and `commit now force` over a warning-only candidate and then over one
// with an error, in file mode and in session mode.
//
// VALIDATES: AC-12 -- a refused `commit now` lists the warnings and names
// `commit now force`; `force` commits over the warnings and says how many it
// skipped; an error refuses `force` too and names the error. AC-26 -- `commit
// verify` lists the issues `commit now` reports and applies nothing.
// PREVENTS: a refusal that only counts the issues ("1 issue(s)") and never says
// what they are or that `force` exists.
func TestCommitRefusalListsIssuesAndNamesForce(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		name := "file mode"
		if user != "" {
			name = "session mode"
		}
		t.Run(name, func(t *testing.T) {
			model, store, configPath := newCommitGrammarModel(t, user)
			_, err := model.dispatchCommand(peerWithoutRemoteASN)
			require.NoError(t, err)
			validation := model.validator.ValidateTransition(model.editor.OriginalContent(), model.editor.WorkingContent())
			require.Empty(t, validation.Errors, "the candidate must raise warnings only")
			require.NotEmpty(t, validation.Warnings, "the candidate must raise a warning")
			warning := validation.Warnings[0].Message

			result, err := model.dispatchCommand("commit now")
			require.NoError(t, err, "a blocked commit is reported as status")
			assert.Contains(t, result.statusMessage, "commit blocked: 0 error(s), 1 warning(s)")
			assert.Contains(t, result.statusMessage, "'commit now force'")
			assert.Contains(t, result.statusMessage, warning, "the refusal lists the warning")
			assert.NotNil(t, result.configView, "the config stays in the viewport with its markers")

			result, err = model.dispatchCommand("commit verify")
			require.NoError(t, err)
			assert.Contains(t, result.statusMessage, "0 error(s), 1 warning(s)")
			assert.Contains(t, result.statusMessage, "nothing was applied")
			assert.Contains(t, result.statusMessage, warning, "verify lists what commit now reports")

			infra.SetBGPPeerValidator(func(*config.Tree) error { return errors.New("peer peer1: refused by the test") })
			t.Cleanup(func() { infra.SetBGPPeerValidator(nil) })

			result, err = model.dispatchCommand("commit now force")
			require.NoError(t, err)
			assert.Contains(t, result.statusMessage, "commit blocked: 1 error(s)")
			assert.Contains(t, result.statusMessage, "never errors")
			assert.Contains(t, result.statusMessage, "refused by the test", "the refusal names the error")
			assert.NotContains(t, result.statusMessage, warning, "force does not list the warnings it skips")

			result, err = model.dispatchCommand("commit verify")
			require.NoError(t, err)
			assert.Contains(t, result.statusMessage, "1 error(s), 1 warning(s)")
			assert.Contains(t, result.statusMessage, "refused by the test")
			assert.Contains(t, result.statusMessage, warning)

			committed, err := store.ReadFile(configPath)
			require.NoError(t, err)
			assert.NotContains(t, string(committed), "peer1", "a refused commit applied the candidate")

			infra.SetBGPPeerValidator(nil)
			result, err = model.dispatchCommand("commit now force")
			require.NoError(t, err)
			assert.NotContains(t, result.statusMessage, "commit blocked")
			assert.Contains(t, result.statusMessage, "skipping 1 warning(s)", "force says how many warnings it skipped")
			committed, err = store.ReadFile(configPath)
			require.NoError(t, err)
			assert.Contains(t, string(committed), "peer1", "commit now force did not apply the candidate")
		})
	}
}

// TestFileModeCommitConfirmedRefusalNamesForce proves the file-mode `commit
// confirmed <seconds>` refusal over warnings names its own forced form.
//
// VALIDATES: AC-12 for `commit confirmed 60 force` in file mode.
// PREVENTS: the confirmed refusal pointing the operator at `commit now force`,
// which commits with no window.
func TestFileModeCommitConfirmedRefusalNamesForce(t *testing.T) {
	model, _, _ := newCommitGrammarModel(t, "")
	_, err := model.dispatchCommand(peerWithoutRemoteASN)
	require.NoError(t, err)

	_, err = model.dispatchCommand("commit confirmed 60")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'commit confirmed 60 force'")
}

// TestSessionWindowCommitRefusalNamesForce proves a session commit through the
// daemon's window names the forced form of the command the operator typed.
//
// VALIDATES: AC-12 over SSH, where the daemon owns the window.
// PREVENTS: a `commit confirmed` refusal naming `commit now force`.
func TestSessionWindowCommitRefusalNamesForce(t *testing.T) {
	f := newWindowFixture(t)
	alice := f.model(t, "alice")
	// The fixture config already holds peer1, so the warning comes from a second peer.
	_, err := alice.dispatchCommand("set bgp peer peer2 connection remote ip 2.2.2.2")
	require.NoError(t, err)
	validation := alice.validator.ValidateTransition(alice.editor.OriginalContent(), alice.editor.WorkingContent())
	require.Empty(t, validation.Errors, "the candidate must raise warnings only")
	require.NotEmpty(t, validation.Warnings, "the candidate must raise a warning")

	result, err := alice.cmdCommitRequest(contract.CommitRequest{Action: contract.CommitNow})
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "'commit now force'")

	result, err = alice.cmdCommitRequest(confirmedRequest(60, false))
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "'commit confirmed 60 force'")
	_, open := f.window.Status()
	assert.False(t, open, "a refused commit opens no window")

	result, err = alice.cmdCommitRequest(confirmedRequest(60, true))
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "commit confirmed 60 force: skipping 1 warning(s)")
	assert.Contains(t, result.statusMessage, "Confirm within 60s")
	_, open = f.window.Status()
	assert.True(t, open, "a forced commit over warnings opens the window")
}
