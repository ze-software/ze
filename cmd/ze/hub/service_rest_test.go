// Design: ai/rules/plugins.md -- ze_rest-gated REST seam test
//
//go:build ze_rest

package hub

// VALIDATES: restBuildImpl treats a config with REST disabled as a clean skip
// (zero handle, no error) without touching the shared engine/sessions. The full
// REST bind path is covered by the test/parse api-rest-*.ci functional tests
// under the ze_rest build.
// PREVENTS: a regression where the REST seam errors or builds a server when REST
// is not enabled in config.
//
// test-relax: this file replaces the combined-ze_api TestAPISeamBuildsBoth (the
// combined ze_api seam was reworked into per-transport ze_rest/ze_grpc seams at
// user request). The combined test's coverage is replaced and broadened by
// TestRESTBuildNotEnabled (here) + TestGRPCBuildNotEnabled (service_grpc_test.go)
// + the REST/gRPC present/absent build-tag tests + the api-rest/api-grpc .ci
// functional tests, not removed.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestRESTBuildNotEnabled(t *testing.T) {
	// RESTOn defaults false: the builder returns a zero handle before using the
	// shared state, so an empty apiShared is sufficient.
	h, err := restBuildImpl(&apiBuildInputs{Config: zeconfig.APIConfig{}}, &apiShared{})
	require.NoError(t, err)
	assert.Nil(t, h.Server, "REST disabled must yield a zero handle (nil Server)")
	assert.Nil(t, h.Shutdown, "REST disabled must yield no shutdown")
}
