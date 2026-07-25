// VALIDATES: the meta command's pure help helpers — pipeFilterHelp renders a
// filter list into name/description/takes-arg maps (nil for empty), and
// bgpEventTypes never surfaces the internal DirectionSent pseudo-event.
// PREVENTS: a malformed pipe-filter help payload, or the sent/received direction
// marker leaking into the advertised BGP event types.

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
