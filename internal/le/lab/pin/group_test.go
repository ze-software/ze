package pin

import (
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// VALIDATES: `le lab pin` is filed under a group that admits a writing
// action, because its table declares one: `update` rewrites the pins in the
// lab Dockerfiles and Go sources.
// PREVENTS: the 2026-10-24 defect, where the area sat under
// leroot.GroupReport ("read the tree, gate nothing") while its bump rewrote a
// committed pin, so a reader trusting the group ran a writer.
//
// Method: the writing action is read from the published rows, not named, so
// the test follows the table; it fails loud if no row writes, because the
// group judgement would then have no subject. TestReportAreasWriteNothing
// (internal/le/group_test.go) states the same rule over every area through
// the help text; this one reads this area's table and registry entry directly.
//
// MUTATION: (register.go, `leroot.Register(area, leroot.GroupGenerate, Answer,`, `leroot.Register(area, leroot.GroupReport, Answer,`): the area is filed as a report while update writes, and this test goes red.
func TestLabPinReportDoesNotWrite(t *testing.T) {
	group, registered := leroot.GroupOf(area)
	if !registered {
		t.Fatalf("le %s is not registered in any group", area)
	}
	writer := ""
	for _, row := range Actions().Actions {
		if row.Writes {
			writer = row.Verb
			break
		}
	}
	if writer == "" {
		t.Fatalf("le %s declares no writing action, so its group has nothing to judge", area)
	}
	if group == leroot.GroupReport {
		t.Errorf("le %s is filed under reports while its %s action writes", area, writer)
	}
}
