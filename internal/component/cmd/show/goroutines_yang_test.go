// Design: goroutines.go -- the three mode words each select a handler
//
// Goal: prove the modes `show system goroutines` offers an operator are the
// modes the handler dispatches on, so a word cannot exist on one side alone.
// Method: read the enumeration at show/system/goroutines/mode out of the
// loaded model with configyang.EnumValues, which fails on a leaf that declares
// no enumeration, then drive the handler with each word and read the mode it
// echoes: a word the switch has no arm for falls to the summary and echoes
// that instead.

package show

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

const goroutinesModeLeaf = "show/system/goroutines/mode"

// TestGoroutineModesMatchTheModel holds the handler's mode words to the model
// in both directions.
func TestGoroutineModesMatchTheModel(t *testing.T) {
	model, err := configyang.EnumValues(goroutinesModeLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", goroutinesModeLeaf, err)
	}

	handled := []string{goroutineModeSummary, goroutineModeBlocked, goroutineModeFull}
	slices.Sort(handled)
	if !slices.Equal(model, handled) {
		t.Errorf("the modes disagree: the model at %s holds %v and goroutines.go handles %v. "+
			"A word only the model carries is answered as a summary, and a word only Go carries is one no operator can ask for",
			goroutinesModeLeaf, model, handled)
	}

	for _, mode := range model {
		resp, err := handleShowSystemGoroutines(nil, []string{mode})
		if err != nil {
			t.Fatalf("mode %q: %v", mode, err)
		}
		data, ok := resp.Data.(plugin.Map)
		if !ok {
			t.Fatalf("mode %q answered %T, not a plugin.Map", mode, resp.Data)
		}
		if data["mode"] != mode {
			t.Errorf("mode %q is offered at %s and the handler answered mode %v, so the word reaches no arm of its own",
				mode, goroutinesModeLeaf, data["mode"])
		}
	}
}
