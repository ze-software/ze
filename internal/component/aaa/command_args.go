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
type CommandArgsAuthorizer interface {
	AuthorizeCommandArgs(username, remoteAddr, command string, args []string, peer string, isReadOnly bool) bool
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
