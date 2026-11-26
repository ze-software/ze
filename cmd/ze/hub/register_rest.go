// Design: ai/rules/plugins.md -- compile-out-able services (feature-gate)
// Related: service_rest.go -- the restBuildImpl this installs
//
// Build-tag-gated installation of the REST API build hook. Compiled only under
// //go:build ze_rest; absent the tag this init() does not exist, so restBuild
// stays nil and the hub builds no REST server.

//go:build ze_rest

package hub

import zeconfig "github.com/ze-software/ze/internal/component/config"

func init() {
	if err := zeconfig.RegisterServiceBlock("environment/api-server/rest", serviceNameREST); err != nil {
		panic("BUG: hub-rest: service block registration: " + err.Error())
	}
	restBuild = restBuildImpl
}
