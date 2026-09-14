// Design: capture_raw.go -- the three action words each select a handler
//
// Goal: prove the actions `show capture raw` offers an operator are the actions
// HandleCaptureRaw dispatches on, so a word cannot exist on one side alone.
// Method: read the enumeration at show/capture/raw/action out of the loaded
// model with configyang.EnumValues, which fails on a leaf that declares no
// enumeration, then drive the handler with each word and read the action it
// echoes: a word the switch has no arm for answers the usage line instead.

package cmd

import (
	"slices"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"

	// The blank import registers ze-diag-cmd with the loader, which declares
	// the leaf read below.
	_ "github.com/ze-software/ze/internal/plugins/diag/yang"
)

const captureRawActionLeaf = "show/capture/raw/action"

// TestCaptureRawActionsMatchTheModel holds the handler's action words to the
// model in both directions.
func TestCaptureRawActionsMatchTheModel(t *testing.T) {
	model, err := configyang.EnumValues(captureRawActionLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", captureRawActionLeaf, err)
	}

	handled := []string{captureRawActionStart, captureRawActionStop, captureRawActionDump}
	slices.Sort(handled)
	if !slices.Equal(model, handled) {
		t.Errorf("the actions disagree: the model at %s holds %v and capture_raw.go handles %v. "+
			"A word only the model carries answers the usage line, and a word only Go carries is one no operator can ask for",
			captureRawActionLeaf, model, handled)
	}

	for _, action := range model {
		resp, err := HandleCaptureRaw(nil, []string{action})
		if err != nil {
			t.Fatalf("action %q: %v", action, err)
		}
		if resp.Status != plugin.StatusDone {
			t.Errorf("%q is offered at %s and the handler refused it: %s", action, captureRawActionLeaf, resp.Error)
			continue
		}
		data, ok := resp.Data.(plugin.Map)
		if !ok {
			t.Fatalf("action %q answered %T, not a plugin.Map", action, resp.Data)
		}
		if data[keyAction] != action {
			t.Errorf("action %q echoed %v, so the word reaches no arm of its own", action, data[keyAction])
		}
	}

	resp, err := HandleCaptureRaw(nil, []string{"pause"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != plugin.StatusError {
		t.Errorf("pause is offered by no leaf and the handler answered %v rather than refusing it", resp.Status)
	}
}
