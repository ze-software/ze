// Design: docs/architecture/doctor-and-health-checks.md -- event codes
// Related: ensure.go -- wrapWithEnsureChain, whose failed rollback raises here
// Related: command_registry.go -- warnDeprecated, whose first use of an old name raises here
// Related: register.go -- serverEventCodes registers both codes for recent-events

package server

import (
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// commandEventSource is the bus source of the command dispatcher's events.
const commandEventSource = "command-dispatcher"

// carriedFixLead opens the delete command an ensure rollback event's message
// carries when the command is too long for the event's fix.
const carriedFixLead = "; delete it with `"

// ensureRollbackFailed reports a parent the ensure-exists chain created for
// a command that then failed, and could not delete again: the resource is
// left in place. The event names the parent path and its fix is the delete
// command with the same selectors, so `show doctor` says what to run.
func ensureRollbackFailed(ctx *CommandContext, step *EnsureStep, err error) {
	parent := ensureCommandLine(step.Path, step.ArgDefs, ctx)
	var head, fix textbuf.Buffer
	head.Str("a command under `").Str(parent).Str("` failed, and deleting what that command had just created failed too, ").
		Str("so it is left in place")
	if len(step.RollbackPath) > 0 {
		fix.Str("ze ").Str(ensureCommandLine(step.RollbackPath, step.ArgDefs, ctx))
	} else {
		fix.Str("run the delete command served by ").Str(step.RollbackWireMethod).Str(" for ").Str(parent)
	}
	// String empties a textbuf.Buffer, so the fix is read once and the log
	// line and the event share that one string. The handler's error, the
	// parent's selectors and the fix built from them can be of any length, so
	// each event field is fitted to the bus, which drops the whole event over
	// a field too long; the log line keeps them whole.
	fixLine := fix.String()
	logger().Warn("ensure-exists rollback failed", "code", diagnostic.CodeDoctorCommandEnsureRollbackFailed,
		"parent", parent, "fix", fixLine, "error", err)
	// A fix past the bus's fix bound is replaced by the errors list, which
	// is where the operator already reads this event, so the message carries
	// the whole command when it fits there. It goes ahead of the error, so
	// FitMessage cuts the error's tail and never the command.
	var message textbuf.Buffer
	headLine := head.String()
	message.Str(headLine)
	if len(fixLine) > report.FixMax && len(headLine)+len(fixLine)+len(carriedFixLead)+1 <= report.MessageMax {
		message.Str(carriedFixLead).Str(fixLine).Byte('`')
	}
	message.Str(": ").Err(err)
	report.RaiseErrorFix(commandEventSource, diagnostic.CodeDoctorCommandEnsureRollbackFailed, fitSubject(parent),
		report.FitMessage(message.String()), busFix(fixLine, fixShowErrors))
}

// ensureCommandLine writes a command path followed by the values the
// dispatcher bound for the ancestor's leaves, in their declared order.
func ensureCommandLine(path []string, defs []command.ArgDef, ctx *CommandContext) string {
	var line textbuf.Buffer
	line.Str(strings.Join(path, " "))
	for i := range defs {
		value := ctx.Selector(defs[i].Name())
		if value == "" {
			continue
		}
		line.Byte(' ').Str(value)
	}
	return line.String()
}

// commandPath answers the tokens of the node serving wireMethod, or nil. The
// walk is bounded by the depth of the YANG command tree, which the schema
// fixes at load; a wire method serves one node, so the first match is it.
func commandPath(node *command.Node, wireMethod string) []string {
	for name, child := range node.Children {
		if child.WireMethod == wireMethod {
			return []string{name}
		}
		if rest := commandPath(child, wireMethod); rest != nil {
			return append([]string{name}, rest...)
		}
	}
	return nil
}

// deprecatedUsed logs and raises the first use of a deprecated command name
// in this session; the caller keeps the once-per-name record. The fix is the
// name to use instead. The names come from command registrations, so each
// event field is fitted to the bus, as ensureRollbackFailed fits its own.
func deprecatedUsed(oldName, newName string) {
	var message textbuf.Buffer
	message.Str("a client ran the deprecated command `").Str(oldName).
		Str("`, which a later release removes; scripts using it must move to `").Str(newName).Str("`")
	logger().Warn("deprecated command used", "code", diagnostic.CodeDoctorCommandDeprecatedUsed,
		"old", oldName, "new", newName, "hint", "use '"+newName+"' instead; old form will be removed in a future release")
	report.RaiseWarningFix(commandEventSource, diagnostic.CodeDoctorCommandDeprecatedUsed, fitSubject(oldName),
		report.FitMessage(message.String()), busFix(newName, fixShowWarnings))
}
