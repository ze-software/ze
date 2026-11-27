// Design: ai/patterns/registration.md -- REST API login-service registration
// Related: auth.go -- passes loginServiceREST to api.Authentication.Authenticate

package rest

import "github.com/ze-software/ze/internal/component/config/loginservice"

// loginServiceREST is the registered login-service name for REST API per-user
// authentication.
const loginServiceREST = "rest"

func init() {
	loginservice.MustRegister(loginservice.Entry{
		Name:        loginServiceREST,
		Owner:       "internal/component/api/rest",
		Description: "REST API per-user authentication",
	})
}
