// Design: docs/architecture/api/commands.md — CLI pipe operators
// Detail: pipe.go — the parser and the document chain that read this catalog
// Related: completer.go — Tab completion, derived from here
// Related: plan/spec-cli-pipe-operator-coverage.md — why this file exists
//
// pipe_catalog.go holds the ONE statement of the operator language: every
// operator, and what each one promises. Before it, the set was hand-copied into
// five surfaces and no two agreed — Tab completion had all sixteen, `ze help
// command --verbose` and the generated wiki page had ten, `ze pipe help` had a
// different ten, and `display` and `fill` appeared in none of the lists a user
// or a tool could reach.
//
// Everything that names an operator MUST derive it from here
// (ai/rules/evidence.md).

package command

import "strings"

// AnswerShape is what a command's answer holds, which decides which operators
// can act on it. The names match the answer head's item type on the wire
// (docs/architecture/api/wire-format.md), so a head and a declaration can be
// checked against each other.
//
// The head alone cannot be the contract: RenderRecords writes `doc` for a walk
// that ends within rpc.AnswerBufferThreshold and `map` or `tab` for one that
// passes it, so the same command answers `doc` at 200 rows and `tab` at 300.
// A command therefore DECLARES its shape and the head reports the shape of the
// answer in hand.
type AnswerShape uint8

const (
	// ShapeDoc is one document or one value. It has no rows, so no row
	// operator has anything to act on.
	ShapeDoc AnswerShape = iota
	// ShapeMap is rows that describe themselves, each carrying its own keys.
	ShapeMap
	// ShapeTab is rows read against column names the answer head declares.
	ShapeTab
)

// String answers the wire spelling of the shape.
func (s AnswerShape) String() string {
	switch s {
	case ShapeMap:
		return "map"
	case ShapeTab:
		return "tab"
	default:
		return "doc"
	}
}

// shapeSet is the set of shapes one operator acts on.
type shapeSet uint8

func setOf(shapes ...AnswerShape) shapeSet {
	var set shapeSet
	for _, shape := range shapes {
		set |= 1 << shape
	}
	return set
}

func (s shapeSet) has(shape AnswerShape) bool { return s&(1<<shape) != 0 }

// PipeClass splits the operators the owner requires of EVERY command from the
// ones a command owes only when its data supports them (owner directive,
// 2026-08-23).
type PipeClass uint8

const (
	// ClassGlobal acts on the answer whatever it holds: the formats, paging,
	// and saving. Every command owes these.
	ClassGlobal PipeClass = iota
	// ClassData acts on rows or fields. A command owes these only where its
	// shape supports them, and MUST refuse them by name where it does not.
	ClassData
	// ClassStream acts on a SEQUENCE of answers rather than on one, so it means
	// something only where a command keeps answering: the monitors.
	//
	// `log` was in the global class and was inert on both surfaces a tool
	// author uses. ApplyPipes calls it "handled by caller" and the exec channel
	// is not that caller, so a one-shot answer accepted the word and did
	// nothing with it. Publishing it as owed by every command asserted support
	// that could not exist for a command answering once: there is no second
	// update to append.
	ClassStream
)

// String answers the class name used in published documentation.
func (c PipeClass) String() string {
	switch c {
	case ClassGlobal:
		return "global"
	case ClassStream:
		return "stream"
	default:
		return "data"
	}
}

// PipeArgKind is what an operator does with an argument, which ValidatePipes
// stated three separate times before this catalog existed.
type PipeArgKind uint8

const (
	// ArgNone takes no argument.
	ArgNone PipeArgKind = iota
	// ArgOptional takes an argument or none.
	ArgOptional
	// ArgText requires a non-empty argument.
	ArgText
	// ArgCount requires a positive whole number.
	ArgCount
	// ArgFields requires one or more field names.
	ArgFields
	// ArgPath requires a filesystem path.
	ArgPath
)

// String answers the argument kind's published name.
func (a PipeArgKind) String() string {
	switch a {
	case ArgOptional:
		return "optional"
	case ArgText:
		return "text"
	case ArgCount:
		return "count"
	case ArgFields:
		return "fields"
	case ArgPath:
		return "path"
	default:
		return "none"
	}
}

// PipeRepeat says what a SECOND occurrence of the same operator in one chain
// means. Four different answers shipped before this was declared: the document
// data path composed, the document metadata path overwrote, the folded-filter
// path appended, and the column path replaced — so `display state | display
// address` recovered the field the first request had dropped.
type PipeRepeat uint8

const (
	// RepeatCompose narrows further: each occurrence applies in chain order.
	RepeatCompose PipeRepeat = iota
	// RepeatIdempotent means a second occurrence changes nothing.
	RepeatIdempotent
	// RepeatRefuse means a second occurrence is an error, because no answer to
	// "which one wins" is honest. Two orderings and two formats are the cases.
	RepeatRefuse
)

// String answers the repetition rule's published name.
func (r PipeRepeat) String() string {
	switch r {
	case RepeatIdempotent:
		return "idempotent"
	case RepeatRefuse:
		return "refuse"
	default:
		return "compose"
	}
}

// PipeOperator is one operator's whole contract.
type PipeOperator struct {
	// Name is the word an operator types after the pipe.
	Name string
	// Kind is the parser's internal tag.
	Kind pipeKind
	// Class decides whether every command owes this operator.
	Class PipeClass
	// Arg is what the operator does with an argument.
	Arg PipeArgKind
	// Repeat is what a second occurrence in one chain means.
	Repeat PipeRepeat
	// Description is the one line every published surface shows.
	Description string
	// NeedsAddressField marks an operator that acts on a field holding an IP
	// address. No shape says a field does, so these are refused until a command
	// declares one: applyResolve and applyOrigin otherwise guess by parsing
	// every value, and decorate anything that happens to parse.
	NeedsAddressField bool

	// shapes is the set this operator acts on.
	shapes shapeSet
}

// Applies answers whether this operator can act on an answer of that shape.
func (op PipeOperator) Applies(shape AnswerShape) bool { return op.shapes.has(shape) }

// ArgHint answers the argument placeholder published surfaces show after the
// operator's name, so `ze pipe help`, root help and the generated pages spell
// an operator's argument the same way.
func (op PipeOperator) ArgHint() string {
	switch op.Arg {
	case ArgOptional:
		return "[<option>]"
	case ArgText:
		return "<text>"
	case ArgCount:
		return "<n>"
	case ArgFields:
		return "<field>..."
	case ArgPath:
		return "<path>"
	default:
		return ""
	}
}

// Shapes answers the shapes this operator acts on, in wire order.
func (op PipeOperator) Shapes() []AnswerShape {
	var out []AnswerShape
	for _, shape := range []AnswerShape{ShapeDoc, ShapeMap, ShapeTab} {
		if op.shapes.has(shape) {
			out = append(out, shape)
		}
	}
	return out
}

// anyShape is every shape: the global class acts on an answer whatever it holds.
var anyShape = setOf(ShapeDoc, ShapeMap, ShapeTab)

// rowShapes are the shapes that carry rows. ShapeDoc is one value and has none.
var rowShapes = setOf(ShapeMap, ShapeTab)

// pipeCatalog is the operator language, in the order published surfaces show
// it: the formats first, then paging, then the operators that act on rows.
var pipeCatalog = []PipeOperator{
	{Name: "json", Kind: pipeJSON, Class: ClassGlobal, Arg: ArgOptional, Repeat: RepeatRefuse, shapes: anyShape,
		Description: "JSON output"},
	{Name: "ndjson", Kind: pipeNDJSON, Class: ClassGlobal, Arg: ArgNone, Repeat: RepeatRefuse, shapes: anyShape,
		Description: "One JSON object per line"},
	{Name: "table", Kind: pipeTable, Class: ClassGlobal, Arg: ArgNone, Repeat: RepeatRefuse, shapes: anyShape,
		Description: "Render as table"},
	{Name: "text", Kind: pipeText, Class: ClassGlobal, Arg: ArgNone, Repeat: RepeatRefuse, shapes: anyShape,
		Description: "Space-aligned columns"},
	{Name: "yaml", Kind: pipeYAML, Class: ClassGlobal, Arg: ArgNone, Repeat: RepeatRefuse, shapes: anyShape,
		Description: "YAML output"},
	{Name: "raw", Kind: pipeRaw, Class: ClassGlobal, Arg: ArgNone, Repeat: RepeatRefuse, shapes: anyShape,
		Description: "Dispatcher JSON, unformatted"},
	{Name: "no-more", Kind: pipeNoMore, Class: ClassGlobal, Arg: ArgNone, Repeat: RepeatIdempotent, shapes: anyShape,
		Description: "Disable paging"},
	{Name: "log", Kind: pipeLog, Class: ClassStream, Arg: ArgNone, Repeat: RepeatIdempotent, shapes: anyShape,
		Description: "Append each update instead of replacing it, where the command keeps answering"},
	{Name: "save", Kind: pipeSave, Class: ClassGlobal, Arg: ArgPath, Repeat: RepeatCompose, shapes: anyShape,
		Description: "Write the answer to a file"},
	{Name: "match", Kind: pipeMatch, Class: ClassData, Arg: ArgText, Repeat: RepeatCompose, shapes: rowShapes,
		Description: "Keep the rows holding this text"},
	{Name: "count", Kind: pipeCount, Class: ClassData, Arg: ArgNone, Repeat: RepeatRefuse, shapes: rowShapes,
		Description: "Count the rows"},
	{Name: "first", Kind: pipeFirst, Class: ClassData, Arg: ArgCount, Repeat: RepeatCompose, shapes: rowShapes,
		Description: "Take first N rows"},
	{Name: "last", Kind: pipeLast, Class: ClassData, Arg: ArgCount, Repeat: RepeatCompose, shapes: rowShapes,
		Description: "Take last N rows"},
	{Name: "display", Kind: pipeDisplay, Class: ClassData, Arg: ArgFields, Repeat: RepeatCompose, shapes: rowShapes,
		Description: "Answer with these fields, in this order"},
	{Name: "fill", Kind: pipeFill, Class: ClassData, Arg: ArgOptional, Repeat: RepeatRefuse, shapes: setOf(ShapeTab),
		Description: "Bring the remaining columns back, in the command's order or a named one"},
	{Name: "resolve", Kind: pipeResolve, Class: ClassData, Arg: ArgNone, Repeat: RepeatIdempotent, shapes: rowShapes,
		Description: "Reverse DNS for IP addresses", NeedsAddressField: true},
	{Name: "origin", Kind: pipeOrigin, Class: ClassData, Arg: ArgNone, Repeat: RepeatIdempotent, shapes: rowShapes,
		Description: "ASN and network for IP addresses", NeedsAddressField: true},
}

// PipeOperatorCatalog answers every operator and its contract, in published
// order. It is the source every surface that names an operator MUST read.
func PipeOperatorCatalog() []PipeOperator {
	out := make([]PipeOperator, len(pipeCatalog))
	copy(out, pipeCatalog)
	return out
}

// PipeOperatorNames answers the operator words, in published order.
func PipeOperatorNames() []string {
	out := make([]string, 0, len(pipeCatalog))
	for _, op := range pipeCatalog {
		out = append(out, op.Name)
	}
	return out
}

// LookupPipeOperator answers the operator of that name.
func LookupPipeOperator(name string) (PipeOperator, bool) {
	for _, op := range pipeCatalog {
		if op.Name == name {
			return op, true
		}
	}
	return PipeOperator{}, false
}

// knownPipeOps maps operator names to their pipeKind, derived from the catalog
// so the parser and every published list cannot disagree.
var knownPipeOps = func() map[string]pipeKind {
	ops := make(map[string]pipeKind, len(pipeCatalog))
	for _, op := range pipeCatalog {
		ops[op.Name] = op.Kind
	}
	return ops
}()

// RenderOperatorReference answers the published operator table, in Markdown.
//
// It lives beside the catalog so the page and the parser cannot disagree: the
// generator that writes docs/features/pipe-operators.generated.md and the gate
// that checks it are the same call. Before this, the operator set was
// hand-copied into five surfaces and no two agreed.
func RenderOperatorReference() string {
	var b strings.Builder
	b.WriteString("<!-- GENERATED by `make ze-docs-pipe-operators-update` from\n")
	b.WriteString("     internal/component/command/pipe_catalog.go. Do not edit by hand:\n")
	b.WriteString("     ze-doc-verify fails when this file and the catalog disagree. -->\n\n")
	b.WriteString("| Operator | Class | Argument | Repeated | What it does |\n")
	b.WriteString("|----------|-------|----------|----------|--------------|\n")
	for _, op := range pipeCatalog {
		arg := op.ArgHint()
		if arg == "" {
			arg = "none"
		}
		repeat := "applies again, in order"
		switch op.Repeat {
		case RepeatIdempotent:
			repeat = "no effect"
		case RepeatRefuse:
			repeat = "refused"
		case RepeatCompose:
		}
		class := "acts on any answer"
		switch op.Class {
		case ClassData:
			class = "acts on rows"
		case ClassStream:
			class = "acts on a stream of updates"
		case ClassGlobal:
		}
		b.WriteString("| `")
		b.WriteString(op.Name)
		b.WriteString("` | ")
		b.WriteString(class)
		b.WriteString(" | ")
		b.WriteString(arg)
		b.WriteString(" | ")
		b.WriteString(repeat)
		b.WriteString(" | ")
		b.WriteString(op.Description)
		b.WriteString(" |\n")
	}
	b.WriteString("\nAn operator that acts on rows is refused BY NAME over an answer that has\n")
	b.WriteString("none, rather than answering something plausible. `resolve` and `origin` act\n")
	b.WriteString("on a field holding an IP address, and are refused unless the command\n")
	b.WriteString("declares one.\n")
	return b.String()
}
