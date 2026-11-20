// Design: docs/architecture/api/commands.md — the pipe chain: declared operations and the residual chain
// Related: pipe.go — the chain parse, ApplyPipes and the surface wrappers that call the partition
// Related: pipe_filter.go — the registry of a command's declared operations
// Related: pipe_catalog.go — overridability and the answer shape an override is held to
//
// pipe_partition.go is the ONE place a typed pipe chain is split between the
// command and the engine. The command part is never rewritten: the operations
// the command declares leave the chain as a structured list its handler
// receives, and the rest of the chain runs over the answer. The check that
// holds an override's answer to its generic's shape lives beside it: for one
// answer (CheckOverrideAnswer) and for each event of a stream
// (overrideEventJudge).

package command

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// pipePartition is a typed chain split for one command.
//
// command is the command part as typed, trimmed and never rewritten. declared
// is the operations the command declares, in chain order, each with the
// argument words the chain grammar decoded. residual is every other operation,
// in chain order, for ApplyPipes; a refusal is carried there as one
// pipeInvalid operation, which every caller's validation reports by its text.
// meta records every row-shaping operation of the whole chain, declared ones
// included, because they shaped the answer as much as a generic one.
type pipePartition struct {
	command  string
	declared []PipeOperation
	residual []pipeOp
	meta     pipeChainMeta
}

// partitionPipeChain splits ops, the expanded and bound chain typed after
// command, between the command's declared operations and the residual chain.
//
// Known catalog arguments and count's document followers are validated over
// the whole chain BEFORE any operation is taken out of it, so a declared count
// cannot hide a follower it makes invalid.
//
// The prefix rule: an operation the command declares is taken only while every
// operation before it was taken too. From the first residual operation on, a
// generic-named operation (an override) stays residual and runs as the generic
// over the answer in hand, and an operation only the command declares is
// refused by name, with the operator's line reordered as its fix: the command
// applies it before its answer exists, so it cannot follow an operation that
// acts on that answer.
//
// A command that declares no set leaves the chain whole. On a command that
// does, an operation that is neither generic nor declared is refused with the
// valid set (unknownFilterError).
func partitionPipeChain(command string, ops []pipeOp) pipePartition {
	part := pipePartition{command: strings.TrimSpace(command), meta: collectPipeMeta(ops)}
	if msg := validateKnownPipeArguments(ops); msg != "" {
		part.residual = []pipeOp{{kind: pipeInvalid, arg: msg}}
		return part
	}
	if msg := validateCountDocumentFollowers(ops); msg != "" {
		part.residual = []pipeOp{{kind: pipeInvalid, arg: msg}}
		return part
	}

	set, ok := lookupPipeFilters(part.command)
	if !ok || len(set.filters) == 0 {
		part.residual = ops
		return part
	}

	firstResidual := -1
	var late []int
	for i, op := range ops {
		filter, args, declared := declaredOperation(set, op)
		if !declared {
			if op.kind == pipeUnknown {
				op = pipeOp{kind: pipeInvalid, arg: unknownFilterError(part.command, op.words, set)}
			}
			if firstResidual < 0 {
				firstResidual = i
			}
			part.residual = append(part.residual, op)
			continue
		}
		if firstResidual >= 0 {
			if _, generic := lookupPipeOperatorByName(filter.Name); generic {
				// An override typed after a residual operation is the generic,
				// over the answer in hand: the order the operator typed.
				part.residual = append(part.residual, op)
				continue
			}
			late = append(late, i)
			continue
		}
		if msg := validateDeclaredArgument(filter, args); msg != "" {
			part.residual = append(part.residual, pipeOp{kind: pipeInvalid, arg: msg})
			continue
		}
		part.declared = append(part.declared, PipeOperation{Name: filter.Name, Args: args})
	}
	if len(late) > 0 {
		part.residual = []pipeOp{{kind: pipeInvalid, arg: lateOperationRefusal(part.command, ops, firstResidual, late)}}
	}
	return part
}

// declaredOperation answers the command's declaration op names and the
// argument words op carries, or false when the command declares no operation of
// that name. A generic operator is declared only through its catalog name, so
// an op built in code with no words still finds its override.
func declaredOperation(set pipeFilterSet, op pipeOp) (PipeFilter, []string, bool) {
	if op.kind == pipeUnknown {
		return lookupFilter(set, op.words)
	}
	entry, generic := lookupPipeOperatorByKind(op.kind)
	if !generic {
		return PipeFilter{}, nil, false
	}
	filter, owned := set.byName[entry.Name]
	if !owned {
		return PipeFilter{}, nil, false
	}
	if len(op.words) > 0 {
		return filter, argumentWords(op.words), true
	}
	if op.arg == "" {
		return filter, nil, true
	}
	return filter, []string{op.arg}, true
}

// argumentWords answers the words after an operation's name, or nil when the
// operation carries none. Nil is the one spelling of "no argument" every branch
// of the partition answers: a handler compares one value, and nil survives a
// JSON round trip unchanged whether the field is written as null or omitted,
// where an omitted empty slice would come back nil and differ from what the
// sender held.
func argumentWords(words []string) []string {
	if len(words) < 2 {
		return nil
	}
	return words[1:]
}

// lookupFilter finds the command's filter that words names, its name first, and
// answers the argument words after the name as the chain grammar decoded them.
func lookupFilter(set pipeFilterSet, words []string) (PipeFilter, []string, bool) {
	if len(words) == 0 {
		return PipeFilter{}, nil, false
	}
	filter, ok := set.byName[strings.ToLower(words[0])]
	if !ok {
		return PipeFilter{}, nil, false
	}
	return filter, argumentWords(words), true
}

// validateDeclaredArgument judges the argument words the chain grammar decoded
// for a declared operation against its argument kind, the closed vocabulary a
// builtin's PipeFilter and a plugin's declaration share. An override's words
// were already judged against the catalog by validateKnownPipeArguments, and
// its recorded kind is the catalog's, so the two judgments agree.
func validateDeclaredArgument(filter PipeFilter, values []string) string {
	var tb textbuf.Buffer
	tb.Str("pipe filter ").Str(filter.Name)
	switch filter.Argument {
	case ArgNone:
		if len(values) == 0 {
			return ""
		}
		return tb.Str(" does not accept an argument").String()
	case ArgOptional:
		if len(values) <= 1 {
			return ""
		}
		return tb.Str(" accepts at most one argument").String()
	case ArgText, ArgFields:
		if len(values) > 0 {
			return ""
		}
		return tb.Str(" requires an argument").String()
	case ArgCount:
		if len(values) != 1 {
			return tb.Str(" accepts one numeric argument").String()
		}
		n, err := strconv.Atoi(values[0])
		if err != nil {
			return tb.Str(" requires a positive number").String()
		}
		if n <= 0 {
			return tb.Str(" requires a positive number").String()
		}
		return ""
	case ArgPath:
		if len(values) == 1 && values[0] != "" {
			return ""
		}
		return tb.Str(" requires one path").String()
	default:
		panic("BUG: pipe filter " + filter.Name + " was recorded with an argument kind outside the vocabulary")
	}
}

// unknownFilterError names the operation words holds, its name first, as no
// filter of the command.
func unknownFilterError(command string, words []string, set pipeFilterSet) string {
	name := ""
	if len(words) > 0 {
		name = words[0]
	}
	var tb textbuf.Buffer
	valid := set.filterNames()
	if valid == "" {
		return tb.Str("unknown pipe filter for ").Str(command).Str(": ").Str(name).String()
	}
	return tb.Str("unknown pipe filter for ").Str(command).Str(": ").Str(name).Str(" (valid: ").Str(valid).Byte(')').String()
}

// lateOperationRefusal refuses the first operation only the command declares
// that was typed after a residual one, and gives the operator's line reordered
// as the fix: the operations before the first residual one, then every late
// declared operation in the order typed, then the rest of the chain.
func lateOperationRefusal(command string, ops []pipeOp, firstResidual int, late []int) string {
	isLate := make(map[int]bool, len(late))
	for _, i := range late {
		isLate[i] = true
	}
	var line textbuf.Buffer
	line.Str(command)
	for _, op := range ops[:firstResidual] {
		writeChainOp(&line, op)
	}
	for _, i := range late {
		writeChainOp(&line, ops[i])
	}
	for i := firstResidual; i < len(ops); i++ {
		if !isLate[i] {
			writeChainOp(&line, ops[i])
		}
	}
	name := chainOpName(ops[late[0]])
	after := chainOpName(ops[firstResidual])
	// The refusal repeats the operator's line, so a secret on it is masked as
	// every echo of a line is (redact.Command).
	var tb textbuf.Buffer
	return tb.Str("pipe operation ").Str(name).Str(" belongs to ").Str(redact.Command(command)).
		Str(" and must come before ").Str(after).
		Str(": the command applies it before its answer exists; use: ").Str(redact.Command(line.String())).String()
}

// writeChainOp appends one operation to a line as the operator types it: a
// pipe, the name, then each argument word, quoted where a byte would change
// its boundary.
func writeChainOp(tb *textbuf.Buffer, op pipeOp) {
	tb.Str(" | ")
	if len(op.words) > 0 {
		for i, word := range op.words {
			if i > 0 {
				tb.Byte(' ')
			}
			WriteInvocationValue(tb, word)
		}
		return
	}
	tb.Str(chainOpName(op))
	if op.arg != "" {
		tb.Byte(' ').Str(op.arg)
	}
}

// chainOpName answers the name an operation was typed with.
func chainOpName(op pipeOp) string {
	if len(op.words) > 0 {
		return op.words[0]
	}
	entry, known := lookupPipeOperatorByKind(op.kind)
	if !known {
		return op.arg
	}
	return entry.Name
}

// AnswerOutcome is how a handler's answer ended, which decides whether an
// override's answer is held to its generic's shape. Its zero value is no
// outcome, and CheckOverrideAnswer refuses to guess one.
type AnswerOutcome uint8

const (
	answerOutcomeUnspecified AnswerOutcome = iota
	// AnswerDone is a complete answer the handler returned whole.
	AnswerDone
	// AnswerFailed is an error answer: it belongs to the error, not to the
	// override, so its shape is not checked.
	AnswerFailed
	// AnswerStreamed is a complete answer delivered as a generator of rows,
	// which the engine does not hold whole. It is the command's rows by
	// construction: it keeps the shape of every override that answers rows,
	// with no count of them, and never the shape of `count`. The events of a
	// monitor are another thing, judged one by one (overrideEventJudge).
	AnswerStreamed
)

// CheckOverrideAnswer holds the answer of a command's override to the shape of
// the generic it overrides (docs/architecture/api/commands.md, "Per-command
// declarations: what a command says about itself"). Every route that hands a
// handler its declared list judges the answer: Dispatch and ServeLocal call
// this function on the one answer, and the stream route judges each event
// inside StreamLine.Run (overrideEventJudge), so the check is a step beside
// the partition and not of one route.
//
// path is the command path, operations the declared list the handler was
// handed, and document the answer's JSON. Only the LAST declared operation is
// judged, and only when it is an override: an error answer, an override
// followed by another declared operation and a list ending in an own-named
// operation are not checked, because the answer then belongs to something
// else.
//
// The answer of a `count` override is one object holding an integer `count`;
// that of `first N` and `last N` the command's rows, at most N of them; that of
// every other data operator (`match`, `display`, `fill`, `resolve`, `origin`)
// the command's rows, the envelope its generic keeps. The catalog entry's
// override shape decides which, so an operator the catalog adds is held to the
// shape its entry states.
//
// On AnswerStreamed document is not read, because the rows were not collected:
// the answer is rows, which a `count` override never answers and every other
// override does, and `first N` and `last N` are not counted.
func CheckOverrideAnswer(path string, operations []PipeOperation, outcome AnswerOutcome, document string) error {
	switch outcome {
	case AnswerDone, AnswerStreamed:
	case AnswerFailed:
		return nil
	case answerOutcomeUnspecified:
		panic("BUG: CheckOverrideAnswer called with no answer outcome")
	default:
		panic("BUG: CheckOverrideAnswer called with an unknown answer outcome")
	}
	generic, args, held := lastOverride(operations)
	if !held {
		return nil
	}
	if outcome == AnswerStreamed {
		return checkOverrideRowGenerator(path, generic)
	}
	var data any
	if err := json.Unmarshal([]byte(document), &data); err != nil {
		return overrideShapeRefusal(path, generic, "an answer that is not one JSON document")
	}
	switch generic.override {
	case overrideCount:
		isObject, holdsCount := countObject(data)
		if !isObject {
			return overrideShapeRefusal(path, generic, "an answer that is not one object")
		}
		if !holdsCount {
			return overrideShapeRefusal(path, generic, "an object with no integer count")
		}
		return nil
	case overrideRowsAtMost:
		rows, _, hasRows := rowsIn(data)
		if !hasRows {
			return overrideShapeRefusal(path, generic, "an answer with no rows")
		}
		limit, err := strconv.Atoi(firstArg(args))
		if err != nil {
			return overrideShapeRefusal(path, generic, "an answer to a count it cannot read")
		}
		if len(rows) > limit {
			return overrideShapeRefusal(path, generic, "more rows than it was asked for")
		}
		return nil
	case overrideRows:
		if _, _, hasRows := rowsIn(data); !hasRows {
			return overrideShapeRefusal(path, generic, "an answer with no rows")
		}
		return nil
	case overrideNone:
		panic("BUG: command " + path + " recorded an override of " + generic.Name + ", which the catalog does not let a command override")
	default:
		panic("BUG: unknown override shape of " + generic.Name)
	}
}

// lastOverride answers the generic the last operation of operations overrides,
// and that operation's argument words. held is false when the list is empty or
// ends in an operation of the command's own name: no override owns the answer
// then, and nothing is judged.
func lastOverride(operations []PipeOperation) (generic PipeOperator, args []string, held bool) {
	if len(operations) == 0 {
		return PipeOperator{}, nil, false
	}
	last := operations[len(operations)-1]
	generic, held = lookupPipeOperatorByName(last.Name)
	if !held {
		return PipeOperator{}, nil, false
	}
	return generic, last.Args, true
}

// countObject answers whether data is one object, and whether that object
// holds an integer `count`, the shape applyCount answers.
func countObject(data any) (isObject, holdsCount bool) {
	object, isObject := data.(map[string]any)
	if !isObject {
		return false, false
	}
	count, isNumber := object[pipeNameCount].(float64)
	if !isNumber {
		return true, false
	}
	return true, count == float64(int64(count))
}

// checkOverrideRowGenerator judges an answer delivered as a generator of rows
// (AnswerStreamed) without reading a row: what was answered is rows.
func checkOverrideRowGenerator(path string, generic PipeOperator) error {
	switch generic.override {
	case overrideCount:
		return overrideShapeRefusal(path, generic, "its rows, one by one, where one count was asked for")
	case overrideRowsAtMost, overrideRows:
		return nil
	case overrideNone:
		panic("BUG: command " + path + " recorded an override of " + generic.Name + ", which the catalog does not let a command override")
	default:
		panic("BUG: unknown override shape of " + generic.Name)
	}
}

// acceptStreamEvent is the judge of a stream no override answers: every event
// passes.
func acceptStreamEvent(string) error { return nil }

// overrideEventJudge answers the function that holds each event of a monitor's
// stream to the shape of the generic the command overrides. It is
// CheckOverrideAnswer for the stream route, where no answer ever ends: the
// same last declared operation is judged, on every event the handler writes.
//
// A stream's rows are its events (applyEventMatch), so an event is one row of
// the command's rows. The event of a `count` override is one object holding an
// integer `count`; that of every other override one JSON document, which is
// the row. The events of `first N` and `last N` are not counted, because the
// engine holds no stream whole. A blank line is no event and is not judged.
//
// path is the command as typed, named in the refusal by the path its
// operations are declared at. held is false when no override owns the stream:
// the judge then passes every event, and a caller need not run it.
func overrideEventJudge(path string, operations []PipeOperation) (judge func(event string) error, held bool) {
	generic, _, held := lastOverride(operations)
	if !held {
		return acceptStreamEvent, false
	}
	if _, declaredAt, found := pipeFilterRegistry.lookupPath(path); found {
		path = declaredAt
	}
	switch generic.override {
	case overrideCount:
		return func(event string) error { return checkCountEvent(path, generic, event) }, true
	case overrideRowsAtMost, overrideRows:
		return func(event string) error { return checkRowEvent(path, generic, event) }, true
	case overrideNone:
		panic("BUG: command " + path + " recorded an override of " + generic.Name + ", which the catalog does not let a command override")
	default:
		panic("BUG: unknown override shape of " + generic.Name)
	}
}

// checkCountEvent holds one event of a `count` override's stream to one object
// holding an integer `count`.
func checkCountEvent(path string, generic PipeOperator, event string) error {
	if strings.TrimSpace(event) == "" {
		return nil
	}
	var data any
	if err := json.Unmarshal([]byte(event), &data); err != nil {
		return overrideEventRefusal(path, generic, "an event that is not one JSON document")
	}
	isObject, holdsCount := countObject(data)
	if !isObject {
		return overrideEventRefusal(path, generic, "an event that is not one object")
	}
	if !holdsCount {
		return overrideEventRefusal(path, generic, "an event with no integer count")
	}
	return nil
}

// checkRowEvent holds one event of a rows override's stream to one row: one
// JSON document, whatever its fields, which are the override's meaning.
func checkRowEvent(path string, generic PipeOperator, event string) error {
	if strings.TrimSpace(event) == "" {
		return nil
	}
	if !json.Valid([]byte(event)) {
		return overrideEventRefusal(path, generic, "an event that is not one JSON document")
	}
	return nil
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// overrideShapeRefusal names the command whose override of generic answered
// what the generic never answers.
func overrideShapeRefusal(path string, generic PipeOperator, answered string) error {
	var problem, why, fix textbuf.Buffer
	problem.Str("The override of ").Str(generic.Name).Str(" by ").Str(path).Str(" answered ").Str(answered)
	why.Str("A command that overrides ").Str(generic.Name).Str(" keeps its answer shape: ").Str(overrideShapeText(generic))
	fix.Str(`show command help "`).Str(path).Byte('"')
	return opererr.New(problem.String(), why.String(), fix.String())
}

// overrideShapeText says in words what an override of generic answers.
func overrideShapeText(generic PipeOperator) string {
	switch generic.override {
	case overrideCount:
		return `one object holding an integer count, {"count": N}`
	case overrideRowsAtMost:
		return "the command's rows, at most the N asked for"
	case overrideRows:
		return "the command's rows"
	case overrideNone:
		return "none, because it cannot be overridden"
	default:
		panic("BUG: unknown override shape of " + generic.Name)
	}
}

// overrideEventRefusal names the command whose override of generic wrote, on
// its stream, an event the generic never answers.
func overrideEventRefusal(path string, generic PipeOperator, answered string) error {
	var problem, why, fix textbuf.Buffer
	problem.Str("The override of ").Str(generic.Name).Str(" by ").Str(path).Str(" answered ").Str(answered)
	why.Str("A command that overrides ").Str(generic.Name).Str(" keeps its answer shape on a stream: ").Str(overrideEventShapeText(generic))
	fix.Str(`show command help "`).Str(path).Byte('"')
	return opererr.New(problem.String(), why.String(), fix.String())
}

// overrideEventShapeText says in words what each event of a stream answered by
// an override of generic is.
func overrideEventShapeText(generic PipeOperator) string {
	switch generic.override {
	case overrideCount:
		return `each event is one object holding an integer count, {"count": N}`
	case overrideRowsAtMost, overrideRows:
		return "each event is one row, a JSON document"
	case overrideNone:
		return "none, because it cannot be overridden"
	default:
		panic("BUG: unknown override shape of " + generic.Name)
	}
}
