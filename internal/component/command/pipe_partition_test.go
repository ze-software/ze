package command

import (
	"slices"
	"strings"
	"testing"
)

// partitionOf partitions input the way every route does: the chain parsed,
// aliases expanded and address fields bound, then split for the command.
func partitionOf(input string) pipePartition {
	return partitionPipeChain(parsePipeChain(input))
}

func residualKinds(ops []pipeOp) []pipeKind {
	kinds := make([]pipeKind, 0, len(ops))
	for _, op := range ops {
		kinds = append(kinds, op.kind)
	}
	return kinds
}

func samePipeOperations(a, b []PipeOperation) bool {
	return slices.EqualFunc(a, b, func(x, y PipeOperation) bool {
		return x.Name == y.Name && slices.Equal(x.Args, y.Args)
	})
}

// TestPartitionTakesDeclaredOnlyAsAPrefix asks whether a declared operation is
// taken from the chain only while no residual operation precedes it (AC-25).
// Method: one command declares an own name (peer) and an override (count);
// each line is partitioned and its declared list and residual chain read.
// VALIDATES: spec-pipe-chain-honored-on-every-command-surface AC-25, D-2.
// PREVENTS: an operation jumping ahead of one the operator typed before it, so
// `| resolve | count` counted before it resolved.
func TestPartitionTakesDeclaredOnlyAsAPrefix(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show prefix rows"},
		PipeFilter{Name: "peer", Description: "rows of one peer", Argument: ArgText},
		PipeFilter{Name: "count", Description: "count rows at the source"},
	)

	t.Run("an override after a residual operation runs as the generic", func(t *testing.T) {
		part := partitionOf("show prefix rows | resolve | count")
		if len(part.declared) != 0 {
			t.Errorf("declared = %+v, want none: count follows resolve", part.declared)
		}
		if got := residualKinds(part.residual); !slices.Equal(got, []pipeKind{pipeResolve, pipeCount}) {
			t.Errorf("residual = %v, want [resolve count] in the order typed", got)
		}
	})

	t.Run("an own name after a residual operation is refused with the reordered line", func(t *testing.T) {
		part := partitionOf("show prefix rows | table | peer 10.0.0.1")
		if len(part.declared) != 0 {
			t.Errorf("declared = %+v, want none", part.declared)
		}
		if len(part.residual) != 1 || part.residual[0].kind != pipeInvalid {
			t.Fatalf("residual = %+v, want one refusal", part.residual)
		}
		msg := part.residual[0].arg
		for _, want := range []string{"peer", "before table", "use: show prefix rows | peer 10.0.0.1 | table"} {
			if !strings.Contains(msg, want) {
				t.Errorf("refusal %q does not hold %q", msg, want)
			}
		}
	})

	t.Run("operations before the first residual one are taken in order", func(t *testing.T) {
		part := partitionOf("show prefix rows | peer 10.0.0.1 | count | table")
		want := []PipeOperation{{Name: "peer", Args: []string{"10.0.0.1"}}, {Name: "count"}}
		if !samePipeOperations(part.declared, want) {
			t.Errorf("declared = %+v, want %+v", part.declared, want)
		}
		if got := residualKinds(part.residual); !slices.Equal(got, []pipeKind{pipeTable}) {
			t.Errorf("residual = %v, want [table]", got)
		}
		if part.command != "show prefix rows" {
			t.Errorf("command = %q, want the command part unchanged", part.command)
		}
	})
}

// TestPartitionTakesParsedOverridesAndAliases asks whether the partition
// takes an override from text the operator typed: an alias expanding to a
// declared name, and a parsed override carrying its argument words.
// Method: a command declares count and match; an alias `tally` expands to
// count. Each line is parsed from text through partitionOf, so the ops carry
// the words the parser decoded (declaredOperation's words branch), and the
// alias goes through expandAliases before the partition.
// VALIDATES: the two facts TestFoldPipeChain held: an alias of a declared
// operation, and a parsed generic-named operation, both reach the command.
// PREVENTS: an alias expansion or a parsed argument escaping the declared list.
func TestPartitionTakesParsedOverridesAndAliases(t *testing.T) {
	resetAliasTables(t)
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show parsed rows"},
		PipeFilter{Name: "count", Description: "count rows at the source"},
		PipeFilter{Name: "match", Description: "match rows at the source", Argument: ArgText},
	)
	RegisterAliases([]string{"show parsed rows"}, Alias{Name: "tally", Expansion: "count"})

	for _, tc := range []struct {
		input string
		want  []PipeOperation
	}{
		{input: "show parsed rows | tally", want: []PipeOperation{{Name: "count"}}},
		{input: "show parsed rows | match established", want: []PipeOperation{{Name: "match", Args: []string{"established"}}}},
	} {
		part := partitionOf(tc.input)
		if part.command != "show parsed rows" {
			t.Errorf("%s: command = %q, want the command part unchanged", tc.input, part.command)
		}
		if !samePipeOperations(part.declared, tc.want) {
			t.Errorf("%s: declared = %+v, want %+v", tc.input, part.declared, tc.want)
		}
		if len(part.residual) != 0 {
			t.Errorf("%s: residual = %v, want none", tc.input, residualKinds(part.residual))
		}
	}
}

// TestPartitionValidatesBeforeTaking asks whether the catalog argument check
// and count's document-follower check run over the whole chain before any
// declared operation leaves it.
// Method: a command declares count; a surplus count argument and a match after
// count are partitioned, and nothing may be declared.
// VALIDATES: the validation the fold ran first is kept by the partition.
// PREVENTS: a declared count hiding a follower it makes invalid.
func TestPartitionValidatesBeforeTaking(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show validated rows"},
		PipeFilter{Name: "count", Description: "count rows at the source"},
	)
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "show validated rows | count extra", want: "count does not accept an argument"},
		{input: "show validated rows | count | match x", want: "match cannot apply after count"},
	} {
		part := partitionOf(tc.input)
		if len(part.declared) != 0 {
			t.Errorf("%s: declared = %+v, want none before validation passed", tc.input, part.declared)
		}
		if len(part.residual) != 1 || part.residual[0].kind != pipeInvalid || !strings.Contains(part.residual[0].arg, tc.want) {
			t.Errorf("%s: residual = %+v, want one refusal holding %q", tc.input, part.residual, tc.want)
		}
	}
}

// TestDeclaredArgumentContractValidatedByPartition asks whether each argument
// kind of the closed vocabulary is enforced on a declared operation before its
// handler would run.
// Method: one operation per kind; a missing, surplus or ill-typed argument
// must leave a refusal and nothing declared, a valid one must be declared.
func TestDeclaredArgumentContractValidatedByPartition(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show kinds"},
		PipeFilter{Name: "flag", Description: "none", Argument: ArgNone},
		PipeFilter{Name: "maybe", Description: "optional", Argument: ArgOptional},
		PipeFilter{Name: "words", Description: "text", Argument: ArgText},
		PipeFilter{Name: "top", Description: "count", Argument: ArgCount},
		PipeFilter{Name: "columns", Description: "fields", Argument: ArgFields},
		PipeFilter{Name: "file", Description: "path", Argument: ArgPath},
	)
	for _, tc := range []struct {
		chain string
		ok    bool
	}{
		{chain: "flag", ok: true}, {chain: "flag x", ok: false},
		{chain: "maybe", ok: true}, {chain: "maybe x", ok: true}, {chain: "maybe x y", ok: false},
		{chain: "words a b", ok: true}, {chain: "words", ok: false},
		{chain: "top 3", ok: true}, {chain: "top", ok: false}, {chain: "top 0", ok: false},
		{chain: "top x", ok: false}, {chain: "top 1 2", ok: false},
		{chain: "columns a b", ok: true}, {chain: "columns", ok: false},
		{chain: "file /tmp/x", ok: true}, {chain: "file", ok: false}, {chain: "file a b", ok: false},
	} {
		part := partitionOf("show kinds | " + tc.chain)
		declared := len(part.declared) == 1 && len(part.residual) == 0
		refused := len(part.declared) == 0 && len(part.residual) == 1 && part.residual[0].kind == pipeInvalid
		if tc.ok && !declared {
			t.Errorf("%s: declared %+v residual %+v, want it declared", tc.chain, part.declared, part.residual)
		}
		if !tc.ok && !refused {
			t.Errorf("%s: declared %+v residual %+v, want it refused", tc.chain, part.declared, part.residual)
		}
	}
}

// recordPanic answers the panic text of register, or "" when it returned.
func recordPanic(register func()) (text string) {
	defer func() {
		if r := recover(); r != nil {
			text, _ = r.(string)
			if text == "" {
				text = "non-string panic"
			}
		}
	}()
	register()
	return ""
}

// TestOverridableComesFromTheCatalog asks whether the catalog alone decides
// which generic names a command may declare: every data operator, and no
// format, paging, `save` or `log` (D-5 as Thomas decided it on 2026-11-20,
// AC-30 in-tree).
// Method: every catalog operator is declared, with its own argument kind, on
// a fresh path; the registration must panic exactly for the operators the
// catalog does not let a command override, and the overridable set is pinned.
func TestOverridableComesFromTheCatalog(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	var overridable []string
	for _, op := range PipeOperatorCatalog() {
		text := recordPanic(func() {
			RegisterPipeFilters([]string{"show overridable " + op.Name}, PipeFilter{Name: op.Name, Description: "own", Argument: op.Arg})
		})
		if op.Overridable() {
			overridable = append(overridable, op.Name)
			if text != "" {
				t.Errorf("declaring overridable %s panicked: %s", op.Name, text)
			}
			continue
		}
		if !strings.Contains(text, "cannot override") {
			t.Errorf("declaring %s: panic %q, want a BUG naming that a command cannot override it", op.Name, text)
		}
	}
	slices.Sort(overridable)
	if want := []string{"count", "display", "fill", "first", "last", "match", "origin", "resolve"}; !slices.Equal(overridable, want) {
		t.Errorf("overridable generics = %v, want %v", overridable, want)
	}
}

// TestOverrideArgContractRefusedAtRegistration asks whether an override that
// states another argument contract than its generic, or a kind outside the
// vocabulary, is refused when it is recorded (AC-17).
func TestOverrideArgContractRefusedAtRegistration(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	text := recordPanic(func() {
		RegisterPipeFilters([]string{"show contract a"}, PipeFilter{Name: "first", Description: "first rows"})
	})
	if !strings.Contains(text, "overrides the generic first with argument none, but the generic takes count") {
		t.Errorf("first declared with no argument: panic %q", text)
	}
	if text := recordPanic(func() {
		RegisterPipeFilters([]string{"show contract b"}, PipeFilter{Name: "first", Description: "first rows", Argument: ArgCount})
	}); text != "" {
		t.Errorf("first declared with the generic's contract panicked: %s", text)
	}
	if text := recordPanic(func() {
		RegisterPipeFilters([]string{"show contract c"}, PipeFilter{Name: "odd", Description: "odd", Argument: PipeArgKind(99)})
	}); !strings.Contains(text, "outside none, optional, text, count, fields and path") {
		t.Errorf("an argument kind outside the vocabulary: panic %q", text)
	}
}

// TestOverrideAnswerShapeRefused asks whether the last declared override's
// done answer is held to the generic's shape (AC-17).
func TestOverrideAnswerShapeRefused(t *testing.T) {
	count := []PipeOperation{{Name: "count"}}
	firstTwo := []PipeOperation{{Name: "first", Args: []string{"2"}}}
	lastOne := []PipeOperation{{Name: "last", Args: []string{"1"}}}
	for _, tc := range []struct {
		name       string
		operations []PipeOperation
		document   string
		refused    bool
	}{
		{name: "count answering a count", operations: count, document: `{"count":5}`},
		{name: "count answering a route list", operations: count, document: `{"routes":[{"prefix":"10.0.0.0/8"}]}`, refused: true},
		{name: "count answering total", operations: count, document: `{"total":5}`, refused: true},
		{name: "count answering a fraction", operations: count, document: `{"count":1.5}`, refused: true},
		{name: "first 2 answering two rows", operations: firstTwo, document: `{"routes":[{"a":1},{"a":2}]}`},
		{name: "first 2 answering three rows", operations: firstTwo, document: `{"routes":[{"a":1},{"a":2},{"a":3}]}`, refused: true},
		{name: "last 1 answering no rows", operations: lastOne, document: `"a sentence"`, refused: true},
	} {
		err := CheckOverrideAnswer("show shape rows", tc.operations, AnswerDone, tc.document)
		if tc.refused && err == nil {
			t.Errorf("%s: accepted", tc.name)
		}
		if !tc.refused && err != nil {
			t.Errorf("%s: refused: %v", tc.name, err)
		}
		if tc.refused && err != nil && !strings.Contains(err.Error(), "show shape rows") {
			t.Errorf("%s: refusal %q does not name the command", tc.name, err)
		}
	}
}

// TestOverrideShapeCheckScope asks which answers the check leaves alone and
// which it judges without reading: an error answer, an override followed by
// another declared operation and no declared operation are not checked,
// whatever the outcome; an answer delivered as a generator of rows
// (AnswerStreamed) is rows, so it is refused for a `count` override, by name,
// and accepted for a rows override with no row counted and no document read.
// The events of a monitor are judged one by one on the stream route
// (TestStreamLineHoldsOverrideEventsToShape).
func TestOverrideShapeCheckScope(t *testing.T) {
	const wrong = `{"total":5}`
	count := []PipeOperation{{Name: "count"}}
	if err := CheckOverrideAnswer("show scope rows", count, AnswerFailed, wrong); err != nil {
		t.Errorf("an error answer was checked: %v", err)
	}
	followed := []PipeOperation{{Name: "count"}, {Name: "histogram"}}
	for _, outcome := range []AnswerOutcome{AnswerDone, AnswerStreamed} {
		if err := CheckOverrideAnswer("show scope rows", followed, outcome, wrong); err != nil {
			t.Errorf("outcome %d: an override followed by another declared operation was checked: %v", outcome, err)
		}
		if err := CheckOverrideAnswer("show scope rows", nil, outcome, wrong); err != nil {
			t.Errorf("outcome %d: an answer with no declared operation was checked: %v", outcome, err)
		}
	}

	// The document of a generated answer is never read: a `count` override
	// is refused even beside a document of the right shape, because what the
	// handler delivered is rows.
	err := CheckOverrideAnswer("show scope rows", count, AnswerStreamed, `{"count":5}`)
	if err == nil {
		t.Fatal("a count override that answered a generator of rows was accepted")
	}
	if !strings.Contains(err.Error(), "show scope rows") || !strings.Contains(err.Error(), "count") {
		t.Errorf("refusal %q does not name the command and the operator", err)
	}
	// A rows override keeps its shape by delivering rows, and `first N` is
	// not counted: three rows beside `first 1`, and a document that is not
	// JSON, are both left unread.
	for _, operation := range []PipeOperation{
		{Name: "first", Args: []string{"1"}},
		{Name: "last", Args: []string{"1"}},
		{Name: "match", Args: []string{"up"}},
		{Name: "display", Args: []string{"prefix"}},
	} {
		declared := []PipeOperation{operation}
		for _, document := range []string{`{"routes":[{"a":1},{"a":2},{"a":3}]}`, `not json`} {
			if err := CheckOverrideAnswer("show scope rows", declared, AnswerStreamed, document); err != nil {
				t.Errorf("%s answering a generator of rows beside %s: refused: %v", operation.Name, document, err)
			}
		}
	}
}

// TestEveryDataOperatorIsOverridable asks whether the catalog states an
// override shape for every operator that acts on a command's data, and for no
// operator that is the data format or the display of a stream (Thomas,
// 2026-11-20: a command may override an operation "as long as their data
// format is not changed").
// Method: every catalog entry's Overridable is compared with its class; a
// presentation operator that answered Overridable would let a command change
// the data format.
func TestEveryDataOperatorIsOverridable(t *testing.T) {
	for _, op := range PipeOperatorCatalog() {
		want := op.Class == ClassData
		if op.Overridable() != want {
			t.Errorf("%s (class %s): Overridable = %v, want %v", op.Name, op.Class, op.Overridable(), want)
		}
		if op.Presentation && op.Overridable() {
			t.Errorf("%s is presentation and overridable: a command could change the data format", op.Name)
		}
	}
}

// TestDataOperatorOverrideAnswerShapeChecked asks whether an override of a
// data operator beyond count, first and last is held to the rows envelope its
// generic answers: display, fill, resolve and origin each keep the command's
// rows, so a done answer with rows is accepted and one with none, or with two
// row sets to choose between, is refused naming the command and the operator.
func TestDataOperatorOverrideAnswerShapeChecked(t *testing.T) {
	for _, operation := range []PipeOperation{
		{Name: "display", Args: []string{"prefix"}},
		{Name: "fill"},
		{Name: "resolve"},
		{Name: "origin"},
	} {
		declared := []PipeOperation{operation}
		if err := CheckOverrideAnswer("show shape rows", declared, AnswerDone, `{"routes":[{"prefix":"10.0.0.0/8"}]}`); err != nil {
			t.Errorf("%s answering the command's rows: refused: %v", operation.Name, err)
		}
		if err := CheckOverrideAnswer("show shape rows", declared, AnswerDone, `{"routes":[]}`); err != nil {
			t.Errorf("%s answering no rows: refused: %v", operation.Name, err)
		}
		for _, wrong := range []string{`"a sentence"`, `{"a":[{"x":1}],"b":[{"y":2}]}`, `not json`} {
			err := CheckOverrideAnswer("show shape rows", declared, AnswerDone, wrong)
			if err == nil {
				t.Errorf("%s answering %s: accepted", operation.Name, wrong)
				continue
			}
			if !strings.Contains(err.Error(), "show shape rows") || !strings.Contains(err.Error(), operation.Name) {
				t.Errorf("%s answering %s: refusal %q does not name the command and the operator", operation.Name, wrong, err)
			}
		}
	}
}

// TestDisplayOverrideRecordedWithItsArgumentContract asks whether a command
// may declare `display` now that every data operator is overridable, held to
// the generic's argument kind, while a format of the same chain stays refused.
func TestDisplayOverrideRecordedWithItsArgumentContract(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	if text := recordPanic(func() {
		RegisterPipeFilters([]string{"show display own"}, PipeFilter{Name: "display", Description: "Select route fields", Argument: ArgFields})
	}); text != "" {
		t.Errorf("display declared with the generic's contract panicked: %s", text)
	}
	if got := PipeFiltersForCommand("show display own"); len(got) != 1 || got[0].Name != "display" {
		t.Errorf("show display own holds %+v, want its display override", got)
	}
	text := recordPanic(func() {
		RegisterPipeFilters([]string{"show display text"}, PipeFilter{Name: "display", Description: "Select route fields", Argument: ArgText})
	})
	if !strings.Contains(text, "overrides the generic display with argument text, but the generic takes fields") {
		t.Errorf("display declared with another argument kind: panic %q", text)
	}
	text = recordPanic(func() {
		RegisterPipeFilters([]string{"show display table"}, PipeFilter{Name: "table", Description: "own table"})
	})
	if !strings.Contains(text, "table") || !strings.Contains(text, "cannot override") {
		t.Errorf("table declared by a command: panic %q, want a BUG naming that a command cannot override it", text)
	}
}

// TestCatalogStatesPresentationOperators asks whether the presentation fact
// holds for the formats, no-more and save, and for no operator that selects
// or reduces.
func TestCatalogStatesPresentationOperators(t *testing.T) {
	want := []string{"json", "ndjson", "no-more", "raw", "save", "table", "text", "yaml"}
	var got []string
	for _, op := range PipeOperatorCatalog() {
		if op.Presentation {
			got = append(got, op.Name)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("presentation operators = %v, want %v", got, want)
	}
}

// TestHasFormatPipeReadsThePartition asks whether HasFormatPipe reads the
// residual chain: a format after a declared operation is found, and a
// declared operation's argument is never read as a format.
func TestHasFormatPipeReadsThePartition(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show format rows"},
		PipeFilter{Name: "peer", Description: "rows of one peer", Argument: ArgText},
	)
	if !HasFormatPipe("show format rows | peer a | table") {
		t.Error("a format after a declared operation was not found")
	}
	if HasFormatPipe("show format rows | peer table") {
		t.Error("a declared operation's argument was read as a format")
	}
}

// TestValidateArgsRefusesPipeOperationWord asks whether a word the command
// declares as a pipe operation is refused as an argument, with the pipe form
// of the line as its fix, before any argument is bound (AC-8, AC-34).
// Method: a command declares received, count and peer; the model route is
// handed the argument form with a definition still open, with its definition
// filled, and with none declared. A word that is no operation passes.
func TestValidateArgsRefusesPipeOperationWord(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	const path = "show operation words"
	RegisterPipeFilters([]string{path},
		PipeFilter{Name: "received", Description: "received rows"},
		PipeFilter{Name: "count", Description: "count rows at the source"},
		PipeFilter{Name: "peer", Description: "rows of one peer", Argument: ArgText},
	)
	open, err := NewStringArg("selector", nil, nil, ArgOptions{})
	if err != nil {
		t.Fatalf("build a definition: %v", err)
	}
	for _, tc := range []struct {
		name string
		defs []ArgDef
		args []string
		fix  string
	}{
		{name: "a definition still open", defs: []ArgDef{open}, args: []string{"received", "count"}, fix: path + " | received | count"},
		{name: "its definition filled", defs: []ArgDef{open}, args: []string{"selector", "x", "peer", "10.0.0.1", "count"}, fix: path + " selector x | peer 10.0.0.1 | count"},
		{name: "no definition declared", defs: nil, args: []string{"count"}, fix: path + " | count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withArgDefSource(t, func(string) ([]ArgDef, error) { return tc.defs, nil })
			_, err := ValidateModelArgs(path, tc.args, nil)
			if err == nil {
				t.Fatalf("%q was accepted as arguments", tc.args)
			}
			if !strings.Contains(err.Error(), "is a pipe operation of "+path) || !strings.Contains(err.Error(), tc.fix) {
				t.Errorf("refusal %q, want it to name the pipe operation and the fix %q", err, tc.fix)
			}
		})
	}
	t.Run("a word that is no operation is judged as today", func(t *testing.T) {
		withArgDefSource(t, func(string) ([]ArgDef, error) { return nil, nil })
		validated, err := ValidateModelArgs(path, []string{"alpha"}, nil)
		if err != nil {
			t.Fatalf("a plain word was refused: %v", err)
		}
		if !slices.Equal(validated.Tokens(), []string{"alpha"}) {
			t.Errorf("tokens = %q, want [alpha]", validated.Tokens())
		}
	})
}
