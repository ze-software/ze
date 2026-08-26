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
	"slices"
	"strings"
	"sync"

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

// owned names every root command le itself registered, in registration order.
//
// It exists because le LINKS the product. A command inventory and a CLI-grammar
// check cannot be computed without loading the registry they judge, so cmd/le
// blank-imports internal/component/plugin/all through the tools that need it,
// and every product command that registers a root at init lands in this
// process's registry beside le's own. Five do today: env, interface, plugin,
// schema and sysctl.
//
// The registry is still the one owner of a name, which is what makes a
// collision, which AC-13 pins. What this list adds is the answer to a different
// question: which of those names is LE's. `le interface` must be an unknown
// command rather than ze's interface editor, so dispatch asks here before it
// looks a name up (cmd/le/dispatch.go, AC-3).
var (
	ownedMu sync.Mutex
	owned   []string
)

// Owned answers every root command name le registered, in registration order.
// The caller receives a copy, so a tool cannot edit le's command set by
// editing what it was handed.
func Owned() []string {
	ownedMu.Lock()
	defer ownedMu.Unlock()
	return slices.Clone(owned)
}

// Owns reports whether le registered name. A name in the shared registry that
// le did not register belongs to another program, and le does not run it.
func Owns(name string) bool {
	ownedMu.Lock()
	defer ownedMu.Unlock()
	return slices.Contains(owned, name)
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

	// After the registry has accepted the name, never before: a duplicate
	// panics above, and a name le failed to register is not a name le owns.
	ownedMu.Lock()
	owned = append(owned, name)
	ownedMu.Unlock()
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

// RefuseArgument reports a value typed after a command that takes none, and
// answers the code it exits with.
//
// Every le tool judges the checkout it is run in, and the rendering is a pipe
// operator, so a tool that is one gate rather than an area has no argument to
// take at all (the CLI rule: keyword before value). The refusal is stated here
// because fourteen such tools owe it, and fourteen hand-written copies is where
// they begin to disagree about what a developer may type.
//
// leaction.refuseValue is the same refusal for an action of an area, where the
// message must also name the action.
func RefuseArgument(name, got string) int {
	var tb textbuf.Buffer
	fmt.Fprintln(os.Stderr, tb.Str("error: ").Str(name).Str(" takes no arguments, got ").Quoted(got).String()) //nolint:errcheck // CLI output
	tb.Reset()
	fmt.Fprintln(os.Stderr, tb.Str("usage: le ").Str(name).Str(" [| json | yaml | table]").String()) //nolint:errcheck // CLI output
	return 1
}
