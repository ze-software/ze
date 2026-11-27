// Design: ai/patterns/registration.md -- gRPC API login-service registration
// Related: server.go -- passes loginServiceGRPC to api.Authentication.Authenticate

package grpc

import "github.com/ze-software/ze/internal/component/config/loginservice"

// loginServiceGRPC is the registered login-service name for gRPC API per-user
// authentication.
const loginServiceGRPC = "grpc"

func init() {
	loginservice.MustRegister(loginservice.Entry{
		Name:        loginServiceGRPC,
		Owner:       "internal/component/api/grpc",
		Description: "gRPC API per-user authentication",
	})
}
