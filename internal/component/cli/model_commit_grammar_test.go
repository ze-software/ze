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
