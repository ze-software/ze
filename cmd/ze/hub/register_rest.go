// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
// Related: service_rest.go -- the restBuildImpl this installs
//
// Build-tag-gated installation of the REST API build hook. Compiled only under
// //go:build ze_rest; absent the tag this init() does not exist, so restBuild
// stays nil and the hub builds no REST server.

//go:build ze_rest

package hub

func init() {
	setRESTInfra(restBuildImpl)
}
