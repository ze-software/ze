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

		case "json":
			arg := jsonPretty
			if len(fields) > 1 && fields[1] == jsonCompact {
				arg = jsonCompact
			}
			ops = append(ops, pipeOp{kind: pipeJSON, arg: arg})

		default:
			ops = append(ops, pipeOp{kind: pipeUnknown, arg: fields[0]})
		}
	}

	return command, ops
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
		case pipeUnknown:
			return "", fmt.Sprintf("unknown pipe operator: %s", op.arg)
		}
	}
	return result, ""
}

// hasFormatOp returns true if the pipe chain contains an explicit format operator (json or table).
func hasFormatOp(ops []pipeOp) bool {
	for _, op := range ops {
		if op.kind == pipeJSON || op.kind == pipeTable {
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

// applyCount returns the number of non-empty lines.
func applyCount(input string) string {
	if input == "" {
		return "0\n"
	}
	n := 0
	for line := range strings.SplitSeq(input, "\n") {
		if line != "" {
			n++
		}
	}
	return strconv.Itoa(n) + "\n"
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
