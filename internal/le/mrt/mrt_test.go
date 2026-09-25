// VALIDATES: `le mrt <subcommand>` reaches the handler internal/analyze
// registered for every subcommand, and a bare `le mrt` lists every one.
// PREVENTS: a subcommand that ze-analyze ran and le does not, and a listing
// that drifts from the registry it describes.

package mrt

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/analyze"
	"github.com/ze-software/ze/internal/core/subdispatch"
	"github.com/ze-software/ze/internal/le/le/root"
)

// TestMrtReachesEveryAnalyzeSubcommand drives `le mrt <subcommand> <arg>`
// through the le dispatcher for every subcommand the analyze registry holds.
// Each handler is replaced by a recorder first, because the real ones open
// files, sockets and BGP sessions. The recorder proves the argv arrived at the
// slot analyze.Dispatch resolves for that name, with the subcommand word
// removed. No other test in this package runs a real handler, so the replaced
// handlers are not restored.
func TestMrtReachesEveryAnalyzeSubcommand(t *testing.T) {
	targets := analyze.Targets()
	if len(targets) == 0 {
		t.Fatal("the analyze registry holds no subcommand")
	}

	for _, target := range targets {
		var got []string
		analyze.Register(target.Name, func(args []string) int {
			got = args
			return 7
		}, subdispatch.SubMeta{Desc: target.Desc})

		code := leroot.Dispatch("le", []string{name, target.Name, "probe-argument"})
		if code != 7 {
			t.Errorf("le mrt %s exited %d, want the handler's 7", target.Name, code)
		}
		if !slices.Equal(got, []string{"probe-argument"}) {
			t.Errorf("le mrt %s handed the handler %q, want [probe-argument]", target.Name, got)
		}
	}

	payload, code := Answer(nil)
	if code != 0 {
		t.Fatalf("bare le mrt exited %d, want 0", code)
	}
	listed, ok := payload.([]subdispatch.Target)
	if !ok {
		t.Fatalf("bare le mrt answered %T, want []subdispatch.Target", payload)
	}
	if !slices.Equal(listed, targets) {
		t.Errorf("bare le mrt listed %v, want the registry's %v", listed, targets)
	}
}
