// Design: docs/architecture/web-interface.md -- shared CLI session preferences
package cli

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// SessionFormatResult is the session-local preference and feedback produced by
// the operational CLI format command. Format stays unchanged after a refusal.
type SessionFormatResult struct {
	Format string
	Output string
}

// IsSessionFormatCommand identifies the local operational command before a
// transport routes other set commands to the configuration editor.
func IsSessionFormatCommand(input string) bool {
	const prefix = "set cli format"
	return input == prefix || strings.HasPrefix(input, prefix+" ")
}

// ExecuteSessionFormat runs the same local command for SSH and browser sessions.
// The schema owns the accepted values and refusal; no process default is written.
func ExecuteSessionFormat(schema *config.Schema, input, current string) (SessionFormatResult, bool, error) {
	answer := SessionFormatResult{Format: current}
	if !IsSessionFormatCommand(input) {
		return answer, false, nil
	}
	rest := strings.TrimSpace(input[len("set cli format"):])
	var tb textbuf.Buffer
	if rest == "" {
		format := effectiveSessionFormat(current)
		if format == "" {
			format = "text"
		}
		answer.Output = tb.Str("cli format: ").Str(format).String()
		return answer, true, nil
	}
	leaf := sessionFormatLeaf(schema)
	if leaf == nil {
		answer.Output = "cli format unavailable: the schema declares no output formats"
		return answer, true, nil
	}
	if len(leaf.Enums) == 0 {
		answer.Output = "cli format unavailable: the schema declares no output formats"
		return answer, true, nil
	}
	if err := config.ValidateLeafValue(leaf, cliFormatCommandPath, rest); err != nil {
		return answer, true, err
	}
	answer.Format = rest
	answer.Output = tb.Str("cli format set to ").Str(rest).String()
	return answer, true, nil
}

// SessionFormatCompletions uses the SSH command completion producer and the
// session schema's format enumeration, including a schema with no formats.
func SessionFormatCompletions(schema *config.Schema, input string) []Completion {
	return appendCLIFormatCompletions(nil, input, sessionFormatNames(schema))
}

func sessionFormatNames(schema *config.Schema) []string {
	leaf := sessionFormatLeaf(schema)
	if leaf == nil {
		return nil
	}
	return slices.Sorted(slices.Values(leaf.Enums))
}

func sessionFormatLeaf(schema *config.Schema) *config.LeafNode {
	if schema == nil {
		return nil
	}
	node, err := schema.Lookup(cliFormatLeafPath)
	// refuse-by-default: guards ExecuteSessionFormat -- nil preserves the session format and sessionFormatNames offers no choices.
	if err != nil {
		return nil
	}
	leaf, _ := node.(*config.LeafNode)
	return leaf
}

func effectiveSessionFormat(override string) string {
	if override != "" {
		return override
	}
	return env.Get("ze.cli.format")
}
