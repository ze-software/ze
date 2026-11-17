// Design: docs/architecture/web-interface.md -- browser terminal streams.
// Related: cli_terminal.go -- ordinary commands share the same terminal route.

package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

const cliStreamOutputBytesMax = 4 * 1024 * 1024

const streamCommandNotRunWhy = "The command was not run."

var errCLIStreamOutputLimit = opererr.New("Live output reached its 4 MiB limit.",
	"The terminal stopped this live output to keep the browser response bounded.",
	"Open /cli and run the live command again to continue")

// HandleCLITerminalStream adds streaming to the existing terminal handler.
// The caller MUST install the same authentication and same-origin middleware as
// the ordinary terminal. Each stream lives for one HTTP request; canceling the
// request MUST cancel the producer. This handler owns no background workers.
func HandleCLITerminalStream(mgr *EditorManager, terminal http.Handler, dispatch plugin.CommandStreamDispatcher, registered ...func(string) bool) http.HandlerFunc {
	return handleCLITerminalStream(mgr, terminal, dispatch, nil, registered...)
}

func handleCLITerminalStream(mgr *EditorManager, terminal http.Handler, dispatch plugin.CommandStreamDispatcher, views *CLIViewTransport, registered ...func(string) bool) http.HandlerFunc {
	var recognizes func(string) bool
	if len(registered) > 0 {
		recognizes = registered[0]
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, opererr.New("The live command was not submitted through the terminal.",
				streamCommandNotRunWhy,
				"Open /cli and press Enter to submit the command").Error(), http.StatusMethodNotAllowed)
			return
		}
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, opererr.New("Your browser session is not signed in.",
				"Live commands require an authenticated account.",
				"Sign in at /login, then return to /cli").Error(), http.StatusUnauthorized)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, editorRequestMax)
		if err := r.ParseForm(); err != nil {
			http.Error(w, opererr.New("The terminal could not read the submitted command.",
				streamCommandNotRunWhy,
				"Open /cli and submit the command again without a file or pasted configuration").Error(), http.StatusBadRequest)
			return
		}
		line := r.FormValue("command")
		if len(line) > maxCommandLength {
			http.Error(w, opererr.New("The submitted live command is too long.",
				streamCommandNotRunWhy,
				"Open /cli and enter a shorter command").Error(), http.StatusBadRequest)
			return
		}
		registeredInput := frozenCLIRegistration(r, line, recognizes)
		path, pathErr := decodeConfigPath(r.FormValue(webPathField))
		if pathErr != nil {
			http.Error(w, opererr.Wrap(pathErr, "The submitted configuration path is invalid.",
				streamCommandNotRunWhy,
				"Open /cli and submit the command again from the configuration root").Error(), http.StatusBadRequest)
			return
		}
		mode, validMode := submittedCLIMode(w, r)
		if !validMode {
			return
		}
		r = recordCLIHistory(w, r, mgr, mode, path, line, registeredInput)
		input, operational := cliStreamInput(mode, parseCLICommand(line), registeredInput)
		if !operational {
			terminal.ServeHTTP(w, r)
			return
		}
		if views.stream(w, r, input) {
			return
		}
		if dispatch == nil {
			terminal.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		writer := newCLIStreamWriter(w, cancel)
		var output io.Writer = writer
		var frames *cliOutputWriter
		if acceptsCLIOutput(r) {
			frames = newCLIOutputWriter(w, ctx, cancel)
			output = frames
		}
		handled, err := dispatch(ctx, plugin.CallerIdentity{
			Username: username, RemoteAddr: r.RemoteAddr,
			Authorizer: plugin.CallerAuthorizer(r.Context()),
		}, input, output)
		if !handled {
			terminal.ServeHTTP(w, r)
			return
		}
		if frames != nil {
			if finishErr := frames.finish(r.Context(), err); finishErr != nil {
				return
			}
			return
		}
		if err == nil {
			err = writer.failure
		}
		if r.Context().Err() != nil {
			return
		}
		if err != nil {
			refusal := cliStreamRefusal(err)
			if !writer.started {
				status := http.StatusBadRequest
				if errors.Is(err, plugin.ErrStreamUnauthorized) {
					status = http.StatusForbidden
				}
				http.Error(w, refusal.Error(), status)
				return
			}
			// The response has started, so an error must travel as visible output
			// rather than a second status code. This fixed-size trailer sits
			// outside the producer's budget, including an output-limit refusal.
			trailer := cliStreamRefusalText(refusal)
			if _, writeErr := fmt.Fprintf(w, "\nerror: %s\n", trailer); writeErr != nil {
				return
			}
			if flushErr := http.NewResponseController(w).Flush(); flushErr != nil {
				return
			}
			return
		}
		if !writer.started {
			writer.start()
		}
	}
}

// cliStreamRefusal gives transport failures browser recovery instructions.
// The producer's structured refusal keeps its own policy or command fix.
func cliStreamRefusal(err error) *opererr.Error {
	if refusal, ok := errors.AsType[*opererr.Error](err); ok {
		return refusal
	}
	if errors.Is(err, plugin.ErrStreamUnauthorized) {
		return opererr.Wrap(err,
			"Your account cannot run this live command.",
			"The command was not started because this account lacks permission",
			"Sign in at /login with an account permitted to run this command")
	}
	return opererr.Wrap(err,
		"The live command could not continue.",
		"No further live output can be shown for this request",
		"Open /cli and run the live command again")
}

// cliStreamRefusalText bounds details before the complete recovery line.
// An oversized recovery cannot be printed intact, so it names that limit and
// gives a browser action instead of displaying a broken partial command.
func cliStreamRefusalText(refusal *opererr.Error) string {
	text := strings.ToValidUTF8(refusal.Error(), "�")
	if len(text) <= maxCommandLength {
		return text
	}
	const clippedDetails = "\nThe error details were truncated."
	fix := "\nfix: " + strings.ToValidUTF8(refusal.Fix(), "�")
	if len(fix) >= maxCommandLength-len(clippedDetails) {
		return opererr.New("The live command failed and its recovery instructions exceed the display limit.",
			"The complete instructions could not fit in this response.",
			"Open /cli and submit a shorter live command").Error()
	}
	end := maxCommandLength - len(clippedDetails) - len(fix)
	for end > 0 {
		if utf8.RuneStart(text[end]) {
			break
		}
		end--
	}
	return text[:end] + clippedDetails + fix
}

// cliStreamInput follows the terminal's existing mode grammar. Config commands
// stay in the ordinary executor; only a run tail or operational command can
// reach the streaming registry. Each top consumes a word, bounded by the command
// length checked at the HTTP boundary.
func cliStreamInput(mode string, cmd cliCommand, registered ...func(string) bool) (string, bool) {
	var recognizes func(string) bool
	if len(registered) > 0 {
		recognizes = registered[0]
	}
	if cmd.parseErr != nil {
		return "", false
	}
	configMode := mode != terminalModeOperational
	for {
		inner, underTop := commandBehindTop(cmd)
		if !underTop {
			break
		}
		cmd = inner
		configMode = true
	}
	if _, lifecycle := contract.LifecycleCommand(cmd.words.String()); lifecycle {
		return "", false
	}
	if cmd.words.String() == verbClear {
		return "", false
	}
	if cmd.Verb == verbRun && configMode {
		if len(cmd.Args) == 0 {
			return "", false
		}
		if cmd.words.Tail().String() == verbClear {
			return "", false
		}
		return cmd.words.Tail().String(), true
	}
	if configMode {
		return "", false
	}
	if cli.ShouldEnterConfigMode(cmd.words.String(), recognizes) {
		return "", false
	}
	switch cmd.Verb {
	case verbConfigure, verbExit, verbQuit, verbHelp:
		return "", false
	}
	return cmd.words.String(), true
}

// cliStreamWriter sends bounded producer output directly to the browser. A
// producer MUST stop on a write error; no event queue or retained server buffer
// grows with stream duration. It is used by one request goroutine only.
type cliStreamWriter struct {
	response  http.ResponseWriter
	remaining int
	started   bool
	cancel    context.CancelFunc
	failure   error
}

func newCLIStreamWriter(w http.ResponseWriter, cancel context.CancelFunc) *cliStreamWriter {
	return &cliStreamWriter{response: w, remaining: cliStreamOutputBytesMax, cancel: cancel}
}

func (w *cliStreamWriter) start() {
	w.response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.response.Header().Set("X-Accel-Buffering", "no")
	w.response.WriteHeader(http.StatusOK)
	w.started = true
}

func (w *cliStreamWriter) Write(p []byte) (int, error) {
	if w.failure != nil {
		return 0, w.failure
	}
	if len(p) > w.remaining {
		w.failure = errCLIStreamOutputLimit
		w.cancel()
		return 0, errCLIStreamOutputLimit
	}
	if !w.started {
		w.start()
	}
	n, err := w.response.Write(p)
	w.remaining -= n
	if err != nil {
		w.failure = err
		w.cancel()
		return n, err
	}
	if err := http.NewResponseController(w.response).Flush(); err != nil {
		w.failure = err
		w.cancel()
		return n, err
	}
	return n, nil
}

var _ io.Writer = (*cliStreamWriter)(nil)
