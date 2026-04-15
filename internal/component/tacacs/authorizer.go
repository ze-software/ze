// Design: (none -- new TACACS+ component)
// Overview: client.go -- TACACS+ TCP client
// Related: authenticator.go -- auth bridge (sibling wrapper around client)
// Related: accounting.go -- accounting bridge (sibling wrapper around client)

// TacacsAuthorizer implements aaa.Authorizer with TACACS+ per-command
// authorization (RFC 8907 Section 6). When enabled, each command is sent to
// the TACACS+ server for approval before execution. On server unreachability,
// falls back to the local authorizer supplied by the hub.
package tacacs

import (
	"log/slog"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
)

// TacacsAuthorizer wraps a local authorizer with TACACS+ per-command authorization.
// When the TACACS+ server is reachable, its decision is authoritative.
// On connection failure, falls back to the local authorizer.
type TacacsAuthorizer struct {
	client *TacacsClient
	local  aaa.Authorizer
	logger *slog.Logger
}

// NewTacacsAuthorizer creates a TacacsAuthorizer.
// The local authorizer is used as fallback when the TACACS+ server is unreachable.
func NewTacacsAuthorizer(client *TacacsClient, local aaa.Authorizer, logger *slog.Logger) *TacacsAuthorizer {
	if logger == nil {
		logger = slog.Default()
	}
	return &TacacsAuthorizer{client: client, local: local, logger: logger}
}

// Authorize sends an AUTHOR REQUEST to the TACACS+ server for the given command.
// RFC 8907 Section 6: service=shell, cmd=<command>.
//
// Returns:
//   - true on PASS_ADD or PASS_REPL (AC-9)
//   - false on FAIL (AC-10)
//   - Falls back to local authorizer on ERROR or connection failure.
func (a *TacacsAuthorizer) Authorize(username, command string, isReadOnly bool) bool {
	req := &AuthorRequest{
		AuthenMethod:  AuthenMethodTACACS,
		PrivLvl:       1,
		AuthenType:    0x01, // ASCII
		AuthenService: 0x01, // login
		User:          username,
		Port:          "ssh",
		Args: []string{
			"service=shell",
			"cmd=" + command,
		},
	}

	resp, err := a.client.SendAuthorization(req)
	if err != nil {
		a.logger.Warn("TACACS+ authorization server unreachable, using local RBAC",
			"username", username, "command", command, "error", err)
		return a.fallback(username, command, isReadOnly)
	}

	if resp.Status == AuthorStatusPassAdd || resp.Status == AuthorStatusPassRepl {
		return true
	}
	if resp.Status == AuthorStatusFail {
		a.logger.Info("TACACS+ authorization denied",
			"username", username, "command", command)
		return false
	}
	if resp.Status == AuthorStatusError {
		a.logger.Warn("TACACS+ authorization error, using local RBAC",
			"username", username, "command", command,
			"server-msg", resp.ServerMsg)
		return a.fallback(username, command, isReadOnly)
	}

	a.logger.Warn("TACACS+ authorization unknown status, using local RBAC",
		"username", username, "command", command, "status", resp.Status)
	return a.fallback(username, command, isReadOnly)
}

func (a *TacacsAuthorizer) fallback(username, command string, isReadOnly bool) bool {
	if a.local == nil {
		return false
	}
	return a.local.Authorize(username, command, isReadOnly)
}
