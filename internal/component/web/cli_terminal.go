// Design: docs/architecture/web-interface.md -- CLI terminal mode and rendering helpers
// Related: cli.go -- command handlers and dispatch

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	terminalOutputCommitSuccessful = "commit successful"
	terminalOutputChangesDiscarded = "changes discarded"
)

// terminalResponse is the JSON envelope returned by the terminal endpoint.
// The JS client updates the output viewport, message area, and prompt from
// these fields, matching the SSH CLI's fixed-zone layout.
type terminalResponse struct {
	Output          string          `json:"output"`
	OutputBytes     json.RawMessage `json:"output-bytes,omitempty"`
	OutputByteCount int             `json:"output-byte-count"`
	Feedback        string          `json:"feedback"`
	Path            string          `json:"path"`
	Prompt          string          `json:"prompt,omitempty"`
	Mode            string          `json:"mode,omitempty"`
	Input           string          `json:"input,omitempty"`
	HistoryWarning  string          `json:"history-warning,omitempty"`
	Confirmation    string          `json:"confirmation,omitempty"`
	Navigate        string          `json:"navigate,omitempty"`
	OutputComplete  bool            `json:"output-complete"`
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
func HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr *EditorManager, schema *config.Schema, tree *config.Tree, dispatch CommandDispatcher, authorizer aaa.Authorizer, recorder audit.Recorder, registered ...func(string) bool) http.HandlerFunc {
	var recognizes func(string) bool
	if len(registered) > 0 {
		recognizes = registered[0]
	}
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

		r.Body = http.MaxBytesReader(w, r.Body, editorRequestMax)

		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
			return
		}

		commandText := r.FormValue(webCommandField)
		if len(commandText) > maxCommandLength {
			http.Error(w, "command too long", http.StatusBadRequest)
			return
		}
		// Registration can change while this request runs. Resolve this input
		// once, so authorization cannot select operational dispatch while a
		// later lookup selects an unapproved configuration mutation.
		registeredInput := frozenCLIRegistration(r, commandText, recognizes)

		pathStr := r.FormValue(webPathField)
		contextPath, pathErr := decodeConfigPath(pathStr)
		if pathErr != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		mode, validMode := submittedCLIMode(w, r)
		if !validMode {
			return
		}
		cmd := parseCLICommand(commandText)
		cmd.confirmed = r.PostFormValue("confirm") == boolTrue
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
		cmd.Operator = contextPath
		r = recordCLIHistory(w, r, mgr, mode, contextPath, commandText, registeredInput)
		if authCommand := terminalAuthCommandForMode(mode, cmd, commandText, registeredInput); authCommand != "" {
			if !authorizeWebConfigMutation(w, r, authorizer, username, authCommand) {
				return
			}
		}
		var auditAction, auditDetail string
		configCommand := mode != terminalModeOperational || cli.ShouldEnterConfigMode(commandText, registeredInput)
		if configCommand {
			var err error
			auditAction, auditDetail, err = terminalAuditContext(mgr, username, cmd, commandText)
			if err != nil {
				http.Error(w, editorSessionRefusal(err).Error(), http.StatusInternalServerError)
				return
			}
		}

		// Keep the committed hub tree as the compare baseline. Display commands
		// use the per-user working tree when one exists.
		viewTree := tree

		result := executeTerminalCommand(r.Context(), schema, viewTree, mgr, username, r.RemoteAddr, contextPath, mode, cmd, commandText, dispatch, registeredInput)
		if result.completion != nil {
			defer result.completion.TransportComplete()
		}
		if terminalAuditSucceeded(cmd, result) {
			if cmd.Verb == verbDiscard {
				// The row names what the discard dropped, never a change it kept.
				var err *opererr.Error
				auditDetail, err = discardAuditDetail(mgr, username, auditDetail)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			recordWebAudit(recorder, r, username, auditAction, auditDetail)
		}

		feedback := ""
		if configCommand {
			feedback = terminalFeedback(cmd, result.output)
		}
		selected := terminalSelectedOutput(result.output)
		if result.selectedOutput != nil {
			selected = *result.selectedOutput
		}
		resp := terminalResponse{
			Output:          result.output,
			OutputByteCount: len(selected),
			Feedback:        feedback,
			Path:            encodeConfigPath(result.path),
			Prompt:          formatTerminalPrompt(result.mode, result.path),
			Mode:            result.mode,
			Input:           result.input,
			HistoryWarning:  cliHistoryWarning(r),
			Confirmation:    result.confirmation,
			Navigate:        result.navigate,
			OutputComplete:  result.outputComplete,
		}
		// The display string can lose invalid UTF-8 when JSON encodes it.
		// Keep only a complete, in-budget artifact in the owner's byte codec;
		// retain its JSON document so this DTO also decodes its own wire form.
		// The original count distinguishes overflow from a malformed carrier.
		if result.outputComplete {
			if resp.OutputByteCount <= cliStreamOutputBytesMax {
				output, err := json.Marshal(command.Blob(selected))
				if err != nil {
					http.Error(w, opererr.Wrap(err,
						"The browser CLI command ran, but its output could not be encoded.",
						"No complete download is available. Do not repeat a command that changes the router.",
						"Open /cli and inspect the current router state before deciding whether to repeat the command").Error(),
						http.StatusInternalServerError)
					return
				}
				resp.OutputBytes = output
			}
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
// session creation which opens a mutable config session. `top <command>` is
// authorized as the command it runs, so `top` never carries a mutation past
// the gate.
func terminalAuthCommand(cmd cliCommand) string {
	if inner, underTop := commandBehindTop(cmd); underTop {
		return terminalAuthCommand(inner)
	}
	switch cmd.Verb {
	case verbSet, verbActivate:
		return webCommandConfigSet
	case verbDelete, verbDeactivate:
		return webCommandConfigDelete
	case verbCommit:
		return webCommandConfigCommit
	case verbDiscard, verbDisconnect:
		return webCommandConfigDiscard
	case verbRollback:
		return webCommandConfigRollback
	case verbRename:
		return webCommandConfigRename
	case verbCopy, verbInsert:
		return webCommandConfigAdd
	case verbSave, verbLoad:
		return webCommandConfigSave
	}
	return ""
}

func terminalAuthCommandForMode(mode string, cmd cliCommand, raw string, registered func(string) bool) string {
	if normalizeTerminalMode(mode) == terminalModeOperational {
		if cli.IsSessionFormatCommand(strings.TrimSpace(raw)) {
			return ""
		}
		if !cli.ShouldEnterConfigMode(raw, registered) {
			return ""
		}
	}
	return terminalAuthCommand(cmd)
}

// terminalSelectedOutput retains successful formatter output before display
// warnings without copying string storage. A nil pointer means no separate
// selection; a nonnil empty value is a valid operational answer.
type terminalSelectedOutput string

type terminalCommandResult struct {
	auditSuccess bool
	// outputComplete follows actual producer/formatter outcomes. It is never
	// inferred from output text, which may itself describe an error.
	outputComplete bool
	input          string
	path           []string
	mode           string
	output         string
	selectedOutput *terminalSelectedOutput
	completion     *plugin.RenderedResponse
	confirmation   string
	navigate       string
	// operational is a confirmed control's canonical daemon command. It is
	// dispatched by the same executor as ordinary operational input, never by
	// a web-owned lifecycle callback.
	operational string
	// navigated is set when the command answered a new position (edit, up,
	// top), so `top <command>` keeps the operator's position unless the
	// command behind it moved it.
	navigated bool
	// discarded is set by a discard that dropped changes, and only by it, so
	// the audit row follows the editor's answer and never the output text: a
	// refusal and a path discard each print their own line.
	discarded bool
}

func executeTerminalCommand(ctx context.Context, schema *config.Schema, viewTree *config.Tree, mgr *EditorManager, username, remoteAddr string, contextPath []string, mode string, cmd cliCommand, raw string, dispatch CommandDispatcher, registered ...func(string) bool) terminalCommandResult {
	result := terminalCommandResult{
		path: append([]string{}, contextPath...),
		mode: normalizeTerminalMode(mode),
	}
	if cmd.parseErr != nil {
		result.output = "error: " + cmd.parseErr.Error()
		return result
	}
	var recognizes func(string) bool
	if len(registered) > 0 {
		recognizes = registered[0]
	}
	controlMode := result.mode
	if controlMode == terminalModeOperational {
		if cli.ShouldEnterConfigMode(raw, recognizes) {
			controlMode = terminalModeConfig
		}
	}
	if control, handled := executeTerminalControl(schema, viewTree, mgr, username, result.path, controlMode, cmd); handled {
		if control.operational != "" {
			control.output, control.completion, control.selectedOutput, control.outputComplete = executeTerminalOperationalForUser(ctx, mgr, dispatch, username, remoteAddr, control.operational)
		}
		return control
	}

	if result.mode == terminalModeOperational {
		return executeTerminalOperationalMode(ctx, schema, viewTree, mgr, username, remoteAddr, result.path, cmd, raw, dispatch, recognizes)
	}
	return executeTerminalConfigMode(ctx, schema, viewTree, mgr, username, remoteAddr, result.path, cmd, dispatch)
}

func executeTerminalConfigMode(ctx context.Context, schema *config.Schema, viewTree *config.Tree, mgr *EditorManager, username, remoteAddr string, contextPath []string, cmd cliCommand, dispatch CommandDispatcher) terminalCommandResult {
	// `top <command>` runs the command once at the root, whatever it is, and
	// answers the operator's position unless the command navigated (Q-9). It
	// is taken before executeTerminalNav splits a pipe, so `top show | ...`
	// runs `show | ...` at the root. The recursion is bounded by the words of
	// the line, which maxCommandLength caps.
	if inner, underTop := commandBehindTop(cmd); underTop {
		answer := executeTerminalConfigMode(ctx, schema, viewTree, mgr, username, remoteAddr, nil, inner, dispatch)
		if !answer.navigated {
			answer.path = contextPath
		}
		return answer
	}
	result := terminalCommandResult{path: contextPath, mode: terminalModeConfig}
	if shared, handled := executeTerminalEditorCommand(mgr, username, contextPath, cmd); handled {
		return shared
	}
	if cmd.words.FirstPipe() >= 0 {
		if cmd.Verb != verbRun {
			if cmd.Verb != verbDiscard {
				result.output = "error: command '" + cmd.Verb + "' does not support piping"
				return result
			}
		}
	}
	switch cmd.Verb {
	case verbRun:
		if len(cmd.Args) == 0 {
			result.output = "usage: run <command>"
			return result
		}
		result.output, result.completion, result.selectedOutput, result.outputComplete = executeTerminalOperationalForUser(ctx, mgr, dispatch, username, remoteAddr, cmd.words.Tail().String())
		return result
	case verbHelp:
		result.output = terminalEditorHelp()
		result.outputComplete = true
		return result
	case verbCommit:
		answer, err := runWebCommit(mgr, username, cmd.Args, cmd.Operator)
		result.output = terminalCommitOutput(answer, err)
		result.auditSuccess = answer.applied
		if err != nil {
			return result
		}
		if len(answer.conflicts) > 0 {
			return result
		}
		if answer.refusal != "" {
			return result
		}
		result.outputComplete = true
		return result
	case verbDiscard:
		// Discard runs here, not in executeTerminalNav, because its success is
		// what the audit row needs and the navigator answers text only. Every
		// token is a config path token; only a bare pipe is an operator.
		if cmd.words.FirstPipe() >= 0 {
			result.output = "error: " + errDiscardPipe.Error()
			return result
		}
		discardArgs := cmd.Args
		if len(discardArgs) > 0 {
			if discardArgs[0] != discardAll {
				discardArgs = slices.Concat(contextPath, discardArgs)
			}
		}
		answer, err := runWebDiscard(mgr, username, discardArgs)
		if err != nil {
			var tb textbuf.Buffer
			result.output = tb.Str("error: ").Err(err).String()
			return result
		}
		result.output = answer
		result.discarded = true
		result.outputComplete = true
		return result
	}

	result.output = executeTerminalNav(schema, viewTree, mgr, username, contextPath, cmd)
	return result
}

func executeTerminalOperationalMode(ctx context.Context, schema *config.Schema, viewTree *config.Tree, mgr *EditorManager, username, remoteAddr string, contextPath []string, cmd cliCommand, raw string, dispatch CommandDispatcher, registered func(string) bool) terminalCommandResult {
	result := terminalCommandResult{path: contextPath, mode: terminalModeOperational}
	raw = strings.TrimSpace(raw)
	if cli.IsSessionFormatCommand(raw) {
		output, err := mgr.sessionFormatCommand(username, raw)
		result.output = output
		result.outputComplete = err == nil
		if err != nil {
			result.output = err.Error()
		}
		return result
	}
	if cli.IsConfigureCommand(raw) {
		result.mode = terminalModeConfig
		result.output = serializeTreeAtPath(displayTree(viewTree, mgr, username), schema, contextPath)
		result.outputComplete = true
		return result
	}
	if cmd.Verb == verbHelp {
		result.output = terminalEditorHelp()
		result.outputComplete = true
		return result
	}
	if cli.ShouldEnterConfigMode(raw, registered) {
		return executeTerminalConfigMode(ctx, schema, viewTree, mgr, username, remoteAddr, contextPath, cmd, dispatch)
	}
	result.output, result.completion, result.selectedOutput, result.outputComplete = executeTerminalOperationalForUser(ctx, mgr, dispatch, username, remoteAddr, raw)
	return result
}

func executeTerminalOperational(ctx context.Context, dispatch CommandDispatcher, username, remoteAddr, input, sessionFormat string) (string, *plugin.RenderedResponse) {
	output, rendered, _, _, _ := executeTerminalOperationalResult(ctx, dispatch, username, remoteAddr, input, sessionFormat) //nolint:errcheck // Output already carries the original failure; this text projection has no export or status result.
	return output, rendered
}

// executeTerminalOperationalResult retains both a failed command's diagnostic
// payload and its original error. A rendered payload never makes a failure
// complete, and an authorization refusal never exposes attached data.
// A successful formatter also returns its selected bytes before display warnings.
func executeTerminalOperationalResult(ctx context.Context, dispatch CommandDispatcher, username, remoteAddr, input, sessionFormat string) (string, *plugin.RenderedResponse, *terminalSelectedOutput, bool, error) {
	if dispatch == nil {
		err := errors.New("operational command dispatch not available")
		return "error: " + err.Error(), nil, nil, false, err
	}
	cmdStr, formatFn, pipeErr := command.ProcessPipesDefaultFormatResult(input, sessionFormat)
	if pipeErr != "" {
		var tb textbuf.Buffer
		return tb.Str("pipe error: ").Str(pipeErr).String(), nil, nil, false, errors.New(pipeErr)
	}
	rendered, executionErr := dispatch.JSON(ctx, plugin.CallerIdentity{
		Username: username, RemoteAddr: remoteAddr, Authorizer: plugin.CallerAuthorizer(ctx),
	}, cmdStr)
	if executionErr != nil {
		output := "error: " + executionErr.Error()
		if line, refused := plugin.RefusalLine(executionErr); refused {
			output = line
		}
		if !operationalPermissionDenied(executionErr) {
			payload, encodeErr := operationalDiagnosticPayload(rendered)
			if encodeErr != nil {
				output += "\nerror: diagnostic payload cannot be encoded: " + encodeErr.Error()
				executionErr = errors.Join(executionErr, encodeErr)
			} else if payload != "" {
				data, applyErr := formatFn(payload)
				if applyErr == "" {
					output += "\n" + data
				} else {
					output += "\n" + command.PipeErrorPrefix + applyErr
					executionErr = errors.Join(executionErr, errors.New(applyErr))
				}
			}
			return operationalWarningOutput(rendered, output), rendered, nil, false, executionErr
		}
		return output, rendered, nil, false, executionErr
	}
	output, applyErr := formatFn(rendered.Output)
	if applyErr != "" {
		return command.PipeErrorPrefix + applyErr, rendered, nil, false, errors.New(applyErr)
	}
	selected := terminalSelectedOutput(output)
	return operationalWarningOutput(rendered, output), rendered, &selected, true, nil
}

// operationalDiagnosticPayload keeps the rendering a failed walk already
// produced. An ordinary built diagnostic payload retains its prior fallback;
// a one-shot record walk is never entered again after its render failed.
func operationalDiagnosticPayload(rendered *plugin.RenderedResponse) (string, error) {
	if rendered == nil {
		return "", nil
	}
	if rendered.Output != "" {
		return rendered.Output, nil
	}
	if rendered.Response == nil {
		return "", nil
	}
	if rendered.Response.Data == nil {
		return "", nil
	}
	if _, generated := plugin.RecordRows(rendered.Response); generated {
		return "", nil
	}
	payload, err := json.Marshal(rendered.Response.Data)
	return string(payload), err
}

func operationalPermissionDenied(err error) bool {
	var denial interface{ PermissionDenied() bool }
	if errors.As(err, &denial) {
		return denial.PermissionDenied()
	}
	return false
}

func operationalWarningOutput(rendered *plugin.RenderedResponse, output string) string {
	// A warning reads above the answer, where the local command prints it
	// first on stderr, including when a failed answer retains useful data.
	warning := rendered.Response.WarningText()
	if warning == "" {
		if response := rendered.Response; response != nil {
			warned := response.Status == plugin.StatusWarning
			if !warned {
				if metadata, ok := response.RecordAnswerMetadata(); ok {
					warned = metadata.Warning
				}
			}
			if warned {
				// This is a verdict label, not a diagnostic the producer supplied.
				warning = "Warning"
			}
		}
	}
	if warning != "" {
		var tb textbuf.Buffer
		return tb.Str(warning).Byte('\n').Str(output).String()
	}
	return output
}

// terminalAuditContext prepares the audit detail before the command runs.
// Callers MUST stop on a snapshot error before executing or recording the action.
func terminalAuditContext(mgr *EditorManager, username string, cmd cliCommand, raw string) (string, string, error) {
	if inner, underTop := commandBehindTop(cmd); underTop {
		return terminalAuditContext(mgr, username, inner, raw)
	}
	// refuse-by-default: guards terminalAuditSucceeded -- commands outside commit, discard and rollback do not produce audit rows
	switch cmd.Verb {
	case verbCommit:
		detail, err := mgr.Diff(username)
		return audit.ActionConfigCommit, detail, err
	case verbDiscard:
		if len(cmd.Args) > 0 {
			if cmd.Args[0] != discardAll {
				detail, err := mgr.Diff(username)
				return audit.ActionConfigDiscard, detail, err
			}
		}
		detail, err := mgr.discardDiff(username)
		return audit.ActionConfigDiscard, detail, err
	case verbRollback:
		return audit.ActionConfigDiscard, raw, nil
	}
	return "", "", nil
}

// terminalAuditSucceeded reports whether the command terminalAuditContext
// prepared a row for succeeded, so the row is owed.
func terminalAuditSucceeded(cmd cliCommand, result terminalCommandResult) bool {
	if inner, underTop := commandBehindTop(cmd); underTop {
		return terminalAuditSucceeded(inner, result)
	}
	switch cmd.Verb {
	case verbCommit:
		return result.auditSuccess
	case verbDiscard:
		return result.discarded
	case verbRollback:
		return result.auditSuccess
	}
	return false
}

// terminalFeedback returns the message-area feedback line for a command.
func terminalFeedback(cmd cliCommand, output string) string {
	if inner, underTop := commandBehindTop(cmd); underTop {
		return terminalFeedback(inner, output)
	}
	var tb textbuf.Buffer
	switch cmd.Verb {
	case verbSet:
		return output
	case verbDelete:
		if len(cmd.Args) >= 1 {
			return tb.Str("deleted ").Join(cmd.Args, " ").String()
		}
	case verbCommit:
		return output
	case verbDiscard:
		// The answer itself: a refusal, the path a path discard dropped, or
		// every change. A fixed line would contradict the first two.
		return output
	case verbEdit:
		return tb.Str("edit ").Join(cmd.Args, " ").String()
	case verbUp:
		return "up"
	case verbTop:
		return "top"
	case verbLoad, verbOption:
		return output
	}
	return ""
}

// terminalCommandFailed recognizes the existing editor refusal envelopes. The
// transports share it so a refused mutation never becomes a success redirect.
func terminalCommandFailed(output string) bool {
	for _, prefix := range []string{"error:", "unknown command:", "invalid path:", "usage:", "commit blocked:", "commit conflicts", "Commit conflicts:", "pipe error:"} {
		if strings.HasPrefix(output, prefix) {
			return true
		}
	}
	return false
}

// executeTerminalNav returns command text for legacy direct callers and the
// terminal fallback. executeTerminalConfigMode obtains navigation state from
// executeTerminalEditorCommand before reaching this fallback.
// Pipe filters (| format config, | match, | head, | tail) are handled
// using the same cli.ApplyPipeFilter as the SSH CLI.
func executeTerminalNav(_ *config.Schema, _ *config.Tree, mgr *EditorManager, username string, contextPath []string, cmd cliCommand) string {
	if shared, handled := executeTerminalEditorCommand(mgr, username, contextPath, cmd); handled {
		return shared.output
	}
	if !knownCLIVerbs[cmd.Verb] && cmd.Verb != "" {
		var tb textbuf.Buffer
		return tb.Str("unknown command: ").Str(cmd.Verb).String()
	}

	if cmd.words.FirstPipe() >= 0 {
		return "error: command '" + cmd.Verb + "' does not support piping"
	}

	switch cmd.Verb {
	case verbCommit:
		return executeTerminalCommit(mgr, username, cmd)

	case verbHelp:
		return terminalEditorHelp()
	case "":
		return ""
	}
	return "unknown command: " + cmd.Verb
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
		panic("BUG: config format classifier returned an unknown format")
	}
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

// webCommitRequest parses the arguments after `commit` with the grammar every
// editor shares.
func webCommitRequest(args []string) (contract.CommitRequest, error) {
	return contract.ParseCommit(args)
}

// executeTerminalCommit handles the commit command in terminal mode, a
// refusal's fixes framed for the operator's position (cmd.Operator).
func executeTerminalCommit(mgr *EditorManager, username string, cmd cliCommand) string {
	answer, err := runWebCommit(mgr, username, cmd.Args, cmd.Operator)
	return terminalCommitOutput(answer, err)
}

func terminalCommitOutput(answer webCommitAnswer, err error) string {
	if err != nil {
		var tb textbuf.Buffer
		return tb.Str("error: ").Err(err).String()
	}
	if len(answer.conflicts) > 0 {
		return formatCommitConflicts("commit conflicts:\n", answer.conflicts)
	}
	if answer.refusal != "" {
		return answer.refusal
	}
	return answer.message
}

// runWebCommit parses and runs one commit subcommand for the terminal, its
// refusal's fixes framed for operator, the position the commit was typed at.
func runWebCommit(mgr *EditorManager, username string, args, operator []string) (webCommitAnswer, error) {
	req, err := webCommitRequest(args)
	if err != nil {
		return webCommitAnswer{}, err
	}
	return mgr.runCommit(username, req, operator)
}

// errDiscardPipe refuses `discard ... | <filter>`: discard prints no output a
// filter could act on, so the pipe would be read as a path segment and the
// refusal would name a path the operator never typed.
var errDiscardPipe = errors.New("discard takes no pipe: it prints no output to filter; " +
	"run `discard <path>` to drop the changes at a path, or `discard` to drop them all")

// runWebDiscard runs one discard for the terminal and the CLI bar alike.
// With no argument, or `all`, it reverts every pending change; with a path it
// drops only the changes at or under that path, the command a commit
// conflict's fix line prints. Its segments are config tokens, a member such
// as 192.0.2.2/30 included, so they are not checked as URL segments: a path
// naming no pending change is refused by EditorManager.DiscardPath, never
// read as done. Callers refuse structural pipes before passing the path;
// quoted pipes remain legitimate data inside a schema-addressed key.
func runWebDiscard(mgr *EditorManager, username string, args []string) (string, error) {
	if len(args) == 0 {
		return discardEveryChange(mgr, username)
	}
	if len(args) == 1 && args[0] == discardAll {
		return discardEveryChange(mgr, username)
	}
	if err := mgr.DiscardPath(username, args); err != nil {
		return "", err
	}
	var tb textbuf.Buffer
	return tb.Str("discarded: ").Join(args, " ").String(), nil
}

// discardEveryChange reverts every pending change of username, the answer to
// `discard` alone and to `discard all`.
func discardEveryChange(mgr *EditorManager, username string) (string, error) {
	if err := mgr.Discard(username); err != nil {
		return "", err
	}
	return terminalOutputChangesDiscarded, nil
}

// formatCommitConflicts renders the conflicts that refused a commit.
func formatCommitConflicts(heading string, conflicts []contract.Conflict) string {
	var msg textbuf.Buffer
	msg.Str(heading)
	for _, c := range conflicts {
		msg.Str("  ").Str(c.Path).Str(": want ").Quoted(c.MyValue).Str(", other (").Str(c.OtherUser).Str(") has ").Quoted(c.OtherValue).Byte('\n')
		if c.Fix != "" {
			msg.Str("    fix: ").Str(c.Fix).Byte('\n')
		}
	}
	return msg.String()
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
		pathStr := r.FormValue(webPathField)
		contextPath, pathErr := decodeConfigPath(pathStr)
		if pathErr != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		if mode == "terminal" {
			writeTerminalContent(w, renderer, contextPath)
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
func writeTerminalContent(w http.ResponseWriter, renderer *Renderer, contextPath []string) {
	writeHTML(w, string(renderer.renderComponent("terminal_content",
		terminalContent(formatCLIPrompt(contextPath)))))
}

// writeCLIResponse writes an HTMX multi-target response with content area,
// breadcrumb OOB swap, CLI prompt OOB swap, path bar OOB swap, and context
// path OOB swap.
func writeCLIResponse(w http.ResponseWriter, renderer *Renderer, path []string, viewData *ConfigViewData) {
	writeHTML(w, string(renderer.renderComponent("cli_response", cliResponse(cliResponseData{
		Prompt:      formatCLIPrompt(path),
		ContextPath: encodeConfigPath(path),
		Crumbs:      buildBreadcrumbs(path),
		View:        viewData,
		PathBar:     &FragmentData{CLIPathSegments: buildPathBarSegments(path)},
	}))))
}

// writeCLINotification writes only a notification OOB swap (for error responses).
func writeCLINotification(w http.ResponseWriter, renderer *Renderer, message, notifType string) {
	writeHTML(w, string(renderer.renderComponent("cli_notification_oob",
		cliNotificationOOB(cliNotificationData{Message: message, Kind: notifType}))))
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
		joined := encodeConfigPath(path[:i+1])
		segments[i] = PathBarSegment{
			Name:   seg,
			URL:    tb.Reset().Str(showPathPrefix).Str(joined).Byte('/').String(),
			HxPath: joined,
		}
	}

	return segments
}

// cliResponseData is what cliResponse renders: the content area a navigation
// command replaces, and the four out-of-band elements that follow the operator.
type cliResponseData struct {
	Prompt      string
	ContextPath string
	Crumbs      []BreadcrumbSegment
	View        *ConfigViewData
	PathBar     *FragmentData
}

// cliShowData is what cliShowResponse renders. Content is the configuration
// text the show verb read, already masked.
type cliShowData struct {
	Prompt      string
	ContextPath string
	Content     string
	Crumbs      []BreadcrumbSegment
	PathBar     *FragmentData
}

// cliNotificationData is what cliNotificationOOB renders. Kind is what the
// command reported, and notificationBarClass turns it into the bar's color.
type cliNotificationData struct {
	Message string
	Kind    string
}

// cliPageData is what cliPage renders.
type cliPageData struct {
	Prompt         string
	Mode           string
	ContextPath    string
	InitialCommand string
	LocalControls  []cliLocalOutputControl
}

// notificationBarClass is the notification bar's class list. Anything that is
// not an error reads as information, so a new kind never renders as a failure.
func notificationBarClass(kind string) string {
	if kind == "error" {
		return "notification-bar notification-error"
	}

	return "notification-bar notification-info"
}

// configLinkClass is the class list of a link to a child config node. The kind
// is in the class, so a container and a list look different in the browser.
func configLinkClass(kind string) string {
	var tb textbuf.Buffer

	return tb.Str("config-link config-link-").Str(kind).String()
}

// configKeyLinks resolves schema addresses before the links reach markup.
// The component receives only local typed display fields, never a schema node.
func configKeyLinks(path, keys []string, listNode *config.ListNode) []ChildEntry {
	links := make([]ChildEntry, 0, len(keys))
	for _, key := range keys {
		links = append(links, ChildEntry{Name: key, URL: configKeyURL(path, key, listNode)})
	}
	return links
}

// configKeyURL is the show URL of one list entry under path.
func configKeyURL(path []string, key string, listNodes ...*config.ListNode) string {
	if len(listNodes) > 0 && listNodes[0] != nil {
		return showPathPrefix + encodeConfigPath(append(append([]string{}, path...), config.ListKeyTokens(listNodes[0], key)...)) + "/"
	}
	var tb textbuf.Buffer

	tb.Str(showPathPrefix)

	if len(path) > 0 {
		tb.Str(encodeConfigPath(path)).Byte('/')
	}

	return tb.Str(url.PathEscape(key)).Byte('/').String()
}

// leafDisplayValue is what one leaf's value cell shows. An unset leaf shows its
// schema default, marked as such, so it never reads as a configured value.
func leafDisplayValue(f LeafField) string {
	if f.Value != "" || f.Default == "" {
		return f.Value
	}

	var tb textbuf.Buffer

	return tb.Str(f.Default).Str(" (default)").String()
}

// buildConfigEditURL constructs the /config/edit/ URL for a context path.
func buildConfigEditURL(path []string) string {
	if len(path) == 0 {
		return configEditPath
	}

	var tb textbuf.Buffer
	return tb.Str(configEditPath).Str(encodeConfigPath(path)).Byte('/').String()
}
