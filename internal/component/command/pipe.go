// Design: docs/architecture/api/commands.md — CLI pipe operators
// Detail: pipe_table.go — table rendering (ApplyTable)
// Related: format.go — YAML and number formatting
//
// pipe.go implements VyOS-style pipe operators for command output.
// Users can append | match <pattern>, | count, | no-more, | json [compact|pretty],
// | table, | yaml to any command. Pipes are client-side filters applied to command output.
package command

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
	pipeCount                   // | count — count items (returns JSON {"count": N})
	pipeNoMore                  // | no-more — disable paging (currently no-op)
	pipeJSON                    // | json [pretty|compact] — format as JSON
	pipeTable                   // | table — nushell-style table rendering with box-drawing
	pipeText                    // | text — space-aligned columns without box-drawing
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

// knownPipeOps maps operator names to their pipeKind.
var knownPipeOps = map[string]pipeKind{
	"match":   pipeMatch,
	"count":   pipeCount,
	"no-more": pipeNoMore,
	"table":   pipeTable,
	"text":    pipeText,
	"yaml":    pipeYAML,
	"json":    pipeJSON,
}

// ParsePipe splits user input into the command and a chain of pipe operators.
// Input "peer list | match established | count" returns ("peer list", [{match,"established"}, {count,""}]).
func ParsePipe(input string) (command string, ops []pipeOp) {
	parts := strings.Split(input, "|")
	command = strings.TrimSpace(parts[0])

	for _, part := range parts[1:] {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 {
			continue
		}

		kind, known := knownPipeOps[fields[0]]
		if !known {
			// Unknown operator — preserved for error reporting in ApplyPipes.
			ops = append(ops, pipeOp{kind: pipeUnknown, arg: strings.Join(fields, " ")})
			continue
		}

		op := pipeOp{kind: kind}
		switch kind { //nolint:exhaustive // only some operators take arguments
		case pipeMatch:
			if len(fields) > 1 {
				op.arg = strings.Join(fields[1:], " ")
			}
		case pipeJSON:
			op.arg = jsonPretty
			if len(fields) > 1 && fields[1] == jsonCompact {
				op.arg = jsonCompact
			}
		}
		ops = append(ops, op)
	}

	return command, ops
}

// FoldServerPipeline rewrites command and ops for commands that support server-side pipelines.
// For "bgp rib routes" commands, pipe segments containing server pipeline keywords are folded
// back into the command string. Only client-side ops (no-more, table) remain as ops.
// Example: "bgp rib routes received | path 65001 | count" → command="bgp rib routes received path 65001 count", ops=nil.
func FoldServerPipeline(command string, ops []pipeOp) (string, []pipeOp) {
	trimmed := strings.TrimSpace(command)
	lower := strings.ToLower(trimmed)

	// Only fold for rib routes and rib show best commands (server-side pipeline).
	if !strings.HasPrefix(lower, "bgp rib routes") && !strings.HasPrefix(lower, "bgp rib show best") {
		return command, ops
	}

	var serverArgs []string
	var clientOps []pipeOp

	for _, op := range ops {
		switch op.kind { //nolint:exhaustive // only classify server vs client ops
		case pipeNoMore, pipeTable, pipeText, pipeYAML:
			// Client-side only
			clientOps = append(clientOps, op)
		case pipeMatch:
			// "match" is a server pipeline keyword for rib routes
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
			// are parsed as pipeUnknown by ParsePipe. Fold them back.
			serverArgs = append(serverArgs, op.arg)
		}
	}

	if len(serverArgs) > 0 {
		command = trimmed + " " + strings.Join(serverArgs, " ")
	}

	return command, clientOps
}

// ApplyPipes runs the output through each pipe operator in order.
// Returns the filtered output and an error message (empty on success).
// Rejects multiple format operators (json, table, text, yaml).
func ApplyPipes(output string, ops []pipeOp) (string, string) {
	formatCount := 0
	for _, op := range ops {
		if op.kind == pipeJSON || op.kind == pipeTable || op.kind == pipeText || op.kind == pipeYAML {
			formatCount++
		}
	}
	if formatCount > 1 {
		return "", "multiple format operators (use only one of: json, table, text, yaml)"
	}

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
			result = ApplyJSON(result, op.arg)
		case pipeTable:
			result = ApplyTable(result)
		case pipeText:
			result = ApplyText(result)
		case pipeYAML:
			result = applyYAML(result)
		case pipeUnknown:
			return "", fmt.Sprintf("unknown pipe operator: %s", op.arg)
		}
	}
	return result, ""
}

// HasFormatOp returns true if the pipe chain contains an explicit display format.
// Count is a data transform (not a format) — it produces JSON for downstream formatting.
func HasFormatOp(ops []pipeOp) bool {
	for _, op := range ops {
		if op.kind == pipeJSON || op.kind == pipeTable || op.kind == pipeText || op.kind == pipeYAML {
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

// applyCount counts items and returns JSON {"count": N}.
// If input is JSON, counts array elements or map keys
// (unwrapping single-key wrappers). Otherwise counts non-empty lines.
func applyCount(input string) string {
	if input == "" {
		return "{\"count\":0}\n"
	}
	trimmed := strings.TrimSpace(input)
	var data any
	if err := json.Unmarshal([]byte(trimmed), &data); err == nil {
		return "{\"count\":" + strconv.Itoa(countItems(data)) + "}\n"
	}
	// Fallback: count non-empty lines.
	n := 0
	for line := range strings.SplitSeq(input, "\n") {
		if line != "" {
			n++
		}
	}
	return "{\"count\":" + strconv.Itoa(n) + "}\n"
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
	}
	return 1
}

// ApplyJSON reformats JSON output. "pretty" indents, "compact" produces one line.
// Non-JSON input passes through unchanged.
func ApplyJSON(input, mode string) string {
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		return input
	}

	if mode == jsonCompact {
		out, err := json.Marshal(data)
		if err != nil {
			return input
		}
		return string(out)
	}

	// Pretty-print (default).
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return input
	}
	return string(out)
}

// applyYAML reformats JSON output as valid YAML.
// Non-JSON input passes through unchanged.
func applyYAML(input string) string {
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		return input
	}
	return RenderYAML(data)
}

// ProcessPipes splits user input into a command and a formatting function.
// The returned function applies pipe operators (table, json, yaml, match, count)
// to raw JSON output. If no pipes are present, the formatter returns raw JSON unchanged.
func ProcessPipes(input string) (command string, format func(string) string) {
	command, ops := ParsePipe(input)
	command, ops = FoldServerPipeline(command, ops)

	if len(ops) == 0 {
		return command, func(s string) string { return s }
	}

	return command, func(rawJSON string) string {
		result, errMsg := ApplyPipes(rawJSON, ops)
		if errMsg != "" {
			return "pipe error: " + errMsg
		}
		return result
	}
}

// ProcessPipesDefaultTable is like ProcessPipes but defaults to table format
// when no explicit format pipe (json, table, yaml, count) is specified.
func ProcessPipesDefaultTable(input string) (command string, format func(string) string) {
	command, ops := ParsePipe(input)
	command, ops = FoldServerPipeline(command, ops)

	if !HasFormatOp(ops) {
		ops = append(ops, pipeOp{kind: pipeTable})
	}

	return command, func(rawJSON string) string {
		result, errMsg := ApplyPipes(rawJSON, ops)
		if errMsg != "" {
			return "pipe error: " + errMsg
		}
		return result
	}
}

// ProcessPipesDefaultFunc is like ProcessPipes but applies defaultFn as the
// formatter when no explicit format pipe (json, table, yaml, text) is specified.
// This allows callers to provide a domain-specific formatter (e.g., compact
// one-liner for streaming monitors) while still respecting explicit pipes.
func ProcessPipesDefaultFunc(input string, defaultFn func(string) string) (command string, format func(string) string) {
	command, ops := ParsePipe(input)
	command, ops = FoldServerPipeline(command, ops)

	if !HasFormatOp(ops) {
		if len(ops) == 0 {
			return command, defaultFn
		}
		// Non-format ops (match, count) still apply before the default formatter.
		return command, func(rawJSON string) string {
			result, errMsg := ApplyPipes(rawJSON, ops)
			if errMsg != "" {
				return "pipe error: " + errMsg
			}
			return defaultFn(result)
		}
	}

	return command, func(rawJSON string) string {
		result, errMsg := ApplyPipes(rawJSON, ops)
		if errMsg != "" {
			return "pipe error: " + errMsg
		}
		return result
	}
}
