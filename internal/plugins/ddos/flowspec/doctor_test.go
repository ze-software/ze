// Design: docs/guide/ddos-mitigation.md -- a FlowSpec rule no responder owns
// Related: doctor.go (answerOrphanRuntimeCheck, checkOrphanRecord)

package flowspec

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// orphanTestMatch is the rule every doctor case records.
func orphanTestMatch() flowspecMatch {
	return buildMatch(ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(exitTestVictim), Proto: 17})
}

// orphanTestFix is the exact withdrawal the report must print for orphanTestMatch.
const orphanTestFix = `ze cli -c "send bgp * update text mp ipv4/flow del destination-ipv4 192.0.2.9/32 protocol =17"`

// TestOrphanCheckReportsARecordWithNoLiveResponder proves that the runtime check
// reports a recorded rule the running plugin does not hold as live, with the
// victim and the exact withdrawal as its fix.
//
// VALIDATES: AC-16, positive: a start-time withdrawal that failed leaves the
// record and no live announcement.
// METHOD: the callback's answer function over a fake holding the record, with
// no live announcement.
// PREVENTS: a rule left at the peers that nothing on the box reports.
func TestOrphanCheckReportsARecordWithNoLiveResponder(t *testing.T) {
	m := orphanTestMatch()
	disp := &fakeDispatcher{recorded: &m}

	diags, err := answerOrphanRuntimeCheck(orphanRuntimeCheck, disp, flowspecMatch{}, false)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if !strings.Contains(diags[0].Message, exitTestVictim) {
		t.Errorf("message %q does not name the victim %s", diags[0].Message, exitTestVictim)
	}
	if diags[0].Fix != orphanTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, orphanTestFix)
	}
}

// TestOrphanCheckIsQuietForTheLiveAnnouncement proves that the runtime check
// reports nothing for the record of the rule the live responder holds.
//
// VALIDATES: AC-16, negative: a running mitigation is recorded by design.
// METHOD: the callback's answer function with the record equal to the live match.
// PREVENTS: a doctor warning during every mitigation.
func TestOrphanCheckIsQuietForTheLiveAnnouncement(t *testing.T) {
	m := orphanTestMatch()
	disp := &fakeDispatcher{recorded: &m}

	diags, err := answerOrphanRuntimeCheck(orphanRuntimeCheck, disp, m, true)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("findings for the live announcement = %v, want none", diags)
	}
}

// TestOrphanRecordCheckReportsARecordWithTheSectionAbsent proves that the
// registered check reports a stored record while ddos flowspec is not
// configured, when no plugin runs to answer the runtime check.
//
// VALIDATES: AC-16 with the plugin's process ended (A-13 broken: the daemon asks
// no removed plugin for its declared checks).
// METHOD: a real store holding a record written by the producer's encoder, and
// a config tree with no ddos flowspec section.
// PREVENTS: a recorded rule invisible to `ze doctor` once its plugin is gone.
func TestOrphanRecordCheckReportsARecordWithTheSectionAbsent(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	record, err := encodeAnnouncementRecord(orphanTestMatch())
	if err != nil {
		t.Fatalf("encode the record: %v", err)
	}
	if err := store.WriteKey(announcementKey.Pattern, record); err != nil {
		t.Fatalf("write the record: %v", err)
	}

	diags := checkOrphanRecord(diagnostic.DoctorCheckContext{Tree: config.NewTree(), Store: store})

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if diags[0].Fix != orphanTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, orphanTestFix)
	}
}

// recordReadTestFix is the command a doctor run that cannot judge the record
// must name: it prints the stored record.
const recordReadTestFix = "ze data cat meta/ddos-flowspec/announcement"

// TestOrphanRecordCheckWithNoStoreNamesTheReadCommand proves that a doctor run
// with no storage refuses the record check by name and gives the operator the
// command that reads the record.
//
// VALIDATES: ai/rules/user-facing-errors.md (every finding names the command
// that clears it), for the doctor-storage-unavailable refusal.
// METHOD: checkOrphanRecord with a config tree holding no ddos flowspec section
// and a nil store.
// PREVENTS: a refusal that leaves the operator nothing to run.
func TestOrphanRecordCheckWithNoStoreNamesTheReadCommand(t *testing.T) {
	diags := checkOrphanRecord(diagnostic.DoctorCheckContext{Tree: config.NewTree()})

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorStorageUnavailable {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorStorageUnavailable)
	}
	if diags[0].Fix != recordReadTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, recordReadTestFix)
	}
}

// TestOrphanRecordCheckWithNoTreeNamesTheReadCommand proves that a doctor run
// holding no parsed configuration says it cannot judge the record and gives the
// command that reads it.
//
// VALIDATES: ai/rules/user-facing-errors.md, for the no-configuration refusal.
// METHOD: checkOrphanRecord with no tree.
// PREVENTS: a refusal that leaves the operator nothing to run, and a silent nil
// read as "no record".
func TestOrphanRecordCheckWithNoTreeNamesTheReadCommand(t *testing.T) {
	diags := checkOrphanRecord(diagnostic.DoctorCheckContext{})

	if len(diags) != 1 {
		t.Fatalf("findings = %v, want one", diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	if diags[0].Fix != recordReadTestFix {
		t.Errorf("fix = %q, want %q", diags[0].Fix, recordReadTestFix)
	}
}
