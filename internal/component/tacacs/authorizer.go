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
	"errors"
	"log/slog"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config"
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
	args = append(args, "cmd="+asciiCommandValue(tokens[0]))
	for _, token := range tokens[1:] {
		args = append(args, "cmd-arg="+asciiCommandValue(token))
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
//   - true on PASS_ADD or PASS_REPL whose effective arguments can be enforced
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
		Port:          portSSH,
		Args:          args,
	}

	resp, err := a.client.SendAuthorization(req)
	if err != nil {
		if errors.Is(err, errRequestInvalid) {
			return false
		}
		if a.strictFallback {
			a.logger.Warn("TACACS+ authorization server unreachable, denying by strict fallback",
				"username", username, "command", config.DisplayCommand(command), "error", err)
			return false
		}
		a.logger.Warn("TACACS+ authorization server unreachable, using local RBAC",
			"username", username, "command", config.DisplayCommand(command), "error", err)
		return fallback()
	}

	if resp.Status == AuthorStatusPassAdd || resp.Status == AuthorStatusPassRepl {
		_, allowed := applyAuthorizationArgs(args, resp, false)
		return allowed
	}
	if resp.Status == AuthorStatusFail {
		a.logger.Info("TACACS+ authorization denied",
			"username", username, "command", config.DisplayCommand(command))
		return false
	}
	// RFC 8907 Section 10.5.5: "TACACS+ clients SHOULD deprecate this
	// feature by treating TAC_PLUS_AUTHEN_STATUS_FOLLOW as
	// TAC_PLUS_AUTHEN_STATUS_FAIL."
	// Section 6.2 applies the same redirection handling to authorization.
	// ERROR already took the client's backup-server/infrastructure path.
	a.logger.Warn("TACACS+ authorization unsupported status, denying",
		"username", username, "command", config.DisplayCommand(command), "status", resp.Status)
	return false
}

// applyAuthorizationArgs checks the effective authorization against the consumer.
// The dispatcher accepts only a boolean decision: it executes the original
// command and cannot install session restrictions or rewrite command arguments.
// Session authorization additionally consumes priv-lvl through profile mapping.
//
// RFC 8907 Sections 6.1 and 6.2: each length-delimited AV is laid out as
// [name bytes][first '=' or '*'][value bytes], at byte offsets 0, separator,
// and separator+1. The separator is mandatory '=' or optional '*'; later
// separators belong to the value. Each AV is 2..255 bytes.
func applyAuthorizationArgs(request []string, reply *AuthorResponse, session bool) (int, bool) {
	privLvl := 1
	serviceSeen, commandSeen, privilegeSeen := false, false, false
	commandArg := 2 // request[0:2] is service=shell followed by cmd.
	effective := [2][]string{request, reply.Args}
	switch reply.Status {
	case AuthorStatusPassAdd:
		// RFC 8907 Section 6.2: "If the status equals
		// TAC_PLUS_AUTHOR_STATUS_PASS_ADD, then the arguments specified in
		// the request are authorized and the arguments in the response MUST
		// be applied according to the rules described above."
	case AuthorStatusPassRepl:
		// RFC 8907 Section 6.2: "If the status equals
		// TAC_PLUS_AUTHOR_STATUS_PASS_REPL, then the client MUST use the
		// authorization argument-value pairs (if any) in the response
		// instead of the authorization argument-value pairs from the request."
		effective[0] = nil
	default:
		return 0, false
	}
	// Each list is bounded by the uint8 argument count in the wire codec.
	for _, args := range effective {
		for _, arg := range args {
			if len(arg) < 2 || len(arg) > 255 {
				return 0, false
			}
			separator := strings.IndexAny(arg, "=*")
			if separator < 1 {
				return 0, false
			}
			name, value := arg[:separator], arg[separator+1:]
			handled := false
			switch name {
			case "service":
				if value != "shell" {
					return 0, false
				}
				serviceSeen = true
				handled = true
			case "cmd":
				if value != request[1][4:] {
					return 0, false
				}
				commandSeen = true
				handled = true
			case "cmd-arg":
				if session {
					break
				}
				if commandArg == len(request) {
					return 0, false
				}
				if value != request[commandArg][8:] {
					return 0, false
				}
				commandArg++
				handled = true
			case "priv-lvl":
				if !session {
					break
				}
				if privilegeSeen {
					return 0, false
				}
				level, valid := parsePrivLvl(value)
				// RFC 8907 Section 8.1: "If the length cannot be
				// accommodated, then the argument MUST be regarded as not
				// handled and the logic in "Authorization" (Section 6.1)
				// regarding the processing of arguments MUST be applied."
				if !valid {
					break
				}
				privLvl = level
				privilegeSeen = true
				handled = true
			}
			// RFC 8907 Section 6.1: "If the client receives a mandatory
			// argument that it cannot handle, it MUST consider the
			// authorization to have failed."
			// RFC 8907 Section 10.5.4: "TACACS+ clients that receive an
			// unrecognized mandatory argument MUST evaluate server response
			// as if they received TAC_PLUS_AUTHOR_STATUS_FAIL."
			if !handled {
				if arg[separator] == '=' {
					return 0, false
				}
			}
		}
	}
	// Replacement must still describe the same complete shell command.
	// Missing service/cmd or a removed/reordered argument cannot authorize
	// the immutable original command the dispatcher will execute.
	return privLvl, serviceSeen && commandSeen && commandArg == len(request)
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
