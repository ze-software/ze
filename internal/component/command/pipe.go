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
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// pipeKind identifies the type of pipe operator.
type pipeKind int

const (
	pipeMatch   pipeKind = iota // | match <pattern> — grep lines
	pipeCount                   // | count — count items (returns JSON {"count": N})
	pipeNoMore                  // | no-more — disable paging (currently no-op)
	pipeJSON                    // | json [pretty|compact] — format as JSON array
	pipeNDJSON                  // | ndjson — one compact JSON object per line
	pipeTable                   // | table — nushell-style table rendering with box-drawing
	pipeText                    // | text — space-aligned columns without box-drawing
	pipeYAML                    // | yaml — YAML-formatted output
	pipeRaw                     // | raw — the dispatcher's JSON, byte for byte
	pipeResolve                 // | resolve — add reverse DNS names for IP address values
	pipeOrigin                  // | origin — add ASN and network name for IP address values
	pipeLog                     // | log — append each update instead of replacing
	pipeFirst                   // | first N — take first N items
	pipeLast                    // | last N — take last N items
	pipeUnknown                 // unrecognized operator
	pipeInvalid                 // validation error produced while folding command filters
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
	"raw":     pipeRaw,
	"json":    pipeJSON,
	"resolve": pipeResolve,
	"origin":  pipeOrigin,
	"ndjson":  pipeNDJSON,
	"log":     pipeLog,
	"first":   pipeFirst,
	"last":    pipeLast,
}

// ParsePipe splits user input into the command and a chain of pipe operators.
// Input "show bgp peer list | match established | count" returns ("show bgp peer list", [{match,"established"}, {count,""}]).
func ParsePipe(input string) (command string, ops []pipeOp) {
	command, rest, hasPipe := strings.Cut(input, "|")
	command = strings.TrimSpace(command)
	if !hasPipe {
		return command, nil
	}

	for part := range strings.SplitSeq(rest, "|") {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 {
			continue
		}

		kind, known := knownPipeOps[fields[0]]
		if !known {
			// Unknown operator — preserved for error reporting in ApplyPipes.
			ops = append(ops, pipeOp{kind: pipeUnknown, arg: textbuf.Join(fields, " ")})
			continue
		}

		op := pipeOp{kind: kind}
		switch kind { //nolint:exhaustive // only some operators take arguments
		case pipeMatch:
			if len(fields) > 1 {
				op.arg = textbuf.Join(fields[1:], " ")
			}
		case pipeJSON:
			op.arg = jsonPretty
			if len(fields) > 1 && fields[1] == jsonCompact {
				op.arg = jsonCompact
			}
		case pipeFirst, pipeLast:
			if len(fields) > 1 {
				op.arg = fields[1]
			}
		}
		ops = append(ops, op)
	}

	return command, ops
}

// foldFilters rewrites command-owned pipe filters into command arguments.
// Generic display and transform pipes stay client-side.
// Returns pipe metadata recording all data-shaping modifiers (both folded
// and remaining). Display-only pipes are excluded from metadata.
func foldFilters(command string, ops []pipeOp) (string, []pipeOp, map[string]any) {
	meta := collectPipeMeta(ops)

	trimmed := strings.TrimSpace(command)

	set, ok := lookupPipeFilters(trimmed)
	if !ok || len(set.filters) == 0 {
		return command, ops, meta
	}

	var leadingArgs []string
	var serverArgs []string
	var clientOps []pipeOp

	for _, op := range ops {
		switch op.kind { //nolint:exhaustive // only classify server vs client ops
		case pipeNoMore, pipeJSON, pipeNDJSON, pipeTable, pipeText, pipeYAML, pipeRaw, pipeResolve, pipeOrigin, pipeLog:
			clientOps = append(clientOps, op)
		case pipeMatch:
			if filter, ok := set.byName["match"]; ok {
				serverArgs = appendFilter(serverArgs, filter, op.arg)
			} else {
				clientOps = append(clientOps, op)
			}
		case pipeCount:
			if filter, ok := set.byName["count"]; ok {
				serverArgs = appendFilter(serverArgs, filter, "")
			} else {
				clientOps = append(clientOps, op)
			}
		case pipeFirst:
			if filter, ok := set.byName["first"]; ok {
				serverArgs = appendFilter(serverArgs, filter, op.arg)
			} else {
				clientOps = append(clientOps, op)
			}
		case pipeLast:
			if filter, ok := set.byName["last"]; ok {
				serverArgs = appendFilter(serverArgs, filter, op.arg)
			} else {
				clientOps = append(clientOps, op)
			}
		case pipeUnknown:
			filter, arg, known := lookupFilter(set, op.arg)
			if !known {
				clientOps = append(clientOps, pipeOp{kind: pipeInvalid, arg: unknownFilterError(trimmed, op.arg, set)})
				continue
			}
			if msg := validateFilter(filter, arg); msg != "" {
				clientOps = append(clientOps, pipeOp{kind: pipeInvalid, arg: msg})
				continue
			}
			if filter.Leading {
				leadingArgs = appendFilter(leadingArgs, filter, arg)
			} else {
				serverArgs = appendFilter(serverArgs, filter, arg)
			}
		}
	}

	allServerArgs := make([]string, 0, len(leadingArgs)+len(serverArgs))
	allServerArgs = append(allServerArgs, leadingArgs...)
	allServerArgs = append(allServerArgs, serverArgs...)
	if len(allServerArgs) > 0 {
		var tb textbuf.Buffer
		command = tb.Str(trimmed).Byte(' ').Join(allServerArgs, " ").String()
	}

	return command, clientOps, meta
}

// collectPipeMeta builds metadata from all data-shaping pipe ops.
// Display-only pipes (json, ndjson, table, text, yaml, resolve, origin, log, no-more)
// are excluded.
func collectPipeMeta(ops []pipeOp) map[string]any {
	var meta map[string]any
	for _, op := range ops {
		switch op.kind { //nolint:exhaustive // only data-shaping ops
		case pipeMatch:
			if meta == nil {
				meta = make(map[string]any)
			}
			meta["match"] = op.arg
		case pipeCount:
			if meta == nil {
				meta = make(map[string]any)
			}
			meta["count"] = true
		case pipeFirst:
			if meta == nil {
				meta = make(map[string]any)
			}
			if n, err := strconv.Atoi(op.arg); err == nil {
				meta["first"] = n
			}
		case pipeLast:
			if meta == nil {
				meta = make(map[string]any)
			}
			if n, err := strconv.Atoi(op.arg); err == nil {
				meta["last"] = n
			}
		case pipeUnknown:
			if meta == nil {
				meta = make(map[string]any)
			}
			fields := strings.Fields(op.arg)
			if len(fields) == 1 {
				meta[fields[0]] = true
			} else if len(fields) >= 2 {
				meta[fields[0]] = textbuf.Join(fields[1:], " ")
			}
		}
	}
	return meta
}

func validateFilter(filter PipeFilter, arg string) string {
	arg = strings.TrimSpace(arg)
	var tb textbuf.Buffer
	if filter.TakesArg {
		if arg == "" {
			return tb.Str("pipe filter ").Str(filter.Name).Str(" requires an argument").String()
		}
		return ""
	}
	if arg != "" {
		return tb.Str("pipe filter ").Str(filter.Name).Str(" does not accept an argument").String()
	}
	return ""
}

func unknownFilterError(command, raw string, set pipeFilterSet) string {
	name := raw
	if fields := strings.Fields(raw); len(fields) > 0 {
		name = fields[0]
	}
	var tb textbuf.Buffer
	valid := set.filterNames()
	if valid == "" {
		return tb.Str("unknown pipe filter for ").Str(command).Str(": ").Str(name).String()
	}
	return tb.Str("unknown pipe filter for ").Str(command).Str(": ").Str(name).Str(" (valid: ").Str(valid).Byte(')').String()
}

func lookupFilter(set pipeFilterSet, raw string) (PipeFilter, string, bool) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return PipeFilter{}, "", false
	}
	filter, ok := set.byName[strings.ToLower(fields[0])]
	if !ok {
		return PipeFilter{}, "", false
	}
	return filter, textbuf.Join(fields[1:], " "), true
}

func appendFilter(args []string, filter PipeFilter, value string) []string {
	args = append(args, filter.Name)
	if value != "" {
		args = append(args, strings.Fields(value)...)
	}
	return args
}

// ApplyPipes runs the output through each pipe operator in order.
// Returns the filtered output and an error message (empty on success).
// Rejects multiple format operators (json, table, text, yaml).
// If meta is non-nil, injects a "pipe" key into JSON output before formatting.
func ApplyPipes(output string, ops []pipeOp, meta map[string]any) (string, string) {
	formatCount := 0
	for _, op := range ops {
		if isFormatOp(op.kind) {
			formatCount++
		}
		if op.kind == pipeRaw {
			// raw answers a program, not a reader. Pipe metadata is display
			// information for a renderer, so injecting it would leave the
			// caller parsing a key the dispatcher never produced.
			meta = nil
		}
	}
	if formatCount > 1 {
		return "", multipleFormatsError
	}

	result := output
	metaInjected := false
	for _, op := range ops {
		if !metaInjected && isFormatOp(op.kind) {
			result = injectPipeMeta(result, meta)
			metaInjected = true
		}
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
		case pipeNDJSON:
			result = applyNDJSON(result)
		case pipeTable:
			result = ApplyTable(result)
		case pipeText:
			result = applyText(result)
		case pipeYAML:
			result = applyYAML(result)
		case pipeRaw:
			// Identity: the dispatcher's JSON is already the answer.
		case pipeResolve:
			result = applyResolve(result)
		case pipeOrigin:
			result = applyOrigin(result)
		case pipeFirst:
			result = applyFirst(result, op.arg)
		case pipeLast:
			result = applyLast(result, op.arg)
		case pipeLog:
			// Display-mode modifier, not a data transform. Handled by caller.
		case pipeUnknown:
			var tb textbuf.Buffer
			return "", tb.Str("unknown pipe operator: ").Str(op.arg).String()
		case pipeInvalid:
			return "", op.arg
		}
	}
	if !metaInjected {
		result = injectPipeMeta(result, meta)
	}
	return result, ""
}

// multipleFormatsError is refused by both the validator and the runner, so the
// two can never disagree about which operators are formats.
const multipleFormatsError = "multiple format operators (use only one of: json, ndjson, table, text, yaml, raw)"

// hasFormatOp returns true if the pipe chain contains an explicit display format.
// Count is a data transform (not a format) — it produces JSON for downstream formatting.
func hasFormatOp(ops []pipeOp) bool {
	for _, op := range ops {
		if isFormatOp(op.kind) {
			return true
		}
	}
	return false
}

// HasFormatPipe reports whether the input's pipe chain names an output format
// (json, ndjson, table, text, yaml).
//
// A caller that carries a default format of its own uses this to step aside:
// what the operator typed outranks a default. ProcessPipesDetectLog already
// applies that precedence internally, by appending the configured default only
// when the chain names no format. A caller that goes through
// ProcessPipesChecked has to apply it itself, and this is what it asks.
func HasFormatPipe(input string) bool {
	command, ops := ParsePipe(input)
	_, ops, _ = foldFilters(command, ops)
	return hasFormatOp(ops)
}

// ValidatePipes checks a pipe chain for errors without running it.
// Returns an error message if invalid, empty string if OK.
func ValidatePipes(ops []pipeOp) string {
	formatCount := 0
	for _, op := range ops {
		if op.kind == pipeInvalid {
			return op.arg
		}
		if op.kind == pipeUnknown {
			var tb textbuf.Buffer
			return tb.Str("unknown pipe operator: ").Str(op.arg).String()
		}
		if isFormatOp(op.kind) {
			formatCount++
		}
		if op.kind == pipeMatch && op.arg == "" {
			return "match requires a pattern"
		}
		if op.kind == pipeFirst || op.kind == pipeLast {
			name := "first"
			if op.kind == pipeLast {
				name = "last"
			}
			if op.arg == "" {
				var tb textbuf.Buffer
				return tb.Str(name).Str(" requires a numeric argument").String()
			}
			n, err := strconv.Atoi(op.arg)
			if err != nil || n <= 0 {
				var tb textbuf.Buffer
				return tb.Str(name).Str(" requires a positive number").String()
			}
		}
	}
	if formatCount > 1 {
		return multipleFormatsError
	}
	return ""
}

// hasLogOp returns true if the pipe chain contains | log.
func hasLogOp(ops []pipeOp) bool {
	for _, op := range ops {
		if op.kind == pipeLog {
			return true
		}
	}
	return false
}

// applyMatch filters lines containing pattern (case-insensitive).
func applyMatch(input, pattern string) string {
	lower := strings.ToLower(pattern)
	var b textbuf.Buffer
	for line := range strings.SplitSeq(input, "\n") {
		if strings.Contains(strings.ToLower(line), lower) {
			b.Str(line).Byte('\n')
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
		return textbuf.StrIntStr("{\"count\":", int64(countItems(data)), "}\n")
	}
	// Fallback: count non-empty lines.
	n := 0
	for line := range strings.SplitSeq(input, "\n") {
		if line != "" {
			n++
		}
	}
	return textbuf.StrIntStr("{\"count\":", int64(n), "}\n")
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

// isFormatOp reports whether the operator decides the shape of the answer.
// A chain names at most one of them, and naming one stops the configured
// default being appended. pipeRaw is one: it names the dispatcher's JSON as
// the shape, which is a choice like any other.
func isFormatOp(k pipeKind) bool {
	return k == pipeJSON || k == pipeNDJSON || k == pipeTable || k == pipeText || k == pipeYAML || k == pipeRaw
}

func injectPipeMeta(input string, meta map[string]any) string {
	if len(meta) == 0 {
		return input
	}
	trimmed := strings.TrimSpace(input)
	var data any
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return input
	}
	switch val := data.(type) {
	case map[string]any:
		val[pipeMetaKey] = meta
		out, err := json.Marshal(val)
		if err != nil {
			return input
		}
		return string(out)
	case []any:
		wrapped := map[string]any{"data": val, pipeMetaKey: meta}
		out, err := json.Marshal(wrapped)
		if err != nil {
			return input
		}
		return string(out)
	}
	return input
}

func applyFirst(input, arg string) string {
	n, err := strconv.Atoi(arg)
	if err != nil || n <= 0 {
		return input
	}
	trimmed := strings.TrimSpace(input)
	var data any
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return applyFirstLines(input, n)
	}
	data = truncateItems(data, n, false)
	out, err := json.Marshal(data)
	if err != nil {
		return input
	}
	return string(out)
}

func applyLast(input, arg string) string {
	n, err := strconv.Atoi(arg)
	if err != nil || n <= 0 {
		return input
	}
	trimmed := strings.TrimSpace(input)
	var data any
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return applyLastLines(input, n)
	}
	data = truncateItems(data, n, true)
	out, err := json.Marshal(data)
	if err != nil {
		return input
	}
	return string(out)
}

func truncateItems(v any, n int, fromEnd bool) any {
	switch val := v.(type) {
	case []any:
		return sliceN(val, n, fromEnd)
	case map[string]any:
		if len(val) == 1 {
			for k, inner := range val {
				if arr, ok := inner.([]any); ok {
					return map[string]any{k: sliceN(arr, n, fromEnd)}
				}
			}
		}
	}
	return v
}

func sliceN(arr []any, n int, fromEnd bool) []any {
	if n >= len(arr) {
		return arr
	}
	if fromEnd {
		return arr[len(arr)-n:]
	}
	return arr[:n]
}

func applyFirstLines(input string, n int) string {
	var b textbuf.Buffer
	i := 0
	for line := range strings.SplitSeq(input, "\n") {
		if i >= n {
			break
		}
		b.Str(line).Byte('\n')
		i++
	}
	return b.String()
}

func applyLastLines(input string, n int) string {
	lines := strings.Split(input, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if n >= len(lines) {
		return input
	}
	var b textbuf.Buffer
	for _, line := range lines[len(lines)-n:] {
		b.Str(line).Byte('\n')
	}
	return b.String()
}

// ApplyJSON reformats JSON output as valid JSON. Single-key wrapper maps
// containing arrays are unwrapped. "pretty" indents, "compact" produces one line.
func ApplyJSON(input, mode string) string {
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		return input
	}

	data = unwrapSingleKeyArray(data)

	if mode == jsonCompact {
		out, err := json.Marshal(data)
		if err != nil {
			return input
		}
		return string(out)
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return input
	}
	return string(out)
}

// applyNDJSON reformats JSON output as newline-delimited JSON (one compact
// object per line). Single-key wrapper maps containing arrays are unwrapped.
func applyNDJSON(input string) string {
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		return input
	}

	data = unwrapSingleKeyArray(data)

	arr, ok := data.([]any)
	if !ok {
		out, err := json.Marshal(data)
		if err != nil {
			return input
		}
		var tb textbuf.Buffer
		tb.Str(string(out)).Byte('\n')
		return tb.String()
	}
	return marshalNDJSON(arr, json.Marshal)
}

func unwrapSingleKeyArray(v any) any {
	m, ok := v.(map[string]any)
	if !ok || len(m) != 1 {
		return v
	}
	for _, inner := range m {
		if arr, isArr := inner.([]any); isArr {
			return arr
		}
	}
	return v
}

func marshalNDJSON(arr []any, marshal func(any) ([]byte, error)) string {
	var sb textbuf.Buffer
	for _, item := range arr {
		out, err := marshal(item)
		if err != nil {
			continue
		}
		sb.Write(out) //nolint:errcheck // textbuf.Write never fails
		sb.Byte('\n')
	}
	return sb.String()
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

func pipeError(msg string) string {
	var tb textbuf.Buffer
	return tb.Str("pipe error: ").Str(msg).String()
}

// ProcessPipesChecked splits user input into a command and a formatting function,
// validating the pipe chain upfront. The returned function applies pipe operators
// (table, json, yaml, match, count) to raw JSON output. If no pipes are present,
// the formatter returns raw JSON unchanged. Returns a non-empty errMsg (and a nil
// format) if the pipe chain is invalid.
func ProcessPipesChecked(input string) (command string, format func(string) string, errMsg string) {
	command, ops := ParsePipe(input)
	command, ops, meta := foldFilters(command, ops)
	if msg := ValidatePipes(ops); msg != "" {
		return command, nil, msg
	}

	if len(ops) == 0 {
		return command, func(s string) string { return injectPipeMeta(s, meta) }, ""
	}

	return command, func(rawJSON string) string {
		result, errMsg := ApplyPipes(rawJSON, ops, meta)
		if errMsg != "" {
			return pipeError(errMsg)
		}
		return result
	}, ""
}

// PipeFlags captures display-mode and data-transform flags from a pipe chain.
type PipeFlags struct {
	Log       bool
	Resolve   bool
	Origin    bool
	HasFormat bool
}

// ProcessPipesDetectLog is like ProcessPipesDefaultFormatChecked but also reports
// pipe flags (log, resolve, origin) and validates the pipe chain upfront.
// Returns a non-empty errMsg if the pipe chain is invalid.
//
// sessionFormat is the caller's per-session format override; pass "" for none.
// See configuredDefault.
func ProcessPipesDetectLog(input, sessionFormat string) (cmd string, format func(string) string, flags PipeFlags, errMsg string) {
	cmd, ops := ParsePipe(input)
	cmd, ops, meta := foldFilters(cmd, ops)

	if msg := ValidatePipes(ops); msg != "" {
		return cmd, nil, PipeFlags{}, msg
	}

	flags.Log = hasLogOp(ops)
	for _, op := range ops {
		switch op.kind { //nolint:exhaustive // only checking data-transform flags
		case pipeResolve:
			flags.Resolve = true
		case pipeOrigin:
			flags.Origin = true
		}
	}

	// Strip log ops from the pipeline (display-mode, not a data transform).
	filtered := ops[:0]
	for _, op := range ops {
		if op.kind != pipeLog {
			filtered = append(filtered, op)
		}
	}
	ops = filtered

	flags.HasFormat = hasFormatOp(ops)

	if !flags.HasFormat {
		ops = append(ops, pipeOp{kind: configuredDefault(sessionFormat)})
	}

	return cmd, func(rawJSON string) string {
		result, pipeErr := ApplyPipes(rawJSON, ops, meta)
		if pipeErr != "" {
			return pipeError(pipeErr)
		}
		return result
	}, flags, ""
}

var _ = env.MustRegister(env.EnvEntry{Key: "ze.cli.format", Type: "string", Default: "text", Description: "Default CLI output format (text, table, json, yaml, ndjson)"})

// configuredDefault returns the default pipe format to use when the input has no
// explicit format pipe.
//
// sessionFormat is the caller's per-session `set cli format` override; empty means
// "no override", in which case the configured default (ze.cli.format, plumbed from
// the environment cli format default YANG leaf) applies. The override is passed in
// rather than read from a global because it is per-session state: a CLI session
// storing it in the environment would change the format for every other concurrent
// session in the process.
//
// Falls back to pipeText if unset or invalid.
func configuredDefault(sessionFormat string) pipeKind {
	v := sessionFormat
	if v == "" {
		v = env.Get("ze.cli.format")
	}
	switch v {
	case "text":
		return pipeText
	case "table":
		return pipeTable
	case "json":
		return pipeJSON
	case "yaml":
		return pipeYAML
	case "ndjson":
		return pipeNDJSON
	default:
		return pipeText
	}
}

// ProcessPipesDefaultFormatChecked is ProcessPipesChecked but defaults to the
// configured format when no explicit format pipe (json, table, yaml, text) is
// specified.
//
// sessionFormat is the caller's per-session format override; pass "" for none.
// See configuredDefault.
func ProcessPipesDefaultFormatChecked(input, sessionFormat string) (command string, format func(string) string, errMsg string) {
	command, ops := ParsePipe(input)
	command, ops, meta := foldFilters(command, ops)
	if msg := ValidatePipes(ops); msg != "" {
		return command, nil, msg
	}

	if !hasFormatOp(ops) {
		ops = append(ops, pipeOp{kind: configuredDefault(sessionFormat)})
	}

	return command, func(rawJSON string) string {
		result, errMsg := ApplyPipes(rawJSON, ops, meta)
		if errMsg != "" {
			return pipeError(errMsg)
		}
		return result
	}, ""
}

// ProcessPipesDefaultFunc is like ProcessPipesChecked but applies defaultFn as the
// formatter when no explicit format pipe (json, table, yaml, text) is specified.
// This allows callers to provide a domain-specific formatter (e.g., compact
// one-liner for streaming monitors) while still respecting explicit pipes.
func ProcessPipesDefaultFunc(input string, defaultFn func(string) string) (command string, format func(string) string) {
	command, ops := ParsePipe(input)
	command, ops, meta := foldFilters(command, ops)

	if !hasFormatOp(ops) {
		if len(ops) == 0 {
			return command, func(s string) string { return defaultFn(injectPipeMeta(s, meta)) }
		}
		// Non-format ops (match, count) still apply before the default formatter.
		return command, func(rawJSON string) string {
			result, errMsg := ApplyPipes(rawJSON, ops, meta)
			if errMsg != "" {
				return pipeError(errMsg)
			}
			return defaultFn(result)
		}
	}

	return command, func(rawJSON string) string {
		result, errMsg := ApplyPipes(rawJSON, ops, meta)
		if errMsg != "" {
			return pipeError(errMsg)
		}
		return result
	}
}
