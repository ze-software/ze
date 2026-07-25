// Design: docs/architecture/config/yang-config-design.md — session visibility commands
// Overview: model_commands.go — command dispatch

package cli

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errUsageDisconnectSessionId = errors.New("usage: disconnect <session-id>")
)

// cmdShowBlame displays blame-annotated configuration with per-line authorship.
func (m *Model) cmdShowBlame() (commandResult, error) { //nolint:unparam // dispatch table requires (commandResult, error)
	return commandResult{output: m.editor.BlameView()}, nil
}

// cmdShowChanges displays pending changes for the current session (default) or all sessions.
func (m *Model) cmdShowChanges(args []string) (commandResult, error) {
	showAll := len(args) > 0 && args[0] == cmdAll

	if showAll {
		return m.cmdShowChangesAll()
	}

	changes := m.editor.PendingChanges(m.editor.SessionID())
	if len(changes) == 0 {
		return commandResult{
			statusMessage: "No pending changes",
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	var tb5 textbuf.Buffer
	tb5.Int(int64(len(changes))).Str(" pending")
	if len(changes) == 1 {
		tb5.Str(" change")
	} else {
		tb5.Str(" changes")
	}
	msg := tb5.String()

	// Show tree with diff gutter, even if changes column is disabled.
	view := m.configViewAtPath(m.contextPath)
	view.forceChanges = true
	return commandResult{
		statusMessage: msg,
		configView:    view,
	}, nil
}

// formatChangeEntry writes a single change entry with appropriate marker and command.
func formatChangeEntry(b *textbuf.Buffer, change config.PendingChange) {
	switch change.Kind {
	case config.PendingChangeDelete:
		b.Str("  - delete ").Str(change.Path)
		if change.Member != "" {
			b.Byte(' ').Str(change.Member)
			b.Str("\n")
			return
		}
		b.Str("  (was: ").Str(change.Previous).Str(")\n")
	case config.PendingChangeRename:
		b.Str("  ~ rename ").Str(change.OldPath).Str(" to ").Str(change.NewPath).Byte('\n')
	case config.PendingChangeDeactivate:
		b.Str("  ~ deactivate ").Str(change.Path).Byte(' ').Str(change.Member).Byte('\n')
	case config.PendingChangeActivate:
		b.Str("  ~ activate ").Str(change.Path).Byte(' ').Str(change.Member).Byte('\n')
	default:
		marker := byte('+')
		annotation := "(new)"
		if change.Previous != "" {
			marker = '*'
			var tb textbuf.Buffer
			annotation = tb.Str("(was: ").Str(change.Previous).Byte(')').String()
		}
		b.Str("  ").Byte(marker).Str(" set ").Str(change.Path).Byte(' ').Str(change.Value).Str("  ").Str(annotation).Byte('\n')
	}
}

// cmdShowChangesAll displays pending changes summary grouped by session.
func (m *Model) cmdShowChangesAll() (commandResult, error) {
	sessions := m.editor.ActiveSessions()
	if len(sessions) == 0 {
		return commandResult{
			statusMessage: "No pending changes",
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	total := 0
	for _, sid := range sessions {
		total += len(m.editor.PendingChanges(sid))
	}
	var tb6 textbuf.Buffer
	tb6.Int(int64(total)).Str(" pending")
	if total == 1 {
		tb6.Str(" change")
	} else {
		tb6.Str(" changes")
	}
	tb6.Str(" across ").Int(int64(len(sessions))).Str(" sessions")
	msg := tb6.String()
	return commandResult{
		statusMessage: msg,
		configView:    m.configViewAtPath(m.contextPath),
	}, nil
}

// cmdWho lists active sessions with pending changes and change counts.
func (m *Model) cmdWho() (commandResult, error) {
	sessions := m.editor.ActiveSessions()
	if len(sessions) == 0 {
		return commandResult{output: "No active sessions."}, nil
	}

	var b textbuf.Buffer
	b.Str("Active editing sessions:\n")
	myID := m.editor.SessionID()
	for _, sid := range sessions {
		if sid == myID {
			b.Str("* ")
		} else {
			b.Str("  ")
		}
		changes := m.editor.PendingChanges(sid)
		changeWord := "changes"
		if len(changes) == 1 {
			changeWord = "change"
		}
		b.Str(sid).Str(" - ").Int(int64(len(changes))).Str(" pending ").Str(changeWord).Byte('\n')
	}
	return commandResult{output: b.String()}, nil
}

// cmdDisconnectSession removes another session's pending changes from the draft.
// Unrestricted for this spec -- any session can disconnect any other session.
// RBAC gating deferred to a future spec when ze gains a role/permission system.
func (m *Model) cmdDisconnectSession(args []string) (commandResult, error) {
	if len(args) == 0 {
		return commandResult{}, errUsageDisconnectSessionId
	}
	targetSession := args[0]
	if targetSession == m.editor.SessionID() {
		return commandResult{}, fmt.Errorf("cannot disconnect own session (use 'discard %s' instead)", cmdAll)
	}

	if err := m.editor.DisconnectSession(targetSession); err != nil {
		return commandResult{}, err
	}

	var tb7 textbuf.Buffer
	return commandResult{
		statusMessage: tb7.Str("Disconnected session: ").Str(targetSession).String(),
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}
