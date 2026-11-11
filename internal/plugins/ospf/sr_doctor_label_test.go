package ospf

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSRLabelDoctorReportsRefusedEntry proves the ospf-sr-label-entries check
// reports a Segment Routing label entry the writer refused, names the entry
// and the writer's cause under doctor-fib-mpls-no-writer with a fix, and
// clears it once a later apply of the same in-label succeeds.
//
// VALIDATES: AC-22 -- an SR entry no writer installs reaches `show doctor`
// with its cause, not only the log.
// METHOD: a writer on the production bus refuses the first pop; the finding
// is read through the plugin-level srLabelDoctorFindings; the writer then
// accepts and SR programs the same in-label again.
func TestSRLabelDoctorReportsRefusedEntry(t *testing.T) {
	writer := &srLabelWriter{refusal: errors.New("vpp: mpls_route_add_del retval -6")}
	fib := newSRFIB(newSRLabelBus(t, writer), mplsSourceOSPFSR)
	t.Cleanup(srLabelSources.add(fib))
	nh := netip.MustParseAddr("10.0.0.2")

	// Other tests' engines may share the process-wide set, so the answer is
	// searched for this fib's cause rather than counted.
	const cause = "retval -6"
	if got := srFindingsNaming(cause); len(got) != 0 {
		t.Fatalf("nothing refused yet, got %v", got)
	}
	fib.installAdjSID(24001, nh)

	findings := srFindingsNaming(cause)
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %v", findings)
	}
	got := findings[0]
	if got.Code != mplsfibevents.CodeDoctorNoWriter {
		t.Errorf("code %q", got.Code)
	}
	if got.Severity != "error" {
		t.Errorf("severity %q", got.Severity)
	}
	for _, want := range []string{"ospf-sr", "pop of in-label 24001", "retval -6"} {
		if !strings.Contains(got.Message, want) {
			t.Errorf("want %q in %q", want, got.Message)
		}
	}
	if got.Fix == "" {
		t.Error("the finding carries no fix")
	}

	writer.refusal = nil
	fib.installAdjSID(24001, nh)
	if got := srFindingsNaming(cause); len(got) != 0 {
		t.Fatalf("an accepted apply clears the refusal, got %v", got)
	}
}

// srFindingsNaming answers the plugin-level ospf-sr-label-entries findings
// whose message names substr.
func srFindingsNaming(substr string) []rpc.DoctorCheckDiagnostic {
	var named []rpc.DoctorCheckDiagnostic
	for _, finding := range srLabelDoctorFindings() {
		if strings.Contains(finding.Message, substr) {
			named = append(named, finding)
		}
	}
	return named
}

// TestSRLabelDoctorReportsUnreplayedEntries proves an entry a writer's replay
// did not install is reported, and a later successful replay clears it.
//
// VALIDATES: AC-22, AC-25 -- SR labels a VPP writer asked for again and did
// not install reach `show doctor` with the writer's cause.
// METHOD: a writer holding a token accepts the first apply, refuses the
// replay batch it asked for, then accepts the next replay.
func TestSRLabelDoctorReportsUnreplayedEntries(t *testing.T) {
	writer := &srLabelWriter{held: mplsfibevents.NewReplayToken()}
	fib := newSRFIB(newSRLabelBus(t, writer), mplsSourceOSPFSR)
	fib.programSwap(16005, 16105, netip.MustParseAddr("10.0.0.2"))
	if got := fib.labelDoctorFindings(); len(got) != 0 {
		t.Fatalf("an accepted apply is no finding, got %v", got)
	}

	writer.refusal = errors.New("vpp: mpls table 0 missing")
	fib.replay(&replay.Request{ReplayID: writer.held})
	findings := fib.labelDoctorFindings()
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "swap of in-label 16005") ||
		!strings.Contains(findings[0].Message, "table 0 missing") {
		t.Fatalf("want the unreplayed swap with its cause, got %v", findings)
	}

	writer.refusal = nil
	fib.replay(&replay.Request{ReplayID: writer.held})
	if got := fib.labelDoctorFindings(); len(got) != 0 {
		t.Fatalf("a successful replay clears the finding, got %v", got)
	}
}

// TestSRLabelDoctorReportsNoEventBus proves an SR producer with no event bus
// (OSPF run as a forked process, where the engine hands it none) reports the
// entries it cannot program rather than only logging them.
//
// VALIDATES: an SR label that can never reach a writer is a doctor finding.
// METHOD: an srFIB built with a nil bus programs a pop.
func TestSRLabelDoctorReportsNoEventBus(t *testing.T) {
	fib := newSRFIB(nil, mplsSourceOSPFv3SR)
	fib.installAdjSID(24002, netip.MustParseAddr("fe80::2"))
	findings := fib.labelDoctorFindings()
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "ospfv3-sr") ||
		!strings.Contains(findings[0].Message, "no event bus") {
		t.Fatalf("want the unprogrammable pop, got %v", findings)
	}
	fib.withdrawAdjSID(24002)
	if got := fib.labelDoctorFindings(); len(got) != 0 {
		t.Fatalf("a withdrawn entry SR never installed is no finding, got %v", got)
	}
}
