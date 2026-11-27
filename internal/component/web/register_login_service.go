// Design: ai/patterns/registration.md -- web login-service registration
// Related: auth.go -- sends authServiceWeb on AuthRequest.Service and validates cookies

package web

import "github.com/ze-software/ze/internal/component/config/loginservice"

// loginServiceWeb is the registered login-service name for the web interface.
// It matches authServiceWeb so the wire value sent to TACACS+ and RADIUS stays
// identical to the registered name.
const loginServiceWeb = authServiceWeb

func init() {
	loginservice.MustRegister(loginservice.Entry{
		Name:        loginServiceWeb,
		Owner:       "internal/component/web",
		Description: "Web UI form, Basic Auth, and session cookie login",
	})
}
