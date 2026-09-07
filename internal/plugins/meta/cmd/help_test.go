// VALIDATES: the meta command's pure help helpers — pipeFilterHelp renders a
// filter list into name/description/takes-arg maps (nil for empty),
// pipeAliasHelp renders an alias list into name/description/expansion maps (nil
// for empty), and bgpEventTypes never surfaces the internal DirectionSent
// pseudo-event.
// PREVENTS: a malformed pipe-filter or pipe-alias help payload, or the
// sent/received direction marker leaking into the advertised BGP event types.

package cmd

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/events"
)

func TestPipeFilterHelp(t *testing.T) {
	if got := pipeFilterHelp(nil); got != nil {
		t.Errorf("empty filters = %v, want nil", got)
	}

	items := pipeFilterHelp([]command.PipeFilter{
		{Name: "grep", Description: "match lines", TakesArg: true},
		{Name: "count", Description: "count lines", TakesArg: false},
	})
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if items[0]["name"] != "grep" || items[0]["description"] != "match lines" || items[0]["takes-arg"] != true {
		t.Errorf("item0 = %v, want grep/match lines/true", items[0])
	}
	if items[1]["takes-arg"] != false {
		t.Errorf("item1 takes-arg = %v, want false", items[1]["takes-arg"])
	}
}

// VALIDATES: AC-11 at the renderer. Every alias a command answers to is
// reported with the name an operator types, the line beside it, and the chain
// it stands for.
// PREVENTS: an alias reported by name alone. An alias takes no argument, so the
// expansion is the whole of what the name does, and a reader who cannot see it
// has to run the command to find out.
func TestPipeAliasHelp(t *testing.T) {
	if got := pipeAliasHelp(nil); got != nil {
		t.Errorf("empty aliases = %v, want nil", got)
	}

	items := pipeAliasHelp([]command.Alias{
		{Name: "summary", Description: "The aggregate half", Expansion: "display router-id local-as"},
		{Name: "peers", Description: "The peer rows", Expansion: "display peers"},
	})
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if items[0]["name"] != "summary" || items[0]["description"] != "The aggregate half" {
		t.Errorf("item0 = %v, want summary/The aggregate half", items[0])
	}
	if items[0]["expansion"] != "display router-id local-as" {
		t.Errorf("item0 expansion = %v, want the chain the name stands for", items[0]["expansion"])
	}
	if items[1]["name"] != "peers" || items[1]["expansion"] != "display peers" {
		t.Errorf("item1 = %v, want peers/display peers", items[1])
	}
}

func TestBgpEventTypesExcludesDirectionSent(t *testing.T) {
	for _, tp := range bgpEventTypes() {
		if tp == events.DirectionSent {
			t.Errorf("bgpEventTypes() contains the DirectionSent marker %q", tp)
		}
		if tp == "" {
			t.Error("bgpEventTypes() contains an empty entry")
		}
	}
}

// VALIDATES: AC-6. `show command help "<name>"` answers what the command's
// ANSWER holds -- its shape, its column orders and its address fields -- beside
// the pipe filters and aliases it already answered. This handler runs inside
// the daemon, so it is the only reader that sees a declaration a plugin sent in
// its Stage 1 message.
// PREVENTS: an operator asking a running daemon what a command answers with and
// being told only which pipe names it accepts. The three registries hold the
// answer and nothing published it.
func TestCommandHelpReportsShapeColumnsAndAddressFields(t *testing.T) {
	// A path no package declares, so the declaration under test is the only
	// one the registries can resolve for it and no in-tree declaration is
	// disturbed (declarationRegistry.declare panics on a real conflict).
	const path = "show meta help fixture"

	command.RegisterShape([]string{path}, command.ShapeTab)
	command.RegisterColumns([]string{path}, command.ColumnOrder{"peer", "state", "uptime"})
	command.RegisterAddressFields([]string{path}, "peer")

	response := commandHelp(commandHelpText{Name: path, Description: "a fixture"})
	data, ok := response.Data.(plugin.Map)
	if !ok {
		t.Fatalf("the answer payload is %T, want plugin.Map", response.Data)
	}

	if got := data[keyAnswerShape]; got != "tab" {
		t.Errorf("answer-shape = %v, want tab", got)
	}
	orders, isOrders := data[keyColumnOrders].([][]string)
	if !isOrders {
		t.Fatalf("column-orders = %T, want [][]string", data[keyColumnOrders])
	}
	if len(orders) != 1 || len(orders[0]) != 3 ||
		orders[0][0] != "peer" || orders[0][1] != "state" || orders[0][2] != "uptime" {
		t.Errorf("column-orders = %v, want one order of peer/state/uptime", orders)
	}
	fields, isFields := data[keyAddressFields].([]string)
	if !isFields {
		t.Fatalf("address-fields = %T, want []string", data[keyAddressFields])
	}
	if len(fields) != 1 || fields[0] != "peer" {
		t.Errorf("address-fields = %v, want [peer]", fields)
	}
}

// VALIDATES: a command that declares nothing carries none of the three keys, so
// a reader can tell "declares one document" from "declares nothing". ShapeDoc
// is the zero AnswerShape, so publishing the key unconditionally would report
// every undeclared command as declaring a document.
// PREVENTS: a catalog that says every command answers one document.
func TestCommandHelpOmitsAnUndeclaredAnswer(t *testing.T) {
	const path = "show meta help undeclared"

	data, ok := commandHelp(commandHelpText{Name: path}).Data.(plugin.Map)
	if !ok {
		t.Fatal("the answer payload is not a plugin.Map")
	}
	for _, key := range []string{keyAnswerShape, keyColumnOrders, keyAddressFields} {
		if value, found := data[key]; found {
			t.Errorf("an undeclared command carries %q = %v", key, value)
		}
	}
}
