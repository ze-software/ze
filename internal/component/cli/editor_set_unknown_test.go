package cli

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetValueRefusesUnknownLeaf proves SetValue refuses a name the schema
// does not hold, in session mode and without a session.
//
// GOAL: every surface that calls SetValue (web, REST, gRPC, gNMI, the SSH
// editor, load) gets the same refusal for a leaf or path the schema lacks.
// METHOD: set an unknown leaf, a misspelled leaf and a leaf under an unknown
// container, then read the error, the change file and the working tree.
//
// VALIDATES: the error names the path and the name, says why, and gives the
// command that fixes it; nothing is written.
// PREVENTS: SetValue writing `set bgp listen 0.0.0.0` to the change file, which
// the editor then cannot parse, so every later edit, commit and partial
// discard of that user refuses.
func TestSetValueRefusesUnknownLeaf(t *testing.T) {
	cases := []struct {
		name     string
		path     []string
		key      string
		contains []string
	}{
		{"unknown leaf", []string{"bgp"}, "listen", []string{"bgp has no setting named listen", "schema does not define it"}},
		{"misspelled leaf", []string{"bgp"}, "router-ix", []string{"bgp has no setting named router-ix", "set bgp router-id <value>"}},
		{"unknown container", []string{"bgp", "no-such-block"}, "x", []string{"bgp has no setting named no-such-block"}},
	}
	for _, session := range []bool{true, false} {
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				configPath := writeTestConfig(t, validBGPConfig)
				store := newTestTreeStore(t, configPath)
				ed, err := NewEditorWithStorage(store, configPath)
				require.NoError(t, err)
				t.Cleanup(func() { _ = ed.Close() })
				if session {
					ed.SetSession(NewEditSession("thomas", "ssh"))
				}

				err = ed.SetValue(c.path, c.key, "0.0.0.0")
				require.Error(t, err)
				for _, want := range c.contains {
					assert.Contains(t, err.Error(), want)
				}
				_, statErr := store.Stat(ChangePath(configPath, "thomas"))
				assert.ErrorIs(t, statErr, fs.ErrNotExist, "no change file is written")
				assert.NotContains(t, ed.WorkingContent(), c.key+" 0.0.0.0", "the working tree is unchanged")
			})
		}
	}
}
