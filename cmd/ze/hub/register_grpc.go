// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
// Related: service_grpc.go -- the grpcBuildImpl this installs
//
// Build-tag-gated installation of the gRPC API build hook. Compiled only under
// //go:build ze_grpc; absent the tag this init() does not exist, so grpcBuild
// stays nil and the hub builds no gRPC server.

//go:build ze_grpc

package hub

func init() {
	setGRPCInfra(grpcBuildImpl)
}
