// Design: docs/architecture/web-components.md -- shared graphical inspection.
// Related: graphical_actions.go -- authenticated form execution and rendering.

package web

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/core/configtext"
)

// GraphicalSession carries the editor's exact identifier, never a display line
// parsed back into an action target. Own sessions use Discard instead.
type GraphicalSession struct {
	ID      string
	Changes int
	Own     bool
}

// graphicalInspectionWords admits only structured display choices. The
// strings here are command/form boundary tokens, not an internal state machine.
func graphicalInspectionWords(r *http.Request) ([]string, bool, error) {
	switch r.FormValue("action") {
	case "inspect":
		words := []string{verbShow}
		switch r.FormValue("source") {
		case "candidate":
		case configWordConfirmed, "saved":
			words = append(words, r.FormValue("source"))
		default:
			return nil, true, graphicalActionRefusal("Select candidate, running configuration or saved draft before inspecting it.")
		}
		format, err := graphicalInspectionFormat(r)
		return append(words, "|", "format", format), true, err
	case "inspect-metadata":
		switch r.FormValue("source") {
		case "", "candidate":
		default:
			return nil, true, graphicalActionRefusal("Metadata inspection applies only to the candidate configuration.")
		}
		words := []string{verbShow, "|"}
		switch view := r.FormValue("view"); view {
		case "blame", "active", "inactive", configWordChanges, verbErrors:
			return append(words, view), true, nil
		case "allchanges":
			return append(words, configWordChanges, "all"), true, nil
		default:
			return nil, true, graphicalActionRefusal("Select a candidate metadata view before inspecting it.")
		}
	case verbCompare:
		target := r.FormValue("target")
		switch target {
		case configWordConfirmed, "saved":
		case configWordRevision, configWordUser:
			// The owning manager resolves these under the execution lock.
		default:
			return nil, true, graphicalActionRefusal("Select a supported comparison target.")
		}
		format, err := graphicalInspectionFormat(r)
		return []string{verbShow, "|", verbCompare, target, "|", "format", format}, true, err
	case configWordSessions:
		return []string{verbWho}, true, nil
	case verbDisconnect:
		session := r.FormValue("session")
		if session == "" {
			return nil, true, graphicalActionRefusal("Choose an editing session before disconnecting it.")
		}
		return []string{verbDisconnect, session}, true, nil
	case "preference":
		for _, option := range contract.EditorOptions() {
			if option.Name != r.FormValue("option") {
				continue
			}
			value := r.FormValue("value")
			if !slices.Contains(option.Values, value) {
				return nil, true, graphicalActionRefusal("Choose one of this display preference's available settings.")
			}
			words := []string{verbOption, option.Name}
			if value != "" {
				words = append(words, value)
			}
			return words, true, nil
		}
		return nil, true, graphicalActionRefusal("Choose a supported display preference.")
	default:
		// Existing editing actions are admitted by graphicalActionCommand.
		return nil, false, nil
	}
}

func graphicalInspectionFormat(r *http.Request) (string, error) {
	switch format := r.FormValue("format"); format {
	case "tree", "config":
		return format, nil
	default:
		return "", graphicalActionRefusal("Select tree or set-command presentation before inspecting configuration.")
	}
}

func graphicalPinnedCompare(r *http.Request) bool {
	if r.FormValue("action") != verbCompare {
		return false
	}
	switch r.FormValue("target") {
	case configWordRevision, configWordUser:
		return true
	default:
		return false
	}
}

// compareGraphicalRevision resolves a stable storage key and executes the
// shared compare while holding one user lock, so a moving history ordinal can
// never silently choose another saved configuration. User targets use exact
// metadata ownership, which does not depend on the spelling of a session ID.
func (m *EditorManager) compareGraphicalRevision(username string, path []string, r *http.Request) (string, error) {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return "", err
	}
	us.mu.Lock()
	defer us.mu.Unlock()
	var target string
	switch r.FormValue("target") {
	case configWordRevision:
		backups, err := us.editor.ListBackups()
		if err != nil {
			return "", err
		}
		for index, backup := range backups {
			if backup.Path == r.FormValue(configWordRevision) {
				target = "rollback " + strconv.Itoa(index+1)
				break
			}
		}
		if target == "" {
			return "", graphicalActionRefusal("That saved revision is no longer available. Refresh history and choose a revision again.")
		}
	case configWordUser:
		for _, change := range us.editor.PendingChanges("") {
			if change.User == r.FormValue("username") {
				target = configtext.QuoteCommandWord(change.User)
				break
			}
		}
		if target == "" {
			return "", graphicalActionRefusal("That user has no candidate metadata to compare. Refresh the candidate and choose a listed user.")
		}
	default:
		return "", graphicalActionRefusal("Select a saved revision or metadata owner to compare.")
	}
	answer, err := us.editor.ExecuteConfigCommand(contract.ConfigCommandRequest{
		Line: "show | compare " + target + " | format " + r.FormValue("format"),
		Path: path, ShowHints: us.showHints,
	})
	if err != nil {
		return "", err
	}
	us.showHints = answer.ShowHints
	return ansiStrip(answer.Output), nil
}

// populateGraphicalInspection snapshots available controls under the editor's
// lock. It MUST NOT call manager methods while holding that lock. Callers MUST
// display its error rather than presenting unavailable history as empty.
func (m *EditorManager) populateGraphicalInspection(v *GraphicalActions, username string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}
	us.mu.Lock()
	defer us.mu.Unlock()
	backups, err := us.editor.ListBackups()
	if err != nil {
		return err
	}
	for index, backup := range backups {
		v.Revisions = append(v.Revisions, GraphicalRevision{Number: index + 1, Timestamp: backup.Timestamp, Key: backup.Path})
	}
	for _, sid := range us.editor.ActiveSessions() {
		v.Sessions = append(v.Sessions, GraphicalSession{ID: sid, Changes: len(us.editor.PendingChanges(sid)), Own: sid == us.editor.SessionID()})
	}
	for _, change := range us.editor.PendingChanges("") {
		if change.User == "" {
			continue
		}
		if !slices.Contains(v.Users, change.User) {
			v.Users = append(v.Users, change.User)
		}
	}
	slices.Sort(v.Users)
	v.Options = contract.EditorOptions()
	return nil
}

// graphicalCommandLine preserves pipe operators as syntax while quoting every
// submitted word. Only show forms may contain operators; all their words come
// from the bounded catalog above. Session IDs and entry names stay quoted.
func graphicalCommandLine(words []string) string {
	var line strings.Builder
	for index, word := range words {
		if index > 0 {
			line.WriteByte(' ')
		}
		if words[0] == verbShow && word == "|" {
			line.WriteByte('|')
		} else {
			line.WriteString(configtext.QuoteCommandWord(word))
		}
	}
	return line.String()
}
