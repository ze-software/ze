// Design: docs/guide/ddos-mitigation.md -- orphan FlowSpec doctor code metadata test.
// Related: doctor_codes.go -- registration of codeDoctorDDoSFlowspecOrphanAnnouncement.

package flowspec

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestOrphanCodeDescriptionNamesTheLiveRemoval proves that the description
// `ze show diagnostic doctor-ddos-flowspec-orphan-announcement` prints names
// the two steps in order (the withdrawal the report prints, then
// `ze data rm meta/ddos-flowspec/announcement`, which works while ze runs),
// why the order matters, and the refusal while a ddos-flowspec plugin runs
// with its restart.
//
// VALIDATES: AC-2, the code description half: it says the same about the
// removal as the reports, registered by the owning ddos-flowspec plugin.
// METHOD: the registered description of the code, read for each fact and for
// the order of the two steps.
// PREVENTS: a description that still says the removal needs ze stopped, while
// the reports say it works while ze runs.
func TestOrphanCodeDescriptionNamesTheLiveRemoval(t *testing.T) {
	meta := diagnostic.Lookup(codeDoctorDDoSFlowspecOrphanAnnouncement)
	if meta == nil {
		t.Fatalf("%s: ze show diagnostic has no entry", codeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	description := meta.Description
	withdrawAt := strings.Index(description, "send the withdrawal the report prints")
	removalAt := strings.Index(description, "then remove the record with ze data rm meta/ddos-flowspec/announcement, which works while ze runs")
	if withdrawAt < 0 {
		t.Errorf("description %q does not name the withdrawal the report prints", description)
	}
	if removalAt < 0 {
		t.Errorf("description %q does not name the removal that works while ze runs", description)
	}
	if removalAt < withdrawAt {
		t.Errorf("description %q names the removal before the withdrawal", description)
	}
	if !strings.Contains(description, "a record removed before its rule is withdrawn leaves the rule at the peers with no trace in the store and no report to find it") {
		t.Errorf("description %q does not say why the withdrawal comes first", description)
	}
	if !strings.Contains(description, "ze refuses the removal while a ddos-flowspec plugin runs") {
		t.Errorf("description %q does not say that a running ddos-flowspec plugin refuses the removal", description)
	}
	if !strings.Contains(description, "ze request plugin restart ddos-flowspec") {
		t.Errorf("description %q does not name the restart for a running plugin", description)
	}
	if strings.Contains(description, "while ze is stopped") {
		t.Errorf("description %q still says the removal needs ze stopped", description)
	}
}
