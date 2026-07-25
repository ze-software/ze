//go:build ze_web

// Design: ai/patterns/registration.md -- AAA registry (VFS-like)
// Related: aaa_lifecycle.go -- aaaBundle atomic slot + liveAAABundleAuthorizer
// Related: service_web.go -- web auth wiring (webAuth)

package hub

import (
	"github.com/ze-software/ze/internal/component/aaa"
)

// liveAAABundleAuthenticator authenticates against the live AAA bundle's chain
// (RADIUS/TACACS backends plus the local backend) once infra setup has installed
// it, and falls back to a statically-built local authenticator otherwise. The
// fallback covers two windows: (1) before the bundle exists -- in the BGP path
// the web server starts during buildServices, before the reactor's InfraHook
// builds the chain (post config load); and (2) users the chain does not
// recognize (a config-file web user may not be present in the chain's local
// backend). Mirroring liveAAABundleAuthorizer, it reads the atomic slot on every
// call so a config reload's freshly-built chain takes effect without restarting
// the web server (AC-2: RADIUS/TACACS admins authenticate on web).
type liveAAABundleAuthenticator struct {
	fallback aaa.Authenticator
}

func (a liveAAABundleAuthenticator) Authenticate(request aaa.AuthRequest) (aaa.AuthResult, error) {
	if bundle := aaaBundle.Load(); bundle != nil && bundle.Authenticator != nil {
		result, err := bundle.Authenticator.Authenticate(request)
		if err == nil && result.Authenticated {
			return result, nil
		}
		// The chain did not authenticate this user. Try the static local users
		// (zefs power user + config-file web users) so pre-existing web login
		// keeps working for users absent from the chain's local backend.
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
