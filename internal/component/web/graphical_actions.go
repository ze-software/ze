// Design: docs/architecture/web-components.md -- graphical configuration actions.
// Related: cli_terminal.go -- shared authorization, audit and execution.

package web

import (
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/opererr"
)

// GraphicalRevision is a revision the graphical history can restore. The
// number is only a display position; Key pins the actual saved object.
type GraphicalRevision struct {
	Number    int
	Timestamp string
	Key       string
}

// GraphicalActions carries the selected schema path and its available tools.
// It belongs to one response and is never shared between users.
type GraphicalActions struct {
	Supplied    *GraphicalSuppliedPanel
	Path        string
	HasNode     bool
	HasList     bool // A configured whole list can be deleted but is not an activation target.
	HasEntry    bool
	Inactive    bool
	Output      string
	History     []GraphicalRevision
	Revisions   []GraphicalRevision
	Sessions    []GraphicalSession
	Users       []string
	Options     []contract.EditorOption
	Window      string
	WindowOwner bool
}

// HandleGraphicalAction adapts button forms to the same editor executor the
// browser CLI uses. The caller MUST apply authentication and same-origin
// middleware. Each mutation is authorized before the shared producer runs.
func HandleGraphicalAction(mgr *EditorManager, schema *config.Schema, renderer *Renderer, authorizer aaa.Authorizer, recorder audit.Recorder, broker *EventBroker, suppliedTrees ...AdminCommandTree) http.HandlerFunc {
	var suppliedTree AdminCommandTree
	if len(suppliedTrees) > 0 {
		suppliedTree = suppliedTrees[0]
	}
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, opererr.New("Sign in before using configuration actions.", "No authenticated browser session is available for this action.", "Open /login and sign in, then reopen Configuration actions").Error(), http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, opererr.New("The configuration action was not submitted.", "This endpoint accepts only the form sent by its action button.", "Open Configuration actions and press the action button").Error(), http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		if err := r.ParseForm(); err != nil {
			http.Error(w, opererr.New("The configuration action form cannot be read.", "The action was not run because the submitted form is malformed or too large.", "Reload the YANG editor, reopen Configuration actions and submit the form again").Error(), http.StatusBadRequest)
			return
		}
		path, command, responsePath, err := graphicalActionCommand(schema, r)
		if err != nil {
			var refusal *opererr.Error
			if existing, ok := errors.AsType[*opererr.Error](err); ok {
				refusal = existing
			} else {
				refusal = opererr.Wrap(err, "The configuration action cannot use the selected item.", "The selected path or action does not match the current configuration form.", "Reload the YANG editor, select the item and submit its Configuration action again")
			}
			http.Error(w, refusal.Error(), http.StatusBadRequest)
			return
		}
		if permission := terminalAuthCommand(command); permission != "" {
			if !authorizeWebConfigMutation(w, r, authorizer, username, permission) {
				return
			}
		}
		if _, err := mgr.GetOrCreate(username); err != nil {
			var refusal *opererr.Error
			if existing, ok := errors.AsType[*opererr.Error](err); ok {
				refusal = existing
			} else {
				refusal = opererr.Wrap(err, "Your candidate configuration cannot be opened.", "The action was not run because your saved changes could not be read.", "Reload the YANG editor and retry the action; if it still fails, ask an administrator to check the configuration store")
			}
			http.Error(w, refusal.Error(), http.StatusInternalServerError)
			return
		}
		line := command.words.String()
		action, detail, err := terminalAuditContext(mgr, username, command, line)
		if err != nil {
			http.Error(w, editorSessionRefusal(err).Error(), http.StatusInternalServerError)
			return
		}
		var result terminalCommandResult
		var revisions []GraphicalRevision
		switch command.Verb {
		case verbShow:
			if graphicalPinnedCompare(r) {
				result.output, err = mgr.compareGraphicalRevision(username, path, r)
				if err != nil {
					result.output = "error: " + err.Error()
				}
			} else {
				result = executeTerminalCommand(r.Context(), schema, mgr.Tree(username), mgr, username, r.RemoteAddr, path, terminalModeConfig, command, line, nil)
			}
		case verbWho, verbOption:
			result = executeTerminalCommand(r.Context(), schema, mgr.Tree(username), mgr, username, r.RemoteAddr, path, terminalModeConfig, command, line, nil)
		case verbRollback:
			result.output, err = mgr.rollbackGraphicalRevision(username, r.FormValue("revision"))
			if err != nil {
				result.output = "error: " + err.Error()
			} else {
				result.auditSuccess = true
			}
		case verbHistory:
			var backups []contract.BackupInfo
			backups, err = mgr.ListBackups(username)
			if err != nil {
				result.output = "error: " + err.Error()
			} else {
				result.output = "No saved revisions are available."
				if len(backups) > 0 {
					result.output = "Select a saved revision to restore to the candidate."
				}
				for i, backup := range backups {
					revisions = append(revisions, GraphicalRevision{Number: i + 1, Timestamp: backup.Timestamp, Key: backup.Path})
				}
			}
		default:
			result = executeTerminalCommand(r.Context(), schema, mgr.Tree(username), mgr, username, r.RemoteAddr, nil, terminalModeConfig, command, line, nil)
		}
		if terminalAuditSucceeded(command, result) {
			if command.Verb == verbDiscard {
				detail = discardAuditDetail(mgr, username, detail)
			}
			recordWebAudit(recorder, r, username, action, detail)
			if result.auditSuccess {
				BroadcastConfigChange(broker, username, "graphical configuration action")
			}
		}
		status := http.StatusOK
		if terminalCommandFailed(result.output) {
			status = http.StatusBadRequest
			responsePath = path
		} else if r.FormValue("action") == "discard-path" {
			responsePath = graphicalSurvivingPath(schema, mgr.Tree(username), responsePath)
		}
		if renderer == nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(status)
			if _, err := w.Write([]byte(result.output)); err != nil {
				return
			}
			return
		}
		data := buildFragmentData(schema, mgr.Tree(username), responsePath)
		data.Username = username
		data.ReadOnly = !canEdit(r, authorizer)
		data.Actions = mgr.graphicalActions(schema, mgr.Tree(username), responsePath, username)
		data.Actions.Supplied = buildGraphicalSuppliedPanel(suppliedTree)
		if data.Actions.Output != "" {
			result.output += "\n" + data.Actions.Output
		}
		data.Actions.Output = result.output
		if revisions != nil {
			data.Actions.History = revisions
		}
		if status == http.StatusOK {
			where := showPathPrefix
			if len(responsePath) > 0 {
				where += encodeConfigPath(responsePath) + "/"
			}
			w.Header().Set("HX-Push-Url", where)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		if _, err := w.Write([]byte(renderer.renderComponent("graphical action response", oobResponse(data)))); err != nil {
			return
		}
		if _, err := w.Write([]byte(renderer.renderComponent("graphical action commit state", oobSaveOK(saveOK(r, authorizer, mgr.ChangeCount(username)))))); err != nil {
			return
		}
	}
}

func graphicalActionCommand(schema *config.Schema, r *http.Request) (path []string, command cliCommand, responsePath []string, err error) {
	where := r.FormValue("path")
	if len(where) > maxCommandLength {
		return nil, command, nil, graphicalActionRefusal("The selected path is too long.")
	}
	path, err = decodeConfigPath(where)
	if err != nil {
		return nil, command, nil, graphicalActionRefusal("The selected path is invalid.")
	}
	responsePath = path
	operation := r.FormValue("action")
	var words []string
	switch operation {
	case verbCopy, verbRename:
		target, err := config.ResolveListEntryTarget(schema, operation, path)
		if err != nil {
			return nil, command, nil, err
		}
		if target.End != len(path) {
			return nil, command, nil, graphicalActionRefusal("Select a named list entry before copying or renaming it.")
		}
		destination := r.FormValue("target")
		if destination == "" {
			return nil, command, nil, graphicalActionRefusal("Enter the new entry name.")
		}
		if len(destination) > 512 {
			return nil, command, nil, graphicalActionRefusal("The new entry name is too long.")
		}
		words = slices.Concat([]string{operation}, path, []string{"to", destination})
		responsePath = slices.Concat(target.ParentPath, []string{target.ListName})
	case verbActivate, verbDeactivate, verbDelete:
		if len(path) == 0 {
			return nil, command, nil, graphicalActionRefusal("Select a configuration item first.")
		}
		words = slices.Concat([]string{operation}, path)
		if operation == verbDelete {
			responsePath = path[:len(path)-1]
			if target, ok, err := config.WalkListEntryPath(schema, path); err == nil && ok {
				if target.End == len(path) {
					responsePath = slices.Concat(target.ParentPath, []string{target.ListName})
				}
			}
		}
	case verbSave, verbHistory:
		words = []string{operation}
	case verbRollback:
		words = []string{verbRollback, r.FormValue("revision")}
		responsePath = nil
	case "verify", "now", "accept", "abort":
		words = []string{verbCommit, operation}
		if operation == "abort" {
			responsePath = nil
		}
	case "confirmed":
		words = []string{verbCommit, operation, r.FormValue("seconds")}
	case verbDiscard:
		words = []string{verbDiscard, discardAll}
		responsePath = nil
	case "discard-path":
		if len(path) == 0 {
			return nil, command, nil, graphicalActionRefusal("Select the configuration item whose pending changes you want to discard.")
		}
		words = slices.Concat([]string{verbDiscard}, path)
	default:
		var inspection bool
		words, inspection, err = graphicalInspectionWords(r)
		if err != nil {
			return nil, command, nil, err
		}
		if !inspection {
			return nil, command, nil, graphicalActionRefusal("This configuration action is unknown.")
		}
	}
	switch operation {
	case "now", "confirmed":
		if selected := r.FormValue("force"); selected != "" {
			force, err := strconv.ParseBool(selected)
			if err != nil {
				return nil, command, nil, graphicalActionRefusal("The override selection is invalid.")
			}
			if force {
				words = append(words, contract.CommitForce)
			}
		}
	}
	line := graphicalCommandLine(words)
	if len(line) > maxCommandLength {
		return nil, command, nil, graphicalActionRefusal("The configuration action is too long.")
	}
	command = parseCLICommand(line)
	if command.parseErr != nil {
		return nil, command, nil, command.parseErr
	}
	return path, command, responsePath, nil
}

func buildGraphicalActions(schema *config.Schema, tree *config.Tree, path []string) *GraphicalActions {
	v := &GraphicalActions{Path: encodeConfigPath(path)}
	if len(path) == 0 {
		return v
	}
	if target, ok, err := config.WalkListEntryPath(schema, path); err == nil {
		v.HasEntry = ok && target.End == len(path) && target.Keyless == ""
	}
	if node := walkTree(tree, schema, path); node != nil {
		v.HasNode = true
		v.Inactive = node.IsInactive()
	} else if parent := walkTree(tree, schema, path[:len(path)-1]); parent != nil {
		name := path[len(path)-1]
		_, v.HasNode = parent.Get(name)
		if len(parent.GetMultiValuesState(name)) > 0 {
			v.HasNode = true
		}
		v.Inactive = parent.IsLeafInactive(name)
		if v.Inactive {
			v.HasNode = true
		}
		v.HasList = len(parent.ListKeys(name)) > 0
	}
	return v
}

// graphicalSurvivingPath returns the parent collection of the first entry a
// path crosses that the discard removed. Prefix enumeration is bounded by the
// submitted path's token count; complete multi-key addresses remain one entry.
func graphicalSurvivingPath(schema *config.Schema, tree *config.Tree, path []string) []string {
	for end := 1; end <= len(path); end++ {
		prefix := path[:end]
		target, atEntry, err := config.WalkListEntryPath(schema, prefix)
		if err != nil {
			// A prefix may stop inside a multi-key address. The successful
			// shared discard already judged the complete submitted path.
			continue
		}
		if !atEntry {
			continue
		}
		if target.End != end {
			continue
		}
		if walkTree(tree, schema, prefix) != nil {
			continue
		}
		return slices.Concat(target.ParentPath, []string{target.ListName})
	}
	return path
}

func graphicalActionRefusal(problem string) error {
	return opererr.New(problem, candidateUnchangedWhy, "Reload the YANG editor and use the configuration action again")
}

func (m *EditorManager) graphicalActions(schema *config.Schema, tree *config.Tree, path []string, username string) *GraphicalActions {
	v := buildGraphicalActions(schema, tree, path)
	if err := m.populateGraphicalInspection(v, username); err != nil {
		v.Output = editorSessionRefusal(err).Error()
	}
	if window := m.daemonWindow(); window != nil {
		if state, open := window.Status(); open {
			v.Window = state.Line(username)
			v.WindowOwner = state.User == username
		}
	}
	return v
}

// rollbackGraphicalRevision accepts only an identifier in this configuration's
// history. The lock spans lookup and the existing editor rollback; a new
// revision cannot change the meaning of a previously rendered restore button.
func (m *EditorManager) rollbackGraphicalRevision(username, key string) (string, error) {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return "", err
	}
	us.mu.Lock()
	defer us.mu.Unlock()
	backups, err := us.editor.ListBackups()
	if err != nil {
		return "", err
	}
	for _, backup := range backups {
		if backup.Path != key {
			continue
		}
		if err := us.editor.Rollback(backup.Path); err != nil {
			return "", err
		}
		return "Restored the saved revision to the candidate. Review and commit to apply it.", nil
	}
	return "", graphicalActionRefusal("That saved revision is no longer available. Refresh history and choose a revision again.")
}
