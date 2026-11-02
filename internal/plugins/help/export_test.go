package help

import (
	"io"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

// The tests live in package help_test, because they read the catalog over
// every registered schema and plugin/all, which they import, would cycle back
// into this package. These names hand them the unexported functions. The
// renderers' diagnostics went to stderr before the move and no test read
// them, so the wrappers that keep the earlier signatures discard them.

type (
	CommandEntry    = commandEntry
	CatalogAnswer   = catalogAnswer
	ReferenceAnswer = referenceAnswer
)

const FlagJSON = flagJSON

var (
	CollectCommands      = collectCommands
	DaemonHandlesPath    = daemonHandlesPath
	ExtractCommandFilter = extractCommandFilter
	FilterCommands       = filterCommands
	FindNode             = findNode
	HasSection           = hasSection
	OperatorsFor         = operatorsFor
	PluginUsage          = pluginUsage
	RenderTo             = renderTo
	DrawCatalog          = drawCatalog
	AnswerHelpCommand    = answerHelpCommand
	AnswerAIHelp         = answerAIHelp
)

func PrintCommandJSON(w io.Writer, entries []commandEntry) int {
	return printCommandJSON(w, io.Discard, entries)
}

func PrintCommandVerbose(rw *helpfmt.RenderWriter, entries []commandEntry) {
	printCommandVerbose(rw, io.Discard, entries)
}

func PrintCommandTable(rw *helpfmt.RenderWriter, entries []commandEntry) {
	printCommandTable(rw, io.Discard, entries)
}

// RenderHelpCommand and RenderAIHelp run the whole local route: the handler's
// answer, then the renderer both commands register, as `ze help` does.
func RenderHelpCommand(w io.Writer, args []string) int {
	return renderAnswer(w, "help command", helpCommandAnswer(args))
}

func RenderAIHelp(w io.Writer, args []string) int {
	return renderAnswer(w, "help ai", aiHelpAnswer(args))
}

func renderAnswer(w io.Writer, path string, answer command.LocalAnswer) int {
	if answer.Code != 0 {
		return answer.Code
	}
	return renderTo(w, io.Discard, path, answer.Payload)
}
