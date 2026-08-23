// Design: docs/architecture/api/commands.md — CLI pipe operators
// Detail: pipe_table.go — table rendering (ApplyTable)
// Detail: pipe_records.go — the same chain over a streamed answer, one record at a time
// Related: format.go — YAML and number formatting
//
// pipe.go implements VyOS-style pipe operators for command output.
// Users can append | match <pattern>, | count, | no-more, | json [compact|pretty],
// | table, | yaml to any command.
//
// The DAEMON runs the chain, not the client. A client sends the operator's text
// with its pipes intact and prints the answer (cliClient.Execute,
// internal/component/cli/client/main.go). The SSH exec handler splits the chain
// off the command and applies it to the dispatcher's JSON (execMiddleware,
// internal/component/ssh/ssh.go). One implementation therefore renders every
// surface, and the daemon is the process that holds the configured default
// format.
package command

import (
	"encoding/json"
	"sort"
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
	pipeDisplay                 // | display <field>... — the fields to answer with, in that order
	pipeFill                    // | fill [<way>] [reverse] — bring the remaining fields back, in a named order
	pipeSave                    // | save <path> — write the answer to a file
	pipeUnknown                 // unrecognized operator
	pipeInvalid                 // validation error produced while expanding an alias or folding a command filter
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

// ParsePipe splits user input into the command and a chain of pipe operators.
// Input "show bgp peer list | match established | count" returns ("show bgp peer list", [{match,"established"}, {count,""}]).
func ParsePipe(input string) (command string, ops []pipeOp) {
	command, rest, hasPipe := strings.Cut(input, "|")
	command = strings.TrimSpace(command)
	if !hasPipe {
		return command, nil
	}
	return command, parsePipeOps(rest)
}

// parsePipeOps parses the operator chain that follows the first pipe character.
// Each segment between two pipe characters is one operator and its argument.
//
// An alias registers its expansion in this same spelling, so RegisterAliases
// parses it here rather than carrying a second reading of the operator names.
func parsePipeOps(rest string) []pipeOp {
	var ops []pipeOp

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
		case pipeMatch, pipeDisplay, pipeFill, pipeSave:
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

	return ops
}

// parsePipeChain splits user input into the command and the operator chain it
// runs, with every alias replaced by the operators it names.
//
// A caller uses it instead of ParsePipe. An alias is a spelling of an operator
// chain rather than a chain of its own. Expansion happens before foldFilters.
// The ops that reach classification are therefore ops the parser already knows.
func parsePipeChain(input string) (command string, ops []pipeOp) {
	command, ops = ParsePipe(input)
	return command, expandAliases(command, ops)
}

// expandAliases replaces each alias in a chain with the operators it names.
//
// This is ONE pass, and it can produce no alias of its own. RegisterAliases
// refuses an alias whose expansion names another alias, so every operator
// spliced in here is one ParsePipe already knows. The result therefore holds at
// most one expansion for each op of the input chain, each of a length fixed at
// registration.
//
// An alias takes no argument. A word after the name is refused rather than
// dropped, so an operator who expected it to filter is told that it does not.
func expandAliases(command string, ops []pipeOp) []pipeOp {
	if len(ops) == 0 {
		return ops
	}

	expanded := make([]pipeOp, 0, len(ops))
	for _, op := range ops {
		// Only an unrecognized operator can be an alias, so no other kind pays
		// for the split.
		if op.kind != pipeUnknown {
			expanded = append(expanded, op)
			continue
		}
		fields := strings.Fields(op.arg)
		if len(fields) == 0 {
			expanded = append(expanded, op)
			continue
		}
		entry, ok := lookupAlias(command, fields[0])
		if !ok {
			expanded = append(expanded, op)
			continue
		}
		if len(fields) > 1 {
			var tb textbuf.Buffer
			message := tb.Str("pipe alias ").Str(entry.alias.Name).Str(" does not accept an argument").String()
			expanded = append(expanded, pipeOp{kind: pipeInvalid, arg: message})
			continue
		}
		expanded = append(expanded, entry.ops...)
	}
	return expanded
}

// foldFilters rewrites command-owned pipe filters into command arguments.
// A folded filter runs in the command handler, at the source of the data.
// Every other operator stays in the chain ApplyPipes runs over the answer.
// Returns pipe metadata recording all data-shaping modifiers (both folded
// and remaining). Display-only pipes are excluded from metadata.
func foldFilters(command string, ops []pipeOp) (string, []pipeOp, PipeChainMeta) {
	meta := collectPipeMeta(ops)

	trimmed := strings.TrimSpace(command)

	set, ok := lookupPipeFilters(trimmed)
	if !ok || len(set.filters) == 0 {
		return command, ops, meta
	}

	var leadingArgs []string
	var serverArgs []string
	var chainOps []pipeOp

	for _, op := range ops {
		switch op.kind {
		case pipeMatch:
			if filter, ok := set.byName["match"]; ok {
				serverArgs = appendFilter(serverArgs, filter, op.arg)
			} else {
				chainOps = append(chainOps, op)
			}
		case pipeCount:
			if filter, ok := set.byName["count"]; ok {
				serverArgs = appendFilter(serverArgs, filter, "")
			} else {
				chainOps = append(chainOps, op)
			}
		case pipeFirst:
			if filter, ok := set.byName["first"]; ok {
				serverArgs = appendFilter(serverArgs, filter, op.arg)
			} else {
				chainOps = append(chainOps, op)
			}
		case pipeLast:
			if filter, ok := set.byName["last"]; ok {
				serverArgs = appendFilter(serverArgs, filter, op.arg)
			} else {
				chainOps = append(chainOps, op)
			}
		case pipeUnknown:
			filter, arg, known := lookupFilter(set, op.arg)
			if !known {
				chainOps = append(chainOps, pipeOp{kind: pipeInvalid, arg: unknownFilterError(trimmed, op.arg, set)})
				continue
			}
			if msg := validateFilter(filter, arg); msg != "" {
				chainOps = append(chainOps, pipeOp{kind: pipeInvalid, arg: msg})
				continue
			}
			if filter.Leading {
				leadingArgs = appendFilter(leadingArgs, filter, arg)
			} else {
				serverArgs = appendFilter(serverArgs, filter, arg)
			}
		default:
			// Every other kind is the operator's own request about the answer
			// they are reading, so it stays in the chain. The default arm is
			// what makes that true for a kind nobody thought of here. Without
			// one, such a kind reached neither side and nothing reported the
			// loss. pipeDisplay, pipeFill and pipeInvalid all arrive this way.
			chainOps = append(chainOps, op)
		}
	}

	allServerArgs := make([]string, 0, len(leadingArgs)+len(serverArgs))
	allServerArgs = append(allServerArgs, leadingArgs...)
	allServerArgs = append(allServerArgs, serverArgs...)
	if len(allServerArgs) > 0 {
		var tb textbuf.Buffer
		command = tb.Str(trimmed).Byte(' ').Join(allServerArgs, " ").String()
	}

	return command, chainOps, meta
}

// PipeChainStep is one operator that shaped an answer.
type PipeChainStep struct {
	Op  string `json:"op"`
	Arg string `json:"arg,omitempty"`
}

// PipeChainMeta records the operators that shaped an answer, IN CHAIN ORDER.
//
// It was a map keyed by operator name, which could not record a chain that
// repeats one: `match idle | match 192` wrote meta["match"] twice and published
// only "192", so the key a tool author parses under-reported the chain that
// actually ran. A map has no order either, and order is what a chain is.
type PipeChainMeta []PipeChainStep

// collectPipeMeta records the data-shaping pipe ops, in chain order.
// Display-only pipes (json, ndjson, table, text, yaml, resolve, origin, log, no-more)
// are excluded: they change no data, so they shaped nothing to report.
func collectPipeMeta(ops []pipeOp) PipeChainMeta {
	var meta PipeChainMeta
	for _, op := range ops {
		switch op.kind { //nolint:exhaustive // only data-shaping ops
		case pipeMatch:
			meta = append(meta, PipeChainStep{Op: "match", Arg: op.arg})
		case pipeCount:
			meta = append(meta, PipeChainStep{Op: "count"})
		case pipeFirst:
			if _, err := strconv.Atoi(op.arg); err == nil {
				meta = append(meta, PipeChainStep{Op: "first", Arg: op.arg})
			}
		case pipeLast:
			if _, err := strconv.Atoi(op.arg); err == nil {
				meta = append(meta, PipeChainStep{Op: "last", Arg: op.arg})
			}
		case pipeUnknown:
			// A command-owned filter or an alias the fold left behind, which
			// shaped the answer as much as a generic operator did.
			fields := strings.Fields(op.arg)
			if len(fields) == 1 {
				meta = append(meta, PipeChainStep{Op: fields[0]})
			} else if len(fields) >= 2 {
				meta = append(meta, PipeChainStep{Op: fields[0], Arg: textbuf.Join(fields[1:], " ")})
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
// Rejects a chain that names more than one format operator. isFormatOp states
// which operators those are, and it is the only place that set is written.
// If meta is non-nil, injects a "pipe" key into JSON output before formatting.
//
// columns carries the column order the command declared, which reaches the
// table and text renderers only. A program reads json, ndjson and yaml, and
// column order carries no meaning for a program (owner directive, 2026-08-19).
//
// An `| display` the operator typed is the other half of that answer. Its
// SEQUENCE reaches the same two renderers and replaces the declared order, and
// so does the sequence an `| fill` asks for. SELECTION reaches every format,
// because which fields to answer with is a question the operator asked out
// loud rather than a rendering choice.
func ApplyPipes(output string, ops []pipeOp, meta PipeChainMeta, columns []ColumnOrder) (string, string) {
	request := columnsInChain(ops)
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
			matched, msg := applyMatch(result, op.arg)
			if msg != "" {
				return "", msg
			}
			result = matched
		case pipeCount:
			counted, msg := applyCount(result)
			if msg != "" {
				return "", msg
			}
			result = counted
		case pipeNoMore:
			// No-op: paging not yet implemented
		case pipeJSON:
			result = ApplyJSON(result, op.arg)
		case pipeNDJSON:
			result = applyNDJSON(result)
		case pipeTable:
			result = applyTableStyled(result, tableStyle{orders: columns, request: request})
		case pipeText:
			result = applyTableStyled(result, tableStyle{plain: true, orders: columns, request: request})
		case pipeYAML:
			result = applyYAML(result)
		case pipeRaw:
			// Identity: the dispatcher's JSON is already the answer.
		case pipeResolve:
			result = applyResolve(result)
		case pipeOrigin:
			result = applyOrigin(result)
		case pipeFirst:
			taken, msg := applyFirst(result, op.arg)
			if msg != "" {
				return "", msg
			}
			result = taken
		case pipeLast:
			taken, msg := applyLast(result, op.arg)
			if msg != "" {
				return "", msg
			}
			result = taken
		case pipeDisplay:
			result = applyDisplaySelect(result, request)
		case pipeFill:
			// A sequence for the remaining fields, which changes no data. The
			// renderers read it from tableStyle.
		case pipeLog:
			// Display-mode modifier, not a data transform. Handled by caller.
		case pipeSave:
			// Applied after the whole chain, below: what an operator means by
			// saving is the answer they are looking at, and the configured
			// default format is appended to the END of a chain that names none.
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
	if paths := savePathsInChain(ops); len(paths) > 0 {
		if msg := applySaves(result, paths); msg != "" {
			return "", msg
		}
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
	command, ops := parsePipeChain(input)
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
		if op.kind == pipeDisplay {
			// A display that names no field selects nothing. Saying so beats
			// an answer the operator cannot tell from a command that produced
			// no data.
			if msg := displayError(op.arg); msg != "" {
				return msg
			}
		}
		if op.kind == pipeFill {
			// A word nobody reads is a request nobody answered, so an
			// unrecognized way is refused by name rather than ignored.
			if msg := fillError(op.arg); msg != "" {
				return msg
			}
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
	if msg := validateRepeats(ops); msg != "" {
		return msg
	}
	if msg := validateDisplayNarrowing(ops); msg != "" {
		return msg
	}
	// The surface half of the save rules is checked by the entry point, which
	// is the only place that knows whose filesystem the write would land on.
	if msg := validateSaveOps(ops, true); msg != "" {
		return msg
	}
	return ""
}

// validatePipesForSurface is ValidatePipes plus the rule that depends on WHICH
// process expands the chain: `| save` writes a file, so it is refused when the
// daemon is expanding the chain for a remote caller.
func validatePipesForSurface(command string, ops []pipeOp, saveAllowed bool) string {
	if msg := ValidatePipes(ops); msg != "" {
		return msg
	}
	if msg := validateSaveOps(ops, saveAllowed); msg != "" {
		return msg
	}
	return validateDeclaredShape(command, ops)
}

// validateDisplayNarrowing refuses a chain whose `| display` requests have no
// field in common, because the answer would carry no fields at all and the
// operator would have no way to see why.
func validateDisplayNarrowing(ops []pipeOp) string {
	var request ColumnOrder
	seen := 0
	for _, op := range ops {
		if op.kind != pipeDisplay {
			continue
		}
		seen++
		request = narrowDisplay(request, parseDisplay(op.arg))
	}
	if seen < 2 || len(request) > 0 {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str("display selects no field: each display narrows the one before it, ").
		Str("and these name nothing in common").String()
}

// validateDeclaredShape refuses an operator the command's DECLARED shape cannot
// support, before the command runs.
//
// The answer's own shape refuses too, at apply time, and that covers every
// command including the ones that declare nothing. This exists for the case the
// answer cannot decide: `show config dump` answers a nested configuration tree,
// and a tree whose one top-level key holds a map of maps is indistinguishable
// from rows keyed by identity. The command knows it is one document; the
// payload does not say so.
//
// It is also what makes the published surface true. `ze help command --json`
// lists a declared command's operators FROM this shape, so without the refusal
// the catalog would promise nine operators and the runtime would accept
// fifteen.
func validateDeclaredShape(command string, ops []pipeOp) string {
	shape, declared := ShapeForCommand(command)
	if !declared {
		return ""
	}
	addressFields := len(AddressFieldsForCommand(command)) > 0

	for _, op := range ops {
		entry, known := lookupPipeOperatorByKind(op.kind)
		if !known {
			continue
		}
		if entry.NeedsAddressField && !addressFields {
			var tb textbuf.Buffer
			return tb.Str(entry.Name).Str(" cannot apply here: no field of this ").
				Str("command's answer is declared to hold an IP address").String()
		}
		if entry.Applies(shape) {
			continue
		}
		// The command is not named: parsePipeChain answers the command WITH its
		// arguments, so naming it here would echo a file path back at an
		// operator who just typed it.
		var tb textbuf.Buffer
		return tb.Str(entry.Name).Str(" cannot apply here: this command answers ").
			Str(shapeDescription(shape)).Str(", and ").Str(entry.Name).
			Str(" acts on rows").String()
	}
	return ""
}

// shapeDescription says what a shape holds, in the words a refusal reads best
// with.
func shapeDescription(shape AnswerShape) string {
	if shape == ShapeDoc {
		return "one document"
	}
	return "rows"
}

// lookupPipeOperatorByKind answers the catalog entry for a parsed operator.
func lookupPipeOperatorByKind(kind pipeKind) (PipeOperator, bool) {
	for _, op := range pipeCatalog {
		if op.Kind == kind {
			return op, true
		}
	}
	return PipeOperator{}, false
}

// validateRepeats refuses a second occurrence of an operator whose catalog
// entry says repetition has no honest answer.
//
// Four different things used to happen when an operator was repeated: the
// document data path composed, the document metadata path overwrote, the folded
// filter path appended, and the column path replaced. `display state | display
// address` therefore WIDENED, recovering the field the first request dropped.
// An operator that composes is checked by its own path; this refuses the ones
// where picking a winner would be a guess, so `fill alpha | fill reverse` no
// longer silently answers the second one alone.
func validateRepeats(ops []pipeOp) string {
	seen := make(map[pipeKind]int, len(ops))
	for _, op := range ops {
		seen[op.kind]++
	}
	for _, entry := range pipeCatalog {
		if entry.Repeat != RepeatRefuse || seen[entry.Kind] < 2 {
			continue
		}
		var tb textbuf.Buffer
		return tb.Str(entry.Name).Str(" cannot be repeated in one chain: ").
			Str("a second ").Str(entry.Name).Str(" would act on the first one's answer").String()
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

// applyMatch keeps the rows holding the text, case-insensitively.
//
// It used to grep LINES of whatever string was in hand. On the default chain
// that string is the dispatcher's compact JSON, which is one line, so a bare
// `| match idle` answered every row or none: three peers in, three peers out.
// The behavior only looked right when a format operator came first, and only
// the second spelling is what an operator types.
//
// A rendered answer still greps lines, because after `| text` the rows ARE
// lines and that is what the reader asked for.
func applyMatch(input, pattern string) (string, string) {
	lower := strings.ToLower(pattern)

	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err == nil {
		rows, keys, key, ok := rowsInKeyed(data)
		if !ok {
			return "", rowOperatorRefusal("match", data)
		}
		keep := make([]int, 0, len(rows))
		for i, row := range rows {
			var hay textbuf.Buffer
			if keys != nil {
				// The identity key IS a value to a reader: it is the peer
				// address `show bgp peer list` is keyed by.
				hay.Str(strings.ToLower(keys[i])).Byte(' ')
			}
			appendValueText(&hay, row)
			if strings.Contains(hay.String(), lower) {
				keep = append(keep, i)
			}
		}
		out, marshalErr := json.Marshal(selectRows(data, key, keys, rows, keep))
		if marshalErr != nil {
			return input, ""
		}
		return string(out), ""
	}

	var b textbuf.Buffer
	for line := range strings.SplitSeq(input, "\n") {
		if strings.Contains(strings.ToLower(line), lower) {
			b.Str(line).Byte('\n')
		}
	}
	return b.String(), ""
}

// appendValueText writes a row's VALUES, lowercased, for matching against.
//
// The field NAMES are deliberately left out. Matching the marshaled row would
// make `| match state` keep every row that has a state field, which is every
// row, and a filter that always matches is worse than one that never does.
func appendValueText(buf *textbuf.Buffer, v any) {
	switch typed := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for name := range typed {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			appendValueText(buf, typed[name])
		}
	case []any:
		for _, item := range typed {
			appendValueText(buf, item)
		}
	case string:
		buf.Str(strings.ToLower(typed)).Byte(' ')
	case nil:
	default:
		encoded, err := json.Marshal(typed)
		if err == nil {
			buf.Str(strings.ToLower(string(encoded))).Byte(' ')
		}
	}
}

// applyCount counts items and returns JSON {"count": N}.
// If input is JSON, counts array elements or map keys
// (unwrapping single-key wrappers). Otherwise counts non-empty lines.
func applyCount(input string) (string, string) {
	if input == "" {
		return "{\"count\":0}\n", ""
	}
	rows, isJSON, msg := rowsForOperator(input, "count")
	if msg != "" {
		return "", msg
	}
	if isJSON {
		return textbuf.StrIntStr("{\"count\":", int64(len(rows)), "}\n"), ""
	}
	// The answer has already been rendered by a format operator upstream, so
	// its rows are lines and counting them is what was asked:
	// `show bgp | text | count`.
	n := 0
	for line := range strings.SplitSeq(input, "\n") {
		if line != "" {
			n++
		}
	}
	return textbuf.StrIntStr("{\"count\":", int64(n), "}\n"), ""
}

// rowsForOperator finds the rows a row operator acts on, and refuses by name
// when the answer has none.
//
// countItems used to answer the number of top-level KEYS for an answer carrying
// rows beside aggregates, so `show bgp | count` answered 6 and
// `show bgp rpki | count` answered 8. Both are plausible numbers, and neither
// is the question that was asked. truncateItems had the same hole from the
// other side: it unwrapped a map of exactly one key and left a map of six
// alone, so `show bgp | first 1` answered the whole payload.
//
// isJSON false means a format operator upstream already rendered the answer,
// and the operator falls back to lines.
func rowsForOperator(input, operator string) (rows []any, isJSON bool, errMsg string) {
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		return nil, false, ""
	}
	rows, _, ok := rowsIn(data)
	if ok {
		return rows, true, ""
	}
	return nil, true, rowOperatorRefusal(operator, data)
}

// rowOperatorRefusal says why a row operator cannot apply, and what to do about
// it. An operator the answer cannot support is refused BY NAME: accepting it
// and answering something is worse, because the answer looks plausible.
func rowOperatorRefusal(operator string, data any) string {
	var tb textbuf.Buffer
	tb.Str(operator).Str(" needs rows, and this answer has none: ")
	if keys := rowKeys(data); len(keys) > 1 {
		sort.Strings(keys)
		tb.Str("it holds several lists (").Str(textbuf.Join(keys, ", ")).Str("), so select one first")
		return tb.String()
	}
	tb.Str("it holds one document")
	return tb.String()
}

// isFormatOp reports whether the operator decides the shape of the answer.
// A chain names at most one of them, and naming one stops the configured
// default being appended. pipeRaw is one: it names the dispatcher's JSON as
// the shape, which is a choice like any other.
func isFormatOp(k pipeKind) bool {
	return k == pipeJSON || k == pipeNDJSON || k == pipeTable || k == pipeText || k == pipeYAML || k == pipeRaw
}

func injectPipeMeta(input string, meta PipeChainMeta) string {
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

func applyFirst(input, arg string) (string, string) {
	return applyTake(input, arg, "first", false)
}

func applyLast(input, arg string) (string, string) {
	return applyTake(input, arg, "last", true)
}

// applyTake is `first` and `last`: both take N rows, from one end or the other.
//
// The rows are replaced IN PLACE, so an envelope keeps its aggregates and only
// its row list is shortened. truncateItems returned the whole value untouched
// for a map of more than one key, which is why `show bgp | first 1` answered
// the entire payload.
func applyTake(input, arg, operator string, fromEnd bool) (string, string) {
	n, err := strconv.Atoi(arg)
	if err != nil || n <= 0 {
		return input, ""
	}
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		if fromEnd {
			return applyLastLines(input, n), ""
		}
		return applyFirstLines(input, n), ""
	}
	rows, keys, key, ok := rowsInKeyed(data)
	if !ok {
		return "", rowOperatorRefusal(operator, data)
	}
	keep := make([]int, 0, n)
	for i := range rows {
		keep = append(keep, i)
	}
	keep = sliceInts(keep, n, fromEnd)
	data = selectRows(data, key, keys, rows, keep)
	out, err := json.Marshal(data)
	if err != nil {
		return input, ""
	}
	return string(out), ""
}

// sliceInts takes N row indices from one end, which is what `first` and `last`
// each ask for.
func sliceInts(idx []int, n int, fromEnd bool) []int {
	if n >= len(idx) {
		return idx
	}
	if fromEnd {
		return idx[len(idx)-n:]
	}
	return idx[:n]
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

// PipeErrorPrefix marks a formatted answer that is a REFUSAL rather than data.
//
// A chain can fail after the command has run — an operator the answer's shape
// cannot support is the common case — and by then the only channel back to the
// caller is the formatted string. A caller that must not treat a refusal as
// data reads this prefix; IsPipeError is that reading.
const PipeErrorPrefix = "pipe error: "

// IsPipeError reports whether a formatted answer is a refusal rather than data,
// so a caller can exit non-zero instead of printing the message as an answer.
func IsPipeError(answer string) bool {
	return strings.HasPrefix(answer, PipeErrorPrefix)
}

func pipeError(msg string) string {
	var tb textbuf.Buffer
	return tb.Str(PipeErrorPrefix).Str(msg).String()
}

// ProcessPipesChecked splits user input into a command and a formatting function,
// validating the pipe chain upfront. The returned function applies pipe operators
// (table, json, yaml, match, count) to raw JSON output. If no pipes are present,
// the formatter returns raw JSON unchanged. Returns a non-empty errMsg (and a nil
// format) if the pipe chain is invalid.
func ProcessPipesChecked(input string) (command string, format func(string) string, errMsg string) {
	command, ops := parsePipeChain(input)
	columns := ColumnsForCommand(command)
	command, ops, meta := foldFilters(command, ops)
	if msg := ValidatePipes(ops); msg != "" {
		return command, nil, msg
	}

	if len(ops) == 0 {
		return command, func(s string) string { return injectPipeMeta(s, meta) }, ""
	}

	return command, func(rawJSON string) string {
		result, errMsg := ApplyPipes(rawJSON, ops, meta, columns)
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
	cmd, ops := parsePipeChain(input)
	columns := ColumnsForCommand(cmd)
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
		result, pipeErr := ApplyPipes(rawJSON, ops, meta, columns)
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
// This is the REMOTE form: the caller is a process expanding the chain on
// somebody else's behalf, so `| save` is refused. The SSH exec channel and
// every web surface use it. A process expanding a chain the operator typed into
// it uses ProcessPipesDefaultFormatLocal instead.
//
// sessionFormat is the caller's per-session format override; pass "" for none.
// See configuredDefault.
func ProcessPipesDefaultFormatChecked(input, sessionFormat string) (command string, format func(string) string, errMsg string) {
	return processPipesDefaultFormat(input, sessionFormat, false)
}

// ProcessPipesDefaultFormatLocal is ProcessPipesDefaultFormatChecked for a
// chain the operator typed into THIS process, so the filesystem operators are
// theirs to use: `| save` writes as them, and the operating system's
// permissions are the whole answer to what they may write.
//
// The interactive client is the caller that needs it, and it is the surface a
// shell redirect cannot reach: the answer is drawn to a terminal and never
// reaches a pipe.
func ProcessPipesDefaultFormatLocal(input, sessionFormat string) (command string, format func(string) string, errMsg string) {
	return processPipesDefaultFormat(input, sessionFormat, true)
}

func processPipesDefaultFormat(input, sessionFormat string, saveAllowed bool) (command string, format func(string) string, errMsg string) {
	command, ops := parsePipeChain(input)
	columns := ColumnsForCommand(command)
	command, ops, meta := foldFilters(command, ops)
	if msg := validatePipesForSurface(command, ops, saveAllowed); msg != "" {
		return command, nil, msg
	}

	if !hasFormatOp(ops) {
		ops = append(ops, pipeOp{kind: configuredDefault(sessionFormat)})
	}

	return command, func(rawJSON string) string {
		result, errMsg := ApplyPipes(rawJSON, ops, meta, columns)
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
	command, ops := parsePipeChain(input)
	columns := ColumnsForCommand(command)
	command, ops, meta := foldFilters(command, ops)

	if !hasFormatOp(ops) {
		if len(ops) == 0 {
			return command, func(s string) string { return defaultFn(injectPipeMeta(s, meta)) }
		}
		// Non-format ops (match, count) still apply before the default formatter.
		return command, func(rawJSON string) string {
			result, errMsg := ApplyPipes(rawJSON, ops, meta, columns)
			if errMsg != "" {
				return pipeError(errMsg)
			}
			return defaultFn(result)
		}
	}

	return command, func(rawJSON string) string {
		result, errMsg := ApplyPipes(rawJSON, ops, meta, columns)
		if errMsg != "" {
			return pipeError(errMsg)
		}
		return result
	}
}
