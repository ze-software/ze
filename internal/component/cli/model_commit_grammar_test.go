package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// newCommitGrammarModel builds a Model over testValidBGPConfig, in session mode
// when user is not empty and in file mode otherwise.
func newCommitGrammarModel(t *testing.T, user string) (*Model, storage.Storage, string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600))
	store := newTestTreeStore(t, configPath)
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // test cleanup
	if user != "" {
		ed.SetSession(NewEditSession(user, "local"))
	}
	model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	return &model, store, configPath
}

// TestCommitGrammar proves every editor Model takes the commit subcommands and
// refuses the old forms, in file mode and in session mode.
//
// VALIDATES: spec-session-editor-file-mode-parity AC-25, AC-26, AC-27, AC-28:
// plain `commit`, `commit force`, `confirm` and `confirm abort` are refused;
// `commit accept` and `commit abort` with no window say none is pending;
// `commit verify` validates and applies nothing; `commit now` applies; `force`
// outside commit is an ordinary value.
// PREVENTS: the Model keeping its own commit parser beside the shared grammar.
func TestCommitGrammar(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		name := "file mode"
		if user != "" {
			name = "session mode"
		}
		t.Run(name, func(t *testing.T) {
			model, store, configPath := newCommitGrammarModel(t, user)

			_, err := model.dispatchCommand("set bgp router-id 5.6.7.8")
			require.NoError(t, err)

			refused := map[string]string{
				"commit":              "commit now",
				"commit bogus":        "commit verify",
				"commit force":        "modifier",
				"confirm":             "unknown command",
				"confirm abort":       "unknown command",
				"commit accept":       "no confirmed commit is pending",
				"commit abort":        "no confirmed commit is pending",
				"commit verify force": "takes no force",
			}
			for input, want := range refused {
				_, err := model.dispatchCommand(input)
				require.Error(t, err, input)
				assert.Contains(t, err.Error(), want, input)
			}

			result, err := model.dispatchCommand("commit verify")
			require.NoError(t, err)
			assert.Contains(t, result.statusMessage, "valid")
			committed, err := store.ReadFile(configPath)
			require.NoError(t, err)
			assert.Contains(t, string(committed), "1.2.3.4", "commit verify applied the candidate")

			_, err = model.dispatchCommand("set bgp router-id force")
			require.Error(t, err, "router-id is an address, so force is refused as a value, not as a keyword")
			assert.NotContains(t, err.Error(), "modifier")

			_, err = model.dispatchCommand("commit now")
			require.NoError(t, err)
			committed, err = store.ReadFile(configPath)
			require.NoError(t, err)
			assert.Contains(t, string(committed), "5.6.7.8", "commit now did not apply the candidate")
		})
	}
}

// TestCommitNowRefusedDuringWindow proves a pending file-mode window refuses
// `commit now` with or without force and names the ways out.
//
// VALIDATES: AC-18 (a) in file mode, and AC-23's plain nested `commit confirmed`.
// PREVENTS: a commit landing inside a window and being reverted with it.
func TestCommitNowRefusedDuringWindow(t *testing.T) {
	model, store, configPath := newCommitGrammarModel(t, "")
	_, err := model.dispatchCommand("set bgp router-id 5.6.7.8")
	require.NoError(t, err)
	model.confirmTimerActive = true

	for _, input := range []string{"commit now", "commit now force", "commit confirmed 30"} {
		_, err := model.dispatchCommand(input)
		require.Error(t, err, input)
		for _, want := range []string{"pending", "commit accept", "commit abort", "commit confirmed <seconds> force"} {
			assert.Contains(t, err.Error(), want, input)
		}
	}
	committed, err := store.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(committed), "1.2.3.4")
}

// TestCommitCompletion proves completion offers the commit grammar from the
// shared table: the five subcommands after `commit`, and `force` only after
// `commit now` and `commit confirmed <seconds>`.
//
// VALIDATES: AC-27: completion offers the five subcommands and never `confirm`.
// PREVENTS: a completion list that disagrees with the parser.
func TestCommitCompletion(t *testing.T) {
	completer := newTestCompleter(t)
	texts := func(input string) []string {
		var out []string
		for _, c := range completer.Complete(input, nil) {
			out = append(out, c.Text)
		}
		return out
	}

	assert.Equal(t, []string{"now", "confirmed", "accept", "abort", "verify"}, texts("commit "))
	assert.Equal(t, []string{"accept", "abort"}, texts("commit a"))
	assert.Equal(t, []string{"force"}, texts("commit now "))
	assert.Equal(t, []string{"force"}, texts("commit confirmed 60 "))
	assert.Empty(t, texts("commit confirmed "), "the seconds are a value, not a keyword to complete")
	assert.Empty(t, texts("commit accept "))
	assert.Empty(t, texts("commit now force "))
	assert.NotContains(t, texts("conf"), "confirm")
}

// TestFileModeNestedCommitConfirmedForce proves file mode's own window takes
// `commit confirmed <seconds> force` from its owner: the new changes apply, the
// countdown restarts at the new seconds, and an abort still restores the
// config from before the FIRST commit of the window.
//
// VALIDATES: AC-23 in file mode (`ze config edit -f`).
// PREVENTS: file mode refusing the nested force the SSH editor takes, and a
// nested commit whose revert restores only the first commit's result.
func TestFileModeNestedCommitConfirmedForce(t *testing.T) {
	model, store, configPath := newCommitGrammarModel(t, "")
	committedHas := func(want, lacks string) {
		t.Helper()
		committed, err := store.ReadFile(configPath)
		require.NoError(t, err)
		if want != "" {
			assert.Contains(t, string(committed), want)
		}
		if lacks != "" {
			assert.NotContains(t, string(committed), lacks)
		}
	}

	_, err := model.dispatchCommand("set bgp router-id 5.6.7.8")
	require.NoError(t, err)
	result, err := model.dispatchCommand("commit confirmed 60")
	require.NoError(t, err)
	model.applyResult(result)
	require.True(t, model.confirmTimerActive)
	firstBackup := model.confirmBackupPath
	require.NotEmpty(t, firstBackup)
	committedHas("5.6.7.8", "")

	_, err = model.dispatchCommand("set bgp router-id 9.9.9.9")
	require.NoError(t, err)
	result, err = model.dispatchCommand("commit confirmed 30 force")
	require.NoError(t, err, "the owner's nested force is taken")
	assert.Contains(t, result.statusMessage, "Confirm within 30s")
	model.applyResult(result)
	committedHas("9.9.9.9", "")
	assert.True(t, model.confirmTimerActive)
	assert.Equal(t, 30, model.confirmSecondsLeft, "the countdown restarted at the new seconds")
	assert.Equal(t, firstBackup, model.confirmBackupPath, "the revert target stays the first commit's")

	result, err = model.dispatchCommand("commit abort")
	require.NoError(t, err)
	model.applyResult(result)
	committedHas("1.2.3.4", "5.6.7.8")
	committedHas("", "9.9.9.9")
	assert.False(t, model.confirmTimerActive)
}

// TestFileModeNestedCountdownKeepsOneTicker proves a nested restart resets
// the running countdown instead of starting a second ticker beside it, which
// would count the window down twice a second.
//
// VALIDATES: AC-23 in file mode: the countdown restarts at <seconds>.
// PREVENTS: two tick chains halving the window the operator asked for.
func TestFileModeNestedCountdownKeepsOneTicker(t *testing.T) {
	model, _, _ := newCommitGrammarModel(t, "")
	model.confirmTimerActive = true
	model.confirmSecondsLeft = 12
	updated, cmd := model.handleCommandResult(commandResultMsg{result: commandResult{
		setConfirmTimer: true, confirmTimerValue: true, confirmBackupPath: "kept", startConfirmCountdown: 30,
	}})
	m, ok := updated.(Model)
	require.True(t, ok)
	assert.Equal(t, 30, m.confirmSecondsLeft)
	assert.Nil(t, cmd, "a running countdown gets no second ticker")
}
