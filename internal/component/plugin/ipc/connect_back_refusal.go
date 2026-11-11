// Design: docs/architecture/api/process-protocol.md -- a refused connect-back
// Related: tls.go -- AuthenticateWithLookup returns the refusal; PluginAcceptor.handleConn reports it
// Related: plugin_name.go -- the name form a refusal for a bad name quotes

package ipc

import (
	"errors"
	"log/slog"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ConnectBackRefusedCode is the doctor code a refused connect-back carries on
// its log line and its report-bus event. It MUST equal
// diagnostic.CodeDoctorPluginConnectBackRefused: diagnostic imports this
// package, so this package cannot import diagnostic, and
// TestConnectBackCodeMatchesDiagnostic compares the two.
const ConnectBackRefusedCode = "doctor-plugin-connect-back-refused"

// connectBackReportSource is the report-bus source of a refused connect-back.
const connectBackReportSource = "plugin-acceptor"

// claimedNameShownMax bounds the claimed name an event and a log line show.
// The auth frame is 4 KiB, so a refused name can be far longer than any name
// the form allows; the form's own maximum is shown whole.
const claimedNameShownMax = 64

// AuthRefusedCause says why an auth request that named a plugin was refused.
// The zero value is not a cause.
type AuthRefusedCause uint8

const (
	// AuthRefusedUnspecified is the zero value and is never returned.
	AuthRefusedUnspecified AuthRefusedCause = iota
	// AuthRefusedName means the claimed name is not of PluginNameForm.
	AuthRefusedName
	// AuthRefusedToken means the token is not the one bound to the claimed
	// name. A name the hub never issued a token for falls back to the shared
	// secret, so an unknown name is refused under this cause too when the
	// caller has a shared secret.
	AuthRefusedToken
	// AuthRefusedNoSecret means no secret is bound to the claimed name and the
	// caller has no shared secret to fall back to: the managed config server
	// refuses a name it has no client entry for under this cause. An empty
	// expected token never authenticates.
	AuthRefusedNoSecret
)

// AuthRefusal is a refused auth request that named a plugin. It carries the
// name the peer claimed and never the token it sent, so its text and every
// event built from it are safe to show an operator.
type AuthRefusal struct {
	Claimed string
	Cause   AuthRefusedCause
}

// Error keeps the text the auth handshake always returned for each cause.
func (r *AuthRefusal) Error() string {
	var tb textbuf.Buffer
	switch r.Cause {
	case AuthRefusedName:
		tb.Str("auth failed: invalid plugin name ").Quoted(shownClaim(r.Claimed))
	case AuthRefusedToken:
		tb.Str("auth failed: invalid token for plugin ").Quoted(shownClaim(r.Claimed))
	case AuthRefusedNoSecret:
		tb.Str("auth failed: no secret is bound to the name ").Quoted(shownClaim(r.Claimed))
	case AuthRefusedUnspecified:
		tb.Str("auth failed: refusal with no cause for plugin ").Quoted(shownClaim(r.Claimed))
	}
	return tb.String()
}

// Unwrap answers errAuthFailedInvalidToken for a token refusal and for a name
// with no secret, so a caller that matched the sentinel still does.
func (r *AuthRefusal) Unwrap() error {
	if r.Cause == AuthRefusedToken {
		return errAuthFailedInvalidToken
	}
	if r.Cause == AuthRefusedNoSecret {
		return errAuthFailedInvalidToken
	}
	return nil
}

// ShownClaim is the claimed name bounded for display: an event, a log line or
// a doctor record keeps no more of it than the name form allows.
func (r *AuthRefusal) ShownClaim() string {
	return shownClaim(r.Claimed)
}

// shownClaim bounds a claimed name for display.
func shownClaim(name string) string {
	if len(name) <= claimedNameShownMax {
		return name
	}
	return name[:claimedNameShownMax] + "..."
}

// reportConnectBackRefused logs a refused connect-back and raises it on the
// report bus under ConnectBackRefusedCode, where `show doctor` renders it
// through the recent-events check. The subject is the claimed name when the
// request named one, else the remote address. Neither the line nor the event
// carries the token.
//
// Before this, the refusal was a log line only: a plugin whose name the form
// refuses (an underscore, a dot, a name over 64 characters) hung for its
// connect-back timeout and doctor said nothing.
func reportConnectBackRefused(remote string, err error) {
	subject := remote
	var message, fix textbuf.Buffer
	message.Str("plugin connect-back from ").Str(remote).Str(" refused: ")

	refusal, named := errors.AsType[*AuthRefusal](err)
	switch {
	case !named:
		message.Err(err).Str(", so no plugin is attached to this connection")
		fix.Str("ze show plugin list")
	case refusal.Cause == AuthRefusedName:
		subject = shownClaim(refusal.Claimed)
		message.Str("the claimed name ").Quoted(subject).Str(" is not ").Str(PluginNameForm).
			Str(", so the plugin never attaches and its start waits for its timeout")
		fix.Str("rename the plugin external block to a name of ").Str(PluginNameForm).Str(", then ze config validate")
	case refusal.Cause == AuthRefusedNoSecret:
		subject = shownClaim(refusal.Claimed)
		message.Str("no secret is bound to plugin ").Quoted(subject).
			Str(", so the process is not one ze started under that name")
		fix.Str("ze show plugin list")
	default:
		subject = shownClaim(refusal.Claimed)
		message.Str("the token is not the one ze issued for plugin ").Quoted(subject).
			Str(", so the process is not the one ze started under that name")
		fix.Str("ze show plugin list")
	}

	text := message.String()
	slog.Warn("acceptor: plugin connect-back rejected",
		"code", ConnectBackRefusedCode, "plugin", subject, "remote", remote, "error", err)
	report.RaiseErrorFix(connectBackReportSource, ConnectBackRefusedCode, subject, text, fix.String())
}
