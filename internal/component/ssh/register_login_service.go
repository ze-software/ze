// Design: ai/patterns/registration.md -- SSH login-service registration
// Related: passwordauth.go -- sends componentSSH on AuthRequest.Service
// Related: pubkey.go -- enforces the SSH login-service gate on public-key login

package ssh

import "github.com/ze-software/ze/internal/component/config/loginservice"

// loginServiceSSH is the registered login-service name for SSH and ze cli
// logins. It matches componentSSH so the wire value sent to TACACS+ and RADIUS
// stays identical to the registered name.
const loginServiceSSH = componentSSH

func init() {
	loginservice.MustRegister(loginservice.Entry{
		Name:        loginServiceSSH,
		Owner:       "internal/component/ssh",
		Description: "SSH and ze cli login",
	})
}
