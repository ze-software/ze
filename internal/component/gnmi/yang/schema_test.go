package yang_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"

	_ "github.com/ze-software/ze/internal/component/cmd/show/yang"
	_ "github.com/ze-software/ze/internal/component/gnmi/yang"
)

func TestGNMISchemaRegistered(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	mod := loader.GetModule("ze-gnmi-conf")
	require.NotNil(t, mod, "ze-gnmi-conf module should exist")

	assert.Equal(t, "urn:ze:gnmi:conf", mod.Namespace.Name)

	var envContainer bool
	for _, c := range mod.Container {
		if c.Name != "environment" {
			continue
		}

		envContainer = true

		var gnmiChild bool
		for _, child := range c.Container {
			if child.Name != "gnmi" {
				continue
			}

			gnmiChild = true

			var hasEnabled, hasToken, hasTLS, hasServer bool
			for _, leaf := range child.Leaf {
				switch leaf.Name {
				case "enabled":
					hasEnabled = true
				case "token":
					hasToken = true
				}
			}
			for _, sub := range child.Container {
				if sub.Name == "tls" {
					hasTLS = true
				}
			}
			for _, lst := range child.List {
				if lst.Name == "server" {
					hasServer = true
				}
			}

			assert.True(t, hasEnabled, "gnmi.enabled leaf should exist")
			assert.True(t, hasToken, "gnmi.token leaf should exist")
			assert.True(t, hasTLS, "gnmi.tls container should exist")
			assert.True(t, hasServer, "gnmi.server list should exist")
			break
		}

		assert.True(t, gnmiChild, "environment.gnmi container should exist")
		break
	}
	assert.True(t, envContainer, "environment container should exist")
}

func TestGNMISchemaListenerPattern(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	entry := loader.GetEntry("ze-gnmi-conf")
	require.NotNil(t, entry)

	env := entry.Dir["environment"]
	require.NotNil(t, env, "environment entry should exist")

	gnmi := env.Dir["gnmi"]
	require.NotNil(t, gnmi, "gnmi entry should exist")

	server := gnmi.Dir["server"]
	require.NotNil(t, server, "server list entry should exist")

	assert.Contains(t, server.Dir, "ip", "server should have ip leaf from zt:listener")
	assert.Contains(t, server.Dir, "port", "server should have port leaf from zt:listener")
}

func TestGNMISchemaTokenSensitive(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	entry := loader.GetEntry("ze-gnmi-conf")
	require.NotNil(t, entry)

	env := entry.Dir["environment"]
	require.NotNil(t, env)

	gnmi := env.Dir["gnmi"]
	require.NotNil(t, gnmi)

	token := gnmi.Dir["token"]
	require.NotNil(t, token, "token leaf should exist")

	var hasSensitive bool
	for _, ext := range token.Exts {
		if ext.Keyword == "ze:sensitive" {
			hasSensitive = true
			break
		}
	}
	assert.True(t, hasSensitive, "token leaf should have ze:sensitive extension")
}
