// Design: docs/architecture/core-design.md -- le's registration adapter
// Detail: dispatch.go -- the loop that runs the commands registered here
// Detail: group.go -- the five groups a registration names, and their order
//
// Package leroot is how an le tool joins the shared command engine. It declares
// no registry, grammar, or operator of its own. Each tool registers structured
// data at its full `le <tool>` path in the command registry.
//
// Run preserves the tool verdict while it renders the payload. This differs
// from the general local-data shortcut, which returns before rendering a
// payload when the handler reports a nonzero code.
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

const pathPrefix = "le "

// pipeWord separates a tool's own arguments from the operator chain. Dispatch
// and splitChain both need to know where the command ends, so the word is
// spelled once: two readers of one contract cannot drift apart, and a second
// place that decides where a chain begins is how a lookup and a split come to
// disagree.
const pipeWord = "|"

// Owned answers every le command name from the shared local-data registry.
func Owned() []string {
	commands := registry.ListLocal()
	owned := make([]string, 0, len(commands))
	for _, entry := range commands {
		if name, ok := strings.CutPrefix(entry.Path, pathPrefix); ok {
			owned = append(owned, name)
		}
	}
	return owned
}

// Owns reports whether le registered name at its canonical local-data path.
func Owns(name string) bool {
	return registry.HasLocal(CommandPath(name))
}

// Register wires one tool into the shared registry at `le <name>`.
//
// group says what the tool is for, and help renders the commands under it
// (group.go). It is a parameter rather than a field of Meta, so a new tool
// that names no group does not compile.
//
// Meta MUST carry a Description, a Mode, and a Section. A registration missing
// one is a programming error at init, so it panics instead of publishing blank
// help.
func Register(name string, group Group, answer Answer, meta registry.Meta) {
	if answer == nil {
		panic("BUG: leroot.Register: nil answer; see the init frame above for the tool")
	}
	if !KnownGroup(group) {
		panic("BUG: leroot.Register: unknown group; see group.go for the five le renders")
	}
	if meta.Description == "" || meta.Mode == "" || meta.Section == "" {
		panic("BUG: leroot.Register: Meta needs Description, Mode and Section")
	}
	setGroup(name, group)
	registry.MustRegisterLocalData(
		CommandPath(name),
		registry.LocalDataHandler(answer),
		meta,
		command.RenderLocalAnswer,
	)
}

// CommandPath answers the canonical local-data path for one le tool.
func CommandPath(name string) string {
	var tb textbuf.Buffer
	return tb.Str(pathPrefix).Str(name).String()
}

// LookupCommand answers the handler registered for one whole command name.
//
// A name may hold a space once a family is a namespace, so the name is split
// into path words rather than passed as one. Trailing words mean the name is a
// prefix of a registered command rather than a command, and that is not a
// match.
//
// This is the only resolver. Dispatch reaches it from argv through resolve,
// and the verification dispatcher reaches it from a stage Identity here, so no
// second place decides what a registered name means. A lookup and its refusal
// spelled twice is how the two come to disagree.
func LookupCommand(name string) registry.LocalDataHandler {
	handler, trailing := registry.LookupLocalData(strings.Fields(CommandPath(name)))
	if len(trailing) != 0 {
		return nil
	}
	return handler
}

// RegisterShape declares the answer shape at the same full path as Register.
// RegisterShape's slice holds complete command paths, not path words.
func RegisterShape(name string, shape command.AnswerShape) {
	command.RegisterShape([]string{CommandPath(name)}, shape)
}

// Run is the whole of an le command's behavior: split the operator's pipe
// chain off the tool's own arguments, run the tool, render its payload through
// the chain, and answer the tool's exit code.
//
// out and errOut are parameters so a test drives the same code path the binary
// runs, rather than a copy of it.
func Run(name string, answer Answer, args []string, out, errOut io.Writer) int {
	toolArgs, pipeStr := splitChain(args)

	input := CommandPath(name)
	if pipeStr != "" {
		var tb textbuf.Buffer
		input = tb.Str(input).Str(" | ").Str(pipeStr).String()
	}

	// A command-owned pipe filter folds its arguments back into the command
	// path. Resolve that path through the same local-data registry rather than
	// assuming how many command words precede those arguments.
	resolved, format, errMsg := command.ProcessPipesDefaultFormatLocal(input, "")
	if errMsg != "" {
		// textbuf rather than Fprintf: `errOut` is injectable so tests can
		// capture it, which puts it outside c_sprintf_new's os.Stderr
		// exemption. Building the line and printing it satisfies both the
		// check and `performance.md`, and keeps the writer a seam.
		var tb textbuf.Buffer
		fmt.Fprintln(errOut, tb.Str("error: ").Str(errMsg).String()) //nolint:errcheck // CLI output
		return 1
	}
	if _, foldedArgs := registry.LookupLocalData(strings.Fields(resolved)); len(foldedArgs) != 0 {
		toolArgs = append(toolArgs, foldedArgs...)
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
		if arg != pipeWord {
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
