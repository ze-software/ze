package command

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// columnsPayload is one command answer with two record shapes in it: a list of
// peer rows, and the record that carries them. Every test below reads it, so a
// change that only works for one shape shows up here.
const columnsPayload = `{"peers":[{"address":"192.0.2.1","description":"transit","remote-as":65001,"state":"established","uptime":"1h0m0s"}]}`

// VALIDATES: `| display` takes every token after its name as its argument,
// which is the precedent `| match <pattern>` set.
// PREVENTS: a parse that keeps the first field name and drops the rest.
func TestParsePipeDisplayJoinsFields(t *testing.T) {
	command, ops := ParsePipe("show bgp | display address state uptime")
	if command != "show bgp" {
		t.Fatalf("command = %q, want %q", command, "show bgp")
	}
	if len(ops) != 1 || ops[0].kind != pipeDisplay {
		t.Fatalf("ops = %+v, want one display op", ops)
	}

	want := ColumnOrder{"address", "state", "uptime"}
	if got := parseDisplay(ops[0].arg); !slices.Equal(got, want) {
		t.Errorf("fields = %v, want %v", got, want)
	}
}

// VALIDATES: every row of the `| fill` table reads as its own way, and
// `reverse` reads as the flag it is rather than as a way.
// PREVENTS: a bare `| fill` being taken for `| fill alpha`, which would leave
// no way to ask for the command's own declared order.
func TestParsePipeFillWayAndReverse(t *testing.T) {
	cases := []struct {
		arg     string
		way     fillWay
		reverse bool
	}{
		{"", fillDefault, false},
		{"reverse", fillDefault, true},
		{"alpha", fillAlpha, false},
		{"alpha reverse", fillAlpha, true},
	}
	for _, c := range cases {
		way, reverse, ok := parseFill(c.arg)
		if !ok {
			t.Errorf("parseFill(%q) was refused", c.arg)
			continue
		}
		if way != c.way || reverse != c.reverse {
			t.Errorf("parseFill(%q) = (%v, %v), want (%v, %v)", c.arg, way, reverse, c.way, c.reverse)
		}
	}

	_, ops := ParsePipe("show test peers | fill alpha reverse")
	if len(ops) != 1 || ops[0].kind != pipeFill {
		t.Fatalf("ops = %+v, want one fill op", ops)
	}
	if ops[0].arg != "alpha reverse" {
		t.Errorf("arg = %q, want the whole tail", ops[0].arg)
	}
}

// VALIDATES: the `overall` way alone was removed, and the operator that carried
// it stayed (owner decision, 2026-08-19). `| fill`, `| fill alpha` and
// `reverse` on either are accepted; `overall` is refused by name.
// PREVENTS: two opposite mistakes. Removing `| fill` with `overall` takes a
// working operator with it, and a refusal that does not NAME the word reads to
// an operator as a typo rather than as a way that no longer exists.
//
// `overall` ordered columns by the width they render at, which measures every
// cell of the whole answer before the first row can be written. That is the one
// property here that a streamed answer cannot carry.
func TestFillOverallWayWasRemoved(t *testing.T) {
	for _, accepted := range []string{
		"show test peers | fill",
		"show test peers | fill alpha",
		"show test peers | fill reverse",
		"show test peers | fill alpha reverse",
	} {
		_, ops := ParsePipe(accepted)
		if len(ops) != 1 || ops[0].kind != pipeFill {
			t.Fatalf("%q parsed as %+v, want one fill op", accepted, ops)
		}
		if msg := ValidatePipes(ops); msg != "" {
			t.Errorf("%q was refused: %s", accepted, msg)
		}
	}

	for _, refused := range []string{
		"show test peers | fill overall",
		"show test peers | fill overall reverse",
	} {
		_, ops := ParsePipe(refused)
		if len(ops) != 1 || ops[0].kind != pipeFill {
			t.Fatalf("%q parsed as %+v, want one fill op: the operator itself stays", refused, ops)
		}
		msg := ValidatePipes(ops)
		if msg == "" {
			t.Fatalf("%q was accepted, want the overall way refused", refused)
		}
		if !strings.Contains(msg, "overall") {
			t.Errorf("%q gave %q, want the refusal to name overall", refused, msg)
		}
		if !strings.Contains(msg, fillWayAlpha) {
			t.Errorf("%q gave %q, want the refusal to name the way that is left", refused, msg)
		}
	}

	// The completer stops offering the word, so nobody is taught it either.
	for _, suggestion := range pipeSubArgs["fill"] {
		if suggestion.Text == "overall" {
			t.Errorf("the completer still offers overall: %+v", pipeSubArgs["fill"])
		}
	}
}

// VALIDATES: an argument neither operator can read is refused by name (AC-7).
// PREVENTS: silence. A word nobody reads is a request nobody answered, and an
// answer that ignores it looks the same as an answer that honored it.
func TestValidatePipesRejectsBadColumnArguments(t *testing.T) {
	cases := []struct {
		input string
		names []string
	}{
		{"show test peers | display", []string{"display", "field"}},
		{"show test peers | fill sideways", []string{"fill", "sideways", "alpha"}},
		{"show test peers | fill alpha sideways", []string{"fill", "sideways"}},
		{"show test peers | fill reverse reverse", []string{"fill", "reverse"}},
	}
	for _, c := range cases {
		_, ops := ParsePipe(c.input)
		msg := ValidatePipes(ops)
		if msg == "" {
			t.Errorf("%q was accepted, want a pipe error", c.input)
			continue
		}
		for _, name := range c.names {
			if !strings.Contains(msg, name) {
				t.Errorf("%q gave %q, want the message to name %q", c.input, msg, name)
			}
		}
	}

	// The good arguments stay good.
	for _, input := range []string{
		"show test peers | display state address",
		"show test peers | fill",
		"show test peers | fill reverse",
		"show test peers | fill alpha",
		"show test peers | fill alpha reverse",
		"show test peers | display state | fill alpha",
	} {
		_, ops := ParsePipe(input)
		if msg := ValidatePipes(ops); msg != "" {
			t.Errorf("%q was refused: %s", input, msg)
		}
	}
}

// VALIDATES: both operators apply to a command that registers pipe filters
// (AC-9, A-3).
// PREVENTS: the silent drop foldFilters performs on any kind its switch does
// not name. The switch has no default arm, so an unclassified operator reaches
// neither the server nor the renderer, and nothing reports it.
func TestColumnOpsSurviveFoldFiltersOnFilteredCommand(t *testing.T) {
	ResetColumnsForTest()
	ResetPipeFiltersForTest()
	t.Cleanup(ResetColumnsForTest)
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show test rib"}, PipeFilter{Name: "source", Description: "Select source", TakesArg: true})

	command, ops := ParsePipe("show test rib | display state address | fill alpha")
	command, ops, _ = foldFilters(command, ops)
	if command != "show test rib" {
		t.Errorf("command = %q, want it unchanged: neither operator is a server-side filter", command)
	}
	kinds := make([]pipeKind, len(ops))
	for i, op := range ops {
		kinds[i] = op.kind
	}
	for _, want := range []pipeKind{pipeDisplay, pipeFill} {
		if !slices.Contains(kinds, want) {
			t.Fatalf("foldFilters dropped kind %v: ops = %+v", want, ops)
		}
	}

	got := headerFields(t, renderThroughPipes(t, "show test rib | display state address | text", columnsPayload))
	want := []string{"state", "address"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want %v: the request did not reach the renderer", got, want)
	}
}

// VALIDATES: `| display` alone answers with the named fields, in the order they
// were named, and with nothing else (AC-2).
// PREVENTS: the operator degrading into ordering, which leaves the nineteen
// column table the operator was cutting down.
func TestDisplayAloneSelectsAndSequences(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	for _, input := range []string{"show test peers | display state address | text", "show test peers | display state address | table"} {
		got := headerFields(t, renderThroughPipes(t, input, columnsPayload))
		want := []string{"state", "address"}
		if !slices.Equal(got, want) {
			t.Errorf("%q header = %v, want %v", input, got, want)
		}
	}

	rendered := renderThroughPipes(t, "show test peers | display state address | text", columnsPayload)
	for _, dropped := range []string{"description", "remote-as", "uptime", "transit"} {
		if strings.Contains(rendered, dropped) {
			t.Errorf("a field the operator did not display survived: %q in %q", dropped, rendered)
		}
	}
}

// VALIDATES: `| fill alpha` after a `| display` brings the remaining fields
// back, by name, behind the displayed ones (AC-1).
// PREVENTS: the two operators being read as one. Each answers its own question,
// and neither changes what the other means.
func TestDisplayThenFillAlpha(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	got := headerFields(t, renderThroughPipes(t, "show test peers | display state address | fill alpha | text", columnsPayload))
	want := []string{"state", "address", "description", "remote-as", "uptime"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want the displayed fields then the rest by name %v", got, want)
	}

	got = headerFields(t, renderThroughPipes(t, "show test peers | display state address | fill alpha reverse | text", columnsPayload))
	want = []string{"state", "address", "uptime", "remote-as", "description"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want the rest in reverse name order %v", got, want)
	}
}

// VALIDATES: `| fill` with no `| display` orders every field, because nothing
// was displayed and every field is therefore a remaining field.
// PREVENTS: fill needing a display to do anything, which would make the two
// operators one operator in two words.
func TestFillAloneOrdersEveryField(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)
	RegisterColumns([]string{"show test peers"}, ColumnOrder{"state", "address"})

	got := headerFields(t, renderThroughPipes(t, "show test peers | fill alpha | text", columnsPayload))
	want := []string{"address", "description", "remote-as", "state", "uptime"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want every field by name %v", got, want)
	}
}

// VALIDATES: the operator's request replaces the command's own declaration,
// whole (AC-4).
// PREVENTS: the request being ranked against the declared orders by how many
// keys each one names, where a two-field request loses to a five-field
// declaration every time.
func TestDisplayOverridesRegisteredOrder(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)
	RegisterColumns([]string{"show test peers"},
		ColumnOrder{"address", "description", "remote-as", "state", "uptime"},
	)

	got := headerFields(t, renderThroughPipes(t, "show test peers | display uptime state | fill alpha | text", columnsPayload))
	want := []string{"uptime", "state", "address", "description", "remote-as"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want the operator's order then the rest %v", got, want)
	}

	// The declared order still governs when the operator asks for nothing.
	declared := headerFields(t, renderThroughPipes(t, "show test peers | text", columnsPayload))
	wantDeclared := []string{"address", "description", "remote-as", "state", "uptime"}
	if !slices.Equal(declared, wantDeclared) {
		t.Errorf("header = %v, want the declared order %v", declared, wantDeclared)
	}
}

// VALIDATES: a name the payload does not carry produces no column, with a
// `| fill` and without one (AC-3).
// PREVENTS: an empty placeholder column, and an answer with nothing in it
// because one name was wrong.
func TestDisplayUnknownFieldIsInert(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	rendered := renderThroughPipes(t, "show test peers | display state routes-sent | fill alpha | text", columnsPayload)
	got := headerFields(t, rendered)
	want := []string{"state", "address", "description", "remote-as", "uptime"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want %v", got, want)
	}
	if strings.Contains(rendered, "routes-sent") {
		t.Errorf("a name absent from the payload created a column: %q", rendered)
	}

	got = headerFields(t, renderThroughPipes(t, "show test peers | display state routes-sent | text", columnsPayload))
	want = []string{"state"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want only the name that matched %v", got, want)
	}
}

// VALIDATES: selection keeps the parent key column of a map-of-maps, so the
// rows stay identifiable (AC-10, R-1).
// PREVENTS: `| display state` on peers indexed by address answering with a
// column of states and nothing saying which peer each one is.
func TestDisplayKeepsParentKeyColumn(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	payload := `{"peers":{"192.0.2.1":{"group":"transit","state":"established","uptime":"1h0m0s"},"192.0.2.2":{"group":"peering","state":"idle","uptime":"0s"}}}`
	rendered := renderThroughPipes(t, "show test peers | display state | text", payload)

	if !strings.Contains(rendered, "192.0.2.1") || !strings.Contains(rendered, "192.0.2.2") {
		t.Errorf("selection dropped the key that identifies each row: %q", rendered)
	}
	if strings.Contains(rendered, "transit") || strings.Contains(rendered, "uptime") {
		t.Errorf("a field the operator did not display survived: %q", rendered)
	}
	if got := headerFields(t, rendered); !slices.Equal(got, []string{"state"}) {
		t.Errorf("header = %v, want [state]", got)
	}
}

// VALIDATES: selection reaches a programmatic format, and sequence does not
// (AC-5).
// PREVENTS: the two halves of `| display` being treated as one. Which fields to
// answer with is a question the operator asked out loud, so a program gets the
// answer. The sequence of JSON keys carries no meaning for a program.
func TestDisplaySelectionReachesJSON(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	for _, input := range []string{"show test peers | display state address | json", "show test peers | display state address | yaml"} {
		rendered := renderThroughPipes(t, input, columnsPayload)
		for _, kept := range []string{"state", "address"} {
			if !strings.Contains(rendered, kept) {
				t.Errorf("%q dropped a field the operator displayed: %q", input, rendered)
			}
		}
		for _, dropped := range []string{"description", "remote-as", "uptime"} {
			if strings.Contains(rendered, dropped) {
				t.Errorf("%q kept a field the operator did not display: %q", input, rendered)
			}
		}
	}

	// JSON keys stay alphabetical: the sequence half never leaves the renderer.
	rendered := renderThroughPipes(t, "show test peers | display state address | json", columnsPayload)
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rendered), &rows); err != nil {
		t.Fatalf("unmarshal %q: %v", rendered, err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if strings.Index(rendered, `"address"`) > strings.Index(rendered, `"state"`) {
		t.Errorf("JSON keys took the operator's sequence: %q", rendered)
	}

	// A `| fill` asks for every field back, so nothing is selected away.
	filled := renderThroughPipes(t, "show test peers | display state address | fill alpha | json", columnsPayload)
	for _, kept := range []string{"description", "remote-as", "uptime"} {
		if !strings.Contains(filled, kept) {
			t.Errorf("| fill did not bring %q back to the JSON: %q", kept, filled)
		}
	}
}

// VALIDATES: a chain with neither operator renders exactly as it did before
// they existed, in every format (AC-6).
// PREVENTS: the request leaking into the built-in path, which would show up
// here as a difference from the style the renderer was called with before.
func TestColumnOpsAbsentLeavesOutputUnchanged(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)
	RegisterColumns([]string{"show test peers"}, ColumnOrder{"state", "address"})

	columns := ColumnsForCommand("show test peers")
	cases := []struct {
		input string
		want  string
	}{
		{"show test peers | text", applyTableStyled(columnsPayload, tableStyle{plain: true, orders: columns})},
		{"show test peers | table", applyTableStyled(columnsPayload, tableStyle{orders: columns})},
		{"show test peers | json", ApplyJSON(columnsPayload, jsonPretty)},
		{"show test peers | yaml", applyYAML(columnsPayload)},
		{"show test peers | raw", columnsPayload},
	}
	for _, c := range cases {
		if got := renderThroughPipes(t, c.input, columnsPayload); got != c.want {
			t.Errorf("%q rendered %q, want %q", c.input, got, c.want)
		}
	}
}

// VALIDATES: a `| match` after a selection matches the text that is left (R-2).
// PREVENTS: the interaction being a surprise rather than a decision. The value
// genuinely is not in the answer any more, so the match genuinely fails.
func TestDisplayThenMatchOnDroppedColumn(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	kept := renderThroughPipes(t, "show test peers | display state description | text | match transit", columnsPayload)
	if !strings.Contains(kept, "transit") {
		t.Fatalf("match found nothing while its column was displayed: %q", kept)
	}

	dropped := renderThroughPipes(t, "show test peers | display state | text | match transit", columnsPayload)
	if strings.Contains(dropped, "transit") {
		t.Errorf("match found a value from a column the operator did not display: %q", dropped)
	}
}

// VALIDATES: a nested record follows the rule the table around it follows. It
// is selected when it carries a field the operator displayed, and it is left
// whole when it carries none.
// PREVENTS: two failures at once. A nested record whose fields the operator
// never named would otherwise render as a box with no rows, and `| text` and
// `| json` would otherwise disagree about which fields a sub-table carries.
func TestDisplaySelectsANestedRecordTheSameWay(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	payload := `{"summary":{"local-as":65000,"peers":[{"address":"192.0.2.1","state":"established","uptime":"1h0m0s"}],"router-id":"192.0.2.254"}}`

	// The peer rows carry "state", so the operator's choice reaches them.
	rendered := renderThroughPipes(t, "show test summary | display peers state | text", payload)
	if !strings.Contains(rendered, "established") {
		t.Errorf("the displayed nested field was dropped: %q", rendered)
	}
	for _, dropped := range []string{"192.0.2.1", "1h0m0s", "65000", "192.0.2.254"} {
		if strings.Contains(rendered, dropped) {
			t.Errorf("%q survived a selection that did not name it: %q", dropped, rendered)
		}
	}

	asJSON := renderThroughPipes(t, "show test summary | display peers state | json", payload)
	if !strings.Contains(asJSON, "established") {
		t.Errorf("| json dropped the displayed nested field: %q", asJSON)
	}
	for _, dropped := range []string{"192.0.2.1", "1h0m0s", "65000", "192.0.2.254"} {
		if strings.Contains(asJSON, dropped) {
			t.Errorf("| json kept %q while | text dropped it: %q", dropped, asJSON)
		}
	}

	// A peer row carries no field called "peers", so it keeps every field.
	whole := renderThroughPipes(t, "show test summary | display peers | text", payload)
	for _, kept := range []string{"192.0.2.1", "established", "1h0m0s"} {
		if !strings.Contains(whole, kept) {
			t.Errorf("a record that names no displayed field lost %q: %q", kept, whole)
		}
	}
	if strings.Contains(whole, "65000") {
		t.Errorf("the outer record kept a field the operator did not display: %q", whole)
	}
}

// VALIDATES: the two operators answer the same whichever side of the format
// operator they are written on.
// PREVENTS: `| table | display state address` rendering the whole table, which
// is what happens when only the payload transform carries the selection and the
// renderer has already run by the time it does.
func TestColumnOpsAfterTheFormatOperator(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	for _, format := range []string{"text", "table"} {
		before := renderThroughPipes(t, "show test peers | display state address | "+format, columnsPayload)
		after := renderThroughPipes(t, "show test peers | "+format+" | display state address", columnsPayload)
		if before != after {
			t.Errorf("| %s answered differently by where the display was written:\nbefore = %q\nafter  = %q", format, before, after)
		}
		if got := headerFields(t, after); !slices.Equal(got, []string{"state", "address"}) {
			t.Errorf("| %s | display header = %v, want [state address]", format, got)
		}
	}
}

// VALIDATES: a bare `| fill` orders the remaining fields by the order the
// COMMAND declared, while the names `| display` gave still order the fields it
// named. Two sequences, two disjoint key sets, one call.
// PREVENTS: the easy bug in that call. The displayed names replace the declared
// order for the fields they name. It is one line's difference for them to
// suppress it for the fields they do not. That would silently turn a bare
// `| fill` into `| fill alpha`.
func TestFillDefaultUsesTheDeclaredOrderForTheRemainder(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)
	// Declared, alphabetical and reverse-declared are three different answers
	// for the remainder, so each assertion below can only pass one way.
	RegisterColumns([]string{"show test peers"},
		ColumnOrder{"state", "address", "description", "uptime", "remote-as"},
	)

	got := headerFields(t, renderThroughPipes(t, "show test peers | display state address | fill | text", columnsPayload))
	want := []string{"state", "address", "description", "uptime", "remote-as"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want the remainder in the declared order %v", got, want)
	}

	got = headerFields(t, renderThroughPipes(t, "show test peers | display state address | fill alpha | text", columnsPayload))
	want = []string{"state", "address", "description", "remote-as", "uptime"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want alpha to force name order over the declaration %v", got, want)
	}

	got = headerFields(t, renderThroughPipes(t, "show test peers | display state address | fill reverse | text", columnsPayload))
	want = []string{"state", "address", "remote-as", "uptime", "description"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want the declared remainder reversed %v", got, want)
	}

	// A command that declared nothing leaves the remainder by name.
	ResetColumnsForTest()
	got = headerFields(t, renderThroughPipes(t, "show test peers | display state address | fill | text", columnsPayload))
	want = []string{"state", "address", "description", "remote-as", "uptime"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want name order where no order is declared %v", got, want)
	}
}

// VALIDATES: a bare `| fill` with no `| display` answers exactly as no operator
// at all does, because every field is then a remaining field and the command's
// own declaration is what orders them.
// PREVENTS: the default way being read from somewhere other than the registry,
// which would make `| fill` change an answer it was asked to leave alone.
func TestFillDefaultAloneMatchesTheBuiltInOrder(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)
	RegisterColumns([]string{"show test peers"},
		ColumnOrder{"state", "address", "description", "uptime", "remote-as"},
	)

	plain := renderThroughPipes(t, "show test peers | text", columnsPayload)
	filled := renderThroughPipes(t, "show test peers | fill | text", columnsPayload)
	if plain != filled {
		t.Errorf("| fill changed the built-in answer:\nplain  = %q\nfilled = %q", plain, filled)
	}

	got := headerFields(t, renderThroughPipes(t, "show test peers | fill reverse | text", columnsPayload))
	want := []string{"remote-as", "uptime", "description", "address", "state"}
	if !slices.Equal(got, want) {
		t.Errorf("header = %v, want the declared order reversed %v", got, want)
	}
}

// TestDisplaySelectsPerRecord checks R-10 of
// spec-streaming-answer-protocol: `| display` selects one record at a
// time, on both paths, so the operator credited with the streaming win produces
// it.
//
// Two halves. On the record path a generator counts what it produced and the
// consumer counts what it read, and the two must stay level: an implementation
// that decoded the answer would run the generator to its end before answering
// the first record. On the string path the streamed sequence must agree byte
// for byte with what a whole decode answers, because the two are renderings of
// one payload.
//
// VALIDATES: R-10, selection happens on one record and needs no other.
// PREVENTS: `| display` decoding the whole payload, which answers the same
// fields at the cost this protocol exists to remove.
func TestDisplaySelectsPerRecord(t *testing.T) {
	const rows = 5

	produced, consumed := 0, 0
	records := func(yield func(rpc.Record) bool) {
		for i := range rows {
			var b textbuf.Buffer
			b.Str(`{"address":"192.0.2.`).Int(int64(i)).Str(`","state":"established","uptime":"1h0m0s"}`)
			produced++
			if !yield(rpc.Record{Item: json.RawMessage(b.String())}) {
				return
			}
		}
	}

	for record := range ApplyPipesRecords("show test peers | display state", records) {
		consumed++
		if produced != consumed {
			t.Errorf("the chain had produced %d records when it answered record %d: selection read more than the record in hand", produced, consumed)
		}
		if want := `{"state":"established"}`; string(record.Item) != want {
			t.Errorf("record %d = %q, want %q", consumed, record.Item, want)
		}
	}
	if consumed != rows {
		t.Errorf("the chain answered %d records, want %d", consumed, rows)
	}

	// The string path, over the two shapes an answer of records takes.
	request := columnRequest{display: ColumnOrder{"state"}}
	keep := keepFields(request.display)
	for _, payload := range []string{
		`[{"address":"192.0.2.1","state":"established"},{"address":"192.0.2.2","state":"idle"}]`,
		`{"peers":[{"address":"192.0.2.1","state":"established"},{"address":"192.0.2.2","state":"idle"}]}`,
		`{"peers":[]}`,
		// A value and a key that json.Marshal escapes. The streamed sequence
		// writes the key itself, so it has its own chance to escape differently.
		`{"peers<&>":[{"state":"a&b<c","address":"192.0.2.1"}]}`,
	} {
		streamed, ok := selectSequence(payload, keep)
		if !ok {
			t.Errorf("%q was not read as a sequence, so the whole payload was decoded to select it", payload)
			continue
		}
		if got := applyDisplaySelect(payload, request); got != streamed {
			t.Errorf("| display answered %q, and the sequence it streamed is %q", got, streamed)
		}
		if want := selectWholePayload(t, payload, keep); streamed != want {
			t.Errorf("the streamed selection is %q, and a whole decode answers %q", streamed, want)
		}
	}
}

// selectWholePayload is the control for the record-at-a-time selection: it
// decodes the payload whole, which is what applyDisplaySelect did before it
// streamed, and answers what that produces.
func selectWholePayload(t *testing.T, payload string, keep map[string]struct{}) string {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()
	var data any
	if err := decoder.Decode(&data); err != nil {
		t.Fatalf("decode %q: %v", payload, err)
	}
	out, err := json.Marshal(selectFields(data, keep))
	if err != nil {
		t.Fatalf("marshal the selection of %q: %v", payload, err)
	}
	return string(out)
}

// TestDisplayNarrowsRatherThanReplaces covers the measured defect:
// `display state | display address` answered {address}, a field the FIRST
// display had already dropped. A second display widened the answer.
func TestDisplayNarrowsRatherThanReplaces(t *testing.T) {
	tests := []struct {
		name  string
		chain []pipeOp
		want  ColumnOrder
	}{
		{
			name:  "one request stands",
			chain: []pipeOp{{kind: pipeDisplay, arg: "address state"}},
			want:  ColumnOrder{"address", "state"},
		},
		{
			name: "the second narrows the first",
			chain: []pipeOp{
				{kind: pipeDisplay, arg: "address state uptime"},
				{kind: pipeDisplay, arg: "state address"},
			},
			// The order is the one the LAST request asked for.
			want: ColumnOrder{"state", "address"},
		},
		{
			name: "a field the first dropped does not come back",
			chain: []pipeOp{
				{kind: pipeDisplay, arg: "state"},
				{kind: pipeDisplay, arg: "state address"},
			},
			want: ColumnOrder{"state"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := columnsInChain(tt.chain).display
			if len(got) != len(tt.want) {
				t.Fatalf("display = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("display = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestDisplayWithNothingInCommonIsRefused is the case narrowing creates: two
// requests naming disjoint fields would answer rows with no fields at all, so
// the chain is refused before it runs and the operator hears why.
func TestDisplayWithNothingInCommonIsRefused(t *testing.T) {
	msg := ValidatePipes([]pipeOp{
		{kind: pipeDisplay, arg: "state"},
		{kind: pipeDisplay, arg: "address"},
	})
	if msg == "" {
		t.Fatal("display state | display address was accepted; it selects no field")
	}
	if !strings.HasPrefix(msg, "display ") {
		t.Errorf("refusal %q does not name the operator", msg)
	}
}
