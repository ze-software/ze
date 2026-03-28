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

	// SSH module defines both system (authentication) and environment (ssh) containers.
	var hasSystem, hasEnvironment bool
	for _, c := range mod.Container {
		if c.Name == "system" {
			hasSystem = true
		}
		if c.Name == "environment" {
			hasEnvironment = true
		}
	}
	assert.True(t, hasSystem, "system container should exist (for authentication)")
	assert.True(t, hasEnvironment, "environment container should exist (for ssh)")
}

// TestSchema_ZeSSHEntry verifies the YANG entry has expected children.
//
// VALIDATES: AC-1 -- config file with ssh block parsed, all fields accessible.
// PREVENTS: Missing fields in SSH YANG schema.
func TestSchema_ZeSSHEntry(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	entry := loader.GetEntry("ze-ssh-conf")
	require.NotNil(t, entry, "ze-ssh-conf entry should exist")

	// SSH settings live under environment.ssh.
	environment := entry.Dir["environment"]
	require.NotNil(t, environment, "environment container should exist in entry")

	ssh := environment.Dir["ssh"]
	require.NotNil(t, ssh, "ssh container should exist inside environment")

	expectedLeaves := []string{"listen", "host-key", "idle-timeout", "max-sessions"}
	for _, name := range expectedLeaves {
		assert.NotNil(t, ssh.Dir[name], "ssh should have leaf %q", name)
	}

	// Authentication stays under system.
	system := entry.Dir["system"]
	require.NotNil(t, system, "system container should exist in entry")

	auth := system.Dir["authentication"]
	require.NotNil(t, auth, "authentication container should exist under system")

	user := auth.Dir["user"]
	require.NotNil(t, user, "user list should exist in authentication")
	assert.Equal(t, "name", user.Key, "user list key should be 'name'")
}
