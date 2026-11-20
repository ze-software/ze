// Design: docs/architecture/api/commands.md -- preserve typed parameters until metadata is available.
package api

import (
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// prepareCommand reads one command generation for both construction and the
// read-only authorization decision. It builds the text before either the
// authorizer or executor sees it; transports never guess selector placement.
func (e *APIEngine) prepareCommand(input string, params map[string]string) (string, bool, error) {
	var all []CommandMeta
	if e.commands != nil {
		var err error
		all, err = e.commands()
		if err != nil {
			return "", false, err
		}
	}
	words, pipeAt, err := command.ScanExecution(input)
	if err != nil {
		return "", false, err
	}
	meta, err := invocationCommandMeta(words, all)
	if err != nil {
		return "", false, err
	}
	built, err := buildInvocation(input, params, words, pipeAt, meta)
	if err != nil {
		return "", false, err
	}
	if meta == nil {
		return built, false, nil
	}
	return built, meta.ReadOnly, nil
}

// buildInvocation normalizes structured flags using the matched declaration.
// Exact paths take typed inline placement. An existing raw invocation is
// retained byte-for-byte and receives only a keyword suffix: it is never
// rebuilt, and supplied selector values are never inserted a second time.
//
// The parameters are written into the command part, before the first pipe
// outside quotes (pipeAt, from command.ScanExecution), so they reach the
// handler as arguments and the chain stays the chain the caller typed.
// WriteInvocation quotes a value holding a pipe or a quote, so no parameter
// opens an operation.
func buildInvocation(input string, params map[string]string, words []string, pipeAt int, meta *CommandMeta) (string, error) {
	if len(params) == 0 {
		return input, nil
	}
	if err := ValidateParams(params); err != nil {
		return "", err
	}
	var defs []command.InvocationArg
	if meta != nil {
		defs = make([]command.InvocationArg, len(meta.Params))
		for i, p := range meta.Params {
			defs[i] = command.InvocationArg{Name: p.Name, Anchor: p.Anchor, Flag: p.Flag}
		}
	}
	values, err := command.NormalizeInvocationFlags(defs, params)
	if err != nil {
		name := input
		if meta != nil {
			name = meta.Name
		}
		return "", command.InvocationFlagError(name, err)
	}
	commandPart, chain := input, ""
	if pipeAt >= 0 {
		commandPart = strings.TrimRightFunc(input[:pipeAt], unicode.IsSpace)
		chain = input[pipeAt:]
	}
	var output textbuf.Buffer
	if meta != nil && strings.EqualFold(strings.Join(words, " "), meta.Name) {
		command.WriteInvocation(&output, strings.Fields(meta.Name), defs, values)
	} else {
		output.Str(commandPart)
		command.WriteInvocation(&output, nil, defs, values)
	}
	if chain != "" {
		output.Byte(' ').Str(chain)
	}
	return output.String(), nil
}

// invocationCommandMeta uses the dispatcher's own token matcher and original
// declarations. It does not derive a second grammar from metadata spelling.
// Longest matching path wins; equally long distinct matches are refused.
func invocationCommandMeta(words []string, all []CommandMeta) (*CommandMeta, error) {
	var selected *CommandMeta
	for i := range all {
		meta := &all[i]
		if selected != nil && len(meta.Name) < len(selected.Name) {
			continue
		}
		if _, _, matched := command.MatchInvocationPath(words, meta.Name, meta.ArgDefs); !matched {
			continue
		}
		if selected != nil && len(meta.Name) == len(selected.Name) && !strings.EqualFold(meta.Name, selected.Name) {
			var problem, fix textbuf.Buffer
			problem.Str("Cannot choose a command for ").Quoted(strings.Join(words, " ")).
				Str(": both ").Quoted(selected.Name).Str(" and ").Quoted(meta.Name).Str(" match")
			fix.Str("ze help command ")
			command.WriteInvocationValue(&fix, meta.Name)
			return nil, opererr.New(problem.String(),
				"No command ran. Supply the complete unambiguous path and arguments shown by command help.", fix.String())
		}
		selected = meta
	}
	return selected, nil
}
