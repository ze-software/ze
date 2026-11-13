// Design: docs/architecture/cli/error-surface.md -- a complete correction masks sensitive operands.
// Related: model_commands_edit.go -- cmdSet's early list-address refusal

package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestModelNonFirstKeyRefusalMasksFollowingSecret enters the early address
// refusal with a later credential. The exact printed correction must retain
// the full command while masking the credential, then replay onto the same
// composite identity when the operator fills only its secret placeholder.
func TestModelNonFirstKeyRefusalMasksFollowingSecret(t *testing.T) {
	const secret = "spaced secret"
	for _, user := range []string{"", "alice"} {
		mode := "file"
		if user != "" {
			mode = "session"
		}
		for _, contextual := range []bool{false, true} {
			position := "root"
			if contextual {
				position = "context"
			}
			for _, tc := range []struct {
				name    string
				address string
				path    []string
			}{
				{"second key", "session 192.0.2.1 vrf blu interface eth0", []string{"multikey-probe", "session", "192.0.2.1", "vrf"}},
				{"third key", "session 192.0.2.1 vrf blue interface eth0x", []string{"multikey-probe", "session", "192.0.2.1", "vrf", "blue", "interface"}},
			} {
				t.Run(mode+"/"+position+"/"+tc.name, func(t *testing.T) {
					m := multikeyRefusalEntryModel(t, user)
					_, err := m.dispatchCommand("set multikey-probe session 192.0.2.9 vrf red interface eth9 label untouched")
					require.NoError(t, err)
					prefix := "multikey-probe "
					if contextual {
						m.contextPath = []string{"multikey-probe"}
						prefix = ""
					}
					before := m.editor.WorkingContent()
					var pending []config.PendingChange
					if user != "" {
						pending = m.editor.PendingChanges(m.editor.session.ID)
					}
					_, err = m.dispatchCommand("set " + prefix + tc.address + " password " + configtext.QuoteCommandWord(secret))
					require.Error(t, err)
					var refusal *configyang.ValidationError
					require.True(t, errors.As(err, &refusal), "typed key refusal: %v", err)
					assert.Equal(t, tc.path, refusal.Segments())
					var operator *opererr.Error
					require.True(t, errors.As(err, &operator), "operator refusal: %v", err)
					for _, displayed := range []string{err.Error(), operator.Problem(), operator.Why(), operator.Fix()} {
						assert.NotContains(t, displayed, secret, "the refusal must not expose the sensitive suffix")
					}
					fix := lastLine(err.Error())
					require.Equal(t, "fix: set "+prefix+"session 192.0.2.1 vrf blue interface eth0 password /* SECRET-DATA */", fix)
					assert.Equal(t, before, m.editor.WorkingContent(), "a refused secret-bearing command changed the candidate")
					if user != "" {
						assert.Equal(t, pending, m.editor.PendingChanges(m.editor.session.ID), "refusal changed pending operations")
					}
					printed := strings.TrimPrefix(fix, "fix: ")
					require.Equal(t, 1, strings.Count(printed, "/* SECRET-DATA */"))
					_, err = m.dispatchCommand(strings.Replace(printed, "/* SECRET-DATA */", configtext.QuoteCommandWord(secret), 1))
					require.NoError(t, err, "replay the actual correction with only its secret placeholder filled")
					entries := m.editor.tree.GetContainer("multikey-probe").GetList("session")
					require.Len(t, entries, 2)
					entry := entries["192.0.2.1 vrf blue interface eth0"]
					require.NotNil(t, entry, "the replay changed the corrected composite identity")
					stored, ok := entry.Get("password")
					assert.True(t, ok)
					assert.Equal(t, secret, stored)
					control := entries["192.0.2.9 vrf red interface eth9"]
					require.NotNil(t, control)
					label, ok := control.Get("label")
					assert.True(t, ok)
					assert.Equal(t, "untouched", label)
				})
			}
		}
	}
}
