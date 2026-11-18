// Design: docs/architecture/api/commands.md -- supplied completion and private codec inputs.
// Related: cmd_completion.go, cmd_secret.go -- the shared native producers.

package cli

import (
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	suppliedConfiguration = "configuration"
	suppliedInput         = "input"
)

// completionCandidate is the native candidate's command document projection.
// Empty text fields are valid; no lifecycle or validation invariant is held.
type completionCandidate struct {
	Text        string `json:"text"`
	Type        string `json:"type"`
	ShortHelp   string `json:"short-help"`
	Description string `json:"description"`
}

func completionCandidates(candidates []cli.Completion) []completionCandidate {
	rows := make([]completionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		rows = append(rows, completionCandidate{Text: candidate.Text, Type: candidate.Type,
			ShortHelp: candidate.ShortHelp, Description: candidate.Description})
	}
	return rows
}

func suppliedCompletion(args command.ValidatedArgs, data []byte, name string) command.LocalAnswer {
	const path = "show config supplied completion"
	configuration, input, err := suppliedPairFields(data, suppliedConfiguration, suppliedInput)
	if err != nil {
		return suppliedRefusal(path, "content must contain configuration and input strings", err)
	}
	completer, tree, err := completerForBytes(configuration, name)
	if err != nil {
		return command.LocalRefusal(exitError, command.LocalErrorLine(err))
	}
	context, _ := args.Keyed("context")
	if args.Flag("ghost") {
		return command.LocalAnswer{Payload: command.Blob(completer.GhostText(tree, string(input), splitContextPath(context)) + "\n")}
	}
	return command.LocalAnswer{Payload: struct {
		Completions []completionCandidate `json:"completions"`
	}{Completions: completionCandidates(completer.Complete(tree, string(input), splitContextPath(context)))}}
}

// suppliedSecret adapts bounded caller bytes to the native codec. The empty
// value reaches the same native refusal as stdin. No host operand is read.
func suppliedSecret(action, path string) command.LocalDataHandler {
	return func(args command.ValidatedArgs) command.LocalAnswer {
		if err := args.Context().Err(); err != nil {
			return suppliedRefusal(path, "request canceled", err)
		}
		if len(args.PositionalTokens()) != 0 {
			return suppliedRefusal(path, "unexpected arguments", nil)
		}
		encoded, found := args.Keyed("content")
		if !found {
			return suppliedRefusal(path, "content is missing", nil)
		}
		data, err := command.DecodeContent(encoded)
		if err != nil {
			return suppliedRefusal(path, "content could not be decoded", err)
		}
		value := string(data)
		if args.Flag("trim-line-ending") {
			value = trimSecretLineEnding(value)
		}
		var answer string
		var refusal *opererr.Error
		answer, refusal = secretActions[action](value)
		if refusal != nil {
			return command.LocalRefusal(exitInvalid, command.LocalErrorLine(refusal))
		}
		if err := args.Context().Err(); err != nil {
			return suppliedRefusal(path, "request canceled", err)
		}
		return command.LocalAnswer{Payload: command.Blob(answer + "\n")}
	}
}
