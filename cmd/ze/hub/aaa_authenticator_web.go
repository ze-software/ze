//go:build ze_web

// Design: ai/patterns/registration.md -- AAA registry (VFS-like)
// Related: aaa_lifecycle.go -- aaaBundle atomic slot + liveAAABundleAuthorizer
// Related: service_web.go -- web auth wiring (webAuth)
// Related: main_servers.go -- liveConfigUsers reads the running config's users

package hub

import (
	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
)

// liveAAABundleAuthenticator authenticates against the live AAA bundle's chain
// (RADIUS/TACACS backends plus the local backend) once infra setup has installed
// it, and falls back to configUsersAuthenticator otherwise. The fallback covers
// two windows: (1) before the bundle exists -- in the BGP path the web server
// starts during buildServices, before the reactor's InfraHook builds the chain
// (post config load); and (2) users the chain does not recognize (a config-file
// web user may not be present in the chain's local backend). It reads the atomic
// slot on every call, mirroring liveAAABundleAuthorizer, so a freshly-built
// chain takes effect without restarting the web server (AC-2: RADIUS/TACACS
// admins authenticate on web).
//
// The fallback answers from the CURRENT configuration, never from a startup
// snapshot. That is what makes the second window safe: the chain not knowing a
// user and the operator having deleted that user look identical from here, and
// only a reader of the running config can tell them apart.
type liveAAABundleAuthenticator struct {
	fallback aaa.Authenticator
}

func (a liveAAABundleAuthenticator) Authenticate(request aaa.AuthRequest) (aaa.AuthResult, error) {
	if bundle := aaaBundle.Load(); bundle != nil && bundle.Authenticator != nil {
		result, err := bundle.Authenticator.Authenticate(request)
		if err == nil && result.Authenticated {
			return result, nil
		}
		// The chain did not authenticate this user. Try the users the running
		// configuration declares (zefs power user + config-file web users) so
		// web login keeps working for users absent from the chain's local
		// backend.
		if a.fallback != nil {
			if fres, ferr := a.fallback.Authenticate(request); ferr == nil && fres.Authenticated {
				return fres, nil
			}
		}
		return result, err
	}
	if a.fallback != nil {
		return a.fallback.Authenticate(request)
	}
	return aaa.AuthResult{}, aaa.ErrAuthRejected
}

// noConfigUsers is the live user source for a surface that runs with no
// configuration loaded, such as `ze web` standalone mode. It reports an empty
// config truthfully, so only the zefs power user authenticates there.
//
// Named rather than left nil so that forgetting to wire a real source is a
// visible choice at the call site instead of a silent "config users never
// authenticate".
func noConfigUsers() ([]authz.UserConfig, error) { return nil, nil }
