// Design: docs/architecture/api/commands.md — CLI pipe operators
// Detail: pipe_table.go — table rendering (applyTable)
// Related: main.go — interactive CLI (executeCommand uses parsePipe)
//
// pipe.go implements VyOS-style pipe operators for the interactive CLI.
// Users can append | match <pattern>, | count, | no-more, | json [compact|pretty],
// | table to any command. Pipes are client-side filters applied to the command output.
package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// pipeKind identifies the type of pipe operator.
type pipeKind int

const (
	pipeMatch   pipeKind = iota // | match <pattern> — grep lines
	pipeCount                   // | count — count lines
	pipeNoMore                  // | no-more — disable paging (currently no-op)
	pipeJSON                    // | json [pretty|compact] — format as JSON
	pipeTable                   // | table — nushell-style table rendering
	pipeYAML                    // | yaml — YAML-formatted output
	pipeUnknown                 // unrecognized operator
)

const (
	jsonPretty  = "pretty"
	jsonCompact = "compact"
)

// pipeOp represents a single pipe operator with its argument.
type pipeOp struct {
	kind pipeKind
	arg  string
}

// parsePipe splits user input into the command and a chain of pipe operators.
// Input "peer list | match established | count" returns ("peer list", [{match,"established"}, {count,""}]).
func parsePipe(input string) (command string, ops []pipeOp) {
	parts := strings.Split(input, "|")
	command = strings.TrimSpace(parts[0])

	for _, part := range parts[1:] {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "match":
			arg := ""
			if len(fields) > 1 {
				arg = strings.Join(fields[1:], " ")
			}
			ops = append(ops, pipeOp{kind: pipeMatch, arg: arg})

		case "count":
			ops = append(ops, pipeOp{kind: pipeCount})

		case "no-more":
			ops = append(ops, pipeOp{kind: pipeNoMore})

		case "table":
			ops = append(ops, pipeOp{kind: pipeTable})

		case "yaml":
			ops = append(ops, pipeOp{kind: pipeYAML})

		case "json":
			arg := jsonPretty
			if len(fields) > 1 && fields[1] == jsonCompact {
				arg = jsonCompact
			}
			ops = append(ops, pipeOp{kind: pipeJSON, arg: arg})

		default:
			ops = append(ops, pipeOp{kind: pipeUnknown, arg: strings.Join(fields, " ")})
		}
	}

	return command, ops
}

// foldServerPipeline rewrites command and ops for commands that support server-side pipelines.
// For "rib show" commands, pipe segments containing server pipeline keywords are folded
// back into the command string. Only client-side ops (no-more, table) remain as ops.
// Example: "rib show received | path 65001 | count" → command="rib show received path 65001 count", ops=nil.
func foldServerPipeline(command string, ops []pipeOp) (string, []pipeOp) {
	trimmed := strings.TrimSpace(command)
	lower := strings.ToLower(trimmed)

	// Only fold for rib show commands (not "rib status", "rib best", etc.)
	if !strings.HasPrefix(lower, "rib show") {
		return command, ops
	}

	var serverArgs []string
	var clientOps []pipeOp

	for _, op := range ops {
		switch op.kind {
		case pipeNoMore, pipeTable, pipeYAML:
			// Client-side only
			clientOps = append(clientOps, op)
		case pipeMatch:
			// "match" is a server pipeline keyword for rib show
			if op.arg != "" {
				serverArgs = append(serverArgs, "match", op.arg)
			} else {
				serverArgs = append(serverArgs, "match")
			}
		case pipeCount:
			serverArgs = append(serverArgs, "count")
		case pipeJSON:
			serverArgs = append(serverArgs, "json")
		case pipeUnknown:
			// Pipeline keywords like "path", "cidr", "community", "family"
			// are parsed as pipeUnknown by parsePipe. Fold them back.
			serverArgs = append(serverArgs, op.arg)
		}
	}

	if len(serverArgs) > 0 {
		command = trimmed + " " + strings.Join(serverArgs, " ")
	}

	return command, clientOps
}

// applyPipes runs the output through each pipe operator in order.
// Returns the filtered output and an error message (empty on success).
func applyPipes(output string, ops []pipeOp) (string, string) {
	result := output
	for _, op := range ops {
		switch op.kind {
		case pipeMatch:
			if op.arg == "" {
				return "", "match requires a pattern"
			}
			result = applyMatch(result, op.arg)
		case pipeCount:
			result = applyCount(result)
		case pipeNoMore:
			// No-op: paging not yet implemented
		case pipeJSON:
			result = applyJSON(result, op.arg)
		case pipeTable:
			result = applyTable(result)
		case pipeYAML:
			result = applyYAML(result)
		case pipeUnknown:
			return "", fmt.Sprintf("unknown pipe operator: %s", op.arg)
		}
	}
	return result, ""
}

// hasFormatOp returns true if the pipe chain contains an explicit format or terminal operator.
// Count is terminal: it suppresses the default table format so it can work on raw JSON.
func hasFormatOp(ops []pipeOp) bool {
	for _, op := range ops {
		if op.kind == pipeJSON || op.kind == pipeTable || op.kind == pipeYAML || op.kind == pipeCount {
			return true
		}
	}
	return false
}

// applyMatch filters lines containing pattern (case-insensitive).
func applyMatch(input, pattern string) string {
	lower := strings.ToLower(pattern)
	var b strings.Builder
	for line := range strings.SplitSeq(input, "\n") {
		if strings.Contains(strings.ToLower(line), lower) {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// applyCount counts items. If input is JSON, counts array elements or map keys
// (unwrapping single-key wrappers). Otherwise counts non-empty lines.
func applyCount(input string) string {
	if input == "" {
		return "0\n"
	}
	trimmed := strings.TrimSpace(input)
	var data any
	if err := json.Unmarshal([]byte(trimmed), &data); err == nil {
		return strconv.Itoa(countItems(data)) + "\n"
	}
	// Fallback: count non-empty lines.
	n := 0
	for line := range strings.SplitSeq(input, "\n") {
		if line != "" {
			n++
		}
	}
	return strconv.Itoa(n) + "\n"
}

// countItems counts the number of items in a JSON value.
func countItems(v any) int {
	switch val := v.(type) {
	case []any:
		return len(val)
	case map[string]any:
		// Single-key wrapper: unwrap and count the inner value.
		if len(val) == 1 {
			for _, inner := range val {
				return countItems(inner)
			}
		}
		return len(val)
	default:
		return 1
	}
}

// applyJSON reformats JSON output. "pretty" indents, "compact" produces one line.
// Non-JSON input passes through unchanged.
func applyJSON(input, mode string) string {
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		return input
	}

	switch mode {
	case jsonCompact:
		out, err := json.Marshal(data)
		if err != nil {
			return input
		}
		return string(out)
	default: // "pretty"
		out, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return input
		}
		return string(out)
	}
}

// applyYAML reformats JSON output as valid YAML.
// Non-JSON input passes through unchanged.
func applyYAML(input string) string {
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		return input
	}
	return renderYAML(data)
}

// ProcessPipes splits user input into a command and a formatting function.
// The returned function applies pipe operators (table, json, yaml, match, count)
// to raw JSON output. If no pipes are present, the formatter returns raw JSON unchanged.
func ProcessPipes(input string) (command string, format func(string) string) {
	command, ops := parsePipe(input)
	command, ops = foldServerPipeline(command, ops)

	if len(ops) == 0 {
		return command, func(s string) string { return s }
	}

	return command, func(rawJSON string) string {
		result, errMsg := applyPipes(rawJSON, ops)
		if errMsg != "" {
			return "pipe error: " + errMsg
		}
		return result
	}
}
