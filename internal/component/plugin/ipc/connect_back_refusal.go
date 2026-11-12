// Design: docs/architecture/api/process-protocol.md -- a refused connect-back
// Related: tls.go -- AuthenticateWithLookup returns the refusal; PluginAcceptor.handleConn reports it
// Related: plugin_name.go -- the name form a refusal for a bad name quotes

package ipc

import (
	"errors"
	"log/slog"
	"strconv"
	"unicode/utf8"

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

// claimedNameShownMax bounds, in bytes after escaping, the claimed name an
// event, a log line and a doctor record show (escapeClaim). The auth frame is
// 4 KiB, so a refused name can be far longer than any name the form allows;
// the form's own maximum is shown whole.
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
		writeQuotedClaim(tb.Str("auth failed: invalid plugin name "), r.Claimed)
	case AuthRefusedToken:
		writeQuotedClaim(tb.Str("auth failed: invalid token for plugin "), r.Claimed)
	case AuthRefusedNoSecret:
		writeQuotedClaim(tb.Str("auth failed: no secret is bound to the name "), r.Claimed)
	case AuthRefusedUnspecified:
		writeQuotedClaim(tb.Str("auth failed: refusal with no cause for plugin "), r.Claimed)
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

// ShownClaim is the claimed name in the form an event subject, a log line or
// a doctor record keeps: safe to print and bounded (see shownClaim).
func (r *AuthRefusal) ShownClaim() string {
	return shownClaim(r.Claimed)
}

// claimElided marks a shown claim cut at claimedNameShownMax.
const claimElided = "..."

// claimEmptyShown is the shown form of an empty claim. The report bus refuses
// an empty subject, so an empty claim shown as itself would drop its event in
// silence; two quote marks cannot be the escaped form of any other claim,
// because escapeClaim writes a quote mark as \".
const claimEmptyShown = `""`

// shownClaim answers a claimed name as text safe to put before an operator
// unquoted: the subject of a report-bus event and the Client of a managed
// doctor record. The name comes off the wire from any peer that finished TLS,
// and neither the bus nor a renderer checks characters, so this is the one
// place it is made printable.
//
// A name of PluginNameForm is answered unchanged: it holds only letters,
// digits and hyphens within claimedNameShownMax bytes, so it is already safe,
// and the record key stays the name the operator configured. Any other name
// is answered escaped and bounded by escapeClaim, with no quote marks around
// it, so a name of the form that a test or a doctor row names is still found
// under itself. Escaping alone is injective, as strconv writes it, so two
// different claims within the bound never share a shown form; the cut is
// not, so two claims whose escaped text agrees up to the cut show the same
// form and share one event subject or doctor row.
func shownClaim(name string) string {
	if ValidPluginName(name) {
		return name
	}
	if name == "" {
		return claimEmptyShown
	}
	return escapeClaim(name)
}

// writeQuotedClaim writes a claimed name to a message, between quote marks.
// It is the quoted form of escapeClaim: for a name within the bound it equals
// strconv's quoting of the name.
func writeQuotedClaim(tb *textbuf.Buffer, name string) *textbuf.Buffer {
	return tb.Byte('"').Str(escapeClaim(name)).Byte('"')
}

// escapeClaim answers name escaped as strconv.Quote escapes it (a control
// character, a non-printable rune and an invalid UTF-8 byte become an escape
// sequence; a quote mark and a backslash are escaped too) without the quote
// marks, and cut, at a rune and never inside an escape, where the escaped text
// would pass claimedNameShownMax bytes, with claimElided after the cut. The
// cut runs on the escaped text, not on the name, because one byte of the name
// can escape to four: a cut before escaping would bound the input and let the
// shown text grow past the report bus's subject limit, which drops the event
// in silence. The result is at most claimedNameShownMax+len(claimElided)
// bytes, all of them valid UTF-8 and none of them a control character.
func escapeClaim(name string) string {
	shown := make([]byte, 0, claimedNameShownMax+len(claimElided))
	var scratch [16]byte // The longest one-rune quote, `"\U0010ffff"`, is 12 bytes.
	for offset := 0; offset < len(name); {
		_, width := utf8.DecodeRuneInString(name[offset:])
		// strconv quotes rune by rune (an invalid byte is one rune of width
		// one), so quoting one rune alone writes the escape it writes in the
		// whole name.
		quoted := strconv.AppendQuote(scratch[:0], name[offset:offset+width])
		escaped := quoted[1 : len(quoted)-1]
		if len(shown)+len(escaped) > claimedNameShownMax {
			return string(append(shown, claimElided...))
		}
		shown = append(shown, escaped...)
		offset += width
	}
	return string(shown)
}

// reportConnectBackRefused logs a refused connect-back and raises it on the
// report bus under ConnectBackRefusedCode, where `show doctor` renders it
// through the recent-events check. The subject is the claimed name, in the
// printable form shownClaim gives it, when the request named one, else the
// remote address. Neither the line nor the event carries the token.
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
		writeQuotedClaim(message.Str("the claimed name "), refusal.Claimed).Str(" is not ").Str(PluginNameForm).
			Str(", so the plugin never attaches and its start waits for its timeout")
		fix.Str("rename the plugin external block to a name of ").Str(PluginNameForm).Str(", then ze config validate")
	case refusal.Cause == AuthRefusedNoSecret:
		subject = shownClaim(refusal.Claimed)
		writeQuotedClaim(message.Str("no secret is bound to plugin "), refusal.Claimed).
			Str(", so the process is not one ze started under that name")
		fix.Str("ze show plugin list")
	default:
		subject = shownClaim(refusal.Claimed)
		writeQuotedClaim(message.Str("the token is not the one ze issued for plugin "), refusal.Claimed).
			Str(", so the process is not the one ze started under that name")
		fix.Str("ze show plugin list")
	}

	// An error that is not an AuthRefusal can carry a verb the peer sent,
	// quoted but up to the 4 KiB auth frame long: the bus refuses a message
	// past its limit and drops the event in silence, so the text is cut to fit.
	text := report.FitMessage(message.String())
	slog.Warn("acceptor: plugin connect-back rejected",
		"code", ConnectBackRefusedCode, "plugin", subject, "remote", remote, "error", err)
	report.RaiseErrorFix(connectBackReportSource, ConnectBackRefusedCode, subject, text, fix.String())
}
