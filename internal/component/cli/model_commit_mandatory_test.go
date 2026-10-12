// Design: docs/guide/config-editor.md -- commit subcommands
// Related: validator.go -- customSectionErrors, the gate under test
package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommitRefusesMissingMandatoryInSection proves the editor's commit applies
// the same section walk LoadConfig applies, so a config the daemon would
// refuse at its next load is refused while the operator still has it on the
// screen.
//
// VALIDATES: a TACACS+ server with no shared key, which LoadConfig
// refuses through config.ValidateCustomSections, is reported by `commit
// verify` as an error naming the leaf and its set command, and `commit
// now` leaves the committed file without it, in file mode and session mode.
// PREVENTS: `commit verify` saying the config is valid and file-mode `commit
// now` writing a config the daemon then refuses to load.
//
// Method: drive the two commit subcommands through dispatchCommand over a
// candidate holding the server, then read the committed file.
func TestCommitRefusesMissingMandatoryInSection(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		name := "file mode"
		if user != "" {
			name = "session mode"
		}
		t.Run(name, func(t *testing.T) {
			model, store, configPath := newCommitGrammarModel(t, user)
			_, err := model.dispatchCommand("set system authentication tacacs server 192.0.2.9 port 49")
			require.NoError(t, err)

			result, err := model.dispatchCommand("commit verify")
			require.NoError(t, err)
			assert.Contains(t, result.statusMessage, "system/authentication/tacacs/server[192.0.2.9]/key")
			assert.Contains(t, result.statusMessage, "`set system authentication tacacs server 192.0.2.9 key <value>`")
			assert.Contains(t, result.statusMessage, "nothing was applied")

			result, err = model.dispatchCommand("commit now")
			require.NoError(t, err, "a blocked commit is reported as status")
			assert.Contains(t, result.statusMessage, "commit blocked")
			committed, err := store.ReadFile(configPath)
			require.NoError(t, err)
			assert.NotContains(t, string(committed), "192.0.2.9", "a refused commit wrote a config the daemon refuses")
		})
	}
}
