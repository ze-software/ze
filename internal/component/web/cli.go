// Design: docs/architecture/web-interface.md -- CLI bar and command handlers
// Related: handler.go -- URL routing and content negotiation
// Related: handler_config.go -- Config handlers that CLI commands dispatch to
// Related: editor.go -- Editor manager for command execution
// Related: cli_terminal.go -- terminal mode and rendering helpers

package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const configEditPath = "/config/edit/"

// CommandCompleter provides operational command completions for terminal mode.
type CommandCompleter interface {
	Complete(input string) []contract.Completion
}

// Web command transports use these field names for the command and context.
const (
	webCommandField = "command"
	webPathField    = "path"
)

// maxCommandLength is the maximum allowed CLI command text length.
const maxCommandLength = 4096

// Config payloads are bounded independently from the command line; form
// encoding can use three bytes for each byte of the selected file.
const editorContentMax = 4 << 20
const editorRequestMax = 3*editorContentMax + 65536

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
	verbLoad       = "load"
	verbOption     = "option"
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
	parseErr error
	words    cli.CommandWords
	content  *string
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
	return cliCommand{Verb: cmd.Args[0], Args: cmd.Args[1:], Operator: cmd.Operator, words: cmd.words.Tail(), content: cmd.content}, true
}

// parseCLICommand splits raw command text into a verb and argument list.
// Handles quoted strings for arguments containing spaces.
func parseCLICommand(input string) cliCommand {
	input = strings.TrimSpace(input)
	if input == "" {
		return cliCommand{}
	}

	parsed, err := cli.ParseCommandWords(input)
	if err != nil {
		return cliCommand{parseErr: err}
	}
	tokens := parsed.Tokens()
	if len(tokens) == 0 {
		return cliCommand{}
	}

	return cliCommand{
		Verb:  tokens[0],
		Args:  tokens[1:],
		words: parsed,
	}
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
		data := cliPageData{Mode: terminalModeConfig}
		data.InitialCommand = r.URL.Query().Get(webCommandField)
		data.ContextPath = r.URL.Query().Get(webPathField)
		path, pathErr := decodeConfigPath(data.ContextPath)
		if len(data.InitialCommand) > maxCommandLength {
			http.Error(w, opererr.New("The command supplied to the CLI page is too long.",
				"The load-content request was not opened.",
				"Open /cli and enter a shorter load command").Error(), http.StatusBadRequest)
			return
		}
		if pathErr != nil {
			http.Error(w, opererr.New("The configuration context supplied to the CLI page is invalid.",
				"The load-content request was not opened.",
				"Open /cli and enter the load command from its configuration context").Error(), http.StatusBadRequest)
			return
		}
		if data.InitialCommand != "" {
			cmd := parseCLICommand(data.InitialCommand)
			for {
				inner, ok := commandBehindTop(cmd)
				if !ok {
					break
				}
				cmd = inner
			}
			// GET may open the content picker, but never execute a mutation.
			if cmd.parseErr != nil {
				http.Error(w, opererr.New("The CLI page can open only a load-content request.",
					"The command was not run because opening a URL must not change configuration.",
					"Open /cli and type the command in the terminal").Error(), http.StatusBadRequest)
				return
			}
			if cmd.Verb != verbLoad {
				http.Error(w, opererr.New("The CLI page can open only a load-content request.",
					"The command was not run because opening a URL must not change configuration.",
					"Open /cli and type the command in the terminal").Error(), http.StatusBadRequest)
				return
			}
		}
		data.Prompt = formatTerminalPrompt(data.Mode, path)
		content := renderer.renderComponent("cli_page", cliPage(data))
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
func HandleCLICommandWithAuthorizerAndAudit(mgr *EditorManager, schema *config.Schema, renderer *Renderer, authorizer aaa.Authorizer, recorder audit.Recorder, dispatchers ...CommandDispatcher) http.HandlerFunc {
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

		r.Body = http.MaxBytesReader(w, r.Body, editorRequestMax)

		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
			return
		}

		command := r.FormValue(webCommandField)
		if len(command) > maxCommandLength {
			http.Error(w, "command too long", http.StatusBadRequest)
			return
		}

		pathStr := r.FormValue(webPathField)
		contextPath, pathErr := decodeConfigPath(pathStr)
		if pathErr != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		cmd := parseCLICommand(command)
		if cmd.parseErr != nil {
			writeCLINotification(w, renderer, cmd.parseErr.Error(), "error")
			return
		}
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
		if values, ok := r.PostForm["content"]; ok {
			if len(values) > 0 {
				if len(values[0]) > editorContentMax {
					http.Error(w, opererr.New("The selected configuration exceeds the 4 MiB limit.",
						candidateUnchangedWhy,
						"Select a configuration file smaller than 4 MiB in /cli").Error(), http.StatusRequestEntityTooLarge)
					return
				}
				cmd.content = &values[0]
			}
		}

		dispatchCLICommand(w, r, cmd, contextPath, mgr, schema, renderer, username, recorder, dispatchers...)
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
	verbLoad: true, verbOption: true,
}

// dispatchCLICommand routes a parsed CLI command to the appropriate handler.
// Returns an error notification for unrecognized verbs. `top <command>` runs
// the command again here with the root as its position; the page's position
// changes only when a navigation handler answers one (writeCLIResponse), so
// the inner command keeps it unless it navigates. The recursion is bounded by
// the words of the line, which maxCommandLength caps.
func dispatchCLICommand(w http.ResponseWriter, r *http.Request, cmd cliCommand, contextPath []string, mgr *EditorManager, schema *config.Schema, renderer *Renderer, username string, recorder audit.Recorder, dispatchers ...CommandDispatcher) {
	var dispatch CommandDispatcher
	if len(dispatchers) > 0 {
		dispatch = dispatchers[0]
	}
	if _, err := mgr.GetOrCreate(username); err != nil {
		http.Error(w, editorSessionRefusal(err).Error(), http.StatusInternalServerError)
		return
	}
	line := cmd.words.String()
	action, detail := terminalAuditContext(mgr, username, cmd, line)
	result := executeTerminalCommand(r.Context(), schema, mgr.Tree(username), mgr, username, r.RemoteAddr, contextPath, terminalModeConfig, cmd, line, dispatch)
	if result.completion != nil {
		defer result.completion.TransportComplete()
	}
	if terminalAuditSucceeded(cmd, result) {
		if cmd.Verb == verbDiscard {
			detail = discardAuditDetail(mgr, username, detail)
		}
		recordWebAudit(recorder, r, username, action, detail)
	}
	if terminalCommandFailed(result.output) {
		writeCLINotification(w, renderer, result.output, "error")
		return
	}
	base := cmd
	for {
		inner, underTop := commandBehindTop(base)
		if !underTop {
			break
		}
		base = inner
	}
	if result.input != "" {
		query := url.Values{webCommandField: {line}, webPathField: {encodeConfigPath(contextPath)}}
		htmxRedirect(w, r, "/cli?"+query.Encode())
		return
	}
	if !result.navigated {
		switch base.Verb {
		case verbSet, verbDelete:
			htmxRedirect(w, r, buildConfigEditURL(result.path))
			return
		case verbDiscard:
			htmxRedirect(w, r, configEditPath)
			return
		case verbCommit:
			if result.output == terminalOutputCommitSuccessful {
				htmxRedirect(w, r, configEditPath)
				return
			}
		}
	}
	if result.navigated {
		view, err := buildConfigViewData(schema, mgr.Tree(username), result.path)
		if err != nil {
			writeCLINotification(w, renderer, err.Error(), "error")
			return
		}
		writeCLIResponse(w, renderer, result.path, view)
		return
	}
	writeHTML(w, string(renderer.renderComponent("cli_show_response", cliShowResponse(cliShowData{
		Prompt: formatCLIPrompt(result.path), ContextPath: encodeConfigPath(result.path),
		Content: result.output, Crumbs: buildBreadcrumbs(result.path),
		PathBar: &FragmentData{CLIPathSegments: buildPathBarSegments(result.path)},
	}))))
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

		pathStr := r.URL.Query().Get(webPathField)
		contextPath, pathErr := decodeConfigPath(pathStr)
		if pathErr != nil {
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
			Explanation string `json:"explanation,omitempty"`
			Type        string `json:"type"`
		}

		items := make([]completionItem, len(completions))
		for i, c := range completions {
			items[i] = completionItem{
				Text:        c.Text,
				Description: c.ShortHelp,
				Explanation: c.Description,
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
	if mode == terminalModeOperational {
		var completions []contract.Completion
		if commandCompleter != nil {
			completions = commandCompleter.Complete(input)
		}
		return append(completions, cli.SessionFormatCompletions(schema, input)...)
	}
	if commandCompleter != nil {
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
