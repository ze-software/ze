// Design: docs/architecture/api/commands.md -- supplied standalone configuration tools.
// Related: cmd_migrate.go -- the shared form converter.
// Related: cmd_fix.go -- the shared diagnostic plan.
// Related: cmd_show.go -- masked source-tree inspection.

package cli

import (
	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func dataMigrate(args command.ValidatedArgs) command.LocalAnswer {
	return fileToolAnswer(args, "show config migrate", suppliedMigrate)
}

func dataFixPlan(args command.ValidatedArgs) command.LocalAnswer {
	return fileToolAnswer(args, "show config fix-plan", suppliedFixPlan)
}

func dataSource(args command.ValidatedArgs) command.LocalAnswer {
	return fileToolAnswer(args, "show config source", fileSource)
}

// fileToolAnswer confines a daemon-host filename before reading it, then
// reaches the same producer as the caller-supplied byte form.
func fileToolAnswer(args command.ValidatedArgs, path string, produce suppliedProducer) command.LocalAnswer {
	file, found := args.Keyed("file")
	if !found {
		positional := args.PositionalTokens()
		if len(positional) != 1 {
			return suppliedRefusal(path, "one file is required", nil)
		}
		file = positional[0]
	}
	readPath, err := command.CallerPath(args.Route(), path, args.Tokens(), file)
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}
	data, err := cliio.ReadFile(readPath)
	if err != nil {
		return suppliedRefusal(path, "configuration file could not be read", err)
	}
	return suppliedAnswer(path, produce, args, data, file)
}

func suppliedMigrate(args command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	form := formSet
	if selected, found := args.Keyed(formKeyword); found {
		form = selected
	}
	converted, err := configMigrateBytes(data, name, form)
	if err != nil {
		return suppliedRefusal("show config supplied migrate", "configuration could not be converted", err)
	}
	var diag textbuf.Buffer
	writePasswordWeaknesses(&diag, converted.weaknesses)
	for _, warning := range converted.warnings {
		diag.Str("warning: ").Str(warning).Byte('\n')
	}
	return command.LocalAnswer{Payload: command.Blob(converted.output), Diagnostic: diag.String()}
}

func suppliedFixPlan(_ command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	return command.LocalAnswer{Payload: fixPlanBytes(data, name)}
}

func suppliedSource(args command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	return sourceBytesAnswer(args, data, name, true)
}

func fileSource(args command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	return sourceBytesAnswer(args, data, name, args.Route().CallerLocal())
}

func sourceBytesAnswer(args command.ValidatedArgs, data []byte, name string, rawSourceAllowed bool) command.LocalAnswer {
	var path []string
	if selected, found := args.Keyed("path"); found {
		words, err := editor.ParseCommandWords(selected)
		if err != nil {
			return suppliedRefusal("show config supplied source", "source path could not be read", err)
		}
		path = words.Tokens()
	}
	ed, err := editor.NewEditorFromContent(data, name)
	if err != nil {
		return suppliedRefusal("show config supplied source", "configuration could not be inspected", err)
	}
	answer := sourceEditorAnswer(ed, name, path)
	if !rawSourceAllowed {
		if ed.DisplayTreeAtPath(nil) == nil {
			answer = suppliedRefusal("show config source", "the source does not parse, so its secret leaves cannot be masked; inspect the file with the local ze config show command", nil)
		}
	}
	if err := ed.Close(); err != nil {
		return suppliedRefusal("show config supplied source", "inspection could not be closed", err)
	}
	return answer
}
