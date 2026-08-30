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
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

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
	handler, args := registry.LookupLocalData(strings.Fields(path))
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

	// The PAYLOAD decides whether there is an answer to render, and the CODE
	// decides what the process exits with. They are independent: `validate
	// config` answers the diagnostics of a config it rejects and exits 1, so a
	// handler that returns both MUST have both honored. A handler with nothing
	// to say has already written its reason to stderr and returns a nil
	// payload.
	payload, code := handler(args)
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

// HasLocalData reports whether a command is served in this process, which is
// what the published catalog reads to say the command reaches the pipe layer.
func HasLocalData(path string) bool {
	handler, _ := registry.LookupLocalData(strings.Fields(path))
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
