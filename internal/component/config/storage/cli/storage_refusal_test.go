package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
)

// TestShowDataListRefusalIsReturned proves `show data list` answers a store it
// cannot open with the printers' error line and exit code 2, in the
// LocalAnswer rather than written by the handler.
// Method: a --path naming nothing, compared with the line built from the same
// stat error.
func TestShowDataListRefusalIsReturned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.zefs")
	_, statErr := os.Stat(path)
	want := "error: open " + path + ": " + statErr.Error() + "\n"

	answer := dataList(commandtest.Args("--path", path))
	if answer.Payload != nil || answer.Code != 2 {
		t.Errorf("answer = payload %v, code %d; want nil, 2", answer.Payload, answer.Code)
	}
	if answer.Diagnostic != want {
		t.Errorf("diagnostic:\n got %q\nwant %q", answer.Diagnostic, want)
	}
}
