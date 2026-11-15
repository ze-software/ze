// Design: docs/architecture/bfd.md -- the problems the BFD engine reports
// Related: loop.go -- handleInbound and sendLocked, which report failed authentication and refused sends
// Related: echo.go -- sendEchoLocked and reflectEchoLocked, which report refused echo sends
// Related: engine.go -- IssueReporter, the seam the reports leave through
//
// The engine reports three operating problems: a session whose packets the
// transport refuses to send, a session whose received Control packets fail
// authentication, and a Meticulous session whose last TX sequence number
// could not be saved to the state store. Each one is a WARN line carrying its
// doctor code and a raise on the daemon's report bus under the same code,
// through the loop's IssueReporter, so the raise reaches the daemon whether
// the bfd plugin runs in it or forked.
package engine

import (
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// AuthFailedCode is the doctor code of a session whose received Control
// packets fail authentication. The Warn line warnAuthFailedLocked writes and
// the report-bus warning it raises carry it; the bfd plugin describes it and
// registers it as an event code (register.go, bfdDiagnosticCodes).
const AuthFailedCode = "doctor-bfd-auth-failed"

// SeqUnsavedCode is the doctor code of a Meticulous session whose last TX
// sequence number the state store did not save. The Warn lines
// reportSeqUnsaved writes and the error events it raises carry it; the bfd
// plugin describes it and registers it as an event code.
const SeqUnsavedCode = "doctor-bfd-auth-seq-unsaved"

// SendFailedCode is the doctor code of a session whose packets the transport
// refuses to send. The Warn line warnSendFailedLocked writes and the
// report-bus warning it raises carry it; the bfd plugin describes it and
// registers it as an event code.
const SendFailedCode = "doctor-bfd-send-failed"

// engineLogFix is where an operator reads what a report that did not reach
// the report bus said: the engine's own Warn lines.
const engineLogFix = "ze show log recent level warn component bfd.engine"

// The two observations of an unsaved sequence number (reportSeqUnsaved): the
// persister's write failed while the session ran, or the session's
// authentication closed with the last write failed.
const (
	seqWriteFailedProblem = "bfd could not save the authentication sequence number of a session to the state store"
	seqCloseFailedProblem = "bfd closed the authentication of a session with its last sequence number unsaved"
	seqUnsavedEffect      = "after ze restarts, the session sends from an older sequence number, and a peer " +
		"using meticulous authentication drops its packets until the number passes the one it last accepted " +
		"(RFC 5880 Section 6.7.3)"
	seqUnsavedFix = "ze doctor"
)

// warnAuthFailedLocked reports a received Control packet of entry that failed
// authentication and was dropped (RFC 5880 Section 6.8.6). A session whose
// packets all fail cannot come Up, and an Up one goes Down when its Detection
// Time passes, so without this line the operator sees the Down and not its
// cause. Anyone who knows the discriminator can send such a packet, so every
// drop is counted (OnAuthFailure) and only the first, and the first after each
// sendWarnInterval, is logged at Warn and raised on the report bus; the rest go
// to Debug. A packet that verifies clears the raise (clearAuthWarning) and
// keeps the window, so packets a sender interleaves with the peer's good ones
// earn one Warn and one raise per window, never one per packet. A peer that
// fails and then falls silent sends nothing that verifies, so tick withdraws
// the raise once no packet has failed for warningQuiet (expireQuietWarnings).
// The Verify error names the check that failed and never the secret. Caller
// MUST hold l.mu.
func (l *Loop) warnAuthFailedLocked(entry *sessionEntry, in transport.Inbound, err error) {
	key := entry.machine.Key()
	if hook := l.metricsHook.Load(); hook != nil {
		(*hook).OnAuthFailure(in.Mode.String())
	}
	now := l.clk.Now()
	entry.authFailedAt = now
	if !entry.authWarnAt.IsZero() {
		if now.Sub(entry.authWarnAt) < sendWarnInterval {
			engineLog().Debug("bfd packet failed authentication", "peer", in.From,
				"interface", key.Interface, "vrf", key.VRF, "mode", key.Mode.String(), "err", err)
			return
		}
	}
	entry.authWarnAt = now
	var fix textbuf.Buffer
	fix.Reset().Str("make auth type, key-id and secret")
	if entry.authFrom != "" {
		fix.Str(" of bfd profile '").Str(entry.authFrom).Byte('\'')
	}
	fix.Str(" match the peer's, on both routers")
	// String leaves the buffer empty, so the text is read once for both uses.
	fixText := fix.String()
	subject := sessionSubject(entry)
	raiseErr := entry.issues.raiseWarning(AuthFailedCode, subject,
		"BFD packets from the peer fail authentication and are dropped: "+err.Error(), fixText,
		authReportFailed(subject))
	if raiseErr == nil {
		entry.authRaisedAt = now
	}
	engineLog().Warn("bfd packet from the peer failed authentication and was dropped, so the session cannot stay Up",
		"code", AuthFailedCode, "peer", in.From, "interface", key.Interface, "vrf", key.VRF,
		"mode", key.Mode.String(), "local-discriminator", entry.machine.LocalDiscriminator(),
		"profile", entry.authFrom, "reason", err.Error(), "fix", fixText, reportOutcome(raiseErr))
}

// clearAuthWarning withdraws the warning warnAuthFailedLocked raised for
// entry, when one is raised, and keeps the Warn window. handleInbound calls it
// for every packet that verifies, and expireQuietWarnings once no packet has
// failed for warningQuiet, so the path with nothing raised is one check. The
// caller MUST hold l.mu of entry's loop.
func clearAuthWarning(entry *sessionEntry) {
	if entry.authRaisedAt.IsZero() {
		return
	}
	entry.authRaisedAt = time.Time{}
	subject := sessionSubject(entry)
	if err := entry.issues.clearWarning(AuthFailedCode, subject, authReportFailed(subject)); err != nil {
		engineLog().Warn("bfd could not clear the authentication warning of a session, so show warnings may still list it",
			"code", AuthFailedCode, "subject", subject, "error", err, "fix", engineLogFix)
	}
}

// resetAuthWarning withdraws entry's raise and restarts its Warn window, for a
// session whose authentication is replaced (replaceAuth) or that is removed
// (removeEntryLocked, Loop.Stop): a failure the old authentication met says
// nothing of the new one. The caller MUST hold l.mu of entry's loop.
func resetAuthWarning(entry *sessionEntry) {
	clearAuthWarning(entry)
	entry.authWarnAt = time.Time{}
	entry.authFailedAt = time.Time{}
}

// sessionSubject names entry on the report bus by what stays fixed for
// the entry's life: its peer, VRF, hop mode and local discriminator.
func sessionSubject(entry *sessionEntry) string {
	key := entry.machine.Key()
	var subject textbuf.Buffer
	subject.Reset().Str("peer ").Addr(entry.machine.PeerAddr()).Str(" vrf ").Str(key.VRF).
		Byte(' ').Str(key.Mode.String()).Str(" discriminator ").Int(int64(entry.machine.LocalDiscriminator()))
	return subject.String()
}

// authReportFailed is the failed callback of an AuthFailedCode raise or
// clear: the daemon did not take it, so the report bus is wrong about the
// session and only the log holds what happened.
func authReportFailed(subject string) func(error) {
	return func(err error) {
		engineLog().Warn("bfd could not update the authentication warning of a session on the report bus, so show warnings may be wrong about it",
			"code", AuthFailedCode, "subject", subject, "error", err, "fix", engineLogFix)
	}
}

// closeAuth closes entry's authentication persister, which flushes the last
// TX sequence number to the state store. A persister whose last write failed
// answers that failure, and it is reported here: the authentication is
// replaced, or the session is gone, whatever Close answers, so the report is
// the only action left. The caller MUST hold l.mu of entry's loop.
func closeAuth(entry *sessionEntry) {
	err := entry.machine.CloseAuth()
	if err == nil {
		return
	}
	reportSeqUnsaved(entry.issues, entry.machine.Key(), seqCloseFailedProblem, err)
}

// reportSeqUnsaved logs at Warn, and raises as an error event, that the TX
// sequence number of the Meticulous session key was not saved. problem is
// seqWriteFailedProblem from the persister's writer goroutine, once per run of
// failed writes, or seqCloseFailedProblem from closeAuth. It takes no lock and
// does not block, so either caller may hold l.mu. err names the store write
// that failed, never the authentication secret.
func reportSeqUnsaved(issues *issueSink, key api.Key, problem string, err error) {
	subject := seqSubject(key)
	raiseErr := issues.raiseError(SeqUnsavedCode, subject, problem+": "+err.Error(), seqUnsavedFix,
		seqReportFailed(subject))
	engineLog().Warn(problem, "code", SeqUnsavedCode, "peer", key.Peer, "vrf", key.VRF,
		"mode", key.Mode.String(), "reason", err.Error(), "effect", seqUnsavedEffect,
		"fix", seqUnsavedFix, reportOutcome(raiseErr))
}

// seqSubject names a session on the report bus by the identity its sequence
// number is saved under: its peer, VRF and hop mode.
func seqSubject(key api.Key) string {
	var subject textbuf.Buffer
	subject.Reset().Str("peer ").Addr(key.Peer).Str(" vrf ").Str(key.VRF).Byte(' ').Str(key.Mode.String())
	return subject.String()
}

// seqReportFailed is the failed callback of a SeqUnsavedCode raise.
func seqReportFailed(subject string) func(error) {
	return func(err error) {
		engineLog().Warn("bfd could not raise an unsaved authentication sequence number on the report bus, so show errors does not list it",
			"code", SeqUnsavedCode, "subject", subject, "error", err, "fix", engineLogFix)
	}
}

// reportOutcome is the attribute a problem's Warn line carries about its
// raise: that the reporter queued it, or why it did not, so a raise that
// reached no report bus is on the line the operator reads rather than dropped.
func reportOutcome(err error) slog.Attr {
	if err != nil {
		return slog.Any("report-error", err)
	}
	return slog.String("report", "queued")
}

// sendKind names the kind of packet a refused send carried. The zero value
// names no kind: each send site passes its own named kind.
type sendKind uint8

const (
	sendKindUnspecified sendKind = iota
	sendKindControl
	sendKindEcho
	sendKindEchoReflect
)

// sendKinds holds, per sendKind, the packet label of the log line, of the
// report subject and of ze_bfd_send_failures_total, and the problem the
// operator reads: what was not sent, and what that does to the session. The
// unspecified row is what a send site that names no kind, a ze defect, would
// report.
var sendKinds = [...]struct{ label, problem string }{
	sendKindUnspecified: {
		label:   "unspecified",
		problem: "bfd could not send a packet of a kind it does not name, a ze defect",
	},
	sendKindControl: {
		label: "control",
		problem: "bfd could not send a Control packet to the peer, so the peer takes the session Down " +
			"when its Detection Time passes",
	},
	sendKindEcho: {
		label: "echo",
		problem: "bfd could not send an Echo packet to the peer, so no Echo packet returns and the session " +
			"goes Down with Echo Function Failed",
	},
	sendKindEchoReflect: {
		label: "echo-reflect",
		problem: "bfd could not return an Echo packet the peer sent, so the peer's echo function fails " +
			"and the peer takes the session Down with Echo Function Failed",
	},
}

// sendRecord is what a session keeps about the refused sends of one packet
// kind. Each time is the zero time when there is none.
type sendRecord struct {
	// warnAt is when a refused send was last logged at Warn and raised.
	warnAt time.Time
	// failedAt is when the last send was refused.
	failedAt time.Time
	// raisedAt is when the SendFailedCode warning was raised, the zero time
	// while none is raised.
	raisedAt time.Time
}

// warnSendFailedLocked reports a packet of kind the transport refused to send
// for entry. The transport refuses a single-hop packet whose interface no
// longer resolves or whose peer is on none of that interface's subnets (RFC
// 5881 Section 6), and the session then falls Down: without this report the
// operator sees the Down and not its cause. Every refusal is counted
// (OnSendFailure). The first of each kind, and the first after each
// sendWarnInterval, is logged at Warn and raised on the report bus under
// SendFailedCode, and the rest go to Debug, so a send that fails on every
// transmit interval gives one line and one raise a minute, never one per
// packet. A send of the same kind that lands clears the raise
// (clearSendWarning), and so does tick once no send of that kind has failed
// for warningQuiet (expireQuietWarnings). Caller MUST hold l.mu.
func (l *Loop) warnSendFailedLocked(entry *sessionEntry, kind sendKind, err error) {
	key := entry.machine.Key()
	label := sendKinds[kind].label
	if hook := l.metricsHook.Load(); hook != nil {
		(*hook).OnSendFailure(key.Mode.String(), label)
	}
	now := l.clk.Now()
	record := &entry.sends[kind]
	record.failedAt = now
	if !record.warnAt.IsZero() {
		if now.Sub(record.warnAt) < sendWarnInterval {
			engineLog().Debug("bfd send failed", "packet", label, "peer", key.Peer,
				"interface", key.Interface, "vrf", key.VRF, "mode", key.Mode.String(), "err", err)
			return
		}
	}
	record.warnAt = now
	problem := sendKinds[kind].problem
	fix := sendFailedFix(key)
	subject := sendSubject(entry, kind)
	raiseErr := entry.issues.raiseWarning(SendFailedCode, subject, problem+": "+err.Error(), fix,
		sendReportFailed(subject))
	if raiseErr == nil {
		record.raisedAt = now
	}
	engineLog().Warn(problem, "code", SendFailedCode, "packet", label, "peer", key.Peer,
		"interface", key.Interface, "vrf", key.VRF, "mode", key.Mode.String(),
		"local-discriminator", entry.machine.LocalDiscriminator(), "reason", err.Error(),
		"fix", fix, reportOutcome(raiseErr))
}

// clearSendWarning withdraws the warning warnSendFailedLocked raised for
// entry's packets of kind, when one is raised, and keeps the Warn window.
// Every send of kind that lands calls it, so the path with nothing raised is
// one check. Caller MUST hold l.mu of entry's loop.
func clearSendWarning(entry *sessionEntry, kind sendKind) {
	record := &entry.sends[kind]
	if record.raisedAt.IsZero() {
		return
	}
	record.raisedAt = time.Time{}
	subject := sendSubject(entry, kind)
	if err := entry.issues.clearWarning(SendFailedCode, subject, sendReportFailed(subject)); err != nil {
		engineLog().Warn("bfd could not clear the send warning of a session, so show warnings may still list it",
			"code", SendFailedCode, "subject", subject, "error", err, "fix", engineLogFix)
	}
}

// clearSendWarnings withdraws every send warning of entry, for a session that
// is removed (removeEntryLocked, Loop.Stop). Caller MUST hold l.mu of
// entry's loop.
func clearSendWarnings(entry *sessionEntry) {
	for kind := range sendKind(len(sendKinds)) {
		clearSendWarning(entry, kind)
	}
}

// expireQuietWarnings withdraws entry's raised warnings whose failures have
// stopped: no packet failed authentication, or no send of that kind was
// refused, for warningQuiet. A failure that goes on refreshes its time on
// every packet, so only a failure that ended is withdrawn: a peer that failed
// authentication and then fell silent, a session whose echo stopped when it
// left Up, a peer that stopped sending the echoes a refused reflection
// answered. tick calls it for every session on each pass, so the path with
// nothing raised is one check per warning. Caller MUST hold l.mu of entry's
// loop.
func expireQuietWarnings(entry *sessionEntry, now time.Time) {
	if !entry.authRaisedAt.IsZero() {
		if now.Sub(entry.authFailedAt) >= warningQuiet {
			clearAuthWarning(entry)
		}
	}
	for kind := range sendKind(len(sendKinds)) {
		record := &entry.sends[kind]
		if record.raisedAt.IsZero() {
			continue
		}
		if now.Sub(record.failedAt) >= warningQuiet {
			clearSendWarning(entry, kind)
		}
	}
}

// sendFailedFix is the command that shows why the transport refuses the
// packets of the session key: for a session bound to an interface, that
// interface's addresses, which the transport matches the peer against;
// otherwise the session itself.
func sendFailedFix(key api.Key) string {
	var fix textbuf.Buffer
	if key.Interface != "" {
		fix.Reset().Str("ze show interface name ").Str(key.Interface).Str(" detail")
		return fix.String()
	}
	fix.Reset().Str("ze show bfd session address ").Addr(key.Peer)
	return fix.String()
}

// sendSubject names entry's refused sends of kind on the report bus: the
// session (sessionSubject) and the packet label, so each kind is raised and
// cleared on its own.
func sendSubject(entry *sessionEntry, kind sendKind) string {
	return sessionSubject(entry) + " packet " + sendKinds[kind].label
}

// sendReportFailed is the failed callback of a SendFailedCode raise or clear:
// the daemon did not take it, so the report bus is wrong about the session
// and only the log holds what happened.
func sendReportFailed(subject string) func(error) {
	return func(err error) {
		engineLog().Warn("bfd could not update the send warning of a session on the report bus, so show warnings may be wrong about it",
			"code", SendFailedCode, "subject", subject, "error", err, "fix", engineLogFix)
	}
}
