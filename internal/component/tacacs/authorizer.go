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

	"github.com/ze-software/ze/internal/component/aaa"
)

// splitTacacsArgs splits a legacy command string into TACACS+ convention arguments.
// RFC 8907 Section 6: service=shell, cmd=<verb>, cmd-arg=<arg1>, cmd-arg=<arg2>, ...
func splitTacacsArgs(command string) []string {
	return splitTacacsTokens(strings.Fields(command))
}

func splitTacacsCommandArgs(command string, args []string, peer string) []string {
	return splitTacacsTokens(aaa.CanonicalCommandTokens(command, args, peer))
}

func splitTacacsTokens(tokens []string) []string {
	args := []string{"service=shell"}
	if len(tokens) == 0 {
		args = append(args, "cmd=")
		return args
	}
	args = append(args, "cmd="+tokens[0])
	for _, token := range tokens[1:] {
		args = append(args, "cmd-arg="+token)
	}
	return args
}

// tacacsAuthorizer wraps a local authorizer with TACACS+ per-command authorization.
// When the TACACS+ server is reachable, its decision is authoritative. On
// connection failure, the default policy falls back to the local authorizer;
// strictFallback changes that fail mode to deny.
type tacacsAuthorizer struct {
	client         *TacacsClient
	local          aaa.Authorizer
	logger         *slog.Logger
	strictFallback bool
}

// newTacacsAuthorizer creates a tacacsAuthorizer with the default logger.
// The local authorizer is used as fallback when the TACACS+ server is unreachable.
func newTacacsAuthorizer(client *TacacsClient, local aaa.Authorizer) *tacacsAuthorizer {
	return newTacacsAuthorizerWithFallback(client, local, nil, false)
}

// newTacacsAuthorizerWithFallback creates a tacacsAuthorizer with explicit
// fallback behavior. strictFallback denies when TACACS+ authorization is
// unavailable instead of falling back to local RBAC.
func newTacacsAuthorizerWithFallback(client *TacacsClient, local aaa.Authorizer, logger *slog.Logger, strictFallback bool) *tacacsAuthorizer {
	if logger == nil {
		logger = slog.Default()
	}
	return &tacacsAuthorizer{client: client, local: local, logger: logger, strictFallback: strictFallback}
}

// BindLocalFallback returns a live TACACS+ authorizer whose server-error
// fallback is the supplied local policy generation.
func (a *tacacsAuthorizer) BindLocalFallback(local aaa.Authorizer) aaa.Authorizer {
	if a == nil {
		return nil
	}
	bound := *a
	bound.local = local
	return &bound
}

// BindProfiles returns a session authorizer whose local fallback uses only the
// profiles resolved by that authentication. Per-command TACACS+ authorization
// remains live.
func (a *tacacsAuthorizer) BindProfiles(profiles []string) aaa.Authorizer {
	if a == nil {
		return nil
	}
	bound := *a
	bound.local = aaa.BindProfiles(a.local, profiles)
	return &bound
}

// Authorize sends an AUTHOR REQUEST to the TACACS+ server for the given command.
// RFC 8907 Section 6: service=shell, cmd=<command>.
//
// Returns:
//   - true on PASS_ADD or PASS_REPL (AC-9)
//   - false on FAIL (AC-10)
//   - Falls back to local authorizer on ERROR or connection failure.
func (a *tacacsAuthorizer) Authorize(username, remoteAddr, command string, isReadOnly bool) bool {
	return a.authorize(username, remoteAddr, command, splitTacacsArgs(command), func() bool {
		return a.fallback(username, remoteAddr, command, isReadOnly)
	})
}

// AuthorizeCommandArgs implements aaa.CommandArgsAuthorizer.
// It preserves typed arg boundaries and peer scoping when building TACACS+
// cmd/cmd-arg fields for inter-plugin command dispatch.
func (a *tacacsAuthorizer) AuthorizeCommandArgs(username, remoteAddr, command string, args []string, peer string, isReadOnly bool) bool {
	authCommand := aaa.CanonicalCommand(command, args, peer)
	return a.authorize(username, remoteAddr, authCommand, splitTacacsCommandArgs(command, args, peer), func() bool {
		return a.fallbackArgs(username, remoteAddr, command, args, peer, isReadOnly)
	})
}

func (a *tacacsAuthorizer) authorize(username, remoteAddr, command string, args []string, fallback func() bool) bool {
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
		if a.strictFallback {
			a.logger.Warn("TACACS+ authorization server unreachable, denying by strict fallback",
				"username", username, "command", command, "error", err)
			return false
		}
		a.logger.Warn("TACACS+ authorization server unreachable, using local RBAC",
			"username", username, "command", command, "error", err)
		return fallback()
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
		if a.strictFallback {
			a.logger.Warn("TACACS+ authorization error, denying by strict fallback",
				"username", username, "command", command,
				"server-msg", resp.ServerMsg)
			return false
		}
		a.logger.Warn("TACACS+ authorization error, using local RBAC",
			"username", username, "command", command,
			"server-msg", resp.ServerMsg)
		return fallback()
	}

	if a.strictFallback {
		a.logger.Warn("TACACS+ authorization unknown status, denying by strict fallback",
			"username", username, "command", command, "status", resp.Status)
		return false
	}
	a.logger.Warn("TACACS+ authorization unknown status, using local RBAC",
		"username", username, "command", command, "status", resp.Status)
	return fallback()
}

func (a *tacacsAuthorizer) fallback(username, remoteAddr, command string, isReadOnly bool) bool {
	if a.local == nil {
		return false
	}
	return a.local.Authorize(username, remoteAddr, command, isReadOnly)
}

func (a *tacacsAuthorizer) fallbackArgs(username, remoteAddr, command string, args []string, peer string, isReadOnly bool) bool {
	if a.local == nil {
		return false
	}
	if authzArgs, ok := a.local.(aaa.CommandArgsAuthorizer); ok {
		return authzArgs.AuthorizeCommandArgs(username, remoteAddr, command, args, peer, isReadOnly)
	}
	return a.local.Authorize(username, remoteAddr, aaa.CanonicalCommand(command, args, peer), isReadOnly)
}
