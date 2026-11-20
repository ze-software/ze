// VALIDATES: the meta command's help answers -- the one pipe-operations list
// commandHelp answers, what a command's answer holds -- and that bgpEventTypes
// never surfaces the internal DirectionSent pseudo-event.
// PREVENTS: help holding its own pipe list, or the sent/received direction
// marker leaking into the advertised BGP event types.

package cmd

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/events"
)

// VALIDATES: AC-19 on `show command help`. The answer carries one
// `pipe-operations` list, the one command.PipeOperationsForCommand derives,
// with an override, an own operation and an alias each in it, and the
// `pipe-filters` and `pipe-aliases` keys are gone.
// PREVENTS: help holding a list of its own beside the completer's and the
// catalog's.
func TestCommandHelpPipeOperations(t *testing.T) {
	// A path no package declares, so no in-tree declaration is disturbed.
	const path = "show meta help operations"
	command.RegisterShape([]string{path}, command.ShapeTab)
	command.RegisterPipeFilters([]string{path},
		command.PipeFilter{Name: "count", Description: "Count matching routes", Argument: command.ArgNone},
		command.PipeFilter{Name: "histogram", Description: "Routes per prefix length", Argument: command.ArgText})
	command.RegisterAliases([]string{path},
		command.Alias{Name: "brief", Description: "The short view", Expansion: "display state"})

	response := commandHelp(commandHelpText{Name: path, ShortHelp: "a fixture"})
	data, ok := response.Data.(plugin.Map)
	if !ok {
		t.Fatalf("the answer payload is %T, want plugin.Map", response.Data)
	}
	for _, gone := range []string{"pipe-filters", "pipe-aliases"} {
		if _, present := data[gone]; present {
			t.Errorf("the answer still carries %s", gone)
		}
	}
	operations, isList := data[keyPipeOperations].([]command.PipeOperationEntry)
	if !isList {
		t.Fatalf("pipe-operations = %T, want []command.PipeOperationEntry", data[keyPipeOperations])
	}
	if want := command.PipeOperationsForCommand(path); !slices.Equal(operations, want) {
		t.Errorf("pipe-operations = %+v, want the introspection list %+v", operations, want)
	}
	sources := make(map[string]string, len(operations))
	for _, operation := range operations {
		sources[operation.Name] = operation.Source
	}
	for name, source := range map[string]string{
		"count": command.PipeSourceOverride, "histogram": command.PipeSourceCommand,
		"brief": command.PipeSourceAlias, "match": command.PipeSourceGeneric,
	} {
		if sources[name] != source {
			t.Errorf("%s has source %q, want %q", name, sources[name], source)
		}
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

	response := commandHelp(commandHelpText{Name: path, ShortHelp: "a fixture"})
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
