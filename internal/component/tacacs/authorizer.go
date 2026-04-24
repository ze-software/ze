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
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
)

// splitTacacsArgs splits a command string into TACACS+ convention arguments.
// RFC 8907 Section 6: service=shell, cmd=<verb>, cmd-arg=<arg1>, cmd-arg=<arg2>, ...
func splitTacacsArgs(command string) []string {
	args := []string{"service=shell"}
	parts := strings.Fields(command)
	if len(parts) == 0 {
		args = append(args, "cmd=")
		return args
	}
	args = append(args, "cmd="+parts[0])
	for _, p := range parts[1:] {
		args = append(args, "cmd-arg="+p)
	}
	return args
}

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
func (a *TacacsAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	args := splitTacacsArgs(command)
	req := &AuthorRequest{
		AuthenMethod:  AuthenMethodTACACS,
		PrivLvl:       1,
		AuthenType:    0x01, // ASCII
		AuthenService: 0x01, // login
		User:          username,
		RemAddr:       remoteAddr,
		Port:          "ssh",
		Args:          args,
	}

	resp, err := a.client.SendAuthorization(req)
	if err != nil {
		a.logger.Warn("TACACS+ authorization server unreachable, using local RBAC",
			"username", username, "command", command, "error", err)
		return a.fallback(username, remoteAddr, command, isReadOnly)
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
		return a.fallback(username, remoteAddr, command, isReadOnly)
	}

	a.logger.Warn("TACACS+ authorization unknown status, using local RBAC",
		"username", username, "command", command, "status", resp.Status)
	return a.fallback(username, remoteAddr, command, isReadOnly)
}

func (a *TacacsAuthorizer) fallback(username, remoteAddr, command string, isReadOnly bool) bool {
	if a.local == nil {
		return false
	}
	return a.local.Authorize(username, remoteAddr, command, isReadOnly)
}
