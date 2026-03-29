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
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
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
	verbEdit    = "edit"
	verbSet     = "set"
	verbDelete  = "delete"
	verbShow    = "show"
	verbTop     = "top"
	verbUp      = "up"
	verbCommit  = "commit"
	verbDiscard = "discard"
	verbHelp    = "help"
	verbWho     = "who"
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

		// If the path ends at a named list (not an entry), step back one level.
		// The CLI can't be "at" a list -- you're before it or inside an entry.
		contextPath = adjustListContext(schema, contextPath)

		cmd := parseCLICommand(command)
		if cmd.Verb == "" {
			http.Error(w, "empty command", http.StatusBadRequest)
			return
		}

		dispatchCLICommand(w, r, cmd, contextPath, mgr, schema, renderer, username)
	}
}

// knownCLIVerbs is the set of CLI verbs handled by the web CLI bar.
var knownCLIVerbs = map[string]bool{
	verbEdit: true, verbSet: true, verbDelete: true, verbShow: true,
	verbTop: true, verbUp: true, verbCommit: true, verbDiscard: true,
	verbHelp: true, verbWho: true,
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
		handleCLIShow(w, contextPath, cmd.Args, mgr, username)
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
func handleCLISet(w http.ResponseWriter, r *http.Request, contextPath, args []string, _ *config.Schema, mgr *EditorManager, username string) {
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
func handleCLIShow(w http.ResponseWriter, contextPath, args []string, mgr *EditorManager, username string) {
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

func HandleCLIComplete(completer *cli.Completer, mgr *EditorManager, schema *config.Schema) http.HandlerFunc {
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
		if userTree := mgr.Tree(username); userTree != nil {
			completer.SetTree(userTree)
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

// HandleCLITerminal returns a POST handler for /cli/terminal that processes
// commands in terminal mode. Commands produce plain text output identical
// to the SSH CLI, returned as pre-formatted text for the terminal scrollback.
func HandleCLITerminal(mgr *EditorManager) http.HandlerFunc {
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
		output := executeTerminalCommand(mgr, username, contextPath, cmd)
		prompt := formatCLIPrompt(contextPath)

		var buf strings.Builder
		fmt.Fprintf(&buf, `<div class="terminal-entry">`)
		fmt.Fprintf(&buf, `<span class="terminal-prompt">%s</span>`, template.HTMLEscapeString(prompt))
		fmt.Fprintf(&buf, `<span class="terminal-command">%s</span>`, template.HTMLEscapeString(command))
		fmt.Fprintf(&buf, `<pre class="terminal-output">%s</pre>`, template.HTMLEscapeString(output))
		fmt.Fprintf(&buf, `</div>`)

		writeHTML(w, buf.String())
	}
}

// executeTerminalCommand runs a CLI command and returns its text output.
func executeTerminalCommand(mgr *EditorManager, username string, contextPath []string, cmd cliCommand) string {
	if !knownCLIVerbs[cmd.Verb] && cmd.Verb != "" {
		return fmt.Sprintf("unknown command: %s", cmd.Verb)
	}

	switch cmd.Verb {
	case verbShow:
		showPath := append(append([]string{}, contextPath...), cmd.Args...)
		return mgr.ContentAtPath(username, showPath)
	case verbSet:
		return executeTerminalSet(mgr, username, contextPath, cmd.Args)
	case verbDelete:
		return executeTerminalDelete(mgr, username, contextPath, cmd.Args)
	case verbCommit:
		return executeTerminalCommit(mgr, username)
	case verbDiscard:
		if err := mgr.Discard(username); err != nil {
			return fmt.Sprintf("error: %s", err)
		}
		return "changes discarded"
	case verbTop:
		return "navigated to root"
	case verbUp:
		return "navigated up"
	case verbHelp:
		return "commands: edit, set, delete, show, top, up, commit, discard, help"
	case "":
		return ""
	}

	return ""
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
// breadcrumb OOB swap, and CLI prompt OOB swap.
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

	_ = renderer // renderer passed for consistency with template-based handlers
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
