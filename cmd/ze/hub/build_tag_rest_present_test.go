// Design: ai/rules/plugins.md -- ze_rest present build validation
//
//go:build ze_rest

package hub

// VALIDATES: with the ze_rest build tag (the default ze / ze-appliance feature
// set), the REST build seam is installed.
// PREVENTS: a regression where ze_rest is set but REST is not wired (e.g. the
// register_rest.go init() is dropped or the tag stops reaching the seam).

import "testing"

func TestBuildTag_REST_Present(t *testing.T) {
	if restBuild == nil {
		t.Fatal("ze_rest build: REST build seam not installed (restBuild is nil)")
	}
}
