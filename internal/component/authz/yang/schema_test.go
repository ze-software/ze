package yang_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

func TestSchema_ZeAuthzOwnsSharedAuthenticationUsers(t *testing.T) {
	loader := configyang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	module := loader.GetModule("ze-authz-conf")
	require.NotNil(t, module, "ze-authz-conf module should exist in every composition")
	assert.Equal(t, "urn:ze:authz:conf", module.Namespace.Name)

	entry := loader.GetEntry("ze-authz-conf")
	require.NotNil(t, entry)
	system := entry.Dir["system"]
	require.NotNil(t, system, "authz must own the shared system container")
	authentication := system.Dir["authentication"]
	require.NotNil(t, authentication, "authz must own shared authentication data")
	user := authentication.Dir["user"]
	require.NotNil(t, user, "authz must own the shared user list")
	assert.Equal(t, "name", user.Key)

	for _, field := range []string{"name", "password", "plaintext-password", "profile"} {
		assert.NotNil(t, user.Dir[field], "authz user must own shared field %q", field)
	}
	assert.Nil(t, user.Dir["public-keys"], "SSH public keys belong only to the ze_ssh augment")
}
