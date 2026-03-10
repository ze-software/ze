package schema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// TestSchema_ZeSSHModule verifies ze-ssh-conf.yang content.
//
// VALIDATES: AC-6 — ze-ssh-conf module loaded and has expected structure.
// PREVENTS: Missing SSH configuration elements in YANG schema.
func TestSchema_ZeSSHModule(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	mod := loader.GetModule("ze-ssh-conf")
	require.NotNil(t, mod, "ze-ssh-conf module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:ssh:conf", mod.Namespace.Name)

	// Find system container (ssh is nested inside)
	var systemContainer bool
	for _, c := range mod.Container {
		if c.Name == "system" {
			systemContainer = true
			break
		}
	}
	assert.True(t, systemContainer, "system container should exist")
}

// TestSchema_ZeSSHEntry verifies the YANG entry has expected children.
//
// VALIDATES: AC-1 — config file with ssh block parsed, all fields accessible.
// PREVENTS: Missing fields in SSH YANG schema.
func TestSchema_ZeSSHEntry(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	entry := loader.GetEntry("ze-ssh-conf")
	require.NotNil(t, entry, "ze-ssh-conf entry should exist")

	system := entry.Dir["system"]
	require.NotNil(t, system, "system container should exist in entry")

	ssh := system.Dir["ssh"]
	require.NotNil(t, ssh, "ssh container should exist inside system")

	// Check expected SSH leaves
	expectedLeaves := []string{"listen", "host-key", "idle-timeout", "max-sessions"}
	for _, name := range expectedLeaves {
		assert.NotNil(t, ssh.Dir[name], "ssh should have leaf %q", name)
	}

	// Check authentication container (at system level, not under ssh)
	auth := system.Dir["authentication"]
	require.NotNil(t, auth, "authentication container should exist under system")

	// Check user list inside authentication
	user := auth.Dir["user"]
	require.NotNil(t, user, "user list should exist in authentication")
	assert.Equal(t, "name", user.Key, "user list key should be 'name'")
}
