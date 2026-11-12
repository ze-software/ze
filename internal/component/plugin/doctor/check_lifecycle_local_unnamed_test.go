package doctor

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/server"
)

// TestFatalStopRowUnnamedBootRefusalFixRuns proves a boot refusal that
// records no plugin (settleLastStop, any refusal but a missing dependency)
// gets a fix that is a working command: ze's log up to the time, with no grep,
// and a message that names no placeholder plugin. A named entry keeps the
// grep of its name.
//
// PREVENTS: the fix `... | grep an unnamed plugin`, which grep reads as the
// pattern "an" and two file names.
func TestFatalStopRowUnnamedBootRefusalFixRuns(t *testing.T) {
	at := time.Date(2026, 11, 11, 9, 30, 0, 0, time.UTC)
	row := fatalStopRow(&server.LastStopEntry{Reason: server.LastStopBootRefused, Detail: "config refused: no such peer", Time: at})
	if strings.Contains(row.Fix, "grep") {
		t.Errorf("unnamed boot refusal fix %q greps for a name nobody recorded", row.Fix)
	}
	if !strings.HasPrefix(row.Fix, "journalctl -u ze --until 2026-11-11T09:30:00Z,") {
		t.Errorf("unnamed boot refusal fix %q does not read ze's log up to the refusal", row.Fix)
	}
	if strings.Contains(row.Message, "unnamed") || strings.Contains(row.Message, "()") {
		t.Errorf("unnamed boot refusal row %q names a placeholder plugin", row.Message)
	}
	if !strings.Contains(row.Message, "config refused: no such peer") {
		t.Errorf("unnamed boot refusal row %q lacks the refusal", row.Message)
	}

	fatal := fatalStopRow(&server.LastStopEntry{Reason: server.LastStopFatalPolicy, Detail: "its failure stops ze", Time: at})
	if strings.Contains(fatal.Fix, "grep") {
		t.Errorf("unnamed fatal stop fix %q greps for a name nobody recorded", fatal.Fix)
	}

	named := fatalStopRow(&server.LastStopEntry{Reason: server.LastStopBootRefused, Plugin: "boot", Detail: "stage 1 refused", Time: at})
	if !strings.Contains(named.Fix, "| grep boot,") {
		t.Errorf("named boot refusal fix %q does not grep the plugin's name", named.Fix)
	}
	if !strings.Contains(named.Message, "(boot)") {
		t.Errorf("named boot refusal row %q does not name the plugin", named.Message)
	}
}
