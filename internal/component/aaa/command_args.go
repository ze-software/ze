// Design: docs/architecture/aaa-tacacs.md -- typed command authorization contract

package aaa

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// CommandArgsAuthorizer is the authoritative authorization contract for typed
// inter-plugin command dispatch. Implementations receive the exact registered
// command name, its pre-tokenized args, and the explicit peer scope without
// first flattening them into a single string.
//
// Built-in authorizers SHOULD implement this interface so policy never depends
// on a reconstructed command string. CanonicalCommand exists only as a
// compatibility fallback for legacy aaa.Authorizer implementations that have
// not adopted typed args yet.
//
// Implementations should preserve the dispatcher's existing authorization
// semantics: if peer is explicit, the command is peer-scoped, but the selector
// value itself is not part of the legacy RBAC token stream. Typed authorizers
// that need the selector can inspect the peer parameter directly.
//
// For a line typed with a pipe chain, args ends with the chain as typed: a
// "|" token before each operation, then the operation's name and its argument
// words, as the chain grammar decoded them (a backslash there stays a literal
// character). They are given for authorization only, so a per-command policy
// sees what the operator entered; the handler's arguments never hold them.
// isReadOnly is the command path's own flag, which a chain never changes. An
// implementation passes the tokens on unchanged, or flattens them with
// CanonicalCommand for a string authorizer.
type CommandArgsAuthorizer interface {
	AuthorizeCommandArgs(username, remoteAddr, command string, args []string, peer string, isReadOnly bool) bool
}

// LocalScopeAuthorizer evaluates canonical command-argument read tokens
// strictly locally against the caller's authenticated policy or profile
// binding. Implementations MUST NEVER perform remote network authorization
// (such as TACACS+ AUTHOR requests).
type LocalScopeAuthorizer interface {
	AuthorizeLocalScope(username, command string, args []string, isReadOnly bool) bool
}

// AuthorizeLocalScope evaluates canonical command-argument tokens strictly
// locally when authorizer implements LocalScopeAuthorizer. When authorizer is
// nil or does not implement LocalScopeAuthorizer, it returns (false, false) so
// callers fail closed without falling back to remote authorization.
func AuthorizeLocalScope(authorizer Authorizer, username, command string, args []string, isReadOnly bool) (allowed, supported bool) {
	localScope, ok := authorizer.(LocalScopeAuthorizer)
	if !ok {
		return false, false
	}
	return localScope.AuthorizeLocalScope(username, command, args, isReadOnly), true
}

// ExplicitAuthorizer evaluates commands that require an explicit default-deny
// authorization grant (such as private-key export). Implementations MUST fail
// closed when RBAC is unconfigured, when a profile permits only via
// default-action allow, or for reserved/break-glass identities.
//
// canonicalPath is the registered canonical command path (for example
// "show pki certificate name bundle pem"). tokens is the typed command token
// stream preserving inline selectors and argument boundaries.
type ExplicitAuthorizer interface {
	AuthorizeExplicit(username, remoteAddr, canonicalPath string, tokens []string, isReadOnly bool) bool
}

// AuthorizeExplicit evaluates an explicit default-deny authorization grant when
// authorizer implements ExplicitAuthorizer. When authorizer is nil or does not
// implement ExplicitAuthorizer, it returns (false, false) so callers fail
// closed without falling back to permissive Authorize semantics.
func AuthorizeExplicit(
	authorizer Authorizer,
	username, remoteAddr, canonicalPath string,
	tokens []string,
	isReadOnly bool,
) (allowed, supported bool) {
	explicit, ok := authorizer.(ExplicitAuthorizer)
	if !ok {
		return false, false
	}
	return explicit.AuthorizeExplicit(username, remoteAddr, canonicalPath, tokens, isReadOnly), true
}

const (
	// FleetDeviceReadCommand is the canonical read command evaluated locally to
	// project per-device fleet visibility.
	FleetDeviceReadCommand = "show fleet device"
	// FleetDeviceNameArg is the canonical argument keyword naming a device in
	// FleetDeviceReadCommand.
	FleetDeviceNameArg = "name"
)

// FleetDeviceReadTokens returns the canonical command and argument tokens
// evaluated locally when projecting fleet visibility for deviceName.
func FleetDeviceReadTokens(deviceName string) (command string, args []string) {
	return FleetDeviceReadCommand, []string{FleetDeviceNameArg, deviceName}
}

// CanonicalCommandTokens returns the authorization token stream for a typed
// command dispatch. It mirrors the dispatcher's legacy string authorization
// shape: explicit peer scoping contributes a leading "peer" token, but not the
// selector value itself, because the dispatcher strips selectors before legacy
// authorization checks.
func CanonicalCommandTokens(command string, args []string, peer string) []string {
	commandTokens := strings.Fields(command)
	capacity := len(commandTokens) + len(args)
	if peer != "" && peer != "*" {
		capacity++
	}
	if capacity == 0 {
		return nil
	}

	tokens := make([]string, 0, capacity)
	if peer != "" && peer != "*" {
		tokens = append(tokens, "peer")
	}
	tokens = append(tokens, commandTokens...)
	tokens = append(tokens, args...)
	return tokens
}

// CanonicalCommand rebuilds a typed command into the legacy RBAC fallback
// string used only for aaa.Authorizer implementations that do not implement
// CommandArgsAuthorizer.
//
// Args that contain whitespace, quotes, or backslashes are quoted with
// textbuf.Buffer.Quoted (strconv.AppendQuote) so the original argument
// boundaries remain visible to regex-based legacy authorizers. This quoting form is a compatibility detail,
// not the primary policy contract for typed dispatch.
func CanonicalCommand(command string, args []string, peer string) string {
	tokens := CanonicalCommandTokens(command, args, peer)
	if len(tokens) == 0 {
		return ""
	}

	total := 0
	for _, token := range tokens {
		total += len(token)
		if needsQuotedToken(token) {
			total += 2
		}
	}
	if len(tokens) > 1 {
		total += len(tokens) - 1
	}

	var b textbuf.Buffer
	b.Reset(total)
	for i, token := range tokens {
		if i > 0 {
			b.Byte(' ')
		}
		if needsQuotedToken(token) {
			b.Quoted(token)
			continue
		}
		b.Str(token)
	}
	return b.String()
}

func needsQuotedToken(token string) bool {
	if token == "" {
		return true
	}
	for _, r := range token {
		switch r {
		case ' ', '\t', '\n', '\r', '"', '\\':
			return true
		}
	}
	return false
}
