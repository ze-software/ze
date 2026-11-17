// Design: docs/architecture/container/image-store.md -- the failures of a fetch and the classes they fall in
// Related: client.go -- the requests whose failures these are

package registry

import (
	"strings"
)

// Failure is the class of a failed request. Each class is one problem class
// of the store's doctor table: the container plugin, which owns the codes and
// the commands that fix them, maps the class to its code and its fix. This
// package names neither, so a code is declared once.
type Failure uint8

const (
	// FailureUnspecified is no class. No Error carries it.
	FailureUnspecified Failure = iota
	// FailureUnreachable: no answer could be had. Name resolution, the
	// connection, TLS, a server error, a stall, a refused redirect.
	FailureUnreachable
	// FailureAuthRefused: the registry or its realm refused the
	// credentials or the access, or asked for credentials the client lacks.
	FailureAuthRefused
	// FailureNotFound: the registry holds no such repository, tag or digest.
	FailureNotFound
	// FailureDigestMismatch: an object did not hash to its digest or did not
	// have its declared size.
	FailureDigestMismatch
	// FailureRefused: the answer is one Ze does not accept: a type, a bound,
	// a digest algorithm, no entry for the running platform.
	FailureRefused
)

// String names the class for a human. Never compare with it.
func (f Failure) String() string {
	switch f {
	case FailureUnspecified:
		return "unspecified"
	case FailureUnreachable:
		return "unreachable"
	case FailureAuthRefused:
		return "auth-refused"
	case FailureNotFound:
		return "not-found"
	case FailureDigestMismatch:
		return "digest-mismatch"
	case FailureRefused:
		return "refused"
	}
	return "unknown"
}

// Error is a failed request. Its text names what went wrong and why, and
// never a credential, a token or an authorization header: every URL in it was
// built by this package with no userinfo, and a registry's own error text is
// reduced to its error code. The fix is not here: the container plugin adds
// it from the class, with the command words it owns.
type Error struct {
	failure Failure
	problem string
	why     string
	cause   error
	// retry marks a failure another attempt may not meet: a transport error
	// or a server error. A mismatch is never retried: it is another file.
	retry bool
}

// failed builds an Error that is not retried.
func failed(failure Failure, problem, why string) *Error {
	return &Error{failure: failure, problem: problem, why: why}
}

// Error renders problem: why, then the cause when there is one.
func (e *Error) Error() string {
	text := e.problem + ": " + e.why
	if e.cause != nil {
		text += ": " + e.cause.Error()
	}
	return text
}

// Unwrap answers the cause, so errors.Is sees a stall or a TLS failure.
func (e *Error) Unwrap() error { return e.cause }

// Failure answers the class.
func (e *Error) Failure() Failure { return e.failure }

// Problem answers what went wrong, in one sentence.
func (e *Error) Problem() string { return e.problem }

// Why answers why it matters or what caused it.
func (e *Error) Why() string { return e.why }

// registryErrorCodeMax bounds the error code a registry's answer names.
const registryErrorCodeMax = 64

// registryErrorCode reduces a registry's error code to the letters, digits
// and underscores of the protocol's code set, bounded, so no text the
// registry chose reaches an operator beyond a word of that set.
func registryErrorCode(code string) string {
	var kept strings.Builder
	for _, c := range code {
		if kept.Len() == registryErrorCodeMax {
			break
		}
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			kept.WriteRune(c)
		}
	}
	return kept.String()
}
