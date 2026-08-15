// Design: docs/architecture/web-interface.md -- CLI terminal mode and rendering helpers
// Related: cli.go -- command handlers and dispatch

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	terminalOutputCommitSuccessful = "commit successful"
	terminalOutputChangesDiscarded = "changes discarded"
	// terminalOutputNoChanges is what compare answers when the two texts match.
	// compareTreesAtPath reads it back, because a changed secret is invisible in
	// the text and still owes the operator a line.
	terminalOutputNoChanges = "(no changes)"
)

// terminalResponse is the JSON envelope returned by the terminal endpoint.
// The JS client updates the output viewport, message area, and prompt from
// these fields, matching the SSH CLI's fixed-zone layout.
type terminalResponse struct {
	Output   string `json:"output"`
	Feedback string `json:"feedback"`
	Path     string `json:"path"`
	Prompt   string `json:"prompt,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

// HandleCLITerminalWithDispatchAuthorizerAndAudit returns a POST handler for
// /cli/terminal that processes commands in terminal mode, supporting both config
// mode and operational mode. It returns a JSON response with structured output so
// the client can update the output viewport and message area separately, matching
// the SSH CLI's layout.
//
// The committed tree is used for show output so the CLI displays the same
// config the workbench shows (the daemon's running config, not just the
// editor's on-disk file).
//
// authorizer enforces profile-based RBAC before direct editor mutations and
// recorder records successful commit/discard/rollback actions; either may be nil.
func HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr *EditorManager, schema *config.Schema, tree *config.Tree, dispatch CommandDispatcher, authorizer aaa.Authorizer, recorder audit.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if _, err := mgr.GetOrCreate(username); err != nil {
			http.Error(w, fmt.Sprintf("session: %v", err), http.StatusInternalServerError)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 65536)

		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
			return
		}

		commandText := r.FormValue("command")
		if len(commandText) > maxCommandLength {
			http.Error(w, "command too long", http.StatusBadRequest)
			return
		}

		pathStr := r.FormValue("path")
		var contextPath []string
		if pathStr != "" {
			contextPath = strings.Split(pathStr, "/")
		}

		if err := ValidatePathSegments(contextPath); err != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		mode := normalizeTerminalMode(r.FormValue("mode"))
		cmd := parseCLICommand(commandText)
		if authCommand := terminalAuthCommandForMode(mode, cmd); authCommand != "" {
			if !authorizeWebConfigMutation(w, r, authorizer, username, authCommand) {
				return
			}
		}
		auditAction, auditDetail := terminalAuditContext(mgr, username, cmd, commandText)

		// Keep the committed hub tree as the compare baseline. Display commands
		// use the per-user working tree when one exists.
		viewTree := tree

		result := executeTerminalCommand(schema, viewTree, mgr, username, r.RemoteAddr, contextPath, mode, cmd, commandText, dispatch)
		if terminalAuditSucceeded(auditAction, result.output) {
			recordWebAudit(recorder, r, username, auditAction, auditDetail)
		}

		resp := terminalResponse{
			Output:   result.output,
			Feedback: terminalFeedback(cmd, result.output),
			Path:     textbuf.Join(result.path, "/"),
			Prompt:   formatTerminalPrompt(result.mode, result.path),
			Mode:     result.mode,
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, fmt.Sprintf("json encode: %v", err), http.StatusInternalServerError)
		}
	}
}

// terminalAuthCommand maps only state-changing verbs to RBAC commands.
// Navigation verbs like edit/up/top are intentionally ungated here: in the web
// CLI bar they only change the visible subtree, unlike the API's "config edit"
// session creation which opens a mutable config session.
func terminalAuthCommand(cmd cliCommand) string {
	switch cmd.Verb {
	case verbSet, verbActivate:
		return webCommandConfigSet
	case verbDelete, verbDeactivate:
		return webCommandConfigDelete
	case verbCommit:
		return webCommandConfigCommit
	case verbDiscard:
		return webCommandConfigDiscard
	case verbRollback:
		return webCommandConfigRollback
	case verbRename:
		return webCommandConfigRename
	case verbCopy, verbInsert:
		return webCommandConfigAdd
	case verbSave:
		return webCommandConfigSave
	}
	return ""
}

func terminalAuthCommandForMode(mode string, cmd cliCommand) string {
	if normalizeTerminalMode(mode) == terminalModeOperational && !terminalConfigCommandInOperationalMode(cmd.Verb) {
		return ""
	}
	return terminalAuthCommand(cmd)
}

type terminalCommandResult struct {
	path   []string
	mode   string
	output string
}

func executeTerminalCommand(schema *config.Schema, viewTree *config.Tree, mgr *EditorManager, username, remoteAddr string, contextPath []string, mode string, cmd cliCommand, raw string, dispatch CommandDispatcher) terminalCommandResult {
	result := terminalCommandResult{
		path: append([]string{}, contextPath...),
		mode: normalizeTerminalMode(mode),
	}

	if result.mode == terminalModeOperational {
		return executeTerminalOperationalMode(schema, viewTree, mgr, username, remoteAddr, result.path, cmd, raw, dispatch)
	}
	return executeTerminalConfigMode(schema, viewTree, mgr, username, remoteAddr, result.path, cmd, dispatch)
}

func executeTerminalConfigMode(schema *config.Schema, viewTree *config.Tree, mgr *EditorManager, username, remoteAddr string, contextPath []string, cmd cliCommand, dispatch CommandDispatcher) terminalCommandResult {
	result := terminalCommandResult{path: contextPath, mode: terminalModeConfig}
	switch cmd.Verb {
	case verbRun:
		if len(cmd.Args) == 0 {
			result.output = "usage: run <command>"
			return result
		}
		result.output = executeTerminalOperational(dispatch, username, remoteAddr, textbuf.Join(cmd.Args, " "))
		return result
	case verbExit:
		if mgr.ChangeCount(username) > 0 {
			result.output = "Pending changes. Use 'commit' or 'discard' before exit."
			return result
		}
		result.mode = terminalModeOperational
		return result
	case verbConfigure:
		result.output = "already in config mode"
		return result
	}

	newPath, output := executeTerminalNav(schema, viewTree, mgr, username, contextPath, cmd)
	result.output = output
	if newPath != nil {
		result.path = append([]string{}, newPath...)
		if cmd.Verb == verbEdit || cmd.Verb == verbUp || cmd.Verb == verbTop {
			result.output = serializeTreeAtPath(displayTree(viewTree, mgr, username), schema, newPath)
		}
	}
	return result
}

func executeTerminalOperationalMode(schema *config.Schema, viewTree *config.Tree, mgr *EditorManager, username, remoteAddr string, contextPath []string, cmd cliCommand, raw string, dispatch CommandDispatcher) terminalCommandResult {
	result := terminalCommandResult{path: contextPath, mode: terminalModeOperational}
	switch cmd.Verb {
	case verbConfigure:
		result.mode = terminalModeConfig
		result.output = serializeTreeAtPath(displayTree(viewTree, mgr, username), schema, contextPath)
		return result
	case verbExit, verbQuit:
		result.output = "exit is not available in the web CLI; use Workbench or Finder to leave CLI."
		return result
	case verbHelp:
		result.output = terminalOperationalHelp()
		return result
	}
	if terminalConfigCommandInOperationalMode(cmd.Verb) {
		return executeTerminalConfigMode(schema, viewTree, mgr, username, remoteAddr, contextPath, cmd, dispatch)
	}
	result.output = executeTerminalOperational(dispatch, username, remoteAddr, raw)
	return result
}

func terminalConfigCommandInOperationalMode(verb string) bool {
	switch verb {
	case verbSet, verbDelete, verbEdit, verbTop, verbUp, verbCommit, verbDiscard,
		verbCompare, verbSave, verbHistory, verbRollback, verbRename, verbCopy,
		verbInsert, verbDeactivate, verbActivate:
		return true
	default:
		return false
	}
}

func executeTerminalOperational(dispatch CommandDispatcher, username, remoteAddr, input string) string {
	if dispatch == nil {
		return "error: operational command dispatch not available"
	}
	// No session format override: the web terminal has no `set cli format` surface
	// (that command is dispatched only by the SSH/TUI model), so it always uses the
	// configured default.
	cmdStr, formatFn, pipeErr := command.ProcessPipesDefaultFormatChecked(input, "")
	if pipeErr != "" {
		var tb textbuf.Buffer
		return tb.Str("pipe error: ").Str(pipeErr).String()
	}
	output, err := dispatch.JSON(context.Background(), plugin.CallerIdentity{Username: username, RemoteAddr: remoteAddr}, cmdStr)
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str("error: ").Err(err).String()
	}
	return formatFn(output)
}

func terminalOperationalHelp() string {
	return `commands:
  configure            Enter config mode
  <command>            Execute operational command
  <command> | json     Use pipe operators
  exit                 Stay on the web CLI page`
}

func terminalAuditContext(mgr *EditorManager, username string, cmd cliCommand, raw string) (string, string) {
	switch cmd.Verb {
	case verbCommit:
		detail, _ := mgr.Diff(username)
		return audit.ActionConfigCommit, detail
	case verbDiscard:
		detail, _ := mgr.Diff(username)
		return audit.ActionConfigDiscard, detail
	case verbRollback:
		return audit.ActionConfigDiscard, raw
	}
	return "", ""
}

func terminalAuditSucceeded(action, output string) bool {
	switch action {
	case audit.ActionConfigCommit:
		return output == terminalOutputCommitSuccessful
	case audit.ActionConfigDiscard:
		return output == terminalOutputChangesDiscarded || strings.HasPrefix(output, "rolled back to ")
	}
	return false
}

// terminalFeedback returns the message-area feedback line for a command.
func terminalFeedback(cmd cliCommand, output string) string {
	var tb textbuf.Buffer
	switch cmd.Verb {
	case verbSet:
		if len(cmd.Args) >= 2 { //nolint:mnd // set <key> <value>
			return tb.Str("set ").Str(cmd.Args[0]).Byte(' ').Join(cmd.Args[1:], " ").String()
		}
	case verbDelete:
		if len(cmd.Args) >= 1 {
			return tb.Str("deleted ").Str(cmd.Args[0]).String()
		}
	case verbCommit:
		if strings.HasPrefix(output, "error") || strings.HasPrefix(output, "commit conflicts") {
			return output
		}
		return terminalOutputCommitSuccessful
	case verbDiscard:
		return terminalOutputChangesDiscarded
	case verbEdit:
		return tb.Str("edit ").Join(cmd.Args, " ").String()
	case verbUp:
		return "up"
	case verbTop:
		return "top"
	}
	return ""
}

// executeTerminalNav runs a CLI command and returns the new context path
// (nil if unchanged) and text output. Navigation commands (edit, up, top)
// return the updated path; other commands return nil.
// Pipe filters (| format config, | match, | head, | tail) are handled
// using the same cli.ApplyPipeFilter as the SSH CLI.
func executeTerminalNav(schema *config.Schema, viewTree *config.Tree, mgr *EditorManager, username string, contextPath []string, cmd cliCommand) (newPath []string, output string) {
	if !knownCLIVerbs[cmd.Verb] && cmd.Verb != "" {
		var tb textbuf.Buffer
		return nil, tb.Str("unknown command: ").Str(cmd.Verb).String()
	}

	// Check for pipe in args: split into command args and pipe filters.
	allTokens := append([]string{cmd.Verb}, cmd.Args...)
	if pipeIdx := cli.FindPipeIndex(allTokens); pipeIdx > 0 {
		baseCmd := cliCommand{Verb: allTokens[0], Args: allTokens[1:pipeIdx]}
		filters := cli.ParsePipeFilters(allTokens[pipeIdx+1:])
		opts, classErr := cli.ClassifyShowPipes(filters)
		if classErr != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("pipe error: ").Err(classErr).String()
		}

		newPath, output = executeTerminalNav(schema, viewTree, mgr, username, contextPath, baseCmd)

		if opts.Format == cli.FmtConfig {
			showPath := contextPath
			if baseCmd.Verb == verbShow && len(baseCmd.Args) > 0 {
				showPath = append(append([]string{}, contextPath...), baseCmd.Args...)
			}
			output = serializeSetAtPath(displayTree(viewTree, mgr, username), schema, showPath)
		}
		if opts.CompareTarget != "" {
			showPath := contextPath
			if baseCmd.Verb == verbShow && len(baseCmd.Args) > 0 {
				showPath = append(append([]string{}, contextPath...), baseCmd.Args...)
			}
			output = compareTargetAtPath(viewTree, mgr, username, schema, showPath, opts.CompareTarget, opts.Format)
		}

		for _, f := range opts.TextFilters {
			filtered, err := cli.ApplyPipeFilter(output, f)
			if err != nil {
				var tb textbuf.Buffer
				return newPath, tb.Str("pipe error: ").Err(err).String()
			}
			output = filtered
		}
		return newPath, output
	}

	switch cmd.Verb {
	case verbEdit:
		target := append(append([]string{}, contextPath...), cmd.Args...)
		if len(target) > 0 {
			if _, err := walkSchema(schema, target); err != nil {
				var tb textbuf.Buffer
				return nil, tb.Str("invalid path: ").Err(err).String()
			}
		}
		return target, ""
	case verbUp:
		if len(contextPath) > 0 {
			return contextPath[:len(contextPath)-1], ""
		}
		return []string{}, ""
	case verbTop:
		return []string{}, ""
	case verbShow:
		showPath := append(append([]string{}, contextPath...), cmd.Args...)
		return nil, serializeTreeAtPath(displayTree(viewTree, mgr, username), schema, showPath)
	case verbSet:
		return nil, executeTerminalSet(mgr, username, contextPath, cmd.Args)
	case verbDelete:
		return nil, executeTerminalDelete(mgr, username, contextPath, cmd.Args)
	case verbCommit:
		return nil, executeTerminalCommit(mgr, username)
	case verbDiscard:
		if err := mgr.Discard(username); err != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("error: ").Err(err).String()
		}
		return nil, terminalOutputChangesDiscarded
	case verbWho:
		sessions := mgr.ActiveSessions()
		if len(sessions) == 0 {
			return nil, "no active sessions"
		}
		var buf textbuf.Buffer
		buf.Str("active sessions:\n")
		for _, s := range sessions {
			buf.Str("  ").Str(s).Byte('\n')
		}
		return nil, buf.String()
	case verbCompare:
		target := cli.SrcConfirmed
		if len(cmd.Args) > 0 {
			target = textbuf.Join(cmd.Args, " ")
		}
		return nil, compareTargetAtPath(viewTree, mgr, username, schema, contextPath, target, cli.FmtTree)
	case verbSave:
		if err := mgr.SaveDraft(username); err != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("error: ").Err(err).String()
		}
		return nil, "changes saved to draft"
	case verbHistory:
		backups, err := mgr.ListBackups(username)
		if err != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("error: ").Err(err).String()
		}
		if len(backups) == 0 {
			return nil, "no backups found"
		}
		var buf textbuf.Buffer
		for i, b := range backups {
			buf.Int(int64(i + 1)).Str(". ").Str(b.Timestamp).Str("  ").Str(b.Path).Byte('\n')
		}
		return nil, buf.String()
	case verbRollback:
		if len(cmd.Args) != 1 {
			return nil, "usage: rollback <number>"
		}
		n, err := strconv.Atoi(cmd.Args[0])
		if err != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("invalid backup number: ").Str(cmd.Args[0]).String()
		}
		backups, bErr := mgr.ListBackups(username)
		if bErr != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("error: ").Err(bErr).String()
		}
		if n < 1 || n > len(backups) {
			var tb textbuf.Buffer
			return nil, tb.Str("backup ").Int(int64(n)).Str(" not found (have ").Int(int64(len(backups))).Str(" backups)").String()
		}
		if err := mgr.Rollback(username, backups[n-1].Path); err != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("error: ").Err(err).String()
		}
		var tb textbuf.Buffer
		return nil, tb.Str("rolled back to ").Str(backups[n-1].Path).String()
	case verbRename:
		return execListEntryOp(cmd.Args, contextPath, "rename", "<old-name>", func(parent []string, list, src, dst string) error {
			return mgr.RenameListEntry(username, parent, list, src, dst)
		})
	case verbCopy:
		return execListEntryOp(cmd.Args, contextPath, "copy", "<source>", func(parent []string, list, src, dst string) error {
			return mgr.CopyListEntry(username, parent, list, src, dst)
		})
	case verbInsert:
		if len(cmd.Args) < 1 {
			return nil, "usage: insert <path>"
		}
		insertPath := append(append([]string{}, contextPath...), cmd.Args...)
		if err := mgr.CreateEntry(username, insertPath); err != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("error: ").Err(err).String()
		}
		var tb textbuf.Buffer
		return nil, tb.Str("inserted entry at ").Join(cmd.Args, " ").String()
	case verbDeactivate:
		if len(cmd.Args) < 1 {
			return nil, "usage: deactivate <path>"
		}
		fullPath := append(append([]string{}, contextPath...), cmd.Args...)
		if err := mgr.DeactivatePath(username, fullPath); err != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("error: ").Err(err).String()
		}
		var tb textbuf.Buffer
		return nil, tb.Str("deactivated ").Join(cmd.Args, " ").String()
	case verbActivate:
		if len(cmd.Args) < 1 {
			return nil, "usage: activate <path>"
		}
		fullPath := append(append([]string{}, contextPath...), cmd.Args...)
		if err := mgr.ActivatePath(username, fullPath); err != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("error: ").Err(err).String()
		}
		var tb textbuf.Buffer
		return nil, tb.Str("activated ").Join(cmd.Args, " ").String()
	case verbErrors:
		// The masked pending-change diff, which is the one diff the web renders.
		// The editor's text diff carried the stored value of every ze:sensitive
		// leaf, and this verb needs no config authorization (secret.go).
		diff, err := mgr.Diff(username)
		if err != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("error: ").Err(err).String()
		}
		return nil, diff
	case verbDisconnect:
		if len(cmd.Args) < 1 {
			return nil, "usage: disconnect <session-id>"
		}
		if err := mgr.DisconnectSession(username, cmd.Args[0]); err != nil {
			var tb textbuf.Buffer
			return nil, tb.Str("error: ").Err(err).String()
		}
		var tb textbuf.Buffer
		return nil, tb.Str("disconnected session ").Str(cmd.Args[0]).String()
	case verbHelp:
		return nil, `commands:
  edit <path>          Enter a subsection context
  top                  Return to root context
  up                   Go up one level
  show [path]          Display configuration
  set <path> <value>   Set a configuration value
  delete <path>        Delete a configuration value
  rename <list> <old> to <new>   Rename a list entry
  copy <list> <src> to <dst>     Copy a list entry
  insert <path>        Insert a keyless list entry
  deactivate <path>    Mark a node inactive
  activate <path>      Re-activate a node
  compare              Show diff vs original
  commit               Save changes
  discard              Revert all changes
  save                 Save draft
  history              List backups
  rollback <N>         Restore backup N
  errors               Show validation errors
  who                  Show active sessions
  disconnect <id>      Disconnect a session
  help                 Show this help`
	case "":
		return nil, ""
	}

	return nil, ""
}

// compareTargetAtPath resolves the compare baseline and diffs it against the
// user's working tree at the selected path.
func compareTargetAtPath(committed *config.Tree, mgr *EditorManager, username string, schema *config.Schema, path []string, target, format string) string {
	baseline, err := compareBaselineTree(committed, mgr, username, schema, target)
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str("compare ").Str(compareTargetLabel(target)).Str(": ").Err(err).String()
	}
	return compareTreesAtPath(baseline, displayTree(committed, mgr, username), schema, path, format)
}

// compareTreesAtPath diffs two in-memory trees at a path.
// baseline is the selected reference tree, working is the editor's tree.
//
// Both texts are masked, so a rotated secret reads as the same placeholder on
// each side and the text diff cannot see it. changedSecretLines names those
// paths and publishes neither value.
func compareTreesAtPath(baseline, working *config.Tree, schema *config.Schema, path []string, format string) string {
	if baseline == nil || working == nil || schema == nil {
		return "(no configuration)"
	}
	baseText := serializeTreeAtPath(baseline, schema, path)
	workText := serializeTreeAtPath(working, schema, path)
	if format == cli.FmtConfig {
		baseText = serializeSetAtPath(baseline, schema, path)
		workText = serializeSetAtPath(working, schema, path)
	}
	diff := terminalOutputNoChanges
	if baseText != workText {
		diff = textDiff(baseText, workText)
	}
	secrets := changedSecretLines(baseline, working, schema, path)
	switch {
	case secrets == "":
		return diff
	case diff == terminalOutputNoChanges:
		return secrets
	default:
		return diff + secrets
	}
}

// changedSecretLines reports one line per secret leaf whose value differs
// between the two trees, under the compared path. Each line names the leaf and
// stands its value in as the display placeholder, which is the shape
// EditorManager.Diff writes for the same change. That is what the operator
// needs, and all they get. The compare verb needs no config authorization
// (secret.go).
func changedSecretLines(baseline, working *config.Tree, schema *config.Schema, path []string) string {
	var changed []string
	if len(path) == 0 {
		changed = config.ChangedSecretPaths(baseline, working, schema)
	} else {
		node, err := walkSchema(schema, path)
		if err != nil || node == nil {
			return ""
		}
		changed = config.ChangedSecretPathsSubtree(
			walkTree(baseline, schema, path), walkTree(working, schema, path), node)
	}

	var b textbuf.Buffer
	for _, leafPath := range changed {
		b.Str("~ ").Str(leafPath).Byte(' ').Str(config.SecretDataPlaceholder).Str(" (secret changed)").Byte('\n')
	}
	return b.String()
}

func compareBaselineTree(committed *config.Tree, mgr *EditorManager, username string, schema *config.Schema, target string) (*config.Tree, error) {
	switch cli.NormalizeCompareTarget(target) {
	case "", cli.SrcConfirmed:
		return committed, nil
	case cli.SrcSaved:
		return savedDraftTree(mgr, schema)
	case cli.CmpRollback:
		return rollbackTree(mgr, username, schema, target)
	default:
		return nil, fmt.Errorf("unsupported compare target %q", target)
	}
}

func compareTargetLabel(target string) string {
	if strings.TrimSpace(target) == "" {
		return cli.SrcConfirmed
	}
	return strings.TrimSpace(target)
}

func savedDraftTree(mgr *EditorManager, schema *config.Schema) (*config.Tree, error) {
	if mgr == nil {
		return nil, errNoEditorManager
	}
	data, err := mgr.store.ReadFile(cli.DraftPath(mgr.configPath))
	if err != nil {
		return nil, errNoSavedDraft
	}
	return parseConfigContent(string(data), schema)
}

func rollbackTree(mgr *EditorManager, username string, schema *config.Schema, target string) (*config.Tree, error) {
	if mgr == nil {
		return nil, errNoEditorManager
	}
	nText := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(target), "rollback "))
	n, err := strconv.Atoi(nText)
	if err != nil {
		return nil, fmt.Errorf("invalid rollback number: %s", nText)
	}
	if n < 1 {
		return nil, fmt.Errorf("rollback number must be >= 1, got %d", n)
	}
	backups, err := mgr.ListBackups(username)
	if err != nil {
		return nil, fmt.Errorf("cannot list backups: %w", err)
	}
	if n > len(backups) {
		return nil, fmt.Errorf("backup %d not found (have %d backups)", n, len(backups))
	}
	data, err := mgr.store.ReadFile(backups[n-1].Path)
	if err != nil {
		return nil, fmt.Errorf("cannot read backup %d: %w", n, err)
	}
	return parseConfigContent(string(data), schema)
}

func parseConfigContent(content string, schema *config.Schema) (*config.Tree, error) {
	switch config.DetectFormat(content) {
	case config.FormatSetMeta:
		tree, _, err := config.NewSetParser(schema).ParseWithMeta(content)
		return tree, err
	case config.FormatSet:
		return config.NewSetParser(schema).Parse(content)
	case config.FormatHierarchical:
		return config.NewParser(schema).Parse(content)
	default:
		return config.NewParser(schema).Parse(content)
	}
}

// textDiff produces an interleaved +/- line diff using LCS so that
// removals and additions appear next to each other.
func textDiff(original, modified string) string {
	origLines := nonEmptyLines(original)
	modLines := nonEmptyLines(modified)
	lcs := lcsLines(origLines, modLines)

	var buf textbuf.Buffer
	oi, mi, li := 0, 0, 0
	for li < len(lcs) {
		for oi < len(origLines) && origLines[oi] != lcs[li] {
			buf.Str("- ").Str(origLines[oi]).Byte('\n')
			oi++
		}
		for mi < len(modLines) && modLines[mi] != lcs[li] {
			buf.Str("+ ").Str(modLines[mi]).Byte('\n')
			mi++
		}
		oi++
		mi++
		li++
	}
	for oi < len(origLines) {
		buf.Str("- ").Str(origLines[oi]).Byte('\n')
		oi++
	}
	for mi < len(modLines) {
		buf.Str("+ ").Str(modLines[mi]).Byte('\n')
		mi++
	}
	if buf.Len() == 0 {
		return terminalOutputNoChanges
	}
	return buf.String()
}

func nonEmptyLines(s string) []string {
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func lcsLines(a, b []string) []string {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			switch {
			case a[i-1] == b[j-1]:
				dp[i][j] = dp[i-1][j-1] + 1
			case dp[i-1][j] >= dp[i][j-1]:
				dp[i][j] = dp[i-1][j]
			default:
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	result := make([]string, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		switch {
		case a[i-1] == b[j-1]:
			result = append(result, a[i-1])
			i--
			j--
		case dp[i-1][j] >= dp[i][j-1]:
			i--
		default:
			j--
		}
	}
	for l, r := 0, len(result)-1; l < r; l, r = l+1, r-1 {
		result[l], result[r] = result[r], result[l]
	}
	return result
}

func displayTree(fallback *config.Tree, mgr *EditorManager, username string) *config.Tree {
	if mgr != nil {
		if tree := mgr.Tree(username); tree != nil {
			return tree
		}
	}
	return fallback
}

// serializeTreeAtPath returns the config text at the given path. At root it
// serializes the full tree; at a subpath it walks to the subtree and serializes
// that section.
func serializeTreeAtPath(tree *config.Tree, schema *config.Schema, path []string) string {
	if tree == nil || schema == nil {
		return ""
	}
	// Mask every secret leaf for display (web CLI terminal show/nav/compare).
	// MaskSecrets clones, so the working/committed tree keeps the real value.
	// The verb that reaches here needs no config authorization, so this text
	// goes to any authenticated session (secret.go).
	tree = config.MaskSecrets(tree, schema)
	if len(path) == 0 {
		return config.Serialize(tree, schema)
	}
	subtree := walkTree(tree, schema, path)
	if subtree == nil {
		return ""
	}
	node, err := walkSchema(schema, path)
	if err != nil || node == nil {
		return ""
	}
	return config.SerializeSubtree(subtree, node)
}

func serializeSetAtPath(tree *config.Tree, schema *config.Schema, path []string) string {
	if tree == nil || schema == nil {
		return ""
	}
	// Mask every secret leaf for display; MaskSecrets clones the tree.
	tree = config.MaskSecrets(tree, schema)
	return config.FilterSetByPath(config.SerializeSet(tree, schema), path)
}

// executeTerminalSet handles the set command in terminal mode.
func executeTerminalSet(mgr *EditorManager, username string, contextPath, args []string) string {
	if len(args) < 2 { //nolint:mnd // set requires key and value
		return "error: usage: set <leaf> <value>"
	}

	var tb textbuf.Buffer
	if err := ValidatePathSegments([]string{args[0]}); err != nil {
		return tb.Str("error: invalid leaf name: ").Str(args[0]).String()
	}

	if err := mgr.SetValue(username, contextPath, args[0], textbuf.Join(args[1:], " ")); err != nil {
		return tb.Reset().Str("error: ").Err(err).String()
	}

	return tb.Reset().Str("set ").Str(args[0]).Byte(' ').Join(args[1:], " ").String()
}

// executeTerminalDelete handles the delete command in terminal mode.
// Schema-aware (mgr.DeleteByPath), so it deletes list entries and containers as
// well as leaves, matching the SSH CLI's `delete` (cli.Model.cmdDelete). The
// leaf-only DeleteValue silently no-ops on a list entry.
func executeTerminalDelete(mgr *EditorManager, username string, contextPath, args []string) string {
	if len(args) < 1 {
		return "error: usage: delete <name>"
	}

	var tb textbuf.Buffer
	if err := ValidatePathSegments([]string{args[0]}); err != nil {
		return tb.Str("error: invalid name: ").Str(args[0]).String()
	}

	if err := mgr.DeleteByPath(username, contextPath, args[0]); err != nil {
		return tb.Reset().Str("error: ").Str(args[0]).Str(": ").Err(err).String()
	}

	return tb.Reset().Str("deleted ").Str(args[0]).String()
}

// executeTerminalCommit handles the commit command in terminal mode.
func executeTerminalCommit(mgr *EditorManager, username string) string {
	result, err := mgr.Commit(username)
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str("error: ").Err(err).String()
	}

	if len(result.Conflicts) > 0 {
		var msg textbuf.Buffer
		msg.Str("commit conflicts:\n")

		for _, c := range result.Conflicts {
			msg.Str("  ").Str(c.Path).Str(": want ").Quoted(c.MyValue).Str(", other (").Str(c.OtherUser).Str(") has ").Quoted(c.OtherValue).Byte('\n')
		}

		return msg.String()
	}

	return terminalOutputCommitSuccessful
}

// HandleCLIModeToggle returns a POST handler for /cli/mode that toggles
// between integrated and terminal CLI modes. Returns the appropriate
// content area HTML for the target mode.
func HandleCLIModeToggle(mgr *EditorManager, schema *config.Schema, renderer *Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 65536)

		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
			return
		}

		mode := r.FormValue("mode")
		pathStr := r.FormValue("path")
		var contextPath []string
		if pathStr != "" {
			contextPath = strings.Split(pathStr, "/")
		}

		if err := ValidatePathSegments(contextPath); err != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		if mode == "terminal" {
			writeTerminalContent(w, contextPath)
			return
		}

		// Integrated mode: render normal GUI content.
		tree := mgr.Tree(username)
		viewData, err := buildConfigViewData(schema, tree, contextPath)
		if err != nil {
			serverLogger.Warn("mode toggle build view failed", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		writeCLIResponse(w, renderer, contextPath, viewData)
	}
}

// writeTerminalContent writes the terminal mode content HTML to w.
func writeTerminalContent(w http.ResponseWriter, contextPath []string) {
	prompt := formatCLIPrompt(contextPath)

	var buf textbuf.Buffer
	buf.Str(`<div class="terminal-container" id="content-area">`)
	buf.Str(`<div class="terminal-scrollback" id="terminal-scrollback"></div>`)
	buf.Str(`<div class="terminal-input-line">`)
	buf.Str(`<span class="terminal-prompt">`).Str(template.HTMLEscapeString(prompt)).Str(`</span>`)
	buf.Str(`<input type="text" class="terminal-input" id="terminal-input" `)
	buf.Str(`autocomplete="off" spellcheck="false" `)
	buf.Str(`hx-post="/cli/terminal" hx-trigger="keydown[key=='Enter']" `)
	buf.Str(`hx-target="#terminal-scrollback" hx-swap="beforeend" `)
	buf.Str(`hx-include="this" name="command">`)
	buf.Str(`</div>`)
	buf.Str(`</div>`)

	writeHTML(w, buf.String())
}

// writeCLIResponse writes an HTMX multi-target response with content area,
// breadcrumb OOB swap, CLI prompt OOB swap, path bar OOB swap, and context
// path OOB swap.
func writeCLIResponse(w http.ResponseWriter, renderer *Renderer, path []string, viewData *ConfigViewData) {
	crumbs := buildBreadcrumbs(path)
	prompt := formatCLIPrompt(path)

	var buf textbuf.Buffer

	// Main content area (must match layout.html element for outerHTML swap).
	buf.Str(`<main class="content-area" id="content-area">`)
	buildViewDataHTML(&buf, viewData)
	buf.Str(`</main>`)

	// OOB breadcrumb update.
	buildBreadcrumbOOB(&buf, crumbs)

	// OOB CLI prompt update.
	buildPromptOOB(&buf, prompt)

	// OOB path bar and context updates.
	buildPathBarOOB(&buf, path, renderer)
	buildContextOOB(&buf, path)
	writeHTML(w, buf.String())
}

// writeCLINotification writes only a notification OOB swap (for error responses).
func writeCLINotification(w http.ResponseWriter, message, notifType string) {
	var buf textbuf.Buffer
	buildNotificationOOB(&buf, message, notifType)
	writeHTML(w, buf.String())
}

// writeHTML writes an HTML string response to w with appropriate content type.
func writeHTML(w http.ResponseWriter, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if _, err := io.WriteString(w, html); err != nil {
		http.Error(w, fmt.Sprintf("write response: %v", err), http.StatusInternalServerError)
	}
}

// PathBarSegment holds one clickable segment in the CLI path bar.
type PathBarSegment struct {
	Name   string // Display name (e.g., "bgp", "peer", "127.0.0.1")
	URL    string // Navigation URL (e.g., "/show/bgp/")
	HxPath string // YANG path for hx-get (e.g., "bgp")
}

// buildPathBarSegments returns the pre-computed segments for the CLI path bar.
func buildPathBarSegments(path []string) []PathBarSegment {
	segments := make([]PathBarSegment, len(path))
	var tb textbuf.Buffer
	for i, seg := range path {
		joined := tb.Reset().Join(path[:i+1], "/").String()
		segments[i] = PathBarSegment{
			Name:   seg,
			URL:    tb.Reset().Str("/show/").Str(joined).Byte('/').String(),
			HxPath: joined,
		}
	}

	return segments
}

// buildPathBarOOB appends a CLI path bar OOB swap rendered by pathBarInner.
// Falls back to empty if renderer is nil.
func buildPathBarOOB(buf *textbuf.Buffer, path []string, renderer *Renderer) {
	buf.Str(`<div class="cli-path-bar" id="cli-path-bar" hx-swap-oob="innerHTML">`)
	if renderer != nil {
		data := &FragmentData{CLIPathSegments: buildPathBarSegments(path)}
		buf.Str(string(renderer.renderComponent("path_bar_inner", pathBarInner(data))))
	}
	buf.Str(`</div>`)
}

// buildContextOOB appends a hidden context path OOB swap element to buf.
func buildContextOOB(buf *textbuf.Buffer, path []string) {
	buf.Str(`<span id="cli-context-path" class="is-hidden" hx-swap-oob="true">`).Str(template.HTMLEscapeString(textbuf.Join(path, "/"))).Str(`</span>`)
}

// buildBreadcrumbOOB appends a breadcrumb OOB swap element to buf.
func buildBreadcrumbOOB(buf *textbuf.Buffer, crumbs []BreadcrumbSegment) {
	buf.Str(`<nav class="breadcrumb-bar" id="breadcrumb-bar" hx-swap-oob="innerHTML">`)
	buildBreadcrumbHTML(buf, crumbs)
	buf.Str(`</nav>`)
}

// buildPromptOOB appends a CLI prompt OOB swap element to buf.
func buildPromptOOB(buf *textbuf.Buffer, prompt string) {
	buf.Str(`<span class="cli-prompt" id="cli-prompt" hx-swap-oob="innerHTML">`).Str(template.HTMLEscapeString(prompt)).Str(`</span>`)
}

// buildNotificationOOB appends a notification OOB swap element to buf.
func buildNotificationOOB(buf *textbuf.Buffer, message, notifType string) {
	cssClass := "notification-info"
	if notifType == "error" {
		cssClass = "notification-error"
	}

	buf.Str(`<aside class="notification-bar `).Str(cssClass).Str(`" id="notification-bar" hx-swap-oob="true">`).Str(template.HTMLEscapeString(message)).Str(`</aside>`)
}

// buildViewDataHTML writes a simple HTML representation of ConfigViewData to buf.
func buildViewDataHTML(buf *textbuf.Buffer, data *ConfigViewData) {
	if data == nil {
		return
	}

	if len(data.Children) > 0 {
		buf.Str(`<ul class="config-children">`)

		for _, child := range data.Children {
			buf.Str(`<li><a href="`).Str(template.HTMLEscapeString(child.URL)).Str(`" class="config-link config-link-`).Str(template.HTMLEscapeString(child.Kind)).Str(`">`).Str(template.HTMLEscapeString(child.Name)).Str(`</a></li>`)
		}

		buf.Str(`</ul>`)
	}

	if len(data.Keys) > 0 {
		var tb textbuf.Buffer
		tb.Str("/show/")
		if len(data.Path) > 0 {
			tb.Join(data.Path, "/").Byte('/')
		}
		prefix := tb.String()

		buf.Str(`<ul class="config-keys">`)

		for _, key := range data.Keys {
			buf.Str(`<li><a href="`).Str(template.HTMLEscapeString(prefix)).Str(template.HTMLEscapeString(key)).Str(`/">`).Str(template.HTMLEscapeString(key)).Str(`</a></li>`)
		}

		buf.Str(`</ul>`)
	}

	if len(data.LeafFields) > 0 {
		buf.Str(`<table class="config-leaves"><thead><tr><th>Name</th><th>Value</th></tr></thead><tbody>`)

		var tb textbuf.Buffer
		for i := range data.LeafFields {
			f := &data.LeafFields[i]
			val := f.Value
			if val == "" && f.Default != "" {
				val = tb.Reset().Str(f.Default).Str(" (default)").String()
			}

			buf.Str(`<tr><td>`).Str(template.HTMLEscapeString(f.Name)).Str(`</td><td>`).Str(template.HTMLEscapeString(val)).Str(`</td></tr>`)
		}

		buf.Str(`</tbody></table>`)
	}
}

// buildBreadcrumbHTML writes breadcrumb navigation HTML to buf.
func buildBreadcrumbHTML(buf *textbuf.Buffer, crumbs []BreadcrumbSegment) {
	buf.Str(`<ol class="breadcrumb-list">`)

	for _, seg := range crumbs {
		if seg.Active {
			buf.Str(`<li class="breadcrumb-item breadcrumb-active"><span>`).Str(template.HTMLEscapeString(seg.Name)).Str(`</span></li>`)
		} else {
			buf.Str(`<li class="breadcrumb-item"><a href="`).Str(template.HTMLEscapeString(seg.URL)).Str(`">`).Str(template.HTMLEscapeString(seg.Name)).Str(`</a></li>`)
		}
	}

	buf.Str(`</ol>`)
}

// buildConfigEditURL constructs the /config/edit/ URL for a context path.
func buildConfigEditURL(path []string) string {
	if len(path) == 0 {
		return configEditPath
	}

	var tb textbuf.Buffer
	return tb.Str(configEditPath).Join(path, "/").Byte('/').String()
}

func execListEntryOp(args, contextPath []string, verb, srcLabel string, op func(parent []string, list, src, dst string) error) ([]string, string) {
	if len(args) < 4 || args[len(args)-2] != "to" {
		var tb textbuf.Buffer
		return nil, tb.Str("usage: ").Str(verb).Str(" <list> ").Str(srcLabel).Str(" to <destination>").String()
	}
	dstKey := args[len(args)-1]
	srcTokens := args[:len(args)-2]
	fullPath := append(append([]string{}, contextPath...), srcTokens...)
	if len(fullPath) < 2 {
		var tb textbuf.Buffer
		return nil, tb.Str(verb).Str(" requires at least a list name and entry key").String()
	}
	srcKey := fullPath[len(fullPath)-1]
	listName := fullPath[len(fullPath)-2]
	parentPath := fullPath[:len(fullPath)-2]
	if err := op(parentPath, listName, srcKey, dstKey); err != nil {
		var tb textbuf.Buffer
		return nil, tb.Str("error: ").Err(err).String()
	}
	var tb textbuf.Buffer
	return nil, tb.Str(verb).Str("d ").Str(listName).Byte(' ').Str(srcKey).Str(" to ").Str(dstKey).String()
}
