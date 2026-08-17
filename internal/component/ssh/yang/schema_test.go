package yang_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"

	_ "github.com/ze-software/ze/internal/component/authz/yang"
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

	// The gated module owns transport configuration, not the shared system user.
	var hasSystem, hasEnvironment bool
	for _, c := range mod.Container {
		switch c.Name {
		case "system":
			hasSystem = true
		case "environment":
			hasEnvironment = true
		}
	}
	assert.False(t, hasSystem, "SSH module must not own the shared system container")
	assert.True(t, hasEnvironment, "SSH module must own environment.ssh")
}

// TestSchema_ZeSSHEntry verifies the SSH transport entry remains intact.
//
// VALIDATES: current environment.ssh config syntax is preserved.
// PREVENTS: the user ownership move accidentally dropping SSH listener fields.
func TestSchema_ZeSSHEntry(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	entry := loader.GetEntry("ze-ssh-conf")
	require.NotNil(t, entry, "ze-ssh-conf entry should exist")

	environment := entry.Dir["environment"]
	require.NotNil(t, environment, "environment container should exist in entry")
	ssh := environment.Dir["ssh"]
	require.NotNil(t, ssh, "ssh container should exist inside environment")

	expectedChildren := []string{"enabled", "server", "host-key", "host-certificate", "idle-timeout", "max-sessions"}
	for _, name := range expectedChildren {
		assert.NotNil(t, ssh.Dir[name], "ssh should have child %q", name)
	}
}

func TestSchema_ZeSSHOwnsPublicKeyAugmentOnly(t *testing.T) {
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	module := loader.GetModule("ze-ssh-conf")
	require.NotNil(t, module)

	var publicKeyAugmentFound bool
	for _, augment := range module.Augment {
		if augment.Name != "/authz:system/authz:authentication/authz:user" {
			continue
		}
		for _, list := range augment.List {
			if list.Name != "public-keys" {
				continue
			}
			publicKeyAugmentFound = true
			assert.Equal(t, "name", list.Key.Name, "public-keys list key should stay name")
			var leaves []string
			for _, leaf := range list.Leaf {
				leaves = append(leaves, leaf.Name)
			}
			assert.ElementsMatch(t, []string{"name", "type", "key"}, leaves)
		}
	}
	assert.True(t, publicKeyAugmentFound,
		"ze_ssh must augment the authz-owned user with public-keys")
}
