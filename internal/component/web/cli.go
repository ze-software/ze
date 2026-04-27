// Design: docs/architecture/web-interface.md -- CLI bar and terminal mode
// Related: handler.go -- URL routing and content negotiation
// Related: handler_config.go -- Config handlers that CLI commands dispatch to
// Related: editor.go -- Editor manager for command execution

package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/cli/contract"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

const configEditPath = "/config/edit/"

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
	var current strings.Builder
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

	return "ze[" + strings.Join(path, " ") + "]# "
}

// HandleCLIPage renders the CLI terminal page content for the workbench.
// Layout matches the SSH CLI: output viewport fills available space, two-line
// message area shows feedback and hints, prompt + input at the very bottom.
func HandleCLIPage(renderer *Renderer) template.HTML {
	prompt := formatCLIPrompt(nil)

	var buf strings.Builder
	fmt.Fprintf(&buf, `<div class="cli-page">`)
	fmt.Fprintf(&buf, `<div class="cli-output" id="cli-output"></div>`)
	fmt.Fprintf(&buf, `<div class="cli-messages" id="cli-messages">`)
	fmt.Fprintf(&buf, `<div class="cli-feedback" id="cli-feedback"></div>`)
	fmt.Fprintf(&buf, `<div class="cli-hint" id="cli-hint">Tab/?: complete, Enter: execute</div>`)
	fmt.Fprintf(&buf, `</div>`)
	fmt.Fprintf(&buf, `<div class="cli-input-line">`)
	fmt.Fprintf(&buf, `<span class="terminal-prompt" id="terminal-prompt">%s</span>`, template.HTMLEscapeString(prompt))
	fmt.Fprintf(&buf, `<input type="text" class="terminal-input" id="terminal-input" `)
	fmt.Fprintf(&buf, `autocomplete="off" spellcheck="false" name="command">`)
	fmt.Fprintf(&buf, `<div class="terminal-completions" id="terminal-completions" style="display:none"></div>`)
	fmt.Fprintf(&buf, `</div>`)
	fmt.Fprintf(&buf, `<span id="cli-context-path" style="display:none"></span>`)
	fmt.Fprintf(&buf, `</div>`)

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
		writeCLINotification(w, fmt.Sprintf("unknown command: %s", cmd.Verb), "error")
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
		writeCLINotification(w, fmt.Sprintf("invalid path: %s", err), "error")
		return
	}

	newPath := append(append([]string{}, contextPath...), args...)

	// Validate the path exists in schema.
	if len(newPath) > 0 {
		if _, err := walkSchema(schema, newPath); err != nil {
			writeCLINotification(w, fmt.Sprintf("invalid path: %s", err), "error")
			return
		}
	}

	tree := mgr.Tree(username)
	viewData, err := buildConfigViewData(schema, tree, newPath)
	if err != nil {
		writeCLINotification(w, fmt.Sprintf("view error: %s", err), "error")
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
				writeCLINotification(w, fmt.Sprintf("%s is not a leaf -- did you forget a value?", key), "error")
				return
			}
		}
	}

	if err := mgr.SetValue(username, setPath, key, value); err != nil {
		writeCLINotification(w, fmt.Sprintf("set error: %s", err), "error")
		return
	}

	htmxRedirect(w, r, buildConfigEditURL(contextPath))
}

// handleCLIDelete processes the "delete" verb: removes a value at the current context path.
func handleCLIDelete(w http.ResponseWriter, r *http.Request, contextPath, args []string, mgr *EditorManager, username string) {
	if len(args) < 1 {
		writeCLINotification(w, "usage: delete <leaf>", "error")
		return
	}

	key := args[0]

	if err := ValidatePathSegments([]string{key}); err != nil {
		writeCLINotification(w, "invalid leaf name", "error")
		return
	}

	if err := mgr.DeleteValue(username, contextPath, key); err != nil {
		writeCLINotification(w, fmt.Sprintf("delete error: %s", err), "error")
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

	var buf strings.Builder
	buf.WriteString("Active web sessions:\n")
	for _, s := range sessions {
		fmt.Fprintf(&buf, "  %s\n", s)
	}
	writeCLINotification(w, buf.String(), "info")
}

// handleCLIShow processes the "show" verb: renders config text in the content area.
func handleCLIShow(w http.ResponseWriter, contextPath, args []string, renderer *Renderer, mgr *EditorManager, username string) {
	if err := ValidatePathSegments(args); err != nil {
		writeCLINotification(w, fmt.Sprintf("invalid path: %s", err), "error")
		return
	}

	showPath := append(append([]string{}, contextPath...), args...)
	content := mgr.ContentAtPath(username, showPath)
	crumbs := buildBreadcrumbs(showPath)
	prompt := formatCLIPrompt(showPath)

	var buf strings.Builder
	buildBreadcrumbOOB(&buf, crumbs)
	fmt.Fprintf(&buf, `<main class="content-area" id="content-area">`)
	fmt.Fprintf(&buf, `<pre class="config-output">%s</pre>`, template.HTMLEscapeString(content))
	fmt.Fprintf(&buf, `</main>`)
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
		writeCLINotification(w, fmt.Sprintf("view error: %s", err), "error")
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
		writeCLINotification(w, fmt.Sprintf("view error: %s", err), "error")
		return
	}

	writeCLIResponse(w, renderer, newPath, viewData)
}

// handleCLICommit processes the "commit" verb.
func handleCLICommit(w http.ResponseWriter, r *http.Request, mgr *EditorManager, username string) {
	result, err := mgr.Commit(username)
	if err != nil {
		writeCLINotification(w, fmt.Sprintf("commit error: %s", err), "error")
		return
	}

	if len(result.Conflicts) > 0 {
		var msg strings.Builder
		msg.WriteString("Commit conflicts:\n")

		for _, c := range result.Conflicts {
			fmt.Fprintf(&msg, "  %s: want %q, other (%s) has %q\n", c.Path, c.MyValue, c.OtherUser, c.OtherValue)
		}

		writeCLINotification(w, msg.String(), "error")

		return
	}

	htmxRedirect(w, r, configEditPath)
}

// handleCLIDiscard processes the "discard" verb.
func handleCLIDiscard(w http.ResponseWriter, r *http.Request, mgr *EditorManager, username string) {
	if err := mgr.Discard(username); err != nil {
		writeCLINotification(w, fmt.Sprintf("discard error: %s", err), "error")
		return
	}

	htmxRedirect(w, r, configEditPath)
}

// HandleCLIComplete returns a GET handler for /cli/complete that provides
// tab-completion candidates for the CLI bar input.
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

func HandleCLIComplete(completer contract.Completer, mgr *EditorManager, schema *config.Schema) http.HandlerFunc {
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

		// Adjust context: CLI can't be "at" a list, step back to parent.
		contextPath = adjustListContext(schema, contextPath)

		// Set the editor tree so completions can see existing list entries.
		if mgr != nil {
			if userTree := mgr.Tree(username); userTree != nil {
				completer.SetTree(userTree)
			}
		}

		completions := completer.Complete(input, contextPath)
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

// terminalResponse is the JSON envelope returned by the terminal endpoint.
// The JS client updates the output viewport, message area, and prompt from
// these fields, matching the SSH CLI's fixed-zone layout.
type terminalResponse struct {
	Output   string `json:"output"`
	Feedback string `json:"feedback"`
	Path     string `json:"path,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
}

// HandleCLITerminal returns a POST handler for /cli/terminal that processes
// commands in terminal mode. Returns a JSON response with structured output
// so the client can update the output viewport and message area separately,
// matching the SSH CLI's layout.
//
// The committed tree is used for show output so the CLI displays the same
// config the workbench shows (the daemon's running config, not just the
// editor's on-disk file).
func HandleCLITerminal(mgr *EditorManager, schema *config.Schema, tree *config.Tree) http.HandlerFunc {
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

		// Use the hub tree (parsed from draft or committed config at startup)
		// so the CLI shows the same view as the SSH editor.
		viewTree := tree

		newPath, output := executeTerminalNav(schema, viewTree, mgr, username, contextPath, cmd)

		resp := terminalResponse{
			Output:   output,
			Feedback: terminalFeedback(cmd, output),
		}

		// After navigation, show config at the new path.
		if newPath != nil {
			resp.Path = strings.Join(newPath, "/")
			resp.Prompt = formatCLIPrompt(newPath)
			if cmd.Verb == verbEdit || cmd.Verb == verbUp || cmd.Verb == verbTop {
				resp.Output = serializeTreeAtPath(viewTree, schema, newPath)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, fmt.Sprintf("json encode: %v", err), http.StatusInternalServerError)
		}
	}
}

// terminalFeedback returns the message-area feedback line for a command.
func terminalFeedback(cmd cliCommand, output string) string {
	switch cmd.Verb {
	case verbSet:
		if len(cmd.Args) >= 2 { //nolint:mnd // set <key> <value>
			return fmt.Sprintf("set %s = %s", cmd.Args[0], strings.Join(cmd.Args[1:], " "))
		}
	case verbDelete:
		if len(cmd.Args) >= 1 {
			return fmt.Sprintf("deleted %s", cmd.Args[0])
		}
	case verbCommit:
		if strings.HasPrefix(output, "error") || strings.HasPrefix(output, "commit conflicts") {
			return output
		}
		return "commit successful"
	case verbDiscard:
		return "changes discarded"
	case verbEdit:
		return "edit " + strings.Join(cmd.Args, " ")
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
		return nil, fmt.Sprintf("unknown command: %s", cmd.Verb)
	}

	// Check for pipe in args: split into command args and pipe filters.
	allTokens := append([]string{cmd.Verb}, cmd.Args...)
	if pipeIdx := cli.FindPipeIndex(allTokens); pipeIdx > 0 {
		baseCmd := cliCommand{Verb: allTokens[0], Args: allTokens[1:pipeIdx]}
		filters := cli.ParsePipeFilters(allTokens[pipeIdx+1:])
		newPath, output = executeTerminalNav(schema, viewTree, mgr, username, contextPath, baseCmd)
		for _, f := range filters {
			if f.Type == "format" && f.Arg == "config" {
				output = mgr.ContentAtPath(username, contextPath)
			} else {
				filtered, err := cli.ApplyPipeFilter(output, f)
				if err != nil {
					return newPath, fmt.Sprintf("pipe error: %s", err)
				}
				output = filtered
			}
		}
		return newPath, output
	}

	switch cmd.Verb {
	case verbEdit:
		target := append(append([]string{}, contextPath...), cmd.Args...)
		if len(target) > 0 {
			if _, err := walkSchema(schema, target); err != nil {
				return nil, fmt.Sprintf("invalid path: %s", err)
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
		return nil, serializeTreeAtPath(viewTree, schema, showPath)
	case verbSet:
		return nil, executeTerminalSet(mgr, username, contextPath, cmd.Args)
	case verbDelete:
		return nil, executeTerminalDelete(mgr, username, contextPath, cmd.Args)
	case verbCommit:
		return nil, executeTerminalCommit(mgr, username)
	case verbDiscard:
		if err := mgr.Discard(username); err != nil {
			return nil, fmt.Sprintf("error: %s", err)
		}
		return nil, "changes discarded"
	case verbWho:
		sessions := mgr.ActiveSessions()
		if len(sessions) == 0 {
			return nil, "no active sessions"
		}
		var buf strings.Builder
		buf.WriteString("active sessions:\n")
		for _, s := range sessions {
			fmt.Fprintf(&buf, "  %s\n", s)
		}
		return nil, buf.String()
	case verbCompare:
		return nil, mgr.Compare(username)
	case verbSave:
		if err := mgr.SaveDraft(username); err != nil {
			return nil, fmt.Sprintf("error: %s", err)
		}
		return nil, "changes saved to draft"
	case verbHistory:
		backups, err := mgr.ListBackups(username)
		if err != nil {
			return nil, fmt.Sprintf("error: %s", err)
		}
		if len(backups) == 0 {
			return nil, "no backups found"
		}
		var buf strings.Builder
		for i, b := range backups {
			fmt.Fprintf(&buf, "%d. %s  %s\n", i+1, b.Timestamp, b.Path)
		}
		return nil, buf.String()
	case verbRollback:
		if len(cmd.Args) != 1 {
			return nil, "usage: rollback <number>"
		}
		n, err := strconv.Atoi(cmd.Args[0])
		if err != nil {
			return nil, fmt.Sprintf("invalid backup number: %s", cmd.Args[0])
		}
		backups, bErr := mgr.ListBackups(username)
		if bErr != nil {
			return nil, fmt.Sprintf("error: %s", bErr)
		}
		if n < 1 || n > len(backups) {
			return nil, fmt.Sprintf("backup %d not found (have %d backups)", n, len(backups))
		}
		if err := mgr.Rollback(username, backups[n-1].Path); err != nil {
			return nil, fmt.Sprintf("error: %s", err)
		}
		return nil, fmt.Sprintf("rolled back to %s", backups[n-1].Path)
	case verbRename:
		if len(cmd.Args) < 4 || cmd.Args[len(cmd.Args)-2] != "to" {
			return nil, "usage: rename <list> <old-name> to <new-name>"
		}
		newKey := cmd.Args[len(cmd.Args)-1]
		oldTokens := cmd.Args[:len(cmd.Args)-2]
		fullPath := append(append([]string{}, contextPath...), oldTokens...)
		if len(fullPath) < 2 {
			return nil, "rename requires at least a list name and entry key"
		}
		oldKey := fullPath[len(fullPath)-1]
		listName := fullPath[len(fullPath)-2]
		parentPath := fullPath[:len(fullPath)-2]
		if err := mgr.RenameListEntry(username, parentPath, listName, oldKey, newKey); err != nil {
			return nil, fmt.Sprintf("error: %s", err)
		}
		return nil, fmt.Sprintf("renamed %s %s to %s", listName, oldKey, newKey)
	case verbCopy:
		if len(cmd.Args) < 4 || cmd.Args[len(cmd.Args)-2] != "to" {
			return nil, "usage: copy <list> <source> to <destination>"
		}
		dstKey := cmd.Args[len(cmd.Args)-1]
		srcTokens := cmd.Args[:len(cmd.Args)-2]
		fullPath := append(append([]string{}, contextPath...), srcTokens...)
		if len(fullPath) < 2 {
			return nil, "copy requires at least a list name and entry key"
		}
		srcKey := fullPath[len(fullPath)-1]
		listName := fullPath[len(fullPath)-2]
		parentPath := fullPath[:len(fullPath)-2]
		if err := mgr.CopyListEntry(username, parentPath, listName, srcKey, dstKey); err != nil {
			return nil, fmt.Sprintf("error: %s", err)
		}
		return nil, fmt.Sprintf("copied %s %s to %s", listName, srcKey, dstKey)
	case verbInsert:
		if len(cmd.Args) < 1 {
			return nil, "usage: insert <path>"
		}
		insertPath := append(append([]string{}, contextPath...), cmd.Args...)
		if err := mgr.CreateEntry(username, insertPath); err != nil {
			return nil, fmt.Sprintf("error: %s", err)
		}
		return nil, fmt.Sprintf("inserted entry at %s", strings.Join(cmd.Args, " "))
	case verbDeactivate:
		if len(cmd.Args) < 1 {
			return nil, "usage: deactivate <path>"
		}
		fullPath := append(append([]string{}, contextPath...), cmd.Args...)
		if err := mgr.DeactivatePath(username, fullPath); err != nil {
			return nil, fmt.Sprintf("error: %s", err)
		}
		return nil, fmt.Sprintf("deactivated %s", strings.Join(cmd.Args, " "))
	case verbActivate:
		if len(cmd.Args) < 1 {
			return nil, "usage: activate <path>"
		}
		fullPath := append(append([]string{}, contextPath...), cmd.Args...)
		if err := mgr.ActivatePath(username, fullPath); err != nil {
			return nil, fmt.Sprintf("error: %s", err)
		}
		return nil, fmt.Sprintf("activated %s", strings.Join(cmd.Args, " "))
	case verbErrors:
		return nil, mgr.Compare(username)
	case verbDisconnect:
		if len(cmd.Args) < 1 {
			return nil, "usage: disconnect <session-id>"
		}
		if err := mgr.DisconnectSession(username, cmd.Args[0]); err != nil {
			return nil, fmt.Sprintf("error: %s", err)
		}
		return nil, fmt.Sprintf("disconnected session %s", cmd.Args[0])
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

// serializeTreeAtPath returns the config text at the given path from the
// committed tree. At root it serializes the full tree; at a subpath it
// walks to the subtree and serializes that section.
func serializeTreeAtPath(tree *config.Tree, schema *config.Schema, path []string) string {
	if tree == nil || schema == nil {
		return ""
	}
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

// executeTerminalSet handles the set command in terminal mode.
func executeTerminalSet(mgr *EditorManager, username string, contextPath, args []string) string {
	if len(args) < 2 { //nolint:mnd // set requires key and value
		return "error: usage: set <leaf> <value>"
	}

	if err := ValidatePathSegments([]string{args[0]}); err != nil {
		return fmt.Sprintf("error: invalid leaf name: %s", args[0])
	}

	if err := mgr.SetValue(username, contextPath, args[0], strings.Join(args[1:], " ")); err != nil {
		return fmt.Sprintf("error: %s", err)
	}

	return fmt.Sprintf("set %s = %s", args[0], strings.Join(args[1:], " "))
}

// executeTerminalDelete handles the delete command in terminal mode.
func executeTerminalDelete(mgr *EditorManager, username string, contextPath, args []string) string {
	if len(args) < 1 {
		return "error: usage: delete <leaf>"
	}

	if err := ValidatePathSegments([]string{args[0]}); err != nil {
		return fmt.Sprintf("error: invalid leaf name: %s", args[0])
	}

	if err := mgr.DeleteValue(username, contextPath, args[0]); err != nil {
		return fmt.Sprintf("error: %s", err)
	}

	return fmt.Sprintf("deleted %s", args[0])
}

// executeTerminalCommit handles the commit command in terminal mode.
func executeTerminalCommit(mgr *EditorManager, username string) string {
	result, err := mgr.Commit(username)
	if err != nil {
		return fmt.Sprintf("error: %s", err)
	}

	if len(result.Conflicts) > 0 {
		var msg strings.Builder
		msg.WriteString("commit conflicts:\n")

		for _, c := range result.Conflicts {
			fmt.Fprintf(&msg, "  %s: want %q, other (%s) has %q\n", c.Path, c.MyValue, c.OtherUser, c.OtherValue)
		}

		return msg.String()
	}

	return "commit successful"
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

	var buf strings.Builder
	fmt.Fprintf(&buf, `<div class="terminal-container" id="content-area">`)
	fmt.Fprintf(&buf, `<div class="terminal-scrollback" id="terminal-scrollback"></div>`)
	fmt.Fprintf(&buf, `<div class="terminal-input-line">`)
	fmt.Fprintf(&buf, `<span class="terminal-prompt">%s</span>`, template.HTMLEscapeString(prompt))
	fmt.Fprintf(&buf, `<input type="text" class="terminal-input" id="terminal-input" `)
	fmt.Fprintf(&buf, `autocomplete="off" spellcheck="false" `)
	fmt.Fprintf(&buf, `hx-post="/cli/terminal" hx-trigger="keydown[key=='Enter']" `)
	fmt.Fprintf(&buf, `hx-target="#terminal-scrollback" hx-swap="beforeend" `)
	fmt.Fprintf(&buf, `hx-include="this" name="command">`)
	fmt.Fprintf(&buf, `</div>`)
	fmt.Fprintf(&buf, `</div>`)

	writeHTML(w, buf.String())
}

// writeCLIResponse writes an HTMX multi-target response with content area,
// breadcrumb OOB swap, CLI prompt OOB swap, path bar OOB swap, and context
// path OOB swap.
func writeCLIResponse(w http.ResponseWriter, renderer *Renderer, path []string, viewData *ConfigViewData) {
	crumbs := buildBreadcrumbs(path)
	prompt := formatCLIPrompt(path)

	var buf strings.Builder

	// Main content area (must match layout.html element for outerHTML swap).
	fmt.Fprintf(&buf, `<main class="content-area" id="content-area">`)
	buildViewDataHTML(&buf, viewData)
	fmt.Fprintf(&buf, `</main>`)

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
	var buf strings.Builder
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
	for i, seg := range path {
		joined := strings.Join(path[:i+1], "/")
		segments[i] = PathBarSegment{
			Name:   seg,
			URL:    "/show/" + joined + "/",
			HxPath: joined,
		}
	}

	return segments
}

// buildPathBarOOB appends a CLI path bar OOB swap using the path_bar_inner
// template rendered via the Renderer. Falls back to empty if renderer is nil.
func buildPathBarOOB(buf *strings.Builder, path []string, renderer *Renderer) {
	fmt.Fprintf(buf, `<div class="cli-path-bar" id="cli-path-bar" hx-swap-oob="innerHTML">`)
	if renderer != nil {
		data := struct {
			CLIPathSegments []PathBarSegment
		}{CLIPathSegments: buildPathBarSegments(path)}
		buf.WriteString(string(renderer.RenderFragment("path_bar_inner", data)))
	}
	fmt.Fprintf(buf, `</div>`)
}

// buildContextOOB appends a hidden context path OOB swap element to buf.
func buildContextOOB(buf *strings.Builder, path []string) {
	fmt.Fprintf(buf, `<span id="cli-context-path" style="display:none" hx-swap-oob="true">%s</span>`,
		template.HTMLEscapeString(strings.Join(path, "/")))
}

// buildBreadcrumbOOB appends a breadcrumb OOB swap element to buf.
func buildBreadcrumbOOB(buf *strings.Builder, crumbs []BreadcrumbSegment) {
	fmt.Fprintf(buf, `<nav class="breadcrumb-bar" id="breadcrumb-bar" hx-swap-oob="innerHTML">`)
	buildBreadcrumbHTML(buf, crumbs)
	fmt.Fprintf(buf, `</nav>`)
}

// buildPromptOOB appends a CLI prompt OOB swap element to buf.
func buildPromptOOB(buf *strings.Builder, prompt string) {
	fmt.Fprintf(buf, `<span class="cli-prompt" id="cli-prompt" hx-swap-oob="innerHTML">%s</span>`,
		template.HTMLEscapeString(prompt))
}

// buildNotificationOOB appends a notification OOB swap element to buf.
func buildNotificationOOB(buf *strings.Builder, message, notifType string) {
	cssClass := "notification-info"
	if notifType == "error" {
		cssClass = "notification-error"
	}

	fmt.Fprintf(buf, `<aside class="notification-bar %s" id="notification-bar" hx-swap-oob="true">%s</aside>`,
		cssClass, template.HTMLEscapeString(message))
}

// buildViewDataHTML writes a simple HTML representation of ConfigViewData to buf.
func buildViewDataHTML(buf *strings.Builder, data *ConfigViewData) {
	if data == nil {
		return
	}

	if len(data.Children) > 0 {
		fmt.Fprintf(buf, `<ul class="config-children">`)

		for _, child := range data.Children {
			fmt.Fprintf(buf, `<li><a href="%s" class="config-link config-link-%s">%s</a></li>`,
				template.HTMLEscapeString(child.URL),
				template.HTMLEscapeString(child.Kind),
				template.HTMLEscapeString(child.Name))
		}

		fmt.Fprintf(buf, `</ul>`)
	}

	if len(data.Keys) > 0 {
		prefix := "/show/"
		if len(data.Path) > 0 {
			prefix += strings.Join(data.Path, "/") + "/"
		}

		fmt.Fprintf(buf, `<ul class="config-keys">`)

		for _, key := range data.Keys {
			fmt.Fprintf(buf, `<li><a href="%s%s/">%s</a></li>`,
				template.HTMLEscapeString(prefix),
				template.HTMLEscapeString(key),
				template.HTMLEscapeString(key))
		}

		fmt.Fprintf(buf, `</ul>`)
	}

	if len(data.LeafFields) > 0 {
		fmt.Fprintf(buf, `<table class="config-leaves"><thead><tr><th>Name</th><th>Value</th></tr></thead><tbody>`)

		for i := range data.LeafFields {
			f := &data.LeafFields[i]
			val := f.Value
			if val == "" && f.Default != "" {
				val = f.Default + " (default)"
			}

			fmt.Fprintf(buf, `<tr><td>%s</td><td>%s</td></tr>`,
				template.HTMLEscapeString(f.Name),
				template.HTMLEscapeString(val))
		}

		fmt.Fprintf(buf, `</tbody></table>`)
	}
}

// buildBreadcrumbHTML writes breadcrumb navigation HTML to buf.
func buildBreadcrumbHTML(buf *strings.Builder, crumbs []BreadcrumbSegment) {
	fmt.Fprintf(buf, `<ol class="breadcrumb-list">`)

	for _, seg := range crumbs {
		if seg.Active {
			fmt.Fprintf(buf, `<li class="breadcrumb-item breadcrumb-active"><span>%s</span></li>`,
				template.HTMLEscapeString(seg.Name))
		} else {
			fmt.Fprintf(buf, `<li class="breadcrumb-item"><a href="%s">%s</a></li>`,
				template.HTMLEscapeString(seg.URL),
				template.HTMLEscapeString(seg.Name))
		}
	}

	fmt.Fprintf(buf, `</ol>`)
}

// buildConfigEditURL constructs the /config/edit/ URL for a context path.
func buildConfigEditURL(path []string) string {
	if len(path) == 0 {
		return configEditPath
	}

	return configEditPath + strings.Join(path, "/") + "/"
}
