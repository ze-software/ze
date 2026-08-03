// Design: ai/rules/plugins.md -- ze_grpc present build validation
//
//go:build ze_grpc

package hub

// VALIDATES: with the ze_grpc build tag (the default ze / ze-appliance feature
// set), the gRPC build seam is installed.
// PREVENTS: a regression where ze_grpc is set but gRPC is not wired (e.g. the
// register_grpc.go init() is dropped or the tag stops reaching the seam).

import "testing"

func TestBuildTag_GRPC_Present(t *testing.T) {
	if grpcBuild == nil {
		t.Fatal("ze_grpc build: gRPC build seam not installed (grpcBuild is nil)")
	}
}
