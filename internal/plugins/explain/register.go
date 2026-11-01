// Design: docs/features/ai-first.md — show diagnostic command registration

package explain

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	// The command answers DATA, so its answer reaches the pipe layer; the
	// drawing gives the text route the explanation as it was always printed.
	command.MustRegisterTextRenderer(keyDiagnostic, drawText)
	command.MustRegisterLocalData("show diagnostic", Run, registry.Meta{
		ShortHelp: "Explain one diagnostic code Ze printed.",
		Description: "The answer gives the meaning of the code, its likely cause and the recommended " +
			"fix. Pass the code you read in a log line or an error message.",
		Mode: "offline",
	}, render)
}
