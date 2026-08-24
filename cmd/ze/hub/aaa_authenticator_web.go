//go:build ze_web

// Design: ai/patterns/registration.md -- AAA registry (VFS-like)
// Related: aaa_lifecycle.go -- aaaBundle atomic slot + liveAAABundleAuthenticator
// Related: service_web.go -- web auth wiring (webAuth)
// Related: main_servers.go -- liveConfigUsers reads the running config's users

package hub

import (
	"github.com/ze-software/ze/internal/component/authz"
)

// noConfigUsers is the live user source for a surface that runs with no
// configuration loaded, such as `ze web` standalone mode. It reports an empty
// config truthfully, so only the zefs power user authenticates there.
//
// Named rather than left nil so that forgetting to wire a real source is a
// visible choice at the call site instead of a silent "config users never
// authenticate".
func noConfigUsers() ([]authz.UserConfig, error) { return nil, nil }
