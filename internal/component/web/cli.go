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
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
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

// CLI verb constants matching the SSH CLI command grammar.
const (
	verbEdit       = "edit"
	verbSet        = "set"
	verbDelete     = "delete"
	verbShow       = "show"
	verbTop        = "top"
	verbUp         = "up"
	verbCommit     = "commit"
	verbDiscard    = "discard"
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
type cliCommand struct {
	Verb string
	Args []string
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
				current.Reset()
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
	prompt := formatTerminalPrompt(terminalModeConfig, nil)

	var buf textbuf.Buffer
	buf.Str(`<div class="cli-page">`)
	buf.Str(`<div class="cli-output" id="cli-output"></div>`)
	buf.Str(`<div class="cli-messages" id="cli-messages">`)
	buf.Str(`<div class="cli-feedback" id="cli-feedback"></div>`)
	buf.Str(`<div class="cli-hint" id="cli-hint">Tab/?: complete, Enter: execute</div>`)
	buf.Str(`</div>`)
	buf.Str(`<div class="cli-input-line">`)
	buf.Str(`<span class="terminal-prompt" id="terminal-prompt">`).Str(template.HTMLEscapeString(prompt)).Str(`</span>`)
	buf.Str(`<input type="text" class="terminal-input" id="terminal-input" `)
	buf.Str(`autocomplete="off" spellcheck="false" name="command">`)
	buf.Str(`<div class="terminal-completions is-hidden" id="terminal-completions"></div>`)
	buf.Str(`</div>`)
	buf.Str(`<span id="cli-context-path" class="is-hidden"></span>`)
	buf.Str(`<span id="cli-mode" class="is-hidden">`).Str(terminalModeConfig).Str(`</span>`)
	buf.Str(`</div>`)

	return template.HTML(buf.String()) //nolint:gosec // trusted template output
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
			ActiveUI:   "cli",
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

		dispatchCLICommand(w, r, cmd, contextPath, mgr, schema, renderer, username)
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
// Returns an error notification for unrecognized verbs.
func dispatchCLICommand(w http.ResponseWriter, r *http.Request, cmd cliCommand, contextPath []string, mgr *EditorManager, schema *config.Schema, renderer *Renderer, username string) {
	if !knownCLIVerbs[cmd.Verb] {
		writeCLINotification(w, "unknown command: "+cmd.Verb, "error")
		return
	}

	switch cmd.Verb {
	case verbEdit:
		handleCLIEdit(w, contextPath, cmd.Args, schema, renderer, mgr, username)
	case verbSet:
		handleCLISet(w, r, contextPath, cmd.Args, schema, mgr, username)
	case verbDelete:
		handleCLIDelete(w, r, contextPath, cmd.Args, mgr, username)
	case verbShow:
		handleCLIShow(w, contextPath, cmd.Args, renderer, mgr, username)
	case verbTop:
		handleCLITop(w, schema, renderer, mgr, username)
	case verbUp:
		handleCLIUp(w, contextPath, schema, renderer, mgr, username)
	case verbCommit:
		handleCLICommit(w, r, mgr, username)
	case verbDiscard:
		handleCLIDiscard(w, r, mgr, username)
	case verbWho:
		handleCLIWho(w, mgr)
	case verbHelp:
		writeCLINotification(w, "commands: edit, set, delete, show, top, up, commit, discard, who, help", "info")
	}
}

// handleCLIEdit processes the "edit" verb: updates context path and returns
// new breadcrumb + content for the target path.
func handleCLIEdit(w http.ResponseWriter, contextPath, args []string, schema *config.Schema, renderer *Renderer, mgr *EditorManager, username string) {
	if err := ValidatePathSegments(args); err != nil {
		writeCLINotification(w, "invalid path: "+err.Error(), "error")
		return
	}

	newPath := append(append([]string{}, contextPath...), args...)

	// Validate the path exists in schema.
	if len(newPath) > 0 {
		if _, err := walkSchema(schema, newPath); err != nil {
			writeCLINotification(w, "invalid path: "+err.Error(), "error")
			return
		}
	}

	tree := mgr.Tree(username)
	viewData, err := buildConfigViewData(schema, tree, newPath)
	if err != nil {
		writeCLINotification(w, "view error: "+err.Error(), "error")
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
func handleCLISet(w http.ResponseWriter, r *http.Request, contextPath, args []string, schema *config.Schema, mgr *EditorManager, username string) {
	if len(args) < 2 { //nolint:mnd // set requires key and value
		writeCLINotification(w, "usage: set <leaf> <value>", "error")
		return
	}

	if err := ValidatePathSegments(args[:len(args)-1]); err != nil {
		writeCLINotification(w, "invalid path", "error")
		return
	}

	// Last token is value, second-to-last is leaf, rest extend the path.
	value := args[len(args)-1]
	key := args[len(args)-2]
	setPath := append(append([]string{}, contextPath...), args[:len(args)-2]...)

	// Validate that the target key is a leaf, not a container or list.
	if schema != nil {
		lookupPath := config.JoinPath(append(setPath, key)...)
		if node, err := schema.Lookup(lookupPath); err == nil {
			if node.Kind() != config.NodeLeaf {
				writeCLINotification(w, key+" is not a leaf -- did you forget a value?", "error")
				return
			}
		}
	}

	if err := mgr.SetValue(username, setPath, key, value); err != nil {
		writeCLINotification(w, "set error: "+err.Error(), "error")
		return
	}

	htmxRedirect(w, r, buildConfigEditURL(contextPath))
}

// handleCLIDelete processes the "delete" verb: removes the node named by the
// argument under the current context path. Schema-aware (mgr.DeleteByPath), so
// it deletes list entries and containers as well as leaves, matching the SSH
// CLI's `delete` (cli.Model.cmdDelete). The leaf-only DeleteValue silently
// no-ops on a list entry.
func handleCLIDelete(w http.ResponseWriter, r *http.Request, contextPath, args []string, mgr *EditorManager, username string) {
	if len(args) < 1 {
		writeCLINotification(w, "usage: delete <name>", "error")
		return
	}

	key := args[0]

	if err := ValidatePathSegments([]string{key}); err != nil {
		writeCLINotification(w, "invalid name", "error")
		return
	}

	if err := mgr.DeleteByPath(username, contextPath, key); err != nil {
		var tb textbuf.Buffer
		writeCLINotification(w, tb.Str("delete error: ").Str(key).Str(": ").Err(err).String(), "error")
		return
	}

	htmxRedirect(w, r, buildConfigEditURL(contextPath))
}

// handleCLIWho processes the "who" verb: lists active web editing sessions.
func handleCLIWho(w http.ResponseWriter, mgr *EditorManager) {
	sessions := mgr.ActiveSessions()
	if len(sessions) == 0 {
		writeCLINotification(w, "No active web sessions.", "info")
		return
	}

	var buf textbuf.Buffer
	buf.Str("Active web sessions:\n")
	for _, s := range sessions {
		buf.Str("  ").Str(s).Byte('\n')
	}
	writeCLINotification(w, buf.String(), "info")
}

// handleCLIShow processes the "show" verb: renders config text in the content area.
func handleCLIShow(w http.ResponseWriter, contextPath, args []string, renderer *Renderer, mgr *EditorManager, username string) {
	if err := ValidatePathSegments(args); err != nil {
		writeCLINotification(w, "invalid path: "+err.Error(), "error")
		return
	}

	showPath := append(append([]string{}, contextPath...), args...)
	content := mgr.ContentAtPath(username, showPath)
	crumbs := buildBreadcrumbs(showPath)
	prompt := formatCLIPrompt(showPath)

	var buf textbuf.Buffer
	buildBreadcrumbOOB(&buf, crumbs)
	buf.Str(`<main class="content-area" id="content-area">`)
	buf.Str(`<pre class="config-output">`).Str(template.HTMLEscapeString(content)).Str(`</pre>`)
	buf.Str(`</main>`)
	buildPromptOOB(&buf, prompt)
	buildPathBarOOB(&buf, showPath, renderer)
	buildContextOOB(&buf, showPath)

	writeHTML(w, buf.String())
}

// handleCLITop processes the "top" verb: navigates to root context.
func handleCLITop(w http.ResponseWriter, schema *config.Schema, renderer *Renderer, mgr *EditorManager, username string) {
	tree := mgr.Tree(username)
	viewData, err := buildConfigViewData(schema, tree, nil)
	if err != nil {
		writeCLINotification(w, "view error: "+err.Error(), "error")
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
		writeCLINotification(w, "view error: "+err.Error(), "error")
		return
	}

	writeCLIResponse(w, renderer, newPath, viewData)
}

// handleCLICommit processes the "commit" verb.
func handleCLICommit(w http.ResponseWriter, r *http.Request, mgr *EditorManager, username string) {
	result, err := mgr.Commit(username)
	if err != nil {
		writeCLINotification(w, "commit error: "+err.Error(), "error")
		return
	}

	if len(result.Conflicts) > 0 {
		var msg textbuf.Buffer
		msg.Str("Commit conflicts:\n")

		for _, c := range result.Conflicts {
			msg.Str("  ").Str(c.Path).Str(": want ").Quoted(c.MyValue).Str(", other (").Str(c.OtherUser).Str(") has ").Quoted(c.OtherValue).Byte('\n')
		}

		writeCLINotification(w, msg.String(), "error")

		return
	}

	htmxRedirect(w, r, configEditPath)
}

// handleCLIDiscard processes the "discard" verb.
func handleCLIDiscard(w http.ResponseWriter, r *http.Request, mgr *EditorManager, username string) {
	if err := mgr.Discard(username); err != nil {
		writeCLINotification(w, "discard error: "+err.Error(), "error")
		return
	}

	htmxRedirect(w, r, configEditPath)
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
func HandleCLIComplete(completer contract.Completer, mgr *EditorManager, schema *config.Schema) http.HandlerFunc {
	return HandleCLICompleteWithCommandCompleter(completer, nil, mgr, schema)
}

// HandleCLICompleteWithCommandCompleter returns a completion handler that can
// complete config-mode paths and operational-mode command trees.
func HandleCLICompleteWithCommandCompleter(completer contract.Completer, commandCompleter CommandCompleter, mgr *EditorManager, schema *config.Schema) http.HandlerFunc {
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
				Description: c.Description,
				Type:        c.Type,
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(items); err != nil {
			http.Error(w, fmt.Sprintf("json encode: %v", err), http.StatusInternalServerError)
		}
	}
}

func completeCLIInput(completer contract.Completer, commandCompleter CommandCompleter, mgr *EditorManager, schema *config.Schema, username, input string, contextPath []string, mode string) []contract.Completion {
	if commandCompleter != nil {
		if mode == terminalModeOperational {
			return commandCompleter.Complete(input)
		}
		if rest, ok := strings.CutPrefix(input, verbRun+" "); ok {
			return commandCompleter.Complete(rest)
		}
	}

	contextPath = adjustListContext(schema, contextPath)
	if mgr != nil {
		if userTree := mgr.Tree(username); userTree != nil {
			completer.SetTree(userTree)
		}
	}
	return completer.Complete(input, contextPath)
}
