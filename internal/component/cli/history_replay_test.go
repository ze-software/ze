// Design: docs/guide/config-editor.md -- masked recall is never a replacement credential.
package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestHistoryMaskedSecretReplayCannotCommit exercises the same recalled line
// through both commit producers, retaining the actual configured credential.
func TestHistoryMaskedSecretReplayCannotCommit(t *testing.T) {
	const module = `module ze-history-replay-conf {
 yang-version 1.1;
 namespace "urn:ze:history-replay";
 prefix probe;
 import ze-extensions { prefix ze; }
 container history-replay {
  leaf material { type string; ze:sensitive; }
  leaf public { type string; }
 }
}`
	t.Cleanup(configyang.RegisterModuleForTest("ze-history-replay-conf.yang", module))
	for _, candidate := range []bool{false, true} {
		name := "direct"
		if candidate {
			name = "candidate"
		}
		t.Run(name, func(t *testing.T) {
			ed, _, path := newLoadSessionEditor(t, validBGPConfig+"\nhistory-replay { material actual-secret; public ordinary; }\n")
			before, err := ed.store.ReadFile(path)
			require.NoError(t, err)
			h := NewHistory(ed.store, "parity")
			t.Cleanup(h.Close)
			recalled, err := h.Record(ModeConfig, []string{"history-replay"}, "set material replacement-secret")
			require.NoError(t, err)
			require.Len(t, recalled, 1)
			assert.NotContains(t, recalled[0], "replacement-secret")
			_, err = ed.ExecuteConfigCommand(contract.ConfigCommandRequest{Line: recalled[0]})
			require.NoError(t, err, "the commit guard owns refusal of a masked candidate")
			if candidate {
				_, _, err = ed.CommitSessionCandidate(time.Now())
			} else {
				_, err = ed.CommitSession()
			}
			require.Error(t, err, "a recalled display placeholder must never become a secret")
			assert.Contains(t, err.Error(), "material")
			assert.Contains(t, err.Error(), "real value")
			after, readErr := ed.store.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, before, after)
		})
	}
}
