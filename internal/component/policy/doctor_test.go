package policy

import (
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// A flooding source import beside rib_test.go's non-flooding one, so the doctor
// check's choice of sources is judged against both answers.
const (
	testFloodRoot     = "test-flood-src"
	testFloodAttach   = "test-flood-src import"
	testFloodProtocol = "test-flood-src-proto"
)

func init() {
	MustRegisterRouteAttachment(testFloodRoot, testFloodAttach, []Field{FieldPrefix, FieldFamily}, nil)
	MustRegisterRIBImport(RIBImport{Attachment: testFloodAttach, Root: testFloodRoot, Scope: RIBScopeSource, Protocol: testFloodProtocol, Floods: true})
}

type doctorChain func(r ribpolicy.Route) ribpolicy.Verdict

// TestDoctorFloodingAttachments proves the check judges only the sources whose
// RIBImport floods: the flooding test source is listed under its attachment,
// and neither the non-flooding source nor the central chain is.
func TestDoctorFloodingAttachments(t *testing.T) {
	flooding := floodingAttachments()
	if got := flooding[redistevents.RegisterProtocol(testFloodProtocol)]; got != testFloodAttach {
		t.Errorf("flooding source maps to %q, want %q", got, testFloodAttach)
	}
	if got, listed := flooding[redistevents.RegisterProtocol(testProtocol)]; listed {
		t.Errorf("non-flooding source %s is judged, under %q", testProtocol, got)
	}
	central := centralAttachment()
	if central == "" {
		t.Fatal("no central import is registered, so the central exclusion is not tested")
	}
	for id, attachment := range flooding {
		if attachment == central {
			t.Errorf("central chain %q is judged as flooding source %d", central, id)
		}
	}
}

func (f doctorChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict { return f(r) } //nolint:gocritic // hugeParam: the Chain signature passes Route by value

// TestDoctorIGPImportHidesNextHop is R-4: an IGP import chain hides
// 10.1.0.0/24 while a selected BGP route's next hop 10.1.0.5 now resolves
// through the IGP's 10.0.0.0/8, so the check warns naming the hidden prefix, the
// chain, the next hop and the fix. A next hop no hidden prefix covers, and one
// a longer visible prefix still resolves, raise nothing.
func TestDoctorIGPImportHidesNextHop(t *testing.T) {
	igp := redistevents.RegisterProtocol("test-doctor-igp")
	bgp := redistevents.RegisterProtocol("test-doctor-bgp")
	hiddenPrefix := netip.MustParsePrefix("10.1.0.0/24")
	snap, err := ribpolicy.NewSnapshot(map[redistevents.ProtocolID]ribpolicy.Chain{
		igp: doctorChain(func(r ribpolicy.Route) ribpolicy.Verdict {
			if r.Prefix == hiddenPrefix {
				return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
			}
			return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
		}),
	}, nil)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	ribpolicy.Set(snap)
	t.Cleanup(func() { ribpolicy.Set(nil) })

	flooding := map[redistevents.ProtocolID]string{igp: "test-doctor-igp import"}
	insert := func(rib *locrib.RIB, prefix string, source redistevents.ProtocolID, nextHop string) {
		rib.Insert(family.IPv4Unicast, netip.MustParsePrefix(prefix), locrib.Path{Source: source, NextHop: netip.MustParseAddr(nextHop)})
	}

	rib := locrib.NewRIB()
	insert(rib, "10.1.0.0/24", igp, "192.0.2.1")
	insert(rib, "10.0.0.0/8", igp, "192.0.2.1")
	insert(rib, "203.0.113.0/24", bgp, "10.1.0.5")
	insert(rib, "198.51.100.0/24", bgp, "10.2.0.5")

	diags := igpImportHidesNextHop(rib, flooding)
	if len(diags) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(diags), diags)
	}
	d := diags[0]
	if d.Code != diagnostic.CodeDoctorPolicyIGPImportHidesNextHop || d.Severity != diagnostic.SeverityWarning {
		t.Fatalf("finding %s/%v, want %s warning", d.Code, d.Severity, diagnostic.CodeDoctorPolicyIGPImportHidesNextHop)
	}
	for _, want := range []string{"10.1.0.0/24", "test-doctor-igp import", "10.1.0.5", "203.0.113.0/24"} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("message %q does not name %s", d.Message, want)
		}
	}
	if !strings.Contains(d.Help, "show rib candidates prefix 10.1.0.0/24") {
		t.Errorf("help %q does not give the command that shows the hidden path", d.Help)
	}

	// A longer visible prefix still resolves the next hop: no blackhole.
	covered := locrib.NewRIB()
	insert(covered, "10.1.0.0/24", igp, "192.0.2.1")
	insert(covered, "10.1.0.0/25", bgp, "192.0.2.2")
	insert(covered, "203.0.113.0/24", bgp, "10.1.0.5")
	if diags := igpImportHidesNextHop(covered, flooding); len(diags) != 0 {
		t.Fatalf("a next hop a longer visible prefix resolves raised %+v", diags)
	}

	// A source that does not flood is not judged.
	if diags := igpImportHidesNextHop(rib, map[redistevents.ProtocolID]string{bgp: "bgp"}); len(diags) != 0 {
		t.Fatalf("a hidden path of a non-flooding source raised %+v", diags)
	}
}

// TestDoctorIGPImportRegistered proves the check is on the doctor registry
// under its code, so `show doctor` runs it.
func TestDoctorIGPImportRegistered(t *testing.T) {
	for _, c := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if c.Name != igpImportDoctorCheck.Name {
			continue
		}
		if slices.Contains(c.Codes, diagnostic.CodeDoctorPolicyIGPImportHidesNextHop) {
			return
		}
		t.Fatalf("check %s does not list %s", c.Name, diagnostic.CodeDoctorPolicyIGPImportHidesNextHop)
	}
	t.Fatalf("doctor check %s is not registered", igpImportDoctorCheck.Name)
}
