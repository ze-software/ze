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
