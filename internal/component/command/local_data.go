// Design: docs/architecture/api/commands.md — where a command is served
// Detail: pipe.go — the chain this runs over a local answer
// Related: plan/spec-cli-pipe-operator-coverage.md — AC-10, AC-11
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

	payload, code := handler(args)
	if code != 0 {
		return "", code, true
	}
	if payload == nil {
		return "", 0, true
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		var tb textbuf.Buffer
		return pipeError(tb.Str("the answer could not be encoded: ").Str(err.Error()).String()), 1, true
	}

	_, format, errMsg := ProcessPipesDefaultFormatLocal(input, sessionFormat)
	if errMsg != "" {
		return pipeError(errMsg), 1, true
	}
	rendered := format(string(encoded))
	if IsPipeError(rendered) {
		return rendered, 1, true
	}
	return rendered, 0, true
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
	writeAnswer(rendered)
	return 0
}

// writeAnswer prints a rendered answer, ending it with exactly one newline.
func writeAnswer(rendered string) {
	if rendered == "" {
		return
	}
	os.Stdout.WriteString(rendered) //nolint:errcheck // CLI output
	if !strings.HasSuffix(rendered, "\n") {
		os.Stdout.WriteString("\n") //nolint:errcheck // CLI output
	}
}
