// Design: docs/architecture/api/commands.md -- supplied standalone configuration tools.
// Related: cmd_migrate.go -- the shared form converter.
// Related: cmd_fix.go -- the shared diagnostic plan.
// Related: cmd_show.go -- masked source-tree inspection.

package cli

import (
	"errors"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	pathShowSource     = "show config source"
	pathSuppliedSource = "show config supplied source"
)

func dataMigrate(args command.ValidatedArgs) command.LocalAnswer {
	return fileToolAnswer(args, "show config migrate", suppliedMigrate)
}

func dataFixPlan(args command.ValidatedArgs) command.LocalAnswer {
	return fileToolAnswer(args, "show config fix-plan", suppliedFixPlan)
}

func dataSource(args command.ValidatedArgs) command.LocalAnswer {
	file, found := args.Keyed("file")
	if !found {
		positional := args.PositionalTokens()
		if len(positional) != 1 {
			return suppliedRefusal(pathShowSource, "one file is required", nil)
		}
		file = positional[0]
	}
	readPath, err := command.CallerPath(args.Route(), pathShowSource, args.Tokens(), file)
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}
	data, err := cliio.ReadFile(readPath)
	if err != nil {
		return suppliedRefusal(pathShowSource, "configuration file could not be read", err)
	}
	produce := func(a command.ValidatedArgs, content []byte, label string) command.LocalAnswer {
		return sourceBytesAnswerWithPath(a, content, label, readPath, a.Route().CallerLocal())
	}
	return suppliedAnswer(pathShowSource, produce, args, data, file)
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

func sourceBytesAnswer(args command.ValidatedArgs, data []byte, name string, rawSourceAllowed bool) command.LocalAnswer {
	return sourceBytesAnswerWithPath(args, data, name, name, rawSourceAllowed)
}

func sourceBytesAnswerWithPath(args command.ValidatedArgs, data []byte, name, configFile string, rawSourceAllowed bool) command.LocalAnswer {
	var path []string
	if selected, found := args.Keyed("path"); found {
		words, err := editor.ParseCommandWords(selected)
		if err != nil {
			return suppliedRefusal(pathSuppliedSource, "source path could not be read", err)
		}
		path = words.Tokens()
	}
	ed, err := editor.NewEditorFromContent(data, name)
	if err != nil {
		return suppliedRefusal(pathSuppliedSource, "configuration could not be inspected", err)
	}
	var answer command.LocalAnswer
	if hasPipeOperation(args.PipeOperations(), pipeOpImplied) {
		answer = sourceImpliedEditorAnswer(ed, name, configFile, path)
	} else {
		answer = sourceEditorAnswer(ed, name, path)
		if !rawSourceAllowed {
			if ed.DisplayTreeAtPath(nil) == nil {
				answer = suppliedRefusal(pathShowSource, "the source does not parse, so its secret leaves cannot be masked; inspect the file with the local ze config show command", nil)
			}
		}
	}
	if err := ed.Close(); err != nil {
		return suppliedRefusal(pathSuppliedSource, "inspection could not be closed", err)
	}
	return answer
}

// sourceImpliedEditorAnswer builds the `{config, implied}` envelope for
// `show config source <file> | implied`. Offline external endpoint declarations
// are queried in declare mode; any query refusal fails closed with zero payload.
func sourceImpliedEditorAnswer(ed *editor.Editor, name, configFile string, path []string) command.LocalAnswer {
	whole := ed.DisplayTreeAtPath(nil)
	if whole == nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(opererr.New(
			name+": the configuration does not parse",
			"Implied configuration cannot be computed until the source parses cleanly.",
			"ze config validate "+name)))
	}
	targetTree := whole
	if len(path) > 0 {
		subtree := ed.DisplayTreeAtPath(path)
		if subtree == nil {
			var b textbuf.Buffer
			return command.LocalRefusal(exitError, command.LocalErrorLine(opererr.New(
				b.Str("path not found: ").Join(path, " ").String(),
				"No subtree at that path exists in the supplied configuration.",
				"ze show config source help")))
		}
		targetTree = subtree
	}
	report, err := config.BuildImpliedReport(whole, config.ImpliedOptions{
		Offline:    true,
		ConfigFile: configFile,
	})
	if err != nil {
		fix := "ze show config source help"
		if op, ok := errors.AsType[*opererr.Error](err); ok && op.Fix() != "" {
			fix = op.Fix()
		}
		return command.LocalRefusal(exitError, command.LocalErrorLine(opererr.Wrap(
			err,
			"cannot display implied configuration",
			"offline implied evaluation failed",
			fix)))
	}
	return command.LocalAnswer{
		Payload: config.ImpliedEnvelope{
			Config:  targetTree.ToMap(),
			Implied: report,
		},
	}
}
