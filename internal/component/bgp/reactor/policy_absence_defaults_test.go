// Design: docs/guide/bgp-policy.md -- internal learned-export policy diagnostics.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// Resolve an eligible claimed role through its actual grant graph. The role
// changes only the internal learned default, never RFC8212's external default.
func TestPolicyAbsencesKeepRoleAndExternalDefaults(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	settings := f.peer.settingsSnapshot()
	settings.PeerAS = settings.LocalAS
	settings.ExportPolicy = filterapi.Policy{}
	ordinary := f.peer.selectedOwnerSnapshot().owner
	role := dryRunRoleOwner(t)
	require.Len(t, PolicyAbsences(settings, ordinary), 1)
	require.Empty(t, PolicyAbsences(settings, role), "an eligible explicit role keeps its internal default")
	require.Empty(t, PolicyAbsences(settings, pluginserver.DisseminationOwner{}), "an unattached internal peer has no ordinary learned output")
	settings.PeerAS++
	absences := PolicyAbsences(settings, role)
	require.NotEmpty(t, absences)
	for _, absence := range absences {
		require.Contains(t, absence.Message, "RFC 8212")
	}
	require.Nil(t, RFC8212Absences(&PeerSettings{LocalAS: 65000, PeerAS: 65000}), "the RFC8212 API remains external-only")
}
