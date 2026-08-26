// Design: docs/architecture/core-design.md -- le's registration adapter
//
// Package leroot is how an le tool joins the shared command engine. It declares
// no registry, no grammar and no operator of its own: every piece of the
// command contract comes from internal/component/command and its registry
// package, which is the same engine cmd/ze dispatches through.
//
// What it adds is the one adapter le needs. A tool answers structured data,
// because `| json`, `| yaml` and `| table` are three renderings of ONE payload
// (ai/rules/cli.md). The engine's root-handler contract is an int, so this
// package holds the conversion in one place instead of once per tool.
package leroot

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Answer is what an le tool implements. The payload MUST be structured data a
// JSON encoder can take: a map, a slice, or a struct. It MUST NOT be text a
// renderer already formatted, because the caller chooses the rendering.
//
// The code is the tool's verdict and it reaches the process exit status
// unchanged. A gate that fails with 3 exits 3.
type Answer func(args []string) (payload any, code int)

// Prose is a payload that has a rendering of its own for a person to read.
//
// A tool whose answer is a REPORT has one: a severity list grouped by check,
// colored, with a summary line, is what its reader expects and what the gate
// printed before it was a command. The engine renders rows as a table, which is
// the right answer for an inventory and the wrong one for a report, so a
// payload that knows better says so by implementing this.
//
// It is the DEFAULT rendering and nothing more. `| json`, `| yaml`, `| table`
// and every data operator go to the engine exactly as they do for a payload
// that implements nothing, because the operator who typed one has chosen the
// rendering. The payload stays structured either way: Text is a second reading
// of the same data, never a substitute for it, and a tool that answers finished
// text instead of a payload has broken ai/rules/cli.md rather than satisfied it.
//
// The precedent is pluginserver.RegisterMonitorEventFormatter, which gives ze's
// monitor commands a compact default in place of the table.
type Prose interface {
	// Text renders the whole answer, ending in a newline. It is called only
	// when the operator typed no pipe chain.
	Text() string
}

// Register wires one tool into the shared registry as a root command, the way
// internal/perf/cli does for ze-perf. The tool is then reachable by name, it
// appears in help and in completion, and nothing else has to be edited.
//
// Meta MUST carry a Description, a Mode and a Section. A registration missing
// one of them is a programming error at init, so it panics rather than leaving
// a command that renders blank in every help page.
func Register(name string, answer Answer, meta registry.Meta) {
	// Both panics are reachable only from a Ze defect at init -- a tool
	// registering nothing, or a Meta that would render blank in every help
	// page -- never from anything a peer or an operator sends. That is what
	// `ze-go-style.md` asks before a panic may stand.
	//
	// The message carries no `name`, deliberately. Interpolating it needs
	// either `+`, which `performance.md` bans, or a textbuf chain, which stops
	// the message being the string literal `c_panic` requires a BUG panic to
	// open with. The panic fires during `init()`, so the stack trace names the
	// registering package on the line above -- which identifies the offending
	// tool more precisely than its command name would.
	if answer == nil {
		panic("BUG: leroot.Register: nil answer; see the init frame above for the tool")
	}
	if meta.Description == "" || meta.Mode == "" || meta.Section == "" {
		panic("BUG: leroot.Register: Meta needs Description, Mode and Section")
	}
	registry.MustRegisterRootHandler(name, func(_ *registry.RuntimeContext, args []string) int {
		return Run(name, answer, args, os.Stdout, os.Stderr)
	}, meta)
}

// Run is the whole of an le command's behavior: split the operator's pipe
// chain off the tool's own arguments, run the tool, render its payload through
// the chain, and answer the tool's exit code.
//
// out and errOut are parameters so a test drives the same code path the binary
// runs, rather than a copy of it.
func Run(name string, answer Answer, args []string, out, errOut io.Writer) int {
	toolArgs, pipeStr := splitChain(args)

	input := name
	if pipeStr != "" {
		var tb textbuf.Buffer
		input = tb.Str(name).Str(" | ").Str(pipeStr).String()
	}

	// The engine folds a command-owned pipe filter back into the command
	// string, so anything it returns beyond the name is an argument the tool
	// must see. No le tool registers such a filter yet; reading the answer
	// rather than assuming it is what keeps that true when one does.
	resolved, format, errMsg := command.ProcessPipesDefaultFormatChecked(input, "")
	if errMsg != "" {
		// textbuf rather than Fprintf: `errOut` is injectable so tests can
		// capture it, which puts it outside c_sprintf_new's os.Stderr
		// exemption. Building the line and printing it satisfies both the
		// check and `performance.md`, and keeps the writer a seam.
		var tb textbuf.Buffer
		fmt.Fprintln(errOut, tb.Str("error: ").Str(errMsg).String()) //nolint:errcheck // CLI output
		return 1
	}
	if folded := strings.Fields(resolved); len(folded) > 1 {
		toolArgs = append(toolArgs, folded[1:]...)
	}

	payload, code := answer(toolArgs)
	if payload == nil {
		return code
	}

	// A payload that renders itself is rendered by itself when the operator
	// asked for nothing else. See Prose: this is the DEFAULT rendering, and
	// typing any operator hands the answer back to the engine.
	if prose, ok := payload.(Prose); ok && pipeStr == "" {
		text := prose.Text()
		io.WriteString(out, text) //nolint:errcheck // CLI output
		if text != "" && !strings.HasSuffix(text, "\n") {
			io.WriteString(out, "\n") //nolint:errcheck // CLI output
		}
		return code
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		var tb textbuf.Buffer
		message := tb.Str("error: ").Str(name).
			Str(" answered a payload that does not encode: ").Err(err).String()
		fmt.Fprintln(errOut, message) //nolint:errcheck // CLI output
		return 1
	}

	result := format(string(raw))
	// A chain can be refused AFTER the tool has run, when the answer's shape
	// cannot support an operator. The caller then holds a refusal rather than
	// data, so it goes to stderr and the status says so. This is not the
	// flattening AC-8 bans: the tool's verdict is not what failed here.
	if command.IsPipeError(result) {
		fmt.Fprintln(errOut, result) //nolint:errcheck // CLI output
		return 1
	}

	io.WriteString(out, result) //nolint:errcheck // CLI output
	if result != "" && !strings.HasSuffix(result, "\n") {
		io.WriteString(out, "\n") //nolint:errcheck // CLI output
	}
	return code
}

// splitChain divides argv at the first pipe operator: what precedes it is the
// tool's own arguments, and what follows is the chain the operator typed. The
// bar arrives as its own word because a shell would otherwise consume it, so
// `le parity | json` is typed as `le parity '|' json`.
func splitChain(args []string) (toolArgs []string, pipeStr string) {
	for i, arg := range args {
		if arg != "|" {
			continue
		}
		var tb textbuf.Buffer
		return args[:i:i], tb.Join(args[i+1:], " ").String()
	}
	return args, ""
}
