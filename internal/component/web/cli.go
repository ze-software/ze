// Design: docs/architecture/web-interface.md -- CLI bar and command handlers
// Related: handler.go -- URL routing and content negotiation
// Related: handler_config.go -- Config handlers that CLI commands dispatch to
// Related: editor.go -- Editor manager for command execution
// Related: cli_terminal.go -- terminal mode and rendering helpers

package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errNoEditorManager = errors.New("no editor manager")
	errNoSavedDraft    = errors.New("no saved draft")
)

const configEditPath = "/config/edit/"

// CommandCompleter provides operational command completions for terminal mode.
type CommandCompleter interface {
	Complete(input string) []contract.Completion
}

// maxCommandLength is the maximum allowed CLI command text length.
const maxCommandLength = 4096

// maxAutocompleteInput is the maximum allowed autocomplete input length.
const maxAutocompleteInput = 1024

// maxCompletionResults caps the number of autocomplete candidates returned.
const maxCompletionResults = 50

// CLI verb constants matching the SSH CLI command grammar. A word that is also
// a canonical CLI verb takes its spelling from the verb registry
// (internal/component/command), so the web bar and the SSH CLI cannot end up
// spelling one verb two ways; the rest are editor-mode words the registry holds
// no entry for.
const (
	verbEdit       = "edit"
	verbSet        = command.VerbSet
	verbDelete     = command.VerbDelete
	verbShow       = command.VerbShow
	verbTop        = "top"
	verbUp         = "up"
	verbCommit     = command.VerbCommit
	verbDiscard    = "discard"
	discardAll     = "all"
	verbHelp       = "help"
	verbWho        = "who"
	verbCompare    = "compare"
	verbSave       = "save"
	verbHistory    = "history"
	verbRollback   = "rollback"
	verbRename     = "rename"
	verbCopy       = "copy"
	verbInsert     = "insert"
	verbDeactivate = "deactivate"
	verbActivate   = "activate"
	verbErrors     = "errors"
	verbDisconnect = "disconnect"
	verbRun        = "run"
	verbConfigure  = "configure"
	verbExit       = "exit"
	verbQuit       = "quit"
)

// cliCommand holds the parsed verb and arguments from a CLI bar input.
// Operator is the position the operator typed the line at, the request's
// `path`: every fix a refusal proposes is framed for it (cli.FrameFix), never
// for the position a command resolves at, which `top <command>` empties. Each
// editor sets it where it parses the line, and commandBehindTop keeps it.
type cliCommand struct {
	Verb     string
	Args     []string
	Operator []string
}

// commandBehindTop answers the command `top <command>` runs once at the root:
// the words after `top`, with the operator's position kept, so a fix the inner
// command proposes is exact from where the operator stands (Q-9). Bare `top`
// answers false: it moves to the root. The web terminal and the CLI bar both
// read `top` here, so one line means one thing in both editors, as on the SSH
// editor (cli.Model cmdTop).
func commandBehindTop(cmd cliCommand) (cliCommand, bool) {
	if cmd.Verb != verbTop {
		return cliCommand{}, false
	}
	if len(cmd.Args) == 0 {
		return cliCommand{}, false
	}
	return cliCommand{Verb: cmd.Args[0], Args: cmd.Args[1:], Operator: cmd.Operator}, true
}

// parseCLICommand splits raw command text into a verb and argument list.
// Handles quoted strings for arguments containing spaces.
func parseCLICommand(input string) cliCommand {
	input = strings.TrimSpace(input)
	if input == "" {
		return cliCommand{}
	}

	tokens := tokenizeCommand(input)
	if len(tokens) == 0 {
		return cliCommand{}
	}

	return cliCommand{
		Verb: tokens[0],
		Args: tokens[1:],
	}
}

// tokenizeCommand splits input on whitespace, respecting double-quoted strings.
func tokenizeCommand(input string) []string {
	var tokens []string
	var current textbuf.Buffer
	inQuote := false

	for _, r := range input {
		if r == '"' {
			inQuote = !inQuote
			continue
		}

		if r == ' ' && !inQuote {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
			}

			continue
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// formatCLIPrompt returns the CLI bar prompt string for the given context path.
// Format: ze[<space-separated path>]# or ze# at root.
func formatCLIPrompt(path []string) string {
	if len(path) == 0 {
		return "ze# "
	}

	var tb textbuf.Buffer
	return tb.Str("ze[").Join(path, " ").Str("]# ").String()
}

const (
	terminalModeConfig      = "config"
	terminalModeOperational = "operational"
)

func normalizeTerminalMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), terminalModeOperational) {
		return terminalModeOperational
	}
	return terminalModeConfig
}

func formatTerminalPrompt(mode string, path []string) string {
	if normalizeTerminalMode(mode) == terminalModeOperational {
		return "ze> "
	}
	return formatCLIPrompt(path)
}

// HandleCLIPage renders the CLI terminal page content for the workbench.
// Layout matches the SSH CLI: output viewport fills available space, two-line
// message area shows feedback and hints, prompt + input at the very bottom.
func HandleCLIPage(renderer *Renderer) template.HTML {
	return renderer.renderComponent("cli_page", cliPage(cliPageData{
		Prompt: formatTerminalPrompt(terminalModeConfig, nil),
		Mode:   terminalModeConfig,
	}))
}

// HandleCLIPageHTTP returns an HTTP handler for /cli/ that renders the CLI
// terminal in the Finder layout (topbar + full-page terminal, no sidebar).
// Both the Finder and Workbench link to this same page.
func HandleCLIPageHTTP(renderer *Renderer, insecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		content := HandleCLIPage(renderer)
		layoutData := LayoutData{
			Title:      "Ze: CLI",
			Content:    content,
			HasSession: true,
			CLIPrompt:  formatCLIPrompt(nil),
			Username:   username,
			Insecure:   insecure,
			ActiveUI:   uiModeTokenCLI,
		}
		if err := renderer.RenderLayout(w, layoutData); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}

// HandleCLICommand returns a POST handler for /cli that dispatches CLI bar
// commands in integrated mode. The command text is parsed into verb + args
// and dispatched to the appropriate EditorManager method.
//
// Returns HTMX multi-target responses: content swap + breadcrumb OOB +
// notification OOB as appropriate per command type.
func HandleCLICommand(mgr *EditorManager, schema *config.Schema, renderer *Renderer) http.HandlerFunc {
	return HandleCLICommandWithAuthorizer(mgr, schema, renderer, nil)
}

// HandleCLICommandWithAuthorizer returns a POST handler for /cli that enforces
// profile RBAC before integrated-mode config mutations.
func HandleCLICommandWithAuthorizer(mgr *EditorManager, schema *config.Schema, renderer *Renderer, authorizer aaa.Authorizer) http.HandlerFunc {
	return HandleCLICommandWithAuthorizerAndAudit(mgr, schema, renderer, authorizer, nil)
}

// HandleCLICommandWithAuthorizerAndAudit returns a POST handler for /cli that
// enforces profile RBAC and records each successful discard in recorder.
func HandleCLICommandWithAuthorizerAndAudit(mgr *EditorManager, schema *config.Schema, renderer *Renderer, authorizer aaa.Authorizer, recorder audit.Recorder) http.HandlerFunc {
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

		command := r.FormValue("command")
		if len(command) > maxCommandLength {
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

		cmd := parseCLICommand(command)
		if cmd.Verb == "" {
			http.Error(w, "empty command", http.StatusBadRequest)
			return
		}
		// Shared terminalAuthCommand intentionally leaves navigation verbs
		// (edit/up/top) unauthorized: the web bar only changes the viewed subtree,
		// it does not create a mutable API config session.
		if authCommand := terminalAuthCommand(cmd); authCommand != "" {
			if !authorizeWebConfigMutation(w, r, authorizer, username, authCommand) {
				return
			}
		}
		// If the path ends at a named list (not an entry), step back one level.
		// The CLI can't be "at" a list -- you're before it or inside an entry.
		// Skip for navigation commands (up/top) that manage their own path.
		if cmd.Verb != verbUp && cmd.Verb != verbTop {
			contextPath = adjustListContext(schema, contextPath)
		}
		cmd.Operator = contextPath

		dispatchCLICommand(w, r, cmd, contextPath, mgr, schema, renderer, username, recorder)
	}
}

// knownCLIVerbs is the set of CLI verbs handled by the web CLI bar.
// Must match the SSH CLI command set (model_commands.go).
var knownCLIVerbs = map[string]bool{
	verbEdit: true, verbSet: true, verbDelete: true, verbShow: true,
	verbTop: true, verbUp: true, verbCommit: true, verbDiscard: true,
	verbHelp: true, verbWho: true, verbCompare: true, verbSave: true,
	verbHistory: true, verbRollback: true, verbRename: true, verbCopy: true,
	verbInsert: true, verbDeactivate: true, verbActivate: true,
	verbErrors: true, verbDisconnect: true,
}

// dispatchCLICommand routes a parsed CLI command to the appropriate handler.
// Returns an error notification for unrecognized verbs. `top <command>` runs
// the command again here with the root as its position; the page's position
// changes only when a navigation handler answers one (writeCLIResponse), so
// the inner command keeps it unless it navigates. The recursion is bounded by
// the words of the line, which maxCommandLength caps.
func dispatchCLICommand(w http.ResponseWriter, r *http.Request, cmd cliCommand, contextPath []string, mgr *EditorManager, schema *config.Schema, renderer *Renderer, username string, recorder audit.Recorder) {
	if !knownCLIVerbs[cmd.Verb] {
		writeCLINotification(w, renderer, "unknown command: "+cmd.Verb, "error")
		return
	}

	switch cmd.Verb {
	case verbEdit:
		handleCLIEdit(w, contextPath, cmd.Args, schema, renderer, mgr, username)
	case verbSet:
		handleCLISet(w, r, contextPath, cmd, schema, renderer, mgr, username)
	case verbDelete:
		handleCLIDelete(w, r, contextPath, cmd.Args, renderer, mgr, username)
	case verbShow:
		handleCLIShow(w, contextPath, cmd.Args, renderer, mgr, username)
	case verbTop:
		if inner, underTop := commandBehindTop(cmd); underTop {
			dispatchCLICommand(w, r, inner, nil, mgr, schema, renderer, username, recorder)
			return
		}
		handleCLITop(w, schema, renderer, mgr, username)
	case verbUp:
		handleCLIUp(w, contextPath, schema, renderer, mgr, username)
	case verbCommit:
		handleCLICommit(w, r, renderer, mgr, username, cmd)
	case verbDiscard:
		handleCLIDiscard(w, r, renderer, mgr, username, cmd.Args, recorder)
	case verbWho:
		handleCLIWho(w, renderer, mgr)
	case verbHelp:
		writeCLINotification(w, renderer, "commands: edit, set, delete, show, top [command], up, commit now, discard [path|all], who, help", "info")
	}
}

// handleCLIEdit processes the "edit" verb: updates context path and returns
// new breadcrumb + content for the target path.
func handleCLIEdit(w http.ResponseWriter, contextPath, args []string, schema *config.Schema, renderer *Renderer, mgr *EditorManager, username string) {
	if err := ValidatePathSegments(args); err != nil {
		writeCLINotification(w, renderer, "invalid path: "+err.Error(), "error")
		return
	}

	newPath := append(append([]string{}, contextPath...), args...)

	// Validate the path exists in schema.
	if len(newPath) > 0 {
		if _, err := walkSchema(schema, newPath); err != nil {
			writeCLINotification(w, renderer, "invalid path: "+err.Error(), "error")
			return
		}
	}

	tree := mgr.Tree(username)
	viewData, err := buildConfigViewData(schema, tree, newPath)
	if err != nil {
		writeCLINotification(w, renderer, "view error: "+err.Error(), "error")
		return
	}

	writeCLIResponse(w, renderer, newPath, viewData)
}

// handleCLISet processes the "set" verb: sets a value at the current context path.
// Supports paths into containers: "set local ip 127.0.0.1" navigates into "local"
// and sets "ip" to "127.0.0.1". The last token is the value, the second-to-last
// is the leaf name, and any preceding tokens extend the context path.
// The full path (context + args) must resolve to a specific list entry, not an
// anonymous list access (which would create a "default" entry).
// errInvalidSetPath refuses a set whose path tokens fail ValidatePathSegments.
var errInvalidSetPath = errors.New("invalid path")

// splitSetArgs reads `set` the way the SSH editor does: the last token is the
// value, the one before it the leaf, and the rest extend the context path. Both
// web surfaces, the integrated CLI and the terminal, call it, so one line means
// one thing in every editor. MUST be called with at least two arguments.
func splitSetArgs(schema *config.Schema, contextPath, args []string) (setPath []string, key, value string, err error) {
	if err := ValidatePathSegments(args[:len(args)-1]); err != nil {
		return nil, "", "", errInvalidSetPath
	}
	value = args[len(args)-1]
	key = args[len(args)-2]
	setPath = append(append([]string{}, contextPath...), args[:len(args)-2]...)

	// The target must be a leaf, not a container or list.
	if schema != nil {
		if node, lookupErr := schema.Lookup(config.JoinPath(append(setPath, key)...)); lookupErr == nil {
			if node.Kind() != config.NodeLeaf {
				return nil, "", "", fmt.Errorf("%s is not a leaf -- did you forget a value?", key)
			}
		}
	}
	return setPath, key, value, nil
}

func handleCLISet(w http.ResponseWriter, r *http.Request, contextPath []string, cmd cliCommand, schema *config.Schema, renderer *Renderer, mgr *EditorManager, username string) {
	if len(cmd.Args) < 2 { //nolint:mnd // set requires key and value
		writeCLINotification(w, renderer, "usage: set <leaf> <value>", "error")
		return
	}

	if _, err := applyWebSet(mgr, schema, username, contextPath, cmd.Operator, cmd.Args); err != nil {
		writeCLINotification(w, renderer, "set error: "+err.Error(), "error")
		return
	}

	htmxRedirect(w, r, buildConfigEditURL(contextPath))
}

// applyWebSet runs one `set` for the web terminal and the CLI bar alike, with
// the checks the SSH editor's `set` makes (cli.Model cmdSet), so a line is
// judged the same in every editor (Q-6, Q-8b). args are the words after `set`,
// relative to at; operator is the position the fix of a refusal is framed for.
// It answers the acknowledgement, a credential in it masked.
//
//   - A path ending at a list's key leaf (`set ospf redistribute source
//     connected`) names the entry to create, which the creator judges and
//     stores under its key, never as a leaf of an anonymous entry.
//   - A path ending at the key leaf of an entry it names is refused: only
//     rename changes the key of an entry that exists (AC-33).
//   - Otherwise the value is judged where it is set (EditorManager.SetValue)
//     and a new list key on the way where its entry is created.
//
// MUST be called with at least two arguments.
func applyWebSet(mgr *EditorManager, schema *config.Schema, username string, at, operator, args []string) (string, error) {
	setPath, key, value, err := splitSetArgs(schema, at, args)
	if err != nil {
		return "", err
	}
	leafPath := slices.Concat(setPath, []string{key})

	var tb textbuf.Buffer
	if listName, keyForm := keyLeafList(schema, setPath, key); keyForm {
		if err := mgr.CreateEntry(username, slices.Concat(setPath, []string{value})); err != nil {
			// The key is the last word typed, so the fix is this `set` with the
			// key corrected.
			return "", cli.FramedSetRefusal(err, at, args, operator)
		}
		return tb.Str("created ").Str(listName).Byte(' ').Str(value).String(), nil
	}
	if schema != nil {
		if err := cli.InEntryKeyLeafRefusal(schema, leafPath, at, operator); err != nil {
			return "", err
		}
	}
	if err := mgr.SetValue(username, setPath, key, value); err != nil {
		framed := framedWebSetRefusal(err, at, args, leafPath, operator)
		// A value refusal is built masked; any other refusal names what it
		// refused, so it is masked here.
		return "", errors.New(config.DisplayMessageAtPath(schema, leafPath, framed.Error(), value))
	}
	// The acknowledgement travels to the browser in the response body, so a
	// credential in it reaches view-source, the disk cache and any proxy that
	// reads the document.
	return tb.Str("set ").Join(args[:len(args)-1], " ").Byte(' ').Str(config.DisplayValueAtPath(schema, leafPath, value)).String(), nil
}

// keyLeafList answers the list whose entry `set <setPath> <key> <value>` names
// when key is that list's first key leaf and setPath ends at the list's own
// name, the form `set <list> <key-leaf> <key>` (the SSH editor's
// Editor.isListKeyLeafPath). A setPath ending at an entry of the list is the
// in-entry form, which answers false here.
func keyLeafList(schema *config.Schema, setPath []string, key string) (string, bool) {
	if schema == nil {
		return "", false
	}
	if len(setPath) == 0 {
		return "", false
	}
	listName := setPath[len(setPath)-1]
	var holder config.ChildProvider = schema
	if len(setPath) > 1 {
		parent, isHolder := schema.LookupTokenPath(setPath[:len(setPath)-1]).(config.ChildProvider)
		if !isHolder {
			return "", false
		}
		holder = parent
	}
	list, isList := holder.Get(listName).(*config.ListNode)
	if !isList {
		return "", false
	}
	return listName, list.FirstKeyLeafName() == key
}

// framedWebSetRefusal answers the refusal of a `set` with its fix replaced by
// the line the operator typed, corrected: the value when the refusal names the
// leaf set (its path is leafPath), else the list key the creator refused on
// the way. An error that holds no schema refusal is answered as it is.
func framedWebSetRefusal(err error, at, args, leafPath, operator []string) error {
	var refusal *configyang.ValidationError
	if !errors.As(err, &refusal) {
		return err
	}
	if !refusal.HoldsOperatorError() {
		return err
	}
	if slices.Equal(refusal.Segments(), leafPath) {
		return cli.FramedSetRefusal(err, at, args, operator)
	}
	return cli.FramedKeyRefusal(err, verbSet, at, args, operator)
}

// handleCLIDelete processes the "delete" verb: removes the node named by the
// argument under the current context path. Schema-aware (mgr.DeleteByPath), so
// it deletes list entries and containers as well as leaves, matching the SSH
// CLI's `delete` (cli.Model.cmdDelete). The leaf-only DeleteValue silently
// no-ops on a list entry.
func handleCLIDelete(w http.ResponseWriter, r *http.Request, contextPath, args []string, renderer *Renderer, mgr *EditorManager, username string) {
	if len(args) < 1 {
		writeCLINotification(w, renderer, "usage: delete <name>", "error")
		return
	}

	key := args[0]

	if err := ValidatePathSegments([]string{key}); err != nil {
		writeCLINotification(w, renderer, "invalid name", "error")
		return
	}

	if err := mgr.DeleteByPath(username, contextPath, key); err != nil {
		var tb textbuf.Buffer
		writeCLINotification(w, renderer, tb.Str("delete error: ").Str(key).Str(": ").Err(err).String(), "error")
		return
	}

	htmxRedirect(w, r, buildConfigEditURL(contextPath))
}

// handleCLIWho processes the "who" verb: lists active web editing sessions.
func handleCLIWho(w http.ResponseWriter, renderer *Renderer, mgr *EditorManager) {
	sessions := mgr.ActiveSessions()
	if len(sessions) == 0 {
		writeCLINotification(w, renderer, "No active web sessions.", "info")
		return
	}

	var buf textbuf.Buffer
	buf.Str("Active web sessions:\n")
	for _, s := range sessions {
		buf.Str("  ").Str(s).Byte('\n')
	}
	writeCLINotification(w, renderer, buf.String(), "info")
}

// handleCLIShow processes the "show" verb: renders config text in the content area.
func handleCLIShow(w http.ResponseWriter, contextPath, args []string, renderer *Renderer, mgr *EditorManager, username string) {
	if err := ValidatePathSegments(args); err != nil {
		writeCLINotification(w, renderer, "invalid path: "+err.Error(), "error")
		return
	}

	showPath := append(append([]string{}, contextPath...), args...)

	writeHTML(w, string(renderer.renderComponent("cli_show_response", cliShowResponse(cliShowData{
		Prompt:      formatCLIPrompt(showPath),
		ContextPath: textbuf.Join(showPath, "/"),
		Content:     mgr.ContentAtPath(username, showPath),
		Crumbs:      buildBreadcrumbs(showPath),
		PathBar:     &FragmentData{CLIPathSegments: buildPathBarSegments(showPath)},
	}))))
}

// handleCLITop processes the "top" verb: navigates to root context.
func handleCLITop(w http.ResponseWriter, schema *config.Schema, renderer *Renderer, mgr *EditorManager, username string) {
	tree := mgr.Tree(username)
	viewData, err := buildConfigViewData(schema, tree, nil)
	if err != nil {
		writeCLINotification(w, renderer, "view error: "+err.Error(), "error")
		return
	}

	writeCLIResponse(w, renderer, nil, viewData)
}

// handleCLIUp processes the "up" verb: navigates one level up in the context path.
func handleCLIUp(w http.ResponseWriter, contextPath []string, schema *config.Schema, renderer *Renderer, mgr *EditorManager, username string) {
	newPath := contextPath
	if len(newPath) > 0 {
		newPath = newPath[:len(newPath)-1]
	}

	tree := mgr.Tree(username)
	viewData, err := buildConfigViewData(schema, tree, newPath)
	if err != nil {
		writeCLINotification(w, renderer, "view error: "+err.Error(), "error")
		return
	}

	writeCLIResponse(w, renderer, newPath, viewData)
}

// handleCLICommit processes the "commit" verb. A refusal's fixes are framed
// for the position the operator typed the commit at (cmd.Operator), so inside
// a context they read `top set <full path> ...` (AC-23, AC-51).
func handleCLICommit(w http.ResponseWriter, r *http.Request, renderer *Renderer, mgr *EditorManager, username string, cmd cliCommand) {
	req, err := webCommitRequest(cmd.Args)
	if err != nil {
		writeCLINotification(w, renderer, "commit error: "+err.Error(), "error")
		return
	}
	answer, err := mgr.runCommit(username, req, cmd.Operator)
	if err != nil {
		writeCLINotification(w, renderer, "commit error: "+err.Error(), "error")
		return
	}
	if len(answer.conflicts) > 0 {
		writeCLINotification(w, renderer, formatCommitConflicts("Commit conflicts:\n", answer.conflicts), "error")
		return
	}
	if answer.refusal != "" {
		writeCLINotification(w, renderer, answer.refusal, "error")
		return
	}
	// A commit that opened or closed a window, or only verified, tells the
	// operator what to do next; a plain commit or an abort reloads the page.
	switch req.Action {
	case contract.CommitConfirmed, contract.CommitAccept, contract.CommitVerify:
		writeCLINotification(w, renderer, answer.message, "info")
		return
	case contract.CommitNow, contract.CommitAbort, contract.CommitActionUnspecified:
	}

	htmxRedirect(w, r, configEditPath)
}

// handleCLIDiscard processes the "discard" verb: `discard` or `discard all`
// reverts every change, `discard <path>` the changes at that path. Each
// successful discard is audited with the diff it dropped, as the terminal,
// the Discard button and SSH record it.
func handleCLIDiscard(w http.ResponseWriter, r *http.Request, renderer *Renderer, mgr *EditorManager, username string, args []string, recorder audit.Recorder) {
	before := mgr.Diff(username)
	if _, err := runWebDiscard(mgr, username, args); err != nil {
		writeCLINotification(w, renderer, "discard error: "+err.Error(), "error")
		return
	}
	recordWebAudit(recorder, r, username, audit.ActionConfigDiscard, discardAuditDetail(mgr, username, before))

	htmxRedirect(w, r, configEditPath)
}

// discardAuditDetail answers the config-discard detail of a discard that
// succeeded: the part of before, the user's diff read just ahead of it, that
// the discard dropped (audit.DroppedDiff). A path discard keeps the changes
// outside its path, and the row MUST NOT name those as dropped. The terminal
// and the CLI bar both call it. MUST be called after the discard succeeded.
func discardAuditDetail(mgr *EditorManager, username, before string) string {
	return audit.DroppedDiff(before, mgr.Diff(username))
}

// adjustListContext corrects the web URL path for CLI use.
// The web can display a named node's table (e.g., /show/bgp/peer/), but in the CLI
// you are always before the named node or after (inside an entry) -- never at it.
// Path ["bgp", "peer"] becomes ["bgp"]. Path ["bgp", "peer", "thomas"] stays.
func adjustListContext(schema *config.Schema, path []string) []string {
	if len(path) == 0 {
		return path
	}
	node, err := walkSchema(schema, path)
	if err != nil {
		return path
	}
	if _, isList := node.(*config.ListNode); isList && !isListEntryPath(schema, path) {
		return path[:len(path)-1]
	}
	return path
}

// HandleCLIComplete returns a GET handler for /cli/complete that provides
// tab-completion candidates for the CLI bar input.
func HandleCLIComplete(completer *cli.Completer, mgr *EditorManager, schema *config.Schema) http.HandlerFunc {
	return HandleCLICompleteWithCommandCompleter(completer, nil, mgr, schema)
}

// HandleCLICompleteWithCommandCompleter returns a completion handler that can
// complete config-mode paths and operational-mode command trees.
func HandleCLICompleteWithCommandCompleter(completer *cli.Completer, commandCompleter CommandCompleter, mgr *EditorManager, schema *config.Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		input := r.URL.Query().Get("input")
		if len(input) > maxAutocompleteInput {
			input = input[:maxAutocompleteInput]
		}

		pathStr := r.URL.Query().Get("path")
		var contextPath []string
		if pathStr != "" {
			contextPath = strings.Split(pathStr, "/")
		}

		if err := ValidatePathSegments(contextPath); err != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		completions := completeCLIInput(completer, commandCompleter, mgr, schema, username, input, contextPath, normalizeTerminalMode(r.URL.Query().Get("mode")))
		if len(completions) > maxCompletionResults {
			completions = completions[:maxCompletionResults]
		}

		type completionItem struct {
			Text        string `json:"text"`
			Description string `json:"description"`
			Type        string `json:"type"`
		}

		items := make([]completionItem, len(completions))
		for i, c := range completions {
			items[i] = completionItem{
				Text:        c.Text,
				Description: c.ShortHelp,
				Type:        c.Type,
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(items); err != nil {
			http.Error(w, fmt.Sprintf("json encode: %v", err), http.StatusInternalServerError)
		}
	}
}

// completeCLIInput answers one completion request. The completer is shared by
// every logged-in user, so the user's candidate tree is passed into the call,
// read under that user's session lock (EditorManager.ReadTree): no other
// user's request and no edit of the same session can change it mid-walk.
func completeCLIInput(completer *cli.Completer, commandCompleter CommandCompleter, mgr *EditorManager, schema *config.Schema, username, input string, contextPath []string, mode string) []contract.Completion {
	if commandCompleter != nil {
		if mode == terminalModeOperational {
			return commandCompleter.Complete(input)
		}
		if rest, ok := strings.CutPrefix(input, verbRun+" "); ok {
			return commandCompleter.Complete(rest)
		}
	}

	contextPath = adjustListContext(schema, contextPath)
	if mgr == nil {
		return completer.Complete(nil, input, contextPath)
	}
	var completions []contract.Completion
	inSession := mgr.ReadTree(username, func(tree *config.Tree) {
		completions = completer.Complete(tree, input, contextPath)
	})
	if !inSession {
		// No editing session yet: there is no candidate config to read, so
		// the answer comes from the schema alone.
		return completer.Complete(nil, input, contextPath)
	}
	return completions
}
