package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "codeberg.org/thomas-mangin/ze/internal/component/api/schema"
)

// VALIDATES: ze-api-conf YANG module loads into environment.api-server.
// PREVENTS: API config rejected as unknown field.
func TestAPISchemaInEnvironment(t *testing.T) {
	s, err := YANGSchema()
	require.NoError(t, err)
	env := s.Get("environment")
	require.NotNil(t, env)
	cn, ok := env.(*ContainerNode)
	require.True(t, ok)
	api := cn.Get("api-server")
	require.NotNil(t, api, "environment.api-server missing from schema")
	apiCN, ok := api.(*ContainerNode)
	require.True(t, ok)
	assert.Contains(t, apiCN.Children(), "rest")
	assert.Contains(t, apiCN.Children(), "grpc")
}
