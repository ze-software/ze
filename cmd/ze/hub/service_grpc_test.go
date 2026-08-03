// Design: ai/rules/plugins.md -- ze_grpc-gated gRPC seam test
//
//go:build ze_grpc

package hub

// VALIDATES: grpcBuildImpl treats a config with gRPC disabled as a clean skip
// (zero handle, no error) without touching the shared engine/sessions. The full
// gRPC bind path is covered by the test/parse api-grpc-*.ci functional tests
// under the ze_grpc build.
// PREVENTS: a regression where the gRPC seam errors or builds a server when gRPC
// is not enabled in config.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestGRPCBuildNotEnabled(t *testing.T) {
	// GRPCOn defaults false: the builder returns a zero handle before using the
	// shared state, so an empty apiShared is sufficient.
	h, err := grpcBuildImpl(&apiBuildInputs{Config: zeconfig.APIConfig{}}, &apiShared{})
	require.NoError(t, err)
	assert.Nil(t, h.Server, "gRPC disabled must yield a zero handle (nil Server)")
	assert.Nil(t, h.Shutdown, "gRPC disabled must yield no shutdown")
}
