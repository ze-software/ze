// Design: docs/architecture/api/commands.md — where a command is served
// Detail: pipe.go — the chain this runs over a local answer
// Related: docs/architecture/api/commands.md — why a local command answers through the pipe layer
//
// local_data.go serves a command in THIS process and runs the pipe chain over
// its answer.
//
// 38 commands reached no pipe layer on any surface. For 18 there is no daemon
// RPC to reach at all. For the other 20 a wire method is declared in YANG and
// no daemon handler implements it, so `ze help command --json` published
// `global-pipes: true` while the daemon answered `unknown command`:
//
//	$ ze cli -c "show env list | json"
//	error: unknown command
//
// Their register files call MustRegisterLocal, so the handler printed text and
// returned an exit code and RunCommand never reached the pipe layer.
//
// The fix is one mechanism rather than 20 new daemon handlers: a command that
// answers with DATA is served here, and the same chain that renders a daemon
// answer renders this one. It also removes the dual-registration asymmetry as a
// side effect, because both forms of a command now run one chain over one
// payload.

package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// LocalDataHandler answers a command with structured DATA instead of printing
// text, so the answer can go through the pipe layer like any other.
//
// It takes the value ValidateArgs returned, never a token slice: only a
// successful judgment of the arguments against the leaves the command's YANG
// declares builds one, so no route can run a data handler on tokens nobody
// judged. Both routes that run one call the validator first: ServeLocal, and
// InvokeLocal for the plain local handler RegisterLocalData builds.
//
// A LocalHandler prints and returns an exit code, which is why 38
// commands reached no pipe layer on any surface: by the time RunCommand had a
// result there was nothing left but an int, and `ze cli -c "show env list |
// json"` answered `unknown command` because the daemon serves no such method.
// A data handler returns the payload, and the caller renders it.
//
// The value MUST be structured data a JSON encoder can take: a map, a slice, or
// a struct. It MUST NOT be text a renderer already formatted, for the reason
// ai/rules/cli.md gives for every other handler: `| json`, `| yaml` and
// `| table` are three renderings of ONE payload.
//
// The two results are INDEPENDENT. The payload says whether there is an answer
// to render, the code says what the process exits with, and a command MAY have
// both: `validate config` answers the diagnostics of a config it rejects and
// exits 1. A handler with nothing to say MUST write its reason to stderr and
// return a nil payload.
type LocalDataHandler func(args ValidatedArgs) (any, int)

// localData holds the commands that answer with data in this process. It lives
// here rather than in the registry package because its handler type names
// ValidatedArgs, which only this package can build, and the registry cannot
// import this package. Safe for concurrent use.
var localData = struct {
	sync.RWMutex
	handlers map[string]LocalDataHandler
}{handlers: make(map[string]LocalDataHandler)}

// errLocalDataEmptyPath is the refusal of a data handler registered with no
// command path.
var errLocalDataEmptyPath = errors.New("command.RegisterLocalData: empty path")

// RegisterLocalData registers a command that answers with structured data in
// this process, so its answer reaches the pipe layer.
//
// It ALSO registers a plain local handler (RegisterLocalMeta), built
// from the same data handler, so `ze <verb>` prints exactly what it printed
// before and the two forms of one command cannot drift apart. That drift is
// real: `ze show interface` took the local path and `ze cli -c "show
// interface"` took the daemon's, and only the second honored a pipe.
func RegisterLocalData(path string, handler LocalDataHandler, meta registry.Meta, render func(string, any) int) error {
	if path == "" {
		return errLocalDataEmptyPath
	}
	if handler == nil {
		return fmt.Errorf("command.RegisterLocalData: nil handler for %q", path)
	}
	if render == nil {
		return fmt.Errorf("command.RegisterLocalData: nil renderer for %q", path)
	}
	if err := RegisterLocalMeta(path, plainLocalData(path, handler, render), meta); err != nil {
		return err
	}
	localData.Lock()
	defer localData.Unlock()
	localData.handlers[path] = handler
	return nil
}

// plainLocalData builds the `ze <verb>` form of a data command: run the
// handler on the value its route judged, render its payload.
//
// The arguments are judged before this runs, by InvokeLocal on every route
// that serves a local handler, as ServeLocal judges them on the `ze cli -c`
// route. This route once ran the handler on raw words, so `ze show env get`
// with a 129-character key reached it although the leaf declares 1..128.
func plainLocalData(path string, handler LocalDataHandler, render func(string, any) int) LocalHandler {
	return func(validated ValidatedArgs) int {
		// A nonzero code with a payload is an ANSWER the command exits
		// nonzero on, not an error with nothing to say: `validate config`
		// renders the diagnostics of a config it rejects and exits 1. The
		// renderer's own failure wins, because then nothing was printed.
		payload, code := handler(validated)
		if payload == nil {
			return code
		}
		if renderCode := render(path, payload); renderCode != 0 {
			return renderCode
		}
		return code
	}
}

// MustRegisterLocalData is RegisterLocalData, and panics rather than letting a
// command register half.
func MustRegisterLocalData(path string, handler LocalDataHandler, meta registry.Meta, render func(string, any) int) {
	err := RegisterLocalData(path, handler, meta, render)
	if err == nil {
		return
	}
	// The detail goes to stderr and the panic value is a literal. A registration
	// failure is a programming error at init, so the process must stop, and the
	// error already names the path that could not register.
	writeLocalRefusal(err)
	panic("BUG: command.MustRegisterLocalData")
}

// LookupLocalData answers the data handler for a command path, by longest
// registered prefix, with the words that follow it as its arguments. The
// caller MUST judge those arguments (ValidateModelArgs, with the registered
// path) and MUST invoke the handler only with the value that judgment
// returned.
func LookupLocalData(words []string) (LocalDataHandler, []string) {
	localData.RLock()
	defer localData.RUnlock()
	for i := len(words); i > 0; i-- {
		if handler, ok := localData.handlers[textbuf.Join(words[:i], " ")]; ok {
			return handler, words[i:]
		}
	}
	return nil, nil
}

// ResetLocalDataForTest clears every registered data handler.
func ResetLocalDataForTest() {
	localData.Lock()
	defer localData.Unlock()
	localData.handlers = make(map[string]LocalDataHandler)
}

// ServeLocal answers a command in this process when a local data handler covers
// it, rendering the answer through the pipe chain the operator typed.
//
// served is false when no local data handler covers the command, and the caller
// then dispatches as it did before. Nothing about this changes which command
// wins: a path with no data handler is untouched.
//
// The chain is expanded LOCALLY, so `| save` is allowed: this is the operator's
// own process writing as the operator.
func ServeLocal(input, sessionFormat string) (answer string, code int, served bool) {
	path, _ := parsePipeChain(input)
	words := strings.Fields(path)
	handler, args := LookupLocalData(words)
	if handler == nil {
		return "", 0, false
	}
	// The chain is validated against the command's DECLARED shape before source
	// work starts, so a command that says it holds one document refuses a row
	// operator here rather than answering something the published catalog says
	// it does not support.
	_, format, errMsg := ProcessPipesDefaultFormatLocal(input, sessionFormat)
	if errMsg != "" {
		return pipeError(errMsg), 1, true
	}

	// The arguments are judged against the leaves the command's YANG declares,
	// by the validator the daemon dispatcher calls, before the handler runs.
	// This route once called no validator at all, so a declared length,
	// pattern or range constrained nothing here: `show env get` with a
	// 129-character name reached the handler although the leaf says 1..128.
	validated, argErr := validateLocalArgs(words, args)
	if argErr != nil {
		writeLocalRefusal(argErr)
		return "", 1, true
	}

	// The PAYLOAD decides whether there is an answer to render, and the CODE
	// decides what the process exits with. They are independent: `validate
	// config` answers the diagnostics of a config it rejects and exits 1, so a
	// handler that returns both MUST have both honored. A handler with nothing
	// to say has already written its reason to stderr and returns a nil
	// payload.
	payload, code := handler(validated)
	if payload == nil {
		return "", code, true
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		var tb textbuf.Buffer
		return pipeError(tb.Str("the answer could not be encoded: ").Str(err.Error()).String()), 1, true
	}

	rendered := format(string(encoded))
	if IsPipeError(rendered) {
		return rendered, 1, true
	}
	return rendered, code, true
}

// ArgDefSource answers the argument definitions the YANG model declares for a
// command path, nil when the path declares none, or an error naming why the
// model could not answer.
type ArgDefSource func(path string) ([]ArgDef, error)

// argDefSource is the model's answer for the local route. The YANG package
// registers it (config/yang register.go), because that package already imports
// this one and so this one cannot import it.
var argDefSource ArgDefSource

// RegisterArgDefSource installs the source every route outside the daemon
// dispatcher reads argument definitions from (ValidateModelArgs), which the
// `ze <verb>` and offline-fallback routes reach through InvokeLocal. One call
// covers every route, so none can be left unvalidated. Called from init();
// not safe for concurrent use with any route.
func RegisterArgDefSource(source ArgDefSource) {
	argDefSource = source
}

// validateLocalArgs judges the arguments of a local-data command.
//
// words is the command as typed and args the tail LookupLocalData left after
// the registered path, so the path is the words before that tail.
func validateLocalArgs(words, args []string) (ValidatedArgs, error) {
	return ValidateModelArgs(strings.Join(words[:len(words)-len(args)], " "), args, nil)
}

// ValidateModelArgs judges args against the leaves the model declares for the
// command path, through ValidateArgs, and is how every route that holds no
// definitions of its own validates: the local routes, SSH streaming, the
// RPC wrapper and plugin forwarding. preMatched is ValidateArgs's: the values
// a route already bound by keyword or selector, or nil.
//
// A path the model declares nothing for is validated against no definitions,
// which passes its tokens through. A process with no source registered refuses
// rather than skipping the check, because a skipped check and a passed one
// look the same to the operator.
func ValidateModelArgs(path string, args []string, preMatched map[string]string) (ValidatedArgs, error) {
	if argDefSource == nil {
		return ValidatedArgs{}, errNoArgDefSource
	}
	defs, err := argDefSource(path)
	if err != nil {
		return ValidatedArgs{}, fmt.Errorf("argument definitions of %q: %w", path, err)
	}
	return ValidateArgs(args, defs, preMatched)
}

// errNoArgDefSource is the refusal of a process that registered no argument
// definition source.
var errNoArgDefSource = errors.New("argument definitions are not loaded in this process")

// writeLocalRefusal writes an argument refusal to stderr, where a local-data
// handler writes its own refusals, so the operator reads both on one channel.
func writeLocalRefusal(err error) {
	var tb textbuf.Buffer
	os.Stderr.WriteString(tb.Str("error: ").Str(err.Error()).Byte('\n').String()) //nolint:errcheck // CLI diagnostic
}

// HasLocalData reports whether a command is served in this process, which is
// what the published catalog reads to say the command reaches the pipe layer.
func HasLocalData(path string) bool {
	handler, _ := LookupLocalData(strings.Fields(path))
	return handler != nil
}

// RenderLocalAnswer prints a local command's answer in the configured default
// format and answers the exit code.
//
// It is what a data handler's `ze <verb>` form uses, so the two forms of one
// command render through the same code and cannot drift apart.
func RenderLocalAnswer(path string, payload any) int {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 1
	}
	// The PATH is passed, not a placeholder: the renderers read the column
	// order the command declared, and a placeholder resolves to none, so every
	// column comes out alphabetical.
	_, format, errMsg := ProcessPipesDefaultFormatLocal(path, "")
	if errMsg != "" {
		return 1
	}
	rendered := format(string(encoded))
	if IsPipeError(rendered) {
		return 1
	}
	WriteAnswer(rendered)
	return 0
}

// WriteAnswer prints a rendered answer, ending it with exactly one newline.
//
// Every surface that prints a locally served answer MUST come through here.
// The `ze <verb>` and `ze cli -c` spellings of one command are two call sites
// in two packages, and a tool author is told they answer alike. While the CLI
// client used fmt.Println over the same rendered string, they did not: a table
// rendering already ends in a newline, so that surface added a second one, and
// `wc -l` disagreed by one between two spellings of one command.
func WriteAnswer(rendered string) {
	if rendered == "" {
		return
	}
	os.Stdout.WriteString(rendered) //nolint:errcheck // CLI output
	if !strings.HasSuffix(rendered, "\n") {
		os.Stdout.WriteString("\n") //nolint:errcheck // CLI output
	}
}
